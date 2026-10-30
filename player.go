package main

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	eyeHeight    = 1.7
	walkSpeed    = 5.0
	sprintFactor = 1.7
	moonGravity  = 1.62
	jumpSpeed    = 3.2
	mouseSens    = 0.0025
)

type Player struct {
	Pos        vec3    `json:"pos"` // feet
	Yaw        float64 `json:"yaw"`
	Pitch      float64 `json:"pitch"`
	vy         float64
	onGround   bool
	walkPhase  float64
	moveAmount float64 // 0 when standing, 1 when walking: drives the tool bob
}

func (p *Player) Eye() vec3 { return p.Pos.add(vec3{0, eyeHeight, 0}) }

// pushOut keeps the player outside a vertical cylinder around center.
func (p *Player) pushOut(center vec3, radius float64) {
	d := vec3{p.Pos.X - center.X, 0, p.Pos.Z - center.Z}
	if l := d.length(); l < radius {
		if l < 1e-6 {
			d, l = vec3{0, 0, -1}, 1
		}
		p.Pos.X = center.X + d.X/l*radius
		p.Pos.Z = center.Z + d.Z/l*radius
	}
}

func (p *Player) Forward() vec3 {
	cp := math.Cos(p.Pitch)
	return vec3{cp * math.Sin(p.Yaw), math.Sin(p.Pitch), cp * math.Cos(p.Yaw)}
}

func (p *Player) Right() vec3 { f := p.Forward(); return vec3{-f.Z, 0, f.X}.norm() }
func (p *Player) Up() vec3    { return p.Right().cross(p.Forward()) }

// Update moves the player. With control false (menus) only gravity applies.
func (p *Player) Update(dt float64, t *Terrain, control bool) {
	var move vec3
	if control {
		d := rl.GetMouseDelta()
		p.Yaw -= float64(d.X) * mouseSens
		p.Pitch = math.Max(-1.5, math.Min(1.5, p.Pitch-float64(d.Y)*mouseSens))

		// Physical key positions: WASD on QWERTY is ZQSD on AZERTY.
		fwd := vec3{math.Sin(p.Yaw), 0, math.Cos(p.Yaw)}
		right := vec3{-fwd.Z, 0, fwd.X}
		if rl.IsKeyDown(rl.KeyW) {
			move = move.add(fwd)
		}
		if rl.IsKeyDown(rl.KeyS) {
			move = move.sub(fwd)
		}
		if rl.IsKeyDown(rl.KeyD) {
			move = move.add(right)
		}
		if rl.IsKeyDown(rl.KeyA) {
			move = move.sub(right)
		}
		move = move.norm().scale(walkSpeed)
		if rl.IsKeyDown(rl.KeyLeftShift) {
			move = move.scale(sprintFactor)
		}
		if p.onGround && rl.IsKeyPressed(rl.KeySpace) {
			p.vy = jumpSpeed
			p.onGround = false
		}
	}

	p.Pos.X = math.Max(1, math.Min(worldSize-1, p.Pos.X+move.X*dt))
	p.Pos.Z = math.Max(1, math.Min(worldSize-1, p.Pos.Z+move.Z*dt))
	p.vy -= moonGravity * dt
	p.Pos.Y += p.vy * dt

	ground := t.Height(p.Pos.X, p.Pos.Z)
	// Stick to the ground on gentle downhill steps instead of floating off.
	if p.onGround && p.vy <= 0 && p.Pos.Y-ground < 0.4 {
		p.Pos.Y = ground
	}
	if p.Pos.Y <= ground {
		p.Pos.Y, p.vy, p.onGround = ground, 0, true
	} else if p.Pos.Y-ground > 0.4 {
		p.onGround = false
	}

	speed := move.length()
	target := 0.0
	if p.onGround && speed > 0 {
		target = 1
		p.walkPhase += dt * speed * 1.6
	}
	p.moveAmount += (target - p.moveAmount) * math.Min(1, dt*8)
}
