package base

import (
	"fmt"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/mesh"
)

// A link (content.Link) goes from a building's port to another's: a glass
// tube on the ground, or a tunnel under it (tunnel.go). A tube is straight, with one bend, or with a small one as it leaves
// each port. It is laid out from the two ports in a frame of its own (its
// middle at the origin, +X toward the second port), never saved.
//
//	( dome )══▐ landing ▌── ramp ──(bend)── ramp ──▐ landing ▌══( dome )

// PortAt is a port of a building standing somewhere.
type PortAt struct {
	At, Out vec3    // in the world: the middle of its outer door, on the floor, and the way out
	Up      vec3    // its building's
	Landing float64 // how far out of it the ground is the building's, level
}

// PortAt is where port i of a building standing at s is, its ground flat
// margin m around its Reach.
func (c *Camp) PortAt(s Site, i int, margin float64) PortAt {
	p := c.Ports[i]
	return PortAt{s.ToWorld(p.At()), s.Dir(p.Out()), s.Y, math.Max(1.2, c.Reach+margin-p.At().Length()+0.1)}
}

// Link is a link laid out, in its own frame.
type Link struct {
	Def      *content.Link
	Site     Site
	Pts      []vec3 // the middle of its floor: at the ports, where they land, where it bends
	Length   float64
	Rise     float64 // how much higher an end is than the other
	Slope    float64 // of its ramp, 0.08 for 8 %
	Walls    []Segment
	Lights   []content.Light
	Key      string // the same for two links of the same shape
	parts    *mesh.Parts
	legs     []linkLeg
	sections []linkSection
	dug      *tunnel // under the ground
}

// linkLeg is a straight stretch between two points of a link, its floor's
// middle; pad is how much longer it is inside, at each end, for its bends.
type linkLeg struct {
	a, b vec3
	pad  [2]float64
}

// linkSection is a cut across the tube: the middle of its floor, the way
// across it, longer than 1 in a bend, and whether a frame stands there.
type linkSection struct {
	at, across vec3
	frame      bool
}

const (
	linkStraight = 8 * math.Pi / 180   // a bend too small to be worth one in the middle
	linkBendEnd  = 60 * math.Pi / 180  // the most a link bends leaving a port
	linkBend     = 100 * math.Pi / 180 // and in the middle
	linkStart    = 0.18                // how far out of a port it starts: its door's frame
	linkSkirt    = 0.35                // how far it goes into the ground
)

// level is v seen from above.
func level(v vec3) vec3 { return vec3{v.X, 0, v.Z} }

// turnAngle is the angle from a way to another, seen from above.
func turnAngle(a, b vec3) float64 {
	a, b = level(a).Norm(), level(b).Norm()
	return math.Acos(math.Max(-1, math.Min(1, a.Dot(b))))
}

// LayLink lays out a link between two ports, or tells why it can't be.
// ground is how far from Mars' center the ground is, in a direction, as it
// was before anything was built: a tunnel goes under it.
func LayLink(def *content.Link, from, to PortAt, ground func(dir vec3) float64) (*Link, string) {
	if def.Under {
		return layTunnel(def, from, to, ground)
	}
	if l := layTube(def, from, to); l != nil {
		return l, ""
	}
	return nil, "ces portes se tournent trop le dos"
}

// layTube lays out a tube; nil if it would bend too much.
func layTube(def *content.Link, from, to PortAt) *Link {
	o := from.At.Lerp(to.At, 0.5)
	up := o.Norm()
	x := to.At.Sub(from.At)
	if x = x.Sub(up.Scale(x.Dot(up))); x.Length() < 0.5 {
		return nil
	}
	x = x.Norm()
	l := &Link{Def: def, Site: Site{o, x, up, x.Cross(up)}}
	a, b := l.Site.ToLocal(from.At), l.Site.ToLocal(to.At)
	oa, ob := level(l.Site.DirLocal(from.Out)).Norm(), level(l.Site.DirLocal(to.Out)).Norm()
	// Level as the buildings are, where it lands.
	la, lb := l.Site.ToLocal(from.At.Add(from.Out.Scale(from.Landing))), l.Site.ToLocal(to.At.Add(to.Out.Scale(to.Landing)))
	if level(la).Sub(level(lb)).Length() < 0.5 { // face to face, closer than both landings
		m := a.Lerp(b, 0.5)
		la, lb = a.Lerp(m, 0.8), b.Lerp(m, 0.8)
	}
	mid := lb.Sub(la)
	ea, eb := turnAngle(oa, mid), turnAngle(mid, ob.Scale(-1))
	pts := []vec3{a, la, lb, b}
	if ea > linkStraight || eb > linkStraight {
		if k, ok := crossing(a, oa, b, ob, la, lb, def.Width/2); ok {
			pts = []vec3{a, la, k, lb, b}
		} else if ea > linkBendEnd || eb > linkBendEnd {
			return nil
		}
	}
	l.Pts = pts
	l.Rise = math.Abs(to.At.Length() - from.At.Length())
	ramp := 0.0
	for i := range len(pts) - 1 {
		d := level(pts[i+1].Sub(pts[i])).Length()
		l.Length += d
		if i > 0 && i < len(pts)-2 {
			ramp += d
		}
	}
	l.Slope = math.Abs(lb.Y-la.Y) / math.Max(0.1, ramp)
	for _, p := range pts {
		l.Key += fmt.Sprintf("%.3f,%.3f,%.3f ", p.X, p.Y, p.Z)
	}
	l.Key = def.ID + " " + l.Key
	l.cut()
	l.build()
	return l
}

// Parts are its meshes, built on first use: a tunnel is long.
func (l *Link) Parts() *mesh.Parts {
	if l.parts == nil {
		l.parts = l.dig()
	}
	return l.parts
}

// crossing is where the ways out of two ports at a and b meet, past where
// they land, the tube half wide; its height on the ramp.
func crossing(a, oa, b, ob, la, lb vec3, half float64) (vec3, bool) {
	// a + t oa = b + u ob, seen from above.
	d := level(b.Sub(a))
	det := oa.Z*ob.X - oa.X*ob.Z
	if math.Abs(det) < 0.02 {
		return vec3{}, false
	}
	t := (d.Z*ob.X - d.X*ob.Z) / det
	u := (oa.X*d.Z - oa.Z*d.X) / det
	bend := turnAngle(oa, ob.Scale(-1))
	need := half*math.Tan(bend/2) + 0.3
	ta, tb := t-level(la.Sub(a)).Length(), u-level(lb.Sub(b)).Length()
	if bend > linkBend || ta < need || tb < need || t+u > 1.5*d.Length() {
		return vec3{}, false
	}
	k := a.Add(oa.Scale(t))
	k.Y = la.Y + (lb.Y-la.Y)*ta/(ta+tb)
	return k, true
}

// cut cuts the tube into sections: at its ends and bends, and every Step
// or so between.
func (l *Link) cut() {
	half := l.Def.Width / 2
	n := len(l.Pts)
	dir := func(i int) vec3 { return level(l.Pts[i+1].Sub(l.Pts[i])).Norm() }
	side := func(d vec3) vec3 { return vec3{-d.Z, 0, d.X} }
	tanAt := func(i int) float64 { // of half the bend at point i
		if i == 0 || i == n-1 {
			return 0
		}
		return math.Tan(turnAngle(dir(i-1), dir(i)) / 2)
	}
	for i := range n - 1 {
		a, b := l.Pts[i], l.Pts[i+1]
		d := dir(i)
		long := level(b.Sub(a)).Length()
		ta, tb := tanAt(i)*half, tanAt(i+1)*half
		l.legs = append(l.legs, linkLeg{a, b, [2]float64{ta, tb}})
		start, end := ta, long-tb // clear of the inside of the bends
		if ta >= 0.3 {
			start += 0.25
		}
		if tb >= 0.3 {
			end -= 0.25
		}
		if i == 0 {
			start = linkStart
			l.sections = append(l.sections, linkSection{a.Add(b.Sub(a).Scale(linkStart / long)), side(d), true})
		} else { // the bend, cut halfway between the two ways
			was := dir(i - 1)
			m := side(was.Add(d).Norm())
			l.sections = append(l.sections, linkSection{a, m.Scale(1 / math.Max(0.5, m.Dot(side(d)))), true})
		}
		if i == n-2 {
			end = long - linkStart
		}
		steps := int(math.Max(1, math.Round((end-start)/l.Def.Step)))
		for k := range steps + 1 {
			if end-start < 0.4 {
				break
			}
			t := start + (end-start)*float64(k)/float64(steps)
			if k == 0 && ta < 0.3 || k == steps && tb < 0.3 {
				continue // the section at that end
			}
			l.sections = append(l.sections, linkSection{a.Add(b.Sub(a).Scale(t / long)), side(d), true})
		}
		if i == n-2 {
			l.sections = append(l.sections, linkSection{a.Add(b.Sub(a).Scale((long - linkStart) / long)), side(d), true})
		}
	}
}

// profile is half the tube's outline, from the top of its curb to the
// middle of its roof.
func (l *Link) profile() [3][2]float64 {
	w, h := l.Def.Width/2, l.Def.Height
	return [3][2]float64{{w, 0.3}, {w, h * 0.72}, {w * 0.6, h - 0.06}}
}

// build builds the tube's meshes, walls and lamps.
func (l *Link) build() {
	def := l.Def
	b := &building{Parts: mesh.NewParts(vec3{})}
	out := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitSun, vec3{}) }
	up := vec3{0, 1, 0}
	prof := l.profile()
	w, h := def.Width/2, def.Height
	pt := func(s linkSection, x, y float64) vec3 { return s.at.Add(s.across.Scale(x)).Add(up.Scale(y)) }
	// A piece from a section to the next, by its corners across the first.
	run := func(tok string, lit float32, s, e linkSection, xy ...[2]float64) {
		var pts []vec3
		for _, c := range xy {
			pts = append(pts, pt(s, c[0], c[1]), pt(e, c[0], c[1]))
		}
		b.Part(tok, lit, vec3{}).Hull(pts)
	}
	rect := func(x0, x1, y0, y1 float64) [][2]float64 { return [][2]float64{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} }

	for i := range len(l.sections) - 1 {
		s, e := l.sections[i], l.sections[i+1]
		// The floor and its curbs: as far as the doors at both ends.
		fs, fe := s, e
		if i == 0 {
			fs.at = l.Pts[0]
		}
		if i == len(l.sections)-2 {
			fe.at = l.Pts[len(l.Pts)-1]
		}
		run(def.Floor, mesh.LitSun, fs, fe, rect(-w+0.14, w-0.14, -linkSkirt, FloorTop)...)
		run(def.Frame, mesh.LitSun, fs, fe, rect(-0.04, 0.04, FloorTop, FloorTop+0.006)...)
		for _, x := range []float64{-1, 1} {
			lo, hi := math.Min(x*(w-0.14), x*(w+0.02)), math.Max(x*(w-0.14), x*(w+0.02))
			run(def.Curb, mesh.LitSun, s, e, rect(lo, hi, -linkSkirt, prof[0][1])...)
			in := x * (w - 0.145)
			run(def.Light, mesh.LitSun, s, e, rect(math.Min(in, x*(w-0.16)), math.Max(in, x*(w-0.16)), 0.12, 0.15)...)
			// The glass, a pane a side of the outline.
			for k := range 2 {
				p, q := prof[k], prof[k+1]
				run(def.Glass, mesh.LitGlass, s, e, [2]float64{x * p[0], p[1]}, [2]float64{x * q[0], q[1]},
					[2]float64{x * (p[0] - 0.02), p[1] - 0.01}, [2]float64{x * (q[0] - 0.02), q[1] - 0.01})
			}
		}
		top := prof[2]
		run(def.Glass, mesh.LitGlass, s, e, rect(-top[0], -0.15, top[1]-0.02, top[1])...)
		run(def.Glass, mesh.LitGlass, s, e, rect(0.15, top[0], top[1]-0.02, top[1])...)
		// The spine along the roof, its lamp under it.
		run(def.Frame, mesh.LitSun, s, e, rect(-0.16, 0.16, top[1]-0.03, h)...)
		run(def.Light, mesh.LitSun, s, e, rect(-0.1, 0.1, top[1]-0.05, top[1]-0.035)...)
	}
	// The frames, along the outline.
	for _, s := range l.sections {
		if !s.frame {
			continue
		}
		n := s.across.Norm().Cross(up)
		bar := func(p, q [2]float64) {
			a, c := pt(s, p[0], p[1]), pt(s, q[0], q[1])
			along := c.Sub(a)
			long := along.Length()
			along = along.Scale(1 / long)
			out(def.Frame).BoxBasis(a.Lerp(c, 0.5), [3]vec3{n, along, n.Cross(along)}, vec3{0.1, long + 0.05, 0.1}, 0.015)
		}
		for _, x := range []float64{-1, 1} {
			at := func(k int) [2]float64 { return [2]float64{x * prof[k][0], prof[k][1]} }
			bar([2]float64{x * w, -0.1}, at(1))
			bar(at(1), at(2))
		}
		bar([2]float64{-prof[2][0], prof[2][1]}, [2]float64{prof[2][0], prof[2][1]})
	}
	// The ends: what the tube leaves open of the doors' frames.
	for _, s := range []linkSection{l.sections[0], l.sections[len(l.sections)-1]} {
		n := s.across.Norm().Cross(up)
		for _, x := range []float64{-1, 1} {
			tri := []vec3{pt(s, x*prof[1][0], prof[1][1]), pt(s, x*prof[2][0], prof[2][1]), pt(s, x*prof[1][0], prof[2][1])}
			out(def.Frame).Slab(tri, n, 0.06)
		}
	}
	b.Unfight()
	l.parts = b.Parts

	for i := range len(l.sections) - 1 {
		s, e := l.sections[i], l.sections[i+1]
		for _, x := range []float64{-1, 1} {
			l.Walls = append(l.Walls, Segment{pt(s, x*w, 0), pt(e, x*w, 0), h})
		}
	}
	// What is left beside the doors.
	for _, s := range []linkSection{l.sections[0], l.sections[len(l.sections)-1]} {
		for _, x := range []float64{-1, 1} {
			l.Walls = append(l.Walls, Segment{pt(s, x*w, 0), pt(s, x*(DoorWidth+0.2)/2, 0), h})
		}
	}
	// A lamp every 6 m or so, at night.
	n := math.Max(1, math.Round(l.Length/6))
	for k := range int(n) {
		t := (float64(k) + 0.5) / n * l.Length
		for i := range len(l.Pts) - 1 {
			a, c := l.Pts[i], l.Pts[i+1]
			long := level(c.Sub(a)).Length()
			if t <= long {
				l.Lights = append(l.Lights, content.Light{At: a.Lerp(c, t/long).Add(up.Scale(h - 0.3)), Power: 0.8, Range: 9, Night: true})
				break
			}
			t -= long
		}
	}
}

// Ways are the rectangles of ground under its ramp, in the world: their
// middle, the way along, up, and half their sides.
func (l *Link) Ways(see func(at, x, up vec3, half [2]float64)) {
	if l.dug != nil {
		l.dug.ways(see)
		return
	}
	for _, g := range l.legs[1 : len(l.legs)-1] {
		d := g.b.Sub(g.a)
		long := d.Length()
		x := d.Scale(1 / long)
		z := level(d).Norm().Cross(vec3{0, 1, 0})
		see(l.Site.ToWorld(g.a.Lerp(g.b, 0.5)), l.Site.Dir(x), l.Site.Dir(z.Cross(x).Norm()), [2]float64{long/2 + 0.2, l.Def.Width/2 + 0.2})
	}
}

// Path goes through points of its floor, one every half metre, where it
// stands on the ground: but the shelters of a tunnel, none.
func (l *Link) Path(see func(p vec3)) {
	if l.dug != nil {
		l.dug.path(func(at vec3) { see(l.Site.ToLocal(at)) })
		return
	}
	for _, g := range l.legs {
		n := int(math.Ceil(g.b.Sub(g.a).Length() / 0.5))
		for k := range n + 1 {
			see(g.a.Lerp(g.b, float64(k)/float64(n)))
		}
	}
}

// Legs are its straight stretches on the ground, seen from above: their
// corners, at the floor's height.
func (l *Link) Legs(see func(corners [4]vec3)) {
	if l.dug != nil {
		l.dug.legs(func(c [4]vec3) {
			for i := range c {
				c[i] = l.Site.ToLocal(c[i])
			}
			see(c)
		})
		return
	}
	for _, g := range l.legs {
		s := vec3{0, 1, 0}.Cross(level(g.b.Sub(g.a)).Norm()).Scale(l.Def.Width / 2)
		see([4]vec3{g.a.Add(s), g.b.Add(s), g.b.Sub(s), g.a.Sub(s)})
	}
}

// in is a point of the frame in a leg's own: along it, over its floor and
// across.
func (g linkLeg) in(p vec3) (t, y, z, long float64) {
	d := level(g.b.Sub(g.a))
	long = d.Length()
	d = d.Scale(1 / long)
	off := p.Sub(g.a)
	t = off.Dot(d)
	return t, off.Y - (g.b.Y-g.a.Y)*t/long, off.X*-d.Z + off.Z*d.X, long
}

// Indoors tells whether a point of its frame is in it.
func (l *Link) Indoors(p vec3) bool {
	if l.dug != nil {
		return l.dug.holds(l.Site.ToWorld(p))
	}
	for _, g := range l.legs {
		t, y, z, long := g.in(p)
		if t > -0.05-g.pad[0] && t < long+0.05+g.pad[1] && y > -0.5 && y < l.Def.Height && math.Abs(z) < l.Def.Width/2 {
			return true
		}
	}
	return false
}

// Ray is how far along a ray of its frame from o along d (length 1), within
// max, it meets the tube from outside.
func (l *Link) Ray(o, d vec3, max float64) (float64, bool) {
	best, ok := max, false
	if l.Indoors(o) {
		return 0, false
	}
	if l.dug != nil {
		return l.dug.ray(l.Site.ToWorld(o), l.Site.Dir(d), max)
	}
	for _, g := range l.legs {
		t0, y0, z0, long := g.in(o)
		t1, y1, z1, _ := g.in(o.Add(d))
		in, out := 0.0, best
		for _, s := range [][4]float64{{t0, t1 - t0, 0, long}, {y0, y1 - y0, -0.2, l.Def.Height}, {z0, z1 - z0, -l.Def.Width / 2, l.Def.Width / 2}} {
			if math.Abs(s[1]) < 1e-9 {
				if s[0] < s[2] || s[0] > s[3] {
					out = -1
				}
				continue
			}
			a, b := (s[2]-s[0])/s[1], (s[3]-s[0])/s[1]
			in, out = math.Max(in, math.Min(a, b)), math.Min(out, math.Max(a, b))
		}
		if in < out && in < best {
			best, ok = in, true
		}
	}
	return best, ok
}

// SampleLink is a link of a kind alone, 6 m long, for its picture.
func SampleLink(def *content.Link) *Link {
	const r = 3000.0
	if def.Under {
		l, _ := layTunnel(def, PortAt{vec3{-14, r, 0}, vec3{1, 0, 0}, vec3{0, 1, 0}, 1.2}, PortAt{vec3{14, r, 0}, vec3{-1, 0, 0}, vec3{0, 1, 0}, 1.2},
			func(vec3) float64 { return r })
		return l
	}
	return layTube(def, PortAt{vec3{-3, r, 0}, vec3{1, 0, 0}, vec3{0, 1, 0}, 1.2}, PortAt{vec3{3, r, 0}, vec3{-1, 0, 0}, vec3{0, 1, 0}, 1.2})
}

// Blocks tells whether something at a point of the camp's frame would be
// within m of its walls, or inside; but the airlock of port skip.
func (c *Camp) Blocks(p vec3, m float64, skip int) bool {
	r := c.Room
	if r.Hall != nil && r.inHall(level(p), m) || r.Hall == nil && math.Hypot(p.X, p.Z) < r.Radius+m {
		return true
	}
	if mid, w, ok := c.BigDoor(); ok { // its way in
		if d := p.Sub(mid); d.X > 0 && d.X < 6+m && math.Abs(d.Z) < w/2+m {
			return true
		}
	}
	for i, port := range c.Ports {
		a := port.Airlock
		d := a.To.Sub(a.From)
		long := d.Length()
		off := level(p).Sub(level(a.From))
		// Up to before its door: to be able to go out.
		if t := off.Dot(d) / long; i != skip && t > -m && t < long+1+m && math.Abs(off.Dot(a.side())) < a.Width/2+WallThick+m {
			return true
		}
	}
	return false
}
