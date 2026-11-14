package game

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/mathx"
	"naturalia/internal/physics"
)

// Blocks (docs/CONSTRUCTION.md) are entities to build with: fixed where
// they were set down, turned any way. Rays find their faces from the shape
// of their parts: boxes, turned around their up, and wedges.

// solid is a convex shape in an entity's frame: the points x with
// n·x <= d for all its planes.
type solid []plane

type plane struct {
	n vec3
	d float64
}

// solidsOf are the shapes of an entity's parts, figured once by kind;
// a prism counts as the box around it.
func (g *Game) solidsOf(def *content.EntityDef) []solid {
	if s, ok := g.solids[def.ID]; ok {
		return s
	}
	var out []solid
	for _, p := range def.Parts {
		a := p.Yaw * math.Pi / 180
		ex, ey := vec3{math.Cos(a), 0, math.Sin(a)}, vec3{0, 1, 0}
		ez := ex.Cross(ey)
		switch p.Shape {
		case "wedge": // standing on at, rising toward +x
			x, y, z := p.Size.X/2, p.Size.Y, p.Size.Z/2
			slope := ey.Scale(1).Sub(ex.Scale(y / p.Size.X)).Norm() // up the slope's face
			out = append(out, solid{
				{ey.Scale(-1), -ey.Dot(p.At)},
				{ex, ex.Dot(p.At) + x},
				{ez, ez.Dot(p.At) + z},
				{ez.Scale(-1), -ez.Dot(p.At) + z},
				{slope, slope.Dot(p.At.Add(ey.Scale(y / 2)))},
			})
		case "prism":
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
		s = append(s, plane{a, a.Dot(c) + half[i]}, plane{a.Scale(-1), -a.Dot(c) + half[i]})
	}
	return s
}

// hit is where a ray from o along d (length 1) comes into the solid,
// within max, and the face's normal.
func (s solid) hit(o, d vec3, max float64) (float64, vec3, bool) {
	in, out := math.Inf(-1), max
	var n vec3
	for _, p := range s {
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
	if in > out || in < 0 {
		return 0, vec3{}, false
	}
	return in, n, true
}

// contains tells if x is inside the solid.
func (s solid) contains(x vec3) bool {
	for _, p := range s {
		if p.n.Dot(x) > p.d {
			return false
		}
	}
	return true
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
// which, where, and the face's normal.
func (g *Game) aimBlock(o, d vec3, max float64) (ecs.ID, vec3, vec3, bool) {
	var best ecs.ID
	var normal vec3
	for _, id := range g.EntitiesNear(max + 12) {
		if def, ok := g.Content.Entities.Get(g.kind(id)); !ok || !def.Build {
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
// straight under at, below from (both from Mars' center); 0 for none.
func (g *Game) topUnder(at vec3, from float64, things bool) float64 {
	start := at.Norm().Scale(from)
	reach := g.World.Relief.Altitude(start)
	if reach <= 0 {
		return 0
	}
	top := 0.0
	for _, id := range g.EntitiesNear(15) {
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
	floor := 0.0
	for _, off := range [...]vec3{{}, t1.Scale(0.22), t1.Scale(-0.22), t2.Scale(0.22), t2.Scale(-0.22)} {
		at := pos.Add(off)
		if top := g.topUnder(at, pos.Length()+stepUp, true); top > 0 {
			floor = math.Max(floor, top-g.World.Relief.Surface(at).Length())
		}
	}
	return floor
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
	if (a != p.player && a != p.car) || b.Kind != physics.Static || b.ID == 0 {
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
		dir := p.dir(at)
		if !g.clearAbove(id, dir.Scale(feet+step), dir, tall-step) {
			return true
		}
	}
	return false
}

// blockBody is a block's body: the outline of the box around its parts
// seen from above, turned as it stands, from its lowest point to its
// highest. It stays where it is.
func (g *Game) blockBody(id ecs.ID, def *content.EntityDef) *physics.Body {
	const round = 0.02
	p := g.phys
	pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(id))
	mid := p.plane(pos)
	var pts []physics.V2
	bottom, top := math.Inf(1), math.Inf(-1)
	for _, c := range ghostCorners(def, ax, up, round) {
		w := pos.Add(c)
		pts = append(pts, p.plane(w).Sub(mid))
		bottom, top = math.Min(bottom, p.height(w)), math.Max(top, p.height(w))
	}
	return &physics.Body{Kind: physics.Static, Shape: physics.Hull(pts, round), Pos: mid, Bottom: bottom - round, Top: top + round,
		Layer: layerStatic, Mask: ^uint32(0), ID: uint32(id), Friction: frictionWall, Restitution: bounceWall}
}
