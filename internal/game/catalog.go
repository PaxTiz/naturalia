package game

import (
	"naturalia/internal/content"
	"naturalia/internal/engine/ecs"
	"naturalia/internal/palette"
	"slices"
)

// The building catalog: what there is to build with, by sections, opened
// anywhere. What is picked there goes in hand, paid for as it is set down
// (what the player carries already is used first); taken back, it gives
// its price back. Blocks come in finishes (content.Finish).

// Shelf is a section of the catalog.
type Shelf struct {
	ID, Name string
	Items    []*content.Item
}

// carriedShelf is the section of what the player carries and the catalog
// doesn't sell.
const carriedShelf = "carried"

// Catalog are the catalog's sections, those with something in them.
func (g *Game) Catalog() []Shelf {
	var out []Shelf
	for _, s := range content.Shelves {
		out = append(out, Shelf{ID: s.ID, Name: s.Name})
	}
	carried := Shelf{ID: carriedShelf, Name: "Sur toi"}
	for _, it := range g.Content.Items.All() {
		id := g.Content.Shelf(it)
		if id == "" {
			if g.State.Items[it.ID] > 0 {
				carried.Items = append(carried.Items, it)
			}
			continue
		}
		i := slices.IndexFunc(out, func(s Shelf) bool { return s.ID == id })
		if i < 0 {
			i, out = len(out), append(out, Shelf{ID: id, Name: id})
		}
		out[i].Items = append(out[i].Items, it)
	}
	out = append(out, carried)
	return slices.DeleteFunc(out, func(s Shelf) bool { return len(s.Items) == 0 })
}

// OpenCatalog opens the building catalog.
func (g *Game) OpenCatalog() { g.setMode(ModeBuild) }

// handy are the items R cycles through: those carried, and those last
// picked in the catalog.
func (g *Game) handy() []*content.Item {
	var out []*content.Item
	for _, it := range g.Content.Items.All() {
		if g.State.Items[it.ID] > 0 || slices.Contains(g.State.Recent, it.ID) && g.Content.Shelf(it) != "" {
			out = append(out, it)
		}
	}
	return out
}

// HeldItem is the item F places, and how many are carried: none of what is
// paid for as it is set down. Nil without one.
func (g *Game) HeldItem() (*content.Item, int) {
	list := g.handy()
	if len(list) == 0 {
		return nil, 0
	}
	it := list[max(0, slices.IndexFunc(list, func(it *content.Item) bool { return it.ID == g.hand }))]
	return it, g.State.Items[it.ID]
}

// nextItem takes the next of the items in hand.
func (g *Game) nextItem() *content.Item {
	list := g.handy()
	if len(list) == 0 {
		return nil
	}
	held, _ := g.HeldItem()
	it := list[(slices.Index(list, held)+1)%len(list)]
	g.hand = it.ID
	return it
}

// recents is how many items picked in the catalog R cycles through.
const recents = 8

// Take puts an item of the catalog in hand, and back to the game.
func (g *Game) Take(it *content.Item) {
	if g.Content.Shelf(it) != "" && !slices.Contains(g.State.Recent, it.ID) {
		g.State.Recent = append(g.State.Recent, it.ID)
		g.State.Recent = g.State.Recent[max(0, len(g.State.Recent)-recents):]
	}
	g.setMode(ModePlay)
	g.hold(it)
	g.sfx.Beep(0.02, 0.1, 0.1, 523, 659)
}

// hold shows an item's ghost: turned as the one it replaces, if any.
func (g *Game) hold(it *content.Item) {
	g.hand = it.ID
	if g.Ghost == nil || g.Ghost.Moving != 0 {
		g.Ghost = &Ghost{rel: [2]vec3{{1, 0, 0}, {0, 1, 0}}, dist: 4}
	}
	g.Ghost.Kind, g.Ghost.item, g.Ghost.Paint, g.Ghost.Link = content.Qualify(it.Places), it.ID, g.State.Paint, nil
	if l, ok := g.Content.Links.Get(it.Link); ok && it.Link != "" {
		g.Ghost.Kind, g.Ghost.Link = "", &LinkGhost{Def: l}
	}
	g.poseGhost()
}

// aimedBlock is the block or the building to take back the player aims at,
// 0 for none, or while they set something down.
func (g *Game) aimedBlock() ecs.ID {
	if g.Ghost != nil {
		return 0
	}
	id := g.AimedMovable()
	if def, ok := g.Content.Entities.Get(g.kind(id)); id == 0 || !ok || !def.Build && !isCamp(def) || !def.Pickup {
		return 0
	}
	return id
}

// takeSame puts in hand the same as what the player aims at, painted like
// it.
func (g *Game) takeSame() bool {
	id := g.AimedMovable()
	def, ok := g.Content.Entities.Get(g.kind(id))
	if id == 0 || !ok {
		return false
	}
	it, ok := g.Content.Items.Get(def.Item)
	if !ok || def.Item == "" || g.Content.Shelf(it) == "" && g.State.Items[it.ID] == 0 {
		return false
	}
	if def.Paint != "" {
		g.State.Paint = g.Paint(id)
	}
	g.Take(it)
	return true
}

// canPay tells whether the player has an item, or can pay for it.
func (g *Game) canPay(it *content.Item) bool {
	return g.State.Items[it.ID] > 0 || g.Content.Shelf(it) != "" && g.State.Credits >= it.Cost
}

// pay takes an item set down as entity id: from those carried, or its
// price.
func (g *Game) pay(id ecs.ID, item string) {
	if g.State.Items[item] > 0 {
		g.State.Items[item]--
		return
	}
	it, ok := g.Content.Items.Get(item)
	if !ok || !g.canPay(it) {
		return
	}
	g.State.Credits -= it.Cost
	g.Ents.Paid.Set(id, it.Cost)
	g.emit(content.Event{Name: content.EvBuy, ID: item, Amount: it.Cost})
}

// giveBack gives back entity id taken from the world: what was paid for
// it, or its item.
func (g *Game) giveBack(id ecs.ID, def *content.EntityDef) {
	if paid := g.Ents.Paid.Get(id); paid != nil && *paid > 0 {
		g.State.Credits += *paid
		g.Floaters = append(g.Floaters, Floater{Pos: g.Player.Eye().Add(g.Player.Forward().Scale(2)), Life: 2, Text: "+" + FmtNum(*paid) + " cr", C: palette.Gold})
		return
	}
	if def.Item != "" {
		g.GiveItem(def.Item, 1)
	}
}

// Paint is the finish entity id is painted with, "" for its own.
func (g *Game) Paint(id ecs.ID) string {
	if p := g.Ents.Paint.Get(id); p != nil {
		return *p
	}
	return ""
}

func (g *Game) setPaint(id ecs.ID, paint string) {
	def, ok := g.Content.Entities.Get(g.kind(id))
	if !ok || def.Paint == "" || paint == "" {
		g.Ents.Paint.Remove(id)
		return
	}
	g.Ents.Paint.Set(id, paint)
}

// SetFinish has blocks painted with a finish, "" for their own; the one
// held too.
func (g *Game) SetFinish(id string) {
	if g.Ghost == nil || g.Ghost.Moving == 0 { // what is moved is painted alone
		g.State.Paint = id
	}
	if g.Ghost != nil {
		g.Ghost.Paint = id
	}
}

// NextFinish takes the next finish, after the last, the block's own.
func (g *Game) NextFinish() {
	ids := append([]string{""}, g.Content.Finishes.IDs()...)
	now := g.State.Paint
	if g.Ghost != nil {
		now = g.Ghost.Paint
	}
	g.SetFinish(ids[(slices.Index(ids, now)+1)%len(ids)])
	g.sfx.Beep(0.02, 0.08, 0.08, 587)
}

// FinishName is a finish's name.
func (g *Game) FinishName(id string) string {
	if f, ok := g.Content.Finishes.Get(id); ok && id != "" {
		return f.Name
	}
	return "d'origine"
}
