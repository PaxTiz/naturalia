package main

import (
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
	m, err := readMod(dir)
	if err != nil {
		t.Fatal(err)
	}
	packs = append(packs, m.pack())
	defer func() { packs = packs[:len(packs)-1] }()
	loadContent()

	g := &Game{st: State{Credits: 5}}
	g.emit(Event{Name: evSell, Amount: 12})
	g.emit(Event{Name: evSell, Amount: 3})
	if v := g.st.Mods["essai"]["ventes"]; v != int64(15) && v != 15.0 {
		t.Errorf("kept %v (%T)", v, v)
	}
	start := time.Now()
	g.emit(Event{Name: evWake}) // an endless loop, cut short
	if d := time.Since(start); d > time.Second {
		t.Errorf("the loop ran %v", d)
	}
	ev, ok := content.Events.Get("essai:pluie")
	if !ok {
		t.Fatal("random event not registered")
	}
	ev.Run(g)
	if g.st.Credits != 4 {
		t.Errorf("credits = %f", g.st.Credits)
	}
}
