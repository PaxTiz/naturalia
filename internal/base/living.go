package base

import (
	"math"
)

// The great hall as a place to live (interior: "home"): a lounge, a
// banquette curving round a low table on a felt rug; built-in storage
// along the walls instead of the site's crates; hydroponic planters under
// the lookout, the only green in it. Each faces +X, standing on the floor
// at its spot, unless said otherwise.

// sofaRadius is how far the arc banquette's seat runs from its middle,
// and sofaSpan the angle it covers, its back toward -X.
const (
	sofaRadius = 1.7
	sofaSpan   = 0.72 * 2 * math.Pi
)

// sofaAngles are the angles (from +X toward +Z) the banquette's pieces
// span, n of them.
func sofaAngles(n int) []float64 {
	var out []float64
	for k := 0; k <= n; k++ {
		out = append(out, math.Pi-sofaSpan/2+sofaSpan*float64(k)/float64(n))
	}
	return out
}

// SofaObstacles are what stops the player around the banquette.
func SofaObstacles() []vec3 {
	var out []vec3
	for _, a := range sofaAngles(6) {
		out = append(out, vec3{math.Cos(a), 0, math.Sin(a)}.Scale(sofaRadius+0.1))
	}
	return out
}

// BuildSofaArc: a banquette curving round its spot, open toward +X: a
// recessed plinth, a white base, oat seat and back cushions against a
// white shell, two cushions of rust and ochre.
func BuildSofaArc(b *building, at vec3, _ int) {
	const n = 20
	as := sofaAngles(n)
	for k := range n {
		a0, a1 := as[k], as[k+1]
		mid := (a0 + a1) / 2
		out := vec3{math.Cos(mid), 0, math.Sin(mid)}
		piece := func(r, w float64) (vec3, vec3, float64) {
			p0 := at.Add(vec3{math.Cos(a0), 0, math.Sin(a0)}.Scale(r))
			p1 := at.Add(vec3{math.Cos(a1), 0, math.Sin(a1)}.Scale(r))
			d := p1.Sub(p0)
			return p0.Lerp(p1, 0.5), d, d.Length() + w
		}
		c, d, l := piece(sofaRadius-0.1, 0.03)
		b.in("home.trim").Box(c.Add(vec3{0, 0.05, 0}), vec3{l, 0.1, 0.5}, d, 0.01)
		c, d, l = piece(sofaRadius, 0.04)
		b.in("home.panel2").Box(c.Add(vec3{0, 0.25, 0}), vec3{l, 0.3, 0.7}, d, 0.03)
		b.in("fabric.oat").Box(c.Sub(out.Scale(0.04)).Add(vec3{0, 0.46, 0}), vec3{l - 0.02, 0.12, 0.62}, d, 0.04)
		c, d, l = piece(sofaRadius+0.42, 0.05)
		b.in("home.panel").Box(c.Add(vec3{0, 0.45, 0}), vec3{l, 0.9, 0.14}, d, 0.05)
		c, d, l = piece(sofaRadius+0.24, 0.02)
		b.in("fabric.oat").Box(c.Add(vec3{0, 0.68, 0}), vec3{l - 0.02, 0.38, 0.18}, d, 0.06)
	}
	for k, a := range []float64{as[3], as[n/2], as[n-4]} {
		p := at.Add(vec3{math.Cos(a), 0, math.Sin(a)}.Scale(sofaRadius + 0.05))
		out := vec3{math.Cos(a), 0, math.Sin(a)}
		b.in([]string{"fabric.rust", "fabric.ochre", "fabric.rust"}[k]).BoxBasis(p.Add(vec3{0, 0.7, 0}), [3]vec3{{-out.Z, 0, out.X}, vec3{0, 0.97, 0}.Add(out.Scale(0.25)).Norm(), out}, vec3{0.42, 0.36, 0.12}, 0.05)
	}
}

// BuildCoffeeTable: a round white top on an anthracite pedestal.
func BuildCoffeeTable(b *building, at vec3, _ int) {
	b.in("home.trim").Prism(at, yUp, 0.32, 0.32, 0.03, 32, 0.01)
	b.in("home.trim").Prism(at, yUp, 0.12, 0.12, 0.4, 16, 0)
	b.in("home.panel").Prism(at.Add(vec3{0, 0.4, 0}), yUp, 0.6, 0.6, 0.05, 40, 0.02)
}

// BuildFeltRug: a round felt rug, 2.6 m across its radius, a darker ring
// round a lighter middle.
func BuildFeltRug(b *building, at vec3, _ int) {
	c := at.Add(vec3{0, FloorTop, 0})
	b.in("fabric.felt").Prism(c, yUp, 2.6, 2.6, 0.012, 64, 0)
	b.in("fabric.felt2").Prism(c, yUp, 2.2, 2.2, 0.014, 64, 0)
}

// BuildStorage: a built-in cupboard 2.4 m wide against the wall at its
// back (-X): white doors with fine joints over a recessed plinth, an open
// niche at chest height lit from above, a few boxes in it.
func BuildStorage(b *building, at vec3, seed int) {
	const w, h, deep = 2.4, 2.3, 0.5
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	b.in("home.trim").Box(p(-0.04, 0.05, 0), vec3{deep - 0.08, 0.1, w - 0.04}, xAxis, 0)
	b.in("home.panel").Box(p(0, (h+0.1)/2, 0), vec3{deep - 0.02, h - 0.1, w}, xAxis, 0.02)
	const niche0, niche1 = 1.15, 1.55
	for _, z := range []float64{-0.9, -0.3, 0.3, 0.9} {
		for _, y := range [][2]float64{{0.11, niche0 - 0.01}, {niche1 + 0.01, h - 0.01}} {
			b.in("home.panel2").Box(p(deep/2, (y[0]+y[1])/2, z), vec3{0.02, y[1] - y[0], 0.594}, xAxis, 0.006)
		}
	}
	// The niche, set in the front, lit along its top.
	b.in("home.trim").Box(p(deep/2-0.15, (niche0+niche1)/2, 0), vec3{0.32, niche1 - niche0, w - 0.08}, xAxis, 0)
	b.in("home.led").Box(p(deep/2-0.04, niche1-0.012, 0), vec3{0.03, 0.01, w - 0.12}, xAxis, 0)
	z := -1.0
	for k := 0; z < 0.95; k++ {
		r := grnd(seed, k)
		s := 0.16 + 0.12*r
		tok := []string{"crate.slate", "home.panel2", "fabric.teal", "crate.sand"}[(seed+k)%4]
		if r < 0.25 {
			b.in("planter.green").Prism(p(deep/2-0.12, niche0, z+0.08), yUp, 0.06, 0.1, 0.14, 10, 0.01)
			b.in("clay").Prism(p(deep/2-0.12, niche0, z+0.08), yUp, 0.07, 0.06, 0.1, 12, 0.01)
			z += 0.26
			continue
		}
		b.in(tok).Box(p(deep/2-0.14, niche0+s*0.4, z+s/2), vec3{0.24, s * 0.8, s}, xAxis, 0.015)
		z += s + 0.06
	}
}

// StorageObstacles are what stops the player around a cupboard.
var StorageObstacles = []vec3{{0, 0, -0.85}, {0, 0, 0}, {0, 0, 0.85}}

// PlanterLight is where a home planter's grow light shines from: its
// light spills a little round it, under the lookout.
var PlanterLight = vec3{-0.1, 1.6, 0}

// BuildHomePlanter: a hydroponic trough in white composite on a recessed
// plinth, along Z, its rows of plants, and a slim grow light over it on
// two thin arms.
func BuildHomePlanter(b *building, at vec3, seed int) {
	z := vec3{0, 0, 1}
	b.in("home.trim").Box(at.Add(vec3{0, 0.06, 0}), vec3{0.45, 0.12, 1.7}, xAxis, 0.01)
	b.in("home.panel2").Box(at.Add(vec3{0, 0.41, 0}), vec3{0.6, 0.6, 1.8}, xAxis, 0.06)
	b.in("soil").Box(at.Add(vec3{0, 0.715, 0}), vec3{0.5, 0.02, 1.7}, xAxis, 0)
	b.tufts(at.Add(vec3{0, 0.72, 0}), z, xAxis, seed, 7)
	for _, s := range []float64{-0.82, 0.82} {
		b.in("home.trim").Box(at.Add(vec3{-0.26, 1.2, s}), vec3{0.03, 0.96, 0.03}, xAxis, 0)
	}
	b.in("home.trim").Box(at.Add(vec3{-0.12, 1.69, 0}), vec3{0.3, 0.04, 1.7}, xAxis, 0.012)
	b.in("lamp.grow").Box(at.Add(vec3{-0.1, 1.665, 0}), vec3{0.2, 0.01, 1.6}, xAxis, 0)
}
