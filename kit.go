package main

import (
	"image/color"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The buildings' design system (docs/MODS.md): tokens say what a surface
// is made of, kit pieces are shapes built once in code, and blueprints are
// data that put pieces together. A blueprint is described once: drawing,
// collisions and interactions are deduced from it.

// token is a material of the base: color, painted material, shine, glow.
type token struct {
	col      color.RGBA
	mat      int
	scale    float32 // texture tiles per meter
	spec     float32
	emissive float32
}

// Off-whites, steel greys, cyan and yellow/black accents: they stand out
// against the orange ground.
var tokens = map[string]token{
	"habitat.white":  {color.RGBA{228, 225, 216, 255}, mtPanel, 1, 0.35, 0},
	"habitat.white2": {color.RGBA{212, 210, 203, 255}, mtPanel, 1, 0.35, 0},
	"steel":          {color.RGBA{140, 146, 158, 255}, mtMetal, 1.2, 0.6, 0},
	"steel.dark":     {color.RGBA{50, 54, 66, 255}, mtPlain, 1, 0.4, 0},
	"rack.slate":     {color.RGBA{58, 64, 80, 255}, mtPanel, 1, 0.4, 0},
	"hazard":         {colWhite, mtHazard, 2.5, 0.3, 0},
	"neon.cyan":      {accentCyan, mtPlain, 1, 0, 0.8},
	"neon.orange":    {accentOrange, mtPlain, 1, 0, 0.8},
	"neon.blue":      {accentBlue, mtPlain, 1, 0, 0.8},
	"neon.purple":    {accentPurple, mtPlain, 1, 0, 0.8},
	"lamp.warm":      {lampWarm, mtPlain, 1, 0, 1},
	"floor.slate":    {color.RGBA{50, 54, 66, 255}, mtPanel, 1, 0.25, 0},
	"wall.lower":     {color.RGBA{86, 100, 118, 255}, mtPanel, 1, 0.3, 0},
	"floor.inlay":    {color.RGBA{68, 74, 90, 255}, mtPlain, 1, 0.3, 0},
	"solar.blue":     {color.RGBA{40, 62, 130, 255}, mtPlain, 1, 0.9, 0},
	"crate.ochre":    {color.RGBA{196, 132, 60, 255}, mtPanel, 1, 0.2, 0},
	"crate.slate":    {color.RGBA{88, 116, 146, 255}, mtPanel, 1, 0.2, 0},
	"crate.sand":     {color.RGBA{158, 148, 118, 255}, mtPanel, 1, 0.2, 0},
	"wood.brown":     {color.RGBA{110, 80, 60, 255}, mtPlain, 1, 0.3, 0},
	"fabric.white":   {matWhite, mtFabric, 1 / fabricTile, 0.1, 0},
	"fabric.purple":  {accentPurple, mtFabric, 1 / fabricTile, 0.1, 0},
	"fabric.rug":     {color.RGBA{150, 70, 60, 255}, mtFabric, 1 / fabricTile, 0, 0},
	// Greebles and the yard (greeble.go).
	"glass.dark":   {color.RGBA{26, 38, 54, 255}, mtPlain, 1, 1.0, 0},
	"pipe.yellow":  {color.RGBA{214, 164, 52, 255}, mtPlain, 1, 0.5, 0},
	"pipe.blue":    {color.RGBA{62, 112, 168, 255}, mtPlain, 1, 0.5, 0},
	"pipe.red":     {color.RGBA{178, 58, 48, 255}, mtPlain, 1, 0.5, 0},
	"barrel.blue":  {color.RGBA{54, 92, 150, 255}, mtPaint, 1 / paintTile, 0.4, 0},
	"barrel.ochre": {color.RGBA{196, 132, 60, 255}, mtPaint, 1 / paintTile, 0.4, 0},
	"rubber":       {color.RGBA{34, 34, 38, 255}, mtPlain, 1, 0.2, 0},
	"locker":       {color.RGBA{72, 88, 108, 255}, mtPanel, 1, 0.4, 0},
	"locker.ochre": {color.RGBA{176, 124, 60, 255}, mtPanel, 1, 0.4, 0},
	"pad.concrete": {color.RGBA{130, 120, 110, 255}, mtPanel, 1, 0.2, 0},
	"screen.dim":   {accentCyan, mtPlain, 1, 0, 0.4},
	"led.green":    {colGreen, mtPlain, 1, 0, 1},
}

// Blueprint is a building as data.
type Blueprint struct {
	ID     string
	At     vec3 // local, center of the floor
	Shape  Dome
	Style  string // token of the walls and roof
	Accent string // token of the glowing stripes inside
	Doors  []Door
	Props  []Prop
}

type Dome struct{ Radius, WallH float64 }

type Door struct{ Facing float64 } // angle, atan2(z, x)

// Prop is a kit piece placed in a building, relative to its center; Seed
// varies pieces that are built by rules.
type Prop struct {
	Kit  string
	At   vec3
	Seed int
}

// polar is the point at distance r from the center, at angle a.
func polar(r, a float64) vec3 { return vec3{math.Cos(a) * r, 0, math.Sin(a) * r} }

var (
	hubPlan = Blueprint{
		ID:     "core:hub",
		Accent: "neon.cyan",
		Shape:  Dome{Radius: 6.5, WallH: 2.8},
		Style:  "habitat.white",
		Doors:  []Door{{east}, {west}, {north}, {south}},
		Props: []Prop{
			{Kit: "maptable"},
			{Kit: "crate.stack", At: polar(5.4, 0.8), Seed: 1},
			{Kit: "crate.stack", At: polar(5.4, 2.4), Seed: 2},
			{Kit: "crate.stack", At: polar(5.4, 3.9), Seed: 4},
			{Kit: "crate.stack", At: polar(5.4, 5.5), Seed: 5},
		},
	}
	salePlan = Blueprint{
		ID:     "core:sale",
		Accent: "neon.orange",
		At:     vec3{-13, 0, 0},
		Shape:  Dome{Radius: 4, WallH: 2.6},
		Style:  "habitat.white",
		Doors:  []Door{{east}},
		Props: []Prop{
			{Kit: "incinerator", At: vec3{-1.6, 0, 0}},
			{Kit: "crate.stack", At: vec3{1.2, 0, 2.4}, Seed: 3},
			{Kit: "crate.stack", At: vec3{0.6, 0, -2.7}, Seed: 8},
		},
	}
	researchPlan = Blueprint{
		ID:     "core:research",
		Accent: "neon.blue",
		At:     vec3{13, 0, 0},
		Shape:  Dome{Radius: 4, WallH: 2.6},
		Style:  "habitat.white",
		Doors:  []Door{{west}},
		Props:  []Prop{{Kit: "console", At: vec3{2.3, 0, 0}}},
	}
	sleepPlan = Blueprint{
		ID:     "core:sleep",
		Accent: "neon.purple",
		At:     vec3{0, 0, 13},
		Shape:  Dome{Radius: 4, WallH: 2.6},
		Style:  "habitat.white",
		Doors:  []Door{{south}},
		Props:  []Prop{{Kit: "bed", At: vec3{0, 0, 2.1}}},
	}

	// plans in the order of rooms: the hub first.
	plans = []*Blueprint{&hubPlan, &salePlan, &researchPlan, &sleepPlan}
)

func (bp *Blueprint) room() room {
	var openings []float64
	for _, d := range bp.Doors {
		openings = append(openings, d.Facing)
	}
	return room{bp.At, bp.Shape.Radius, bp.Shape.WallH, openings, bp}
}

// kitProp is a piece of furniture: what it adds to the merged meshes, what
// it draws each frame (moving or glowing parts), how the player bumps into
// it and uses it. Positions are local, relative to the prop.
type kitProp struct {
	build     func(b *building, at vec3, seed int)
	draw      func(g *Game, at vec3)
	obstacles []obstacle
	reach     float64
	use       interaction
	prompt    string
}

var kitProps = map[string]kitProp{
	"maptable": {build: buildMapTable, obstacles: []obstacle{{vec3{}, 1.25}},
		reach: 2.6, use: useMap, prompt: "[E] Consulter l'état de Mars"},
	"incinerator": {build: buildIncinerator, draw: drawIncinerator, obstacles: []obstacle{{vec3{}, 1.4}},
		reach: 2.8, use: useIncinerator, prompt: "[E] Déposer la poussière dans l'incinérateur"},
	"console": {build: buildConsole, draw: drawConsole,
		obstacles: []obstacle{{vec3{-0.3, 0, 0}, 1.0}, {vec3{0, 0, 2.3}, 0.6}, {vec3{0, 0, -2.3}, 0.6}},
		reach:     2.8, use: useResearch, prompt: "[E] Ouvrir le laboratoire de recherche"},
	"bed": {build: buildBed, draw: drawBed, obstacles: []obstacle{{vec3{-0.55, 0, 0}, 0.6}, {vec3{0.55, 0, 0}, 0.6}},
		reach: 2.6, use: useBed, prompt: "[E] Dormir"},
	"crate.stack": {build: buildCrates, obstacles: []obstacle{{vec3{}, 0.55}}},
}

// spot returns where the first prop of a kit sits in the base.
func (bp *Blueprint) spot(kit string) vec3 {
	for _, p := range bp.Props {
		if p.Kit == kit {
			return bp.At.add(p.At)
		}
	}
	return bp.At
}

func planObstacles() (out []obstacle) {
	out = yardObstacles()
	for _, bp := range plans {
		for _, p := range bp.Props {
			for _, o := range kitProps[p.Kit].obstacles {
				out = append(out, obstacle{bp.At.add(p.At).add(o.at), o.radius})
			}
		}
	}
	return out
}

func planInteractables() (out []interactable) {
	for _, bp := range plans {
		for _, p := range bp.Props {
			if k := kitProps[p.Kit]; k.use != useNone {
				out = append(out, interactable{bp.At.add(p.At), k.reach, k.use, k.prompt})
			}
		}
	}
	return out
}

// building holds the base's merged meshes: one per token, lighting mode
// and room (the world point that litTowards walls face).
type building struct {
	parts  map[partKey]*meshBuilder
	keys   []partKey
	models []rl.Model
}

type partKey struct {
	tok string
	lit float32
	at  vec3
}

func (b *building) part(tok string, lit float32, at vec3) *meshBuilder {
	k := partKey{tok, lit, at}
	if b.parts[k] == nil {
		b.parts[k] = &meshBuilder{}
		b.keys = append(b.keys, k)
	}
	return b.parts[k]
}

// buildBase builds every static piece of the base.
func buildBase(sh rl.Shader) *building {
	b := &building{parts: map[partKey]*meshBuilder{}}
	for i, bp := range plans {
		b.room(bp)
		b.greebleRoom(bp, bp.room(), i*97+3)
	}
	for i, c := range corridors {
		b.corridor(c)
		b.greebleCorridor(c, i*31+5)
	}
	b.yard()
	for _, p := range junctionPlates() {
		room := toWorld(p.mid.sub(p.normal.scale(2)).add(vec3{0, 1.2, 0}))
		across := vec3{-p.normal.Z, 0, p.normal.X}
		b.part("habitat.white2", litTowards, room).box(p.mid.add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, wallThick * 1.5}, across, 0.04)
	}
	b.airlock()
	b.outside()
	for _, k := range b.keys {
		b.models = append(b.models, b.parts[k].model(sh))
	}
	return b
}

func (b *building) room(bp *Blueprint) {
	r := bp.room()
	in := toWorld(r.center.add(vec3{0, 1.2, 0}))
	// Walls: bevelled panels, a dark plinth, a steel collar under the dome.
	for i, p := range r.panels() {
		tok := bp.Style
		if i%2 == 1 {
			tok += "2"
		}
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		tangent := vec3{-out.Z, 0, out.X}
		b.part(tok, litTowards, in).box(p.mid.add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, wallThick}, tangent, 0.04)
		b.part("steel", litTowards, in).box(p.mid.add(vec3{0, p.y1 - 0.1, 0}).add(out.scale(0.03)), vec3{p.width, 0.2, wallThick + 0.08}, tangent, 0.035)
		if p.y0 == 0 {
			b.part("steel.dark", litTowards, in).box(p.mid.add(vec3{0, 0.15, 0}).add(out.scale(0.03)), vec3{p.width, 0.3, wallThick + 0.08}, tangent, 0.035)
		}
	}
	// The dome stays a smooth shell: dust settles on its top.
	b.part(bp.Style, litBack, in).dome(r.center.add(vec3{0, r.wallH, 0}), r.radius+0.12, r.radius*0.55)
	// Door frames: steel jambs and a hazard header.
	for _, d := range bp.Doors {
		out := vec3{math.Cos(d.Facing), 0, math.Sin(d.Facing)}
		tangent := vec3{-out.Z, 0, out.X}
		w := r.center.add(out.scale(r.radius))
		for _, s := range []float64{-1, 1} {
			b.part("steel", litTowards, in).box(w.add(tangent.scale(s*(doorWidth/2+0.13))).add(vec3{0, (doorHeight + 0.1) / 2, 0}), vec3{0.26, doorHeight + 0.1, 0.42}, tangent, 0.05)
		}
		b.part("hazard", litTowards, in).box(w.add(vec3{0, doorHeight + 0.2, 0}), vec3{doorWidth + 0.52, 0.22, 0.42}, tangent, 0.05)
		b.part(bp.Accent, litIndoor, vec3{}).box(w.add(vec3{0, doorHeight + 0.2, 0}).sub(out.scale(0.22)), vec3{doorWidth * 0.6, 0.05, 0.02}, tangent, 0)
	}
	b.interior(bp, r)
	for _, p := range bp.Props {
		if k := kitProps[p.Kit]; k.build != nil {
			k.build(b, bp.At.add(p.At), p.Seed)
		}
	}
}

// interior: a flat floor with a steel-ringed inlay, a glowing stripe along
// the walls, steel ribs under the dome and a ceiling lamp.
func (b *building) interior(bp *Blueprint, r room) {
	up := vec3{0, 1, 0}
	floor := r.center.add(vec3{0, floorTop - 0.1, 0})
	b.part("floor.slate", litIndoor, vec3{}).prism(floor, up, r.radius, r.radius, 0.1, 48, 0)
	b.part("steel", litIndoor, vec3{}).prism(floor, up, r.radius*0.47, r.radius*0.47, 0.104, 40, 0)
	b.part("floor.inlay", litIndoor, vec3{}).prism(floor, up, r.radius*0.44, r.radius*0.44, 0.106, 40, 0)
	for _, p := range r.panels() {
		if p.y0 > 0 {
			continue
		}
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		along := vec3{-out.Z, 0, out.X}
		// Two-tone walls: a steel-blue wainscot capped by the accent strip,
		// so the light upper walls stand out over a darker base.
		inner := p.mid.sub(out.scale(wallThick/2 + 0.02))
		b.part("wall.lower", litIndoor, vec3{}).box(inner.add(vec3{0, 0.5, 0}), vec3{p.width * 0.99, 0.96, 0.04}, along, 0.012)
		b.part("steel.dark", litIndoor, vec3{}).box(inner.add(vec3{0, 0.07, 0}).sub(out.scale(0.02)), vec3{p.width * 0.99, 0.14, 0.04}, along, 0.01)
		strip := p.mid.sub(out.scale(wallThick/2 + 0.045)).add(vec3{0, 1.0, 0})
		b.part(bp.Accent, litIndoor, vec3{}).box(strip, vec3{p.width * 0.94, 0.07, 0.03}, along, 0.008)
	}
	// Ribs follow the inside of the dome up to the lamp's collar.
	a, bh := r.radius+0.02, r.radius*0.55-0.06
	top := r.center.add(vec3{0, r.wallH, 0})
	const ribs, steps = 8, 7
	for k := range ribs {
		th := (float64(k) + 0.5) * 2 * math.Pi / ribs
		out := vec3{math.Cos(th), 0, math.Sin(th)}
		side := vec3{-out.Z, 0, out.X}
		pt := func(i int) vec3 {
			phi := float64(i) / steps * 1.25
			return top.add(out.scale(a * math.Cos(phi))).add(vec3{0, bh * math.Sin(phi), 0})
		}
		for i := range steps {
			p0, p1 := pt(i), pt(i+1)
			along := p1.sub(p0)
			l := along.length()
			along = along.scale(1 / l)
			b.part("steel", litIndoor, vec3{}).boxBasis(p0.lerp(p1, 0.5).sub(along.cross(side).scale(0.05)), [3]vec3{side, along, side.cross(along)}, vec3{0.12, l + 0.02, 0.1}, 0.02)
		}
	}
	lamp := r.center.add(vec3{0, r.wallH + r.radius*0.5, 0})
	b.part("steel", litIndoor, vec3{}).prism(lamp.add(vec3{0, -0.02, 0}), up, 0.62, 0.55, 0.12, 24, 0.02)
	b.part("lamp.warm", litIndoor, vec3{}).prism(lamp.add(vec3{0, -0.06, 0}), up, 0.48, 0.48, 0.05, 24, 0)
}

// corridor: panels with joints, a dark plinth outside, a roof slab and
// steel ribs at the joints.
func (b *building) corridor(c corridor) {
	mid := c.from.lerp(c.to, 0.5)
	dir := c.to.sub(c.from)
	length := dir.length()
	dir = dir.scale(1 / length)
	side := c.side()
	in := toWorld(mid.add(vec3{0, 1.2, 0}))
	n := int(math.Ceil(length / 1.6))
	seg := length / float64(n)
	for i := range n {
		at := c.from.add(dir.scale((float64(i) + 0.5) * seg))
		tok := "habitat.white"
		if i%2 == 1 {
			tok += "2"
		}
		for _, s := range []float64{-1, 1} {
			wall := at.add(side.scale(s * (c.width/2 + wallThick/2)))
			b.part(tok, litTowards, in).box(wall.add(vec3{0, c.height / 2, 0}), vec3{seg * 1.01, c.height, wallThick}, dir, 0.04)
			b.part("steel.dark", litTowards, in).box(wall.add(side.scale(s*0.03)).add(vec3{0, 0.15, 0}), vec3{seg * 1.01, 0.3, wallThick + 0.08}, dir, 0.035)
		}
	}
	b.part("habitat.white2", litTowards, in).box(mid.add(vec3{0, c.height + 0.06, 0}), vec3{length, 0.12, c.width + 2*wallThick}, dir, 0.04)
	for i := 1; i < n; i++ {
		at := c.from.add(dir.scale(float64(i) * seg))
		for _, s := range []float64{-1, 1} {
			post := at.add(side.scale(s * (c.width/2 + wallThick/2 + 0.04))).add(vec3{0, (c.height + 0.12) / 2, 0})
			b.part("steel", litTowards, in).box(post, vec3{0.14, c.height + 0.12, wallThick + 0.08}, dir, 0.03)
		}
		b.part("steel", litTowards, in).box(at.add(vec3{0, c.height + 0.14, 0}), vec3{0.14, 0.1, c.width + 2*wallThick + 0.16}, dir, 0.03)
	}
	// Inside: two-tone walls like the rooms, a flat floor edged with steel,
	// a lamp strip under the roof.
	for _, s := range []float64{-1, 1} {
		inner := mid.add(side.scale(s * (c.width/2 - 0.02)))
		b.part("wall.lower", litIndoor, vec3{}).box(inner.add(vec3{0, 0.5, 0}), vec3{length, 0.96, 0.04}, dir, 0.012)
		b.part("neon.cyan", litIndoor, vec3{}).box(inner.add(vec3{0, 1.0, 0}).sub(side.scale(s*0.015)), vec3{length * 0.98, 0.06, 0.03}, dir, 0.008)
	}
	b.part("floor.slate", litIndoor, vec3{}).box(mid.add(vec3{0, floorTop - 0.05, 0}), vec3{length, 0.1, c.width}, dir, 0)
	for _, s := range []float64{-1, 1} {
		b.part("steel", litIndoor, vec3{}).box(mid.add(side.scale(s*(c.width/2-0.1))).add(vec3{0, floorTop - 0.045, 0}), vec3{length, 0.1, 0.2}, dir, 0.01)
	}
	b.part("steel", litIndoor, vec3{}).box(mid.add(vec3{0, c.height - 0.03, 0}), vec3{length * 0.72, 0.06, 0.34}, dir, 0.015)
	b.part("lamp.warm", litIndoor, vec3{}).box(mid.add(vec3{0, c.height - 0.07, 0}), vec3{length * 0.7, 0.03, 0.25}, dir, 0)
}

// airlock: the outer frame. The sliding doors move: see drawAirlock.
func (b *building) airlock() {
	end := airlockEnd
	across := vec3{1, 0, 0}
	half := (doorWidth+0.2)/2 + wallThick
	for _, s := range []float64{-1, 1} {
		b.part("hazard", litSun, vec3{}).box(end.add(vec3{s * half, 1.35, 0}), vec3{0.3, 2.7, 0.35}, across, 0.04)
	}
	b.part("steel.dark", litSun, vec3{}).box(end.add(vec3{0, 2.6, 0}), vec3{2*half + 0.3, 0.3, 0.35}, across, 0.05)
}

// outside: an antenna and a dish on the hub, solar panels nearby.
func (b *building) outside() {
	top := hubRoom.wallH + hubRoom.radius*0.55
	up := vec3{0, 1, 0}
	steel := b.part("steel", litSun, vec3{})
	steel.prism(vec3{1.5, top - 0.8, 1.5}, up, 0.05, 0.05, 3.0, 6, 0)
	steel.box(vec3{1.5, top - 0.05, 1.5}, vec3{0.3, 0.2, 0.3}, vec3{1, 0, 0}, 0.04)
	dish := vec3{math.Sin(25 * math.Pi / 180), math.Cos(25 * math.Pi / 180), 0}
	steel.prism(vec3{-1.8, top - 0.7, -1.2}, up, 0.08, 0.08, 0.35, 8, 0)
	b.part("habitat.white", litSun, vec3{}).prism(vec3{-1.8, top - 0.4, -1.2}, dish, 0.15, 0.75, 0.28, 16, 0.03)
	tilt := -30 * math.Pi / 180
	ax := [3]vec3{{1, 0, 0}, {0, math.Cos(tilt), math.Sin(tilt)}, {}}
	ax[2] = ax[0].cross(ax[1])
	for _, z := range []float64{-7.5, -10.5} {
		at := vec3{-12, 0, z}
		steel.prism(at, up, 0.06, 0.06, 1.2, 8, 0)
		panel := at.add(vec3{0, 1.28, 0})
		steel.boxBasis(panel, ax, vec3{2.7, 0.05, 1.6}, 0.02)
		cells := panel.add(ax[1].scale(0.03))
		b.part("solar.blue", litSun, vec3{}).boxBasis(cells, ax, vec3{2.56, 0.03, 1.46}, 0.01)
		for _, x := range []float64{-0.43, 0.43} {
			steel.boxBasis(cells.add(ax[0].scale(x)).add(ax[1].scale(0.012)), ax, vec3{0.03, 0.02, 1.46}, 0)
		}
	}
}

func (b *building) draw(gfx *Graphics) {
	for i, k := range b.keys {
		t := tokens[k.tok]
		gfx.setMat(t.mat, t.scale, false)
		gfx.setLighting(k.lit, k.at)
		rl.SetShaderValue(gfx.objectSh, gfx.oLoc.surface, []float32{t.spec, t.emissive}, rl.ShaderUniformVec2)
		rl.DrawModel(b.models[i], rl.Vector3{}, 1, t.col)
	}
}

// buildCrates stacks two or three bevelled crates, chosen by the seed.
func buildCrates(b *building, at vec3, seed int) {
	cols := []string{"crate.ochre", "crate.slate", "crate.sand"}
	rnd := func(k int) float64 { return math.Mod(math.Abs(math.Sin(float64(seed*7+k)*12.9898))*43758.5453, 1) }
	y := 0.0
	for i := range 2 + seed%2 {
		s := 0.75 - 0.15*float64(i) - 0.1*rnd(i)
		a := rnd(i+10) * math.Pi
		off := vec3{(rnd(i+20) - 0.5) * 0.15, 0, (rnd(i+30) - 0.5) * 0.15}
		b.part(cols[(seed+i)%3], litIndoor, vec3{}).box(at.add(off).add(vec3{0, y + s/2, 0}), vec3{s, s, s}, vec3{math.Cos(a), 0, math.Sin(a)}, 0.05)
		y += s
	}
}

// buildMapTable: a faceted pedestal, a steel top and a glowing disc under
// the hologram (drawGlobe).
func buildMapTable(b *building, t vec3, _ int) {
	up := vec3{0, 1, 0}
	b.part("steel.dark", litIndoor, vec3{}).prism(t, up, 0.85, 0.85, 0.85, 20, 0.03)
	b.part("steel", litIndoor, vec3{}).prism(t.add(vec3{0, 0.85, 0}), up, 1.2, 1.2, 0.08, 24, 0.02)
	b.part("neon.cyan", litIndoor, vec3{}).prism(t.add(vec3{0, 0.93, 0}), up, 1.0, 1.0, 0.02, 24, 0)
}

func buildIncinerator(b *building, in vec3, _ int) {
	up := vec3{0, 1, 0}
	steel, dark := b.part("steel", litIndoor, vec3{}), b.part("steel.dark", litIndoor, vec3{})
	steel.prism(in, up, 1.1, 1.1, 1.9, 20, 0.04)
	steel.prism(in.add(vec3{0, 1.9, 0}), up, 0.12, 0.95, 0.85, 20, 0.03)
	dark.prism(in.add(vec3{0, 0.25, 0}), up, 1.14, 1.14, 0.12, 20, 0.02)
	dark.prism(in.add(vec3{-0.6, 1.5, 0.6}), up, 0.18, 0.18, 3.4, 10, 0.03)
	dark.box(in.add(vec3{1.0, 1.0, 1.25}), vec3{0.5, 0.7, 0.3}, vec3{0, 0, 1}, 0.04)
	b.part("hazard", litIndoor, vec3{}).prism(in.add(vec3{0, 1.6, 0}), up, 1.14, 1.14, 0.14, 20, 0.02)
}

// drawIncinerator: the window glows harder while it burns dust.
func drawIncinerator(g *Game, in vec3) {
	gfx := g.gfx
	gfx.setLighting(litIndoor, vec3{})
	gfx.setMat(mtPlain, 1, false)
	glow := float32(0.55 + 0.45*clamp01(g.burn))
	gfx.box(in.add(vec3{1.08, 0.95, 0}), vec3{0.8, 0.55, 0.06}, vec3{0, 0, 1}, accentOrange, 0, glow)
	gfx.drawScreen(in.add(vec3{1.16, 1.1, 1.25}), vec3{1, 0, 0}, 0.38, 0.28, gfx.screens[3], 0.8)
}

// buildConsole: a desk between two server racks.
func buildConsole(b *building, c vec3, _ int) {
	b.part("steel", litIndoor, vec3{}).box(c.add(vec3{-0.2, 0.78, 0}), vec3{0.9, 0.06, 2.2}, vec3{1, 0, 0}, 0.02)
	for _, z := range []float64{-0.95, 0.95} {
		b.part("steel.dark", litIndoor, vec3{}).box(c.add(vec3{-0.2, 0.38, z}), vec3{0.8, 0.76, 0.08}, vec3{1, 0, 0}, 0.02)
	}
	for _, z := range []float64{-2.3, 2.3} {
		b.part("rack.slate", litIndoor, vec3{}).box(c.add(vec3{0, 0.95, z}), vec3{0.7, 1.9, 0.7}, vec3{1, 0, 0}, 0.05)
	}
}

// drawConsole: three screens and the blinking lights of the racks.
func drawConsole(g *Game, c vec3) {
	gfx := g.gfx
	gfx.setLighting(litIndoor, vec3{})
	gfx.setMat(mtPlain, 1, false)
	for i, z := range []float64{-0.75, 0, 0.75} {
		facing := vec3{-1, 0, z * 0.4}.norm()
		gfx.drawScreen(c.add(vec3{0.1, 1.28, z}), facing, 0.66, 0.42, gfx.screens[i], 0.8)
	}
	gfx.setLighting(litIndoor, vec3{})
	gfx.setMat(mtPlain, 1, false)
	for _, z := range []float64{-2.3, 2.3} {
		for k := range 5 {
			on := float32(0.3)
			if math.Sin(g.clock*3+float64(k)*1.7+z) > 0 {
				on = 1
			}
			gfx.box(c.add(vec3{-0.36, 0.4 + float64(k)*0.3, z}), vec3{0.5, 0.04, 0.02}, vec3{0, 0, 1}, colGreen, 0, on)
		}
	}
}

// buildBed: a bed against the far wall, a bedside table and a rug.
func buildBed(b *building, at vec3, _ int) {
	x := vec3{1, 0, 0}
	wood := b.part("wood.brown", litIndoor, vec3{})
	wood.box(at.add(vec3{0, 0.2, 0}), vec3{2.1, 0.4, 1.05}, x, 0.04)
	wood.box(at.add(vec3{1.45, 0.3, 0.2}), vec3{0.5, 0.6, 0.5}, x, 0.04)
	white := b.part("fabric.white", litIndoor, vec3{})
	white.box(at.add(vec3{0, 0.48, 0}), vec3{2.0, 0.16, 0.98}, x, 0.05)
	white.box(at.add(vec3{-0.72, 0.62, 0}), vec3{0.42, 0.12, 0.7}, x, 0.05)
	b.part("fabric.purple", litIndoor, vec3{}).box(at.add(vec3{0.32, 0.58, 0}), vec3{1.36, 0.06, 1.0}, x, 0.02)
	b.part("fabric.rug", litIndoor, vec3{}).box(at.add(vec3{0, floorTop + 0.005, -1.3}), vec3{1.8, 0.01, 1.1}, x, 0)
}

func drawBed(g *Game, at vec3) {
	gfx := g.gfx
	gfx.setLighting(litIndoor, vec3{})
	gfx.setMat(mtPlain, 1, false)
	gfx.put(gfx.sphere, toWorld(at.add(vec3{1.45, 0.75, 0.2})), vec3{0.13, 0.13, 0.13}, vec3{0, 1, 0}, 0, lampWarm, 0, 1)
}

// meshBuilder collects triangles, in world coordinates unless local.
type meshBuilder struct {
	v, n, uv []float32
	local    bool
	// uvf lays the plate texture (mtPanel) on the surface being built: the
	// texture coordinates of a local point. Without it, the middle of a plate.
	uvf func(p vec3) [2]float64
}

func (m *meshBuilder) emit(p, nrm vec3) {
	t := [2]float64{0.5, 0.5}
	if m.uvf != nil {
		t = m.uvf(p)
	}
	if !m.local {
		p = toWorld(p)
	}
	m.v = append(m.v, float32(p.X), float32(p.Y), float32(p.Z))
	m.n = append(m.n, float32(nrm.X), float32(nrm.Y), float32(nrm.Z))
	m.uv = append(m.uv, float32(t[0]), float32(t[1]))
}

// unwrap brings angle a within half a turn of ref.
func unwrap(a, ref float64) float64 {
	for a < ref-math.Pi {
		a += 2 * math.Pi
	}
	for a > ref+math.Pi {
		a -= 2 * math.Pi
	}
	return a
}

// tri adds a flat triangle facing away from center.
func (m *meshBuilder) tri(a, b, c, center vec3) {
	nrm := b.sub(a).cross(c.sub(a))
	if nrm.dot(a.add(b).add(c).scale(1.0/3).sub(center)) < 0 {
		b, c = c, b
		nrm = nrm.scale(-1)
	}
	nrm = nrm.norm()
	m.emit(a, nrm)
	m.emit(b, nrm)
	m.emit(c, nrm)
}

// smooth adds a triangle with its own vertex normals, wound to face them.
func (m *meshBuilder) smooth(a, b, c, na, nb, nc vec3) {
	if b.sub(a).cross(c.sub(a)).dot(na.add(nb).add(nc)) < 0 {
		b, c, nb, nc = c, b, nc, nb
	}
	m.emit(a, na)
	m.emit(b, nb)
	m.emit(c, nc)
}

func (m *meshBuilder) quad(a, b, c, d, center vec3) {
	m.tri(a, b, c, center)
	m.tri(a, c, d, center)
}

// box adds a box centered at the local point c, its width (X) along the
// horizontal direction across.
func (m *meshBuilder) box(c, size, across vec3, bevel float64) {
	ex := across.norm()
	m.boxBasis(c, [3]vec3{ex, {0, 1, 0}, ex.cross(vec3{0, 1, 0})}, size, bevel)
}

// boxBasis adds a box along the unit axes ax, with edges and corners
// chamfered by bevel: they catch the light instead of being sharp.
func (m *meshBuilder) boxBasis(c vec3, ax [3]vec3, size vec3, bevel float64) {
	h := [3]float64{size.X / 2, size.Y / 2, size.Z / 2}
	bevel = math.Min(bevel, 0.45*math.Min(h[0], math.Min(h[1], h[2])))
	// pt is the corner s pushed out along one axis: the corner of that face.
	pt := func(axis int, s [3]float64) vec3 {
		p := c
		for k := range 3 {
			q := s[k] * (h[k] - bevel)
			if k == axis {
				q = s[k] * h[k]
			}
			p = p.add(ax[k].scale(q))
		}
		return p
	}
	signs := [2]float64{-1, 1}
	for a := range 3 {
		u, v := (a+1)%3, (a+2)%3
		for _, sa := range signs {
			var q [4]vec3
			for i, uv := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
				var s [3]float64
				s[a], s[u], s[v] = sa, uv[0], uv[1]
				q[i] = pt(a, s)
			}
			// Plates about 1.2 m wide and 1 m tall fill the face exactly, the
			// texture going down its most vertical axis.
			U, V := u, v
			if math.Abs(ax[u].Y) > math.Abs(ax[v].Y) {
				U, V = v, u
			}
			down := 1.0
			if ax[V].Y < 0 {
				down = -1
			}
			nu, nv := math.Max(1, math.Round(2*h[U]/1.2)), math.Max(1, math.Round(2*h[V]/1.0))
			m.uvf = func(p vec3) [2]float64 {
				d := p.sub(c)
				return [2]float64{(d.dot(ax[U]) + h[U]) / (2 * h[U]) * nu, (h[V] - down*d.dot(ax[V])) / (2 * h[V]) * nv}
			}
			m.quad(q[0], q[1], q[2], q[3], c)
			m.uvf = nil
			if bevel == 0 {
				continue
			}
			// The bevel along axis v between faces a and u.
			for _, su := range signs {
				var s0, s1 [3]float64
				s0[a], s0[u], s0[v] = sa, su, -1
				s1[a], s1[u], s1[v] = sa, su, 1
				m.quad(pt(a, s0), pt(a, s1), pt(u, s1), pt(u, s0), c)
			}
		}
	}
	if bevel == 0 {
		return
	}
	for _, sx := range signs {
		for _, sy := range signs {
			for _, sz := range signs {
				s := [3]float64{sx, sy, sz}
				m.tri(pt(0, s), pt(1, s), pt(2, s), c)
			}
		}
	}
}

// prism adds a faceted cylinder or cone frustum standing on base along
// axis: radius r0 at the bottom, r1 at the top, its top edge chamfered.
func (m *meshBuilder) prism(base, axis vec3, r0, r1, h float64, sides int, bevel float64) {
	axis = axis.norm()
	u, w := tangentBasis(axis)
	center := base.add(axis.scale(h / 2))
	type level struct{ y, r float64 }
	levels := []level{{0, r0}, {h, r1}}
	if bevel > 0 {
		levels = []level{{0, r0}, {h - bevel, r1}, {h, r1 - bevel}}
	}
	ring := func(l level, k int) vec3 {
		a := 2 * math.Pi * float64(k) / float64(sides)
		return base.add(axis.scale(l.y)).add(u.scale(l.r * math.Cos(a))).add(w.scale(l.r * math.Sin(a)))
	}
	// Plates around the sides, square tiles on the caps.
	nu, nv := math.Max(1, math.Round(2*math.Pi*math.Max(r0, r1)/1.2)), math.Max(1, math.Round(h/1.0))
	for i := 0; i+1 < len(levels); i++ {
		for k := range sides {
			ac := 2 * math.Pi * (float64(k) + 0.5) / float64(sides)
			m.uvf = func(p vec3) [2]float64 {
				d := p.sub(base)
				a := unwrap(math.Atan2(d.dot(w), d.dot(u)), ac)
				return [2]float64{a / (2 * math.Pi) * nu, (h - d.dot(axis)) / h * nv}
			}
			m.quad(ring(levels[i], k), ring(levels[i], k+1), ring(levels[i+1], k+1), ring(levels[i+1], k), center)
		}
	}
	m.uvf = func(p vec3) [2]float64 {
		d := p.sub(base)
		return [2]float64{d.dot(u)/1.2 + 0.5, d.dot(w)/1.2 + 0.5}
	}
	top := levels[len(levels)-1]
	for k := range sides {
		m.tri(base, ring(levels[0], k), ring(levels[0], k+1), center)
		m.tri(base.add(axis.scale(h)), ring(top, k), ring(top, k+1), center)
	}
	m.uvf = nil
}

// dome adds a smooth half ellipsoid on c: radius a, height bh, no floor.
func (m *meshBuilder) dome(c vec3, a, bh float64) {
	const rings, segs = 12, 40
	pt := func(i, j int) (vec3, vec3) {
		phi := float64(i) / rings * math.Pi / 2
		th := float64(j) / segs * 2 * math.Pi
		cp, sp := math.Cos(phi), math.Sin(phi)
		p := c.add(vec3{a * cp * math.Cos(th), bh * sp, a * cp * math.Sin(th)})
		return p, vec3{cp * math.Cos(th) / a, sp / bh, cp * math.Sin(th) / a}.norm()
	}
	// Plates in rows of four down from the top, about 1.6 m wide.
	n := math.Max(4, math.Round(2*math.Pi*a/1.6))
	for i := range rings {
		for j := range segs {
			thc := (float64(j) + 0.5) / segs * 2 * math.Pi
			m.uvf = func(p vec3) [2]float64 {
				d := p.sub(c)
				th := unwrap(math.Atan2(d.Z, d.X), thc)
				phi := math.Asin(math.Max(-1, math.Min(1, d.Y/bh)))
				return [2]float64{th / (2 * math.Pi) * n, (1 - phi/(math.Pi/2)) * 4}
			}
			p00, n00 := pt(i, j)
			p01, n01 := pt(i, j+1)
			p10, n10 := pt(i+1, j)
			p11, n11 := pt(i+1, j+1)
			m.smooth(p00, p01, p11, n00, n01, n11)
			if i+1 < rings {
				m.smooth(p00, p11, p10, n00, n11, n10)
			}
		}
	}
	m.uvf = nil
}

// model uploads the triangles. The slices stay referenced by the builder:
// the mesh is never unloaded.
func (m *meshBuilder) model(sh rl.Shader) rl.Model {
	mesh := rl.Mesh{VertexCount: int32(len(m.v) / 3), TriangleCount: int32(len(m.v) / 9)}
	mesh.Vertices, mesh.Normals, mesh.Texcoords = &m.v[0], &m.n[0], &m.uv[0]
	rl.UploadMesh(&mesh, false)
	mod := rl.LoadModelFromMesh(mesh)
	mod.GetMaterials()[0].Shader = sh
	return mod
}

// doorPanelModel is one sliding leaf of the airlock, built around its
// center so it can move.
func doorPanelModel(sh rl.Shader) rl.Model {
	m := &meshBuilder{local: true}
	m.box(vec3{}, vec3{(doorWidth + 0.2) / 2, doorHeight, 0.08}, vec3{1, 0, 0}, 0.03)
	keep = append(keep, m)
	return m.model(sh)
}

// keep holds builders whose slices back GPU meshes.
var keep []*meshBuilder
