package game

import (
	"math"
	"math/rand/v2"
	"testing"
)

func near(a, b vec3) bool { return a.Sub(b).Length() < 1e-9 }

func TestAxesRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for range 500 {
		dir := vec3{rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()}.Norm()
		e := Transform{Dir: dir, Yaw: rng.Float64() * 7, Pitch: rng.Float64()*3 - 1.5, Roll: rng.Float64()*7 - 3.5}
		ax, up, az := e.Axes()
		f := Transform{Dir: dir}
		f.SetAxes(ax, up)
		bx, bu, bz := f.Axes()
		if !near(ax, bx) || !near(up, bu) || !near(az, bz) {
			t.Fatalf("%+v came back as %+v", e, f)
		}
	}
	// Nose straight up: the roll goes into the yaw.
	e := Transform{Dir: vec3{0, 1, 0}, Yaw: 0.4, Pitch: math.Pi / 2, Roll: 0.3}
	ax, up, _ := e.Axes()
	f := Transform{Dir: e.Dir}
	f.SetAxes(ax, up)
	if bx, bu, _ := f.Axes(); !near(ax, bx) || !near(up, bu) {
		t.Errorf("straight up: %+v came back as %+v", e, f)
	}
}

func TestTurn(t *testing.T) {
	e := Transform{Dir: vec3{0, 0, 1}, Yaw: 0.2}
	ax, up, az := e.Axes()
	// Around its up: only the yaw changes (which way depends on the
	// ground's tangent basis).
	e.Turn(up, math.Pi/12)
	if math.Abs(math.Abs(e.Yaw-0.2)-math.Pi/12) > 1e-9 || math.Abs(e.Pitch)+math.Abs(e.Roll) > 1e-9 {
		t.Errorf("turned around up: %+v", e)
	}
	// Four quarter turns around any axis: back where it was.
	e = Transform{Dir: vec3{0, 0, 1}, Yaw: 0.2}
	axis := vec3{1, 2, 3}.Norm()
	for range 4 {
		e.Turn(axis, math.Pi/2)
	}
	if bx, bu, bz := e.Axes(); !near(ax, bx) || !near(up, bu) || !near(az, bz) {
		t.Errorf("four quarter turns: %+v", e)
	}
	// A quarter turn around its side stands it on its nose... or its back.
	e = Transform{Dir: vec3{0, 0, 1}}
	_, _, az = e.Axes()
	e.Turn(az, math.Pi/2)
	if bx, _, _ := e.Axes(); !near(bx, e.Dir) && !near(bx, e.Dir.Scale(-1)) {
		t.Errorf("tipped over: front %v", bx)
	}
}
