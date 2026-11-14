package physics

import (
	"math"
	"testing"
)

func place(s Shape, pos V2, angle float64) *poly {
	var p poly
	s.world(pos, MakeRot(angle), &p)
	return &p
}

func near(a, b, eps float64) bool { return math.Abs(a-b) < eps }

func TestCollideShapes(t *testing.T) {
	for _, c := range []struct {
		name   string
		a, b   *poly
		n      int
		normal V2
		sep    float64
	}{
		{"circles", place(Circle(1), V2{}, 0), place(Circle(0.5), V2{1.4, 0}, 0), 1, V2{1, 0}, -0.1},
		{"circles apart", place(Circle(1), V2{}, 0), place(Circle(0.5), V2{2, 0}, 0), 0, V2{}, 0},
		{"box on box, face to face", place(Box(1, 1, 0), V2{}, 0), place(Box(0.5, 0.5, 0), V2{1.4, 0.2}, 0), 2, V2{1, 0}, -0.1},
		{"box corner first", place(Box(1, 1, 0), V2{}, 0), place(Box(0.5, 0.5, 0), V2{1.6, 0}, math.Pi/4), 1, V2{1, 0}, -1 + 1.6 - 0.5*math.Sqrt2},
		{"circle on a face", place(Box(1, 1, 0), V2{}, 0), place(Circle(0.5), V2{0, 1.3}, 0), 1, V2{0, 1}, -0.2},
		{"circle round a corner", place(Box(1, 1, 0), V2{}, 0), place(Circle(0.5), V2{1.3, 1.3}, 0), 1, V2{1, 1}.Norm(), 0.3*math.Sqrt2 - 0.5},
		{"capsule along a wall", place(Capsule(V2{-5, 0}, V2{5, 0}, 0.1), V2{}, 0), place(Capsule(V2{-1, 0}, V2{1, 0}, 0.4), V2{0, 0.45}, 0), 2, V2{0, 1}, -0.05},
		{"capsule end on a wall", place(Capsule(V2{-5, 0}, V2{5, 0}, 0.1), V2{}, 0), place(Capsule(V2{0, 0}, V2{0, 2}, 0.4), V2{0, 0.45}, 0), 1, V2{0, 1}, -0.05},
		{"rounded boxes, corners", place(Box(1, 1, 0.2), V2{}, 0), place(Box(1, 1, 0.2), V2{2.2, 2.2}, 0), 1, V2{1, 1}.Norm(), 0.2*math.Sqrt2 - 0.4},
	} {
		m := collide(c.a, c.b)
		if m.N != c.n {
			t.Errorf("%s: %d points, want %d", c.name, m.N, c.n)
			continue
		}
		if c.n == 0 {
			continue
		}
		if m.Normal.Sub(c.normal).Len() > 1e-6 || !near(m.MinSep(), c.sep, 1e-6) {
			t.Errorf("%s: normal %v sep %.4f, want %v %.4f", c.name, m.Normal, m.MinSep(), c.normal, c.sep)
		}
		// Swapped, the normal turns around.
		if r := collide(c.b, c.a); r.N != m.N || r.Normal.Add(m.Normal).Len() > 1e-6 || !near(r.MinSep(), m.MinSep(), 1e-6) {
			t.Errorf("%s swapped: %d points, normal %v, sep %.4f", c.name, r.N, r.Normal, r.MinSep())
		}
	}
}

// wall is a static segment from a to b, 0.2 m thick, 3 m high.
func wall(w *World, a, b V2) *Body {
	return w.Add(&Body{Kind: Static, Shape: Capsule(a, b, 0.1), Top: 3, Layer: 1, Mask: ^uint32(0), Friction: 0.5})
}

func ball(w *World, pos, vel V2, r float64) *Body {
	return w.Add(&Body{Kind: Dynamic, Shape: Circle(r), Pos: pos, Vel: vel, Mass: 80, Top: 1.8, Layer: 1, Mask: ^uint32(0), FixedRotation: true})
}

func run(w *World, seconds float64, each func()) {
	for range int(seconds * 60) {
		if each != nil {
			each()
		}
		w.Step(1.0 / 60)
	}
}

func TestStopsAtWall(t *testing.T) {
	w := NewWorld()
	wall(w, V2{5, -10}, V2{5, 10})
	b := ball(w, V2{}, V2{}, 0.35)
	// Walking into it, the speed set again every frame.
	run(w, 3, func() { b.Vel = V2{4, 0} })
	if x := b.Pos.X; x > 5-0.1-0.35+linearSlop*2 || x < 5-0.1-0.35-0.05 {
		t.Errorf("stopped at x = %.3f", x)
	}
	// At an angle, it slides along (no friction on the walker).
	b.Friction = 0
	y := b.Pos.Y
	run(w, 1, func() { b.Vel = V2{4, 3} })
	if b.Pos.Y-y < 2.5 || b.Pos.X > 4.56 {
		t.Errorf("slid to %v", b.Pos)
	}
}

func TestCornerHolds(t *testing.T) {
	w := NewWorld()
	wall(w, V2{0, 0}, V2{5, 0})
	wall(w, V2{5, 0}, V2{5, 5})
	b := ball(w, V2{2, 2}, V2{}, 0.35)
	var prev V2
	for i := range 240 {
		b.Vel = V2{3, -3}
		w.Step(1.0 / 60)
		if i > 120 && b.Pos.Sub(prev).Len() > 1e-3 {
			t.Fatalf("still moving in the corner: %v → %v", prev, b.Pos)
		}
		prev = b.Pos
	}
	if b.Pos.X > 5-0.45+0.01 || b.Pos.Y < 0.45-0.01 {
		t.Errorf("in the corner at %v", b.Pos)
	}
}

func TestNoTunneling(t *testing.T) {
	w := NewWorld()
	wall(w, V2{3, -5}, V2{3, 5})
	b := ball(w, V2{}, V2{120, 0}, 0.2) // 2 m a frame
	w.Step(1.0 / 30)                    // a slow frame too
	run(w, 0.5, nil)
	if b.Pos.X > 3 {
		t.Errorf("went through the wall: x = %.2f", b.Pos.X)
	}
}

func TestBounce(t *testing.T) {
	w := NewWorld()
	wall(w, V2{2, -5}, V2{2, 5})
	b := ball(w, V2{}, V2{6, 0}, 0.5)
	b.Restitution = 0.5
	run(w, 1, nil)
	if !near(b.Vel.X, -3, 0.3) {
		t.Errorf("bounced back at %.2f m/s", b.Vel.X)
	}
	if len(w.Impacts) != 0 {
		t.Error("impacts kept from an earlier step")
	}
	c := ball(w, V2{0, 3}, V2{6, 0}, 0.5)
	var hit []Impact
	run(w, 1, func() { hit = append(hit, w.Impacts...) })
	if len(hit) != 1 || hit[0].A != c && hit[0].B != c || !near(hit[0].Speed, 6, 0.2) {
		t.Errorf("impacts %+v", hit)
	}
}

func TestOffCenterSpins(t *testing.T) {
	w := NewWorld()
	crate := w.Add(&Body{Kind: Dynamic, Shape: Box(0.5, 0.5, 0), Pos: V2{3, 1.2}, Mass: 30, Top: 1, Layer: 1, Mask: ^uint32(0), Damping: 1, SpinDamping: 1})
	car := w.Add(&Body{Kind: Dynamic, Shape: Box(2, 1, 0.05), Mass: 1500, Vel: V2{8, 0}, Top: 1.5, Layer: 1, Mask: ^uint32(0)})
	run(w, 0.5, nil)
	if crate.Pos.X < 4 || math.Abs(crate.Spin) < 0.1 && math.Abs(crate.Angle) < 0.05 {
		t.Errorf("crate at %v, angle %.2f, spin %.2f", crate.Pos, crate.Angle, crate.Spin)
	}
	if car.Vel.X > 8 || car.Vel.X < 6 {
		t.Errorf("the car goes on at %.2f m/s", car.Vel.X)
	}
}

func TestSleepAndWake(t *testing.T) {
	w := NewWorld()
	crate := w.Add(&Body{Kind: Dynamic, Shape: Box(0.5, 0.5, 0), Pos: V2{3, 0}, Vel: V2{1, 0}, Mass: 30, Top: 1, Layer: 1, Mask: ^uint32(0), Damping: 4})
	run(w, 2, nil)
	if !crate.Asleep() {
		t.Fatalf("still awake at %.3f m/s", crate.Vel.Len())
	}
	at := crate.Pos
	b := ball(w, V2{0, 0}, V2{}, 0.35)
	run(w, 2, func() { b.Vel = V2{2, 0} })
	if crate.Pos.X < at.X+1 {
		t.Errorf("pushed to %v from %v", crate.Pos, at)
	}
}

func TestHeightsAndLayers(t *testing.T) {
	w := NewWorld()
	wall(w, V2{2, -5}, V2{2, 5})
	flier := ball(w, V2{}, V2{4, 0}, 0.5)
	flier.Bottom, flier.Top = 3.5, 5
	ghost := ball(w, V2{0, 2}, V2{4, 0}, 0.5)
	ghost.Mask = 0
	run(w, 1, nil)
	if flier.Pos.X < 3.9 || ghost.Pos.X < 3.9 {
		t.Errorf("stopped: flier at %v, ghost at %v", flier.Pos, ghost.Pos)
	}
}

func TestSensor(t *testing.T) {
	w := NewWorld()
	pad := w.Add(&Body{Kind: Static, Sensor: true, Shape: Box(1, 1, 0), Pos: V2{3, 0}, Top: 0.2, Layer: 2, Mask: 1})
	b := ball(w, V2{}, V2{4, 0}, 0.3)
	var ins, outs int
	run(w, 2, func() {
		b.Vel = V2{4, 0}
		for _, s := range w.Senses {
			if s.Sensor != pad || s.Body != b {
				t.Fatalf("sense %+v", s)
			}
			if s.In {
				ins++
			} else {
				outs++
			}
		}
	})
	if ins != 1 || outs != 1 || b.Pos.X < 7 {
		t.Errorf("%d in, %d out, at %v", ins, outs, b.Pos)
	}
}

func TestRepeats(t *testing.T) {
	pile := func() *World {
		w := NewWorld()
		wall(w, V2{6, -10}, V2{6, 10})
		for i := range 20 {
			w.Add(&Body{Kind: Dynamic, Shape: Box(0.4, 0.4, 0.02), Pos: V2{3 + float64(i%4)*0.85, float64(i/4)*0.85 - 2}, Mass: 20, Top: 1, Layer: 1, Mask: ^uint32(0), Friction: 0.4, Damping: 2, SpinDamping: 2})
		}
		car := w.Add(&Body{Kind: Dynamic, Shape: Box(1.5, 1, 0.05), Pos: V2{-3, 0}, Mass: 1200, Top: 1.5, Layer: 1, Mask: ^uint32(0)})
		run(w, 3, func() { car.Vel = V2{5, 0.3} })
		return w
	}
	a, b := pile(), pile()
	for i, x := range a.Bodies() {
		if y := b.Bodies()[i]; x.Pos != y.Pos {
			t.Fatalf("body %d at %v and %v", i, x.Pos, y.Pos)
		}
		if x.Kind == Dynamic && x.Pos.X > 6-0.4 {
			t.Errorf("a crate went through the wall: %v", x.Pos)
		}
	}
}

func TestQueries(t *testing.T) {
	w := NewWorld()
	wl := wall(w, V2{0, -5}, V2{0, 5})
	if !w.Overlap(Circle(0.5), V2{0.3, 0}, 0, 0, 2, 1, nil) || w.Overlap(Circle(0.5), V2{1, 0}, 0, 0, 2, 1, nil) {
		t.Error("overlap")
	}
	if w.Overlap(Circle(0.5), V2{0.3, 0}, 0, 0, 2, 1, wl) || w.Overlap(Circle(0.5), V2{0.3, 0}, 0, 3.5, 4, 1, nil) {
		t.Error("overlap skips")
	}
	w.Teleport(wl, V2{10, 0}, 0)
	if w.Overlap(Circle(0.5), V2{0.3, 0}, 0, 0, 2, 1, nil) || !w.Overlap(Circle(0.5), V2{10.3, 0}, 0, 0, 2, 1, nil) {
		t.Error("teleported wall")
	}
	w.Remove(wl)
	if w.Overlap(Circle(0.5), V2{10.3, 0}, 0, 0, 2, 1, nil) || len(w.Bodies()) != 0 {
		t.Error("removed wall")
	}
}

func BenchmarkCrowd(b *testing.B) {
	w := NewWorld()
	for i := range 300 {
		a := float64(i) * 0.7
		wall(w, V2{math.Cos(a) * 60, math.Sin(a) * 60}, V2{math.Cos(a+0.6) * 60, math.Sin(a+0.6) * 60})
	}
	var movers []*Body
	for i := range 200 {
		movers = append(movers, w.Add(&Body{Kind: Dynamic, Shape: Box(0.4, 0.4, 0.02), Pos: V2{float64(i%20)*2 - 20, float64(i/20)*2 - 10},
			Mass: 20, Top: 1, Layer: 1, Mask: ^uint32(0), Friction: 0.4}))
	}
	b.ResetTimer()
	for range b.N {
		for i, m := range movers {
			m.Vel = V2{math.Cos(float64(i)), math.Sin(float64(i))}.Scale(3)
		}
		w.Step(1.0 / 60)
	}
}

func TestTouching(t *testing.T) {
	w := NewWorld()
	wl := wall(w, V2{1, -5}, V2{1, 5})
	b := ball(w, V2{}, V2{}, 0.35)
	run(w, 1, func() { b.Vel = V2{3, 0} })
	var got []*Body
	w.Touching(b, func(o *Body, n V2) {
		got = append(got, o)
		if n.Sub(V2{1, 0}).Len() > 1e-6 {
			t.Errorf("normal %v", n)
		}
	})
	if len(got) != 1 || got[0] != wl {
		t.Errorf("touching %v", got)
	}
}

func TestHull(t *testing.T) {
	// A unit square's corners, shuffled, with its middle and doubles.
	s := Hull([]V2{{1, 1}, {0, 0}, {0.5, 0.5}, {1, 0}, {0, 1}, {1, 1}, {0, 0}}, 0)
	if len(s.V) != 4 {
		t.Fatalf("%d corners: %v", len(s.V), s.V)
	}
	for i := range s.V {
		a, b, c := s.V[i], s.V[(i+1)%4], s.V[(i+2)%4]
		if b.Sub(a).Cross(c.Sub(b)) <= 0 {
			t.Errorf("not counter-clockwise: %v", s.V)
		}
	}
	// Twelve points on a circle: eight kept.
	var ring []V2
	for i := range 12 {
		a := float64(i) * math.Pi / 6
		ring = append(ring, V2{math.Cos(a), math.Sin(a)})
	}
	if s := Hull(ring, 0); len(s.V) != 8 {
		t.Errorf("%d corners from 12", len(s.V))
	}
	// A segment seen edge on: a capsule.
	if s := Hull([]V2{{0, 0}, {2, 0}, {1, 0}}, 0.1); len(s.V) != 2 {
		t.Errorf("segment: %v", s.V)
	}
}
