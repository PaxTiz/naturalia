package main

import "math"

const (
	marsRadius  = 3389.5 // the real planet at 1/1 000 scale
	padRadius   = 22.0   // the ground is flat under the whole base
	maxMountain = 420.0  // upper bound of the relief, for horizon culling
	plainRadius = 160.0  // around the base, only gentle plains
)

// The base sits on the north pole of the cube, where "up" is +Y. Its
// altitude follows the ground there; it is set when the terrain is created.
var (
	baseUp  = vec3{0, 1, 0}
	basePos = baseUp.scale(marsRadius)
)

// toward returns the direction dist meters away from the base, heading
// along bearing (a horizontal vector at the base).
func toward(bearing vec3, dist float64) vec3 {
	a := dist / marsRadius
	return baseUp.scale(math.Cos(a)).add(bearing.norm().scale(math.Sin(a)))
}

// The three landmarks of Mars, placed around the base so that the volcano
// rises ahead when leaving the airlock (toward -Z), the polar cap shines
// behind the base and the canyon opens to the east.
var (
	volcanoCenter = toward(vec3{-0.6, 0, -1}, 1750)
	capCenter     = toward(vec3{0, 0, 1}, 760)
	canyonCenter  = toward(vec3{1, 0, 0}, 1050)
	// The canyon follows the great circle through its center along this
	// direction: its axis, then the axis across it.
	canyonAlong  = tangentAt(canyonCenter, vec3{0.3, 0, 1})
	canyonNormal = canyonCenter.cross(canyonAlong)
)

const (
	volcanoRadius  = 1500.0
	volcanoHeight  = 380.0
	calderaRadius  = 190.0
	calderaDepth   = 40.0
	capRadius      = 640.0
	capThickness   = 26.0
	canyonDepth    = 80.0
	canyonHalfW    = 260.0
	canyonFrom     = -1100.0 // along the axis, from its center
	canyonTo       = 1700.0
	canyonEndTaper = 500.0
)

// tangentAt projects v on the ground plane at d.
func tangentAt(d, v vec3) vec3 { return v.sub(d.scale(v.dot(d))).norm() }

// Relief is the ground shape of Mars, smooth everywhere: vast red plains
// with fields of soft dunes, regions of rounded hills, a few big massifs
// and craters, plus three landmarks: a shield volcano, a canyon and the
// polar ice cap. Scattered features live on 3D grids of cells, so the
// whole planet costs no memory.
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
	t := terrain()
	p := d.scale(marsRadius)
	fromBase := arcDist(d, baseUp)
	away := smoothstep(plainRadius, plainRadius*2.2, fromBase) // 0 near the base
	ice := r.Ice(d)
	canyon, inCanyon := r.canyon(d, p)
	volcano, onVolcano := r.volcano(d, p)
	inCanyon *= math.Min(1, t.Canyon)
	onVolcano *= math.Min(1, t.Volcano)

	// Plains: long gentle swells, enough to catch the low light and break
	// the flat color areas, still no small-scale bumps.
	h := t.Plains * (14*(fbm3(p.scale(1.0/900), r.seed+11, 2)-0.5) +
		7*(fbm3(p.scale(1.0/220), r.seed+12, 2)-0.5) +
		2.5*(fbm3(p.scale(1.0/70), r.seed+13, 2)-0.5))

	// Hills and dunes keep off the landmarks, whose shapes read better clean.
	clear := away * (1 - ice) * (1 - onVolcano) * (1 - inCanyon)
	if t.Hills != 0 {
		h += r.hills(d, p) * clear * t.Hills
	}
	if t.Dunes != 0 {
		h += r.dunes(p) * away * (1 - ice) * (1 - onVolcano*0.8) * t.Dunes
	}
	if t.Mountains != 0 {
		h += r.mountains(d, p) * t.Mountains
	}
	if t.Craters != 0 {
		h += (r.craters(d, 520, 0.12, 35, 120, r.seed+40) + r.craters(d, 150, 0.05, 8, 30, r.seed+50)) * clear * t.Craters
	}

	h += volcano*t.Volcano + canyon*t.Canyon
	h += r.iceCap(d, p, ice) * t.IceCap
	h += t.features(d)
	for _, layer := range t.Layers {
		h += layer(r.seed, d, p)
	}
	return h
}

// hillMask says where hills gather (1) and where plains stretch out (0).
func (r Relief) hillMask(p vec3) float64 {
	return smoothstep(0.47, 0.64, fbm3(p.scale(1.0/1400), r.seed+60, 2))
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

// dunes: fields of long parallel ridges shaped by the wind, a gentle side
// facing it and a steeper (but still rounded) lee side. They gather where
// hills don't, and break up into shorter crests at the edges of a field.
func (r Relief) dunes(p vec3) float64 {
	mask := smoothstep(0.42, 0.56, fbm3(p.scale(1.0/1100), r.seed+70, 2)) * (1 - r.hillMask(p))
	if mask <= 0 {
		return 0
	}
	const wavelength = 55.0
	wind := vec3{0.8, 0.25, -0.55}.norm()
	s := p.dot(wind)/wavelength + 1.6*fbm3(p.scale(1.0/160), r.seed+71, 2)
	f := s - math.Floor(s)
	ridge := smoothstep(0, 0.7, f)
	if f > 0.7 {
		ridge = 1 - smoothstep(0.7, 1, f)
	}
	crest := smoothstep(0.3, 0.6, fbm3(p.scale(1.0/90), r.seed+72, 2)+mask*0.25)
	return 7 * ridge * crest * mask
}

// volcano: a wide shield with a caldera on top and lava flows running down
// its flanks. It also returns how much d is on it (0..1).
func (r Relief) volcano(d, p vec3) (float64, float64) {
	dist := arcDist(d, volcanoCenter)
	if dist > volcanoRadius*1.15 {
		return 0, 0
	}
	warp := (fbm3(p.scale(1.0/450), r.seed+81, 2) - 0.5) * 140
	x := (dist + warp) / volcanoRadius
	on := smoothstep(1.05, 0.6, x)
	if x >= 1 {
		return 0, on
	}
	bump := 1 - x*x
	h := volcanoHeight * bump * bump

	// Lava flows: radial ridges, strongest mid-flank.
	t1, t2 := tangentBasis(volcanoCenter)
	angle := math.Atan2(d.dot(t2), d.dot(t1))
	flows := math.Sin(angle*28+6*fbm3(p.scale(1.0/300), r.seed+82, 2)) * 0.5
	flows += math.Sin(angle*61+4*fbm3(p.scale(1.0/200), r.seed+83, 2)) * 0.25
	h += 4 * flows * smoothstep(0.3, 0.5, x) * (1 - smoothstep(0.75, 1, x))

	// Caldera: a flat-floored pit, plus a smaller one nested off-center.
	h -= calderaDepth * (1 - smoothstep(calderaRadius*0.75, calderaRadius*1.2, dist))
	inner := arcDist(d, toward2(volcanoCenter, t1, 60))
	h -= 20 * (1 - smoothstep(45, 95, inner))
	return h, on
}

// toward2 is toward from any point: dist meters from c along tangent t.
func toward2(c, t vec3, dist float64) vec3 {
	a := dist / marsRadius
	return c.scale(math.Cos(a)).add(t.scale(math.Sin(a)))
}

// canyon: a long winding trench with a flat floor and walls in two steps.
// It also returns how much d is in it (0..1).
func (r Relief) canyon(d, p vec3) (float64, float64) {
	along := math.Atan2(d.dot(canyonAlong), d.dot(canyonCenter)) * marsRadius
	if along < canyonFrom-200 || along > canyonTo+200 {
		return 0, 0
	}
	across := math.Asin(math.Max(-1, math.Min(1, d.dot(canyonNormal)))) * marsRadius
	if math.Abs(across) > canyonHalfW*2.2 {
		return 0, 0
	}
	taper := smoothstep(canyonFrom, canyonFrom+canyonEndTaper, along) * (1 - smoothstep(canyonTo-canyonEndTaper, canyonTo, along))
	center := 70*math.Sin(along/380) + 80*(fbm3(p.scale(1.0/900), r.seed+91, 2)-0.5)
	width := canyonHalfW * (0.85 + 0.3*fbm3(p.scale(1.0/600), r.seed+92, 2)) * (0.5 + 0.5*taper)
	x := (math.Abs(across-center) + 30*(fbm3(p.scale(1.0/120), r.seed+93, 2)-0.5)) / width
	depth := 0.5*(1-smoothstep(0.35, 0.65, x)) + 0.5*(1-smoothstep(0.7, 1, x))
	return -canyonDepth * taper * depth, smoothstep(1.4, 1, x) * taper
}

// capInside is how far inside the polar cap's ragged edge d is, in meters
// (negative outside).
func (r Relief) capInside(d vec3) float64 {
	dist := arcDist(d, capCenter)
	if dist > capRadius*1.3 {
		return -dist
	}
	return capRadius + 70*(fbm3(d.scale(marsRadius/260), r.seed+101, 2)-0.5) - dist
}

// Ice is how much of the polar cap covers direction d (0..1). The white
// starts right at the foot of the ice cliff.
func (r Relief) Ice(d vec3) float64 {
	if terrain().IceCap == 0 {
		return 0
	}
	return smoothstep(-12, 12, r.capInside(d))
}

// iceCap: a low dome of ice, carved by spiral troughs like the real one.
func (r Relief) iceCap(d, p vec3, ice float64) float64 {
	if ice <= 0 {
		return 0
	}
	dist := arcDist(d, capCenter)
	h := capThickness * smoothstep(-10, 70, r.capInside(d)) * (0.6 + 0.4*(1-math.Pow(dist/capRadius, 2)))
	t1, t2 := tangentBasis(capCenter)
	angle := math.Atan2(d.dot(t2), d.dot(t1))
	s := angle/(2*math.Pi)*3 + dist/250 + 0.3*fbm3(p.scale(1.0/150), r.seed+102, 2)
	f := s - math.Floor(s)
	trough := smoothstep(0, 0.15, f) * (1 - smoothstep(0.25, 0.4, f))
	return h - 4*trough*ice*smoothstep(80, 200, dist)
}

// Basalt is how much of the volcano's dark rock shows at d (0..1).
func (r Relief) Basalt(d vec3) float64 {
	if terrain().Volcano == 0 {
		return 0
	}
	dist := arcDist(d, volcanoCenter)
	return smoothstep(volcanoRadius*1.05, volcanoRadius*0.55, dist)
}

// landmarkClear says whether a feature of the given radius at dir stays
// away from the base and from the three landmarks.
func landmarkClear(dir vec3, radius float64) bool {
	if arcDist(dir, baseUp) < radius+plainRadius*2 ||
		arcDist(dir, volcanoCenter) < radius+volcanoRadius+100 ||
		arcDist(dir, capCenter) < radius+capRadius+100 {
		return false
	}
	along := math.Atan2(dir.dot(canyonAlong), dir.dot(canyonCenter)) * marsRadius
	across := math.Abs(math.Asin(math.Max(-1, math.Min(1, dir.dot(canyonNormal)))) * marsRadius)
	return along < canyonFrom-radius-100 || along > canyonTo+radius+100 || across > radius+canyonHalfW*1.6
}

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
	featuresNear(d, cell, 0.25, r.seed+30, func(f feature) {
		radius := 380 + 270*f.rand(4)
		dist := f.dir.sub(d).length() * marsRadius
		if dist > radius+80 || !landmarkClear(f.dir, radius+80) {
			return
		}
		height := 120 + 90*f.rand(5)
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
		x := f.dir.sub(d).length() * marsRadius / radius
		if x > 1.8 || !landmarkClear(f.dir, radius*1.8) {
			return
		}
		bowl := math.Max(0, 1-x*x)
		lip := (x - 1) / 0.35
		h += radius * 0.12 * (-bowl*bowl + 0.25*math.Exp(-lip*lip))
	})
	return h
}

// arcDist is the distance in meters along the surface between two directions.
func arcDist(a, b vec3) float64 {
	return math.Acos(math.Max(-1, math.Min(1, a.dot(b)))) * marsRadius
}

// rand returns the k-th random value tied to this feature.
func (f feature) rand(k int) float64 { return hash3(f.x, f.y, f.z, f.seed+uint64(k)*0x9E3779B9) }
