package base

import (
	"math"
	"naturalia/internal/mesh"
)

// Berms: regolith banked against a hall's walls (content.Blueprint.Berm),
// and along the sides of the corridors that lead to it, half burying them.
// A bank leans on a wall: a flat top along it, a slope down to its toe,
// its ends sloping too where they stand free (hipped).
//
//	   wall │▔▔▔╲  top, then the slope
//	        │    ╲
//	        │     ╲ toe
//	────────┴──────▔▔▔▔ ground

const (
	bermTop   = 0.8 // the width of a hall's bank on top
	bermSlope = 1.2 // how far a bank runs out for each meter it rises
)

// bank is a bank of regolith against a wall from a to b (local, at the
// foot of its outer face), out the way it runs, h how high it stands and
// top how wide it is up there. A hipped end slopes down along the wall;
// the other stops square, against something.
type bank struct {
	a, b, out  vec3
	h, top     float64
	hipA, hipB bool
}

// run is how far a bank reaches out from its wall.
func (k bank) run() float64 { return k.top + k.h*bermSlope }

// bermRun is how far the banks of a hall reach out from its walls, 0 if
// it has none.
func (r Room) bermRun() float64 {
	if r.Hall == nil || r.plan == nil || r.plan.Berm <= 0 {
		return 0
	}
	return bermTop + r.plan.Berm*bermSlope
}

// banks are the banks against hall r's walls, in its own frame: along
// each wall but the big door's, between its doorways. At the corners they
// reach past the wall's end, so the banks of two walls meet.
func (r Room) banks() []bank {
	h := r.plan.Berm
	if r.Hall == nil || h <= 0 {
		return nil
	}
	var out []bank
	walls := r.straightWalls()
	if r.bigDoor() {
		walls = walls[:3]
	}
	for _, w := range walls {
		along := w[1].Sub(w[0]).Norm()
		o := r.wallOut(w[0].Lerp(w[1], 0.5))
		face := o.Scale(hallWall / 2)
		for _, s := range r.spans(w[0], w[1]) {
			a, b := s[0].Add(face), s[1].Add(face)
			// The wall's own ends are corners: the bank goes round them.
			k := bank{a: a, b: b, out: o, h: h, top: bermTop}
			if s[0].Sub(w[0]).Length() < 0.01 {
				k.a, k.hipA = a.Sub(along.Scale(hallWall/2+k.run())), true
			}
			if s[1].Sub(w[1]).Length() < 0.01 {
				k.b, k.hipB = b.Add(along.Scale(hallWall/2+k.run())), true
			}
			out = append(out, k)
		}
	}
	return out
}

// corridorBanks are the banks along both sides of a corridor to a hall
// with a berm: lower, stopping square against the hall's own banks and
// sloping down at the other end.
func (c Corridor) banks() []bank {
	h, at := 0.0, -1
	for i, id := range c.Rooms {
		if r := Ents.Building.Get(id); id != 0 && r != nil && r.plan != nil && r.plan.Berm > 0 {
			h, at = math.Min(r.plan.Berm, c.Height*0.65), i
		}
	}
	if at < 0 {
		return nil
	}
	side := c.side()
	var out []bank
	for _, s := range []float64{-1, 1} {
		o := side.Scale(s)
		face := o.Scale(c.Width/2 + WallThick)
		out = append(out, bank{a: c.From.Add(face), b: c.To.Add(face), out: o, h: h, top: 0.3, hipA: at != 0, hipB: at != 1})
	}
	return out
}

// allBanks are the banks of the whole base, in its frame.
func allBanks() []bank {
	var out []bank
	for _, r := range Buildings() {
		for _, k := range r.banks() {
			k.a, k.b, k.out = r.Turn(k.a), r.Turn(k.b), r.TurnDir(k.out)
			out = append(out, k)
		}
	}
	for _, c := range Corridors() {
		out = append(out, c.banks()...)
	}
	return out
}

// bankWalls stop the player and the vehicles halfway up the banks.
func bankWalls() []Segment {
	var out []Segment
	for _, k := range allBanks() {
		mid := k.out.Scale(k.top + k.h*bermSlope/2)
		out = append(out, Segment{k.a.Add(mid), k.b.Add(mid), k.h})
	}
	return out
}

// berms builds the base's banks of regolith: lit by the sun, the top a
// shade darker where dust has packed.
func (b *building) berms() {
	for _, k := range allBanks() {
		along := k.b.Sub(k.a).Norm()
		up := vec3{0, k.h, 0}
		inA, inB := vec3{}, vec3{}
		if k.hipA {
			inA = along.Scale(k.run())
		}
		if k.hipB {
			inB = along.Scale(-k.run())
		}
		ta, tb := k.a.Add(inA).Add(up), k.b.Add(inB).Add(up)
		oa, ob := ta.Add(k.out.Scale(k.top)), tb.Add(k.out.Scale(k.top))
		toe := k.out.Scale(k.run())
		a1, b1 := k.a.Add(toe), k.b.Add(toe)
		center := k.a.Lerp(k.b, 0.5).Add(k.out.Scale(k.top / 2)).Add(up.Scale(0.3))
		b.Part("regolith", mesh.LitSun, vec3{}).Hull(center,
			[]vec3{oa, ob, b1, a1},
			[]vec3{k.a, ta, oa, a1},
			[]vec3{k.b, b1, ob, tb})
		b.Part("regolith.dark", mesh.LitSun, vec3{}).Hull(center, []vec3{ta, tb, ob, oa})
	}
}
