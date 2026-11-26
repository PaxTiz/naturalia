package game

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// An entity standing on a hollow's floor stands there (Frame), keeps it
// through a save, and those of older saves, which don't tell, stand on the
// relief.
func TestStandsOnFloor(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	up := g.Player.LocalUp()
	ground := g.World.Relief.SurfaceRadius(up)
	floor := ground - 12

	id := g.add(Placed{Kind: "core:crate", Dir: up, Floor: floor, Alt: 0.5})
	pos, _, _, _ := g.Frame(g.Ents.Transform.Get(id))
	if d := pos.Length() - (floor + 0.5); math.Abs(d) > 1e-9 {
		t.Errorf("on the floor: %.3f m off", d)
	}

	var back []Placed
	data, _ := json.Marshal(g.placed())
	if err := json.Unmarshal(data, &back); err != nil || len(back) != 1 || back[0].Floor != floor {
		t.Fatalf("saved floor: %+v %v", back, err)
	}

	var old Placed
	if err := json.Unmarshal([]byte(`{"kind":"core:crate","dir":{"X":0,"Y":1,"Z":0},"yaw":0}`), &old); err != nil {
		t.Fatal(err)
	}
	g2 := fleetGame(t, LoadContent(), map[string]int{})
	old.Dir = g2.Player.LocalUp()
	e := g2.Ents.Transform.Get(g2.add(old))
	if e.Floor != 0 || g2.foot(e) != g2.World.Relief.Surface(e.Dir) {
		t.Errorf("an older save's entity stands %.3f m off the relief", g2.foot(e).Length()-g2.World.Relief.SurfaceRadius(e.Dir))
	}
	if s, _ := json.Marshal(g2.placed()); strings.Contains(string(s), "floor") {
		t.Errorf("an entity on the relief saves a floor: %s", s)
	}
}
