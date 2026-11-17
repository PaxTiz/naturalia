package game

import (
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
