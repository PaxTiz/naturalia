package game

import (
	"math"
	"math/rand/v2"
	"naturalia/internal/content"
	"naturalia/internal/planet"
	_ "naturalia/packs/core"
	"os"
	"path/filepath"
	"testing"
)

// fleetGame is a game on a Mars without crystals (picking one up beeps,
// and tests have no sound), the player 200 m from the base.
func fleetGame(t *testing.T, c *content.Content, levels map[string]int) *Game {
	t.Helper()
	set := c.Terrain
	set.Gems = 0
	g := &Game{Content: c, Riding: -1, Rng: rand.New(rand.NewPCG(1, 2)), State: State{Levels: levels}}
	g.World = planet.NewWorld(7, &set)
	g.Player.Pos = g.World.Relief.Surface(planet.Toward(vec3{1, 0, 0}, 200))
	g.Player.Heading = vec3{0, 0, 1}
	return g
}

func TestFleetFromOldDrones(t *testing.T) {
	c := LoadContent()
	// Drones were an upgrade under the same ID: its level is the count.
	g := fleetGame(t, c, map[string]int{"core:drone": 3, "core:crawler": 2, "core:rover": 1})
	g.syncFleet()
	g.syncFleet()
	if n := g.Entities("core:drone"); n != 3 {
		t.Errorf("%d drones", n)
	}
	if n := g.Entities("core:rover"); n != 1 {
		t.Errorf("%d rovers", n)
	}
	if n := g.State.Items["core:crawler"]; n != 2 || g.Entities("core:crawler") != 0 {
		t.Errorf("%d crawlers carried", n)
	}
	for _, it := range g.ShopItems() {
		switch it.id {
		case "core:drone":
			if want := 300 * math.Pow(1.9, 3); math.Abs(it.Cost-want) > 1e-6 {
				t.Errorf("4th drone costs %f, want %f", it.Cost, want)
			}
		case "core:rover":
			if !it.Maxed {
				t.Error("a second rover is for sale")
			}
		}
	}
}

func TestPatrolSweepsAroundItsSpot(t *testing.T) {
	c := LoadContent()
	g := fleetGame(t, c, map[string]int{})
	g.State.Levels["core:tank"] = 20 // room for what it sweeps
	g.spawn("core:crawler", g.Player.Pos)
	home := g.State.Entities[0].Dir
	for range 60 * 60 {
		g.updateFleet(1.0 / 60)
	}
	e := g.State.Entities[0]
	if e.Home != home {
		t.Errorf("home moved")
	}
	if d := planet.ArcDist(e.Dir, home); d > 31 || d < 0.5 {
		t.Errorf("%.1f m from its spot after a minute", d)
	}
	if g.State.Tank < 10 {
		t.Errorf("swept %.1f g in a minute", g.State.Tank)
	}
}

func TestModVehicles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mod.json"), []byte(`{"id": "flotte", "vehicles": [
		{"id": "taupe", "name": "Taupe", "cost": 10, "drive": "auto", "ai": "patrol", "speed": 2, "sweep": 2, "rate": 1, "range": 12,
		 "parts": [{"shape": "box", "token": "core:steel", "size": [1, 0.5, 0.6]}]},
		{"id": "luge", "name": "Luge", "cost": 10, "speed": 9, "hover": 0.3, "grip": 2, "sweep": 4, "hopper": 100}
	], "patch": {"vehicles": {"core:rover": {"sweep": 6}}}}`), 0o644)
	m, err := content.ReadMod(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := LoadContent(m.Pack())
	if e, _ := c.Entities.Get("flotte:taupe"); e == nil || !e.Pickup || e.Prompt != "Ramasser : Taupe" {
		t.Errorf("patrol entity: %+v", e)
	}
	if it, _ := c.Items.Get("flotte:taupe"); it == nil || it.Places != "flotte:taupe" || it.Cost != 0 {
		t.Errorf("patrol item: %+v", it)
	}
	if v, _ := c.Vehicles.Get("flotte:luge"); v == nil || v.Auto() || v.Max != 1 || v.Hover != 0.3 {
		t.Errorf("driven vehicle: %+v", v)
	}
	if v, _ := c.Vehicles.Get("core:rover"); v.Sweep != 6 {
		t.Errorf("patched rover sweeps %f", v.Sweep)
	}
}

func TestDriving(t *testing.T) {
	c := LoadContent()
	for _, id := range []string{"core:rover", "core:hover"} {
		g := fleetGame(t, c, map[string]int{})
		v, _ := c.Vehicles.Get(id)
		g.drive.fwd = g.Player.Heading
		start := g.Player.Pos
		for range 5 * 60 { // full throttle for 5 s
			g.move(v, 1, 0, 1.0/60)
		}
		if s := g.drive.vel.Length(); math.Abs(s-v.Speed) > 0.01 {
			t.Errorf("%s: top speed %.2f, want %.2f", id, s, v.Speed)
		}
		if d := g.Player.Pos.Sub(start).Length(); d < v.Speed*2 {
			t.Errorf("%s: only %.1f m in 5 s", id, d)
		}
		// Steering at speed, then letting go: wheels keep no sideways
		// speed, a hovercraft drifts on.
		for range 30 {
			g.move(v, 1, 1, 1.0/60)
		}
		for range 10 {
			g.move(v, 0, 0, 1.0/60)
		}
		fwd := g.drive.vel.Dot(g.drive.fwd)
		side := g.drive.vel.Sub(g.drive.fwd.Scale(fwd)).Length()
		if drift := side / g.drive.vel.Length(); (v.Hover > 0) != (drift > 0.05) {
			t.Errorf("%s: drifts %.3f of its speed", id, drift)
		}
		for range 10 * 60 {
			g.move(v, -1, 0, 1.0/60)
		}
		if s := g.drive.vel.Dot(g.drive.fwd); math.Abs(s+v.Speed*0.35) > 0.01 {
			t.Errorf("%s: backs up at %.2f", id, s)
		}
	}
}
