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
	if n := len(yardObstacles()); n != 15 {
		t.Errorf("the core base leaves out yard objects: %d obstacles", n)
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

func TestExampleMod(t *testing.T) {
	m, err := readMod("examples/mods/serre")
	if err != nil {
		t.Fatal(err)
	}
	packs = append(packs, m.pack())
	defer func() { packs = packs[:len(packs)-1] }()
	loadContent()

	ids := content.Tools.IDs()
	if ids[3] != "serre:rateau" || ids[4] != "core:vacuum" {
		t.Errorf("tools in order: %v", ids)
	}
	if b, _ := content.Tools.Get("broom"); b.Cost != 400 || b.Look.Head != headFlare {
		t.Errorf("patched broom: %+v", b)
	}
	if r, _ := content.Tools.Get("serre:rateau"); r.Look.HeadCol.G != 0x8c || r.Look.HandleLen != 0.6 {
		t.Errorf("rake look: %+v", r.Look)
	}
	if len(rooms) != 5 || len(corridors) != 5 {
		t.Fatalf("%d rooms, %d corridors", len(rooms), len(corridors))
	}
	bp := plans[4]
	if bp.ID != "serre:serre" || bp.Props[0].Kit != "serre:arbre" || bp.Props[1].Kit != "core:planter" || len(bp.Doors) != 1 {
		t.Errorf("greenhouse: %+v", bp)
	}
	if len(plans[0].Doors) != 5 {
		t.Errorf("hub doors: %v", plans[0].Doors)
	}
	if tk, _ := content.Tokens.Get("serre:leaf"); tk.col.G != 0xa0 || tk.scale != 1 {
		t.Errorf("leaf token: %+v", tk)
	}
	if n := len(yardObstacles()); n != 14 {
		t.Errorf("the greenhouse should push the barrels away: %d yard obstacles", n)
	}
	if got := buildingAt(vec3{-10, 1, 10}); got != "serre:serre" {
		t.Errorf("in the greenhouse: %q", got)
	}
}
