package base

import (
	"math"
	"naturalia/internal/engine/mesh"
)

// Kit pieces for the outside of the buildings: what stands on their roofs
// and around them, lit by the sun. Placed like furniture (a blueprint's
// Props), outside the walls: At.Y is the roof's height there.

// out is a builder lit by the sun.
func (b *building) out(tok string) *mesh.Builder { return b.Part(tok, mesh.LitSun, vec3{}) }

// BuildSolarRack: three solar panels side by side along Z on a steel
// frame, tilted 25° to face -X, on legs standing at the spot.
func BuildSolarRack(b *building, at vec3, _ int) {
	const tilt = 25 * math.Pi / 180
	up := vec3{-math.Sin(tilt), math.Cos(tilt), 0} // the panels' face
	slope := vec3{math.Cos(tilt), math.Sin(tilt), 0}
	ax := [3]vec3{slope, up, {0, 0, 1}}
	mid := at.Add(vec3{0, 0.75, 0})
	b.out("steel").BoxBasis(mid.Sub(up.Scale(0.06)), ax, vec3{1.75, 0.06, 5.3}, 0.01)
	for k := -1; k <= 1; k++ {
		z := float64(k) * 1.75
		b.out("solar.blue").BoxBasis(mid.Add(vec3{0, 0, z}), ax, vec3{1.6, 0.04, 1.65}, 0.01)
		for _, s := range []float64{-1, 1} { // a leg at each end, the high one taller
			foot := at.Add(vec3{s * 0.7, -0.1, z})
			h := 0.75 + s*0.7*math.Tan(tilt) + 0.05
			b.out("steel.dark").Box(foot.Add(vec3{0, h / 2, 0}), vec3{0.06, h, 0.06}, xAxis, 0)
		}
	}
}

// BuildRoofVent: an extractor on a roof, a white box with a dark louvred
// face toward +X and a steel cap.
func BuildRoofVent(b *building, at vec3, _ int) {
	b.out("habitat.white2").Box(at.Add(vec3{0, 0.45, 0}), vec3{1.1, 0.9, 1.4}, xAxis, 0.04)
	b.out("steel").Box(at.Add(vec3{0, 0.95, 0}), vec3{1.3, 0.1, 1.6}, xAxis, 0.02)
	for k := range 4 {
		b.out("steel.dark").Box(at.Add(vec3{0.56, 0.2 + float64(k)*0.17, 0}), vec3{0.04, 0.08, 1.1}, xAxis, 0)
	}
}

// BuildAnnex: the life support's technical annex, a low closed box in the
// working buildings' steel grey, standing on the spot: a door with a
// hazard frame on its -Z face, louvres on its sides, thermal radiators on
// its roof, a purge stack at a corner.
func BuildAnnex(b *building, at vec3, _ int) {
	const w, d, h = 4.4, 3.6, 3.0
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	for k := range 4 {
		x := -w/2 + w/8 + float64(k)*w/4
		b.out([]string{"clad.sand", "clad.sand2"}[k%2]).Box(p(x, h/2, 0), vec3{w/4 + 0.005, h, d}, xAxis, 0.03)
	}
	b.out("steel.dark").Box(p(0, 0.18, 0), vec3{w + 0.08, 0.36, d + 0.08}, xAxis, 0.02)
	b.out("steel").Box(p(0, h+0.06, 0), vec3{w + 0.15, 0.12, d + 0.15}, xAxis, 0.02)
	// The door, and a lamp over it.
	b.out("hazard").Box(p(-0.8, 1.15, -d/2-0.03), vec3{1.3, 2.3, 0.06}, xAxis, 0.01)
	b.out("door.plate").Box(p(-0.8, 1.1, -d/2-0.06), vec3{1.0, 2.1, 0.06}, xAxis, 0.01)
	b.out("lamp.warm").Box(p(-0.8, 2.5, -d/2-0.1), vec3{0.5, 0.06, 0.12}, xAxis, 0)
	b.out("led.green").Box(p(0.3, 1.6, -d/2-0.04), vec3{0.08, 0.08, 0.04}, xAxis, 0)
	// Louvres on both sides.
	for _, s := range []float64{-1, 1} {
		for k := range 5 {
			b.out("steel.dark").Box(p(s*(w/2+0.03), 0.9+float64(k)*0.22, 0.4), vec3{0.06, 0.1, 1.6}, xAxis, 0)
		}
	}
	// Radiators on the roof: thin white fins on a steel frame.
	for k := range 9 {
		z := -d/2 + 0.5 + float64(k)*(d-1.0)/8
		b.out("habitat.white2").Box(p(0.3, h+0.75, z), vec3{3.2, 1.2, 0.04}, xAxis, 0)
	}
	for _, x := range []float64{-1.3, 1.9} {
		b.out("steel").Box(p(x, h+0.75, 0), vec3{0.08, 1.3, d - 0.8}, xAxis, 0.01)
	}
	// The purge stack, at the back corner, with a hazard band.
	stack := p(-w/2+0.5, h, d/2-0.5)
	b.out("steel").Prism(stack, yUp, 0.24, 0.2, 1.8, 10, 0.02)
	b.out("hazard").Prism(stack.Add(vec3{0, 1.3, 0}), yUp, 0.215, 0.215, 0.2, 10, 0)
	b.out("steel.dark").Prism(stack.Add(vec3{0, 1.8, 0}), yUp, 0.3, 0.3, 0.08, 10, 0.01)
}

// AnnexObstacles: what stops the player around the annex, from its spot.
var AnnexObstacles = []vec3{{-1.2, 0, 0}, {1.2, 0, 0}}
