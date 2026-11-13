package game

import (
	"naturalia/internal/base"
	"naturalia/internal/ecs"
	"naturalia/internal/mathx"
	"naturalia/internal/physics"
	"naturalia/internal/planet"
	"slices"
)

// Collisions go through package physics, on a plane tangent to Mars at an
// anchor near the player, laid again when they stray from it. A point goes
// on the plane where the line from Mars' center through it meets the plane
// (a gnomonic projection): straight walls stay straight, and back again is
// exact. Heights are from Mars' center, less the anchor's ground: walls
// stop what stands lower than their top.
//
// The world (Player.Pos, the entities' Transform) stays the truth: bodies
// are set from it every frame, then it takes back where they went.

// What touches what (physics.Body Layer and Mask).
const (
	layerStatic uint32 = 1 << iota
	layerPlayer
	layerEntity
)

const (
	physReach = 120.0 // m from the anchor where things collide
	reanchor  = 50.0  // m the player strays before the plane is laid again
	walkAccel = 60.0  // m/s²: how fast walking reaches the speed asked
	bodyFeet  = 1.0   // m below the ground where a body starts
	personH   = 1.8
	thingH    = 2.0 // entities and furniture, without a better idea
	minRock   = 0.6 // smaller rocks are walked over
)

type phys struct {
	w          *physics.World
	up, t1, t2 vec3    // the plane: its normal, then its x and y
	r0         float64 // the anchor's ground, from Mars' center
	player     *physics.Body
	doorSegs   []base.Segment
	doors      []*physics.Body
}

// plane is where a point of the world goes on the plane.
func (p *phys) plane(pos vec3) physics.V2 {
	q := pos.Scale(p.r0 / pos.Dot(p.up))
	return physics.V2{X: q.Dot(p.t1), Y: q.Dot(p.t2)}
}

// dir is the direction from Mars' center through a point of the plane.
func (p *phys) dir(v physics.V2) vec3 {
	return p.up.Scale(p.r0).Add(p.t1.Scale(v.X)).Add(p.t2.Scale(v.Y)).Norm()
}

// vecPlane is a speed along the ground at pos, on the plane.
func (p *phys) vecPlane(pos, v vec3) physics.V2 {
	const e = 1e-2
	return p.plane(pos.Add(v.Scale(e))).Sub(p.plane(pos)).Scale(1 / e)
}

// vecWorld is a speed on the plane, along the ground at pos.
func (p *phys) vecWorld(pos vec3, v physics.V2) vec3 {
	const e = 1e-2
	to := p.dir(p.plane(pos).Add(v.Scale(e))).Scale(pos.Length())
	return planet.TangentAt(pos.Norm(), to.Sub(pos)).Scale(to.Sub(pos).Length() / e)
}

// height is how high a point stands on the plane's scale.
func (p *phys) height(pos vec3) float64 { return pos.Length() - p.r0 }

// slice is the height a thing standing at pos fills, h m tall.
func (p *phys) slice(pos vec3, h float64) (bottom, top float64) {
	g := p.height(pos)
	return g - bodyFeet, g + h
}

// stepPhysics moves the walking player along the ground, out of walls,
// rocks and entities.
func (g *Game) stepPhysics(dt float64) {
	pl := &g.Player
	if g.phys == nil || planet.ArcDist(pl.LocalUp(), g.phys.up) > reanchor {
		g.layPlane()
	}
	p := g.phys
	g.syncDoors()
	g.syncEntities()

	b := p.player
	walking := !pl.Fly && !pl.seated
	if !walking {
		b.Layer, b.Mask, b.Vel = 0, 0, physics.V2{}
		p.w.Step(dt)
		return
	}
	b.Layer, b.Mask = layerPlayer, layerStatic|layerEntity
	b.Pos = p.plane(pl.Pos)
	b.Bottom = p.height(pl.Pos) + 0.3 // steps and pebbles
	b.Top = b.Bottom + personH - 0.3
	// The speed asked, reached quickly: what pushed the player fades.
	vel, want := p.vecPlane(pl.Pos, pl.vel), p.vecPlane(pl.Pos, pl.want)
	d := want.Sub(vel)
	if l := d.Len(); l > walkAccel*dt {
		d = d.Scale(walkAccel * dt / l)
	}
	b.Vel = vel.Add(d)
	p.w.Step(dt)
	r := pl.Pos.Length()
	pl.Pos = p.dir(b.Pos).Scale(r)
	pl.vel = p.vecWorld(pl.Pos, b.Vel)
}

// layPlane lays the plane under the player, with the base and the rocks
// around.
func (g *Game) layPlane() {
	up := g.Player.LocalUp()
	t1, t2 := mathx.TangentBasis(up)
	p := &phys{w: physics.NewWorld(), up: up, t1: t1, t2: t2, r0: g.World.Relief.Surface(up).Length()}
	g.phys = p
	for id := range g.Ents.Body.All() {
		g.Ents.Body.Remove(id)
	}
	p.player = p.w.Add(&physics.Body{Kind: physics.Dynamic, Shape: physics.Circle(0.35), Mass: 80, FixedRotation: true})

	if planet.ArcDist(up, planet.BaseUp) < physReach+base.Reach+60 {
		for _, s := range g.walls {
			g.addWall(s)
		}
		for _, o := range base.Obstacles {
			at := base.ToWorld(o.At)
			bottom, top := p.slice(at, thingH)
			p.w.Add(&physics.Body{Kind: physics.Static, Shape: physics.Circle(o.Radius), Pos: p.plane(at),
				Bottom: bottom, Top: top, Layer: layerStatic, Mask: ^uint32(0)})
		}
	}
	p.doorSegs, p.doors = nil, nil
	for _, r := range g.World.RocksNear(up.Scale(p.r0), physReach+reanchor) {
		if r.Size < minRock {
			continue
		}
		at := g.World.Relief.Surface(r.Dir)
		bottom, top := p.slice(at, r.Size*0.8)
		p.w.Add(&physics.Body{Kind: physics.Static, Shape: physics.Circle(r.Size * 0.75), Pos: p.plane(at),
			Bottom: bottom, Top: top, Layer: layerStatic, Mask: ^uint32(0)})
	}
}

// addWall adds one of the base's walls.
func (g *Game) addWall(s base.Segment) *physics.Body {
	p := g.phys
	a, b, top := s.Ends()
	pa, pb := p.plane(base.ToWorld(a)), p.plane(base.ToWorld(b))
	mid := pa.Lerp(pb, 0.5)
	return p.w.Add(&physics.Body{Kind: physics.Static, Shape: physics.Capsule(pa.Sub(mid), pb.Sub(mid), base.WallThick/2), Pos: mid,
		Bottom: p.height(base.ToWorld(a)) - bodyFeet, Top: p.height(base.ToWorld(a.Add(vec3{0, top, 0}))), Layer: layerStatic, Mask: ^uint32(0)})
}

// syncDoors sets the doors' walls where the leaves stand.
func (g *Game) syncDoors() {
	p := g.phys
	if planet.ArcDist(p.up, planet.BaseUp) > physReach+base.Reach+60 {
		return
	}
	segs := g.wallsNow()[len(g.walls):]
	if slices.Equal(segs, p.doorSegs) {
		return
	}
	for _, b := range p.doors {
		p.w.Remove(b)
	}
	p.doorSegs, p.doors = slices.Clone(segs), p.doors[:0]
	for _, s := range segs {
		p.doors = append(p.doors, g.addWall(s))
	}
}

// syncEntities gives the entities near the player that take room a body
// where they stand, and takes them from those gone.
func (g *Game) syncEntities() {
	p := g.phys
	for _, b := range slices.Clone(p.w.Bodies()) {
		if id := ecs.ID(b.ID); id != 0 && (!g.Ents.Alive(id) || g.Ents.Body.Get(id) == nil) {
			p.w.Remove(b)
		}
	}
	near := g.EntitiesNear(physReach)
	slices.Sort(near) // the same bodies in the same order: steps repeat exactly
	for id, bp := range g.Ents.Body.All() {
		if _, ok := slices.BinarySearch(near, id); !ok || id == g.Riding {
			p.w.Remove(*bp)
			g.Ents.Body.Remove(id)
		}
	}
	for _, id := range near {
		def, ok := g.Content.Entities.Get(g.kind(id))
		if !ok || def.Radius <= 0 || id == g.Riding {
			continue
		}
		e := g.Ents.Transform.Get(id)
		b := g.Ents.Body.Get(id)
		if b == nil {
			b = g.Ents.Body.Set(id, p.w.Add(&physics.Body{Kind: physics.Kinematic, Shape: physics.Circle(def.Radius),
				Layer: layerEntity, Mask: ^uint32(0), ID: uint32(id)}))
		}
		at := g.World.Relief.Surface(e.Dir)
		(*b).Pos, (*b).Angle = p.plane(at), 0
		(*b).Bottom, (*b).Top = p.slice(at.Add(e.Dir.Scale(e.Alt)), thingH)
	}
}
