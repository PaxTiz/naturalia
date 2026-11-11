package render

import (
	"mars_sweeper/internal/game"
	"mars_sweeper/internal/mathx"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// drawDevils draws the whirlwinds as see-through swirling tubes, shaped
// and animated by their shaders. Drawn after the opaque scenery.
func (gfx *Graphics) drawDevils(g *game.Game) {
	if len(g.Devils) == 0 {
		return
	}
	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()
	rl.DisableBackfaceCulling()
	rl.BeginBlendMode(rl.BlendAlpha)
	for _, d := range g.Devils {
		up := d.Dir
		t1, t2 := mathx.TangentBasis(up)
		foot := g.World.Relief.Surface(d.Dir).Sub(up.Scale(1))
		rl.SetShaderValue(gfx.devilSh, gfx.devilLoc, []float32{float32(d.Seed), float32(d.Strength()), 0, 0}, rl.ShaderUniformVec4)
		ax, ay, az := t1.Scale(d.Radius), up.Scale(d.Height), t2.Scale(d.Radius)
		rl.DrawMesh(gfx.devilMesh, gfx.devilMat, rl.Matrix{
			M0: float32(ax.X), M1: float32(ax.Y), M2: float32(ax.Z),
			M4: float32(ay.X), M5: float32(ay.Y), M6: float32(ay.Z),
			M8: float32(az.X), M9: float32(az.Y), M10: float32(az.Z),
			M12: float32(foot.X), M13: float32(foot.Y), M14: float32(foot.Z), M15: 1,
		})
	}
	rl.EndBlendMode()
	rl.EnableBackfaceCulling()
	rl.EnableDepthMask()
}
