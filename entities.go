package main

import (
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/planet"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Entities are things standing in the world that aren't part of the base:
// a flag planted on a dune, a beacon, a crate dropped by a mod's event.
// Their shape is made of kit shapes, like a mod's furniture; where they
// stand is saved with the game. Items are what the player carries and
// places as entities (key F): bought in the shop, picked back up with E.

// EntityDef describes a kind of entity, in its own frame: y up, standing
// on the ground at the origin.
type EntityDef struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Parts  []partJSON `json:"parts"`
	Radius float64    `json:"radius"` // what the player bumps into; 0 walks through
	Prompt string     `json:"prompt"` // what E says; empty: nothing to do
	Pickup bool       `json:"pickup"` // E puts it back in the inventory
	Item   string     `json:"item"`   // the item it gives back (set by the item that places it)

	models []entityModel // built on first draw
}

type entityModel struct {
	model rl.Model
	tok   token
}

// Item is carried in the inventory and placed as an entity.
type Item struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Desc   string  `json:"desc"`
	Icon   string  `json:"icon"`
	Cost   float64 `json:"cost"`   // in the shop; 0 keeps it out of it
	Places string  `json:"places"` // ID of the entity it becomes
}

// Placed is an entity standing in the world, saved.
type Placed struct {
	Kind string  `json:"kind"`
	Dir  vec3    `json:"dir"` // from Mars' center, unit
	Yaw  float64 `json:"yaw"`
}

const (
	entityDrawDist = 250.0
	entityReach    = 2.6
)

// spawn sets an entity on the ground at world position at, facing the
// player.
func (g *Game) spawn(kind string, at vec3) bool {
	def, ok := content.Entities.Get(kind)
	if !ok {
		return false
	}
	up := at.Norm()
	t1, t2 := mathx.TangentBasis(up)
	toward := planet.TangentAt(up, g.player.Pos.Sub(at))
	yaw := math.Atan2(toward.Dot(t2), toward.Dot(t1))
	g.st.Entities = append(g.st.Entities, Placed{Kind: def.ID, Dir: up, Yaw: yaw})
	return true
}

// placeAt is where F puts an item: the aimed ground, or right ahead.
func (g *Game) placeAt() vec3 {
	if g.aimOK && g.aim.Sub(g.player.Pos).Length() < 6 {
		return g.aim
	}
	return g.terrain.Surface(g.player.Pos.Add(g.player.Heading.Scale(1.5)))
}

// heldItem is the item F places, cycled with R, among those carried.
func (g *Game) heldItem() (*Item, int) {
	var owned []*Item
	for _, it := range content.Items.All() {
		if g.st.Items[it.ID] > 0 {
			owned = append(owned, it)
		}
	}
	if len(owned) == 0 {
		return nil, 0
	}
	it := owned[g.itemSel%len(owned)]
	return it, g.st.Items[it.ID]
}

func (g *Game) updateItems() {
	if g.mode != modePlay {
		return
	}
	if rl.IsKeyPressed(rl.KeyR) {
		g.itemSel++
	}
	it, n := g.heldItem()
	if it == nil || n == 0 || !rl.IsKeyPressed(rl.KeyF) {
		return
	}
	at := g.placeAt()
	if !g.spawn(it.Places, at) {
		return
	}
	g.st.Items[it.ID]--
	g.sfx.beep(0.02, 0.15, 0.12, 392, 523)
	g.emit(Event{Name: evPlace, ID: qualify(it.Places), At: at})
}

// entitiesNear lists the placed entities within dist of the player, with
// their index in the save.
func (g *Game) entitiesNear(dist float64) []int {
	var out []int
	up := g.player.Pos.Norm()
	for i, e := range g.st.Entities {
		if planet.ArcDist(up, e.Dir) < dist {
			out = append(out, i)
		}
	}
	return out
}

func (g *Game) collideEntities() {
	for _, i := range g.entitiesNear(10) {
		if i == g.driving {
			continue
		}
		e := g.st.Entities[i]
		if def, ok := content.Entities.Get(e.Kind); ok && def.Radius > 0 {
			g.player.pushOut(g.terrain.Surface(e.Dir), def.Radius)
		}
	}
}

// nearbyEntity is the closest entity with something to do, within reach.
func (g *Game) nearbyEntity() (int, float64) {
	best, bestDist := -1, math.Inf(1)
	for _, i := range g.entitiesNear(entityReach + 3) {
		if i == g.driving {
			continue
		}
		e := g.st.Entities[i]
		def, ok := content.Entities.Get(e.Kind)
		if !ok || (def.Prompt == "" && !def.Pickup) {
			continue
		}
		pos := g.terrain.Surface(e.Dir)
		to := pos.Sub(g.player.Pos)
		d := to.Length()
		if d > entityReach+def.Radius || d >= bestDist || (d > 0.8 && g.player.Heading.Dot(to.Norm()) < 0.3) {
			continue
		}
		best, bestDist = i, d
	}
	return best, bestDist
}

// entityUse turns an entity into something E can use.
func (g *Game) entityUse(i int) interactable {
	e := g.st.Entities[i]
	def, _ := content.Entities.Get(e.Kind)
	prompt := def.Prompt
	if prompt == "" {
		prompt = "Ramasser : " + def.Name
	}
	if _, ok := content.Vehicles.Get(e.Kind); ok {
		return interactable{kit: def.ID, at: toLocal(g.terrain.Surface(e.Dir)), prompt: prompt,
			use: func(g *Game, _ vec3) { g.getIn(i) }}
	}
	return interactable{
		kit:    def.ID,
		at:     toLocal(g.terrain.Surface(e.Dir)),
		prompt: prompt,
		use: func(g *Game, _ vec3) {
			if def.Pickup {
				g.st.Entities = append(g.st.Entities[:i], g.st.Entities[i+1:]...)
				if def.Item != "" {
					if g.st.Items == nil {
						g.st.Items = map[string]int{}
					}
					g.st.Items[def.Item]++
				}
				g.sfx.beep(0.02, 0.15, 0.12, 523, 392)
			}
		},
	}
}

// drawEntities draws the entities around the player, standing up from the
// ground.
func (g *Game) drawEntities() {
	gfx := g.gfx
	for _, i := range g.entitiesNear(entityDrawDist) {
		e := g.st.Entities[i]
		def, ok := content.Entities.Get(e.Kind)
		if !ok {
			continue // its mod was removed: it stays in the save
		}
		if def.models == nil {
			def.models = buildEntity(def, gfx)
		}
		pos := g.terrain.Surface(e.Dir)
		up := e.Dir
		t1, t2 := mathx.TangentBasis(up)
		ax := t1.Scale(math.Cos(e.Yaw)).Add(t2.Scale(math.Sin(e.Yaw)))
		az := ax.Cross(up)
		lit := float32(litSun)
		if indoors(toLocal(pos)) {
			lit = litIndoor
		}
		gfx.setLighting(lit, vec3{})
		for _, m := range def.models {
			gfx.setMat(m.tok.mat, m.tok.scale, false)
			gfx.putBasis(m.model, pos, ax, up, az, m.tok.col, m.tok.spec, m.tok.emissive)
		}
	}
	gfx.setMat(mtPlain, 1, false)
	gfx.setLighting(litSun, vec3{})
}

// buildEntity merges an entity's parts into one model per material, in
// its own frame.
func buildEntity(def *EntityDef, gfx *Graphics) []entityModel {
	b := &building{parts: map[partKey]*meshBuilder{}, local: true}
	kitJSON{Parts: def.Parts}.kit().build(b, vec3{}, 0)
	var out []entityModel
	for _, k := range b.keys {
		m := b.parts[k]
		if len(m.v) == 0 {
			continue
		}
		model := m.model(gfx.objectSh)
		gfx.bindMaps(model)
		out = append(out, entityModel{model, tok(k.tok)})
	}
	if out == nil {
		out = []entityModel{} // nothing to draw, but built
	}
	return out
}
