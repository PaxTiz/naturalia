package game

import (
	"math"
	"testing"
)

// Out on the ground, in a building, down a tunnel's gallery: each its
// space, and what goes with it.
func TestSpaces(t *testing.T) {
	g, a, _, l := tunnelFrom(t, "core:carrefour", "core:serre", vec3{60, 0, 0}, math.Pi, 0, 0, 0)

	out := g.SpaceAt(g.World.Relief.Surface(a.ToWorld(vec3{-25, 0, 0})).Add(a.Site.Y.Scale(1.7)))
	if out.Kind != SpaceOut || !out.Open() || out.Indoors || out.Deep || !out.Wind() || out.Echo() || out.Haze() != 1 {
		t.Errorf("out on the ground: %+v", out)
	}
	if out.Light(0.2) != 0.2 {
		t.Errorf("out on the ground, the sun at 0.2 lights as %.2f", out.Light(0.2))
	}

	in := g.SpaceAt(a.ToWorld(a.Layout.Room.Center.Add(vec3{0, 1.7, 0})))
	if in.Kind != SpaceIndoors || !in.Open() || !in.Indoors || in.Wind() || in.Echo() {
		t.Errorf("in a building: %+v", in)
	}

	// Halfway along the gallery, the eye over its floor.
	mid := len(l.Pts) / 2
	eye := l.Site.ToWorld(l.Pts[mid])
	eye = eye.Add(eye.Norm().Scale(1.7))
	gal := g.SpaceAt(eye)
	if gal.Kind != SpaceGallery || gal.Hollow != l || gal.Open() || !gal.Deep || !gal.Echo() || gal.Wind() {
		t.Errorf("in the gallery: %+v", gal)
	}
	if gal.Depth != 1 || gal.Haze() != 0 || gal.Light(-0.5) != 1 {
		t.Errorf("deep in the gallery: depth %.2f, haze %.2f, light at night %.2f", gal.Depth, gal.Haze(), gal.Light(-0.5))
	}
}
