package base

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/mesh"
)

// The buildings' design system (docs/MODS.md): tokens say what a surface
// is made of, kit pieces are shapes built once in code, and blueprints are
// data that put pieces together. A blueprint is described once: drawing,
// collisions and interactions are deduced from it.

// roomOf is the room a blueprint lays out.
func roomOf(bp *content.Blueprint) Room {
	var openings, alongs []float64
	for _, d := range bp.Doors {
		openings = append(openings, d.Facing+bp.Yaw*math.Pi/180)
		alongs = append(alongs, d.Along)
	}
	if bp.Hall != nil {
		return Room{Center: bp.At, Yaw: bp.Yaw, WallH: bp.Hall.WallH, Hall: bp.Hall, openings: openings, alongs: alongs, plan: bp}
	}
	return Room{Center: bp.At, Yaw: bp.Yaw, Radius: bp.Shape.Radius, WallH: bp.Shape.WallH, openings: openings, alongs: alongs, plan: bp}
}

// KitBuild lets a kit piece use the base's building helpers.
func KitBuild(f func(b *building, at vec3, seed int)) func(*mesh.Parts, vec3, int) {
	return func(p *mesh.Parts, at vec3, seed int) { f(&building{Parts: p}, at, seed) }
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
	i := 0
	for id, r := range Buildings() {
		if r.Hall != nil {
			b.hall(id, *r)
			b.turned(*r, func(b *building) { b.clutter(r.plan) })
		} else {
			b.room(id, *r)
			b.greebleRoom(id, *r, i*97+3)
			b.turned(*r, func(b *building) { b.clutter(r.plan) })
		}
		b.floors(id)
		b.rails(id)
		i++
	}
	i = 0
	for _, c := range Corridors() {
		b.corridor(*c)
		b.greebleCorridor(*c, i*31+5)
		i++
	}
	b.yard()
	b.ground()
	for _, p := range junctionPlates() {
		room := ToWorld(p.mid.Sub(p.normal.Scale(2)).Add(vec3{0, 1.2, 0}))
		across := vec3{-p.normal.Z, 0, p.normal.X}
		b.Part("habitat.white2", mesh.LitTowards, room).Box(p.mid.Add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, WallThick * 1.5}, across, 0.04)
	}
	b.airlock()
	b.entry()
	b.outside()
	b.Unfight()
	return b.Parts
}

// turned builds what build makes in building r's own frame, facing east,
// turned with r around its center.
func (b *building) turned(r Room, build func(b *building)) {
	if r.Yaw == 0 {
		build(b)
		return
	}
	alone := &building{Parts: mesh.NewParts(vec3{}), kits: b.kits}
	build(alone)
	b.MergeTurned(alone.Parts, r.Turn)
}

// DoorPanel is one sliding leaf of the airlock, built around its center so
// it can move.
func DoorPanel() *mesh.Builder {
	m := &mesh.Builder{}
	m.Box(vec3{}, vec3{(DoorWidth + 0.2) / 2, DoorHeight, 0.08}, vec3{1, 0, 0}, 0.03)
	return m
}

func (b *building) room(id ecs.ID, r Room) {
	bp := r.plan
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
	// Door frames: steel jambs and a hazard header (a home's: homeDoorFrame). They stand a little
	// into the opening: flush with the corridor's walls and ceiling, they
	// would fight them for the depth and flicker.
	for _, a := range r.openings {
		out := vec3{math.Cos(a), 0, math.Sin(a)}
		tangent := vec3{-out.Z, 0, out.X}
		w := r.Center.Add(out.Scale(r.Radius))
		if r.home() {
			b.homeDoorFrame(w, out, homeDoorHalf, DoorHeight+0.1, 0.46, "home.accent")
			continue
		}
		for _, s := range []float64{-1, 1} {
			b.Part("steel", mesh.LitTowards, in).Box(w.Add(tangent.Scale(s*(DoorWidth/2+0.08))).Add(vec3{0, (DoorHeight + 0.1) / 2, 0}), vec3{0.26, DoorHeight + 0.1, 0.42}, tangent, 0.05)
		}
		b.Part("hazard", mesh.LitTowards, in).Box(w.Add(vec3{0, DoorHeight + 0.12, 0}), vec3{DoorWidth + 0.42, 0.22, 0.42}, tangent, 0.05)
		b.Part(bp.Accent, mesh.LitIndoor, vec3{}).Box(w.Add(vec3{0, DoorHeight + 0.12, 0}).Sub(out.Scale(0.22)), vec3{DoorWidth * 0.6, 0.05, 0.02}, tangent, 0)
	}
	if r.home() {
		b.domeLining(r)
	}
	b.interior(bp, r)
	b.props(id)
}

// props builds the furniture of building in, turned as each piece is.
func (b *building) props(in ecs.ID) {
	for _, f := range FurnitureOf(in) {
		k, _ := b.kits.Get(f.Kit)
		if k == nil || k.Build == nil {
			continue
		}
		b.piece(*f, func(p *mesh.Parts) { k.Build(p, f.At, f.Seed) })
	}
}

// piece builds a piece of furniture f, turned around its spot.
func (b *building) piece(f Furniture, build func(p *mesh.Parts)) {
	if f.Yaw == 0 {
		build(b.Parts)
		return
	}
	alone := mesh.NewParts(vec3{})
	build(alone)
	b.MergeTurned(alone, func(q vec3) vec3 { return f.At.Add(content.Turn(q.Sub(f.At), f.Yaw)) })
}

// Stage is the stage furniture id has grown to after some nights: one
// more each night, up to its last.
func Stage(id ecs.ID, nights int) int {
	n := Ents.Grows.Get(id)
	if n == nil {
		return 0
	}
	return min(max(nights, 0), *n-1)
}

// BuildGrowth builds what has grown of furniture id at a stage, apart
// from the rest of the base: it changes night after night.
func BuildGrowth(kits *content.Registry[content.Kit], id ecs.ID, stage int) *mesh.Parts {
	b := &building{Parts: mesh.NewParts(Origin), kits: kits}
	f := Ents.Furniture.Get(id)
	if k, _ := kits.Get(f.Kit); k != nil && k.Grow != nil {
		b.piece(*f, func(p *mesh.Parts) { k.Grow(p, f.At, stage) })
	}
	b.Unfight()
	return b.Parts
}

// interior: a flat floor with a steel-ringed inlay, a glowing stripe along
// the walls, steel ribs under the dome and a ceiling lamp. A home's floor
// has paths instead of the inlay, its walls are lined (domeLining) and its
// ribs are white, under its inner shell.
func (b *building) interior(bp *content.Blueprint, r Room) {
	up := vec3{0, 1, 0}
	floor := r.Center.Add(vec3{0, FloorTop - 0.1, 0})
	b.Part(r.floorTok(), mesh.LitIndoor, vec3{}).Prism(floor, up, r.Radius, r.Radius, 0.1, 48, 0)
	if r.home() {
		var doors []vec3
		for _, o := range r.openings {
			doors = append(doors, r.Center.Add(vec3{math.Cos(o), 0, math.Sin(o)}.Scale(r.Radius)))
		}
		b.paths(r.Center, doors, r.Radius*0.4)
	} else {
		b.Part("steel", mesh.LitIndoor, vec3{}).Prism(floor, up, r.Radius*0.47, r.Radius*0.47, 0.104, 40, 0)
		b.Part("floor.inlay", mesh.LitIndoor, vec3{}).Prism(floor, up, r.Radius*0.44, r.Radius*0.44, 0.106, 40, 0)
	}
	for _, p := range r.panels() {
		if p.y0 > 0 || r.home() {
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
	rib := "steel"
	if r.home() {
		a, bh = r.domeLining()
		a, bh, rib = a-0.02, bh-0.02, "home.rib"
	}
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
			b.Part(rib, mesh.LitIndoor, vec3{}).BoxBasis(p0.Lerp(p1, 0.5).Sub(along.Cross(side).Scale(0.05)), [3]vec3{side, along, side.Cross(along)}, vec3{0.12, l + 0.02, 0.1}, 0.02)
		}
	}
	if r.home() { // a ring of light, lower
		_, bh := r.domeLining()
		b.ringPendant(r.Center.Add(vec3{0, r.WallH + 0.6, 0}), 0.8, r.WallH+bh)
		return
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
	// Steel posts at the joints, and every third one a heavier rib round
	// the outside: the corridor reads as a passage.
	for i := 1; i < n; i++ {
		at := c.From.Add(dir.Scale(float64(i) * seg))
		w, out := 0.14, 0.04
		if i%3 == 0 {
			w, out = 0.32, 0.12
		}
		for _, s := range []float64{-1, 1} {
			post := at.Add(side.Scale(s * (c.Width/2 + WallThick/2 + out))).Add(vec3{0, (c.Height + 0.12) / 2, 0})
			b.Part("steel", mesh.LitTowards, in).Box(post, vec3{w, c.Height + 0.12 + 2*(out-0.04), WallThick + 2*out}, dir, 0.03)
		}
		b.Part("steel", mesh.LitTowards, in).Box(at.Add(vec3{0, c.Height + 0.14 + out - 0.04, 0}), vec3{w, 0.1 + 2*(out-0.04), c.Width + 2*WallThick + 4*out}, dir, 0.03)
	}
	// Inside: two-tone walls like the rooms, a flat floor edged with steel,
	// a lamp strip under the roof; or a home's lining and paths.
	if c.home() {
		b.corridorLining(c)
		return
	}
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

// roof is the height of the outside of the room's roof over local point
// at: a dome's taken where its antenna stands.
func (r Room) roof(at vec3) float64 {
	if r.Hall == nil {
		return r.WallH + r.Radius*0.55 - 0.8
	}
	return r.vault(r.Unturn(at).X-r.Center.X) + 0.24
}

// HubTop is how high the hub's roof rises over the ground.
func HubTop() float64 {
	_, hub := Hub()
	if hub.Hall != nil {
		return hub.WallH + hub.Hall.Rise
	}
	return hub.WallH + hub.Radius*0.55
}

// mast and dish are where the hub's antenna and dish stand on its roof.
func mast() vec3 { _, hub := Hub(); return hub.Center.Add(vec3{1.5, 0, 1.5}) }
func dish() vec3 { _, hub := Hub(); return hub.Center.Add(vec3{-1.8, 0, -1.2}) }

// BeaconTop is where the red light blinks, on top of the antenna (local).
func BeaconTop() vec3 {
	_, hub := Hub()
	return mast().Add(vec3{0, hub.roof(mast()) + 3.05, 0})
}

// outside: an antenna and a dish on the hub.
func (b *building) outside() {
	_, hub := Hub()
	up := vec3{0, 1, 0}
	steel := b.Part("steel", mesh.LitSun, vec3{})
	m := mast()
	m.Y = hub.roof(m)
	steel.Prism(m, up, 0.05, 0.05, 3.0, 6, 0)
	steel.Box(m.Add(vec3{0, 0.1, 0}), vec3{0.3, 0.2, 0.3}, vec3{1, 0, 0}, 0.04)
	tilt := vec3{math.Sin(25 * math.Pi / 180), math.Cos(25 * math.Pi / 180), 0}
	d := dish()
	d.Y = hub.roof(d)
	steel.Prism(d.Add(vec3{0, 0.1, 0}), up, 0.08, 0.08, 0.35, 8, 0)
	b.Part("habitat.white", mesh.LitSun, vec3{}).Prism(d.Add(vec3{0, 0.4, 0}), tilt, 0.15, 0.75, 0.28, 16, 0.03)
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

// BuildMapTable: a smooth white island, a line of light round its dark
// top, and a glowing disc under the hologram (drawGlobe).
func BuildMapTable(b *building, t vec3, _ int) {
	b.in("home.trim").Prism(t, yUp, 0.8, 0.8, 0.08, 40, 0)
	b.in("home.panel2").Prism(t.Add(vec3{0, 0.08, 0}), yUp, 0.9, 1.05, 0.76, 48, 0.04)
	b.in("home.led").Prism(t.Add(vec3{0, 0.84, 0}), yUp, 1.18, 1.18, 0.02, 48, 0)
	b.in("home.trim").Prism(t.Add(vec3{0, 0.86, 0}), yUp, 1.2, 1.2, 0.06, 48, 0.02)
	b.in("neon.cyan").Prism(t.Add(vec3{0, 0.92, 0}), yUp, 0.55, 0.55, 0.01, 40, 0)
}

// BuildIncinerator: a white casing round the furnace, its window and its
// screen in the front (+X, drawIncinerator), a hatch on top, a line of the
// sale's orange.
func BuildIncinerator(b *building, in vec3, _ int) {
	p := func(x, y, z float64) vec3 { return in.Add(vec3{x, y, z}) }
	b.in("home.trim").Box(p(0, 0.05, 0.25), vec3{1.9, 0.1, 2.4}, xAxis, 0)
	b.in("home.panel").Box(p(0.02, 1.1, 0.25), vec3{2.04, 2.0, 2.5}, xAxis, 0.1)
	b.in("home.trim").Box(p(1.04, 0.95, 0), vec3{0.04, 0.78, 1.0}, xAxis, 0.02)
	b.in("home.trim").Box(p(1.07, 1.1, 1.25), vec3{0.06, 0.4, 0.5}, xAxis, 0.02)
	b.in("home.trim").Box(p(0.1, 2.11, 0.25), vec3{1.2, 0.03, 1.4}, xAxis, 0.01)
	b.in("neon.orange").Box(p(1.045, 1.6, 0.25), vec3{0.01, 0.025, 2.2}, xAxis, 0)
}

// BuildConsole: a white desk between two white server cabinets, their
// lights behind a dark strip (drawConsole), the screens on slim stands.
func BuildConsole(b *building, c vec3, _ int) {
	p := func(x, y, z float64) vec3 { return c.Add(vec3{x, y, z}) }
	b.in("home.panel").Box(p(-0.2, 0.78, 0), vec3{0.9, 0.06, 2.2}, xAxis, 0.025)
	for _, z := range []float64{-0.95, 0.95} {
		b.in("home.panel2").Box(p(-0.15, 0.375, z), vec3{0.75, 0.75, 0.08}, xAxis, 0.02)
	}
	b.in("home.trim").Box(p(0.18, 0.4, 0), vec3{0.04, 0.6, 1.82}, xAxis, 0.01)
	for _, z := range []float64{-0.75, 0, 0.75} {
		b.in("home.trim").Prism(p(0.16, 0.81, z), yUp, 0.1, 0.1, 0.012, 16, 0)
		b.in("home.trim").Box(p(0.16, 1.0, z), vec3{0.03, 0.4, 0.05}, xAxis, 0)
	}
	for _, z := range []float64{-2.3, 2.3} {
		b.in("home.trim").Box(p(0, 0.05, z), vec3{0.6, 0.1, 0.6}, xAxis, 0.01)
		b.in("home.panel").Box(p(0, 1.0, z), vec3{0.7, 1.8, 0.7}, xAxis, 0.05)
		b.in("home.trim").Box(p(-0.345, 1.0, z), vec3{0.02, 1.6, 0.58}, xAxis, 0.005)
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
