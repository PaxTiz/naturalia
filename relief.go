package main

import "math"

const (
	marsRadius  = 3389.5 // the real planet at 1/1 000 scale
	padRadius   = 22.0   // the ground is flat under the whole base
	maxMountain = 330.0  // upper bound of the relief, for horizon culling

	// The base sits at the bottom of a big crater: a flat floor, then smooth
	// walls (about 27° at most) up to a soft rim.
	spawnDepth = 50.0
	spawnFloor = 45.0  // radius of the flat floor
	spawnRim   = 190.0 // radius of the rim
)

// The base sits on the north pole, where "up" is +Y. Its altitude follows
// the ground there; it is set when the terrain is created.
var (
	baseUp  = vec3{0, 1, 0}
	basePos = baseUp.scale(marsRadius)
)

// Relief is Mars' ground shape, smooth everywhere: vast plains with a
// light relief, regions of rounded hills of every size, a few big
// mountains and craters now and then. Hills, mountains and craters are
// scattered procedurally over 3D grids of cells, so the whole of Mars costs
// no memory.
type Relief struct {
	seed  uint64
	baseH float64 // ground altitude at the base, before flattening the pad
}

func newRelief(seed uint64) Relief {
	r := Relief{seed: seed}
	r.baseH = r.raw(baseUp)
	basePos = baseUp.scale(marsRadius + r.baseH)
	return r
}

// Altitude of the ground above the mean radius in direction d (unit).
func (r Relief) Height(d vec3) float64 {
	h := r.raw(d)
	// Blend to a flat pad around the base.
	return r.baseH + (h-r.baseH)*smoothstep(padRadius, padRadius+35, arcDist(d, baseUp))
}

func (r Relief) raw(d vec3) float64 {
	p := d.scale(marsRadius)
	fromBase := arcDist(d, baseUp)

	// Plains: long gentle swells, no small-scale bumps.
	h := 14*(fbm3(p.scale(1.0/900), r.seed+11, 2)-0.5) +
		4*(fbm3(p.scale(1.0/220), r.seed+12, 2)-0.5) +
		1*(fbm3(p.scale(1.0/70), r.seed+13, 2)-0.5)

	h += r.hills(d, p) * smoothstep(spawnRim*0.9, spawnRim*1.6, fromBase)
	h += r.mountains(d, p)
	h += r.craters(d, 320, 0.15, 35, 130, r.seed+40)
	h += r.craters(d, 90, 0.07, 8, 30, r.seed+50)

	// The spawn crater: flat floor, smooth walls, soft rim.
	if fromBase < spawnRim*1.6 {
		rim := (fromBase - spawnRim) / 40
		h += -spawnDepth*(1-smoothstep(spawnFloor, spawnRim, fromBase)) + 10*math.Exp(-rim*rim)
	}
	return h
}

// hillMask says where hills gather (1) and where plains stretch out (0).
func (r Relief) hillMask(p vec3) float64 {
	return smoothstep(0.45, 0.62, fbm3(p.scale(1.0/1400), r.seed+60, 2))
}

// hills: rounded bumps of every size, grouped in regions.
func (r Relief) hills(d, p vec3) float64 {
	mask := r.hillMask(p)
	if mask <= 0 {
		return 0
	}
	h := 0.0
	featuresNear(d, 230, 0.6, r.seed+61, func(f feature) {
		radius := 45 + 70*f.rand(4)
		x := f.dir.sub(d).length() * marsRadius / radius
		if x >= 1 {
			return
		}
		height := 4 + 30*math.Pow(f.rand(5), 1.5) // mostly modest, a few generous ones
		bump := 1 - x*x
		h += height * bump * bump
	})
	return h * mask
}

// feature is a random point scattered in one cell of a 3D grid, kept only
// when it lands close to Mars' surface (so features spread evenly).
type feature struct {
	dir     vec3
	x, y, z int // its grid cell
	seed    uint64
}

// featuresNear calls fn for the features of the cells around d. Features
// must not reach farther than one cell size from their center.
func featuresNear(d vec3, cell, chance float64, seed uint64, fn func(f feature)) {
	p := d.scale(marsRadius)
	cx, cy, cz := int(math.Floor(p.X/cell)), int(math.Floor(p.Y/cell)), int(math.Floor(p.Z/cell))
	for dz := -1; dz <= 1; dz++ {
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				x, y, z := cx+dx, cy+dy, cz+dz
				if hash3(x, y, z, seed) > chance {
					continue
				}
				f := feature{x: x, y: y, z: z, seed: seed}
				c := vec3{(float64(x) + f.rand(1)) * cell, (float64(y) + f.rand(2)) * cell, (float64(z) + f.rand(3)) * cell}
				if math.Abs(c.length()-marsRadius) > cell/2 {
					continue
				}
				f.dir = c.norm()
				fn(f)
			}
		}
	}
}

// mountains: a few big massifs, rounded on top, with irregular outlines.
// Radius plus warp stays under the cell size.
func (r Relief) mountains(d, p vec3) float64 {
	const cell = 1400.0
	h, warp := 0.0, math.NaN()
	featuresNear(d, cell, 0.3, r.seed+30, func(f feature) {
		radius := 380 + 270*f.rand(4)
		if arcDist(f.dir, baseUp) < radius+spawnRim+300 {
			return // keep them clear of the spawn crater
		}
		height := 140 + 100*f.rand(5)
		dist := f.dir.sub(d).length() * marsRadius
		if dist > radius+80 {
			return
		}
		if math.IsNaN(warp) {
			warp = (fbm3(p.scale(1.0/400), r.seed+31, 2) - 0.5) * 160
		}
		x := (dist + warp) / radius
		if x >= 1 {
			return
		}
		bump := 1 - x*x
		h += height * bump * bump * (0.85 + 0.3*fbm3(p.scale(1.0/350), r.seed+32, 2))
	})
	return h
}

// craters: shallow bowls with a soft raised lip, radius between rMin and rMax.
func (r Relief) craters(d vec3, cell, chance, rMin, rMax float64, seed uint64) float64 {
	h := 0.0
	featuresNear(d, cell, chance, seed, func(f feature) {
		radius := rMin + (rMax-rMin)*math.Pow(f.rand(4), 2)
		if arcDist(f.dir, baseUp) < radius*1.8+spawnRim*1.5 {
			return // keep the spawn crater clean
		}
		x := f.dir.sub(d).length() * marsRadius / radius
		if x > 1.8 {
			return
		}
		bowl := math.Max(0, 1-x*x)
		lip := (x - 1) / 0.35
		h += radius * 0.14 * (-bowl*bowl + 0.25*math.Exp(-lip*lip))
	})
	return h
}

// arcDist is the distance in meters along the surface between two directions.
func arcDist(a, b vec3) float64 {
	return math.Acos(math.Max(-1, math.Min(1, a.dot(b)))) * marsRadius
}

// rand returns the k-th random value tied to this feature.
func (f feature) rand(k int) float64 { return hash3(f.x, f.y, f.z, f.seed+uint64(k)*0x9E3779B9) }
