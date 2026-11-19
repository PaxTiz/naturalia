package base

import (
	"math"
	"naturalia/internal/mesh"
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
