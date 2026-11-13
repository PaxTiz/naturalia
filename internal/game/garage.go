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
// put away when the game loads. Driving near its bay, G parks it there
// by itself, right in the middle. The hangar's door opens as the player, on
// foot or driving, comes near it, from inside or out.

const (
	hallDoorReach = 24.0 // m from the door's middle where it opens
	hallDoorSpeed = 0.35 // share of its travel per second: a giant door is slow
	dockReach     = 6.0  // m from its bay's edge where G parks a vehicle
	dockTime      = 1.6  // seconds it takes, gliding in
)

// park sets vehicle v in its bay: entity i, or a new one if i < 0. It
// tells if v has a bay.
func (g *Game) park(v *content.Vehicle, i int) bool {
	bay, ok := base.BayOf(v.ID)
	if !ok {
		return false
	}
	if i < 0 {
		if !g.spawn(v.ID, base.ToWorld(bay.Spot)) {
			return false
		}
		i = len(g.State.Entities) - 1
	}
	e := &g.State.Entities[i]
	e.Dir = base.ToWorld(bay.Spot).Norm()
	e.Yaw = yawOf(e.Dir, vec3{1, 0, 0}) // nose to the door
	e.Alt, e.Pitch, e.Roll = v.Hover, 0, 0
	return true
}

// garageYard puts away the driven vehicles standing in the base's yard,
// where they used to wait, and those in the hangar, in their bays as they
// are now.
func (g *Game) garageYard() {
	for i, e := range g.State.Entities {
		v, ok := g.Content.Vehicles.Get(e.Kind)
		home := planet.ArcDist(e.Dir, planet.BaseUp) < base.Reach+10 || base.Covered(base.ToLocal(e.Dir.Scale(planet.Radius)))
		if !ok || v.Auto() || i == g.Riding || !home {
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
	d.vel, d.want = vec3{}, g.Player.Pos
	if d.dock.t >= 1 {
		d.dock = nil
		g.Say(v.Name + " garé")
	}
}
