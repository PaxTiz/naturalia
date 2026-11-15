package ui

import (
	"fmt"
	"naturalia/internal/game"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// settingsUI is the pause screen's sound panel.
type settingsUI struct {
	sel  int // the slider under the arrows
	drag int // the slider held by the mouse, or -1
}

// The pause screen's two panels, side by side: the keys, then the sound.
func pauseBoxes() (keys, sound rl.Rectangle) {
	u := uiScale()
	w, h := screenSize()
	height := 5*38*u + 50*u
	x := w/2 - 520*u
	return rl.NewRectangle(x, h*0.53, 660*u, height), rl.NewRectangle(x+680*u, h*0.53, 360*u, height)
}

// settingRow is a slider's row in the sound panel, and its bar.
func settingRow(box rl.Rectangle, i int) (row, bar rl.Rectangle) {
	u := uiScale()
	row = rl.NewRectangle(box.X+12*u, box.Y+40*u+float32(i)*52*u, box.Width-24*u, 48*u)
	bar = rl.NewRectangle(row.X+12*u, row.Y+32*u, row.Width-24*u, 6*u)
	return row, bar
}

// updateSettings follows the sound panel: up and down pick a slider, left
// and right move it; the mouse picks the one it is over and moves it by a
// click or a drag along its bar.
func (hud *HUD) updateSettings(g *game.Game) {
	s := &hud.settings
	if g.Mode != game.ModePaused {
		s.drag = -1
		return
	}
	pressed := func(k int32) bool { return rl.IsKeyPressed(k) || rl.IsKeyPressedRepeat(k) }
	switch {
	case pressed(rl.KeyDown):
		s.sel = (s.sel + 1) % game.SoundSettings
	case pressed(rl.KeyUp):
		s.sel = (s.sel + game.SoundSettings - 1) % game.SoundSettings
	case pressed(rl.KeyRight):
		g.SetSound(s.sel, g.Sound(s.sel)+0.05)
	case pressed(rl.KeyLeft):
		g.SetSound(s.sel, g.Sound(s.sel)-0.05)
	}

	_, box := pauseBoxes()
	mouse := rl.GetMousePosition()
	click := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	if click && !rl.CheckCollisionPointRec(mouse, box) {
		g.Resume()
	}
	for i := range game.SoundSettings {
		row, bar := settingRow(box, i)
		if rl.CheckCollisionPointRec(mouse, row) && s.drag < 0 {
			s.sel = i
			if click {
				s.drag = i
			}
		}
		if s.drag == i {
			g.SetSound(i, float64((mouse.X-bar.X)/bar.Width))
		}
	}
	if !rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		s.drag = -1
	}
}

// drawSettings draws the sound panel: a slider per volume, the one
// selected lit, its value in percent.
func (hud *HUD) drawSettings(g *game.Game, box rl.Rectangle) {
	u := uiScale()
	visorPanel(box, visorCyan)
	hud.caps("SON", box.X+24*u, box.Y+14*u, 13*u, visorDim)
	for i, name := range game.SoundNames {
		row, bar := settingRow(box, i)
		v := float32(g.Sound(i))
		label, fill := uiMuted, withAlpha(uiWhite, 0.55)
		if i == hud.settings.sel {
			fillCut(row, 6*u, uiRowLit)
			rl.DrawRectangleRec(rl.NewRectangle(row.X, row.Y+6*u, 3*u, row.Height-6*u), visorCyan)
			label, fill = uiWhite, visorCyan
		}
		value := fmt.Sprintf("%.0f %%", v*100)
		if v == 0 {
			value = "coupé"
		}
		hud.text(name, row.X+12*u, row.Y+5*u, 16*u, label)
		hud.textRight(value, row.X+row.Width-12*u, row.Y+5*u, 16*u, label)
		rl.DrawRectangleRec(bar, withAlpha(uiWhite, 0.12))
		rl.DrawRectangleRec(rl.NewRectangle(bar.X, bar.Y, bar.Width*v, bar.Height), fill)
		rl.DrawRectangleRec(rl.NewRectangle(bar.X+bar.Width*v-2*u, bar.Y-4*u, 4*u, bar.Height+8*u), fill)
	}
	hud.caps("FLÈCHES OU SOURIS : RÉGLER", box.X+24*u, box.Y+box.Height-24*u, 11*u, visorDim)
}
