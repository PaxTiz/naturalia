package render

import (
	"image/color"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/engine/gpu"
	"naturalia/internal/engine/mathx"
	"naturalia/internal/engine/mesh"
	"naturalia/internal/game"
	"naturalia/internal/planet"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	pebbleShapes   = 3
	pebblesPerTile = 900  // about 0.7 per square meter
	pebbleDist     = 24.0 // drawn within this many meters of the player
)

// pebble is a small stone strewn on the ground, drawn only around the
// player, many at once with instancing.
type pebble struct {
	dir    vec3
	ground vec3 // surface point, cached: the relief is costly to sample
	size   float64
	shape  int
	ax, az vec3 // its horizontal axes, turned at random
}

// pebblesIn places the pebbles of a dust tile, mostly a few centimeters
// wide. None on the base's pad, under a bootcamp nor on the polar ice.
func (gfx *Graphics) pebblesIn(g *game.Game, k planet.TileKey) []pebble {
	f, tx, ty := k.Split()
	var out []pebble
	for i := range pebblesPerTile {
		s := g.World.Seed + 900 + uint64(i)*5
		a := (float64(tx*planet.TileCells) + mathx.Hash3(f, tx, ty, s)*planet.TileCells) / planet.FaceCells * 2
		b := (float64(ty*planet.TileCells) + mathx.Hash3(f, tx, ty, s+1)*planet.TileCells) / planet.FaceCells * 2
		d := mathx.FaceDir(f, a-1, b-1)
		if planet.ArcDist(d, planet.BaseUp) < base.Reach+2 || base.Covered(base.ToLocal(d.Scale(planet.Radius))) || gfx.Terrain.Ice(d) > 0.3 || gfx.Terrain.OnPad(d, 0.5) || gfx.Terrain.Built(d, 0.2) {
			continue
		}
		t1, t2 := mathx.TangentBasis(d)
		yaw := mathx.Hash3(f, tx, ty, s+3) * 2 * math.Pi
		ax := t1.Scale(math.Cos(yaw)).Add(t2.Scale(math.Sin(yaw)))
		out = append(out, pebble{
			dir:    d,
			ground: gfx.Terrain.Surface(d),
			size:   0.025 + 0.11*math.Pow(mathx.Hash3(f, tx, ty, s+2), 2.5),
			shape:  int(mathx.Hash3(f, tx, ty, s+4) * pebbleShapes),
			ax:     ax,
			az:     ax.Cross(d),
		})
	}
	return out
}

// drawPebbles draws the pebbles near the player, one instanced call per
// shape. They sink into thick dust and come out where it is swept.
func (gfx *Graphics) drawPebbles(g *game.Game) {
	if flat := g.World.Relief.Flat(); gfx.nearPebbles == nil || flat != gfx.pebbleFlat {
		gfx.nearPebbles = map[planet.TileKey][]pebble{} // none under what was set down
		gfx.pebbleFlat = flat
	}
	near := map[planet.TileKey]bool{}
	var mats [pebbleShapes][]rl.Matrix
	planet.TilesNear(g.Player.Pos, pebbleDist, func(k planet.TileKey) {
		near[k] = true
		ps, ok := gfx.nearPebbles[k]
		if !ok {
			ps = gfx.pebblesIn(g, k)
			gfx.nearPebbles[k] = ps
		}
		for _, p := range ps {
			dist := p.ground.Sub(g.Player.Pos).Length()
			if dist > pebbleDist {
				continue
			}
			// Shrink away near the edge rather than pop.
			size := p.size * (1 - mathx.Smoothstep(pebbleDist*0.75, pebbleDist, dist))
			ground := p.ground.Sub(p.dir.Scale(size * (0.15 + 0.45*g.World.DustAt(p.dir))))
			mats[p.shape] = append(mats[p.shape], gpu.Basis(ground, p.ax.Scale(size), p.dir.Scale(size), p.az.Scale(size)))
		}
	})
	for k := range gfx.nearPebbles {
		if !near[k] {
			delete(gfx.nearPebbles, k)
		}
	}

	gfx.stats.Pebbles = 0
	for _, m := range mats {
		gfx.stats.Pebbles += len(m)
	}
	gfx.drawInstanced(gfx.pebbles, mats[:], func(k int) color.RGBA {
		v := 120 + 30*float64(k)
		return mathx.RGB(v, v*0.58, v*0.45)
	})
}

// drawInstanced draws rock models, each at all of its transforms in one
// call, tinted by tone(shape).
func (gfx *Graphics) drawInstanced(models []rl.Model, mats [][]rl.Matrix, tone func(k int) color.RGBA) {
	gfx.setLighting(mesh.LitSun, vec3{})
	gfx.setMat(content.MtRock, float32(1/regolithTile), false)
	gfx.objectSh.Vec2("surface", 0.1, 0)
	gfx.objectSh.Float("instanced", 1)
	for k, m := range models {
		if len(mats[k]) == 0 {
			continue
		}
		mat := m.GetMaterials()[0]
		mat.GetMap(rl.MapAlbedo).Color = tone(k)
		rl.DrawMeshInstanced(m.GetMeshes()[0], mat, mats[k], len(mats[k]))
		mat.GetMap(rl.MapAlbedo).Color = rl.White
	}
	gfx.objectSh.Float("instanced", 0)
	gfx.setMat(content.MtPlain, 1, false)
}
