package planet

import (
	"math"
	"naturalia/internal/engine/mathx"
)

// Layer is a shape the relief is made of: plains, dunes, the volcano... The
// ground's height is the sum of Settings.Layers, each scaled by its
// strength. Packs add their own, or take the place of one of Mars' by its
// ID (Settings.SetLayer). The relief is computed on many threads, for every
// vertex of the planet: Height must be pure, quick, without state.
type Layer struct {
	ID       string
	Strength func(t *Settings) float64 // scales it, 0 skips it; nil for 1
	Height   func(s Spot) float64      // in m
}

// Spot is a point of the ground a layer shapes, with what layers share.
type Spot struct {
	Dir, P vec3 // its direction (unit), and the point on the mean sphere
	Seed   uint64
	Set    *Settings
	// How much the spot is, 0 to 1: away from the base, on the polar cap,
	// in the canyon, on the volcano; and Clear of all of them, where shapes
	// that would blur them belong.
	Away, Ice, InCanyon, OnVolcano, Clear float64

	canyon, volcano float64 // the landmarks' own heights
}

func (r Relief) spot(d vec3) Spot {
	t := r.set
	s := Spot{Dir: d, P: d.Scale(Radius), Seed: r.seed, Set: t}
	s.Away = mathx.Smoothstep(plainRadius, plainRadius*2.2, ArcDist(d, BaseUp))
	s.Ice = r.Ice(d)
	s.canyon, s.InCanyon = canyon(s.Seed, d, s.P)
	s.volcano, s.OnVolcano = volcano(s.Seed, d, s.P)
	s.InCanyon *= math.Min(1, t.Canyon)
	s.OnVolcano *= math.Min(1, t.Volcano)
	s.Clear = s.Away * (1 - s.Ice) * (1 - s.OnVolcano) * (1 - s.InCanyon)
	return s
}

// CoreLayers are the layers Mars is made of.
func CoreLayers() []Layer {
	return []Layer{
		{"plains", func(t *Settings) float64 { return t.Plains }, plains},
		{"hills", func(t *Settings) float64 { return t.Hills }, hills},
		{"dunes", func(t *Settings) float64 { return t.Dunes }, dunes},
		{"mountains", func(t *Settings) float64 { return t.Mountains }, mountains},
		{"craters", func(t *Settings) float64 { return t.Craters }, craters},
		{"volcano", func(t *Settings) float64 { return t.Volcano }, func(s Spot) float64 { return s.volcano }},
		{"canyon", func(t *Settings) float64 { return t.Canyon }, func(s Spot) float64 { return s.canyon }},
		{"ice", func(t *Settings) float64 { return t.IceCap }, iceCap},
		{"features", nil, func(s Spot) float64 { return s.Set.FeatureHeight(s.Dir) }},
	}
}

// plains: long gentle swells, enough to catch the low light and break the
// flat color areas, still no small-scale bumps.
func plains(s Spot) float64 {
	return 14*(mathx.Fbm3(s.P.Scale(1.0/900), s.Seed+11, 2)-0.5) +
		7*(mathx.Fbm3(s.P.Scale(1.0/220), s.Seed+12, 2)-0.5) +
		2.5*(mathx.Fbm3(s.P.Scale(1.0/70), s.Seed+13, 2)-0.5)
}

// hillMask says where hills gather (1) and where plains stretch out (0).
func (s Spot) hillMask() float64 {
	return mathx.Smoothstep(0.47, 0.64, mathx.Fbm3(s.P.Scale(1.0/1400), s.Seed+60, 2))
}

// hills: rounded bumps of every size, grouped in regions.
func hills(s Spot) float64 {
	mask := s.hillMask()
	if mask <= 0 {
		return 0
	}
	h := 0.0
	featuresNear(s.Dir, 230, 0.6, s.Seed+61, func(f feature) {
		radius := 45 + 70*f.rand(4)
		x := f.dir.Sub(s.Dir).Length() * Radius / radius
		if x >= 1 {
			return
		}
		height := 4 + 30*math.Pow(f.rand(5), 1.5) // mostly modest, a few generous ones
		bump := 1 - x*x
		h += height * bump * bump
	})
	return h * mask * s.Clear
}

// dunes: fields of long parallel ridges shaped by the wind, a gentle side
// facing it and a steeper (but still rounded) lee side. They gather where
// hills don't, and break up into shorter crests at the edges of a field.
func dunes(s Spot) float64 {
	mask := mathx.Smoothstep(0.42, 0.56, mathx.Fbm3(s.P.Scale(1.0/1100), s.Seed+70, 2)) * (1 - s.hillMask())
	if mask <= 0 {
		return 0
	}
	const wavelength = 55.0
	wind := vec3{0.8, 0.25, -0.55}.Norm()
	x := s.P.Dot(wind)/wavelength + 1.6*mathx.Fbm3(s.P.Scale(1.0/160), s.Seed+71, 2)
	f := x - math.Floor(x)
	ridge := mathx.Smoothstep(0, 0.7, f)
	if f > 0.7 {
		ridge = 1 - mathx.Smoothstep(0.7, 1, f)
	}
	crest := mathx.Smoothstep(0.3, 0.6, mathx.Fbm3(s.P.Scale(1.0/90), s.Seed+72, 2)+mask*0.25)
	return 7 * ridge * crest * mask * s.Away * (1 - s.Ice) * (1 - s.OnVolcano*0.8)
}

// mountains: a few big massifs, rounded on top, with irregular outlines.
// Radius plus warp stays under the cell size.
func mountains(s Spot) float64 {
	const cell = 1400.0
	h, warp := 0.0, math.NaN()
	featuresNear(s.Dir, cell, 0.25, s.Seed+30, func(f feature) {
		radius := 380 + 270*f.rand(4)
		dist := f.dir.Sub(s.Dir).Length() * Radius
		if dist > radius+80 || !landmarkClear(f.dir, radius+80) {
			return
		}
		height := 120 + 90*f.rand(5)
		if math.IsNaN(warp) {
			warp = (mathx.Fbm3(s.P.Scale(1.0/400), s.Seed+31, 2) - 0.5) * 160
		}
		x := (dist + warp) / radius
		if x >= 1 {
			return
		}
		bump := 1 - x*x
		h += height * bump * bump * (0.85 + 0.3*mathx.Fbm3(s.P.Scale(1.0/350), s.Seed+32, 2))
	})
	return h
}

// craters: a few big ones, more small ones, off the landmarks.
func craters(s Spot) float64 {
	return (craterField(s.Dir, 520, 0.12, 35, 120, s.Seed+40) + craterField(s.Dir, 150, 0.05, 8, 30, s.Seed+50)) * s.Clear
}

// craterField: shallow bowls with a soft raised lip, radius between rMin and rMax.
func craterField(d vec3, cell, chance, rMin, rMax float64, seed uint64) float64 {
	h := 0.0
	featuresNear(d, cell, chance, seed, func(f feature) {
		radius := rMin + (rMax-rMin)*math.Pow(f.rand(4), 2)
		x := f.dir.Sub(d).Length() * Radius / radius
		if x > 1.8 || !landmarkClear(f.dir, radius*1.8) {
			return
		}
		bowl := math.Max(0, 1-x*x)
		lip := (x - 1) / 0.35
		h += radius * 0.12 * (-bowl*bowl + 0.25*math.Exp(-lip*lip))
	})
	return h
}
