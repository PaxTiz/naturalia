package base

import "math"

// Warm finishes for the living spaces, laid as kits where they are wanted:
// acoustic felt panels on a wall, a wall of tiles in the ceramic fired on
// site, a tiled or technical floor over a zone; and lamps low down, where
// one sits, for an evening light. Each stands at its spot
// with its back against the wall at -X (a floor, centered on it), facing
// +X, unless said otherwise.

// feltTones are the felt panels' tones by seed: terracotta, sand or sage
// leading, the others between.
var feltTones = [][]string{
	{"felt.terracotta", "felt.sand", "felt.terracotta", "felt.sage", "felt.terracotta"},
	{"felt.sand", "felt.terracotta", "felt.sand", "felt.sand", "felt.sage"},
	{"felt.sage", "felt.sand", "felt.sage", "felt.terracotta", "felt.sage"},
}

// feltWall lays acoustic felt panels w wide (along Z) and up to h high
// from at: vertical boards of a few heights side by side, soft-edged,
// their tones by seed.
func (b *building) feltWall(at vec3, w, h float64, seed int) {
	const gap, th = 0.04, 0.04
	tones := feltTones[seed%len(feltTones)]
	n := len(tones)
	bw := (w - gap*float64(n-1)) / float64(n)
	for k, tok := range tones {
		r := grnd(seed, k)
		y0, y1 := 0.12*r, h-0.25*grnd(seed, k+9)
		z := -w/2 + bw/2 + float64(k)*(bw+gap)
		b.in(tok).Box(at.Add(vec3{th / 2, (y0 + y1) / 2, z}), vec3{th, y1 - y0, bw}, xAxis, 0.015)
	}
}

// BuildFeltWall: felt panels 2.8 m wide, 2.4 m high.
func BuildFeltWall(b *building, at vec3, seed int) { b.feltWall(at, 2.8, 2.4, seed) }

// BuildFeltLow: felt panels 2.4 m wide, 1.3 m high, over a seat.
func BuildFeltLow(b *building, at vec3, seed int) { b.feltWall(at, 2.4, 1.3, seed) }

// ceramicTone is the tone of tile k of a ceramic wall or floor: mostly
// the clay's own, some darker, a few lighter.
func ceramicTone(seed, k int) string {
	switch r := grnd(seed, k); {
	case r < 0.25:
		return "ceramic.dark"
	case r < 0.33:
		return "ceramic.sand"
	}
	return "ceramic"
}

// BuildCeramicWall: a wall of small tiles 2.4 m wide, 2.4 m high, in the
// ceramic's tones, on a sand-grouted backing, a slim anthracite ledge on
// top lit warm underneath.
func BuildCeramicWall(b *building, at vec3, seed int) {
	const w, h, tile, joint = 2.4, 2.4, 0.2, 0.008
	b.in("ceramic.sand").Box(at.Add(vec3{0.005, h / 2, 0}), vec3{0.01, h, w}, xAxis, 0)
	nz, ny := int(w/tile), int(h/tile)
	for j := range ny {
		for i := range nz {
			c := at.Add(vec3{0.02, (float64(j) + 0.5) * tile, -w/2 + (float64(i)+0.5)*tile})
			b.in(ceramicTone(seed, j*nz+i)).Box(c, vec3{0.02, tile - joint, tile - joint}, xAxis, 0.003)
		}
	}
	b.in("home.trim").Box(at.Add(vec3{0.08, h + 0.02, 0}), vec3{0.16, 0.04, w + 0.04}, xAxis, 0.01)
	b.in("home.led.amber").Box(at.Add(vec3{0.12, h - 0.002, 0}), vec3{0.03, 0.006, w - 0.1}, xAxis, 0)
}

// ceramicFloor tiles a floor w across X and d across Z, centered on at,
// in square tiles over a dark grout bed, an anthracite edge round it.
func (b *building) ceramicFloor(at vec3, w, d float64, seed int) {
	const tile, joint, th = 0.4, 0.008, 0.012
	base := at.Add(vec3{0, FloorTop, 0})
	b.in("ceramic.dark").Box(base.Add(vec3{0, 0.003, 0}), vec3{w, 0.006, d}, xAxis, 0)
	nx, nz := int(w/tile+0.5), int(d/tile+0.5)
	tx, tz := w/float64(nx), d/float64(nz)
	for i := range nx {
		for j := range nz {
			c := base.Add(vec3{-w/2 + (float64(i)+0.5)*tx, 0.006 + th/2, -d/2 + (float64(j)+0.5)*tz})
			b.in(ceramicTone(seed, i*nz+j)).Box(c, vec3{tx - joint, th, tz - joint}, xAxis, 0.002)
		}
	}
	b.floorEdge(base, w, d, 0.006+th)
}

// floorEdge runs a thin anthracite edge round a zone of floor w by d,
// centered on at, h high.
func (b *building) floorEdge(at vec3, w, d, h float64) {
	const e = 0.03
	trim := b.in("home.trim")
	for _, s := range []float64{-1, 1} {
		trim.Box(at.Add(vec3{s * (w/2 + e/2), h / 2, 0}), vec3{e, h, d + 2*e}, xAxis, 0)
		trim.Box(at.Add(vec3{0, h / 2, s * (d/2 + e/2)}), vec3{w, h, e}, xAxis, 0)
	}
}

// BuildCeramicFloor: a tiled floor of 4 m across X by 3 m.
func BuildCeramicFloor(b *building, at vec3, seed int) { b.ceramicFloor(at, 4, 3, seed) }

// BuildTechFloor: a technical floor of 5 m by 5 m, in plates with soft
// joints, where work is done.
func BuildTechFloor(b *building, at vec3, _ int) {
	const w, d, th = 5, 5, 0.014
	base := at.Add(vec3{0, FloorTop, 0})
	b.in("home.path").Box(base.Add(vec3{0, th / 2, 0}), vec3{w, th, d}, xAxis, 0)
	b.floorEdge(base, w, d, th)
}

// arcReach is how far an arc lamp's shade hangs from its foot, along +X,
// and arcShade how high.
const arcReach, arcShade = 2.3, 1.95

// ArcLight is where an arc lamp's light is, from its foot.
var ArcLight = vec3{arcReach, arcShade - 0.12, 0}

// BuildArcLamp: a floor lamp whose arm arcs over to hang its shade above
// a low table: a heavy ceramic foot, a slim anthracite arm, a wide shade
// lit amber inside.
func BuildArcLamp(b *building, at vec3, _ int) {
	b.in("ceramic.dark").Prism(at, yUp, 0.2, 0.18, 0.06, 24, 0.02)
	const n = 14
	pt := func(k int) vec3 {
		// A quarter ellipse up from the foot, over and down to the shade.
		a := float64(k) / n * math.Pi / 2
		return at.Add(vec3{arcReach * (1 - math.Cos(a)), 0.06 + (arcShade+0.35)*math.Sin(a), 0})
	}
	arm := b.in("home.trim")
	for k := range n {
		p0, p1 := pt(k), pt(k+1)
		d := p1.Sub(p0)
		arm.Prism(p0, d.Norm(), 0.014, 0.014, d.Length()+0.004, 8, 0)
	}
	top := pt(n)
	shade := at.Add(vec3{arcReach, arcShade, 0})
	arm.Prism(shade.Add(vec3{0, 0.12, 0}), yUp, 0.01, 0.01, top.Y-shade.Y-0.12, 6, 0)
	b.in("home.trim").Prism(shade.Add(vec3{0, 0.14, 0}), vec3{0, -1, 0}, 0.06, 0.3, 0.16, 24, 0)
	b.in("lamp.amber").Prism(shade.Sub(vec3{0, 0.025, 0}), yUp, 0.27, 0.27, 0.01, 24, 0)
}

// FloorLight is where a floor lamp's light is, over its foot.
var FloorLight = vec3{0, 1.35, 0}

// BuildFloorLamp: a slim floor lamp, an oat fabric drum on an anthracite
// rod over a ceramic foot, glowing amber top and bottom.
func BuildFloorLamp(b *building, at vec3, _ int) {
	b.in("ceramic.dark").Prism(at, yUp, 0.16, 0.14, 0.04, 20, 0.015)
	b.in("home.trim").Prism(at.Add(vec3{0, 0.04, 0}), yUp, 0.012, 0.012, 1.2, 8, 0)
	b.in("fabric.oat").Prism(at.Add(vec3{0, 1.18, 0}), yUp, 0.2, 0.2, 0.36, 24, 0)
	for _, y := range []float64{1.175, 1.545} {
		b.in("lamp.amber").Prism(at.Add(vec3{0, y, 0}), yUp, 0.185, 0.185, 0.01, 24, 0)
	}
}

// BuildCanopy: a round acoustic canopy of felt, 2.4 m across its radius,
// hung at its spot's height on four cables 3 m long: it lowers the
// ceiling over a lounge. A warm line of light runs round its rim, under
// it.
func BuildCanopy(b *building, at vec3, _ int) {
	const rad, th = 2.4, 0.06
	b.in("felt.sand").Prism(at, yUp, rad, rad, th, 64, 0.02)
	b.in("home.trim").Prism(at.Sub(vec3{0, 0.012, 0}), yUp, rad+0.02, rad+0.02, 0.012, 64, 0)
	b.in("home.led.amber").Prism(at.Sub(vec3{0, 0.016, 0}), yUp, rad-0.1, rad-0.1, 0.006, 64, 0)
	b.in("felt.sand").Prism(at.Sub(vec3{0, 0.02, 0}), yUp, rad-0.16, rad-0.16, 0.012, 64, 0)
	for k := range 4 {
		a := float64(k)*math.Pi/2 + math.Pi/4
		p := at.Add(vec3{math.Cos(a), 0, math.Sin(a)}.Scale(rad * 0.8))
		b.in("home.trim").Prism(p.Add(vec3{0, th, 0}), yUp, 0.005, 0.005, 3, 4, 0)
	}
}

// pottedPlant sets a plant in a ceramic pot on at: a pot of radius rad,
// leaves arching out of it about h high.
func (b *building) pottedPlant(at vec3, rad, h float64, seed int) {
	b.in(ceramicTone(seed, 1)).Prism(at, yUp, rad*0.8, rad, rad*1.4, 16, 0.01)
	b.in("soil").Prism(at.Add(vec3{0, rad * 1.3, 0}), yUp, rad*0.92, rad*0.92, 0.02, 16, 0)
	from := at.Add(vec3{0, rad * 1.3, 0})
	for i := range 11 {
		a := float64(i)*2.4 + 6*grnd(seed, i)
		out := vec3{math.Cos(a), 0, math.Sin(a)}
		lift := 0.4 + 1.6*grnd(seed, i+20)
		tok := "planter.green"
		if i%3 == 1 {
			tok = "planter.lime"
		}
		leaf(b.in(tok), from.Add(out.Scale(rad*0.2)), out.Add(vec3{0, lift, 0}).Norm(), h*(0.5+0.4*grnd(seed, i+40)))
	}
}

// BuildDivider: a low open bookcase 2.4 m long (along Z), 1.05 m high,
// open on both sides, between two zones: white, in two rows of three
// cubbies with books, oat fabric boxes and ceramic vases, plants on top.
func BuildDivider(b *building, at vec3, seed int) {
	const l, h, deep, board = 2.4, 1.05, 0.4, 0.03
	p := func(y, z float64) vec3 { return at.Add(vec3{0, y, z}) }
	b.in("home.trim").Box(p(0.05, 0), vec3{deep - 0.08, 0.1, l - 0.06}, xAxis, 0.01)
	white := b.in("home.panel")
	for _, y := range []float64{0.1 + board/2, 0.1 + (h-0.1)/2, h - board/2} {
		white.Box(p(y, 0), vec3{deep, board, l}, xAxis, 0.008)
	}
	for k := range 4 {
		z := -l/2 + board/2 + float64(k)*(l-board)/3
		white.Box(p(0.1+(h-0.1)/2, z), vec3{deep, h - 0.1, board}, xAxis, 0.008)
	}
	cell := (l - board) / 3
	for row, y := range []float64{0.1 + board, 0.1 + (h-0.1)/2 + board/2} {
		for k := range 3 {
			z := -l/2 + board + (float64(k)+0.5)*cell - board/2
			c := p(y, z)
			switch (seed + row*3 + k) % 4 {
			case 0: // books standing, leaning
				books := []string{"fabric.rust", "felt.sage", "crate.sand", "fabric.ochre", "fabric.slate"}
				for i := range 6 {
					r := grnd(seed+row, k*10+i)
					bh := 0.24 + 0.08*r
					b.in(books[(i+k)%len(books)]).Box(c.Add(vec3{0, bh / 2, -cell/2 + 0.08 + float64(i)*0.05}), vec3{0.2, bh, 0.04}, xAxis, 0.004)
				}
			case 1: // a fabric box
				b.in("fabric.oat").Box(c.Add(vec3{0, 0.14, 0}), vec3{0.32, 0.28, cell - 0.12}, xAxis, 0.02)
			case 2: // a vase
				b.in(ceramicTone(seed, row*3+k+50)).Prism(c, yUp, 0.06, 0.08, 0.26, 16, 0.01)
			}
		}
	}
	for k, z := range []float64{-0.7, 0.75} {
		b.pottedPlant(p(h, z), 0.1, 0.32, seed+k)
	}
}

// DividerObstacles stop the player along a divider.
var DividerObstacles = []vec3{{0, 0, -0.9}, {0, 0, 0}, {0, 0, 0.9}}
