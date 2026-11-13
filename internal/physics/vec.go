// Package physics moves bodies on a plane and keeps them from going
// through each other, in the manner of Box2D: rounded convex shapes,
// contacts with up to two points, an impulse solver, a grid to find what is
// close, bodies that fall asleep. Mars is flat at the scale of a bump: the
// game lays the plane tangent to the ground around the player, and gives
// each body the slice of height it fills, so what flies over a wall misses
// it.
package physics

import "math"

// V2 is a point or a vector on the plane.
type V2 struct{ X, Y float64 }

func (a V2) Add(b V2) V2            { return V2{a.X + b.X, a.Y + b.Y} }
func (a V2) Sub(b V2) V2            { return V2{a.X - b.X, a.Y - b.Y} }
func (a V2) Scale(s float64) V2     { return V2{a.X * s, a.Y * s} }
func (a V2) Dot(b V2) float64       { return a.X*b.X + a.Y*b.Y }
func (a V2) Cross(b V2) float64     { return a.X*b.Y - a.Y*b.X }
func (a V2) Len() float64           { return math.Hypot(a.X, a.Y) }
func (a V2) Perp() V2               { return V2{-a.Y, a.X} } // a quarter turn to the left
func (a V2) Lerp(b V2, t float64) V2 { return a.Add(b.Sub(a).Scale(t)) }

func (a V2) Norm() V2 {
	if l := a.Len(); l > 1e-12 {
		return a.Scale(1 / l)
	}
	return V2{}
}

// crossSV is s × a: a turned a quarter to the left, times s (a spin times
// an arm gives the speed of the arm's end).
func crossSV(s float64, a V2) V2 { return V2{-s * a.Y, s * a.X} }

// Rot is a rotation by an angle, kept as its cosine and sine.
type Rot struct{ C, S float64 }

func MakeRot(angle float64) Rot { return Rot{math.Cos(angle), math.Sin(angle)} }

func (r Rot) Apply(v V2) V2  { return V2{r.C*v.X - r.S*v.Y, r.S*v.X + r.C*v.Y} }
func (r Rot) Invert(v V2) V2 { return V2{r.C*v.X + r.S*v.Y, -r.S*v.X + r.C*v.Y} }

// AABB is an axis-aligned box.
type AABB struct{ Lo, Hi V2 }

func (a AABB) Overlaps(b AABB) bool {
	return a.Lo.X <= b.Hi.X && b.Lo.X <= a.Hi.X && a.Lo.Y <= b.Hi.Y && b.Lo.Y <= a.Hi.Y
}

func (a AABB) Union(b AABB) AABB {
	return AABB{V2{math.Min(a.Lo.X, b.Lo.X), math.Min(a.Lo.Y, b.Lo.Y)}, V2{math.Max(a.Hi.X, b.Hi.X), math.Max(a.Hi.Y, b.Hi.Y)}}
}
