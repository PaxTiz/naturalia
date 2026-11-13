package content

import (
	"image/color"
	"math"
	"naturalia/internal/mesh"
)

// Token is a material of the base: color, painted material, shine, glow.
type Token struct {
	Col      color.RGBA
	Mat      int
	Scale    float32 // texture tiles per meter
	Spec     float32
	Emissive float32
}

// Blueprint is a building as data.
type Blueprint struct {
	ID     string `json:"id"`
	At     vec3   `json:"at"`   // local, center of the floor
	Link   string `json:"link"` // building a corridor leads to; doors are added at both ends
	Sign   string `json:"sign"` // painted over the door that leads here from the hub
	Shape  Dome   `json:"shape"`
	Style  string `json:"style"`  // token of the walls and roof
	Accent string `json:"accent"` // token of the glowing stripes inside
	Doors  []Door `json:"doors"`
	Props  []Prop `json:"props"`
}

type Dome struct {
	Radius float64 `json:"radius"`
	WallH  float64 `json:"wall_height"`
}

type Door struct {
	Facing float64 `json:"facing"` // angle, atan2(z, x)
}

// Prop is a kit piece placed in a building, relative to its center; Seed
// varies pieces that are built by rules.
type Prop struct {
	Kit  string `json:"kit"`
	At   vec3   `json:"at"`
	Seed int    `json:"seed"`
}

// Polar is the point at distance r from the center, at angle a.
func Polar(r, a float64) vec3 { return vec3{math.Cos(a) * r, 0, math.Sin(a) * r} }

// Kit is a piece of furniture: what it adds to the merged meshes, how
// the player bumps into it and uses it. Positions are local, relative to
// the prop. What moves or glows is drawn each frame by the renderer.
type Kit struct {
	Build     func(b *mesh.Parts, at vec3, seed int)
	Obstacles []Obstacle
	Reach     float64
	Use       func(g Game, at vec3) // what E does; at is the prop's local position
	Prompt    string
}

type headShape int

const (
	HeadBlock  headShape = iota // straight block (toothbrush bristles)
	HeadTaper                   // narrows to a point (paintbrush)
	HeadFlare                   // widens toward the ground (broom)
	HeadNozzle                  // motor bulb and nozzle (powered tools)
)

// ToolLook describes the model held in first person: a handle, a metal
// collar, then the head.
// In mods it reads (mods.go):
//
//	{"handle_len": 0.5, "handle_r": 0.02, "handle_color": "#a57346",
//	 "head_len": 0.24, "head_r": 0.12, "head_color": "#f0cd5f",
//	 "head": "block|taper|flare|nozzle", "glow": 0}
type ToolLook struct {
	HandleLen, HandleR float64
	HandleCol          color.RGBA
	HeadLen, HeadR     float64
	HeadCol            color.RGBA
	Head               headShape
	Glow               float32 // emissive strength of the nozzle ring
}

type Tool struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Radius float64  `json:"radius"` // brush radius in meters
	Reach  float64  `json:"reach"`  // max aiming distance in meters
	Rate   float64  `json:"rate"`   // grams per second removed from a cell at the brush center
	Cost   float64  `json:"cost"`
	Look   ToolLook `json:"look"`
}

// Upgrade is bought level after level; each level applies its effects to
// the game's stats.
type Upgrade struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Desc    string   `json:"desc"`
	Icon    string   `json:"icon"`   // pictogram drawn by the shop (icon)
	Base    float64  `json:"base"`   // price of the first level...
	Growth  float64  `json:"growth"` // ...times this for each level bought
	Max     int      `json:"max"`    // 0 means unlimited
	Effects []Effect `json:"effects"`
}

// Effect changes a stat for each level: (base + Add×level) × Mult^level.
type Effect struct {
	Stat string  `json:"stat"`
	Mult float64 `json:"mult"`
	Add  float64 `json:"add"`
}

// Stats are what upgrades act on, with their value before any upgrade.
var Stats = map[string]float64{
	"tank":     50, // grams the tank holds
	"power":    1,  // multiplies how fast tools and vehicles sweep
	"price":    1,  // multiplies what dust and crystals sell for
	"autosell": 0,  // above 0, the tank sells itself when full
}

// EntityDef describes a kind of entity, in its own frame: y up, standing
// on the ground at the origin.
type EntityDef struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Parts  []PartJSON `json:"parts"`
	Radius float64    `json:"radius"` // what the player bumps into; 0 walks through
	Prompt string     `json:"prompt"` // what E says; empty: nothing to do
	Pickup bool       `json:"pickup"` // E puts it back in the inventory
	Item   string     `json:"item"`   // the item it gives back (set by the item that places it)
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

// Vehicles are entities that sweep the dust: bought in the shop, they come
// in two kinds.
//
// Driven ones ("drive": "manual") wait on the landing pad; E gets in and
// out. Z/S throttle and brake, Q/D steer, the mouse looks around; they
// sweep the dust they roll over into the tank, plus their own hopper.
//
// Automatic ones ("drive": "auto") sweep on their own, into the tank too
// (it is teleported). Their "ai" is "follow" (they stay around the player,
// like the drones) or "patrol" (they go into the inventory, are set down
// with F and sweep around that spot, until E picks them back up).
type Vehicle struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Desc   string     `json:"desc"`
	Icon   string     `json:"icon"`   // in the shop: "vehicle" if left out
	Cost   float64    `json:"cost"`   // the first one...
	Growth float64    `json:"growth"` // ...times this for each one owned (automatic ones)
	Max    int        `json:"max"`    // how many can be owned: 1 for driven ones, 0 unlimited
	Drive  string     `json:"drive"`  // "manual" (default) or "auto"
	AI     string     `json:"ai"`     // automatic: "follow" (default) or "patrol"
	Range  float64    `json:"range"`  // automatic: how far from the player, or its spot, it looks for dust
	Speed  float64    `json:"speed"`  // m/s
	Turn   float64    `json:"turn"`   // driven: steering, radians per second at full speed
	Grip   float64    `json:"grip"`   // driven: how fast sideways sliding stops; low drifts (hover)
	Hover  float64    `json:"hover"`  // height it floats or flies at; 0 rolls on the ground
	Sweep  float64    `json:"sweep"`  // radius it sweeps under it; 0 doesn't
	Rate   float64    `json:"rate"`   // grams per second, like a tool's
	Hopper float64    `json:"hopper"` // driven: grams it adds to the tank while driven
	Seat   float64    `json:"seat"`   // driven: eye height, over the hover height
	Radius float64    `json:"radius"` // what the player bumps into on foot
	Light  vec3       `json:"light"`  // a status light, blinking green (red when the tank is full); 0,0,0 has none
	Parts  []PartJSON `json:"parts"`  // its shape, front toward +X
}

func (v *Vehicle) Auto() bool   { return v.Drive == "auto" }
func (v *Vehicle) Patrol() bool { return v.Auto() && v.AI == "patrol" }

// vehicleEntities gives each vehicle its entity, for drawing and E, and
// patrolling ones the item they are carried as.
func (c *Content) vehicleEntities() {
	for _, v := range c.Vehicles.All() {
		if v.Max == 0 && !v.Auto() {
			v.Max = 1
		}
		if _, ok := c.Entities.Get(v.ID); !ok {
			e := EntityDef{ID: v.ID, Name: v.Name, Parts: v.Parts, Radius: OrZero(v.Radius, 1.2), Prompt: "Conduire : " + v.Name}
			if v.Auto() {
				e.Radius, e.Prompt = v.Radius, ""
			}
			if v.Patrol() {
				e.Prompt, e.Pickup = "Ramasser : "+v.Name, true
			}
			c.Entities.pack = c.Vehicles.owner[v.ID]
			c.Entities.Add(v.ID, e)
		}
		if _, ok := c.Items.Get(v.ID); v.Patrol() && !ok {
			c.Items.pack = c.Vehicles.owner[v.ID]
			c.Items.Add(v.ID, Item{Name: v.Name, Desc: v.Desc, Icon: OrDefault(v.Icon, "vehicle"), Places: v.ID})
		}
	}
}

// Obstacle is a round piece of furniture the player walks around.
type Obstacle struct {
	At     vec3
	Radius float64
}

// Painted materials of the object shader (the mat* names are colors).
const (
	MtPlain  = 0
	MtPaint  = 1
	MtPanel  = 2
	MtMetal  = 3
	MtFloor  = 4
	MtFabric = 5
	MtRock   = 6
	MtHazard = 7
	MtDecal  = 8 // the model's own texture, by UVs (signs)
)

// Texture tiles per meter of the paint and fabric materials, which tokens
// made of them default to.
const (
	PaintTile  = 0.8
	FabricTile = 0.25
)

// Game is what packs see of the running game: their handlers, random
// events, furniture and debug lines act through it, never on the game's
// internals. Scripts get the same, in JavaScript (internal/script).
type Game interface {
	// Reading.
	Credits() float64
	Tank() float64     // grams carried
	Capacity() float64 // grams the tank holds
	ToolID() string
	Level(id string) int      // of an upgrade, or 1 for a vehicle owned
	SunHeight() float64       // sine of the sun's height over the player's horizon
	Inside() string           // ID of the building the player is in
	Clock() float64           // seconds played
	PlayerPos() vec3          // from Mars' center
	Driving() string          // ID of the vehicle driven
	Entities(kind string) int // placed entities of a kind, all of them for ""
	DustDevils() int
	Random() float64

	// Acting.
	Say(text string)
	GiveCredits(n float64) // negative takes them back, down to 0
	GiveItem(id string, n int)
	Burst(n int, c color.RGBA) // sparks around the player
	Beep(freq float64)
	Spawn(kind string) bool // an entity where the player aims, or right ahead
	SpawnDustDevil()
	OpenMap()
	OpenShop()
	Sleep()
	Deposit(at vec3) // sells the tank at the incinerator standing at a local point

	// ModData is what a pack keeps in the save, by key.
	ModData(pack string) map[string]any
}
