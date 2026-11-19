package base

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/mathx"
)

// The ground around the base: paths of concrete slabs from the entry's
// forecourt to the landing pad and on to a terrace before the bay window,
// low lit bollards along them, a low wall of regolith blocks sheltering
// the water tanks. They give way to a mod's building (groundFree).
//
//	          hall    ║ terrace
//	   entry ▦▦▦──▦▦▦ ⬡ pad ─▦▦
//	  forecourt

var (
	yardPaths = [][2]vec3{
		{{3.3, 0, -13.6}, {8.0, 0, -13.6}}, // the forecourt to the pad
		{{12.2, 0, -9.1}, {13.7, 0, -7.0}}, // the pad to the terrace
	}
	// The terrace: before the east wall, under the eaves, out to x1.
	terraceX0, terraceX1, terraceZ = 12.75, 16.65, 6.5
	yardBollards                   = []vec3{{4.5, 0, -14.6}, {7.0, 0, -12.6}, {-3.4, 0, -17.4}, {3.4, 0, -17.4}, {13.4, 0, -9.4}, {17.1, 0, -7.1}, {17.1, 0, 7.1}}
	yardWall                       = [2]vec3{{-11.2, 0, -19.3}, {-6.0, 0, -19.3}}
)

const (
	pathW, pathSlab = 1.3, 1.0
	bollardH        = 0.8
)

// groundFree tells whether something flat on the ground, of radius r at
// p, stays clear of the buildings, their banks and the corridors.
func groundFree(p vec3, r float64) bool {
	flat := vec3{p.X, 0, p.Z}
	for _, rm := range Buildings() {
		if rm.Hall != nil {
			if rm.inHall(flat, r+rm.bermRun()) {
				return false
			}
			continue
		}
		if flat.Sub(rm.Center).Length() < rm.Radius+r {
			return false
		}
	}
	for _, c := range Corridors() {
		d := c.To.Sub(c.From)
		t := mathx.Clamp01(flat.Sub(c.From).Dot(d) / d.Dot(d))
		if flat.Sub(c.From.Add(d.Scale(t))).Length() < c.Width/2+WallThick+r {
			return false
		}
	}
	return true
}

// ground builds the paths, the terrace, the bollards and the low wall.
func (b *building) ground() {
	for _, seg := range yardPaths {
		d := seg[1].Sub(seg[0])
		l := d.Length()
		d = d.Scale(1 / l)
		n := max(1, int(math.Round(l/pathSlab)))
		for i := range n {
			at := seg[0].Add(d.Scale((float64(i) + 0.5) * l / float64(n)))
			if groundFree(at, pathW/2) {
				b.out("pad.concrete").Box(at.Add(vec3{0, 0.025, 0}), vec3{l/float64(n) - 0.07, 0.05, pathW}, d, 0.01)
			}
		}
	}
	const slab = 1.3
	for x := terraceX0 + slab/2; x < terraceX1; x += slab {
		for z := -terraceZ; z <= terraceZ+0.01; z += slab {
			at := vec3{x, 0, z}
			if groundFree(at, slab/2) {
				b.out("pad.concrete").Box(at.Add(vec3{0, 0.025, 0}), vec3{slab - 0.06, 0.05, slab - 0.06}, xAxis, 0.01)
			}
		}
	}
	for _, p := range yardBollards {
		if groundFree(p, 0.3) {
			b.out("steel.dark").Prism(p, yUp, 0.09, 0.08, bollardH, 8, 0.01)
			b.out("lamp.warm").Prism(p.Add(vec3{0, bollardH - 0.16, 0}), yUp, 0.085, 0.085, 0.1, 8, 0)
			b.out("steel").Prism(p.Add(vec3{0, bollardH, 0}), yUp, 0.11, 0.1, 0.04, 8, 0.01)
		}
	}
	if groundFree(yardWall[0].Lerp(yardWall[1], 0.5), 2.8) {
		d := yardWall[1].Sub(yardWall[0])
		l := d.Length()
		d = d.Scale(1 / l)
		n := int(math.Round(l / 1.2))
		for i := range n {
			at := yardWall[0].Add(d.Scale((float64(i) + 0.5) * l / float64(n)))
			h := 0.8 + 0.15*grnd(7, i)
			b.out([]string{"regolith.dark", "regolith"}[i%2]).Box(at.Add(vec3{0, h / 2, 0}), vec3{l/float64(n) - 0.04, h, 0.6}, d, 0.06)
		}
	}
}

// groundObstacles: the bollards and the low wall.
func groundObstacles() []content.Obstacle {
	var out []content.Obstacle
	for _, p := range yardBollards {
		if groundFree(p, 0.3) {
			out = append(out, content.Obstacle{At: p, Radius: 0.15})
		}
	}
	if groundFree(yardWall[0].Lerp(yardWall[1], 0.5), 2.8) {
		for t := 0.05; t < 1; t += 0.15 {
			out = append(out, content.Obstacle{At: yardWall[0].Lerp(yardWall[1], t), Radius: 0.4})
		}
	}
	return out
}
