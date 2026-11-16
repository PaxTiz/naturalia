package ui

import (
	"fmt"
	"image/color"
	"naturalia/internal/game"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// drawScope is the view through the telescope: black all around a round
// field, a dark rim, a fine cross, the zoom, and the keys.
func (hud *HUD) drawScope(g *game.Game) {
	u := uiScale()
	w, h := screenSize()
	c := rl.Vector2{X: w / 2, Y: h / 2}
	r := h * 0.46
	rl.DrawRing(c, r, w, 0, 360, 96, rl.Black)
	rl.DrawRing(c, r-10*u, r+1, 0, 360, 96, color.RGBA{0, 0, 0, 150})
	rl.DrawRing(c, r-3*u, r, 0, 360, 96, color.RGBA{20, 22, 28, 255})
	fine := color.RGBA{0, 0, 0, 90}
	rl.DrawLineEx(rl.Vector2{X: c.X - r*0.35, Y: c.Y}, rl.Vector2{X: c.X - r*0.06, Y: c.Y}, 1.5*u, fine)
	rl.DrawLineEx(rl.Vector2{X: c.X + r*0.06, Y: c.Y}, rl.Vector2{X: c.X + r*0.35, Y: c.Y}, 1.5*u, fine)
	rl.DrawLineEx(rl.Vector2{X: c.X, Y: c.Y + r*0.06}, rl.Vector2{X: c.X, Y: c.Y + r*0.35}, 1.5*u, fine)
	zoom := g.Fov / g.Scope.Fov
	hud.textCenter(fmt.Sprintf("× %.1f", zoom), c.X, c.Y+r*0.82, 18*u, visorDim)
	hud.prompt(game.KeyLabel(game.ActUse), "Revenir", h-60*u, visorCyan)
	hud.text("MOLETTE  zoom    "+game.KeyLabel(game.ActPhoto)+"  photo", 24*u, h-40*u, 16*u, visorDim)
}
