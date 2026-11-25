package planet

import (
	"math"
	"testing"
)

func TestGrid(t *testing.T) {
	up := vec3{0.3, 0.8, -0.2}.Norm()
	g := Grid{At: up.Scale(Radius), X: TangentAt(up, vec3{1, 0, 0}), Up: up}
	p := vec3{3, 1, -2}
	if d := g.Local(g.World(p)).Sub(p).Length(); d > 1e-9 {
		t.Errorf("there and back, %g m off", d)
	}
	for i, a := range g.Axes() {
		if f := g.Front(a.Add(g.X.Scale(0.1))); i != 1 && f.Sub(a).Length() > 1e-9 {
			t.Errorf("front %d: %v", i, f)
		}
	}
	for _, c := range []struct{ lo, hi, step, want float64 }{
		{0.1, 4.1, 1, -0.1},    // a slab: both ends
		{0.7, 0.9, 1, 0.1},     // a wall's far face is nearer
		{-0.05, 0.15, 1, 0.05}, // its near face
	} {
		if got := SnapSpan(c.lo, c.hi, c.step); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("SnapSpan(%v, %v, %v) = %v, want %v", c.lo, c.hi, c.step, got, c.want)
		}
	}
}
