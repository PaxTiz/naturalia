package base

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/mesh"
)

// Floors are where the player walks up in a building (content.Floor): a
// slab they also walk under, like a mezzanine, or stairs, a slope under
// the steps that they walk up a step at a time.

// Floor is a building's floor laid out in the base: a rectangle from one
// end to the other, From and To (local) the middles of its ends on its top.
type Floor struct {
	From, To vec3
	Width    float64
	Thick    float64 // under its top; 0: down to the ground
	In       ecs.ID  // its building
}

// addFloor lays floor fl of blueprint bp out in building in.
func addFloor(in ecs.ID, bp *content.Blueprint, fl content.Floor) {
	at := func(p vec3) vec3 { return bp.At.Add(content.Turn(p, bp.Yaw)) }
	Ents.Floor.Set(Ents.New(), Floor{From: at(fl.From), To: at(fl.To), Width: fl.Width, Thick: fl.Thick, In: in})
}

// frame is the floor's direction from From to To seen from above, its
// length that way, and the direction across.
func (f Floor) frame() (along vec3, length float64, across vec3) {
	d := f.To.Sub(f.From)
	along = vec3{d.X, 0, d.Z}
	length = along.Length()
	along = along.Scale(1 / length)
	return along, length, vec3{-along.Z, 0, along.X}
}

// TopAt is the height of the floor's top over the local point p, and
// whether p stands over it.
func (f Floor) TopAt(p vec3) (float64, bool) {
	along, l, across := f.frame()
	q := vec3{p.X - f.From.X, 0, p.Z - f.From.Z}
	t, s := q.Dot(along), q.Dot(across)
	if t < 0 || t > l || math.Abs(s) > f.Width/2 {
		return 0, false
	}
	return f.From.Y + (f.To.Y-f.From.Y)*t/l, true
}

// Slab is the floor as a level slab: its middle (local), the way along
// it, half its length and its width.
func (f Floor) Slab() (mid, along vec3, half, width float64) {
	along, l, _ := f.frame()
	return f.From.Lerp(f.To, 0.5), along, l / 2, f.Width
}

// Level tells a slab from stairs.
func (f Floor) Level() bool { return math.Abs(f.To.Y-f.From.Y) < 0.01 }

// Bottom is the lowest of the floor: under its lowest top, or the ground.
func (f Floor) Bottom() float64 {
	if f.Thick <= 0 {
		return 0
	}
	return math.Min(f.From.Y, f.To.Y) - f.Thick
}

// Top is the highest of its top.
func (f Floor) Top() float64 { return math.Max(f.From.Y, f.To.Y) }

// Corners are the floor's four corners seen from above (local, y = 0).
func (f Floor) Corners() [4]vec3 {
	along, l, across := f.frame()
	o := vec3{f.From.X, 0, f.From.Z}
	w := across.Scale(f.Width / 2)
	e := o.Add(along.Scale(l))
	return [4]vec3{o.Sub(w), e.Sub(w), e.Add(w), o.Add(w)}
}

// TopUnder is the highest top of a floor straight under the local point,
// no higher than it: and whether there is one.
func TopUnder(local vec3) (float64, bool) {
	best, found := math.Inf(-1), false
	for _, f := range Ents.Floor.All() {
		if top, ok := f.TopAt(local); ok && top <= local.Y && top > best {
			best, found = top, true
		}
	}
	return best, found
}

// Ceiling is the lowest bottom of a floor straight over the local point,
// above it: +Inf for none.
func Ceiling(local vec3) float64 {
	c := math.Inf(1)
	for _, f := range Ents.Floor.All() {
		if f.Thick <= 0 {
			continue
		}
		if top, ok := f.TopAt(local); ok && top-f.Thick > local.Y {
			c = math.Min(c, top-f.Thick)
		}
	}
	return c
}

// FloorOf is floor id, if it is one.
func FloorOf(id ecs.ID) (Floor, bool) {
	if f := Ents.Floor.Get(id); f != nil {
		return *f, true
	}
	return Floor{}, false
}

// floors builds the floors of building in: a slab, steel under a slate
// top; stairs, steel steps with a yellow nosing. A home's are white under
// its floor, its stairs lit under each nosing (homeStairs).
func (b *building) floors(in ecs.ID) {
	steel := b.Part("steel", mesh.LitIndoor, vec3{})
	slab, top := steel, "floor.slate"
	home := Ents.Building.Get(in).home()
	if home {
		slab, top = b.Part("home.panel", mesh.LitIndoor, vec3{}), "home.floor"
	}
	for _, f := range ordered(Ents.Floor) {
		if f.In != in {
			continue
		}
		along, l, _ := f.frame()
		if f.Level() {
			mid := f.From.Lerp(f.To, 0.5)
			slab.Box(mid.Sub(vec3{0, f.Thick / 2, 0}), vec3{l, f.Thick - 0.04, f.Width}, along, 0.03)
			b.Part(top, mesh.LitIndoor, vec3{}).Box(mid.Sub(vec3{0, 0.02, 0}), vec3{l - 0.04, 0.04, f.Width - 0.04}, along, 0.005)
			continue
		}
		// Steps of about 20 cm, from the low end up, each as high as the slope
		// at its far edge.
		if home {
			b.homeStairs(in, *f)
			continue
		}
		lo, hi := f.From, f.To
		if lo.Y > hi.Y {
			lo, hi, along = hi, lo, along.Scale(-1)
		}
		n := max(1, int(math.Round((hi.Y-lo.Y)/0.2)))
		run := l / float64(n)
		for k := range n {
			top := lo.Y + (hi.Y-lo.Y)*float64(k+1)/float64(n)
			bottom := 0.0
			if f.Thick > 0 {
				bottom = math.Max(0, top-0.2-f.Thick)
			}
			mid := vec3{lo.X, 0, lo.Z}.Add(along.Scale((float64(k) + 0.5) * run))
			steel.Box(mid.Add(vec3{0, (top + bottom) / 2, 0}), vec3{run, top - bottom, f.Width}, along, 0.01)
			nose := mid.Add(along.Scale(-run/2 + 0.04)).Add(vec3{0, top + 0.004, 0})
			b.Part("pipe.yellow", mesh.LitIndoor, vec3{}).Box(nose, vec3{0.06, 0.008, f.Width - 0.1}, along, 0)
		}
	}
}
