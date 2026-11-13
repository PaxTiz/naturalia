package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/planet"
	"testing"
)

// baseGame is fleetGame with the player at the base.
func baseGame(t *testing.T) *Game {
	g := fleetGame(t, LoadContent(), map[string]int{})
	base.Origin = planet.BaseUp.Scale(planet.Radius + g.World.Relief.BaseHeight())
	g.walls = base.Walls()
	return g
}

// walk walks the player toward the world direction to, at walking speed,
// for some seconds.
func walk(g *Game, to vec3, seconds float64) {
	for range int(seconds * 60) {
		up := g.Player.LocalUp()
		g.Player.want = planet.TangentAt(up, to).Scale(walkSpeed)
		g.stepPhysics(1.0 / 60)
		g.Player.fall(1.0/60, g.World.Relief)
	}
}

func TestWalkIntoWalls(t *testing.T) {
	g := baseGame(t)
	crossed, tried := 0, 0
	for i, s := range g.walls {
		if i%7 != 0 {
			continue
		}
		a, b, _ := s.Ends()
		mid := a.Lerp(b, 0.5)
		n := vec3{-(b.Z - a.Z), 0, b.X - a.X}.Norm()
		tried++
		for _, side := range []float64{-1, 1} {
			start := mid.Add(n.Scale(side * 1.5))
			g.Player.Pos = g.World.Relief.Surface(base.ToWorld(start.Add(vec3{0, 1, 0})))
			g.Player.vel = vec3{}
			walk(g, n.Scale(-side), 1.5)
			at := base.ToLocal(g.Player.Pos)
			if d := at.Sub(a).Dot(n) * side; d < 0.35+base.WallThick/2-0.03 {
				crossed++
				t.Errorf("wall %d (%v–%v), side %v: ended %.2f m from it, at %v", i, a, b, side, d, at)
			}
		}
	}
	if crossed > 0 || tried < 10 {
		t.Errorf("%d walls walked through, of %d", crossed, tried)
	}
}

func TestWalkAlongWall(t *testing.T) {
	g := baseGame(t)
	// A long hall wall: walking into it at an angle slides along it.
	var long base.Segment
	for _, s := range g.walls {
		if s.Half() > long.Half() {
			long = s
		}
	}
	a, b, _ := long.Ends()
	along := b.Sub(a).Norm()
	n := vec3{-along.Z, 0, along.X}
	start := a.Lerp(b, 0.3).Add(n.Scale(1))
	g.Player.Pos = g.World.Relief.Surface(base.ToWorld(start.Add(vec3{0, 1, 0})))
	walk(g, n.Scale(-1).Add(along).Norm(), 1)
	moved := base.ToLocal(g.Player.Pos).Sub(start).Dot(along)
	if moved < walkSpeed*0.6 {
		t.Errorf("slid %.2f m along a %.0f m wall", moved, 2*long.Half())
	}
}

func TestWalkIntoEntities(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	at := g.Player.Pos.Add(g.Player.Heading.Scale(5))
	id := g.spawn("core:rover", g.World.Relief.Surface(at))
	walk(g, g.Player.Heading, 3)
	def, _ := g.Content.Entities.Get("core:rover")
	pos := g.World.Relief.Surface(g.Ents.Transform.Get(id).Dir)
	if d := planet.ArcDist(pos.Norm(), g.Player.LocalUp()); d < def.Radius+0.35-0.03 || d > def.Radius+0.35+0.1 {
		t.Errorf("stopped %.2f m from the rover's middle, radius %.2f", d, def.Radius)
	}
	// Sold, it no longer stops anyone.
	g.Ents.Kill(id)
	walk(g, g.Player.Heading, 2)
	if planet.ArcDist(pos.Norm(), g.Player.LocalUp()) < 3 {
		t.Error("stopped by a vehicle gone")
	}
	for _, b := range g.phys.w.Bodies() {
		if b.ID != 0 {
			t.Errorf("body of entity %d left", b.ID)
		}
	}
}

func TestPlaneRoundTrip(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	g.stepPhysics(0)
	p := g.phys
	for _, off := range []vec3{{0, 0, 0}, {30, 0, -20}, {-100, 0, 90}} {
		at := g.World.Relief.Surface(g.Player.Pos.Add(off))
		if d := p.dir(p.plane(at)).Sub(at.Norm()).Length() * planet.Radius; d > 1e-6 {
			t.Errorf("%v: %.2g m off", off, d)
		}
		v := planet.TangentAt(at.Norm(), vec3{1, 0.3, 0.2}).Scale(4)
		if w := p.vecWorld(at, p.vecPlane(at, v)); w.Sub(v).Length() > 1e-3 || math.IsNaN(w.X) {
			t.Errorf("%v: speed %v back as %v", off, v, w)
		}
	}
}

func BenchmarkWalkAtBase(b *testing.B) {
	g := fleetGame(&testing.T{}, LoadContent(), map[string]int{"core:drone": 6, "core:rover": 1, "core:harvester": 1})
	base.Origin = planet.BaseUp.Scale(planet.Radius + g.World.Relief.BaseHeight())
	g.walls = base.Walls()
	g.Player.Pos = g.World.Relief.Surface(base.ToWorld(vec3{5, 1, 5}))
	g.syncFleet()
	b.ResetTimer()
	for i := range b.N {
		a := float64(i) * 0.05
		g.Player.want = planet.TangentAt(g.Player.LocalUp(), vec3{math.Cos(a), 0, math.Sin(a)}).Scale(walkSpeed)
		g.stepPhysics(1.0 / 60)
		g.Player.fall(1.0/60, g.World.Relief)
	}
}

// clearRun finds a long wall of the base and a spot 25 m out from it,
// with nothing in between: the wall's end a, its outward normal n, and the
// spot (local).
func clearRun(t *testing.T, g *Game) (a, n, start vec3) {
	crosses := func(p, q, a, b vec3) bool {
		side := func(o, u, v vec3) float64 { return (u.X-o.X)*(v.Z-o.Z) - (u.Z-o.Z)*(v.X-o.X) }
		return side(p, q, a)*side(p, q, b) < 0 && side(a, b, p)*side(a, b, q) < 0
	}
	for i, s := range g.walls {
		a, b, _ := s.Ends()
		if s.Half() < 5 {
			continue
		}
		along := b.Sub(a).Norm()
		mid := a.Lerp(b, 0.5)
		for _, n := range []vec3{{-along.Z, 0, along.X}, {along.Z, 0, -along.X}} {
			start := mid.Add(n.Scale(25))
			if base.Indoors(vec3{start.X, 1, start.Z}) || base.Covered(start) {
				continue
			}
			clear := true
			for j, o := range g.walls {
				oa, ob, _ := o.Ends()
				// A lane 5 m wide.
				for _, off := range []float64{-2.5, 0, 2.5} {
					p, q := start.Add(along.Scale(off)), mid.Add(n.Scale(0.5)).Add(along.Scale(off))
					if j != i && crosses(p, q, oa, ob) {
						clear = false
					}
				}
			}
			for _, o := range base.Obstacles {
				if d := o.At.Sub(mid); math.Abs(d.Dot(along)) < 4 && d.Dot(n) > 0 && d.Dot(n) < 26 {
					clear = false
				}
			}
			if clear {
				return a, n, start
			}
		}
	}
	t.Fatal("no wall to drive into")
	return
}

func TestCrashIntoWall(t *testing.T) {
	g := baseGame(t)
	g.Mode = ModePlay
	a, n, start := clearRun(t, g)
	g.Player.Pos = g.World.Relief.Surface(base.ToWorld(start.Add(vec3{0, 1, 0})))
	// Not quite square to the wall: it glances off turning.
	along := vec3{n.Z, 0, -n.X}
	g.Player.Heading = planet.TangentAt(g.Player.LocalUp(), n.Scale(-1).Add(along.Scale(0.25)).Norm())
	g.Board("core:rover")
	var hits []float64
	g.Content.On(content.EvImpact, func(_ content.Game, e *content.Event) { hits = append(hits, e.Amount) })
	v, _ := g.Driven()
	heading := g.drive.fwd
	lo, _ := v.Footprint()
	closest, after := math.Inf(1), -1
	for i := range 3 * 60 {
		step(g, v, controls{throttle: 1, boost: true}, 1)
		closest = math.Min(closest, base.ToLocal(g.Player.Pos).Sub(a).Dot(n))
		if len(hits) > 0 && after < 0 {
			after = i
		}
		if after >= 0 && i == after+3 {
			// Thrown back off the wall.
			if into := g.drive.vel.Dot(n.Scale(-1)); into > 0 {
				t.Errorf("still going into the wall at %.1f m/s after the knock", into)
			}
			if turned := math.Acos(math.Min(1, heading.Dot(g.drive.fwd))); turned < 0.05 {
				t.Errorf("turned by %.2f rad only", turned)
			}
		}
	}
	if closest < -lo.X*0.8 {
		t.Errorf("the rover's middle came %.2f m from the wall, through it", closest)
	}
	if len(hits) == 0 || hits[0] < 10 {
		t.Errorf("impacts %v", hits)
	}
}
