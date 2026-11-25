package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/mathx"
	"naturalia/internal/mesh"
	"naturalia/internal/physics"
	"naturalia/internal/planet"
	"slices"
)

// Blocks (docs/CONSTRUCTION.md) are entities to build with: fixed where
// they were set down, turned any way. Rays find their faces from the shape
// of their parts: boxes, turned around their up, and wedges.

// solid is a convex shape in an entity's frame: the points x with
// n·x <= d for all its planes; pts are its corners.
type solid struct {
	planes []plane
	pts    []vec3
}

type plane struct {
	n vec3
	d float64
}

// solidsOf are the shapes of an entity's parts (or of its Shape), figured
// once by kind; but for a block's, a prism counts as the box around it.
func (g *Game) solidsOf(def *content.EntityDef) []solid {
	if s, ok := g.solids[def.ID]; ok {
		return s
	}
	var out []solid
	parts := def.Parts
	if len(def.Shape) > 0 {
		parts = def.Shape
	}
	for _, p := range parts {
		a := p.Yaw * math.Pi / 180
		ex, ey := vec3{math.Cos(a), 0, math.Sin(a)}, vec3{0, 1, 0}
		ez := ex.Cross(ey)
		switch p.Shape {
		case "wedge": // standing on at, rising toward +x
			x, y, z := p.Size.X/2, p.Size.Y, p.Size.Z/2
			slope := ey.Scale(1).Sub(ex.Scale(y / p.Size.X)).Norm() // up the slope's face
			out = append(out, solid{[]plane{
				{ey.Scale(-1), -ey.Dot(p.At)},
				{ex, ex.Dot(p.At) + x},
				{ez, ez.Dot(p.At) + z},
				{ez.Scale(-1), -ez.Dot(p.At) + z},
				{slope, slope.Dot(p.At.Add(ey.Scale(y / 2)))},
			}, p.Corners()})
		case "hull":
			out = append(out, hullSolid(p.Corners()))
		case "prism":
			if def.Build {
				out = append(out, hullSolid(p.Corners()))
				break
			}
			lo, hi := content.Bounds([]content.PartJSON{p})
			out = append(out, boxSolid(lo.Add(hi).Scale(0.5), [3]vec3{{1, 0, 0}, ey, {0, 0, 1}}, hi.Sub(lo).Scale(0.5)))
		default:
			out = append(out, boxSolid(p.At, [3]vec3{ex, ey, ez}, p.Size.Scale(0.5)))
		}
	}
	if g.solids == nil {
		g.solids = map[string][]solid{}
	}
	g.solids[def.ID] = out
	return out
}

func boxSolid(c vec3, ax [3]vec3, h vec3) solid {
	half := [3]float64{h.X, h.Y, h.Z}
	var s solid
	for i, a := range ax {
		s.planes = append(s.planes, plane{a, a.Dot(c) + half[i]}, plane{a.Scale(-1), -a.Dot(c) + half[i]})
	}
	for i := range 8 {
		p := c
		for k, a := range ax {
			p = p.Add(a.Scale(half[k] * float64(i>>k&1*2-1)))
		}
		s.pts = append(s.pts, p)
	}
	return s
}

// hullSolid is the convex hull of points.
func hullSolid(pts []vec3) solid {
	s := solid{pts: pts}
	for _, f := range mesh.HullFaces(pts) {
		s.planes = append(s.planes, plane{f.N, f.D})
	}
	return s
}

// hit is where a ray from o along d (length 1) comes into the solid,
// within max, and the face's normal.
func (s solid) hit(o, d vec3, max float64) (float64, vec3, bool) {
	in, out := math.Inf(-1), max
	var n vec3
	for _, p := range s.planes {
		toward, room := p.n.Dot(d), p.d-p.n.Dot(o)
		if math.Abs(toward) < 1e-12 {
			if room < 0 {
				return 0, vec3{}, false
			}
			continue
		}
		t := room / toward
		if toward < 0 { // coming in through this plane
			if t > in {
				in, n = t, p.n
			}
		} else {
			out = math.Min(out, t)
		}
	}
	if in > out || in < 0 || len(s.planes) == 0 {
		return 0, vec3{}, false
	}
	return in, n, true
}

// contains tells if x is inside the solid.
func (s solid) contains(x vec3) bool {
	for _, p := range s.planes {
		if p.n.Dot(x) > p.d {
			return false
		}
	}
	return len(s.planes) > 0
}

// local is an entity's shape and frame, to turn world points into it.
func (g *Game) local(id ecs.ID) (solids []solid, toLocal func(v vec3) vec3, toWorld func(v vec3) vec3, pos vec3, ok bool) {
	def, ok := g.Content.Entities.Get(g.kind(id))
	if !ok {
		return nil, nil, nil, vec3{}, false
	}
	pos, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
	toLocal = func(v vec3) vec3 { return vec3{v.Dot(ax), v.Dot(up), v.Dot(az)} }
	toWorld = func(v vec3) vec3 { return ax.Scale(v.X).Add(up.Scale(v.Y)).Add(az.Scale(v.Z)) }
	return g.solidsOf(def), toLocal, toWorld, pos, true
}

// rayEntity is where a ray from o along d (length 1) meets an entity's
// parts, within max, and the face's normal, in the world; nothing from
// inside one of them.
func (g *Game) rayEntity(id ecs.ID, o, d vec3, max float64) (float64, vec3, bool) {
	solids, toLocal, toWorld, pos, ok := g.local(id)
	if !ok {
		return 0, vec3{}, false
	}
	lo, ld := toLocal(o.Sub(pos)), toLocal(d)
	for _, s := range solids {
		if s.contains(lo) {
			return 0, vec3{}, false
		}
	}
	best, normal, found := max, vec3{}, false
	for _, s := range solids {
		if t, n, ok := s.hit(lo, ld, best); ok {
			best, normal, found = t, n, true
		}
	}
	return best, toWorld(normal), found
}

// clearAbove tells if nothing of entity id is at o, nor above it along up
// within h.
func (g *Game) clearAbove(id ecs.ID, o, up vec3, h float64) bool {
	solids, toLocal, _, pos, ok := g.local(id)
	if !ok {
		return true
	}
	lo, lu := toLocal(o.Sub(pos)), toLocal(up)
	for _, s := range solids {
		if s.contains(lo) {
			return false
		}
		if _, _, ok := s.hit(lo, lu, h); ok {
			return false
		}
	}
	return true
}

// aimBlock is the nearest block a ray from o along d meets within max:
// which, where, and the face's normal; with things, crates count too.
func (g *Game) aimBlock(o, d vec3, max float64, things bool) (ecs.ID, vec3, vec3, bool) {
	var best ecs.ID
	var normal vec3
	for _, id := range g.EntitiesNear(max + 12) {
		if def, ok := g.Content.Entities.Get(g.kind(id)); !ok || !(def.Build || things && def.Mass > 0) || g.Held(id) {
			continue
		}
		if t, n, ok := g.rayEntity(id, o, d, max); ok {
			max, best, normal = t, id, n
		}
	}
	return best, o.Add(d.Scale(max)), normal, best != 0
}

// How high a step the player walks up without jumping, and a driven
// vehicle rolls up.
const (
	stepUp  = 0.45
	carStep = 0.6
)

// topUnder is the highest top of a block (with things, of a crate too)
// straight under at, below from (both from Mars' center), but for skip; 0
// for none.
func (g *Game) topUnder(at vec3, from float64, things bool, skip ecs.ID) float64 {
	start := at.Norm().Scale(from)
	reach := g.World.Relief.Altitude(start)
	if reach <= 0 {
		return 0
	}
	top := 0.0
	for id, t := range g.Ents.Transform.All() {
		if id == skip || planet.ArcDist(start.Norm(), t.Dir) > 15 {
			continue
		}
		def, ok := g.Content.Entities.Get(g.kind(id))
		if !ok || !(def.Build || things && def.Mass > 0) || g.Ents.Air.Has(id) || id == g.held() {
			continue
		}
		if d, _, ok := g.rayEntity(id, start, at.Norm().Scale(-1), reach); ok {
			top = math.Max(top, from-d)
		}
	}
	return top
}

// floorUnder is what stands under feet at pos, as high over the ground as
// the highest top of a block (or a crate) there, no higher than a step up:
// 0 on the bare ground. It looks a bit around the feet, so the player
// stands on an edge.
func (g *Game) floorUnder(pos vec3) float64 {
	t1, t2 := mathx.TangentBasis(pos.Norm())
	atBase := planet.ArcDist(pos.Norm(), planet.BaseUp) < base.Extent
	floor := 0.0
	for _, off := range [...]vec3{{}, t1.Scale(0.22), t1.Scale(-0.22), t2.Scale(0.22), t2.Scale(-0.22)} {
		at := pos.Add(off)
		if top := g.topUnder(at, pos.Length()+stepUp, true, 0); top > 0 {
			floor = math.Max(floor, top-g.World.Relief.Surface(at).Length())
		}
		floor = math.Max(floor, g.roofUnder(at))
		// The base's floors: a mezzanine, its stairs.
		if !atBase {
			continue
		}
		if top, ok := base.TopUnder(base.ToLocal(at).Add(vec3{0, stepUp, 0})); ok {
			floor = math.Max(floor, top-base.ToLocal(g.World.Relief.Surface(at)).Y)
		}
	}
	return floor
}

// ceilingOver is how high over the ground at pos the lowest of the blocks
// and of the base's floors over the player's head starts: 0 for none.
func (g *Game) ceilingOver(pos vec3) float64 {
	from := pos.Add(pos.Norm().Scale(stepUp + 0.05))
	low, found := 2*personH, false
	for id, t := range g.Ents.Transform.All() {
		if planet.ArcDist(pos.Norm(), t.Dir) > 15 {
			continue
		}
		if def, ok := g.Content.Entities.Get(g.kind(id)); !ok || !def.Build || g.Held(id) {
			continue
		}
		if d, _, ok := g.rayEntity(id, from, pos.Norm(), low); ok {
			low, found = d, true
		}
	}
	if found {
		return g.World.Relief.Altitude(from) + low - 0.05 // the head just short of it
	}
	if planet.ArcDist(pos.Norm(), planet.BaseUp) >= base.Extent {
		return 0
	}
	c := base.Ceiling(base.ToLocal(pos).Add(vec3{0, 0.5, 0}))
	if math.IsInf(c, 1) {
		return 0
	}
	return c - base.ToLocal(g.World.Relief.Surface(pos)).Y
}

// allow tells the engine whether the walking player, or the vehicle they
// drive, bumps into a block: not if, under them and where they meet it,
// the block is at most a step up, with nothing of it above up to their
// top.
func (g *Game) allow(a, b *physics.Body, m *physics.Manifold) bool {
	p := g.phys
	n := m.Normal // from a to b
	if b == p.player || b == p.car {
		a, b, n = b, a, n.Scale(-1)
	}
	if a.Layer == layerProp || b.Layer == layerProp {
		return !g.rests(a, b) && !g.rests(b, a)
	}
	if (a != p.player && a != p.car) || b.Kind != physics.Static {
		return true
	}
	if f, ok := p.floors[b]; ok {
		return a == p.car || !g.walkOnto(f, a, n)
	}
	if b.ID == 0 {
		return true
	}
	id := ecs.ID(b.ID)
	if def, ok := g.Content.Entities.Get(g.kind(id)); !ok || !def.Build {
		return true
	}
	feet, step, tall := p.r0+a.Bottom-personStep, stepUp, personH // from Mars' center
	look := []physics.V2{a.Pos, a.Pos.Add(n.Scale(a.Shape.Radius + 0.2))}
	if a == p.car {
		if g.drive.r <= 0 {
			return true
		}
		feet, step, tall = g.drive.r, carStep, a.Top-a.Bottom
		look = look[:1]
		for i := range m.N {
			look = append(look, m.Points[i].Point.Add(n.Scale(0.2))) // into the block, past its rim
		}
	}
	for _, at := range look {
		dir, h := p.dir(at), feet
		if a == p.car { // its front is up the slope it rolls on
			r := g.Player.Pos.Length()
			// From where its height is known: this step moves it on already.
			top, _ := g.rollTo(g.Player.Pos, p.dir(at.Sub(n.Scale(0.5))).Scale(r), feet)
			h = math.Max(feet, top)
			// Its corners, over the edge of what it rolls on, go up with it.
			ahead := at.Sub(a.Pos).Dot(physics.V2{X: math.Cos(a.Angle), Y: math.Sin(a.Angle)})
			nose := a.Pos.Add(physics.V2{X: math.Cos(a.Angle), Y: math.Sin(a.Angle)}.Scale(ahead - math.Copysign(math.Min(0.5, math.Abs(ahead)), ahead)))
			top, _ = g.rollTo(g.Player.Pos, p.dir(nose).Scale(r), feet)
			h = math.Max(h, top)
		}
		if !g.clearAbove(id, dir.Scale(h+step), dir, tall-step) {
			return true
		}
	}
	return false
}

// walkOnto tells if the walking player, its body a, steps onto floor f
// going along n: where they stand and just ahead, the floor is at most a
// step up, or not there; and if it is high, it is above their head.
func (g *Game) walkOnto(f base.Floor, a *physics.Body, n physics.V2) bool {
	p := g.phys
	feet := p.r0 + a.Bottom - personStep // from Mars' center
	for _, at := range []physics.V2{a.Pos, a.Pos.Add(n.Scale(a.Shape.Radius + 0.2))} {
		l := base.ToLocal(p.dir(at).Scale(feet))
		if top, over := f.TopAt(l); over && top > l.Y+stepUp && f.Bottom() < l.Y+personH {
			return false
		}
	}
	return true
}

// addBlock adds a block's bodies: one, or one for each part of its Shape.
// It returns the first.
func (g *Game) addBlock(id ecs.ID, def *content.EntityDef) *physics.Body {
	const round = 0.02
	pos, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
	if len(def.Shape) == 0 {
		return g.phys.w.Add(g.blockBody(id, pos, ghostCorners(def, ax, up, round), round))
	}
	var first *physics.Body
	for _, s := range g.solidsOf(def) {
		var corners []vec3
		var mid vec3
		for _, c := range s.pts {
			mid = mid.Add(c.Scale(1 / float64(len(s.pts))))
		}
		for _, c := range s.pts {
			in := c.Sub(mid)
			c = c.Sub(in.Norm().Scale(math.Min(round, in.Length()/2)))
			corners = append(corners, ax.Scale(c.X).Add(up.Scale(c.Y)).Add(az.Scale(c.Z)))
		}
		b := g.phys.w.Add(g.blockBody(id, pos, corners, round))
		if first == nil {
			first = b
		}
	}
	return first
}

// dropBodies takes entity id's bodies out of the collisions.
func (g *Game) dropBodies(id ecs.ID) {
	if bp := g.Ents.Body.Get(id); bp != nil && g.phys != nil {
		for _, b := range slices.Clone(g.phys.w.Bodies()) {
			if b == *bp || b.ID == uint32(id) {
				g.phys.w.Remove(b)
			}
		}
	}
	g.Ents.Body.Remove(id)
}

// blockBody is a block's body: the outline of its corners (about pos) seen
// from above, from the lowest to the highest. It stays where it is.
func (g *Game) blockBody(id ecs.ID, pos vec3, corners []vec3, round float64) *physics.Body {
	p := g.phys
	mid := p.plane(pos)
	var pts []physics.V2
	bottom, top := math.Inf(1), math.Inf(-1)
	for _, c := range corners {
		w := pos.Add(c)
		pts = append(pts, p.plane(w).Sub(mid))
		bottom, top = math.Min(bottom, p.height(w)), math.Max(top, p.height(w))
	}
	return &physics.Body{Kind: physics.Static, Shape: physics.Hull(pts, round), Pos: mid, Bottom: bottom - round, Top: top + round,
		Layer: layerStatic, Mask: ^uint32(0), ID: uint32(id), Friction: frictionWall, Restitution: bounceWall}
}
