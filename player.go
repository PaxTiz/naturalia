package main

import (
	"math"

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
	walkPhase  float64
	moveAmount float64 // 0 when standing, 1 when walking: drives the tool bob
	speed      float64 // driving: the vehicle's speed, no sprint nor jump
	eyeH       float64 // driving: the seat's height
}

func (p *Player) LocalUp() vec3 { return p.Pos.norm() }
func (p *Player) Eye() vec3     { return p.Pos.add(p.LocalUp().scale(orZero(p.eyeH, eyeHeight))) }

func (p *Player) Forward() vec3 {
	return p.Heading.scale(math.Cos(p.Pitch)).add(p.LocalUp().scale(math.Sin(p.Pitch)))
}

func (p *Player) Right() vec3 { return p.Heading.cross(p.LocalUp()).norm() }
func (p *Player) Up() vec3    { return p.Right().cross(p.Forward()) }

// tangentHeading keeps the heading flat on the ground after moving.
func (p *Player) tangentHeading() {
	up := p.LocalUp()
	h := p.Heading.sub(up.scale(p.Heading.dot(up)))
	if h.length() < 1e-6 {
		h, _ = tangentBasis(up)
	}
	p.Heading = h.norm()
}

// Update moves the player. With control false (menus) only gravity applies.
func (p *Player) Update(dt float64, t *Terrain, control bool) {
	p.tangentHeading()
	up := p.LocalUp()
	var move vec3
	if control {
		d := rl.GetMouseDelta()
		turn := -float64(d.X) * mouseSens
		p.Heading = p.Heading.scale(math.Cos(turn)).add(up.cross(p.Heading).scale(math.Sin(turn)))
		p.Pitch = math.Max(-1.5, math.Min(1.5, p.Pitch-float64(d.Y)*mouseSens))

		// Physical key positions: WASD on QWERTY is ZQSD on AZERTY.
		right := p.Right()
		if rl.IsKeyDown(rl.KeyW) {
			move = move.add(p.Heading)
		}
		if rl.IsKeyDown(rl.KeyS) {
			move = move.sub(p.Heading)
		}
		if rl.IsKeyDown(rl.KeyD) {
			move = move.add(right)
		}
		if rl.IsKeyDown(rl.KeyA) {
			move = move.sub(right)
		}
		move = move.norm().scale(orZero(p.speed, walkSpeed))
		if rl.IsKeyDown(rl.KeyLeftShift) && p.speed == 0 {
			move = move.scale(sprintFactor)
		}
		if p.onGround && rl.IsKeyPressed(rl.KeySpace) && p.speed == 0 {
			p.vr = jumpSpeed
			p.onGround = false
		}
	}

	p.Pos = p.Pos.add(move.scale(dt))
	p.vr -= marsGravity * dt
	p.Pos = p.Pos.add(p.LocalUp().scale(p.vr * dt))

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

	speed := move.length()
	target := 0.0
	if p.onGround && speed > 0 {
		target = 1
		p.walkPhase += dt * speed * 1.6
	}
	p.moveAmount += (target - p.moveAmount) * math.Min(1, dt*8)
}

// pushOut keeps the player outside a vertical cylinder around center.
func (p *Player) pushOut(center vec3, radius float64) {
	up := center.norm()
	d := p.Pos.sub(center)
	d = d.sub(up.scale(d.dot(up)))
	if l := d.length(); l < radius {
		if l < 1e-6 {
			d, l = p.Heading.scale(-1), 1
		}
		p.Pos = p.Pos.add(d.scale((radius - l) / l))
	}
}
