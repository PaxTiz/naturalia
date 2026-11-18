package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/planet"
	_ "naturalia/packs/core"
	"testing"
)

func TestBays(t *testing.T) {
	c := LoadContent()
	for _, v := range c.Vehicles.All() {
		bay, ok := base.BayOf(v.ID)
		if ok == v.Auto() {
			t.Errorf("%s: bay %v, auto %v", v.ID, ok, v.Auto())
			continue
		}
		if ok && !base.Covered(bay.At) {
			t.Errorf("%s: bay at %v out of the hangar", v.ID, bay.At)
		}
	}
	// Side by side, never overlapping.
	for i, a := range base.Bays {
		for _, b := range base.Bays[i+1:] {
			if math.Abs(a.At.X-b.At.X) < 1 && math.Abs(a.At.Z-b.At.Z) < (a.Width+b.Width)/2-0.01 {
				t.Errorf("bays %s and %s overlap", a.ID, b.ID)
			}
		}
	}
	if base.Covered(vec3{0, 0, 30}) || !base.Covered(vec3{-46, 0, 0}) {
		t.Error("only the halls are covered")
	}
	// The harvester's header fits between the lines.
	v, _ := c.Vehicles.Get("core:harvester")
	bay, _ := base.BayOf(v.ID)
	lo, hi := v.Footprint()
	if hi.Z-lo.Z < 6.4 || bay.Width < hi.Z-lo.Z+2 || math.Abs(bay.Spot.Z-bay.At.Z) > 0.5 {
		t.Errorf("harvester %v..%v in a bay %+v", lo, hi, bay)
	}
}

func TestDriveOutOfBay(t *testing.T) {
	c := LoadContent()
	g := fleetGame(t, c, map[string]int{"core:harvester": 1, "core:drone": 4})
	g.walls = base.Walls()
	g.syncFleet()
	h := g.entityOf("core:harvester")
	g.getIn(h)
	// Drones hang around, right over it: it drives through them.
	for id, e := range g.Ents.Transform.All() {
		if id != h {
			e.Dir = g.Player.Pos.Norm()
		}
	}
	v, _ := g.Driven()
	start := g.Player.Pos
	step(g, v, controls{throttle: 1}, 120)
	if d := planet.ArcDist(start.Norm(), g.Player.Pos.Norm()); d < 6 {
		t.Errorf("the harvester left its bay by %.1f m only", d)
	}
}

func TestDock(t *testing.T) {
	c := LoadContent()
	g := fleetGame(t, c, map[string]int{"core:rover": 1})
	g.syncFleet()
	rover := g.entityOf("core:rover")
	g.getIn(rover)
	bay, _ := base.BayOf("core:rover")
	// Askew, 4 m off its bay, turned away from the door.
	g.Player.Pos = g.World.Relief.Surface(base.ToWorld(bay.Spot.Add(bay.Nose.Scale(3)).Add(bay.Side().Scale(2.5))))
	g.drive.fwd = bay.Nose.Scale(-0.6).Add(bay.Side().Scale(0.8))
	if !g.CanDock() {
		t.Fatal("can't park next to its bay")
	}
	g.startDock()
	v, _ := g.Driven()
	for range 3 * 60 {
		g.driveVehicle(1.0 / 60)
		g.stepPhysics(1.0 / 60)
		g.liftVehicle(1.0 / 60)
		g.updateVehicle(1.0 / 60)
	}
	at := base.ToLocal(g.Player.Pos)
	if math.Hypot(at.X-bay.Spot.X, at.Z-bay.Spot.Z) > 0.05 || g.drive.fwd.Dot(bay.Nose) < 0.999 || g.drive.dock != nil || g.Riding != rover {
		t.Errorf("%s parked at %v, facing %v", v.ID, at, g.drive.fwd)
	}
	// And it drives off again: the base's ground is a plane, rising a
	// little away from its middle, which used to hold it there.
	start := g.Player.Pos
	step(g, v, controls{throttle: 1}, 120)
	if d := g.Player.Pos.Sub(start).Length(); d < 8 {
		t.Errorf("it left its bay by %.2f m only", d)
	}
	// Far out, no.
	g.Player.Pos = g.World.Relief.Surface(planet.Toward(vec3{1, 0, 0}, 200))
	if g.CanDock() {
		t.Error("parks from afar")
	}
}

func TestGarage(t *testing.T) {
	c := LoadContent()
	g := fleetGame(t, c, map[string]int{"core:rover": 1})
	g.syncFleet()
	bay, _ := base.BayOf("core:rover")
	rover := g.entityOf("core:rover")
	e := g.Ents.Transform.Get(rover)
	if d := planet.ArcDist(e.Dir, base.ToWorld(bay.At)); d > 0.01 {
		t.Errorf("the bought rover is %f m from its bay", d)
	}

	// One left in the yard by an older save is put away.
	e.Dir = base.ToWorld(vec3{-5, 0, 18}).Norm()
	g.garageYard()
	if d := planet.ArcDist(e.Dir, base.ToWorld(bay.At)); d > 0.01 {
		t.Errorf("the yard rover is %f m from its bay", d)
	}
	// So is one in the hangar when it stood east of the base.
	e.Dir = base.ToWorld(vec3{45, 0, 20}).Norm()
	g.garageYard()
	if d := planet.ArcDist(e.Dir, base.ToWorld(bay.At)); d > 0.01 {
		t.Errorf("the rover of the old hangar is %f m from its bay", d)
	}

	// Getting out in the hangar parks it; out on Mars, it stays there.
	g.Riding = rover
	e.Dir = base.ToWorld(vec3{-40, 0, 10}).Norm()
	g.Player.Pos = base.ToWorld(vec3{-40, base.FloorTop, 10})
	g.getOut()
	if d := planet.ArcDist(e.Dir, base.ToWorld(bay.At)); d > 0.01 {
		t.Errorf("got out in the hangar: the rover is %f m from its bay", d)
	}
	far := planet.Toward(vec3{1, 0, 0}, 200)
	g.Riding = rover
	e.Dir = far
	g.Player.Pos = g.World.Relief.Surface(far)
	g.getOut()
	if e.Dir != far {
		t.Error("got out on Mars: the rover went home")
	}
}

func TestHangarDoor(t *testing.T) {
	g := baseGame(t)
	i := base.Halls()[0]
	mid, _ := base.HallDoor(i)
	hall := base.Ents.Building.Get(i)
	out := hall.TurnDir(vec3{1, 0, 0}) // through the door, outward
	outside := base.ToWorld(mid.Add(out.Scale(3)).Add(vec3{0, base.FloorTop, 0}))

	// Open, the leaves stand within the hangar's ends.
	hz := hall.Hall.Size[1] / 2
	for _, l := range base.HallLeaves(i, 1) {
		if d := math.Abs(l.At.Z-hall.Center.Z) + l.Size.Z/2; d > hz {
			t.Errorf("a leaf sticks out %.1f m", d-hz)
		}
	}

	// Far away it stays shut, and walls the hangar off.
	g.updateHallDoors(1)
	// stopped tells if the door stops a player walking in from 1 m out,
	// z m from its middle.
	stopped := func(z float64) bool {
		was := g.Player.Pos
		g.Player.Pos, g.Player.vel = g.World.Relief.Surface(base.ToWorld(mid.Add(out).Add(vec3{0, 1, z}))), vec3{}
		walk(g, out.Scale(-1), 0.5)
		in := base.ToLocal(g.Player.Pos).Sub(mid).Dot(out) < 0.3
		g.Player.Pos = was
		return !in
	}
	if g.HallOpen[0] != 0 || !stopped(0) {
		t.Fatalf("door far: open %f, walked in", g.HallOpen[0])
	}

	// From inside, only close to it: not from the middle of the hangar.
	g.Player.Pos = base.ToWorld(mid.Add(out.Scale(-15)).Add(vec3{0, base.FloorTop, 0}))
	g.updateHallDoors(1)
	if g.HallOpen[0] != 0 {
		t.Errorf("opens from 15 m inside: %f", g.HallOpen[0])
	}
	g.Player.Pos = base.ToWorld(mid.Add(out.Scale(-8)).Add(vec3{0, base.FloorTop, 0}))
	g.updateHallDoors(1)
	if g.HallOpen[0] == 0 {
		t.Error("doesn't open from 8 m inside")
	}
	g.HallOpen[0] = 0

	// Near it, it opens slowly; a second later, the player already goes
	// through the middle, though not past its sides.
	g.Player.Pos = outside
	g.updateHallDoors(1)
	if o := g.HallOpen[0]; o <= 0 || o >= 1 {
		t.Errorf("after 1 s the door is %f open", o)
	}
	if stopped(0) || stopped(2) {
		t.Error("can't go through the opening door")
	}
	if !stopped(hall.Hall.Door[0]/2 - 1) {
		t.Error("went through a leaf")
	}
	for range 5 {
		g.updateHallDoors(1)
	}
	if g.HallOpen[0] != 1 || len(g.wallsNow()) != len(g.walls)+2*len(base.InnerDoors()) { // the corridor's door is shut
		t.Errorf("door near: open %f", g.HallOpen[0])
	}
}

func TestCorridorToHangar(t *testing.T) {
	g := baseGame(t)
	// From the great hall, west through the corridor, into the hangar,
	// indoors all the way.
	// Its door at the hangar's end opens as they come, walking at 4 m/s.
	west := func(steps int, doors bool) vec3 {
		for range steps {
			if doors {
				g.updateInnerDoors(1.0 / 60)
			}
			g.Player.want = planet.TangentAt(g.Player.LocalUp(), vec3{-1, 0, 0}).Scale(4)
			g.stepPhysics(1.0 / 60)
			g.Player.fall(1.0/60, g.World.Relief)
			if at := base.ToLocal(g.Player.Pos); !base.Indoors(vec3{at.X, 1, at.Z}) {
				t.Fatalf("outdoors at %v", at)
			}
		}
		return base.ToLocal(g.Player.Pos)
	}
	g.Player.Pos = g.World.Relief.Surface(base.ToWorld(vec3{-9, 1, 0}))
	if at := west(330, true); at.X > -29.9 {
		t.Errorf("stopped at %v", at)
	}
	// Shut, it stops them.
	g.Player.Pos, g.Player.vel = g.World.Relief.Surface(base.ToWorld(vec3{-21, 1, 0})), vec3{}
	g.InnerOpen = []float64{0}
	if at := west(30, false); at.X < -22 {
		t.Errorf("went through the shut door: %v", at)
	}
	// The bays leave the aisle from the doorway free.
	for _, b := range base.Bays {
		if math.Abs(b.At.Z) < b.Width/2+1.5 {
			t.Errorf("bay %s at %v blocks the aisle", b.ID, b.At)
		}
	}
}

func TestGarageShop(t *testing.T) {
	c := LoadContent()
	g := fleetGame(t, c, map[string]int{"core:drone": 1})
	g.State.Credits = 1e9
	item := func(id string) ShopItem {
		for _, it := range g.ShopItems() {
			if it.id == id {
				return it
			}
		}
		t.Fatalf("no %s in the shop (garage %v)", id, g.Garage)
		return ShopItem{}
	}
	// The garage sells the vehicles to drive, the lab the rest.
	g.Garage = true
	for _, it := range g.ShopItems() {
		if v, ok := c.Vehicles.Get(it.id); !ok || v.Auto() {
			t.Errorf("the garage sells %s", it.id)
		}
	}
	g.Buy(item("core:rover"))
	g.Buy(item("core:buggy"))
	if g.Entities("core:rover") != 1 || g.Entities("core:buggy") != 1 {
		t.Fatal("bought vehicles missing")
	}

	// Sold back while driving the buggy, which comes after it.
	g.getIn(g.entityOf("core:buggy"))
	before := g.State.Credits
	rover := item("core:rover")
	g.Sell(rover)
	if g.Entities("core:rover") != 0 || g.State.Levels["core:rover"] != 0 || g.State.Credits != before+rover.Refund || rover.Refund != 7500 {
		t.Errorf("sold rover: %d left, credits +%f", g.Entities("core:rover"), g.State.Credits-before)
	}
	if g.kind(g.Riding) != "core:buggy" {
		t.Error("lost track of the buggy driven")
	}
	// Not the one driven.
	g.Sell(item("core:buggy"))
	if g.Entities("core:buggy") != 1 {
		t.Error("sold the buggy from under the player")
	}
	// And it can be bought again.
	if it := item("core:rover"); it.Maxed || it.Refund != 0 {
		t.Errorf("rover after selling: %+v", it)
	}

	g.Garage = false
	item("core:drone")
	for _, it := range g.ShopItems() {
		if v, ok := c.Vehicles.Get(it.id); ok && !v.Auto() {
			t.Errorf("the lab sells %s", it.id)
		}
	}
}

func TestGarageTerminal(t *testing.T) {
	c := LoadContent()
	levels := map[string]int{}
	for _, v := range c.Vehicles.All() {
		if !v.Auto() {
			levels[v.ID] = 1
		}
	}
	g := fleetGame(t, c, levels)
	g.syncFleet()
	// In front of it, facing it, every vehicle in its bay: E opens the
	// garage, not a vehicle.
	var at vec3
	for _, it := range base.Interactables() {
		if it.Kit == "core:garage.terminal" {
			at = it.At
		}
	}
	g.Player.Pos = base.ToWorld(at.Add(vec3{1.4, base.FloorTop, 0}))
	g.Player.Heading = vec3{-1, 0, 0}
	if it, ok := g.NearbyUse(); !ok || it.Kit != "core:garage.terminal" {
		t.Errorf("E in front of the terminal: %q", it.Kit)
	}
}

// A hall turned a quarter turn: its door, leaves, walls and bays turn with
// it.
func TestTurnedHall(t *testing.T) {
	defer LoadContent()
	LoadContent(content.Pack{ID: "t", Requires: []string{"core"}, Register: func(c *content.Content) {
		h, _ := c.Buildings.Get("core:hangar")
		h.Yaw, h.At, h.Link = 90, vec3{0, 0, -75}, ""
	}})
	i := base.Halls()[0]
	r := base.Ents.Building.Get(i)
	mid, closed := base.HallDoor(i)
	if mid.Sub(vec3{0, 0, -62}).Length() > 1e-9 {
		t.Errorf("door at %v", mid)
	}
	if a, b, _ := closed.Ends(); math.Abs(a.Z+62) > 1e-9 || math.Abs(b.Z+62) > 1e-9 || math.Abs(a.X-b.X) < 50 {
		t.Errorf("door's wall from %v to %v", a, b)
	}
	if !base.Covered(vec3{30, 0, -75}) || base.Covered(vec3{0, 0, -96}) {
		t.Error("the hall's floor didn't turn")
	}
	for _, l := range base.HallLeaves(i, 0) {
		if out := l.At.Z + 62; l.Yaw != 90 || out < 0 || out > 3 || math.Abs(l.At.X) > 26 { // on their rails, outside
			t.Errorf("closed leaf at %v", l.At)
		}
	}
	if n := len(base.HallDoorWalls(i, 0.5)); n != 2 {
		t.Errorf("%d walls half open", n)
	}
	for _, b := range base.Bays {
		if !r.Plan().Garage || !base.Covered(b.At) || b.Nose.Sub(vec3{0, 0, 1}).Length() > 1e-9 || !base.Covered(b.Spot) {
			t.Errorf("bay %s at %v, nose %v", b.ID, b.At, b.Nose)
		}
	}
	for _, s := range base.Walls() {
		a, b, _ := s.Ends()
		if a.Z < -60 && math.Abs(a.Z-b.Z) > 1e-6 && math.Abs(a.X-b.X) > 1e-6 {
			t.Errorf("a hall wall askew: %v to %v", a, b)
		}
	}
}
