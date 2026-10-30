package main

import (
	"math"
	"math/rand/v2"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	dustRes = 1024 // dust cells per cube face side: ~27 cm cells
	// Arc length of one cell, and how much dust a full cell holds (4 g/m²).
	cellArc      = math.Pi / 2 * moonRadius / dustRes
	gramsPerCell = cellArc * cellArc * 4
)

// Gem is a helium-3 crystal hidden under the dust, at a direction from the
// Moon's center.
type Gem struct {
	Dir vec3 `json:"dir"`
}

type dirtyRect struct{ minI, minJ, maxI, maxJ int }

// World is the dust layer covering the whole Moon: each cell holds between
// 0 and 1 (full). The GPU gets it as an atlas of the 6 cube faces laid out
// 3×2; only the parts that changed are re-uploaded.
type World struct {
	Seed  uint64
	Gems  []Gem
	dust  []float32
	dirty [6]*dirtyRect
	tex   rl.Texture2D
}

func NewWorld(seed uint64) *World {
	w := &World{Seed: seed, dust: make([]float32, 6*dustRes*dustRes)}
	var wg sync.WaitGroup
	for f := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range dustRes {
				for i := range dustRes {
					d := cellDir(f, i, j).scale(moonRadius / 12)
					w.dust[cellIndex(f, i, j)] = float32(0.55 + 0.45*valueNoise3(d, seed+7))
				}
			}
		}()
	}
	wg.Wait()
	return w
}

func (w *World) ScatterGems(rng *rand.Rand, count int) {
	for len(w.Gems) < count {
		d := vec3{rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()}.norm()
		if arcDist(d, vec3{0, 1, 0}) > padRadius {
			w.Gems = append(w.Gems, Gem{d})
		}
	}
}

func cellIndex(f, i, j int) int { return (f*dustRes+j)*dustRes + i }

func cellDir(f, i, j int) vec3 {
	return faceDir(f, (float64(i)+0.5)/dustRes*2-1, (float64(j)+0.5)/dustRes*2-1)
}

func cellOf(d vec3) (f, i, j int) {
	f, a, b := dirToFace(d)
	i = max(0, min(dustRes-1, int((a+1)/2*dustRes)))
	j = max(0, min(dustRes-1, int((b+1)/2*dustRes)))
	return f, i, j
}

func (w *World) DustAt(d vec3) float64 {
	f, i, j := cellOf(d.norm())
	return float64(w.dust[cellIndex(f, i, j)])
}

func (w *World) markDirty(f, i, j int) {
	r := w.dirty[f]
	if r == nil {
		w.dirty[f] = &dirtyRect{i, j, i, j}
		return
	}
	r.minI, r.minJ = min(r.minI, i), min(r.minJ, j)
	r.maxI, r.maxJ = max(r.maxI, i), max(r.maxJ, j)
}

// brush calls fn for every cell within radius meters of direction c, with
// the squared normalized distance to the center. fn returns false to stop.
// Brushes overlapping a cube edge continue on the neighboring face.
func (w *World) brush(c vec3, radius float64, fn func(idx int, d2 float64) bool) {
	c = c.norm()
	f, a, b := dirToFace(c)
	ci, cj := (a+1)/2*dustRes, (b+1)/2*dustRes
	// Cells shrink toward face corners, so look a bit wider than the radius.
	span := int(radius/cellArc*1.5) + 1
	for j := int(cj) - span; j <= int(cj)+span; j++ {
		for i := int(ci) - span; i <= int(ci)+span; i++ {
			d := cellDir(f, i, j)
			chord := d.sub(c).length() * moonRadius
			d2 := chord * chord / (radius * radius)
			if d2 > 1 {
				continue
			}
			ff, ii, jj := f, i, j
			if i < 0 || j < 0 || i >= dustRes || j >= dustRes {
				ff, ii, jj = cellOf(d)
			}
			w.markDirty(ff, ii, jj)
			if !fn(cellIndex(ff, ii, jj), d2) {
				return
			}
		}
	}
}

// Clean removes up to amount (fraction of a full cell) per cell, less on the
// brush edges, and never more than budget grams in total. Returns the grams
// picked up.
func (w *World) Clean(center vec3, radius, amount, budget float64) float64 {
	got := 0.0
	if budget <= 0 {
		return 0
	}
	w.brush(center, math.Max(radius, cellArc), func(idx int, d2 float64) bool {
		take := min(amount*(1-0.5*d2), float64(w.dust[idx]), (budget-got)/gramsPerCell)
		w.dust[idx] -= float32(take)
		got += take * gramsPerCell
		return got < budget
	})
	return got
}

// FindDusty samples random cells within spread meters of around and
// returns the direction of the dustiest one.
func (w *World) FindDusty(rng *rand.Rand, around vec3, spread float64) vec3 {
	up := around.norm()
	t1, t2 := tangentBasis(up)
	best, bestDir := -1.0, up
	for range 24 {
		a, r := rng.Float64()*2*math.Pi, math.Sqrt(rng.Float64())*spread
		d := up.scale(moonRadius).add(t1.scale(math.Cos(a) * r)).add(t2.scale(math.Sin(a) * r)).norm()
		if dust := w.DustAt(d); dust > best {
			best, bestDir = dust, d
		}
	}
	return bestDir
}

// CollectGems removes and returns the gems that are uncovered and under the brush.
func (w *World) CollectGems(center vec3, radius float64) []Gem {
	c := center.norm()
	var got []Gem
	kept := w.Gems[:0]
	for _, g := range w.Gems {
		if g.Dir.sub(c).length()*moonRadius <= radius+0.5 && w.DustAt(g.Dir) < 0.2 {
			got = append(got, g)
			continue
		}
		kept = append(kept, g)
	}
	w.Gems = kept
	return got
}

// Upload sends the dust that changed since last frame to the GPU.
func (w *World) Upload() rl.Texture2D {
	if w.tex.ID == 0 {
		img := rl.GenImageColor(3*dustRes, 2*dustRes, rl.White)
		rl.ImageFormat(img, rl.UncompressedGrayscale)
		w.tex = rl.LoadTextureFromImage(img)
		rl.UnloadImage(img)
		rl.SetTextureFilter(w.tex, rl.FilterBilinear)
		rl.SetTextureWrap(w.tex, rl.WrapClamp)
		for f := range 6 {
			w.dirty[f] = &dirtyRect{0, 0, dustRes - 1, dustRes - 1}
		}
	}
	for f, r := range w.dirty {
		if r == nil {
			continue
		}
		cw, ch := r.maxI-r.minI+1, r.maxJ-r.minJ+1
		buf := make([]byte, cw*ch)
		for j := range ch {
			row := w.dust[cellIndex(f, r.minI, r.minJ+j):]
			for i := range cw {
				buf[j*cw+i] = byte(row[i] * 255)
			}
		}
		rect := rl.NewRectangle(float32((f%3)*dustRes+r.minI), float32((f/3)*dustRes+r.minJ), float32(cw), float32(ch))
		rl.UpdateTextureRec(w.tex, rect, buf)
		w.dirty[f] = nil
	}
	return w.tex
}

func (w *World) DustBytes() []byte {
	b := make([]byte, len(w.dust))
	for i, d := range w.dust {
		b[i] = byte(math.Round(float64(d) * 255))
	}
	return b
}

func (w *World) SetDustBytes(b []byte) {
	if len(b) != len(w.dust) {
		return
	}
	for i, v := range b {
		w.dust[i] = float32(v) / 255
	}
}
