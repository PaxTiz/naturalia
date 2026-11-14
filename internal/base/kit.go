package base

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/mesh"
)

// The buildings' design system (docs/MODS.md): tokens say what a surface
// is made of, kit pieces are shapes built once in code, and blueprints are
// data that put pieces together. A blueprint is described once: drawing,
// collisions and interactions are deduced from it.

// roomOf is the room a blueprint lays out.
func roomOf(bp *content.Blueprint) Room {
	var openings []float64
	for _, d := range bp.Doors {
		openings = append(openings, d.Facing)
	}
	if bp.Hall != nil {
		return Room{Center: bp.At, WallH: bp.Hall.WallH, Hall: bp.Hall, openings: openings, plan: bp}
	}
	return Room{bp.At, bp.Shape.Radius, bp.Shape.WallH, nil, openings, bp}
}

// KitBuild lets a kit piece use the base's building helpers.
func KitBuild(f func(b *building, at vec3, seed int)) func(*mesh.Parts, vec3, int) {
	return func(p *mesh.Parts, at vec3, seed int) { f(&building{Parts: p}, at, seed) }
}

func planObstacles(kits *content.Registry[content.Kit]) (out []content.Obstacle) {
	out = YardObstacles()
	for _, bp := range Plans {
		for _, p := range bp.Props {
			k, _ := kits.Get(p.Kit)
			if k == nil {
				continue
			}
			for _, o := range k.Obstacles {
				out = append(out, content.Obstacle{bp.At.Add(p.At).Add(content.Turn(o.At, p.Yaw)), o.Radius})
			}
		}
	}
	return out
}

func planInteractables(kits *content.Registry[content.Kit]) (out []Interactable) {
	for _, bp := range Plans {
		for _, p := range bp.Props {
			if k, _ := kits.Get(p.Kit); k != nil && k.Use != nil {
				out = append(out, Interactable{content.Qualify(p.Kit), bp.At.Add(p.At), k.Reach, k.Use, k.Prompt})
			}
		}
	}
	return out
}

// building builds the base's meshes, with its helpers for walls, greebles
// and furniture.
type building struct {
	*mesh.Parts
	kits *content.Registry[content.Kit] // furniture of the blueprints
}

// Build builds every static piece of the base, furniture from kits.
func Build(kits *content.Registry[content.Kit]) *mesh.Parts {
	b := &building{Parts: mesh.NewParts(Origin), kits: kits}
	for i, bp := range Plans {
		if bp.Hall != nil {
			b.hall(bp, roomOf(bp))
			continue
		}
		b.room(bp)
		b.greebleRoom(bp, roomOf(bp), i*97+3)
		b.clutter(bp)
	}
	for i, c := range Corridors {
		b.corridor(c)
		b.greebleCorridor(c, i*31+5)
	}
	b.yard()
	for _, p := range junctionPlates() {
		room := ToWorld(p.mid.Sub(p.normal.Scale(2)).Add(vec3{0, 1.2, 0}))
		across := vec3{-p.normal.Z, 0, p.normal.X}
		b.Part("habitat.white2", mesh.LitTowards, room).Box(p.mid.Add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, WallThick * 1.5}, across, 0.04)
	}
	b.airlock()
	b.outside()
	b.Unfight()
	return b.Parts
}

// DoorPanel is one sliding leaf of the airlock, built around its center so
// it can move.
func DoorPanel() *mesh.Builder {
	m := &mesh.Builder{}
	m.Box(vec3{}, vec3{(DoorWidth + 0.2) / 2, DoorHeight, 0.08}, vec3{1, 0, 0}, 0.03)
	return m
}

func (b *building) room(bp *content.Blueprint) {
	r := roomOf(bp)
	in := ToWorld(r.Center.Add(vec3{0, 1.2, 0}))
	// Walls: bevelled panels, a dark plinth, a steel collar under the dome.
	for i, p := range r.panels() {
		tok := bp.Style
		if i%2 == 1 {
			tok += "2"
		}
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		tangent := vec3{-out.Z, 0, out.X}
		b.Part(tok, mesh.LitTowards, in).Box(p.mid.Add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, WallThick}, tangent, 0.04)
		b.Part("steel", mesh.LitTowards, in).Box(p.mid.Add(vec3{0, p.y1 - 0.1, 0}).Add(out.Scale(0.03)), vec3{p.width, 0.2, WallThick + 0.08}, tangent, 0.035)
		if p.y0 == 0 {
			b.Part("steel.dark", mesh.LitTowards, in).Box(p.mid.Add(vec3{0, 0.15, 0}).Add(out.Scale(0.03)), vec3{p.width, 0.3, WallThick + 0.08}, tangent, 0.035)
		}
	}
	// The dome stays a smooth shell: dust settles on its top.
	b.Part(bp.Style, mesh.LitBack, in).Dome(r.Center.Add(vec3{0, r.WallH, 0}), r.Radius+0.12, r.Radius*0.55)
	// Door frames: steel jambs and a hazard header. They stand a little
	// into the opening: flush with the corridor's walls and ceiling, they
	// would fight them for the depth and flicker.
	for _, d := range bp.Doors {
		out := vec3{math.Cos(d.Facing), 0, math.Sin(d.Facing)}
		tangent := vec3{-out.Z, 0, out.X}
		w := r.Center.Add(out.Scale(r.Radius))
		for _, s := range []float64{-1, 1} {
			b.Part("steel", mesh.LitTowards, in).Box(w.Add(tangent.Scale(s*(DoorWidth/2+0.08))).Add(vec3{0, (DoorHeight + 0.1) / 2, 0}), vec3{0.26, DoorHeight + 0.1, 0.42}, tangent, 0.05)
		}
		b.Part("hazard", mesh.LitTowards, in).Box(w.Add(vec3{0, DoorHeight + 0.12, 0}), vec3{DoorWidth + 0.42, 0.22, 0.42}, tangent, 0.05)
		b.Part(bp.Accent, mesh.LitIndoor, vec3{}).Box(w.Add(vec3{0, DoorHeight + 0.12, 0}).Sub(out.Scale(0.22)), vec3{DoorWidth * 0.6, 0.05, 0.02}, tangent, 0)
	}
	b.interior(bp, r)
	b.props(bp)
}

// props builds the furniture of a building, turned as each piece is.
func (b *building) props(bp *content.Blueprint) {
	for _, p := range bp.Props {
		k, _ := b.kits.Get(p.Kit)
		if k == nil || k.Build == nil {
			continue
		}
		at := bp.At.Add(p.At)
		if p.Yaw == 0 {
			k.Build(b.Parts, at, p.Seed)
			continue
		}
		// Turned: built apart, then turned around its spot.
		alone := mesh.NewParts(vec3{})
		k.Build(alone, at, p.Seed)
		b.MergeTurned(alone, func(q vec3) vec3 { return at.Add(content.Turn(q.Sub(at), p.Yaw)) })
	}
}

// interior: a flat floor with a steel-ringed inlay, a glowing stripe along
// the walls, steel ribs under the dome and a ceiling lamp.
func (b *building) interior(bp *content.Blueprint, r Room) {
	up := vec3{0, 1, 0}
	floor := r.Center.Add(vec3{0, FloorTop - 0.1, 0})
	b.Part("floor.slate", mesh.LitIndoor, vec3{}).Prism(floor, up, r.Radius, r.Radius, 0.1, 48, 0)
	b.Part("steel", mesh.LitIndoor, vec3{}).Prism(floor, up, r.Radius*0.47, r.Radius*0.47, 0.104, 40, 0)
	b.Part("floor.inlay", mesh.LitIndoor, vec3{}).Prism(floor, up, r.Radius*0.44, r.Radius*0.44, 0.106, 40, 0)
	for _, p := range r.panels() {
		if p.y0 > 0 {
			continue
		}
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		along := vec3{-out.Z, 0, out.X}
		// Two-tone walls: a steel-blue wainscot capped by the accent strip,
		// so the light upper walls stand out over a darker base.
		inner := p.mid.Sub(out.Scale(WallThick/2 + 0.02))
		b.Part("wall.lower", mesh.LitIndoor, vec3{}).Box(inner.Add(vec3{0, 0.5, 0}), vec3{p.width * 0.99, 0.96, 0.04}, along, 0.012)
		b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(inner.Add(vec3{0, 0.07, 0}).Sub(out.Scale(0.02)), vec3{p.width * 0.99, 0.14, 0.04}, along, 0.01)
		strip := p.mid.Sub(out.Scale(WallThick/2 + 0.045)).Add(vec3{0, 1.0, 0})
		b.Part(bp.Accent, mesh.LitIndoor, vec3{}).Box(strip, vec3{p.width * 0.94, 0.07, 0.03}, along, 0.008)
	}
	// Ribs follow the inside of the dome up to the lamp's collar.
	a, bh := r.Radius+0.02, r.Radius*0.55-0.06
	top := r.Center.Add(vec3{0, r.WallH, 0})
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
	lamp := r.Center.Add(vec3{0, r.WallH + r.Radius*0.5, 0})
	b.Part("steel", mesh.LitIndoor, vec3{}).Prism(lamp.Add(vec3{0, -0.02, 0}), up, 0.62, 0.55, 0.12, 24, 0.02)
	b.Part("lamp.warm", mesh.LitIndoor, vec3{}).Prism(lamp.Add(vec3{0, -0.06, 0}), up, 0.48, 0.48, 0.05, 24, 0)
}

// corridor: panels with joints, a dark plinth outside, a roof slab and
// steel ribs at the joints.
func (b *building) corridor(c Corridor) {
	mid := c.From.Lerp(c.To, 0.5)
	dir := c.To.Sub(c.From)
	length := dir.Length()
	dir = dir.Scale(1 / length)
	side := c.side()
	in := ToWorld(mid.Add(vec3{0, 1.2, 0}))
	n := int(math.Ceil(length / 1.6))
	seg := length / float64(n)
	for i := range n {
		at := c.From.Add(dir.Scale((float64(i) + 0.5) * seg))
		tok := "habitat.white"
		if i%2 == 1 {
			tok += "2"
		}
		for _, s := range []float64{-1, 1} {
			wall := at.Add(side.Scale(s * (c.Width/2 + WallThick/2)))
			b.Part(tok, mesh.LitTowards, in).Box(wall.Add(vec3{0, c.Height / 2, 0}), vec3{seg * 1.01, c.Height, WallThick}, dir, 0.04)
			b.Part("steel.dark", mesh.LitTowards, in).Box(wall.Add(side.Scale(s*0.03)).Add(vec3{0, 0.15, 0}), vec3{seg * 1.01, 0.3, WallThick + 0.08}, dir, 0.035)
		}
	}
	b.Part("habitat.white2", mesh.LitTowards, in).Box(mid.Add(vec3{0, c.Height + 0.06, 0}), vec3{length, 0.12, c.Width + 2*WallThick}, dir, 0.04)
	for i := 1; i < n; i++ {
		at := c.From.Add(dir.Scale(float64(i) * seg))
		for _, s := range []float64{-1, 1} {
			post := at.Add(side.Scale(s * (c.Width/2 + WallThick/2 + 0.04))).Add(vec3{0, (c.Height + 0.12) / 2, 0})
			b.Part("steel", mesh.LitTowards, in).Box(post, vec3{0.14, c.Height + 0.12, WallThick + 0.08}, dir, 0.03)
		}
		b.Part("steel", mesh.LitTowards, in).Box(at.Add(vec3{0, c.Height + 0.14, 0}), vec3{0.14, 0.1, c.Width + 2*WallThick + 0.16}, dir, 0.03)
	}
	// Inside: two-tone walls like the rooms, a flat floor edged with steel,
	// a lamp strip under the roof.
	for _, s := range []float64{-1, 1} {
		inner := mid.Add(side.Scale(s * (c.Width/2 - 0.02)))
		b.Part("wall.lower", mesh.LitIndoor, vec3{}).Box(inner.Add(vec3{0, 0.5, 0}), vec3{length, 0.96, 0.04}, dir, 0.012)
		b.Part("neon.cyan", mesh.LitIndoor, vec3{}).Box(inner.Add(vec3{0, 1.0, 0}).Sub(side.Scale(s*0.015)), vec3{length * 0.98, 0.06, 0.03}, dir, 0.008)
	}
	b.Part("floor.slate", mesh.LitIndoor, vec3{}).Box(mid.Add(vec3{0, FloorTop - 0.05, 0}), vec3{length, 0.1, c.Width}, dir, 0)
	for _, s := range []float64{-1, 1} {
		b.Part("steel", mesh.LitIndoor, vec3{}).Box(mid.Add(side.Scale(s*(c.Width/2-0.1))).Add(vec3{0, FloorTop - 0.045, 0}), vec3{length, 0.1, 0.2}, dir, 0.01)
	}
	b.Part("steel", mesh.LitIndoor, vec3{}).Box(mid.Add(vec3{0, c.Height - 0.03, 0}), vec3{length * 0.72, 0.06, 0.34}, dir, 0.015)
	b.Part("lamp.warm", mesh.LitIndoor, vec3{}).Box(mid.Add(vec3{0, c.Height - 0.07, 0}), vec3{length * 0.7, 0.03, 0.25}, dir, 0)
}

// airlock: the outer frame. The sliding doors move: see drawAirlock.
func (b *building) airlock() {
	end := AirlockEnd
	across := vec3{1, 0, 0}
	half := (DoorWidth+0.2)/2 + WallThick
	for _, s := range []float64{-1, 1} {
		b.Part("hazard", mesh.LitSun, vec3{}).Box(end.Add(vec3{s * half, 1.35, 0}), vec3{0.3, 2.7, 0.35}, across, 0.04)
	}
	b.Part("steel.dark", mesh.LitSun, vec3{}).Box(end.Add(vec3{0, 2.6, 0}), vec3{2*half + 0.3, 0.3, 0.35}, across, 0.05)
}

// outside: an antenna and a dish on the hub, solar panels nearby.
func (b *building) outside() {
	top := HubRoom.WallH + HubRoom.Radius*0.55
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

// BuildCrates stacks two or three bevelled crates, chosen by the seed.
func BuildCrates(b *building, at vec3, seed int) {
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

// BuildMapTable: a faceted pedestal, a steel top and a glowing disc under
// the hologram (drawGlobe).
func BuildMapTable(b *building, t vec3, _ int) {
	up := vec3{0, 1, 0}
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Prism(t, up, 0.85, 0.85, 0.85, 20, 0.03)
	b.Part("steel", mesh.LitIndoor, vec3{}).Prism(t.Add(vec3{0, 0.85, 0}), up, 1.2, 1.2, 0.08, 24, 0.02)
	b.Part("neon.cyan", mesh.LitIndoor, vec3{}).Prism(t.Add(vec3{0, 0.93, 0}), up, 1.0, 1.0, 0.02, 24, 0)
}

func BuildIncinerator(b *building, in vec3, _ int) {
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

// BuildConsole: a desk between two server racks.
func BuildConsole(b *building, c vec3, _ int) {
	b.Part("steel", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{-0.2, 0.78, 0}), vec3{0.9, 0.06, 2.2}, vec3{1, 0, 0}, 0.02)
	for _, z := range []float64{-0.95, 0.95} {
		b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{-0.2, 0.38, z}), vec3{0.8, 0.76, 0.08}, vec3{1, 0, 0}, 0.02)
	}
	for _, z := range []float64{-2.3, 2.3} {
		b.Part("rack.slate", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{0, 0.95, z}), vec3{0.7, 1.9, 0.7}, vec3{1, 0, 0}, 0.05)
	}
}

// BuildGarageTerminal: a kiosk standing on a plinth, its screen facing
// +X, a hazard band at its foot.
func BuildGarageTerminal(b *building, c vec3, _ int) {
	x := vec3{1, 0, 0}
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{0, 0.05, 0}), vec3{0.9, 0.1, 1.2}, x, 0.02)
	b.Part("steel", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{-0.05, 0.6, 0}), vec3{0.45, 1.0, 0.8}, x, 0.04)
	b.Part("hazard", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{-0.05, 0.25, 0}), vec3{0.47, 0.18, 0.82}, x, 0)
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{-0.05, 1.4, 0}), vec3{0.2, 0.75, 1.1}, x, 0.04)
	b.Part("pipe.yellow", mesh.LitIndoor, vec3{}).Box(c.Add(vec3{-0.05, 1.82, 0}), vec3{0.3, 0.08, 1.2}, x, 0.02)
}

// BuildBed: a bed against the far wall, a bedside table and a rug.
func BuildBed(b *building, at vec3, _ int) {
	x := vec3{1, 0, 0}
	wood := b.Part("wood.brown", mesh.LitIndoor, vec3{})
	wood.Box(at.Add(vec3{0, 0.2, 0}), vec3{2.1, 0.4, 1.05}, x, 0.04)
	wood.Box(at.Add(vec3{1.45, 0.3, 0.2}), vec3{0.5, 0.6, 0.5}, x, 0.04)
	white := b.Part("fabric.white", mesh.LitIndoor, vec3{})
	white.Box(at.Add(vec3{0, 0.48, 0}), vec3{2.0, 0.16, 0.98}, x, 0.05)
	white.Box(at.Add(vec3{-0.72, 0.62, 0}), vec3{0.42, 0.12, 0.7}, x, 0.05)
	b.Part("fabric.purple", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{0.32, 0.58, 0}), vec3{1.36, 0.06, 1.0}, x, 0.02)
	b.Part("fabric.rug", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{0, FloorTop + 0.005, -1.3}), vec3{1.8, 0.01, 1.1}, x, 0)
	// A padded headboard with a reading light, the blanket folded back.
	wood.Box(at.Add(vec3{-1.08, 0.55, 0}), vec3{0.08, 1.1, 1.1}, x, 0.02)
	b.Part("fabric.seat", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{-1.02, 0.78, 0}), vec3{0.06, 0.5, 0.95}, x, 0.03)
	b.Part("fabric.purple", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{-0.33, 0.62, 0}), vec3{0.24, 0.08, 1.02}, x, 0.03)
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{1.45, 0.62, 0.2}), vec3{0.3, 0.03, 0.3}, x, 0)
	b.Part("pipe.blue", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{1.4, 0.65, 0.03}), vec3{0.2, 0.04, 0.14}, x, 0.005)
}
