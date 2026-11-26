package render

import (
	"naturalia/internal/engine/gpu"
	"naturalia/internal/game"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// look reads the photo mode's settings into the picture's; out of it, the
// game's usual look.
func (gfx *Graphics) look(g *game.Game) gpu.Look {
	lk := gpu.Look{Vignette: 0.45, Grain: 0.2}
	if g.Mode != game.ModePhoto {
		return lk
	}
	s := &g.Photo.Set
	lk.Exposure = s[game.PhotoExposure]
	lk.Focus = game.FocusDistances[int(s[game.PhotoFocus])]
	// The largest blur grows with the window, to look the same at any size.
	lk.Blur = s[game.PhotoAperture] * 0.03 * float64(rl.GetRenderHeight())
	lk.Filter = int(s[game.PhotoFilter])
	lk.Contrast, lk.Sat = s[game.PhotoContrast], s[game.PhotoSaturation]
	lk.Vignette, lk.Grain = s[game.PhotoVignette], s[game.PhotoGrain]
	return lk
}
