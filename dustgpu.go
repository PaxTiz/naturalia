package main

import (
	"cmp"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/planet"
	"math"
	"slices"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	atlasTiles   = 16    // the GPU keeps a 16×16 cache of tiles...
	cacheRadius  = 300.0 // ...picked among the touched ones within this distance
	cacheRefresh = 30    // frames between cache refreshes
)

// dustGPU is the dust as the terrain shader sees it: a cache of the touched
// tiles near the player, a table telling where each tile sits in it, and
// the cleaned fraction of every tile for the hologram of the map table.
type dustGPU struct {
	atlas     rl.Texture2D
	indir     rl.Texture2D
	indirPix  []byte
	slotOf    map[planet.TileKey]int
	slotTile  []planet.TileKey // -1 when free
	cacheTick int

	covTex   rl.Texture2D
	covPix   []byte
	covDirty bool
}

// Coverage is the texture of cleaned fractions per tile.
func (c *dustGPU) Coverage() rl.Texture2D { return c.covTex }

// Upload refreshes the GPU cache around the player and returns the tile
// atlas and the lookup table.
func (c *dustGPU) Upload(w *planet.World, player vec3) (rl.Texture2D, rl.Texture2D) {
	if c.atlas.ID == 0 {
		c.init(w)
	}
	up := player.Norm()
	c.cacheTick++
	if c.cacheTick >= cacheRefresh {
		c.cacheTick = 0
		c.refreshCache(w, up)
	}
	for _, k := range w.Dirty() {
		slot, ok := c.slotOf[k]
		if !ok {
			// Freshly touched tile: take a slot right away.
			if slot = c.freeSlot(up); slot < 0 {
				continue
			}
			c.assign(k, slot)
		}
		c.uploadTile(w, k, slot)
		c.covPix[c.indirIndex(k)/4] = byte(math.Round(mathx.Clamp01(w.Cleaned(k)) * 255))
		c.covDirty = true
		w.MarkUploaded(k)
	}
	if c.covDirty {
		rl.UpdateTexture(c.covTex, c.covPix)
		c.covDirty = false
	}
	if c.indirPix != nil {
		rl.UpdateTexture(c.indir, c.indirPix)
	}
	return c.atlas, c.indir
}

func (c *dustGPU) init(w *planet.World) {
	img := rl.GenImageColor(atlasTiles*planet.TileCells, atlasTiles*planet.TileCells, rl.White)
	rl.ImageFormat(img, rl.UncompressedGrayscale)
	c.atlas = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.SetTextureFilter(c.atlas, rl.FilterBilinear)

	img = rl.GenImageColor(3*planet.TilesPerFace, 2*planet.TilesPerFace, rl.Blank)
	c.indir = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.SetTextureFilter(c.indir, rl.FilterPoint)

	c.indirPix = make([]byte, 3*planet.TilesPerFace*2*planet.TilesPerFace*4)

	img = rl.GenImageColor(3*planet.TilesPerFace, 2*planet.TilesPerFace, rl.Black)
	rl.ImageFormat(img, rl.UncompressedGrayscale)
	c.covTex = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	c.covPix = make([]byte, 3*planet.TilesPerFace*2*planet.TilesPerFace)
	c.slotOf = map[planet.TileKey]int{}
	c.slotTile = make([]planet.TileKey, atlasTiles*atlasTiles)
	for i := range c.slotTile {
		c.slotTile[i] = -1
	}
	w.MarkAllDirty()
}

// refreshCache keeps the touched tiles nearest to the player in the cache.
func (c *dustGPU) refreshCache(w *planet.World, up vec3) {
	type cand struct {
		k    planet.TileKey
		dist float64
	}
	var near []cand
	for _, k := range w.Touched() {
		if d := planet.TileCenter(k).Sub(up).Length() * planet.Radius; d < cacheRadius {
			near = append(near, cand{k, d})
		}
	}
	slices.SortFunc(near, func(a, b cand) int { return cmp.Compare(a.dist, b.dist) })
	near = near[:min(len(near), len(c.slotTile))]
	keep := map[planet.TileKey]bool{}
	for _, n := range near {
		keep[n.k] = true
	}
	for slot, k := range c.slotTile {
		if k >= 0 && !keep[k] {
			c.unassign(slot)
		}
	}
	for _, n := range near {
		if _, ok := c.slotOf[n.k]; !ok {
			slot := c.freeSlot(up)
			c.assign(n.k, slot)
			c.uploadTile(w, n.k, slot)
		}
	}
}

// freeSlot returns an empty slot, evicting the farthest tile if needed.
func (c *dustGPU) freeSlot(up vec3) int {
	far, farSlot := -1.0, -1
	for slot, k := range c.slotTile {
		if k < 0 {
			return slot
		}
		if d := planet.TileCenter(k).Sub(up).Length(); d > far {
			far, farSlot = d, slot
		}
	}
	if farSlot >= 0 {
		c.unassign(farSlot)
	}
	return farSlot
}

func (c *dustGPU) indirIndex(k planet.TileKey) int {
	f, tx, ty := k.Split()
	x, y := (f%3)*planet.TilesPerFace+tx, (f/3)*planet.TilesPerFace+ty
	return (y*3*planet.TilesPerFace + x) * 4
}

func (c *dustGPU) assign(k planet.TileKey, slot int) {
	c.slotOf[k], c.slotTile[slot] = slot, k
	p := c.indirPix[c.indirIndex(k):]
	p[0], p[1], p[2], p[3] = byte(slot%atlasTiles), byte(slot/atlasTiles), 255, 255
}

func (c *dustGPU) unassign(slot int) {
	k := c.slotTile[slot]
	delete(c.slotOf, k)
	c.slotTile[slot] = -1
	p := c.indirPix[c.indirIndex(k):]
	p[0], p[1], p[2], p[3] = 0, 0, 0, 0
}

func (c *dustGPU) uploadTile(w *planet.World, k planet.TileKey, slot int) {
	t := w.Tile(k)
	buf := make([]byte, len(t))
	for i, v := range t {
		buf[i] = byte(v >> 8)
	}
	x, y := (slot%atlasTiles)*planet.TileCells, (slot/atlasTiles)*planet.TileCells
	rl.UpdateTextureRec(c.atlas, rl.NewRectangle(float32(x), float32(y), planet.TileCells, planet.TileCells), buf)
}
