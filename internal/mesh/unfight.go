package mesh

import (
	"cmp"
	"math"
	"slices"
)

// Two surfaces in the same plane, one on the other, get the same depth:
// the depth buffer can't tell which is in front, and they take turns
// (z-fighting), flickering as the camera moves. The pieces are built flush
// on purpose (a frame on a wall, a stripe on a floor, a crate on another),
// so Unfight settles it once the meshes are built, without moving them:
// each piece gets a layer, one more than the pieces it lies on, and the
// shaders bring its depth forward by a few steps of the depth buffer per
// layer (object.vert). The detail is the piece built after: the base is
// built from the walls in, the furniture last. Two flush pieces of the same mesh look
// alike: either may win, they only need different layers (a row of beams
// overlapping end to end alternates instead of climbing).

// FightGap is how close two parallel surfaces must stand to fight. Half a
// millimeter apart, the depth buffer still tells them apart 30 m away; the
// vertices, in float32 3 km from Mars' center, are good to a quarter.
const FightGap = 0.0005

// fightArea is the smallest overlap that counts (m²): pieces touching
// along an edge overlap by nothing.
const fightArea = 1e-5

// MaxLayer is the highest layer the shaders take.
const MaxLayer = 15

// Surface is a triangle of a built mesh, in the frame of its Parts, with
// the piece it belongs to.
type Surface struct {
	P     [3]vec3
	N     vec3 // unit, facing out
	Piece int  // index in the list of pieces of Surfaces
	Mesh  int  // and the mesh it is in, in Keys
	Layer int  // given by Unfight
}

// piece is a run of vertices of one builder.
type piece struct {
	b          *Builder
	start, end int // vertices
	seq        int // the order it was built in
}

// Fight is two pieces with surfaces on top of each other; Top is the one
// that must win (A or B), the later built. Either may if Free.
type Fight struct {
	A, B, Top int
	Free      bool
}

// Unfight gives each piece its layer (see FightGap): above every piece
// it lies on, and apart from the flush pieces of its mesh. All of them
// were built before it: one pass in the order they were built settles it.
func (s *Parts) Unfight() {
	pieces, tris := s.surfaces()
	under := make([][]Fight, len(pieces)) // the fights each piece must win
	for _, f := range fights(tris, pieces) {
		under[f.Top] = append(under[f.Top], f)
	}
	order := make([]int, len(pieces))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int { return cmp.Compare(pieces[a].seq, pieces[b].seq) })
	layer := make([]int, len(pieces))
	for _, i := range order {
		taken := map[int]bool{}
		for _, f := range under[i] {
			j := f.A + f.B - i
			if f.Free {
				taken[layer[j]] = true
			} else {
				layer[i] = max(layer[i], layer[j]+1)
			}
		}
		for taken[layer[i]] {
			layer[i]++
		}
		if layer[i] > 0 {
			pieces[i].setLayer(min(layer[i], MaxLayer))
		}
	}
}

// setLayer stores the layer in the length of the normals: 1 + layer. The
// shaders normalize them anyway.
func (p piece) setLayer(l int) {
	for v := p.start; v < p.end; v++ {
		k := float32(1+l) / float32(p.b.layerAt(v))
		for i := 3 * v; i < 3*v+3; i++ {
			p.b.N[i] *= k
		}
	}
}

// layerAt is the layer a vertex was given, from its normal.
func (m *Builder) layerAt(v int) int {
	n := vec3{float64(m.N[3*v]), float64(m.N[3*v+1]), float64(m.N[3*v+2])}
	return int(math.Round(n.Length()))
}

// Fights lists the triangles of every mesh, in the frame of the Parts,
// and the pieces fighting among them.
func (s *Parts) Fights() ([]Surface, []Fight) {
	pieces, tris := s.surfaces()
	return tris, fights(tris, pieces)
}

func (s *Parts) surfaces() ([]piece, []Surface) {
	var pieces []piece
	n := 0
	for _, b := range s.byKey {
		n += len(b.V) / 9
	}
	tris := make([]Surface, 0, n)
	for mi, k := range s.Keys {
		b := s.byKey[k]
		n := len(b.V) / 3
		starts := b.pieces
		if len(starts) == 0 || starts[0] > 0 {
			starts = append([]int{0}, starts...)
		}
		seqs := b.seqs
		if len(starts) > len(seqs) { // a first run before any piece
			seqs = append([]int{-1}, seqs...)
		}
		for i, st := range starts {
			end := n
			if i+1 < len(starts) {
				end = starts[i+1]
			}
			if end <= st {
				continue
			}
			id := len(pieces)
			pieces = append(pieces, piece{b, st, end, seqs[i]})
			layer := b.layerAt(st) - 1
			for v := st; v+2 < end; v += 3 {
				p := [3]vec3{b.at(v, s.origin), b.at(v+1, s.origin), b.at(v+2, s.origin)}
				nrm := p[1].Sub(p[0]).Cross(p[2].Sub(p[0]))
				if nrm.Length() < 1e-9 {
					continue
				}
				tris = append(tris, Surface{p, nrm.Norm(), id, mi, layer})
			}
		}
	}
	return pieces, tris
}

// at is vertex v, less origin.
func (m *Builder) at(v int, origin vec3) vec3 {
	return vec3{float64(m.V[3*v]), float64(m.V[3*v+1]), float64(m.V[3*v+2])}.Sub(origin)
}

// fights returns the pieces that have surfaces fighting: facing the same
// way (back to back, they are pieces touching, hidden), closer than
// FightGap, and overlapping. The triangles are sorted into buckets by
// plane, then swept along the plane: only neighbors are compared.
func fights(tris []Surface, pieces []piece) []Fight {
	type key struct{ nx, ny, nz, d int }
	keyOf := func(t Surface) key {
		n := t.N.Scale(40)
		return key{int(math.Round(n.X)), int(math.Round(n.Y)), int(math.Round(n.Z)), int(math.Floor(t.N.Dot(t.P[0]) / FightGap))}
	}
	buckets := map[key][]int{}
	lo, hi := make([]vec3, len(tris)), make([]vec3, len(tris)) // their bounds
	for i, t := range tris {
		k := keyOf(t)
		buckets[k] = append(buckets[k], i)
		lo[i], hi[i] = t.P[0], t.P[0]
		for _, p := range t.P[1:] {
			lo[i] = vec3{min(lo[i].X, p.X), min(lo[i].Y, p.Y), min(lo[i].Z, p.Z)}
			hi[i] = vec3{max(hi[i].X, p.X), max(hi[i].Y, p.Y), max(hi[i].Z, p.Z)}
		}
	}
	apart := func(i, j int) bool {
		return lo[i].X > hi[j].X+FightGap || lo[j].X > hi[i].X+FightGap ||
			lo[i].Y > hi[j].Y+FightGap || lo[j].Y > hi[i].Y+FightGap ||
			lo[i].Z > hi[j].Z+FightGap || lo[j].Z > hi[i].Z+FightGap
	}
	seen := map[[2]int]bool{}
	var out []Fight
	for k, here := range buckets {
		// With the next plane up: two surfaces closer than FightGap are in
		// the same bucket or neighbors.
		next := k
		next.d++
		cand := append(slices.Clone(here), buckets[next]...)
		// Swept along the axis the plane spreads most on.
		n := tris[here[0]].N
		ax := func(v vec3) float64 { return v.X }
		if math.Abs(n.Y) < math.Abs(n.X) && math.Abs(n.Y) <= math.Abs(n.Z) {
			ax = func(v vec3) float64 { return v.Y }
		} else if math.Abs(n.Z) < math.Abs(n.X) {
			ax = func(v vec3) float64 { return v.Z }
		}
		slices.SortFunc(cand, func(a, b int) int { return cmp.Compare(ax(lo[a]), ax(lo[b])) })
		for x, i := range cand {
			a := tris[i]
			da := a.N.Dot(a.P[0])
			for _, j := range cand[x+1:] {
				if ax(lo[j]) > ax(hi[i])+FightGap {
					break
				}
				b := tris[j]
				if a.Piece == b.Piece || apart(i, j) || a.N.Dot(b.N) < 0.9995 {
					continue
				}
				pair := [2]int{min(a.Piece, b.Piece), max(a.Piece, b.Piece)}
				gap := b.N.Dot(b.P[0]) - da
				if seen[pair] || math.Abs(gap) >= FightGap {
					continue
				}
				if overlap(a, b) > fightArea {
					seen[pair] = true
					top := a.Piece
					if pieces[b.Piece].seq > pieces[a.Piece].seq {
						top = b.Piece
					}
					out = append(out, Fight{pair[0], pair[1], top, a.Mesh == b.Mesh})
				}
			}
		}
	}
	return out
}

// overlap is the area common to two triangles of about the same plane,
// projected on a's.
func overlap(a, b Surface) float64 {
	u := a.P[1].Sub(a.P[0]).Norm()
	v := a.N.Cross(u)
	flat := func(t Surface) [][2]float64 {
		var out [][2]float64
		for _, p := range t.P {
			q := p.Sub(a.P[0])
			out = append(out, [2]float64{q.Dot(u), q.Dot(v)})
		}
		if area2(out) < 0 {
			out[1], out[2] = out[2], out[1]
		}
		return out
	}
	poly, clip := flat(b), flat(a)
	for i := range clip {
		e0, e1 := clip[i], clip[(i+1)%len(clip)]
		side := func(p [2]float64) float64 {
			return (e1[0]-e0[0])*(p[1]-e0[1]) - (e1[1]-e0[1])*(p[0]-e0[0])
		}
		var next [][2]float64
		for k := range poly {
			p, q := poly[k], poly[(k+1)%len(poly)]
			sp, sq := side(p), side(q)
			if sp >= 0 {
				next = append(next, p)
			}
			if (sp >= 0) != (sq >= 0) {
				f := sp / (sp - sq)
				next = append(next, [2]float64{p[0] + f*(q[0]-p[0]), p[1] + f*(q[1]-p[1])})
			}
		}
		poly = next
		if len(poly) < 3 {
			return 0
		}
	}
	return area2(poly)
}

// area2 is the signed area of a polygon.
func area2(p [][2]float64) float64 {
	s := 0.0
	for i := range p {
		q, r := p[i], p[(i+1)%len(p)]
		s += q[0]*r[1] - r[0]*q[1]
	}
	return s / 2
}
