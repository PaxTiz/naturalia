package planet

import (
	"encoding/binary"
	"math"
	"math/rand/v2"
	"naturalia/internal/mathx"
)

const (
	TileCells    = 128 // dust cells per tile side
	TilesPerFace = 156 // tiles per cube face side
	FaceCells    = TileCells * TilesPerFace
	// Arc length of one cell (~27 cm), and the dust a full cell holds (4 g/m²).
	cellArc      = math.Pi / 2 * Radius / FaceCells
	gramsPerCell = cellArc * cellArc * 4
	tileArc      = cellArc * TileCells

	fullDust = math.MaxUint16
)

// TileKey identifies a tile: face, then row, then column.
type TileKey int32

func keyOf(f, tx, ty int) TileKey { return TileKey((f*TilesPerFace+ty)*TilesPerFace + tx) }
func (k TileKey) Split() (f, tx, ty int) {
	i := int(k)
	return i / (TilesPerFace * TilesPerFace), i % TilesPerFace, i / TilesPerFace % TilesPerFace
}

// Gem is a crystal hidden under the dust. There is at most one per
// tile, placed procedurally, so only the collected ones need remembering.
type Gem struct {
	Key TileKey
	Dir vec3
}

// World is the dust covering the whole of Mars. Untouched ground is implicitly
// full; only the tiles that were cleaned are stored.
type World struct {
	Seed      uint64
	tiles     map[TileKey][]uint16
	collected map[TileKey]bool
	dirty     map[TileKey]bool // changed since they were last uploaded

	tan       []float64 // tangent of each cell center's angle on a face
	totalGems int       // crystals on the whole of Mars at the start
	Relief    Relief    // the ground under the dust
}

func NewWorld(seed uint64, set *Settings) *World {
	w := &World{
		Seed:      seed,
		Relief:    newRelief(seed, set),
		tiles:     map[TileKey][]uint16{},
		collected: map[TileKey]bool{},
		dirty:     map[TileKey]bool{},
		tan:       make([]float64, FaceCells),
	}
	for i := range w.tan {
		w.tan[i] = math.Tan(((float64(i)+0.5)/FaceCells*2 - 1) * math.Pi / 4)
	}
	return w
}

func cellDir(f, i, j int) vec3 {
	return mathx.FaceDir(f, (float64(i)+0.5)/FaceCells*2-1, (float64(j)+0.5)/FaceCells*2-1)
}

func cellOf(d vec3) (f, i, j int) {
	f, a, b := mathx.DirToFace(d)
	i = max(0, min(FaceCells-1, int((a+1)/2*FaceCells)))
	j = max(0, min(FaceCells-1, int((b+1)/2*FaceCells)))
	return f, i, j
}

// dirOf is cellDir with a fast path for cells inside the face.
func (w *World) dirOf(f, i, j int) vec3 {
	if i < 0 || j < 0 || i >= FaceCells || j >= FaceCells {
		return cellDir(f, i, j)
	}
	cf := mathx.CubeFaces[f]
	return cf.N.Add(cf.U.Scale(w.tan[i])).Add(cf.V.Scale(w.tan[j])).Norm()
}

func (w *World) DustAt(d vec3) float64 {
	f, i, j := cellOf(d.Norm())
	t := w.tiles[keyOf(f, i/TileCells, j/TileCells)]
	if t == nil {
		return 1
	}
	return float64(t[(j%TileCells)*TileCells+i%TileCells]) / fullDust
}

// Clean removes up to amount (fraction of a full cell) per cell, less on the
// brush edges, and never more than budget grams in total: when the budget is
// short the whole brush is weakened evenly, so it never leaves a cut edge.
// Brushes that overlap a cube edge continue on the neighboring face.
// Returns the grams picked up.
func (w *World) Clean(center vec3, radius, amount, budget float64) float64 {
	if budget <= 0 {
		return 0
	}
	type cut struct {
		cell *uint16
		take float64
	}
	var cuts []cut
	total := 0.0
	w.brush(center, radius, func(cell *uint16, d2 float64) {
		if take := min(amount*(1-0.5*d2), float64(*cell)/fullDust); take > 0 {
			cuts = append(cuts, cut{cell, take})
			total += take * gramsPerCell
		}
	})
	scale := 1.0
	if total > budget {
		scale = budget / total
	}
	for _, c := range cuts {
		*c.cell -= uint16(math.Round(c.take * scale * fullDust))
	}
	return total * scale
}

// brush calls fn for every cell within radius meters of center, with the
// squared normalized distance to the center, creating tiles as needed.
func (w *World) brush(center vec3, radius float64, fn func(cell *uint16, d2 float64)) {
	c := center.Norm()
	radius = math.Max(radius, cellArc)
	f, a, b := mathx.DirToFace(c)
	ci, cj := int((a+1)/2*FaceCells), int((b+1)/2*FaceCells)
	// Cells shrink toward face corners, so look a bit wider than the radius.
	span := int(radius/cellArc*1.5) + 1
	var tile []uint16
	lastKey := TileKey(-1)
	for j := cj - span; j <= cj+span; j++ {
		for i := ci - span; i <= ci+span; i++ {
			d := w.dirOf(f, i, j)
			chord := d.Sub(c).Length() * Radius
			d2 := chord * chord / (radius * radius)
			if d2 > 1 {
				continue
			}
			ff, ii, jj := f, i, j
			if i < 0 || j < 0 || i >= FaceCells || j >= FaceCells {
				ff, ii, jj = cellOf(d)
			}
			key := keyOf(ff, ii/TileCells, jj/TileCells)
			if key != lastKey {
				tile, lastKey = w.tile(key), key
				w.dirty[key] = true
			}
			fn(&tile[(jj%TileCells)*TileCells+ii%TileCells], d2)
		}
	}
}

// tile returns a tile's cells, creating it full of dust if needed.
func (w *World) tile(k TileKey) []uint16 {
	t := w.tiles[k]
	if t == nil {
		t = make([]uint16, TileCells*TileCells)
		for i := range t {
			t[i] = fullDust
		}
		w.tiles[k] = t
	}
	return t
}

// FindDusty samples random cells within spread meters of around and
// returns the direction of the dustiest one.
func (w *World) FindDusty(rng *rand.Rand, around vec3, spread float64) vec3 {
	up := around.Norm()
	t1, t2 := mathx.TangentBasis(up)
	best, bestDir := -1.0, up
	for range 24 {
		a, r := rng.Float64()*2*math.Pi, math.Sqrt(rng.Float64())*spread
		d := up.Scale(Radius).Add(t1.Scale(math.Cos(a) * r)).Add(t2.Scale(math.Sin(a) * r)).Norm()
		if dust := w.DustAt(d); dust > best {
			best, bestDir = dust, d
		}
	}
	return bestDir
}

// gemIn returns the crystal of a tile, if it has one left.
func (w *World) gemIn(k TileKey) (Gem, bool) {
	f, tx, ty := k.Split()
	if mathx.Hash3(f, tx, ty, w.Seed+101) > w.Relief.set.Gems || w.collected[k] {
		return Gem{}, false
	}
	i := tx*TileCells + 8 + int(mathx.Hash3(f, tx, ty, w.Seed+102)*(TileCells-16))
	j := ty*TileCells + 8 + int(mathx.Hash3(f, tx, ty, w.Seed+103)*(TileCells-16))
	d := cellDir(f, i, j)
	if ArcDist(d, BaseUp) < padRadius+2 {
		return Gem{}, false
	}
	return Gem{k, d}, true
}

// TilesNear calls fn for each tile within dist meters of c (roughly).
func TilesNear(c vec3, dist float64, fn func(k TileKey)) {
	f, a, b := mathx.DirToFace(c.Norm())
	ti, tj := int((a+1)/2*TilesPerFace), int((b+1)/2*TilesPerFace)
	span := int(dist/tileArc*1.5) + 1
	seen := map[TileKey]bool{}
	for j := tj - span; j <= tj+span; j++ {
		for i := ti - span; i <= ti+span; i++ {
			// Tiles past the face edge are found on the neighboring face.
			mid := mathx.FaceDir(f, (float64(i)+0.5)/TilesPerFace*2-1, (float64(j)+0.5)/TilesPerFace*2-1)
			ff, ci, cj := cellOf(mid)
			k := keyOf(ff, ci/TileCells, cj/TileCells)
			if !seen[k] {
				seen[k] = true
				fn(k)
			}
		}
	}
}

// GemsNear lists the remaining crystals within dist meters of c.
func (w *World) GemsNear(c vec3, dist float64) []Gem {
	c = c.Norm()
	var gems []Gem
	TilesNear(c, dist, func(k TileKey) {
		if g, ok := w.gemIn(k); ok && g.Dir.Sub(c).Length()*Radius <= dist {
			gems = append(gems, g)
		}
	})
	return gems
}

// CollectGems removes and returns the gems that are uncovered and under the brush.
func (w *World) CollectGems(center vec3, radius float64) []Gem {
	c := center.Norm()
	var got []Gem
	for _, g := range w.GemsNear(c, radius+0.5) {
		if w.DustAt(g.Dir) < 0.2 {
			w.collected[g.Key] = true
			got = append(got, g)
		}
	}
	return got
}

func TileCenter(k TileKey) vec3 {
	f, tx, ty := k.Split()
	return cellDir(f, tx*TileCells+TileCells/2, ty*TileCells+TileCells/2)
}

// TileBytes encodes the cleaned tiles for saving.
func (w *World) TileBytes() map[TileKey][]byte {
	out := make(map[TileKey][]byte, len(w.tiles))
	for k, t := range w.tiles {
		b := make([]byte, len(t)*2)
		for i, v := range t {
			binary.LittleEndian.PutUint16(b[i*2:], v)
		}
		out[k] = b
	}
	return out
}

func (w *World) SetTileBytes(tiles map[TileKey][]byte) {
	for k, b := range tiles {
		if len(b) != TileCells*TileCells*2 {
			continue
		}
		t := make([]uint16, TileCells*TileCells)
		for i := range t {
			t[i] = binary.LittleEndian.Uint16(b[i*2:])
		}
		w.tiles[k] = t
	}
}

func (w *World) CollectedKeys() []TileKey {
	keys := make([]TileKey, 0, len(w.collected))
	for k := range w.collected {
		keys = append(keys, k)
	}
	return keys
}

func (w *World) SetCollected(keys []TileKey) {
	for _, k := range keys {
		w.collected[k] = true
	}
}

// Tile is a touched tile's cells, nil for an untouched one.
func (w *World) Tile(k TileKey) []uint16 { return w.tiles[k] }

// Touched lists the tiles that have been cleaned at least a little.
func (w *World) Touched() []TileKey {
	keys := make([]TileKey, 0, len(w.tiles))
	for k := range w.tiles {
		keys = append(keys, k)
	}
	return keys
}

// Dirty lists the tiles changed since they were last marked uploaded.
func (w *World) Dirty() []TileKey {
	keys := make([]TileKey, 0, len(w.dirty))
	for k := range w.dirty {
		keys = append(keys, k)
	}
	return keys
}

func (w *World) MarkUploaded(k TileKey) { delete(w.dirty, k) }

// MarkAllDirty has every touched tile uploaded again.
func (w *World) MarkAllDirty() {
	for k := range w.tiles {
		w.dirty[k] = true
	}
}

// Cleaned is the fraction of a tile's dust removed.
func (w *World) Cleaned(k TileKey) float64 {
	t := w.tiles[k]
	if t == nil {
		return 0
	}
	sum := 0
	for _, v := range t {
		sum += int(v)
	}
	return 1 - float64(sum)/float64(len(t)*fullDust)
}

// countGems counts the crystals of the whole of Mars, collected or not.
func (w *World) countGems() int {
	collected := w.collected
	w.collected = nil // gemIn skips collected ones: count them all
	n := 0
	for k := range TileKey(6 * TilesPerFace * TilesPerFace) {
		if _, ok := w.gemIn(k); ok {
			n++
		}
	}
	w.collected = collected
	return n
}

// Stats summarizes the cleaning progress for the map table.
type Stats struct {
	Cleaned        float64 // fraction of Mars' dust removed
	TilesVisited   int
	TilesTotal     int
	Gems, GemTotal int
}

func (w *World) Stats(totalDust float64) Stats {
	if w.totalGems == 0 {
		w.totalGems = w.countGems()
	}
	marsDust := 6 * FaceCells * FaceCells * gramsPerCell
	return Stats{
		Cleaned:      totalDust / marsDust,
		TilesVisited: len(w.tiles),
		TilesTotal:   6 * TilesPerFace * TilesPerFace,
		Gems:         len(w.collected),
		GemTotal:     w.totalGems,
	}
}
