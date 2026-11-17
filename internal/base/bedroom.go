package base

import (
	"math"
	"naturalia/internal/mesh"
)

// The bedroom wing: the one room of the base made to be lived in, and the
// one the player sees closest. Its kits are finer than the rest of the
// furniture: a double bed, nightstands and a bedside lamp, a desk, a
// shelf of books, an armchair, lockers, a rug, a garland, and a few
// personal things (a photo of the Earth, books, a guitar). Each faces +X,
// standing on the floor at its spot, unless said otherwise.

// in is a builder of the room's furniture, lit by its lamps.
func (b *building) in(tok string) *mesh.Builder { return b.Part(tok, mesh.LitIndoor, vec3{}) }

var xAxis = vec3{1, 0, 0}

// BuildBed: a double bed, its head toward -X against the wall: a wooden
// frame on short legs, a padded headboard, the mattress, two pillows, the
// duvet folded back and a throw across its foot.
func BuildBed(b *building, at vec3, _ int) {
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	wood, dark := b.in("wood.light"), b.in("wood.brown")
	wood.Box(p(0.03, 0.24, 0), vec3{2.08, 0.2, 1.56}, xAxis, 0.03)
	for _, x := range []float64{-0.95, 1.0} {
		for _, z := range []float64{-0.7, 0.7} {
			dark.Box(p(x, 0.07, z), vec3{0.08, 0.14, 0.08}, xAxis, 0.01)
		}
	}
	wood.Box(p(-1.04, 0.6, 0), vec3{0.08, 1.2, 1.62}, xAxis, 0.03)
	for _, z := range []float64{-0.39, 0.39} {
		b.in("fabric.teal").Box(p(-0.98, 0.82, z), vec3{0.06, 0.5, 0.74}, xAxis, 0.03)
	}
	b.in("fabric.white").Box(p(0.05, 0.43, 0), vec3{1.98, 0.18, 1.48}, xAxis, 0.05)
	// The pillows lean on the headboard.
	t := 25 * math.Pi / 180
	tilt := [3]vec3{{math.Cos(t), -math.Sin(t), 0}, {math.Sin(t), math.Cos(t), 0}, {0, 0, 1}}
	for _, z := range []float64{-0.37, 0.37} {
		b.in("fabric.white").BoxBasis(p(-0.76, 0.62, z), tilt, vec3{0.44, 0.15, 0.64}, 0.06)
	}
	duvet := b.in("fabric.duvet")
	duvet.Box(p(0.36, 0.56, 0), vec3{1.36, 0.08, 1.54}, xAxis, 0.035)
	for _, s := range []float64{-1, 1} {
		duvet.Box(p(0.36, 0.43, s*0.77), vec3{1.36, 0.3, 0.04}, xAxis, 0.01)
	}
	duvet.Box(p(1.06, 0.43, 0), vec3{0.04, 0.3, 1.58}, xAxis, 0.01)
	duvet.Prism(p(-0.31, 0.58, -0.78), vec3{0, 0, 1}, 0.055, 0.055, 1.56, 10, 0)
	wool := b.in("fabric.wool")
	wool.Box(p(0.8, 0.615, 0), vec3{0.42, 0.03, 1.6}, xAxis, 0.01)
	wool.Box(p(1.095, 0.49, 0), vec3{0.03, 0.27, 1.6}, xAxis, 0.01)
}

// BuildNightstand: a small wooden cabinet, two drawers with knobs.
func BuildNightstand(b *building, at vec3, _ int) {
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	wood, dark := b.in("wood.light"), b.in("wood.brown")
	wood.Box(p(0, 0.3, 0), vec3{0.44, 0.46, 0.42}, xAxis, 0.02)
	wood.Box(p(0, 0.545, 0), vec3{0.48, 0.03, 0.46}, xAxis, 0.01)
	for _, x := range []float64{-0.18, 0.18} {
		for _, z := range []float64{-0.17, 0.17} {
			dark.Box(p(x, 0.035, z), vec3{0.04, 0.07, 0.04}, xAxis, 0.005)
		}
	}
	for _, y := range []float64{0.19, 0.41} {
		dark.Box(p(0.23, y, 0), vec3{0.02, 0.19, 0.38}, xAxis, 0.005)
		b.in("steel").Prism(p(0.24, y, 0), xAxis, 0.016, 0.014, 0.025, 8, 0)
	}
}

// BuildBedLamp: a small lamp to set on furniture, its fabric shade lit
// from inside.
func BuildBedLamp(b *building, at vec3, _ int) {
	b.in("steel.dark").Prism(at, yUp, 0.075, 0.065, 0.03, 16, 0.005)
	b.in("steel").Prism(at.Add(vec3{0, 0.03, 0}), yUp, 0.012, 0.012, 0.2, 8, 0)
	b.in("lamp.shade").Prism(at.Add(vec3{0, 0.19, 0}), yUp, 0.14, 0.09, 0.17, 20, 0)
	b.in("lamp.amber").Prism(at.Add(vec3{0, 0.2, 0}), yUp, 0.05, 0.05, 0.03, 10, 0)
}

// BedLampLight is where a bedside lamp's light comes from, over its spot.
var BedLampLight = vec3{0, 0.3, 0}

// BuildWritingDesk: a wooden desk, drawers on its left (-Z), the user's
// side toward +X: an open laptop, an articulated lamp at the back right,
// room at the left end for a pot.
func BuildWritingDesk(b *building, at vec3, _ int) {
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	wood, dark, steel := b.in("wood.light"), b.in("wood.brown"), b.in("steel")
	wood.Box(p(0, 0.74, 0), vec3{0.62, 0.04, 1.24}, xAxis, 0.012)
	wood.Box(p(0, 0.36, -0.42), vec3{0.56, 0.72, 0.36}, xAxis, 0.015)
	for _, y := range []float64{0.14, 0.38, 0.6} {
		dark.Box(p(0.29, y, -0.42), vec3{0.02, 0.2, 0.32}, xAxis, 0.005)
		steel.Box(p(0.305, y+0.05, -0.42), vec3{0.015, 0.02, 0.12}, xAxis, 0.004)
	}
	for _, x := range []float64{-0.26, 0.26} {
		b.in("steel.dark").Box(p(x, 0.36, 0.56), vec3{0.04, 0.72, 0.04}, xAxis, 0.005)
	}
	wood.Box(p(-0.27, 0.45, 0.08), vec3{0.02, 0.5, 0.96}, xAxis, 0.005)
	// The laptop, its screen up at the back.
	b.in("steel.dark").Box(p(0.08, 0.768, -0.05), vec3{0.24, 0.016, 0.34}, xAxis, 0.004)
	b.in("steel.dark").Box(p(-0.05, 0.89, -0.05), vec3{0.015, 0.22, 0.33}, xAxis, 0.004)
	b.in("screen.dim").Box(p(-0.0405, 0.89, -0.05), vec3{0.004, 0.19, 0.3}, xAxis, 0)
	// The lamp: a foot, two arms, a head looking down at the desk.
	foot, elbow, head := p(-0.2, 0.76, 0.45), p(-0.12, 1.08, 0.42), p(0.06, 1.12, 0.32)
	dark = b.in("steel.dark")
	dark.Prism(foot, yUp, 0.065, 0.06, 0.025, 14, 0.004)
	for _, arm := range [][2]vec3{{foot, elbow}, {elbow, head}} {
		d := arm[1].Sub(arm[0])
		dark.Prism(arm[0], d.Norm(), 0.012, 0.012, d.Length(), 6, 0)
	}
	dark.Prism(elbow.Sub(vec3{0, 0, 0.025}), vec3{0, 0, 1}, 0.022, 0.022, 0.05, 10, 0)
	b.in("pipe.yellow").Prism(head.Add(vec3{0, 0.04, 0}), vec3{0, -1, 0}, 0.03, 0.09, 0.12, 14, 0)
	b.in("lamp.amber").Prism(head.Sub(vec3{0, 0.085, 0}), yUp, 0.07, 0.07, 0.01, 12, 0)
}

// DeskLight is where a writing desk's lamp lights from, over its spot.
var DeskLight = vec3{0.06, 0.98, 0.32}

// BuildShelf: a tall open bookcase 1 m wide (along Z), four shelves of
// books, a rock and a box, its back against the wall at -X.
func BuildShelf(b *building, at vec3, seed int) {
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	wood := b.in("wood.light")
	const w, d, h = 1.0, 0.32, 1.9
	for _, s := range []float64{-1, 1} {
		wood.Box(p(0, h/2, s*(w/2-0.015)), vec3{d, h, 0.03}, xAxis, 0.008)
	}
	b.in("wood.brown").Box(p(-d/2+0.01, h/2, 0), vec3{0.02, h, w - 0.06}, xAxis, 0)
	shelves := []float64{0.05, 0.5, 0.95, 1.4, h - 0.015}
	for _, y := range shelves {
		wood.Box(p(0.005, y, 0), vec3{d - 0.01, 0.03, w - 0.06}, xAxis, 0.005)
	}
	books := []string{"pipe.red", "fabric.teal", "crate.sand", "pipe.blue", "fabric.duvet", "paper", "wood.brown"}
	for k, y := range shelves[:4] {
		if k == 2 {
			// A Martian rock and a box on this one.
			b.in("rock.red").Prism(p(0.02, y+0.015, -0.25), yUp, 0.09, 0.05, 0.1, 6, 0.02)
			b.in("crate.slate").Box(p(0.0, y+0.075, 0.2), vec3{0.24, 0.12, 0.3}, xAxis, 0.01)
			continue
		}
		z := -w/2 + 0.06
		end := w/2 - 0.06
		if k == 1 {
			end = 0.05 // room left for the plant's leaves to come in
		}
		for i := 0; z < end; i++ {
			r := grnd(seed+k*13, i)
			thick, tall := 0.03+0.03*r, 0.24+0.14*grnd(seed+k*7, i)
			if z+thick > end {
				break
			}
			b.in(books[(i*3+k+seed)%len(books)]).Box(p(0.0, y+0.015+tall/2, z+thick/2), vec3{0.2 + 0.04*r, tall, thick}, xAxis, 0.004)
			z += thick + 0.004
		}
		// A last one leaning on the others.
		if k == 0 {
			a := 0.35
			b.in("pipe.blue").BoxBasis(p(0, y+0.015+0.15*math.Cos(a), z+0.015+0.15*math.Sin(a)),
				[3]vec3{xAxis, {0, math.Cos(a), -math.Sin(a)}, {0, math.Sin(a), math.Cos(a)}}, vec3{0.21, 0.3, 0.03}, 0.004)
		}
	}
}

// BuildArmchair: a deep fabric armchair on wooden feet, a cushion and a
// throw over one arm.
func BuildArmchair(b *building, at vec3, _ int) {
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	cloth := b.in("fabric.teal")
	for _, x := range []float64{-0.32, 0.32} {
		for _, z := range []float64{-0.34, 0.34} {
			b.in("wood.brown").Prism(p(x, 0, z), yUp, 0.03, 0.022, 0.12, 8, 0)
		}
	}
	cloth.Box(p(0, 0.25, 0), vec3{0.8, 0.26, 0.84}, xAxis, 0.05)
	b.in("fabric.wool").Box(p(0.06, 0.43, 0), vec3{0.62, 0.12, 0.58}, xAxis, 0.05)
	for _, s := range []float64{-1, 1} {
		cloth.Box(p(0, 0.5, s*0.36), vec3{0.8, 0.28, 0.14}, xAxis, 0.06)
	}
	t := 12 * math.Pi / 180
	back := [3]vec3{{math.Cos(t), math.Sin(t), 0}, {-math.Sin(t), math.Cos(t), 0}, {0, 0, 1}}
	cloth.BoxBasis(p(-0.33, 0.68, 0), back, vec3{0.16, 0.62, 0.84}, 0.06)
	b.in("fabric.duvet").BoxBasis(p(-0.18, 0.62, 0.08), back, vec3{0.12, 0.34, 0.36}, 0.05)
	// The throw, folded over the left arm.
	b.in("fabric.wool").Box(p(0.05, 0.645, 0.36), vec3{0.42, 0.02, 0.16}, xAxis, 0.008)
	b.in("fabric.wool").Box(p(0.05, 0.52, 0.445), vec3{0.42, 0.26, 0.02}, xAxis, 0.008)
}

// BuildLockers: two tall lockers side by side along Z, their doors
// toward +X: vents, handles, a name tag.
func BuildLockers(b *building, at vec3, seed int) {
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	const h = 1.9
	for k, z := range []float64{-0.23, 0.23} {
		tok := "locker"
		if (k+seed)%2 == 1 {
			tok = "locker.ochre"
		}
		b.in("steel.dark").Box(p(0, h/2, z), vec3{0.48, h, 0.45}, xAxis, 0.01)
		b.in(tok).Box(p(0.245, h/2+0.03, z), vec3{0.02, h - 0.12, 0.42}, xAxis, 0.008)
		for i := range 4 {
			b.in("steel.dark").Box(p(0.26, h-0.25-float64(i)*0.05, z), vec3{0.01, 0.018, 0.26}, xAxis, 0)
		}
		b.in("steel").Box(p(0.265, 1.05, z-0.14), vec3{0.02, 0.16, 0.03}, xAxis, 0.005)
		b.in("paper").Box(p(0.257, 1.45, z), vec3{0.004, 0.06, 0.16}, xAxis, 0)
	}
	b.in("rubber").Box(p(0, 0.03, 0), vec3{0.5, 0.06, 0.92}, xAxis, 0.005)
}

// BuildRug: a woven rug 2.6 × 1.7 m, a light border around a red field.
func BuildRug(b *building, at vec3, _ int) {
	c := at.Add(vec3{0, FloorTop + 0.006, 0})
	b.in("fabric.wool").Box(c, vec3{2.6, 0.012, 1.7}, xAxis, 0)
	b.in("fabric.rug").Box(c, vec3{2.3, 0.012, 1.4}, xAxis, 0)
	b.in("fabric.duvet").Box(c, vec3{1.5, 0.012, 0.12}, xAxis, 0)
}

// BuildGarland: a string of amber bulbs sagging between two hooks along
// X, hung at its spot: seed is its length in decimeters, 4 m if 0.
func BuildGarland(b *building, at vec3, seed int) {
	l := garlandLength(seed)
	const sag = 0.2
	pt := func(t float64) vec3 {
		u := 2*t - 1
		return at.Add(vec3{(t - 0.5) * l, -sag * (1 - u*u), 0})
	}
	wire, bulb := b.in("rubber"), b.in("neon.amber")
	n := max(2, int(math.Round(l/0.12)))
	for i := range n {
		a, e := pt(float64(i)/float64(n)), pt(float64(i+1)/float64(n))
		d := e.Sub(a)
		wire.Prism(a, d.Norm(), 0.006, 0.006, d.Length(), 5, 0)
	}
	for _, t := range []float64{0, 1} {
		b.in("steel").Prism(pt(t).Sub(vec3{0, 0.02, 0}), yUp, 0.015, 0.015, 0.05, 6, 0)
	}
	bulbs := max(2, int(math.Round(l/0.3)))
	for i := 1; i < bulbs; i++ {
		c := pt(float64(i) / float64(bulbs))
		bulb.Prism(c.Sub(vec3{0, 0.07, 0}), yUp, 0.022, 0.03, 0.06, 8, 0.01)
		wire.Prism(c.Sub(vec3{0, 0.012, 0}), yUp, 0.012, 0.012, 0.014, 6, 0)
	}
}

func garlandLength(seed int) float64 {
	if seed <= 0 {
		return 4
	}
	return float64(seed) / 10
}

// GarlandLight is where a garland lights from, under the middle of its
// string.
var GarlandLight = vec3{0, -0.3, 0}

// BuildGuitar: an acoustic guitar standing on its end, leaning back
// against the wall at -X, its strings toward +X.
func BuildGuitar(b *building, at vec3, _ int) {
	t := 15 * math.Pi / 180
	up := vec3{-math.Sin(t), math.Cos(t), 0} // along the neck
	face := vec3{math.Cos(t), math.Sin(t), 0}
	z := vec3{0, 0, 1}
	pt := func(s, f float64) vec3 { return at.Add(up.Scale(s)).Add(face.Scale(f)) }
	axes := [3]vec3{face, up, z}
	wood, dark := b.in("wood.light"), b.in("wood.brown")
	wood.Prism(pt(0.2, -0.045), face, 0.19, 0.19, 0.09, 24, 0.012)
	wood.Prism(pt(0.47, -0.045), face, 0.145, 0.145, 0.09, 24, 0.012)
	dark.Prism(pt(0.2, -0.05), face, 0.195, 0.195, 0.1, 24, 0) // the sides' band
	b.in("rubber").Prism(pt(0.4, 0.04), face, 0.05, 0.05, 0.01, 16, 0)
	dark.BoxBasis(pt(0.16, 0.05), axes, vec3{0.012, 0.03, 0.14}, 0.003)
	dark.BoxBasis(pt(0.86, 0.02), axes, vec3{0.04, 0.56, 0.055}, 0.005)
	dark.BoxBasis(pt(1.22, 0.02), axes, vec3{0.03, 0.18, 0.085}, 0.008)
	for _, s := range []float64{1.17, 1.22, 1.27} {
		for _, side := range []float64{-1, 1} {
			b.in("steel").Prism(pt(s, 0.02).Add(z.Scale(side*0.04)), z.Scale(side), 0.008, 0.008, 0.025, 6, 0)
		}
	}
	for _, w := range []float64{-0.012, 0, 0.012} {
		b.in("steel").BoxBasis(pt(0.68, 0.047).Add(z.Scale(w)), axes, vec3{0.003, 1.02, 0.003}, 0)
	}
}

// BuildPhotoEarth: a little standing frame, the Earth from space in it,
// a gift from home.
func BuildPhotoEarth(b *building, at vec3, _ int) {
	t := 12 * math.Pi / 180
	face := vec3{math.Cos(t), math.Sin(t), 0}
	up := vec3{-math.Sin(t), math.Cos(t), 0}
	z := vec3{0, 0, 1}
	c := at.Add(vec3{0, 0.105, 0})
	b.in("wood.brown").BoxBasis(c, [3]vec3{face, up, z}, vec3{0.02, 0.21, 0.17}, 0.004)
	b.in("steel.dark").BoxBasis(c.Add(face.Scale(0.009)), [3]vec3{face, up, z}, vec3{0.006, 0.17, 0.13}, 0)
	b.in("wood.brown").Box(at.Add(vec3{-0.06, 0.08, 0}), vec3{0.012, 0.16, 0.03}, vec3{0.3, 0, 1}, 0)
	b.in("earth.blue").Prism(c.Add(face.Scale(0.008)), face, 0.05, 0.05, 0.006, 20, 0)
	for _, l := range [][3]float64{{0.015, -0.012, 0.02}, {-0.018, 0.02, 0.014}, {0.022, 0.02, 0.01}} {
		b.in("earth.green").Prism(c.Add(face.Scale(0.011)).Add(up.Scale(l[0])).Add(z.Scale(l[1])), face, l[2], l[2]*0.8, 0.006, 7, 0)
	}
	b.in("paper").BoxBasis(c.Add(face.Scale(0.016)).Add(up.Scale(-0.025)), [3]vec3{face, up, z}, vec3{0.003, 0.008, 0.06}, 0)
}

// BuildBooks: a pile of books lying, three to five by the seed.
func BuildBooks(b *building, at vec3, seed int) {
	books := []string{"pipe.red", "fabric.teal", "crate.sand", "pipe.blue", "fabric.duvet", "wood.brown"}
	y := 0.0
	for i := range 3 + seed%3 {
		r := grnd(seed+5, i)
		thick := 0.03 + 0.025*r
		a := (grnd(seed+9, i) - 0.5) * 0.6
		along := vec3{math.Cos(a), 0, math.Sin(a)}
		c := at.Add(vec3{0, y + thick/2, 0})
		b.in(books[(i+seed)%len(books)]).Box(c, vec3{0.24 - 0.04*r, thick, 0.17 - 0.02*r}, along, 0.004)
		// The pages show along the fore-edge.
		b.in("paper").Box(c.Add(vec3{-along.Z, 0, along.X}.Scale(0.016)), vec3{0.22 - 0.04*r, thick - 0.01, 0.15 - 0.02*r}, along, 0)
		y += thick
	}
}

// The plant on the desk grows a stage each night slept (content.Kit's
// Grow): a sprout, a seedling, a young plant, a bush, then a plant that
// spills over its pot and climbs: a vine runs 0.6 m along +Z, then up
// 1.1 m and along the top of what stands there (the shelf, in the
// bedroom).

// PlantStages is how many stages the plant goes through.
const PlantStages = 5

// BuildPlantPot: a clay pot with a rim and a saucer, earth in it.
func BuildPlantPot(b *building, at vec3, _ int) {
	b.in("clay").Prism(at, yUp, 0.1, 0.1, 0.012, 16, 0.003)
	b.in("clay").Prism(at.Add(vec3{0, 0.012, 0}), yUp, 0.065, 0.085, 0.12, 16, 0.004)
	b.in("clay").Prism(at.Add(vec3{0, 0.12, 0}), yUp, 0.095, 0.095, 0.035, 16, 0.006)
	b.in("soil").Prism(at.Add(vec3{0, 0.13, 0}), yUp, 0.083, 0.083, 0.02, 16, 0)
}

// plantTop is the earth in the pot, over its spot.
const plantTop = 0.15

// BuildPlantStage: what has grown of the plant at a stage, from 0.
func BuildPlantStage(b *building, at vec3, stage int) {
	stage = min(max(stage, 0), PlantStages-1)
	height := []float64{0.035, 0.09, 0.17, 0.25, 0.3}[stage]
	leaves := []int{2, 4, 7, 12, 18}[stage]
	size := []float64{0.025, 0.045, 0.065, 0.085, 0.095}[stage]
	base := at.Add(vec3{0, plantTop - 0.01, 0})
	stem := b.in("planter.green")
	stem.Prism(base, yUp, 0.006+0.002*float64(stage), 0.004, height, 6, 0)
	for i := range leaves {
		f := float64(i) / float64(max(1, leaves-1))
		a := float64(i)*2.4 + 0.3
		out := vec3{math.Cos(a), 0, math.Sin(a)}
		// The low ones of a grown plant hang over the rim.
		lift := 1.2 - 0.9*f
		if stage >= 3 && i%3 == 0 && f < 0.6 {
			lift = -0.5 - 0.3*grnd(stage, i)
		}
		from := base.Add(vec3{0, height * (0.25 + 0.75*(1-f)), 0})
		if stage == 0 {
			from = base.Add(vec3{0, height, 0})
			lift = 0.5
		}
		tok := "planter.green"
		if i%3 == 1 {
			tok = "planter.lime"
		}
		l := size * (0.75 + 0.4*grnd(stage+3, i))
		if lift < 0 {
			l *= 1.5
			from = from.Add(out.Scale(0.06)) // from the rim
		}
		leaf(b.in(tok), from, out.Add(vec3{0, lift, 0}).Norm(), l)
	}
	if stage < 3 {
		return
	}
	// The vine, toward the shelf and up it: started at the fourth stage,
	// there at the fifth.
	path := []vec3{{0, 0.2, 0.03}, {-0.02, 0.28, 0.6}, {-0.02, 0.75, 0.63}, {-0.02, 1.17, 0.63}, {0.1, 1.165, 1.15}}
	if stage == 3 {
		path = []vec3{{0, 0.2, 0.03}, {-0.01, 0.25, 0.3}}
	}
	vine := b.in("planter.green")
	k := 0
	for i := 0; i+1 < len(path); i++ {
		p0, p1 := at.Add(path[i]), at.Add(path[i+1])
		d := p1.Sub(p0)
		vine.Prism(p0, d.Norm(), 0.006, 0.005, d.Length()+0.005, 5, 0)
		n := max(1, int(d.Length()/0.05))
		for j := range n {
			p := p0.Add(d.Scale((float64(j) + 0.5) / float64(n)))
			side := vec3{0, 0, 1} // up the shelf's edge: to both sides of it
			if math.Hypot(d.X, d.Z) > 0.01 {
				side = vec3{-d.Z, 0, d.X}.Norm()
			}
			s := float64(k%2*2 - 1)
			tok := "planter.green"
			if k%3 == 2 {
				tok = "planter.lime"
			}
			leaf(b.in(tok), p, side.Scale(s).Add(vec3{0, 0.5, 0}).Add(xAxis.Scale(-0.4)).Norm(), 0.06+0.03*grnd(9, k))
			k++
		}
	}
}

// leaf lays a flat pointed leaf from p along dir, l long: a thin diamond
// on a short stalk.
func leaf(m *mesh.Builder, p, dir vec3, l float64) {
	across := dir.Cross(yUp)
	if across.Length() < 0.01 {
		across = xAxis
	}
	across = across.Norm()
	n := across.Cross(dir).Norm()
	m.Prism(p, dir, 0.003, 0.003, l*0.25, 4, 0)
	// A square turned a quarter on its corner, stretched along dir.
	u, v := dir.Add(across.Scale(0.6)).Norm(), dir.Sub(across.Scale(0.6)).Norm()
	m.BoxBasis(p.Add(dir.Scale(l*0.6)), [3]vec3{u, v.Sub(u.Scale(v.Dot(u))).Norm(), n}, vec3{l * 0.5, l * 0.5, 0.004}, 0.001)
}
