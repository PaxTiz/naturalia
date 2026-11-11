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
	speed      float64 // driving: the vehicle's speed, no sprint nor jump
	eyeH       float64 // driving: the seat's height
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

// Update moves the player. With control false (menus) only gravity applies.
func (p *Player) Update(dt float64, t planet.Relief, control bool) {
	p.tangentHeading()
	up := p.LocalUp()
	var move vec3
	if control {
		d := rl.GetMouseDelta()
		turn := -float64(d.X) * mouseSens
		p.Heading = p.Heading.Scale(math.Cos(turn)).Add(up.Cross(p.Heading).Scale(math.Sin(turn)))
		p.Pitch = math.Max(-1.5, math.Min(1.5, p.Pitch-float64(d.Y)*mouseSens))

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
		move = move.Norm().Scale(content.OrZero(p.speed, walkSpeed))
		if rl.IsKeyDown(rl.KeyLeftShift) && p.speed == 0 {
			move = move.Scale(sprintFactor)
		}
		if p.onGround && rl.IsKeyPressed(rl.KeySpace) && p.speed == 0 {
			p.vr = jumpSpeed
			p.onGround = false
		}
	}

	p.Pos = p.Pos.Add(move.Scale(dt))
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

	speed := move.Length()
	target := 0.0
	if p.onGround && speed > 0 {
		target = 1
		p.WalkPhase += dt * speed * 1.6
	}
	p.MoveAmount += (target - p.MoveAmount) * math.Min(1, dt*8)
}

// pushOut keeps the player outside a vertical cylinder around center.
func (p *Player) pushOut(center vec3, radius float64) {
	up := center.Norm()
	d := p.Pos.Sub(center)
	d = d.Sub(up.Scale(d.Dot(up)))
	if l := d.Length(); l < radius {
		if l < 1e-6 {
			d, l = p.Heading.Scale(-1), 1
		}
		p.Pos = p.Pos.Add(d.Scale((radius - l) / l))
	}
}
