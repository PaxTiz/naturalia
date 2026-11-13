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
			g.move(v, controls{throttle: 1}, 1.0/60)
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
			g.move(v, controls{throttle: 1, steer: 1}, 1.0/60)
		}
		for range 10 {
			g.move(v, controls{}, 1.0/60)
		}
		fwd := g.drive.vel.Dot(g.drive.fwd)
		side := g.drive.vel.Sub(g.drive.fwd.Scale(fwd)).Length()
		if drift := side / g.drive.vel.Length(); (v.Hover > 0) != (drift > 0.05) {
			t.Errorf("%s: drifts %.3f of its speed", id, drift)
		}
		for range 10 * 60 {
			g.move(v, controls{throttle: -1}, 1.0/60)
		}
		if s := g.drive.vel.Dot(g.drive.fwd); math.Abs(s+v.Speed*0.35) > 0.01 {
			t.Errorf("%s: backs up at %.2f", id, s)
		}
	}
}

// ride boards a new vehicle of kind id on fleetGame's plain.
func ride(t *testing.T, id string) (*Game, *content.Vehicle) {
	t.Helper()
	c := LoadContent()
	g := fleetGame(t, c, map[string]int{})
	g.Mode = ModePlay
	v, ok := c.Vehicles.Get(id)
	if !ok {
		t.Fatalf("no %s", id)
	}
	g.Board(id)
	return g, v
}

// step drives for n frames as the game does: move, then set the vehicle.
func step(g *Game, v *content.Vehicle, in controls, n int) {
	for range n {
		g.move(v, in, 1.0/60)
		g.updateVehicle(1.0 / 60)
		in.jump = false
	}
}

func TestJump(t *testing.T) {
	g, v := ride(t, "core:buggy")
	var lands []float64
	g.Content.On(content.EvLand, func(_ content.Game, e *content.Event) { lands = append(lands, e.Amount) })
	step(g, v, controls{throttle: 1}, 120)
	step(g, v, controls{throttle: 1, jump: true}, 1)
	peak := 0.0
	for range 60 * 6 {
		step(g, v, controls{throttle: 1}, 1)
		peak = math.Max(peak, g.DriveAlt())
	}
	if want := v.Jump * v.Jump / (2 * marsGravity * v.Gravity); math.Abs(peak-want) > 0.6 {
		t.Errorf("jumped %.2f m high, want %.2f", peak, want)
	}
	if len(lands) == 0 || lands[0] < 1.5 {
		t.Errorf("landings %v", lands)
	}
	if g.drive.air {
		t.Error("still in the air")
	}
	// A double jump goes higher, and flips.
	step(g, v, controls{throttle: 1, jump: true}, 1)
	step(g, v, controls{throttle: 1}, 40)
	step(g, v, controls{throttle: 1, jump: true}, 1)
	step(g, v, controls{throttle: 1}, 10)
	if g.drive.flip == 0 || g.drive.jumps != 2 {
		t.Errorf("second jump: flip %.2f, %d jumps", g.drive.flip, g.drive.jumps)
	}
	step(g, v, controls{throttle: 1, jump: true}, 1)
	if g.drive.jumps != 2 {
		t.Error("a third jump")
	}
}

func TestRampLaunches(t *testing.T) {
	g, v := ride(t, "core:rover")
	// A ramp 25 m ahead, rising away.
	at := g.World.Relief.Surface(g.Player.Pos.Add(g.Player.Heading.Scale(25)))
	g.spawn("core:ramp", at)
	g.State.Entities[len(g.State.Entities)-1].Yaw = yawOf(at.Norm(), g.Player.Heading)
	peak := 0.0
	for range 60 * 6 {
		step(g, v, controls{throttle: 1, boost: true}, 1)
		peak = math.Max(peak, g.DriveAlt())
	}
	if peak < 2.5 {
		t.Errorf("only %.2f m high off the ramp", peak)
	}
}

func TestBoostAndPad(t *testing.T) {
	g, v := ride(t, "core:rover")
	step(g, v, controls{throttle: 1, boost: true}, 200)
	if s := g.drive.vel.Length(); s < v.Speed*1.3 {
		t.Errorf("boosted to %.1f m/s, want %.1f", s, v.Speed*v.Boost)
	}
	if g.drive.fuel > 0.1 {
		t.Errorf("%.1f s of boost left after 3.3 s", g.drive.fuel)
	}
	step(g, v, controls{throttle: 1}, 60*8)
	if math.Abs(g.drive.vel.Length()-v.Speed) > 0.05 || g.drive.fuel < 2.9 {
		t.Errorf("after the boost: %.1f m/s, %.1f s of fuel", g.drive.vel.Length(), g.drive.fuel)
	}
	at := g.World.Relief.Surface(g.Player.Pos.Add(g.Player.Heading.Scale(10)))
	g.spawn("core:pad", at)
	fast := 0.0
	for range 120 {
		step(g, v, controls{throttle: 1}, 1)
		fast = math.Max(fast, g.drive.vel.Length())
	}
	if fast < v.Speed+10 {
		t.Errorf("the pad threw it to %.1f m/s", fast)
	}
}

func TestRocket(t *testing.T) {
	g, v := ride(t, "core:rocket")
	start := g.Player.LocalUp()
	step(g, v, controls{throttle: 1}, 60)
	if planet.ArcDist(start, g.Player.LocalUp()) > 0.5 {
		t.Error("rolls on the ground")
	}
	step(g, v, controls{climb: 1}, 60*25)
	if alt := g.DriveAlt(); alt < v.Fly.Ceiling*0.9 || alt > v.Fly.Ceiling+100 {
		t.Errorf("climbed to %.0f m", alt)
	}
	step(g, v, controls{throttle: 1}, 60*10)
	if s := g.DriveSpeed(); s < v.Speed*0.9 {
		t.Errorf("flies at %.0f m/s", s)
	}
	// Round Mars in under a minute.
	far := planet.ArcDist(start, g.Player.LocalUp())
	if far < 3000 {
		t.Errorf("only %.0f m from the start", far)
	}
	alt := g.DriveAlt()
	step(g, v, controls{}, 60*20)
	step(g, v, controls{climb: -1}, 60*60)
	if g.DriveAlt() > 0.01 || g.drive.air {
		t.Errorf("still %.1f m up after descending from %.0f m", g.DriveAlt(), alt)
	}
}
