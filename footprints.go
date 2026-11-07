package main

import rl "github.com/gen2brain/raylib-go/raylib"

const (
	maxSteps   = 48   // footprints kept: a trail of about 35 m
	stepLength = 0.75 // meters between two prints
)

// footprint is a boot print pressed in the dust, drawn by the terrain
// shader. The newest ones are the sharpest; the oldest fade out.
type footprint struct {
	pos, dir vec3
}

// footprints records the player's trail, a print left then right.
type footprints struct {
	ring [maxSteps]footprint
	n    int // prints made so far; the newest is at (n-1) % maxSteps
	last vec3
}

// update reports whether a new step was taken.
func (f *footprints) update(p *Player) bool {
	if !p.onGround {
		return false
	}
	if f.n > 0 && p.Pos.sub(f.last).length() < stepLength {
		return false
	}
	f.last = p.Pos
	side := 0.12
	if f.n%2 == 1 {
		side = -side
	}
	f.ring[f.n%maxSteps] = footprint{p.Pos.add(p.Right().scale(side)), p.Heading}
	f.n++
	return true
}

// upload sends the prints to the terrain shader: positions, then heading
// with the print's strength in w.
func (f *footprints) upload(gfx *Graphics) {
	count := min(f.n, maxSteps)
	pos := make([]float32, 0, count*4)
	dir := make([]float32, 0, count*4)
	for i := range count {
		s := f.ring[(f.n-1-i)%maxSteps] // newest first
		strength := 1 - smoothstep(maxSteps*0.7, maxSteps, float64(i))
		pos = append(pos, float32(s.pos.X), float32(s.pos.Y), float32(s.pos.Z), 0)
		dir = append(dir, float32(s.dir.X), float32(s.dir.Y), float32(s.dir.Z), float32(strength))
	}
	if count > 0 {
		rl.SetShaderValueV(gfx.terrainSh, gfx.tLoc.steps, pos, rl.ShaderUniformVec4, int32(count))
		rl.SetShaderValueV(gfx.terrainSh, gfx.tLoc.stepDirs, dir, rl.ShaderUniformVec4, int32(count))
	}
	rl.SetShaderValue(gfx.terrainSh, gfx.tLoc.stepCount, []float32{float32(count)}, rl.ShaderUniformFloat)
}
