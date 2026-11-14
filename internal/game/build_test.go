package game

import (
	"math"
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
