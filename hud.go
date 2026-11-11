package main

import (
	"fmt"
	"image/color"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/planet"
	"math"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The interface is the helmet's visor display, in the base's kit language:
// flat charcoal plates with two chamfered corners, off-white headers like
// the habitat panels, hazard blocks, small capitals for labels and
// monospace for readouts. Cyan marks what can be used, gold what is earned.
// It is laid out for a 720 px tall window and scaled from there.
func uiScale() float32 { return float32(rl.GetScreenHeight()) / 720 }

func screenSize() (float32, float32) {
	return float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())
}

var (
	visorCyan  = color.RGBA{120, 235, 245, 255}
	visorDim   = color.RGBA{150, 156, 166, 255} // labels
	visorAmber = color.RGBA{255, 190, 80, 255}
	// The base's tokens, flat on the visor: off-white, charcoal, hazard.
	uiWhite   = color.RGBA{238, 234, 224, 255}
	uiMuted   = color.RGBA{160, 164, 172, 255}
	uiFaint   = color.RGBA{92, 98, 110, 255}
	uiInk     = color.RGBA{20, 22, 28, 255}
	uiPlate   = color.RGBA{20, 22, 29, 205}
	uiRow     = color.RGBA{34, 37, 46, 235}
	uiRowLit  = color.RGBA{50, 58, 72, 245}
	uiTrack   = color.RGBA{52, 56, 68, 230}
	uiCostOff = color.RGBA{205, 120, 105, 255}
	uiHazardY = color.RGBA{240, 190, 40, 255}
	uiHazardK = color.RGBA{22, 22, 26, 255}
)

func withAlpha(c color.RGBA, a float64) color.RGBA {
	c.A = uint8(float64(c.A) * mathx.Clamp01(a))
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

func (gfx *Graphics) capsRight(s string, right, y, size float32, c color.RGBA) {
	gfx.caps(s, right-gfx.capsWidth(s, size), y, size, c)
}

// mono writes a readout in the monospace font.
func (gfx *Graphics) monoAt(s string, x, y, size float32, c color.RGBA) float32 {
	rl.DrawTextEx(gfx.mono, s, rl.NewVector2(x, y), size, 0, c)
	return rl.MeasureTextEx(gfx.mono, s, size, 0).X
}

func (gfx *Graphics) monoRight(s string, right, y, size float32, c color.RGBA) {
	w := rl.MeasureTextEx(gfx.mono, s, size, 0).X
	rl.DrawTextEx(gfx.mono, s, rl.NewVector2(right-w, y), size, 0, c)
}

// title writes big bold capitals, slightly spaced.
func (gfx *Graphics) title(s string, x, y, size float32, c color.RGBA) {
	rl.DrawTextEx(gfx.bold, s, rl.NewVector2(x, y), size, size*0.08, c)
}

func (gfx *Graphics) titleWidth(s string, size float32) float32 {
	return rl.MeasureTextEx(gfx.bold, s, size, size*0.08).X
}

// tri fills a triangle whatever the order of its corners.
func tri(a, b, c rl.Vector2, col color.RGBA) {
	if (b.X-a.X)*(c.Y-a.Y)-(b.Y-a.Y)*(c.X-a.X) > 0 {
		b, c = c, b
	}
	rl.DrawTriangle(a, b, c, col)
}

// chamfer returns the outline of r with each corner cut by its own amount
// (top left, top right, bottom right, bottom left), like the kit's bevels.
func chamfer(r rl.Rectangle, tl, tr, br, bl float32) [8]rl.Vector2 {
	x0, y0, x1, y1 := r.X, r.Y, r.X+r.Width, r.Y+r.Height
	return [8]rl.Vector2{
		{X: x0 + tl, Y: y0}, {X: x1 - tr, Y: y0}, {X: x1, Y: y0 + tr}, {X: x1, Y: y1 - br},
		{X: x1 - br, Y: y1}, {X: x0 + bl, Y: y1}, {X: x0, Y: y1 - bl}, {X: x0, Y: y0 + tl},
	}
}

// fillShape fills a convex outline from chamfer.
func fillShape(p [8]rl.Vector2, col color.RGBA) {
	var c rl.Vector2
	for _, v := range p {
		c.X += v.X / 8
		c.Y += v.Y / 8
	}
	for i := range p {
		tri(c, p[i], p[(i+1)%8], col)
	}
}

// fillBevel fills a rectangle with its four corners chamfered.
func fillBevel(r rl.Rectangle, cut float32, col color.RGBA) {
	fillShape(chamfer(r, cut, cut, cut, cut), col)
}

// fillCut fills the plate shape: top left and bottom right corners cut.
func fillCut(r rl.Rectangle, cut float32, col color.RGBA) {
	fillShape(chamfer(r, cut, 0, cut, 0), col)
}

// visorPanel draws a visor panel: flat charcoal, two chamfered corners, a lit
// top edge like a bevel catching the light, and an accent notch on the left.
func visorPanel(r rl.Rectangle, accent color.RGBA) {
	u := uiScale()
	cut := 12 * u
	fillCut(r, cut, uiPlate)
	rl.DrawRectangleRec(rl.NewRectangle(r.X+cut, r.Y, r.Width-cut, 1*u), color.RGBA{255, 255, 255, 26})
	rl.DrawRectangleRec(rl.NewRectangle(r.X, r.Y+cut+4*u, 3*u, 20*u), accent)
}

// hazard paints yellow and black stripes across r.
func hazard(r rl.Rectangle) {
	u := uiScale()
	rl.DrawRectangleRec(r, uiHazardK)
	rl.BeginScissorMode(int32(r.X), int32(r.Y), int32(math.Ceil(float64(r.Width))), int32(math.Ceil(float64(r.Height))))
	for x := r.X - r.Height; x < r.X+r.Width; x += 14 * u {
		a, b := rl.NewVector2(x, r.Y+r.Height), rl.NewVector2(x+7*u, r.Y+r.Height)
		c, d := rl.NewVector2(x+7*u+r.Height, r.Y), rl.NewVector2(x+r.Height, r.Y)
		tri(a, b, c, uiHazardY)
		tri(a, c, d, uiHazardY)
	}
	rl.EndScissorMode()
}

// keycap draws a keyboard key with its label and returns its width.
func (gfx *Graphics) keycap(label string, x, y, size float32) float32 {
	u := uiScale()
	w := rl.MeasureTextEx(gfx.mono, label, size, 0).X + 12*u
	h := size + 7*u
	fillBevel(rl.NewRectangle(x, y+2*u, w, h), 3*u, color.RGBA{110, 114, 126, 255})
	fillBevel(rl.NewRectangle(x, y, w, h-1*u), 3*u, uiWhite)
	rl.DrawTextEx(gfx.mono, label, rl.NewVector2(x+6*u, y+3*u), size, 0, uiInk)
	return w
}

// prompt is a centered visor message, optionally led by a key to press.
func (gfx *Graphics) prompt(key, msg string, y float32, c color.RGBA) {
	u := uiScale()
	w, _ := screenSize()
	size := 20 * u
	kw := float32(0)
	if key != "" {
		kw = rl.MeasureTextEx(gfx.mono, key, size*0.75, 0).X + 24*u
	}
	tw := gfx.width(msg, size)
	total := kw + tw
	visorPanel(rl.NewRectangle(w/2-total/2-22*u, y-10*u, total+44*u, size+20*u), c)
	x := w/2 - total/2
	if key != "" {
		gfx.keycap(key, x, y, size*0.75)
	}
	gfx.text(msg, x+kw, y, size, c)
}

// icon draws a small flat pictogram centered on (cx, cy), s being its
// half size: the shop's tools and upgrades.
func icon(kind string, cx, cy, s float32, c color.RGBA) {
	u := uiScale()
	v := func(x, y float32) rl.Vector2 { return rl.NewVector2(cx+x*s, cy+y*s) }
	rect := func(x0, y0, x1, y1 float32, col color.RGBA) {
		rl.DrawRectangleRec(rl.NewRectangle(cx+x0*s, cy+y0*s, (x1-x0)*s, (y1-y0)*s), col)
	}
	switch kind {
	case "tool": // a push broom
		rl.DrawLineEx(v(0.55, -1), v(0.05, 0.15), 2.6*u, c)
		tri(v(-0.75, 0.1), v(0.75, 0.1), v(0.75, 0.42), c)
		tri(v(-0.75, 0.1), v(0.75, 0.42), v(-0.75, 0.42), c)
		for i := range 6 {
			x := -0.65 + float32(i)*0.26
			rl.DrawLineEx(v(x, 0.5), v(x-0.08, 1), 1.6*u, c)
		}
	case "tank": // a canister, two thirds full
		rect(-0.25, -1, 0.25, -0.72, c)
		rl.DrawRectangleRounded(rl.NewRectangle(cx-0.6*s, cy-0.75*s, 1.2*s, 1.75*s), 0.3, 6, c)
		rect(-0.38, -0.5, 0.38, 0.05, uiInk)
	case "power": // a lightning bolt
		tri(v(0.35, -1), v(-0.55, 0.2), v(0.15, 0.05), c)
		tri(v(-0.15, -0.05), v(0.55, -0.2), v(-0.35, 1), c)
	case "price": // a coin marked with a C
		rl.DrawCircleV(v(0, 0), 0.95*s, c)
		rl.DrawRing(v(0, 0), 0.32*s, 0.55*s, 50, 310, 24, uiInk)
	case "drone": // a quadcopter seen from above
		for _, d := range [][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
			rl.DrawLineEx(v(0, 0), v(d[0]*0.62, d[1]*0.62), 2.2*u, c)
			rl.DrawRing(v(d[0]*0.62, d[1]*0.62), 0.2*s, 0.36*s, 0, 360, 20, c)
		}
		rl.DrawCircleV(v(0, 0), 0.3*s, c)
	case "auto": // a teleporter pad: rings and a beam
		rl.DrawRing(v(0, 0.55), 0.7*s, 0.95*s, 0, 360, 32, c)
		rect(-0.18, -1, 0.18, 0.4, c)
		tri(v(-0.5, -0.45), v(0.5, -0.45), v(0, -1.05), c)
	case "vehicle": // a rover from the side: body, cab, two wheels
		rect(-0.95, -0.1, 0.95, 0.35, c)
		rect(-0.3, -0.6, 0.45, -0.1, c)
		rect(-0.15, -0.48, 0.3, -0.2, uiInk)
		for _, x := range []float32{-0.55, 0.55} {
			rl.DrawCircleV(v(x, 0.5), 0.38*s, uiInk)
			rl.DrawCircleV(v(x, 0.5), 0.28*s, c)
		}
	case "flag": // a pole and a pennant
		rect(-0.55, -1, -0.4, 1, c)
		tri(v(-0.4, -1), v(-0.4, 0.05), v(0.85, -0.47), c)
	default: // anything else: a crate
		rl.DrawRectangleLinesEx(rl.NewRectangle(cx-0.75*s, cy-0.75*s, 1.5*s, 1.5*s), 2.2*u, c)
		rl.DrawLineEx(v(-0.75, -0.75), v(0.75, 0.75), 2*u, c)
	}
}

// shopPanel and the item rows are shared by drawing and mouse handling.
func shopPanel() rl.Rectangle {
	u := uiScale()
	w, h := screenSize()
	return rl.NewRectangle((w-700*u)/2, (h-470*u)/2, 700*u, 470*u)
}

// shopRows is how many rows the shop shows at once; the wheel scrolls.
const shopRows = 6

func shopItemRect(i int) rl.Rectangle {
	p, u := shopPanel(), uiScale()
	return rl.NewRectangle(p.X+18*u, p.Y+72*u+float32(i)*64*u, p.Width-36*u, 58*u)
}

func (g *Game) drawHUD(cam rl.Camera3D) {
	gfx, u := g.gfx, uiScale()
	_, h := screenSize()

	for _, f := range g.floaters {
		if f.Pos.Sub(g.player.Eye()).Dot(g.player.Forward()) <= 0 {
			continue // behind the camera
		}
		sp := rl.GetWorldToScreen(toRL(f.Pos), cam)
		gfx.textCenter(f.Text, sp.X+1*u, sp.Y+2*u, 26*u, withAlpha(color.RGBA{0, 0, 0, 170}, f.Life))
		gfx.textCenter(f.Text, sp.X, sp.Y, 26*u, withAlpha(f.C, f.Life))
	}

	if g.mode == modePlay || g.mode == modeMap || g.mode == modeShop {
		g.drawVisor()
	}

	if g.mode == modePlay {
		g.drawBaseMarker(cam)
		g.drawCompass()
		g.drawReticle()
		g.drawHeldItem()
		switch {
		case g.driving >= 0:
			v, _ := g.driven()
			gfx.prompt("E", "Descendre du "+strings.ToLower(v.Name), h*0.64, visorCyan)
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
		g.drawHints()
		if g.messageT > 0 {
			gfx.prompt("", g.message, h*0.2, withAlpha(visorCyan, math.Min(1, g.messageT)))
		}
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
}

// drawVisor lays the helmet's visor over the view.
func (g *Game) drawVisor() {
	w, h := screenSize()
	src := rl.NewRectangle(0, 0, float32(g.gfx.visor.Width), float32(g.gfx.visor.Height))
	rl.DrawTexturePro(g.gfx.visor, src, rl.NewRectangle(0, 0, w, h), rl.NewVector2(0, 0), 0, rl.White)
}

// drawHints lists the controls along the bottom of the visor, as keys.
// drawHeldItem: what F places, under the status panel.
func (g *Game) drawHeldItem() {
	it, n := g.heldItem()
	if it == nil {
		return
	}
	gfx, u := g.gfx, uiScale()
	x, y := 40*u, 158*u
	label := fmt.Sprintf("%s ×%d", it.Name, n)
	w := gfx.width(label, 16*u) + 150*u
	visorPanel(rl.NewRectangle(x, y, w, 34*u), visorCyan)
	icon(orDefault(it.Icon, "flag"), x+18*u, y+17*u, 9*u, visorCyan)
	gfx.text(label, x+36*u, y+8*u, 16*u, uiWhite)
	kx := x + 44*u + gfx.width(label, 16*u)
	kx += gfx.keycap("F", kx, y+9*u, 11*u) + 4*u
	gfx.caps("poser", kx, y+11*u, 12*u, visorDim)
	kx += gfx.capsWidth("poser", 12*u) + 10*u
	kx += gfx.keycap("R", kx, y+9*u, 11*u) + 4*u
	gfx.caps("changer", kx, y+11*u, 12*u, visorDim)
}

func (g *Game) drawHints() {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	hints := [][2]string{
		{"ZQSD", "bouger"}, {"MAJ", "courir"}, {"ESPACE", "sauter"}, {"CLIC", "balayer"},
		{"E", "utiliser"}, {"ÉCHAP", "pause"}, {"F3", "debug"},
	}
	key, label := 11*u, 12*u
	total := float32(0)
	for _, k := range hints {
		total += rl.MeasureTextEx(gfx.mono, k[0], key, 0).X + 12*u + 6*u + gfx.capsWidth(k[1], label) + 18*u
	}
	x, y := w/2-(total-18*u)/2, h-36*u
	for _, k := range hints {
		x += gfx.keycap(k[0], x, y, key) + 6*u
		gfx.caps(k[1], x, y+2*u, label, withAlpha(uiWhite, 0.8))
		x += gfx.capsWidth(k[1], label) + 18*u
	}
}

// drawReticle: a thin ring with four ticks; brighter when the ground is in reach.
func (g *Game) drawReticle() {
	u := uiScale()
	w, h := screenSize()
	cx, cy := w/2, h/2
	c := withAlpha(uiWhite, 0.45)
	r := 9 * u
	if g.aimOK {
		c = withAlpha(visorCyan, 0.95)
		r = 7 * u
	}
	rl.DrawRing(rl.NewVector2(cx, cy), r-1.2*u, r, 0, 360, 32, c)
	for _, d := range [][2]float32{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
		rl.DrawLineEx(rl.NewVector2(cx+d[0]*(r+3*u), cy+d[1]*(r+3*u)), rl.NewVector2(cx+d[0]*(r+8*u), cy+d[1]*(r+8*u)), 1.5*u, c)
	}
	rl.DrawCircleV(rl.NewVector2(cx, cy), 1.2*u, c)
}

// drawStatus: credits, tool and tank, top left of the visor.
func (g *Game) drawStatus() {
	gfx, u := g.gfx, uiScale()
	r := rl.NewRectangle(32*u, 28*u, 300*u, 112*u)
	visorPanel(r, colGold)
	left, right, y := r.X+20*u, r.X+r.Width-18*u, r.Y

	// Credits, with a little coin.
	gfx.caps("CRÉDITS", left, y+12*u, 13*u, visorDim)
	coin := rl.NewVector2(left+8*u, y+46*u)
	rl.DrawCircleV(coin, 8*u, colGold)
	rl.DrawRing(coin, 3*u, 5*u, 50, 310, 16, color.RGBA{150, 105, 30, 255})
	vw := gfx.monoAt(fmtNum(g.st.Credits), left+24*u, y+30*u, 30*u, colGold)
	if g.sellPop > 0 {
		rise := float32(1.5-g.sellPop) * 10 * u
		gfx.monoAt(g.sellPopText, left+30*u+vw, y+38*u-rise, 16*u, withAlpha(colGold, g.sellPop))
	}

	// Tool, with its tier as pips.
	gfx.capsRight("OUTIL", right, y+12*u, 13*u, visorDim)
	gfx.textRight(g.tool().Name, right, y+29*u, 17*u, uiWhite)
	n := content.Tools.Len()
	for i := range n {
		c := uiTrack
		if i <= g.toolTier() {
			c = visorCyan
		}
		x := right - float32(n-i)*10*u + 2*u
		fillCut(rl.NewRectangle(x, y+53*u, 8*u, 4*u), 1.5*u, c)
	}

	// Tank: a gauge with quarter ticks.
	ty := y + 70*u
	gfx.caps("RÉSERVOIR", left, ty, 13*u, visorDim)
	gfx.monoRight(fmtNum(g.st.Tank)+" / "+fmtNum(g.capacity())+" g", right, ty-1*u, 14*u, uiWhite)
	bar := rl.NewRectangle(left, ty+19*u, right-left, 9*u)
	fillCut(bar, 3*u, uiTrack)
	fill := float32(mathx.Clamp01(g.st.Tank / g.capacity()))
	c := visorCyan
	if fill > 0.8 {
		c = visorAmber
	}
	if g.tankFull() && math.Mod(g.clock, 0.5) < 0.25 {
		c = colRed
	}
	if fw := bar.Width * fill; fw > 1 {
		fillCut(rl.NewRectangle(bar.X, bar.Y, fw, bar.Height), min(3*u, fw/2), c)
	}
	for i := 1; i < 4; i++ {
		rl.DrawRectangleRec(rl.NewRectangle(bar.X+bar.Width*float32(i)/4, bar.Y, 1.5*u, bar.Height), uiInk)
	}
}

// drawCompass: a heading strip at the top of the visor, north being the
// way to the base (it stands on the north pole).
func (g *Game) drawCompass() {
	gfx, u := g.gfx, uiScale()
	w, _ := screenSize()
	up := g.player.LocalUp()
	north := planet.BaseUp.Sub(up.Scale(planet.BaseUp.Dot(up)))
	if north.Length() < 1e-6 {
		return // standing on the pole itself
	}
	north = north.Norm()
	east := north.Cross(up)
	heading := math.Atan2(g.player.Heading.Dot(east), g.player.Heading.Dot(north)) * 180 / math.Pi

	cx, top, half := w/2, 24*u, 220*u
	// A dark band fading out at both ends, and a baseline.
	shade := color.RGBA{10, 10, 14, 150}
	fade := 60 * u
	rl.DrawRectangleGradientH(int32(cx-half-fade), int32(top), int32(fade), int32(34*u), color.RGBA{}, shade)
	rl.DrawRectangleRec(rl.NewRectangle(cx-half, top, 2*half, 34*u), shade)
	rl.DrawRectangleGradientH(int32(cx+half), int32(top), int32(fade), int32(34*u), shade, color.RGBA{})
	labels := map[int]string{0: "N", 90: "E", 180: "S", 270: "O", 45: "NE", 135: "SE", 225: "SO", 315: "NO"}
	for a := 0; a < 360; a += 15 {
		rel := math.Mod(float64(a)-heading+540, 360) - 180
		if math.Abs(rel) > 62 {
			continue
		}
		x := cx + float32(rel/62)*half
		alpha := 1 - mathx.Smoothstep(40, 62, math.Abs(rel))
		switch l, ok := labels[a]; {
		case ok && a%90 == 0:
			c := uiWhite
			if a == 0 {
				c = colGold // the base
			}
			gfx.capsCenter(l, x, top+4*u, 18*u, withAlpha(c, alpha))
			rl.DrawRectangleRec(rl.NewRectangle(x-1*u, top+25*u, 2*u, 9*u), withAlpha(c, alpha))
		case ok:
			gfx.capsCenter(l, x, top+8*u, 12*u, withAlpha(uiMuted, alpha))
			rl.DrawRectangleRec(rl.NewRectangle(x-0.75*u, top+27*u, 1.5*u, 7*u), withAlpha(uiMuted, alpha))
		default:
			rl.DrawRectangleRec(rl.NewRectangle(x-0.75*u, top+29*u, 1.5*u, 5*u), withAlpha(uiFaint, alpha))
		}
	}
	tri(rl.NewVector2(cx, top+30*u), rl.NewVector2(cx-6*u, top+38*u), rl.NewVector2(cx+6*u, top+38*u), visorCyan)
	if dist := planet.ArcDist(up, planet.BaseUp); dist > 15 {
		s := "BASE  " + fmtNum(dist) + " M"
		sw := gfx.capsWidth(s, 13*u)
		chip := rl.NewRectangle(cx-sw/2-18*u, top+42*u, sw+30*u, 20*u)
		fillCut(chip, 5*u, uiPlate)
		rl.DrawPoly(rl.NewVector2(chip.X+10*u, chip.Y+10*u), 4, 4*u, 0, colGold)
		gfx.caps(s, chip.X+20*u, chip.Y+3*u, 13*u, colGold)
	}
}

// drawBaseMarker shows where the base is when it is in view.
func (g *Game) drawBaseMarker(cam rl.Camera3D) {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	if planet.ArcDist(g.player.LocalUp(), planet.BaseUp) < 25 {
		return
	}
	target := basePos.Add(vec3{0, 9, 0})
	if target.Sub(g.player.Eye()).Dot(g.player.Forward()) <= 0 {
		return
	}
	sp := rl.GetWorldToScreen(toRL(target), cam)
	if sp.X < 0 || sp.X > w || sp.Y < 0 || sp.Y > h {
		return
	}
	rl.DrawPolyLinesEx(sp, 4, 8*u, 0, 2*u, colGold)
	rl.DrawPoly(sp, 4, 3*u, 0, colGold)
	gfx.capsCenter("BASE", sp.X+1*u, sp.Y+13*u, 13*u, color.RGBA{0, 0, 0, 140})
	gfx.capsCenter("BASE", sp.X, sp.Y+12*u, 13*u, colGold)
}

// marsEmblem paints a little flat Mars: bands of rust and ochre, a white
// polar cap and the night side in violet, row by row.
func marsEmblem(cx, cy, r float32) {
	bands := []color.RGBA{
		{206, 112, 66, 255}, {194, 98, 58, 255}, {214, 124, 74, 255}, {186, 90, 54, 255}, {200, 106, 62, 255},
	}
	night := func(c color.RGBA) color.RGBA {
		return color.RGBA{uint8(float32(c.R)*0.32 + 22), uint8(float32(c.G)*0.28 + 12), uint8(float32(c.B)*0.4 + 30), 255}
	}
	for y := -r; y < r; y++ {
		my := y + 0.5
		half := float32(math.Sqrt(float64(r*r - my*my)))
		lat := my / r
		// Wavy bands, like the dark and light regions seen from orbit.
		c := bands[min(len(bands)-1, max(0, int((lat+1+0.12*float32(math.Sin(float64(lat*9))))/2*float32(len(bands)))))]
		if lat < -0.78 {
			c = color.RGBA{240, 234, 228, 255}
		}
		term := half * 0.42 // the terminator, an ellipse: the night is on the right
		rl.DrawRectangleRec(rl.NewRectangle(cx-half, cy+y, half+term, 1), c)
		rl.DrawRectangleRec(rl.NewRectangle(cx+term, cy+y, half-term, 1), night(c))
	}
	rl.DrawRing(rl.NewVector2(cx, cy), r+2*uiScale(), r+4*uiScale(), 200, 340, 48, color.RGBA{255, 170, 120, 120})
}

func (g *Game) drawPause() {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	rl.DrawRectangleGradientV(0, 0, int32(w), int32(h), color.RGBA{8, 4, 10, 215}, color.RGBA{20, 8, 10, 235})
	g.drawVisor()

	// Emblem and title, centered as one block.
	title := "MARS SWEEPER"
	size := 76 * u
	tw := gfx.titleWidth(title, size)
	er := 46 * u
	x0 := w/2 - (2*er+28*u+tw)/2
	ty := h * 0.16
	marsEmblem(x0+er, ty+size*0.5+6*u, er)
	tx := x0 + 2*er + 28*u
	gfx.title(title, tx+3*u, ty+4*u, size, color.RGBA{0, 0, 0, 150})
	gfx.title(title, tx, ty, size, uiWhite)
	hazard(rl.NewRectangle(tx+2*u, ty+size+4*u, tw, 7*u))
	if math.Mod(g.clock, 1.4) < 1.0 {
		gfx.caps("CLIQUE POUR ENFILER TON CASQUE", tx+2*u, ty+size+22*u, 18*u, visorCyan)
	}

	keys := [][2]string{
		{"ZQSD", "se déplacer"}, {"MAJ", "courir"}, {"ESPACE", "sauter"}, {"F11", "plein écran"},
		{"CLIC", "balayer la poussière visée"}, {"E", "incinérateur, labo, carte, lit"}, {"ÉCHAP", "pause"},
	}
	box := rl.NewRectangle(w/2-330*u, h*0.53, 660*u, 4*38*u+50*u)
	visorPanel(box, visorCyan)
	gfx.caps("COMMANDES", box.X+24*u, box.Y+14*u, 13*u, visorDim)
	for i, k := range keys {
		col := float32(i / 4)
		x := box.X + 24*u + col*300*u
		y := box.Y + 42*u + float32(i%4)*38*u
		kw := rl.MeasureTextEx(gfx.mono, k[0], 15*u, 0).X + 12*u
		gfx.keycap(k[0], x+70*u-kw, y, 15*u)
		gfx.text(k[1], x+84*u, y+1*u, 19*u, uiWhite)
	}
}

// terminal draws a base computer screen: a charcoal body under an
// off-white header, like the habitat panels, with an accent square, a
// hazard block at its end and an accent line beneath.
func (g *Game) terminal(p rl.Rectangle, title string, accent color.RGBA) {
	gfx, u := g.gfx, uiScale()
	w, h := screenSize()
	rl.DrawRectangleGradientV(0, 0, int32(w), int32(h), color.RGBA{0, 0, 0, 90}, color.RGBA{0, 0, 0, 170})
	cut := 16 * u
	fillCut(rl.NewRectangle(p.X+5*u, p.Y+7*u, p.Width, p.Height), cut, color.RGBA{0, 0, 0, 110})
	fillCut(p, cut, color.RGBA{22, 24, 31, 246})
	head := rl.NewRectangle(p.X, p.Y, p.Width, 52*u)
	fillShape(chamfer(head, cut, 0, 0, 0), uiWhite)
	hazard(rl.NewRectangle(p.X+p.Width-56*u, p.Y, 56*u, 52*u))
	rl.DrawRectangleRec(rl.NewRectangle(p.X, p.Y+52*u, p.Width, 3*u), accent)
	fillCut(rl.NewRectangle(p.X+22*u, p.Y+15*u, 12*u, 22*u), 3*u, uiInk)
	gfx.title(title, p.X+46*u, p.Y+13*u, 25*u, uiInk)
}

func (g *Game) drawShop() {
	gfx, u := g.gfx, uiScale()
	p := shopPanel()
	g.terminal(p, "LABORATOIRE DE RECHERCHE", visorCyan)
	right := p.X + p.Width - 72*u
	vw := rl.MeasureTextEx(gfx.mono, fmtNum(g.st.Credits)+" cr", 24*u, 0).X
	coin := rl.NewVector2(right-vw-16*u, p.Y+26*u)
	rl.DrawCircleV(coin, 9*u, colGold)
	rl.DrawRing(coin, 3.5*u, 5.5*u, 50, 310, 16, color.RGBA{150, 105, 30, 255})
	gfx.monoRight(fmtNum(g.st.Credits)+" cr", right, p.Y+13*u, 24*u, uiInk)

	items := g.shopItems()
	for i, it := range items {
		if i < g.shopScroll || i >= g.shopScroll+shopRows {
			continue
		}
		r := shopItemRect(i - g.shopScroll)
		bg := uiRow
		if i == g.hover {
			bg = uiRowLit
		}
		fillCut(r, 8*u, bg)
		state, costCol := uiFaint, uiCostOff
		if it.affordable(g.st.Credits) {
			state, costCol = visorCyan, colGold
		}
		if it.maxed {
			state = colGold
		}
		if i == g.hover {
			rl.DrawRectangleRec(rl.NewRectangle(r.X, r.Y+8*u, 3*u, r.Height-8*u), state)
		}
		tile := rl.NewRectangle(r.X+8*u, r.Y+7*u, 44*u, 44*u)
		fillCut(tile, 6*u, uiInk)
		icon(it.icon, tile.X+22*u, tile.Y+22*u, 14*u, state)
		gfx.text(it.name, r.X+66*u, r.Y+7*u, 20*u, uiWhite)
		gfx.text(it.desc, r.X+66*u, r.Y+32*u, 15*u, uiMuted)
		right := r.X + r.Width - 16*u
		if it.maxed {
			cw := gfx.capsWidth("MAX", 15*u)
			fillCut(rl.NewRectangle(right-cw-14*u, r.Y+8*u, cw+14*u, 22*u), 5*u, colGold)
			gfx.caps("MAX", right-cw-7*u, r.Y+11*u, 15*u, uiInk)
		} else {
			gfx.monoRight(fmtNum(it.cost)+" cr", right, r.Y+8*u, 20*u, costCol)
		}
		gfx.capsRight(strings.ToUpper(it.info), right, r.Y+35*u, 12*u, visorDim)
	}
	// A scroll bar when everything doesn't fit.
	if len(items) > shopRows {
		top, bottom := shopItemRect(0), shopItemRect(shopRows-1)
		track := rl.NewRectangle(p.X+p.Width-12*u, top.Y, 4*u, bottom.Y+bottom.Height-top.Y)
		rl.DrawRectangleRec(track, uiTrack)
		h := track.Height * shopRows / float32(len(items))
		y := track.Y + (track.Height-h)*float32(g.shopScroll)/float32(len(items)-shopRows)
		rl.DrawRectangleRec(rl.NewRectangle(track.X, y, track.Width, h), visorCyan)
	}
	g.closeHint(p.X+p.Width/2, p.Y+p.Height+16*u, true)
}

// closeHint says how to leave a terminal, under it.
func (g *Game) closeHint(x, y float32, centered bool) {
	gfx, u := g.gfx, uiScale()
	s := "OU ÉCHAP POUR FERMER"
	kw := rl.MeasureTextEx(gfx.mono, "E", 14*u, 0).X + 12*u
	if centered {
		x -= (kw + 8*u + gfx.capsWidth(s, 13*u)) / 2
	}
	gfx.keycap("E", x, y, 14*u)
	gfx.caps(s, x+kw+8*u, y+3*u, 13*u, withAlpha(uiWhite, 0.8))
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
	p := rl.NewRectangle(40*u, h/2-190*u, 440*u, 380*u)
	g.terminal(p, "ÉTAT DE MARS", visorCyan)

	rows := []struct{ label, value string }{
		{"MARS NETTOYÉE", fmt.Sprintf("%.4f %%", s.Cleaned*100)},
		{"POUSSIÈRE COLLECTÉE", fmtNum(g.st.TotalDust) + " g"},
		{"ZONES VISITÉES", fmt.Sprintf("%d / %d", s.TilesVisited, s.TilesTotal)},
		{"CRISTAUX TROUVÉS", fmt.Sprintf("%d / %d", s.Gems, s.GemTotal)},
		{"CRÉDITS GAGNÉS", fmtNum(g.st.TotalEarned) + " cr"},
		{"DISTANCE DE LA BASE", fmtNum(planet.ArcDist(g.player.LocalUp(), planet.BaseUp)) + " m"},
	}
	for i, r := range rows {
		y := p.Y + 74*u + float32(i)*42*u
		if i%2 == 0 {
			rl.DrawRectangleRec(rl.NewRectangle(p.X+14*u, y-8*u, p.Width-28*u, 38*u), color.RGBA{255, 255, 255, 8})
		}
		gfx.caps(r.label, p.X+26*u, y+4*u, 14*u, visorDim)
		gfx.monoRight(r.value, p.X+p.Width-26*u, y+1*u, 19*u, uiWhite)
	}
	y := p.Y + p.Height - 38*u
	legend := []struct {
		c     color.RGBA
		label string
	}{{colGold, "BASE"}, {color.RGBA{255, 90, 70, 255}, "TOI"}, {color.RGBA{190, 255, 255, 255}, "NETTOYÉ"}}
	x := p.X + 28*u
	for _, l := range legend {
		rl.DrawPoly(rl.NewVector2(x, y+8*u), 4, 6*u, 0, l.c)
		gfx.caps(l.label, x+12*u, y+1*u, 14*u, uiMuted)
		x += gfx.capsWidth(l.label, 14*u) + 40*u
	}
	g.closeHint(p.X, p.Y+p.Height+16*u, false)
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
	rl.DrawRectangle(0, 0, int32(w), int32(h), color.RGBA{4, 2, 10, uint8(255 * mathx.Clamp01(dark))})
	if t > sleepFadeOut*0.6 && t < sleepWake+0.3 {
		a := math.Min(t-sleepFadeOut*0.6, sleepWake+0.3-t) * 2
		dots := strings.Repeat(" z", 1+int(t*2)%3)
		gfx.capsCenter("Z"+dots+"…", w/2, h/2-30*u, 44*u, withAlpha(color.RGBA{180, 170, 255, 255}, a))
		if t > sleepSave {
			gfx.capsCenter("PARTIE SAUVEGARDÉE", w/2, h/2+30*u, 16*u, withAlpha(uiMuted, a))
		}
	}
}
