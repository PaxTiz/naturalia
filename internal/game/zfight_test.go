package game

import (
	"naturalia/internal/base"
	"naturalia/internal/mesh"
	"testing"
)

// TestBaseZFighting checks that the base's surfaces lying on top of each
// other are on different layers (mesh.Unfight), the one in front higher:
// otherwise they would flicker as the camera moves.
func TestBaseZFighting(t *testing.T) {
	c := LoadContent()
	checkFights(t, "the base", base.Build(&c.Kits))
	// What grows is built apart, stage by stage.
	for id, n := range base.Ents.Grows.All() {
		for s := range *n {
			checkFights(t, base.Ents.Furniture.Get(id).Kit, base.BuildGrowth(&c.Kits, id, s))
		}
	}
	// The bootcamps, laid out apart.
	for _, def := range c.Entities.All() {
		if def.Camp != nil {
			checkFights(t, def.ID, base.LayCamp(&c.Kits, *def.Camp, def.Ports).Parts)
		}
	}
}

// checkFights checks the surfaces of parts lying on top of each other.
func checkFights(t *testing.T, what string, parts *mesh.Parts) {
	t.Helper()
	tris, fights := parts.Fights()
	if len(fights) == 0 && what == "the base" {
		t.Fatal("no surfaces on top of each other: the test sees nothing")
	}
	first := map[int]int{} // a triangle of each piece
	for i, s := range tris {
		if _, ok := first[s.Piece]; !ok {
			first[s.Piece] = i
		}
	}
	bad := 0
	for _, f := range fights {
		under := f.A
		if f.Top == f.A {
			under = f.B
		}
		top, low := tris[first[f.Top]], tris[first[under]]
		if top.Layer > low.Layer || f.Free && top.Layer != low.Layer {
			continue
		}
		if bad++; bad <= 10 {
			t.Logf("%s: %s (layer %d) on %s (layer %d), near %.2f (base coordinates)",
				what, parts.Keys[top.Mesh].Tok, top.Layer, parts.Keys[low.Mesh].Tok, low.Layer, top.P[0])
		}
	}
	if bad > 0 {
		t.Errorf("%s: %d of %d pairs of surfaces on top of each other would fight", what, bad, len(fights))
	}
}
