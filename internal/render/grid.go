package render

import (
	"image/color"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/game"
	"naturalia/internal/mesh"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// gridRings are the parts of the grid's lines shown around the ghost, from
// its middle out, fainter and fainter.
const gridRings = 5

// gridModels are the lines of the grid of a step, a model per ring, built
// on first use.
func (gfx *Graphics) gridModels(step float64) []rl.Model {
	if m, ok := gfx.grids[step]; ok {
		return m
	}
	reach := 4 + 2*step
	n := int(reach / step)
	var out []rl.Model
	for ring := range gridRings {
		r0, r1 := reach*float64(ring)/gridRings, reach*float64(ring+1)/gridRings
		var parts []content.PartJSON
		line := func(off, a, b float64) { // a piece of the two lines off from the middle
			if b-a < 1e-6 {
				return
			}
			mid, long := (a+b)/2, b-a
			parts = append(parts,
				content.PartJSON{Shape: "box", Token: "neon.cyan", At: vec3{mid, 0, off}, Size: vec3{long, 0.004, 0.02}, Lit: "outdoor"},
				content.PartJSON{Shape: "box", Token: "neon.cyan", At: vec3{off, 0, mid}, Size: vec3{0.02, 0.004, long}, Lit: "outdoor"})
		}
		for i := -n; i <= n; i++ {
			off := float64(i) * step
			if math.Abs(off) >= r1 {
				continue
			}
			end := math.Sqrt(r1*r1 - off*off)
			if math.Abs(off) >= r0 {
				line(off, -end, end)
				continue
			}
			start := math.Sqrt(r0*r0 - off*off)
			line(off, start, end)
			line(off, -end, -start)
		}
		if len(parts) == 0 {
			continue
		}
		b := mesh.NewParts(vec3{})
		content.KitJSON{Parts: parts}.Kit().Build(b, vec3{}, 0)
		for _, k := range b.Keys {
			m := upload(b.Mesh(k), gfx.objectSh)
			gfx.bindMaps(m)
			out = append(out, m)
		}
	}
	if gfx.grids == nil {
		gfx.grids = map[float64][]rl.Model{}
	}
	gfx.grids[step] = out
	return out
}

// drawSheet draws the grid's lines around the ghost, faintly through the
// ground and the blocks before them.
func (gfx *Graphics) drawSheet(s *game.Sheet) {
	models := gfx.gridModels(s.Step)
	draw := func(alpha float64) {
		for i, m := range models {
			fade := 1 - float64(i)/float64(len(models))
			gfx.putBasis(m, s.At, s.U, s.V.Cross(s.U), s.V, color.RGBA{110, 235, 245, uint8(alpha * fade)}, 0, 0.7)
		}
		rl.DrawRenderBatchActive()
	}
	rl.DisableDepthTest()
	draw(35)
	rl.EnableDepthTest()
	draw(110)
}
