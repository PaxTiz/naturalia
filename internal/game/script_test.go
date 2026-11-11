package game

import (
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/script"
	_ "mars_sweeper/packs/core"
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

	g := &Game{State: State{Credits: 5}, Content: c}
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
