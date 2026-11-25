package base

import (
	"math"
	"naturalia/internal/mesh"
)

// Glazed domes (content.Blueprint.Glazed): a geodesic dome of glass on a
// white frame, its round wall glazed above a low plinth of dark ceramic,
// the floor and what grows inside lit by the sun through the glass. A
// greenhouse.

// glazed tells a room of glass.
func (r Room) glazed() bool { return r.plan != nil && r.plan.Glazed }

// skylight tells a room whose dome only is of glass.
func (r Room) skylight() bool { return r.plan != nil && r.plan.Skylight }

const glassSill = 0.5 // the plinth's height, under the glass

// glassShell builds a glazed dome's walls and dome, and their frame.
func (b *building) glassShell(r Room) {
	bp := r.plan
	in := ToWorld(r.Center.Add(vec3{0, 1.2, 0}))
	glass := b.Part("glass.pane", mesh.LitGlass, in)
	frame := b.out(bp.Style)
	panels := r.panels()
	for i, p := range panels {
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		tangent := vec3{-out.Z, 0, out.X}
		if p.y0 > 0 { // over a doorway, solid
			frame.Box(p.mid.Add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, WallThick}, tangent, 0.03)
		} else {
			b.out("ceramic.dark").Box(p.mid.Add(vec3{0, glassSill / 2, 0}), vec3{p.width * 0.98, glassSill, WallThick + 0.04}, tangent, 0.02)
			glass.Box(p.mid.Add(vec3{0, (glassSill + p.y1) / 2, 0}), vec3{p.width, p.y1 - glassSill, 0.03}, tangent, 0)
		}
		// A mullion at the panel's edge, over the joints, the collar on
		// top; between two over a doorway, none in it.
		edge := p.mid.Add(tangent.Scale(p.width / 2))
		foot := 0.01
		if next := panels[(i+1)%len(panels)]; p.y0 > 0 && next.y0 > 0 {
			foot = p.y0 + 0.01
		}
		frame.Box(edge.Add(vec3{0, (foot + p.y1 - 0.13) / 2, 0}), vec3{0.07, p.y1 - 0.13 - foot, WallThick + 0.06}, tangent, 0.01)
		frame.Box(p.mid.Add(vec3{0, p.y1 - 0.06, 0}), vec3{p.width * 0.99, 0.12, WallThick + 0.08}, tangent, 0.02)
	}
	b.glassDome(r)
}

// glassDome builds a dome of glass: its gores, white ribs along their
// edges and three rings across, a cap at the top.
func (b *building) glassDome(r Room) {
	in := ToWorld(r.Center.Add(vec3{0, 1.2, 0}))
	glass, frame := b.Part("glass.pane", mesh.LitGlass, in), b.out(r.plan.Style)
	top := r.Center.Add(vec3{0, r.WallH, 0})
	a, bh := r.Radius+0.02, r.Radius*0.55
	glass.Dome(top, a, bh)
	pt := func(t, phi float64) vec3 {
		return top.Add(vec3{math.Cos(t) * a * math.Cos(phi), bh * math.Sin(phi), math.Sin(t) * a * math.Cos(phi)})
	}
	bar := func(p0, p1 vec3) {
		along := p1.Sub(p0)
		l := along.Length()
		along = along.Scale(1 / l)
		side := along.Cross(vec3{0, 1, 0})
		if side.Length() < 1e-6 {
			side = vec3{1, 0, 0}
		}
		side = side.Norm()
		nrm := side.Cross(along)
		mid := p0.Lerp(p1, 0.5)
		if nrm.Dot(mid.Sub(top)) < 0 {
			nrm = nrm.Scale(-1)
		}
		frame.BoxBasis(mid.Add(nrm.Scale(0.03)), [3]vec3{side, along, nrm}, vec3{0.07, l + 0.06, 0.07}, 0.01)
	}
	const steps, capPhi = 8, 1.38
	for g := range mesh.DomeGores {
		t := float64(g) * 2 * math.Pi / mesh.DomeGores
		for i := range steps {
			bar(pt(t, capPhi*float64(i)/steps), pt(t, capPhi*float64(i+1)/steps))
		}
		for _, phi := range []float64{0.4, 0.8, 1.15} {
			bar(pt(t, phi), pt(t+2*math.Pi/mesh.DomeGores, phi))
		}
	}
	frame.Prism(top.Add(vec3{0, bh*math.Sin(capPhi) - 0.05, 0}), yUp, a*math.Cos(capPhi)+0.08, 0.15, 0.25, 16, 0.02)
}

// glassFloor lays a glazed dome's floor, lit by the sun: warm ceramic
// tiles, a lighter path from each doorway to the middle.
func (b *building) glassFloor(r Room) {
	floor := r.Center.Add(vec3{0, FloorTop - 0.1, 0})
	b.out("ceramic.sand").Prism(floor, yUp, r.Radius, r.Radius, 0.1, 48, 0)
	for _, o := range r.openings {
		d := vec3{math.Cos(o), 0, math.Sin(o)}
		l := r.Radius - 0.1
		b.out("pad.concrete").Box(r.Center.Add(d.Scale(l/2)).Add(vec3{0, FloorTop - 0.005, 0}), vec3{l, 0.012, 1.3}, d, 0)
	}
}

// GreenBeds are a greenhouse's beds: a ring along the glass, open toward
// its doors, and a round bed in the middle.
type GreenBeds struct {
	in, out float64
	segs    int
	doors   []float64 // the ways of the doors, in radians
	gap     float64   // half an opening's angle, before a door
	middle  float64   // the middle bed's radius
}

var (
	// BaseBeds are the base's greenhouse's: its door at +Z.
	BaseBeds = GreenBeds{2.7, 3.9, 20, []float64{math.Pi / 2}, 0.36, 1.1}
	// SerreBeds are those of the greenhouse set down anywhere, walked
	// through from +X to -X around the middle.
	SerreBeds = GreenBeds{3.1, 4.4, 22, []float64{0, math.Pi}, 0.3, 1.25}
)

// segments are the angles of the ring's segments, the openings left out.
func (g GreenBeds) segments() []float64 {
	var out []float64
	for k := range g.segs {
		t := (float64(k) + 0.5) * 2 * math.Pi / float64(g.segs)
		open := false
		for _, d := range g.doors {
			open = open || angleDist(t, d) <= g.gap+math.Pi/float64(g.segs)
		}
		if !open {
			out = append(out, t)
		}
	}
	return out
}

// Obstacles: what stops the player among the beds, and how wide each is.
func (g GreenBeds) Obstacles() (at []vec3, radius []float64) {
	mid := (g.in + g.out) / 2
	for _, t := range g.segments() {
		at = append(at, vec3{math.Cos(t), 0, math.Sin(t)}.Scale(mid))
		radius = append(radius, (g.out-g.in)/2+0.05)
	}
	return append(at, vec3{}), append(radius, g.middle+0.2)
}

// BuildSerre: the greenhouse set down anywhere, full of plants, two young
// trees growing out of its ring.
func BuildSerre(b *building, at vec3, seed int) {
	SerreBeds.build(b, at, seed)
	for k, t := range []float64{math.Pi / 2, -math.Pi / 2} {
		p := at.Add(vec3{math.Cos(t), 0, math.Sin(t)}.Scale((SerreBeds.in + SerreBeds.out) / 2)).Add(vec3{0, 0.45, 0})
		b.tree(p, 0.7+0.1*float64(k), seed+k)
	}
	// A bench against the middle bed, on each side of the path.
	for _, s := range []float64{-1, 1} {
		p := at.Add(vec3{0, 0, s * (SerreBeds.middle + 0.24)})
		b.out("wood.light").Box(p.Add(vec3{0, 0.43, 0}), vec3{1.5, 0.06, 0.38}, xAxis, 0.015)
		for _, x := range []float64{-0.6, 0.6} {
			b.out("ceramic.dark").Box(p.Add(vec3{x, 0.2, 0}), vec3{0.1, 0.4, 0.32}, xAxis, 0.01)
		}
	}
}

// tree grows a small tree of round facets on the soil at p.
func (b *building) tree(p vec3, size float64, seed int) {
	b.out("wood.brown").Prism(p, yUp, 0.1*size, 0.06*size, 1.5*size, 8, 0)
	for i, l := range []struct{ y, r, h float64 }{{1.1, 0.95, 0.9}, {1.7, 0.7, 0.75}, {2.25, 0.42, 0.6}} {
		tok := []string{"leaf.dark", "planter.green", "planter.lime"}[(i+seed)%3]
		b.out(tok).Prism(p.Add(vec3{0, l.y * size, 0}), yUp, l.r*size, 0.1*size, l.h*size, 9, 0.03)
	}
}

// BuildGreenhousePlants: a greenhouse full of plants, lit by the sun: beds
// of fired ceramic in a ring along the glass, open toward the door at +Z,
// thick with bushes, tomatoes on stakes, ferns, broad leaves and flowers;
// a small tree in the round bed in the middle.
func BuildGreenhousePlants(b *building, at vec3, seed int) { BaseBeds.build(b, at, seed) }

func (g GreenBeds) build(b *building, at vec3, seed int) {
	mid, w := (g.in+g.out)/2, g.out-g.in
	chord := 2 * mid * math.Sin(math.Pi/float64(g.segs)) * 1.08
	for k, t := range g.segments() {
		out := vec3{math.Cos(t), 0, math.Sin(t)}
		across := vec3{-out.Z, 0, out.X}
		c := at.Add(out.Scale(mid))
		b.out("ceramic").Box(c.Add(vec3{0, 0.22, 0}), vec3{chord, 0.44, w}, across, 0.03)
		b.out("soil").Box(c.Add(vec3{0, 0.445, 0}), vec3{chord - 0.1, 0.02, w - 0.12}, across, 0)
		for j := range 3 {
			r := grnd(seed+k, j)
			p := c.Add(across.Scale((float64(j) - 1) * chord / 3)).Add(out.Scale((r - 0.5) * 0.5)).Add(vec3{0, 0.45, 0})
			b.plant(p, int(grnd(seed*7+k, j+5)*6), seed+k*3+j)
		}
	}
	// The middle bed and its tree.
	b.out("ceramic.dark").Prism(at, yUp, g.middle, g.middle, 0.5, 24, 0.03)
	b.out("soil").Prism(at.Add(vec3{0, 0.5, 0}), yUp, g.middle-0.08, g.middle-0.08, 0.02, 24, 0)
	b.out("wood.brown").Prism(at.Add(vec3{0, 0.5, 0}), yUp, 0.13, 0.08, 1.5, 8, 0)
	for i, l := range []struct{ y, r, h float64 }{{1.4, 1.15, 1.0}, {2.05, 0.85, 0.85}, {2.65, 0.5, 0.7}} {
		b.out([]string{"leaf.dark", "planter.green", "planter.lime"}[i]).Prism(at.Add(vec3{0, l.y, 0}), yUp, l.r, 0.12, l.h, 9, 0.04)
	}
	for k := range 8 {
		t := float64(k) * math.Pi / 4
		p := at.Add(vec3{math.Cos(t), 0, math.Sin(t)}.Scale(0.75)).Add(vec3{0, 0.5, 0})
		b.out([]string{"bloom.coral", "bloom.yellow"}[k%2]).Prism(p, yUp, 0.12, 0.05, 0.22, 6, 0)
	}
}

// plant grows one plant of a kind on the soil at p: a bush, tomatoes on
// a stake, a fern, broad leaves, flowers or a tall grass.
func (b *building) plant(p vec3, kind, seed int) {
	r := grnd(seed, 1)
	leaf := []string{"planter.green", "planter.lime", "leaf.dark", "leaf.olive"}[seed%4]
	switch kind {
	case 0: // a bush
		b.out(leaf).Prism(p, yUp, 0.32+0.1*r, 0.08, 0.6+0.5*r, 7, 0.03)
		b.out("leaf.dark").Prism(p.Add(vec3{0.15, 0, -0.1}), yUp, 0.22, 0.05, 0.45+0.3*r, 6, 0.02)
	case 1: // tomatoes on a stake
		b.out("wood.light").Prism(p, yUp, 0.02, 0.02, 1.7, 5, 0)
		for i := range 4 {
			y := 0.2 + float64(i)*0.38
			b.out(leaf).Prism(p.Add(vec3{0, y, 0}), yUp, 0.26-0.04*float64(i), 0.06, 0.36, 6, 0.02)
			b.out("bloom.coral").Prism(p.Add(vec3{0.14, y + 0.1, 0.06}), yUp, 0.05, 0.05, 0.08, 6, 0)
		}
	case 2: // a fern
		b.tuftsLit(p, xAxis, vec3{0, 0, 1}, seed, 2, mesh.LitSun)
	case 3: // broad leaves fanning out
		for i := range 6 {
			t := float64(i)*1.05 + r
			out := vec3{math.Cos(t), 0, math.Sin(t)}
			d := out.Add(vec3{0, 1.3, 0}).Norm()
			across := vec3{-out.Z, 0, out.X}
			b.out(leaf).BoxBasis(p.Add(d.Scale(0.4)), [3]vec3{across, d, across.Cross(d)}, vec3{0.24, 0.8, 0.02}, 0.01)
		}
	case 4: // flowers
		b.out("leaf.olive").Prism(p, yUp, 0.28, 0.2, 0.3, 7, 0.02)
		for i := range 5 {
			t := float64(i)*1.3 + r
			q := p.Add(vec3{math.Cos(t) * 0.16, 0.3 + 0.06*float64(i%2), math.Sin(t) * 0.16})
			b.out([]string{"bloom.coral", "bloom.yellow"}[(seed+i)%2]).Prism(q, yUp, 0.07, 0.04, 0.06, 6, 0)
		}
	default: // tall grass
		for i := range 7 {
			t := float64(i)*0.9 + r
			d := vec3{math.Cos(t) * 0.3, 1, math.Sin(t) * 0.3}.Norm()
			b.out(leaf).Prism(p, d, 0.03, 0.005, 0.9+0.4*grnd(seed, i), 4, 0)
		}
	}
}
