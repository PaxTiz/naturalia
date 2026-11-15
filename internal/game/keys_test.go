package game

import (
	"testing"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// A key taken from an action that acts in the same places swaps with it;
// one used elsewhere (the photo mode) is shared.
func TestBind(t *testing.T) {
	g := &Game{}
	g.loadKeys()
	defer g.ResetKeys()

	g.Bind(ActForward, rl.KeyS)
	if KeyOf(ActForward) != rl.KeyS || KeyOf(ActBack) != rl.KeyW {
		t.Errorf("swap: forward %d, back %d", KeyOf(ActForward), KeyOf(ActBack))
	}
	g.Bind(ActUse, rl.KeyQ)
	if KeyOf(ActUse) != rl.KeyQ || KeyOf(ActRollLeft) != rl.KeyQ {
		t.Errorf("shared: use %d, roll %d", KeyOf(ActUse), KeyOf(ActRollLeft))
	}
	g.Bind(ActUse, rl.KeyEscape)
	if KeyOf(ActUse) != rl.KeyQ {
		t.Error("Escape bound")
	}
	if _, ok := g.Settings.Keys["forward"]; !ok {
		t.Error("forward not kept in the settings")
	}
	g.ResetKeys()
	if len(g.Settings.Keys) != 0 || KeyOf(ActBack) != rl.KeyS {
		t.Errorf("reset: %v", g.Settings.Keys)
	}
}

// No two actions acting in the same places share a default key.
func TestDefaultKeys(t *testing.T) {
	for a := range Actions {
		for b := a + 1; b < Actions; b++ {
			da, db := ActionDefs[a], ActionDefs[b]
			if da.Key == db.Key && da.where&db.where != 0 {
				t.Errorf("%s and %s share %s", da.ID, db.ID, KeyName(da.Key))
			}
		}
	}
}
