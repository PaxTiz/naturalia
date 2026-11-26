package render

import (
	"naturalia/internal/engine/mathx"
	"naturalia/internal/game"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// uploadSteps sends the prints to the terrain shader: positions, then heading
// with the print's strength in w.
func (gfx *Graphics) uploadSteps(f *game.Footprints) {
	count := min(f.N, game.MaxSteps)
	pos := make([]float32, 0, count*4)
	dir := make([]float32, 0, count*4)
	for i := range count {
		s := f.Ring[(f.N-1-i)%game.MaxSteps] // newest first
		strength := 1 - mathx.Smoothstep(game.MaxSteps*0.7, game.MaxSteps, float64(i))
		pos = append(pos, float32(s.Pos.X), float32(s.Pos.Y), float32(s.Pos.Z), 0)
		dir = append(dir, float32(s.Dir.X), float32(s.Dir.Y), float32(s.Dir.Z), float32(strength))
	}
	for _, s := range gfx.grounds() {
		if count > 0 {
			rl.SetShaderValueV(s.sh, s.loc.steps, pos, rl.ShaderUniformVec4, int32(count))
			rl.SetShaderValueV(s.sh, s.loc.stepDirs, dir, rl.ShaderUniformVec4, int32(count))
		}
		rl.SetShaderValue(s.sh, s.loc.stepCount, []float32{float32(count)}, rl.ShaderUniformFloat)
	}
}
