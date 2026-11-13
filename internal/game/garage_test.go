package game

import (
	"math"
	"naturalia/internal/base"
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
	if base.Covered(vec3{0, 0, 0}) || !base.Covered(vec3{39, 0, 0}) {
		t.Error("only the hangar is covered")
	}
}

func TestGarage(t *testing.T) {
	c := LoadContent()
	g := fleetGame(t, c, map[string]int{"core:rover": 1})
	g.syncFleet()
	bay, _ := base.BayOf("core:rover")
	e := g.State.Entities[0]
	if d := planet.ArcDist(e.Dir, base.ToWorld(bay.At)); d > 0.01 {
		t.Errorf("the bought rover is %f m from its bay", d)
	}

	// One left in the yard by an older save is put away.
	g.State.Entities[0].Dir = base.ToWorld(vec3{-5, 0, 18}).Norm()
	g.garageYard()
	if d := planet.ArcDist(g.State.Entities[0].Dir, base.ToWorld(bay.At)); d > 0.01 {
		t.Errorf("the yard rover is %f m from its bay", d)
	}

	// Getting out in the hangar parks it; out on Mars, it stays there.
	g.Riding = 0
	g.State.Entities[0].Dir = base.ToWorld(vec3{40, 0, 10}).Norm()
	g.Player.Pos = base.ToWorld(vec3{40, base.FloorTop, 10})
	g.getOut()
	if d := planet.ArcDist(g.State.Entities[0].Dir, base.ToWorld(bay.At)); d > 0.01 {
		t.Errorf("got out in the hangar: the rover is %f m from its bay", d)
	}
	far := planet.Toward(vec3{1, 0, 0}, 200)
	g.Riding = 0
	g.State.Entities[0].Dir = far
	g.Player.Pos = g.World.Relief.Surface(far)
	g.getOut()
	if g.State.Entities[0].Dir != far {
		t.Error("got out on Mars: the rover went home")
	}
}

func TestHangarDoor(t *testing.T) {
	c := LoadContent()
	g := fleetGame(t, c, map[string]int{})
	i := base.Halls()[0]
	mid, _ := base.HallDoor(i)
	outside := base.ToWorld(mid.Add(vec3{3, base.FloorTop, 0}))

	// Far away it stays shut, and walls the hangar off.
	g.updateHallDoors(1)
	if g.HallOpen[0] != 0 || len(g.wallsNow()) != len(g.walls)+1 {
		t.Fatalf("door far: open %f", g.HallOpen[0])
	}

	// Near it, it opens slowly, then lets the player in.
	g.Player.Pos = outside
	g.updateHallDoors(1)
	if o := g.HallOpen[0]; o <= 0 || o >= 1 {
		t.Errorf("after 1 s the door is %f open", o)
	}
	for range 5 {
		g.updateHallDoors(1)
	}
	if g.HallOpen[0] != 1 || len(g.wallsNow()) != len(g.walls) {
		t.Errorf("door near: open %f", g.HallOpen[0])
	}
}
