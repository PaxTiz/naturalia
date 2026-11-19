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

// BuildEntry: where one comes in from outside and leaves it behind,
// against the wall at -X, 2.4 m along Z: a white bench with boots under
// it (-Z end), a shelf over it with a helmet and pegs below, a jacket
// and a scarf on them, and at the +Z end a suit hung up to dry.
func BuildEntry(b *building, at vec3, _ int) {
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	// The bench, its oat cushion, boots under it.
	b.in("home.trim").Box(p(0.2, 0.04, -0.5), vec3{0.34, 0.08, 1.36}, xAxis, 0.01)
	b.in("home.panel2").Box(p(0.21, 0.26, -0.5), vec3{0.4, 0.36, 1.4}, xAxis, 0.03)
	b.in("fabric.oat").Box(p(0.21, 0.47, -0.5), vec3{0.38, 0.06, 1.36}, xAxis, 0.025)
	for k, z := range []float64{-1.0, -0.78, -0.3, -0.08} {
		x := 0.52 + 0.05*grnd(3, k)
		b.in("fabric.slate").Box(p(x, 0.13, z), vec3{0.26, 0.26, 0.1}, xAxis, 0.03)
		b.in("fabric.slate").Box(p(x+0.09, 0.04, z), vec3{0.12, 0.08, 0.1}, xAxis, 0.02)
	}
	// The shelf and its pegs.
	b.in("home.panel").Box(p(0.14, 1.8, -0.5), vec3{0.28, 0.03, 1.4}, xAxis, 0.008)
	b.in("home.led.amber").Box(p(0.2, 1.783, -0.5), vec3{0.03, 0.006, 1.3}, xAxis, 0)
	b.helmet(p(0.15, 1.815, -0.85), 0.4)
	for k := range 4 {
		z := -1.0 + float64(k)*0.33
		b.in("home.trim").Prism(p(0.02, 1.6, z), xAxis, 0.012, 0.012, 0.1, 8, 0)
	}
	// A jacket and a scarf hanging from them.
	b.in("fabric.rust").Box(p(0.11, 1.28, -0.67), vec3{0.1, 0.62, 0.42}, xAxis, 0.04)
	b.in("fabric.ochre").Box(p(0.1, 1.33, -0.01), vec3{0.04, 0.5, 0.14}, xAxis, 0.02)
	// The suit, hung up by its shoulders: torso, arms, legs, gloves.
	suit, dark := b.in("habitat.white"), b.in("fabric.slate")
	b.in("home.trim").Box(p(0.06, 1.72, 0.75), vec3{0.12, 0.03, 0.5}, xAxis, 0.01)
	suit.Box(p(0.2, 1.38, 0.75), vec3{0.26, 0.6, 0.48}, xAxis, 0.08)
	b.in("fabric.ochre").Box(p(0.335, 1.45, 0.75), vec3{0.01, 0.12, 0.3}, xAxis, 0)
	for _, s := range []float64{-1, 1} {
		suit.Box(p(0.2, 1.3, 0.75+s*0.31), vec3{0.15, 0.62, 0.14}, xAxis, 0.06)
		dark.Box(p(0.2, 0.95, 0.75+s*0.31), vec3{0.12, 0.12, 0.1}, xAxis, 0.03)
		suit.Box(p(0.2, 0.68, 0.75+s*0.12), vec3{0.18, 0.62, 0.17}, xAxis, 0.06)
		dark.Box(p(0.22, 0.3, 0.75+s*0.12), vec3{0.2, 0.14, 0.15}, xAxis, 0.04)
	}
	dark.Box(p(0.2, 1.06, 0.75), vec3{0.27, 0.06, 0.46}, xAxis, 0.02)
}

// EntryObstacles stop the player at the bench and the suit.
var EntryObstacles = []vec3{{0.25, 0, -0.9}, {0.25, 0, -0.2}, {0.25, 0, 0.75}}

// BuildPouf: a round pouf of the seed's fabric, a seam round its middle.
func BuildPouf(b *building, at vec3, seed int) {
	tok := []string{"fabric.ochre", "fabric.rust", "fabric.oat"}[seed%3]
	b.in(tok).Prism(at, yUp, 0.26, 0.26, 0.4, 24, 0.06)
	b.in("fabric.slate").Prism(at.Add(vec3{0, 0.195, 0}), yUp, 0.262, 0.262, 0.01, 24, 0)
}

// BuildFrame: a picture 0.9 m wide, 0.65 m high, in a slim anthracite
// frame with a white mat, on the wall at -X, its middle at the spot; the
// seed picks it: a Martian sunset, the Earth seen from here, a mission
// patch, a map of the region.
func BuildFrame(b *building, at vec3, seed int) {
	const w, h = 0.9, 0.65
	p := func(x, y, z float64) vec3 { return at.Add(vec3{x, y, z}) }
	b.in("home.trim").Box(p(0.015, 0, 0), vec3{0.03, h, w}, xAxis, 0.004)
	b.in("home.rib").Box(p(0.031, 0, 0), vec3{0.003, h - 0.05, w - 0.05}, xAxis, 0)
	iw, ih := w-0.2, h-0.2
	face := 0.034
	art := func(tok string, y, z, hh, ww float64) {
		face += 0.0008
		b.in(tok).Box(p(face, y, z), vec3{0.0016, hh, ww}, xAxis, 0)
	}
	switch seed % 4 {
	case 0: // a sunset over the plain: a dusky sky, the sun, dunes
		art("felt.sand", 0.06, 0, ih-0.12, iw)
		art("fabric.ochre", -0.04, 0, 0.1, iw)
		b.in("lamp.amber").Prism(p(face+0.002, 0.02, 0.1), xAxis, 0.05, 0.05, 0.002, 16, 0)
		art("ceramic", -ih/2+0.08, 0, 0.16, iw)
		art("ceramic.dark", -ih/2+0.03, 0, 0.06, iw)
	case 1: // the Earth, small, in a dark sky
		art("fabric.slate", 0, 0, ih, iw)
		b.in("earth.blue").Prism(p(face+0.002, 0.04, -0.08), xAxis, 0.07, 0.07, 0.002, 20, 0)
		b.in("earth.green").Prism(p(face+0.003, 0.05, -0.1), xAxis, 0.025, 0.025, 0.002, 10, 0)
		art("ceramic", -ih/2+0.04, 0, 0.08, iw)
	case 2: // a mission patch: rings round a red disc
		art("fabric.oat", 0, 0, ih, iw)
		b.in("fabric.slate").Prism(p(face+0.001, 0, 0), xAxis, 0.18, 0.18, 0.002, 32, 0)
		b.in("fabric.ochre").Prism(p(face+0.002, 0, 0), xAxis, 0.15, 0.15, 0.002, 32, 0)
		b.in("ceramic").Prism(p(face+0.003, 0, 0), xAxis, 0.11, 0.11, 0.002, 32, 0)
	default: // a map of the region, its contours in bands
		art("felt.sand", 0, 0, ih, iw)
		for k := range 5 {
			art([]string{"felt.sage", "ceramic.sand", "ceramic"}[k%3], -ih/2+0.05+float64(k)*0.085, 0.04*float64(k%2), 0.03, iw-0.1*float64(k))
		}
	}
}

// curtainH is how high a curtain hangs, from its rail down; a tall one,
// from the top of the bay window down to the lookout's floor.
const curtainH, tallCurtainH = 2.5, 5.1

// BuildCurtain: a sheer oat curtain drawn back, 1 m wide in its folds,
// hanging from a slim rail at the spot's height, against a wall or a
// window at -X.
func BuildCurtain(b *building, at vec3, _ int) { b.curtain(at, curtainH) }

// BuildTallCurtain: the same, 5.1 m long.
func BuildTallCurtain(b *building, at vec3, _ int) { b.curtain(at, tallCurtainH) }

func (b *building) curtain(at vec3, curtainH float64) {
	const n, w, fold = 9, 1.0, 0.07
	b.in("home.trim").Box(at.Add(vec3{0.1, 0, 0}), vec3{0.025, 0.025, w + 0.3}, xAxis, 0)
	for k := range n {
		z0 := -w/2 + float64(k)*w/n
		z1 := z0 + w/n
		x0, x1 := 0.06, 0.06+fold
		if k%2 == 1 {
			x0, x1 = x1, x0
		}
		pts := []vec3{
			at.Add(vec3{x0, -0.02, z0}), at.Add(vec3{x1, -0.02, z1}),
			at.Add(vec3{x1, -curtainH, z1}), at.Add(vec3{x0, -curtainH, z0}),
		}
		b.in("fabric.oat").Slab(pts, vec3{0, 1, 0}.Cross(at.Add(vec3{x1, 0, z1}).Sub(at.Add(vec3{x0, 0, z0}))).Norm(), 0.008)
	}
}

// BuildHangingPlant: a white ceramic bowl hung on three cords from 0.4 m
// above its spot, vines trailing from it.
func BuildHangingPlant(b *building, at vec3, seed int) {
	b.in("home.panel").Prism(at, yUp, 0.1, 0.17, 0.16, 18, 0.02)
	b.in("soil").Prism(at.Add(vec3{0, 0.14, 0}), yUp, 0.155, 0.155, 0.02, 18, 0)
	for k := range 3 {
		a := float64(k) * 2 * math.Pi / 3
		p := at.Add(vec3{math.Cos(a), 0, math.Sin(a)}.Scale(0.16)).Add(vec3{0, 0.16, 0})
		top := at.Add(vec3{0, 0.4, 0})
		d := top.Sub(p)
		b.in("home.trim").Prism(p, d.Norm(), 0.003, 0.003, d.Length(), 4, 0)
	}
	for i := range 9 {
		a := float64(i)*2.4 + 3*grnd(seed, i)
		out := vec3{math.Cos(a), 0, math.Sin(a)}
		from := at.Add(out.Scale(0.13)).Add(vec3{0, 0.15, 0})
		// A vine falls over the rim, leaves close along it.
		l := 0.12 + 0.3*grnd(seed, i+10)
		end := from.Add(out.Scale(0.08)).Sub(vec3{0, l, 0})
		d := end.Sub(from)
		b.in("planter.green").Prism(from, d.Norm(), 0.005, 0.004, d.Length(), 5, 0)
		for j := 0; float64(j)*0.07 < l; j++ {
			r := grnd(seed*7+i, j)
			p := from.Add(d.Scale((float64(j)*0.07 + 0.03*r) / l))
			tok := "planter.green"
			if r < 0.35 {
				tok = "planter.lime"
			}
			a := float64(j)*2.4 + 6*r
			side := vec3{math.Cos(a), 0, math.Sin(a)}.Add(out.Scale(0.7)).Norm()
			leaf(b.in(tok), p, side.Add(vec3{0, 0.3 - 0.9*r, 0}).Norm(), 0.06+0.04*r)
		}
	}
}

// BuildBigPlant: a tall plant in a ceramic pot on the floor.
func BuildBigPlant(b *building, at vec3, seed int) { b.pottedPlant(at, 0.22, 1.1, seed) }
