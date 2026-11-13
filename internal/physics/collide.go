package physics

import "math"

// Contacts follow Box2D (v3): the normal goes from A to B, each point lies
// midway between the two surfaces, and its separation is negative where
// they overlap. Points are found a little before the shapes touch
// (speculative), so that the solver stops them right at the surface.

const (
	linearSlop  = 0.005 // m of overlap left alone, against jitter
	speculative = 4 * linearSlop
)

type Manifold struct {
	Normal V2 // from A to B
	Points [2]ManifoldPoint
	N      int
}

type ManifoldPoint struct {
	Point V2
	Sep   float64
	ID    uint16 // which features touch: keeps a point's impulse from step to step
}

func makeID(a, b int) uint16 { return uint16(a&0xff)<<8 | uint16(b&0xff) }

// MinSep is the deepest point's separation, +Inf without points.
func (m *Manifold) MinSep() float64 {
	s := math.Inf(1)
	for i := range m.N {
		s = math.Min(s, m.Points[i].Sep)
	}
	return s
}

func (m *Manifold) add(p V2, sep float64, id uint16) {
	if sep > speculative {
		return
	}
	m.Points[m.N] = ManifoldPoint{p, sep, id}
	m.N++
}

// collide finds where two shapes in the world touch.
func collide(a, b *poly) Manifold {
	switch {
	case a.n == 1 && b.n == 1:
		return collideCircles(a, b)
	case b.n == 1:
		return collidePolyCircle(a, b)
	case a.n == 1:
		m := collidePolyCircle(b, a)
		m.Normal = m.Normal.Scale(-1)
		return m
	}
	return collidePolygons(a, b)
}

func collideCircles(a, b *poly) Manifold {
	var m Manifold
	d := b.v[0].Sub(a.v[0])
	l := d.Len()
	n := V2{1, 0}
	if l > 1e-9 {
		n = d.Scale(1 / l)
	}
	ca, cb := a.v[0].Add(n.Scale(a.r)), b.v[0].Sub(n.Scale(b.r))
	m.Normal = n
	m.add(ca.Lerp(cb, 0.5), l-a.r-b.r, 0)
	return m
}

// collidePolyCircle: a polygon or a capsule a, a circle b.
func collidePolyCircle(a, b *poly) Manifold {
	var m Manifold
	c := b.v[0]
	radius := a.r + b.r
	// The face the center is the farthest out of.
	best, sep := 0, math.Inf(-1)
	for i := range a.n {
		if s := a.nm[i].Dot(c.Sub(a.v[i])); s > sep {
			best, sep = i, s
		}
	}
	if sep > radius+speculative {
		return m
	}
	v1, v2 := a.v[best], a.v[(best+1)%a.n]
	n := a.nm[best]
	at, corner := v1, false
	if sep > 1e-6 {
		switch {
		case c.Sub(v1).Dot(v2.Sub(v1)) < 0:
			corner = true
		case c.Sub(v2).Dot(v1.Sub(v2)) < 0:
			at, corner = v2, true
		}
	}
	id := 0
	if corner {
		// Around a corner: the normal goes from the corner to the center.
		d := c.Sub(at)
		l := d.Len()
		if l > radius+speculative {
			return m
		}
		if l > 1e-9 {
			n = d.Scale(1 / l)
		}
		sep = l
		id = 1
	}
	ca := c.Sub(n.Scale(sep)).Add(n.Scale(a.r)) // on a's surface
	cb := c.Sub(n.Scale(b.r))
	m.Normal = n
	m.add(ca.Lerp(cb, 0.5), sep-radius, makeID(best, id))
	return m
}

// maxSeparation is the edge of p along whose normal q stands the farthest
// out, and how far.
func maxSeparation(p, q *poly) (int, float64) {
	best, sep := 0, math.Inf(-1)
	for i := range p.n {
		s := math.Inf(1)
		for j := range q.n {
			s = math.Min(s, p.nm[i].Dot(q.v[j].Sub(p.v[i])))
		}
		if s > sep {
			best, sep = i, s
		}
	}
	return best, sep
}

// collidePolygons: two polygons or capsules.
func collidePolygons(a, b *poly) Manifold {
	var m Manifold
	edgeA, sepA := maxSeparation(a, b)
	edgeB, sepB := maxSeparation(b, a)
	radius := a.r + b.r
	if sepA > speculative+radius || sepB > speculative+radius {
		return m
	}
	// The reference face is the one standing out the most, A's on a tie.
	p1, p2, edge1, sep, flip := a, b, edgeA, sepA, false
	if sepB > 0.1*linearSlop+sepA {
		p1, p2, edge1, sep, flip = b, a, edgeB, sepB, true
	}
	n := p1.nm[edge1]
	// The incident edge: the one of p2 that faces it the most.
	edge2, low := 0, math.Inf(1)
	for i := range p2.n {
		if d := n.Dot(p2.nm[i]); d < low {
			edge2, low = i, d
		}
	}
	i11, i12 := edge1, (edge1+1)%p1.n
	i21, i22 := edge2, (edge2+1)%p2.n
	v11, v12, v21, v22 := p1.v[i11], p1.v[i12], p2.v[i21], p2.v[i22]

	if sep > 0.1*linearSlop {
		// Apart at the core: rounded corners may meet first.
		f1, f2, d2 := segmentDistance(v11, v12, v21, v22)
		corner := func(c1, c2 V2, j1, j2 int) Manifold {
			d := math.Sqrt(d2)
			if d > speculative+radius {
				return m
			}
			nn := c2.Sub(c1).Norm()
			s1, s2 := c1.Add(nn.Scale(p1.r)), c2.Sub(nn.Scale(p2.r))
			if flip {
				nn = nn.Scale(-1)
			}
			m.Normal = nn
			m.add(s1.Lerp(s2, 0.5), d-radius, makeID(j1, j2))
			return m
		}
		switch {
		case f1 == 0 && f2 == 0:
			return corner(v11, v21, i11, i21)
		case f1 == 0 && f2 == 1:
			return corner(v11, v22, i11, i22)
		case f1 == 1 && f2 == 0:
			return corner(v12, v21, i12, i21)
		case f1 == 1 && f2 == 1:
			return corner(v12, v22, i12, i22)
		}
	}
	return clip(p1, p2, i11, i12, i21, i22, flip)
}

// clip cuts the incident edge v21–v22 to the sides of the reference edge
// v11–v12: up to two points.
func clip(p1, p2 *poly, i11, i12, i21, i22 int, flip bool) Manifold {
	var m Manifold
	v11, v12, v21, v22 := p1.v[i11], p1.v[i12], p2.v[i21], p2.v[i22]
	n := p1.nm[i11]
	t := n.Perp()
	lower1, upper1 := 0.0, v12.Sub(v11).Dot(t)
	// The incident edge runs the other way.
	upper2, lower2 := v21.Sub(v11).Dot(t), v22.Sub(v11).Dot(t)
	vLower, vUpper := v22, v21
	if lower2 < lower1 && upper2-lower2 > 1e-9 {
		vLower = v22.Lerp(v21, clamp01((lower1-lower2)/(upper2-lower2)))
	}
	if upper2 > upper1 && upper2-lower2 > 1e-9 {
		vUpper = v22.Lerp(v21, clamp01((upper1-lower2)/(upper2-lower2)))
	}
	sLower, sUpper := vLower.Sub(v11).Dot(n), vUpper.Sub(v11).Dot(n)
	// Midway between the surfaces.
	vLower = vLower.Add(n.Scale(0.5 * (p1.r - p2.r - sLower)))
	vUpper = vUpper.Add(n.Scale(0.5 * (p1.r - p2.r - sUpper)))
	radius := p1.r + p2.r
	if !flip {
		m.Normal = n
		m.add(vLower, sLower-radius, makeID(i11, i22))
		m.add(vUpper, sUpper-radius, makeID(i12, i21))
	} else {
		m.Normal = n.Scale(-1)
		m.add(vUpper, sUpper-radius, makeID(i21, i12))
		m.add(vLower, sLower-radius, makeID(i22, i11))
	}
	if m.N == 2 && m.Points[0].Point.Sub(m.Points[1].Point).Len() < linearSlop {
		// One point twice (an edge seen end on).
		if m.Points[1].Sep < m.Points[0].Sep {
			m.Points[0] = m.Points[1]
		}
		m.N = 1
	}
	return m
}

// segmentDistance finds the closest points of segments p1–q1 and p2–q2:
// their fractions along each, and the squared distance between them.
func segmentDistance(p1, q1, p2, q2 V2) (f1, f2, d2 float64) {
	d1, d2v, r := q1.Sub(p1), q2.Sub(p2), p1.Sub(p2)
	dd1, dd2 := d1.Dot(d1), d2v.Dot(d2v)
	rd1, rd2 := r.Dot(d1), r.Dot(d2v)
	const eps = 1e-12
	switch {
	case dd1 < eps && dd2 < eps:
		// Two points.
	case dd1 < eps:
		f2 = clamp01(rd2 / dd2)
	case dd2 < eps:
		f1 = clamp01(-rd1 / dd1)
	default:
		d12 := d1.Dot(d2v)
		den := dd1*dd2 - d12*d12
		if den != 0 {
			f1 = clamp01((d12*rd2 - rd1*dd2) / den)
		}
		f2 = (d12*f1 + rd2) / dd2
		// Back on the segments.
		if f2 < 0 {
			f2, f1 = 0, clamp01(-rd1/dd1)
		} else if f2 > 1 {
			f2, f1 = 1, clamp01((d12-rd1)/dd1)
		}
	}
	c1, c2 := p1.Add(d1.Scale(f1)), p2.Add(d2v.Scale(f2))
	d := c1.Sub(c2)
	return f1, f2, d.Dot(d)
}

func clamp01(x float64) float64 { return math.Max(0, math.Min(1, x)) }
