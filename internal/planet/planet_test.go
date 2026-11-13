package planet

import (
	"math"
	"testing"
)

func TestReliefSettings(t *testing.T) {
	set := DefaultSettings()
	set.Features = []Feature{{Kind: "mesa", Dir: [2]float64{1, 1}, Distance: 320, Radius: 70, Height: 45}}
	set.Prepare()
	w := NewWorld(42, &set)
	top := Toward(vec3{1, 0, 1}, 320)
	if h := set.FeatureHeight(top); h != 45 {
		t.Errorf("on the mesa: %f", h)
	}
	if d := math.Abs(w.Relief.Height(BaseUp) - w.Relief.BaseHeight()); d > 1e-9 {
		t.Errorf("the pad under the base isn't flat: %f", d)
	}
	// A plane: 60 m out, still at the base's level along its up.
	far := w.Relief.Surface(Toward(vec3{1, 0, 0.4}, 60))
	if d := far.Dot(BaseUp) - (Radius + w.Relief.BaseHeight()); math.Abs(d) > 1e-3 {
		t.Errorf("the pad 60 m out is %f m off the plane", d)
	}
	flat := DefaultSettings()
	flat.Plains, flat.Hills, flat.Dunes, flat.Mountains, flat.Craters = 0, 0, 0, 0, 0
	flat.Volcano, flat.Canyon, flat.IceCap = 0, 0, 0
	if h := NewWorld(42, &flat).Relief.Height(Toward(vec3{0.3, 0, -1}, 900)); h != 0 {
		t.Errorf("every layer off, still %f m of relief", h)
	}
}

func TestCleanAndSave(t *testing.T) {
	set := DefaultSettings()
	w := NewWorld(7, &set)
	at := Toward(vec3{1, 0, 0}, 200)
	if got := w.Clean(at, 1, 5, 1e9); got <= 0 {
		t.Fatalf("nothing swept: %f", got)
	}
	back := NewWorld(7, &set)
	back.SetTileBytes(w.TileBytes())
	if a, b := w.DustAt(at), back.DustAt(at); a != b || a >= 1 {
		t.Errorf("dust after reload: %f, before %f", b, a)
	}
}
