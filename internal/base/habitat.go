package base

import "math"

// The furniture of the dwelling set down anywhere (packs/core/camps.go).

// BuildKitchenette: a kitchen corner 2.2 m long (along Z), its back at -X
// against a wall: white fronts on a recessed plinth, a ceramic top with a
// sink and a hob, tiles lit warm under a shelf of jars and bowls.
func BuildKitchenette(b *building, at vec3, seed int) {
	const w, d, h = 2.2, 0.56, 0.9
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	b.in("home.trim").Box(p(-0.03, 0.05, 0), vec3{d - 0.08, 0.1, w - 0.04}, xAxis, 0)
	b.in("home.panel").Box(p(-0.01, 0.47, 0), vec3{d - 0.04, 0.74, w}, xAxis, 0.015)
	for i, z := range []float64{-0.825, -0.275, 0.275, 0.825} {
		if i == 2 { // three drawers
			for k := range 3 {
				b.in("home.panel2").Box(p(d/2-0.02, 0.22+float64(k)*0.245, z), vec3{0.02, 0.235, 0.54}, xAxis, 0.006)
			}
			continue
		}
		b.in("home.panel2").Box(p(d/2-0.02, 0.465, z), vec3{0.02, 0.725, 0.54}, xAxis, 0.006)
	}
	b.in("ceramic.sand").Box(p(0, h-0.03, 0), vec3{d + 0.02, 0.06, w + 0.04}, xAxis, 0.012)
	// The sink and its tap; the hob, two rings.
	b.in("home.trim").Box(p(0.02, h+0.004, -0.65), vec3{0.34, 0.016, 0.46}, xAxis, 0.004)
	b.in("steel").Prism(p(-0.2, h, -0.65), yUp, 0.018, 0.018, 0.28, 10, 0)
	b.in("steel").Box(p(-0.12, h+0.27, -0.65), vec3{0.18, 0.025, 0.03}, xAxis, 0.008)
	b.in("home.trim").Box(p(0.0, h+0.005, 0.6), vec3{0.42, 0.018, 0.6}, xAxis, 0.004)
	for _, z := range []float64{0.45, 0.75} {
		b.in("home.led.amber").Prism(p(0, h+0.014, z), yUp, 0.1, 0.1, 0.008, 24, 0)
		b.in("home.trim").Prism(p(0, h+0.014, z), yUp, 0.075, 0.075, 0.014, 24, 0)
	}
	// A pot on the hob, a bowl of fruit.
	b.in("ceramic.dark").Prism(p(0, h+0.028, 0.45), yUp, 0.11, 0.12, 0.13, 16, 0.01)
	b.in("ceramic").Prism(p(0.02, h, -0.05), yUp, 0.07, 0.14, 0.08, 16, 0.01)
	for k := range 4 {
		t := float64(k) * math.Pi / 2
		b.in([]string{"bloom.coral", "bloom.yellow"}[k%2]).Prism(p(0.02+0.055*math.Cos(t), h+0.07, -0.05+0.055*math.Sin(t)), yUp, 0.045, 0.02, 0.07, 8, 0.01)
	}
	// The splash of tiles, standing on the top, and the shelf over it.
	b.in("home.trim").Box(p(-d/2+0.015, h+0.35, 0), vec3{0.03, 0.7, w}, xAxis, 0)
	const cols, rows = 11, 4
	for i := range cols {
		for j := range rows {
			tok := []string{"ceramic.sand", "ceramic", "ceramic.sand", "home.panel2"}[int(grnd(seed+i*7, j)*4)%4]
			b.in(tok).Box(p(-d/2+0.04, h+0.09+float64(j)*0.16, -w/2+0.1+float64(i)*(w-0.2)/(cols-1)), vec3{0.02, 0.148, 0.185}, xAxis, 0.004)
		}
	}
	b.in("home.panel").Box(p(-d/2+0.15, h+0.75, 0), vec3{0.3, 0.05, w}, xAxis, 0.012)
	b.in("home.led").Box(p(-d/2+0.22, h+0.718, 0), vec3{0.03, 0.01, w - 0.2}, xAxis, 0)
	for k := range 7 {
		z := -0.9 + float64(k)*0.3
		r := grnd(seed+3, k)
		switch {
		case r < 0.3:
			b.in("clay").Prism(p(-d/2+0.14, h+0.775, z), yUp, 0.06, 0.08, 0.1, 12, 0.01)
			b.in("planter.green").Prism(p(-d/2+0.14, h+0.875, z), yUp, 0.1, 0.05, 0.16, 7, 0.02)
		case r < 0.65:
			b.in([]string{"crate.sand", "fabric.rust", "crate.slate"}[k%3]).Prism(p(-d/2+0.14, h+0.775, z), yUp, 0.055, 0.055, 0.17, 14, 0.01)
			b.in("wood.light").Prism(p(-d/2+0.14, h+0.945, z), yUp, 0.05, 0.05, 0.025, 14, 0)
		default:
			b.in("ceramic").Prism(p(-d/2+0.14, h+0.775, z), yUp, 0.07, 0.11, 0.07, 16, 0.01)
		}
	}
}

// KitchenetteLight shines from under its shelf.
var KitchenetteLight = vec3{0.1, 1.55, 0}

// BuildMealTable: a round table for three on a ceramic foot, three stools
// around, plates and cups, a small lamp in its middle.
func BuildMealTable(b *building, at vec3, seed int) {
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	b.in("ceramic.dark").Prism(p(0, 0, 0), yUp, 0.3, 0.22, 0.05, 28, 0.01)
	b.in("home.trim").Prism(p(0, 0.05, 0), yUp, 0.07, 0.07, 0.65, 16, 0)
	b.in("wood.light").Prism(p(0, 0.7, 0), yUp, 0.58, 0.6, 0.05, 40, 0.015)
	for k := range 3 {
		t := float64(k)*2*math.Pi/3 + math.Pi/2
		out := vec3{math.Cos(t), 0, math.Sin(t)}
		s := at.Add(out.Scale(mealStools))
		b.in("home.trim").Prism(s, yUp, 0.15, 0.04, 0.03, 20, 0.005)
		b.in("home.trim").Prism(s.Add(vec3{0, 0.03, 0}), yUp, 0.035, 0.035, 0.36, 12, 0)
		b.in([]string{"fabric.rust", "fabric.teal", "fabric.ochre"}[(k+seed)%3]).Prism(s.Add(vec3{0, 0.39, 0}), yUp, 0.19, 0.19, 0.08, 24, 0.03)
		b.in("home.panel2").Prism(at.Add(out.Scale(0.38)).Add(vec3{0, 0.75, 0}), yUp, 0.13, 0.15, 0.015, 24, 0)
		side := vec3{-out.Z, 0, out.X}
		b.in("ceramic").Prism(at.Add(out.Scale(0.3)).Add(side.Scale(0.22)).Add(vec3{0, 0.75, 0}), yUp, 0.035, 0.04, 0.09, 14, 0.005)
	}
	b.in("ceramic.sand").Prism(p(0, 0.75, 0), yUp, 0.08, 0.06, 0.05, 16, 0.01)
	b.in("lamp.shade").Prism(p(0, 0.8, 0), yUp, 0.075, 0.06, 0.16, 16, 0.02)
}

const mealStools = 0.8

// MealObstacles are its stools.
var MealObstacles = []vec3{{0, 0, mealStools}, {mealStools * math.Cos(math.Pi/2+2*math.Pi/3), 0, mealStools * math.Sin(math.Pi/2+2*math.Pi/3)},
	{mealStools * math.Cos(math.Pi/2+4*math.Pi/3), 0, mealStools * math.Sin(math.Pi/2+4*math.Pi/3)}}

// MealLight is the lamp on the table.
var MealLight = vec3{0, 1.05, 0}
