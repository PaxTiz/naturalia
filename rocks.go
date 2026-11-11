package main

import (
	"image/color"
	"math"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	rockShapes   = 5
	rockDrawDist = 150.0 // far enough to dot the plains at mid distance
)

// rockModels sculpts count rock shapes with the given sphere resolution.
func rockModels(sh rl.Shader, maps []rl.Texture2D, count int, seed uint64, rings, slices int) []rl.Model {
	var models []rl.Model
	for k := range count {
		m := rl.LoadModelFromMesh(sculptRock(seed+uint64(k)*17, rings, slices))
		mat := &m.GetMaterials()[0]
		mat.Shader = sh
		for i, t := range maps {
			rl.SetMaterialTexture(mat, int32(i), t)
		}
		models = append(models, m)
	}
	return models
}

// sculptRock makes a sphere bumped by noise, with a flattened base so it
// sits on the ground.
func sculptRock(seed uint64, rings, slices int) rl.Mesh {
	mesh := rl.GenMeshSphere(1, rings, slices)
	n := int(mesh.VertexCount)
	verts := unsafe.Slice(mesh.Vertices, n*3)
	norms := unsafe.Slice(mesh.Normals, n*3)
	pos := make([]vec3, n)
	for i := range n {
		d := vec3{float64(verts[i*3]), float64(verts[i*3+1]), float64(verts[i*3+2])}.norm()
		r := 1 + 0.55*(fbm3(d.scale(1.4), seed, 3)-0.5) + 0.12*(fbm3(d.scale(4), seed+1, 2)-0.5)
		p := d.scale(r)
		p.Y *= 0.68
		if p.Y < -0.15 {
			p.Y = -0.15 + (p.Y+0.15)*0.25 // flat-ish bottom
		}
		pos[i] = p
	}
	// Smooth normals: average the faces around each shared position.
	key := func(p vec3) [3]int32 {
		return [3]int32{int32(math.Round(p.X * 1e4)), int32(math.Round(p.Y * 1e4)), int32(math.Round(p.Z * 1e4))}
	}
	acc := map[[3]int32]vec3{}
	for t := 0; t+2 < n; t += 3 {
		fn := pos[t+1].sub(pos[t]).cross(pos[t+2].sub(pos[t]))
		if fn.dot(pos[t]) < 0 {
			fn = fn.scale(-1)
		}
		for j := range 3 {
			acc[key(pos[t+j])] = acc[key(pos[t+j])].add(fn)
		}
	}
	for i, p := range pos {
		nr := acc[key(p)].norm()
		verts[i*3], verts[i*3+1], verts[i*3+2] = float32(p.X), float32(p.Y), float32(p.Z)
		norms[i*3], norms[i*3+1], norms[i*3+2] = float32(nr.X), float32(nr.Y), float32(nr.Z)
	}
	rl.UpdateMeshBuffer(mesh, 0, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Vertices)), n*12), 0)
	rl.UpdateMeshBuffer(mesh, 2, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Normals)), n*12), 0)
	return mesh
}

// rock is a boulder lying on the ground.
type rock struct {
	dir   vec3 // from Mars' center
	size  float64
	shape int
	yaw   float64
	tone  float64
}

// rocksIn lists the rocks of a dust tile: placed procedurally, mostly small
// stones, now and then a big boulder. None around the base
// nor on the polar ice.
func (w *World) rocksIn(k tileKey) []rock {
	f, tx, ty := k.split()
	count := int(hash3(f, tx, ty, w.Seed+201) * 8 * terrain().Rocks)
	var rocks []rock
	for i := range count {
		s := w.Seed + 210 + uint64(i)*7
		ci := tx*tileCells + int(hash3(f, tx, ty, s)*tileCells)
		cj := ty*tileCells + int(hash3(f, tx, ty, s+1)*tileCells)
		size := 0.2 + 0.7*math.Pow(hash3(f, tx, ty, s+2), 3)
		if hash3(f, tx, ty, s+3) < 0.1 {
			size = 1.4 + 1.3*hash3(f, tx, ty, s+4) // a big boulder
		}
		d := cellDir(f, ci, cj)
		if arcDist(d, baseUp) < baseReach+size+4 || (Relief{seed: w.Seed}).Ice(d) > 0.3 {
			continue
		}
		rocks = append(rocks, rock{d, size, int(hash3(f, tx, ty, s+5) * rockShapes), hash3(f, tx, ty, s+6) * 2 * math.Pi, hash3(f, tx, ty, s+7)})
	}
	return rocks
}

// RocksNear lists the rocks within dist meters of c.
func (w *World) RocksNear(c vec3, dist float64) []rock {
	c = c.norm()
	var out []rock
	tilesNear(c, dist, func(k tileKey) {
		for _, r := range w.rocksIn(k) {
			if r.dir.sub(c).length()*marsRadius <= dist {
				out = append(out, r)
			}
		}
	})
	return out
}

// drawRocks draws the rocks around the player, one instanced call per shape.
// They shrink away at the edge of the drawing distance rather than pop.
func (g *Game) drawRocks() {
	gfx := g.gfx
	var mats [rockShapes][]rl.Matrix
	for _, r := range g.nearRocks {
		up := r.dir
		t1, t2 := tangentBasis(up)
		ax := t1.scale(math.Cos(r.yaw)).add(t2.scale(math.Sin(r.yaw)))
		az := ax.cross(up)
		ground := g.terrain.Surface(r.dir)
		size := r.size * (1 - smoothstep(rockDrawDist*0.8, rockDrawDist, ground.sub(g.player.Pos).length()))
		mats[r.shape] = append(mats[r.shape], basisMatrix(ground.sub(up.scale(size*0.12)), ax.scale(size), up.scale(size), az.scale(size)))
	}
	gfx.drawInstanced(gfx.rocks, mats[:], func(k int) color.RGBA {
		v := 140 + 15*float64(k)
		return rgb(v, v*0.6, v*0.46) // rusty red, like the regolith
	})
}

// collideRocks keeps the player out of the bigger rocks.
func (g *Game) collideRocks() {
	for _, r := range g.nearRocks {
		if r.size < 0.6 {
			continue
		}
		g.player.pushOut(g.terrain.Surface(r.dir), r.size*0.85)
	}
}
