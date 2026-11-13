package game

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/mathx"
	"naturalia/internal/planet"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	eyeHeight    = 1.7
	walkSpeed    = 5.0
	sprintFactor = 1.7
	marsGravity  = 3.71
	jumpSpeed    = 2.9
	mouseSens    = 0.0025
	flySpeed     = 20.0 // creative flight, near the ground; faster higher up
)

// Player walks on the sphere: "up" is always away from Mars' center and
// the heading is a vector tangent to the ground, carried along when walking.
type Player struct {
	Pos        vec3    `json:"pos"` // feet
	Heading    vec3    `json:"heading"`
	Pitch      float64 `json:"pitch"`
	vr         float64 // radial (vertical) speed
	onGround   bool
	WalkPhase  float64
	MoveAmount float64 // 0 when standing, 1 when walking: drives the tool bob
	seated     bool    // driving: the vehicle moves the player, who only looks around
	want       vec3    // walking: the speed asked along the ground...
	vel        vec3    // ...and the one the collisions left
	eyeH       float64 // driving: the seat's height
	// Fly is the debug creative mode: no gravity nor collisions, Space
	// climbs, Ctrl descends, the wheel sets FlyBoost.
	Fly      bool    `json:"-"`
	FlyBoost float64 `json:"-"`
}

// FlySpeed is the creative flight's speed at the player's altitude alt:
// it grows with altitude so a view of the whole region is seconds away.
func (p *Player) FlySpeed(alt float64) float64 {
	return flySpeed * content.OrZero(p.FlyBoost, 1) * math.Max(1, alt/30)
}

// SetFly turns the creative flight on or off; leaving it lands the
// player on the ground below.
func (p *Player) SetFly(on bool, t planet.Relief) {
	p.Fly, p.vr = on, 0
	if !on {
		p.Pos, p.onGround = t.Surface(p.Pos), true
	}
}

func (p *Player) LocalUp() vec3 { return p.Pos.Norm() }
func (p *Player) Eye() vec3     { return p.Pos.Add(p.LocalUp().Scale(content.OrZero(p.eyeH, eyeHeight))) }

func (p *Player) Forward() vec3 {
	return p.Heading.Scale(math.Cos(p.Pitch)).Add(p.LocalUp().Scale(math.Sin(p.Pitch)))
}

func (p *Player) Right() vec3 { return p.Heading.Cross(p.LocalUp()).Norm() }
func (p *Player) Up() vec3    { return p.Right().Cross(p.Forward()) }

// tangentHeading keeps the heading flat on the ground after moving.
func (p *Player) tangentHeading() {
	up := p.LocalUp()
	h := p.Heading.Sub(up.Scale(p.Heading.Dot(up)))
	if h.Length() < 1e-6 {
		h, _ = mathx.TangentBasis(up)
	}
	p.Heading = h.Norm()
}

// Update turns the view and sets where the player walks: the collisions
// (stepPhysics) carry them along the ground, then fall moves them up and
// down. With control false (menus), they stand still.
func (p *Player) Update(dt float64, t planet.Relief, control bool) {
	if p.Fly {
		p.fly(dt, t, control)
		return
	}
	p.tangentHeading()
	up := p.LocalUp()
	var move vec3
	if control {
		d := rl.GetMouseDelta()
		turn := -float64(d.X) * mouseSens
		p.Heading = p.Heading.Scale(math.Cos(turn)).Add(up.Cross(p.Heading).Scale(math.Sin(turn)))
		p.Pitch = math.Max(-1.5, math.Min(1.5, p.Pitch-float64(d.Y)*mouseSens))
	}
	p.want = vec3{}
	if p.seated {
		p.vr, p.onGround, p.MoveAmount, p.vel = 0, true, 0, vec3{}
		p.tangentHeading()
		return
	}
	if control {
		// Physical key positions: WASD on QWERTY is ZQSD on AZERTY.
		right := p.Right()
		if rl.IsKeyDown(rl.KeyW) {
			move = move.Add(p.Heading)
		}
		if rl.IsKeyDown(rl.KeyS) {
			move = move.Sub(p.Heading)
		}
		if rl.IsKeyDown(rl.KeyD) {
			move = move.Add(right)
		}
		if rl.IsKeyDown(rl.KeyA) {
			move = move.Sub(right)
		}
		move = move.Norm().Scale(walkSpeed)
		if rl.IsKeyDown(rl.KeyLeftShift) {
			move = move.Scale(sprintFactor)
		}
		if p.onGround && rl.IsKeyPressed(rl.KeySpace) {
			p.vr = jumpSpeed
			p.onGround = false
		}
	}
	p.want = move
}

// fall moves the walking player up and down: jumps, gravity, the ground.
func (p *Player) fall(dt float64, t planet.Relief) {
	if p.Fly || p.seated {
		return
	}
	p.vr -= marsGravity * dt
	p.Pos = p.Pos.Add(p.LocalUp().Scale(p.vr * dt))

	alt := t.Altitude(p.Pos)
	// Stick to the ground on gentle downhill steps instead of floating off.
	if p.onGround && p.vr <= 0 && alt < 0.4 {
		p.Pos, alt = t.Surface(p.Pos), 0
	}
	if alt <= 0 {
		p.Pos, p.vr, p.onGround = t.Surface(p.Pos), 0, true
	} else if alt > 0.4 {
		p.onGround = false
	}
	p.tangentHeading()

	speed := p.want.Length()
	target := 0.0
	if p.onGround && speed > 0 {
		target = 1
		p.WalkPhase += dt * speed * 1.6
	}
	p.MoveAmount += (target - p.MoveAmount) * math.Min(1, dt*8)
}

// fly moves the player in creative flight: like walking, plus Space and
// Ctrl for up and down, Shift for more speed, never below the ground.
func (p *Player) fly(dt float64, t planet.Relief, control bool) {
	p.tangentHeading()
	p.onGround, p.MoveAmount = false, 0
	if !control {
		return
	}
	up := p.LocalUp()
	d := rl.GetMouseDelta()
	turn := -float64(d.X) * mouseSens
	p.Heading = p.Heading.Scale(math.Cos(turn)).Add(up.Cross(p.Heading).Scale(math.Sin(turn)))
	p.Pitch = math.Max(-1.5, math.Min(1.5, p.Pitch-float64(d.Y)*mouseSens))
	if w := rl.GetMouseWheelMove(); w != 0 {
		p.FlyBoost = math.Max(0.1, math.Min(50, content.OrZero(p.FlyBoost, 1)*math.Pow(1.25, float64(w))))
	}

	var move vec3
	right := p.Right()
	for _, k := range []struct {
		key int32
		dir vec3
	}{{rl.KeyW, p.Heading}, {rl.KeyS, p.Heading.Scale(-1)}, {rl.KeyD, right}, {rl.KeyA, right.Scale(-1)},
		{rl.KeySpace, up}, {rl.KeyLeftControl, up.Scale(-1)}} {
		if rl.IsKeyDown(k.key) {
			move = move.Add(k.dir)
		}
	}
	speed := p.FlySpeed(t.Altitude(p.Pos))
	if rl.IsKeyDown(rl.KeyLeftShift) {
		speed *= 4
	}
	p.Pos = p.Pos.Add(move.Norm().Scale(speed * dt))
	if t.Altitude(p.Pos) < 0 {
		p.Pos = t.Surface(p.Pos)
	}
	p.tangentHeading()
}
