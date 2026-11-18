package base

import (
	"math"
	"naturalia/internal/mesh"
)

// Home interiors (interior: "home" on a blueprint): the living spaces are
// finished like a well-made Mars habitat, inside only. Off-white composite
// panels with fine joints line the walls and the roof, a warm grey floor
// has a lighter path from each doorway to the middle, the arches are
// smooth white ribs and the ducts run boxed in along the ceiling. The
// shell outside doesn't change. Its finishes are tokens (home.*): a mod
// can repaint them.

const (
	lining    = 0.05 // the panels' thickness
	liningGap = 0.02 // between them and the wall: not flush, they would flicker
	// The panels are about this wide and tall, a fine joint between them.
	homePanelW, homePanelH, homeJoint = 1.6, 2.6, 0.008
)

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

// floorTok is the token of the room's floor.
func (r Room) floorTok() string {
	if r.home() {
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
	arch := func(u float64) vec3 { return vec3{u, r.vault(u), 0} }
	for i := range steps {
		u0, u1 := -hx+2*hx*float64(i)/steps, -hx+2*hx*float64(i+1)/steps
		a, e := arch(u0), arch(u1)
		along := e.Sub(a)
		l := along.Length()
		along = along.Scale(1 / l)
		nrm := Z.Cross(along) // up and out
		mid := c.Add(a.Lerp(e, 0.5)).Sub(nrm.Scale(liningGap + lining/2))
		n := max(1, int(math.Round(2*(hz-in)/homePanelH)))
		w := 2 * (hz - in) / float64(n)
		for k := range n {
			z := -hz + in + (float64(k)+0.5)*w
			b.in("home.panel").BoxBasis(mid.Add(vec3{0, 0, z}), [3]vec3{along, nrm, Z}, vec3{l - homeJoint, lining, w - homeJoint}, 0.012)
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
// and ceiling, a duct casing along the top of each wall, the floor and its
// path, and the lamp strip lowered under the lining.
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
	}
	b.in("home.panel").Box(mid.Add(vec3{0, c.Height - liningGap - lining/2, 0}), vec3{length, lining, c.Width - 2*in}, dir, 0)
	b.in(c.floorTok()).Box(mid.Add(vec3{0, FloorTop - 0.05, 0}), vec3{length, 0.1, c.Width}, dir, 0)
	b.in("home.path").Box(mid.Add(vec3{0, FloorTop - 0.01, 0}), vec3{length, 0.02, 1.0}, dir, 0)
	drop := c.Height - in
	b.in("steel").Box(mid.Add(vec3{0, drop - 0.03, 0}), vec3{length * 0.72, 0.06, 0.34}, dir, 0.015)
	b.in("lamp.warm").Box(mid.Add(vec3{0, drop - 0.07, 0}), vec3{length * 0.7, 0.03, 0.25}, dir, 0)
}

// floorTok is the token of the corridor's floor.
func (c Corridor) floorTok() string {
	if c.home() {
		return "home.floor"
	}
	return "floor.slate"
}
