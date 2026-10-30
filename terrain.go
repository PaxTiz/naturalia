package main

import (
	"math"
	"math/rand/v2"
	"sync"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	moonRadius = 173.7 // the real Moon at 1/10 000 scale
	faceQuads  = 256   // terrain quads per cube face side: about one per meter
	chunkQuads = 64    // quads per mesh chunk side, keeps indices within 16 bits
	padRadius  = 7.0   // the landing pad is flat within this distance
)

// The base sits on the north pole, where "up" is +Y.
var basePos = vec3{0, moonRadius, 0}

type crater struct {
	dir           vec3
	r, depth, cos float64 // radius (m), depth (m), cosine of its reach
}

type terrainChunk struct {
	model  rl.Model
	center vec3 // unit direction of the chunk's middle, for culling
}

// Terrain is the Moon's relief: gentle rolling ground and wide shallow
// craters, smooth enough to drive on later.
type Terrain struct {
	seed    uint64
	craters []crater
	chunks  []terrainChunk
}

func NewTerrain(seed uint64) *Terrain {
	t := &Terrain{seed: seed}
	rng := rand.New(rand.NewPCG(seed, 2))
	for len(t.craters) < 140 {
		d := vec3{rng.NormFloat64(), rng.NormFloat64(), rng.NormFloat64()}.norm()
		r := 5 + math.Pow(rng.Float64(), 2)*25
		if arcDist(d, vec3{0, 1, 0}) < r*1.7+padRadius+4 {
			continue // keep the landing pad clear
		}
		t.craters = append(t.craters, crater{dir: d, r: r, depth: r * 0.12, cos: math.Cos(r * 1.7 / moonRadius)})
	}
	t.buildMeshes()
	return t
}

// arcDist is the distance in meters along the surface between two directions.
func arcDist(a, b vec3) float64 {
	return math.Acos(math.Max(-1, math.Min(1, a.dot(b)))) * moonRadius
}

// Relief is the ground altitude above the mean radius in direction d (unit).
func (t *Terrain) Relief(d vec3) float64 {
	p := d.scale(moonRadius)
	h := 2.2*(fbm3(p.scale(1.0/80), t.seed+11, 4)-0.5) + 0.5*(fbm3(p.scale(1.0/25), t.seed+12, 2)-0.5)
	for _, c := range t.craters {
		if d.dot(c.dir) < c.cos {
			continue
		}
		x := arcDist(d, c.dir) / c.r
		// Smooth bowl (zero slope at the rim) plus a soft raised lip.
		bowl := math.Max(0, 1-x*x)
		lip := (x - 1) / 0.35
		h += c.depth * (-bowl*bowl + 0.25*math.Exp(-lip*lip))
	}
	return h * smoothstep(padRadius, padRadius+6, arcDist(d, vec3{0, 1, 0}))
}

func (t *Terrain) SurfaceRadius(d vec3) float64 { return moonRadius + t.Relief(d) }

// Surface returns the ground point in direction d (any length).
func (t *Terrain) Surface(d vec3) vec3 {
	d = d.norm()
	return d.scale(t.SurfaceRadius(d))
}

// Altitude is the height of p above the ground right below it.
func (t *Terrain) Altitude(p vec3) float64 { return p.length() - t.SurfaceRadius(p.norm()) }

// Raycast marches along the ray and returns the first ground hit within maxDist.
func (t *Terrain) Raycast(from, dir vec3, maxDist float64) (vec3, bool) {
	const step = 0.1
	prev := from
	for d := step; d <= maxDist; d += step {
		p := from.add(dir.scale(d))
		if t.Altitude(p) <= 0 {
			// Refine between the last point above ground and this one.
			lo, hi := prev, p
			for range 8 {
				mid := lo.lerp(hi, 0.5)
				if t.Altitude(mid) <= 0 {
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

// buildMeshes samples the relief on each cube face (in parallel), then fills
// raylib-allocated plane meshes with the sphere's positions and normals.
func (t *Terrain) buildMeshes() {
	const g = faceQuads + 3 // vertex grid with a 1-vertex border for normals
	var grids [6][]vec3
	var wg sync.WaitGroup
	for f := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pos := make([]vec3, g*g)
			for j := range g {
				for i := range g {
					a := float64(i-1)/faceQuads*2 - 1
					b := float64(j-1)/faceQuads*2 - 1
					d := faceDir(f, a, b)
					pos[j*g+i] = d.scale(t.SurfaceRadius(d))
				}
			}
			grids[f] = pos
		}()
	}
	wg.Wait()

	for _, pos := range grids {
		at := func(i, j int) vec3 { return pos[(j+1)*g+i+1] }
		for cj := 0; cj < faceQuads/chunkQuads; cj++ {
			for ci := 0; ci < faceQuads/chunkQuads; ci++ {
				mesh := rl.GenMeshPlane(1, 1, chunkQuads, chunkQuads)
				n := int(mesh.VertexCount)
				verts := unsafe.Slice(mesh.Vertices, n*3)
				norms := unsafe.Slice(mesh.Normals, n*3)
				// GenMeshPlane lays vertices out row by row: index = x + z*(res+1).
				for vz := 0; vz <= chunkQuads; vz++ {
					for vx := 0; vx <= chunkQuads; vx++ {
						i, j := ci*chunkQuads+vx, cj*chunkQuads+vz
						p := at(i, j)
						nrm := at(i+1, j).sub(at(i-1, j)).cross(at(i, j+1).sub(at(i, j-1))).norm()
						if nrm.dot(p) < 0 {
							nrm = nrm.scale(-1)
						}
						k := (vz*(chunkQuads+1) + vx) * 3
						verts[k], verts[k+1], verts[k+2] = float32(p.X), float32(p.Y), float32(p.Z)
						norms[k], norms[k+1], norms[k+2] = float32(nrm.X), float32(nrm.Y), float32(nrm.Z)
					}
				}
				rl.UpdateMeshBuffer(mesh, 0, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Vertices)), n*12), 0)
				rl.UpdateMeshBuffer(mesh, 2, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Normals)), n*12), 0)
				mid := at(ci*chunkQuads+chunkQuads/2, cj*chunkQuads+chunkQuads/2).norm()
				t.chunks = append(t.chunks, terrainChunk{model: rl.LoadModelFromMesh(mesh), center: mid})
			}
		}
	}
}
