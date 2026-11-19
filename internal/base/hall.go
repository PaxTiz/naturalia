package base

import (
	"cmp"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
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
//
// That is in the hall's own frame: turned (Room.Yaw), its door faces
// elsewhere. The methods below lay it out in that frame; what the rest of
// the game reads (walls, doors, leaves, lamps, bays) is turned with it.

const (
	hallWall     = 0.3 // the walls' thickness
	hallLampDrop = 1.2 // how far the lamps hang under the vault
	pendantDrop  = 1.0 // a warm hall's lamp
)

func (r Room) half() (hx, hz float64) { return r.Hall.Size[0] / 2, r.Hall.Size[1] / 2 }

// bigDoor tells a hall with a big door from one closed all around, like
// the great hall.
func (r Room) bigDoor() bool { return r.Hall != nil && r.Hall.Door[0] > 0 }

// vault is the inside of the roof's height over the floor, u m east of the
// hall's middle: a parabola from the walls' top to the ridge, a straight
// slope up to the east wall, or level with the walls' top.
func (r Room) vault(u float64) float64 {
	hx, _ := r.half()
	k := u / hx
	switch r.Hall.Roof {
	case "shed":
		return r.WallH + r.Hall.Rise*(k+1)/2
	case "flat":
		return r.WallH
	}
	return r.WallH + r.Hall.Rise*(1-k*k)
}

// sloped tells a roof that rises from one long wall to the other: the
// east wall stands higher than the west one.
func (r Room) sloped() bool { return r.Hall.Roof == "shed" }

// wallTop is how high the wall from a to b (local, in the hall's frame)
// rises: a long wall up to the roof over it, a gable end up to where the
// roof starts (the gable fills in above).
func (r Room) wallTop(a, b vec3) float64 {
	if math.Abs(a.X-b.X) < 0.01 {
		return r.vault(a.X - r.Center.X)
	}
	return r.WallH
}

// gable is the outline of a gable end at z (in the hall's frame), y m
// under the roof, from the top of the gable walls: none under a flat roof.
func (r Room) gable(z, y float64) []vec3 {
	hx, _ := r.half()
	if r.Hall.Roof == "flat" {
		return nil
	}
	const steps = 24
	var pts []vec3
	for i := 0; i <= steps; i++ {
		u := -hx + 2*hx*float64(i)/steps
		pts = append(pts, r.Center.Add(vec3{u, r.vault(u) - y, z}))
	}
	if r.sloped() { // down the east wall to the west one's top
		pts = append(pts, r.Center.Add(vec3{hx, r.WallH - y, z}))
	}
	return pts
}

// reach is how far the room stands from its center.
func (r Room) reach() float64 {
	if r.Hall == nil {
		return r.Radius
	}
	hx, hz := r.half()
	return math.Hypot(hx+2+r.bermRun(), hz+r.bermRun())
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
	d := r.Unturn(local).Sub(r.Center)
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
			out = append(out, Segment{r.Turn(s[0]), r.Turn(s[1]), top})
		}
	}
	return out
}

// straightWalls are the hall's walls from end to end at floor level
// (local), the big door left out.
func (r Room) straightWalls() [][2]vec3 {
	hx, hz := r.half()
	p := func(x, z float64) vec3 { return r.Center.Add(vec3{x, 0, z}) }
	walls := [][2]vec3{
		{p(-hx, -hz), p(-hx, hz)},
		{p(-hx, -hz), p(hx, -hz)},
		{p(-hx, hz), p(hx, hz)},
	}
	if !r.bigDoor() {
		return append(walls, [2]vec3{p(hx, -hz), p(hx, hz)})
	}
	dw := r.Hall.Door[0] / 2
	return append(walls, [2]vec3{p(hx, -hz), p(hx, -dw)}, [2]vec3{p(hx, dw), p(hx, hz)})
}

// hallGap is half the width of a corridor's doorway in a hall's wall.
const hallGap = DoorWidth/2 + WallThick + 0.05

// doorways are where corridors open into the hall: on its walls, at floor
// level (in its own frame).
func (r Room) doorways() []vec3 {
	var out []vec3
	for _, a := range r.openings {
		d := vec3{math.Cos(a), 0, math.Sin(a)}
		out = append(out, r.Center.Add(content.Turn(d, -r.Yaw).Scale(edge(r.plan, d))))
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

// pane is a window in a hall's wall: its ends at floor level (in the
// hall's own frame), from its sill up to its head.
type pane struct {
	a, b   vec3
	y0, y1 float64
}

// panes are the hall's windows in the stretch of wall from a to b, in
// order along it, kept within it and under the top of the wall.
func (r Room) panes(a, b vec3) []pane {
	along := b.Sub(a)
	l := along.Length()
	along = along.Scale(1 / l)
	var out []pane
	end := 0.0 // of the last one, along the wall
	wins := slices.Clone(r.plan.Windows)
	slices.SortFunc(wins, func(v, w content.Window) int { return cmp.Compare(v.At.Sub(w.At).Dot(along), 0) })
	for _, w := range wins {
		p := r.Center.Add(vec3{w.At.X, 0, w.At.Z})
		t := p.Sub(a).Dot(along)
		if off := p.Sub(a.Add(along.Scale(t))).Length(); off > 0.5 {
			continue // on another wall
		}
		t0, t1 := math.Max(math.Max(0.3, end), t-w.Width/2), math.Min(l-0.3, t+w.Width/2)
		y0, y1 := math.Max(0, w.At.Y), math.Min(r.wallTop(a, b)-0.2, w.At.Y+w.Height)
		if t1-t0 < 0.3 || y1-y0 < 0.3 {
			continue
		}
		out = append(out, pane{a.Add(along.Scale(t0)), a.Add(along.Scale(t1)), y0, y1})
		end = t1
	}
	return out
}

// solid are the stretches of the wall from a to b left standing full
// height beside its windows.
func (r Room) solid(a, b vec3) [][2]vec3 {
	var out [][2]vec3
	from := a
	for _, pn := range r.panes(a, b) {
		if pn.a.Sub(from).Length() > 0.01 {
			out = append(out, [2]vec3{from, pn.a})
		}
		from = pn.b
	}
	if b.Sub(from).Length() > 0.01 {
		out = append(out, [2]vec3{from, b})
	}
	return out
}

// InnerDoor is a sliding door where a corridor opens into a hall, keeping
// the base's air in when the hall's big door is open: its middle on the
// floor (local), the way its two leaves slide (along the wall) and the
// way out of the hall.
type InnerDoor struct {
	Mid, Along, Out vec3
	// Home: the door of a living space onto a corridor to a working
	// building, finished like the space and as high as its frame.
	Home bool
}

// InnerLeaf is the width of each of an inner door's two leaves.
const InnerLeaf = (DoorWidth + 0.2) / 2

// InnerDoors are the corridor doors of the halls with a big door, then
// those of the living halls onto a corridor to a working building (the
// hangar): its industry stays behind them.
func InnerDoors() []InnerDoor {
	var out []InnerDoor
	for _, r := range Buildings() {
		if !r.bigDoor() {
			continue
		}
		for _, w := range r.doorways() {
			o := r.wallOut(w)
			out = append(out, InnerDoor{r.Turn(w), r.TurnDir(vec3{-o.Z, 0, o.X}), r.TurnDir(o), false})
		}
	}
	for _, c := range ordered(Ents.Corridor) {
		for i, end := range []vec3{c.From, c.To} {
			r, other := Ents.Building.Get(c.Rooms[i]), Ents.Building.Get(c.Rooms[1-i])
			if r == nil || other == nil || r.Hall == nil || !r.home() || other.home() {
				continue
			}
			var best vec3
			for k, w := range r.doorways() {
				if k == 0 || r.Turn(w).Sub(end).Length() < r.Turn(best).Sub(end).Length() {
					best = w
				}
			}
			o := r.wallOut(best)
			out = append(out, InnerDoor{r.Turn(best), r.TurnDir(vec3{-o.Z, 0, o.X}), r.TurnDir(o), true})
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

// HallDoor is the big door of hall building i: its middle on the floor
// (local), and the wall it closes, for collisions while it isn't open.
func HallDoor(i ecs.ID) (mid vec3, closed Segment) {
	r := Ents.Building.Get(i)
	mid, closed = r.door()
	return r.Turn(mid), Segment{r.Turn(closed.a), r.Turn(closed.b), closed.top}
}

// door is the big door's middle and the wall it closes, in the hall's
// own frame.
func (r Room) door() (mid vec3, closed Segment) {
	hx, _ := r.half()
	dw := r.Hall.Door[0] / 2
	mid = r.Center.Add(vec3{hx, 0, 0})
	return mid, Segment{mid.Add(vec3{0, 0, -dw}), mid.Add(vec3{0, 0, dw}), r.Hall.Door[1]}
}

// HallDoorWalls are the walls the door of hall building i makes, open
// from 0 to 1: on each side, from the jamb to the inner edge of its leaves,
// so the opening lets through as soon as it is wide enough.
func HallDoorWalls(i ecs.ID, open float64) []Segment {
	r := Ents.Building.Get(i)
	mid, closed := r.door()
	half := r.Hall.Door[0] / 2
	gap := half // how far from the middle the leaves start, on each side
	for _, l := range r.doorLeaves(open) {
		gap = math.Min(gap, math.Abs(l.At.Z-mid.Z)-l.Size.Z/2)
	}
	if gap >= half-0.1 {
		return nil
	}
	gap = math.Max(gap, 0)
	var out []Segment
	for _, s := range []float64{-1, 1} {
		out = append(out, Segment{r.Turn(mid.Add(vec3{0, 0, s * gap})), r.Turn(mid.Add(vec3{0, 0, s * half})), closed.top})
	}
	return out
}

// Halls are the hall buildings with a big door.
func Halls() []ecs.ID {
	var out []ecs.ID
	for id, r := range Buildings() {
		if r.bigDoor() {
			out = append(out, id)
		}
	}
	return out
}

// Leaf is a door leaf where it stands: its middle (local), its size along
// the hall's own axes, and how the hall is turned (degrees, as
// content.Turn).
type Leaf struct {
	At, Size vec3
	Yaw      float64
}

// HallLeaves are where the door leaves of hall building i stand, open
// from 0 (closed) to 1. Each runs on its own rail, so they stack in the
// pockets.
func HallLeaves(i ecs.ID, open float64) []Leaf {
	r := Ents.Building.Get(i)
	leaves := r.doorLeaves(open)
	for k := range leaves {
		leaves[k].At, leaves[k].Yaw = r.Turn(leaves[k].At), r.Yaw
	}
	return leaves
}

// doorLeaves are the door's leaves in the hall's own frame.
func (r Room) doorLeaves(open float64) []Leaf {
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
			out = append(out, Leaf{At: r.Center.Add(vec3{x, (dh + rise) / 2, z}), Size: vec3{0.22, dh + rise, w}})
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
// along the ridge and down both sides of the vault. They are as bright as
// it takes for the hangar (26 × 64 m), less in a smaller hall, and reach
// across it.
// A warm hall has only one, a dim amber pendant hanging lower.
func (r Room) hallLamps() []Lamp {
	hx, hz := r.half()
	if r.warm() {
		return []Lamp{{r.Center.Add(vec3{0, r.vault(0) - pendantDrop, 0}), 0.6, 2*math.Max(hx, hz) + 2, 1}}
	}
	power := 12 * math.Max(0.12, math.Min(1, 4*hx*hz/(26*64)))
	if r.home() {
		power *= homeDim // its white lining and lines of light do the rest
	}
	reach := math.Min(24, 2*math.Max(hx, hz)+4)
	at := func(u, w float64) Lamp {
		return Lamp{r.Center.Add(vec3{u, r.vault(u) - hallLampDrop, w}), power, reach, r.lampWarmth()}
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
// on the floor around it (its middle, local, and its size along and
// across Nose), and its spot, where the vehicle stands in the middle of
// it, nose toward the door (+X in the hall's own frame).
type Bay struct {
	ID            string
	At            vec3
	Length, Width float64
	Spot          vec3
	Nose          vec3 // toward the door, unit
	// Its name on the wall behind it, between the arches: the middle of
	// the sign (local) and its width.
	SignAt vec3
	SignW  float64
}

// Side is the unit vector across the bay, to the nose's right.
func (b Bay) Side() vec3 { return vec3{-b.Nose.Z, 0, b.Nose.X} }

// bayMargin is the room around a vehicle in its bay, on each side.
const bayMargin = 1.2

// layBays gives each driven vehicle a bay in the first garage, sized to
// it, side by side along the back wall, a second row in front if they
// don't fit. An aisle runs between them from a corridor's doorway in the
// back wall to the big door.
func layBays(c *content.Content) []Bay {
	var r *Room
	for _, h := range Buildings() {
		if h.Hall != nil && h.plan.Garage {
			r = h
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
			z, w := r.signSpot(b.At.Z-r.Center.Z, b.Width)
			b.SignAt, b.SignW = r.Turn(r.Center.Add(vec3{-hx + 0.17, 3.4, z})), w
			b.At, b.Spot, b.Nose = r.Turn(b.At), r.Turn(b.Spot), r.TurnDir(vec3{1, 0, 0})
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
// and pockets, its furniture and the parking bays.
func (b *building) hall(id ecs.ID, r Room) {
	b.turned(r, func(b *building) { b.hallShell(r) })
	b.props(id)
	b.turned(r, func(b *building) { b.hallBays(r) })
}

// hallShell builds all of a hall but its furniture and bays, in its own
// frame.
func (b *building) hallShell(r Room) {
	bp := r.plan
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
	inside(r.floorTok()).Box(p(0, FloorTop-0.05, 0), vec3{2 * hx, 0.1, 2 * hz}, X, 0)
	// The arches and pilasters inside: steel, or in a home smooth white
	// ribs that stand less into the room, through its lining.
	rib, ribIn, ribDeep := "steel", 0.38, 0.5
	if r.home() {
		rib, ribIn, ribDeep = "home.rib", 0.32, 0.3
	}
	if dw > 0 {
		sun("pad.concrete").Box(p(hx+5, 0.02, 0), vec3{10, 0.1, dw + 4}, X, 0.02)
		sun("hazard").Box(p(hx+0.6, 0.075, 0), vec3{0.5, 0.02, dw}, X, 0)
	}

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
	// Around the windows: the wall under the sill and over the head.
	for _, w := range r.straightWalls() {
		top := r.wallTop(w[0], w[1])
		for _, s := range r.spans(w[0], w[1]) {
			for _, sd := range r.solid(s[0], s[1]) {
				panels(sd[0], sd[1], 0, top)
			}
			for _, pn := range r.panes(s[0], s[1]) {
				if pn.y0 > 0.01 {
					panels(pn.a, pn.b, 0, pn.y0)
				}
				panels(pn.a, pn.b, pn.y1, top)
				b.window(r, pn)
			}
		}
	}
	if dw > 0 {
		panels(p(hx, 0, -dw/2), p(hx, 0, dw/2), dh, r.vault(hx))
	}
	// The corridors' doorways: a lintel above, a steel frame on both
	// faces, hazard stripes over it.
	for _, w := range r.doorways() {
		out := r.wallOut(w)
		along := vec3{-out.Z, 0, out.X}
		panels(w.Sub(along.Scale(hallGap)), w.Add(along.Scale(hallGap)), DoorHeight+0.1, r.wallTop(w.Sub(along), w.Add(along)))
		if r.home() {
			b.homeDoorFrame(w, out, homeDoorHalf, DoorHeight+0.1, hallWall+0.2, "home.accent")
			continue // and lined (hallLining)
		}
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

	// The gables, up under the roof.
	const steps = 24
	for _, s := range []float64{-1, 1} {
		if pts := r.gable(s*hz, 0); pts != nil {
			wall(bp.Style).Slab(pts, Z, hallWall)
		}
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
			inside(rib).BoxBasis(mid.Sub(nrm.Scale(ribIn)).Add(vec3{0, 0, z}), [3]vec3{along, nrm, Z}, vec3{l + 0.06, ribDeep, 0.35}, 0.03)
			if r.home() { // a line of light along the rib's underside
				inside(r.led()).BoxBasis(mid.Sub(nrm.Scale(ribIn+ribDeep/2)).Add(vec3{0, 0, z}), [3]vec3{along, nrm, Z}, vec3{l + 0.06, 0.012, 0.07}, 0)
			}
		}
	}
	// A ridge cap, and skylight vents along it; on a slope, eaves.
	if r.sloped() {
		b.eaves(r)
	} else {
		sun("steel").Box(p(0, r.vault(0)+0.42, 0), vec3{1.2, 0.14, 2*hz + 0.9}, X, 0.04)
	}
	if hz < 6 && r.Hall.Roof == "" { // a small one has room for one
		sun("steel.dark").Box(p(0, r.vault(0)+0.62, 0), vec3{1.6, 0.3, math.Min(3, hz)}, X, 0.05)
	}
	for k := -2; k <= 2 && hz >= 6 && r.Hall.Roof == ""; k++ {
		sun("steel.dark").Box(p(0, r.vault(0)+0.62, float64(k)*hz*0.38), vec3{2.2, 0.35, 3}, X, 0.05)
	}

	// Pilasters at every arch, inside and out, and around the outside a
	// dark plinth.
	for _, z := range r.pillars() {
		for _, s := range []float64{-1, 1} {
			if s > 0 && dw > 0 && math.Abs(z) < dw/2+0.5 {
				continue // the door
			}
			x := s * hx
			if r.nearDoorway(p(x, 0, z), 0.5) {
				continue
			}
			h := r.vault(x)
			sun("steel").Box(p(x+s*0.3, h/2, z), vec3{0.3, h, 0.6}, X, 0.04)
			inside(rib).Box(p(x-s*(hallWall/2+(ribDeep-0.1)/2), h/2, z), vec3{ribDeep - 0.1, h, 0.45}, X, 0.04)
		}
	}
	for k, w := range r.straightWalls() {
		if k >= 3 && dw > 0 {
			break // the door's wall
		}
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

	// Inside: the two-tone base of the walls and the accent strip, or a
	// home's lining.
	if r.home() {
		b.hallLining(r)
	}
	strip := func(from, to vec3, nrm vec3) {
		d := to.Sub(from)
		mid := from.Lerp(to, 0.5).Add(nrm.Scale(hallWall/2 + 0.03))
		inside("wall.lower").Box(mid.Add(vec3{0, 0.6, 0}), vec3{d.Length(), 1.2, 0.05}, d, 0.01)
		inside(bp.Accent).Box(mid.Add(vec3{0, 1.24, 0}).Add(nrm.Scale(0.02)), vec3{d.Length(), 0.08, 0.04}, d, 0)
	}
	for k, w := range r.straightWalls() {
		if r.home() {
			break
		}
		nrm := []vec3{X, Z, Z.Scale(-1), X.Scale(-1), X.Scale(-1)}[k]
		for _, s := range r.spans(w[0], w[1]) {
			for _, sd := range r.solid(s[0], s[1]) {
				strip(sd[0], sd[1], nrm)
			}
			// Under a window, the base of the wall up to its sill.
			for _, pn := range r.panes(s[0], s[1]) {
				if h := math.Min(pn.y0, 1.2); h > 0.05 {
					mid := pn.a.Lerp(pn.b, 0.5).Add(nrm.Scale(hallWall/2 + 0.03))
					inside("wall.lower").Box(mid.Add(vec3{0, h / 2, 0}), vec3{pn.b.Sub(pn.a).Length(), h, 0.05}, pn.b.Sub(pn.a), 0.01)
				}
			}
		}
	}

	// The lamps: a housing on a rod from the vault; in a warm hall, a
	// cord and a glowing drum shade; in a home, a ring over the ridge's
	// (the others shine from the ribs' lines).
	for i, l := range r.hallLamps() {
		top := r.vault(l.At.X - c.X)
		if r.home() && !r.warm() {
			if math.Abs(l.At.X-c.X) < 0.01 {
				b.ringPendant(l.At, []float64{1.3, 0.8}[min(i, 1)], top-liningGap-lining)
			}
			continue
		}
		if r.warm() {
			inside("rubber").Prism(l.At.Add(vec3{0, 0.2, 0}), yUp, 0.008, 0.008, top-l.At.Y-0.2, 6, 0)
			inside("lamp.shade").Prism(l.At.Sub(vec3{0, 0.08, 0}), yUp, 0.32, 0.26, 0.3, 24, 0)
			inside("steel.dark").Prism(l.At.Add(vec3{0, 0.2, 0}), yUp, 0.05, 0.03, 0.06, 10, 0)
			inside("lamp.amber").Prism(l.At.Sub(vec3{0, 0.06, 0}), yUp, 0.1, 0.1, 0.02, 12, 0)
			continue
		}
		inside("steel").Prism(l.At.Add(vec3{0, 0.25, 0}), yUp, 0.04, 0.04, top-(l.At.Y)-0.2, 6, 0)
		inside("steel.dark").Prism(l.At.Sub(vec3{0, 0.05, 0}), yUp, 0.75, 0.3, 0.45, 16, 0.03)
		inside("lamp.warm").Prism(l.At.Sub(vec3{0, 0.08, 0}), yUp, 0.6, 0.6, 0.04, 16, 0)
	}

	if dw > 0 {
		b.hallDoor(r)
	}
}

// hallEaves is how far a sloped roof reaches out over the east wall; a
// small hall's, less.
const hallEaves = 1.6

// eaves finish a sloped roof, in the hall's own frame: along its high
// edge it reaches out over the east wall, a thin white plate on a steel
// fascia, shading the wall; along its low one, a steel coping.
func (b *building) eaves(r Room) {
	hx, hz := r.half()
	c := r.Center
	sun := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitSun, vec3{}) }
	Z := vec3{0, 0, 1}
	hallEaves := math.Min(hallEaves, 0.3*hx)
	slope := vec3{2 * hx, r.vault(hx) - r.vault(-hx), 0}.Norm()
	nrm := Z.Cross(slope) // up and out
	top := c.Add(vec3{hx, r.vault(hx), 0}).Add(nrm.Scale(0.12))
	mid := top.Add(slope.Scale(hallEaves / 2))
	sun(r.plan.Style).BoxBasis(mid, [3]vec3{slope, nrm, Z}, vec3{hallEaves + 0.1, 0.24, 2*hz + 0.8}, 0.02)
	sun("steel").BoxBasis(top.Add(slope.Scale(hallEaves+0.05)).Sub(nrm.Scale(0.1)), [3]vec3{slope, nrm, Z}, vec3{0.12, 0.5, 2*hz + 0.9}, 0.02)
	sun("steel").Box(c.Add(vec3{-hx - 0.05, r.vault(-hx) + 0.2, 0}), vec3{0.5, 0.2, 2*hz + 0.9}, vec3{1, 0, 0}, 0.03)
}

// window glazes pane pn of hall r: the glass, a steel frame around it,
// mullions about every 2.4 m and transoms about every 2.6 m; in a home,
// thinner anthracite profiles.
func (b *building) window(r Room, pn pane) {
	in := ToWorld(r.Center.Add(vec3{0, 1.2, 0}))
	frame, bar, deep := "steel", 0.14, hallWall+0.12
	if r.home() { // thin anthracite profiles
		frame, bar, deep = "home.trim", 0.08, hallWall+0.08
	}
	steel := b.Part(frame, mesh.LitTowards, in)
	d := pn.b.Sub(pn.a)
	l, h := d.Length(), pn.y1-pn.y0
	mid := pn.a.Lerp(pn.b, 0.5).Add(vec3{0, (pn.y0 + pn.y1) / 2, 0})
	b.Part("glass.pane", mesh.LitGlass, in).Box(mid, vec3{l, h, 0.04}, d, 0)
	for _, s := range []float64{-1, 1} {
		steel.Box(mid.Add(vec3{0, s * (h - bar) / 2, 0}), vec3{l, bar, deep}, d, 0.02)
	}
	cols := max(1, int(math.Round(l/2.4)))
	for i := 0; i <= cols; i++ {
		at := pn.a.Lerp(pn.b, float64(i)/float64(cols))
		steel.Box(vec3{at.X, mid.Y, at.Z}, vec3{bar, h, deep}, d, 0.02)
	}
	rows := max(1, int(math.Round(h/2.6)))
	for j := 1; j < rows; j++ {
		steel.Box(mid.Add(vec3{0, h * (float64(j)/float64(rows) - 0.5), 0}), vec3{l, 0.08, deep - 0.06}, d, 0.01)
	}
}

// hallDoor builds the big door's frame: a hazard header over the opening,
// jambs, rails on the ground, a gantry over the leaves' path and the
// pockets' posts. The header and jambs stop short of the leaves' rails;
// their faces turned to the hall are lit by its lamps, the others by the
// sun.
func (b *building) hallDoor(r Room) {
	hx, hz := r.half()
	c := r.Center
	dw, dh := r.Hall.Door[0], r.Hall.Door[1]
	in := ToWorld(c.Add(vec3{0, 1.2, 0}))
	wall := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitTowards, in) }
	inside := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitIndoor, vec3{}) }
	sun := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitSun, vec3{}) }
	p := func(x, y, z float64) vec3 { return c.Add(vec3{x, y, z}) }
	X := vec3{1, 0, 0}
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
}

// hallBays marks the parking bays on the floor of a hall, in its own
// frame: yellow lines, and a stop bar at the back.
func (b *building) hallBays(r Room) {
	inside := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitIndoor, vec3{}) }
	X := vec3{1, 0, 0}
	for _, bay := range Bays {
		if !r.inHall(bay.At, 0) {
			continue
		}
		w, l := bay.Width-0.3, bay.Length
		at := r.Unturn(bay.At).Add(vec3{0, FloorTop + 0.005, 0})
		for _, s := range []float64{-1, 1} {
			inside("pipe.yellow").Box(at.Add(vec3{0, 0, s * w / 2}), vec3{l, 0.012, 0.15}, X, 0)
		}
		inside("pipe.yellow").Box(at.Add(vec3{l / 2, 0, 0}), vec3{0.15, 0.012, w}, X, 0)
		inside("hazard").Box(at.Add(vec3{-l/2 + 0.2, 0.1, 0}), vec3{0.3, 0.2, w * 0.6}, X, 0.03)
	}
}
