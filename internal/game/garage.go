package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/planet"
)

// The garage (base.Bays): every driven vehicle has its bay in the hangar.
// Bought ones are delivered there, getting out inside the hangar parks
// the vehicle in its bay, and those left in the yard by older saves are
// put away when the game loads. Driving near its bay, G parks it there
// by itself, right in the middle. The hangar's door opens as the player, on
// foot or driving, comes near it, from inside or out.

const (
	hallDoorReach = 24.0 // m from the door's middle where it opens, coming from outside
	hallDoorInner = 10.0 // m from the door where it opens, from inside the hall
	hallDoorSpeed = 0.35 // share of its travel per second: a giant door is slow
	dockReach     = 6.0  // m from its bay's edge where G parks a vehicle
	dockTime      = 1.6  // seconds it takes, gliding in
)

// park sets vehicle v in its bay: entity id, or a new one if 0. It tells
// if v has a bay.
func (g *Game) park(v *content.Vehicle, id ecs.ID) bool {
	bay, ok := base.BayOf(v.ID)
	if !ok {
		return false
	}
	if id == 0 {
		if id = g.spawn(v.ID, base.ToWorld(bay.Spot)); id == 0 {
			return false
		}
	}
	e := g.Ents.Transform.Get(id)
	e.Dir = base.ToWorld(bay.Spot).Norm()
	e.Yaw = yawOf(e.Dir, vec3{1, 0, 0}) // nose to the door
	e.Alt, e.Pitch, e.Roll = v.Hover, 0, 0
	return true
}

// garageYard puts away the driven vehicles standing in the base's yard,
// where they used to wait, and those in the hangar, in their bays as they
// are now.
func (g *Game) garageYard() {
	for id, e := range g.Ents.Transform.All() {
		v, ok := g.vehicleOf(id)
		home := planet.ArcDist(e.Dir, planet.BaseUp) < base.Reach+10 || base.Covered(base.ToLocal(e.Dir.Scale(planet.Radius)))
		if !ok || v.Auto() || id == g.Riding || !home {
			continue
		}
		g.park(v, id)
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
		mid, closed := base.HallDoor(i)
		want := 0.0
		inside := base.RoomAt(vec3{at.X, 1, at.Z}) == i
		switch {
		case inside && math.Abs(at.X-mid.X) < hallDoorInner && math.Abs(at.Z-mid.Z) < closed.Half()+4:
			want = 1 // walking or driving up to it
		case !inside && math.Hypot(at.X-mid.X, at.Z-mid.Z) < hallDoorReach:
			want = 1
		}
		was := g.HallOpen[k]
		g.HallOpen[k] = approach(was, want, hallDoorSpeed*dt)
		if (was == 0 && want == 1) || (was == 1 && want == 0) {
			g.sfx.Beep(0.25, 0.6, 0.12, 110, 98, 82) // the motors start
		}
	}
}

// innerReach is how close to a hall's corridor door it opens, in m.
const innerReach = 3.5

// updateInnerDoors opens the halls' corridor doors as the player comes.
func (g *Game) updateInnerDoors(dt float64) {
	doors := base.InnerDoors()
	if len(g.InnerOpen) != len(doors) {
		g.InnerOpen = make([]float64, len(doors))
	}
	at := base.ToLocal(g.Player.Pos)
	for k, d := range doors {
		want := 0.0
		if math.Hypot(at.X-d.Mid.X, at.Z-d.Mid.Z) < innerReach {
			want = 1
		}
		g.InnerOpen[k] += (want - g.InnerOpen[k]) * math.Min(1, dt*4)
	}
}

// wallsNow are the base's walls, with the halls' door leaves where they
// stand: the player goes through as soon as the opening lets them.
func (g *Game) wallsNow() []base.Segment {
	walls := g.walls[:len(g.walls):len(g.walls)]
	for k, i := range base.Halls() {
		open := 0.0
		if k < len(g.HallOpen) {
			open = g.HallOpen[k]
		}
		walls = append(walls, base.HallDoorWalls(i, open)...)
	}
	for k, d := range base.InnerDoors() {
		open := 0.0
		if k < len(g.InnerOpen) {
			open = g.InnerOpen[k]
		}
		walls = append(walls, d.Walls(open)...)
	}
	return walls
}

// docking is a vehicle gliding into its bay: from where (local) and which
// way it pointed (radians from +X toward +Z), how far along (0 to 1).
type docking struct {
	from vec3
	yaw  float64
	t    float64
}

// CanDock tells if G parks the driven vehicle: in a hall, slowly, near
// its bay.
func (g *Game) CanDock() bool {
	v, ok := g.Driven()
	if !ok || g.drive.dock != nil || g.DriveAlt() > 0.5 || g.DriveSpeed() > 6 {
		return false
	}
	bay, ok := base.BayOf(v.ID)
	at := base.ToLocal(g.Player.Pos)
	if !ok || !base.Covered(at) {
		return false
	}
	return math.Abs(at.X-bay.At.X) < bay.Length/2+dockReach && math.Abs(at.Z-bay.At.Z) < bay.Width/2+dockReach
}

// startDock sets the driven vehicle gliding into its bay.
func (g *Game) startDock() {
	at := base.ToLocal(g.Player.Pos)
	fwd := base.ToLocal(g.Player.Pos.Add(g.drive.fwd)).Sub(at)
	g.drive.dock = &docking{from: vec3{at.X, 0, at.Z}, yaw: math.Atan2(fwd.Z, fwd.X)}
	g.drive.vel = vec3{}
	g.sfx.Beep(0.08, 0.15, 0.1, 440, 554, 659)
}

// glideDock moves the vehicle on into its bay, and parks it there at the
// end, the player still in it.
func (g *Game) glideDock(v *content.Vehicle, dt float64) {
	d := &g.drive
	bay, _ := base.BayOf(v.ID)
	d.dock.t = math.Min(1, d.dock.t+dt/dockTime)
	k := d.dock.t * d.dock.t * (3 - 2*d.dock.t)
	at := d.dock.from.Lerp(vec3{bay.Spot.X, 0, bay.Spot.Z}, k)
	yaw := d.dock.yaw * (1 - k) // toward 0: the nose to the door
	g.Player.Pos = g.World.Relief.Surface(base.ToWorld(at))
	up := g.Player.LocalUp()
	d.fwd = planet.TangentAt(up, vec3{math.Cos(yaw), 0, math.Sin(yaw)})
	g.Player.Heading = d.fwd
	d.vel, d.spin = vec3{}, 0
	if d.dock.t >= 1 {
		d.dock = nil
		g.Say(v.Name + " garé")
	}
}
