package game

import (
	"math"
	"naturalia/internal/ecs"
	"naturalia/internal/planet"
	"testing"
)

// ahead is the ground d m ahead of the player.
func ahead(g *Game, d float64) vec3 {
	return g.World.Relief.Surface(g.Player.Pos.Add(g.Player.Heading.Scale(d)))
}

// roam asks the vehicle id for to as the fleet does, for some seconds,
// and calls each after every frame.
func roam(g *Game, id ecs.ID, to vec3, speed, seconds float64, each func()) {
	for range int(seconds * 60) {
		g.moveToward(id, g.Ents.Transform.Get(id), g.Ents.Roam.Get(id), to.Norm(), speed, 1.0/60)
		g.stepPhysics(1.0 / 60)
		if each != nil {
			each()
		}
	}
}

func TestRobotGoesAround(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	id := g.spawn("core:crawler", ahead(g, 6))
	rover := g.spawn("core:rover", ahead(g, 14))
	to := ahead(g, 24)
	closest := math.Inf(1)
	roam(g, id, to, 2.5, 20, func() {
		closest = math.Min(closest, planet.ArcDist(g.Ents.Transform.Get(id).Dir, g.Ents.Transform.Get(rover).Dir))
	})
	if d := planet.ArcDist(g.Ents.Transform.Get(id).Dir, to.Norm()); d > 0.5 {
		t.Errorf("stopped %.1f m short", d)
	}
	if closest < 0.6+1.5-0.05 {
		t.Errorf("went %.2f m from the rover's middle", closest)
	}
}

func TestBuggyHitsRobot(t *testing.T) {
	g, v := ride(t, "core:buggy")
	at := ahead(g, 20)
	id := g.spawn("core:crawler", at)
	top, low := 0.0, math.Inf(1)
	for range 3 * 60 {
		step(g, v, controls{throttle: 1}, 1)
		s := g.drive.vel.Length()
		top = math.Max(top, s)
		if top > 5 {
			low = math.Min(low, s)
		}
	}
	if moved := planet.ArcDist(at.Norm(), g.Ents.Transform.Get(id).Dir); moved < 1 {
		t.Errorf("the robot moved %.2f m", moved)
	}
	if low > top*0.8 {
		t.Errorf("the buggy went through: %.1f m/s, never under %.1f", top, low)
	}
}

func TestDroneOverVehicles(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	id := g.spawn("core:drone", ahead(g, 6))
	g.spawn("core:titan", ahead(g, 7))
	g.stepPhysics(1.0 / 60)
	if c := g.clearance(id); c < 5.28+overhead {
		t.Errorf("flies %.2f m over a titan", c)
	}
	// The driven buggy goes under them.
	g2, v := ride(t, "core:buggy")
	d := g2.spawn("core:drone", ahead(g2, 15))
	g2.Ents.Transform.Get(d).Alt = 1.4
	at := g2.Ents.Transform.Get(d).Dir
	step(g2, v, controls{throttle: 1}, 3*60)
	if moved := planet.ArcDist(at, g2.Ents.Transform.Get(d).Dir); moved > 0.01 {
		t.Errorf("the buggy pushed a drone %.2f m", moved)
	}
}

func TestDronesSpread(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	a, b := g.spawn("core:drone", ahead(g, 6)), g.spawn("core:drone", ahead(g, 6.1))
	for _, id := range []ecs.ID{a, b} {
		g.Ents.Transform.Get(id).Alt = 1.4
	}
	to := ahead(g, 10)
	for range 3 * 60 {
		for _, id := range []ecs.ID{a, b} {
			g.moveToward(id, g.Ents.Transform.Get(id), g.Ents.Roam.Get(id), to.Norm(), 5, 1.0/60)
		}
		g.stepPhysics(1.0 / 60)
	}
	r := (*g.Ents.Body.Get(a)).Shape.Radius
	if d := planet.ArcDist(g.Ents.Transform.Get(a).Dir, g.Ents.Transform.Get(b).Dir); d < 2*r-0.05 {
		t.Errorf("drones %.2f m apart, %.2f wide", d, 2*r)
	}
}
