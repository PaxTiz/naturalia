package physics

import "math"

// The solver works on speeds by impulses, contact after contact, a few
// times over (sequential impulses, as in Box2D): friction, then the push
// that keeps bodies apart, then the bounce; then it moves the bodies out of
// what overlap is left.

type contact struct {
	a, b    *Body
	normal  V2
	pts     [2]cpoint
	n       int
	seen    int // the sub-step it was last found in
	sensing bool

	friction, restitution float64
}

type cpoint struct {
	id     uint16
	point  V2      // in the world, when found
	sep    float64 // then
	rA, rB V2      // from each body's middle
	lA, lB V2      // the same, in each body's frame

	normalMass, tangentMass       float64
	normalImpulse, tangentImpulse float64
	maxImpulse                    float64
	relVel                        float64 // along the normal, before solving
}

// update takes the new contact points, keeping the impulses of those
// still there, to start from (warm starting).
func (c *contact) update(m Manifold) {
	old, oldN := c.pts, c.n
	c.normal, c.n = m.Normal, m.N
	for i := range m.N {
		mp := m.Points[i]
		p := cpoint{id: mp.ID, point: mp.Point, sep: mp.Sep}
		for j := range oldN {
			if old[j].id == p.id {
				p.normalImpulse, p.tangentImpulse = old[j].normalImpulse, old[j].tangentImpulse
			}
		}
		c.pts[i] = p
	}
}

func (c *contact) sense(in bool) Sense {
	if c.a.Sensor {
		return Sense{c.a, c.b, in}
	}
	return Sense{c.b, c.a, in}
}

// relative is b's speed at its point p, less a's.
func relative(a, b *Body, rA, rB V2) V2 {
	return b.Vel.Add(crossSV(b.Spin, rB)).Sub(a.Vel.Add(crossSV(a.Spin, rA)))
}

func apply(a, b *Body, rA, rB, imp V2) {
	a.Vel = a.Vel.Sub(imp.Scale(a.invMass))
	a.Spin -= a.invI * rA.Cross(imp)
	b.Vel = b.Vel.Add(imp.Scale(b.invMass))
	b.Spin += b.invI * rB.Cross(imp)
}

// solve sets the speeds so that touching bodies don't go into each other.
func (w *World) solve(h float64) []*contact {
	cs := w.active[:0]
	for _, c := range w.list {
		a, b := c.a, c.b
		if c.sensing || a.Sensor || b.Sensor || c.n == 0 || (!a.moves() && !b.moves()) {
			continue
		}
		if w.Mix != nil {
			c.friction, c.restitution = w.Mix(a, b)
		} else {
			c.friction, c.restitution = math.Sqrt(a.Friction*b.Friction), math.Max(a.Restitution, b.Restitution)
		}
		n := c.normal
		t := V2{n.Y, -n.X}
		for i := range c.n {
			p := &c.pts[i]
			p.rA, p.rB = p.point.Sub(a.Pos), p.point.Sub(b.Pos)
			p.lA, p.lB = a.rot.Invert(p.rA), b.rot.Invert(p.rB)
			rnA, rnB := p.rA.Cross(n), p.rB.Cross(n)
			if k := a.invMass + b.invMass + a.invI*rnA*rnA + b.invI*rnB*rnB; k > 0 {
				p.normalMass = 1 / k
			}
			rtA, rtB := p.rA.Cross(t), p.rB.Cross(t)
			if k := a.invMass + b.invMass + a.invI*rtA*rtA + b.invI*rtB*rtB; k > 0 {
				p.tangentMass = 1 / k
			}
			p.relVel = relative(a, b, p.rA, p.rB).Dot(n)
			p.maxImpulse = 0
			apply(a, b, p.rA, p.rB, n.Scale(p.normalImpulse).Add(t.Scale(p.tangentImpulse)))
		}
		cs = append(cs, c)
	}
	w.active = cs

	for range w.Iterations {
		for _, c := range cs {
			c.solveVelocity(h)
		}
	}
	// The bounce, for the contacts that came together fast enough.
	for _, c := range cs {
		if c.restitution == 0 {
			continue
		}
		for i := range c.n {
			p := &c.pts[i]
			if p.relVel > -restitutionMin || p.maxImpulse == 0 {
				continue
			}
			vn := relative(c.a, c.b, p.rA, p.rB).Dot(c.normal)
			imp := -p.normalMass * (vn + c.restitution*p.relVel)
			total := math.Max(p.normalImpulse+imp, 0)
			imp = total - p.normalImpulse
			p.normalImpulse = total
			apply(c.a, c.b, p.rA, p.rB, c.normal.Scale(imp))
		}
	}
	return cs
}

func (c *contact) solveVelocity(h float64) {
	a, b, n := c.a, c.b, c.normal
	t := V2{n.Y, -n.X}
	for i := range c.n {
		p := &c.pts[i]
		vt := relative(a, b, p.rA, p.rB).Dot(t)
		limit := c.friction * p.normalImpulse
		total := math.Max(-limit, math.Min(limit, p.tangentImpulse-p.tangentMass*vt))
		imp := total - p.tangentImpulse
		p.tangentImpulse = total
		apply(a, b, p.rA, p.rB, t.Scale(imp))
	}
	for i := range c.n {
		p := &c.pts[i]
		vn := relative(a, b, p.rA, p.rB).Dot(n)
		target := 0.0
		if p.sep > 0 {
			target = -p.sep / h // not touching yet: it may come up to the surface
		}
		total := math.Max(p.normalImpulse-p.normalMass*(vn-target), 0)
		imp := total - p.normalImpulse
		p.normalImpulse = total
		p.maxImpulse = math.Max(p.maxImpulse, imp)
		apply(a, b, p.rA, p.rB, n.Scale(imp))
	}
}

// correct moves overlapping bodies apart, a share of the overlap at a
// time, for what the speeds left.
func (w *World) correct(cs []*contact) {
	for range 3 {
		for _, c := range cs {
			a, b, n := c.a, c.b, c.normal
			for i := range c.n {
				p := &c.pts[i]
				rA, rB := a.rot.Apply(p.lA), b.rot.Apply(p.lB)
				sep := p.sep + b.Pos.Add(rB).Sub(a.Pos.Add(rA)).Dot(n)
				corr := math.Max(-maxCorrection, math.Min(0, baumgarte*(sep+linearSlop)))
				if corr == 0 {
					continue
				}
				rnA, rnB := rA.Cross(n), rB.Cross(n)
				k := a.invMass + b.invMass + a.invI*rnA*rnA + b.invI*rnB*rnB
				if k <= 0 {
					continue
				}
				imp := n.Scale(-corr / k)
				a.Pos = a.Pos.Sub(imp.Scale(a.invMass))
				a.Angle -= a.invI * rA.Cross(imp)
				b.Pos = b.Pos.Add(imp.Scale(b.invMass))
				b.Angle += b.invI * rB.Cross(imp)
				a.rot, b.rot = MakeRot(a.Angle), MakeRot(b.Angle)
			}
		}
	}
}
