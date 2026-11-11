package main

import "testing"

func TestRegistryOverrideKeepsOrder(t *testing.T) {
	var r Registry[int]
	r.init("test")
	r.pack = "core"
	r.Add("core:a", 1)
	r.Add("core:b", 2)
	r.Add("bad", 3)
	r.pack = "mod"
	r.Add("core:a", 10)
	r.Patch("core:b", func(v *int) { *v *= 5 })
	if got := r.IDs(); len(got) != 2 || got[0] != "core:a" {
		t.Fatalf("ids = %v", got)
	}
	if a, _ := r.Get("core:a"); *a != 10 {
		t.Errorf("override: %d", *a)
	}
	if b, _ := r.Get("core:b"); *b != 10 {
		t.Errorf("patch: %d", *b)
	}
}

func TestMigrateOldSave(t *testing.T) {
	loadContent()
	st := State{OldTier: 4, Levels: map[string]int{"tank": 3, "fusee:turbo": 2}}
	migrateState(&st)
	if st.Tool != "core:blower" {
		t.Errorf("tool = %s", st.Tool)
	}
	if st.Levels["core:tank"] != 3 || st.Levels["fusee:turbo"] != 2 || len(st.Levels) != 2 {
		t.Errorf("levels = %v", st.Levels)
	}
	g := &Game{st: st}
	if c := g.capacity(); c < 50*4.09 || c > 50*4.1 {
		t.Errorf("capacity = %f", c)
	}
}

func TestEventsAndBuildings(t *testing.T) {
	loadContent()
	if got := buildingAt(vec3{-13, 1, 0}); got != "core:sale" {
		t.Errorf("in the sale room: %q", got)
	}
	if got := buildingAt(vec3{0, 1, -9}); got != "" {
		t.Errorf("in the airlock corridor: %q", got)
	}
	if len(corridors) != 4 || len(rooms) != 4 || len(interactables) != 4 {
		t.Errorf("%d corridors, %d rooms, %d interactables", len(corridors), len(rooms), len(interactables))
	}
	var sold float64
	content.On(evSell, func(g *Game, e *Event) { sold += e.Amount })
	(&Game{}).emit(Event{Name: evSell, Amount: 12})
	if sold != 12 {
		t.Errorf("sold = %f", sold)
	}
}
