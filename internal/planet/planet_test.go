package planet

import (
	"math"
	"naturalia/internal/mathx"
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

// A pad lays a plane at the ground's height in its middle, and gives the
// relief back beyond its blend; rocks and crystals keep off it.
func TestPads(t *testing.T) {
	set := DefaultSettings()
	w := NewWorld(42, &set)
	mid := Toward(vec3{1, 0, 0.5}, 900)
	t1, t2 := mathx.TangentBasis(mid)
	edge := Toward2(mid, t1, 6)
	far := Toward2(mid, t2, 7+PadBlend+1)
	before := w.Relief.Height(far)
	raw := w.Relief.Height(mid)
	if w.Relief.PadSlope(mid, 7) <= 0 {
		t.Errorf("no slope at all around %v: the test sees nothing", mid)
	}
	w.Relief.SetPads([]Pad{{Dir: mid, Radius: 7}})
	plane := Radius + raw
	if d := w.Relief.Surface(edge).Dot(mid) - plane; math.Abs(d) > 1e-6 {
		t.Errorf("6 m from its middle, %f m off the plane", d)
	}
	if d := w.Relief.Height(mid) - raw; math.Abs(d) > 1e-9 {
		t.Errorf("its middle moved %f m", d)
	}
	if h := w.Relief.Height(far); h != before {
		t.Errorf("past its blend, the ground went from %f to %f m", before, h)
	}
	for _, r := range w.RocksNear(mid.Scale(Radius), 30) {
		if ArcDist(r.Dir, mid) < 7 {
			t.Errorf("a rock %.1f m from its middle", ArcDist(r.Dir, mid))
		}
	}
	w.Relief.SetPads(nil)
	if d := w.Relief.Height(edge) - w.Relief.raw(edge); math.Abs(d) > 1e-9 {
		t.Errorf("taken off, the ground stays %f m off", d)
	}
}
