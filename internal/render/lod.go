package render

import (
	"math"
	"naturalia/internal/mathx"
	"naturalia/internal/planet"
	"runtime"
	"slices"
	"time"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	nodeQuads      = 32   // quads per node side
	maxLevel       = 8    // deepest level: ~0.65 m between vertices
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
	state         int  // nodeEmpty, nodePending or nodeReady
	redo          bool // ready, built again meanwhile: the ground changed under it (Refresh)
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
	uvs          []float32 // ground kind for the shader: polar ice, volcanic basalt
}

// Terrain draws the relief with a level of detail that follows the player:
// fine near them, coarse far away. Meshes are computed by background
// workers and uploaded on the main thread.
type Terrain struct {
	planet.Relief
	roots   [6]*lodNode
	jobs    chan *lodNode
	results chan nodeData
	loaded  []*lodNode
	pending int
	// Freed meshes, reused for new nodes: they all share the same layout.
	// (raylib-go's UnloadMesh is broken without cgo, and recycling is faster.)
	pool  []rl.Mesh
	frame uint64
	pads  []planet.Pad // the flattened discs the meshes were built with
}

func NewTerrain(relief planet.Relief) *Terrain {
	t := &Terrain{
		Relief:  relief,
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
	n.center = mathx.FaceDir(face, a0+size/2, b0+size/2)
	n.centerPos = t.Surface(n.center)
	ang := size * math.Pi / 4 // equiangular faces: coordinates are angles
	n.arc = ang * planet.Radius
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
			d := mathx.FaceDir(n.face, n.a0+n.size*float64(i-1)/nodeQuads, n.b0+n.size*float64(j-1)/nodeQuads)
			pos[j*g+i] = d.Scale(t.SurfaceRadius(d))
		}
	}
	at := func(i, j int) vec3 { return pos[(j+1)*g+i+1] }

	// The mesh is a raylib plane of (nodeQuads+3)² vertices: its outer ring
	// becomes the skirt.
	const side = nodeQuads + 3
	verts := make([]float32, side*side*3)
	norms := make([]float32, side*side*3)
	uvs := make([]float32, side*side*2)
	kinds := make([][2]float32, (nodeQuads+1)*(nodeQuads+1))
	for j := range nodeQuads + 1 {
		for i := range nodeQuads + 1 {
			d := at(i, j).Norm()
			kinds[j*(nodeQuads+1)+i] = [2]float32{float32(t.Ice(d)), float32(t.Basalt(d))}
		}
	}
	// The light is costly: the occlusion is computed every other vertex,
	// the sun's course every fourth, then interpolated.
	sample := func(step int, f func(p vec3) float64) func(i, j int) float64 {
		n := nodeQuads/step + 1
		v := make([]float64, n*n)
		for j := range n {
			for i := range n {
				v[j*n+i] = f(at(i*step, j*step))
			}
		}
		return func(i, j int) float64 {
			i0, j0 := min(i/step, n-2), min(j/step, n-2)
			fx, fy := float64(i-i0*step)/float64(step), float64(j-j0*step)/float64(step)
			k := j0*n + i0
			return (v[k]*(1-fx)+v[k+1]*fx)*(1-fy) + (v[k+n]*(1-fx)+v[k+n+1]*fx)*fy
		}
	}
	var sets []float64
	rise := sample(4, func(p vec3) float64 {
		r, s := t.course(p)
		sets = append(sets, s)
		return r
	})
	k := 0
	set := sample(4, func(vec3) float64 { k++; return sets[k-1] })
	ao := sample(2, t.occlusion)
	type light struct{ rise, set, ao float64 }
	lights := make([]light, (nodeQuads+1)*(nodeQuads+1))
	for j := range nodeQuads + 1 {
		for i := range nodeQuads + 1 {
			lights[j*(nodeQuads+1)+i] = light{rise(i, j), set(i, j), ao(i, j)}
		}
	}
	skirt := n.arc/nodeQuads*1.5 + 0.5
	for vz := range side {
		for vx := range side {
			i := max(0, min(nodeQuads, vx-1))
			j := max(0, min(nodeQuads, vz-1))
			p := at(i, j)
			nrm := at(i+1, j).Sub(at(i-1, j)).Cross(at(i, j+1).Sub(at(i, j-1))).Norm()
			if nrm.Dot(p) < 0 {
				nrm = nrm.Scale(-1)
			}
			l := lights[j*(nodeQuads+1)+i]
			nrm = nrm.Scale(1 + l.ao)
			if vx != i+1 || vz != j+1 {
				p = p.Sub(p.Norm().Scale(skirt))
			}
			k := (vz*side + vx) * 3
			verts[k], verts[k+1], verts[k+2] = float32(p.X), float32(p.Y), float32(p.Z)
			norms[k], norms[k+1], norms[k+2] = float32(nrm.X), float32(nrm.Y), float32(nrm.Z)
			kind := kinds[j*(nodeQuads+1)+i]
			uvs[(vz*side+vx)*2] = courseCode(-l.rise) + kind[0]*0.99
			uvs[(vz*side+vx)*2+1] = courseCode(l.set) + kind[1]*0.99
		}
	}
	return nodeData{n, verts, norms, uvs}
}

// The relief's shadow is baked in the vertices: the sun follows the same
// course every sol (planet/sun.go), so each ground point has the hour
// angles, from its local noon, where the sun comes out from behind the
// relief in the morning and goes behind it in the evening. The shader
// compares them to the sun's hour angle (terrain.vert). A point the relief
// hides at noon never sees the sun; one it hides only in the morning and
// the evening is taken as lit in between. They travel in the integer part
// of the texture coordinates, in thousandths of half a turn (courseCode):
// 1000 is never hidden.

// courseCode turns a hour angle from noon (0..π) into a texture coordinate's
// integer part.
func courseCode(a float64) float32 {
	if a >= math.Pi {
		return 1000
	}
	return float32(min(999, math.Round(a/math.Pi*1000)))
}

// sunThrough is how much of the sun along dir the relief lets through at
// the ground point p (soft-edged). Sampling distances are fixed in meters,
// so the result does not change with the level of detail.
func (t *Terrain) sunThrough(p, dir vec3) float64 {
	up := p.Norm()
	start := p.Add(up.Scale(0.3))
	sun := 1.0
	for s := 1.5; s < 4000; s *= 1.45 {
		q := start.Add(dir.Scale(s))
		if q.Length()-planet.Radius > planet.MaxMountain {
			break // above every peak
		}
		h := q.Length() - t.SurfaceRadius(q.Norm())
		sun = min(sun, 14*h/s) // penumbra of about 4°
		if sun <= 0 {
			return 0
		}
	}
	return sun
}

// course returns the hour angles where the sun clears the relief at p in
// the morning (-π..0) and goes behind it in the evening (0..π): ±π when the
// relief never hides it before it stands 3° high, 0 and 0 when it hides
// it at noon. Pure: the workers call it.
func (t *Terrain) course(p vec3) (rise, set float64) {
	up := p.Norm()
	seen := func(h float64) bool { return t.sunThrough(p, planet.SunAtHourAngle(up, h)) >= 0.5 }
	if !seen(0) {
		return 0, 0
	}
	low := math.Pi // the hour angle where the sun stands 3° high
	if c, s3 := math.Cos(planet.Latitude(up)), math.Sin(3*math.Pi/180); c > s3 {
		low = math.Acos(s3 / c)
	}
	edge := func(side float64) float64 {
		if seen(side * low) {
			return side * math.Pi
		}
		hidden, lit := low, 0.0
		for range 5 {
			if mid := (hidden + lit) / 2; seen(side * mid) {
				lit = mid
			} else {
				hidden = mid
			}
		}
		return side * (hidden + lit) / 2
	}
	return edge(-1), edge(1)
}

// occlusion is how open the sky is around a ground point (ambient
// occlusion): the steepest horizon in 6 directions, at a few distances.
func (t *Terrain) occlusion(p vec3) float64 {
	up := p.Norm()
	u, v := mathx.TangentBasis(up)
	h0 := t.SurfaceRadius(up)
	open := 0.0
	for k := range 6 {
		a := float64(k) * math.Pi / 3
		dir := u.Scale(math.Cos(a)).Add(v.Scale(math.Sin(a)))
		slope := 0.0
		for _, dist := range [...]float64{2, 6, 16, 45} {
			d := p.Add(dir.Scale(dist)).Norm()
			slope = max(slope, (t.SurfaceRadius(d)-h0)/dist)
		}
		open += 1 - slope/math.Sqrt(1+slope*slope) // 1 - sin of the horizon angle
	}
	return open / 6
}

// light is how much of the sun, where it stands now, the relief lets
// through at p, and how open the sky is there: for the objects near the
// player.
func (t *Terrain) light(p vec3) (sun, ao float64) {
	return t.sunThrough(p, planet.SunDir), t.occlusion(p)
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
			copy(unsafe.Slice(mesh.Texcoords, n*2), r.uvs)
			rl.UpdateMeshBuffer(mesh, 0, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Vertices)), n*12), 0)
			rl.UpdateMeshBuffer(mesh, 1, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Texcoords)), n*8), 0)
			rl.UpdateMeshBuffer(mesh, 2, unsafe.Slice((*byte)(unsafe.Pointer(mesh.Normals)), n*12), 0)
			if r.n.redo && r.n.state == nodeReady { // built again: the old mesh goes
				t.pool = append(t.pool, r.n.mesh)
			} else {
				t.loaded = append(t.loaded, r.n)
			}
			r.n.mesh, r.n.state, r.n.redo = mesh, nodeReady, false
		default:
			return
		}
	}
}

// refreshPads builds again the meshes over the ground flattened or given
// back since the last frame, where a bootcamp was set down or moved off.
// They are drawn as they were until then.
func (t *Terrain) refreshPads() {
	pads := t.Pads()
	if slices.Equal(pads, t.pads) {
		return
	}
	for _, list := range [2][]planet.Pad{pads, t.pads} {
		for _, p := range list {
			if !slices.Contains(pads, p) || !slices.Contains(t.pads, p) {
				for _, r := range t.roots {
					t.refresh(r, p.Dir, (p.Radius+planet.PadBlend)/planet.Radius)
				}
			}
		}
	}
	t.pads = slices.Clone(pads)
}

// refresh builds again the ready meshes of n and under within angle a of d.
func (t *Terrain) refresh(n *lodNode, d vec3, a float64) {
	if math.Acos(math.Max(-1, math.Min(1, n.center.Dot(d)))) > n.angulRad+a {
		return
	}
	if n.state == nodeReady && !n.redo {
		n.redo = true
		t.pending++
		t.jobs <- n
	}
	for _, c := range n.children {
		t.refresh(c, d, a)
	}
}

// Visible returns the nodes to draw this frame for a viewer at eye, and
// queues the meshes it is missing.
func (t *Terrain) Visible(eye vec3) []*lodNode {
	t.frame++
	t.refreshPads()
	t.upload(uploadsPerTick)
	// Beyond this angle around Mars, even mountain tops are below the horizon.
	horizon := math.Acos(math.Min(1, planet.Radius/eye.Length())) + math.Acos(planet.Radius/(planet.Radius+planet.MaxMountain))
	var draw []*lodNode
	for _, r := range t.roots {
		t.visit(r, eye, eye.Norm(), horizon, &draw)
	}
	t.evict()
	return draw
}

func (t *Terrain) visit(n *lodNode, eye, up vec3, horizon float64, draw *[]*lodNode) {
	if math.Acos(math.Max(-1, math.Min(1, n.center.Dot(up))))-n.angulRad > horizon {
		return
	}
	n.lastUsed = t.frame
	wantSplit := n.level < maxLevel && eye.Sub(n.centerPos).Length() < splitFactor*n.arc
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
