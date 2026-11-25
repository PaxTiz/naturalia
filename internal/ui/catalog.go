package ui

import (
	"fmt"
	"image/color"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/game"
	"naturalia/internal/mesh"
	"naturalia/internal/palette"
	"slices"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The building catalog: a grid of tiles by sections, each with a picture
// of what it is, in the finish chosen at the bottom.

const (
	catCols, catRows = 6, 3
	thumbPx          = 256
)

type catalogUI struct {
	tab    string
	scroll int // first row shown
	hover  int // tile under the mouse, -1 for none
	thumbs map[string]rl.RenderTexture2D
}

func catPanel() rl.Rectangle {
	u := uiScale()
	w, h := screenSize()
	return rl.NewRectangle((w-960*u)/2, (h-640*u)/2, 960*u, 640*u)
}

func catTabRect(i, n int) rl.Rectangle {
	p, u := catPanel(), uiScale()
	w := (p.Width - 36*u + 6*u) / float32(max(1, n))
	return rl.NewRectangle(p.X+18*u+float32(i)*w, p.Y+66*u, w-6*u, 30*u)
}

// catTileRect is the tile i places down the page shown.
func catTileRect(i int) rl.Rectangle {
	p, u := catPanel(), uiScale()
	w := (p.Width - 36*u - 14*u + 8*u) / catCols
	return rl.NewRectangle(p.X+18*u+float32(i%catCols)*w, p.Y+106*u+float32(i/catCols)*140*u, w-8*u, 132*u)
}

func catFinishRect(i int) rl.Rectangle {
	p, u := catPanel(), uiScale()
	return rl.NewRectangle(p.X+110*u+float32(i)*40*u, p.Y+p.Height-48*u, 32*u, 32*u)
}

// catPage is the catalog's sections and the one open.
func (hud *HUD) catPage(g *game.Game) ([]game.Shelf, game.Shelf) {
	list := g.Catalog()
	if len(list) == 0 {
		return nil, game.Shelf{}
	}
	at := max(0, slices.IndexFunc(list, func(s game.Shelf) bool { return s.ID == hud.catalog.tab }))
	return list, list[at]
}

// finishes are the finishes to pick from: the blocks' own first.
func finishes(g *game.Game) []string { return append([]string{""}, g.Content.Finishes.IDs()...) }

// OpenShelf opens a section of the catalog, for screenshots.
func (hud *HUD) OpenShelf(id string) { hud.catalog.tab = id }

// updateCatalog switches the sections, scrolls, picks a finish, and takes
// the item clicked.
func (hud *HUD) updateCatalog(g *game.Game) {
	c := &hud.catalog
	list, open := hud.catPage(g)
	mouse, click := rl.GetMousePosition(), rl.IsMouseButtonPressed(rl.MouseButtonLeft)
	at := slices.IndexFunc(list, func(s game.Shelf) bool { return s.ID == open.ID })
	to := at
	if n := len(list); n > 0 {
		switch {
		case rl.IsKeyPressed(rl.KeyLeft), game.KeyPressed(game.ActLeft):
			to = (at + n - 1) % n
		case rl.IsKeyPressed(rl.KeyRight), game.KeyPressed(game.ActRight):
			to = (at + 1) % n
		}
		for i := range list {
			if click && rl.CheckCollisionPointRec(mouse, catTabRect(i, n)) {
				to = i
			}
		}
	}
	if to != at {
		c.tab, c.scroll = list[to].ID, 0
		_, open = hud.catPage(g)
	}
	switch wheel := rl.GetMouseWheelMove(); {
	case wheel < 0, rl.IsKeyPressed(rl.KeyDown):
		c.scroll++
	case wheel > 0, rl.IsKeyPressed(rl.KeyUp):
		c.scroll--
	}
	rows := (len(open.Items) + catCols - 1) / catCols
	c.scroll = max(0, min(c.scroll, rows-catRows))
	if game.KeyPressed(game.ActFinish) {
		g.NextFinish()
	}
	for i, id := range finishes(g) {
		if click && rl.CheckCollisionPointRec(mouse, catFinishRect(i)) {
			g.SetFinish(id)
		}
	}
	c.hover = -1
	for i := range min(catCols*catRows, max(0, len(open.Items)-c.scroll*catCols)) {
		if rl.CheckCollisionPointRec(mouse, catTileRect(i)) {
			c.hover = c.scroll*catCols + i
		}
	}
	// The pictures are painted out of the frame's drawing, a few at a time.
	made := 0
	for _, it := range open.Items {
		if made < 4 && hud.paintThumb(g, it) {
			made++
		}
	}
	if click && c.hover >= 0 {
		g.Take(open.Items[c.hover])
	}
}

func (hud *HUD) drawCatalog(g *game.Game) {
	u := uiScale()
	c := &hud.catalog
	list, open := hud.catPage(g)
	p := catPanel()
	hud.terminal(g, p, "CONSTRUCTION", visorCyan)
	right := p.X + p.Width - 72*u
	credits := game.FmtNum(g.State.Credits) + " cr"
	vw := rl.MeasureTextEx(hud.mono, credits, 24*u, 0).X
	coin := rl.NewVector2(right-vw-16*u, p.Y+26*u)
	rl.DrawCircleV(coin, 9*u, palette.Gold)
	rl.DrawRing(coin, 3.5*u, 5.5*u, 50, 310, 16, color.RGBA{150, 105, 30, 255})
	hud.monoRight(credits, right, p.Y+13*u, 24*u, uiInk)

	mouse := rl.GetMousePosition()
	for i, t := range list {
		r := catTabRect(i, len(list))
		bg, col := uiRow, uiMuted
		switch {
		case t.ID == open.ID:
			bg, col = visorCyan, uiInk
		case rl.CheckCollisionPointRec(mouse, r):
			bg, col = uiRowLit, uiWhite
		}
		fillCut(r, 6*u, bg)
		name, size := strings.ToUpper(t.Name), 14*u
		if w := hud.capsWidth(name, size); w > r.Width-14*u {
			size *= (r.Width - 14*u) / w
		}
		hud.capsCenter(name, r.X+r.Width/2, r.Y+(r.Height-size)/2, size, col)
	}

	var told *content.Item
	held, _ := g.HeldItem()
	for i := range min(catCols*catRows, max(0, len(open.Items)-c.scroll*catCols)) {
		at := c.scroll*catCols + i
		it, r := open.Items[at], catTileRect(i)
		bg := uiRow
		if at == c.hover {
			bg, told = uiRowLit, it
		}
		fillCut(r, 8*u, bg)
		if it == held {
			rl.DrawRectangleRec(rl.NewRectangle(r.X, r.Y+8*u, 3*u, r.Height-8*u), visorCyan)
		}
		box := rl.NewRectangle(r.X+(r.Width-86*u)/2, r.Y+4*u, 86*u, 86*u)
		if tex, ok := c.thumbs[thumbKey(g, it)]; ok {
			rl.DrawTexturePro(tex.Texture, rl.NewRectangle(0, 0, thumbPx, -thumbPx), box, rl.NewVector2(0, 0), 0, rl.White)
		} else {
			icon(content.OrDefault(it.Icon, "crate"), box.X+box.Width/2, box.Y+box.Height/2, 22*u, visorDim)
		}
		size := 14 * u
		if w := hud.width(it.Name, size); w > r.Width-12*u {
			size *= (r.Width - 12*u) / w
		}
		hud.textCenter(it.Name, r.X+r.Width/2, r.Y+92*u, size, uiWhite)
		n := g.State.Items[it.ID]
		col, cost := palette.Gold, itemPrice(it)
		switch {
		case n > 0:
			col, cost = visorCyan, fmt.Sprintf("%d sur toi", n)
		case g.State.Credits < it.Cost:
			col = uiCostOff
		}
		hud.textCenter(cost, r.X+r.Width/2, r.Y+111*u, 13*u, col)
	}
	if rows := (len(open.Items) + catCols - 1) / catCols; rows > catRows {
		top, bottom := catTileRect(0), catTileRect(catCols*catRows-1)
		track := rl.NewRectangle(p.X+p.Width-18*u, top.Y, 4*u, bottom.Y+bottom.Height-top.Y)
		rl.DrawRectangleRec(track, uiTrack)
		h := track.Height * catRows / float32(rows)
		rl.DrawRectangleRec(rl.NewRectangle(track.X, track.Y+(track.Height-h)*float32(c.scroll)/float32(rows-catRows), track.Width, h), visorCyan)
	}
	line := "Clique ce que tu veux poser : tu le paies en le posant, tu es remboursé en le reprenant"
	if told != nil {
		line = told.Name + " : " + told.Desc
	}
	hud.text(hud.fit(line, 15*u, p.Width-44*u), p.X+22*u, p.Y+p.Height-86*u, 15*u, uiMuted)

	hud.caps("FINITION", p.X+22*u, p.Y+p.Height-39*u, 14*u, visorDim)
	name := g.State.Paint
	for i, id := range finishes(g) {
		r := catFinishRect(i)
		if id == g.State.Paint {
			fillCut(rl.NewRectangle(r.X-3*u, r.Y-3*u, r.Width+6*u, r.Height+6*u), 7*u, visorCyan)
		}
		if rl.CheckCollisionPointRec(mouse, r) {
			name = id
		}
		if f, ok := g.Content.Finishes.Get(id); ok && id != "" {
			fillCut(r, 6*u, thumbColor(g, f.Token))
			continue
		}
		fillCut(r, 6*u, uiRow) // the block's own
		rl.DrawLineEx(rl.NewVector2(r.X+8*u, r.Y+r.Height-8*u), rl.NewVector2(r.X+r.Width-8*u, r.Y+8*u), 2*u, uiMuted)
	}
	hud.capsRight(strings.ToUpper(g.FinishName(name)), p.X+p.Width-22*u, p.Y+p.Height-39*u, 14*u, uiWhite)

	s := "OU ÉCHAP POUR FERMER"
	key := game.KeyLabel(game.ActCatalog)
	x := p.X + p.Width/2 - (rl.MeasureTextEx(hud.mono, key, 14*u, 0).X+20*u+hud.capsWidth(s, 13*u))/2
	kw := hud.keycap(key, x, p.Y+p.Height+10*u, 14*u)
	hud.caps(s, x+kw+8*u, p.Y+p.Height+13*u, 13*u, withAlpha(uiWhite, 0.8))
}

// thumbKey is the picture of an item, in the finish chosen.
func thumbKey(g *game.Game, it *content.Item) string {
	if def, ok := g.Content.Entities.Get(it.Places); ok && def.Paint != "" {
		return it.ID + " " + g.State.Paint
	}
	return it.ID
}

func thumbColor(g *game.Game, token string) color.RGBA {
	t, ok := g.Content.Tokens.Get(token)
	if !ok {
		return color.RGBA{255, 0, 255, 255}
	}
	if t.Mat == content.MtHazard {
		return color.RGBA{226, 178, 48, 255}
	}
	return t.Col
}

// paintThumb paints an item's picture, if there is none yet: what it
// places, seen from above its front, under a painted light.
func (hud *HUD) paintThumb(g *game.Game, it *content.Item) bool {
	key := thumbKey(g, it)
	def, ok := g.Content.Entities.Get(it.Places)
	link, isLink := g.Content.Links.Get(it.Link)
	isLink = isLink && it.Link != ""
	if isLink {
		def, ok = &content.EntityDef{}, true
	}
	if _, done := hud.catalog.thumbs[key]; done || !ok || !isLink && len(def.Parts) == 0 && def.Camp == nil {
		return false
	}
	if hud.catalog.thumbs == nil {
		hud.catalog.thumbs = map[string]rl.RenderTexture2D{}
	}
	paint := ""
	if def.Paint != "" {
		paint = g.State.Paint
	}
	parts := g.Content.Painted(def, paint, def.Closed())
	lo, hi := content.Bounds(def.Parts)
	if l := g.CampLayout(def.ID); l != nil {
		parts = l.Parts
		lo, hi = vec3{-l.Reach, 0, -l.Reach}, vec3{l.Reach, l.Beacon.Y, l.Reach}
	}
	if isLink {
		l := base.SampleLink(link)
		parts = l.Parts
		lo, hi = vec3{-l.Length / 2, 0, -link.Width / 2}, vec3{l.Length / 2, link.Height, link.Width / 2}
	}
	mid := lo.Add(hi).Scale(0.5)
	size := math.Max(0.6, hi.Sub(lo).Length())
	if def.Camp != nil {
		size *= 0.7 // round: nothing in the corners
	}

	// The view: right, up, and toward the eye.
	eye := vec3{1, 0.75, 0.62}.Norm()
	side := vec3{0, 1, 0}.Cross(eye).Norm()
	up := eye.Cross(side)
	light := vec3{0.45, 0.8, 0.15}.Norm()

	tex := rl.LoadRenderTexture(thumbPx, thumbPx)
	rl.SetTextureFilter(tex.Texture, rl.FilterBilinear)
	rl.BeginTextureMode(tex)
	rl.ClearBackground(color.RGBA{})
	for _, through := range []bool{false, true} { // the glass last
		type face struct {
			p   [3]vec3
			col color.RGBA
		}
		var faces []face
		for _, k := range parts.Keys {
			if through != (k.Lit == mesh.LitGlass) {
				continue
			}
			m, base := parts.Mesh(k), thumbColor(g, k.Tok)
			t, _ := g.Content.Tokens.Get(k.Tok)
			for i := 0; i+8 < len(m.V); i += 9 {
				var f face
				var n vec3
				for j := range 3 {
					f.p[j] = vec3{float64(m.V[i+j*3]), float64(m.V[i+j*3+1]), float64(m.V[i+j*3+2])}.Sub(mid).Scale(1 / size)
					n = n.Add(vec3{float64(m.N[i+j*3]), float64(m.N[i+j*3+1]), float64(m.N[i+j*3+2])})
				}
				if n = n.Norm(); n.Dot(eye) <= 0 {
					continue
				}
				lit := 0.45 + 0.6*math.Max(0, n.Dot(light))
				if t != nil && t.Emissive > 0 {
					lit = 1.1
				}
				f.col = base
				for _, ch := range []*uint8{&f.col.R, &f.col.G, &f.col.B} {
					*ch = uint8(math.Min(255, float64(*ch)*lit))
				}
				if through {
					f.col.A = 190
				}
				faces = append(faces, f)
			}
		}
		drawThumbFaces(len(faces), func(i int) ([3]vec3, color.RGBA) { return faces[i].p, faces[i].col }, side, up, eye)
	}
	rl.EndTextureMode()
	hud.catalog.thumbs[key] = tex
	return true
}

// drawThumbFaces draws triangles seen along eye, in a unit box, the nearer
// over the others.
func drawThumbFaces(n int, face func(i int) ([3]vec3, color.RGBA), side, up, eye vec3) {
	rl.DrawRenderBatchActive()
	rl.EnableDepthTest()
	rl.DisableBackfaceCulling()
	rl.Begin(rl.Triangles)
	for i := range n {
		p, col := face(i)
		rl.Color4ub(col.R, col.G, col.B, col.A)
		for _, v := range p {
			// In the 2D projection: y down, depths from -1 to 0, the nearest.
			rl.Vertex3f(float32(thumbPx/2+v.Dot(side)*thumbPx*0.98), float32(thumbPx/2-v.Dot(up)*thumbPx*0.98), float32(v.Dot(eye)*0.9-0.5))
		}
	}
	rl.End()
	rl.DrawRenderBatchActive()
	rl.EnableBackfaceCulling()
	rl.DisableDepthTest()
}
