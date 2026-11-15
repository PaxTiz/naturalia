package ui

import (
	"fmt"
	"image/color"
	"math"
	"naturalia/internal/audio"
	"naturalia/internal/game"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The pause screen is a menu: play on, the settings, quit. The settings
// are a terminal with three tabs, the sound, the keys and the game; every
// row is reached by the arrows or the mouse.

// menuUI is the pause menu's state.
type menuUI struct {
	open     bool // the pause screen was open last frame
	settings bool // the settings page, else the main one
	sel      int  // the main page's button, or the tab's row
	tab      int
	scroll   int  // the tab's first row shown
	drag     int  // the slider row held by the mouse, or -1
	capture  bool // the selected key row waits for a key
	note     string
	noteT    float64
}

var menuButtons = [...]string{"REPRENDRE", "PARAMÈTRES", "QUITTER"}

const (
	tabSound = iota
	tabKeys
	tabGame
)

var menuTabs = [...]string{"SON", "TOUCHES", "JEU"}

// menuRows is how many rows a tab shows at once; the wheel scrolls.
const menuRows = 9

type rowKind int

const (
	rowHeader rowKind = iota
	rowSlider
	rowToggle
	rowKey
	rowButton
)

// menuRow is a line of a tab: a group's title, a slider (get and set,
// from lo to hi by step), a switch, a key or a button.
type menuRow struct {
	kind         rowKind
	label        string
	lo, hi, step float64
	get          func() float64
	set          func(float64)
	value        func() string
	on           func() bool
	press        func() // a switch flips, a button acts
	act          game.Action
}

// tabRows lists the open tab's rows.
func (hud *HUD) tabRows(g *game.Game) []menuRow {
	var rows []menuRow
	switch hud.menu.tab {
	case tabSound:
		for i, name := range game.SoundNames {
			rows = append(rows, menuRow{kind: rowSlider, label: name, hi: 1, step: 0.05,
				get: func() float64 { return g.Sound(i) },
				set: func(v float64) { g.SetSound(i, v) },
				value: func() string {
					if g.Sound(i) == 0 {
						return "coupé"
					}
					return fmt.Sprintf("%.0f %%", g.Sound(i)*100)
				}})
		}
	case tabKeys:
		group := ""
		for a, def := range game.ActionDefs {
			if def.Group != group {
				group = def.Group
				rows = append(rows, menuRow{kind: rowHeader, label: group})
			}
			rows = append(rows, menuRow{kind: rowKey, label: def.Name, act: game.Action(a)})
		}
		rows = append(rows, menuRow{kind: rowButton, label: "Rétablir les touches par défaut", press: func() {
			g.ResetKeys()
			hud.say("Touches par défaut rétablies")
		}})
	case tabGame:
		rows = append(rows,
			menuRow{kind: rowSlider, label: "Sensibilité de la souris", lo: game.MouseMin, hi: game.MouseMax, step: game.MouseStep,
				get:   func() float64 { return g.Settings.Mouse },
				set:   g.SetMouse,
				value: func() string { return strings.Replace(fmt.Sprintf("× %.2f", g.Settings.Mouse), ".", ",", 1) }},
			menuRow{kind: rowToggle, label: "Inverser l'axe vertical",
				on: func() bool { return g.Settings.InvertY }, press: func() { g.SetInvertY(!g.Settings.InvertY) }},
			menuRow{kind: rowToggle, label: "Plein écran",
				on: func() bool { return g.Settings.Fullscreen }, press: g.ToggleFullscreen},
		)
	}
	return rows
}

// say shows a short note at the bottom of the settings.
func (hud *HUD) say(s string) { hud.menu.note, hud.menu.noteT = s, 3 }

// The main page's buttons, under the title.
func menuButton(i int) rl.Rectangle {
	u := uiScale()
	w, h := screenSize()
	return rl.NewRectangle(w/2-190*u, h*0.45+float32(i)*64*u, 380*u, 54*u)
}

func settingsPanel() rl.Rectangle {
	u := uiScale()
	w, h := screenSize()
	return rl.NewRectangle((w-760*u)/2, (h-540*u)/2, 760*u, 540*u)
}

func settingsTab(i int) rl.Rectangle {
	p, u := settingsPanel(), uiScale()
	w := (p.Width - 36*u + 6*u) / float32(len(menuTabs))
	return rl.NewRectangle(p.X+18*u+float32(i)*w, p.Y+68*u, w-6*u, 30*u)
}

// settingsRow is the i-th row shown, and a slider's bar in it.
func settingsRow(i int) (row, bar rl.Rectangle) {
	p, u := settingsPanel(), uiScale()
	row = rl.NewRectangle(p.X+18*u, p.Y+112*u+float32(i)*42*u, p.Width-50*u, 38*u)
	bar = rl.NewRectangle(row.X+row.Width*0.48, row.Y+row.Height/2-3*u, row.Width*0.52-130*u, 6*u)
	return row, bar
}

// settingsBack is the footer's way back to the main page.
func settingsBack() rl.Rectangle {
	p, u := settingsPanel(), uiScale()
	return rl.NewRectangle(p.X+18*u, p.Y+p.Height-38*u, 150*u, 28*u)
}

// updateMenu follows the pause menu: the arrows, Enter and Escape, the
// mouse, and the key to bind when a key row waits for one.
func (hud *HUD) updateMenu(g *game.Game, dt float64) {
	m := &hud.menu
	m.noteT = math.Max(0, m.noteT-dt)
	if g.Mode != game.ModePaused {
		m.open, m.capture, m.drag = false, false, -1
		g.KeyCapture = false
		return
	}
	if !m.open {
		*m = menuUI{open: true, drag: -1}
	}
	// The key bound this frame must not act in the game too (game.Update).
	g.KeyCapture = m.capture
	mouse := rl.GetMousePosition()
	moved := rl.Vector2Length(rl.GetMouseDelta()) > 0
	click := rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	pressed := func(k int32) bool { return rl.IsKeyPressed(k) || rl.IsKeyPressedRepeat(k) }
	confirm := rl.IsKeyPressed(rl.KeyEnter) || rl.IsKeyPressed(rl.KeyKpEnter) || rl.IsKeyPressed(rl.KeySpace)

	if !m.settings {
		switch {
		case rl.IsKeyPressed(rl.KeyEscape):
			g.Resume()
			return
		case pressed(rl.KeyDown):
			m.sel = (m.sel + 1) % len(menuButtons)
		case pressed(rl.KeyUp):
			m.sel = (m.sel + len(menuButtons) - 1) % len(menuButtons)
		}
		hit := -1
		for i := range menuButtons {
			if rl.CheckCollisionPointRec(mouse, menuButton(i)) {
				hit = i
				if moved {
					m.sel = i
				}
			}
		}
		if confirm || click && hit >= 0 {
			if click {
				m.sel = hit
			}
			switch m.sel {
			case 0:
				g.Resume()
			case 1:
				m.settings, m.sel, m.scroll = true, 0, 0
				m.sel = hud.nextRow(g, -1, 1)
			case 2:
				g.Quit = true
			}
		}
		return
	}

	rows := hud.tabRows(g)
	if m.capture {
		for _, k := range g.KeysPressed {
			if k == rl.KeyEscape {
				m.capture = false
				break
			}
			if !game.Reserved(k) {
				hud.bind(g, rows[m.sel].act, k)
				m.capture = false
				break
			}
		}
		if click && m.capture { // a click elsewhere gives up
			m.capture = false
		}
		return
	}

	switchTab := func(t int) {
		m.tab, m.scroll, m.drag = t, 0, -1
		m.sel = hud.nextRow(g, -1, 1)
	}
	row := &rows[m.sel]
	switch {
	case rl.IsKeyPressed(rl.KeyEscape):
		m.settings, m.sel = false, 1
		g.SaveSettings()
		return
	case rl.IsKeyPressed(rl.KeyTab):
		step := 1
		if rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) {
			step = len(menuTabs) - 1
		}
		switchTab((m.tab + step) % len(menuTabs))
		return
	case pressed(rl.KeyDown):
		m.sel = hud.nextRow(g, m.sel, 1)
	case pressed(rl.KeyUp):
		m.sel = hud.nextRow(g, m.sel, -1)
	case pressed(rl.KeyRight) && row.kind == rowSlider:
		row.set(row.get() + row.step)
	case pressed(rl.KeyLeft) && row.kind == rowSlider:
		row.set(row.get() - row.step)
	case (pressed(rl.KeyRight) || pressed(rl.KeyLeft)) && row.kind == rowToggle:
		row.press()
	case confirm:
		hud.activate(g, row)
	}

	for i := range menuTabs {
		if click && rl.CheckCollisionPointRec(mouse, settingsTab(i)) && i != m.tab {
			switchTab(i)
			return
		}
	}
	if click && rl.CheckCollisionPointRec(mouse, settingsBack()) {
		m.settings, m.sel = false, 1
		g.SaveSettings()
		return
	}
	if w := rl.GetMouseWheelMove(); w != 0 {
		m.scroll -= int(math.Copysign(1, float64(w)))
	}
	m.scroll = max(0, min(m.scroll, len(rows)-menuRows))
	for i := m.scroll; i < min(len(rows), m.scroll+menuRows); i++ {
		r, _ := settingsRow(i - m.scroll)
		if rows[i].kind == rowHeader || m.drag >= 0 || !rl.CheckCollisionPointRec(mouse, r) {
			continue
		}
		if moved || click {
			m.sel = i
		}
		if !click {
			continue
		}
		if rows[i].kind == rowSlider {
			m.drag = i
		} else {
			hud.activate(g, &rows[i])
		}
	}
	if m.drag >= 0 {
		r := rows[m.drag]
		_, bar := settingsRow(m.drag - m.scroll)
		f := math.Max(0, math.Min(1, float64((mouse.X-bar.X)/bar.Width)))
		r.set(r.lo + f*(r.hi-r.lo))
		if !rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			m.drag = -1
		}
	}
	hud.showRow()
}

// OpenSettings shows the settings at a tab, for the screenshot tool.
func (hud *HUD) OpenSettings(tab int) {
	m := &hud.menu
	if !m.settings || m.tab != tab {
		m.open, m.settings, m.tab, m.scroll = true, true, tab%len(menuTabs), 0
		m.sel = 0
	}
}

// activate is Enter or a click on a row.
func (hud *HUD) activate(g *game.Game, r *menuRow) {
	switch r.kind {
	case rowToggle, rowButton:
		r.press()
	case rowKey:
		hud.menu.capture = true
	}
}

// bind gives the key to a, and says which action took a's old key.
func (hud *HUD) bind(g *game.Game, a game.Action, k int32) {
	var before [game.Actions]int32
	for b := range game.Actions {
		before[b] = game.KeyOf(b)
	}
	g.Bind(a, k)
	for b := range game.Actions {
		if b != a && game.KeyOf(b) != before[b] {
			hud.say(fmt.Sprintf("%s passe sur %s", game.ActionDefs[b].Name, game.KeyName(game.KeyOf(b))))
		}
	}
}

// nextRow is the next row from i that can be selected, going by dir;
// i itself at the ends.
func (hud *HUD) nextRow(g *game.Game, i, dir int) int {
	rows := hud.tabRows(g)
	for j := i + dir; j >= 0 && j < len(rows); j += dir {
		if rows[j].kind != rowHeader {
			return j
		}
	}
	return max(i, 0)
}

// showRow scrolls so the selected row shows, with its group's title.
func (hud *HUD) showRow() {
	m := &hud.menu
	if m.sel < m.scroll+1 {
		m.scroll = max(0, m.sel-1)
	}
	if m.sel >= m.scroll+menuRows {
		m.scroll = m.sel - menuRows + 1
	}
}

func (hud *HUD) drawPause(g *game.Game) {
	u := uiScale()
	w, h := screenSize()
	rl.DrawRectangleGradientV(0, 0, int32(w), int32(h), color.RGBA{8, 4, 10, 215}, color.RGBA{20, 8, 10, 235})
	hud.drawVisor(g)
	if hud.menu.settings {
		hud.drawSettings(g)
		return
	}

	// Emblem and title, centered as one block.
	title := "NATURALIA"
	size := 76 * u
	tw := hud.titleWidth(title, size)
	er := 46 * u
	x0 := w/2 - (2*er+28*u+tw)/2
	ty := h * 0.16
	marsEmblem(x0+er, ty+size*0.5+6*u, er)
	tx := x0 + 2*er + 28*u
	hud.title(title, tx+3*u, ty+4*u, size, color.RGBA{0, 0, 0, 150})
	hud.title(title, tx, ty, size, uiWhite)
	hazard(rl.NewRectangle(tx+2*u, ty+size+4*u, tw, 7*u))

	for i, label := range menuButtons {
		if i == 0 && g.WakingUp() {
			label = "ENFILER LE CASQUE"
		}
		r := menuButton(i)
		bg, col := uiPlate, uiWhite
		if i == hud.menu.sel {
			bg, col = visorCyan, uiInk
		}
		fillCut(rl.NewRectangle(r.X+4*u, r.Y+5*u, r.Width, r.Height), 12*u, color.RGBA{0, 0, 0, 90})
		fillCut(r, 12*u, bg)
		if i == hud.menu.sel {
			hazard(rl.NewRectangle(r.X+r.Width-34*u, r.Y, 22*u, r.Height))
		}
		hud.capsCenter(label, r.X+r.Width/2, r.Y+(r.Height-22*u)/2, 22*u, col)
	}
	hud.footer(w/2, menuButton(len(menuButtons)-1).Y+80*u, true, [][2]string{
		{"ÉCHAP", "reprendre"}, {"ENTRÉE", "choisir"},
	})
	cw := hud.width(audio.Credits, 15*u)
	hud.text(audio.Credits, w/2-cw/2, h-34*u, 15*u, visorDim)
}

// footer lines up keys and what they do, from x or centered on it.
func (hud *HUD) footer(x, y float32, centered bool, keys [][2]string) {
	u := uiScale()
	key, label := 11*u, 12*u
	if centered {
		total := float32(0)
		for _, k := range keys {
			total += rl.MeasureTextEx(hud.mono, k[0], key, 0).X + 12*u + 6*u + hud.capsWidth(k[1], label) + 18*u
		}
		x -= (total - 18*u) / 2
	}
	for _, k := range keys {
		x += hud.keycap(k[0], x, y, key) + 6*u
		hud.caps(k[1], x, y+2*u, label, withAlpha(uiWhite, 0.8))
		x += hud.capsWidth(k[1], label) + 18*u
	}
}

// drawSettings draws the settings' terminal: the tabs, the rows of the
// one open, the way back and what the keys do.
func (hud *HUD) drawSettings(g *game.Game) {
	u := uiScale()
	m := &hud.menu
	p := settingsPanel()
	hud.terminal(g, p, "PARAMÈTRES", visorCyan)
	mouse := rl.GetMousePosition()
	for i, name := range menuTabs {
		r := settingsTab(i)
		bg, col := uiRow, uiMuted
		switch {
		case i == m.tab:
			bg, col = visorCyan, uiInk
		case rl.CheckCollisionPointRec(mouse, r):
			bg, col = uiRowLit, uiWhite
		}
		fillCut(r, 6*u, bg)
		hud.capsCenter(name, r.X+r.Width/2, r.Y+(r.Height-14*u)/2, 14*u, col)
	}

	rows := hud.tabRows(g)
	for i := m.scroll; i < min(len(rows), m.scroll+menuRows); i++ {
		hud.drawRow(&rows[i], i == m.sel, i-m.scroll, g)
	}
	if len(rows) > menuRows {
		top, _ := settingsRow(0)
		bottom, _ := settingsRow(menuRows - 1)
		track := rl.NewRectangle(p.X+p.Width-22*u, top.Y, 4*u, bottom.Y+bottom.Height-top.Y)
		rl.DrawRectangleRec(track, uiTrack)
		th := track.Height * menuRows / float32(len(rows))
		ty := track.Y + (track.Height-th)*float32(m.scroll)/float32(len(rows)-menuRows)
		rl.DrawRectangleRec(rl.NewRectangle(track.X, ty, track.Width, th), visorCyan)
	}

	back := settingsBack()
	if rl.CheckCollisionPointRec(mouse, back) {
		fillCut(back, 6*u, uiRowLit)
	}
	hud.footer(back.X+6*u, back.Y+5*u, false, [][2]string{{"ÉCHAP", "retour"}})
	right := p.X + p.Width - 18*u
	switch {
	case m.capture:
		hud.capsRight("ÉCHAP : ANNULER", right, back.Y+7*u, 12*u, visorDim)
	case m.noteT > 0:
		hud.capsRight(strings.ToUpper(m.note), right, back.Y+7*u, 12*u, withAlpha(visorAmber, m.noteT))
	default:
		hud.capsRight("TAB : ONGLET · FLÈCHES : CHOISIR, RÉGLER · ENTRÉE : CHANGER", right, back.Y+7*u, 12*u, visorDim)
	}
}

// drawRow draws a row of the settings at its place among those shown.
func (hud *HUD) drawRow(r *menuRow, lit bool, at int, g *game.Game) {
	u := uiScale()
	row, bar := settingsRow(at)
	if r.kind == rowHeader {
		hud.caps(strings.ToUpper(r.label), row.X+4*u, row.Y+row.Height-20*u, 13*u, visorDim)
		x := row.X + 4*u + hud.capsWidth(strings.ToUpper(r.label), 13*u) + 12*u
		rl.DrawRectangleRec(rl.NewRectangle(x, row.Y+row.Height-13*u, row.X+row.Width-x, 1*u), withAlpha(uiWhite, 0.15))
		return
	}
	label, accent := uiMuted, withAlpha(uiWhite, 0.55)
	fillCut(row, 6*u, uiRow)
	if lit {
		fillCut(row, 6*u, uiRowLit)
		rl.DrawRectangleRec(rl.NewRectangle(row.X, row.Y+6*u, 3*u, row.Height-6*u), visorCyan)
		label, accent = uiWhite, visorCyan
	}
	ty := row.Y + (row.Height-17*u)/2
	right := row.X + row.Width - 14*u
	switch r.kind {
	case rowButton:
		if lit {
			label = visorAmber
		}
		hud.capsCenter(strings.ToUpper(r.label), row.X+row.Width/2, ty+1*u, 14*u, label)
		return
	case rowSlider:
		f := float32((r.get() - r.lo) / (r.hi - r.lo))
		rl.DrawRectangleRec(bar, withAlpha(uiWhite, 0.12))
		rl.DrawRectangleRec(rl.NewRectangle(bar.X, bar.Y, bar.Width*f, bar.Height), accent)
		rl.DrawRectangleRec(rl.NewRectangle(bar.X+bar.Width*f-2*u, bar.Y-5*u, 4*u, bar.Height+10*u), accent)
		hud.monoRight(r.value(), right, ty+1*u, 15*u, label)
	case rowToggle:
		on := r.on()
		sw := rl.NewRectangle(right-56*u, row.Y+(row.Height-20*u)/2, 56*u, 20*u)
		fillBevel(sw, 4*u, withAlpha(uiWhite, 0.12))
		knob := rl.NewRectangle(sw.X+3*u, sw.Y+3*u, 24*u, 14*u)
		if on {
			fillBevel(sw, 4*u, withAlpha(visorCyan, 0.35))
			knob.X = sw.X + sw.Width - 27*u
		}
		state, knobCol := "NON", uiMuted
		if on {
			state, knobCol = "OUI", visorCyan
		}
		fillBevel(knob, 3*u, knobCol)
		hud.capsRight(state, sw.X-12*u, ty+2*u, 13*u, label)
	case rowKey:
		if lit && hud.menu.capture {
			if math.Mod(g.Elapsed*2, 1) < 0.7 {
				hud.capsRight("APPUIE SUR UNE TOUCHE…", right, ty+2*u, 13*u, visorCyan)
			}
		} else {
			name := game.KeyLabel(r.act)
			kw := rl.MeasureTextEx(hud.mono, name, 14*u, 0).X + 12*u
			hud.keycap(name, right-kw, row.Y+(row.Height-21*u)/2, 14*u)
		}
	}
	hud.text(r.label, row.X+16*u, ty, 17*u, label)
}
