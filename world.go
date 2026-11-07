package main

import (
	"cmp"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"slices"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	tileCells    = 128 // dust cells per tile side
	tilesPerFace = 156 // tiles per cube face side
	faceCells    = tileCells * tilesPerFace
	// Arc length of one cell (~27 cm), and the dust a full cell holds (4 g/m²).
	cellArc      = math.Pi / 2 * marsRadius / faceCells
	gramsPerCell = cellArc * cellArc * 4
	tileArc      = cellArc * tileCells

	fullDust     = math.MaxUint16
	atlasTiles   = 16    // the GPU keeps a 16×16 cache of tiles...
	cacheRadius  = 300.0 // ...picked among the touched ones within this distance
	gemChance    = 0.12  // a tile holds a crystal with this probability
	cacheRefresh = 30    // frames between cache refreshes
)

// tileKey identifies a tile: face, then row, then column.
type tileKey int32

func keyOf(f, tx, ty int) tileKey { return tileKey((f*tilesPerFace+ty)*tilesPerFace + tx) }
func (k tileKey) split() (f, tx, ty int) {
	i := int(k)
	return i / (tilesPerFace * tilesPerFace), i % tilesPerFace, i / tilesPerFace % tilesPerFace
}

// Gem is a crystal hidden under the dust. There is at most one per
// tile, placed procedurally, so only the collected ones need remembering.
type Gem struct {
	Key tileKey
	Dir vec3
}

// World is the dust covering the whole of Mars. Untouched ground is implicitly
// full; only the tiles that were cleaned are stored. The GPU gets a cache of
// the nearby touched tiles plus a table telling where each tile sits in it.
type World struct {
	Seed      uint64
	tiles     map[tileKey][]uint16
	collected map[tileKey]bool
	dirty     map[tileKey]bool // changed since they were last uploaded

	tan       []float64 // tangent of each cell center's angle on a face
	atlas     rl.Texture2D
	indir     rl.Texture2D
	indirPix  []byte
	slotOf    map[tileKey]int
	slotTile  []tileKey // -1 when free
	cacheTick int

	// Cleaned fraction of every tile, for the hologram of the map table.
	covTex   rl.Texture2D
	covPix   []byte
	covDirty bool

	totalGems int // crystals on the whole of Mars at the start
}

func NewWorld(seed uint64) *World {
	w := &World{
		Seed:      seed,
		tiles:     map[tileKey][]uint16{},
		collected: map[tileKey]bool{},
		dirty:     map[tileKey]bool{},
		tan:       make([]float64, faceCells),
	}
	for i := range w.tan {
		w.tan[i] = math.Tan(((float64(i)+0.5)/faceCells*2 - 1) * math.Pi / 4)
	}
	return w
}

func cellDir(f, i, j int) vec3 {
	return faceDir(f, (float64(i)+0.5)/faceCells*2-1, (float64(j)+0.5)/faceCells*2-1)
}

func cellOf(d vec3) (f, i, j int) {
	f, a, b := dirToFace(d)
	i = max(0, min(faceCells-1, int((a+1)/2*faceCells)))
	j = max(0, min(faceCells-1, int((b+1)/2*faceCells)))
	return f, i, j
}

// dirOf is cellDir with a fast path for cells inside the face.
func (w *World) dirOf(f, i, j int) vec3 {
	if i < 0 || j < 0 || i >= faceCells || j >= faceCells {
		return cellDir(f, i, j)
	}
	cf := cubeFaces[f]
	return cf.n.add(cf.u.scale(w.tan[i])).add(cf.v.scale(w.tan[j])).norm()
}

func (w *World) DustAt(d vec3) float64 {
	f, i, j := cellOf(d.norm())
	t := w.tiles[keyOf(f, i/tileCells, j/tileCells)]
	if t == nil {
		return 1
	}
	return float64(t[(j%tileCells)*tileCells+i%tileCells]) / fullDust
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
	c := center.norm()
	radius = math.Max(radius, cellArc)
	f, a, b := dirToFace(c)
	ci, cj := int((a+1)/2*faceCells), int((b+1)/2*faceCells)
	// Cells shrink toward face corners, so look a bit wider than the radius.
	span := int(radius/cellArc*1.5) + 1
	var tile []uint16
	lastKey := tileKey(-1)
	for j := cj - span; j <= cj+span; j++ {
		for i := ci - span; i <= ci+span; i++ {
			d := w.dirOf(f, i, j)
			chord := d.sub(c).length() * marsRadius
			d2 := chord * chord / (radius * radius)
			if d2 > 1 {
				continue
			}
			ff, ii, jj := f, i, j
			if i < 0 || j < 0 || i >= faceCells || j >= faceCells {
				ff, ii, jj = cellOf(d)
			}
			key := keyOf(ff, ii/tileCells, jj/tileCells)
			if key != lastKey {
				tile, lastKey = w.tile(key), key
				w.dirty[key] = true
			}
			fn(&tile[(jj%tileCells)*tileCells+ii%tileCells], d2)
		}
	}
}

// tile returns a tile's cells, creating it full of dust if needed.
func (w *World) tile(k tileKey) []uint16 {
	t := w.tiles[k]
	if t == nil {
		t = make([]uint16, tileCells*tileCells)
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
	up := around.norm()
	t1, t2 := tangentBasis(up)
	best, bestDir := -1.0, up
	for range 24 {
		a, r := rng.Float64()*2*math.Pi, math.Sqrt(rng.Float64())*spread
		d := up.scale(marsRadius).add(t1.scale(math.Cos(a) * r)).add(t2.scale(math.Sin(a) * r)).norm()
		if dust := w.DustAt(d); dust > best {
			best, bestDir = dust, d
		}
	}
	return bestDir
}

// gemIn returns the crystal of a tile, if it has one left.
func (w *World) gemIn(k tileKey) (Gem, bool) {
	f, tx, ty := k.split()
	if hash3(f, tx, ty, w.Seed+101) > gemChance || w.collected[k] {
		return Gem{}, false
	}
	i := tx*tileCells + 8 + int(hash3(f, tx, ty, w.Seed+102)*(tileCells-16))
	j := ty*tileCells + 8 + int(hash3(f, tx, ty, w.Seed+103)*(tileCells-16))
	d := cellDir(f, i, j)
	if arcDist(d, baseUp) < padRadius+2 {
		return Gem{}, false
	}
	return Gem{k, d}, true
}

// tilesNear calls fn for each tile within dist meters of c (roughly).
func tilesNear(c vec3, dist float64, fn func(k tileKey)) {
	f, a, b := dirToFace(c.norm())
	ti, tj := int((a+1)/2*tilesPerFace), int((b+1)/2*tilesPerFace)
	span := int(dist/tileArc*1.5) + 1
	seen := map[tileKey]bool{}
	for j := tj - span; j <= tj+span; j++ {
		for i := ti - span; i <= ti+span; i++ {
			// Tiles past the face edge are found on the neighboring face.
			mid := faceDir(f, (float64(i)+0.5)/tilesPerFace*2-1, (float64(j)+0.5)/tilesPerFace*2-1)
			ff, ci, cj := cellOf(mid)
			k := keyOf(ff, ci/tileCells, cj/tileCells)
			if !seen[k] {
				seen[k] = true
				fn(k)
			}
		}
	}
}

// GemsNear lists the remaining crystals within dist meters of c.
func (w *World) GemsNear(c vec3, dist float64) []Gem {
	c = c.norm()
	var gems []Gem
	tilesNear(c, dist, func(k tileKey) {
		if g, ok := w.gemIn(k); ok && g.Dir.sub(c).length()*marsRadius <= dist {
			gems = append(gems, g)
		}
	})
	return gems
}

// CollectGems removes and returns the gems that are uncovered and under the brush.
func (w *World) CollectGems(center vec3, radius float64) []Gem {
	c := center.norm()
	var got []Gem
	for _, g := range w.GemsNear(c, radius+0.5) {
		if w.DustAt(g.Dir) < 0.2 {
			w.collected[g.Key] = true
			got = append(got, g)
		}
	}
	return got
}

func tileCenter(k tileKey) vec3 {
	f, tx, ty := k.split()
	return cellDir(f, tx*tileCells+tileCells/2, ty*tileCells+tileCells/2)
}

// Upload refreshes the GPU cache around the player and returns the tile
// atlas and the lookup table.
func (w *World) Upload(player vec3) (rl.Texture2D, rl.Texture2D) {
	if w.atlas.ID == 0 {
		w.initGPU()
	}
	up := player.norm()
	w.cacheTick++
	if w.cacheTick >= cacheRefresh {
		w.cacheTick = 0
		w.refreshCache(up)
	}
	for k := range w.dirty {
		slot, ok := w.slotOf[k]
		if !ok {
			// Freshly touched tile: take a slot right away.
			if slot = w.freeSlot(up); slot < 0 {
				continue
			}
			w.assign(k, slot)
		}
		w.uploadTile(k, slot)
		w.measure(k)
		delete(w.dirty, k)
	}
	if w.covDirty {
		rl.UpdateTexture(w.covTex, w.covPix)
		w.covDirty = false
	}
	if w.indirPix != nil {
		rl.UpdateTexture(w.indir, w.indirPix)
	}
	return w.atlas, w.indir
}

func (w *World) initGPU() {
	img := rl.GenImageColor(atlasTiles*tileCells, atlasTiles*tileCells, rl.White)
	rl.ImageFormat(img, rl.UncompressedGrayscale)
	w.atlas = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.SetTextureFilter(w.atlas, rl.FilterBilinear)

	img = rl.GenImageColor(3*tilesPerFace, 2*tilesPerFace, rl.Blank)
	w.indir = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.SetTextureFilter(w.indir, rl.FilterPoint)

	w.indirPix = make([]byte, 3*tilesPerFace*2*tilesPerFace*4)

	img = rl.GenImageColor(3*tilesPerFace, 2*tilesPerFace, rl.Black)
	rl.ImageFormat(img, rl.UncompressedGrayscale)
	w.covTex = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	w.covPix = make([]byte, 3*tilesPerFace*2*tilesPerFace)
	w.slotOf = map[tileKey]int{}
	w.slotTile = make([]tileKey, atlasTiles*atlasTiles)
	for i := range w.slotTile {
		w.slotTile[i] = -1
	}
	for k := range w.tiles {
		w.dirty[k] = true
	}
}

// refreshCache keeps the touched tiles nearest to the player in the cache.
func (w *World) refreshCache(up vec3) {
	type cand struct {
		k    tileKey
		dist float64
	}
	var near []cand
	for k := range w.tiles {
		if d := tileCenter(k).sub(up).length() * marsRadius; d < cacheRadius {
			near = append(near, cand{k, d})
		}
	}
	slices.SortFunc(near, func(a, b cand) int { return cmp.Compare(a.dist, b.dist) })
	near = near[:min(len(near), len(w.slotTile))]
	keep := map[tileKey]bool{}
	for _, c := range near {
		keep[c.k] = true
	}
	for slot, k := range w.slotTile {
		if k >= 0 && !keep[k] {
			w.unassign(slot)
		}
	}
	for _, c := range near {
		if _, ok := w.slotOf[c.k]; !ok {
			slot := w.freeSlot(up)
			w.assign(c.k, slot)
			w.uploadTile(c.k, slot)
		}
	}
}

// freeSlot returns an empty slot, evicting the farthest tile if needed.
func (w *World) freeSlot(up vec3) int {
	far, farSlot := -1.0, -1
	for slot, k := range w.slotTile {
		if k < 0 {
			return slot
		}
		if d := tileCenter(k).sub(up).length(); d > far {
			far, farSlot = d, slot
		}
	}
	if farSlot >= 0 {
		w.unassign(farSlot)
	}
	return farSlot
}

func (w *World) indirIndex(k tileKey) int {
	f, tx, ty := k.split()
	x, y := (f%3)*tilesPerFace+tx, (f/3)*tilesPerFace+ty
	return (y*3*tilesPerFace + x) * 4
}

func (w *World) assign(k tileKey, slot int) {
	w.slotOf[k], w.slotTile[slot] = slot, k
	p := w.indirPix[w.indirIndex(k):]
	p[0], p[1], p[2], p[3] = byte(slot%atlasTiles), byte(slot/atlasTiles), 255, 255
}

func (w *World) unassign(slot int) {
	k := w.slotTile[slot]
	delete(w.slotOf, k)
	w.slotTile[slot] = -1
	p := w.indirPix[w.indirIndex(k):]
	p[0], p[1], p[2], p[3] = 0, 0, 0, 0
}

func (w *World) uploadTile(k tileKey, slot int) {
	t := w.tiles[k]
	buf := make([]byte, len(t))
	for i, v := range t {
		buf[i] = byte(v >> 8)
	}
	x, y := (slot%atlasTiles)*tileCells, (slot/atlasTiles)*tileCells
	rl.UpdateTextureRec(w.atlas, rl.NewRectangle(float32(x), float32(y), tileCells, tileCells), buf)
}

// TileBytes encodes the cleaned tiles for saving.
func (w *World) TileBytes() map[tileKey][]byte {
	out := make(map[tileKey][]byte, len(w.tiles))
	for k, t := range w.tiles {
		b := make([]byte, len(t)*2)
		for i, v := range t {
			binary.LittleEndian.PutUint16(b[i*2:], v)
		}
		out[k] = b
	}
	return out
}

func (w *World) SetTileBytes(tiles map[tileKey][]byte) {
	for k, b := range tiles {
		if len(b) != tileCells*tileCells*2 {
			continue
		}
		t := make([]uint16, tileCells*tileCells)
		for i := range t {
			t[i] = binary.LittleEndian.Uint16(b[i*2:])
		}
		w.tiles[k] = t
	}
}

func (w *World) CollectedKeys() []tileKey {
	keys := make([]tileKey, 0, len(w.collected))
	for k := range w.collected {
		keys = append(keys, k)
	}
	return keys
}

func (w *World) SetCollected(keys []tileKey) {
	for _, k := range keys {
		w.collected[k] = true
	}
}

// measure records how much of a tile has been cleaned, for the hologram.
func (w *World) measure(k tileKey) {
	sum := 0
	for _, v := range w.tiles[k] {
		sum += int(v)
	}
	cleaned := 1 - float64(sum)/float64(len(w.tiles[k])*fullDust)
	w.covPix[w.indirIndex(k)/4] = byte(math.Round(clamp01(cleaned) * 255))
	w.covDirty = true
}

// Coverage is the texture of cleaned fractions per tile.
func (w *World) Coverage() rl.Texture2D { return w.covTex }

// countGems counts the crystals of the whole of Mars, collected or not.
func (w *World) countGems() int {
	collected := w.collected
	w.collected = nil // gemIn skips collected ones: count them all
	n := 0
	for k := range tileKey(6 * tilesPerFace * tilesPerFace) {
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
	marsDust := 6 * faceCells * faceCells * gramsPerCell
	return Stats{
		Cleaned:      totalDust / marsDust,
		TilesVisited: len(w.tiles),
		TilesTotal:   6 * tilesPerFace * tilesPerFace,
		Gems:         len(w.collected),
		GemTotal:     w.totalGems,
	}
}
