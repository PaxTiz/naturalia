// Package planet is Mars without graphics: the relief, the dust covering
// it, its crystals and rocks, and the settings mods change them with.
package planet

import "math"

const (
	Radius      = 3389.5 // the real planet at 1/1 000 scale
	padRadius   = 76.0   // the ground is a plane under the whole base, hangar and its apron included
	clearRadius = 24.0   // and clean around its domes: no dust nor crystals
	MaxMountain = 420.0  // upper bound of the relief, for horizon culling
	plainRadius = 160.0  // around the base, only gentle plains
)

// The base sits on the north pole of the cube, where "up" is +Y.
var BaseUp = vec3{0, 1, 0}

// Toward returns the direction dist meters away from the base, heading
// along bearing (a horizontal vector at the base).
func Toward(bearing vec3, dist float64) vec3 {
	a := dist / Radius
	return BaseUp.Scale(math.Cos(a)).Add(bearing.Norm().Scale(math.Sin(a)))
}

// TangentAt projects v on the ground plane at d.
func TangentAt(d, v vec3) vec3 { return v.Sub(d.Scale(v.Dot(d))).Norm() }

// Toward2 is toward from any point: dist meters from c along tangent t.
func Toward2(c, t vec3, dist float64) vec3 {
	a := dist / Radius
	return c.Scale(math.Cos(a)).Add(t.Scale(math.Sin(a)))
}

// ArcDist is the distance in meters along the surface between two directions.
func ArcDist(a, b vec3) float64 {
	return math.Acos(math.Max(-1, math.Min(1, a.Dot(b)))) * Radius
}

// Relief is the ground shape of Mars, smooth everywhere. It is made in two
// steps: the layers (layers.go), what Mars is; then what is made flat on it
// to build on (ground.go): the base's pad, the plots under constructions,
// the bootcamps' pads.
type Relief struct {
	seed  uint64
	set   *Settings // how mods shape it
	base  Pad       // a true plane under the base
	works *works    // flattened since, shared by copies (the terrain's)
}

func newRelief(seed uint64, set *Settings) Relief {
	r := Relief{seed: seed, set: set, works: &works{}}
	r.base = Pad{Dir: BaseUp, Radius: padRadius}.laid(r, baseBlend)
	return r
}

// BaseHeight is the ground's altitude under the base.
func (r Relief) BaseHeight() float64 { return r.base.h }

// Height is the ground's altitude over the mean radius in direction d (unit).
func (r Relief) Height(d vec3) float64 {
	f := r.works.now.Load()
	h := r.base.flatten(d, r.raw(d))
	if f == nil {
		return h
	}
	return f.flatten(d, f.grade(d, h))
}

// Natural is the ground's altitude before any plot graded it.
func (r Relief) Natural(d vec3) float64 {
	h := r.base.flatten(d, r.raw(d))
	if f := r.works.now.Load(); f != nil {
		return f.flatten(d, h)
	}
	return h
}

// raw is the sum of the layers.
func (r Relief) raw(d vec3) float64 {
	s := r.spot(d)
	h := 0.0
	for _, l := range r.set.Layers {
		k := 1.0
		if l.Strength != nil {
			if k = l.Strength(r.set); k == 0 {
				continue
			}
		}
		h += l.Height(s) * k
	}
	return h
}
