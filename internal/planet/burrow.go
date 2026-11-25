package planet

import "math"

// Burrow is what is dug under the ground: stairwells, galleries. The relief
// knows one height a direction, the ground's, and keeps it: a burrow stands
// beside it, for those who walk in it. It is entered where it comes up to
// the surface, and nowhere else.
type Burrow struct {
	digs []dig
}

// dig is what was dug together, and the ball around it.
type dig struct {
	mid     vec3
	reach   float64
	hollows []Hollow
}

// Hollow is a straight stretch of a burrow: a floor from A to B, upright
// walls on its sides, a ceiling over it.
type Hollow struct {
	A, B   vec3       // the middle of its floor at both ends, from Mars' center
	Half   float64    // from its middle to its walls
	Height float64    // from its floor to its ceiling
	Pad    [2]float64 // how far it goes beyond A and B, for the bends
}

// hollowUnder is how far under its floor feet still count as in a hollow.
const hollowUnder = 1.0

// NewBurrow is the burrow of some digs.
func NewBurrow(digs [][]Hollow) *Burrow {
	b := &Burrow{}
	for _, list := range digs {
		if len(list) == 0 {
			continue
		}
		d := dig{hollows: list}
		for _, h := range list {
			d.mid = d.mid.Add(h.A.Lerp(h.B, 0.5).Scale(1 / float64(len(list))))
		}
		for _, h := range list {
			far := h.Half + h.Height + 1
			d.reach = math.Max(d.reach, math.Max(h.A.Sub(d.mid).Length(), h.B.Sub(d.mid).Length())+far)
		}
		b.digs = append(b.digs, d)
	}
	return b
}

// in is where a point is in the hollow: how far beyond its ends (0 between
// them), its floor and its ceiling there, as radii; ok if it is in it.
func (h Hollow) in(p vec3) (beyond, floor, ceiling float64, ok bool) {
	up := h.A.Add(h.B).Norm()
	d := h.B.Sub(h.A)
	d = d.Sub(up.Scale(d.Dot(up)))
	long := d.Length()
	if long < 1e-6 {
		return 0, 0, 0, false
	}
	d = d.Scale(1 / long)
	off := p.Sub(h.A)
	t, z := off.Dot(d), off.Dot(d.Cross(up))
	if t < -h.Pad[0] || t > long+h.Pad[1] || math.Abs(z) > h.Half {
		return 0, 0, 0, false
	}
	k := math.Max(0, math.Min(1, t/long))
	floor = h.A.Length()*(1-k) + h.B.Length()*k
	ceiling = floor + h.Height
	r := p.Length()
	return math.Max(0, math.Max(-t, t-long)), floor, ceiling, r > floor-hollowUnder && r < ceiling
}

// At is the floor and the ceiling of the burrow around a point, from Mars'
// center; ok if it is in it.
func (b *Burrow) At(p vec3) (floor, ceiling float64, ok bool) {
	if b == nil {
		return 0, 0, false
	}
	best := math.Inf(1)
	for _, d := range b.digs {
		if p.Sub(d.mid).Length() > d.reach {
			continue
		}
		for _, h := range d.hollows {
			if beyond, f, c, in := h.in(p); in && beyond < best {
				best, floor, ceiling, ok = beyond, f, c, true
			}
		}
	}
	return floor, ceiling, ok
}
