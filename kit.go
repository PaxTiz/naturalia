package main

import (
	"image/color"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/mesh"
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

// tok is a material by ID; an unknown one shows up magenta.
func tok(id string) token {
	if t, ok := reg.Tokens.Get(id); ok {
		return *t
	}
	return token{color.RGBA{255, 0, 255, 255}, mtPlain, 1, 0, 0.5}
}

// Blueprint is a building as data.
type Blueprint struct {
	ID     string `json:"id"`
	At     vec3   `json:"at"`   // local, center of the floor
	Link   string `json:"link"` // building a corridor leads to; doors are added at both ends
	Sign   string `json:"sign"` // painted over the door that leads here from the hub
	Shape  Dome   `json:"shape"`
	Style  string `json:"style"`  // token of the walls and roof
	Accent string `json:"accent"` // token of the glowing stripes inside
	Doors  []Door `json:"doors"`
	Props  []Prop `json:"props"`
}

type Dome struct {
	Radius float64 `json:"radius"`
	WallH  float64 `json:"wall_height"`
}

type Door struct {
	Facing float64 `json:"facing"` // angle, atan2(z, x)
}

// Prop is a kit piece placed in a building, relative to its center; Seed
// varies pieces that are built by rules.
type Prop struct {
	Kit  string `json:"kit"`
	At   vec3   `json:"at"`
	Seed int    `json:"seed"`
}

// polar is the point at distance r from the center, at angle a.
func polar(r, a float64) vec3 { return vec3{math.Cos(a) * r, 0, math.Sin(a) * r} }

// roomOf is the room a blueprint lays out.
func roomOf(bp *Blueprint) room {
	var openings []float64
	for _, d := range bp.Doors {
		openings = append(openings, d.Facing)
	}
	return room{bp.At, bp.Shape.Radius, bp.Shape.WallH, openings, bp}
}

// kitProp is a piece of furniture: what it adds to the merged meshes, how
// the player bumps into it and uses it. Positions are local, relative to
// the prop. What moves or glows is drawn each frame by kitDraws.
type kitProp struct {
	build     func(b *mesh.Parts, at vec3, seed int)
	obstacles []obstacle
	reach     float64
	use       func(g gameAPI, at vec3) // what E does; at is the prop's local position
	prompt    string
}

// kitBuild lets a kit piece use the base's building helpers.
func kitBuild(f func(b *building, at vec3, seed int)) func(*mesh.Parts, vec3, int) {
	return func(p *mesh.Parts, at vec3, seed int) { f(&building{Parts: p}, at, seed) }
}

// spot returns where the first prop of a kit sits in the base.
func (bp *Blueprint) spot(kit string) vec3 {
	for _, p := range bp.Props {
		if p.Kit == kit {
			return bp.At.Add(p.At)
		}
	}
	return bp.At
}

func planObstacles() (out []obstacle) {
	out = yardObstacles()
	for _, bp := range plans {
		for _, p := range bp.Props {
			k, _ := reg.Kits.Get(p.Kit)
			if k == nil {
				continue
			}
			for _, o := range k.obstacles {
				out = append(out, obstacle{bp.At.Add(p.At).Add(o.at), o.radius})
			}
		}
	}
	return out
}

func planInteractables() (out []interactable) {
	for _, bp := range plans {
		for _, p := range bp.Props {
			if k, _ := reg.Kits.Get(p.Kit); k != nil && k.use != nil {
				use := k.use
				out = append(out, interactable{qualify(p.Kit), bp.At.Add(p.At), k.reach, func(g *Game, at vec3) { use(g, at) }, k.prompt})
			}
		}
	}
	return out
}

// building holds the base's merged meshes, uploaded: one model per piece.
type building struct {
	*mesh.Parts
	models []rl.Model
	toks   []token // of each model, looked up once
}

// buildBase builds every static piece of the base.
func buildBase(sh rl.Shader) *building {
	b := &building{Parts: mesh.NewParts(basePos)}
	for i, bp := range plans {
		b.room(bp)
		b.greebleRoom(bp, roomOf(bp), i*97+3)
		b.clutter(bp)
	}
	for i, c := range corridors {
		b.corridor(c)
		b.greebleCorridor(c, i*31+5)
	}
	b.yard()
	for _, p := range junctionPlates() {
		room := toWorld(p.mid.Sub(p.normal.Scale(2)).Add(vec3{0, 1.2, 0}))
		across := vec3{-p.normal.Z, 0, p.normal.X}
		b.Part("habitat.white2", mesh.LitTowards, room).Box(p.mid.Add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, wallThick * 1.5}, across, 0.04)
	}
	b.airlock()
	b.outside()
	for _, k := range b.Keys {
		b.models = append(b.models, upload(b.Mesh(k), sh))
		b.toks = append(b.toks, tok(k.Tok))
	}
	return b
}

func (b *building) room(bp *Blueprint) {
	r := roomOf(bp)
	in := toWorld(r.center.Add(vec3{0, 1.2, 0}))
	// Walls: bevelled panels, a dark plinth, a steel collar under the dome.
	for i, p := range r.panels() {
		tok := bp.Style
		if i%2 == 1 {
			tok += "2"
		}
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		tangent := vec3{-out.Z, 0, out.X}
		b.Part(tok, mesh.LitTowards, in).Box(p.mid.Add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, wallThick}, tangent, 0.04)
		b.Part("steel", mesh.LitTowards, in).Box(p.mid.Add(vec3{0, p.y1 - 0.1, 0}).Add(out.Scale(0.03)), vec3{p.width, 0.2, wallThick + 0.08}, tangent, 0.035)
		if p.y0 == 0 {
			b.Part("steel.dark", mesh.LitTowards, in).Box(p.mid.Add(vec3{0, 0.15, 0}).Add(out.Scale(0.03)), vec3{p.width, 0.3, wallThick + 0.08}, tangent, 0.035)
		}
	}
	// The dome stays a smooth shell: dust settles on its top.
	b.Part(bp.Style, mesh.LitBack, in).Dome(r.center.Add(vec3{0, r.wallH, 0}), r.radius+0.12, r.radius*0.55)
	// Door frames: steel jambs and a hazard header. They stand a little
	// into the opening: flush with the corridor's walls and ceiling, they
	// would fight them for the depth and flicker.
	for _, d := range bp.Doors {
		out := vec3{math.Cos(d.Facing), 0, math.Sin(d.Facing)}
		tangent := vec3{-out.Z, 0, out.X}
		w := r.center.Add(out.Scale(r.radius))
		for _, s := range []float64{-1, 1} {
			b.Part("steel", mesh.LitTowards, in).Box(w.Add(tangent.Scale(s*(doorWidth/2+0.08))).Add(vec3{0, (doorHeight + 0.1) / 2, 0}), vec3{0.26, doorHeight + 0.1, 0.42}, tangent, 0.05)
		}
		b.Part("hazard", mesh.LitTowards, in).Box(w.Add(vec3{0, doorHeight + 0.12, 0}), vec3{doorWidth + 0.42, 0.22, 0.42}, tangent, 0.05)
		b.Part(bp.Accent, mesh.LitIndoor, vec3{}).Box(w.Add(vec3{0, doorHeight + 0.12, 0}).Sub(out.Scale(0.22)), vec3{doorWidth * 0.6, 0.05, 0.02}, tangent, 0)
	}
	b.interior(bp, r)
	for _, p := range bp.Props {
		if k, _ := reg.Kits.Get(p.Kit); k != nil && k.build != nil {
			k.build(b.Parts, bp.At.Add(p.At), p.Seed)
		}
	}
}

// interior: a flat floor with a steel-ringed inlay, a glowing stripe along
// the walls, steel ribs under the dome and a ceiling lamp.
func (b *building) interior(bp *Blueprint, r room) {
	up := vec3{0, 1, 0}
	floor := r.center.Add(vec3{0, floorTop - 0.1, 0})
	b.Part("floor.slate", mesh.LitIndoor, vec3{}).Prism(floor, up, r.radius, r.radius, 0.1, 48, 0)
	b.Part("steel", mesh.LitIndoor, vec3{}).Prism(floor, up, r.radius*0.47, r.radius*0.47, 0.104, 40, 0)
	b.Part("floor.inlay", mesh.LitIndoor, vec3{}).Prism(floor, up, r.radius*0.44, r.radius*0.44, 0.106, 40, 0)
	for _, p := range r.panels() {
		if p.y0 > 0 {
			continue
		}
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		along := vec3{-out.Z, 0, out.X}
		// Two-tone walls: a steel-blue wainscot capped by the accent strip,
		// so the light upper walls stand out over a darker base.
		inner := p.mid.Sub(out.Scale(wallThick/2 + 0.02))
		b.Part("wall.lower", mesh.LitIndoor, vec3{}).Box(inner.Add(vec3{0, 0.5, 0}), vec3{p.width * 0.99, 0.96, 0.04}, along, 0.012)
		b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(inner.Add(vec3{0, 0.07, 0}).Sub(out.Scale(0.02)), vec3{p.width * 0.99, 0.14, 0.04}, along, 0.01)
		strip := p.mid.Sub(out.Scale(wallThick/2 + 0.045)).Add(vec3{0, 1.0, 0})
		b.Part(bp.Accent, mesh.LitIndoor, vec3{}).Box(strip, vec3{p.width * 0.94, 0.07, 0.03}, along, 0.008)
	}
	// Ribs follow the inside of the dome up to the lamp's collar.
	a, bh := r.radius+0.02, r.radius*0.55-0.06
	top := r.center.Add(vec3{0, r.wallH, 0})
	const ribs, steps = 8, 7
	for k := range ribs {
		th := (float64(k) + 0.5) * 2 * math.Pi / ribs
		out := vec3{math.Cos(th), 0, math.Sin(th)}
		side := vec3{-out.Z, 0, out.X}
		pt := func(i int) vec3 {
			phi := float64(i) / steps * 1.25
			return top.Add(out.Scale(a * math.Cos(phi))).Add(vec3{0, bh * math.Sin(phi), 0})
		}
		for i := range steps {
			p0, p1 := pt(i), pt(i+1)
			along := p1.Sub(p0)
			l := along.Length()
			along = along.Scale(1 / l)
			b.Part("steel", mesh.LitIndoor, vec3{}).BoxBasis(p0.Lerp(p1, 0.5).Sub(along.Cross(side).Scale(0.05)), [3]vec3{side, along, side.Cross(along)}, vec3{0.12, l + 0.02, 0.1}, 0.02)
		}
	}
	lamp := r.center.Add(vec3{0, r.wallH + r.radius*0.5, 0})
	b.Part("steel", mesh.LitIndoor, vec3{}).Prism(lamp.Add(vec3{0, -0.02, 0}), up, 0.62, 0.55, 0.12, 24, 0.02)
	b.Part("lamp.warm", mesh.LitIndoor, vec3{}).Prism(lamp.Add(vec3{0, -0.06, 0}), up, 0.48, 0.48, 0.05, 24, 0)
}

// corridor: panels with joints, a dark plinth outside, a roof slab and
// steel ribs at the joints.
func (b *building) corridor(c corridor) {
	mid := c.from.Lerp(c.to, 0.5)
	dir := c.to.Sub(c.from)
	length := dir.Length()
	dir = dir.Scale(1 / length)
	side := c.side()
	in := toWorld(mid.Add(vec3{0, 1.2, 0}))
	n := int(math.Ceil(length / 1.6))
	seg := length / float64(n)
	for i := range n {
		at := c.from.Add(dir.Scale((float64(i) + 0.5) * seg))
		tok := "habitat.white"
		if i%2 == 1 {
			tok += "2"
		}
		for _, s := range []float64{-1, 1} {
			wall := at.Add(side.Scale(s * (c.width/2 + wallThick/2)))
			b.Part(tok, mesh.LitTowards, in).Box(wall.Add(vec3{0, c.height / 2, 0}), vec3{seg * 1.01, c.height, wallThick}, dir, 0.04)
			b.Part("steel.dark", mesh.LitTowards, in).Box(wall.Add(side.Scale(s*0.03)).Add(vec3{0, 0.15, 0}), vec3{seg * 1.01, 0.3, wallThick + 0.08}, dir, 0.035)
		}
	}
	b.Part("habitat.white2", mesh.LitTowards, in).Box(mid.Add(vec3{0, c.height + 0.06, 0}), vec3{length, 0.12, c.width + 2*wallThick}, dir, 0.04)
	for i := 1; i < n; i++ {
		at := c.from.Add(dir.Scale(float64(i) * seg))
		for _, s := range []float64{-1, 1} {
			post := at.Add(side.Scale(s * (c.width/2 + wallThick/2 + 0.04))).Add(vec3{0, (c.height + 0.12) / 2, 0})
			b.Part("steel", mesh.LitTowards, in).Box(post, vec3{0.14, c.height + 0.12, wallThick + 0.08}, dir, 0.03)
		}
		b.Part("steel", mesh.LitTowards, in).Box(at.Add(vec3{0, c.height + 0.14, 0}), vec3{0.14, 0.1, c.width + 2*wallThick + 0.16}, dir, 0.03)
	}
	// Inside: two-tone walls like the rooms, a flat floor edged with steel,
	// a lamp strip under the roof.
	for _, s := range []float64{-1, 1} {
		inner := mid.Add(side.Scale(s * (c.width/2 - 0.02)))
		b.Part("wall.lower", mesh.LitIndoor, vec3{}).Box(inner.Add(vec3{0, 0.5, 0}), vec3{length, 0.96, 0.04}, dir, 0.012)
		b.Part("neon.cyan", mesh.LitIndoor, vec3{}).Box(inner.Add(vec3{0, 1.0, 0}).Sub(side.Scale(s*0.015)), vec3{length * 0.98, 0.06, 0.03}, dir, 0.008)
	}
	b.Part("floor.slate", mesh.LitIndoor, vec3{}).Box(mid.Add(vec3{0, floorTop - 0.05, 0}), vec3{length, 0.1, c.width}, dir, 0)
	for _, s := range []float64{-1, 1} {
		b.Part("steel", mesh.LitIndoor, vec3{}).Box(mid.Add(side.Scale(s*(c.width/2-0.1))).Add(vec3{0, floorTop - 0.045, 0}), vec3{length, 0.1, 0.2}, dir, 0.01)
	}
	b.Part("steel", mesh.LitIndoor, vec3{}).Box(mid.Add(vec3{0, c.height - 0.03, 0}), vec3{length * 0.72, 0.06, 0.34}, dir, 0.015)
	b.Part("lamp.warm", mesh.LitIndoor, vec3{}).Box(mid.Add(vec3{0, c.height - 0.07, 0}), vec3{length * 0.7, 0.03, 0.25}, dir, 0)
}

// airlock: the outer frame. The sliding doors move: see drawAirlock.
func (b *building) airlock() {
	end := airlockEnd
	across := vec3{1, 0, 0}
	half := (doorWidth+0.2)/2 + wallThick
	for _, s := range []float64{-1, 1} {
		b.Part("hazard", mesh.LitSun, vec3{}).Box(end.Add(vec3{s * half, 1.35, 0}), vec3{0.3, 2.7, 0.35}, across, 0.04)
	}
	b.Part("steel.dark", mesh.LitSun, vec3{}).Box(end.Add(vec3{0, 2.6, 0}), vec3{2*half + 0.3, 0.3, 0.35}, across, 0.05)
}

// outside: an antenna and a dish on the hub, solar panels nearby.
func (b *building) outside() {
	top := hubRoom.wallH + hubRoom.radius*0.55
	up := vec3{0, 1, 0}
	steel := b.Part("steel", mesh.LitSun, vec3{})
	steel.Prism(vec3{1.5, top - 0.8, 1.5}, up, 0.05, 0.05, 3.0, 6, 0)
	steel.Box(vec3{1.5, top - 0.05, 1.5}, vec3{0.3, 0.2, 0.3}, vec3{1, 0, 0}, 0.04)
	dish := vec3{math.Sin(25 * math.Pi / 180), math.Cos(25 * math.Pi / 180), 0}
	steel.Prism(vec3{-1.8, top - 0.7, -1.2}, up, 0.08, 0.08, 0.35, 8, 0)
	b.Part("habitat.white", mesh.LitSun, vec3{}).Prism(vec3{-1.8, top - 0.4, -1.2}, dish, 0.15, 0.75, 0.28, 16, 0.03)
	tilt := -30 * math.Pi / 180
	ax := [3]vec3{{1, 0, 0}, {0, math.Cos(tilt), math.Sin(tilt)}, {}}
	ax[2] = ax[0].Cross(ax[1])
	for _, z := range []float64{-7.5, -10.5} {
		at := vec3{-12, 0, z}
		steel.Prism(at, up, 0.06, 0.06, 1.2, 8, 0)
		panel := at.Add(vec3{0, 1.28, 0})
		steel.BoxBasis(panel, ax, vec3{2.7, 0.05, 1.6}, 0.02)
		cells := panel.Add(ax[1].Scale(0.03))
		b.Part("solar.blue", mesh.LitSun, vec3{}).BoxBasis(cells, ax, vec3{2.56, 0.03, 1.46}, 0.01)
		for _, x := range []float64{-0.43, 0.43} {
			steel.BoxBasis(cells.Add(ax[0].Scale(x)).Add(ax[1].Scale(0.012)), ax, vec3{0.03, 0.02, 1.46}, 0)
		}
	}
}

func (b *building) draw(gfx *Graphics) {
	for i, k := range b.Keys {
		t := b.toks[i]
		gfx.setMat(t.mat, t.scale, false)
		gfx.setLighting(k.Lit, k.At)
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
		ax := vec3{math.Cos(a), 0, math.Sin(a)}
		sd := vec3{-ax.Z, 0, ax.X}
		c := at.Add(off).Add(vec3{0, y + s/2, 0})
		b.Part(cols[(seed+i)%3], mesh.LitIndoor, vec3{}).Box(c, vec3{s * 0.94, s * 0.94, s * 0.94}, ax, 0.03)
		// Steel corner posts and rims, a strap around, a label.
		dark := b.Part("steel.dark", mesh.LitIndoor, vec3{})
		for _, sx := range []float64{-1, 1} {
			for _, sz := range []float64{-1, 1} {
				dark.Box(c.Add(ax.Scale(sx*s*0.47)).Add(sd.Scale(sz*s*0.47)), vec3{0.07, s, 0.07}, ax, 0.015)
			}
		}
		for _, sy := range []float64{-1, 1} {
			for _, d := range []vec3{ax, sd} {
				o := vec3{-d.Z, 0, d.X}
				for _, sz := range []float64{-1, 1} {
					dark.Box(c.Add(vec3{0, sy * s * 0.47, 0}).Add(o.Scale(sz*s*0.47)), vec3{s, 0.06, 0.06}, d, 0.012)
				}
			}
		}
		b.Part("rubber", mesh.LitIndoor, vec3{}).Box(c, vec3{0.08, s * 0.95, s * 0.95}, ax, 0.01)
		b.Part("habitat.white", mesh.LitIndoor, vec3{}).Box(c.Add(sd.Scale(s*0.475)).Add(ax.Scale(s*0.2)).Add(vec3{0, s * 0.1, 0}), vec3{s * 0.3, s * 0.2, 0.01}, ax, 0)
		y += s
	}
}

// buildMapTable: a faceted pedestal, a steel top and a glowing disc under
// the hologram (drawGlobe).
func buildMapTable(b *building, t vec3, _ int) {
	up := vec3{0, 1, 0}
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Prism(t, up, 0.85, 0.85, 0.85, 20, 0.03)
	b.Part("steel", mesh.LitIndoor, vec3{}).Prism(t.Add(vec3{0, 0.85, 0}), up, 1.2, 1.2, 0.08, 24, 0.02)
	b.Part("neon.cyan", mesh.LitIndoor, vec3{}).Prism(t.Add(vec3{0, 0.93, 0}), up, 1.0, 1.0, 0.02, 24, 0)
}

func buildIncinerator(b *building, in vec3, _ int) {
	up := vec3{0, 1, 0}
	steel, dark := b.Part("steel", mesh.LitIndoor, vec3{}), b.Part("steel.dark", mesh.LitIndoor, vec3{})
	steel.Prism(in, up, 1.1, 1.1, 1.9, 20, 0.04)
	steel.Prism(in.Add(vec3{0, 1.9, 0}), up, 0.12, 0.95, 0.85, 20, 0.03)
	dark.Prism(in.Add(vec3{0, 0.25, 0}), up, 1.14, 1.14, 0.12, 20, 0.02)
	dark.Prism(in.Add(vec3{-0.6, 1.5, 0.6}), up, 0.18, 0.18, 3.4, 10, 0.03)
	dark.Box(in.Add(vec3{1.0, 1.0, 1.25}), vec3{0.5, 0.7, 0.3}, vec3{0, 0, 1}, 0.04)
	b.Part("hazard", mesh.LitIndoor, vec3{}).Prism(in.Add(vec3{0, 1.6, 0}), up, 1.14, 1.14, 0.14, 20, 0.02)
	// A framed furnace window with bars, rivets around the body, a gauge.
	dark.Box(in.Add(vec3{1.03, 0.95, 0}), vec3{0.98, 0.72, 0.08}, vec3{0, 0, 1}, 0.03)
	for _, z := range []float64{-0.2, 0, 0.2} {
		steel.Box(in.Add(vec3{1.13, 0.95, z}), vec3{0.03, 0.56, 0.03}, vec3{0, 0, 1}, 0.005)
	}
	for k := range 20 {
		a := (float64(k) + 0.5) * 2 * math.Pi / 20
		dark.Box(in.Add(vec3{math.Cos(a) * 1.11, 1.78, math.Sin(a) * 1.11}), vec3{0.04, 0.04, 0.04}, vec3{-math.Sin(a), 0, math.Cos(a)}, 0)
	}
	b.Part("pipe.red", mesh.LitIndoor, vec3{}).Prism(in.Add(vec3{0.75, 1.3, -0.75}), vec3{1, 0, -1}, 0.12, 0.12, 0.05, 14, 0.01)
	b.Part("habitat.white", mesh.LitIndoor, vec3{}).Prism(in.Add(vec3{0.77, 1.3, -0.77}), vec3{1, 0, -1}, 0.09, 0.09, 0.05, 14, 0)
}

// drawIncinerator: the window glows harder while it burns dust.
func drawIncinerator(g *Game, in vec3) {
	gfx := g.gfx
	gfx.setLighting(mesh.LitIndoor, vec3{})
	gfx.setMat(mtPlain, 1, false)
	glow := float32(0.55 + 0.45*mathx.Clamp01(g.burn))
	gfx.box(in.Add(vec3{1.08, 0.95, 0}), vec3{0.8, 0.55, 0.06}, vec3{0, 0, 1}, accentOrange, 0, glow)
	gfx.drawScreen(in.Add(vec3{1.16, 1.1, 1.25}), vec3{1, 0, 0}, 0.38, 0.28, gfx.screens[3], 0.8)
}

// buildConsole: a desk between two server racks.
func buildConsole(b *building, c vec3, _ int) {
	b.Part("steel", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{-0.2, 0.78, 0}), vec3{0.9, 0.06, 2.2}, vec3{1, 0, 0}, 0.02)
	for _, z := range []float64{-0.95, 0.95} {
		b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{-0.2, 0.38, z}), vec3{0.8, 0.76, 0.08}, vec3{1, 0, 0}, 0.02)
	}
	for _, z := range []float64{-2.3, 2.3} {
		b.Part("rack.slate", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{0, 0.95, z}), vec3{0.7, 1.9, 0.7}, vec3{1, 0, 0}, 0.05)
	}
}

// drawConsole: three screens and the blinking lights of the racks.
func drawConsole(g *Game, c vec3) {
	gfx := g.gfx
	gfx.setLighting(mesh.LitIndoor, vec3{})
	gfx.setMat(mtPlain, 1, false)
	for i, z := range []float64{-0.75, 0, 0.75} {
		facing := vec3{-1, 0, z * 0.4}.Norm()
		gfx.drawScreen(c.Add(vec3{0.1, 1.28, z}), facing, 0.66, 0.42, gfx.screens[i], 0.8)
	}
	gfx.setLighting(mesh.LitIndoor, vec3{})
	gfx.setMat(mtPlain, 1, false)
	for _, z := range []float64{-2.3, 2.3} {
		for k := range 5 {
			on := float32(0.3)
			if math.Sin(g.clock*3+float64(k)*1.7+z) > 0 {
				on = 1
			}
			gfx.box(c.Add(vec3{-0.36, 0.4 + float64(k)*0.3, z}), vec3{0.5, 0.04, 0.02}, vec3{0, 0, 1}, colGreen, 0, on)
		}
	}
}

// buildBed: a bed against the far wall, a bedside table and a rug.
func buildBed(b *building, at vec3, _ int) {
	x := vec3{1, 0, 0}
	wood := b.Part("wood.brown", mesh.LitIndoor, vec3{})
	wood.Box(at.Add(vec3{0, 0.2, 0}), vec3{2.1, 0.4, 1.05}, x, 0.04)
	wood.Box(at.Add(vec3{1.45, 0.3, 0.2}), vec3{0.5, 0.6, 0.5}, x, 0.04)
	white := b.Part("fabric.white", mesh.LitIndoor, vec3{})
	white.Box(at.Add(vec3{0, 0.48, 0}), vec3{2.0, 0.16, 0.98}, x, 0.05)
	white.Box(at.Add(vec3{-0.72, 0.62, 0}), vec3{0.42, 0.12, 0.7}, x, 0.05)
	b.Part("fabric.purple", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{0.32, 0.58, 0}), vec3{1.36, 0.06, 1.0}, x, 0.02)
	b.Part("fabric.rug", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{0, floorTop + 0.005, -1.3}), vec3{1.8, 0.01, 1.1}, x, 0)
	// A padded headboard with a reading light, the blanket folded back.
	wood.Box(at.Add(vec3{-1.08, 0.55, 0}), vec3{0.08, 1.1, 1.1}, x, 0.02)
	b.Part("fabric.seat", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{-1.02, 0.78, 0}), vec3{0.06, 0.5, 0.95}, x, 0.03)
	b.Part("fabric.purple", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{-0.33, 0.62, 0}), vec3{0.24, 0.08, 1.02}, x, 0.03)
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{1.45, 0.62, 0.2}), vec3{0.3, 0.03, 0.3}, x, 0)
	b.Part("pipe.blue", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{1.4, 0.65, 0.03}), vec3{0.2, 0.04, 0.14}, x, 0.005)
}

func drawBed(g *Game, at vec3) {
	gfx := g.gfx
	gfx.setLighting(mesh.LitIndoor, vec3{})
	gfx.setMat(mtPlain, 1, false)
	gfx.put(gfx.sphere, toWorld(at.Add(vec3{1.45, 0.75, 0.2})), vec3{0.13, 0.13, 0.13}, vec3{0, 1, 0}, 0, lampWarm, 0, 1)
}

// upload sends the triangles to the GPU. The slices stay referenced by the builder:
// the mesh is never unloaded.
func upload(m *mesh.Builder, sh rl.Shader) rl.Model {
	mesh := rl.Mesh{VertexCount: int32(len(m.V) / 3), TriangleCount: int32(len(m.V) / 9)}
	mesh.Vertices, mesh.Normals, mesh.Texcoords = &m.V[0], &m.N[0], &m.UV[0]
	rl.UploadMesh(&mesh, false)
	mod := rl.LoadModelFromMesh(mesh)
	mod.GetMaterials()[0].Shader = sh
	return mod
}

// doorPanelModel is one sliding leaf of the airlock, built around its
// center so it can move.
func doorPanelModel(sh rl.Shader) rl.Model {
	m := &mesh.Builder{}
	m.Box(vec3{}, vec3{(doorWidth + 0.2) / 2, doorHeight, 0.08}, vec3{1, 0, 0}, 0.03)
	keep = append(keep, m)
	return upload(m, sh)
}

// keep holds builders whose slices back GPU meshes.
var keep []*mesh.Builder
