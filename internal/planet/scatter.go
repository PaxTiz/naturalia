package planet

import (
	"math"
	"naturalia/internal/engine/mathx"
)

// Scattered features live on 3D grids of cells, so the whole planet costs
// no memory.

// feature is a random point scattered in one cell of a 3D grid, kept only
// when it lands close to the surface (so features spread evenly).
type feature struct {
	dir     vec3
	x, y, z int // its grid cell
	seed    uint64
}

// featuresNear calls fn for the features of the cells around d. Features
// must not reach farther than one cell size from their center.
func featuresNear(d vec3, cell, chance float64, seed uint64, fn func(f feature)) {
	p := d.Scale(Radius)
	cx, cy, cz := int(math.Floor(p.X/cell)), int(math.Floor(p.Y/cell)), int(math.Floor(p.Z/cell))
	for dz := -1; dz <= 1; dz++ {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				x, y, z := cx+dx, cy+dy, cz+dz
				if mathx.Hash3(x, y, z, seed) > chance {
					continue
				}
				f := feature{x: x, y: y, z: z, seed: seed}
				c := vec3{(float64(x) + f.rand(1)) * cell, (float64(y) + f.rand(2)) * cell, (float64(z) + f.rand(3)) * cell}
				if math.Abs(c.Length()-Radius) > cell/2 {
					continue
				}
				f.dir = c.Norm()
				fn(f)
			}
		}
	}
}

// rand returns the k-th random value tied to this feature.
func (f feature) rand(k int) float64 { return mathx.Hash3(f.x, f.y, f.z, f.seed+uint64(k)*0x9E3779B9) }
