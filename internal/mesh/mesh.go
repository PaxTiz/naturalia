package mesh

import (
	"math"
	"naturalia/internal/mathx"
)

// Meshes are built on the CPU, triangle by triangle, then merged: one
// mesh per material, lighting mode and room, so the base is drawn in a few
// calls.

// Lighting modes of the object shader.
const (
	LitSun     = 0 // outdoors
	LitIndoor  = 1 // ceiling lamps
	LitBack    = 2 // shells: sun outside, lamps on the back faces seen from inside
	LitTowards = 3 // walls: lamps on the faces turned toward the room's center
)

// Parts holds merged meshes by piece, in the order they appear.
type Parts struct {
	byKey  map[Key]*Builder
	Keys   []Key
	origin vec3 // of every mesh
}

// Key says what a mesh is: material token, lighting mode, and the world
// point that LitTowards walls face.
type Key struct {
	Tok string
	Lit float32
	At  vec3
}

// NewParts starts an empty set whose meshes are moved by origin.
func NewParts(origin vec3) *Parts {
	return &Parts{byKey: map[Key]*Builder{}, origin: origin}
}

// Part is the mesh of a piece, created on first use.
func (s *Parts) Part(tok string, lit float32, at vec3) *Builder {
	k := Key{tok, lit, at}
	if s.byKey[k] == nil {
		s.byKey[k] = &Builder{origin: s.origin}
		s.Keys = append(s.Keys, k)
	}
	return s.byKey[k]
}

// Mesh is the mesh of a piece already built.
func (s *Parts) Mesh(k Key) *Builder { return s.byKey[k] }

// Builder collects triangles, moved by origin.
type Builder struct {
	V, N, UV []float32
	origin   vec3 // added to every point: the frame the triangles are built in
	// uvf lays the plate texture (the panel material) on the surface being built: the
	// texture coordinates of a local point. Without it, the middle of a plate.
	uvf func(p vec3) [2]float64
}

func (m *Builder) emit(p, nrm vec3) {
	t := [2]float64{0.5, 0.5}
	if m.uvf != nil {
		t = m.uvf(p)
	}
	p = m.origin.Add(p)
	m.V = append(m.V, float32(p.X), float32(p.Y), float32(p.Z))
	m.N = append(m.N, float32(nrm.X), float32(nrm.Y), float32(nrm.Z))
	m.UV = append(m.UV, float32(t[0]), float32(t[1]))
}

// unwrap brings angle a within half a turn of ref.
func unwrap(a, ref float64) float64 {
	for a < ref-math.Pi {
		a += 2 * math.Pi
	}
	for a > ref+math.Pi {
		a -= 2 * math.Pi
	}
	return a
}

// tri adds a flat triangle facing away from center.
func (m *Builder) tri(a, b, c, center vec3) {
	nrm := b.Sub(a).Cross(c.Sub(a))
	if nrm.Dot(a.Add(b).Add(c).Scale(1.0/3).Sub(center)) < 0 {
		b, c = c, b
		nrm = nrm.Scale(-1)
	}
	nrm = nrm.Norm()
	m.emit(a, nrm)
	m.emit(b, nrm)
	m.emit(c, nrm)
}

// smooth adds a triangle with its own vertex normals, wound to face them.
func (m *Builder) smooth(a, b, c, na, nb, nc vec3) {
	if b.Sub(a).Cross(c.Sub(a)).Dot(na.Add(nb).Add(nc)) < 0 {
		b, c, nb, nc = c, b, nc, nb
	}
	m.emit(a, na)
	m.emit(b, nb)
	m.emit(c, nc)
}

func (m *Builder) quad(a, b, c, d, center vec3) {
	m.tri(a, b, c, center)
	m.tri(a, c, d, center)
}

// Box adds a box centered at the local point c, its width (X) along the
// horizontal direction across.
func (m *Builder) Box(c, size, across vec3, bevel float64) {
	ex := across.Norm()
	m.BoxBasis(c, [3]vec3{ex, {0, 1, 0}, ex.Cross(vec3{0, 1, 0})}, size, bevel)
}

// BoxBasis adds a box along the unit axes ax, with edges and corners
// chamfered by bevel: they catch the light instead of being sharp.
func (m *Builder) BoxBasis(c vec3, ax [3]vec3, size vec3, bevel float64) {
	h := [3]float64{size.X / 2, size.Y / 2, size.Z / 2}
	bevel = math.Min(bevel, 0.45*math.Min(h[0], math.Min(h[1], h[2])))
	// pt is the corner s pushed out along one axis: the corner of that face.
	pt := func(axis int, s [3]float64) vec3 {
		p := c
		for k := range 3 {
			q := s[k] * (h[k] - bevel)
			if k == axis {
				q = s[k] * h[k]
			}
			p = p.Add(ax[k].Scale(q))
		}
		return p
	}
	signs := [2]float64{-1, 1}
	for a := range 3 {
		u, v := (a+1)%3, (a+2)%3
		for _, sa := range signs {
			var q [4]vec3
			for i, uv := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
				var s [3]float64
				s[a], s[u], s[v] = sa, uv[0], uv[1]
				q[i] = pt(a, s)
			}
			// Plates about 1.2 m wide and 1 m tall fill the face exactly, the
			// texture going down its most vertical axis.
			U, V := u, v
			if math.Abs(ax[u].Y) > math.Abs(ax[v].Y) {
				U, V = v, u
			}
			down := 1.0
			if ax[V].Y < 0 {
				down = -1
			}
			nu, nv := math.Max(1, math.Round(2*h[U]/1.2)), math.Max(1, math.Round(2*h[V]/1.0))
			m.uvf = func(p vec3) [2]float64 {
				d := p.Sub(c)
				return [2]float64{(d.Dot(ax[U]) + h[U]) / (2 * h[U]) * nu, (h[V] - down*d.Dot(ax[V])) / (2 * h[V]) * nv}
			}
			m.quad(q[0], q[1], q[2], q[3], c)
			m.uvf = nil
			if bevel == 0 {
				continue
			}
			// The bevel along axis v between faces a and u.
			for _, su := range signs {
				var s0, s1 [3]float64
				s0[a], s0[u], s0[v] = sa, su, -1
				s1[a], s1[u], s1[v] = sa, su, 1
				m.quad(pt(a, s0), pt(a, s1), pt(u, s1), pt(u, s0), c)
			}
		}
	}
	if bevel == 0 {
		return
	}
	for _, sx := range signs {
		for _, sy := range signs {
			for _, sz := range signs {
				s := [3]float64{sx, sy, sz}
				m.tri(pt(0, s), pt(1, s), pt(2, s), c)
			}
		}
	}
}

// Slab extrudes the convex polygon pts, given in order on a plane, by
// depth along that plane's normal n: a gable end, any flat odd shape.
func (m *Builder) Slab(pts []vec3, n vec3, depth float64) {
	n = n.Norm().Scale(depth / 2)
	var mid vec3
	for _, p := range pts {
		mid = mid.Add(p)
	}
	mid = mid.Scale(1 / float64(len(pts)))
	for i := range pts {
		a, b := pts[i], pts[(i+1)%len(pts)]
		if i > 0 && i < len(pts)-1 {
			m.tri(pts[0].Add(n), a.Add(n), b.Add(n), mid)
			m.tri(pts[0].Sub(n), a.Sub(n), b.Sub(n), mid)
		}
		m.quad(a.Add(n), b.Add(n), b.Sub(n), a.Sub(n), mid)
	}
}

// Wedge adds a ramp standing on the local point c: size.X long along the
// horizontal direction across, size.Z wide, rising from nothing to size.Y.
func (m *Builder) Wedge(c, size, across vec3) {
	up := vec3{0, 1, 0}
	ex := across.Norm()
	ez := ex.Cross(up)
	x, z := ex.Scale(size.X/2), ez.Scale(size.Z/2)
	b0, b1 := c.Sub(x).Sub(z), c.Add(x).Sub(z)
	b2, b3 := c.Add(x).Add(z), c.Sub(x).Add(z)
	t1, t2 := b1.Add(up.Scale(size.Y)), b2.Add(up.Scale(size.Y))
	mid := c.Add(ex.Scale(size.X / 6)).Add(up.Scale(size.Y / 3))
	m.quad(b0, b1, b2, b3, mid)
	m.quad(b0, t1, t2, b3, mid)
	m.quad(b1, t1, t2, b2, mid)
	m.tri(b0, b1, t1, mid)
	m.tri(b3, b2, t2, mid)
}

// Prism adds a faceted cylinder or cone frustum standing on base along
// axis: radius r0 at the bottom, r1 at the top, its top edge chamfered.
func (m *Builder) Prism(base, axis vec3, r0, r1, h float64, sides int, bevel float64) {
	// Round pieces are round: more sides and smooth normals around them
	// (the chamfer still creases). Few sides stay faceted on purpose.
	round := sides >= 8
	if sides >= 10 {
		sides = min(64, sides*2)
	}
	axis = axis.Norm()
	u, w := mathx.TangentBasis(axis)
	center := base.Add(axis.Scale(h / 2))
	type level struct{ y, r float64 }
	levels := []level{{0, r0}, {h, r1}}
	if bevel > 0 {
		levels = []level{{0, r0}, {h - bevel, r1}, {h, r1 - bevel}}
	}
	radial := func(k int) vec3 {
		a := 2 * math.Pi * float64(k) / float64(sides)
		return u.Scale(math.Cos(a)).Add(w.Scale(math.Sin(a)))
	}
	ring := func(l level, k int) vec3 {
		return base.Add(axis.Scale(l.y)).Add(radial(k).Scale(l.r))
	}
	// Plates around the sides, square tiles on the caps.
	nu, nv := math.Max(1, math.Round(2*math.Pi*math.Max(r0, r1)/1.2)), math.Max(1, math.Round(h/1.0))
	for i := 0; i+1 < len(levels); i++ {
		lo, hi := levels[i], levels[i+1]
		for k := range sides {
			ac := 2 * math.Pi * (float64(k) + 0.5) / float64(sides)
			m.uvf = func(p vec3) [2]float64 {
				d := p.Sub(base)
				a := unwrap(math.Atan2(d.Dot(w), d.Dot(u)), ac)
				return [2]float64{a / (2 * math.Pi) * nu, (h - d.Dot(axis)) / h * nv}
			}
			if !round {
				m.quad(ring(lo, k), ring(lo, k+1), ring(hi, k+1), ring(hi, k), center)
				continue
			}
			// The side's normal leans with its slope.
			nrm := func(k int) vec3 { return radial(k).Scale(hi.y - lo.y).Add(axis.Scale(lo.r - hi.r)).Norm() }
			n0, n1 := nrm(k), nrm(k+1)
			m.smooth(ring(lo, k), ring(lo, k+1), ring(hi, k+1), n0, n1, n1)
			m.smooth(ring(lo, k), ring(hi, k+1), ring(hi, k), n0, n1, n0)
		}
	}
	m.uvf = func(p vec3) [2]float64 {
		d := p.Sub(base)
		return [2]float64{d.Dot(u)/1.2 + 0.5, d.Dot(w)/1.2 + 0.5}
	}
	top := levels[len(levels)-1]
	for k := range sides {
		m.tri(base, ring(levels[0], k), ring(levels[0], k+1), center)
		m.tri(base.Add(axis.Scale(h)), ring(top, k), ring(top, k+1), center)
	}
	m.uvf = nil
}

// DomeGores is the number of flat gores of a dome.
const DomeGores = 16

// Dome adds a half ellipsoid on c (radius a, height bh, no floor) made of
// flat gores, like a geodesic shell: each gore is flat across and smooth
// from bottom to top, its corners on the ellipsoid.
func (m *Builder) Dome(c vec3, a, bh float64) {
	const rings, per = 12, 3 // per: rows of vertices across a gore
	pt := func(i int, g, f float64) (vec3, vec3) {
		phi := float64(i) / rings * math.Pi / 2
		cp, sp := math.Cos(phi), math.Sin(phi)
		t0 := g * 2 * math.Pi / DomeGores
		t1 := (g + 1) * 2 * math.Pi / DomeGores
		e0 := vec3{math.Cos(t0), 0, math.Sin(t0)}
		e1 := vec3{math.Cos(t1), 0, math.Sin(t1)}
		dir := e0.Lerp(e1, f) // straight across the gore
		p := c.Add(dir.Scale(a * cp)).Add(vec3{0, bh * sp, 0})
		mid := e0.Add(e1).Norm()
		return p, vec3{cp * mid.X / a, sp / bh, cp * mid.Z / a}.Norm()
	}
	for g := range DomeGores {
		gf := float64(g)
		// One plate per gore and band of three rings.
		m.uvf = func(p vec3) [2]float64 {
			d := p.Sub(c)
			phi := math.Asin(math.Max(-1, math.Min(1, d.Y/bh)))
			t0 := gf * 2 * math.Pi / DomeGores
			th := unwrap(math.Atan2(d.Z, d.X), t0+math.Pi/DomeGores)
			return [2]float64{(th - t0) / (2 * math.Pi / DomeGores), (1 - phi/(math.Pi/2)) * 4}
		}
		for i := range rings {
			for j := range per {
				f0, f1 := float64(j)/per, float64(j+1)/per
				p00, n00 := pt(i, gf, f0)
				p01, n01 := pt(i, gf, f1)
				p10, n10 := pt(i+1, gf, f0)
				p11, n11 := pt(i+1, gf, f1)
				m.smooth(p00, p01, p11, n00, n01, n11)
				if i+1 < rings {
					m.smooth(p00, p11, p10, n00, n11, n10)
				}
			}
		}
	}
	m.uvf = nil
}
