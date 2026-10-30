package main

import (
	"math"
	"math/rand/v2"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	worldSize = 128.0 // playable area, meters per side
	dustRes   = 512   // dust cells per side
	cellM     = worldSize / dustRes
)

// Gem is a helium-3 crystal hidden under the dust.
type Gem struct{ X, Z float64 }

// World is the dust layer covering the playable area: each cell holds between
// 0 and 1 gram. The GPU gets it as a grayscale texture; all the colors are
// computed by the terrain shader.
type World struct {
	Seed uint64
	Gems []Gem
	dust []float32
	pix  []byte
	tex  rl.Texture2D
}

func NewWorld(seed uint64) *World {
	n := dustRes * dustRes
	w := &World{Seed: seed, dust: make([]float32, n), pix: make([]byte, n)}
	for i := range n {
		fx, fz := float64(i%dustRes), float64(i/dustRes)
		w.dust[i] = float32(0.55 + 0.45*fbm(fx/40, fz/40, seed+7, 3))
	}
	return w
}

func (w *World) ScatterGems(rng *rand.Rand, count int) {
	for range count {
		w.Gems = append(w.Gems, Gem{4 + rng.Float64()*(worldSize-8), 4 + rng.Float64()*(worldSize-8)})
	}
}

func cellIndex(x, z float64) (int, bool) {
	cx, cz := int(x/cellM), int(z/cellM)
	if cx < 0 || cz < 0 || cx >= dustRes || cz >= dustRes {
		return 0, false
	}
	return cz*dustRes + cx, true
}

func (w *World) DustAt(x, z float64) float64 {
	i, ok := cellIndex(x, z)
	if !ok {
		return 0
	}
	return float64(w.dust[i])
}

// brush calls fn for every cell within radius meters of (x, z), with the
// squared normalized distance to the center. fn returns false to stop.
func brush(x, z, radius float64, fn func(i int, d2 float64) bool) {
	cx, cz := x/cellM, z/cellM
	r := math.Max(0.75, radius/cellM)
	for iz := max(0, int(cz-r)); iz <= min(dustRes-1, int(cz+r)); iz++ {
		for ix := max(0, int(cx-r)); ix <= min(dustRes-1, int(cx+r)); ix++ {
			dx, dz := float64(ix)+0.5-cx, float64(iz)+0.5-cz
			d2 := (dx*dx + dz*dz) / (r * r)
			if d2 <= 1 && !fn(iz*dustRes+ix, d2) {
				return
			}
		}
	}
}

// Clean removes up to amount grams per cell (less on the brush edges) and
// never more than budget in total. Returns how much dust was picked up.
// Amounts are per square meter scale, so changing dustRes keeps the economy.
func (w *World) Clean(x, z, radius, amount, budget float64) float64 {
	const gramsPerCell = cellM * cellM * 4 // one 0.5 m cell used to hold 1 g
	got := 0.0
	if budget <= 0 {
		return 0
	}
	brush(x, z, radius, func(i int, d2 float64) bool {
		take := min(amount*(1-0.5*d2), float64(w.dust[i]), (budget-got)/gramsPerCell)
		w.dust[i] -= float32(take)
		got += take * gramsPerCell
		return got < budget
	})
	return got
}

func (w *World) Deposit(x, z, radius, amount float64) {
	brush(x, z, radius, func(i int, d2 float64) bool {
		w.dust[i] = float32(math.Min(1, float64(w.dust[i])+amount*(1-d2)))
		return true
	})
}

// Regen makes the dust slowly settle back everywhere.
func (w *World) Regen(dt, rate float64) {
	add := float32(dt * rate)
	for i, d := range w.dust {
		if d < 1 {
			w.dust[i] = min(1, d+add)
		}
	}
}

// FindDusty samples a few random cells and returns the dustiest one.
func (w *World) FindDusty(rng *rand.Rand) (float64, float64) {
	best, bx, bz := float32(-1), 0, 0
	for range 24 {
		x, z := rng.IntN(dustRes), rng.IntN(dustRes)
		if d := w.dust[z*dustRes+x]; d > best {
			best, bx, bz = d, x, z
		}
	}
	return (float64(bx) + 0.5) * cellM, (float64(bz) + 0.5) * cellM
}

// CollectGems removes and returns the gems that are uncovered and under the brush.
func (w *World) CollectGems(x, z, radius float64) []Gem {
	var got []Gem
	kept := w.Gems[:0]
	for _, g := range w.Gems {
		if math.Hypot(g.X-x, g.Z-z) <= radius+0.5 && w.DustAt(g.X, g.Z) < 0.2 {
			got = append(got, g)
			continue
		}
		kept = append(kept, g)
	}
	w.Gems = kept
	return got
}

// Upload sends the dust amounts to the GPU.
func (w *World) Upload() rl.Texture2D {
	if w.tex.ID == 0 {
		img := rl.GenImageColor(dustRes, dustRes, rl.White)
		rl.ImageFormat(img, rl.UncompressedGrayscale)
		w.tex = rl.LoadTextureFromImage(img)
		rl.UnloadImage(img)
		rl.SetTextureFilter(w.tex, rl.FilterBilinear)
		rl.SetTextureWrap(w.tex, rl.WrapClamp)
	}
	for i, d := range w.dust {
		w.pix[i] = byte(d * 255)
	}
	rl.UpdateTexture(w.tex, w.pix)
	return w.tex
}

func (w *World) DustBytes() []byte {
	b := make([]byte, len(w.dust))
	for i, d := range w.dust {
		b[i] = byte(math.Round(float64(d) * 255))
	}
	return b
}

// SetDustBytes restores the dust layer, upscaling saves made with a lower
// resolution.
func (w *World) SetDustBytes(b []byte) {
	res := int(math.Sqrt(float64(len(b))))
	if res == 0 || res*res != len(b) || dustRes%res != 0 {
		return
	}
	k := dustRes / res
	for i := range w.dust {
		x, z := i%dustRes/k, i/dustRes/k
		w.dust[i] = float32(b[z*res+x]) / 255
	}
}
