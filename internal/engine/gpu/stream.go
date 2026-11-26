package gpu

import (
	"runtime"
	"unsafe"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Stream builds meshes on background workers and sends them to the GPU on
// the main thread, a few per frame. The meshes all share one layout and
// are recycled: one given back (Free) is filled again for the next
// (raylib-go's UnloadMesh is broken without cgo, and recycling is faster).
//
// K tells the pieces apart: the terrain's patches, a cave's blocks.
type Stream[K comparable] struct {
	jobs    chan K
	results chan Built[K]
	pending int
	pool    []rl.Mesh
	newMesh func() rl.Mesh
}

// Data is a mesh's vertices as the workers build them: positions and
// normals by 3, texture coordinates by 2 (none: left as they are). The
// layout's mesh holds at least as many.
type Data struct {
	Verts, Norms, UVs []float32
}

// Built is a piece built, not on the GPU yet.
type Built[K comparable] struct {
	Key K
	Data
}

// NewStream starts a worker per CPU building pieces with build, which must
// be safe to call from several goroutines. newMesh makes a mesh of the
// layout, on the main thread.
func NewStream[K comparable](newMesh func() rl.Mesh, build func(K) Data) *Stream[K] {
	s := &Stream[K]{
		jobs:    make(chan K, 8192),
		results: make(chan Built[K], 8192),
		newMesh: newMesh,
	}
	for range runtime.NumCPU() {
		go func() {
			for k := range s.jobs {
				s.results <- Built[K]{k, build(k)}
			}
		}()
	}
	return s
}

// Request queues a piece to build.
func (s *Stream[K]) Request(k K) {
	s.pending++
	s.jobs <- k
}

// Pending is how many pieces are queued or being built, or built and not
// sent yet.
func (s *Stream[K]) Pending() int { return s.pending }

// Take gives up to limit pieces built, without waiting.
func (s *Stream[K]) Take(limit int) []Built[K] {
	var out []Built[K]
	for range limit {
		select {
		case b := <-s.results:
			s.pending--
			out = append(out, b)
		default:
			return out
		}
	}
	return out
}

// Upload sends up to limit pieces built to the GPU, each into a mesh of the
// pool, and hands it to done (which may Request it again).
func (s *Stream[K]) Upload(limit int, done func(k K, m rl.Mesh)) {
	for _, b := range s.Take(limit) {
		done(b.Key, s.fill(b.Data))
	}
}

// fill copies data into a mesh of the pool, or a new one, and sends it.
func (s *Stream[K]) fill(d Data) rl.Mesh {
	var m rl.Mesh
	if k := len(s.pool); k > 0 {
		m, s.pool = s.pool[k-1], s.pool[:k-1]
	} else {
		m = s.newMesh()
	}
	put := func(buf int, dst *float32, src []float32) {
		if len(src) == 0 {
			return
		}
		copy(unsafe.Slice(dst, len(src)), src)
		rl.UpdateMeshBuffer(m, buf, unsafe.Slice((*byte)(unsafe.Pointer(dst)), len(src)*4), 0)
	}
	put(0, m.Vertices, d.Verts)
	put(1, m.Texcoords, d.UVs)
	put(2, m.Normals, d.Norms)
	return m
}

// Free gives a mesh back to the pool, to be filled again.
func (s *Stream[K]) Free(m rl.Mesh) { s.pool = append(s.pool, m) }
