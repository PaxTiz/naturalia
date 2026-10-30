package main

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The UI is laid out for a 720 px tall window and scaled from there.
func uiScale() float32 { return float32(rl.GetScreenHeight()) / 720 }

func screenSize() (float32, float32) {
	return float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())
}

func (gfx *Graphics) text(s string, x, y, size float32, c color.RGBA) {
	rl.DrawTextEx(gfx.font, s, rl.NewVector2(x, y), size, 0, c)
}

func (gfx *Graphics) title(s string, x, y, size float32, c color.RGBA) {
	rl.DrawTextEx(gfx.bold, s, rl.NewVector2(x, y), size, 0, c)
}

func (gfx *Graphics) width(s string, size float32) float32 {
	return rl.MeasureTextEx(gfx.font, s, size, 0).X
}

func (gfx *Graphics) textRight(s string, right, y, size float32, c color.RGBA) {
	gfx.text(s, right-gfx.width(s, size), y, size, c)
}

func (gfx *Graphics) textCenter(s string, cx, y, size float32, c color.RGBA) {
	gfx.text(s, cx-gfx.width(s, size)/2, y, size, c)
}

// banner draws a centered message on a dark strip.
func (gfx *Graphics) banner(s string, y, size float32, c color.RGBA) {
	w, _ := screenSize()
	tw := gfx.width(s, size)
	pad := size * 0.6
	rl.DrawRectangleRec(rl.NewRectangle(w/2-tw/2-pad, y-pad/2, tw+2*pad, size+pad), color.RGBA{0, 0, 0, 170})
	gfx.textCenter(s, w/2, y, size, c)
}

func shopPanel() rl.Rectangle {
	u := uiScale()
	w, h := screenSize()
	return rl.NewRectangle((w-660*u)/2, (h-472*u)/2, 660*u, 472*u)
}

func shopItemRect(i int) rl.Rectangle {
	p, u := shopPanel(), uiScale()
	return rl.NewRectangle(p.X+20*u, p.Y+76*u+float32(i)*64*u, p.Width-40*u, 58*u)
}

func (g *Game) drawHUD(cam rl.Camera3D) {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()

	for _, f := range g.floaters {
		if f.Pos.sub(g.player.Eye()).dot(g.player.Forward()) <= 0 {
			continue // behind the camera
		}
		sp := rl.GetWorldToScreen(f.Pos.rl(), cam)
		c := f.C
		c.A = uint8(255 * clamp01(f.Life))
		gfx.textCenter(f.Text, sp.X, sp.Y, 26*u, c)
	}

	g.drawStatus()

	if g.mode == modePlay {
		g.drawBaseMarker(cam)
		// Crosshair: a ring when the ground is within reach, a dot otherwise.
		if g.aimOK {
			rl.DrawCircleLines(int32(w/2), int32(h/2), 6*u, color.RGBA{255, 255, 255, 200})
		} else {
			rl.DrawCircle(int32(w/2), int32(h/2), 2*u, color.RGBA{255, 255, 255, 120})
		}

		switch {
		case g.st.TotalDust < 5:
			gfx.banner("Vise le sol et maintiens le clic gauche pour balayer la poussière", h*0.62, 22*u, colWhite)
		case g.hasUse():
			it, _ := g.nearbyUse()
			gfx.banner(it.prompt, h*0.62, 24*u, colCyan)
		case g.tankFull() && !g.autoSell() && math.Mod(g.clock, 1) < 0.7:
			gfx.banner("RÉSERVOIR PLEIN ! Retourne à la base pour vendre", h*0.62, 24*u, colRed)
		case !g.aimOK && rl.IsMouseButtonDown(rl.MouseButtonLeft):
			gfx.banner("Trop loin : rapproche-toi du sol", h*0.62, 20*u, colGray)
		}
		gfx.text("ZQSD/WASD bouger · Maj courir · Espace sauter · Clic balayer · Échap pause · F11 plein écran", 16*u, h-28*u, 16*u, colGray)
	}

	switch g.mode {
	case modePaused:
		g.drawPause()
	case modeShop:
		g.drawShop()
	case modeMap:
		g.drawMapPanel()
	case modeSleep:
		g.drawSleep()
	}
}

func (g *Game) drawStatus() {
	gfx, u := g.gfx, uiScale()
	x, y := 16*u, 16*u
	rl.DrawRectangleRec(rl.NewRectangle(x, y, 320*u, 112*u), color.RGBA{0, 0, 0, 150})
	x, right := x+14*u, x+306*u

	gfx.text("Crédits", x, y+10*u, 20*u, colGray)
	gfx.textRight(fmtNum(g.st.Credits)+" cr", right, y+6*u, 28*u, colGold)
	if g.sellPop > 0 {
		c := colGold
		c.A = uint8(255 * clamp01(g.sellPop))
		gfx.textRight(g.sellPopText, right, y-26*u+float32(1.5-g.sellPop)*-12*u, 22*u, c)
	}

	gfx.text("Réservoir", x, y+42*u, 20*u, colGray)
	gfx.textRight(fmtNum(g.st.Tank)+" / "+fmtNum(g.capacity())+" g", right, y+42*u, 20*u, colWhite)
	bar := rl.NewRectangle(x, y+68*u, right-x, 8*u)
	rl.DrawRectangleRec(bar, color.RGBA{40, 44, 70, 255})
	bar.Width *= float32(clamp01(g.st.Tank / g.capacity()))
	barCol := colBlue
	if g.tankFull() && math.Mod(g.clock, 0.5) < 0.25 {
		barCol = colRed
	}
	rl.DrawRectangleRec(bar, barCol)

	gfx.text("Outil : "+g.tool().Name, x, y+84*u, 18*u, colWhite)
}

// drawBaseMarker points to the base when it is far away.
func (g *Game) drawBaseMarker(cam rl.Camera3D) {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	dist := arcDist(g.player.LocalUp(), basePos.norm())
	if dist < 15 {
		return
	}
	label := "BASE · " + fmtNum(dist) + " m"
	target := basePos.add(vec3{0, 7, 0})
	if target.sub(g.player.Eye()).dot(g.player.Forward()) > 0 {
		sp := rl.GetWorldToScreen(target.rl(), cam)
		if sp.X > 0 && sp.X < w && sp.Y > 0 && sp.Y < h {
			rl.DrawPoly(sp, 4, 7*u, 0, colGold)
			gfx.textCenter(label, sp.X, sp.Y+10*u, 18*u, colGold)
			return
		}
	}
	// Off screen: show it on the side the player has to turn to.
	side := target.sub(g.player.Eye()).dot(g.player.Right())
	if side > 0 {
		gfx.textRight(label+"  ►", w-20*u, h/2, 20*u, colGold)
	} else {
		gfx.text("◄  "+label, 20*u, h/2, 20*u, colGold)
	}
}

func (g *Game) drawPause() {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	rl.DrawRectangle(0, 0, int32(w), int32(h), color.RGBA{0, 0, 10, 170})
	t := "MOON SWEEPER"
	tw := rl.MeasureTextEx(gfx.bold, t, 72*u, 0).X
	gfx.title(t, w/2-tw/2, h*0.22, 72*u, colGold)
	gfx.textCenter("Clique pour jouer", w/2, h*0.22+90*u, 30*u, colWhite)

	lines := []string{
		"ZQSD / WASD : se déplacer      Maj : courir      Espace : sauter (gravité lunaire !)",
		"Clic gauche : balayer la poussière visée",
		"E : utiliser (incinérateur pour vendre, laboratoire, table de la carte, lit)",
		"Échap : pause      F11 : plein écran",
	}
	for i, l := range lines {
		gfx.textCenter(l, w/2, h*0.55+float32(i)*30*u, 20*u, colGray)
	}
}

func (g *Game) drawShop() {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	rl.DrawRectangle(0, 0, int32(w), int32(h), color.RGBA{0, 0, 10, 140})
	p := shopPanel()
	rl.DrawRectangleRec(p, color.RGBA{16, 18, 34, 245})
	rl.DrawRectangleLinesEx(p, 2*u, color.RGBA{60, 200, 220, 255})

	gfx.title("LABORATOIRE DE RECHERCHE", p.X+20*u, p.Y+20*u, 28*u, colCyan)
	gfx.textRight(fmtNum(g.st.Credits)+" cr", p.X+p.Width-20*u, p.Y+20*u, 28*u, colGold)

	for i, it := range g.shopItems() {
		r := shopItemRect(i)
		bg := color.RGBA{28, 32, 56, 255}
		if i == g.hover {
			bg = color.RGBA{42, 48, 82, 255}
		}
		rl.DrawRectangleRec(r, bg)
		border, costCol := color.RGBA{60, 66, 100, 255}, colRed
		if it.affordable(g.st.Credits) {
			border, costCol = colGreen, colGreen
		}
		rl.DrawRectangleLinesEx(r, 1*u, border)
		gfx.text(it.name, r.X+14*u, r.Y+6*u, 22*u, colWhite)
		gfx.text(it.desc, r.X+14*u, r.Y+33*u, 16*u, colGray)
		right := r.X + r.Width - 14*u
		if it.maxed {
			gfx.textRight("MAX", right, r.Y+6*u, 22*u, colGold)
		} else {
			gfx.textRight(fmtNum(it.cost)+" cr", right, r.Y+6*u, 22*u, costCol)
		}
		gfx.textRight(it.info, right, r.Y+33*u, 16*u, colGray)
	}
	gfx.textCenter("[E] ou [Échap] pour fermer", p.X+p.Width/2, p.Y+p.Height+10*u, 18*u, colGray)
}

// hasUse tells whether something can be used right now with E.
func (g *Game) hasUse() bool {
	_, ok := g.nearbyUse()
	return ok
}

// drawMapPanel shows the state of the Moon next to the hologram.
func (g *Game) drawMapPanel() {
	gfx, u := g.gfx, uiScale()
	_, h := screenSize()
	s := g.world.Stats(g.st.TotalDust)
	p := rl.NewRectangle(24*u, h/2-190*u, 420*u, 380*u)
	rl.DrawRectangleRec(p, color.RGBA{10, 24, 34, 225})
	rl.DrawRectangleLinesEx(p, 2*u, color.RGBA{60, 200, 220, 255})
	gfx.title("ÉTAT DE LA LUNE", p.X+20*u, p.Y+18*u, 28*u, colCyan)

	rows := []struct{ label, value string }{
		{"Lune nettoyée", fmt.Sprintf("%.4f %%", s.Cleaned*100)},
		{"Poussière collectée", fmtNum(g.st.TotalDust) + " g"},
		{"Zones visitées", fmt.Sprintf("%d / %d", s.TilesVisited, s.TilesTotal)},
		{"Cristaux trouvés", fmt.Sprintf("%d / %d", s.Gems, s.GemTotal)},
		{"Crédits gagnés", fmtNum(g.st.TotalEarned) + " cr"},
		{"Distance de la base", fmtNum(arcDist(g.player.LocalUp(), baseUp)) + " m"},
	}
	for i, r := range rows {
		y := p.Y + 72*u + float32(i)*40*u
		gfx.text(r.label, p.X+20*u, y, 20*u, colGray)
		gfx.textRight(r.value, p.X+p.Width-20*u, y, 20*u, colWhite)
	}
	y := p.Y + p.Height - 44*u
	rl.DrawCircle(int32(p.X+26*u), int32(y+10*u), 6*u, colGold)
	gfx.text("Base", p.X+38*u, y, 18*u, colGray)
	rl.DrawCircle(int32(p.X+116*u), int32(y+10*u), 6*u, color.RGBA{255, 90, 70, 255})
	gfx.text("Toi", p.X+128*u, y, 18*u, colGray)
	rl.DrawCircle(int32(p.X+196*u), int32(y+10*u), 6*u, color.RGBA{190, 255, 255, 255})
	gfx.text("Zones nettoyées", p.X+208*u, y, 18*u, colGray)
	gfx.text("[E] ou [Échap] pour fermer", p.X, p.Y+p.Height+12*u, 18*u, colGray)
}

// drawSleep fades to black, shows the night passing, then fades back in.
func (g *Game) drawSleep() {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	t := g.sleepT
	dark := 1.0
	switch {
	case t < sleepFadeOut:
		dark = t / sleepFadeOut
	case t > sleepWake:
		dark = 1 - (t-sleepWake)/(sleepEnd-sleepWake)
	}
	rl.DrawRectangle(0, 0, int32(w), int32(h), color.RGBA{0, 0, 4, uint8(255 * clamp01(dark))})
	if t > sleepFadeOut*0.6 && t < sleepWake+0.3 {
		a := uint8(255 * clamp01(math.Min(t-sleepFadeOut*0.6, sleepWake+0.3-t)*2))
		dots := strings.Repeat(" z", 1+int(t*2)%3)
		gfx.textCenter("Z"+dots+"…", w/2, h/2-30*u, 44*u, color.RGBA{180, 190, 255, a})
		if t > sleepSave {
			gfx.textCenter("Partie sauvegardée", w/2, h/2+30*u, 20*u, color.RGBA{150, 155, 178, a})
		}
	}
}
