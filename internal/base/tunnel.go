package base

import (
	"fmt"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/planet"
)

// A tunnel (content.Link, Under) goes from a port to another under the
// ground. At each port, a shelter, a wedge of concrete over a stairwell;
// between the feet of the stairs, a vaulted gallery, made of straight
// stretches along a way that leaves each stair straight ahead and turns in
// arcs. Its floor keeps under the ground, its slopes gentle.
//
//	( dome )══▐▔▔▔╲_____________________________╱▔▔▔▌══( dome )
//	               ╲                           ╱
//	                ╲_________________________╱
//
// Everything is figured in the world, then told in the link's frame. The
// heights are radii, from Mars' center: up is away from it all along.

const (
	headLanding = 1.2  // m of level floor between a port and the stairs
	stepRise    = 0.18 // of a stair's step
	stepRun     = 0.26
	headWall    = 0.2  // of the shelter's walls
	headRoof    = 0.25 // and roof
	headCurb    = 0.28 // how high the shelter is where its roof goes under
	headEnd     = 6.6  // how far out of its port it ends
	headGround  = 6.8  // how far the ground is made level
	headDeeper  = 8.0  // m of stairs more, at most, to get under the ground
	tunnelTurn  = 6.0  // radius of the gallery's turns
	tunnelLead  = 1.5  // straight on at the foot of the stairs
	tunnelApart = 1.2  // m of rock between two parts of it, beside its width
	tunnelLow   = 0.2  // how much lower than its vault's middle it is for a head
)

// tunnel is a link dug under the ground.
type tunnel struct {
	def      *content.Link
	heads    [2]tunnelHead
	nodes    []tunnelNode // the gallery's: the feet of the stairs at both ends
	sections []tunnelSection
	digs     *planet.Digs
	hollows  []planet.Hollow
}

// tunnelHead is a shelter and its stairs, in a frame of its own: from the
// port's outer door, X out, Y up.
type tunnelHead struct {
	Site
	landing float64 // where the ground stops being its building's
	drop    float64 // from the port's floor to the gallery's
	steps   int
}

// run is how far out of the port the stairs end; slope, how steep they are.
func (h tunnelHead) run() float64   { return headLanding + float64(h.steps)*stepRun }
func (h tunnelHead) slope() float64 { return h.drop / (float64(h.steps) * stepRun) }

// floor is the middle of its floor, x m out of the port.
func (h tunnelHead) floor(x float64) vec3 {
	return h.ToWorld(vec3{x, -math.Max(0, math.Min(x, h.run())-headLanding) * h.slope(), 0})
}

// deepen makes the stairs go down drop m.
func (h *tunnelHead) deepen(drop float64) {
	h.drop = drop
	h.steps = int(math.Ceil(drop/stepRise - 1e-6))
}

// tunnelNode is a joint of the gallery: the middle of its floor.
type tunnelNode struct {
	at vec3
	s  float64 // how far from the first
}

// tunnelSection is a cut across the tunnel: upright, from the middle of its
// floor, the way across it longer than 1 in a bend; head is whose stairs or
// shelter it stands in, x how far out of the port; -1 in the gallery.
type tunnelSection struct {
	at, across, up vec3
	head           int
	x              float64
}

// tunnelWall is a wall underground: its ends, and between which radii.
type tunnelWall struct {
	A, B      vec3
	Low, High float64
}

// layTunnel lays out a tunnel.
func layTunnel(def *content.Link, from, to PortAt, ground func(dir vec3) float64) (*Link, string) {
	t := &tunnel{def: def}
	for i, p := range []PortAt{from, to} {
		t.heads[i] = tunnelHead{Site: Site{p.At, p.Out, p.Up, p.Out.Cross(p.Up)}, landing: p.Landing}
		t.heads[i].deepen(def.Depth)
	}
	o := from.At.Lerp(to.At, 0.5)
	up := o.Norm()
	x := planet.TangentAt(up, to.At.Sub(from.At))
	if from.At.Sub(to.At).Length() < 0.5 {
		return nil, "trop court"
	}
	l := &Link{Def: def, Site: Site{o, x, up, x.Cross(up)}, dug: t}

	// The stairs go as deep as it takes for the gallery to be under the
	// ground from their foot on: where that is moves with them.
	for try := 0; ; try++ {
		line, ok := l.way()
		if !ok {
			return nil, "pas la place de tourner"
		}
		deeper := t.slopes(line, l.Site, ground)
		if deeper[0] < 0.01 && deeper[1] < 0.01 {
			break
		}
		for i, d := range deeper {
			if d > 0 {
				t.heads[i].deepen(t.heads[i].drop + d + 0.02)
			}
		}
		if t.heads[0].drop > def.Depth+headDeeper || t.heads[1].drop > def.Depth+headDeeper {
			return nil, "trop de dénivelé"
		}
		if try > 12 {
			return nil, "sol trop accidenté"
		}
	}
	last := t.nodes[len(t.nodes)-1].s
	l.Length = t.heads[0].run() + last + t.heads[1].run()
	l.Rise = math.Abs(from.At.Length() - to.At.Length())
	t.cut()
	t.hollow()
	for _, s := range t.sections {
		p := l.Site.ToLocal(s.at)
		l.Pts = append(l.Pts, p)
		l.Key += fmt.Sprintf("%.3f,%.3f,%.3f ", p.X, p.Y, p.Z)
	}
	l.Key = def.ID + " " + l.Key
	return l, ""
}

// plan is where a point of the world is on the link's plane, seen from
// Mars' center: straight lines on it are straight on the ground.
func (l *Link) plan(p vec3) (x, z float64) {
	p = l.Site.DirLocal(p)
	k := l.Site.Origin.Length() / p.Y
	return p.X * k, p.Z * k
}

// unplan is the direction from Mars' center through a point of the plane.
func (l *Link) unplan(x, z float64) vec3 {
	return l.Site.Dir(vec3{x, l.Site.Origin.Length(), z}).Norm()
}

// way is the gallery's, on the plane, from the foot of a stair to the
// other's: the shortest whose parts keep apart; none if they all cross.
func (l *Link) way() ([][2]float64, bool) {
	t := l.dug
	var foot, lead [2][2]float64
	var turn [2]float64 // the headings, from the first stairs, to the second
	for i, h := range t.heads {
		f := h.floor(h.run())
		fx, fz := l.plan(f)
		px, pz := l.plan(f.Add(h.X.Scale(tunnelLead)))
		foot[i], lead[i] = [2]float64{fx, fz}, [2]float64{px, pz}
		turn[i] = math.Atan2(pz-fz, px-fx)
	}
	turn[1] += math.Pi
	// The stairs, and the gallery's lead, on the way there and out of it.
	var in, out [][2]float64
	for i, h := range t.heads {
		for x := 0.0; x < h.run(); x++ {
			px, pz := l.plan(h.floor(x))
			if i == 0 {
				in = append(in, [2]float64{px, pz})
			} else {
				out = append([][2]float64{{px, pz}}, out...)
			}
		}
	}
	var best [][2]float64
	long := math.Inf(1)
	for _, arcs := range turns(lead[0], turn[0], lead[1], turn[1], tunnelTurn) {
		line := [][2]float64{foot[0]}
		at, dir := lead[0], turn[0]
		line = append(line, at)
		for _, a := range arcs {
			if a.long < 1e-4 {
				continue
			}
			n := 1
			if a.turn != 0 {
				n = int(math.Ceil(math.Abs(a.turn) * tunnelTurn / 2))
			} else {
				n = int(math.Max(1, math.Ceil(a.long/t.def.Step-0.2)))
			}
			for range n {
				d := a.turn / float64(n)
				step := a.long / float64(n)
				if d != 0 {
					step = 2 * tunnelTurn * math.Sin(math.Abs(d)/2) // the chord
				}
				dir += d / 2
				at = [2]float64{at[0] + math.Cos(dir)*step, at[1] + math.Sin(dir)*step}
				dir += d / 2
				line = append(line, at)
			}
		}
		line[len(line)-1] = lead[1] // rounding
		line = append(line, foot[1])
		sum := 0.0
		for i := range len(line) - 1 {
			sum += math.Hypot(line[i+1][0]-line[i][0], line[i+1][1]-line[i][1])
		}
		whole := append(append(append([][2]float64{}, in...), line...), out...)
		if sum < long && apart(whole, t.def.Width+tunnelApart) {
			best, long = line, sum
		}
	}
	return best, best != nil
}

// apart tells whether a way keeps d m from where it was before, but for
// where it just was.
func apart(line [][2]float64, d float64) bool {
	s := make([]float64, len(line))
	for i := range len(line) - 1 {
		s[i+1] = s[i] + math.Hypot(line[i+1][0]-line[i][0], line[i+1][1]-line[i][1])
	}
	for i := range line {
		for j := range i {
			// A turn of tunnelTurn is wider than d across once it is πd/2 long.
			if s[i]-s[j] > d*math.Pi/2+2 && math.Hypot(line[i][0]-line[j][0], line[i][1]-line[j][1]) < d {
				return false
			}
		}
	}
	return true
}

// arc is a part of a way: that long, straight or turning by so much.
type arc struct{ long, turn float64 }

// turns are the ways from a point and heading to another, by a turn of
// radius r, a straight line, and another turn; left is a growing angle.
func turns(a [2]float64, ha float64, b [2]float64, hb float64, r float64) [][3]arc {
	whole := func(v float64) float64 { // from 0 to a turn
		v = math.Mod(math.Mod(v, 2*math.Pi)+2*math.Pi, 2*math.Pi)
		if v > 2*math.Pi-1e-6 {
			return 0
		}
		return v
	}
	center := func(p [2]float64, h, side float64) [2]float64 {
		return [2]float64{p[0] - math.Sin(h)*side*r, p[1] + math.Cos(h)*side*r}
	}
	var out [][3]arc
	for _, sides := range [][2]float64{{1, 1}, {-1, -1}, {1, -1}, {-1, 1}} {
		ca, cb := center(a, ha, sides[0]), center(b, hb, sides[1])
		dx, dz := cb[0]-ca[0], cb[1]-ca[1]
		d, dir := math.Hypot(dx, dz), math.Atan2(dz, dx)
		straight := d
		if sides[0] != sides[1] {
			if d < 2*r {
				continue
			}
			straight = math.Sqrt(d*d - 4*r*r)
			dir += sides[0] * math.Asin(2*r/d)
		}
		if d < 1e-9 {
			dir = ha
		}
		one, two := whole(sides[0]*(dir-ha)), whole(sides[1]*(hb-dir))
		var w [3]arc
		w[0] = arc{one * r, sides[0] * one}
		w[1] = arc{straight, 0}
		w[2] = arc{two * r, sides[1] * two}
		out = append(out, w)
	}
	return out
}

// slopes lays the gallery's floor along its way: straight from a stair's
// foot to the other's, lower where the ground is, and never too steep. It
// returns how much deeper the stairs should go for it.
func (t *tunnel) slopes(line [][2]float64, site Site, ground func(dir vec3) float64) (deeper [2]float64) {
	l := &Link{Site: site, dug: t}
	t.nodes = t.nodes[:0]
	n := len(line)
	var feet [2]float64
	for i, h := range t.heads {
		feet[i] = h.floor(h.run()).Length()
	}
	s, low := make([]float64, n), make([]float64, n)
	dirs := make([]vec3, n)
	for i, p := range line {
		dirs[i] = l.unplan(p[0], p[1])
		if i > 0 {
			s[i] = s[i-1] + math.Hypot(p[0]-line[i-1][0], p[1]-line[i-1][1])
		}
	}
	for i := range n {
		// The ground over it, and a little to its sides.
		a, b := dirs[max(0, i-1)], dirs[min(n-1, i+1)]
		side := b.Sub(a).Cross(dirs[i]).Norm()
		low[i] = math.Inf(1)
		for _, off := range []float64{-1, 0, 1} {
			d := dirs[i].Add(side.Scale(off * (t.def.Width/2 + 0.8) / planet.Radius)).Norm()
			low[i] = math.Min(low[i], ground(d)-t.def.Cover-t.def.Height)
		}
	}
	for i := range n {
		k := s[i] / math.Max(1e-6, s[n-1])
		low[i] = math.Min(low[i], feet[0]*(1-k)+feet[1]*k)
	}
	for i := 1; i < n; i++ {
		low[i] = math.Min(low[i], low[i-1]+t.def.Slope*(s[i]-s[i-1]))
	}
	for i := n - 2; i >= 0; i-- {
		low[i] = math.Min(low[i], low[i+1]+t.def.Slope*(s[i+1]-s[i]))
	}
	for i := range n {
		t.nodes = append(t.nodes, tunnelNode{dirs[i].Scale(low[i]), s[i]})
	}
	return [2]float64{feet[0] - low[0], feet[1] - low[n-1]}
}

// cut cuts the tunnel into sections, from a port to the other.
func (t *tunnel) cut() {
	head := func(i int) []tunnelSection {
		h := t.heads[i]
		across := h.Z
		if i == 1 {
			across = across.Scale(-1) // as from the first
		}
		at := func(x float64) tunnelSection { return tunnelSection{h.floor(x), across, h.Y, i, x} }
		out := []tunnelSection{at(linkStart), at(headLanding)}
		n := int(math.Ceil(float64(h.steps) / 5))
		for k := range n - 1 {
			out = append(out, at(headLanding+float64((k+1)*5)*stepRun))
		}
		return out
	}
	t.sections = head(0)
	for i, nd := range t.nodes {
		up := nd.at.Norm()
		var d vec3
		long := 1.0
		switch i {
		case 0:
			d = t.heads[0].X
		case len(t.nodes) - 1:
			d = t.heads[1].X.Scale(-1)
		default:
			a := planet.TangentAt(up, nd.at.Sub(t.nodes[i-1].at))
			b := planet.TangentAt(up, t.nodes[i+1].at.Sub(nd.at))
			d = a.Add(b).Norm()
			long = 1 / math.Max(0.5, a.Dot(d))
		}
		t.sections = append(t.sections, tunnelSection{nd.at, d.Cross(up).Scale(long), up, -1, 0})
	}
	end := head(1)
	for i := range end {
		t.sections = append(t.sections, end[len(end)-1-i])
	}
}

// hollow is the room there is to walk in.
func (t *tunnel) hollow() {
	w, h := t.def.Width/2, t.def.Height-tunnelLow
	add := func(a, b vec3, pads [2]float64) {
		t.hollows = append(t.hollows, planet.Hollow{A: a, B: b, Half: w, Height: h, Pad: pads})
	}
	for _, hd := range t.heads {
		add(hd.floor(0), hd.floor(headLanding), [2]float64{0.1, 0.05})
		add(hd.floor(headLanding), hd.floor(hd.run()), [2]float64{0.05, 0.05})
	}
	n := len(t.nodes)
	dir := func(i int) vec3 { return t.nodes[i+1].at.Sub(t.nodes[i].at).Norm() }
	pad := func(i int) float64 { // of the bend at node i
		if i <= 0 || i >= n-1 {
			return 0.05
		}
		c := math.Max(-1, math.Min(1, dir(i-1).Dot(dir(i))))
		return w*math.Tan(math.Acos(c)/2) + 0.05
	}
	for i := range n - 1 {
		add(t.nodes[i].at, t.nodes[i+1].at, [2]float64{pad(i), pad(i + 1)})
	}
	t.digs = planet.NewDigs([][]planet.Hollow{t.hollows})
}

// holds tells whether a point of the world is in it.
func (t *tunnel) holds(p vec3) bool {
	_, _, ok := t.digs.At(p)
	return ok
}

// Under tells whether it goes under the ground.
func (l *Link) Under() bool { return l.dug != nil }

// Hollows are a tunnel's, in the world.
func (l *Link) Hollows() []planet.Hollow {
	if l.dug == nil {
		return nil
	}
	return l.dug.hollows
}

// Deep goes through points of a tunnel's floor, one every metre or so, from
// its first stairs to its last, in the world.
func (l *Link) Deep(see func(at vec3)) {
	if l.dug == nil {
		return
	}
	for _, h := range l.dug.hollows {
		n := int(math.Ceil(h.A.Sub(h.B).Length()))
		for k := range n + 1 {
			see(h.A.Lerp(h.B, float64(k)/float64(n)))
		}
	}
}

// Buried goes through the points of a tunnel that must stay under the
// ground: the top of its vault, its middle and sides, once its shelters are
// behind; and how much ground it wants there.
func (l *Link) Buried(see func(at vec3, want float64)) {
	if l.dug == nil {
		return
	}
	t := l.dug
	for i, s := range t.sections {
		want := t.def.Cover
		if i == 0 || i == len(t.sections)-1 || s.head >= 0 && s.x < headEnd {
			continue
		}
		if s.head >= 0 {
			want = 0.3
		}
		for _, x := range []float64{-1, 0, 1} {
			see(s.at.Add(s.across.Scale(x*t.def.Width/2)).Add(s.up.Scale(t.def.Height)), want)
		}
	}
}

// roofTop is how high over its port's floor a shelter is, x m out of it.
func (t *tunnel) roofTop(h tunnelHead, x float64) float64 {
	return math.Max(headCurb, t.def.Height+headRoof-math.Max(0, x-headLanding)*h.slope())
}

// Roof is the top of the shelter of a tunnel over or under a point of the
// world, from Mars' center.
func (l *Link) Roof(p vec3) (float64, bool) {
	if l.dug == nil {
		return 0, false
	}
	for _, h := range l.dug.heads {
		q := h.ToLocal(p)
		if q.X > 0.1 && q.X < headEnd && math.Abs(q.Z) < l.Def.Width/2+headWall && math.Abs(q.Y) < 20 {
			return h.ToWorld(vec3{q.X, l.dug.roofTop(h, q.X), q.Z}).Length(), true
		}
	}
	return 0, false
}

// Sides are a tunnel's walls, in the world.
func (l *Link) Sides() []tunnelWall {
	if l.dug == nil {
		return nil
	}
	t := l.dug
	var out []tunnelWall
	w, h := t.def.Width/2, t.def.Height
	for i := range len(t.sections) - 1 {
		s, e := t.sections[i], t.sections[i+1]
		low, high := math.Min(s.at.Length(), e.at.Length()), math.Max(s.at.Length(), e.at.Length())
		for _, x := range []float64{-w, w} {
			out = append(out, tunnelWall{s.at.Add(s.across.Scale(x)), e.at.Add(e.across.Scale(x)), low - 0.4, high + h})
		}
	}
	// What is left beside the doors.
	for _, s := range []tunnelSection{t.sections[0], t.sections[len(t.sections)-1]} {
		for _, x := range []float64{-1, 1} {
			out = append(out, tunnelWall{s.at.Add(s.across.Scale(x * w)), s.at.Add(s.across.Scale(x * (DoorWidth + 0.2) / 2)), s.at.Length() - 0.4, s.at.Length() + h})
		}
	}
	return out
}

// Holes are the rectangles of ground open over a tunnel's stairs, in frames
// whose X is out of the port and Y up: from their origin, along and across.
func (l *Link) Holes(see func(s Site, x0, x1, half float64)) {
	if l.dug == nil {
		return
	}
	t := l.dug
	for _, h := range t.heads {
		see(h.Site, linkStart+0.1, headLanding+(t.def.Height+0.35)/h.slope(), t.def.Width/2+headWall/2)
	}
}

// ways: the ground level under a tunnel's shelters.
func (t *tunnel) ways(see func(at, x, up vec3, half [2]float64)) {
	for _, h := range t.heads {
		if h.landing > headGround-0.2 {
			continue
		}
		a, b := h.landing-0.3, headGround
		see(h.ToWorld(vec3{(a + b) / 2, 0, 0}), h.X, h.Y, [2]float64{(b - a) / 2, t.def.Width/2 + headWall + 0.2})
	}
}

// path: points of the floor of the shelters.
func (t *tunnel) path(see func(at vec3)) {
	for _, h := range t.heads {
		for x := 0.0; x <= headEnd; x += 0.5 {
			see(h.ToWorld(vec3{x, 0, 0}))
		}
	}
}

// legs: the shelters, seen from above.
func (t *tunnel) legs(see func(corners [4]vec3)) {
	for _, h := range t.heads {
		w := t.def.Width/2 + headWall
		see([4]vec3{h.ToWorld(vec3{0.2, 0, -w}), h.ToWorld(vec3{headEnd, 0, -w}), h.ToWorld(vec3{headEnd, 0, w}), h.ToWorld(vec3{0.2, 0, w})})
	}
}

// ray is how far along a ray the shelters are, from outside.
func (t *tunnel) ray(o, d vec3, max float64) (float64, bool) {
	for k := 0.0; k < max; k += 0.05 {
		p := o.Add(d.Scale(k))
		for _, h := range t.heads {
			q := h.ToLocal(p)
			if q.X > 0.1 && q.X < headEnd && math.Abs(q.Z) < t.def.Width/2+headWall && q.Y > -0.2 && q.Y < t.roofTop(h, q.X) {
				return k, true
			}
		}
	}
	return 0, false
}

// galleryIn is how far past the stairs nothing of the outside is seen.
const galleryIn = 2.5

// InGallery tells whether a point of the world is in a tunnel's gallery,
// far enough from its stairs to see nothing of the outside.
func (l *Link) InGallery(p vec3) bool {
	if l.dug == nil || !l.dug.holds(p) {
		return false
	}
	for _, h := range l.dug.heads {
		q := h.ToLocal(p)
		if q.X < h.run()+galleryIn && q.X > -1 && math.Abs(q.Z) < l.Def.Width && q.Y < l.Def.Height && q.Y > -h.drop-1 {
			return false
		}
	}
	return true
}

// Near is how far a point of the world is from the closest end of a tunnel.
func (l *Link) Near(p vec3) float64 {
	near := math.Inf(1)
	for _, h := range l.dug.heads {
		near = math.Min(near, h.Origin.Sub(p).Length())
	}
	return near
}

// Plan is the way of a tunnel's gallery, its floor's middle, in the world.
func (l *Link) Plan() []vec3 {
	var out []vec3
	if l.dug != nil {
		for _, n := range l.dug.nodes {
			out = append(out, n.at)
		}
	}
	return out
}
