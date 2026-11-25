package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/planet"
)

// The garages set down anywhere (base.Camp.Bays): their big door opens as
// the player comes, on foot or driving; a vehicle that fits is parked in one
// of their bays, and their terminal brings there one left elsewhere.

const (
	campHallReach = 12.0 // m from its middle where the big door opens, from outside
	campHallInner = 5.0  // and from inside
	campHallSpeed = 0.6  // share of its travel per second
)

// BigDoor is how open the big door of a building set down is, 0 to 1.
func (g *Game) BigDoor(id ecs.ID) float64 { return g.doors[doorKey{id, -2}].open }

// updateCampHalls opens the big doors near.
func (g *Game) updateCampHalls(dt float64) {
	for _, c := range g.CampsNear(g.Player.Pos, 40) {
		mid, width, ok := c.Layout.BigDoor()
		if !ok {
			continue
		}
		at := c.Site.ToLocal(g.Player.Pos)
		d := at.Sub(mid)
		want := 0.0
		if inside := c.Layout.Indoors(vec3{at.X, 1, at.Z}); inside && math.Abs(d.X) < campHallInner && math.Abs(d.Z) < width/2+2 ||
			!inside && math.Hypot(d.X, d.Z) < campHallReach {
			want = 1
		}
		if g.doors == nil {
			g.doors = map[doorKey]door{}
		}
		k := doorKey{c.Ent, -2}
		was := g.doors[k].open
		if (was == 0 && want == 1) || (was == 1 && want == 0) {
			g.sfx.Beep(0.25, 0.6, 0.12, 110, 98, 82) // the motors start
		}
		g.doors[k] = door{approach(was, want, campHallSpeed*dt), want}
	}
}

// inBay is the vehicle standing in a bay of building c, 0 for none.
func (g *Game) inBay(c CampAt, b base.Bay) ecs.ID {
	for id, t := range g.Ents.Transform.All() {
		if v, ok := g.vehicleOf(id); !ok || v.Auto() || id == g.Riding || planet.ArcDist(t.Dir, c.Site.Origin.Norm()) > c.Layout.Reach+10 {
			continue
		}
		if b.Holds(c.Site.ToLocal(g.World.Relief.Surface(t.Dir)), 0.3) {
			return id
		}
	}
	return 0
}

// freeBay is the bay of building c closest to the player where vehicle v
// goes, if any is free.
func (g *Game) freeBay(c CampAt, v *content.Vehicle) (base.Bay, bool) {
	best, found, near := base.Bay{}, false, math.Inf(1)
	at := c.Site.ToLocal(g.Player.Pos)
	for _, b := range c.Layout.Bays {
		if d := b.At.Sub(vec3{at.X, 0, at.Z}).Length(); b.Fits(v) && d < near && g.inBay(c, b) == 0 {
			best, found, near = b, true, d
		}
	}
	return best, found
}

// parkIn sets vehicle id in a bay of building c, nose to the door.
func (g *Game) parkIn(c CampAt, b base.Bay, v *content.Vehicle, id ecs.ID) {
	e := g.Ents.Transform.Get(id)
	e.Dir = c.ToWorld(b.SpotOf(v)).Norm()
	e.Yaw = yawOf(e.Dir, planet.TangentAt(e.Dir, c.Site.Dir(b.Nose)))
	e.Alt, e.Pitch, e.Roll = v.Hover, 0, 0
	g.dropBodies(id)
}

// leaveInCamp has the player step out of vehicle id in building c: it goes
// to a bay if one is free.
func (g *Game) leaveInCamp(c CampAt, v *content.Vehicle, id ecs.ID) {
	b, ok := g.freeBay(c, v)
	if !ok {
		side := g.Player.Right().Scale(content.OrZero(v.Radius, 1.2) + 0.6)
		g.Player.Pos = g.World.Relief.Surface(g.Player.Pos.Add(side))
		return
	}
	g.parkIn(c, b, v, id)
	g.Player.Pos = g.World.Relief.Surface(c.ToWorld(b.At.Add(b.Nose.Scale(b.Length/2 + 0.8))))
	g.Say(v.Name + " rangé au garage")
}

// callItem is a vehicle owned, at the terminal of building c's garage: where
// it is, and whether it is brought there.
func (g *Game) callItem(it *ShopItem, c CampAt, v *content.Vehicle) {
	id := g.entityOf(v.ID)
	fits := false
	for _, b := range c.Layout.Bays {
		if fits = fits || b.Fits(v); id != 0 && g.inBay(c, b) == id {
			it.Info = "Ici"
			return
		}
	}
	switch {
	case !fits:
		it.Info = "Trop grand pour ce garage"
	case id != 0 && id != g.Riding:
		if it.Info == "Au garage" {
			it.Info = "À la base"
		}
		it.Refund, it.sell = 0, nil // brought here first
		it.Call, it.call = true, func() { g.callVehicle(c, v, id) }
	}
}

// callVehicle brings vehicle id to building c: to a free bay, or in the
// place of another, which goes back to its own at the base.
func (g *Game) callVehicle(c CampAt, v *content.Vehicle, id ecs.ID) {
	b, ok := g.freeBay(c, v)
	for i := 0; !ok && i < len(c.Layout.Bays); i++ {
		b = c.Layout.Bays[i]
		other := g.inBay(c, b)
		if o, is := g.vehicleOf(other); b.Fits(v) && is && g.park(o, other) {
			g.dropBodies(other)
			g.Say(o.Name + " renvoyé à la base")
			ok = true
		}
	}
	if !ok {
		g.Say("Plus de place dans ce garage")
		g.sfx.Beep(0, 0.15, 0.1, 180)
		return
	}
	g.parkIn(c, b, v, id)
	g.Say(v.Name + " amené ici")
	g.sfx.Beep(0.06, 0.18, 0.14, 523, 784)
}
