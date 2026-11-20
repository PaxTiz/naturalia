package ui

import (
	"fmt"
	"image/color"
	"math"
	"naturalia/internal/game"
	"naturalia/internal/mathx"
	"naturalia/internal/palette"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The map table's terminal has two tabs: the state of Mars, and travel to
// the base or a bootcamp (docs/TELEPORTEURS.md).

var mapTabs = []string{"ÉTAT DE MARS", "VOYAGER"}

// travelRows are the destinations shown at once; the rest scroll.
const travelRows = 6

// mapPanel is the map table's terminal, left of the hologram.
func mapPanel() rl.Rectangle {
	u := uiScale()
	_, h := screenSize()
	return rl.NewRectangle(40*u, h/2-215*u, 460*u, 430*u)
}

// mapTabRect is tab i's button, under the terminal's header.
func mapTabRect(i int) rl.Rectangle {
	u := uiScale()
	p := mapPanel()
	w := (p.Width - 28*u - 8*u) / float32(len(mapTabs))
	return rl.NewRectangle(p.X+14*u+float32(i)*(w+8*u), p.Y+66*u, w, 30*u)
}

// travelRowRect is the destination row i places down the list shown.
func travelRowRect(i int) rl.Rectangle {
	u := uiScale()
	p := mapPanel()
	return rl.NewRectangle(p.X+14*u, p.Y+108*u+float32(i)*48*u, p.Width-28*u, 44*u)
}

// updateMap switches the map table's tabs (a click, left and right),
// scrolls the destinations, follows the mouse over them and travels to
// the one clicked.
func (hud *HUD) updateMap(g *game.Game) {
	mouse := rl.GetMousePosition()
	click := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	switch {
	case rl.IsKeyPressed(rl.KeyLeft), game.KeyPressed(game.ActLeft):
		g.MapTab = (g.MapTab + len(mapTabs) - 1) % len(mapTabs)
	case rl.IsKeyPressed(rl.KeyRight), game.KeyPressed(game.ActRight):
		g.MapTab = (g.MapTab + 1) % len(mapTabs)
	}
	for i := range mapTabs {
		if click && rl.CheckCollisionPointRec(mouse, mapTabRect(i)) {
			g.MapTab, hud.mapScroll = i, 0
			click = false
		}
	}
	g.MapHover = -1
	if g.MapTab != 1 {
		return
	}
	dests := g.Destinations()
	wheel := rl.GetMouseWheelMove()
	if wheel < 0 || rl.IsKeyPressed(rl.KeyDown) {
		hud.mapScroll++
	} else if wheel > 0 || rl.IsKeyPressed(rl.KeyUp) {
		hud.mapScroll--
	}
	hud.mapScroll = max(0, min(hud.mapScroll, len(dests)-travelRows))
	for i := hud.mapScroll; i < min(len(dests), hud.mapScroll+travelRows); i++ {
		if rl.CheckCollisionPointRec(mouse, travelRowRect(i-hud.mapScroll)) {
			g.MapHover = i
		}
	}
	if click && g.MapHover >= 0 {
		g.Travel(dests[g.MapHover])
	}
}

// drawMapPanel shows the map table's terminal next to the hologram: the
// state of Mars, or where to travel.
func (hud *HUD) drawMapPanel(g *game.Game) {
	u := uiScale()
	p := mapPanel()
	hud.terminal(g, p, "TABLE DE CARTOGRAPHIE", visorCyan)
	mouse := rl.GetMousePosition()
	for i, name := range mapTabs {
		r := mapTabRect(i)
		bg, col := uiRow, uiMuted
		switch {
		case i == g.MapTab:
			bg, col = visorCyan, uiInk
		case rl.CheckCollisionPointRec(mouse, r):
			bg, col = uiRowLit, uiWhite
		}
		fillCut(r, 6*u, bg)
		hud.capsCenter(name, r.X+r.Width/2, r.Y+8*u, 14*u, col)
	}
	if g.MapTab == 1 {
		hud.drawTravel(g, p)
	} else {
		hud.drawMarsState(g, p)
	}
	hud.closeHint(g, p.X, p.Y+p.Height+16*u, false)
}

// drawTravel lists where to travel: the base, then the bootcamps, how far
// and which way, and what it costs.
func (hud *HUD) drawTravel(g *game.Game, p rl.Rectangle) {
	u := uiScale()
	dests := g.Destinations()
	for i, d := range dests {
		if i < hud.mapScroll || i >= hud.mapScroll+travelRows {
			continue
		}
		r := travelRowRect(i - hud.mapScroll)
		bg := uiRow
		if i == g.MapHover {
			bg = uiRowLit
		}
		fillCut(r, 7*u, bg)
		mark := visorCyan
		if d.Camp == 0 {
			mark = palette.Gold
		}
		rl.DrawPoly(rl.NewVector2(r.X+18*u, r.Y+r.Height/2), 4, 6*u, 0, mark)
		if i == g.MapHover {
			rl.DrawRectangleRec(rl.NewRectangle(r.X, r.Y+7*u, 3*u, r.Height-7*u), mark)
		}
		right := r.X + r.Width - 14*u
		hud.text(hud.fit(d.Name, 18*u, r.Width*0.5), r.X+34*u, r.Y+5*u, 18*u, uiWhite)
		where := fmt.Sprintf("%s m · %s", game.FmtNum(d.Dist), d.Bearing)
		if d.Here {
			where = "ici"
		}
		hud.caps(where, r.X+34*u, r.Y+27*u, 12*u, visorDim)
		switch {
		case d.Here:
			hud.capsRight("TU Y ES", right, r.Y+15*u, 14*u, visorDim)
		case g.CanTravel(d):
			hud.monoRight(game.FmtNum(game.TravelCost)+" cr", right, r.Y+12*u, 18*u, palette.Gold)
		default:
			hud.monoRight(game.FmtNum(game.TravelCost)+" cr", right, r.Y+12*u, 18*u, uiCostOff)
		}
	}
	if len(dests) > travelRows { // a scroll bar
		top, bottom := travelRowRect(0), travelRowRect(travelRows-1)
		track := rl.NewRectangle(p.X+p.Width-10*u, top.Y, 3*u, bottom.Y+bottom.Height-top.Y)
		rl.DrawRectangleRec(track, uiTrack)
		h := track.Height * travelRows / float32(len(dests))
		y := track.Y + (track.Height-h)*float32(hud.mapScroll)/float32(len(dests)-travelRows)
		rl.DrawRectangleRec(rl.NewRectangle(track.X, y, track.Width, h), visorCyan)
	}
	note := "UN CLIC POUR Y ALLER, À PIED"
	if len(dests) == 1 {
		note = "POSE DES BOOTCAMPS POUR Y VOYAGER"
	}
	hud.caps(note, p.X+28*u, p.Y+p.Height-36*u, 13*u, uiMuted)
}

// drawMarsState shows how far Mars is cleaned.
func (hud *HUD) drawMarsState(g *game.Game, p rl.Rectangle) {
	u := uiScale()
	s := g.World.Stats(g.State.TotalDust)
	rows := []struct{ label, value string }{
		{"MARS NETTOYÉE", fmt.Sprintf("%.4f %%", s.Cleaned*100)},
		{"POUSSIÈRE COLLECTÉE", game.FmtNum(g.State.TotalDust) + " g"},
		{"ZONES VISITÉES", fmt.Sprintf("%d / %d", s.TilesVisited, s.TilesTotal)},
		{"CRISTAUX TROUVÉS", fmt.Sprintf("%d / %d", s.Gems, s.GemTotal)},
		{"CRÉDITS GAGNÉS", game.FmtNum(g.State.TotalEarned) + " cr"},
		{"DISTANCE DE LA BASE", game.FmtNum(g.Destinations()[0].Dist) + " m"},
	}
	for i, r := range rows {
		y := p.Y + 120*u + float32(i)*42*u
		if i%2 == 0 {
			rl.DrawRectangleRec(rl.NewRectangle(p.X+14*u, y-8*u, p.Width-28*u, 38*u), color.RGBA{255, 255, 255, 8})
		}
		hud.caps(r.label, p.X+26*u, y+4*u, 14*u, visorDim)
		hud.monoRight(r.value, p.X+p.Width-26*u, y+1*u, 19*u, uiWhite)
	}
	y := p.Y + p.Height - 38*u
	legend := []struct {
		c     color.RGBA
		label string
	}{{palette.Gold, "BASE"}, {visorCyan, "BOOTCAMPS"}, {color.RGBA{255, 90, 70, 255}, "TOI"}, {color.RGBA{190, 255, 255, 255}, "NETTOYÉ"}}
	x := p.X + 28*u
	for _, l := range legend {
		rl.DrawPoly(rl.NewVector2(x, y+8*u), 4, 6*u, 0, l.c)
		hud.caps(l.label, x+12*u, y+1*u, 14*u, uiMuted)
		x += hud.capsWidth(l.label, 14*u) + 30*u
	}
}

// drawTravelFade washes the view to cyan, says where to, and clears again
// where the player has arrived.
func (hud *HUD) drawTravelFade(g *game.Game) {
	u := uiScale()
	w, h := screenSize()
	t := g.TravelT
	k := t / game.TravelOut
	if t > game.TravelJump {
		k = 1 - (t-game.TravelJump)/(game.TravelEnd-game.TravelJump)
	}
	k = mathx.Clamp01(k)
	k = k * k * (3 - 2*k)
	rl.DrawRectangle(0, 0, int32(w), int32(h), color.RGBA{120, 235, 255, uint8(235 * k)})
	// Lines streaming up, as the beam takes the player.
	for i := range 24 {
		x := float32(math.Mod(float64(i)*0.6180339*float64(w), float64(w)))
		y := float32(math.Mod((t*1.7+float64(i)*0.37)*float64(h), float64(h)))
		rl.DrawRectangleRec(rl.NewRectangle(x, h-y, 2*u, 60*u), color.RGBA{255, 255, 255, uint8(150 * k)})
	}
	hud.capsCenter("VOYAGE VERS "+strings.ToUpper(g.TravelingTo()), w/2, h/2-12*u, 24*u, withAlpha(uiInk, k))
}
