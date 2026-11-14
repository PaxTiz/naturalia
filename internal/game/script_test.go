package game

import (
	"naturalia/internal/content"
	"naturalia/internal/script"
	_ "naturalia/packs/core"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScripts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mod.json"), []byte(`{"id": "essai", "scripts": ["a.js"]}`), 0o644)
	os.WriteFile(filepath.Join(dir, "a.js"), []byte(`
		game.on("sell", e => { game.set("ventes", (game.get("ventes") || 0) + e.amount); });
		game.on("wake", () => { while (true) {} });
		game.randomEvent({id: "pluie", every: 10, run: () => game.giveCredits(-1)});
		try { game.credits(); throw "no"; } catch (e) { if (e === "no") throw "credits() should throw while loading"; }
	`), 0o644)
	m, err := content.ReadMod(dir, script.Run)
	if err != nil {
		t.Fatal(err)
	}
	c := LoadContent(m.Pack())

	g := &Game{State: State{Credits: 5}, Content: c, Ents: newEnts()}
	g.emit(content.Event{Name: content.EvSell, Amount: 12})
	g.emit(content.Event{Name: content.EvSell, Amount: 3})
	if v := g.State.Mods["essai"]["ventes"]; v != int64(15) && v != 15.0 {
		t.Errorf("kept %v (%T)", v, v)
	}
	start := time.Now()
	g.emit(content.Event{Name: content.EvWake}) // an endless loop, cut short
	if d := time.Since(start); d > time.Second {
		t.Errorf("the loop ran %v", d)
	}
	ev, ok := c.Events.Get("essai:pluie")
	if !ok {
		t.Fatal("random event not registered")
	}
	ev.Run(g)
	if g.State.Credits != 4 {
		t.Errorf("credits = %f", g.State.Credits)
	}
}

func TestScriptsTouch(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mod.json"), []byte(`{"id": "essai", "scripts": ["a.js"],
		"entities": {"zone": {"name": "Zone", "trigger": true, "radius": 1.5, "data": {"bonus": 7}}},
		"patch": {"entities": {"core:crate": {"mass": 80, "data": {"lourde": true}}}}}`), 0o644)
	os.WriteFile(filepath.Join(dir, "a.js"), []byte(`
		game.on("trigger_enter", e => {
			if (e.id === "essai:zone" && e.by === "player" && e.entity > 0) game.giveCredits(game.data("zone").bonus);
		});
		game.on("trigger_leave", e => game.set("out", (game.get("out") || 0) + 1));
		game.on("sell", () => {
			for (const e of game.near("core:crate", 20)) {
				const l = Math.hypot(e.at.x, e.at.y, e.at.z);
				if (game.data("core:crate").lourde) game.impulse(e.id, {x: e.at.x / l * 4, y: e.at.y / l * 4, z: e.at.z / l * 4});
			}
		});
	`), 0o644)
	m, err := content.ReadMod(dir, script.Run)
	if err != nil {
		t.Fatal(err)
	}
	c := LoadContent(m.Pack())
	if e, _ := c.Entities.Get("core:crate"); e.Mass != 80 || len(e.Parts) == 0 {
		t.Errorf("patched crate: mass %v, %d parts", e.Mass, len(e.Parts))
	}
	g := fleetGame(t, c, map[string]int{})
	g.spawn("essai:zone", ahead(g, 3))
	walk(g, g.Player.Heading, 2.5)
	if g.State.Credits != 7 || g.State.Mods["essai"]["out"] == nil {
		t.Errorf("walked through the zone: %v credits, left it %v times", g.State.Credits, g.State.Mods["essai"]["out"])
	}
	crate := g.spawn("core:crate", ahead(g, 4))
	g.stepPhysics(1.0 / 60)
	g.emit(content.Event{Name: content.EvSell})
	if a := g.Ents.Air.Get(crate); a == nil || a.vz < 3.9 {
		t.Errorf("the crate wasn't lifted: %+v", a)
	}
}
