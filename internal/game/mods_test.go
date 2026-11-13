package game

import (
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/planet"
	"naturalia/internal/script"
	_ "naturalia/packs/core"
	"testing"
)

func TestMigrateOldSave(t *testing.T) {
	c := LoadContent()
	st := State{OldTier: 4, Levels: map[string]int{"tank": 3, "fusee:turbo": 2}}
	migrateState(&st, c)
	if st.Tool != "core:blower" {
		t.Errorf("tool = %s", st.Tool)
	}
	if st.Levels["core:tank"] != 3 || st.Levels["fusee:turbo"] != 2 || len(st.Levels) != 2 {
		t.Errorf("levels = %v", st.Levels)
	}
	g := &Game{State: st, Content: c}
	if c := g.Capacity(); c < 50*4.09 || c > 50*4.1 {
		t.Errorf("capacity = %f", c)
	}
}

func TestEventsAndBuildings(t *testing.T) {
	c := LoadContent()
	if got := buildingAt(vec3{-13, 1, 0}); got != "core:sale" {
		t.Errorf("in the sale room: %q", got)
	}
	if got := buildingAt(vec3{0, 1, -9}); got != "" {
		t.Errorf("in the airlock corridor: %q", got)
	}
	if n := len(base.YardObstacles()); n != 15 {
		t.Errorf("the core base leaves out yard objects: %d obstacles", n)
	}
	if len(base.Corridors) != 5 || len(base.Rooms) != 5 || len(base.Interactables) != 5 {
		t.Errorf("%d corridors, %d rooms, %d interactables", len(base.Corridors), len(base.Rooms), len(base.Interactables))
	}
	var sold float64
	c.On(content.EvSell, func(g content.Game, e *content.Event) { sold += e.Amount })
	(&Game{Content: c}).emit(content.Event{Name: content.EvSell, Amount: 12})
	if sold != 12 {
		t.Errorf("sold = %f", sold)
	}
}

func TestExampleMod(t *testing.T) {
	m, err := content.ReadMod("../../examples/mods/serre", script.Run)
	if err != nil {
		t.Fatal(err)
	}
	c := LoadContent(m.Pack())

	ids := c.Tools.IDs()
	if ids[3] != "serre:rateau" || ids[4] != "core:vacuum" {
		t.Errorf("tools in order: %v", ids)
	}
	if b, _ := c.Tools.Get("broom"); b.Cost != 400 || b.Look.Head != content.HeadFlare {
		t.Errorf("patched broom: %+v", b)
	}
	if r, _ := c.Tools.Get("serre:rateau"); r.Look.HeadCol.G != 0x8c || r.Look.HandleLen != 0.6 {
		t.Errorf("rake look: %+v", r.Look)
	}
	if len(base.Rooms) != 6 || len(base.Corridors) != 6 {
		t.Fatalf("%d rooms, %d corridors", len(base.Rooms), len(base.Corridors))
	}
	bp := base.Plans[5]
	if bp.ID != "serre:serre" || bp.Props[0].Kit != "serre:arbre" || bp.Props[1].Kit != "core:planter" || len(bp.Doors) != 1 {
		t.Errorf("greenhouse: %+v", bp)
	}
	if len(base.Plans[0].Doors) != 5 {
		t.Errorf("hub doors: %v", base.Plans[0].Doors)
	}
	if tk, _ := c.Tokens.Get("serre:leaf"); tk.Col.G != 0xa0 || tk.Scale != 1 {
		t.Errorf("leaf token: %+v", tk)
	}
	if n := len(base.YardObstacles()); n != 14 {
		t.Errorf("the greenhouse should push the barrels away: %d yard obstacles", n)
	}
	if tr := c.Terrain; tr.Gems != 0.15 || len(tr.Features) != 2 || tr.Hills != 1 {
		t.Errorf("terrain: %+v", tr)
	}
	mesa := planet.Toward(vec3{1, 0, 1}, 320)
	if h := c.Terrain.FeatureHeight(mesa); h != 45 {
		t.Errorf("on the mesa: %f", h)
	}
	if it, _ := c.Items.Get("serre:pousse"); it == nil || it.Places != "serre:pousse" {
		t.Errorf("item: %+v", it)
	}
	if e, _ := c.Entities.Get("serre:pousse"); e == nil || e.Item != "serre:pousse" {
		t.Errorf("entity: %+v", e)
	}
	if e, _ := c.Entities.Get("serre:brouette"); e == nil || e.Parts[1].Token != "serre:leaf" {
		t.Errorf("vehicle's entity: %+v", e)
	}
	if len(c.Debug) != 1 {
		t.Errorf("%d debug lines", len(c.Debug))
	}
	if got := buildingAt(vec3{-10, 1, 10}); got != "serre:serre" {
		t.Errorf("in the greenhouse: %q", got)
	}
}

func TestStuntMod(t *testing.T) {
	m, err := content.ReadMod("../../examples/mods/cascadeur", script.Run)
	if err != nil {
		t.Fatal(err)
	}
	c := LoadContent(m.Pack())
	if e, _ := c.Entities.Get("cascadeur:mega-tremplin"); e == nil || e.Ramp == nil || e.Ramp.Height != 5 || e.Parts[0].Shape != "wedge" {
		t.Errorf("ramp: %+v", e)
	}
	if v, _ := c.Vehicles.Get("core:rover"); v.Jumps != 2 || v.Jump != 5.5 {
		t.Errorf("patched rover: jump %f, %d jumps", v.Jump, v.Jumps)
	}
	g := fleetGame(t, c, map[string]int{})
	g.Mode = ModePlay
	v, _ := c.Vehicles.Get("cascadeur:moto")
	g.Board(v.ID)
	if !g.Chase() {
		t.Error("the bike starts in the chase view")
	}
	// Two jumps landed in a row make a combo: the script pushes the
	// bike and pays.
	before := g.State.Credits
	for range 2 {
		step(g, v, controls{throttle: 1}, 30)
		step(g, v, controls{throttle: 1, jump: true}, 1)
		for i := 0; i < 600 && (g.drive.air || i < 10); i++ {
			step(g, v, controls{throttle: 1}, 1)
		}
	}
	if g.State.Credits-before < 400 {
		t.Errorf("combo paid %.0f cr", g.State.Credits-before)
	}
	if rec, _ := g.ModData("cascadeur")["record"].(float64); rec < 1 {
		t.Errorf("record %v", g.ModData("cascadeur")["record"])
	}
	if len(c.Debug) != 1 {
		t.Fatalf("%d debug lines", len(c.Debug))
	}
	if line := c.Debug[0](g); line == "" {
		t.Error("empty debug line")
	}
}
