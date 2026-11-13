package base

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/mesh"
)

// Halls are the big rectangular buildings (content.Hall), like the
// vehicles' hangar: straight walls around a slate floor, a barrel vault
// along Z, and a giant door in the east wall (+X) whose leaves slide away
// on rails, three on each side, into pockets past the ends of the hall.
//
//	        pocket ║                              ║ pocket
//	   ┌───────────╨──────────────────────────────╨──────────┐
//	   │  west wall          bays, nose to the door   ═════  │ east wall
//	   │                      ▭  ▭  ▭  ▭  ▭  ▭       door    │ (+X)
//	   └─────────────────────────────────────────────────────┘

const (
	hallWall     = 0.3 // the walls' thickness
	leavesSide   = 3   // door leaves on each side
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
	return math.Hypot(hx, hz+r.Hall.Door[0]/leavesSide/2)
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
	hx, hz := r.half()
	c := r.Center
	top := r.WallH + r.Hall.Rise
	p := func(x, z float64) vec3 { return c.Add(vec3{x, 0, z}) }
	dw := r.Hall.Door[0] / 2
	return []Segment{
		{p(-hx, -hz), p(-hx, hz), top},
		{p(-hx, -hz), p(hx, -hz), top},
		{p(-hx, hz), p(hx, hz), top},
		{p(hx, -hz), p(hx, -dw), top},
		{p(hx, dw), p(hx, hz), top},
	}
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
	lw := dw / (2 * leavesSide)
	var out []Leaf
	for _, s := range []float64{-1, 1} {
		for k := range leavesSide {
			shut := s * (dw/2 - (float64(k)+0.5)*lw)
			away := s * (hz + 0.5 + lw/2)
			x := hx + hallWall/2 + 0.3 + 0.32*float64(leavesSide-1-k)
			z := shut + (away-shut)*smooth(open)
			out = append(out, Leaf{r.Center.Add(vec3{x, dh / 2, z}), vec3{0.22, dh, lw + 0.15}})
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

// Bay is where a driven vehicle parks in the garage: its spot on the floor
// (local), nose toward the door (+X), and the width marked for it.
type Bay struct {
	ID    string
	At    vec3
	Width float64
}

// layBays gives each driven vehicle a bay in the first garage, side by
// side along the back wall, a second row in front if they don't fit.
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
	var rows [][]Bay
	var row []Bay
	width := 0.0
	for _, v := range c.Vehicles.All() {
		if v.Auto() {
			continue
		}
		w := 2*content.OrZero(v.Radius, 1.2) + 3
		if width+w > 2*hz-2 && len(row) > 0 {
			rows, row, width = append(rows, row), nil, 0
		}
		row = append(row, Bay{ID: v.ID, Width: w})
		width += w
	}
	rows = append(rows, row)
	var out []Bay
	for k, row := range rows {
		width := 0.0
		for _, b := range row {
			width += b.Width
		}
		z := -width / 2
		x := -hx + 7 + float64(k)*12
		for _, b := range row {
			b.At = r.Center.Add(vec3{x, 0, z + b.Width/2})
			z += b.Width
			out = append(out, b)
		}
	}
	return out
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
	panels(p(-hx, 0, -hz), p(-hx, 0, hz), 0, r.WallH)
	panels(p(-hx, 0, -hz), p(hx, 0, -hz), 0, r.WallH)
	panels(p(-hx, 0, hz), p(hx, 0, hz), 0, r.WallH)
	panels(p(hx, 0, -hz), p(hx, 0, -dw/2), 0, r.WallH)
	panels(p(hx, 0, dw/2), p(hx, 0, hz), 0, r.WallH)
	panels(p(hx, 0, -dw/2), p(hx, 0, dw/2), dh, r.WallH)

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
	for k := 0; k <= ribs; k++ {
		z := math.Max(-hz+0.3, math.Min(hz-0.3, -hz+2*hz*float64(k)/float64(ribs)))
		for _, s := range []float64{-1, 1} {
			if s > 0 && math.Abs(z) < dw/2+0.5 {
				continue // the door
			}
			x := s * hx
			sun("steel").Box(p(x+s*0.3, r.WallH/2, z), vec3{0.3, r.WallH, 0.6}, X, 0.04)
			inside("steel").Box(p(x-s*0.35, r.WallH/2, z), vec3{0.4, r.WallH, 0.45}, X, 0.04)
		}
	}
	for _, s := range []float64{-1, 1} {
		sun("steel.dark").Box(p(-hx-0.05, 0.2, 0), vec3{0.42, 0.4, 2*hz + 0.4}, X, 0.03)
		sun("steel.dark").Box(p(0, 0.2, s*(hz+0.05)), vec3{2*hx + 0.4, 0.4, 0.42}, X, 0.03)
	}

	// Inside: the two-tone base of the walls and the accent strip.
	strip := func(from, to vec3, nrm vec3) {
		d := to.Sub(from)
		mid := from.Lerp(to, 0.5).Add(nrm.Scale(hallWall/2 + 0.03))
		inside("wall.lower").Box(mid.Add(vec3{0, 0.6, 0}), vec3{d.Length(), 1.2, 0.05}, d, 0.01)
		inside(bp.Accent).Box(mid.Add(vec3{0, 1.24, 0}).Add(nrm.Scale(0.02)), vec3{d.Length(), 0.08, 0.04}, d, 0)
	}
	strip(p(-hx, 0, -hz), p(-hx, 0, hz), X)
	strip(p(-hx, 0, -hz), p(hx, 0, -hz), Z)
	strip(p(-hx, 0, hz), p(hx, 0, hz), Z.Scale(-1))
	strip(p(hx, 0, -hz), p(hx, 0, -dw/2), X.Scale(-1))
	strip(p(hx, 0, dw/2), p(hx, 0, hz), X.Scale(-1))

	// The lamps: a housing on a rod from the vault.
	for _, l := range r.hallLamps() {
		top := r.vault(l.At.X - c.X)
		inside("steel").Prism(l.At.Add(vec3{0, 0.25, 0}), yUp, 0.04, 0.04, top-(l.At.Y)-0.2, 6, 0)
		inside("steel.dark").Prism(l.At.Sub(vec3{0, 0.05, 0}), yUp, 0.75, 0.3, 0.45, 16, 0.03)
		inside("lamp.warm").Prism(l.At.Sub(vec3{0, 0.08, 0}), yUp, 0.6, 0.6, 0.04, 16, 0)
	}

	// The door: a hazard header over the opening, jambs, rails on the
	// ground, a gantry over the leaves' path and the pockets' posts.
	sun("hazard").Box(p(hx+0.2, dh+0.25, 0), vec3{0.5, 0.5, dw + 0.6}, X, 0.04)
	for _, s := range []float64{-1, 1} {
		sun("steel").Box(p(hx+0.25, dh/2, s*(dw/2+0.2)), vec3{0.6, dh, 0.4}, X, 0.04)
	}
	lw := dw / (2 * leavesSide)
	reach := hz + 0.8 + lw
	railX := hx + hallWall/2 + 0.3
	for k := range leavesSide {
		x := railX + 0.32*float64(k)
		sun("steel.dark").Box(p(x, 0.04, 0), vec3{0.12, 0.08, 2 * reach}, X, 0.01)
	}
	gantry := p(railX+0.32, dh+0.45, 0)
	sun("steel").Box(gantry, vec3{1.2, 0.5, 2*reach + 0.4}, X, 0.04)
	sun("hazard").Box(gantry.Add(vec3{0.61, 0, 0}), vec3{0.02, 0.3, 2 * reach}, X, 0)
	for _, s := range []float64{-1, 1} {
		for _, z := range []float64{hz + 0.4, reach} {
			sun("steel").Box(p(railX+0.32, (dh+0.7)/2, s*z), vec3{1.2, dh + 0.7, 0.35}, X, 0.04)
		}
		sun("lamp.warm").Box(p(hx+0.5, dh+0.05, s*(dw/2-1)), vec3{0.3, 0.06, 1.2}, X, 0)
	}

	// The bays: yellow lines on the floor, and a stop bar at the back.
	for _, bay := range Bays {
		w := bay.Width - 0.6
		at := bay.At.Add(vec3{0, FloorTop + 0.005, 0})
		for _, s := range []float64{-1, 1} {
			inside("pipe.yellow").Box(at.Add(vec3{0, 0, s * w / 2}), vec3{11, 0.012, 0.15}, X, 0)
		}
		inside("pipe.yellow").Box(at.Add(vec3{5.5, 0, 0}), vec3{0.15, 0.012, w}, X, 0)
		inside("hazard").Box(at.Add(vec3{-5.6, 0.1, 0}), vec3{0.3, 0.2, w * 0.6}, X, 0.03)
	}
}
