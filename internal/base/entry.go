package base

import "math"

// The entrance: the airlock's corridor wrapped in a taller volume clad in
// the ceramic fired on site, the warm of the living spaces showing from
// outside; a canopy over the door, a forecourt of concrete slabs before
// it. Its front stands a little ahead of the door, which slides into the
// pockets behind it.
//
//	hall wall ║  ┌──────────┐
//	          ║  │ corridor │▐ door   canopy ▔▔▔▔
//	          ║  └──────────┘
//	              entry volume       forecourt ▦▦▦

const (
	entryHalf   = 2.5  // half the entry's width, to its outer faces
	entryH      = 4.0  // its height
	entryFront  = 0.3  // how far its front stands ahead of the door
	canopyDepth = 2.3  // how far the canopy reaches out
	canopyY     = 2.95 // the canopy's underside
)

// entryBox is the entry's footprint (local): from the hall's wall at the
// back to its front, and the way out.
func entryBox() (back, front, out vec3) {
	_, hub := Hub()
	out = vec3{0, 0, -1}
	back = hub.Center.Add(out.Scale(edge(hub.plan, out) + hallWall/2))
	return vec3{AirlockEnd.X, 0, back.Z}, AirlockEnd.Add(out.Scale(entryFront)), out
}

// entryWalls are its walls, for collisions: its sides, and its front
// beside the door.
func entryWalls() []Segment {
	back, front, out := entryBox()
	side := vec3{-out.Z, 0, out.X}
	var walls []Segment
	for _, s := range []float64{-1, 1} {
		e := side.Scale(s * entryHalf)
		walls = append(walls, Segment{back.Add(e), front.Add(e), entryH},
			Segment{front.Add(side.Scale(s * (DoorWidth/2 + 0.4))), front.Add(e), entryH})
	}
	return walls
}

// entry builds the entrance.
func (b *building) entry() {
	back, front, out := entryBox()
	side := vec3{-out.Z, 0, out.X}
	depth := front.Sub(back).Length()
	mid := back.Lerp(front, 0.5)
	const th = 0.3
	jamb := DoorWidth/2 + 0.4 // the airlock's frame stands within
	// Its sides and its front, in ceramic panels, a dark plinth at the
	// foot; a white roof with a steel coping.
	for _, s := range []float64{-1, 1} {
		wall := mid.Add(side.Scale(s * (entryHalf - th/2)))
		n := int(math.Round(depth / 1.4))
		for i := range n {
			at := back.Add(out.Scale((float64(i) + 0.5) * depth / float64(n))).Add(side.Scale(s * (entryHalf - th/2)))
			b.out([]string{"ceramic.sand", "ceramic"}[i%2]).Box(at.Add(vec3{0, entryH / 2, 0}), vec3{depth/float64(n) - 0.02, entryH, th}, out, 0.03)
		}
		b.out("steel.dark").Box(wall.Add(side.Scale(s*0.03)).Add(vec3{0, 0.18, 0}), vec3{depth, 0.36, th + 0.06}, out, 0.02)
		w := entryHalf - jamb
		cheek := front.Add(side.Scale(s * (jamb + w/2))).Sub(out.Scale(th / 2))
		b.out("ceramic.sand").Box(cheek.Add(vec3{0, entryH / 2, 0}), vec3{w, entryH, th}, side, 0.03)
		b.out("steel.dark").Box(cheek.Add(out.Scale(0.03)).Add(vec3{0, 0.18, 0}), vec3{w, 0.36, th + 0.06}, side, 0.02)
	}
	lintel := front.Sub(out.Scale(th / 2))
	b.out("ceramic").Box(lintel.Add(vec3{0, (entryH + DoorHeight + 0.45) / 2, 0}), vec3{2 * jamb, entryH - DoorHeight - 0.45, th}, side, 0.03)
	b.out("habitat.white").Box(mid.Add(vec3{0, entryH + 0.1, 0}), vec3{2*entryHalf + 0.2, 0.2, depth + 0.2}, side, 0.03)
	b.out("steel").Box(front.Add(vec3{0, entryH + 0.25, 0}).Sub(out.Scale(0.05)), vec3{2*entryHalf + 0.3, 0.12, 0.2}, side, 0.02)

	// The canopy: a thin white plate on a steel fascia, a warm line of
	// light under its edge.
	c := front.Add(out.Scale(canopyDepth / 2)).Add(vec3{0, canopyY + 0.09, 0})
	b.out("habitat.white").Box(c, vec3{2*entryHalf + 0.6, 0.18, canopyDepth}, side, 0.02)
	b.out("steel").Box(c.Add(out.Scale(canopyDepth/2)).Add(vec3{0, 0.02, 0}), vec3{2*entryHalf + 0.6, 0.3, 0.1}, side, 0.02)
	b.out("lamp.warm").Box(c.Add(out.Scale(canopyDepth/2-0.35)).Sub(vec3{0, 0.1, 0}), vec3{2 * entryHalf, 0.02, 0.12}, side, 0)

	// The forecourt: concrete slabs with dark joints, from the door out.
	const slab, cols, rows = 1.5, 4, 3
	for i := range cols {
		for j := range rows {
			at := front.Add(side.Scale((float64(i) - float64(cols-1)/2) * slab)).Add(out.Scale(0.1 + (float64(j)+0.5)*slab))
			b.out("pad.concrete").Box(at.Add(vec3{0, 0.025, 0}), vec3{slab - 0.05, 0.05, slab - 0.05}, side, 0.01)
		}
	}
}
