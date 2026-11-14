package physics

import "math"

type Kind uint8

const (
	Static    Kind = iota // never moves: walls, rocks, furniture
	Kinematic             // moves at its speed, pushes and is never pushed
	Dynamic               // has a mass, is pushed, bounces, spins
)

// Body is something that takes room on the plane. Set its fields before
// adding it to a World; afterwards the game sets the speeds of dynamic and
// kinematic bodies and reads back where they went. Static ones only move
// through World.Teleport.
type Body struct {
	Kind  Kind
	Shape Shape
	Pos   V2
	Angle float64
	Vel   V2
	Spin  float64 // radians per second, counter-clockwise
	Mass  float64 // kg, dynamic ones (1 if left out)
	// FixedRotation: contacts never turn it (someone walking).
	FixedRotation bool
	// Bottom and Top are the slice of height it fills: two bodies only
	// touch where their slices overlap.
	Bottom, Top float64
	// A body touches those whose Layer is in its Mask, and its Layer in
	// theirs.
	Layer, Mask uint32
	// A sensor touches nothing: it tells who comes in and goes out.
	Sensor                bool
	Friction, Restitution float64
	// Damping slows it down, a share of its speed per second.
	Damping, SpinDamping float64
	ID                   uint32 // the game's: its entity

	world   *World
	serial  uint32
	index   int
	invMass float64
	invI    float64
	rot     Rot
	aabb    AABB
	sleepT  float64
	asleep  bool
	stamp   int
}

// Asleep tells if it lies still, out of the solver until touched or set
// going.
func (b *Body) Asleep() bool { return b.asleep }

// Wake puts it back in the solver.
func (b *Body) Wake() { b.asleep, b.sleepT = false, 0 }

// Moves tells if it takes part in the step: awake, not static.
func (b *Body) moves() bool { return b.Kind != Static && !b.asleep }

func (b *Body) setMass() {
	b.invMass, b.invI = 0, 0
	if b.Kind != Dynamic {
		return
	}
	m := b.Mass
	if m <= 0 {
		m = 1
	}
	b.invMass = 1 / m
	if !b.FixedRotation {
		if i := b.Shape.inertia(m); i > 0 {
			b.invI = 1 / i
		}
	}
}

// Impact is a contact that began this step, with how fast the bodies
// came together along the normal, in m/s.
type Impact struct {
	A, B   *Body
	Point  V2
	Normal V2 // from A to B
	Speed  float64
}

// Sense is a body coming into a sensor (In) or leaving it.
type Sense struct {
	Sensor, Body *Body
	In           bool
}

// World holds the bodies and steps them.
type World struct {
	// MaxStep is the longest sub-step, in s: Step cuts longer frames.
	MaxStep float64
	// Iterations of the velocity solver per sub-step.
	Iterations int
	// Mix gives a contact's friction and restitution; nil takes the
	// geometric mean of the frictions, and the larger restitution.
	Mix func(a, b *Body) (friction, restitution float64)
	// ImpactSpeed is the slowest a contact must begin at to be an Impact.
	ImpactSpeed float64
	// Allow, if set, tells whether two bodies that overlap there push each
	// other apart: false lets them through (a step walked onto).
	Allow func(a, b *Body, m *Manifold) bool

	// Filled by Step, until the next.
	Impacts []Impact
	Senses  []Sense

	bodies   []*Body
	serial   uint32
	contacts map[uint64]*contact
	list     []*contact // the same, in the order they began: steps repeat exactly
	active   []*contact
	statics  grid
	movers   grid
	dirty    bool // a static body was added, moved or removed
	step     int  // stamps grid queries
	pass     int  // counts sub-steps: contacts not seen in one have ended
	poly     [2]poly
}

const (
	cellSize       = 4.0
	sleepSpeed     = 0.04 // m/s, and radians per second
	sleepTime      = 0.5
	restitutionMin = 1.0 // m/s: slower contacts don't bounce
	baumgarte      = 0.2
	maxCorrection  = 0.2
)

func NewWorld() *World {
	return &World{MaxStep: 1.0 / 120, Iterations: 8, ImpactSpeed: 1, contacts: map[uint64]*contact{},
		statics: newGrid(), movers: newGrid()}
}

// Add puts b in the world.
func (w *World) Add(b *Body) *Body {
	if b.world != nil {
		panic("physics: body added twice")
	}
	w.serial++
	b.world, b.serial, b.index = w, w.serial, len(w.bodies)
	b.rot = MakeRot(b.Angle)
	b.setMass()
	b.aabb = b.Shape.aabb(b.Pos, b.rot)
	w.bodies = append(w.bodies, b)
	if b.Kind == Static {
		w.dirty = true
	}
	return b
}

// Remove takes b out of the world, with its contacts.
func (w *World) Remove(b *Body) {
	if b.world != w {
		return
	}
	last := w.bodies[len(w.bodies)-1]
	w.bodies[b.index], last.index = last, b.index
	w.bodies = w.bodies[:len(w.bodies)-1]
	for _, c := range w.list {
		if (c.a == b || c.b == b) && c.seen >= 0 {
			if c.sensing {
				w.Senses = append(w.Senses, c.sense(false))
			}
			delete(w.contacts, pairKey(c.a, c.b))
			c.seen = -1 // dropped from the list next sub-step
		}
	}
	if b.Kind == Static {
		w.dirty = true
	}
	b.world = nil
}

// Teleport moves b there at once, waking it.
func (w *World) Teleport(b *Body, pos V2, angle float64) {
	b.Pos, b.Angle, b.rot = pos, angle, MakeRot(angle)
	b.aabb = b.Shape.aabb(pos, b.rot)
	b.Wake()
	if b.Kind == Static {
		w.dirty = true
	}
}

// SetKind turns b static, kinematic or dynamic.
func (w *World) SetKind(b *Body, k Kind) {
	if b.Kind == k {
		return
	}
	if b.Kind == Static || k == Static {
		w.dirty = true
	}
	b.Kind = k
	b.setMass()
	b.Wake()
	if k == Static {
		b.Vel, b.Spin = V2{}, 0
	}
}

// Bodies are the bodies in the world, in no particular order.
func (w *World) Bodies() []*Body { return w.bodies }

// Query calls fn for each body whose box overlaps box, until it returns
// false.
func (w *World) Query(box AABB, fn func(*Body) bool) {
	w.refreshStatics()
	w.step++
	for _, b := range w.bodies {
		if b.Kind != Static && b.aabb.Overlaps(box) && !fn(b) {
			return
		}
	}
	ok := true
	w.statics.query(box, w.step, func(b *Body) {
		if ok && b.aabb.Overlaps(box) {
			ok = fn(b)
		}
	})
}

// Overlap tells if a shape at pos, turned by angle, in the slice bottom to
// top, overlaps a body of the layers in mask (sensors aside), other than
// skip.
func (w *World) Overlap(s Shape, pos V2, angle, bottom, top float64, mask uint32, skip *Body) bool {
	rot := MakeRot(angle)
	box := s.aabb(pos, rot)
	s.world(pos, rot, &w.poly[0])
	hit := false
	w.Query(box, func(b *Body) bool {
		if b == skip || b.Sensor || b.Layer&mask == 0 || b.Bottom >= top || bottom >= b.Top {
			return true
		}
		b.Shape.world(b.Pos, b.rot, &w.poly[1])
		m := collide(&w.poly[0], &w.poly[1])
		hit = m.N > 0 && m.MinSep() < 0
		return !hit
	})
	return hit
}

// Touching calls fn for each body b touched in the last step, or nearly
// (sensors aside), with the normal from b toward it.
func (w *World) Touching(b *Body, fn func(o *Body, n V2)) {
	for _, c := range w.list {
		if c.seen != w.pass || c.sensing || c.a.Sensor || c.b.Sensor || (c.a != b && c.b != b) {
			continue
		}
		touch := false
		for i := range c.n {
			touch = touch || c.pts[i].sep < speculative
		}
		if !touch {
			continue
		}
		if c.a == b {
			fn(c.b, c.normal)
		} else {
			fn(c.a, c.normal.Scale(-1))
		}
	}
}

func (w *World) refreshStatics() {
	if !w.dirty {
		return
	}
	w.dirty = false
	w.statics.clear()
	for _, b := range w.bodies {
		if b.Kind == Static {
			b.aabb = b.Shape.aabb(b.Pos, b.rot)
			w.statics.insert(b)
		}
	}
}

// Step moves the world on by dt, in sub-steps no longer than MaxStep.
func (w *World) Step(dt float64) {
	w.Impacts, w.Senses = w.Impacts[:0], w.Senses[:0]
	if dt <= 0 {
		return
	}
	n := max(1, int(math.Ceil(dt/w.MaxStep-1e-9)))
	for range n {
		w.substep(dt / float64(n))
	}
}

func (w *World) substep(h float64) {
	w.refreshStatics()
	for _, b := range w.bodies {
		if b.asleep && (b.Vel != V2{} || b.Spin != 0) {
			b.Wake() // the game set it going
		}
		if !b.moves() {
			continue
		}
		b.rot = MakeRot(b.Angle)
		if b.Kind == Dynamic {
			b.Vel = b.Vel.Scale(1 / (1 + h*b.Damping))
			b.Spin /= 1 + h*b.SpinDamping
		}
		if b.FixedRotation || b.Kind == Static {
			b.Spin = 0
		}
	}
	w.findContacts(h)
	cs := w.solve(h)
	w.integrate(h)
	w.correct(cs)
	w.sleep(h)
}

// touches tells if a and b may collide at all.
func touches(a, b *Body) bool {
	if a.Layer&b.Mask == 0 || b.Layer&a.Mask == 0 || a.Bottom >= b.Top || b.Bottom >= a.Top {
		return false
	}
	if a.Sensor && b.Sensor {
		return false
	}
	// Two bodies the solver can't move: only a sensor cares.
	if a.invMass == 0 && b.invMass == 0 && !a.Sensor && !b.Sensor {
		return false
	}
	return true
}

func pairKey(a, b *Body) uint64 {
	if a.serial > b.serial {
		a, b = b, a
	}
	return uint64(a.serial)<<32 | uint64(b.serial)
}

// findContacts finds the pairs close enough to touch, and their contact
// points: new contacts are impacts, sensors tell who comes in and out.
func (w *World) findContacts(h float64) {
	w.pass++
	w.movers.clear()
	for _, b := range w.bodies {
		if b.moves() {
			// Swollen by the motion to come.
			box := b.Shape.aabb(b.Pos, b.rot)
			d := b.Vel.Scale(h)
			box = box.Union(AABB{box.Lo.Add(d), box.Hi.Add(d)})
			m := V2{speculative, speculative}
			b.aabb = AABB{box.Lo.Sub(m), box.Hi.Add(m)}
		}
		if b.Kind != Static {
			w.movers.insert(b) // sleeping ones too, to be woken
		}
	}
	for _, a := range w.bodies {
		if !a.moves() {
			continue
		}
		w.step++
		visit := func(b *Body) {
			if b == a || !b.aabb.Overlaps(a.aabb) {
				return
			}
			// Both move: the first of the two in the list takes the pair.
			if b.moves() && b.index < a.index {
				return
			}
			w.pair(a, b)
		}
		w.statics.query(a.aabb, w.step, visit)
		w.movers.query(a.aabb, w.step, visit)
	}
	// Contacts not seen this sub-step have ended.
	kept := w.list[:0]
	for _, c := range w.list {
		switch {
		case c.seen == w.pass:
			kept = append(kept, c)
		case c.seen >= 0:
			if c.sensing {
				w.Senses = append(w.Senses, c.sense(false))
			}
			delete(w.contacts, pairKey(c.a, c.b))
		}
	}
	clear(w.list[len(kept):])
	w.list = kept
}

func (w *World) pair(a, b *Body) {
	if !touches(a, b) {
		return
	}
	if a.serial > b.serial {
		a, b = b, a
	}
	a.Shape.world(a.Pos, MakeRot(a.Angle), &w.poly[0])
	b.Shape.world(b.Pos, MakeRot(b.Angle), &w.poly[1])
	m := collide(&w.poly[0], &w.poly[1])
	if m.N == 0 {
		return
	}
	key := pairKey(a, b)
	c := w.contacts[key]
	if !a.Sensor && !b.Sensor && w.Allow != nil && !w.Allow(a, b, &m) {
		if c != nil {
			delete(w.contacts, key)
			c.seen = -1 // dropped from the list next sub-step
		}
		return
	}
	if a.Sensor || b.Sensor {
		in := m.MinSep() < 0
		if c == nil {
			if !in {
				return
			}
			c = &contact{a: a, b: b}
			w.contacts[key] = c
			w.list = append(w.list, c)
		}
		c.seen = w.pass
		if in && !c.sensing {
			c.sensing = true
			w.Senses = append(w.Senses, c.sense(true))
		}
		if !in && c.sensing {
			c.sensing = false
			w.Senses = append(w.Senses, c.sense(false))
		}
		return
	}
	fresh := c == nil
	if fresh {
		c = &contact{a: a, b: b}
		w.contacts[key] = c
		w.list = append(w.list, c)
	}
	c.seen = w.pass
	c.update(m)
	// What sleeps wakes up when something moving comes against it.
	if a.asleep && b.moves() || b.asleep && a.moves() {
		a.Wake()
		b.Wake()
	}
	if fresh && m.MinSep() < linearSlop {
		p := m.Points[0]
		dv := b.Vel.Add(crossSV(b.Spin, p.Point.Sub(b.Pos))).Sub(a.Vel.Add(crossSV(a.Spin, p.Point.Sub(a.Pos))))
		if s := -dv.Dot(m.Normal); s >= w.ImpactSpeed {
			w.Impacts = append(w.Impacts, Impact{a, b, p.Point, m.Normal, s})
		}
	}
}

// integrate moves the bodies by their speeds; fast ones go no farther than
// the first static body in their way.
func (w *World) integrate(h float64) {
	for _, b := range w.bodies {
		if !b.moves() {
			continue
		}
		d := b.Vel.Scale(h)
		if b.Kind == Dynamic && d.Len() > b.Shape.inner()*0.5 {
			d = w.sweep(b, d)
		}
		b.Pos = b.Pos.Add(d)
		b.Angle += b.Spin * h
		b.rot = MakeRot(b.Angle)
	}
}

// sweep is how far along d the body goes before it overlaps a static body
// it touches: in steps of half its thickness, then halving the last.
func (w *World) sweep(b *Body, d V2) V2 {
	l := d.Len()
	n := int(math.Ceil(l / (b.Shape.inner() * 0.5)))
	n = min(n, 64)
	start := b.Pos
	box := b.Shape.aabb(start, b.rot)
	box = box.Union(AABB{box.Lo.Add(d), box.Hi.Add(d)})
	var near []*Body
	w.step++
	w.statics.query(box, w.step, func(s *Body) {
		if s.aabb.Overlaps(box) && touches(b, s) && !s.Sensor {
			near = append(near, s)
		}
	})
	if len(near) == 0 {
		return d
	}
	hits := func(t float64) bool {
		b.Shape.world(start.Add(d.Scale(t)), b.rot, &w.poly[0])
		for _, s := range near {
			s.Shape.world(s.Pos, s.rot, &w.poly[1])
			if m := collide(&w.poly[0], &w.poly[1]); m.N > 0 && m.MinSep() < -linearSlop && (w.Allow == nil || w.Allow(b, s, &m)) {
				return true
			}
		}
		return false
	}
	if hits(0) {
		return d // already in it: the contacts sort it out
	}
	prev := 0.0
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		if hits(t) {
			lo, hi := prev, t
			for range 6 {
				if mid := (lo + hi) / 2; hits(mid) {
					hi = mid
				} else {
					lo = mid
				}
			}
			return d.Scale(lo)
		}
		prev = t
	}
	return d
}

func (w *World) sleep(h float64) {
	for _, b := range w.bodies {
		if b.Kind != Dynamic || b.asleep {
			continue
		}
		if b.Vel.Len() < sleepSpeed && math.Abs(b.Spin) < sleepSpeed {
			if b.sleepT += h; b.sleepT > sleepTime {
				b.asleep, b.Vel, b.Spin = true, V2{}, 0
				b.aabb = b.Shape.aabb(b.Pos, b.rot)
			}
		} else {
			b.sleepT = 0
		}
	}
}

// grid buckets bodies by the cells their boxes cover.
type grid struct {
	cells map[[2]int32][]*Body
	used  [][2]int32
}

func newGrid() grid { return grid{cells: map[[2]int32][]*Body{}} }

func cellOf(x float64) int32 { return int32(math.Floor(x / cellSize)) }

func (g *grid) clear() {
	for _, k := range g.used {
		g.cells[k] = g.cells[k][:0]
	}
	g.used = g.used[:0]
}

func (g *grid) insert(b *Body) {
	for x := cellOf(b.aabb.Lo.X); x <= cellOf(b.aabb.Hi.X); x++ {
		for y := cellOf(b.aabb.Lo.Y); y <= cellOf(b.aabb.Hi.Y); y++ {
			k := [2]int32{x, y}
			if len(g.cells[k]) == 0 {
				g.used = append(g.used, k)
			}
			g.cells[k] = append(g.cells[k], b)
		}
	}
}

// query calls fn once per body in the cells box covers.
func (g *grid) query(box AABB, stamp int, fn func(*Body)) {
	for x := cellOf(box.Lo.X); x <= cellOf(box.Hi.X); x++ {
		for y := cellOf(box.Lo.Y); y <= cellOf(box.Hi.Y); y++ {
			for _, b := range g.cells[[2]int32{x, y}] {
				if b.stamp != stamp {
					b.stamp = stamp
					fn(b)
				}
			}
		}
	}
}
