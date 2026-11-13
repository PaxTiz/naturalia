package game

import (
	"image/color"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/mathx"
	"naturalia/internal/palette"
	"naturalia/internal/planet"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Driving: the player sits in a vehicle (content.Vehicle) that rolls,
// hovers or flies. Rolling ones fly off crests and ramps under Mars'
// weak gravity, jump with Space and boost with Shift; flying ones are
// driven the same way, and Space and Ctrl set their height.

// drive is the driven vehicle's motion. The player's position stays on
// the ground under it; r says how high the vehicle is.
type drive struct {
	fwd  vec3 // where its nose points
	vel  vec3 // along the ground
	want vec3 // where it went before walls and rocks pushed it back
	// boarded is set on the frame the player gets in, whose E press must
	// not get them out again.
	boarded bool

	r        float64 // its bottom, from Mars' center (0: not set yet)
	vr       float64 // its vertical speed
	air      bool    // off the ground, more than a bump
	airtime  float64 // seconds since it last touched the ground
	jumps    int     // jumps made since then
	fuel     float64 // seconds of boost left
	boosting bool
	pitch    float64  // nose up, radians: the slope, or the jump's arc
	roll     float64  // right side down
	shake    float64  // camera shake, 0 to 1, fading
	kicked   ecs.ID   // the boost pad being crossed (0: none)
	flip     float64  // a second jump's somersault so far, radians (0: none)
	cruise   float64  // flying: the height over the ground set with Space and Ctrl
	climbing bool     // Space or Ctrl held last frame
	dock     *docking // gliding into its bay (G), the controls left aside
}

// controls is what the player asks of the vehicle this frame.
type controls struct {
	throttle float64 // forward 1, back -1
	steer    float64 // left 1, right -1
	climb    float64 // flying: up 1, down -1
	jump     bool    // pressed this frame
	boost    bool    // held
}

const (
	coyoteTime = 0.2 // a jump still works this long after rolling off an edge
	airAbove   = 0.3 // m over the ground: in the air, no longer a bump
	padReach   = 1.8 // m: a boost pad kicks what passes this close
	aloft      = 3.0 // m over the ground: what flies that high goes over walls and rocks
)

// Driven is the vehicle the player drives, if any.
func (g *Game) Driven() (*content.Vehicle, bool) {
	return g.vehicleOf(g.Riding)
}

// DriveSpeed is how fast the driven vehicle goes, in m/s.
func (g *Game) DriveSpeed() float64 {
	if g.Riding == 0 {
		return 0
	}
	return math.Hypot(g.drive.vel.Length(), g.drive.vr)
}

// DriveAlt is how high the driven vehicle is over the ground, in m.
func (g *Game) DriveAlt() float64 {
	if g.Riding == 0 || g.drive.r == 0 {
		return 0
	}
	return math.Max(0, g.drive.r-g.World.Relief.Surface(g.Player.Pos).Length())
}

// Boosting tells if the driven vehicle's boost is on, and how much of it
// is left (0 to 1).
func (g *Game) Boosting() (on bool, left float64) {
	v, ok := g.Driven()
	if !ok || v.Boost <= 0 {
		return false, 0
	}
	return g.drive.boosting, g.drive.fuel / content.OrZero(v.Fuel, 3)
}

// Shake is how much the camera shakes, 0 to 1.
func (g *Game) Shake() float64 { return g.drive.shake }

// Chase is on when the camera follows the driven vehicle from behind.
func (g *Game) Chase() bool { return g.chase && g.Riding != 0 }

// getIn or out of the vehicle standing as entity id.
func (g *Game) getIn(id ecs.ID) {
	e := g.Ents.Transform.Get(id)
	g.Riding = id
	t1, t2 := mathx.TangentBasis(e.Dir)
	g.drive = drive{fwd: t1.Scale(math.Cos(e.Yaw)).Add(t2.Scale(math.Sin(e.Yaw))), boarded: true}
	g.Player.Pos = g.World.Relief.Surface(e.Dir)
	g.Player.Heading, g.Player.seated = g.drive.fwd, true
	if v, ok := g.vehicleOf(id); ok {
		g.drive.fuel = content.OrZero(v.Fuel, 3)
		g.chase = v.View == "chase"
		// What flies stands where it landed.
		if v.Fly != nil || v.Hover > 0 {
			g.drive.r = g.Player.Pos.Length() + math.Max(0, e.Alt-v.Hover)
			g.drive.cruise = math.Max(0, e.Alt-v.Hover)
		}
	}
	g.sfx.Beep(0.02, 0.2, 0.12, 220, 330)
	g.emit(content.Event{Name: content.EvDrive, ID: g.kind(id)})
}

func (g *Game) getOut() {
	v, ok := g.Driven()
	id := g.Riding
	if ok {
		// Getting out up high leaves it standing on the ground below.
		e := g.Ents.Transform.Get(id)
		e.Alt, e.Pitch, e.Roll = v.Hover, 0, 0
	}
	g.Riding = 0
	g.Player.seated, g.Player.eyeH = false, 0
	g.drive = drive{}
	if !ok {
		return
	}
	// In a garage, it goes to its bay; the player steps out into the
	// aisle in front of it.
	if r := base.RoomAt(base.ToLocal(g.Player.Pos)); r >= 0 && base.Rooms[r].Hall != nil && g.park(v, id) {
		bay, _ := base.BayOf(v.ID)
		g.Player.Pos = g.World.Relief.Surface(base.ToWorld(bay.At.Add(vec3{bay.Length/2 + 1.5, 0, 0})))
		g.Say(v.Name + " rangé au garage")
	} else {
		side := g.Player.Right().Scale(content.OrZero(v.Radius, 1.2) + 0.6)
		g.Player.Pos = g.World.Relief.Surface(g.Player.Pos.Add(side))
	}
	g.sfx.Beep(0.02, 0.2, 0.12, 330, 220)
	g.emit(content.Event{Name: content.EvDrive, ID: ""})
}

// driveVehicle moves the driven vehicle, with the player in it, before the
// collisions: Z/S throttle and brake, Q/D steer, Space jumps (or climbs),
// Ctrl descends, Shift boosts, V switches the camera.
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
		if g.DriveAlt() > aloft && v.Fly != nil {
			g.Say("Descends d'abord (Ctrl)")
		} else {
			g.getOut()
			return
		}
	}
	d.boarded = false
	if d.dock != nil {
		g.glideDock(v, dt)
		return
	}
	if g.Mode == ModePlay && rl.IsKeyPressed(rl.KeyG) && g.CanDock() {
		g.startDock()
		return
	}
	var in controls
	if g.Mode == ModePlay {
		// Physical key positions: WASD on QWERTY is ZQSD on AZERTY.
		for _, k := range []struct {
			key int32
			to  *float64
			by  float64
		}{{rl.KeyW, &in.throttle, 1}, {rl.KeyS, &in.throttle, -1}, {rl.KeyA, &in.steer, 1}, {rl.KeyD, &in.steer, -1},
			{rl.KeySpace, &in.climb, 1}, {rl.KeyLeftControl, &in.climb, -1}} {
			if rl.IsKeyDown(k.key) {
				*k.to += k.by
			}
		}
		in.jump = rl.IsKeyPressed(rl.KeySpace)
		in.boost = rl.IsKeyDown(rl.KeyLeftShift)
		if rl.IsKeyPressed(rl.KeyV) {
			g.chase = !g.chase
		}
	}
	g.move(v, in, dt)
}

// groundAt is the height a driven vehicle rolls on at p, from Mars'
// center: the ground, or a ramp standing there.
func (g *Game) groundAt(p vec3) float64 {
	r := g.World.Relief.Surface(p).Length()
	up := p.Norm()
	for id, e := range g.Ents.Transform.All() {
		def, ok := g.Content.Entities.Get(g.kind(id))
		if !ok || def.Ramp == nil || planet.ArcDist(up, e.Dir) > def.Ramp.Length {
			continue
		}
		foot := g.World.Relief.Surface(e.Dir)
		ax, _, az := entityAxes(e.Dir, e.Yaw)
		off := p.Sub(foot)
		x, z := off.Dot(ax), off.Dot(az)
		rp := def.Ramp
		if math.Abs(x) < rp.Length/2 && math.Abs(z) < rp.Width/2 {
			r = math.Max(r, foot.Length()+rp.Height*(x/rp.Length+0.5))
		}
	}
	return r
}

// entityAxes are an entity's front, up and side, standing at dir turned
// by yaw.
func entityAxes(dir vec3, yaw float64) (ax, up, az vec3) {
	t1, t2 := mathx.TangentBasis(dir)
	ax = t1.Scale(math.Cos(yaw)).Add(t2.Scale(math.Sin(yaw)))
	return ax, dir, ax.Cross(dir)
}

// Frame is where an entity stands and its axes (front, up, side), tilted
// by its pitch and roll.
func (g *Game) Frame(e *Transform) (pos, ax, up, az vec3) {
	ax, up, az = entityAxes(e.Dir, e.Yaw)
	pos = g.World.Relief.Surface(e.Dir).Add(up.Scale(e.Alt))
	if e.Pitch != 0 {
		c, s := math.Cos(e.Pitch), math.Sin(e.Pitch)
		ax, up = ax.Scale(c).Add(up.Scale(s)), up.Scale(c).Sub(ax.Scale(s))
	}
	if e.Roll != 0 {
		c, s := math.Cos(e.Roll), math.Sin(e.Roll)
		up, az = up.Scale(c).Add(az.Scale(s)), az.Scale(c).Sub(up.Scale(s))
	}
	return pos, ax, up, az
}

// move drives the vehicle for dt.
func (g *Game) move(v *content.Vehicle, in controls, dt float64) {
	if dt <= 0 { // the first frame: speeds are divided by dt
		return
	}
	d := &g.drive
	up := g.Player.LocalUp()
	d.fwd = planet.TangentAt(up, d.fwd)
	d.vel = d.vel.Sub(up.Scale(d.vel.Dot(up)))
	if d.r == 0 {
		d.r = g.groundAt(g.Player.Pos)
	}
	gravity := marsGravity * content.OrZero(v.Gravity, 2)

	top := v.Speed
	accel, brake, coast := top*0.5, top*1.5, math.Max(top, 5)*0.4
	if v.Fly != nil {
		// Gently at first, then faster and faster: in hand near the
		// base, a few seconds to its top speed.
		s := math.Abs(d.vel.Dot(d.fwd))
		gain := content.OrZero(v.Fly.Gain, 0.7)
		accel, brake, coast = math.Max(8, s*gain), math.Max(15, s*1.2), math.Max(4, s*0.3)
	}
	if v.Boost > 0 {
		full := content.OrZero(v.Fuel, 3)
		was := d.boosting
		d.boosting = in.boost && d.fuel > 0 && in.throttle >= 0
		switch {
		case d.boosting:
			d.fuel = math.Max(0, d.fuel-dt)
			top *= v.Boost
			if v.Fly == nil {
				accel = top // there in a second
			}
			in.throttle = 1
			d.shake = math.Max(d.shake, 0.15)
			if !was {
				g.sfx.Beep(0.03, 0.25, 0.14, 180, 270, 360)
				g.emit(content.Event{Name: content.EvBoost, ID: v.ID})
			}
		case !in.boost:
			d.fuel = math.Min(full, d.fuel+dt/2)
		}
	}

	s := d.vel.Dot(d.fwd)
	side := d.vel.Sub(d.fwd.Scale(s))
	if !d.air || v.Fly != nil {
		switch {
		// A crawl the other way, like rolling back down a slight slope,
		// isn't braked: that would stop it there for good.
		case in.throttle > 0 && s < -0.2, in.throttle < 0 && s > 0.2:
			s = approach(s, 0, brake*dt)
		case s > top && in.throttle >= 0:
			s = approach(s, top, coast*dt) // coming down from a boost or a pad
		case in.throttle > 0:
			s = approach(s, top, accel*dt)
		case in.throttle < 0:
			back := top * 0.35
			if v.Fly != nil {
				back = 15 // what flies backs up to turn around, no more
			}
			s = approach(s, -back, accel*dt)
		default:
			s = approach(s, 0, coast*dt)
		}
		// Hills: down is faster, up slower.
		if v.Fly == nil {
			s -= gravity * math.Sin(d.pitch) * dt
		}
		side = side.Scale(math.Exp(-content.OrZero(v.Grip, 8) * dt))
	}

	// Wheels only turn when rolling, and backward when backing up; a
	// hovering vehicle spins in place too. In the air, only a little.
	grip := math.Min(1, math.Abs(s)/(math.Max(top, 1)*0.3))
	if v.Hover > 0 || v.Fly != nil {
		grip = math.Max(grip, 0.6)
	}
	if d.air && v.Fly == nil {
		grip = content.OrZero(v.Air, 0.35)
	}
	if s < -0.1 {
		grip = -grip
	}
	// The nose turns, the motion lags: grip pulls it along next frames.
	d.vel = d.fwd.Scale(s).Add(side)
	if a := in.steer * content.OrZero(v.Turn, 1.2) * grip * dt; a != 0 {
		d.fwd = d.fwd.Scale(math.Cos(a)).Add(up.Cross(d.fwd).Scale(math.Sin(a)))
		g.Player.Heading = g.Player.Heading.Scale(math.Cos(a)).Add(up.Cross(g.Player.Heading).Scale(math.Sin(a)))
	}
	d.want = g.World.Relief.Surface(g.Player.Pos.Add(d.vel.Scale(dt)))
	g.Player.Pos = d.want

	if v.Fly != nil {
		g.climb(v, in, dt)
	} else {
		g.fall(v, in, gravity, dt)
	}
}

// fall moves a rolling vehicle up and down: it follows the ground, which
// throws it up on crests and ramps, then gravity brings it back.
func (g *Game) fall(v *content.Vehicle, in controls, gravity, dt float64) {
	d := &g.drive
	if in.jump && v.Jump > 0 && d.jumps < max(1, v.Jumps) {
		d.vr = math.Max(d.vr, 0) + v.Jump
		d.jumps++
		d.air = true
		if d.jumps > 1 {
			d.flip = 1e-6
		}
		g.sfx.Beep(0.02, 0.15, 0.12, 300, 450)
		g.emit(content.Event{Name: content.EvJump, ID: v.ID, Amount: v.Jump})
	}
	ground := g.groundAt(g.Player.Pos)
	d.vr -= gravity * dt
	r := d.r + d.vr*dt
	// Going down a slope it stays on the ground; only what goes up a
	// crest or a ramp flies off it.
	if !d.air && d.vr <= 0 && r-ground < 0.15 {
		r = math.Min(r, ground)
	}
	if r <= ground {
		bounce := 0.0
		if d.air || d.airtime > 0 {
			bounce = g.land(v, -d.vr)
		}
		d.airtime, d.jumps, d.air, d.flip = 0, 0, false, 0
		// The ground's rise is the speed it throws the vehicle up with;
		// a step much steeper than the motion stops it instead.
		d.vr = math.Max(bounce, math.Min((ground-d.r)/dt, math.Max(4, d.vel.Length()*1.2)))
		r = ground
	} else {
		d.airtime += dt
		if r-ground > airAbove {
			d.air = true
		}
		if d.airtime > coyoteTime && d.jumps == 0 {
			d.jumps = 1
		}
	}
	d.r = r
}

// land ends a flight: a bump, and the "land" event after a real jump. A
// hard landing bounces back up at the speed it returns.
func (g *Game) land(v *content.Vehicle, impact float64) (bounce float64) {
	d := &g.drive
	if impact > 4 {
		d.shake = math.Min(1, impact/12)
		g.sfx.Beep(0, 0.18, math.Min(0.3, impact*0.02), 70, 55)
		g.dustBurst(g.Player.Pos, int(math.Min(40, impact*3)))
		if impact > 7 {
			bounce = impact * content.OrZero(v.Bounce, 0.25)
		}
	}
	if d.airtime > 0.4 {
		g.emit(content.Event{Name: content.EvLand, ID: v.ID, Amount: d.airtime, At: g.Player.Pos})
	}
	return bounce
}

// dustBurst throws n puffs of dust up from the ground at p.
func (g *Game) dustBurst(p vec3, n int) {
	up := p.Norm()
	for range n {
		out := vec3{g.Rng.Float64() - 0.5, g.Rng.Float64() - 0.5, g.Rng.Float64() - 0.5}
		out = out.Sub(up.Scale(out.Dot(up))).Norm().Scale(2 + g.Rng.Float64()*4)
		g.addParticle(Particle{Pos: p.Add(up.Scale(0.2)), Vel: out.Add(up.Scale(1 + g.Rng.Float64()*2)),
			MaxLife: 0.8 + g.Rng.Float64()*0.6, Size: 0.12 + g.Rng.Float64()*0.15, Falls: true, C: color.RGBA{200, 135, 95, 255}})
	}
}

// climb moves a flying vehicle up and down: Space raises it, Ctrl lowers
// it, and it holds its height over the ground otherwise, whatever its
// speed. At 0 it hovers like the others.
func (g *Game) climb(v *content.Vehicle, in controls, dt float64) {
	d := &g.drive
	f := v.Fly
	rate := content.OrZero(f.Climb, 40)
	ground := g.groundAt(g.Player.Pos)
	switch {
	case in.climb != 0:
		// Slow near the ground, to land; faster high up.
		step := math.Min(rate, math.Max(6, d.cruise*0.8)) * dt
		d.cruise = math.Max(0, math.Min(content.OrZero(f.Ceiling, 500), d.cruise+in.climb*step))
	case d.climbing:
		d.cruise = math.Max(0, d.r-ground) // it stops where the key was let go
	}
	d.climbing = in.climb != 0
	// It follows the ground's rises and falls quicker than it climbs.
	lim := rate + d.vel.Length()*0.5
	d.vr = math.Max(-lim, math.Min(lim, (ground+d.cruise-d.r)*4))
	r := d.r + d.vr*dt
	if r <= ground || d.cruise == 0 {
		r, d.vr = ground, math.Max(0, d.vr)
	}
	if r-ground > airAbove {
		d.airtime += dt
		d.air = true
	} else if d.air {
		g.land(v, 0)
		d.air, d.airtime = false, 0
	}
	d.r = r
}

// approach moves v toward target by at most step.
func approach(v, target, step float64) float64 {
	if v < target {
		return math.Min(target, v+step)
	}
	return math.Max(target, v-step)
}

// Push adds speed to the driven vehicle: forward along its nose, and up.
func (g *Game) Push(forward, up float64) {
	if g.Riding == 0 || math.IsNaN(forward+up) || math.IsInf(forward+up, 0) {
		return
	}
	d := &g.drive
	d.vel = d.vel.Add(d.fwd.Scale(forward))
	d.vr += up
	if up > 0 {
		d.air = true
	}
}

// Vehicle is the driven vehicle, for packs.
func (g *Game) Vehicle() (content.VehicleState, bool) {
	v, ok := g.Driven()
	if !ok {
		return content.VehicleState{}, false
	}
	on, left := g.Boosting()
	return content.VehicleState{ID: v.ID, Speed: g.DriveSpeed(), Air: g.drive.air, Alt: g.DriveAlt(),
		Airtime: g.drive.airtime, Fuel: left, Boosting: on}, true
}

// updateVehicle sets the driven vehicle where the collisions left the
// player, tilts it, and sweeps under it.
func (g *Game) updateVehicle(dt float64) {
	v, ok := g.Driven()
	if !ok {
		return
	}
	d := &g.drive
	d.shake = math.Max(0, d.shake-dt*2.5)
	// Walls and rocks stop the motion that goes into them.
	if push := g.Player.Pos.Sub(d.want); push.Length() > 1e-4 {
		n := push.Norm()
		if into := d.vel.Dot(n); into < 0 {
			d.vel = d.vel.Sub(n.Scale(into))
		}
	}
	e := g.Ents.Transform.Get(g.Riding)
	e.Dir = g.Player.Pos.Norm()
	e.Yaw = yawOf(e.Dir, d.fwd)
	ground := g.World.Relief.Surface(g.Player.Pos).Length()
	if d.r == 0 {
		d.r = ground
	}
	e.Alt = v.Hover + math.Max(0, d.r-ground)
	if v.Hover > 0 {
		e.Alt += math.Sin(g.Elapsed*2.2) * 0.04
	}
	g.Player.eyeH = e.Alt + content.OrZero(v.Seat, 2.4)
	g.tilt(v, e, dt)
	g.kick(v)
	g.exhaust(v, e)
	if g.Mode == ModePlay && d.r-ground < 1 {
		g.sweepUnder(v, e, dt)
	}
}

// tilt leans the vehicle: along the slope on the ground, along its arc in
// the air, into turns when it hovers; what flies leans forward with speed.
func (g *Game) tilt(v *content.Vehicle, e *Transform, dt float64) {
	d := &g.drive
	s := d.vel.Dot(d.fwd)
	turn := d.vel.Dot(g.Player.LocalUp().Cross(d.fwd)) // sideways speed, to the left
	var pitch, roll float64
	switch {
	case v.Fly != nil:
		// Nose up as it climbs, banking into turns.
		pitch = math.Atan2(d.vr, math.Max(math.Abs(s), 10)) * 0.6
		roll = math.Max(-0.5, math.Min(0.5, turn*0.02))
	case d.air:
		pitch = math.Atan2(d.vr, math.Max(math.Abs(s), 4)) * 0.7
	default:
		l := content.OrZero(v.Radius, 1.2) * 0.8
		right := d.fwd.Cross(g.Player.LocalUp())
		at := func(off vec3) float64 { return g.groundAt(g.Player.Pos.Add(off)) }
		pitch = math.Atan2(at(d.fwd.Scale(l))-at(d.fwd.Scale(-l)), 2*l)
		roll = math.Atan2(at(right.Scale(-l*0.7))-at(right.Scale(l*0.7)), 1.4*l)
		if v.Hover > 0 {
			roll += math.Max(-0.25, math.Min(0.25, turn*0.04))
		}
	}
	k := math.Min(1, dt*8)
	if d.air && v.Fly == nil {
		k = math.Min(1, dt*3)
	}
	d.pitch += (pitch - d.pitch) * k
	d.roll += (roll - d.roll) * k
	e.Pitch, e.Roll = d.pitch, d.roll
	// A second jump turns it head over heels, once.
	if d.flip > 0 {
		if d.flip += dt * 7; d.flip >= 2*math.Pi {
			d.flip = 0
		}
		e.Pitch -= d.flip
	}
}

// kick: a boost pad throws the vehicle crossing it forward, once, and
// fills its boost.
func (g *Game) kick(v *content.Vehicle) {
	d := &g.drive
	up := g.Player.LocalUp()
	for id, e := range g.Ents.Transform.All() {
		def, ok := g.Content.Entities.Get(g.kind(id))
		if !ok || def.Kick <= 0 || id == g.Riding {
			continue
		}
		near := planet.ArcDist(up, e.Dir) < padReach && d.r-g.World.Relief.Surface(e.Dir).Length() < 1.5
		switch {
		case near && d.kicked != id && d.vel.Dot(d.fwd) > 0:
			d.kicked = id
			d.vel = d.vel.Add(d.fwd.Scale(def.Kick))
			d.fuel = content.OrZero(v.Fuel, 3)
			d.shake = math.Max(d.shake, 0.4)
			g.sfx.Beep(0.03, 0.2, 0.14, 440, 660, 880)
		case !near && d.kicked == id:
			d.kicked = 0
		}
	}
}

// exhaust: flames behind a boosting vehicle, under a flying one.
func (g *Game) exhaust(v *content.Vehicle, e *Transform) {
	d := &g.drive
	if v.Flame == (vec3{}) || g.Mode != ModePlay {
		return
	}
	pos, ax, up, az := g.Frame(e)
	out := ax.Scale(-1)
	burn := 0.0
	switch {
	case v.Fly != nil:
		if s := d.vel.Length(); s > 1 {
			burn = 0.3 + 0.7*math.Min(1, s/math.Max(1, v.Speed)*3)
		}
	case d.boosting:
		burn = 1
	}
	if burn <= 0 {
		return
	}
	at := pos.Add(ax.Scale(v.Flame.X)).Add(up.Scale(v.Flame.Y)).Add(az.Scale(v.Flame.Z))
	r := content.OrZero(v.FlameR, 0.3)
	cols := []color.RGBA{palette.AccentOrange, {255, 210, 120, 255}, {255, 120, 60, 255}}
	for range int(1 + burn*2) {
		jit := vec3{g.Rng.Float64() - 0.5, g.Rng.Float64() - 0.5, g.Rng.Float64() - 0.5}.Scale(r)
		life := 0.08 + g.Rng.Float64()*0.2*burn
		g.addParticle(Particle{Pos: at.Add(jit), Vel: d.vel.Add(out.Scale(6 + 14*burn*g.Rng.Float64())),
			MaxLife: life, Size: r * (0.3 + g.Rng.Float64()*0.4), C: cols[g.Rng.IntN(len(cols))], Additive: true})
	}
}

// ChaseCam is the chase camera's eye and the point it looks at, when it
// is on: behind the vehicle along the player's view, which the mouse
// turns around it.
func (g *Game) ChaseCam() (eye, at vec3, ok bool) {
	v, ok := g.Driven()
	if !ok || !g.chase {
		return vec3{}, vec3{}, false
	}
	e := g.Ents.Transform.Get(g.Riding)
	pos, _, _, _ := g.Frame(e)
	at = pos.Add(e.Dir.Scale(content.OrZero(v.Seat, 2.4) * 0.6))
	dist := content.OrZero(v.Chase, content.OrZero(v.Radius, 1.2)*3+4)
	eye = at.Sub(g.Player.Forward().Scale(dist)).Add(e.Dir.Scale(dist * 0.2))
	if a := g.World.Relief.Altitude(eye); a < 0.8 {
		eye = eye.Add(eye.Norm().Scale(0.8 - a))
	}
	return eye, at, true
}

// Lift sets the driven vehicle alt m up, going speed m/s, with the chase
// camera on or off: for screenshots.
func (g *Game) Lift(alt, speed float64, chase bool) {
	d := &g.drive
	d.r = g.groundAt(g.Player.Pos) + alt
	d.vel = d.fwd.Scale(speed)
	d.air = alt > airAbove
	g.chase = chase
}
