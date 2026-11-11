package content

// The event bus (docs/MODS.md): the game says what happens, packs listen.
// The base game listens like any mod does: its dust devils are a random
// event.

// Events the game emits, and what they carry.
const (
	EvTick  = "tick"           // every frame of play: Dt
	EvDust  = "dust_collected" // Amount grams, At where it was swept
	EvGem   = "gem_collected"  // Amount credits, At
	EvSell  = "sell"           // Amount credits
	EvBuy   = "buy"            // ID of the tool or upgrade, Amount its price
	EvUse   = "use"            // ID of the kit used, At (local)
	EvEnter = "enter_building" // ID of the building
	EvLeave = "leave_building" // ID of the building
	EvWake  = "wake"           // the player gets up after sleeping
	EvPlace = "place"          // ID of the entity placed with an item, At
	EvDrive = "drive"          // ID of the vehicle the player gets in; empty when getting out
)

type Event struct {
	Name   string
	ID     string
	At     vec3
	Amount float64
	Dt     float64
}

type Handler func(g Game, e *Event)

// RandomEvent happens now and then, while its condition holds: on average
// once every Every seconds.
type RandomEvent struct {
	ID    string
	Every float64
	When  func(g Game) bool // nil: always
	Run   func(g Game)
}

// On registers a handler for an event, for the pack being loaded.
func (c *Content) On(name string, h Handler) {
	if c.handlers == nil {
		c.handlers = map[string][]Handler{}
	}
	c.handlers[name] = append(c.handlers[name], h)
}

// Handlers are the handlers of an event, in the order packs registered them.
func (c *Content) Handlers(name string) []Handler { return c.handlers[name] }
