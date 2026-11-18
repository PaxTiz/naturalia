package base

import (
	"math"
	"naturalia/internal/ecs"
	"naturalia/internal/mesh"
)

// Guard rails run along the edges of the slabs that open onto the room
// below: not those against a wall, and not where stairs arrive. Nothing
// to declare: they follow from the floors.

const railHeight = 1.05 // over the slab's top

// Rail is a guard rail: its ends on the slab's top (local), from Y0 up to
// Y1. It stops the player on the slab, not those walking underneath.
type Rail struct {
	A, B   vec3
	Y0, Y1 float64
	In     ecs.ID
}

// Rails are the guard rails of every slab.
func Rails() []Rail {
	var out []Rail
	for _, f := range ordered(Ents.Floor) {
		out = append(out, f.rails()...)
	}
	return out
}

// rails are the guard rails of slab f.
func (f Floor) rails() []Rail {
	if !f.Level() || f.Thick <= 0 {
		return nil
	}
	r := Ents.Building.Get(f.In)
	c := f.Corners()
	var out []Rail
	for k := range 4 {
		a, b := c[k], c[(k+1)%4]
		if r != nil && r.againstWall(a) && r.againstWall(b) && r.againstWall(a.Lerp(b, 0.5)) {
			continue
		}
		along := b.Sub(a)
		l := along.Length()
		along = along.Scale(1 / l)
		var gaps [][2]float64
		for _, s := range ordered(Ents.Floor) {
			if s.In != f.In || s.Level() {
				continue
			}
			hi := s.To
			if s.From.Y > hi.Y {
				hi = s.From
			}
			t := vec3{hi.X - a.X, 0, hi.Z - a.Z}.Dot(along)
			off := vec3{hi.X, 0, hi.Z}.Sub(a.Add(along.Scale(t))).Length()
			if off < 0.3 && math.Abs(hi.Y-f.Top()) < 0.3 {
				gaps = append(gaps, [2]float64{t - s.Width/2 - 0.05, t + s.Width/2 + 0.05})
			}
		}
		t0 := 0.0
		y := f.Top()
		put := func(t1 float64) {
			if t1-t0 > 0.3 {
				out = append(out, Rail{a.Add(along.Scale(t0)).Add(vec3{0, y, 0}), a.Add(along.Scale(t1)).Add(vec3{0, y, 0}), y, y + railHeight, f.In})
			}
		}
		for _, g := range gaps {
			put(g[0])
			t0 = math.Max(t0, g[1])
		}
		put(l)
	}
	return out
}

// againstWall tells whether a local point stands within 0.4 m of the
// room's walls, inside.
func (r Room) againstWall(p vec3) bool {
	if r.Hall != nil {
		return !r.inHall(p, -0.4)
	}
	return math.Hypot(p.X-r.Center.X, p.Z-r.Center.Z) > r.Radius-0.4
}

// rails builds the guard rails of building in: steel posts about every
// 1.2 m, a handrail and a mid rail, a kick plate along the slab; and the
// handrail of its stairs on the sides away from the walls.
func (b *building) rails(in ecs.ID) {
	if r := Ents.Building.Get(in); r != nil && r.home() {
		b.glassRails(in)
		return
	}
	steel := b.Part("steel", mesh.LitIndoor, vec3{})
	for _, f := range ordered(Ents.Floor) {
		if f.In != in {
			continue
		}
		for _, rl := range f.rails() {
			d := rl.B.Sub(rl.A)
			l := d.Length()
			n := max(1, int(math.Round(l/1.2)))
			for i := 0; i <= n; i++ {
				p := rl.A.Lerp(rl.B, float64(i)/float64(n))
				steel.Prism(p, yUp, 0.035, 0.035, railHeight, 8, 0)
			}
			mid := rl.A.Lerp(rl.B, 0.5)
			steel.Box(mid.Add(vec3{0, railHeight, 0}), vec3{l + 0.08, 0.06, 0.07}, d, 0.02)
			steel.Box(mid.Add(vec3{0, railHeight * 0.5, 0}), vec3{l, 0.03, 0.03}, d, 0)
			b.Part("pipe.yellow", mesh.LitIndoor, vec3{}).Box(mid.Add(vec3{0, 0.06, 0}), vec3{l, 0.12, 0.03}, d, 0)
		}
		if f.Level() {
			continue
		}
		r := Ents.Building.Get(in)
		_, l, across := f.frame()
		for _, s := range []float64{-1, 1} {
			side := across.Scale(s * (f.Width/2 - 0.05))
			a, e := f.From.Add(side), f.To.Add(side)
			if r != nil && r.againstWall(a) && r.againstWall(e) {
				continue
			}
			n := max(1, int(math.Round(l/1.2)))
			for i := 0; i <= n; i++ {
				p := a.Lerp(e, float64(i)/float64(n))
				steel.Prism(p, yUp, 0.035, 0.035, railHeight, 8, 0)
			}
			// The handrail, along the slope.
			d := e.Sub(a)
			dn := d.Norm()
			up := vec3{0, 1, 0}.Sub(dn.Scale(dn.Y)).Norm()
			steel.BoxBasis(a.Lerp(e, 0.5).Add(vec3{0, railHeight, 0}), [3]vec3{dn, up, across}, vec3{d.Length() + 0.08, 0.06, 0.07}, 0.02)
		}
	}
}
