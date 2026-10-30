package main

import "math"

const (
	moonRadius  = 1737.4 // the real Moon at 1/1 000 scale
	padRadius   = 7.0    // the landing pad is flat within this distance
	maxMountain = 200.0  // upper bound of the relief, for horizon culling
)

// The base sits on the north pole, where "up" is +Y. Its altitude follows
// the ground there; it is set when the terrain is created.
var (
	baseUp  = vec3{0, 1, 0}
	basePos = baseUp.scale(moonRadius)
)

// Relief is the Moon's ground shape: mostly rolling hills of every size,
// with big rounded mountains here and there and craters now and then.
// Mountains and craters are scattered procedurally over a 3D grid of cells,
// so the whole Moon costs no memory.
type Relief struct {
	seed  uint64
	baseH float64 // ground altitude at the base, before flattening the pad
}

func newRelief(seed uint64) Relief {
	r := Relief{seed: seed}
	r.baseH = r.raw(baseUp)
	basePos = baseUp.scale(moonRadius + r.baseH)
	return r
}

// Altitude of the ground above the mean radius in direction d (unit).
func (r Relief) Height(d vec3) float64 {
	h := r.raw(d)
	// Blend to a flat pad around the base.
	return r.baseH + (h-r.baseH)*smoothstep(padRadius, padRadius+35, arcDist(d, baseUp))
}

func (r Relief) raw(d vec3) float64 {
	p := d.scale(moonRadius)
	h := 28*(fbm3(p.scale(1.0/700), r.seed+11, 3)-0.5) +
		9*(fbm3(p.scale(1.0/180), r.seed+12, 3)-0.5) +
		2.5*(fbm3(p.scale(1.0/45), r.seed+13, 2)-0.5) +
		0.6*(fbm3(p.scale(1.0/12), r.seed+14, 2)-0.5)
	h += r.mountains(d, p)
	h += r.craters(d, 300, 0.22, 30, 120, r.seed+40)
	h += r.craters(d, 80, 0.25, 5, 28, r.seed+50)
	return h
}

// feature is a random point scattered in one cell of a 3D grid, kept only
// when it lands close to the Moon's surface (so features spread evenly).
type feature struct {
	dir     vec3
	x, y, z int // its grid cell
	seed    uint64
}

// featuresNear calls fn for the features of the cells around d. Features
// must not reach farther than one cell size from their center.
func featuresNear(d vec3, cell, chance float64, seed uint64, fn func(f feature)) {
	p := d.scale(moonRadius)
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
				if math.Abs(c.length()-moonRadius) > cell/2 {
					continue
				}
				f.dir = c.norm()
				fn(f)
			}
		}
	}
}

// mountains: a few big massifs, rounded on top, with irregular outlines.
func (r Relief) mountains(d, p vec3) float64 {
	const cell = 900.0
	h, warp := 0.0, math.NaN()
	featuresNear(d, cell, 0.3, r.seed+30, func(f feature) {
		if arcDist(f.dir, baseUp) < 900 {
			return // keep the base in the plains
		}
		radius := 220 + 200*f.rand(4)
		height := 90 + 90*f.rand(5)
		dist := f.dir.sub(d).length() * moonRadius
		if dist > radius+80 {
			return
		}
		if math.IsNaN(warp) {
			warp = (fbm3(p.scale(1.0/300), r.seed+31, 3) - 0.5) * 160
		}
		x := (dist + warp) / radius
		if x >= 1 {
			return
		}
		bump := 1 - x*x
		h += height * bump * bump * (0.8 + 0.4*fbm3(p.scale(1.0/120), r.seed+32, 2))
	})
	return h
}

// craters: shallow bowls with a soft raised lip, radius between rMin and rMax.
func (r Relief) craters(d vec3, cell, chance, rMin, rMax float64, seed uint64) float64 {
	h := 0.0
	featuresNear(d, cell, chance, seed, func(f feature) {
		radius := rMin + (rMax-rMin)*math.Pow(f.rand(4), 2)
		if arcDist(f.dir, baseUp) < radius*1.8+padRadius+10 {
			return // keep the landing pad clear
		}
		x := f.dir.sub(d).length() * moonRadius / radius
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
	return math.Acos(math.Max(-1, math.Min(1, a.dot(b)))) * moonRadius
}

// rand returns the k-th random value tied to this feature.
func (f feature) rand(k int) float64 { return hash3(f.x, f.y, f.z, f.seed+uint64(k)*0x9E3779B9) }
