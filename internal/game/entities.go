package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/mathx"
	"naturalia/internal/planet"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Entities are things standing in the world that aren't part of the base:
// a flag planted on a dune, a beacon, a crate dropped by a mod's event.
// Their shape is made of kit shapes, like a mod's furniture; where they
// stand is saved with the game. Items are what the player carries and
// places as entities (key F): bought in the shop, picked back up with E.

// Placed is an entity standing in the world, saved.
type Placed struct {
	Kind string  `json:"kind"`
	Dir  vec3    `json:"dir"` // from Mars' center, unit
	Yaw  float64 `json:"yaw"`
	Alt  float64 `json:"alt,omitempty"`  // over the ground: what flies or hovers
	Home vec3    `json:"home,omitempty"` // a patrolling vehicle's spot
	// The driven vehicle leans on slopes and in jumps: nose up, right
	// side down, in radians.
	Pitch float64 `json:"pitch,omitempty"`
	Roll  float64 `json:"roll,omitempty"`

	target   vec3 // an automatic vehicle's next dusty spot...
	retarget float64
}

const (
	EntityDrawDist = 250.0
	entityReach    = 2.6
)

// spawn sets an entity on the ground at world position at, facing the
// player.
func (g *Game) spawn(kind string, at vec3) bool {
	def, ok := g.Content.Entities.Get(kind)
	if !ok {
		return false
	}
	up := at.Norm()
	t1, t2 := mathx.TangentBasis(up)
	toward := planet.TangentAt(up, g.Player.Pos.Sub(at))
	yaw := math.Atan2(toward.Dot(t2), toward.Dot(t1))
	if def.Ramp != nil || def.Kick > 0 {
		yaw += math.Pi // rising away from the player, toward where they drive
	}
	g.State.Entities = append(g.State.Entities, Placed{Kind: def.ID, Dir: up, Yaw: yaw})
	return true
}

// placeAt is where F puts an item: the aimed ground, or right ahead.
func (g *Game) placeAt() vec3 {
	if g.AimOK && g.Aim.Sub(g.Player.Pos).Length() < 6 {
		return g.Aim
	}
	return g.World.Relief.Surface(g.Player.Pos.Add(g.Player.Heading.Scale(1.5)))
}

// HeldItem is the item F places, cycled with R, among those carried.
func (g *Game) HeldItem() (*content.Item, int) {
	var owned []*content.Item
	for _, it := range g.Content.Items.All() {
		if g.State.Items[it.ID] > 0 {
			owned = append(owned, it)
		}
	}
	if len(owned) == 0 {
		return nil, 0
	}
	it := owned[g.itemSel%len(owned)]
	return it, g.State.Items[it.ID]
}

func (g *Game) updateItems() {
	if g.Mode != ModePlay {
		return
	}
	if rl.IsKeyPressed(rl.KeyR) {
		g.itemSel++
	}
	it, n := g.HeldItem()
	if it == nil || n == 0 || !rl.IsKeyPressed(rl.KeyF) {
		return
	}
	at := g.placeAt()
	if !g.spawn(it.Places, at) {
		return
	}
	g.State.Items[it.ID]--
	g.sfx.Beep(0.02, 0.15, 0.12, 392, 523)
	g.emit(content.Event{Name: content.EvPlace, ID: content.Qualify(it.Places), At: at})
}

// EntitiesNear lists the placed entities within dist of the player, with
// their index in the save.
func (g *Game) EntitiesNear(dist float64) []int {
	var out []int
	up := g.Player.Pos.Norm()
	for i, e := range g.State.Entities {
		if planet.ArcDist(up, e.Dir) < dist {
			out = append(out, i)
		}
	}
	return out
}

func (g *Game) collideEntities() {
	for _, i := range g.EntitiesNear(10) {
		if i == g.Riding {
			continue
		}
		e := g.State.Entities[i]
		if def, ok := g.Content.Entities.Get(e.Kind); ok && def.Radius > 0 {
			g.Player.pushOut(g.World.Relief.Surface(e.Dir), def.Radius)
		}
	}
}

// nearbyEntity is the closest entity with something to do, within reach.
func (g *Game) nearbyEntity() (int, float64) {
	best, bestDist := -1, math.Inf(1)
	for _, i := range g.EntitiesNear(entityReach + 3) {
		if i == g.Riding {
			continue
		}
		e := g.State.Entities[i]
		def, ok := g.Content.Entities.Get(e.Kind)
		if !ok || (def.Prompt == "" && !def.Pickup) {
			continue
		}
		pos := g.World.Relief.Surface(e.Dir)
		to := pos.Sub(g.Player.Pos)
		d := to.Length()
		if d > entityReach+def.Radius || d >= bestDist || (d > 0.8 && g.Player.Heading.Dot(to.Norm()) < 0.3) {
			continue
		}
		best, bestDist = i, d
	}
	return best, bestDist
}

// entityUse turns an entity into something E can use.
func (g *Game) entityUse(i int) base.Interactable {
	e := g.State.Entities[i]
	def, _ := g.Content.Entities.Get(e.Kind)
	prompt := def.Prompt
	if prompt == "" {
		prompt = "Ramasser : " + def.Name
	}
	if v, ok := g.Content.Vehicles.Get(e.Kind); ok && !v.Auto() {
		return base.Interactable{Kit: def.ID, At: base.ToLocal(g.World.Relief.Surface(e.Dir)), Prompt: prompt,
			Use: func(content.Game, vec3) { g.getIn(i) }}
	}
	return base.Interactable{
		Kit:    def.ID,
		At:     base.ToLocal(g.World.Relief.Surface(e.Dir)),
		Prompt: prompt,
		Use: func(content.Game, vec3) {
			if def.Pickup {
				g.State.Entities = append(g.State.Entities[:i], g.State.Entities[i+1:]...)
				if def.Item != "" {
					if g.State.Items == nil {
						g.State.Items = map[string]int{}
					}
					g.State.Items[def.Item]++
				}
				g.sfx.Beep(0.02, 0.15, 0.12, 523, 392)
			}
		},
	}
}

// NearbyUse returns what the player can use right now: the closest
// interactable within reach that they roughly face.
func (g *Game) NearbyUse() (base.Interactable, bool) {
	if i, _ := g.nearbyEntity(); i >= 0 {
		return g.entityUse(i), true
	}
	local := base.ToLocal(g.Player.Pos)
	flat := vec3{local.X, 0, local.Z}
	best, found := base.Interactable{}, false
	bestDist := math.Inf(1)
	for _, it := range base.Interactables {
		to := vec3{it.At.X, 0, it.At.Z}.Sub(flat)
		d := to.Length()
		if d > it.Reach || d >= bestDist {
			continue
		}
		heading := base.ToLocal(g.Player.Pos.Add(g.Player.Heading)).Sub(local)
		if d > 0.8 && heading.Dot(to.Norm()) < 0.3 {
			continue // standing next to it but looking away
		}
		best, found, bestDist = it, true, d
	}
	return best, found
}
