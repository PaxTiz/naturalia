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
	"strings"
)

// Bootcamps (docs/TELEPORTEURS.md), and the other buildings set down
// anywhere (docs/BATIMENTS.md), are entities whose kind lays out a building
// (content.EntityDef.Camp): the base's code builds it once per kind in its
// own frame (base.LayCamp), and each one stands it where it was placed
// (base.Site). What it is for comes from what it holds: one travels to
// those with a hologram, and wakes up in those with a bed.

// Bootcamp is what a building keeps: an ID that stays the same from save
// to save, and its name.
type Bootcamp struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	// Level is the altitude of its floor, that of the building it was set
	// down next to; the ground's there if left out.
	Level *float64  `json:"level,omitempty"`
	site  base.Site // where it stands, set when it is placed (placeCamp)
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
		c = base.LayCamp(&g.Content.Kits, *def.Camp, def.Ports)
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
	for id, b := range g.Ents.Camp.All() {
		if l := g.campLayout(g.kind(id)); l != nil {
			p := planet.Pad{Dir: g.Ents.Transform.Get(id).Dir, Radius: l.Reach + padMargin}
			if b.Level != nil {
				p.Level, p.Set = *b.Level, true
			}
			pads = append(pads, p)
		}
	}
	g.reshape(func() { g.World.Relief.SetPads(pads) })
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
	return g.Ents.Camp.Set(id, Bootcamp{ID: next, Name: g.campName(g.kind(id))})
}

// campName is a new building's name: its kind's, and the first number no
// other has.
func (g *Game) campName(kind string) string {
	what := "Bootcamp"
	if def, ok := g.Content.Entities.Get(kind); ok {
		what = content.OrDefault(def.Name, what)
	}
	for n := 1; ; n++ {
		name, free := fmt.Sprintf("%s %d", what, n), true
		for _, b := range g.Ents.Camp.All() {
			free = free && !strings.EqualFold(b.Name, name)
		}
		if free {
			return name
		}
	}
}

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

// CampDoor is how open the door of a bootcamp's port i is, 0 closed to 1
// open.
func (g *Game) CampDoor(id ecs.ID, i int) float64 { return g.doors[doorKey{id, i}].open }

// updateCampDoors slides the doors of the bootcamps near open when the
// player comes close, as the base's airlock.
func (g *Game) updateCampDoors(dt float64) {
	for _, c := range g.CampsNear(g.Player.Pos, 30) {
		at := c.Site.ToLocal(g.Player.Pos)
		for i, p := range c.Layout.Ports {
			g.slideDoor(doorKey{c.Ent, i}, at.Sub(p.At()).Length() < 4, dt)
		}
	}
}

// CampLayout is how a kind of bootcamp is laid out, nil for what isn't
// one.
func (g *Game) CampLayout(kind string) *base.Camp { return g.campLayout(kind) }

const (
	campFromBase = 40.0 // m kept between a bootcamp and the base's buildings
	campGap      = 3.0  // m between two buildings, at least
	maxCampSlope = 2.0  // m the flattened ground may make up, at most
	maxCampBank  = 0.25 // the slope of the ground between two buildings, at most
	campTerrace  = 1.2  // m a building is raised or dug to be level with the one next to it
	padMargin    = 0.5  // m of flat ground around a building
)

// neighbours goes through the buildings whose flat ground meets that of one
// laid out as l at dir, but moving: how far apart they are, and the altitude
// of their floor.
func (g *Game) neighbours(l *base.Camp, dir vec3, moving ecs.ID, see func(id ecs.ID, gap, level float64)) {
	for id, b := range g.Ents.Camp.All() {
		o := g.campLayout(g.kind(id))
		if id == moving || o == nil {
			continue
		}
		if gap := planet.ArcDist(dir, b.site.Origin.Norm()) - l.Reach - o.Reach; gap < planet.PadBlend {
			see(id, gap, b.site.Origin.Length()-planet.Radius)
		}
	}
}

// campLevel is the altitude of the floor of a building laid out as l at
// dir: that of the nearest around if the ground is about as high, so that
// they are level; else the ground's, nil.
func (g *Game) campLevel(l *base.Camp, dir vec3, moving ecs.ID) *float64 {
	ground, near := g.World.Relief.Layers(dir), math.Inf(1)
	var out *float64
	g.neighbours(l, dir, moving, func(_ ecs.ID, gap, level float64) {
		if math.Abs(level-ground) <= campTerrace && gap < near {
			near, out = gap, &level
		}
	})
	return out
}

// campFits tells whether a building laid out as l would stand at dir, its
// floor at level (the ground's if nil): away from the base and from the
// others (but moving, the one moved), on ground flat enough, with nothing in
// its way; if not, why.
func (g *Game) campFits(l *base.Camp, dir vec3, level *float64, moving ecs.ID) (bool, string) {
	if planet.ArcDist(dir, planet.BaseUp) < base.Extent+campFromBase+l.Reach {
		return false, "trop près de la base"
	}
	h := g.World.Relief.Layers(dir)
	if level != nil {
		h = *level
	}
	why := ""
	g.neighbours(l, dir, moving, func(_ ecs.ID, gap, other float64) {
		switch {
		case gap < campGap:
			why = "trop près d'un bâtiment"
		case why == "" && math.Abs(h-other) > maxCampBank*gap:
			why = "trop haut ou trop bas pour le bâtiment voisin"
		}
	})
	if why != "" {
		return false, why
	}
	if g.World.Relief.PadSlope(dir, l.Reach+padMargin, h) > maxCampSlope {
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
	gh.floor = g.campLevel(l, dir, gh.Moving)
	gh.OK, gh.Why = g.campFits(l, dir, gh.floor, gh.Moving)
	if gh.floor != nil { // shown as high as it will stand
		gh.Pose.Alt = planet.Radius + *gh.floor - g.World.Relief.Surface(dir).Length()
	}
}

// rayCamp is how far along a ray from o along d (length 1), within max,
// it meets bootcamp c's walls or dome; never from inside it.
func rayCamp(c CampAt, o, d vec3, max float64) (float64, bool) {
	lo, ld := c.Site.ToLocal(o), c.Site.DirLocal(d)
	if c.Layout.Indoors(lo) {
		return 0, false
	}
	return c.Layout.Ray(lo, ld, max)
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
