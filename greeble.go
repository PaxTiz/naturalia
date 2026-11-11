package main

import "math"

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
func (b *building) greebleRoom(bp *Blueprint, r room, seed int) {
	panels := r.panels()
	ladder := false
	for i, p := range panels {
		if p.y0 > 0 {
			continue // the doorway: its frame is enough
		}
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		along := vec3{-out.Z, 0, out.X}
		outer := p.mid.add(out.scale(wallThick / 2))
		inner := p.mid.sub(out.scale(wallThick/2 + 0.04)) // in front of the wainscot
		nextToDoor := false
		for _, o := range r.openings {
			if angleDist(p.angle, o) < (doorWidth/2+1.3)/r.radius {
				nextToDoor = true
			}
		}
		blocked := false
		for _, pr := range bp.Props {
			if d := bp.At.add(pr.At).sub(inner); math.Hypot(d.X, d.Z) < 1.7 {
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
		b.part("steel", litSun, vec3{}).box(outer.add(along.scale(p.width/2)).add(out.scale(0.12)).add(vec3{0, 0.14, 0}), vec3{0.22, 0.28, 0.3}, along, 0.03)
		pipeRun(b.part("pipe.yellow", litSun, vec3{}), outer.add(out.scale(0.1)).add(vec3{0, 2.02, 0}), along, p.width, 0.045)
		pipeRun(b.part("pipe.blue", litSun, vec3{}), outer.add(out.scale(0.09)).add(vec3{0, 1.88, 0}), along, p.width, 0.035)
		b.part("steel.dark", litSun, vec3{}).box(outer.add(out.scale(0.07)).add(vec3{0, 1.95, 0}), vec3{0.08, 0.3, 0.14}, along, 0.015)
		b.outsidePiece(kind, outer, out, along, p.width, seed+i)

		// Inside: conduits under the dome, then the panel's piece.
		ceil := r.wallH - 0.28
		pipeRun(b.part("steel", litIndoor, vec3{}), inner.sub(out.scale(0.1)).add(vec3{0, ceil, 0}), along, p.width, 0.05)
		pipeRun(b.part("pipe.blue", litIndoor, vec3{}), inner.sub(out.scale(0.22)).add(vec3{0, ceil + 0.1, 0}), along, p.width, 0.04)
		b.part("steel.dark", litIndoor, vec3{}).box(inner.sub(out.scale(0.14)).add(vec3{0, ceil + 0.05, 0}), vec3{0.08, 0.1, 0.32}, along, 0.015)
		if !blocked {
			b.insidePiece(kind, inner, out, along, seed+i, bp.Accent)
		}
	}

	// The dome: ribs outside, a hatch and a vent on top.
	top := r.center.add(vec3{0, r.wallH, 0})
	a, bh := r.radius+0.12, r.radius*0.55
	// A raised seam along every gore, wider steel ribs on every other one,
	// and two bands around. Each stands off the shell: sharing its surface,
	// they would fight it for the depth and flicker, and poking through it
	// they would show inside as dashes.
	for k := range domeGores {
		th := float64(k) * 2 * math.Pi / domeGores
		if k%2 == 1 {
			domeRib(b.part("steel", litSun, vec3{}), top, a, bh, th, 0.1, 0.06)
		} else {
			domeRib(b.part("habitat.white2", litSun, vec3{}), top, a, bh, th, 0.05, 0.03)
		}
	}
	for _, phi := range []float64{0.42, 0.85} {
		r, y := a*math.Cos(phi)+0.035*math.Cos(phi), (bh+0.035)*math.Sin(phi)
		ring := b.part("habitat.white2", litSun, vec3{})
		for k := range domeGores {
			t0, t1 := float64(k)*2*math.Pi/domeGores, float64(k+1)*2*math.Pi/domeGores
			p0 := top.add(vec3{math.Cos(t0) * r, y, math.Sin(t0) * r})
			p1 := top.add(vec3{math.Cos(t1) * r, y, math.Sin(t1) * r})
			along := p1.sub(p0)
			out := vec3{math.Cos((t0 + t1) / 2), 0, math.Sin((t0 + t1) / 2)}
			nrm := out.scale(math.Cos(phi) / a).add(vec3{0, math.Sin(phi) / bh, 0}).norm()
			ax := along.norm()
			ring.boxBasis(p0.lerp(p1, 0.5), [3]vec3{ax, nrm.cross(ax), nrm}, vec3{along.length() + 0.04, 0.06, 0.03}, 0.01)
		}
	}
	apex := top.add(vec3{0, bh + 0.01, 0}) // on the shell, not through it: it would show inside
	b.part("steel", litSun, vec3{}).prism(apex, yUp, 0.4, 0.62, 0.16, 16, 0.03)
	b.part("habitat.white2", litSun, vec3{}).prism(apex.add(vec3{0, 0.16, 0}), yUp, 0.5, 0.42, 0.1, 16, 0.02)
	b.part("steel.dark", litSun, vec3{}).box(apex.add(vec3{0.3, 0.3, 0}), vec3{0.3, 0.12, 0.3}, vec3{1, 0, 0}, 0.02)
}

// pipeRun lays a pipe along a wall panel, its middle at c.
func pipeRun(m *meshBuilder, c, along vec3, width, radius float64) {
	m.prism(c.sub(along.scale(width/2)), along, radius, radius, width, 8, 0)
}

// domeRib follows the outside of a dome (radius a, height bh) along one
// meridian. Its straight pieces cut under the curve between their ends, so
// they ride high enough above the shell that their underside clears it.
func domeRib(m *meshBuilder, top vec3, a, bh, th, w, d float64) {
	out := vec3{math.Cos(th), 0, math.Sin(th)}
	side := vec3{-out.Z, 0, out.X}
	const steps = 16
	lift := 0.015 + d/2 + 0.012 // gap, half the depth, the pieces' sag
	pt := func(i int) vec3 {
		phi := float64(i) / steps * 1.5
		cp, sp := math.Cos(phi), math.Sin(phi)
		n := out.scale(cp / a).add(vec3{0, sp / bh, 0}).norm()
		return top.add(out.scale(a * cp)).add(vec3{0, bh * sp, 0}).add(n.scale(lift))
	}
	for i := range steps {
		p0, p1 := pt(i), pt(i+1)
		along := p1.sub(p0)
		l := along.length()
		along = along.scale(1 / l)
		m.boxBasis(p0.lerp(p1, 0.5), [3]vec3{side, along, side.cross(along)}, vec3{w, l + 0.02, d}, 0.015)
	}
}

// porthole puts a round window on a wall face at c, looking along n.
func (b *building) porthole(c, n vec3, lit float32) {
	b.part("steel", lit, vec3{}).prism(c, n, 0.34, 0.31, 0.07, 18, 0.02)
	b.part("glass.dark", lit, vec3{}).prism(c, n, 0.25, 0.25, 0.085, 18, 0)
	for k := range 4 {
		a := float64(k)*math.Pi/2 + math.Pi/4
		t1, t2 := tangentBasis(n)
		p := c.add(t1.scale(math.Cos(a) * 0.29)).add(t2.scale(math.Sin(a) * 0.29))
		b.part("steel.dark", lit, vec3{}).prism(p, n, 0.025, 0.025, 0.09, 6, 0)
	}
}

// outsidePiece is what a wall panel carries outside.
func (b *building) outsidePiece(kind int, outer, out, along vec3, width float64, seed int) {
	at := func(x, y, o float64) vec3 { return outer.add(along.scale(x)).add(vec3{0, y, 0}).add(out.scale(o)) }
	switch kind {
	case 0: // a porthole, seen from both sides (insidePiece draws the other)
		b.porthole(at(0, 1.5, 0), out, litSun)
	case 1: // a vent with slats
		b.part("steel.dark", litSun, vec3{}).box(at(0, 0.85, 0.06), vec3{0.62, 0.48, 0.12}, along, 0.02)
		for k := range 4 {
			b.part("steel", litSun, vec3{}).box(at(0, 0.7+float64(k)*0.1, 0.12), vec3{0.54, 0.035, 0.06}, along, 0.008)
		}
	case 2: // a junction box with its conduit, and a wall lamp
		b.part("habitat.white2", litSun, vec3{}).box(at(-0.2, 1.15, 0.09), vec3{0.42, 0.56, 0.18}, along, 0.025)
		b.part("hazard", litSun, vec3{}).box(at(-0.2, 1.4, 0.1), vec3{0.43, 0.07, 0.19}, along, 0.01)
		b.part("steel", litSun, vec3{}).prism(at(-0.2, 1.43, 0.09), yUp, 0.025, 0.025, 0.5, 6, 0)
		b.part("led.green", litSun, vec3{}).box(at(-0.08, 1.0, 0.185), vec3{0.04, 0.04, 0.01}, along, 0)
		b.part("steel.dark", litSun, vec3{}).box(at(0.35, 2.35, 0.12), vec3{0.26, 0.1, 0.24}, along, 0.02)
		b.part("lamp.warm", litSun, vec3{}).box(at(0.35, 2.295, 0.13), vec3{0.2, 0.015, 0.17}, along, 0)
	case 3: // a pipe going down into the ground, with a valve
		x := width * 0.28
		b.part("pipe.yellow", litSun, vec3{}).prism(at(x, 0, 0.1), yUp, 0.045, 0.045, 2.02, 8, 0)
		b.part("steel", litSun, vec3{}).box(at(x, 2.02, 0.1), vec3{0.12, 0.12, 0.12}, along, 0.02)
		b.part("steel", litSun, vec3{}).prism(at(x, 0, 0.1), yUp, 0.08, 0.08, 0.12, 8, 0.02)
		b.part("pipe.red", litSun, vec3{}).prism(at(x, 1.05, 0.15), out, 0.13, 0.13, 0.03, 10, 0)
		b.part("steel.dark", litSun, vec3{}).prism(at(x, 1.05, 0.1), out, 0.03, 0.03, 0.08, 6, 0)
	case 4: // a ladder up to the dome
		for _, s := range []float64{-0.22, 0.22} {
			b.part("steel", litSun, vec3{}).box(at(s, 1.3, 0.16), vec3{0.05, 2.6, 0.05}, along, 0.01)
			b.part("steel.dark", litSun, vec3{}).box(at(s, 2.3, 0.08), vec3{0.06, 0.06, 0.16}, along, 0.01)
		}
		for k := range 8 {
			b.part("steel", litSun, vec3{}).box(at(0, 0.3+float64(k)*0.3, 0.16), vec3{0.44, 0.035, 0.035}, along, 0.008)
		}
	}
	_ = seed
}

// insidePiece is what a wall panel carries inside the room.
func (b *building) insidePiece(kind int, inner, out, along vec3, seed int, accent string) {
	in := out.scale(-1)
	at := func(x, y, o float64) vec3 { return inner.add(along.scale(x)).add(vec3{0, y, 0}).add(in.scale(o)) }
	switch kind {
	case 0: // the porthole's inner side
		b.porthole(inner.add(out.scale(0.04)).add(vec3{0, 1.5, 0}), in, litIndoor)
	case 1: // a locker, with its doors, handles, vents and a status light
		tok := "locker"
		if seed%2 == 0 {
			tok = "locker.ochre"
		}
		b.part(tok, litIndoor, vec3{}).box(at(0, 1.0, 0.2), vec3{0.8, 2.0, 0.4}, along, 0.03)
		b.part("steel.dark", litIndoor, vec3{}).box(at(0, 1.0, 0.401), vec3{0.015, 1.86, 0.01}, along, 0)
		for _, s := range []float64{-0.06, 0.06} {
			b.part("steel", litIndoor, vec3{}).box(at(s, 1.05, 0.41), vec3{0.025, 0.18, 0.03}, along, 0.005)
		}
		for k := range 3 {
			for _, s := range []float64{-0.2, 0.2} {
				b.part("steel.dark", litIndoor, vec3{}).box(at(s, 1.75+float64(k)*0.06, 0.401), vec3{0.22, 0.02, 0.01}, along, 0)
			}
		}
		b.part(accent, litIndoor, vec3{}).box(at(0.3, 1.9, 0.402), vec3{0.08, 0.03, 0.01}, along, 0)
	case 2: // a shelf of boxes and canisters
		for _, s := range []float64{-0.38, 0.38} {
			b.part("steel", litIndoor, vec3{}).box(at(s, 1.65, 0.15), vec3{0.04, 0.55, 0.3}, along, 0.008)
		}
		for j, y := range []float64{1.4, 1.85} {
			b.part("steel", litIndoor, vec3{}).box(at(0, y, 0.17), vec3{0.9, 0.035, 0.34}, along, 0.01)
			x := -0.36
			for k := 0; x < 0.3; k++ {
				r := grnd(seed, j*10+k)
				if r < 0.3 { // a canister
					tok := []string{"pipe.red", "pipe.blue", "pipe.yellow"}[k%3]
					b.part(tok, litIndoor, vec3{}).prism(at(x+0.07, y+0.0175, 0.17), yUp, 0.065, 0.065, 0.18+0.1*r, 10, 0.015)
					x += 0.17
				} else {
					s := 0.14 + 0.12*r
					tok := []string{"crate.ochre", "crate.slate", "crate.sand"}[(seed+k)%3]
					b.part(tok, litIndoor, vec3{}).box(at(x+s/2, y+0.0175+s*0.4, 0.17), vec3{s, s * 0.8, 0.26}, along, 0.015)
					x += s + 0.04
				}
			}
		}
	case 3: // a wall screen with a keypad
		b.part("steel.dark", litIndoor, vec3{}).box(at(0, 1.6, 0.04), vec3{0.74, 0.48, 0.07}, along, 0.02)
		b.part("screen.dim", litIndoor, vec3{}).box(at(0, 1.62, 0.076), vec3{0.62, 0.36, 0.01}, along, 0)
		b.part(accent, litIndoor, vec3{}).box(at(-0.22, 1.73, 0.083), vec3{0.12, 0.03, 0.005}, along, 0)
		b.part("steel.dark", litIndoor, vec3{}).box(at(0.25, 1.15, 0.035), vec3{0.18, 0.24, 0.06}, along, 0.015)
		b.part("led.green", litIndoor, vec3{}).box(at(0.25, 1.22, 0.066), vec3{0.04, 0.03, 0.005}, along, 0)
	case 4, 5: // a vent high on the wall and a pair of pipes, or an extinguisher
		if kind == 5 && seed%3 == 0 {
			b.part("steel.dark", litIndoor, vec3{}).box(at(0, 1.35, 0.03), vec3{0.12, 0.5, 0.05}, along, 0.01)
			b.part("pipe.red", litIndoor, vec3{}).prism(at(0, 1.08, 0.12), yUp, 0.08, 0.08, 0.48, 10, 0.03)
			b.part("steel.dark", litIndoor, vec3{}).prism(at(0, 1.56, 0.12), yUp, 0.03, 0.03, 0.07, 6, 0)
			b.part("hazard", litIndoor, vec3{}).box(at(0, 1.85, 0.02), vec3{0.3, 0.12, 0.02}, along, 0.005)
			return
		}
		b.part("steel.dark", litIndoor, vec3{}).box(at(0, 2.05, 0.04), vec3{0.6, 0.34, 0.07}, along, 0.02)
		for k := range 3 {
			b.part("steel", litIndoor, vec3{}).box(at(0, 1.95+float64(k)*0.09, 0.08), vec3{0.52, 0.03, 0.04}, along, 0.006)
		}
		for _, s := range []float64{0.48, 0.6} {
			b.part("pipe.blue", litIndoor, vec3{}).prism(at(s, 0, 0.08), yUp, 0.035, 0.035, 2.4, 8, 0)
		}
	}
}

// greebleCorridor: conduits outside, pipes and vents along the ceiling.
func (b *building) greebleCorridor(c corridor, seed int) {
	mid := c.from.lerp(c.to, 0.5)
	dir := c.to.sub(c.from)
	length := dir.length()
	dir = dir.scale(1 / length)
	side := c.side()
	for _, s := range []float64{-1, 1} {
		wall := mid.add(side.scale(s * (c.width/2 + wallThick)))
		pipeRun(b.part("pipe.yellow", litSun, vec3{}), wall.add(side.scale(s*0.1)).add(vec3{0, 1.95, 0}), dir, length, 0.045)
		pipeRun(b.part("pipe.blue", litSun, vec3{}), wall.add(side.scale(s*0.09)).add(vec3{0, 1.8, 0}), dir, length, 0.035)
		in := mid.add(side.scale(s * (c.width/2 - 0.12)))
		pipeRun(b.part("steel", litIndoor, vec3{}), in.add(vec3{0, c.height - 0.18, 0}), dir, length*0.98, 0.05)
		pipeRun(b.part("pipe.blue", litIndoor, vec3{}), in.add(side.scale(-s*0.13)).add(vec3{0, c.height - 0.12, 0}), dir, length*0.98, 0.04)
	}
	n := int(length / 1.6)
	for i := range n {
		at := c.from.add(dir.scale((float64(i) + 0.5) * length / float64(n)))
		s := 1.0
		if grnd(seed, i) < 0.5 {
			s = -1
		}
		face := at.add(side.scale(s * (c.width/2 - 0.04)))
		switch int(grnd(seed, i+50) * 3) {
		case 0:
			b.part("steel.dark", litIndoor, vec3{}).box(face.add(vec3{0, 1.6, 0}), vec3{0.5, 0.32, 0.06}, dir, 0.015)
			b.part("screen.dim", litIndoor, vec3{}).box(face.add(vec3{0, 1.6, 0}).sub(side.scale(s*0.032)), vec3{0.42, 0.24, 0.01}, dir, 0)
		case 1:
			b.part("steel.dark", litIndoor, vec3{}).box(face.add(vec3{0, 0.45, 0}), vec3{0.5, 0.3, 0.06}, dir, 0.015)
			for k := range 3 {
				b.part("steel", litIndoor, vec3{}).box(face.add(vec3{0, 0.37 + float64(k)*0.08, 0}).sub(side.scale(s*0.035)), vec3{0.44, 0.025, 0.03}, dir, 0.005)
			}
		}
	}
}

// yard furnishes the ground around the base. yardObstacles below must
// match it.
func (b *building) yard() {
	sun := func(tok string) *meshBuilder { return b.part(tok, litSun, vec3{}) }

	// A landing pad: concrete plates, a hazard rim, a white ring and lights.
	pad := vec3{11, 0, -12.5}
	sun("hazard").prism(pad, yUp, 3.6, 3.5, 0.06, 6, 0.02)
	sun("pad.concrete").prism(pad, yUp, 3.3, 3.3, 0.09, 6, 0.01)
	sun("habitat.white").prism(pad, yUp, 1.3, 1.3, 0.095, 24, 0)
	sun("pad.concrete").prism(pad, yUp, 1.05, 1.05, 0.1, 24, 0)
	for k := range 6 {
		a := float64(k) * math.Pi / 3
		p := pad.add(vec3{math.Cos(a) * 3.45, 0, math.Sin(a) * 3.45})
		sun("steel.dark").prism(p, yUp, 0.12, 0.1, 0.12, 8, 0.02)
		sun("lamp.warm").prism(p.add(vec3{0, 0.12, 0}), yUp, 0.07, 0.07, 0.03, 8, 0)
	}

	// Water tanks on cradles, banded, with a pipe to the airlock.
	for j, z := range []float64{-14.2, -12.4} {
		start := vec3{-6.8, 0.95, z}
		x := vec3{1, 0, 0}
		sun("habitat.white").prism(start, x, 0.78, 0.78, 3.6, 18, 0.06)
		for _, t := range []float64{0.3, 1.8, 3.3} {
			sun("steel").prism(start.add(x.scale(t-0.06)), x, 0.81, 0.81, 0.12, 18, 0)
			if t != 1.8 {
				sun("steel.dark").box(start.add(x.scale(t)).add(vec3{0, -0.6, 0}), vec3{0.2, 0.7, 1.3}, x, 0.03)
			}
		}
		sun("hazard").prism(start.add(x.scale(3.6)), x, 0.5, 0.45, 0.04, 18, 0)
		sun("pipe.blue").prism(vec3{-3.2, 0.5, z}, x, 0.06, 0.06, 1.6-float64(j)*0.2, 8, 0)
		sun("pipe.blue").prism(vec3{-1.6 - float64(j)*0.2, 0, z}, yUp, 0.06, 0.06, 0.56, 8, 0)
	}
	sun("rubber").box(vec3{-1.7, 0.03, -11.5}, vec3{0.08, 0.06, 4}, vec3{1, 0, 0}, 0.02)

	// Barrels, standing and one fallen.
	for c, cl := range [][2]float64{{5.4, -9.4}, {-9.4, 8.6}} {
		for k := range 5 {
			a := float64(k)*1.3 + float64(c)
			r := 0.35 + 0.35*grnd(c, k)
			p := vec3{cl[0] + math.Cos(a)*r, 0, cl[1] + math.Sin(a)*r}
			tok := []string{"barrel.blue", "barrel.ochre", "pipe.red"}[(k+c)%3]
			if k == 4 {
				axis := vec3{math.Cos(a + 1), 0, math.Sin(a + 1)}
				p = p.add(vec3{math.Cos(a), 0, math.Sin(a)}.scale(0.5)).add(vec3{0, 0.29, 0})
				sun(tok).prism(p.sub(axis.scale(0.45)), axis, 0.29, 0.29, 0.9, 14, 0.03)
				continue
			}
			sun(tok).prism(p, yUp, 0.29, 0.29, 0.9, 14, 0.03)
			for _, y := range []float64{0.22, 0.66} {
				sun("steel").prism(p.add(vec3{0, y, 0}), yUp, 0.305, 0.305, 0.04, 14, 0)
			}
		}
	}

	// A cargo container: corrugated sides, corner posts, hazard doors.
	{
		c := vec3{9.6, 0, 9.4}
		ang := 0.55
		ax := vec3{math.Cos(ang), 0, math.Sin(ang)}
		sd := vec3{-ax.Z, 0, ax.X}
		sun("crate.slate").box(c.add(vec3{0, 1.05, 0}), vec3{4.4, 2.1, 2.1}, ax, 0.05)
		for k := -10; k <= 10; k++ {
			for _, s := range []float64{-1, 1} {
				sun("crate.slate").box(c.add(ax.scale(float64(k)*0.2)).add(sd.scale(s*1.06)).add(vec3{0, 1.05, 0}), vec3{0.08, 1.9, 0.04}, ax, 0.01)
			}
		}
		for _, x := range []float64{-1, 1} {
			for _, s := range []float64{-1, 1} {
				sun("steel").box(c.add(ax.scale(x*2.17)).add(sd.scale(s*1.02)).add(vec3{0, 1.05, 0}), vec3{0.14, 2.14, 0.14}, ax, 0.02)
			}
		}
		end := c.add(ax.scale(2.21))
		sun("hazard").box(end.add(vec3{0, 1.9, 0}), vec3{0.04, 0.16, 1.9}, ax, 0)
		for _, s := range []float64{-0.45, 0.45} {
			sun("steel").box(end.add(sd.scale(s)).add(vec3{0, 1.0, 0}), vec3{0.05, 1.7, 0.05}, ax, 0.01)
		}
	}

	// Floodlight masts.
	for _, m := range yardMasts {
		sun("steel.dark").box(m.add(vec3{0, 0.2, 0}), vec3{0.5, 0.4, 0.5}, vec3{1, 0, 0}, 0.04)
		sun("steel").prism(m, yUp, 0.08, 0.06, 4.6, 8, 0)
		in := vec3{-m.X, 0, -m.Z}.norm()
		head := m.add(vec3{0, 4.6, 0}).add(in.scale(0.2))
		sun("steel.dark").box(head, vec3{0.7, 0.28, 0.3}, vec3{-in.Z, 0, in.X}, 0.03)
		sun("lamp.warm").box(head.add(vec3{0, -0.15, 0}).add(in.scale(0.03)), vec3{0.6, 0.02, 0.22}, vec3{-in.Z, 0, in.X}, 0)
	}

	// Cables on the ground, from the solar panels to the sale room.
	for _, seg := range [][2]vec3{{{-12, 0.03, -9}, {-12, 0.03, -4.6}}, {{-12, 0.03, -4.6}, {-13, 0.03, -4.0}}} {
		d := seg[1].sub(seg[0])
		sun("rubber").box(seg[0].lerp(seg[1], 0.5), vec3{0.07, 0.06, d.length()}, vec3{d.Z, 0, -d.X}.norm(), 0.02)
	}
}

var yardMasts = []vec3{{-17.5, 0, 6}, {17.5, 0, 6}, {15.5, 0, -15}, {-8, 0, 17.5}}

// yardObstacles: what the player bumps into in the yard.
func yardObstacles() []obstacle {
	out := []obstacle{
		{vec3{5.4, 0, -9.4}, 0.95}, {vec3{-9.4, 0, 8.6}, 0.95},
	}
	for _, z := range []float64{-14.2, -12.4} {
		for _, x := range []float64{-6.3, -5.0, -3.7} {
			out = append(out, obstacle{vec3{x, 0, z}, 0.8})
		}
	}
	c, ax := vec3{9.6, 0, 9.4}, vec3{math.Cos(0.55), 0, math.Sin(0.55)}
	for _, t := range []float64{-1.4, 0, 1.4} {
		out = append(out, obstacle{c.add(ax.scale(t)), 1.15})
	}
	for _, m := range yardMasts {
		out = append(out, obstacle{m, 0.35})
	}
	return out
}

// More furniture, as kit pieces (kitProps): a chair, a workbench, a
// hydroponic planter and a desk.

// buildChair: a swivel chair facing toward -X (rotate with the seed: the
// angle is seed × 45°).
func buildChair(b *building, at vec3, seed int) {
	a := float64(seed) * math.Pi / 4
	f := vec3{math.Cos(a), 0, math.Sin(a)}
	sd := vec3{-f.Z, 0, f.X}
	dark, seat := b.part("steel.dark", litIndoor, vec3{}), b.part("fabric.seat", litIndoor, vec3{})
	for k := range 5 {
		t := float64(k) * 2 * math.Pi / 5
		leg := vec3{math.Cos(t), 0, math.Sin(t)}
		dark.box(at.add(leg.scale(0.18)).add(vec3{0, 0.05, 0}), vec3{0.36, 0.04, 0.05}, leg, 0.01)
	}
	dark.prism(at, yUp, 0.03, 0.03, 0.45, 8, 0)
	seat.box(at.add(vec3{0, 0.5, 0}), vec3{0.5, 0.09, 0.5}, f, 0.04)
	seat.boxBasis(at.sub(f.scale(0.25)).add(vec3{0, 0.85, 0}), [3]vec3{sd, vec3{0, 0.99, 0.1}.norm(), f}, vec3{0.46, 0.55, 0.08}, 0.04)
	dark.box(at.sub(f.scale(0.22)).add(vec3{0, 0.6, 0}), vec3{0.06, 0.2, 0.04}, sd, 0.01)
}

// buildWorkbench: a steel bench with a pegboard, tools and a specimen jar.
func buildWorkbench(b *building, at vec3, seed int) {
	a := float64(seed) * math.Pi / 4
	f := vec3{math.Cos(a), 0, math.Sin(a)} // toward the user
	sd := vec3{-f.Z, 0, f.X}
	steel, dark := b.part("steel", litIndoor, vec3{}), b.part("steel.dark", litIndoor, vec3{})
	b.part("wood.brown", litIndoor, vec3{}).box(at.add(vec3{0, 0.9, 0}), vec3{1.8, 0.06, 0.75}, sd, 0.015)
	for _, x := range []float64{-0.82, 0.82} {
		for _, z := range []float64{-0.3, 0.3} {
			dark.box(at.add(sd.scale(x)).add(f.scale(z)).add(vec3{0, 0.44, 0}), vec3{0.06, 0.88, 0.06}, sd, 0.01)
		}
	}
	steel.box(at.add(vec3{0, 0.25, 0}), vec3{1.7, 0.03, 0.65}, sd, 0.01)
	b.part("crate.slate", litIndoor, vec3{}).box(at.add(sd.scale(-0.45)).add(vec3{0, 0.42, 0}), vec3{0.6, 0.3, 0.45}, sd, 0.02)
	board := at.sub(f.scale(0.38)).add(vec3{0, 1.45, 0})
	b.part("wall.lower", litIndoor, vec3{}).box(board, vec3{1.7, 0.9, 0.04}, sd, 0.015)
	for k := range 5 {
		x := -0.6 + float64(k)*0.3
		steel.box(board.add(sd.scale(x)).add(f.scale(0.04)).add(vec3{0, 0.1 * float64(k%2), 0}), vec3{0.04, 0.32, 0.03}, sd, 0.005)
	}
	b.part("pipe.red", litIndoor, vec3{}).box(at.add(sd.scale(0.5)).add(vec3{0, 0.99, 0}), vec3{0.42, 0.12, 0.22}, sd, 0.02)
	b.part("glass.dark", litIndoor, vec3{}).prism(at.add(sd.scale(-0.1)).add(vec3{0, 0.93, 0}), yUp, 0.1, 0.1, 0.24, 12, 0.02)
	b.part("planter.green", litIndoor, vec3{}).prism(at.add(sd.scale(-0.1)).add(vec3{0, 0.95, 0}), yUp, 0.07, 0.05, 0.1, 6, 0)
	steel.prism(at.add(sd.scale(-0.1)).add(vec3{0, 1.17, 0}), yUp, 0.11, 0.11, 0.03, 12, 0.01)
}

// buildPlanter: a hydroponic trough on legs with rows of faceted plants
// and a grow lamp above.
func buildPlanter(b *building, at vec3, seed int) {
	a := float64(seed%8) * math.Pi / 4
	f := vec3{math.Cos(a), 0, math.Sin(a)}
	sd := vec3{-f.Z, 0, f.X}
	steel, dark := b.part("steel", litIndoor, vec3{}), b.part("steel.dark", litIndoor, vec3{})
	b.part("habitat.white2", litIndoor, vec3{}).box(at.add(vec3{0, 0.7, 0}), vec3{1.7, 0.32, 0.6}, sd, 0.04)
	b.part("soil", litIndoor, vec3{}).box(at.add(vec3{0, 0.865, 0}), vec3{1.6, 0.02, 0.5}, sd, 0)
	for _, x := range []float64{-0.75, 0.75} {
		dark.box(at.add(sd.scale(x)).add(vec3{0, 0.27, 0}), vec3{0.06, 0.54, 0.5}, sd, 0.01)
		steel.box(at.add(sd.scale(x)).add(vec3{0, 1.35, 0}), vec3{0.04, 0.95, 0.04}, sd, 0.005)
	}
	b.part("pipe.blue", litIndoor, vec3{}).prism(at.add(sd.scale(-0.85)).add(f.scale(-0.2)).add(vec3{0, 0.75, 0}), sd, 0.03, 0.03, 1.7, 8, 0)
	lamp := at.add(vec3{0, 1.82, 0})
	dark.box(lamp, vec3{1.6, 0.06, 0.32}, sd, 0.015)
	b.part("lamp.grow", litIndoor, vec3{}).box(lamp.add(vec3{0, -0.035, 0}), vec3{1.5, 0.01, 0.24}, sd, 0)
	for k := range 6 {
		for j, z := range []float64{-0.12, 0.12} {
			r := grnd(seed, k*2+j)
			p := at.add(sd.scale(-0.65 + float64(k)*0.26)).add(f.scale(z)).add(vec3{0, 0.87, 0})
			h := 0.18 + 0.22*r
			tok := "planter.green"
			if (k+j)%3 == 0 {
				tok = "planter.lime"
			}
			// A tuft of leaves arching out from the stem.
			leaf := b.part(tok, litIndoor, vec3{})
			leaf.prism(p, yUp, 0.025, 0.015, h*0.6, 8, 0)
			for l := range 6 {
				t := float64(l)*2.4 + r*6
				out := vec3{math.Cos(t), 0, math.Sin(t)}
				lift := 0.5 + 0.4*grnd(seed, k*20+j*7+l)
				dirl := out.add(vec3{0, lift * 1.6, 0}).norm()
				length := h * (0.55 + 0.35*grnd(seed, k*30+l))
				c := p.add(vec3{0, h * (0.15 + 0.07*float64(l)), 0}).add(dirl.scale(length / 2))
				across := vec3{-out.Z, 0, out.X}
				leaf.boxBasis(c, [3]vec3{across, dirl, across.cross(dirl)}, vec3{0.06 + 0.03*lift, length, 0.008}, 0.004)
			}
		}
	}
}

// buildDesk: a small desk with a lamp, a terminal and a stack of books.
func buildDesk(b *building, at vec3, seed int) {
	a := float64(seed) * math.Pi / 4
	f := vec3{math.Cos(a), 0, math.Sin(a)}
	sd := vec3{-f.Z, 0, f.X}
	b.part("wood.brown", litIndoor, vec3{}).box(at.add(vec3{0, 0.74, 0}), vec3{1.3, 0.05, 0.6}, sd, 0.015)
	for _, x := range []float64{-0.6, 0.6} {
		b.part("steel.dark", litIndoor, vec3{}).box(at.add(sd.scale(x)).add(vec3{0, 0.36, 0}), vec3{0.05, 0.72, 0.55}, sd, 0.01)
	}
	b.part("steel.dark", litIndoor, vec3{}).box(at.sub(f.scale(0.18)).add(sd.scale(0.25)).add(vec3{0, 1.0, 0}), vec3{0.5, 0.34, 0.04}, sd, 0.015)
	b.part("screen.dim", litIndoor, vec3{}).box(at.sub(f.scale(0.155)).add(sd.scale(0.25)).add(vec3{0, 1.0, 0}), vec3{0.44, 0.28, 0.01}, sd, 0)
	b.part("steel.dark", litIndoor, vec3{}).box(at.sub(f.scale(0.18)).add(sd.scale(0.25)).add(vec3{0, 0.8, 0}), vec3{0.06, 0.1, 0.06}, sd, 0)
	for k, tok := range []string{"pipe.red", "crate.sand", "pipe.blue"} {
		b.part(tok, litIndoor, vec3{}).box(at.add(sd.scale(-0.35)).add(vec3{0, 0.79 + float64(k)*0.045, 0}), vec3{0.3, 0.04, 0.22}, vec3{math.Cos(a + float64(k)*0.3), 0, math.Sin(a + float64(k)*0.3)}, 0.005)
	}
	b.part("steel", litIndoor, vec3{}).prism(at.add(sd.scale(-0.5)).sub(f.scale(0.15)).add(vec3{0, 0.765, 0}), yUp, 0.02, 0.02, 0.35, 6, 0)
	b.part("lamp.warm", litIndoor, vec3{}).prism(at.add(sd.scale(-0.5)).sub(f.scale(0.05)).add(vec3{0, 1.05, 0}), vec3{0, -1, 0}, 0.12, 0.06, 0.1, 10, 0)
}

// Clutter: small things left around by the crew, laid on surfaces.

// mug puts a mug with its handle at p (on a surface).
func (b *building) mug(p vec3, tok string, a float64) {
	b.part(tok, litIndoor, vec3{}).prism(p, yUp, 0.045, 0.05, 0.1, 14, 0.008)
	b.part("steel.dark", litIndoor, vec3{}).prism(p.add(vec3{0, 0.085, 0}), yUp, 0.04, 0.04, 0.016, 14, 0)
	h := vec3{math.Cos(a), 0, math.Sin(a)}
	b.part(tok, litIndoor, vec3{}).box(p.add(h.scale(0.06)).add(vec3{0, 0.05, 0}), vec3{0.02, 0.06, 0.012}, vec3{-h.Z, 0, h.X}, 0.004)
}

// papers lays a small fanned stack of sheets at p.
func (b *building) papers(p vec3, a float64, n int) {
	for k := range n {
		t := a + float64(k)*0.25 - 0.3
		b.part("paper", litIndoor, vec3{}).box(p.add(vec3{0, 0.003 + float64(k)*0.003, 0}).add(vec3{math.Cos(t*3) * 0.02, 0, math.Sin(t*3) * 0.02}), vec3{0.21, 0.002, 0.29}, vec3{math.Cos(t), 0, math.Sin(t)}, 0)
	}
}

// tablet lays a data pad, its screen glowing.
func (b *building) tablet(p vec3, a float64) {
	d := vec3{math.Cos(a), 0, math.Sin(a)}
	b.part("steel.dark", litIndoor, vec3{}).box(p.add(vec3{0, 0.008, 0}), vec3{0.24, 0.016, 0.17}, d, 0.006)
	b.part("screen.dim", litIndoor, vec3{}).box(p.add(vec3{0, 0.017, 0}), vec3{0.21, 0.002, 0.14}, d, 0)
}

// toolbox puts a closed red toolbox with a handle at p.
func (b *building) toolbox(p vec3, a float64) {
	d := vec3{math.Cos(a), 0, math.Sin(a)}
	b.part("pipe.red", litIndoor, vec3{}).box(p.add(vec3{0, 0.1, 0}), vec3{0.5, 0.2, 0.24}, d, 0.02)
	b.part("steel.dark", litIndoor, vec3{}).box(p.add(vec3{0, 0.205, 0}), vec3{0.5, 0.012, 0.25}, d, 0.004)
	for _, s := range []float64{-0.12, 0.12} {
		b.part("steel", litIndoor, vec3{}).box(p.add(d.scale(s)).add(vec3{0, 0.25, 0}), vec3{0.02, 0.08, 0.02}, d, 0.004)
	}
	b.part("steel", litIndoor, vec3{}).box(p.add(vec3{0, 0.29, 0}), vec3{0.26, 0.02, 0.025}, d, 0.006)
	b.part("steel", litIndoor, vec3{}).box(p.add(d.scale(0.2)).add(vec3{0, 0.14, 0}), vec3{0.04, 0.04, 0.25}, d, 0.006)
}

// coil lays a coiled cable on the floor at p.
func (b *building) coil(p vec3, r float64, tok string) {
	m := b.part(tok, litIndoor, vec3{})
	for loop := range 3 {
		rr := r - float64(loop)*0.035
		const n = 14
		for k := range n {
			t0, t1 := float64(k)*2*math.Pi/n, float64(k+1)*2*math.Pi/n
			p0 := p.add(vec3{math.Cos(t0) * rr, 0.018 + float64(loop)*0.02, math.Sin(t0) * rr})
			p1 := p.add(vec3{math.Cos(t1) * rr, 0.018 + float64(loop)*0.02, math.Sin(t1) * rr})
			m.prism(p0, p1.sub(p0), 0.016, 0.016, p1.sub(p0).length()+0.01, 6, 0)
		}
	}
}

// helmet sets a spare helmet down at p, visor toward a.
func (b *building) helmet(p vec3, a float64) {
	d := vec3{math.Cos(a), 0, math.Sin(a)}
	b.part("habitat.white", litIndoor, vec3{}).dome(p.add(vec3{0, 0.06, 0}), 0.16, 0.2)
	b.part("steel", litIndoor, vec3{}).prism(p, yUp, 0.165, 0.165, 0.07, 20, 0.01)
	b.part("visor.gold", litIndoor, vec3{}).boxBasis(p.add(d.scale(0.13)).add(vec3{0, 0.14, 0}), [3]vec3{{-d.Z, 0, d.X}, vec3{0, 1, 0}, d}, vec3{0.2, 0.12, 0.06}, 0.03)
}

// clutter dresses a room according to what it is.
func (b *building) clutter(bp *Blueprint) {
	at := func(v vec3) vec3 { return bp.At.add(v) }
	switch bp.ID {
	case "core:hub":
		b.coil(at(vec3{-4.3, floorTop, 2.6}), 0.26, "pipe.yellow")
		b.toolbox(at(vec3{2.5, floorTop, 4.6}), 0.4)
		b.helmet(at(polar(5.4, 0.8)).add(vec3{0, 1.24, 0}), 3.9)
		b.tablet(at(vec3{0.6, 0.94, 0.6}), 0.7)
		b.mug(at(vec3{-0.75, 0.93, -0.45}), "pipe.blue", 1)
	case "core:sale":
		b.coil(at(vec3{-2.8, floorTop, 1.6}), 0.3, "rubber")
		b.toolbox(at(vec3{0.4, floorTop, 1.3}), 2.1)
		b.papers(at(vec3{0.6, 0, -2.7}).add(vec3{0, 1.35, 0}), 0.4, 3)
	case "core:research":
		c := at(vec3{2.3, 0, 0})
		b.mug(c.add(vec3{-0.4, 0.81, 0.85}), "habitat.white", 2.5)
		b.papers(c.add(vec3{-0.35, 0.81, -0.75}), 0.2, 4)
		b.tablet(c.add(vec3{-0.45, 0.81, 0.35}), 1.4)
		wb := at(vec3{-0.6, 0, 3.0})
		b.mug(wb.add(vec3{0.65, 0.93, 0.1}), "barrel.ochre", 0.3)
		b.papers(wb.add(vec3{0.2, 0.93, 0.15}), 1.2, 2)
		b.coil(at(vec3{-2.4, floorTop, 0.4}), 0.28, "pipe.blue")
		b.helmet(at(vec3{3.0, floorTop, -2.9}), 3.5)
	case "core:sleep":
		b.mug(at(vec3{1.4, 0.63, 2.0}), "fabric.purple", 4)
		desk := at(vec3{2.75, 0, 0.6})
		b.papers(desk.add(vec3{0.05, 0.77, 0.3}), 2.0, 3)
		b.tablet(desk.add(vec3{0.1, 0.77, -0.05}), 1.8)
		b.mug(desk.add(vec3{-0.05, 0.77, -0.45}), "pipe.red", 2)
		b.helmet(at(vec3{-1.4, floorTop, 2.9}), 5.2)
	}
}
