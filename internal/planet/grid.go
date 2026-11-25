package planet

import "math"

// Grid is a flat metric grid on a part of Mars: what is built lines up on
// it. Squares can't cover a sphere, so each construction has its own: its
// origin At, and its axes X, Up and across (Axes). Its points are written
// in its own frame: x along X, y up, z across.
type Grid struct {
	At vec3 `json:"at"`
	X  vec3 `json:"x"`
	Up vec3 `json:"up"`
}

const (
	// GridRise is the height between the levels of any grid, in m.
	GridRise = 0.25
	// GridSpan is how far from its origin a grid is used, in m: there, Mars
	// has sunk 1 m under its plane.
	GridSpan = 80.0
)

// Axes are the grid's x, y and z axes.
func (g Grid) Axes() [3]vec3 { return [3]vec3{g.X, g.Up, g.X.Cross(g.Up)} }

// Local is a point of the world in the grid's frame.
func (g Grid) Local(p vec3) vec3 { return g.DirLocal(p.Sub(g.At)) }

// DirLocal is a direction of the world in the grid's frame.
func (g Grid) DirLocal(v vec3) vec3 { return vec3{v.Dot(g.X), v.Dot(g.Up), v.Dot(g.X.Cross(g.Up))} }

// World is a point of the grid's frame in the world.
func (g Grid) World(p vec3) vec3 { return g.At.Add(g.Dir(p)) }

// Dir is a direction of the grid's frame in the world.
func (g Grid) Dir(v vec3) vec3 {
	return g.X.Scale(v.X).Add(g.Up.Scale(v.Y)).Add(g.X.Cross(g.Up).Scale(v.Z))
}

// Front is the grid's axis, level, that points the nearest to v.
func (g Grid) Front(v vec3) vec3 {
	l := g.DirLocal(v)
	if math.Abs(l.X) >= math.Abs(l.Z) {
		return g.X.Scale(math.Copysign(1, l.X))
	}
	return g.X.Cross(g.Up).Scale(math.Copysign(1, l.Z))
}

// Same tells whether two grids have the same lines.
func (g Grid) Same(o Grid) bool {
	const e = 1e-6
	return g.At.Sub(o.At).Length() < e && g.X.Sub(o.X).Length() < e && g.Up.Sub(o.Up).Length() < e
}

// Reaches tells whether the grid is used at p: near enough to its origin.
func (g Grid) Reaches(p vec3) bool { return g.At.Sub(p).Length() < GridSpan }

// Snap is the multiple of step nearest to v.
func Snap(v, step float64) float64 { return math.Round(v/step) * step }

// SnapSpan is how far to move the span lo..hi for one of its ends to fall
// on a multiple of step, the least.
func SnapSpan(lo, hi, step float64) float64 {
	a, b := Snap(lo, step)-lo, Snap(hi, step)-hi
	if math.Abs(b) < math.Abs(a)-1e-9 {
		return b
	}
	return a
}

// Plot is the ground under the rectangle of the grid lo..hi (x and z), at
// height y.
func (g Grid) Plot(lo, hi vec3, y float64) Plot {
	return Plot{
		At: g.World(vec3{(lo.X + hi.X) / 2, y, (lo.Z + hi.Z) / 2}),
		X:  g.X, Up: g.Up,
		Half: [2]float64{(hi.X - lo.X) / 2, (hi.Z - lo.Z) / 2},
	}
}
