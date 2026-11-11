package main

import (
	"image/color"
	"mars_sweeper/internal/game"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/planet"
	"math"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// far enough to dot the plains at mid distance

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
		d := vec3{float64(verts[i*3]), float64(verts[i*3+1]), float64(verts[i*3+2])}.Norm()
		r := 1 + 0.55*(mathx.Fbm3(d.Scale(1.4), seed, 3)-0.5) + 0.12*(mathx.Fbm3(d.Scale(4), seed+1, 2)-0.5)
		p := d.Scale(r)
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
		fn := pos[t+1].Sub(pos[t]).Cross(pos[t+2].Sub(pos[t]))
		if fn.Dot(pos[t]) < 0 {
			fn = fn.Scale(-1)
		}
		for j := range 3 {
			acc[key(pos[t+j])] = acc[key(pos[t+j])].Add(fn)
		}
	}
	for i, p := range pos {
		nr := acc[key(p)].Norm()
		verts[i*3], verts[i*3+1], verts[i*3+2] = float32(p.X), float32(p.Y), float32(p.Z)
		norms[i*3], norms[i*3+1], norms[i*3+2] = float32(nr.X), float32(nr.Y), float32(nr.Z)
	}
	rl.UpdateMeshBuffer(mesh, 0, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Vertices)), n*12), 0)
	rl.UpdateMeshBuffer(mesh, 2, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Normals)), n*12), 0)
	return mesh
}

// drawRocks draws the rocks around the player, one instanced call per shape.
// They shrink away at the edge of the drawing distance rather than pop.
func (gfx *Graphics) drawRocks(g *game.Game) {
	var mats [planet.RockShapes][]rl.Matrix
	for _, r := range g.NearRocks {
		up := r.Dir
		t1, t2 := mathx.TangentBasis(up)
		ax := t1.Scale(math.Cos(r.Yaw)).Add(t2.Scale(math.Sin(r.Yaw)))
		az := ax.Cross(up)
		ground := gfx.terrain.Surface(r.Dir)
		size := r.Size * (1 - mathx.Smoothstep(game.RockDrawDist*0.8, game.RockDrawDist, ground.Sub(g.Player.Pos).Length()))
		mats[r.Shape] = append(mats[r.Shape], basisMatrix(ground.Sub(up.Scale(size*0.12)), ax.Scale(size), up.Scale(size), az.Scale(size)))
	}
	gfx.drawInstanced(gfx.rocks, mats[:], func(k int) color.RGBA {
		v := 140 + 15*float64(k)
		return mathx.RGB(v, v*0.6, v*0.46) // rusty red, like the regolith
	})
}
