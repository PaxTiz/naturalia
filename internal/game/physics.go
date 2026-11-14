package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
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
	layerVehicle // the one driven
	layerRobot   // automatic vehicles on the ground
	layerProp    // things pushed around: crates
	layerDrone   // automatic vehicles flying: over the others
	layerTrigger // zones, ramps and pads: they tell who is in them
)

// What things are made of: how they rub and bounce. A contact takes the
// geometric mean of the frictions and the larger bounce; the player only
// bounces off what moves by itself (mix).
const (
	frictionWall, bounceWall     = 0.6, 0.0
	frictionCar, bounceCar       = 0.6, 0.35
	frictionPerson, bouncePerson = 0.0, 0.0
	frictionProp, bounceProp     = 0.5, 0.3
)

const (
	physReach  = 120.0 // m from the anchor where things collide
	reanchor   = 50.0  // m the player strays before the plane is laid again
	walkAccel  = 60.0  // m/s²: how fast walking reaches the speed asked
	bodyFeet   = 1.0   // m below the ground where a body starts
	personH    = 1.8
	thingH     = 2.0 // entities and furniture, without a better idea
	minRock    = 0.6 // smaller rocks are walked over
	pushSpeed  = 1.6 // m/s: pushing a crate on foot goes slowly
	pushAhead  = 0.4 // m: how far ahead the player sees it coming
	tossed     = 4.0 // m/s: a thing hit harder flies off
	tossFall   = 2.5 // times Mars's gravity on what flies off: it lands quick
	rub        = 6.0 // the ground's friction on what is pushed around
	knocked    = 8.0 // m/s: the fastest something knocks the player back
	fleetAccel = 6.0 // m/s²: how fast an automatic vehicle gets going, at least
	overhead   = 0.6 // m a drone keeps over what it flies above
)

type phys struct {
	w          *physics.World
	up, t1, t2 vec3    // the plane: its normal, then its x and y
	r0         float64 // the anchor's ground, from Mars' center
	player     *physics.Body
	car        *physics.Body // the driven vehicle's
	carKind    string
	side       float64 // +1 if a vehicle's right side (+Z) is the plane's left of its nose
	doorSegs   []base.Segment
	doors      []*physics.Body
	carried    map[ecs.ID]moving  // from the plane before, until put back
	bounds     map[string][2]vec3 // the box around each kind's parts
}

// partsBounds is the box around the parts of an entity of that kind,
// figured once.
func (p *phys) partsBounds(def *content.EntityDef) (lo, hi vec3) {
	b, ok := p.bounds[def.ID]
	if !ok {
		b[0], b[1] = content.Bounds(def.Parts)
		p.bounds[def.ID] = b
	}
	return b[0], b[1]
}

// moving is how a thing pushed around went when the plane was laid
// again: its speed in the world, its spin seen from above.
type moving struct {
	vel  vec3
	spin float64
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

// stepPhysics moves the walking player, or the vehicle they drive, along
// the ground, out of walls, rocks and entities.
func (g *Game) stepPhysics(dt float64) {
	pl := &g.Player
	if g.phys == nil || planet.ArcDist(pl.LocalUp(), g.phys.up) > reanchor {
		g.layPlane()
	}
	p := g.phys
	g.syncDoors()
	g.syncEntities()
	g.fleetBefore(dt)

	b := p.player
	b.Layer, b.Mask, b.Vel = 0, 0, physics.V2{} // in a vehicle, or flying through everything
	var walk physics.V2
	if !pl.Fly && !pl.seated {
		b.Layer, b.Mask = layerPlayer, layerStatic|layerEntity|layerRobot|layerVehicle|layerProp|layerTrigger
		b.Pos = p.plane(pl.Pos)
		b.Bottom = p.height(pl.Pos) + 0.3 // steps and pebbles
		b.Top = b.Bottom + personH - 0.3
		// The speed asked, reached quickly: what pushed the player fades.
		vel, want := p.vecPlane(pl.Pos, pl.vel), p.vecPlane(pl.Pos, pl.want)
		// Slowed down before reaching a crate, not to kick it away.
		if l := want.Len(); l > pushSpeed {
			ahead := b.Pos.Add(want.Scale(pushAhead / l))
			if p.w.Overlap(b.Shape, ahead, 0, b.Bottom, b.Top, layerProp, b) {
				want = want.Scale(pushSpeed / l)
			}
		}
		d := want.Sub(vel)
		if l := d.Len(); l > walkAccel*dt {
			d = d.Scale(walkAccel * dt / l)
		}
		b.Vel = vel.Add(d)
		walk = b.Vel
	}
	v, driving := g.Driven()
	if driving {
		g.carBefore(v)
	} else if p.car != nil {
		p.w.Remove(p.car)
		p.car = nil
	}
	if p.car == nil || p.car.Mask == 0 {
		g.drive.over = nil
	}

	p.w.Step(dt)

	r := pl.Pos.Length()
	switch {
	case driving:
		g.carAfter(v)
	case b.Mask != 0:
		pl.Pos = p.dir(b.Pos).Scale(r)
		pl.vel = p.vecWorld(pl.Pos, b.Vel)
	}
	for _, hit := range p.w.Impacts {
		for _, o := range [2]*physics.Body{hit.A, hit.B} {
			if o.Layer == layerProp {
				g.toss(ecs.ID(o.ID), hit.Speed)
			}
		}
		// Less what the player walked into it.
		if hit.A == b {
			g.bump(hit.Speed - walk.Dot(hit.Normal))
		} else if hit.B == b {
			g.bump(hit.Speed + walk.Dot(hit.Normal))
		}
	}
	g.senses(v)
	g.entitiesAfter(dt)
	g.updateTossed(dt)
}

// bump: something came at the walking player that fast, and threw them
// back a little, and up.
func (g *Game) bump(speed float64) {
	pl := &g.Player
	if speed < 2 {
		return
	}
	if s := pl.vel.Length(); s > knocked {
		pl.vel = pl.vel.Scale(knocked / s)
	}
	pl.vr, pl.onGround = math.Max(pl.vr, math.Min(3, speed*0.3)), false
	g.sfx.Beep(0, 0.15, 0.15, 140, 90)
}

// toss throws a thing hit hard up in the air, tumbling.
func (g *Game) toss(id ecs.ID, speed float64) {
	if speed < tossed || !g.Ents.Alive(id) {
		return
	}
	a := g.Ents.Air.Get(id)
	if a == nil {
		a = g.Ents.Air.Set(id, Air{})
	}
	a.vz = math.Max(a.vz, math.Min(6, (speed-2)*0.35))
	a.tumble = (g.Rng.Float64()*2 - 1) * math.Min(9, speed)
	g.rubs(id, false)
}

// rubs: what lies on the ground slows down, what flies does not.
func (g *Game) rubs(id ecs.ID, on bool) {
	if bp := g.Ents.Body.Get(id); bp != nil {
		r := 0.0
		if on {
			r = rub
		}
		(*bp).Damping, (*bp).SpinDamping = r, r
	}
}

// updateTossed flies the things tossed up, until they land.
func (g *Game) updateTossed(dt float64) {
	for id, a := range g.Ents.Air.All() {
		e := g.Ents.Transform.Get(id)
		e.Alt += a.vz * dt
		a.vz -= tossFall * marsGravity * dt
		e.Pitch += a.tumble * dt
		if e.Alt > 0 {
			continue
		}
		e.Alt = 0
		if a.vz < -2 {
			a.vz, a.tumble = -a.vz*0.3, a.tumble*0.5 // a bounce
			continue
		}
		// Down on a face.
		e.Pitch = math.Round(e.Pitch/(math.Pi/2)) * math.Pi / 2
		e.Roll = math.Round(e.Roll/(math.Pi/2)) * math.Pi / 2
		g.Ents.Air.Remove(id)
		g.rubs(id, true)
	}
}

// entitiesAfter moves the things pushed around where the collisions left
// them; the fleet turns its nose the way it goes.
func (g *Game) entitiesAfter(dt float64) {
	p := g.phys
	for id, bp := range g.Ents.Body.All() {
		b := *bp
		if b.Kind != physics.Dynamic || b.Asleep() {
			continue
		}
		e := g.Ents.Transform.Get(id)
		e.Dir = p.dir(b.Pos)
		at := e.Dir.Scale(p.r0) // near enough the ground, for a speed
		if !b.FixedRotation {
			e.Yaw = yawOf(e.Dir, p.vecWorld(at, physics.V2{X: math.Cos(b.Angle), Y: math.Sin(b.Angle)}))
		} else if b.Vel.Len() > 0.3 {
			turn := math.Remainder(yawOf(e.Dir, p.vecWorld(at, b.Vel))-e.Yaw, 2*math.Pi)
			e.Yaw += turn * math.Min(1, dt*4)
		}
	}
}

// fleetBefore sets the automatic vehicles near the player going at the
// speed the fleet asked (moveToward), the ground ones around what stands
// in their way. Pushed off, they come back to it.
func (g *Game) fleetBefore(dt float64) {
	p := g.phys
	for id, r := range g.Ents.Roam.All() {
		bp := g.Ents.Body.Get(id)
		if bp == nil {
			continue
		}
		b := *bp
		want := p.vecPlane(g.Ents.Transform.Get(id).Dir.Scale(p.r0), r.want)
		r.want = vec3{} // asked again each frame
		if b.Layer == layerRobot {
			want = g.steer(b, r, want)
		}
		d, most := want.Sub(b.Vel), math.Max(fleetAccel, 2*want.Len())*dt
		if l := d.Len(); l > most {
			d = d.Scale(most / l)
		}
		b.Vel = b.Vel.Add(d)
		if want.Len() > 0 {
			b.Wake()
		}
	}
}

// What a ground robot goes around.
const inTheWay = layerStatic | layerEntity | layerVehicle | layerProp | layerRobot | layerPlayer

// steer turns the way a ground robot asks for around what stands ahead:
// the nearest free heading, on the side it already took. Hemmed in, it
// only pushes gently.
func (g *Game) steer(b *physics.Body, r *Roam, want physics.V2) physics.V2 {
	speed := want.Len()
	if speed < 0.01 {
		r.dodge = 0
		return want
	}
	ahead := b.Shape.Radius + 0.4 + speed*0.6
	side := r.dodge
	if side == 0 {
		side = 1
	}
	for _, deg := range [...]float64{0, 35, 70, 110, -35, -70, -110} {
		dir := physics.MakeRot(side * deg * math.Pi / 180).Apply(want.Scale(1 / speed))
		if g.phys.w.Overlap(b.Shape, b.Pos.Add(dir.Scale(ahead*0.5)), 0, b.Bottom, b.Top, inTheWay, b) ||
			g.phys.w.Overlap(b.Shape, b.Pos.Add(dir.Scale(ahead)), 0, b.Bottom, b.Top, inTheWay, b) {
			continue
		}
		switch {
		case deg == 0:
			r.dodge = 0
		case deg > 0:
			r.dodge = side
		default:
			r.dodge = -side
		}
		return dir.Scale(speed)
	}
	return want.Scale(0.2)
}

// clearance is the height a drone flies at to pass over what stands
// below it and ahead: vehicles, the player, rocks, walls.
func (g *Game) clearance(id ecs.ID) float64 {
	bp := g.Ents.Body.Get(id)
	if bp == nil || g.phys == nil {
		return 0
	}
	b, p := *bp, g.phys
	def, ok := g.Content.Entities.Get(g.kind(id))
	if !ok {
		return 0
	}
	lo, _ := p.partsBounds(def)
	reach := b.Shape.Radius + 1
	box := physics.AABB{Lo: b.Pos.Sub(physics.V2{X: reach, Y: reach}), Hi: b.Pos.Add(physics.V2{X: reach, Y: reach})}
	box = box.Union(physics.AABB{Lo: box.Lo.Add(b.Vel), Hi: box.Hi.Add(b.Vel)}) // where it is in a second
	ground := p.height(g.World.Relief.Surface(g.Ents.Transform.Get(id).Dir))
	top := 0.0
	p.w.Query(box, func(o *physics.Body) bool {
		if o != b && o.Layer != 0 && o.Layer != layerDrone && !o.Sensor {
			top = math.Max(top, o.Top-ground)
		}
		return true
	})
	if top == 0 {
		return 0
	}
	return top + overhead - lo.Y
}

// carBefore sets the driven vehicle's body where it stands, going at the
// speed move set.
func (g *Game) carBefore(v *content.Vehicle) {
	p, d, pl := g.phys, &g.drive, &g.Player
	if p.car == nil || p.carKind != v.ID {
		if p.car != nil {
			p.w.Remove(p.car)
		}
		lo, hi := v.Footprint()
		const round = 0.15
		mid := physics.V2{X: (lo.X + hi.X) / 2, Y: p.side * (lo.Z + hi.Z) / 2}
		p.car = p.w.Add(&physics.Body{Kind: physics.Dynamic, Mass: v.Weight(), SpinDamping: 1.5,
			Shape:    physics.OffsetBox((hi.X-lo.X)/2-round, (hi.Z-lo.Z)/2-round, mid, 0, round),
			Friction: frictionCar, Restitution: bounceCar})
		p.carKind = v.ID
	}
	c := p.car
	c.Layer, c.Mask = layerVehicle, layerStatic|layerEntity|layerPlayer|layerProp|layerRobot|layerTrigger
	if d.dock != nil {
		c.Layer, c.Mask = 0, 0 // gliding into its bay
	}
	c.Pos = p.plane(pl.Pos)
	f := p.vecPlane(pl.Pos, d.fwd)
	c.Angle = math.Atan2(f.Y, f.X)
	c.Vel, c.Spin = p.vecPlane(pl.Pos, d.vel), d.spin
	bottom := p.height(pl.Pos)
	if d.r != 0 {
		bottom = d.r - p.r0
	}
	c.Bottom = bottom + v.Hover + 0.25 // over pebbles and kerbs
	c.Top = bottom + v.Hover + math.Max(1.2, content.OrZero(v.Seat, 2.4))
}

// carAfter moves the player where the collisions left the vehicle: a knock
// changes its speed, and turns it, the view with it.
func (g *Game) carAfter(v *content.Vehicle) {
	p, d, pl := g.phys, &g.drive, &g.Player
	c := p.car
	pl.Pos = p.dir(c.Pos).Scale(pl.Pos.Length())
	up := pl.LocalUp()
	d.vel = p.vecWorld(pl.Pos, c.Vel)
	d.spin = c.Spin
	old := planet.TangentAt(up, d.fwd)
	fwd := p.vecWorld(pl.Pos, physics.V2{X: math.Cos(c.Angle), Y: math.Sin(c.Angle)}).Norm()
	if a := math.Atan2(up.Dot(old.Cross(fwd)), old.Dot(fwd)); a != 0 {
		pl.Heading = pl.Heading.Scale(math.Cos(a)).Add(up.Cross(pl.Heading).Scale(math.Sin(a)))
	}
	d.fwd = fwd
	for _, hit := range p.w.Impacts {
		if hit.A == c || hit.B == c {
			g.knock(v, hit)
		}
	}
}

// knock: the driven vehicle hit something, or was hit.
func (g *Game) knock(v *content.Vehicle, hit physics.Impact) {
	const felt = 2.5 // m/s: slower is a nudge
	if hit.Speed < felt {
		return
	}
	p, d := g.phys, &g.drive
	at := g.World.Relief.Surface(p.dir(hit.Point))
	d.shake = math.Max(d.shake, math.Min(1, hit.Speed/14))
	g.sfx.Beep(0, 0.2, math.Min(0.3, hit.Speed*0.02), 90, 60)
	g.dustBurst(at, int(math.Min(30, hit.Speed*2)))
	other := hit.B
	if other == p.car {
		other = hit.A
	}
	g.emit(content.Event{Name: content.EvImpact, ID: g.kind(ecs.ID(other.ID)), Entity: other.ID, Amount: hit.Speed, At: at})
}

// layPlane lays the plane under the player, with the base and the rocks
// around.
func (g *Game) layPlane() {
	up := g.Player.LocalUp()
	t1, t2 := mathx.TangentBasis(up)
	p := &phys{w: physics.NewWorld(), up: up, t1: t1, t2: t2, r0: g.World.Relief.Surface(up).Length(),
		carried: map[ecs.ID]moving{}, bounds: map[string][2]vec3{}}
	p.side = t1.Cross(up).Dot(t2)
	p.w.Mix = mix
	for id, bp := range g.Ents.Body.All() {
		if b := *bp; b.Kind == physics.Dynamic && !b.Asleep() {
			at := g.World.Relief.Surface(g.Ents.Transform.Get(id).Dir)
			p.carried[id] = moving{g.phys.vecWorld(at, b.Vel), b.Spin * g.phys.side}
		}
		g.Ents.Body.Remove(id)
	}
	g.phys = p
	p.player = p.w.Add(&physics.Body{Kind: physics.Dynamic, Shape: physics.Circle(0.35), Mass: 80, FixedRotation: true,
		Friction: frictionPerson, Restitution: bouncePerson})

	if planet.ArcDist(up, planet.BaseUp) < physReach+base.Extent {
		for _, s := range g.walls {
			g.addWall(s)
		}
		for _, o := range base.Obstacles {
			at := base.ToWorld(o.At)
			bottom, top := p.slice(at, thingH)
			p.w.Add(&physics.Body{Kind: physics.Static, Shape: physics.Circle(o.Radius), Pos: p.plane(at),
				Bottom: bottom, Top: top, Layer: layerStatic, Mask: ^uint32(0), Friction: frictionWall, Restitution: bounceWall})
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
			Bottom: bottom, Top: top, Layer: layerStatic, Mask: ^uint32(0), Friction: frictionWall, Restitution: bounceWall})
	}
}

func mix(a, b *physics.Body) (friction, bounce float64) {
	friction, bounce = math.Sqrt(a.Friction*b.Friction), math.Max(a.Restitution, b.Restitution)
	if a.Layer == layerPlayer {
		a, b = b, a
	}
	if b.Layer == layerPlayer && a.Layer != layerRobot && a.Layer != layerVehicle {
		bounce = 0 // pushed, not kicked
	}
	return friction, bounce
}

// addWall adds one of the base's walls.
func (g *Game) addWall(s base.Segment) *physics.Body {
	p := g.phys
	a, b, top := s.Ends()
	pa, pb := p.plane(base.ToWorld(a)), p.plane(base.ToWorld(b))
	mid := pa.Lerp(pb, 0.5)
	return p.w.Add(&physics.Body{Kind: physics.Static, Shape: physics.Capsule(pa.Sub(mid), pb.Sub(mid), base.WallThick/2), Pos: mid,
		Bottom: p.height(base.ToWorld(a)) - bodyFeet, Top: p.height(base.ToWorld(a.Add(vec3{0, top, 0}))), Layer: layerStatic, Mask: ^uint32(0),
		Friction: frictionWall, Restitution: bounceWall})
}

// syncDoors sets the doors' walls where the leaves stand.
func (g *Game) syncDoors() {
	p := g.phys
	if planet.ArcDist(p.up, planet.BaseUp) > physReach+base.Extent {
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
		if !ok || id == g.Riding {
			continue
		}
		v, isVehicle := g.vehicleOf(id)
		if def.Radius <= 0 && def.Mass <= 0 && !(isVehicle && v.Auto()) && !trigger(def) && !def.Build {
			continue
		}
		bp := g.Ents.Body.Get(id)
		switch {
		case bp != nil && (*bp).Kind == physics.Static:
			continue // a block: it stays put
		case bp == nil && def.Build:
			g.Ents.Body.Set(id, p.w.Add(g.blockBody(id, def)))
			continue
		}
		e := g.Ents.Transform.Get(id)
		at := g.World.Relief.Surface(e.Dir)
		if bp == nil {
			bp = g.Ents.Body.Set(id, p.w.Add(g.entityBody(id, def)))
			ax, _, _ := entityAxes(e.Dir, e.Yaw)
			f := p.vecPlane(at, ax)
			(*bp).Pos, (*bp).Angle = p.plane(at), math.Atan2(f.Y, f.X)
			if m, ok := p.carried[id]; ok {
				(*bp).Vel, (*bp).Spin = p.vecPlane(at, m.vel), m.spin*p.side
				delete(p.carried, id)
			}
		}
		b := *bp
		if b.Kind != physics.Dynamic {
			ax, _, _ := entityAxes(e.Dir, e.Yaw)
			f := p.vecPlane(at, ax)
			b.Pos, b.Angle = p.plane(at), math.Atan2(f.Y, f.X) // where it went by itself
		}
		lo, hi := p.partsBounds(def)
		if hi.Y <= 0 {
			hi.Y = thingH
		}
		if b.Sensor {
			hi.Y += triggerH
		}
		b.Bottom, b.Top = p.slice(at.Add(e.Dir.Scale(e.Alt)), hi.Y)
		if b.Layer == layerDrone {
			b.Bottom = b.Top - hi.Y + lo.Y
		}
	}
}

// trigger tells if an entity does something to the vehicle driven over
// it: a ramp lifts it, a pad throws it.
func trigger(def *content.EntityDef) bool { return def.Ramp != nil || def.Kick > 0 || def.Trigger }

// senses tells packs who comes into the zones and goes out, keeps the
// ramps and pads the driven vehicle is over, and kicks it off the pads it
// comes onto.
func (g *Game) senses(v *content.Vehicle) {
	p, d := g.phys, &g.drive
	for _, s := range p.w.Senses {
		id := ecs.ID(s.Sensor.ID)
		by := "player"
		switch {
		case s.Body == p.car && p.car != nil && v != nil:
			by = v.ID
			d.over = slices.DeleteFunc(d.over, func(o ecs.ID) bool { return o == id || !g.Ents.Alive(o) })
			if s.In {
				d.over = append(d.over, id)
				g.kick(v, id)
			}
		case s.Body != p.player:
			continue
		}
		ev := content.Event{Name: content.EvTriggerOut, ID: g.kind(id), Entity: uint32(id), By: by}
		if s.In {
			ev.Name = content.EvTriggerIn
		}
		if e := g.Ents.Transform.Get(id); e != nil {
			ev.At = g.World.Relief.Surface(e.Dir)
		}
		g.emit(ev)
	}
}

// entityBody is the body an entity gets: pushed around if it has a mass,
// standing its ground otherwise.
func (g *Game) entityBody(id ecs.ID, def *content.EntityDef) *physics.Body {
	b := &physics.Body{Kind: physics.Kinematic, Shape: physics.Circle(def.Radius), Layer: layerEntity, Mask: ^uint32(0), ID: uint32(id),
		Friction: frictionWall, Restitution: bounceWall}
	if trigger(def) {
		b.Sensor, b.Layer, b.Mask = true, layerTrigger, layerVehicle|layerPlayer
		lo, hi := g.phys.partsBounds(def)
		switch {
		case def.Ramp != nil:
			b.Shape = physics.Box(def.Ramp.Length/2, def.Ramp.Width/2, 0)
		case def.Kick > 0:
			b.Shape = physics.Circle(padReach)
		case def.Radius > 0:
			b.Shape = physics.Circle(def.Radius)
		case hi.X > lo.X && hi.Z > lo.Z:
			mid := physics.V2{X: (lo.X + hi.X) / 2, Y: g.phys.side * (lo.Z + hi.Z) / 2}
			b.Shape = physics.OffsetBox((hi.X-lo.X)/2, (hi.Z-lo.Z)/2, mid, 0, 0)
		default:
			b.Shape = physics.Circle(1)
		}
		return b
	}
	if v, ok := g.vehicleOf(id); ok && v.Auto() {
		// Driven by the fleet, pushed around like a heavy crate.
		b.Kind, b.Mass, b.FixedRotation = physics.Dynamic, v.Weight(), true
		b.Friction, b.Restitution = frictionCar, bounceCar
		b.Layer, b.Mask = layerRobot, inTheWay
		if v.Hover > 0 {
			lo, hi := g.phys.partsBounds(def)
			b.Shape = physics.Circle(math.Max(0.3, math.Max(math.Max(-lo.X, hi.X), math.Max(-lo.Z, hi.Z))))
			b.Layer, b.Mask = layerDrone, layerStatic|layerDrone
		}
		return b
	}
	if def.Mass > 0 {
		const round = 0.03
		lo, hi := g.phys.partsBounds(def)
		mid := physics.V2{X: (lo.X + hi.X) / 2, Y: g.phys.side * (lo.Z + hi.Z) / 2}
		b.Kind, b.Layer, b.Mass = physics.Dynamic, layerProp, def.Mass
		b.Shape = physics.OffsetBox(math.Max(0.05, (hi.X-lo.X)/2-round), math.Max(0.05, (hi.Z-lo.Z)/2-round), mid, 0, round)
		b.Friction, b.Restitution = frictionProp, bounceProp
		if !g.Ents.Air.Has(id) {
			b.Damping, b.SpinDamping = rub, rub
		}
	}
	return b
}
