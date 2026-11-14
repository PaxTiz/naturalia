package game

import (
	"math"
	"naturalia/internal/ecs"
	"naturalia/internal/planet"
	"testing"
)

func TestGhost(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	g.Player.Pitch = -0.5 // looking at the ground ahead
	g.GiveItem("core:crate", 2)
	g.stepPhysics(1.0 / 60)
	g.Ghost = &Ghost{Kind: "core:crate", item: "core:crate", rel: [2]vec3{{1, 0, 0}, {0, 1, 0}}}
	g.poseGhost()
	gh := g.Ghost
	if math.Abs(gh.Pose.Alt) > 0.05 || !gh.OK {
		t.Errorf("on the ground: %.2f m high, fits %v", gh.Pose.Alt, gh.OK)
	}
	// Tipped 45° on an edge: it rests on it, lifted.
	gh.turn(vec3{1, 0, 0}, math.Pi/4)
	g.poseGhost()
	if want := 0.4 * math.Sqrt2 / 2; math.Abs(gh.Pose.Alt-want) > 0.05 || math.Abs(math.Abs(gh.Pose.Roll)-math.Pi/4) > 1e-6 {
		t.Errorf("on its edge: %.2f m high (want %.2f), roll %.2f", gh.Pose.Alt, want, gh.Pose.Roll)
	}
	// Set down, it falls on a face.
	g.placeGhost()
	id := g.entityOf("core:crate")
	if g.State.Items["core:crate"] != 1 || !g.Ents.Air.Has(id) {
		t.Fatalf("placed: %d left, falling %v", g.State.Items["core:crate"], g.Ents.Air.Has(id))
	}
	for range 120 {
		g.stepPhysics(1.0 / 60)
	}
	if e := g.Ents.Transform.Get(id); g.Ents.Air.Has(id) || e.Alt != 0 || math.Mod(math.Abs(e.Roll), math.Pi/2) > 1e-6 {
		t.Errorf("landed: alt %.2f, roll %.2f", e.Alt, e.Roll)
	}
	// Where it lies now, the next one doesn't fit.
	gh.rel = [2]vec3{{1, 0, 0}, {0, 1, 0}}
	g.stepPhysics(1.0 / 60)
	g.poseGhost()
	if d := planet.ArcDist(gh.Pose.Dir, g.Ents.Transform.Get(id).Dir); d < 0.5 && gh.OK {
		t.Errorf("fits %.2f m from a crate", d)
	}
	// Free, it floats ahead, where it is held; into the ground, it doesn't
	// fit.
	gh.Free, gh.dist = true, 4
	g.poseGhost()
	if gh.OK {
		t.Errorf("free under the ground: %.2f m high, fits", gh.Pose.Alt)
	}
	g.Player.Pitch = 0
	g.poseGhost()
	if gh.Pose.Alt < 0.5 || !gh.OK {
		t.Errorf("free: %.2f m high, fits %v", gh.Pose.Alt, gh.OK)
	}
}

// block sets a block d m ahead of the player, lying flat.
func block(g *Game, kind string, d float64) ecs.ID {
	id := g.spawn(kind, ahead(g, d))
	g.stepPhysics(1.0 / 60)
	return id
}

func TestBlockRays(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	for _, c := range []struct {
		kind string
		x    float64 // along its front, from its middle
		want float64 // its top there
	}{{"core:slab", 0, 0.2}, {"core:slope", 0, 0.5}, {"core:slope", -1, 0.75}, {"core:crate-2", 0.5, 1.94}} {
		id := block(g, c.kind, 6)
		pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(id))
		from := pos.Add(ax.Scale(c.x)).Add(up.Scale(10))
		d, n, ok := g.rayEntity(id, from, up.Scale(-1), 20)
		if !ok || math.Abs(10-d-c.want) > 1e-6 || n.Dot(up) < 0.5 {
			t.Errorf("%s at %.1f: hit %v, %.3f m high, normal %v", c.kind, c.x, ok, 10-d, n)
		}
		g.Ents.Kill(id)
	}
}

func TestBlocksStand(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	id := block(g, "core:crate-2", 4)
	at := g.Ents.Transform.Get(id).Dir
	walk(g, g.Player.Heading, 3)
	if d := planet.ArcDist(g.Player.LocalUp(), at); d < 1+0.35-0.05 {
		t.Errorf("walked %.2f m from the middle of a 2 m crate", d)
	}
	if planet.ArcDist(at, g.Ents.Transform.Get(id).Dir) > 1e-9 {
		t.Error("the block moved")
	}
	// A ghost aimed at its face (too high to see its top) rests against
	// it, lined up; between it and the player, it doesn't fit.
	g.Player.Pitch = -0.2
	g.Ghost = &Ghost{Kind: "core:crate", item: "core:crate", rel: [2]vec3{{1, 0, 0}, {0, 1, 0}}}
	g.poseGhost()
	gh := g.Ghost
	if d := planet.ArcDist(gh.Pose.Dir, at); math.Abs(d-1.4) > 0.05 || gh.OK {
		t.Errorf("against the crate: %.2f m from its middle, fits %v", d, gh.OK)
	}
}

func TestStackOnSlab(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	block(g, "core:slab", 4)
	g.Player.Pitch = -0.5
	g.Ghost = &Ghost{Kind: "core:crate-2", item: "core:crate-2", rel: [2]vec3{{1, 0, 0}, {0, 1, 0}}}
	g.poseGhost()
	if gh := g.Ghost; math.Abs(gh.Pose.Alt-0.2) > 0.03 || !gh.OK {
		t.Errorf("on the slab: %.2f m high, fits %v", gh.Pose.Alt, gh.OK)
	}
}

func TestDriveIntoBlock(t *testing.T) {
	g, v := ride(t, "core:buggy")
	id := block(g, "core:crate-4", 25)
	at := g.Ents.Transform.Get(id).Dir
	step(g, v, controls{throttle: 1}, 4*60)
	if d := planet.ArcDist(g.Player.LocalUp(), at); d < 2 {
		t.Errorf("drove %.2f m from the middle of a 4 m crate", d)
	}
	if planet.ArcDist(at, g.Ents.Transform.Get(id).Dir) > 1e-9 {
		t.Error("the block moved")
	}
}
