package main

import (
	"image/color"
	"mars_sweeper/internal/mathx"
	"math"

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
// wide. None on the base's pad nor on the polar ice.
func (g *Game) pebblesIn(k tileKey) []pebble {
	f, tx, ty := k.split()
	var out []pebble
	for i := range pebblesPerTile {
		s := g.world.Seed + 900 + uint64(i)*5
		a := (float64(tx*tileCells) + mathx.Hash3(f, tx, ty, s)*tileCells) / faceCells * 2
		b := (float64(ty*tileCells) + mathx.Hash3(f, tx, ty, s+1)*tileCells) / faceCells * 2
		d := mathx.FaceDir(f, a-1, b-1)
		if arcDist(d, baseUp) < baseReach+2 || g.terrain.Ice(d) > 0.3 {
			continue
		}
		t1, t2 := mathx.TangentBasis(d)
		yaw := mathx.Hash3(f, tx, ty, s+3) * 2 * math.Pi
		ax := t1.Scale(math.Cos(yaw)).Add(t2.Scale(math.Sin(yaw)))
		out = append(out, pebble{
			dir:    d,
			ground: g.terrain.Surface(d),
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
func (g *Game) drawPebbles() {
	gfx := g.gfx
	if g.pebbles == nil {
		g.pebbles = map[tileKey][]pebble{}
	}
	near := map[tileKey]bool{}
	var mats [pebbleShapes][]rl.Matrix
	tilesNear(g.player.Pos, pebbleDist, func(k tileKey) {
		near[k] = true
		ps, ok := g.pebbles[k]
		if !ok {
			ps = g.pebblesIn(k)
			g.pebbles[k] = ps
		}
		for _, p := range ps {
			dist := p.ground.Sub(g.player.Pos).Length()
			if dist > pebbleDist {
				continue
			}
			// Shrink away near the edge rather than pop.
			size := p.size * (1 - mathx.Smoothstep(pebbleDist*0.75, pebbleDist, dist))
			ground := p.ground.Sub(p.dir.Scale(size * (0.15 + 0.45*g.world.DustAt(p.dir))))
			mats[p.shape] = append(mats[p.shape], basisMatrix(ground, p.ax.Scale(size), p.dir.Scale(size), p.az.Scale(size)))
		}
	})
	for k := range g.pebbles {
		if !near[k] {
			delete(g.pebbles, k)
		}
	}

	g.dbg.pebbles = 0
	for _, m := range mats {
		g.dbg.pebbles += len(m)
	}
	gfx.drawInstanced(gfx.pebbles, mats[:], func(k int) color.RGBA {
		v := 120 + 30*float64(k)
		return mathx.RGB(v, v*0.58, v*0.45)
	})
}

// drawInstanced draws rock models, each at all of its transforms in one
// call, tinted by tone(shape).
func (gfx *Graphics) drawInstanced(models []rl.Model, mats [][]rl.Matrix, tone func(k int) color.RGBA) {
	gfx.setLighting(litSun, vec3{})
	gfx.setMat(mtRock, float32(1/regolithTile), false)
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.surface, []float32{0.1, 0}, rl.ShaderUniformVec2)
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.instanced, []float32{1}, rl.ShaderUniformFloat)
	for k, m := range models {
		if len(mats[k]) == 0 {
			continue
		}
		mat := m.GetMaterials()[0]
		mat.GetMap(rl.MapAlbedo).Color = tone(k)
		rl.DrawMeshInstanced(m.GetMeshes()[0], mat, mats[k], len(mats[k]))
		mat.GetMap(rl.MapAlbedo).Color = rl.White
	}
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.instanced, []float32{0}, rl.ShaderUniformFloat)
	gfx.setMat(mtPlain, 1, false)
}

// basisMatrix sends a model's X, Y and Z axes to ax, ay and az, at pos.
func basisMatrix(pos, ax, ay, az vec3) rl.Matrix {
	return rl.Matrix{
		M0: float32(ax.X), M1: float32(ax.Y), M2: float32(ax.Z),
		M4: float32(ay.X), M5: float32(ay.Y), M6: float32(ay.Z),
		M8: float32(az.X), M9: float32(az.Y), M10: float32(az.Z),
		M12: float32(pos.X), M13: float32(pos.Y), M14: float32(pos.Z), M15: 1,
	}
}
