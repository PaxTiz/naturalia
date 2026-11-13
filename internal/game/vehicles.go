package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/mathx"
	"naturalia/internal/planet"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Vehicles (content.Vehicle) are entities that sweep. The ones the player
// drives wait on the landing pad; automatic ones work on their own: some
// follow the player, others sweep around the spot where they were set
// down. State.Levels counts how many of each were bought.

// padSpot is where bought vehicles wait.
var padSpot = vec3{11, 0, -12.5}

const (
	// While the player is at the base, followers circle above the hub...
	dockAltitude = 7.5
	dockRadius   = 3.5
	// ...and anywhere near it, they fly over the domes, not through them.
	domesReach = base.Reach + 6
)

// drive is the driven vehicle's motion, tangent to the ground.
type drive struct {
	fwd  vec3 // where its nose points
	vel  vec3
	want vec3 // where it went before walls and rocks pushed it back
	// boarded is set on the frame the player gets in, whose E press must
	// not get them out again.
	boarded bool
}

// buyVehicle adds one to the fleet.
func (g *Game) buyVehicle(v *content.Vehicle) {
	g.State.Levels[v.ID]++
	g.syncFleet()
}

// owned counts the vehicles of a kind standing in the world or carried.
func (g *Game) owned(v *content.Vehicle) int {
	return g.Entities(v.ID) + g.State.Items[v.ID]
}

// syncFleet brings every vehicle bought into the world: driven ones on the
// landing pad, followers next to the player, patrolling ones into the
// inventory. It also brings back the drones of saves where they were an
// upgrade, under the same ID.
func (g *Game) syncFleet() {
	for _, v := range g.Content.Vehicles.All() {
		for n := g.owned(v); n < g.State.Levels[v.ID]; n++ {
			switch {
			case v.Patrol():
				g.GiveItem(v.ID, 1)
			case v.Auto():
				g.spawn(v.ID, g.Player.Pos)
				g.State.Entities[len(g.State.Entities)-1].Alt = v.Hover
			default:
				off := float64(n%4) * 0.8
				g.spawn(v.ID, g.World.Relief.Surface(base.ToWorld(padSpot.Add(vec3{off - 1.2, 0, off - 1.2}))))
			}
		}
	}
}

// Driven is the vehicle the player drives, if any.
func (g *Game) Driven() (*content.Vehicle, bool) {
	if g.Riding < 0 || g.Riding >= len(g.State.Entities) {
		return nil, false
	}
	return g.Content.Vehicles.Get(g.State.Entities[g.Riding].Kind)
}

// DriveSpeed is how fast the driven vehicle goes, in m/s.
func (g *Game) DriveSpeed() float64 {
	if g.Riding < 0 {
		return 0
	}
	return g.drive.vel.Length()
}

// getIn or out of the vehicle standing as entity i.
func (g *Game) getIn(i int) {
	e := g.State.Entities[i]
	g.Riding = i
	t1, t2 := mathx.TangentBasis(e.Dir)
	g.drive = drive{fwd: t1.Scale(math.Cos(e.Yaw)).Add(t2.Scale(math.Sin(e.Yaw))), boarded: true}
	g.Player.Pos = g.World.Relief.Surface(e.Dir)
	g.Player.Heading, g.Player.seated = g.drive.fwd, true
	g.sfx.Beep(0.02, 0.2, 0.12, 220, 330)
	g.emit(content.Event{Name: content.EvDrive, ID: e.Kind})
}

func (g *Game) getOut() {
	v, ok := g.Driven()
	g.Riding = -1
	g.Player.seated, g.Player.eyeH = false, 0
	g.drive = drive{}
	if !ok {
		return
	}
	side := g.Player.Right().Scale(content.OrZero(v.Radius, 1.2) + 0.6)
	g.Player.Pos = g.World.Relief.Surface(g.Player.Pos.Add(side))
	g.sfx.Beep(0.02, 0.2, 0.12, 330, 220)
	g.emit(content.Event{Name: content.EvDrive, ID: ""})
}

// driveVehicle moves the driven vehicle, with the player in it, before the
// collisions: Z/S throttle and brake, Q/D steer. Wheels grip the ground;
// what hovers slides on.
func (g *Game) driveVehicle(dt float64) {
	v, ok := g.Driven()
	if !ok {
		if g.Player.seated {
			g.getOut()
		}
		return
	}
	d := &g.drive
	if g.Mode == ModePlay && rl.IsKeyPressed(rl.KeyE) && !d.boarded {
		g.getOut()
		return
	}
	d.boarded = false
	var throttle, steer float64
	if g.Mode == ModePlay {
		// Physical key positions: WASD on QWERTY is ZQSD on AZERTY.
		if rl.IsKeyDown(rl.KeyW) {
			throttle++
		}
		if rl.IsKeyDown(rl.KeyS) {
			throttle--
		}
		if rl.IsKeyDown(rl.KeyA) {
			steer++
		}
		if rl.IsKeyDown(rl.KeyD) {
			steer--
		}
	}
	g.move(v, throttle, steer, dt)
}

// move drives the vehicle for dt: throttle forward (1) or back (-1),
// steer left (1) or right (-1).
func (g *Game) move(v *content.Vehicle, throttle, steer, dt float64) {
	d := &g.drive
	up := g.Player.LocalUp()
	d.fwd = planet.TangentAt(up, d.fwd)
	d.vel = d.vel.Sub(up.Scale(d.vel.Dot(up)))
	top := v.Speed
	accel, brake := top*0.5, top*1.5
	s := d.vel.Dot(d.fwd)
	side := d.vel.Sub(d.fwd.Scale(s))
	switch {
	case throttle > 0 && s < 0, throttle < 0 && s > 0:
		s = approach(s, 0, brake*dt)
	case throttle > 0:
		s = approach(s, top, accel*dt)
	case throttle < 0:
		s = approach(s, -top*0.35, accel*dt)
	default:
		s = approach(s, 0, top*0.4*dt)
	}
	side = side.Scale(math.Exp(-content.OrZero(v.Grip, 8) * dt))

	// Wheels only turn when rolling, and backward when backing up; a
	// hovering vehicle spins in place too.
	grip := math.Min(1, math.Abs(s)/(top*0.3))
	if v.Hover > 0 {
		grip = math.Max(grip, 0.6)
	}
	if s < -0.1 {
		grip = -grip
	}
	// The nose turns, the motion lags: grip pulls it along next frames.
	d.vel = d.fwd.Scale(s).Add(side)
	if a := steer * content.OrZero(v.Turn, 1.2) * grip * dt; a != 0 {
		d.fwd = d.fwd.Scale(math.Cos(a)).Add(up.Cross(d.fwd).Scale(math.Sin(a)))
		g.Player.Heading = g.Player.Heading.Scale(math.Cos(a)).Add(up.Cross(g.Player.Heading).Scale(math.Sin(a)))
	}
	d.want = g.World.Relief.Surface(g.Player.Pos.Add(d.vel.Scale(dt)))
	g.Player.Pos = d.want
}

// approach moves v toward target by at most step.
func approach(v, target, step float64) float64 {
	if v < target {
		return math.Min(target, v+step)
	}
	return math.Max(target, v-step)
}

// updateVehicle sets the driven vehicle where the collisions left the
// player, and sweeps under it.
func (g *Game) updateVehicle(dt float64) {
	v, ok := g.Driven()
	if !ok {
		return
	}
	d := &g.drive
	// Walls and rocks stop the motion that goes into them.
	if push := g.Player.Pos.Sub(d.want); push.Length() > 1e-4 {
		n := push.Norm()
		if into := d.vel.Dot(n); into < 0 {
			d.vel = d.vel.Sub(n.Scale(into))
		}
	}
	e := &g.State.Entities[g.Riding]
	e.Dir = g.Player.Pos.Norm()
	e.Yaw = yawOf(e.Dir, d.fwd)
	e.Alt = 0
	if v.Hover > 0 {
		e.Alt = v.Hover + math.Sin(g.Elapsed*2.2)*0.04
	}
	g.Player.eyeH = e.Alt + content.OrZero(v.Seat, 2.4)
	if g.Mode == ModePlay {
		g.sweepUnder(v, e, dt)
	}
}

// yawOf is the angle of the tangent direction fwd at up, as entities
// store it.
func yawOf(up, fwd vec3) float64 {
	t1, t2 := mathx.TangentBasis(up)
	return math.Atan2(fwd.Dot(t2), fwd.Dot(t1))
}

// sweepUnder cleans the dust under a vehicle into the tank, away from the
// base's ground.
func (g *Game) sweepUnder(v *content.Vehicle, e *Placed, dt float64) {
	if v.Sweep <= 0 || planet.ArcDist(e.Dir, planet.BaseUp) < base.Reach {
		return
	}
	at := g.World.Relief.Surface(e.Dir)
	got := g.World.Clean(at, v.Sweep, v.Rate*g.powerMult()*dt, g.space())
	g.collect(got, at, v.Sweep, at.Add(e.Dir.Scale(e.Alt+0.8)))
	for _, gem := range g.World.CollectGems(at, v.Sweep) {
		g.collectGem(gem)
	}
}

// updateFleet runs the automatic vehicles. They stop while the tank is
// full, waiting for it to be emptied.
func (g *Game) updateFleet(dt float64) {
	if g.fleetHeld {
		return
	}
	docked := planet.ArcDist(g.Player.LocalUp(), planet.BaseUp) < base.Reach+15
	nth := map[string]int{} // followers of each kind so far, to share the circle above the hub
	for i := range g.State.Entities {
		e := &g.State.Entities[i]
		v, ok := g.Content.Vehicles.Get(e.Kind)
		if !ok || !v.Auto() {
			continue
		}
		bob := math.Sin(g.Elapsed*3+float64(i)) * 0.08
		if v.Patrol() {
			g.patrol(v, e, dt)
			e.Alt = v.Hover
			if v.Hover > 0 {
				e.Alt += bob
			}
		} else {
			g.follow(v, e, dt, docked, nth[v.ID], g.Entities(v.ID))
			nth[v.ID]++
			alt := v.Hover
			switch {
			case docked:
				alt = dockAltitude
			case g.TankFull():
				alt += 0.5 // waiting
			}
			if planet.ArcDist(e.Dir, planet.BaseUp) < domesReach {
				alt = math.Max(alt, dockAltitude)
			}
			e.Alt += (alt + bob - e.Alt) * math.Min(1, dt*3)
		}
		if !docked || v.Patrol() {
			if !g.TankFull() {
				g.sweepUnder(v, e, dt)
			}
		}
	}
}

// follow: the vehicle looks for dust around the player, and keeps up with
// them; at the base it circles above the hub with the others of its kind.
func (g *Game) follow(v *content.Vehicle, e *Placed, dt float64, docked bool, i, n int) {
	speed := content.OrZero(v.Speed, 5)
	spread := content.OrZero(v.Range, 25)
	switch {
	case docked:
		a := float64(i)/float64(n)*2*math.Pi + g.Elapsed*0.2
		e.target = base.ToWorld(vec3{math.Cos(a) * dockRadius, 0, math.Sin(a) * dockRadius}).Norm()
	case g.TankFull():
		return
	default:
		e.retarget -= dt
		arrived := planet.ArcDist(e.Dir, e.target) < 0.3
		if e.target == (vec3{}) || e.retarget <= 0 || (arrived && g.World.DustAt(e.target) < 0.05) || planet.ArcDist(e.target, planet.BaseUp) < base.Reach {
			e.target = g.World.FindDusty(g.Rng, g.Player.Pos, spread)
			e.retarget = 8 + g.Rng.Float64()*5
		}
	}
	// Left behind (the player drives off), it hurries back.
	if far := planet.ArcDist(e.Dir, g.Player.LocalUp()); far > spread {
		speed *= far / spread
	}
	g.moveToward(e, e.target, speed, dt)
}

// patrol: the vehicle sweeps the dustiest spots around where it was set
// down, staying off the base's ground.
func (g *Game) patrol(v *content.Vehicle, e *Placed, dt float64) {
	if e.Home == (vec3{}) {
		e.Home = e.Dir
	}
	if g.TankFull() {
		return
	}
	e.retarget -= dt
	arrived := planet.ArcDist(e.Dir, e.target) < 0.5
	if e.target == (vec3{}) || e.retarget <= 0 || (arrived && g.World.DustAt(e.target) < 0.05) {
		e.target = g.World.FindDusty(g.Rng, e.Home.Scale(planet.Radius), content.OrZero(v.Range, 30))
		e.retarget = 10 + g.Rng.Float64()*6
		if planet.ArcDist(e.target, planet.BaseUp) < base.Reach+2 {
			e.target = e.Dir // try again next time
			e.retarget = 0.5
		}
	}
	g.moveToward(e, e.target, content.OrZero(v.Speed, 3), dt)
}

// moveToward rolls or flies an entity along the ground toward the
// direction to, turning its nose that way.
func (g *Game) moveToward(e *Placed, to vec3, speed, dt float64) {
	dist := planet.ArcDist(e.Dir, to)
	if dist < 0.05 {
		return
	}
	fwd := planet.TangentAt(e.Dir, to)
	step := math.Min(dist, speed*dt)
	e.Dir = e.Dir.Scale(planet.Radius).Add(fwd.Scale(step)).Norm()
	want := yawOf(e.Dir, fwd)
	turn := math.Remainder(want-e.Yaw, 2*math.Pi)
	e.Yaw += turn * math.Min(1, dt*4)
}

// LineUp sets one of each vehicle in two rows ahead of the player, facing
// them, for screenshots; the automatic ones stand still.
func (g *Game) LineUp() {
	g.fleetHeld = true
	right := g.Player.Right()
	rows := [2][]*content.Vehicle{}
	for _, v := range g.Content.Vehicles.All() {
		big := content.OrZero(v.Radius, 1.2) > 2
		rows[btoi(big)] = append(rows[btoi(big)], v)
	}
	for r, row := range rows {
		width := 0.0
		for _, v := range row {
			width += 2*content.OrZero(v.Radius, 1) + 1.5
		}
		x := -width / 2
		for _, v := range row {
			w := 2*content.OrZero(v.Radius, 1) + 1.5
			at := g.Player.Pos.Add(g.Player.Heading.Scale(12 + 14*float64(r))).Add(right.Scale(x + w/2))
			x += w
			g.spawn(v.ID, g.World.Relief.Surface(at))
			e := &g.State.Entities[len(g.State.Entities)-1]
			e.Alt, e.Home = v.Hover, e.Dir
			e.Yaw += math.Pi / 3 // three-quarter view
		}
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Board gets in a new vehicle of kind id, for screenshots.
func (g *Game) Board(id string) {
	if g.spawn(id, g.Player.Pos) {
		g.State.Entities[len(g.State.Entities)-1].Yaw = yawOf(g.Player.LocalUp(), g.Player.Heading)
		g.getIn(len(g.State.Entities) - 1)
		g.updateVehicle(0)
	}
}
