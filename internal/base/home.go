package base

import (
	"math"
	"naturalia/internal/ecs"
	"naturalia/internal/mesh"
)

// Home interiors (interior: "home" on a blueprint): the living spaces are
// finished like a well-made Mars habitat, inside only. Off-white composite
// panels with fine joints line the walls and the roof, a warm grey floor
// has a lighter path from each doorway to the middle, the arches are
// smooth white ribs and the ducts run boxed in along the ceiling. The
// shell outside doesn't change. Its finishes are tokens (home.*): a mod
// can repaint them. Warmer finishes are laid as kits (cosy.go).

const (
	lining    = 0.05 // the panels' thickness
	liningGap = 0.02 // between them and the wall: not flush, they would flicker
	// The panels are about this wide and tall, a fine joint between them.
	homePanelW, homePanelH, homeJoint = 1.6, 2.6, 0.008
)

// homeWarm is how warm a living space's lamps are: an evening white,
// nearer amber than the base's white (Lamp.Warm).
const homeWarm = 0.55

// homeDim is how bright a living space's ceiling lamps are, against the
// base's: its lines of light and its lamps low down do the rest.
const homeDim = 0.3

// lampPower is how bright a dome's ceiling lamp is.
func (r Room) lampPower() float64 {
	if r.home() {
		return homeDim
	}
	return 1
}

// lampWarmth is how warm the room's ceiling lamps are.
func (r Room) lampWarmth() float64 {
	switch {
	case r.warm():
		return 1
	case r.home():
		return homeWarm
	}
	return 0
}

// led is the token of the lights built into a living space: warm white,
// amber in a warm room.
func (r Room) led() string {
	if r.warm() {
		return "home.led.amber"
	}
	return "home.led"
}

// home tells a living space, finished like a home.
func (r Room) home() bool { return r.plan != nil && r.plan.Interior == "home" }

// home tells a corridor between two living spaces.
func (c Corridor) home() bool {
	for _, id := range c.Rooms {
		if id == 0 || !Ents.Building.Get(id).home() {
			return false
		}
	}
	return true
}

// floorTok is the token of the room's floor: in a living space a
// seamless warm floor, a carpet in a warm room (the bedroom).
func (r Room) floorTok() string {
	switch {
	case r.home() && r.warm():
		return "carpet.oat"
	case r.home():
		return "home.floor"
	}
	return "floor.slate"
}

// liningPanel lines the wall from a to b (at floor level, its inner face
// a wall's half thickness th off that line, toward nrm) from y0 to y1, in
// panels.
func (b *building) liningPanel(a, e, nrm vec3, th, y0, y1 float64) {
	d := e.Sub(a)
	l := d.Length()
	d = d.Scale(1 / l)
	nu, nv := max(1, int(math.Round(l/homePanelW))), max(1, int(math.Round((y1-y0)/homePanelH)))
	w, h := l/float64(nu), (y1-y0)/float64(nv)
	base := a.Add(nrm.Scale(th + liningGap + lining/2))
	for i := range nu {
		for j := range nv {
			at := base.Add(d.Scale((float64(i) + 0.5) * w)).Add(vec3{0, y0 + (float64(j)+0.5)*h, 0})
			b.in("home.panel").Box(at, vec3{w - homeJoint, h - homeJoint, lining}, d, 0.012)
		}
	}
}

// paths lays a lighter band on the floor from each doorway to the middle
// of the room, ending in a disc of radius ring: where one walks.
func (b *building) paths(c vec3, doors []vec3, ring float64) {
	const w, th = 1.6, 0.02
	top := c.Add(vec3{0, FloorTop - th, 0})
	b.in("home.path").Prism(top, yUp, ring, ring, th, 48, 0)
	for _, d := range doors {
		d = vec3{d.X, c.Y, d.Z}
		along := c.Sub(d)
		l := along.Length() - ring + 0.1
		if l <= 0 {
			continue
		}
		along = along.Scale(1 / along.Length())
		b.in("home.path").Box(d.Add(along.Scale(l/2)).Add(vec3{0, FloorTop - th/2, 0}), vec3{l, th, w}, along, 0)
	}
}

// hallLining lines a home hall inside, in its own frame: its walls around
// the windows and above the doorways, the gables up under the vault, the
// vault itself, and a duct casing along the top of the long walls.
func (b *building) hallLining(r Room) {
	hx, hz := r.half()
	c := r.Center
	X, Z := vec3{1, 0, 0}, vec3{0, 0, 1}
	p := func(x, y, z float64) vec3 { return c.Add(vec3{x, y, z}) }
	th := hallWall / 2
	for k, w := range r.straightWalls() {
		nrm := []vec3{X, Z, Z.Scale(-1), X.Scale(-1), X.Scale(-1)}[k]
		for _, s := range r.spans(w[0], w[1]) {
			for _, sd := range r.solid(s[0], s[1]) {
				b.liningPanel(sd[0], sd[1], nrm, th, 0, r.WallH)
			}
			for _, pn := range r.panes(s[0], s[1]) {
				if pn.y0 > 0.01 {
					b.liningPanel(pn.a, pn.b, nrm, th, 0, pn.y0)
				}
				b.liningPanel(pn.a, pn.b, nrm, th, pn.y1, r.WallH)
			}
			if r.panes(s[0], s[1]) == nil || r.panes(s[0], s[1])[0].y0 > 0.2 {
				b.footLight(s[0], s[1], nrm, th, r.led())
			}
		}
	}
	for _, w := range r.doorways() {
		out := r.wallOut(w)
		along := vec3{-out.Z, 0, out.X}
		b.liningPanel(w.Sub(along.Scale(hallGap)), w.Add(along.Scale(hallGap)), out.Scale(-1), th, DoorHeight+0.1, r.WallH)
	}

	// The gables, under the vault's lining, and the vault's lining, in
	// plates across like the roof.
	const steps = 24
	in := th + liningGap + lining
	for _, s := range []float64{-1, 1} {
		var pts []vec3
		for i := 0; i <= steps; i++ {
			u := -hx + 2*hx*float64(i)/steps
			pts = append(pts, p(u, r.vault(u)-liningGap-lining, s*(hz-in+lining/2)))
		}
		b.in("home.panel").Slab(pts, Z, lining)
	}
	// A panel spans a few of the vault's plates, about homePanelW across:
	// a joint only where it ends.
	arch := func(u float64) vec3 { return vec3{u, r.vault(u), 0} }
	per := max(1, int(math.Round(homePanelW/arch(hx/steps).Sub(arch(-hx/steps)).Length()*2)))
	for i := range steps {
		u0, u1 := -hx+2*hx*float64(i)/steps, -hx+2*hx*float64(i+1)/steps
		a, e := arch(u0), arch(u1)
		along := e.Sub(a)
		l := along.Length()
		along = along.Scale(1 / l)
		nrm := Z.Cross(along) // up and out
		mid := c.Add(a.Lerp(e, 0.5)).Sub(nrm.Scale(liningGap + lining/2))
		joint := 0.0
		if (i+1)%per == 0 || i == steps-1 {
			joint = homeJoint
			mid = mid.Sub(along.Scale(joint / 2))
		}
		n := max(1, int(math.Round(2*(hz-in)/homePanelH)))
		w := 2 * (hz - in) / float64(n)
		for k := range n {
			z := -hz + in + (float64(k)+0.5)*w
			b.in("home.panel").BoxBasis(mid.Add(vec3{0, 0, z}), [3]vec3{along, nrm, Z}, vec3{l - joint, lining, w - homeJoint}, 0)
		}
	}

	// The duct casing, where the long walls meet the vault: smaller in a
	// small hall.
	deep, tall := 0.45, 0.35
	if r.WallH < 4 {
		deep, tall = 0.3, 0.24
	}
	for _, s := range []float64{-1, 1} {
		x := s * (hx - in - deep/2)
		b.in("home.panel2").Box(p(x, r.WallH-tall/2, 0), vec3{deep, tall, 2 * (hz - in)}, X, 0.03)
		b.casingLight(p(x-s*(deep/2-0.06), r.WallH-tall, 0), Z, 2*(hz-in), r.led())
	}
}

// domeLining lines a home dome inside: its wall panels, a shell under the
// dome, and a duct casing around the top of the wall.
func (b *building) domeLining(r Room) {
	th := WallThick / 2
	for _, p := range r.panels() {
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		along := vec3{-out.Z, 0, out.X}
		t := along.Scale(p.width / 2)
		b.liningPanel(p.mid.Sub(t), p.mid.Add(t), out.Scale(-1), th, p.y0, p.y1)
		if p.y0 > 0 {
			continue // over a doorway
		}
		// Up over the foot of the shell: from under it, the seam doesn't
		// show.
		casing := p.mid.Sub(out.Scale(th + liningGap + 0.2)).Add(vec3{0, r.WallH - 0.04, 0})
		b.in("home.panel2").Box(casing, vec3{p.width, 0.32, 0.36}, along, 0.03)
		b.casingLight(casing.Sub(out.Scale(0.12)).Sub(vec3{0, 0.16, 0}), along, p.width, r.led())
		b.footLight(p.mid.Sub(t), p.mid.Add(t), out.Scale(-1), th, r.led())
	}
	a, bh := r.domeLining()
	b.Part("home.panel", mesh.LitBack, vec3{}).Dome(r.Center.Add(vec3{0, r.WallH, 0}), a, bh)
}

// domeLining is the size of a home dome's inner shell: its radius and
// height over the top of the wall.
func (r Room) domeLining() (a, bh float64) {
	return r.Radius - WallThick/2 - liningGap - lining, r.Radius*0.55 - 0.1
}

// corridorLining lines a corridor between two living spaces: its walls
// and ceiling, a duct casing along the top of each wall, lit underneath,
// a line of amber light at the foot of the walls, the floor.
func (b *building) corridorLining(c Corridor) {
	mid := c.From.Lerp(c.To, 0.5)
	dir := c.To.Sub(c.From)
	length := dir.Length()
	dir = dir.Scale(1 / length)
	side := c.side()
	in := liningGap + lining
	for _, s := range []float64{-1, 1} {
		wall := mid.Add(side.Scale(s * c.Width / 2))
		b.liningPanel(wall.Sub(dir.Scale(length/2)), wall.Add(dir.Scale(length/2)), side.Scale(-s), 0, 0, c.Height)
		casing := wall.Sub(side.Scale(s * (in + 0.1))).Add(vec3{0, c.Height - in - 0.07, 0})
		b.in("home.panel2").Box(casing, vec3{length, 0.18, 0.2}, dir, 0.02)
		b.casingLight(casing.Sub(side.Scale(s*0.04)).Sub(vec3{0, 0.09, 0}), dir, length, "home.led")
		b.footLight(wall.Sub(dir.Scale(length/2)), wall.Add(dir.Scale(length/2)), side.Scale(-s), 0, "home.led.amber")
	}
	b.in("home.panel").Box(mid.Add(vec3{0, c.Height - liningGap - lining/2, 0}), vec3{length, lining, c.Width - 2*in}, dir, 0)
	b.in(c.floorTok()).Box(mid.Add(vec3{0, FloorTop - 0.05, 0}), vec3{length, 0.1, c.Width}, dir, 0)
}

// floorTok is the token of the corridor's floor.
func (c Corridor) floorTok() string {
	if c.home() {
		return "home.floor"
	}
	return "floor.slate"
}

// homeDoorHalf is half the opening of a living space's doorway, inside
// its frame: just wider than the corridor's lining, not flush with it.
const homeDoorHalf = DoorWidth/2 - 0.04

// homeDoorFrame frames a doorway of a living space: w its middle on the
// floor (in the middle of the wall), out the way out of the room, half
// the opening's half width and h its height, deep the frame's depth
// through the wall. A rounded anthracite profile, and a lit line of accent
// along its inner edge, facing the room.
func (b *building) homeDoorFrame(w, out vec3, half, h, deep float64, accent string) {
	along := vec3{-out.Z, 0, out.X}
	const jamb = 0.26
	trim := b.in("home.trim")
	for _, s := range []float64{-1, 1} {
		trim.Box(w.Add(along.Scale(s*(half+jamb/2))).Add(vec3{0, (h + jamb) / 2, 0}), vec3{jamb, h + jamb, deep}, along, 0.06)
	}
	trim.Box(w.Add(vec3{0, h + jamb/2, 0}), vec3{2 * half, jamb, deep}, along, 0.06)
	face := w.Sub(out.Scale(deep/2 + 0.002))
	for _, s := range []float64{-1, 1} {
		b.in(accent).Box(face.Add(along.Scale(s*(half+0.08))).Add(vec3{0, (h + 0.08) / 2, 0}), vec3{0.025, h + 0.08, 0.012}, along, 0)
	}
	b.in(accent).Box(face.Add(vec3{0, h + 0.08, 0}), vec3{2*half + 0.185, 0.025, 0.012}, along, 0)
}

// glassRails builds the guard rails of a living space: panes of glass
// between a base channel and an anthracite handrail, along the slab and
// the stairs' open sides, over their stringer.
func (b *building) glassRails(in ecs.ID) {
	for _, f := range ordered(Ents.Floor) {
		if f.In != in {
			continue
		}
		trim := b.in("home.trim")
		// Seen from inside on both sides: no room point (mesh.LitGlass).
		glass := b.Part("glass.rail", mesh.LitGlass, vec3{})
		for _, rl := range f.rails() {
			d := rl.B.Sub(rl.A)
			l := d.Length()
			mid := rl.A.Lerp(rl.B, 0.5)
			trim.Box(mid.Add(vec3{0, 0.05, 0}), vec3{l, 0.1, 0.07}, d, 0.015)
			n := max(1, int(math.Round(l/1.2)))
			for i := range n {
				p := rl.A.Lerp(rl.B, (float64(i)+0.5)/float64(n))
				glass.Box(p.Add(vec3{0, (0.1 + railHeight - 0.03) / 2, 0}), vec3{l/float64(n) - 0.012, railHeight - 0.13, 0.02}, d, 0)
			}
			trim.Box(mid.Add(vec3{0, railHeight, 0}), vec3{l + 0.08, 0.05, 0.08}, d, 0.02)
		}
		if f.Level() {
			continue
		}
		for _, side := range f.openSides(in) {
			a, e := f.From.Add(side), f.To.Add(side)
			d := e.Sub(a)
			dn := d.Norm()
			up := vec3{0, 1, 0}.Sub(dn.Scale(dn.Y)).Norm()
			across := side.Norm()
			glass.Slab([]vec3{a.Add(vec3{0, stringerRise, 0}), e.Add(vec3{0, stringerRise, 0}), e.Add(vec3{0, railHeight - 0.03, 0}), a.Add(vec3{0, railHeight - 0.03, 0})}, across, 0.02)
			trim.BoxBasis(a.Lerp(e, 0.5).Add(vec3{0, railHeight, 0}), [3]vec3{dn, up, across}, vec3{d.Length() + 0.08, 0.05, 0.08}, 0.02)
		}
	}
}

// stringerRise is how far the stairs' solid stringer rises over their
// slope.
const stringerRise = 0.12

// openSides are the sides of stairs f in building in that are away from
// the walls, from the middle of their width to their edge.
func (f Floor) openSides(in ecs.ID) []vec3 {
	r := Ents.Building.Get(in)
	_, _, across := f.frame()
	var out []vec3
	for _, s := range []float64{-1, 1} {
		side := across.Scale(s * (f.Width/2 - 0.05))
		if r != nil && r.againstWall(f.From.Add(side)) && r.againstWall(f.To.Add(side)) {
			continue
		}
		out = append(out, side)
	}
	return out
}

// homeStairs builds stairs f of living space in: white steps under a
// floor tread whose nosing overhangs a lit strip, and a solid stringer
// along each open side.
func (b *building) homeStairs(in ecs.ID, f Floor) {
	along, l, across := f.frame()
	lo, hi := f.From, f.To
	if lo.Y > hi.Y {
		lo, hi, along = hi, lo, along.Scale(-1)
	}
	n := max(1, int(math.Round((hi.Y-lo.Y)/0.2)))
	run := l / float64(n)
	const tread, nose = 0.03, 0.03
	for k := range n {
		top := lo.Y + (hi.Y-lo.Y)*float64(k+1)/float64(n)
		bottom := 0.0
		if f.Thick > 0 {
			bottom = math.Max(0, top-0.2-f.Thick)
		}
		mid := vec3{lo.X, 0, lo.Z}.Add(along.Scale((float64(k) + 0.5) * run))
		b.in("home.panel").Box(mid.Add(vec3{0, (top - tread + bottom) / 2, 0}), vec3{run, top - tread - bottom, f.Width}, along, 0.008)
		b.in("home.floor").Box(mid.Sub(along.Scale(nose/2)).Add(vec3{0, top - tread/2, 0}), vec3{run + nose, tread, f.Width}, along, 0.008)
		led := mid.Sub(along.Scale(run/2 - 0.006)).Add(vec3{0, top - tread - 0.012, 0})
		b.in("home.led").Box(led, vec3{0.012, 0.016, f.Width - 0.2}, along, 0)
	}
	for _, side := range f.openSides(in) {
		s := across.Scale(math.Copysign(f.Width/2+0.03, side.Dot(across)))
		a, e := vec3{lo.X, 0, lo.Z}.Add(s), vec3{hi.X, 0, hi.Z}.Add(s)
		rise := lo.Y + (hi.Y-lo.Y)/float64(n) + stringerRise
		b.in("home.panel2").Slab([]vec3{a, e, e.Add(vec3{0, hi.Y + stringerRise, 0}), a.Add(vec3{0, rise, 0})}, across, 0.06)
	}
}

// ringPendant hangs a ring of light of radius rad at, on three cables
// from the ceiling at height top: a white ring, lit underneath.
func (b *building) ringPendant(at vec3, rad, top float64) {
	const n = 32
	for k := range n {
		a0, a1 := float64(k)*2*math.Pi/n, float64(k+1)*2*math.Pi/n
		p0 := at.Add(vec3{math.Cos(a0), 0, math.Sin(a0)}.Scale(rad))
		p1 := at.Add(vec3{math.Cos(a1), 0, math.Sin(a1)}.Scale(rad))
		d := p1.Sub(p0)
		mid := p0.Lerp(p1, 0.5)
		b.in("home.rib").Box(mid, vec3{d.Length() + 0.01, 0.07, 0.12}, d, 0.02)
		b.in("home.led").Box(mid.Sub(vec3{0, 0.036, 0}), vec3{d.Length() + 0.01, 0.006, 0.07}, d, 0)
	}
	for k := range 3 {
		a := float64(k)*2*math.Pi/3 + 0.3
		p := at.Add(vec3{math.Cos(a), 0, math.Sin(a)}.Scale(rad))
		b.in("home.trim").Prism(p.Add(vec3{0, 0.035, 0}), yUp, 0.006, 0.006, top-at.Y-0.035, 4, 0)
	}
}

// footLight runs a line of light at the foot of the lining from a to b
// (as liningPanel), shining down on the floor.
func (b *building) footLight(a, e, nrm vec3, th float64, tok string) {
	d := e.Sub(a)
	face := a.Lerp(e, 0.5).Add(nrm.Scale(th + liningGap + lining)).Add(vec3{0, 0.09, 0})
	b.in("home.trim").Box(face.Add(nrm.Scale(0.011)).Add(vec3{0, 0.022, 0}), vec3{d.Length(), 0.02, 0.02}, d, 0)
	b.in(tok).Box(face.Add(nrm.Scale(0.008)), vec3{d.Length(), 0.02, 0.012}, d, 0)
}

// casingLight is a line of light under a duct casing, its middle at,
// running l along along: it washes the wall below.
func (b *building) casingLight(at, along vec3, l float64, tok string) {
	b.in(tok).Box(at.Sub(vec3{0, 0.004, 0}), vec3{l, 0.01, 0.05}, along, 0)
}
