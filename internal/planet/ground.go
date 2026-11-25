package planet

import (
	"math"
	"naturalia/internal/mathx"
	"sync/atomic"
)

// Ground made flat to build on, over the layers: Pads, discs under what
// stands whole (the base, a bootcamp), and Plots, rectangles under the
// blocks built on a grid (grid.go). Both blend back to the relief around.

// Pad is a disc of flat ground: a plane tangent to Mars at its middle, at
// the height the layers give there.
type Pad struct {
	Dir    vec3    // its middle, from Mars' center, unit
	Radius float64 // flat out to there, in m
	h      float64 // the plane's height, over the mean radius
	blend  float64 // how far beyond its edge the ground takes back its shape
	cosOut float64 // cosine of the angle out to where it has
}

const (
	// PadBlend is how far beyond a bootcamp's pad's edge the ground takes
	// back its shape; baseBlend, the base's.
	PadBlend  = 14.0
	baseBlend = 35.0
	// PlotBlend is the bank's width around a plot, beyond the PlotMargin
	// kept flat around it: more than a vertex of the terrain's meshes, so
	// the ground they draw doesn't cut the block's edge.
	PlotBlend  = 5.0
	PlotMargin = 0.9
	// MaxGrade is how much a plot digs or fills, at most.
	MaxGrade = 1.5
)

// laid is the pad set at the ground's height, with its blend.
func (p Pad) laid(r Relief, blend float64) Pad {
	p.Dir = p.Dir.Norm()
	p.h, p.blend = r.raw(p.Dir), blend
	p.cosOut = math.Cos((p.Radius + blend) / Radius)
	return p
}

// flatten blends height h at d to the pad.
func (p Pad) flatten(d vec3, h float64) float64 {
	c := d.Dot(p.Dir)
	if c < p.cosOut {
		return h
	}
	k := mathx.Smoothstep(p.Radius, p.Radius+p.blend, ArcDist(d, p.Dir))
	flat := (Radius+p.h)/c - Radius
	return flat + (h-flat)*k
}

// Area is where a pad or a plot shapes the ground: within Reach m of Dir.
type Area struct {
	Dir   vec3
	Reach float64
}

func (p Pad) area() Area { return Area{p.Dir, p.Radius + p.blend} }

// Plot is a rectangle of flat ground, level on a grid: its middle At, its
// sides along X and across it, its plane's normal Up.
type Plot struct {
	At, X, Up vec3
	Half      [2]float64 // half its sides, along X and across
}

// dist is how far the ground along d is from the plot's edge, 0 over it,
// and the altitude its plane has there.
func (p *Plot) dist(d vec3) (dist, h float64) {
	r := p.At.Dot(p.Up) / d.Dot(p.Up)
	off := d.Scale(r).Sub(p.At)
	x := math.Max(0, math.Abs(off.Dot(p.X))-p.Half[0])
	z := math.Max(0, math.Abs(off.Dot(p.X.Cross(p.Up)))-p.Half[1])
	return math.Hypot(x, z), r - Radius
}

func (p Plot) area() Area {
	return Area{p.At.Norm(), math.Hypot(p.Half[0], p.Half[1]) + PlotMargin + PlotBlend}
}

// works are the flattened grounds: replaced whole, they are read by the
// terrain's workers meanwhile.
type works struct{ now atomic.Pointer[Flat] }

// Flat is the ground flattened at some time.
type Flat struct {
	Pads  []Pad
	Plots []Plot
	cells map[[3]int32][]int32 // the plots that reach into each cell of the mean sphere
}

// flatCell is the side of the cells the plots are kept in, in m.
const flatCell = 16.0

// SetPads flattens the ground under these discs (Dir and Radius), and no
// others.
func (r Relief) SetPads(list []Pad) {
	f := *r.Flat()
	f.Pads = make([]Pad, len(list))
	for i, p := range list {
		f.Pads[i] = p.laid(r, PadBlend)
	}
	r.works.now.Store(&f)
}

// SetPlots grades the ground under these rectangles, and no others.
func (r Relief) SetPlots(list []Plot) {
	f := *r.Flat()
	f.Plots, f.cells = list, map[[3]int32][]int32{}
	cell := func(v float64) int32 { return int32(math.Floor(v / flatCell)) }
	for i, p := range list {
		a := p.area()
		c := a.Dir.Scale(Radius)
		for x := cell(c.X - a.Reach); x <= cell(c.X+a.Reach); x++ {
			for y := cell(c.Y - a.Reach); y <= cell(c.Y+a.Reach); y++ {
				for z := cell(c.Z - a.Reach); z <= cell(c.Z+a.Reach); z++ {
					k := [3]int32{x, y, z}
					f.cells[k] = append(f.cells[k], int32(i))
				}
			}
		}
	}
	r.works.now.Store(&f)
}

// Flat is the ground flattened now: another pointer once it changed.
func (r Relief) Flat() *Flat {
	if r.works != nil {
		if f := r.works.now.Load(); f != nil {
			return f
		}
	}
	return &noFlat
}

var noFlat Flat

// Areas are where the ground was flattened.
func (f *Flat) Areas() map[Area]any {
	out := map[Area]any{}
	for _, p := range f.Pads {
		out[p.area()] = p
	}
	for _, p := range f.Plots {
		out[p.area()] = p
	}
	return out
}

func (f *Flat) flatten(d vec3, h float64) float64 {
	for _, p := range f.Pads {
		h = p.flatten(d, h)
	}
	return h
}

// near are the plots that may reach the ground along d.
func (f *Flat) near(d vec3) []int32 {
	if len(f.cells) == 0 {
		return nil
	}
	p := d.Scale(Radius / flatCell)
	return f.cells[[3]int32{int32(math.Floor(p.X)), int32(math.Floor(p.Y)), int32(math.Floor(p.Z))}]
}

// grade blends height h at d to the plots around: on one (its margin
// too), their lowest plane, so that none pokes through another; around,
// the planes of those near, the nearer the more, then the relief.
func (f *Flat) grade(d vec3, h float64) float64 {
	low, natural := math.Inf(1), 1.0
	sum, weight := 0.0, 0.0
	for _, i := range f.near(d) {
		dist, plane := f.Plots[i].dist(d)
		if dist <= PlotMargin {
			low = math.Min(low, plane)
			continue
		}
		e := dist - PlotMargin
		if e >= PlotBlend {
			continue
		}
		k := mathx.Smoothstep(0, PlotBlend, e)
		natural = math.Min(natural, k)
		w := (1 - k) / (e * e)
		sum, weight = sum+plane*w, weight+w
	}
	if !math.IsInf(low, 1) {
		return low
	}
	if weight == 0 {
		return h
	}
	flat := sum / weight
	return flat + (h-flat)*natural
}

// OnPad tells whether d is on flattened ground (the base's aside), or
// within margin m of its edge.
func (r Relief) OnPad(d vec3, margin float64) bool {
	f := r.Flat()
	for _, p := range f.Pads {
		if d.Dot(p.Dir) > p.cosOut && ArcDist(d, p.Dir) < p.Radius+margin {
			return true
		}
	}
	for _, i := range f.near(d) {
		if dist, _ := f.Plots[i].dist(d); dist < margin {
			return true
		}
	}
	return false
}

// ByPad tells whether d is where the base's pad or a bootcamp's shapes
// the ground.
func (r Relief) ByPad(d vec3) bool {
	if d.Dot(r.base.Dir) > r.base.cosOut {
		return true
	}
	for _, p := range r.Flat().Pads {
		if d.Dot(p.Dir) > p.cosOut {
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
