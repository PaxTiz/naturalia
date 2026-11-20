package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/ecs"
	"testing"
)

// The plant in the bedroom grows a stage for each night slept, up to its
// last, and the nights are counted in the save.
func TestPlantGrows(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // the save goes there
	g := baseGame(t)
	var plant ecs.ID
	for id, f := range base.AllFurniture() {
		if f.Kit == "core:plant" {
			plant = id
		}
	}
	if plant == 0 {
		t.Fatal("no plant in the base")
	}
	for night := range 7 {
		if got, want := base.Stage(plant, g.Nights()), min(night, base.PlantStages-1); got != want {
			t.Errorf("night %d: stage %d, want %d", night, got, want)
		}
		g.Sleep()
		for g.SleepT < SleepWake { // past the save; waking up takes the window's cursor
			g.updateSleep(1.0 / 30)
		}
		g.Mode = ModePlay
	}
	if g.State.Nights != 7 {
		t.Errorf("%d nights counted", g.State.Nights)
	}
	g.load()
	if g.State.Nights != 7 {
		t.Errorf("%d nights after loading the save", g.State.Nights)
	}
	// Every stage builds, the last one up the shelf.
	for s := range base.PlantStages {
		if len(base.BuildGrowth(&g.Content.Kits, plant, s).Keys) == 0 {
			t.Errorf("stage %d: nothing built", s)
		}
	}
}

// The clock runs while playing, stops on the pause screen, and sleeping
// wakes up at dawn where the bed is, the next morning; the hour is saved.
func TestSleepToDawn(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	g := baseGame(t)
	if h := g.BaseHour(); h != Dawn {
		t.Fatalf("a new game starts at %.2f h", h)
	}
	g.Mode = ModePlay
	g.tick(g.Content.Terrain.Sol / 2) // half a sol: evening
	if h := g.BaseHour(); math.Abs(h-(Dawn+12)) > 0.01 {
		t.Fatalf("half a sol later it is %.2f h", h)
	}
	g.Mode = ModePaused
	g.tick(100)
	if h := g.BaseHour(); math.Abs(h-(Dawn+12)) > 0.01 {
		t.Fatalf("the pause screen ran the clock to %.2f h", h)
	}
	sol := g.State.Sol
	g.Sleep()
	for g.SleepT < SleepWake {
		g.updateSleep(1.0 / 30)
	}
	if h := g.LocalHour(g.Player.Pos); math.Abs(h-Dawn) > 1e-6 {
		t.Errorf("woke up at %.2f h", h)
	}
	if d := g.State.Sol - sol; d <= 0 || d > 1 {
		t.Errorf("slept %.2f sol", d)
	}
	sol = g.State.Sol
	g.load()
	if g.State.Sol != sol {
		t.Errorf("sol %g after loading, want %g", g.State.Sol, sol)
	}
}
