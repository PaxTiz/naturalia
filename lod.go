package main

import (
	"math"
	"runtime"
	"slices"
	"time"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	nodeQuads      = 32   // quads per node side
	maxLevel       = 7    // deepest level: ~0.67 m between vertices
	splitFactor    = 2.5  // split a node when closer than this many node sizes
	uploadsPerTick = 16   // node meshes sent to the GPU per frame at most
	maxLoadedNodes = 1000 // older unused meshes get freed beyond this
)

// lodNode is a square patch of one cube face, refined into 4 children
// when the player gets close.
type lodNode struct {
	face, level   int
	a0, b0, size  float64 // face coordinates of its lower corner and its side
	center        vec3    // unit direction of its middle
	centerPos     vec3    // ground point at its middle
	arc, angulRad float64 // side in meters, angular radius bounding it
	children      []*lodNode
	mesh          rl.Mesh
	state         int // nodeEmpty, nodePending or nodeReady
	lastUsed      uint64
}

const (
	nodeEmpty = iota
	nodePending
	nodeReady
)

type nodeData struct {
	n            *lodNode
	verts, norms []float32
}

// Terrain draws the relief with a level of detail that follows the player:
// fine near them, coarse far away. Meshes are computed by background
// workers and uploaded on the main thread.
type Terrain struct {
	Relief
	roots   [6]*lodNode
	jobs    chan *lodNode
	results chan nodeData
	loaded  []*lodNode
	pending int
	// Freed meshes, reused for new nodes: they all share the same layout.
	// (raylib-go's UnloadMesh is broken without cgo, and recycling is faster.)
	pool  []rl.Mesh
	frame uint64
}

func NewTerrain(seed uint64) *Terrain {
	t := &Terrain{
		Relief:  newRelief(seed),
		jobs:    make(chan *lodNode, 8192),
		results: make(chan nodeData, 8192),
	}
	for range runtime.NumCPU() {
		go func() {
			for n := range t.jobs {
				t.results <- t.build(n)
			}
		}()
	}
	for f := range 6 {
		t.roots[f] = t.newNode(f, 0, -1, -1, 2)
		t.request(t.roots[f])
	}
	return t
}

func (t *Terrain) newNode(face, level int, a0, b0, size float64) *lodNode {
	n := &lodNode{face: face, level: level, a0: a0, b0: b0, size: size}
	n.center = faceDir(face, a0+size/2, b0+size/2)
	n.centerPos = t.Surface(n.center)
	ang := size * math.Pi / 4 // equiangular faces: coordinates are angles
	n.arc = ang * moonRadius
	n.angulRad = ang * 0.75
	return n
}

func (t *Terrain) request(n *lodNode) {
	if n.state != nodeEmpty {
		return
	}
	n.state = nodePending
	t.pending++
	t.jobs <- n
}

// build computes a node's vertices: a grid sampled on the sphere plus a
// skirt hanging below its border, which hides cracks between levels.
func (t *Terrain) build(n *lodNode) nodeData {
	const g = nodeQuads + 3 // grid with a 1-vertex border, for normals
	pos := make([]vec3, g*g)
	for j := range g {
		for i := range g {
			d := faceDir(n.face, n.a0+n.size*float64(i-1)/nodeQuads, n.b0+n.size*float64(j-1)/nodeQuads)
			pos[j*g+i] = d.scale(t.SurfaceRadius(d))
		}
	}
	at := func(i, j int) vec3 { return pos[(j+1)*g+i+1] }

	// The mesh is a raylib plane of (nodeQuads+3)² vertices: its outer ring
	// becomes the skirt.
	const side = nodeQuads + 3
	verts := make([]float32, side*side*3)
	norms := make([]float32, side*side*3)
	skirt := n.arc/nodeQuads*1.5 + 0.5
	for vz := range side {
		for vx := range side {
			i := max(0, min(nodeQuads, vx-1))
			j := max(0, min(nodeQuads, vz-1))
			p := at(i, j)
			nrm := at(i+1, j).sub(at(i-1, j)).cross(at(i, j+1).sub(at(i, j-1))).norm()
			if nrm.dot(p) < 0 {
				nrm = nrm.scale(-1)
			}
			if vx != i+1 || vz != j+1 {
				p = p.sub(p.norm().scale(skirt))
			}
			k := (vz*side + vx) * 3
			verts[k], verts[k+1], verts[k+2] = float32(p.X), float32(p.Y), float32(p.Z)
			norms[k], norms[k+1], norms[k+2] = float32(nrm.X), float32(nrm.Y), float32(nrm.Z)
		}
	}
	return nodeData{n, verts, norms}
}

// upload turns finished node data into GPU meshes, a few per frame.
func (t *Terrain) upload(limit int) {
	for range limit {
		select {
		case r := <-t.results:
			t.pending--
			var mesh rl.Mesh
			if k := len(t.pool); k > 0 {
				mesh, t.pool = t.pool[k-1], t.pool[:k-1]
			} else {
				mesh = rl.GenMeshPlane(1, 1, nodeQuads+2, nodeQuads+2)
			}
			n := int(mesh.VertexCount)
			copy(unsafe.Slice(mesh.Vertices, n*3), r.verts)
			copy(unsafe.Slice(mesh.Normals, n*3), r.norms)
			rl.UpdateMeshBuffer(mesh, 0, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Vertices)), n*12), 0)
			rl.UpdateMeshBuffer(mesh, 2, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Normals)), n*12), 0)
			r.n.mesh, r.n.state = mesh, nodeReady
			t.loaded = append(t.loaded, r.n)
		default:
			return
		}
	}
}

// Visible returns the nodes to draw this frame for a viewer at eye, and
// queues the meshes it is missing.
func (t *Terrain) Visible(eye vec3) []*lodNode {
	t.frame++
	t.upload(uploadsPerTick)
	// Beyond this angle around the Moon, even mountain tops are below the horizon.
	horizon := math.Acos(math.Min(1, moonRadius/eye.length())) + math.Acos(moonRadius/(moonRadius+maxMountain))
	var draw []*lodNode
	for _, r := range t.roots {
		t.visit(r, eye, eye.norm(), horizon, &draw)
	}
	t.evict()
	return draw
}

func (t *Terrain) visit(n *lodNode, eye, up vec3, horizon float64, draw *[]*lodNode) {
	if math.Acos(math.Max(-1, math.Min(1, n.center.dot(up))))-n.angulRad > horizon {
		return
	}
	n.lastUsed = t.frame
	wantSplit := n.level < maxLevel && eye.sub(n.centerPos).length() < splitFactor*n.arc
	if wantSplit || n.state != nodeReady {
		if n.level < maxLevel && n.children == nil && wantSplit {
			h := n.size / 2
			n.children = []*lodNode{
				t.newNode(n.face, n.level+1, n.a0, n.b0, h),
				t.newNode(n.face, n.level+1, n.a0+h, n.b0, h),
				t.newNode(n.face, n.level+1, n.a0, n.b0+h, h),
				t.newNode(n.face, n.level+1, n.a0+h, n.b0+h, h),
			}
		}
		ready := n.children != nil
		for _, c := range n.children {
			if wantSplit {
				t.request(c)
			}
			ready = ready && c.state == nodeReady
		}
		// Refine once all 4 children are there; until then the parent stays.
		if ready {
			for _, c := range n.children {
				t.visit(c, eye, up, horizon, draw)
			}
			return
		}
	}
	t.request(n)
	if n.state == nodeReady {
		*draw = append(*draw, n)
	}
}

// evict frees the meshes unused for a while when too many are loaded.
func (t *Terrain) evict() {
	if len(t.loaded) <= maxLoadedNodes {
		return
	}
	slices.SortFunc(t.loaded, func(a, b *lodNode) int { return int(b.lastUsed) - int(a.lastUsed) })
	for _, n := range t.loaded[maxLoadedNodes*3/4:] {
		if n.lastUsed == t.frame || n.level == 0 {
			continue
		}
		t.pool = append(t.pool, n.mesh)
		n.mesh, n.state = rl.Mesh{}, nodeEmpty
	}
	t.loaded = slices.DeleteFunc(t.loaded, func(n *lodNode) bool { return n.state != nodeReady })
}

// Warm builds the terrain around eye before the first frame, so the game
// doesn't start on blurry ground. It gives up after timeout.
func (t *Terrain) Warm(eye vec3, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		t.Visible(eye)
		if t.pending == 0 {
			return
		}
		t.upload(1 << 20)
		time.Sleep(2 * time.Millisecond)
	}
}

func (t *Terrain) SurfaceRadius(d vec3) float64 { return moonRadius + t.Height(d) }

// Surface returns the ground point in direction d (any length).
func (t *Terrain) Surface(d vec3) vec3 {
	d = d.norm()
	return d.scale(t.SurfaceRadius(d))
}

// Altitude is the height of p above the ground right below it.
func (t *Terrain) Altitude(p vec3) float64 { return p.length() - t.SurfaceRadius(p.norm()) }

// Raycast marches along the ray and returns the first ground hit within
// maxDist. Steps grow with the height above ground (slopes stay gentle).
func (t *Terrain) Raycast(from, dir vec3, maxDist float64) (vec3, bool) {
	const minStep = 0.1
	prev := from
	for d := minStep; d <= maxDist; {
		p := from.add(dir.scale(d))
		alt := t.Altitude(p)
		if alt > 0 {
			prev = p
			d += math.Max(minStep, alt*0.5)
			continue
		}
		// Refine between the last point above ground and this one.
		lo, hi := prev, p
		for range 10 {
			mid := lo.lerp(hi, 0.5)
			if t.Altitude(mid) <= 0 {
				hi = mid
			} else {
				lo = mid
			}
		}
		return hi, true
	}
	return vec3{}, false
}
