package planet

import (
	"mars_sweeper/internal/mathx"
	"math"
)

// Settings is how Mars is generated, open to packs: each layer of
// the relief is scaled (0 removes it), and packs add their own shapes.
// The relief is computed on many threads, for every vertex of the planet:
// layers must be pure Go functions, quick, without state. Scripts can't
// shape the ground, JSON features and Go layers can.
type Settings struct {
	Plains    float64 `json:"plains"`    // long gentle swells
	Hills     float64 `json:"hills"`     // rounded hills, in regions
	Dunes     float64 `json:"dunes"`     // fields of wind ridges
	Mountains float64 `json:"mountains"` // the big massifs
	Craters   float64 `json:"craters"`   // depth of the craters
	Volcano   float64 `json:"volcano"`   // the shield volcano
	Canyon    float64 `json:"canyon"`    // the canyon's depth
	IceCap    float64 `json:"ice_cap"`   // the polar cap; 0 also removes its ice
	Gems      float64 `json:"gems"`      // chance that a tile hides a crystal
	Rocks     float64 `json:"rocks"`     // how many rocks lie around

	Features []Feature `json:"features"` // shapes added by mods

	// Layers are added to the height in Go, by compiled packs: d is the
	// unit direction, p the point on the mean sphere.
	Layers []func(seed uint64, d, p vec3) float64 `json:"-"`
}

// Feature is a shape set on the ground: Dir is the local direction
// from the base [x, z], Distance in meters along the ground.
type Feature struct {
	Kind     string     `json:"kind"` // "crater", "mountain", "mesa" or "pit"
	Dir      [2]float64 `json:"dir"`
	Distance float64    `json:"distance"`
	Radius   float64    `json:"radius"`
	Height   float64    `json:"height"` // of the mountain or mesa, depth of the pit or crater
	center   vec3
}

func DefaultSettings() Settings {
	return Settings{
		Plains: 1, Hills: 1, Dunes: 1, Mountains: 1, Craters: 1,
		Volcano: 1, Canyon: 1, IceCap: 1, Gems: 0.12, Rocks: 1,
	}
}

// Prepare places the features, once every pack is loaded.
func (t *Settings) Prepare() {
	for i := range t.Features {
		f := &t.Features[i]
		dir := vec3{f.Dir[0], 0, f.Dir[1]}
		if dir.Length() < 1e-9 {
			dir = vec3{1, 0, 0}
		}
		f.center = Toward(dir.Norm(), f.Distance)
		if f.Radius <= 0 {
			f.Radius = 50
		}
	}
}

// FeatureHeight is the height the mods' featureHeight add at d.
func (t *Settings) FeatureHeight(d vec3) float64 {
	h := 0.0
	for _, f := range t.Features {
		dist := ArcDist(d, f.center)
		x := dist / f.Radius
		switch f.Kind {
		case "mountain": // a rounded peak
			if x < 1 {
				b := 1 - x*x
				h += f.Height * b * b
			}
		case "mesa": // a flat top with steep sides
			h += f.Height * (1 - mathx.Smoothstep(0.8, 1, x))
		case "pit": // a flat floor with steep walls
			h -= f.Height * (1 - mathx.Smoothstep(0.8, 1, x))
		default: // crater: a bowl and a raised lip
			if x < 1.8 {
				depth := f.Height
				if depth == 0 {
					depth = f.Radius * 0.12
				}
				bowl := math.Max(0, 1-x*x)
				lip := (x - 1) / 0.35
				h += depth * (-bowl*bowl + 0.25*math.Exp(-lip*lip))
			}
		}
	}
	return h
}
