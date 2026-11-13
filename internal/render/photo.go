package render

import (
	"naturalia/internal/game"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// look reads the photo mode's settings into the picture's; out of it, the
// game's usual look.
func (gfx *Graphics) look(g *game.Game) look {
	lk := look{vignette: 0.45, grain: 0.2, near: gfx.near, far: gfx.far}
	if g.Mode != game.ModePhoto {
		return lk
	}
	s := &g.Photo.Set
	lk.exposure = s[game.PhotoExposure]
	lk.focus = game.FocusDistances[int(s[game.PhotoFocus])]
	// The largest blur grows with the window, to look the same at any size.
	lk.blur = s[game.PhotoAperture] * 0.03 * float64(rl.GetRenderHeight())
	lk.filter = int(s[game.PhotoFilter])
	lk.contrast, lk.sat = s[game.PhotoContrast], s[game.PhotoSaturation]
	lk.vignette, lk.grain = s[game.PhotoVignette], s[game.PhotoGrain]
	return lk
}
