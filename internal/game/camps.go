package game

import (
	"cmp"
	"fmt"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/physics"
	"naturalia/internal/planet"
	"slices"
)

// Bootcamps (docs/TELEPORTEURS.md) are entities whose kind lays out a
// small dome (content.EntityDef.Camp): the base's code builds it once per
// kind in its own frame (base.LayCamp), and each one stands it where it
// was placed (base.Site).

// Bootcamp is what a bootcamp keeps: an ID that stays the same from save
// to save, and its name.
type Bootcamp struct {
	ID   int       `json:"id"`
	Name string    `json:"name"`
	site base.Site // where it stands, set when it is placed (placeCamp)
}

// CampAt is a bootcamp standing on Mars: its entity, what it keeps, where
// it stands and how it is laid out.
type CampAt struct {
	Ent ecs.ID
	*Bootcamp
	Site   base.Site
	Layout *base.Camp
}

// ToWorld takes a point of the bootcamp's frame to the world.
func (c CampAt) ToWorld(local vec3) vec3 { return c.Site.ToWorld(local) }

// campLayout is how a kind of bootcamp is laid out, figured once; nil for
// what isn't one.
func (g *Game) campLayout(kind string) *base.Camp {
	def, ok := g.Content.Entities.Get(kind)
	if !ok || def.Camp == nil {
		return nil
	}
	c, ok := g.layouts[def.ID]
	if !ok {
		c = base.LayCamp(&g.Content.Kits, *def.Camp)
		if g.layouts == nil {
			g.layouts = map[string]*base.Camp{}
		}
		g.layouts[def.ID] = c
	}
	return c
}

// Camps are the bootcamps standing, in the order they were first placed;
// the one moved isn't.
func (g *Game) Camps() []CampAt {
	var out []CampAt
	for id, b := range g.Ents.Camp.All() {
		l := g.campLayout(g.kind(id))
		if l == nil || g.Held(id) {
			continue
		}
		out = append(out, CampAt{id, b, b.site, l})
	}
	slices.SortFunc(out, func(a, b CampAt) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// CampsNear are the bootcamps within dist of a point of the world, by
// their walls.
func (g *Game) CampsNear(at vec3, dist float64) []CampAt {
	var out []CampAt
	for _, c := range g.Camps() {
		if planet.ArcDist(at.Norm(), c.Site.Origin.Norm()) < dist+c.Layout.Reach {
			out = append(out, c)
		}
	}
	return out
}

// CampAround is the bootcamp a point of the world is inside, if any.
func (g *Game) CampAround(at vec3) (CampAt, bool) {
	for _, c := range g.CampsNear(at, 1) {
		if c.Layout.Indoors(c.Site.ToLocal(at)) {
			return c, true
		}
	}
	return CampAt{}, false
}

// Indoors tells whether a point of the world is inside the base or a
// bootcamp.
func (g *Game) Indoors(at vec3) bool {
	if base.Indoors(base.ToLocal(at)) {
		return true
	}
	_, ok := g.CampAround(at)
	return ok
}

// buildingAt is the ID of the building of the base whose room holds a
// local point.
func buildingAt(local vec3) string {
	if id := base.RoomAt(local); id != 0 {
		return base.Ents.Building.Get(id).Plan().ID
	}
	return ""
}

// buildingAt is the ID of the building a point of the world is in: a
// room of the base, or a bootcamp (its kind's); "" outside.
func (g *Game) buildingAt(at vec3) string {
	if in := buildingAt(base.ToLocal(at)); in != "" {
		return in
	}
	if c, ok := g.CampAround(at); ok {
		return g.kind(c.Ent)
	}
	return ""
}

// standCamps flattens the ground under every bootcamp, the one moved
// too, and stands each upright on it.
func (g *Game) standCamps() {
	var pads []planet.Pad
	for id := range g.Ents.Camp.All() {
		if l := g.campLayout(g.kind(id)); l != nil {
			pads = append(pads, planet.Pad{Dir: g.Ents.Transform.Get(id).Dir, Radius: l.Reach + 0.5})
		}
	}
	g.World.Relief.SetPads(pads)
	for id, b := range g.Ents.Camp.All() {
		t := g.Ents.Transform.Get(id)
		t.Alt, t.Pitch, t.Roll = 0, 0, 0
		pos, ax, up, az := g.Frame(t)
		b.site = base.Site{Origin: pos, X: ax, Y: up, Z: az}
	}
	g.relay = true // their walls, laid again
}

// newCamp makes entity id a bootcamp, with the next ID and a name.
func (g *Game) newCamp(id ecs.ID) *Bootcamp {
	next := 1
	for _, b := range g.Ents.Camp.All() {
		next = max(next, b.ID+1)
	}
	return g.Ents.Camp.Set(id, Bootcamp{ID: next, Name: campName(next)})
}

// campName is a new bootcamp's name.
func campName(n int) string { return fmt.Sprintf("Bootcamp %d", n) }

// heldCamp is the bootcamp being moved, 0 for none.
func (g *Game) heldCamp() ecs.ID {
	if id := g.held(); g.Ents.Camp.Has(id) {
		return id
	}
	return 0
}

// Kind is entity id's kind, "" for none.
func (g *Game) Kind(id ecs.ID) string { return g.kind(id) }

// isCamp tells whether an entity's kind is a bootcamp.
func isCamp(def *content.EntityDef) bool { return def != nil && def.Camp != nil }

// CampDoor is how open a bootcamp's door is, 0 closed to 1 open.
func (g *Game) CampDoor(id ecs.ID) float64 { return g.campDoors[id].open }

// campDoor is a bootcamp's door: how open, and where it is going.
type campDoor struct{ open, want float64 }

// updateCampDoors slides the doors of the bootcamps near open when the
// player comes close, as the base's airlock.
func (g *Game) updateCampDoors(dt float64) {
	if g.campDoors == nil {
		g.campDoors = map[ecs.ID]campDoor{}
	}
	for _, c := range g.CampsNear(g.Player.Pos, 30) {
		door, _ := c.Layout.Door()
		d := g.campDoors[c.Ent]
		want := 0.0
		if c.Site.ToLocal(g.Player.Pos).Sub(door).Length() < 4 {
			want = 1
		}
		if want != d.want {
			d.want = want
			g.sfx.Airlock()
		}
		d.open += (want - d.open) * math.Min(1, dt*4)
		g.campDoors[c.Ent] = d
	}
}

// CampLayout is how a kind of bootcamp is laid out, nil for what isn't
// one.
func (g *Game) CampLayout(kind string) *base.Camp { return g.campLayout(kind) }

const (
	campFromBase = 40.0 // m kept between a bootcamp and the base's buildings
	maxCampSlope = 2.0  // m the flattened ground may make up, at most
)

// campFits tells whether a bootcamp laid out as l would stand at dir:
// away from the base and from the other bootcamps (but moving, the one
// moved), on ground flat enough, with nothing in its way; if not, why.
func (g *Game) campFits(l *base.Camp, dir vec3, moving ecs.ID) (bool, string) {
	if planet.ArcDist(dir, planet.BaseUp) < base.Extent+campFromBase+l.Reach {
		return false, "trop près de la base"
	}
	for id, b := range g.Ents.Camp.All() {
		if o := g.campLayout(g.kind(id)); id != moving && o != nil &&
			planet.ArcDist(dir, b.site.Origin.Norm()) < l.Reach+o.Reach+planet.PadBlend {
			return false, "trop près d'un bootcamp"
		}
	}
	if g.World.Relief.PadSlope(dir, l.Reach+0.5) > maxCampSlope {
		return false, "sol trop pentu"
	}
	if p := g.phys; p != nil {
		at := g.World.Relief.Surface(dir)
		bottom, top := p.slice(at, 4)
		if p.w.Overlap(physics.Circle(l.Reach), p.plane(at), 0, bottom, top, layerEntity|layerVehicle|layerRobot|layerProp|layerPlayer, nil) {
			return false, "quelque chose gêne"
		}
	}
	return true, ""
}

// poseCamp sets a bootcamp's ghost where the player looks: its near edge
// on the aimed ground, its door toward them, upright, turned only around
// its up.
func (g *Game) poseCamp(gh *Ghost, l *base.Camp) {
	p := &g.Player
	eye, fwd := p.Eye(), p.Forward()
	hit, ok := g.World.Relief.Raycast(eye, fwd, buildReach)
	if !ok {
		hit = g.World.Relief.Surface(eye.Add(planet.TangentAt(p.LocalUp(), fwd).Scale(buildReach)))
	}
	n := hit.Norm()
	dir := planet.Toward2(n, planet.TangentAt(n, p.Heading), l.Reach)
	face := planet.TangentAt(dir, p.Heading).Scale(-1)
	side := face.Cross(dir)
	r := gh.rel[0]
	ax := face.Scale(r.X).Add(side.Scale(r.Z)).Norm()
	gh.Pose = Transform{Dir: dir}
	gh.Pose.SetAxes(ax, dir)
	gh.OK, gh.Why = g.campFits(l, dir, gh.Moving)
}

// rayCamp is how far along a ray from o along d (length 1), within max,
// it meets bootcamp c's walls or dome, seen as a round tower; never from
// inside it.
func rayCamp(c CampAt, o, d vec3, max float64) (float64, bool) {
	lo, ld := c.Site.ToLocal(o), c.Site.DirLocal(d)
	r := c.Layout.Room
	top := r.WallH + r.Radius*0.55
	if c.Layout.Indoors(lo) {
		return 0, false
	}
	// x² + z² = R² along the ray, seen from above.
	a := ld.X*ld.X + ld.Z*ld.Z
	b := 2 * (lo.X*ld.X + lo.Z*ld.Z)
	k := lo.X*lo.X + lo.Z*lo.Z - r.Radius*r.Radius
	if a < 1e-12 || b*b-4*a*k < 0 {
		return 0, false
	}
	t := (-b - math.Sqrt(b*b-4*a*k)) / (2 * a)
	if y := lo.Y + t*ld.Y; t < 0 || t > max || y < 0 || y > top {
		return 0, false
	}
	return t, true
}

// ShotCamp sets a bootcamp of kind dist m from the base along a bearing
// (degrees from +X toward +Z), its door toward the base, for screenshots.
func (g *Game) ShotCamp(kind string, dist, bearing float64) (CampAt, bool) {
	a := bearing * math.Pi / 180
	dir := planet.Toward(vec3{math.Cos(a), 0, math.Sin(a)}, dist)
	id := g.add(Placed{Kind: kind, Dir: dir})
	t := g.Ents.Transform.Get(id)
	t.SetAxes(planet.TangentAt(dir, planet.BaseUp.Sub(dir)), dir)
	g.standCamps()
	for _, c := range g.Camps() {
		if c.Ent == id {
			return c, true
		}
	}
	return CampAt{}, false
}

// WakeSpot is where the player wakes up, and the way they face: by the
// bed slept in last, the base's if that bootcamp is gone.
func (g *Game) WakeSpot() (at, heading vec3) {
	for _, c := range g.Camps() {
		if c.ID == g.State.Wake && g.State.Wake != 0 {
			w := c.Layout.Wake
			return c.ToWorld(vec3{w.X, base.FloorTop, w.Z}), c.Site.Dir(vec3{-w.X, 0, -w.Z}.Norm())
		}
	}
	return base.ToWorld(base.SpawnSpot), vec3{0, 0, -1}
}
