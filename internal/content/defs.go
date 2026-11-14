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
	Hall   *Hall  `json:"hall"`   // a big rectangular hall instead of a dome
	Garage bool   `json:"garage"` // its floor has a parking bay for each driven vehicle
	Style  string `json:"style"`  // token of the walls and roof
	Accent string `json:"accent"` // token of the glowing stripes inside
	Doors  []Door `json:"doors"`
	Props  []Prop `json:"props"`
}

// Hall is a rectangular building, like an aircraft hangar: walls around
// its floor, a barrel vault along Z over them, and a giant sliding door
// in the middle of its east wall (+X), whose leaves slide away into
// pockets past the ends.
type Hall struct {
	Size  [2]float64 `json:"size"`        // along X and Z, in m
	WallH float64    `json:"wall_height"` // up to where the vault starts
	Rise  float64    `json:"rise"`        // the vault's height over the walls
	Door  [2]float64 `json:"door"`        // the big door's width and height
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
	Kit  string  `json:"kit"`
	At   vec3    `json:"at"`
	Seed int     `json:"seed"`
	Yaw  float64 `json:"yaw"` // degrees, turning the kit's X toward Z
}

// Turn is p turned by yaw degrees around the vertical, X toward Z.
func Turn(p vec3, yaw float64) vec3 {
	if yaw == 0 {
		return p
	}
	c, s := math.Cos(yaw*math.Pi/180), math.Sin(yaw*math.Pi/180)
	return vec3{p.X*c - p.Z*s, p.Y, p.X*s + p.Z*c}
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
	// Mass, in kg, makes it a thing pushed around, that bounces and flies
	// off when hit hard, the size of its parts; 0 stands fixed.
	Mass   float64 `json:"mass"`
	Prompt string  `json:"prompt"` // what E says; empty: nothing to do
	Pickup bool    `json:"pickup"` // E puts it back in the inventory
	Item   string  `json:"item"`   // the item it gives back (set by the item that places it)
	// Ramp lifts driven vehicles that roll over it: it rises along its
	// front (+X), from 0 at -length/2 to height at +length/2. The player
	// sets it facing away.
	Ramp *Ramp `json:"ramp"`
	// Kick throws a driven vehicle that crosses it forward, adding this
	// speed in m/s (a boost pad).
	Kick float64 `json:"kick"`
	// Build makes it a block to build with: set down turned any way, it
	// stays put, and the player walks on it.
	Build bool `json:"build"`
	// Trigger makes it a zone, walked and driven through: who comes in and
	// goes out says trigger_enter and trigger_leave. It spans radius, or
	// its parts without one, up to 1.5 m over them.
	Trigger bool `json:"trigger"`
	// Data is the pack's own components, anything JSON holds, for its
	// scripts (game.data).
	Data map[string]any `json:"data"`
}

type Ramp struct {
	Length float64 `json:"length"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
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
	Mass   float64    `json:"mass"`   // kg: how hard it hits and is hit (from its size if left out)
	Light  vec3       `json:"light"`  // a status light, blinking green (red when the tank is full); 0,0,0 has none
	Parts  []PartJSON `json:"parts"`  // its shape, front toward +X

	// Driven ones fly off crests and ramps, under Mars' gravity times
	// Gravity (2 if left out: Mars' alone makes them float); in the air,
	// Air (0.35) of the steering still works.
	Gravity float64 `json:"gravity"`
	Air     float64 `json:"air"`
	Jump    float64 `json:"jump"`   // Space: upward speed in m/s; 0 can't jump
	Jumps   int     `json:"jumps"`  // jumps before landing again: 2 jumps again in the air (1 if left out)
	Bounce  float64 `json:"bounce"` // share of a hard landing's speed it bounces back up with (0.25)
	Boost   float64 `json:"boost"`  // Shift: top speed times this; 0 has no boost
	Fuel    float64 `json:"fuel"`   // seconds of boost when full (3), refilled in twice that
	View    string  `json:"view"`   // "seat" (default) or "chase": the camera it starts with, V switches
	Chase   float64 `json:"chase"`  // chase camera's distance (from its radius if left out)
	// Fly makes it fly, driven like the others: Space climbs, Ctrl comes
	// down, and it holds its height over the ground otherwise, at any
	// speed; at the bottom it hovers (at hover). It speeds up gradually,
	// so that it can be handled slowly and still go fast.
	Fly *Flight `json:"fly"`
	// Flame is where its engine's exhaust comes out, pointing back along
	// -X, and how wide; none if 0,0,0.
	Flame  vec3    `json:"flame"`
	FlameR float64 `json:"flame_r"`
}

// Flight is how a flying vehicle climbs.
type Flight struct {
	Ceiling float64 `json:"ceiling"` // highest it goes over the ground, m (500)
	Climb   float64 `json:"climb"`   // fastest it goes up and down, m/s (40)
	Gain    float64 `json:"gain"`    // how fast it speeds up: this share of its speed per second (0.7)
}

// VehicleState is the driven vehicle seen by packs.
type VehicleState struct {
	ID       string
	Speed    float64 // m/s
	Air      bool    // off the ground
	Alt      float64 // m over the ground
	Airtime  float64 // seconds since it left the ground
	Fuel     float64 // share of the boost left, 0 to 1
	Boosting bool
}

func (v *Vehicle) Auto() bool   { return v.Drive == "auto" }
func (v *Vehicle) Patrol() bool { return v.Auto() && v.AI == "patrol" }

// Weight is its mass in kg: Mass, or from the ground it covers.
func (v *Vehicle) Weight() float64 {
	if v.Mass > 0 {
		return v.Mass
	}
	lo, hi := v.Footprint()
	return 350 * (hi.X - lo.X) * (hi.Z - lo.Z)
}

// Footprint is the ground its parts cover, front toward +X: the corners of
// a box around them, seen from above, never smaller than its radius.
func (v *Vehicle) Footprint() (lo, hi vec3) {
	r := OrZero(v.Radius, 1.2)
	lo, hi = Bounds(v.Parts)
	lo.X, lo.Z = math.Min(lo.X, -r), math.Min(lo.Z, -r)
	hi.X, hi.Z = math.Max(hi.X, r), math.Max(hi.Z, r)
	return vec3{lo.X, 0, lo.Z}, vec3{hi.X, 0, hi.Z}
}

// Bounds is the box around parts, in their frame (y up); nothing at all
// without parts.
func Bounds(parts []PartJSON) (lo, hi vec3) {
	if len(parts) == 0 {
		return vec3{}, vec3{}
	}
	inf := math.Inf(1)
	lo, hi = vec3{inf, inf, inf}, vec3{-inf, -inf, -inf}
	grow := func(c, e vec3) {
		lo = vec3{math.Min(lo.X, c.X-e.X), math.Min(lo.Y, c.Y-e.Y), math.Min(lo.Z, c.Z-e.Z)}
		hi = vec3{math.Max(hi.X, c.X+e.X), math.Max(hi.Y, c.Y+e.Y), math.Max(hi.Z, c.Z+e.Z)}
	}
	for _, p := range parts {
		switch p.Shape {
		case "prism":
			axis := p.Axis
			if axis == (vec3{}) {
				axis = vec3{0, 1, 0}
			}
			rr := math.Max(p.R0, OrZero(p.R1, p.R0))
			grow(p.At, vec3{rr, 0, rr})
			grow(p.At.Add(axis.Norm().Scale(p.H)), vec3{rr, 0, rr})
		default: // a box from its middle, or a wedge standing on at, maybe turned
			a := p.Yaw * math.Pi / 180
			c, s := math.Abs(math.Cos(a)), math.Abs(math.Sin(a))
			mid := p.At
			if p.Shape == "wedge" {
				mid.Y += p.Size.Y / 2
			}
			grow(mid, vec3{(p.Size.X*c + p.Size.Z*s) / 2, p.Size.Y / 2, (p.Size.X*s + p.Size.Z*c) / 2})
		}
	}
	return lo, hi
}

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

// EntityState is an entity of the world, for packs: ID stays its own
// while it lives.
type EntityState struct {
	ID   uint32
	Kind string
	At   vec3    // its foot on the ground, from Mars' center
	Alt  float64 // over the ground
	Dist float64 // from the player, m
}

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
	// Near are the entities of a kind ("": all) within dist m of the
	// player, the nearest first.
	Near(kind string, dist float64) []EntityState
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
	OpenShop()   // the research lab: tools, upgrades, drones and robots, items
	OpenGarage() // the garage's terminal: the vehicles to drive, bought and sold back
	Sleep()
	Deposit(at vec3) // sells the tank at the incinerator standing at a local point

	// The driven vehicle: what it does, and a push in m/s along its nose
	// and up.
	Vehicle() (VehicleState, bool)
	Push(forward, up float64)
	// Impulse adds speed v (m/s, world axes) to an entity near the player
	// pushed around (a crate, a robot) or driven: along the ground it
	// slides, up it flies off. False if it can't be.
	Impulse(id uint32, v vec3) bool

	// ModData is what a pack keeps in the save, by key.
	ModData(pack string) map[string]any
}
