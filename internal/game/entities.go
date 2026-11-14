package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/mathx"
	"naturalia/internal/physics"
	"naturalia/internal/planet"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Entities are things standing in the world that aren't part of the base:
// a flag planted on a dune, a beacon, a crate dropped by a mod's event, the
// vehicles. Their shape is made of kit shapes, like a mod's furniture;
// where they stand is saved with the game. Items are what the player
// carries and places as entities (key F): bought in the shop, picked back
// up with E.

// Ents are the entities standing in the world: an ID each (package ecs),
// and their components.
type Ents struct {
	*ecs.World
	Kind      *ecs.Store[string] // its EntityDef's ID, and its Vehicle's
	Transform *ecs.Store[Transform]
	Roam      *ecs.Store[Roam]          // automatic vehicles
	Body      *ecs.Store[*physics.Body] // near the player, what takes room (physics.go)
	Air       *ecs.Store[Air]           // tossed up
}

// Air is a thing tossed up by a knock: its speed up, and how fast it
// tumbles.
type Air struct{ vz, tumble float64 }

func newEnts() *Ents {
	w := ecs.NewWorld()
	return &Ents{World: w, Kind: ecs.NewStore[string](w), Transform: ecs.NewStore[Transform](w), Roam: ecs.NewStore[Roam](w),
		Body: ecs.NewStore[*physics.Body](w), Air: ecs.NewStore[Air](w)}
}

// Transform is where an entity stands.
type Transform struct {
	Dir vec3 // from Mars' center, unit
	Yaw float64
	Alt float64 // over the ground: what flies or hovers
	// The driven vehicle leans on slopes and in jumps: nose up, right
	// side down, in radians.
	Pitch, Roll float64
}

// Roam is where an automatic vehicle goes.
type Roam struct {
	Home     vec3 // a patrolling vehicle's spot
	target   vec3 // its next dusty spot...
	retarget float64
	want     vec3    // near the player, the speed it asks of its body this frame
	dodge    float64 // a ground robot going around something: the side, ±1
}

// Placed is an entity as the save keeps it.
type Placed struct {
	Kind  string  `json:"kind"`
	Dir   vec3    `json:"dir"`
	Yaw   float64 `json:"yaw"`
	Alt   float64 `json:"alt,omitempty"`
	Home  vec3    `json:"home,omitempty"`
	Pitch float64 `json:"pitch,omitempty"`
	Roll  float64 `json:"roll,omitempty"`
}

// add brings a saved entity into the world.
func (g *Game) add(p Placed) ecs.ID {
	id := g.Ents.New()
	g.Ents.Kind.Set(id, p.Kind)
	g.Ents.Transform.Set(id, Transform{p.Dir, p.Yaw, p.Alt, p.Pitch, p.Roll})
	if v, ok := g.Content.Vehicles.Get(p.Kind); (ok && v.Auto()) || p.Home != (vec3{}) {
		g.Ents.Roam.Set(id, Roam{Home: p.Home})
	}
	return id
}

// placed are the entities for the save, always in the same order.
func (g *Game) placed() []Placed {
	var out []Placed
	for id := range g.Ents.All() {
		t := g.Ents.Transform.Get(id)
		p := Placed{Kind: g.kind(id), Dir: t.Dir, Yaw: t.Yaw, Alt: t.Alt, Pitch: t.Pitch, Roll: t.Roll}
		if r := g.Ents.Roam.Get(id); r != nil {
			p.Home = r.Home
		}
		out = append(out, p)
	}
	return out
}

// kind is entity id's kind, "" for none.
func (g *Game) kind(id ecs.ID) string {
	if k := g.Ents.Kind.Get(id); k != nil {
		return *k
	}
	return ""
}

// vehicleOf is entity id's vehicle, if it is one.
func (g *Game) vehicleOf(id ecs.ID) (*content.Vehicle, bool) {
	return g.Content.Vehicles.Get(g.kind(id))
}

const (
	EntityDrawDist = 250.0
	entityReach    = 2.6
)

// spawn sets an entity on the ground at world position at, facing the
// player: its ID, 0 for an unknown kind.
func (g *Game) spawn(kind string, at vec3) ecs.ID {
	def, ok := g.Content.Entities.Get(kind)
	if !ok {
		return 0
	}
	up := at.Norm()
	t1, t2 := mathx.TangentBasis(up)
	toward := planet.TangentAt(up, g.Player.Pos.Sub(at))
	yaw := math.Atan2(toward.Dot(t2), toward.Dot(t1))
	if def.Ramp != nil || def.Kick > 0 {
		yaw += math.Pi // rising away from the player, toward where they drive
	}
	return g.add(Placed{Kind: def.ID, Dir: up, Yaw: yaw})
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
	if g.spawn(it.Places, at) == 0 {
		return
	}
	g.State.Items[it.ID]--
	g.sfx.Beep(0.02, 0.15, 0.12, 392, 523)
	g.emit(content.Event{Name: content.EvPlace, ID: content.Qualify(it.Places), At: at})
}

// EntitiesNear lists the entities within dist of the player.
func (g *Game) EntitiesNear(dist float64) []ecs.ID {
	var out []ecs.ID
	up := g.Player.Pos.Norm()
	for id, t := range g.Ents.Transform.All() {
		if planet.ArcDist(up, t.Dir) < dist {
			out = append(out, id)
		}
	}
	return out
}

// nearbyEntity is the closest entity with something to do, within reach.
func (g *Game) nearbyEntity() (ecs.ID, float64) {
	best, bestDist := ecs.ID(0), math.Inf(1)
	for _, id := range g.EntitiesNear(entityReach + 3) {
		if id == g.Riding {
			continue
		}
		def, ok := g.Content.Entities.Get(g.kind(id))
		if !ok || (def.Prompt == "" && !def.Pickup) {
			continue
		}
		pos := g.World.Relief.Surface(g.Ents.Transform.Get(id).Dir)
		to := pos.Sub(g.Player.Pos)
		d := to.Length()
		if d > entityReach+def.Radius || d >= bestDist || (d > 0.8 && g.Player.Heading.Dot(to.Norm()) < 0.3) {
			continue
		}
		best, bestDist = id, d
	}
	return best, bestDist
}

// entityUse turns an entity into something E can use.
func (g *Game) entityUse(id ecs.ID) base.Interactable {
	def, _ := g.Content.Entities.Get(g.kind(id))
	prompt := def.Prompt
	if prompt == "" {
		prompt = "Ramasser : " + def.Name
	}
	at := base.ToLocal(g.World.Relief.Surface(g.Ents.Transform.Get(id).Dir))
	if v, ok := g.vehicleOf(id); ok && !v.Auto() {
		return base.Interactable{Kit: def.ID, At: at, Prompt: prompt,
			Use: func(content.Game, vec3) { g.getIn(id) }}
	}
	return base.Interactable{
		Kit:    def.ID,
		At:     at,
		Prompt: prompt,
		Use: func(content.Game, vec3) {
			if def.Pickup {
				g.Ents.Kill(id)
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
	if id, _ := g.nearbyEntity(); id != 0 {
		return g.entityUse(id), true
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
