package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/planet"
)

// The garage (base.Bays): every driven vehicle has its bay in the hangar.
// Bought ones are delivered there, getting out inside the hangar parks
// the vehicle in its bay, and those left in the yard by older saves are
// put away when the game loads. The hangar's door opens as the player, on
// foot or driving, comes near it, from inside or out.

const (
	hallDoorReach = 24.0 // m from the door's middle where it opens
	hallDoorSpeed = 0.35 // share of its travel per second: a giant door is slow
)

// park sets vehicle v in its bay: entity i, or a new one if i < 0. It
// tells if v has a bay.
func (g *Game) park(v *content.Vehicle, i int) bool {
	bay, ok := base.BayOf(v.ID)
	if !ok {
		return false
	}
	if i < 0 {
		if !g.spawn(v.ID, base.ToWorld(bay.At)) {
			return false
		}
		i = len(g.State.Entities) - 1
	}
	e := &g.State.Entities[i]
	e.Dir = base.ToWorld(bay.At).Norm()
	e.Yaw = yawOf(e.Dir, vec3{1, 0, 0}) // nose to the door
	e.Alt, e.Pitch, e.Roll = v.Hover, 0, 0
	return true
}

// garageYard puts away the driven vehicles standing in the base's yard,
// where they used to wait.
func (g *Game) garageYard() {
	for i, e := range g.State.Entities {
		v, ok := g.Content.Vehicles.Get(e.Kind)
		if !ok || v.Auto() || i == g.Riding || planet.ArcDist(e.Dir, planet.BaseUp) > base.Reach+10 {
			continue
		}
		g.park(v, i)
	}
}

// updateHallDoors opens the halls' doors when the player comes near.
func (g *Game) updateHallDoors(dt float64) {
	halls := base.Halls()
	if len(g.HallOpen) != len(halls) {
		g.HallOpen = make([]float64, len(halls))
	}
	at := base.ToLocal(g.Player.Pos)
	for k, i := range halls {
		mid, _ := base.HallDoor(i)
		want := 0.0
		if math.Hypot(at.X-mid.X, at.Z-mid.Z) < hallDoorReach {
			want = 1
		}
		was := g.HallOpen[k]
		g.HallOpen[k] = approach(was, want, hallDoorSpeed*dt)
		if (was == 0 && want == 1) || (was == 1 && want == 0) {
			g.sfx.Beep(0.25, 0.6, 0.12, 110, 98, 82) // the motors start
		}
	}
}

// wallsNow are the base's walls, with the halls' doors where they are
// still closed enough to stop the player.
func (g *Game) wallsNow() []base.Segment {
	walls := g.walls
	for k, i := range base.Halls() {
		if k < len(g.HallOpen) && g.HallOpen[k] > 0.85 {
			continue
		}
		_, door := base.HallDoor(i)
		walls = append(walls[:len(walls):len(walls)], door)
	}
	return walls
}
