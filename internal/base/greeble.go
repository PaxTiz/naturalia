package base

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/mathx"
	"naturalia/internal/mesh"
)

// Greebles: the small machinery that makes the base look built and lived
// in. Conduits, vents, portholes, ladders, lockers, shelves and screens are
// scattered on the walls by rules and a seed, so two rooms with the same
// plan never look alike; tanks, barrels, a container, floodlights and a
// landing pad fill the yard. Everything is merged into the base's meshes
// (building.part): it costs nothing per frame.

// grnd is a cheap deterministic random number in [0, 1).
func grnd(seed, k int) float64 {
	v := math.Sin(float64(seed*131+k*17)+0.5) * 43758.5453
	return v - math.Floor(v)
}

var yUp = vec3{0, 1, 0}

// greebleRoom dresses one room, outside and inside.
func (b *building) greebleRoom(bp *content.Blueprint, r Room, seed int) {
	panels := r.panels()
	ladder := false
	for i, p := range panels {
		if p.y0 > 0 {
			continue // the doorway: its frame is enough
		}
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		along := vec3{-out.Z, 0, out.X}
		outer := p.mid.Add(out.Scale(WallThick / 2))
		inner := p.mid.Sub(out.Scale(WallThick/2 + 0.04)) // in front of the wainscot
		nextToDoor := false
		for _, o := range r.openings {
			if angleDist(p.angle, o) < (DoorWidth/2+1.3)/r.Radius {
				nextToDoor = true
			}
		}
		blocked := false
		for _, pr := range bp.Props {
			if d := bp.At.Add(pr.At).Sub(inner); math.Hypot(d.X, d.Z) < 1.7 {
				blocked = true
			}
		}
		kind := int(grnd(seed, i) * 6)
		if kind == 4 && (ladder || nextToDoor) {
			kind = 5
		}
		ladder = ladder || kind == 4

		// Outside: an anchor foot at each joint, two conduits running
		// around the wall, and the panel's own piece.
		b.Part("steel", mesh.LitSun, vec3{}).Box(outer.Add(along.Scale(p.width/2)).Add(out.Scale(0.12)).Add(vec3{0, 0.14, 0}), vec3{0.22, 0.28, 0.3}, along, 0.03)
		pipeRun(b.Part("pipe.yellow", mesh.LitSun, vec3{}), outer.Add(out.Scale(0.1)).Add(vec3{0, 2.02, 0}), along, p.width, 0.045)
		pipeRun(b.Part("pipe.blue", mesh.LitSun, vec3{}), outer.Add(out.Scale(0.09)).Add(vec3{0, 1.88, 0}), along, p.width, 0.035)
		b.Part("steel.dark", mesh.LitSun, vec3{}).Box(outer.Add(out.Scale(0.07)).Add(vec3{0, 1.95, 0}), vec3{0.08, 0.3, 0.14}, along, 0.015)
		b.outsidePiece(kind, outer, out, along, p.width, seed+i)

		// Inside: conduits under the dome, then the panel's piece.
		ceil := r.WallH - 0.28
		pipeRun(b.Part("steel", mesh.LitIndoor, vec3{}), inner.Sub(out.Scale(0.1)).Add(vec3{0, ceil, 0}), along, p.width, 0.05)
		pipeRun(b.Part("pipe.blue", mesh.LitIndoor, vec3{}), inner.Sub(out.Scale(0.22)).Add(vec3{0, ceil + 0.1, 0}), along, p.width, 0.04)
		b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(inner.Sub(out.Scale(0.14)).Add(vec3{0, ceil + 0.05, 0}), vec3{0.08, 0.1, 0.32}, along, 0.015)
		if !blocked {
			b.insidePiece(kind, inner, out, along, seed+i, bp.Accent)
		}
	}

	// The dome: ribs outside, a hatch and a vent on top.
	top := r.Center.Add(vec3{0, r.WallH, 0})
	a, bh := r.Radius+0.12, r.Radius*0.55
	// A raised seam along every gore, wider steel ribs on every other one,
	// and two bands around. Each stands off the shell: sharing its surface,
	// they would fight it for the depth and flicker, and poking through it
	// they would show inside as dashes.
	for k := range mesh.DomeGores {
		th := float64(k) * 2 * math.Pi / mesh.DomeGores
		if k%2 == 1 {
			domeRib(b.Part("steel", mesh.LitSun, vec3{}), top, a, bh, th, 0.1, 0.06)
		} else {
			domeRib(b.Part("habitat.white2", mesh.LitSun, vec3{}), top, a, bh, th, 0.05, 0.03)
		}
	}
	for _, phi := range []float64{0.42, 0.85} {
		r, y := a*math.Cos(phi)+0.035*math.Cos(phi), (bh+0.035)*math.Sin(phi)
		ring := b.Part("habitat.white2", mesh.LitSun, vec3{})
		for k := range mesh.DomeGores {
			t0, t1 := float64(k)*2*math.Pi/mesh.DomeGores, float64(k+1)*2*math.Pi/mesh.DomeGores
			p0 := top.Add(vec3{math.Cos(t0) * r, y, math.Sin(t0) * r})
			p1 := top.Add(vec3{math.Cos(t1) * r, y, math.Sin(t1) * r})
			along := p1.Sub(p0)
			out := vec3{math.Cos((t0 + t1) / 2), 0, math.Sin((t0 + t1) / 2)}
			nrm := out.Scale(math.Cos(phi) / a).Add(vec3{0, math.Sin(phi) / bh, 0}).Norm()
			ax := along.Norm()
			ring.BoxBasis(p0.Lerp(p1, 0.5), [3]vec3{ax, nrm.Cross(ax), nrm}, vec3{along.Length() + 0.04, 0.06, 0.03}, 0.01)
		}
	}
	apex := top.Add(vec3{0, bh + 0.01, 0}) // on the shell, not through it: it would show inside
	b.Part("steel", mesh.LitSun, vec3{}).Prism(apex, yUp, 0.4, 0.62, 0.16, 16, 0.03)
	b.Part("habitat.white2", mesh.LitSun, vec3{}).Prism(apex.Add(vec3{0, 0.16, 0}), yUp, 0.5, 0.42, 0.1, 16, 0.02)
	b.Part("steel.dark", mesh.LitSun, vec3{}).Box(apex.Add(vec3{0.3, 0.3, 0}), vec3{0.3, 0.12, 0.3}, vec3{1, 0, 0}, 0.02)
}

// pipeRun lays a pipe along a wall panel, its middle at c.
func pipeRun(m *mesh.Builder, c, along vec3, width, radius float64) {
	m.Prism(c.Sub(along.Scale(width/2)), along, radius, radius, width, 8, 0)
}

// domeRib follows the outside of a dome (radius a, height bh) along one
// meridian. Its straight pieces cut under the curve between their ends, so
// they ride high enough above the shell that their underside clears it.
func domeRib(m *mesh.Builder, top vec3, a, bh, th, w, d float64) {
	out := vec3{math.Cos(th), 0, math.Sin(th)}
	side := vec3{-out.Z, 0, out.X}
	const steps = 16
	lift := 0.015 + d/2 + 0.012 // gap, half the depth, the pieces' sag
	pt := func(i int) vec3 {
		phi := float64(i) / steps * 1.5
		cp, sp := math.Cos(phi), math.Sin(phi)
		n := out.Scale(cp / a).Add(vec3{0, sp / bh, 0}).Norm()
		return top.Add(out.Scale(a * cp)).Add(vec3{0, bh * sp, 0}).Add(n.Scale(lift))
	}
	for i := range steps {
		p0, p1 := pt(i), pt(i+1)
		along := p1.Sub(p0)
		l := along.Length()
		along = along.Scale(1 / l)
		m.BoxBasis(p0.Lerp(p1, 0.5), [3]vec3{side, along, side.Cross(along)}, vec3{w, l + 0.02, d}, 0.015)
	}
}

// porthole puts a round window on a wall face at c, looking along n.
func (b *building) porthole(c, n vec3, lit float32) {
	b.Part("steel", lit, vec3{}).Prism(c, n, 0.34, 0.31, 0.07, 18, 0.02)
	b.Part("glass.dark", lit, vec3{}).Prism(c, n, 0.25, 0.25, 0.085, 18, 0)
	for k := range 4 {
		a := float64(k)*math.Pi/2 + math.Pi/4
		t1, t2 := mathx.TangentBasis(n)
		p := c.Add(t1.Scale(math.Cos(a) * 0.29)).Add(t2.Scale(math.Sin(a) * 0.29))
		b.Part("steel.dark", lit, vec3{}).Prism(p, n, 0.025, 0.025, 0.09, 6, 0)
	}
}

// outsidePiece is what a wall panel carries outside.
func (b *building) outsidePiece(kind int, outer, out, along vec3, width float64, seed int) {
	at := func(x, y, o float64) vec3 {
		return outer.Add(along.Scale(x)).Add(vec3{0, y, 0}).Add(out.Scale(o))
	}
	switch kind {
	case 0: // a porthole, seen from both sides (insidePiece draws the other)
		b.porthole(at(0, 1.5, 0), out, mesh.LitSun)
	case 1: // a vent with slats
		b.Part("steel.dark", mesh.LitSun, vec3{}).Box(at(0, 0.85, 0.06), vec3{0.62, 0.48, 0.12}, along, 0.02)
		for k := range 4 {
			b.Part("steel", mesh.LitSun, vec3{}).Box(at(0, 0.7+float64(k)*0.1, 0.12), vec3{0.54, 0.035, 0.06}, along, 0.008)
		}
	case 2: // a junction box with its conduit, and a wall lamp
		b.Part("habitat.white2", mesh.LitSun, vec3{}).Box(at(-0.2, 1.15, 0.09), vec3{0.42, 0.56, 0.18}, along, 0.025)
		b.Part("hazard", mesh.LitSun, vec3{}).Box(at(-0.2, 1.4, 0.1), vec3{0.43, 0.07, 0.19}, along, 0.01)
		b.Part("steel", mesh.LitSun, vec3{}).Prism(at(-0.2, 1.43, 0.09), yUp, 0.025, 0.025, 0.5, 6, 0)
		b.Part("led.green", mesh.LitSun, vec3{}).Box(at(-0.08, 1.0, 0.185), vec3{0.04, 0.04, 0.01}, along, 0)
		b.Part("steel.dark", mesh.LitSun, vec3{}).Box(at(0.35, 2.35, 0.12), vec3{0.26, 0.1, 0.24}, along, 0.02)
		b.Part("lamp.warm", mesh.LitSun, vec3{}).Box(at(0.35, 2.295, 0.13), vec3{0.2, 0.015, 0.17}, along, 0)
	case 3: // a pipe going down into the ground, with a valve
		x := width * 0.28
		b.Part("pipe.yellow", mesh.LitSun, vec3{}).Prism(at(x, 0, 0.1), yUp, 0.045, 0.045, 2.02, 8, 0)
		b.Part("steel", mesh.LitSun, vec3{}).Box(at(x, 2.02, 0.1), vec3{0.12, 0.12, 0.12}, along, 0.02)
		b.Part("steel", mesh.LitSun, vec3{}).Prism(at(x, 0, 0.1), yUp, 0.08, 0.08, 0.12, 8, 0.02)
		b.Part("pipe.red", mesh.LitSun, vec3{}).Prism(at(x, 1.05, 0.15), out, 0.13, 0.13, 0.03, 10, 0)
		b.Part("steel.dark", mesh.LitSun, vec3{}).Prism(at(x, 1.05, 0.1), out, 0.03, 0.03, 0.08, 6, 0)
	case 4: // a ladder up to the dome
		for _, s := range []float64{-0.22, 0.22} {
			b.Part("steel", mesh.LitSun, vec3{}).Box(at(s, 1.3, 0.16), vec3{0.05, 2.6, 0.05}, along, 0.01)
			b.Part("steel.dark", mesh.LitSun, vec3{}).Box(at(s, 2.3, 0.08), vec3{0.06, 0.06, 0.16}, along, 0.01)
		}
		for k := range 8 {
			b.Part("steel", mesh.LitSun, vec3{}).Box(at(0, 0.3+float64(k)*0.3, 0.16), vec3{0.44, 0.035, 0.035}, along, 0.008)
		}
	}
	_ = seed
}

// insidePiece is what a wall panel carries inside the room.
func (b *building) insidePiece(kind int, inner, out, along vec3, seed int, accent string) {
	in := out.Scale(-1)
	at := func(x, y, o float64) vec3 {
		return inner.Add(along.Scale(x)).Add(vec3{0, y, 0}).Add(in.Scale(o))
	}
	switch kind {
	case 0: // the porthole's inner side
		b.porthole(inner.Add(out.Scale(0.04)).Add(vec3{0, 1.5, 0}), in, mesh.LitIndoor)
	case 1: // a locker, with its doors, handles, vents and a status light
		tok := "locker"
		if seed%2 == 0 {
			tok = "locker.ochre"
		}
		b.Part(tok, mesh.LitIndoor, vec3{}).Box(at(0, 1.0, 0.2), vec3{0.8, 2.0, 0.4}, along, 0.03)
		b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at(0, 1.0, 0.401), vec3{0.015, 1.86, 0.01}, along, 0)
		for _, s := range []float64{-0.06, 0.06} {
			b.Part("steel", mesh.LitIndoor, vec3{}).Box(at(s, 1.05, 0.41), vec3{0.025, 0.18, 0.03}, along, 0.005)
		}
		for k := range 3 {
			for _, s := range []float64{-0.2, 0.2} {
				b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at(s, 1.75+float64(k)*0.06, 0.401), vec3{0.22, 0.02, 0.01}, along, 0)
			}
		}
		b.Part(accent, mesh.LitIndoor, vec3{}).Box(at(0.3, 1.9, 0.402), vec3{0.08, 0.03, 0.01}, along, 0)
	case 2: // a shelf of boxes and canisters
		for _, s := range []float64{-0.38, 0.38} {
			b.Part("steel", mesh.LitIndoor, vec3{}).Box(at(s, 1.65, 0.15), vec3{0.04, 0.55, 0.3}, along, 0.008)
		}
		for j, y := range []float64{1.4, 1.85} {
			b.Part("steel", mesh.LitIndoor, vec3{}).Box(at(0, y, 0.17), vec3{0.9, 0.035, 0.34}, along, 0.01)
			x := -0.36
			for k := 0; x < 0.3; k++ {
				r := grnd(seed, j*10+k)
				if r < 0.3 { // a canister
					tok := []string{"pipe.red", "pipe.blue", "pipe.yellow"}[k%3]
					b.Part(tok, mesh.LitIndoor, vec3{}).Prism(at(x+0.07, y+0.0175, 0.17), yUp, 0.065, 0.065, 0.18+0.1*r, 10, 0.015)
					x += 0.17
				} else {
					s := 0.14 + 0.12*r
					tok := []string{"crate.ochre", "crate.slate", "crate.sand"}[(seed+k)%3]
					b.Part(tok, mesh.LitIndoor, vec3{}).Box(at(x+s/2, y+0.0175+s*0.4, 0.17), vec3{s, s * 0.8, 0.26}, along, 0.015)
					x += s + 0.04
				}
			}
		}
	case 3: // a wall screen with a keypad
		b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at(0, 1.6, 0.04), vec3{0.74, 0.48, 0.07}, along, 0.02)
		b.Part("screen.dim", mesh.LitIndoor, vec3{}).Box(at(0, 1.62, 0.076), vec3{0.62, 0.36, 0.01}, along, 0)
		b.Part(accent, mesh.LitIndoor, vec3{}).Box(at(-0.22, 1.73, 0.083), vec3{0.12, 0.03, 0.005}, along, 0)
		b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at(0.25, 1.15, 0.035), vec3{0.18, 0.24, 0.06}, along, 0.015)
		b.Part("led.green", mesh.LitIndoor, vec3{}).Box(at(0.25, 1.22, 0.066), vec3{0.04, 0.03, 0.005}, along, 0)
	case 4, 5: // a vent high on the wall and a pair of pipes, or an extinguisher
		if kind == 5 && seed%3 == 0 {
			b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at(0, 1.35, 0.03), vec3{0.12, 0.5, 0.05}, along, 0.01)
			b.Part("pipe.red", mesh.LitIndoor, vec3{}).Prism(at(0, 1.08, 0.12), yUp, 0.08, 0.08, 0.48, 10, 0.03)
			b.Part("steel.dark", mesh.LitIndoor, vec3{}).Prism(at(0, 1.56, 0.12), yUp, 0.03, 0.03, 0.07, 6, 0)
			b.Part("hazard", mesh.LitIndoor, vec3{}).Box(at(0, 1.85, 0.02), vec3{0.3, 0.12, 0.02}, along, 0.005)
			return
		}
		b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at(0, 2.05, 0.04), vec3{0.6, 0.34, 0.07}, along, 0.02)
		for k := range 3 {
			b.Part("steel", mesh.LitIndoor, vec3{}).Box(at(0, 1.95+float64(k)*0.09, 0.08), vec3{0.52, 0.03, 0.04}, along, 0.006)
		}
		for _, s := range []float64{0.48, 0.6} {
			b.Part("pipe.blue", mesh.LitIndoor, vec3{}).Prism(at(s, 0, 0.08), yUp, 0.035, 0.035, 2.4, 8, 0)
		}
	}
}

// greebleCorridor: conduits outside, pipes and vents along the ceiling.
func (b *building) greebleCorridor(c Corridor, seed int) {
	mid := c.From.Lerp(c.To, 0.5)
	dir := c.To.Sub(c.From)
	length := dir.Length()
	dir = dir.Scale(1 / length)
	side := c.side()
	for _, s := range []float64{-1, 1} {
		wall := mid.Add(side.Scale(s * (c.Width/2 + WallThick)))
		pipeRun(b.Part("pipe.yellow", mesh.LitSun, vec3{}), wall.Add(side.Scale(s*0.1)).Add(vec3{0, 1.95, 0}), dir, length, 0.045)
		pipeRun(b.Part("pipe.blue", mesh.LitSun, vec3{}), wall.Add(side.Scale(s*0.09)).Add(vec3{0, 1.8, 0}), dir, length, 0.035)
		in := mid.Add(side.Scale(s * (c.Width/2 - 0.12)))
		pipeRun(b.Part("steel", mesh.LitIndoor, vec3{}), in.Add(vec3{0, c.Height - 0.18, 0}), dir, length*0.98, 0.05)
		pipeRun(b.Part("pipe.blue", mesh.LitIndoor, vec3{}), in.Add(side.Scale(-s*0.13)).Add(vec3{0, c.Height - 0.12, 0}), dir, length*0.98, 0.04)
	}
	n := int(length / 1.6)
	for i := range n {
		at := c.From.Add(dir.Scale((float64(i) + 0.5) * length / float64(n)))
		s := 1.0
		if grnd(seed, i) < 0.5 {
			s = -1
		}
		face := at.Add(side.Scale(s * (c.Width/2 - 0.04)))
		switch int(grnd(seed, i+50) * 3) {
		case 0:
			b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(face.Add(vec3{0, 1.6, 0}), vec3{0.5, 0.32, 0.06}, dir, 0.015)
			b.Part("screen.dim", mesh.LitIndoor, vec3{}).Box(face.Add(vec3{0, 1.6, 0}).Sub(side.Scale(s*0.032)), vec3{0.42, 0.24, 0.01}, dir, 0)
		case 1:
			b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(face.Add(vec3{0, 0.45, 0}), vec3{0.5, 0.3, 0.06}, dir, 0.015)
			for k := range 3 {
				b.Part("steel", mesh.LitIndoor, vec3{}).Box(face.Add(vec3{0, 0.37 + float64(k)*0.08, 0}).Sub(side.Scale(s*0.035)), vec3{0.44, 0.025, 0.03}, dir, 0.005)
			}
		}
	}
}

// yard furnishes the ground around the base. yardObstacles below must
// match it.
func (b *building) yard() {
	sun := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitSun, vec3{}) }

	// A landing pad: concrete plates, a hazard rim, a white ring and lights.
	pad := vec3{11, 0, -12.5}
	if yardFree(pad, 3.6) {
		b.landingPad(pad)
	}

	// Water tanks on cradles, banded, with a pipe to the airlock.
	if yardFree(yardTanks, 2.6) {
		b.waterTanks()
	}

	// Barrels, standing and one fallen.
	for c, cl := range yardBarrels {
		if !yardFree(cl, 1.2) {
			continue
		}
		for k := range 5 {
			a := float64(k)*1.3 + float64(c)
			r := 0.35 + 0.35*grnd(c, k)
			p := vec3{cl.X + math.Cos(a)*r, 0, cl.Z + math.Sin(a)*r}
			tok := []string{"barrel.blue", "barrel.ochre", "pipe.red"}[(k+c)%3]
			if k == 4 {
				axis := vec3{math.Cos(a + 1), 0, math.Sin(a + 1)}
				p = p.Add(vec3{math.Cos(a), 0, math.Sin(a)}.Scale(0.5)).Add(vec3{0, 0.29, 0})
				sun(tok).Prism(p.Sub(axis.Scale(0.45)), axis, 0.29, 0.29, 0.9, 14, 0.03)
				continue
			}
			sun(tok).Prism(p, yUp, 0.29, 0.29, 0.9, 14, 0.03)
			for _, y := range []float64{0.22, 0.66} {
				sun("steel").Prism(p.Add(vec3{0, y, 0}), yUp, 0.305, 0.305, 0.04, 14, 0)
			}
		}
	}

	// A cargo container: corrugated sides, corner posts, hazard doors.
	if yardFree(yardContainer, 2.5) {
		c := yardContainer
		ang := 0.55
		ax := vec3{math.Cos(ang), 0, math.Sin(ang)}
		sd := vec3{-ax.Z, 0, ax.X}
		sun("crate.slate").Box(c.Add(vec3{0, 1.05, 0}), vec3{4.4, 2.1, 2.1}, ax, 0.05)
		for k := -10; k <= 10; k++ {
			for _, s := range []float64{-1, 1} {
				sun("crate.slate").Box(c.Add(ax.Scale(float64(k)*0.2)).Add(sd.Scale(s*1.06)).Add(vec3{0, 1.05, 0}), vec3{0.08, 1.9, 0.04}, ax, 0.01)
			}
		}
		for _, x := range []float64{-1, 1} {
			for _, s := range []float64{-1, 1} {
				sun("steel").Box(c.Add(ax.Scale(x*2.17)).Add(sd.Scale(s*1.02)).Add(vec3{0, 1.05, 0}), vec3{0.14, 2.14, 0.14}, ax, 0.02)
			}
		}
		end := c.Add(ax.Scale(2.21))
		sun("hazard").Box(end.Add(vec3{0, 1.9, 0}), vec3{0.04, 0.16, 1.9}, ax, 0)
		for _, s := range []float64{-0.45, 0.45} {
			sun("steel").Box(end.Add(sd.Scale(s)).Add(vec3{0, 1.0, 0}), vec3{0.05, 1.7, 0.05}, ax, 0.01)
		}
	}

	// Floodlight masts.
	for _, m := range yardMasts {
		if !yardFree(m, 0.5) {
			continue
		}
		sun("steel.dark").Box(m.Add(vec3{0, 0.2, 0}), vec3{0.5, 0.4, 0.5}, vec3{1, 0, 0}, 0.04)
		sun("steel").Prism(m, yUp, 0.08, 0.06, 4.6, 8, 0)
		in := vec3{-m.X, 0, -m.Z}.Norm()
		head := m.Add(vec3{0, 4.6, 0}).Add(in.Scale(0.2))
		sun("steel.dark").Box(head, vec3{0.7, 0.28, 0.3}, vec3{-in.Z, 0, in.X}, 0.03)
		sun("lamp.warm").Box(head.Add(vec3{0, -0.15, 0}).Add(in.Scale(0.03)), vec3{0.6, 0.02, 0.22}, vec3{-in.Z, 0, in.X}, 0)
	}

	// Cables on the ground, from the solar panels to the sale room.
	for _, seg := range [][2]vec3{{{-12, 0.03, -9}, {-12, 0.03, -4.6}}, {{-12, 0.03, -4.6}, {-13, 0.03, -4.0}}} {
		d := seg[1].Sub(seg[0])
		sun("rubber").Box(seg[0].Lerp(seg[1], 0.5), vec3{0.07, 0.06, d.Length()}, vec3{d.Z, 0, -d.X}.Norm(), 0.02)
	}
}

func (b *building) landingPad(pad vec3) {
	sun := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitSun, vec3{}) }
	sun("hazard").Prism(pad, yUp, 3.6, 3.5, 0.06, 6, 0.02)
	sun("pad.concrete").Prism(pad, yUp, 3.3, 3.3, 0.09, 6, 0.01)
	sun("habitat.white").Prism(pad, yUp, 1.3, 1.3, 0.095, 24, 0)
	sun("pad.concrete").Prism(pad, yUp, 1.05, 1.05, 0.1, 24, 0)
	for k := range 6 {
		a := float64(k) * math.Pi / 3
		p := pad.Add(vec3{math.Cos(a) * 3.45, 0, math.Sin(a) * 3.45})
		sun("steel.dark").Prism(p, yUp, 0.12, 0.1, 0.12, 8, 0.02)
		sun("lamp.warm").Prism(p.Add(vec3{0, 0.12, 0}), yUp, 0.07, 0.07, 0.03, 8, 0)
	}

}

func (b *building) waterTanks() {
	sun := func(tok string) *mesh.Builder { return b.Part(tok, mesh.LitSun, vec3{}) }
	for j, z := range []float64{-14.2, -12.4} {
		start := vec3{-6.8, 0.95, z}
		x := vec3{1, 0, 0}
		sun("habitat.white").Prism(start, x, 0.78, 0.78, 3.6, 18, 0.06)
		for _, t := range []float64{0.3, 1.8, 3.3} {
			sun("steel").Prism(start.Add(x.Scale(t-0.06)), x, 0.81, 0.81, 0.12, 18, 0)
			if t != 1.8 {
				sun("steel.dark").Box(start.Add(x.Scale(t)).Add(vec3{0, -0.6, 0}), vec3{0.2, 0.7, 1.3}, x, 0.03)
			}
		}
		sun("hazard").Prism(start.Add(x.Scale(3.6)), x, 0.5, 0.45, 0.04, 18, 0)
		sun("pipe.blue").Prism(vec3{-3.2, 0.5, z}, x, 0.06, 0.06, 1.6-float64(j)*0.2, 8, 0)
		sun("pipe.blue").Prism(vec3{-1.6 - float64(j)*0.2, 0, z}, yUp, 0.06, 0.06, 0.56, 8, 0)
	}
	sun("rubber").Box(vec3{-1.7, 0.03, -11.5}, vec3{0.08, 0.06, 4}, vec3{1, 0, 0}, 0.02)
}

var (
	yardMasts     = []vec3{{-17.5, 0, 6}, {17.5, 0, 6}, {15.5, 0, -15}, {-8, 0, 17.5}}
	yardBarrels   = []vec3{{5.4, 0, -9.4}, {-9.4, 0, 8.6}}
	yardTanks     = vec3{-5, 0, -13.3}
	yardContainer = vec3{9.6, 0, 9.4}
)

// yardFree tells whether a yard object of radius r at p stays clear of the
// buildings and corridors: a mod's building takes the place of what was
// standing there.
func yardFree(p vec3, r float64) bool {
	const margin = 0.6
	flat := vec3{p.X, 0, p.Z}
	for _, rm := range Rooms {
		if flat.Sub(rm.Center).Length() < rm.Radius+r+margin {
			return false
		}
	}
	for _, c := range Corridors {
		d := c.To.Sub(c.From)
		t := mathx.Clamp01(flat.Sub(c.From).Dot(d) / d.Dot(d))
		if flat.Sub(c.From.Add(d.Scale(t))).Length() < c.Width/2+r+margin {
			return false
		}
	}
	return true
}

// YardObstacles: what the player bumps into in the yard.
func YardObstacles() []content.Obstacle {
	var out []content.Obstacle
	for _, cl := range yardBarrels {
		if yardFree(cl, 1.2) {
			out = append(out, content.Obstacle{cl, 0.95})
		}
	}
	if yardFree(yardTanks, 2.6) {
		for _, z := range []float64{-14.2, -12.4} {
			for _, x := range []float64{-6.3, -5.0, -3.7} {
				out = append(out, content.Obstacle{vec3{x, 0, z}, 0.8})
			}
		}
	}
	if c, ax := yardContainer, (vec3{math.Cos(0.55), 0, math.Sin(0.55)}); yardFree(c, 2.5) {
		for _, t := range []float64{-1.4, 0, 1.4} {
			out = append(out, content.Obstacle{c.Add(ax.Scale(t)), 1.15})
		}
	}
	for _, m := range yardMasts {
		if yardFree(m, 0.5) {
			out = append(out, content.Obstacle{m, 0.35})
		}
	}
	return out
}

// More furniture, as kit pieces (content.Kit): a chair, a workbench, a
// hydroponic planter and a desk.

// BuildChair: a swivel chair facing toward -X (rotate with the seed: the
// angle is seed × 45°).
func BuildChair(b *building, at vec3, seed int) {
	a := float64(seed) * math.Pi / 4
	f := vec3{math.Cos(a), 0, math.Sin(a)}
	sd := vec3{-f.Z, 0, f.X}
	dark, seat := b.Part("steel.dark", mesh.LitIndoor, vec3{}), b.Part("fabric.seat", mesh.LitIndoor, vec3{})
	for k := range 5 {
		t := float64(k) * 2 * math.Pi / 5
		leg := vec3{math.Cos(t), 0, math.Sin(t)}
		dark.Box(at.Add(leg.Scale(0.18)).Add(vec3{0, 0.05, 0}), vec3{0.36, 0.04, 0.05}, leg, 0.01)
	}
	dark.Prism(at, yUp, 0.03, 0.03, 0.45, 8, 0)
	seat.Box(at.Add(vec3{0, 0.5, 0}), vec3{0.5, 0.09, 0.5}, f, 0.04)
	seat.BoxBasis(at.Sub(f.Scale(0.25)).Add(vec3{0, 0.85, 0}), [3]vec3{sd, vec3{0, 0.99, 0.1}.Norm(), f}, vec3{0.46, 0.55, 0.08}, 0.04)
	dark.Box(at.Sub(f.Scale(0.22)).Add(vec3{0, 0.6, 0}), vec3{0.06, 0.2, 0.04}, sd, 0.01)
}

// BuildWorkbench: a steel bench with a pegboard, tools and a specimen jar.
func BuildWorkbench(b *building, at vec3, seed int) {
	a := float64(seed) * math.Pi / 4
	f := vec3{math.Cos(a), 0, math.Sin(a)} // toward the user
	sd := vec3{-f.Z, 0, f.X}
	steel, dark := b.Part("steel", mesh.LitIndoor, vec3{}), b.Part("steel.dark", mesh.LitIndoor, vec3{})
	b.Part("wood.brown", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{0, 0.9, 0}), vec3{1.8, 0.06, 0.75}, sd, 0.015)
	for _, x := range []float64{-0.82, 0.82} {
		for _, z := range []float64{-0.3, 0.3} {
			dark.Box(at.Add(sd.Scale(x)).Add(f.Scale(z)).Add(vec3{0, 0.44, 0}), vec3{0.06, 0.88, 0.06}, sd, 0.01)
		}
	}
	steel.Box(at.Add(vec3{0, 0.25, 0}), vec3{1.7, 0.03, 0.65}, sd, 0.01)
	b.Part("crate.slate", mesh.LitIndoor, vec3{}).Box(at.Add(sd.Scale(-0.45)).Add(vec3{0, 0.42, 0}), vec3{0.6, 0.3, 0.45}, sd, 0.02)
	board := at.Sub(f.Scale(0.38)).Add(vec3{0, 1.45, 0})
	b.Part("wall.lower", mesh.LitIndoor, vec3{}).Box(board, vec3{1.7, 0.9, 0.04}, sd, 0.015)
	for k := range 5 {
		x := -0.6 + float64(k)*0.3
		steel.Box(board.Add(sd.Scale(x)).Add(f.Scale(0.04)).Add(vec3{0, 0.1 * float64(k%2), 0}), vec3{0.04, 0.32, 0.03}, sd, 0.005)
	}
	b.Part("pipe.red", mesh.LitIndoor, vec3{}).Box(at.Add(sd.Scale(0.5)).Add(vec3{0, 0.99, 0}), vec3{0.42, 0.12, 0.22}, sd, 0.02)
	b.Part("glass.dark", mesh.LitIndoor, vec3{}).Prism(at.Add(sd.Scale(-0.1)).Add(vec3{0, 0.93, 0}), yUp, 0.1, 0.1, 0.24, 12, 0.02)
	b.Part("planter.green", mesh.LitIndoor, vec3{}).Prism(at.Add(sd.Scale(-0.1)).Add(vec3{0, 0.95, 0}), yUp, 0.07, 0.05, 0.1, 6, 0)
	steel.Prism(at.Add(sd.Scale(-0.1)).Add(vec3{0, 1.17, 0}), yUp, 0.11, 0.11, 0.03, 12, 0.01)
}

// BuildPlanter: a hydroponic trough on legs with rows of faceted plants
// and a grow lamp above.
func BuildPlanter(b *building, at vec3, seed int) {
	a := float64(seed%8) * math.Pi / 4
	f := vec3{math.Cos(a), 0, math.Sin(a)}
	sd := vec3{-f.Z, 0, f.X}
	steel, dark := b.Part("steel", mesh.LitIndoor, vec3{}), b.Part("steel.dark", mesh.LitIndoor, vec3{})
	b.Part("habitat.white2", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{0, 0.7, 0}), vec3{1.7, 0.32, 0.6}, sd, 0.04)
	b.Part("soil", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{0, 0.865, 0}), vec3{1.6, 0.02, 0.5}, sd, 0)
	for _, x := range []float64{-0.75, 0.75} {
		dark.Box(at.Add(sd.Scale(x)).Add(vec3{0, 0.27, 0}), vec3{0.06, 0.54, 0.5}, sd, 0.01)
		steel.Box(at.Add(sd.Scale(x)).Add(vec3{0, 1.35, 0}), vec3{0.04, 0.95, 0.04}, sd, 0.005)
	}
	b.Part("pipe.blue", mesh.LitIndoor, vec3{}).Prism(at.Add(sd.Scale(-0.85)).Add(f.Scale(-0.2)).Add(vec3{0, 0.75, 0}), sd, 0.03, 0.03, 1.7, 8, 0)
	lamp := at.Add(vec3{0, 1.82, 0})
	dark.Box(lamp, vec3{1.6, 0.06, 0.32}, sd, 0.015)
	b.Part("lamp.grow", mesh.LitIndoor, vec3{}).Box(lamp.Add(vec3{0, -0.035, 0}), vec3{1.5, 0.01, 0.24}, sd, 0)
	for k := range 6 {
		for j, z := range []float64{-0.12, 0.12} {
			r := grnd(seed, k*2+j)
			p := at.Add(sd.Scale(-0.65 + float64(k)*0.26)).Add(f.Scale(z)).Add(vec3{0, 0.87, 0})
			h := 0.18 + 0.22*r
			tok := "planter.green"
			if (k+j)%3 == 0 {
				tok = "planter.lime"
			}
			// A tuft of leaves arching out from the stem.
			leaf := b.Part(tok, mesh.LitIndoor, vec3{})
			leaf.Prism(p, yUp, 0.025, 0.015, h*0.6, 8, 0)
			for l := range 6 {
				t := float64(l)*2.4 + r*6
				out := vec3{math.Cos(t), 0, math.Sin(t)}
				lift := 0.5 + 0.4*grnd(seed, k*20+j*7+l)
				dirl := out.Add(vec3{0, lift * 1.6, 0}).Norm()
				length := h * (0.55 + 0.35*grnd(seed, k*30+l))
				c := p.Add(vec3{0, h * (0.15 + 0.07*float64(l)), 0}).Add(dirl.Scale(length / 2))
				across := vec3{-out.Z, 0, out.X}
				leaf.BoxBasis(c, [3]vec3{across, dirl, across.Cross(dirl)}, vec3{0.06 + 0.03*lift, length, 0.008}, 0.004)
			}
		}
	}
}

// BuildDesk: a small desk with a lamp, a terminal and a stack of books.
func BuildDesk(b *building, at vec3, seed int) {
	a := float64(seed) * math.Pi / 4
	f := vec3{math.Cos(a), 0, math.Sin(a)}
	sd := vec3{-f.Z, 0, f.X}
	b.Part("wood.brown", mesh.LitIndoor, vec3{}).Box(at.Add(vec3{0, 0.74, 0}), vec3{1.3, 0.05, 0.6}, sd, 0.015)
	for _, x := range []float64{-0.6, 0.6} {
		b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at.Add(sd.Scale(x)).Add(vec3{0, 0.36, 0}), vec3{0.05, 0.72, 0.55}, sd, 0.01)
	}
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at.Sub(f.Scale(0.18)).Add(sd.Scale(0.25)).Add(vec3{0, 1.0, 0}), vec3{0.5, 0.34, 0.04}, sd, 0.015)
	b.Part("screen.dim", mesh.LitIndoor, vec3{}).Box(at.Sub(f.Scale(0.155)).Add(sd.Scale(0.25)).Add(vec3{0, 1.0, 0}), vec3{0.44, 0.28, 0.01}, sd, 0)
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(at.Sub(f.Scale(0.18)).Add(sd.Scale(0.25)).Add(vec3{0, 0.8, 0}), vec3{0.06, 0.1, 0.06}, sd, 0)
	for k, tok := range []string{"pipe.red", "crate.sand", "pipe.blue"} {
		b.Part(tok, mesh.LitIndoor, vec3{}).Box(at.Add(sd.Scale(-0.35)).Add(vec3{0, 0.79 + float64(k)*0.045, 0}), vec3{0.3, 0.04, 0.22}, vec3{math.Cos(a + float64(k)*0.3), 0, math.Sin(a + float64(k)*0.3)}, 0.005)
	}
	b.Part("steel", mesh.LitIndoor, vec3{}).Prism(at.Add(sd.Scale(-0.5)).Sub(f.Scale(0.15)).Add(vec3{0, 0.765, 0}), yUp, 0.02, 0.02, 0.35, 6, 0)
	b.Part("lamp.warm", mesh.LitIndoor, vec3{}).Prism(at.Add(sd.Scale(-0.5)).Sub(f.Scale(0.05)).Add(vec3{0, 1.05, 0}), vec3{0, -1, 0}, 0.12, 0.06, 0.1, 10, 0)
}

// Clutter: small things left around by the crew, laid on surfaces.

// mug puts a mug with its handle at p (on a surface).
func (b *building) mug(p vec3, tok string, a float64) {
	b.Part(tok, mesh.LitIndoor, vec3{}).Prism(p, yUp, 0.045, 0.05, 0.1, 14, 0.008)
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Prism(p.Add(vec3{0, 0.085, 0}), yUp, 0.04, 0.04, 0.016, 14, 0)
	h := vec3{math.Cos(a), 0, math.Sin(a)}
	b.Part(tok, mesh.LitIndoor, vec3{}).Box(p.Add(h.Scale(0.06)).Add(vec3{0, 0.05, 0}), vec3{0.02, 0.06, 0.012}, vec3{-h.Z, 0, h.X}, 0.004)
}

// papers lays a small fanned stack of sheets at p.
func (b *building) papers(p vec3, a float64, n int) {
	for k := range n {
		t := a + float64(k)*0.25 - 0.3
		b.Part("paper", mesh.LitIndoor, vec3{}).Box(p.Add(vec3{0, 0.003 + float64(k)*0.003, 0}).Add(vec3{math.Cos(t*3) * 0.02, 0, math.Sin(t*3) * 0.02}), vec3{0.21, 0.002, 0.29}, vec3{math.Cos(t), 0, math.Sin(t)}, 0)
	}
}

// tablet lays a data pad, its screen glowing.
func (b *building) tablet(p vec3, a float64) {
	d := vec3{math.Cos(a), 0, math.Sin(a)}
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(p.Add(vec3{0, 0.008, 0}), vec3{0.24, 0.016, 0.17}, d, 0.006)
	b.Part("screen.dim", mesh.LitIndoor, vec3{}).Box(p.Add(vec3{0, 0.017, 0}), vec3{0.21, 0.002, 0.14}, d, 0)
}

// toolbox puts a closed red toolbox with a handle at p.
func (b *building) toolbox(p vec3, a float64) {
	d := vec3{math.Cos(a), 0, math.Sin(a)}
	b.Part("pipe.red", mesh.LitIndoor, vec3{}).Box(p.Add(vec3{0, 0.1, 0}), vec3{0.5, 0.2, 0.24}, d, 0.02)
	b.Part("steel.dark", mesh.LitIndoor, vec3{}).Box(p.Add(vec3{0, 0.205, 0}), vec3{0.5, 0.012, 0.25}, d, 0.004)
	for _, s := range []float64{-0.12, 0.12} {
		b.Part("steel", mesh.LitIndoor, vec3{}).Box(p.Add(d.Scale(s)).Add(vec3{0, 0.25, 0}), vec3{0.02, 0.08, 0.02}, d, 0.004)
	}
	b.Part("steel", mesh.LitIndoor, vec3{}).Box(p.Add(vec3{0, 0.29, 0}), vec3{0.26, 0.02, 0.025}, d, 0.006)
	b.Part("steel", mesh.LitIndoor, vec3{}).Box(p.Add(d.Scale(0.2)).Add(vec3{0, 0.14, 0}), vec3{0.04, 0.04, 0.25}, d, 0.006)
}

// coil lays a coiled cable on the floor at p.
func (b *building) coil(p vec3, r float64, tok string) {
	m := b.Part(tok, mesh.LitIndoor, vec3{})
	for loop := range 3 {
		rr := r - float64(loop)*0.035
		const n = 14
		for k := range n {
			t0, t1 := float64(k)*2*math.Pi/n, float64(k+1)*2*math.Pi/n
			p0 := p.Add(vec3{math.Cos(t0) * rr, 0.018 + float64(loop)*0.02, math.Sin(t0) * rr})
			p1 := p.Add(vec3{math.Cos(t1) * rr, 0.018 + float64(loop)*0.02, math.Sin(t1) * rr})
			m.Prism(p0, p1.Sub(p0), 0.016, 0.016, p1.Sub(p0).Length()+0.01, 6, 0)
		}
	}
}

// helmet sets a spare helmet down at p, visor toward a.
func (b *building) helmet(p vec3, a float64) {
	d := vec3{math.Cos(a), 0, math.Sin(a)}
	b.Part("habitat.white", mesh.LitIndoor, vec3{}).Dome(p.Add(vec3{0, 0.06, 0}), 0.16, 0.2)
	b.Part("steel", mesh.LitIndoor, vec3{}).Prism(p, yUp, 0.165, 0.165, 0.07, 20, 0.01)
	b.Part("visor.gold", mesh.LitIndoor, vec3{}).BoxBasis(p.Add(d.Scale(0.13)).Add(vec3{0, 0.14, 0}), [3]vec3{{-d.Z, 0, d.X}, vec3{0, 1, 0}, d}, vec3{0.2, 0.12, 0.06}, 0.03)
}

// clutter dresses a room according to what it is.
func (b *building) clutter(bp *content.Blueprint) {
	at := func(v vec3) vec3 { return bp.At.Add(v) }
	switch bp.ID {
	case "core:hub":
		b.coil(at(vec3{-4.3, FloorTop, 2.6}), 0.26, "pipe.yellow")
		b.toolbox(at(vec3{2.5, FloorTop, 4.6}), 0.4)
		b.helmet(at(content.Polar(5.4, 0.8)).Add(vec3{0, 1.24, 0}), 3.9)
		b.tablet(at(vec3{0.6, 0.94, 0.6}), 0.7)
		b.mug(at(vec3{-0.75, 0.93, -0.45}), "pipe.blue", 1)
	case "core:sale":
		b.coil(at(vec3{-2.8, FloorTop, 1.6}), 0.3, "rubber")
		b.toolbox(at(vec3{0.4, FloorTop, 1.3}), 2.1)
		b.papers(at(vec3{0.6, 0, -2.7}).Add(vec3{0, 1.35, 0}), 0.4, 3)
	case "core:research":
		c := at(vec3{2.3, 0, 0})
		b.mug(c.Add(vec3{-0.4, 0.81, 0.85}), "habitat.white", 2.5)
		b.papers(c.Add(vec3{-0.35, 0.81, -0.75}), 0.2, 4)
		b.tablet(c.Add(vec3{-0.45, 0.81, 0.35}), 1.4)
		wb := at(vec3{-0.6, 0, 3.0})
		b.mug(wb.Add(vec3{0.65, 0.93, 0.1}), "barrel.ochre", 0.3)
		b.papers(wb.Add(vec3{0.2, 0.93, 0.15}), 1.2, 2)
		b.coil(at(vec3{-2.4, FloorTop, 0.4}), 0.28, "pipe.blue")
		b.helmet(at(vec3{3.0, FloorTop, -2.9}), 3.5)
	case "core:sleep":
		b.mug(at(vec3{1.4, 0.63, 2.0}), "fabric.purple", 4)
		desk := at(vec3{2.75, 0, 0.6})
		b.papers(desk.Add(vec3{0.05, 0.77, 0.3}), 2.0, 3)
		b.tablet(desk.Add(vec3{0.1, 0.77, -0.05}), 1.8)
		b.mug(desk.Add(vec3{-0.05, 0.77, -0.45}), "pipe.red", 2)
		b.helmet(at(vec3{-1.4, FloorTop, 2.9}), 5.2)
	}
}
