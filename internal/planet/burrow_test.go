package planet

import (
	"math"
	"testing"
)

// slab is a volume: a floor and a ceiling, everywhere.
type slab struct{ floor, ceiling float64 }

func (s slab) At(p vec3) (float64, float64, bool) {
	r := p.Length()
	return s.floor, s.ceiling, r > s.floor-hollowUnder && r < s.ceiling
}

// Two stories, one over the other: each holds its own, and between them
// the feet stand on the upper floor once over it.
func TestBurrowStories(t *testing.T) {
	low, high := slab{Radius - 20, Radius - 16}, slab{Radius - 15.5, Radius - 10}
	b := NewBurrow(low, nil, high, (*Digs)(nil))
	up := vec3{0, 1, 0}
	for _, c := range []struct {
		r     float64
		floor float64
		in    bool
	}{
		{Radius - 18, low.floor, true},
		{Radius - 12, high.floor, true},
		{Radius - 17, low.floor, true},    // under the upper floor's reach
		{Radius - 16.2, high.floor, true}, // within it (hollowUnder): it wins
		{Radius - 5, 0, false},            // over both
		{Radius - 30, 0, false},           // under both
	} {
		f, _, ok := b.At(up.Scale(c.r))
		if ok != c.in || ok && f != c.floor {
			t.Errorf("at %.1f m: floor %.1f, in %v; want %.1f, %v", c.r-Radius, f-Radius, ok, c.floor-Radius, c.in)
		}
	}
	var none *Burrow
	if _, _, ok := none.At(up); ok {
		t.Error("no burrow holds something")
	}
}

// Ground is the relief's outside the hollows, their floor in them.
func TestPlacesGround(t *testing.T) {
	set := DefaultSettings()
	w := NewWorld(7, &set)
	up := Toward(vec3{1, 0, 0}, 200)
	ground := w.Relief.SurfaceRadius(up)
	pl := Places{Relief: w.Relief, Burrow: NewBurrow(slab{ground - 8, ground - 5})}
	if at, under := pl.Ground(up.Scale(ground + 3)); under || math.Abs(at.Length()-ground) > 1e-9 {
		t.Errorf("over the ground: %.2f m, under %v", at.Length()-ground, under)
	}
	if at, under := pl.Ground(up.Scale(ground - 6)); !under || math.Abs(at.Length()-(ground-8)) > 1e-9 {
		t.Errorf("in the hollow: %.2f m, under %v", at.Length()-ground, under)
	}
	if r := pl.GroundRadius(up.Scale(ground - 6)); r != ground-8 {
		t.Errorf("GroundRadius %.2f", r-ground)
	}
	if at, _ := (Places{Relief: w.Relief}).Ground(up.Scale(ground - 6)); math.Abs(at.Length()-ground) > 1e-9 {
		t.Error("no burrow: not the relief")
	}
}
