package ui

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"naturalia/internal/game"
	"path/filepath"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// photoUI is the photo mode's interface: the settings panel, and the
// picture being taken.
type photoUI struct {
	sel    int     // the setting under the arrows
	hidden bool    // H: nothing over the view but the frame
	shoot  bool    // take the picture at the next draw, before any interface
	flash  float64 // the shutter's white flash, fading
	saved  string  // the last picture's name...
	savedT float64 // ...shown for this long still
}

// updatePhoto follows the panel's keys: arrows to pick and change a
// setting, Backspace to reset it, H to hide everything, Enter or a click
// to take the picture.
func (hud *HUD) updatePhoto(g *game.Game, dt float64) {
	p := &hud.photo
	p.flash = math.Max(0, p.flash-dt)
	p.savedT = math.Max(0, p.savedT-dt)
	if g.Mode != game.ModePhoto {
		return
	}
	pressed := func(k int32) bool { return rl.IsKeyPressed(k) || rl.IsKeyPressedRepeat(k) }
	switch {
	case pressed(rl.KeyDown):
		p.sel = (p.sel + 1) % game.PhotoSettings
	case pressed(rl.KeyUp):
		p.sel = (p.sel + game.PhotoSettings - 1) % game.PhotoSettings
	case pressed(rl.KeyRight):
		g.Photo.Adjust(p.sel, 1)
	case pressed(rl.KeyLeft):
		g.Photo.Adjust(p.sel, -1)
	case rl.IsKeyPressed(rl.KeyBackspace):
		g.Photo.Reset(p.sel)
	case rl.IsKeyPressed(rl.KeyH):
		p.hidden = !p.hidden
	case rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter) || rl.IsMouseButtonPressed(rl.MouseButtonLeft):
		p.shoot = true
	}
}

// HidePhoto hides the photo mode's panel, for the screenshot tool.
func (hud *HUD) HidePhoto() { hud.photo.hidden = true }

// photoFrame is the picture's rectangle on the screen, centered, in the
// chosen proportions.
func photoFrame(g *game.Game) rl.Rectangle {
	w, h := screenSize()
	a := float32(g.Photo.Aspect())
	switch {
	case a == 0:
		return rl.NewRectangle(0, 0, w, h)
	case a > w/h:
		return rl.NewRectangle(0, (h-w/a)/2, w, w/a)
	default:
		return rl.NewRectangle((w-h*a)/2, 0, h*a, h)
	}
}

// drawPhoto draws the photo mode over the view: black bars around the
// frame, the guides and the panel. When a picture is due it is taken
// first, so none of it shows.
func (hud *HUD) drawPhoto(g *game.Game) {
	p := &hud.photo
	u := uiScale()
	w, h := screenSize()
	fr := photoFrame(g)
	if p.shoot {
		p.shoot = false
		hud.takePhoto(fr)
		g.Shutter()
	}
	bars := color.RGBA{0, 0, 0, 255}
	rl.DrawRectangleRec(rl.NewRectangle(0, 0, w, fr.Y), bars)
	rl.DrawRectangleRec(rl.NewRectangle(0, fr.Y+fr.Height, w, h-fr.Y-fr.Height), bars)
	rl.DrawRectangleRec(rl.NewRectangle(0, 0, fr.X, h), bars)
	rl.DrawRectangleRec(rl.NewRectangle(fr.X+fr.Width, 0, w-fr.X-fr.Width, h), bars)

	if p.flash > 0 {
		rl.DrawRectangle(0, 0, int32(w), int32(h), withAlpha(color.RGBA{255, 255, 255, 255}, p.flash/0.3))
	}
	if p.savedT > 0 {
		hud.prompt("", "Photo enregistrée · "+p.saved, h*0.08, withAlpha(visorCyan, math.Min(1, p.savedT)))
	}
	if p.hidden {
		return
	}
	set := &g.Photo.Set
	guide := color.RGBA{255, 255, 255, 70}
	if set[game.PhotoGrid] != 0 {
		for i := float32(1); i < 3; i++ {
			rl.DrawRectangleRec(rl.NewRectangle(fr.X+fr.Width*i/3, fr.Y, 1*u, fr.Height), guide)
			rl.DrawRectangleRec(rl.NewRectangle(fr.X, fr.Y+fr.Height*i/3, fr.Width, 1*u), guide)
		}
	}
	// The autofocus point: corners around the screen's center.
	if set[game.PhotoAperture] > 0 && set[game.PhotoFocus] == 0 {
		cx, cy, s, l := w/2, h/2, 16*u, 6*u
		for _, d := range [][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
			x, y := cx+d[0]*s, cy+d[1]*s
			rl.DrawRectangleRec(rl.NewRectangle(min(x, x-d[0]*l), y-u, l, 2*u), visorCyan)
			rl.DrawRectangleRec(rl.NewRectangle(x-u, min(y, y-d[1]*l), 2*u, l), visorCyan)
		}
	}
	hud.drawPhotoPanel(g)
	hud.drawPhotoHints()
}

// drawPhotoPanel lists the settings, the selected one with its arrows.
func (hud *HUD) drawPhotoPanel(g *game.Game) {
	u := uiScale()
	_, h := screenSize()
	row := 27 * u
	box := rl.NewRectangle(24*u, 0, 330*u, 78*u+row*game.PhotoSettings)
	box.Y = (h - box.Height) / 2
	visorPanel(box, visorCyan)
	hud.caps("MODE PHOTO", box.X+22*u, box.Y+14*u, 15*u, uiWhite)
	ph := &g.Photo
	hud.monoRight(fmt.Sprintf("%.0f mm", ph.FocalMM()), box.X+box.Width-18*u, box.Y+14*u, 14*u, visorDim)
	for i, d := range game.PhotoSettingDefs {
		y := box.Y + 46*u + float32(i)*row
		r := rl.NewRectangle(box.X+12*u, y, box.Width-24*u, row-3*u)
		// Values left at their default are dimmer: what was changed stands out.
		label, value := uiMuted, withAlpha(uiWhite, 0.55)
		if ph.Set[i] != d.Default {
			value = uiWhite
		}
		if i == hud.photo.sel {
			fillCut(r, 6*u, uiRowLit)
			rl.DrawRectangleRec(rl.NewRectangle(r.X, r.Y+6*u, 3*u, r.Height-6*u), visorCyan)
			label, value = uiWhite, visorCyan
		}
		v := ph.Set[i]
		var s string
		if d.Labels != nil {
			s = d.Labels[int(v)]
		} else {
			s = d.Format(v)
		}
		hud.text(d.Name, r.X+12*u, r.Y+3*u, 16*u, label)
		right := r.X + r.Width - 12*u
		if i == hud.photo.sel {
			hud.textRight("►", right, r.Y+3*u, 14*u, visorCyan)
			right -= 22 * u
			hud.textRight(s, right, r.Y+3*u, 16*u, value)
			hud.textRight("◄", right-hud.width(s, 16*u)-8*u, r.Y+3*u, 14*u, visorCyan)
		} else {
			hud.textRight(s, right, r.Y+3*u, 16*u, value)
		}
	}
	hud.caps("FLÈCHES : CHOISIR, RÉGLER · RETOUR : DÉFAUT", box.X+22*u, box.Y+box.Height-24*u, 11*u, visorDim)
}

// drawPhotoHints lists the camera's keys along the bottom.
func (hud *HUD) drawPhotoHints() {
	u := uiScale()
	w, h := screenSize()
	hints := [][2]string{
		{"ZQSD", "bouger"}, {"ESPACE/CTRL", "monter, descendre"}, {"A/E", "pencher"}, {"MOLETTE", "zoom"},
		{"ENTRÉE", "photo"}, {"H", "masquer"}, {"P", "quitter"},
	}
	key, label := 11*u, 12*u
	total := float32(0)
	for _, k := range hints {
		total += rl.MeasureTextEx(hud.mono, k[0], key, 0).X + 12*u + 6*u + hud.capsWidth(k[1], label) + 18*u
	}
	x, y := w/2-(total-18*u)/2, h-36*u
	for _, k := range hints {
		x += hud.keycap(k[0], x, y, key) + 6*u
		hud.caps(k[1], x, y+2*u, label, withAlpha(uiWhite, 0.8))
		x += hud.capsWidth(k[1], label) + 18*u
	}
}

// takePhoto saves what is on the screen, cropped to the frame, in the
// settings folder's photos/.
func (hud *HUD) takePhoto(fr rl.Rectangle) {
	p := &hud.photo
	path, err := game.PhotoPath()
	if err != nil {
		log.Println("photo:", err)
		return
	}
	img := rl.LoadImageFromScreen()
	defer rl.UnloadImage(img)
	// The image is in pixels, the screen in points on high density displays.
	w, h := screenSize()
	sx, sy := float32(img.Width)/w, float32(img.Height)/h
	rl.ImageCrop(img, rl.NewRectangle(float32(math.Round(float64(fr.X*sx))), float32(math.Round(float64(fr.Y*sy))),
		float32(math.Round(float64(fr.Width*sx))), float32(math.Round(float64(fr.Height*sy)))))
	if !rl.ExportImage(*img, path) {
		log.Println("photo: could not write", path)
		return
	}
	log.Println("photo:", path)
	p.flash, p.saved, p.savedT = 0.3, filepath.Base(path), 3
}
