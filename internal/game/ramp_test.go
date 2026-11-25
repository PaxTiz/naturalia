package game

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/planet"
	"testing"
)

// lay sets a block straight ahead, its foot's middle d m away and y m up,
// rising away.
func lay(g *Game, kind string, d, y float64) ecs.ID {
	def, _ := g.Content.Entities.Get(kind)
	up := g.Player.LocalUp()
	at := g.Player.Pos.Add(g.Player.Heading.Scale(d)).Add(up.Scale(y))
	id := g.spawn(kind, at)
	e := g.Ents.Transform.Get(id)
	e.Alt = at.Length() - g.World.Relief.SurfaceRadius(e.Dir)
	ax := g.Player.Heading
	if l, ok := rise(def); ok {
		ax = ax.Scale(l.X)
	}
	e.SetAxes(ax, up)
	return id
}

// Two ramps in a row, the second on a base, throw every rolling vehicle
// higher than one, straight, without a knock on the way.
func TestTwoLevels(t *testing.T) {
	jump := func(kind, second string, levels int) (peak, far float64) {
		g, v := ride(t, kind)
		g.World.Relief.SetPads(nil)
		lay(g, "core:ramp", 40, 0)
		if levels == 2 {
			lay(g, "core:ramp-base", 46, 0)
			lay(g, second, 46, 2)
		}
		start, head := g.Player.Pos, g.Player.Heading
		for range 60 * 14 {
			in := controls{throttle: 1, boost: planet.ArcDist(start.Norm(), g.Player.LocalUp()) > 15}
			g.move(v, in, 1.0/60)
			g.stepPhysics(1.0 / 60)
			g.liftVehicle(1.0 / 60)
			g.updateVehicle(1.0 / 60)
			for _, hit := range g.phys.w.Impacts {
				if hit.A == g.phys.car || hit.B == g.phys.car {
					if d := g.Player.Pos.Sub(start).Dot(head); d < 60 {
						t.Errorf("%s, %s, %d levels: knocked at %.1f m, %.1f m/s", kind, second, levels, d, hit.Speed)
						return
					}
				}
			}
			if alt := g.DriveAlt(); alt > peak {
				peak = alt
			}
		}
		return peak, g.Player.Pos.Sub(start).Length()
	}
	for _, kind := range []string{"core:rover", "core:buggy", "core:hover", "core:harvester"} {
		one, _ := jump(kind, "", 1)
		two, _ := jump(kind, "core:ramp", 2)
		giga, _ := jump(kind, "core:ramp-steep", 2)
		t.Logf("%s: %.1f m, %.1f m on two levels, %.1f m with the steep one", kind, one, two, giga)
		if two < one+1 || giga < two+1 {
			t.Errorf("%s: %.1f m, %.1f m on two levels, %.1f m with the steep one", kind, one, two, giga)
		}
	}
}

// A vehicle rolls up a ramp at a crawl, whatever its slope, and doesn't go
// through its back.
func TestRollUpRamps(t *testing.T) {
	for _, c := range []struct {
		kind string
		top  float64
	}{{"core:slope", 1}, {"core:ramp", 2}, {"core:ramp-steep", 4}} {
		g, v := ride(t, "core:buggy")
		lay(g, c.kind, 12, 0)
		top := 0.0
		for range 60 * 4 {
			step(g, v, controls{throttle: 0.35}, 1)
			if !g.drive.air {
				top = math.Max(top, g.DriveAlt())
			}
		}
		if math.Abs(top-c.top) > 0.25 {
			t.Errorf("%s: rolled %.2f m high, want %.2f", c.kind, top, c.top)
		}
		if c.top < 2 {
			continue
		}
		g, v = ride(t, "core:buggy")
		id := lay(g, c.kind, 20, 0)
		e := g.Ents.Transform.Get(id)
		ax, up, _ := e.Axes()
		e.SetAxes(ax.Scale(-1), up)
		step(g, v, controls{throttle: 1}, 60*3)
		if d := planet.ArcDist(g.Player.LocalUp(), e.Dir); d < 3 {
			t.Errorf("%s: drove through its back, to %.1f m of its middle", c.kind, d)
		}
	}
}

// Built as a player would: a ramp, then on it the next one, which goes on
// from its top; and a base under that one.
func TestBuildTwoLevels(t *testing.T) {
	for _, second := range []string{"core:ramp", "core:ramp-steep"} {
		for _, frac := range []float64{0.1, 0.35} { // where on the first one's upper half
			g := gridGame(t)
			a := put(g, "core:ramp", ahead(g, 7), 7)
			if a == 0 {
				t.Fatalf("no ramp: %q", g.Ghost.Why)
			}
			grid, _ := g.gridOf(a)
			lo, hi, _ := g.boxOf(grid, a)
			pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(a))
			at := pos.Add(ax.Scale(6 * frac)).Add(up.Scale(2 * (0.5 + frac)))
			b := put(g, second, at, 6)
			if b == 0 {
				t.Fatalf("%s doesn't go on: %q", second, g.Ghost.Why)
			}
			blo, _, _ := g.boxOf(grid, b)
			bpos, bax, _, _ := g.Frame(g.Ents.Transform.Get(b))
			if d := bpos.Sub(pos); math.Abs(d.Dot(ax)-6) > 1e-6 || math.Abs(d.Dot(up)-2) > 1e-6 || d.Sub(ax.Scale(6)).Sub(up.Scale(2)).Length() > 1e-6 || bax.Dot(ax) < 1-1e-9 {
				t.Errorf("%s: %v from the first one, %v", second, grid.DirLocal(d), blo.Sub(lo))
			}
			// The base: aimed at the first one again, it stands behind it.
			c := put(g, "core:ramp-base", at, 6)
			if c == 0 {
				t.Fatalf("no base under it: %q", g.Ghost.Why)
			}
			clo, chi, _ := g.boxOf(grid, c)
			if math.Abs(clo.Y-lo.Y) > 1e-6 || math.Abs(chi.Y-hi.Y) > 1e-6 || math.Abs(clo.X-blo.X) > 1e-6 || math.Abs(clo.Z-blo.Z) > 1e-6 {
				t.Errorf("%s: the base at %v, the second ramp at %v, the first at %v", second, clo, blo, lo)
			}
		}
	}
	// The other way round: the base first, then the ramp on it.
	g := gridGame(t)
	a := put(g, "core:ramp", ahead(g, 7), 7)
	pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(a))
	at := pos.Add(ax.Scale(2)).Add(up.Scale(5.0 / 3))
	base := put(g, "core:ramp-base", at, 6)
	b := put(g, "core:ramp", at, 6)
	if base == 0 || b == 0 {
		t.Fatal("not built", base, b)
	}
	bpos, _, _, _ := g.Frame(g.Ents.Transform.Get(b))
	if d := bpos.Sub(pos).Sub(ax.Scale(6)).Sub(up.Scale(2)); d.Length() > 1e-6 {
		t.Errorf("%v off", d)
	}
}

// The kit fits together: what goes on a level is as high as the others.
func TestKitSizes(t *testing.T) {
	c := LoadContent()
	for id, want := range map[string]vec3{"core:ramp": {6, 2, 4}, "core:ramp-base": {6, 2, 4}, "core:ramp-steep": {6, 4, 4}, "core:crate-1": {1, 1, 1}} {
		def, ok := c.Entities.Get(id)
		if !ok {
			t.Errorf("no %s", id)
			continue
		}
		lo, hi := nominal(def)
		blo, bhi := content.Bounds(def.Parts)
		if hi.Sub(lo) != want || !def.Build || bhi.Sub(blo).Sub(want).Length() > 0.06 {
			t.Errorf("%s: %v, parts %v", id, hi.Sub(lo), bhi.Sub(blo))
		}
	}
}

// Built by aiming, then driven over: at any frame rate, on the boost or
// thrown by a pad, the vehicle flies off the second level, unhurt.
func TestDriveBuiltRamps(t *testing.T) {
	for _, kind := range []string{"core:rover", "core:buggy", "core:hover"} {
		for _, fps := range []float64{30, 60, 144} {
			for _, speed := range []float64{12, 30, 60} {
				g := gridGame(t)
				a := put(g, "core:ramp", ahead(g, 7), 7)
				pos, ax, up, az := g.Frame(g.Ents.Transform.Get(a))
				at := pos.Add(ax.Scale(2)).Add(up.Scale(5.0 / 3))
				b, c := put(g, "core:ramp-base", at, 6), put(g, "core:ramp-steep", at, 6)
				if a == 0 || b == 0 || c == 0 {
					t.Fatal("not built", a, b, c)
				}
				g.Ghost = nil
				g.Player.Pos = g.World.Relief.Surface(pos.Sub(ax.Scale(10)).Add(az.Scale(0.6)))
				g.Player.Heading = planet.TangentAt(g.Player.LocalUp(), ax)
				id := g.spawn(kind, g.Player.Pos)
				g.Ents.Transform.Get(id).Yaw = yawOf(g.Player.LocalUp(), g.Player.Heading)
				g.getIn(id)
				g.drive.vel = g.Player.Heading.Scale(speed)
				v, _ := g.Driven()
				dt, peak := 1/fps, 0.0
				for range int(3 * fps) {
					g.move(v, controls{throttle: 1, boost: true}, dt)
					g.stepPhysics(dt)
					g.liftVehicle(dt)
					g.updateVehicle(dt)
					for _, hit := range g.phys.w.Impacts {
						if d := g.Player.Pos.Sub(pos).Dot(ax); (hit.A == g.phys.car || hit.B == g.phys.car) && d < 10 {
							t.Fatalf("%s, %g fps, %g m/s: knocked %.1f m from the first ramp, at %.1f m/s", kind, fps, speed, d, hit.Speed)
						}
					}
					if d, side := g.Player.Pos.Sub(pos).Dot(ax), g.Player.Pos.Sub(pos).Dot(az); d < 9 && math.Abs(side-0.6) > 0.05 {
						t.Fatalf("%s, %g fps, %g m/s: thrown %.2f m aside at %.1f m", kind, fps, speed, side-0.6, d)
					}
					if d := g.Player.Pos.Sub(pos).Dot(ax); d > 9 && d < 9+speed*dt+1 && g.drive.r-pos.Length() < 4.5 {
						t.Fatalf("%s, %g fps, %g m/s: %.1f m up at the end of the second ramp", kind, fps, speed, g.drive.r-pos.Length())
					}
					peak = math.Max(peak, g.drive.r-pos.Length())
				}
				if peak < 9 {
					t.Errorf("%s, %g fps, %g m/s: %.1f m high", kind, fps, speed, peak)
				}
			}
		}
	}
}

// Solid, a ramp is walked up, and stops who comes at its back.
func TestWalkRamp(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	lay(g, "core:ramp", 5, 0)
	if top := walkOn(g, 4, -1); math.Abs(top-2) > 0.15 {
		t.Errorf("walked %.2f m up a ramp of 2 m", top)
	}
}

// Taken askew or near its edge, a ramp on another doesn't knock either.
func TestRampsAskew(t *testing.T) {
	for _, kind := range []string{"core:rover", "core:buggy", "core:hover"} {
		for _, turn := range []float64{-0.15, 0, 0.15} {
			for _, side := range []float64{-1.5, -0.6, 0, 0.6, 1.5} {
				if math.Abs(side-math.Copysign(6*math.Tan(math.Abs(turn)), turn)) > 1.6 {
					continue // it would come at the first one's side
				}
				g, v := ride(t, kind)
				lay(g, "core:ramp", 30, 0)
				lay(g, "core:ramp-base", 36, 0)
				lay(g, "core:ramp-steep", 36, 2)
				start, head := g.Player.Pos, g.Player.Heading
				across := head.Cross(g.Player.LocalUp())
				// To cross the joint of the two at side.
				back := head.Scale(math.Cos(turn)).Add(across.Scale(math.Sin(turn)))
				g.Player.Pos = g.World.Relief.Surface(start.Add(head.Scale(33)).Add(across.Scale(side)).Sub(back.Scale(20)))
				g.Player.Heading = planet.TangentAt(g.Player.LocalUp(), back)
				g.drive.fwd, g.drive.at, g.drive.r = g.Player.Heading, vec3{}, 0
				g.drive.vel = g.Player.Heading.Scale(v.Speed)
				for range 60 * 3 {
					g.move(v, controls{throttle: 1}, 1.0/60)
					g.stepPhysics(1.0 / 60)
					g.liftVehicle(1.0 / 60)
					g.updateVehicle(1.0 / 60)
					d := g.Player.Pos.Sub(start)
					for _, hit := range g.phys.w.Impacts {
						if along := d.Dot(head); (hit.A == g.phys.car || hit.B == g.phys.car) && along < 38 {
							t.Fatalf("%s, %.2f rad, %.1f m aside: knocked at %.1f m, %.1f m/s", kind, turn, side, along, hit.Speed)
						}
					}
				}
			}
		}
	}
}
