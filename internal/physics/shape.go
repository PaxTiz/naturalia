package physics

import (
	"cmp"
	"math"
	"slices"
)

const maxVerts = 8

// Shape is a convex polygon rounded by Radius, in its body's frame: one
// corner makes a circle, two a capsule (or a wall with some thickness),
// three to eight a polygon. The body turns around its frame's origin,
// which should be the shape's middle.
type Shape struct {
	V      []V2 // the core's corners, counter-clockwise
	N      []V2 // outward normal of each edge V[i] → V[i+1]; a capsule's two sides
	Radius float64
}

func Circle(r float64) Shape { return Shape{V: []V2{{}}, Radius: r} }

// Capsule is a segment from a to b, rounded by r.
func Capsule(a, b V2, r float64) Shape {
	n := b.Sub(a).Norm()
	return Shape{V: []V2{a, b}, N: []V2{{n.Y, -n.X}, {-n.Y, n.X}}, Radius: r}
}

// Box is a rectangle hx by hy from its middle, its corners rounded by r.
func Box(hx, hy, r float64) Shape {
	return Polygon([]V2{{-hx, -hy}, {hx, -hy}, {hx, hy}, {-hx, hy}}, r)
}

// OffsetBox is a box centered at c, turned by angle.
func OffsetBox(hx, hy float64, c V2, angle, r float64) Shape {
	s := Box(hx, hy, r)
	rot := MakeRot(angle)
	for i := range s.V {
		s.V[i] = rot.Apply(s.V[i]).Add(c)
		s.N[i] = rot.Apply(s.N[i])
	}
	return s
}

// Polygon is the convex polygon with these corners, counter-clockwise,
// rounded by r. Up to 8.
func Polygon(pts []V2, r float64) Shape {
	if len(pts) < 3 || len(pts) > maxVerts {
		panic("physics: a polygon has 3 to 8 corners")
	}
	s := Shape{V: append([]V2(nil), pts...), N: make([]V2, len(pts)), Radius: r}
	for i := range pts {
		e := pts[(i+1)%len(pts)].Sub(pts[i]).Norm()
		s.N[i] = V2{e.Y, -e.X}
	}
	return s
}

// Extent is how far the shape reaches from its frame's origin.
func (s Shape) Extent() float64 {
	m := 0.0
	for _, v := range s.V {
		m = math.Max(m, v.Len())
	}
	return m + s.Radius
}

// inner is about the thinnest the shape is, halved: what a step may move
// it without going through a thin wall.
func (s Shape) inner() float64 {
	if len(s.V) < 3 {
		return s.Radius
	}
	m := math.Inf(1)
	for i, n := range s.N {
		// From the middle to each edge.
		m = math.Min(m, -s.V[i].Dot(n))
	}
	return math.Max(m, 0) + s.Radius
}

// inertia is the moment of inertia of a body of mass m with this shape,
// turning about its frame's origin: the core polygon (or segment) swollen
// by the radius, roughly.
func (s Shape) inertia(m float64) float64 {
	r := s.Radius
	switch len(s.V) {
	case 1:
		return m * (r*r/2 + s.V[0].Dot(s.V[0]))
	case 2:
		l := s.V[1].Sub(s.V[0]).Len() + 2*r
		mid := s.V[0].Lerp(s.V[1], 0.5)
		return m * ((l*l+4*r*r)/12 + mid.Dot(mid))
	}
	// A polygon: by triangles from the origin.
	area, i := 0.0, 0.0
	for k, a := range s.V {
		b := s.V[(k+1)%len(s.V)]
		cr := a.Cross(b)
		area += cr / 2
		i += cr * (a.Dot(a) + a.Dot(b) + b.Dot(b)) / 12
	}
	if area <= 0 {
		return m * r * r / 2
	}
	// The rounding adds about a band of width r all around.
	ext := s.Extent()
	return m * (i/area + r*ext*0.5)
}

// world places the shape at pos turned by rot, into p.
func (s *Shape) world(pos V2, rot Rot, p *poly) {
	p.n = len(s.V)
	p.r = s.Radius
	for i, v := range s.V {
		p.v[i] = rot.Apply(v).Add(pos)
	}
	for i, n := range s.N {
		p.nm[i] = rot.Apply(n)
	}
}

// aabb is the box around the shape at pos turned by rot.
func (s *Shape) aabb(pos V2, rot Rot) AABB {
	lo, hi := V2{math.Inf(1), math.Inf(1)}, V2{math.Inf(-1), math.Inf(-1)}
	for _, v := range s.V {
		w := rot.Apply(v).Add(pos)
		lo = V2{math.Min(lo.X, w.X), math.Min(lo.Y, w.Y)}
		hi = V2{math.Max(hi.X, w.X), math.Max(hi.Y, w.Y)}
	}
	r := V2{s.Radius, s.Radius}
	return AABB{lo.Sub(r), hi.Add(r)}
}

// poly is a shape in the world, without allocating.
type poly struct {
	v  [maxVerts]V2
	nm [maxVerts]V2
	n  int
	r  float64
}

// Hull is the convex polygon around points (a box seen from above,
// turned any way), rounded by r; a circle of radius r around them if they
// barely spread. Up to 8 corners: the farthest apart are kept.
func Hull(pts []V2, r float64) Shape {
	ps := slices.Clone(pts)
	slices.SortFunc(ps, func(a, b V2) int {
		if c := cmp.Compare(a.X, b.X); c != 0 {
			return c
		}
		return cmp.Compare(a.Y, b.Y)
	})
	// Andrew's monotone chain, counter-clockwise.
	var h []V2
	for pass := range 2 {
		start := len(h)
		for i := range ps {
			p := ps[i]
			if pass == 1 {
				p = ps[len(ps)-1-i]
			}
			for len(h) >= start+2 && h[len(h)-1].Sub(h[len(h)-2]).Cross(p.Sub(h[len(h)-2])) <= 1e-9 {
				h = h[:len(h)-1]
			}
			h = append(h, p)
		}
		h = h[:len(h)-1]
	}
	for len(h) > maxVerts { // drop the corner that matters least
		least, at := math.Inf(1), 0
		for i := range h {
			a, b, c := h[(i+len(h)-1)%len(h)], h[i], h[(i+1)%len(h)]
			if area := b.Sub(a).Cross(c.Sub(a)); area < least {
				least, at = area, i
			}
		}
		h = slices.Delete(h, at, at+1)
	}
	if len(h) < 3 {
		var c V2
		for _, p := range pts {
			c = c.Add(p)
		}
		c = c.Scale(1 / float64(max(1, len(pts))))
		far := 0.0
		for _, p := range pts {
			far = math.Max(far, p.Sub(c).Len())
		}
		if len(h) == 2 {
			return Capsule(h[0], h[1], r)
		}
		s := Circle(r + far)
		s.V[0] = c
		return s
	}
	return Polygon(h, r)
}
