package base

import (
	"math"
	"naturalia/internal/engine/mesh"
)

// BuildShowcase: a display cabinet of the research lab, 1.2 m wide, its
// glass front toward +X: three shelves lit from above, Martian samples on
// them, rocks and crystals, each on its little plinth.
func BuildShowcase(b *building, at vec3, seed int) {
	const w, h, deep = 1.2, 1.9, 0.45
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	b.in("home.trim").Box(p(-0.02, 0.05, 0), vec3{deep - 0.06, 0.1, w - 0.06}, xAxis, 0.01)
	b.in("home.panel").Box(p(-deep/2+0.02, (h+0.1)/2, 0), vec3{0.04, h - 0.1, w}, xAxis, 0.01)
	for _, s := range []float64{-1, 1} {
		b.in("home.panel").Box(p(0, (h+0.1)/2, s*(w/2-0.02)), vec3{deep, h - 0.1, 0.04}, xAxis, 0.01)
	}
	b.in("home.panel").Box(p(0, 0.2, 0), vec3{deep, 0.2, w}, xAxis, 0.01)
	b.in("home.panel").Box(p(0, h-0.03, 0), vec3{deep, 0.06, w}, xAxis, 0.01)
	for j, y := range []float64{0.3, 0.83, 1.36} {
		if j > 0 {
			b.in("glass.dark").Box(p(0, y-0.01, 0), vec3{deep - 0.06, 0.02, w - 0.08}, xAxis, 0)
		}
		b.in("home.led").Box(p(deep/2-0.06, y+0.5, 0), vec3{0.02, 0.008, w - 0.1}, xAxis, 0)
		for k := range 3 {
			r := grnd(seed, j*3+k)
			at := p(-0.02, y, (float64(k)-1)*0.34)
			b.in("home.trim").Box(at.Add(vec3{0, 0.03, 0}), vec3{0.16, 0.06, 0.16}, xAxis, 0.008)
			top := at.Add(vec3{0, 0.06, 0})
			if (j+k+seed)%3 == 0 { // a cluster of crystals
				for c := range 4 {
					a := float64(c)*1.7 + r*3
					tilt := vec3{math.Cos(a) * 0.35, 1, math.Sin(a) * 0.35}.Norm()
					b.in("crystal.blue").Prism(top, tilt, 0.025, 0.004, 0.09+0.06*grnd(seed, c+j*5), 6, 0)
				}
				continue
			}
			rock := []string{"rock.red", "clay", "crate.sand"}[(j+k)%3]
			b.in(rock).Box(top.Add(vec3{0, 0.04, 0}), vec3{0.1 + 0.05*r, 0.08 + 0.04*r, 0.09}, vec3{math.Cos(r * 3), 0, math.Sin(r * 3)}, 0.02)
		}
	}
	// The glass, in a thin frame: seen through, from inside the room on
	// both sides (no room point).
	b.Part("glass.rail", mesh.LitGlass, vec3{}).Box(p(deep/2-0.01, (0.3+h-0.06)/2, 0), vec3{0.01, h - 0.36, w - 0.08}, xAxis, 0)
	b.in("home.trim").Box(p(deep/2, 0.31, 0), vec3{0.02, 0.02, w}, xAxis, 0)
}
