package main

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The interface is the helmet's visor display: tinted glass panels with
// corner brackets, small capitals for labels, monospace for readouts.
// It is laid out for a 720 px tall window and scaled from there.
func uiScale() float32 { return float32(rl.GetScreenHeight()) / 720 }

func screenSize() (float32, float32) {
	return float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())
}

var (
	visorCyan   = color.RGBA{120, 235, 245, 255}
	visorDim    = color.RGBA{70, 140, 160, 255}
	visorAmber  = color.RGBA{255, 190, 80, 255}
	visorGlass  = color.RGBA{10, 20, 30, 185}
	visorStroke = color.RGBA{80, 190, 210, 110}
	// The base's tokens, flat on the visor: off-white, dark steel, hazard.
	uiWhite   = color.RGBA{228, 225, 216, 255}
	uiSteel   = color.RGBA{30, 38, 50, 240}
	uiRow     = color.RGBA{18, 32, 44, 220}
	uiRowLit  = color.RGBA{28, 54, 68, 235}
	uiHazardY = color.RGBA{240, 190, 40, 255}
	uiHazardK = color.RGBA{22, 22, 26, 255}
)

func withAlpha(c color.RGBA, a float64) color.RGBA {
	c.A = uint8(float64(c.A) * clamp01(a))
	return c
}

func (gfx *Graphics) text(s string, x, y, size float32, c color.RGBA) {
	rl.DrawTextEx(gfx.font, s, rl.NewVector2(x, y), size, 0, c)
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

// caps writes a label in spaced small capitals.
func (gfx *Graphics) caps(s string, x, y, size float32, c color.RGBA) {
	rl.DrawTextEx(gfx.smallcaps, s, rl.NewVector2(x, y), size, size*0.12, c)
}

func (gfx *Graphics) capsWidth(s string, size float32) float32 {
	return rl.MeasureTextEx(gfx.smallcaps, s, size, size*0.12).X
}

func (gfx *Graphics) capsCenter(s string, cx, y, size float32, c color.RGBA) {
	gfx.caps(s, cx-gfx.capsWidth(s, size)/2, y, size, c)
}

// mono writes a readout in the monospace font, right-aligned on right.
func (gfx *Graphics) monoRight(s string, right, y, size float32, c color.RGBA) {
	w := rl.MeasureTextEx(gfx.mono, s, size, 0).X
	rl.DrawTextEx(gfx.mono, s, rl.NewVector2(right-w, y), size, 0, c)
}

// tri fills a triangle whatever the order of its corners.
func tri(a, b, c rl.Vector2, col color.RGBA) {
	if (b.X-a.X)*(c.Y-a.Y)-(b.Y-a.Y)*(c.X-a.X) > 0 {
		b, c = c, b
	}
	rl.DrawTriangle(a, b, c, col)
}

// bevel returns the outline of a rectangle with chamfered corners, like
// the base's kit pieces.
func bevel(r rl.Rectangle, cut float32) [8]rl.Vector2 {
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.Width, r.Y+r.Height
	return [8]rl.Vector2{
		{X: x0 + cut, Y: y0}, {X: x1 - cut, Y: y0}, {X: x1, Y: y0 + cut}, {X: x1, Y: y1 - cut},
		{X: x1 - cut, Y: y1}, {X: x0 + cut, Y: y1}, {X: x0, Y: y1 - cut}, {X: x0, Y: y0 + cut},
	}
}

// fillBevel fills a chamfered rectangle.
func fillBevel(r rl.Rectangle, cut float32, col color.RGBA) {
	p := bevel(r, cut)
	c := rl.NewVector2(r.X+r.Width/2, r.Y+r.Height/2)
	for i := range p {
		tri(c, p[i], p[(i+1)%8], col)
	}
}

// outline draws the edge of a chamfered rectangle.
func outline(r rl.Rectangle, cut, thick float32, col color.RGBA) {
	p := bevel(r, cut)
	for i := range p {
		rl.DrawLineEx(p[i], p[(i+1)%8], thick, col)
	}
}

// glass draws a visor panel: tinted glass with chamfered corners, a thin
// outline and an accent tab on the top edge.
func glass(r rl.Rectangle, accent color.RGBA) {
	u := uiScale()
	cut := 10 * u
	fillBevel(r, cut, visorGlass)
	outline(r, cut, 1*u, visorStroke)
	rl.DrawRectangleRec(rl.NewRectangle(r.X+cut+6*u, r.Y-1.5*u, 34*u, 3*u), accent)
	rl.DrawRectangleRec(rl.NewRectangle(r.X+r.Width-cut-12*u, r.Y+r.Height-1.5*u, 6*u, 3*u), accent)
}

// hazard paints yellow and black stripes across r.
func hazard(r rl.Rectangle) {
	u := uiScale()
	rl.DrawRectangleRec(r, uiHazardK)
	rl.BeginScissorMode(int32(r.X), int32(r.Y), int32(r.Width), int32(r.Height))
	for x := r.X - r.Height; x < r.X+r.Width; x += 16 * u {
		a, b := rl.NewVector2(x, r.Y+r.Height), rl.NewVector2(x+8*u, r.Y+r.Height)
		c, d := rl.NewVector2(x+8*u+r.Height, r.Y), rl.NewVector2(x+r.Height, r.Y)
		tri(a, b, c, uiHazardY)
		tri(a, c, d, uiHazardY)
	}
	rl.EndScissorMode()
}

// keycap draws a keyboard key with its label and returns its width.
func (gfx *Graphics) keycap(label string, x, y, size float32) float32 {
	u := uiScale()
	w := rl.MeasureTextEx(gfx.mono, label, size, 0).X + 14*u
	r := rl.NewRectangle(x, y, w, size+8*u)
	fillBevel(r, 4*u, uiWhite)
	fillBevel(rl.NewRectangle(x, y+r.Height-4*u, w, 4*u), 2*u, color.RGBA{120, 126, 140, 255})
	rl.DrawTextEx(gfx.mono, label, rl.NewVector2(x+7*u, y+3*u), size, 0, color.RGBA{20, 26, 34, 255})
	return w
}

// prompt is a centered visor message, optionally led by a key to press.
func (gfx *Graphics) prompt(key, msg string, y float32, c color.RGBA) {
	u := uiScale()
	w, _ := screenSize()
	size := 22 * u
	kw := float32(0)
	if key != "" {
		kw = rl.MeasureTextEx(gfx.mono, key, size*0.8, 0).X + 26*u
	}
	tw := gfx.width(msg, size)
	total := kw + tw
	r := rl.NewRectangle(w/2-total/2-18*u, y-8*u, total+36*u, size+16*u)
	glass(r, c)
	x := w/2 - total/2
	if key != "" {
		gfx.keycap(key, x, y-1*u, size*0.8)
	}
	gfx.text(msg, x+kw, y, size, c)
}

// shopPanel and the item rows are shared by drawing and mouse handling.
func shopPanel() rl.Rectangle {
	u := uiScale()
	w, h := screenSize()
	return rl.NewRectangle((w-680*u)/2, (h-490*u)/2, 680*u, 490*u)
}

func shopItemRect(i int) rl.Rectangle {
	p, u := shopPanel(), uiScale()
	return rl.NewRectangle(p.X+20*u, p.Y+84*u+float32(i)*64*u, p.Width-40*u, 58*u)
}

func (g *Game) drawHUD(cam rl.Camera3D) {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()

	for _, f := range g.floaters {
		if f.Pos.sub(g.player.Eye()).dot(g.player.Forward()) <= 0 {
			continue // behind the camera
		}
		sp := rl.GetWorldToScreen(f.Pos.rl(), cam)
		gfx.textCenter(f.Text, sp.X+1*u, sp.Y+1*u, 26*u, withAlpha(color.RGBA{0, 0, 0, 160}, f.Life))
		gfx.textCenter(f.Text, sp.X, sp.Y, 26*u, withAlpha(f.C, f.Life))
	}

	if g.mode == modePlay || g.mode == modeMap || g.mode == modeShop {
		g.drawVisor()
	}

	if g.mode == modePlay {
		g.drawBaseMarker(cam)
		g.drawCompass()
		g.drawReticle()
		switch {
		case g.st.TotalDust < 5:
			gfx.prompt("CLIC", "Maintiens pour balayer la poussière visée", h*0.64, visorCyan)
		case g.hasUse():
			it, _ := g.nearbyUse()
			gfx.prompt("E", strings.TrimPrefix(it.prompt, "[E] "), h*0.64, visorCyan)
		case g.tankFull() && !g.autoSell() && math.Mod(g.clock, 1) < 0.7:
			gfx.prompt("", "RÉSERVOIR PLEIN · rapporte la poussière à l'incinérateur", h*0.64, visorAmber)
		case !g.aimOK && rl.IsMouseButtonDown(rl.MouseButtonLeft):
			gfx.prompt("", "Trop loin : rapproche-toi du sol", h*0.64, visorDim)
		}
		gfx.caps("ZQSD bouger · MAJ courir · ESPACE sauter · CLIC balayer · E utiliser · ÉCHAP pause · F3 debug", 34*u, h-40*u, 14*u, withAlpha(visorDim, 0.9))
	}
	if g.mode != modePaused {
		g.drawStatus()
	}

	g.drawDebug()

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
	_ = w
}

// drawVisor lays the helmet's visor over the view.
func (g *Game) drawVisor() {
	w, h := screenSize()
	src := rl.NewRectangle(0, 0, float32(g.gfx.visor.Width), float32(g.gfx.visor.Height))
	rl.DrawTexturePro(g.gfx.visor, src, rl.NewRectangle(0, 0, w, h), rl.NewVector2(0, 0), 0, rl.White)
}

// drawReticle: a thin ring with four ticks; brighter when the ground is in reach.
func (g *Game) drawReticle() {
	u := uiScale()
	w, h := screenSize()
	cx, cy := w/2, h/2
	c := withAlpha(visorCyan, 0.45)
	r := 9 * u
	if g.aimOK {
		c = withAlpha(visorCyan, 0.95)
		r = 7 * u
	}
	rl.DrawRing(rl.NewVector2(cx, cy), r-1*u, r, 0, 360, 32, c)
	for _, d := range [][2]float32{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		rl.DrawLineEx(rl.NewVector2(cx+d[0]*(r+3*u), cy+d[1]*(r+3*u)), rl.NewVector2(cx+d[0]*(r+8*u), cy+d[1]*(r+8*u)), 1.5*u, c)
	}
	rl.DrawCircleV(rl.NewVector2(cx, cy), 1.2*u, c)
}

// drawStatus: credits, tank and tool readouts, top left of the visor.
func (g *Game) drawStatus() {
	gfx, u := g.gfx, uiScale()
	x, y := 40*u, 34*u
	r := rl.NewRectangle(x, y, 300*u, 128*u)
	glass(r, visorCyan)
	left, right := x+16*u, x+r.Width-16*u

	// Credits, with a little coin.
	coin := rl.NewVector2(left+9*u, y+24*u)
	rl.DrawCircleV(coin, 9*u, colGold)
	rl.DrawCircleV(coin, 6*u, color.RGBA{200, 150, 40, 255})
	rl.DrawCircleV(rl.NewVector2(coin.X-3*u, coin.Y-3*u), 2*u, color.RGBA{255, 240, 180, 255})
	gfx.caps("CRÉDITS", left+26*u, y+15*u, 15*u, visorDim)
	gfx.monoRight(fmtNum(g.st.Credits), right, y+10*u, 28*u, colGold)
	if g.sellPop > 0 {
		gfx.monoRight(g.sellPopText, right, y-24*u-float32(1.5-g.sellPop)*12*u, 20*u, withAlpha(colGold, g.sellPop))
	}

	// Tank: a canister icon and a segmented gauge.
	ty := y + 52*u
	rl.DrawRectangleRounded(rl.NewRectangle(left+2*u, ty+2*u, 14*u, 20*u), 0.3, 4, visorDim)
	rl.DrawRectangleRec(rl.NewRectangle(left+6*u, ty-1*u, 6*u, 4*u), visorDim)
	gfx.caps("RÉSERVOIR", left+26*u, ty+2*u, 15*u, visorDim)
	gfx.monoRight(fmtNum(g.st.Tank)+"/"+fmtNum(g.capacity())+" g", right, ty, 18*u, colWhite)
	const segs = 20
	fill := clamp01(g.st.Tank / g.capacity())
	segW := (right - left - 26*u) / segs
	for i := range segs {
		c := color.RGBA{30, 60, 75, 200}
		if float64(i)+0.5 < fill*segs {
			c = colBlue
			if fill > 0.8 {
				c = visorAmber
			}
			if g.tankFull() && math.Mod(g.clock, 0.5) < 0.25 {
				c = colRed
			}
		}
		rl.DrawRectangleRec(rl.NewRectangle(left+26*u+float32(i)*segW, ty+26*u, segW-2*u, 8*u), c)
	}

	gfx.caps("OUTIL", left, y+r.Height-30*u, 15*u, visorDim)
	gfx.textRight(g.tool().Name, right, y+r.Height-32*u, 18*u, colWhite)
}

// drawCompass: a heading strip at the top of the visor, north being the
// way to the base (it stands on the north pole).
func (g *Game) drawCompass() {
	gfx, u := g.gfx, uiScale()
	w, _ := screenSize()
	up := g.player.LocalUp()
	north := baseUp.sub(up.scale(baseUp.dot(up)))
	if north.length() < 1e-6 {
		return // standing on the pole itself
	}
	north = north.norm()
	east := north.cross(up)
	heading := math.Atan2(g.player.Heading.dot(east), g.player.Heading.dot(north)) * 180 / math.Pi

	cx, top, half := w/2, 30*u, 220*u
	r := rl.NewRectangle(cx-half-14*u, top-6*u, 2*half+28*u, 46*u)
	glass(r, visorDim)
	labels := map[int]string{0: "N", 90: "E", 180: "S", 270: "O"}
	for a := 0; a < 360; a += 15 {
		rel := math.Mod(float64(a)-heading+540, 360) - 180
		if math.Abs(rel) > 60 {
			continue
		}
		x := cx + float32(rel/60)*half
		alpha := 1 - math.Abs(rel)/70
		if l, ok := labels[a]; ok {
			c := visorCyan
			if a == 0 {
				c = colGold
			}
			gfx.capsCenter(l, x, top+2*u, 18*u, withAlpha(c, alpha))
		} else {
			rl.DrawRectangleRec(rl.NewRectangle(x-0.75*u, top+6*u, 1.5*u, 8*u), withAlpha(visorDim, alpha))
		}
	}
	rl.DrawTriangle(rl.NewVector2(cx, top+26*u), rl.NewVector2(cx-5*u, top+33*u), rl.NewVector2(cx+5*u, top+33*u), visorCyan)
	if dist := arcDist(up, baseUp); dist > 15 {
		gfx.capsCenter("BASE "+fmtNum(dist)+" M", cx, top+44*u, 14*u, withAlpha(colGold, 0.9))
	}
}

// drawBaseMarker shows where the base is when it is in view.
func (g *Game) drawBaseMarker(cam rl.Camera3D) {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	if arcDist(g.player.LocalUp(), baseUp) < 25 {
		return
	}
	target := basePos.add(vec3{0, 9, 0})
	if target.sub(g.player.Eye()).dot(g.player.Forward()) <= 0 {
		return
	}
	sp := rl.GetWorldToScreen(target.rl(), cam)
	if sp.X < 0 || sp.X > w || sp.Y < 0 || sp.Y > h {
		return
	}
	rl.DrawPolyLinesEx(sp, 4, 8*u, 0, 2*u, colGold)
	rl.DrawPoly(sp, 4, 3*u, 0, colGold)
	gfx.capsCenter("BASE", sp.X, sp.Y+12*u, 14*u, colGold)
}

func (g *Game) drawPause() {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	rl.DrawRectangle(0, 0, int32(w), int32(h), color.RGBA{2, 6, 14, 200})
	g.drawVisor()

	title := "MARS SWEEPER"
	size := 84 * u
	tw := rl.MeasureTextEx(gfx.smallcaps, title, size, size*0.12).X
	gfx.caps(title, w/2-tw/2+3*u, h*0.2+3*u, size, color.RGBA{0, 0, 0, 160})
	gfx.caps(title, w/2-tw/2, h*0.2, size, colGold)
	rl.DrawRectangleRec(rl.NewRectangle(w/2-tw/2, h*0.2+size+4*u, tw, 2*u), withAlpha(visorCyan, 0.6))
	if math.Mod(g.clock, 1.4) < 1.0 {
		gfx.capsCenter("CLIQUE POUR ENFILER TON CASQUE", w/2, h*0.2+size+22*u, 22*u, visorCyan)
	}

	keys := [][2]string{
		{"ZQSD", "se déplacer"}, {"MAJ", "courir"}, {"ESPACE", "sauter"},
		{"CLIC", "balayer la poussière visée"}, {"E", "utiliser : incinérateur, labo, carte, lit"},
		{"ÉCHAP", "pause"}, {"F11", "plein écran"},
	}
	panel := rl.NewRectangle(w/2-300*u, h*0.5, 600*u, float32(len(keys))*34*u+28*u)
	glass(panel, visorCyan)
	for i, k := range keys {
		y := panel.Y + 16*u + float32(i)*34*u
		kw := rl.MeasureTextEx(gfx.mono, k[0], 16*u, 0).X + 14*u
		gfx.keycap(k[0], panel.X+170*u-kw, y, 16*u)
		gfx.text(k[1], panel.X+190*u, y+2*u, 20*u, colWhite)
	}
}

// terminal draws a base computer screen: glass, a striped header bar and
// a faint grid, with the title in small capitals.
func (g *Game) terminal(p rl.Rectangle, title string, accent color.RGBA) {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	rl.DrawRectangle(0, 0, int32(w), int32(h), color.RGBA{0, 4, 10, 120})
	fillBevel(p, 14*u, color.RGBA{8, 16, 24, 238})
	// Header: a dark steel bar, an accent tab and a hazard strip.
	fillBevel(rl.NewRectangle(p.X+3*u, p.Y+3*u, p.Width-6*u, 52*u), 11*u, uiSteel)
	rl.DrawRectangleRec(rl.NewRectangle(p.X+14*u, p.Y+18*u, 5*u, 22*u), accent)
	hazard(rl.NewRectangle(p.X+p.Width-170*u, p.Y+48*u, 150*u, 5*u))
	outline(p, 14*u, 1*u, visorStroke)
	rl.DrawRectangleRec(rl.NewRectangle(p.X+20*u, p.Y-1.5*u, 60*u, 3*u), accent)
	gfx.caps(title, p.X+30*u, p.Y+16*u, 24*u, uiWhite)
}

func (g *Game) drawShop() {
	gfx, u := g.gfx, uiScale()
	p := shopPanel()
	g.terminal(p, "LABORATOIRE DE RECHERCHE", visorCyan)
	gfx.monoRight(fmtNum(g.st.Credits)+" cr", p.X+p.Width-22*u, p.Y+12*u, 26*u, colGold)

	for i, it := range g.shopItems() {
		r := shopItemRect(i)
		bg := uiRow
		if i == g.hover {
			bg = uiRowLit
		}
		fillBevel(r, 8*u, bg)
		accent, costCol := color.RGBA{60, 90, 104, 255}, colRed
		if it.affordable(g.st.Credits) {
			accent, costCol = colGreen, colGreen
		}
		if it.maxed {
			accent = colGold
		}
		rl.DrawRectangleRec(rl.NewRectangle(r.X+2*u, r.Y+10*u, 4*u, r.Height-20*u), accent)
		gfx.text(it.name, r.X+18*u, r.Y+6*u, 22*u, uiWhite)
		gfx.text(it.desc, r.X+18*u, r.Y+33*u, 16*u, color.RGBA{150, 180, 190, 255})
		right := r.X + r.Width - 14*u
		if it.maxed {
			gfx.caps("MAX", right-gfx.capsWidth("MAX", 20*u), r.Y+8*u, 20*u, colGold)
		} else {
			gfx.monoRight(fmtNum(it.cost)+" cr", right, r.Y+7*u, 22*u, costCol)
		}
		gfx.caps(strings.ToUpper(it.info), right-gfx.capsWidth(strings.ToUpper(it.info), 14*u), r.Y+35*u, 14*u, visorDim)
	}
	w := gfx.keycap("E", p.X+p.Width/2-70*u, p.Y+p.Height+14*u, 16*u)
	gfx.caps("OU ÉCHAP POUR FERMER", p.X+p.Width/2-62*u+w, p.Y+p.Height+17*u, 15*u, visorDim)
}

// hasUse tells whether something can be used right now with E.
func (g *Game) hasUse() bool {
	_, ok := g.nearbyUse()
	return ok
}

// drawMapPanel shows the state of Mars next to the hologram.
func (g *Game) drawMapPanel() {
	gfx, u := g.gfx, uiScale()
	_, h := screenSize()
	s := g.world.Stats(g.st.TotalDust)
	p := rl.NewRectangle(40*u, h/2-170*u, 440*u, 390*u)
	g.terminal(p, "ÉTAT DE MARS", visorCyan)

	rows := []struct{ label, value string }{
		{"MARS NETTOYÉE", fmt.Sprintf("%.4f %%", s.Cleaned*100)},
		{"POUSSIÈRE COLLECTÉE", fmtNum(g.st.TotalDust) + " g"},
		{"ZONES VISITÉES", fmt.Sprintf("%d/%d", s.TilesVisited, s.TilesTotal)},
		{"CRISTAUX TROUVÉS", fmt.Sprintf("%d/%d", s.Gems, s.GemTotal)},
		{"CRÉDITS GAGNÉS", fmtNum(g.st.TotalEarned) + " cr"},
		{"DISTANCE DE LA BASE", fmtNum(arcDist(g.player.LocalUp(), baseUp)) + " m"},
	}
	for i, r := range rows {
		y := p.Y + 76*u + float32(i)*42*u
		gfx.caps(r.label, p.X+22*u, y+3*u, 16*u, visorDim)
		gfx.monoRight(r.value, p.X+p.Width-22*u, y, 20*u, colWhite)
		rl.DrawRectangleRec(rl.NewRectangle(p.X+22*u, y+30*u, p.Width-44*u, 1), color.RGBA{60, 120, 140, 70})
	}
	y := p.Y + p.Height - 40*u
	legend := []struct {
		c     color.RGBA
		label string
	}{{colGold, "BASE"}, {color.RGBA{255, 90, 70, 255}, "TOI"}, {color.RGBA{190, 255, 255, 255}, "NETTOYÉ"}}
	x := p.X + 26*u
	for _, l := range legend {
		rl.DrawCircleV(rl.NewVector2(x, y+8*u), 5*u, l.c)
		gfx.caps(l.label, x+12*u, y, 15*u, visorDim)
		x += gfx.capsWidth(l.label, 15*u) + 40*u
	}
	kw := gfx.keycap("E", p.X, p.Y+p.Height+14*u, 16*u)
	gfx.caps("OU ÉCHAP POUR FERMER", p.X+kw+8*u, p.Y+p.Height+17*u, 15*u, visorDim)
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
		a := math.Min(t-sleepFadeOut*0.6, sleepWake+0.3-t) * 2
		dots := strings.Repeat(" z", 1+int(t*2)%3)
		gfx.capsCenter("Z"+dots+"…", w/2, h/2-30*u, 44*u, withAlpha(color.RGBA{180, 190, 255, 255}, a))
		if t > sleepSave {
			gfx.capsCenter("PARTIE SAUVEGARDÉE", w/2, h/2+30*u, 18*u, withAlpha(visorDim, a))
		}
	}
}
