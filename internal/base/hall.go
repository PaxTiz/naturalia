package base

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/mesh"
	"slices"
)

// Halls are the big rectangular buildings (content.Hall), like the
// vehicles' hangar: straight walls around a slate floor, a barrel vault
// along Z, and a giant door in the east wall (+X) whose leaves slide away
// on rails, each on its own, and stack up in front of the wall on both
// sides of the opening: there are as many as it takes for them to fit
// there, so nothing sticks out past the ends of the hall.
//
//	   ┌─────────────────────────────────────────────────────┐
//	   │  west wall          bays, nose to the door      ▐▐▐ │ east wall
//	   │                      ▭  ▭  ▭  ▭  ▭  ▭       door    │ (+X)
//	   └─────────────────────────────────────────────────────┘

const (
	hallWall     = 0.3 // the walls' thickness
	hallLampDrop = 1.2 // how far the lamps hang under the vault
)

func (r Room) half() (hx, hz float64) { return r.Hall.Size[0] / 2, r.Hall.Size[1] / 2 }

// vault is the inside of the roof's height over the floor, u m east of the
// hall's middle: a parabola from the walls' top to the ridge.
func (r Room) vault(u float64) float64 {
	hx, _ := r.half()
	k := u / hx
	return r.WallH + r.Hall.Rise*(1-k*k)
}

// reach is how far the room stands from its center.
func (r Room) reach() float64 {
	if r.Hall == nil {
		return r.Radius
	}
	hx, hz := r.half()
	return math.Hypot(hx+2, hz)
}

// leaves is how many leaves the door has on each side, and how wide they
// are: few enough to stack between the opening and the end of the hall.
func (r Room) leaves() (n int, width float64) {
	_, hz := r.half()
	half := r.Hall.Door[0] / 2
	room := hz - half - 0.6
	n = 8
	if room > 0 {
		n = min(8, max(2, int(math.Ceil(half/room))))
	}
	return n, half / float64(n)
}

// inHall tells whether a local point is inside the hall's walls, grown by
// margin.
func (r Room) inHall(local vec3, margin float64) bool {
	hx, hz := r.half()
	d := local.Sub(r.Center)
	return math.Abs(d.X) < hx+margin && math.Abs(d.Z) < hz+margin
}

// hallWalls are the hall's walls for collisions, the door left open
// (HallDoors closes it).
func (r Room) hallWalls() []Segment {
	if r.Hall == nil {
		return nil
	}
	top := r.WallH + r.Hall.Rise
	var out []Segment
	for _, w := range r.straightWalls() {
		for _, s := range r.spans(w[0], w[1]) {
			out = append(out, Segment{s[0], s[1], top})
		}
	}
	return out
}

// straightWalls are the hall's walls from end to end at floor level
// (local), the big door left out.
func (r Room) straightWalls() [][2]vec3 {
	hx, hz := r.half()
	p := func(x, z float64) vec3 { return r.Center.Add(vec3{x, 0, z}) }
	dw := r.Hall.Door[0] / 2
	return [][2]vec3{
		{p(-hx, -hz), p(-hx, hz)},
		{p(-hx, -hz), p(hx, -hz)},
		{p(-hx, hz), p(hx, hz)},
		{p(hx, -hz), p(hx, -dw)},
		{p(hx, dw), p(hx, hz)},
	}
}

// hallGap is half the width of a corridor's doorway in a hall's wall.
const hallGap = DoorWidth/2 + WallThick + 0.05

// doorways are where corridors open into the hall: on its walls, at floor
// level (local).
func (r Room) doorways() []vec3 {
	var out []vec3
	for _, a := range r.openings {
		d := vec3{math.Cos(a), 0, math.Sin(a)}
		out = append(out, r.Center.Add(d.Scale(edge(r.plan, d))))
	}
	return out
}

// spans are the stretches of the wall from a to b left standing between
// the doorways, as their ends.
func (r Room) spans(a, b vec3) [][2]vec3 {
	along := b.Sub(a)
	l := along.Length()
	along = along.Scale(1 / l)
	t0 := 0.0
	var out [][2]vec3
	var gaps []float64
	for _, w := range r.doorways() {
		t := w.Sub(a).Dot(along)
		if off := w.Sub(a.Add(along.Scale(t))).Length(); off < 0.1 && t > 0 && t < l {
			gaps = append(gaps, t)
		}
	}
	slices.Sort(gaps)
	for _, t := range gaps {
		if t-hallGap > t0 {
			out = append(out, [2]vec3{a.Add(along.Scale(t0)), a.Add(along.Scale(t - hallGap))})
		}
		t0 = t + hallGap
	}
	if t0 < l {
		out = append(out, [2]vec3{a.Add(along.Scale(t0)), b})
	}
	return out
}

// InnerDoor is a sliding door where a corridor opens into a hall, keeping
// the base's air in when the hall's big door is open: its middle on the
// floor (local), the way its two leaves slide (along the wall) and the
// way out of the hall.
type InnerDoor struct {
	Mid, Along, Out vec3
}

// InnerLeaf is the width of each of an inner door's two leaves.
const InnerLeaf = (DoorWidth + 0.2) / 2

// InnerDoors are the halls' corridor doors.
func InnerDoors() []InnerDoor {
	var out []InnerDoor
	for _, r := range Rooms {
		if r.Hall == nil {
			continue
		}
		for _, w := range r.doorways() {
			o := r.wallOut(w)
			out = append(out, InnerDoor{w, vec3{-o.Z, 0, o.X}, o})
		}
	}
	return out
}

// Walls are what an inner door, open from 0 to 1, stops: its leaves, the
// opening between them left free.
func (d InnerDoor) Walls(open float64) []Segment {
	gap := open * InnerLeaf
	if gap >= hallGap {
		return nil
	}
	return []Segment{
		{d.Mid.Add(d.Along.Scale(gap)), d.Mid.Add(d.Along.Scale(hallGap)), DoorHeight},
		{d.Mid.Sub(d.Along.Scale(gap)), d.Mid.Sub(d.Along.Scale(hallGap)), DoorHeight},
	}
}

// wallOut is the way out of the hall through the wall at local point w.
func (r Room) wallOut(w vec3) vec3 {
	hx, hz := r.half()
	d := w.Sub(r.Center)
	if math.Abs(d.X/hx) > math.Abs(d.Z/hz) {
		return vec3{math.Copysign(1, d.X), 0, 0}
	}
	return vec3{0, 0, math.Copysign(1, d.Z)}
}

// pillars are where the arches stand along the long walls: z from the
// hall's middle, the end ones against the gables.
func (r Room) pillars() []float64 {
	_, hz := r.half()
	ribs := int(math.Round(2 * hz / 8))
	var out []float64
	for k := 0; k <= ribs; k++ {
		out = append(out, math.Max(-hz+0.3, math.Min(hz-0.3, -hz+2*hz*float64(k)/float64(ribs))))
	}
	return out
}

// nearDoorway tells if a local point stands within m of a doorway's sides.
func (r Room) nearDoorway(p vec3, m float64) bool {
	for _, w := range r.doorways() {
		if math.Hypot(p.X-w.X, p.Z-w.Z) < hallGap+m {
			return true
		}
	}
	return false
}

// HallDoor is the big door of hall room i: its middle on the floor (local),
// and the wall it closes, for collisions while it isn't open.
func HallDoor(i int) (mid vec3, closed Segment) {
	r := Rooms[i]
	hx, _ := r.half()
	dw := r.Hall.Door[0] / 2
	mid = r.Center.Add(vec3{hx, 0, 0})
	return mid, Segment{mid.Add(vec3{0, 0, -dw}), mid.Add(vec3{0, 0, dw}), r.Hall.Door[1]}
}

// HallDoorWalls are the walls the door of hall room i makes, open from 0
// to 1: on each side, from the jamb to the inner edge of its leaves, so
// the opening lets through as soon as it is wide enough.
func HallDoorWalls(i int, open float64) []Segment {
	mid, closed := HallDoor(i)
	half := Rooms[i].Hall.Door[0] / 2
	gap := half // how far from the middle the leaves start, on each side
	for _, l := range HallLeaves(i, open) {
		gap = math.Min(gap, math.Abs(l.At.Z-mid.Z)-l.Size.Z/2)
	}
	if gap >= half-0.1 {
		return nil
	}
	gap = math.Max(gap, 0)
	var out []Segment
	for _, s := range []float64{-1, 1} {
		out = append(out, Segment{mid.Add(vec3{0, 0, s * gap}), mid.Add(vec3{0, 0, s * half}), closed.top})
	}
	return out
}

// Halls are the indexes of the hall rooms.
func Halls() []int {
	var out []int
	for i, r := range Rooms {
		if r.Hall != nil {
			out = append(out, i)
		}
	}
	return out
}

// Leaf is a door leaf where it stands: its middle (local) and its size.
type Leaf struct {
	At, Size vec3
}

// HallLeaves are where the door leaves of hall room i stand, open from 0
// (closed) to 1. Each runs on its own rail, so they stack in the pockets.
func HallLeaves(i int, open float64) []Leaf {
	r := Rooms[i]
	hx, hz := r.half()
	dw, dh := r.Hall.Door[0], r.Hall.Door[1]
	n, lw := r.leaves()
	var out []Leaf
	// The leaves rise behind the header, and the outer ones reach behind
	// the jambs: no light comes through at their edges.
	const over, rise = 0.5, 0.4
	for _, s := range []float64{-1, 1} {
		for k := range n {
			w := lw + 0.15
			shut := s * (dw/2 - (float64(k)+0.5)*lw)
			if k == 0 {
				w += over
				shut += s * over / 2
			}
			away := s * math.Min(math.Max(dw/2+w/2+0.3, hz-0.3-lw/2), hz-0.1-w/2)
			x := hx + hallWall/2 + 0.3 + 0.32*float64(k) // the outer ones against the wall, behind the jambs
			z := shut + (away-shut)*smooth(open)
			out = append(out, Leaf{r.Center.Add(vec3{x, (dh + rise) / 2, z}), vec3{0.22, dh + rise, w}})
		}
	}
	return out
}

func smooth(t float64) float64 {
	t = math.Max(0, math.Min(1, t))
	return t * t * (3 - 2*t)
}

// LeafModel is a door leaf built around its middle, to be scaled to size:
// a plate with ribs across, along +Z.
func LeafModel() *mesh.Builder {
	m := &mesh.Builder{}
	m.Box(vec3{}, vec3{1, 1, 1}, vec3{0, 0, 1}, 0.01)
	return m
}

// hallLamps: the first hangs in the middle (it casts shadows), the others
// along the ridge and down both sides of the vault.
func (r Room) hallLamps() []Lamp {
	hx, hz := r.half()
	at := func(u, w float64) Lamp {
		return Lamp{r.Center.Add(vec3{u, r.vault(u) - hallLampDrop, w}), 12, 24}
	}
	lamps := []Lamp{at(0, 0)}
	for _, w := range []float64{-0.66, 0.66} {
		lamps = append(lamps, at(0, w*hz))
	}
	for _, u := range []float64{-0.55, 0.55} {
		for _, w := range []float64{-0.4, 0.4} {
			lamps = append(lamps, at(u*hx, w*hz))
		}
	}
	return lamps
}

// Bay is where a driven vehicle parks in the garage: the rectangle marked
// on the floor around it (its middle, local, and its size along X and Z),
// and its spot, where the vehicle stands in the middle of it, nose toward
// the door (+X).
type Bay struct {
	ID            string
	At            vec3
	Length, Width float64
	Spot          vec3
	// Its name on the wall behind it, between the arches: the middle of
	// the sign along the wall (z, local) and its width.
	SignZ, SignW float64
}

// bayMargin is the room around a vehicle in its bay, on each side.
const bayMargin = 1.2

// layBays gives each driven vehicle a bay in the first garage, sized to
// it, side by side along the back wall, a second row in front if they
// don't fit. An aisle runs between them from a corridor's doorway in the
// back wall to the big door.
func layBays(c *content.Content) []Bay {
	var r *Room
	for i := range Rooms {
		if Rooms[i].Hall != nil && Rooms[i].plan.Garage {
			r = &Rooms[i]
			break
		}
	}
	if r == nil {
		return nil
	}
	hx, hz := r.half()
	aisle, half := 0.0, 0.0 // z of the aisle from the hall's middle, half its width
	for _, w := range r.doorways() {
		if w.X-r.Center.X < -hx+0.5 {
			aisle, half = w.Z-r.Center.Z, hallGap+1.5
			break
		}
	}
	// Each row fills both sides of the aisle, the vehicle going where
	// there is less so far.
	type row struct {
		bays        []Bay
		left, right float64 // widths taken on each side
	}
	room := [2]float64{aisle - half - (-hz + 1), hz - 1 - (aisle + half)}
	rows := []*row{{}}
	for _, v := range c.Vehicles.All() {
		if v.Auto() {
			continue
		}
		lo, hi := v.Footprint()
		b := Bay{ID: v.ID, Length: hi.X - lo.X + 2*bayMargin, Width: hi.Z - lo.Z + 2*bayMargin,
			Spot: lo.Add(hi).Scale(-0.5)} // from the bay's middle for now
		rw := rows[len(rows)-1]
		left := rw.left <= rw.right
		if (left && rw.left+b.Width > room[0]) || (!left && rw.right+b.Width > room[1]) {
			left = !left
		}
		if (left && rw.left+b.Width > room[0]) || (!left && rw.right+b.Width > room[1]) {
			if len(rw.bays) > 0 {
				rw = &row{}
				rows = append(rows, rw)
			}
			left = true
		}
		if left {
			rw.left += b.Width
			b.At.Z = aisle - half - rw.left + b.Width/2
		} else {
			rw.right += b.Width
			b.At.Z = aisle + half + rw.right - b.Width/2
		}
		rw.bays = append(rw.bays, b)
	}
	var out []Bay
	back := -hx + 1.5 // the row's back line
	for _, rw := range rows {
		depth := 0.0
		for _, b := range rw.bays {
			depth = math.Max(depth, b.Length)
		}
		for _, b := range rw.bays {
			b.At = r.Center.Add(vec3{back + b.Length/2, 0, b.At.Z})
			b.Spot = b.Spot.Add(b.At)
			b.Spot.Y = 0
			b.SignZ, b.SignW = r.signSpot(b.At.Z-r.Center.Z, b.Width)
			b.SignZ += r.Center.Z
			out = append(out, b)
		}
		back += depth + 6 // and an aisle
	}
	return out
}

// signSpot fits a bay's sign on the back wall between the arches: the
// widest stretch free of pillars over the bay (z from the hall's middle,
// its width), at most 4.2 m wide, as close to the bay's middle as it goes.
func (r Room) signSpot(z, width float64) (float64, float64) {
	const clear = 0.25 + 0.2 // half a pillar, and a margin
	lo, hi := z-width/2+0.6, z+width/2-0.6
	edges := []float64{lo}
	for _, p := range r.pillars() {
		if p > lo && p < hi {
			edges = append(edges, p-clear, p+clear)
		}
	}
	edges = append(edges, hi)
	bestZ, bestW := z, 0.0
	for i := 0; i+1 < len(edges); i += 2 {
		a, b := edges[i], edges[i+1]
		w := math.Min(b-a, 4.2)
		if w <= 0 {
			continue
		}
		mid := math.Max(a+w/2, math.Min(b-w/2, z))
		if w > bestW+0.01 || (math.Abs(w-bestW) < 0.01 && math.Abs(mid-z) < math.Abs(bestZ-z)) {
			bestZ, bestW = mid, w
		}
	}
	return bestZ, bestW
}

// BayOf is the bay of vehicle id.
func BayOf(id string) (Bay, bool) {
	for _, b := range Bays {
		if b.ID == id {
			return b, true
		}
	}
	return Bay{}, false
}

// hall builds a hall: floor, walls, vault, ribs, lamps, the door's rails
// and pockets, and the parking bays.
func (b *building) hall(bp *content.Blueprint, r Room) {
	hx, hz := r.half()
	c := r.Center
	dw, dh := r.Hall.Door[0], r.Hall.Door[1]
	in := ToWorld(c.Add(vec3{0, 1.2, 0}))
	style := func(i int) string {
		if i%2 == 1 {
			return bp.Style + "2"
		}
		return bp.Style
	}
	wall := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitTowards, in) }
	inside := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitIndoor, vec3{}) }
	sun := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitSun, vec3{}) }
	p := func(x, y, z float64) vec3 { return c.Add(vec3{x, y, z}) }
	X, Z := vec3{1, 0, 0}, vec3{0, 0, 1}

	// Floor, and a concrete apron before the door.
	inside("floor.slate").Box(p(0, FloorTop-0.05, 0), vec3{2 * hx, 0.1, 2 * hz}, X, 0)
	sun("pad.concrete").Box(p(hx+5, 0.02, 0), vec3{10, 0.1, dw + 4}, X, 0.02)
	sun("hazard").Box(p(hx+0.6, 0.075, 0), vec3{0.5, 0.02, dw}, X, 0)

	// Walls, in panels: x0 to x1 along the wall from a to b at height y0
	// to y1. The east one leaves the door open below the header.
	panels := func(from, to vec3, y0, y1 float64) {
		d := to.Sub(from)
		l := d.Length()
		n := max(1, int(math.Round(l/4)))
		for i := range n {
			mid := from.Add(d.Scale((float64(i) + 0.5) / float64(n)))
			wall(style(i)).Box(mid.Add(vec3{0, (y0 + y1) / 2, 0}), vec3{l / float64(n) * 1.005, y1 - y0, hallWall}, d, 0.03)
		}
	}
	for _, w := range r.straightWalls() {
		for _, s := range r.spans(w[0], w[1]) {
			panels(s[0], s[1], 0, r.WallH)
		}
	}
	panels(p(hx, 0, -dw/2), p(hx, 0, dw/2), dh, r.WallH)
	// The corridors' doorways: a lintel above, a steel frame on both
	// faces, hazard stripes over it.
	for _, w := range r.doorways() {
		out := r.wallOut(w)
		along := vec3{-out.Z, 0, out.X}
		panels(w.Sub(along.Scale(hallGap)), w.Add(along.Scale(hallGap)), DoorHeight+0.1, r.WallH)
		for _, f := range []float64{-1, 1} {
			face := w.Add(out.Scale(f * (hallWall/2 + 0.06)))
			lit := wall("steel")
			if f > 0 {
				lit = sun("steel")
			}
			for _, s := range []float64{-1, 1} {
				lit.Box(face.Add(along.Scale(s*(hallGap-0.1))).Add(vec3{0, (DoorHeight + 0.1) / 2, 0}), vec3{0.24, DoorHeight + 0.1, 0.14}, along, 0.03)
			}
			lit.Box(face.Add(vec3{0, DoorHeight + 0.22, 0}), vec3{2*hallGap + 0.04, 0.24, 0.14}, along, 0.03)
		}
		wall("hazard").Box(w.Sub(out.Scale(hallWall/2+0.14)).Add(vec3{0, DoorHeight + 0.5, 0}), vec3{2 * hallGap, 0.25, 0.04}, along, 0)
		wall(bp.Accent).Box(w.Sub(out.Scale(hallWall/2+0.14)).Add(vec3{0, DoorHeight + 0.75, 0}), vec3{2 * hallGap, 0.06, 0.03}, along, 0)
	}

	// The gables, up under the vault.
	const steps = 24
	for _, s := range []float64{-1, 1} {
		var pts []vec3
		for i := 0; i <= steps; i++ {
			u := -hx + 2*hx*float64(i)/steps
			pts = append(pts, p(u, r.vault(u), s*hz))
		}
		wall(bp.Style).Slab(pts, Z, hallWall)
	}

	// The vault: plates across, the outside along the sun, the inside
	// under the lamps; steel arches over it and under it every 8 m.
	arch := func(u float64) vec3 { return vec3{u, r.vault(u), 0} }
	ribs := int(math.Round(2 * hz / 8))
	for i := range steps {
		u0, u1 := -hx+2*hx*float64(i)/steps, -hx+2*hx*float64(i+1)/steps
		a, e := arch(u0), arch(u1)
		along := e.Sub(a)
		l := along.Length()
		along = along.Scale(1 / l)
		nrm := Z.Cross(along) // up and out
		mid := c.Add(a.Lerp(e, 0.5)).Add(nrm.Scale(0.12))
		wall(style(i)).BoxBasis(mid, [3]vec3{along, nrm, Z}, vec3{l + 0.04, 0.24, 2*hz + 0.8}, 0.02)
		for k := 0; k <= ribs; k++ {
			z := -hz + 2*hz*float64(k)/float64(ribs)
			z = math.Max(-hz+0.6, math.Min(hz-0.6, z))
			sun("steel").BoxBasis(mid.Add(nrm.Scale(0.2)).Add(vec3{0, 0, z}), [3]vec3{along, nrm, Z}, vec3{l + 0.06, 0.16, 0.5}, 0.03)
			inside("steel").BoxBasis(mid.Sub(nrm.Scale(0.38)).Add(vec3{0, 0, z}), [3]vec3{along, nrm, Z}, vec3{l + 0.06, 0.5, 0.35}, 0.03)
		}
	}
	// A ridge cap, and skylight vents along it.
	sun("steel").Box(p(0, r.vault(0)+0.42, 0), vec3{1.2, 0.14, 2*hz + 0.9}, X, 0.04)
	for k := -2; k <= 2; k++ {
		sun("steel.dark").Box(p(0, r.vault(0)+0.62, float64(k)*hz*0.38), vec3{2.2, 0.35, 3}, X, 0.05)
	}

	// Pilasters at every arch, inside and out, and around the outside a
	// dark plinth.
	for _, z := range r.pillars() {
		for _, s := range []float64{-1, 1} {
			if s > 0 && math.Abs(z) < dw/2+0.5 {
				continue // the door
			}
			x := s * hx
			if r.nearDoorway(p(x, 0, z), 0.5) {
				continue
			}
			sun("steel").Box(p(x+s*0.3, r.WallH/2, z), vec3{0.3, r.WallH, 0.6}, X, 0.04)
			inside("steel").Box(p(x-s*0.35, r.WallH/2, z), vec3{0.4, r.WallH, 0.45}, X, 0.04)
		}
	}
	for _, w := range r.straightWalls()[:3] { // the west, north and south ones
		for _, s := range r.spans(w[0], w[1]) {
			d := s[1].Sub(s[0])
			mid := s[0].Lerp(s[1], 0.5)
			out := mid.Sub(c)
			out = vec3{out.X, 0, out.Z}
			if d.X != 0 {
				out = vec3{0, 0, math.Copysign(0.05, out.Z)}
			} else {
				out = vec3{math.Copysign(0.05, out.X), 0, 0}
			}
			sun("steel.dark").Box(mid.Add(out).Add(vec3{0, 0.2, 0}), vec3{d.Length() + 0.4, 0.4, 0.42}, d, 0.03)
		}
	}

	// Inside: the two-tone base of the walls and the accent strip.
	strip := func(from, to vec3, nrm vec3) {
		d := to.Sub(from)
		mid := from.Lerp(to, 0.5).Add(nrm.Scale(hallWall/2 + 0.03))
		inside("wall.lower").Box(mid.Add(vec3{0, 0.6, 0}), vec3{d.Length(), 1.2, 0.05}, d, 0.01)
		inside(bp.Accent).Box(mid.Add(vec3{0, 1.24, 0}).Add(nrm.Scale(0.02)), vec3{d.Length(), 0.08, 0.04}, d, 0)
	}
	for k, w := range r.straightWalls() {
		nrm := []vec3{X, Z, Z.Scale(-1), X.Scale(-1), X.Scale(-1)}[k]
		for _, s := range r.spans(w[0], w[1]) {
			strip(s[0], s[1], nrm)
		}
	}

	// The lamps: a housing on a rod from the vault.
	for _, l := range r.hallLamps() {
		top := r.vault(l.At.X - c.X)
		inside("steel").Prism(l.At.Add(vec3{0, 0.25, 0}), yUp, 0.04, 0.04, top-(l.At.Y)-0.2, 6, 0)
		inside("steel.dark").Prism(l.At.Sub(vec3{0, 0.05, 0}), yUp, 0.75, 0.3, 0.45, 16, 0.03)
		inside("lamp.warm").Prism(l.At.Sub(vec3{0, 0.08, 0}), yUp, 0.6, 0.6, 0.04, 16, 0)
	}

	// The door: a hazard header over the opening, jambs, rails on the
	// ground, a gantry over the leaves' path and the pockets' posts.
	// The header and jambs stop short of the leaves' rails; their faces
	// turned to the hall are lit by its lamps, the others by the sun.
	wall("hazard").Box(p(hx+0.05, dh+0.25, 0), vec3{0.5, 0.5, dw + 0.6}, X, 0.04)
	for _, s := range []float64{-1, 1} {
		wall("steel").Box(p(hx-0.05, dh/2, s*(dw/2+0.2)), vec3{0.7, dh, 0.4}, X, 0.04)
	}
	inside("steel.dark").Box(p(hx-0.25, dh+0.3, 0), vec3{0.3, 0.6, dw + 1.1}, X, 0.04)
	inside("hazard").Box(p(hx-0.41, dh+0.3, 0), vec3{0.02, 0.3, dw + 0.4}, X, 0)
	n, _ := r.leaves()
	reach := hz - 0.1
	railX := hx + hallWall/2 + 0.3
	for k := range n {
		x := railX + 0.32*float64(k)
		sun("steel.dark").Box(p(x, 0.04, 0), vec3{0.12, 0.08, 2 * reach}, X, 0.01)
	}
	deep := 0.32*float64(n-1) + 0.6 // the rails' depth, under the gantry
	gantry := p(railX+deep/2-0.3, dh+0.45, 0)
	sun("steel").Box(gantry, vec3{deep + 0.4, 0.5, 2 * reach}, X, 0.04)
	sun("hazard").Box(gantry.Add(vec3{deep/2 + 0.21, 0, 0}), vec3{0.02, 0.3, 2*reach - 0.4}, X, 0)
	for _, s := range []float64{-1, 1} {
		sun("steel").Box(p(railX+deep/2-0.3, (dh+0.7)/2, s*(reach-0.2)), vec3{deep + 0.4, dh + 0.7, 0.35}, X, 0.04)
		sun("lamp.warm").Box(p(hx+0.5, dh+0.05, s*(dw/2-1)), vec3{0.3, 0.06, 1.2}, X, 0)
	}

	// The bays: yellow lines on the floor, and a stop bar at the back.
	for _, bay := range Bays {
		w, l := bay.Width-0.3, bay.Length
		at := bay.At.Add(vec3{0, FloorTop + 0.005, 0})
		for _, s := range []float64{-1, 1} {
			inside("pipe.yellow").Box(at.Add(vec3{0, 0, s * w / 2}), vec3{l, 0.012, 0.15}, X, 0)
		}
		inside("pipe.yellow").Box(at.Add(vec3{l / 2, 0, 0}), vec3{0.15, 0.012, w}, X, 0)
		inside("hazard").Box(at.Add(vec3{-l/2 + 0.2, 0.1, 0}), vec3{0.3, 0.2, w * 0.6}, X, 0.03)
	}
}
