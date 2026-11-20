package planet

import (
	"math"
	"naturalia/internal/mathx"
	"sync/atomic"
)

// Pads are discs of ground made flat for something standing on them, a
// bootcamp: a plane tangent to Mars at their middle, at the height the
// ground had there, blended back to the relief beyond their edge. The
// base's own pad is apart (Relief.Height).

// Pad is a disc of flattened ground.
type Pad struct {
	Dir    vec3    // its middle, from Mars' center, unit
	Radius float64 // flat out to there, in m
	h      float64 // the plane's height, over the mean radius
	cosOut float64 // cosine of the angle out to where the relief is back
}

// PadBlend is how far beyond a pad's edge the ground takes back its shape.
const PadBlend = 14.0

// pads are the relief's, shared by its copies (the terrain's): replaced
// whole, they are read by the terrain's workers meanwhile.
type pads struct{ list atomic.Pointer[[]Pad] }

// SetPads flattens the ground under these discs (Dir and Radius), and
// only them.
func (r Relief) SetPads(list []Pad) {
	out := make([]Pad, len(list))
	for i, p := range list {
		p.Dir = p.Dir.Norm()
		p.h = r.raw(p.Dir)
		p.cosOut = math.Cos((p.Radius + PadBlend) / Radius)
		out[i] = p
	}
	r.pads.list.Store(&out)
}

// Pads are the discs flattened.
func (r Relief) Pads() []Pad {
	if r.pads == nil {
		return nil
	}
	if p := r.pads.list.Load(); p != nil {
		return *p
	}
	return nil
}

// flatten blends height h at d to the pads around.
func (r Relief) flatten(d vec3, h float64) float64 {
	for _, p := range r.Pads() {
		c := d.Dot(p.Dir)
		if c < p.cosOut {
			continue
		}
		k := mathx.Smoothstep(p.Radius, p.Radius+PadBlend, ArcDist(d, p.Dir))
		flat := (Radius+p.h)/c - Radius
		h = flat + (h-flat)*k
	}
	return h
}

// OnPad tells whether d is on a pad, or within margin m of its edge.
func (r Relief) OnPad(d vec3, margin float64) bool {
	for _, p := range r.Pads() {
		if d.Dot(p.Dir) > p.cosOut && ArcDist(d, p.Dir) < p.Radius+margin {
			return true
		}
	}
	return false
}

// PadSlope is how far the ground strays from the plane a pad of that
// radius would lay at d, out to where it takes back its shape: what the
// blend would have to make up.
func (r Relief) PadSlope(d vec3, radius float64) float64 {
	d = d.Norm()
	h := r.raw(d)
	t1, t2 := mathx.TangentBasis(d)
	worst := 0.0
	for _, dist := range []float64{radius * 0.5, radius, radius + PadBlend*0.5, radius + PadBlend} {
		for k := range 12 {
			a := float64(k) * math.Pi / 6
			q := Toward2(d, t1.Scale(math.Cos(a)).Add(t2.Scale(math.Sin(a))), dist)
			flat := (Radius+h)/q.Dot(d) - Radius
			worst = math.Max(worst, math.Abs(r.raw(q)-flat))
		}
	}
	return worst
}
