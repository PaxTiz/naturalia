package main

import (
	"math"
	"math/rand/v2"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	// The mesh extends well beyond the playable area so mountains fill the horizon.
	terrainSize   = 320.0
	terrainOrigin = -(terrainSize - worldSize) / 2
	terrainRes    = 321 // vertices per side: one every meter
	terrainStep   = terrainSize / (terrainRes - 1)
	padHeight     = 5.0 // altitude of the flattened landing pad
)

// The base sits in the middle of the playable area.
var basePos = vec3{worldSize / 2, padHeight, worldSize / 2}

// Terrain is the lunar relief: a smooth heightfield kept on the CPU too, so
// walking and aiming match exactly what is drawn.
type Terrain struct {
	h     []float64
	model rl.Model
}

type crater struct{ x, z, r, depth float64 }

func NewTerrain(seed uint64) *Terrain {
	rng := rand.New(rand.NewPCG(seed, 2))
	var craters []crater
	for len(craters) < 70 {
		c := crater{
			x: terrainOrigin + rng.Float64()*terrainSize,
			z: terrainOrigin + rng.Float64()*terrainSize,
			r: 3 + math.Pow(rng.Float64(), 2.2)*20,
		}
		c.depth = c.r * 0.32
		if math.Hypot(c.x-basePos.X, c.z-basePos.Z) < c.r*1.6+12 {
			continue // keep the landing pad clear
		}
		craters = append(craters, c)
	}

	t := &Terrain{h: make([]float64, terrainRes*terrainRes)}
	for iz := range terrainRes {
		for ix := range terrainRes {
			x, z := terrainOrigin+float64(ix)*terrainStep, terrainOrigin+float64(iz)*terrainStep
			t.h[iz*terrainRes+ix] = reliefAt(x, z, seed, craters)
		}
	}
	t.buildMesh()
	return t
}

func reliefAt(x, z float64, seed uint64, craters []crater) float64 {
	h := 4 + 6*fbm(x/45, z/45, seed+11, 4) + 0.8*fbm(x/9, z/9, seed+12, 3)
	for _, c := range craters {
		d := math.Hypot(x-c.x, z-c.z) / c.r
		if d > 2 {
			continue
		}
		// Smooth bowl (zero slope at the rim) plus a soft raised lip.
		bowl := math.Max(0, 1-d*d)
		lip := (d - 1) / 0.3
		h += c.depth * (-bowl*bowl + 0.3*math.Exp(-lip*lip))
	}
	h = padHeight + (h-padHeight)*smoothstep(7, 13, math.Hypot(x-basePos.X, z-basePos.Z))

	// Mountains rise outside the playable area.
	ox := math.Max(0, math.Max(-x, x-worldSize))
	oz := math.Max(0, math.Max(-z, z-worldSize))
	out := math.Hypot(ox, oz)
	h += smoothstep(10, 110, out) * (14 + 30*fbm(x/55, z/55, seed+21, 5))
	return h
}

func (t *Terrain) at(ix, iz int) float64 {
	ix = max(0, min(terrainRes-1, ix))
	iz = max(0, min(terrainRes-1, iz))
	return t.h[iz*terrainRes+ix]
}

// buildMesh lets raylib allocate a heightmap mesh, then overwrites its
// heights with full-precision floats and smooth per-vertex normals.
func (t *Terrain) buildMesh() {
	img := rl.GenImageColor(terrainRes, terrainRes, rl.Black)
	mesh := rl.GenMeshHeightmap(*img, rl.NewVector3(terrainSize, 1, terrainSize))
	rl.UnloadImage(img)

	n := int(mesh.VertexCount)
	verts := unsafe.Slice(mesh.Vertices, n*3)
	norms := unsafe.Slice(mesh.Normals, n*3)
	for i := range n {
		ix := int(math.Round(float64(verts[i*3]) / terrainStep))
		iz := int(math.Round(float64(verts[i*3+2]) / terrainStep))
		verts[i*3+1] = float32(t.at(ix, iz))
		nrm := vec3{t.at(ix-1, iz) - t.at(ix+1, iz), 2 * terrainStep, t.at(ix, iz-1) - t.at(ix, iz+1)}.norm()
		norms[i*3], norms[i*3+1], norms[i*3+2] = float32(nrm.X), float32(nrm.Y), float32(nrm.Z)
	}
	rl.UpdateMeshBuffer(mesh, 0, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Vertices)), n*12), 0)
	rl.UpdateMeshBuffer(mesh, 2, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Normals)), n*12), 0)
	t.model = rl.LoadModelFromMesh(mesh)
}

// Height returns the ground altitude at (x, z), interpolated on the same
// triangles raylib's GenMeshHeightmap builds.
func (t *Terrain) Height(x, z float64) float64 {
	gx := math.Max(0, math.Min(terrainRes-1.0001, (x-terrainOrigin)/terrainStep))
	gz := math.Max(0, math.Min(terrainRes-1.0001, (z-terrainOrigin)/terrainStep))
	ix, iz := int(gx), int(gz)
	fx, fz := gx-float64(ix), gz-float64(iz)
	h00, h10 := t.at(ix, iz), t.at(ix+1, iz)
	h01, h11 := t.at(ix, iz+1), t.at(ix+1, iz+1)
	if fx+fz <= 1 {
		return h00 + (h10-h00)*fx + (h01-h00)*fz
	}
	return h11 + (h01-h11)*(1-fx) + (h10-h11)*(1-fz)
}

// Raycast marches along the ray and returns the first ground hit within maxDist.
func (t *Terrain) Raycast(from, dir vec3, maxDist float64) (vec3, bool) {
	const step = 0.1
	prev := from
	for d := step; d <= maxDist; d += step {
		p := from.add(dir.scale(d))
		if p.Y <= t.Height(p.X, p.Z) {
			// Refine between the last point above ground and this one.
			lo, hi := prev, p
			for range 8 {
				mid := lo.lerp(hi, 0.5)
				if mid.Y <= t.Height(mid.X, mid.Z) {
					hi = mid
				} else {
					lo = mid
				}
			}
			return hi, true
		}
		prev = p
	}
	return vec3{}, false
}
