package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/planet"
	"testing"
)

// floorGame is a base with a workshop east of it: a hall with a
// mezzanine along its west wall, 3 m up, and stairs rising to it; the
// mezzanine's open edges have guard rails.
func floorGame(t *testing.T) *Game {
	t.Cleanup(func() { LoadContent() })
	c := LoadContent(content.Pack{ID: "t", Requires: []string{"core"}, Register: func(c *content.Content) {
		c.Buildings.Add("t:atelier", content.Blueprint{ID: "t:atelier", At: vec3{35, 0, 0}, Style: "habitat.white", Accent: "neon.cyan",
			Hall: &content.Hall{Size: [2]float64{16, 12}, WallH: 7, Rise: 2, Door: [2]float64{6, 4}},
			Floors: []content.Floor{
				{From: vec3{-6, 3, -5.5}, To: vec3{-6, 3, 5.5}, Width: 4, Thick: 0.3},
				{From: vec3{0.5, 0, 4.8}, To: vec3{-4, 3, 4.8}, Width: 1.2},
			}})
	}})
	g := fleetGame(t, c, map[string]int{})
	base.Origin = planet.BaseUp.Scale(planet.Radius + g.World.Relief.BaseHeight())
	g.walls = base.Walls()
	return g
}

// walkFor walks the player along the local direction to for n frames, at
// 3 m/s, and tells where they end (local).
func walkFor(g *Game, to vec3, n int) vec3 {
	for range n {
		g.Player.want = planet.TangentAt(g.Player.LocalUp(), to).Scale(3)
		g.stepPhysics(1.0 / 60)
		g.Player.fall(1.0/60, g.World.Relief)
	}
	return base.ToLocal(g.Player.Pos)
}

// standAt puts the player on the ground at local x, z.
func standAt(g *Game, x, z float64) {
	g.Player.Pos, g.Player.vel, g.Player.vr, g.Player.floor = g.World.Relief.Surface(base.ToWorld(vec3{x, 0, z})), vec3{}, 0, 0
	g.phys = nil
}

func TestStairsAndMezzanine(t *testing.T) {
	g := floorGame(t)
	if base.Ents.Floor.Len() != 4 { // and the great hall's
		t.Fatalf("%d floors", base.Ents.Floor.Len())
	}
	ground := base.ToLocal(g.World.Relief.Surface(base.ToWorld(vec3{33, 0, 0}))).Y

	// Up the stairs, a step at a time, onto the mezzanine.
	standAt(g, 36.5, 4.8)
	at := walkFor(g, vec3{-1, 0, 0}, 150)
	if at.X > 30 || at.Y-ground < 2.95 {
		t.Fatalf("up the stairs: at %v", at)
	}
	// Along it, up there.
	at = walkFor(g, vec3{0, 0, -1}, 120)
	if at.Z > 0 || at.Y-ground < 2.95 {
		t.Errorf("along the mezzanine: at %v", at)
	}
	// Its guard rail holds them up there, at its edge (x = 31).
	at = walkFor(g, vec3{1, 0, 0}, 60)
	if at.X < 30.4 || at.X > 31 || at.Y-ground < 2.95 {
		t.Errorf("at the guard rail: at %v", at)
	}
	// Along the rail to where the stairs arrive, and down them.
	walkFor(g, vec3{0, 0, 1}, 150)
	at = walkFor(g, vec3{1, 0, 0}, 150)
	if at.X < 34 || at.Y-ground > 0.05 {
		t.Errorf("down the stairs: at %v", at)
	}

	// Under it, on the ground, walking about.
	standAt(g, 32, -2)
	at = walkFor(g, vec3{-1, 0, 0}, 60)
	if at.X > 29.6 || at.Y-ground > 0.05 {
		t.Errorf("under the mezzanine: at %v", at)
	}
	// A jump: the head bumps into the slab.
	g.Player.vr = jumpSpeed
	top := 0.0
	for range 60 {
		g.stepPhysics(1.0 / 60)
		g.Player.fall(1.0/60, g.World.Relief)
		top = math.Max(top, base.ToLocal(g.Player.Pos).Y-ground)
	}
	if top+personH > 2.7+0.01 {
		t.Errorf("jumped %.2f m: the head went into the slab", top)
	}

	// The side of the stairs, halfway up, stops someone walking into it.
	standAt(g, 33.2, 2.5)
	at = walkFor(g, vec3{0, 0, 1}, 90)
	if at.Z > 4.2-0.3 || at.Y-ground > 0.05 {
		t.Errorf("into the side of the stairs: at %v", at)
	}
}

// The great hall's lookout: up the stairs along the south wall, onto the
// gallery, where the telescope is in reach; from under it, it is not.
func TestLookout(t *testing.T) {
	g := baseGame(t)
	ground := base.ToLocal(g.World.Relief.Surface(base.ToWorld(vec3{3, 0, -5}))).Y
	usable := func() bool {
		it, ok := g.NearbyUse()
		return ok && it.Kit == "core:telescope"
	}
	standAt(g, 9.4, -3.5)
	g.Player.Heading = vec3{1, 0, 0}
	if usable() {
		t.Error("the telescope is in reach from under the gallery")
	}
	standAt(g, 1.5, -6.9)
	at := walkFor(g, vec3{1, 0, 0}, 150)
	if at.X < 8 || at.Y-ground < 2.95 {
		t.Fatalf("up the stairs: at %v", at)
	}
	at = walkFor(g, vec3{0, 0, 1}, 80)
	g.Player.Heading = vec3{1, 0, 0}
	if !usable() {
		t.Errorf("the telescope is out of reach on the gallery, at %v", at)
	}
	// The rail along the gallery's west edge holds them.
	at = walkFor(g, vec3{-1, 0, 0}, 90)
	if at.X < 7.75 || at.Y-ground < 2.95 {
		t.Errorf("through the guard rail: at %v", at)
	}
}
