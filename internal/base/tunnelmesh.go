package base

import (
	"fmt"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/engine/mathx"
	"naturalia/internal/engine/mesh"
)

// A tunnel's meshes: its inside, lit by its own lamps, in the link's frame;
// and the outside of its shelters, in their own.

const (
	tunnelLamp = 6.0 // m between two lamps
	rockEvery  = 9   // joints of the gallery from a patch of bare rock to the next
	rockLong   = 3   // and of one
)

// profile is half the vault's outline, from its floor to its top.
func profile(def *content.Link) [][2]float64 {
	w, h := def.Width/2, def.Height
	return [][2]float64{{w, 0}, {w, 0.55 * h}, {0.87 * w, 0.75 * h}, {0.62 * w, 0.89 * h}, {0.3 * w, 0.975 * h}, {0, h}}
}

// vaultAt is how high the vault is, x m from its middle.
func vaultAt(def *content.Link, x float64) float64 {
	prof := profile(def)
	for k := len(prof) - 1; k > 0; k-- {
		if p, q := prof[k-1], prof[k]; x <= p[0] && p[0] > q[0] {
			return q[1] + (p[1]-q[1])*(x-q[0])/(p[0]-q[0])
		}
	}
	return prof[1][1]
}

// tunnelMesh builds in the frame of a link.
type tunnelMesh struct {
	*building
	l   *Link
	def *content.Link
	t   *tunnel
}

// pt is a point of a cut, across and over its floor, along it from there.
func (m *tunnelMesh) pt(s tunnelSection, x, y, along float64) vec3 {
	n := s.across.Cross(s.up).Norm().Scale(-1)
	return m.l.Site.ToLocal(s.at.Add(s.across.Scale(x)).Add(s.up.Scale(y)).Add(n.Scale(along)))
}

// run is a piece from a cut to the next, by its corners across them.
func (m *tunnelMesh) run(tok string, s, e tunnelSection, xy ...[2]float64) {
	var pts []vec3
	for _, c := range xy {
		pts = append(pts, m.pt(s, c[0], c[1], 0), m.pt(e, c[0], c[1], 0))
	}
	m.in(tok).Hull(pts)
}

func rect(x0, x1, y0, y1 float64) [][2]float64 {
	return [][2]float64{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}}
}

// arch is a rib along the vault at a cut: from deep m outside of its
// outline to deep m inside, thick m along.
func (m *tunnelMesh) arch(tok string, s tunnelSection, from, to, thick float64) {
	prof := profile(m.def)
	w, h := m.def.Width/2, m.def.Height
	in := func(p [2]float64, d float64) [2]float64 { return [2]float64{p[0] * (1 - d/w), p[1] * (1 - d/h)} }
	for _, x := range []float64{-1, 1} {
		for k := range len(prof) - 1 {
			var pts []vec3
			for _, p := range [][2]float64{in(prof[k], from), in(prof[k], to), in(prof[k+1], from), in(prof[k+1], to)} {
				pts = append(pts, m.pt(s, x*p[0], p[1], -thick/2), m.pt(s, x*p[0], p[1], thick/2))
			}
			m.in(tok).Hull(pts)
		}
	}
}

// rocky tells whether the gallery's stretch from node i on has bare rock
// for walls.
func (t *tunnel) rocky(i int) bool {
	return i >= 4 && i < len(t.nodes)-5 && i%rockEvery >= rockEvery-rockLong
}

// dig builds a tunnel's inside.
func (l *Link) dig() *mesh.Parts {
	m := &tunnelMesh{&building{Parts: mesh.NewParts(vec3{})}, l, l.Def, l.dug}
	t, def := m.t, m.def
	w := def.Width / 2
	prof := profile(def)
	first := 0 // the gallery's first section
	for i, s := range t.sections {
		if s.head < 0 {
			first = i
			break
		}
	}
	for i := range len(t.sections) - 1 {
		s, e := t.sections[i], t.sections[i+1]
		gallery := s.head < 0 && e.head < 0
		rock := gallery && t.rocky(i-first)
		for _, x := range []float64{-1, 1} {
			for k := range len(prof) - 1 {
				if rock && k == 0 {
					m.rock(s, e, x, i)
					continue
				}
				p, q := prof[k], prof[k+1]
				m.run(def.Wall, s, e, [2]float64{x * p[0], p[1]}, [2]float64{x * q[0], q[1]},
					[2]float64{x * p[0] * 1.04, p[1] * 1.02}, [2]float64{x * q[0] * 1.04, q[1]*1.02 + 0.02})
			}
			if !rock {
				lo, hi := math.Min(x*(w-0.03), x*(w+0.02)), math.Max(x*(w-0.03), x*(w+0.02))
				m.run(def.Curb, s, e, rect(lo, hi, -0.1, 0.3)...)
			}
		}
		// The floor: level in the shelters and the gallery, steps between.
		fs, fe := s, e
		switch {
		case s.head == 0 && s.x == linkStart:
			fs.at = t.heads[0].floor(0)
		case e.head == 1 && e.x == linkStart:
			fe.at = t.heads[1].floor(0)
		case !gallery:
			m.cable(s, e)
			continue
		}
		wide := w
		if rock {
			wide += 0.35
		}
		m.run(def.Floor, fs, fe, rect(-wide, wide, -0.15, FloorTop)...)
		if gallery {
			m.run(def.Frame, s, e, rect(-0.55, -0.51, FloorTop, FloorTop+0.006)...)
			m.run(def.Frame, s, e, rect(0.51, 0.55, FloorTop, FloorTop+0.006)...)
		}
		m.cable(s, e)
	}
	// Its ribs: at each joint of the gallery; heavier ones, with a line of
	// light, at the feet and the heads of the stairs.
	for i, s := range t.sections {
		switch {
		case s.head >= 0 && s.x == linkStart:
		case s.head >= 0 && s.x == headLanding, i == first, s.head < 0 && t.sections[i+1].head >= 0:
			m.arch(def.Frame, s, -0.05, 0.09, 0.3)
			m.arch(def.Light, s, 0.085, 0.095, 0.05)
		case s.head < 0:
			m.arch(def.Frame, s, -0.05, 0.05, 0.16)
		}
	}
	for _, h := range t.heads {
		m.stairs(h)
		m.front(h)
	}
	m.lamps()
	m.Unfight()
	return m.Parts
}

// cable is the tray along a wall.
func (m *tunnelMesh) cable(s, e tunnelSection) {
	w, h := m.def.Width/2, m.def.Height
	m.run(m.def.Curb, s, e, rect(0.97*w-0.2, 0.97*w, 0.5*h, 0.5*h+0.04)...)
	m.run(m.def.Pipe, s, e, rect(0.97*w-0.17, 0.97*w-0.11, 0.5*h+0.03, 0.5*h+0.09)...)
	m.run(m.def.Curb, s, e, rect(0.97*w-0.08, 0.97*w-0.04, 0.5*h+0.03, 0.5*h+0.07)...)
}

// rock is the bare rock of a stretch's wall: its facets, uneven.
func (m *tunnelMesh) rock(s, e tunnelSection, x float64, seed int) {
	w, h := m.def.Width/2, m.def.Height
	const rows, cols = 3, 4
	at := func(i, j int) vec3 {
		k := float64(i) / cols
		y := -0.1 + (0.55*h+0.12)*float64(j)/rows
		out := 0.0
		if i > 0 && i < cols {
			out = -0.06 + 0.34*mathx.Hash3(i, j, int(x), uint64(seed*3))
			k += 0.1 * (mathx.Hash3(i, j, int(x), uint64(seed*3+1)) - 0.5)
		}
		if j > 0 && j < rows {
			y += 0.16 * (mathx.Hash3(i, j, int(x), uint64(seed*3+2)) - 0.5)
		}
		a, b := m.pt(s, x*(w+0.05+out), y, 0), m.pt(e, x*(w+0.05+out), y, 0)
		return a.Lerp(b, k)
	}
	side := m.l.Site.DirLocal(s.across).Norm().Scale(x * 0.3)
	for i := range cols {
		for j := range rows {
			a, b, c, d := at(i, j), at(i+1, j), at(i+1, j+1), at(i, j+1)
			for _, tri := range [][3]vec3{{a, b, c}, {a, c, d}} {
				m.in(m.def.Rock).Hull([]vec3{tri[0], tri[1], tri[2], tri[0].Add(side), tri[1].Add(side), tri[2].Add(side)})
			}
		}
	}
}

// stairs builds a head's steps and their rails.
func (m *tunnelMesh) stairs(h tunnelHead) {
	w := m.def.Width / 2
	pt := func(x, y, z float64) vec3 { return m.l.Site.ToLocal(h.ToWorld(vec3{x, y, z})) }
	box := func(tok string, x0, x1, y0, y1, z0, z1 float64) {
		var pts []vec3
		for _, x := range []float64{x0, x1} {
			for _, y := range []float64{y0, y1} {
				pts = append(pts, pt(x, y, z0), pt(x, y, z1))
			}
		}
		m.in(tok).Hull(pts)
	}
	rise := h.drop / float64(h.steps)
	for k := range h.steps {
		x, y := headLanding+float64(k)*stepRun, -float64(k+1)*rise+FloorTop
		box(m.def.Floor, x, x+stepRun+0.01, y-0.3, y, -w, w)
		box(m.def.Frame, x+0.005, x+0.04, y-0.01, y+0.006, -w+0.06, w-0.06)
	}
	for _, s := range []float64{-1, 1} {
		z := s * (w - 0.09)
		a, b := vec3{headLanding - 0.2, 0.9, z}, vec3{h.run(), 0.9 - h.drop, z}
		wa, wb := pt(a.X, a.Y, a.Z), pt(b.X, b.Y, b.Z)
		d := wb.Sub(wa)
		m.in(m.def.Curb).Prism(wa, d, 0.025, 0.025, d.Length(), 8, 0)
		for k := 0.1; k < 1; k += 0.2 {
			p := a.Lerp(b, k)
			m.in(m.def.Curb).Hull([]vec3{pt(p.X-0.015, p.Y-0.05, z), pt(p.X+0.015, p.Y-0.05, z), pt(p.X-0.015, p.Y, z), pt(p.X+0.015, p.Y, z),
				pt(p.X-0.015, p.Y-0.05, s*w), pt(p.X+0.015, p.Y-0.05, s*w), pt(p.X-0.015, p.Y, s*w), pt(p.X+0.015, p.Y, s*w)})
		}
	}
}

// front closes a shelter around its port's door, seen from inside.
func (m *tunnelMesh) front(h tunnelHead) {
	prof := profile(m.def)
	half := (DoorWidth + 0.2) / 2
	top := math.Min(DoorHeight+0.15, vaultAt(m.def, half))
	slab := func(zy ...[2]float64) {
		var pts []vec3
		for _, c := range zy {
			for _, x := range []float64{linkStart - 0.02, linkStart + 0.05} {
				pts = append(pts, m.l.Site.ToLocal(h.ToWorld(vec3{x, c[1], c[0]})))
			}
		}
		m.in(m.def.Frame).Hull(pts)
	}
	for _, s := range []float64{-1, 1} {
		side := [][2]float64{{s * half, 0}, {s * half, vaultAt(m.def, half)}}
		over := [][2]float64{{s * half, top}, {s * half, vaultAt(m.def, half)}, {0, top}, {0, m.def.Height}}
		for _, p := range prof {
			if p[0] > half {
				side = append(side, [2]float64{s * p[0], p[1]})
			} else {
				over = append(over, [2]float64{s * p[0], p[1]})
			}
		}
		slab(side...)
		slab(over...)
	}
}

// lamps are the bars of light along the vault's top.
func (m *tunnelMesh) lamps() {
	for _, p := range m.t.lampSpots() {
		m.run(m.def.Curb, p[0], p[1], rect(-0.16, 0.16, m.def.Height-0.09, m.def.Height+0.01)...)
		m.run(m.def.Light, p[0], p[1], rect(-0.12, 0.12, m.def.Height-0.1, m.def.Height-0.08)...)
	}
}

// lampSpots are where its lamps hang, from a cut to another: one in each
// shelter, two over each stair, then one every tunnelLamp m or so.
func (t *tunnel) lampSpots() [][2]tunnelSection {
	var out [][2]tunnelSection
	spot := func(s, e tunnelSection, from, to float64) {
		at := func(k float64) tunnelSection {
			return tunnelSection{at: s.at.Lerp(e.at, k), across: s.across.Norm(), up: s.up}
		}
		a, b := at(from), at(to)
		b.across = a.across
		out = append(out, [2]tunnelSection{a, b})
	}
	for _, h := range t.heads {
		for _, x := range []float64{0.7, headLanding + (h.run()-headLanding)*0.3, headLanding + (h.run()-headLanding)*0.75} {
			s := tunnelSection{at: h.floor(x - 0.35), across: h.Z, up: h.Y}
			e := s
			e.at = h.floor(x + 0.35)
			spot(s, e, 0, 1)
		}
	}
	last := t.nodes[len(t.nodes)-1].s
	n := math.Max(1, math.Round(last/tunnelLamp))
	for k := range int(n) {
		at := (float64(k) + 0.5) / n * last
		for i := range len(t.nodes) - 1 {
			a, b := t.nodes[i], t.nodes[i+1]
			if at > b.s {
				continue
			}
			long := b.s - a.s
			mid := math.Max(0.5/long, math.Min(1-0.5/long, (at-a.s)/long)) // clear of the ribs
			if long < 1 {
				mid = 0.5
			}
			half := math.Min(0.4, long/2-0.1) / long
			d := b.at.Sub(a.at)
			up := a.at.Add(b.at).Norm()
			s := tunnelSection{at: a.at, across: d.Cross(up).Norm(), up: up}
			e := s
			e.at = b.at
			spot(s, e, mid-half, mid+half)
			break
		}
	}
	return out
}

// Lamps are a tunnel's, in the world.
func (l *Link) Lamps() []Lamp {
	if l.dug == nil {
		return nil
	}
	if l.lamps == nil {
		for i, p := range l.dug.lampSpots() {
			at := p[0].at.Lerp(p[1].at, 0.5).Add(p[0].up.Scale(l.Def.Height - 0.45))
			lamp := Lamp{at, 0.75, 9, 0}
			if i%3 == 0 && i < 6 { // a shelter's: not through its building's wall
				lamp.Power, lamp.Range = 0.6, 5
			} else if i%3 == 1 && i < 6 {
				lamp.Range = 6.5
			}
			l.lamps = append(l.lamps, lamp)
		}
	}
	return l.lamps
}

// Shell is the outside of a tunnel's shelter, in its frame; Key is the same
// for two alike.
type Shell struct {
	Site  Site
	Key   string
	Parts func() *mesh.Parts
}

// Shells are the outsides of a tunnel's shelters.
func (l *Link) Shells() []Shell {
	if l.dug == nil {
		return nil
	}
	var out []Shell
	for _, h := range l.dug.heads {
		out = append(out, Shell{h.Site, fmt.Sprintf("%s %.3f", l.Def.ID, h.slope()), func() *mesh.Parts { return l.dug.shell(h) }})
	}
	return out
}

// shell builds the outside of a shelter.
func (t *tunnel) shell(h tunnelHead) *mesh.Parts {
	b := &building{Parts: mesh.NewParts(vec3{})}
	w, deep := t.def.Width/2, 0.6
	out := w + headWall
	knee := headLanding + (t.def.Height+headRoof-headCurb)/h.slope() // where its roof gets level
	hull := func(tok string, z0, z1 float64, xy ...[2]float64) {
		var pts []vec3
		for _, c := range xy {
			pts = append(pts, vec3{c[0], c[1], z0}, vec3{c[0], c[1], z1})
		}
		b.out(tok).Hull(pts)
	}
	top := func(x float64) float64 { return t.roofTop(h, x) }
	const start = -0.04
	for _, s := range []float64{-1, 1} {
		a, c := math.Min(s*(w+0.01), s*out), math.Max(s*(w+0.01), s*out)
		hull(t.def.Wall, a, c, [2]float64{start, -deep}, [2]float64{start, top(0)}, [2]float64{headLanding, top(0)}, [2]float64{knee, headCurb}, [2]float64{knee, -deep})
		hull(t.def.Wall, a, c, [2]float64{knee, -deep}, [2]float64{knee, headCurb}, [2]float64{headEnd, headCurb}, [2]float64{headEnd, -deep})
		// A dark foot, and a white edge along the roof.
		f0, f1 := math.Min(s*out, s*(out+0.06)), math.Max(s*out, s*(out+0.06))
		hull(t.def.Curb, f0, f1, [2]float64{start, -deep}, [2]float64{start, 0.24}, [2]float64{headEnd + 0.06, 0.24}, [2]float64{headEnd + 0.06, -deep})
		e0, e1 := math.Min(s*(out-0.12), s*(out+0.08)), math.Max(s*(out-0.12), s*(out+0.08))
		edge := func(x0, x1 float64) {
			hull(t.def.Frame, e0, e1, [2]float64{x0, top(x0) - 0.1}, [2]float64{x0, top(x0) + 0.06}, [2]float64{x1, top(x1) - 0.1}, [2]float64{x1, top(x1) + 0.06})
		}
		edge(start, headLanding)
		edge(headLanding, knee-0.1)
	}
	roof := func(x0, x1 float64) {
		hull(t.def.Wall, -out+0.01, out-0.01, [2]float64{x0, top(x0) - 0.12}, [2]float64{x0, top(x0) - 0.005}, [2]float64{x1, top(x1) - 0.12}, [2]float64{x1, top(x1) - 0.005})
	}
	roof(start, headLanding)
	roof(headLanding, knee)
	hull(t.def.Wall, -out+0.01, out-0.01, [2]float64{knee, headCurb - 0.12}, [2]float64{knee, headCurb - 0.005}, [2]float64{headEnd - 0.01, headCurb - 0.12}, [2]float64{headEnd - 0.01, headCurb - 0.005})
	// Its end, and what is over the door.
	hull(t.def.Wall, -out+0.01, out-0.01, [2]float64{headEnd - 0.15, -deep}, [2]float64{headEnd - 0.15, headCurb - 0.02}, [2]float64{headEnd - 0.005, -deep}, [2]float64{headEnd - 0.005, headCurb - 0.02})
	hull(t.def.Curb, -out, out, [2]float64{headEnd, -deep}, [2]float64{headEnd, 0.24}, [2]float64{headEnd + 0.06, -deep}, [2]float64{headEnd + 0.06, 0.24})
	hull(t.def.Frame, -out+0.01, out-0.01, [2]float64{start - 0.03, 2.7}, [2]float64{start - 0.03, top(0) + 0.06}, [2]float64{start, 2.7}, [2]float64{start, top(0) + 0.06})
	// Ribs across the slope, and a line of amber light at its foot.
	for k := range 3 {
		x := headLanding + (knee-headLanding)*(0.2+0.25*float64(k))
		hull(t.def.Frame, -out+0.14, out-0.14, [2]float64{x, top(x) - 0.02}, [2]float64{x, top(x) + 0.025}, [2]float64{x + 0.14, top(x+0.14) - 0.02}, [2]float64{x + 0.14, top(x+0.14) + 0.025})
	}
	hull(t.def.Light, -out+0.3, out-0.3, [2]float64{headEnd - 0.5, headCurb - 0.01}, [2]float64{headEnd - 0.5, headCurb + 0.01}, [2]float64{headEnd - 0.45, headCurb - 0.01}, [2]float64{headEnd - 0.45, headCurb + 0.01})
	b.Unfight()
	return b.Parts
}
