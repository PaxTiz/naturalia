package mesh

import (
	"cmp"
	"math"
	"slices"
)

// Face is a flat face of a convex hull: its corners in order around it,
// its normal out, and the plane n·x = D it lies in.
type Face struct {
	N   vec3
	D   float64
	Pts []vec3
}

// HullFaces are the faces of the convex hull of a few points (a dozen or
// so: every plane through three of them is tried); none if they are flat.
func HullFaces(pts []vec3) []Face {
	const eps = 1e-6
	var out []Face
	for i := range pts {
		for j := i + 1; j < len(pts); j++ {
			for k := j + 1; k < len(pts); k++ {
				n := pts[j].Sub(pts[i]).Cross(pts[k].Sub(pts[i]))
				if n.Length() < eps {
					continue
				}
				n = n.Norm()
				d := n.Dot(pts[i])
				under, over := 0.0, 0.0
				for _, p := range pts {
					h := n.Dot(p) - d
					under, over = math.Min(under, h), math.Max(over, h)
				}
				if over > eps && under < -eps || over < eps && under > -eps {
					continue // through the middle, or all flat
				}
				if over > eps {
					n, d = n.Scale(-1), -d
				}
				if slices.ContainsFunc(out, func(f Face) bool { return f.N.Dot(n) > 1-eps && math.Abs(f.D-d) < eps*10 }) {
					continue
				}
				out = append(out, Face{n, d, around(pts, n, d)})
			}
		}
	}
	return out
}

// around are the points on a plane at the corners of their outline, in
// order around the normal n.
func around(pts []vec3, n vec3, d float64) []vec3 {
	const eps = 1e-5
	var on []vec3
	var mid vec3
	for _, p := range pts {
		if math.Abs(n.Dot(p)-d) < eps && !slices.ContainsFunc(on, func(q vec3) bool { return q.Sub(p).Length() < eps }) {
			on = append(on, p)
			mid = mid.Add(p)
		}
	}
	mid = mid.Scale(1 / float64(len(on)))
	u := on[0].Sub(mid).Norm()
	w := n.Cross(u)
	angle := func(p vec3) float64 { return math.Atan2(p.Sub(mid).Dot(w), p.Sub(mid).Dot(u)) }
	slices.SortFunc(on, func(a, b vec3) int { return cmp.Compare(angle(a), angle(b)) })
	// Points along an edge, or inside the outline, aren't corners.
	for again := true; again && len(on) > 3; {
		again = false
		for i := range on {
			a, b, c := on[(i+len(on)-1)%len(on)], on[i], on[(i+1)%len(on)]
			if b.Sub(a).Cross(c.Sub(b)).Dot(n) < eps {
				on = slices.Delete(on, i, i+1)
				again = true
				break
			}
		}
	}
	return on
}

// Hull adds the convex hull of points: a triangle of floor, a gable, the
// corner of a roof.
func (m *Builder) Hull(pts []vec3) {
	m.begin()
	var mid vec3
	for _, p := range pts {
		mid = mid.Add(p)
	}
	mid = mid.Scale(1 / float64(len(pts)))
	for _, f := range HullFaces(pts) {
		// Plates about 1.2 m wide and 1 m tall, as on a box's faces, down the
		// face's steepest way.
		v := vec3{0, 1, 0}.Sub(f.N.Scale(f.N.Y))
		if v.Length() < 0.05 {
			v = vec3{0, 0, 1}
		}
		v = v.Norm()
		u := v.Cross(f.N)
		u0, u1, v0, v1 := math.Inf(1), math.Inf(-1), math.Inf(1), math.Inf(-1)
		for _, p := range f.Pts {
			u0, u1 = math.Min(u0, p.Dot(u)), math.Max(u1, p.Dot(u))
			v0, v1 = math.Min(v0, p.Dot(v)), math.Max(v1, p.Dot(v))
		}
		nu, nv := math.Max(1, math.Round((u1-u0)/1.2)), math.Max(1, math.Round((v1-v0)/1.0))
		m.uvf = func(p vec3) [2]float64 {
			return [2]float64{(p.Dot(u) - u0) / (u1 - u0) * nu, (v1 - p.Dot(v)) / (v1 - v0) * nv}
		}
		for i := 1; i+1 < len(f.Pts); i++ {
			m.tri(f.Pts[0], f.Pts[i], f.Pts[i+1], mid)
		}
	}
	m.uvf = nil
}
