package main

import "math"

// The event bus (docs/MODS.md): the game says what happens, packs listen.
// The base game listens like any mod does: its dust devils are a random
// event and move on "tick".

// Events the game emits, and what they carry.
const (
	evTick  = "tick"           // every frame of play: Dt
	evDust  = "dust_collected" // Amount grams, At where it was swept
	evGem   = "gem_collected"  // Amount credits, At
	evSell  = "sell"           // Amount credits
	evBuy   = "buy"            // ID of the tool or upgrade, Amount its price
	evUse   = "use"            // ID of the kit used, At (local)
	evEnter = "enter_building" // ID of the building
	evLeave = "leave_building" // ID of the building
	evWake  = "wake"           // the player gets up after sleeping
)

type Event struct {
	Name   string
	ID     string
	At     vec3
	Amount float64
	Dt     float64
}

type Handler func(g *Game, e *Event)

// RandomEvent happens now and then, while its condition holds: on average
// once every Every seconds.
type RandomEvent struct {
	ID    string
	Every float64
	When  func(g *Game) bool // nil: always
	Run   func(g *Game)
}

// On registers a handler for an event, for the pack being loaded.
func (c *Content) On(name string, h Handler) {
	if c.handlers == nil {
		c.handlers = map[string][]Handler{}
	}
	c.handlers[name] = append(c.handlers[name], h)
}

// emit runs the handlers of an event, in the order packs registered them.
func (g *Game) emit(e Event) {
	for _, h := range content.handlers[e.Name] {
		h(g, &e)
	}
}

// updateEvents ticks the listeners, rolls the random events and follows
// the player in and out of buildings.
func (g *Game) updateEvents(dt float64) {
	if g.mode != modePlay {
		return
	}
	g.emit(Event{Name: evTick, Dt: dt})
	for _, ev := range content.Events.All() {
		if ev.Every <= 0 || (ev.When != nil && !ev.When(g)) {
			continue
		}
		if g.rng.Float64() < -math.Expm1(-dt/ev.Every) {
			ev.Run(g)
		}
	}
	in := buildingAt(toLocal(g.player.Pos))
	if in != g.inside {
		if g.inside != "" {
			g.emit(Event{Name: evLeave, ID: g.inside})
		}
		if in != "" {
			g.emit(Event{Name: evEnter, ID: in})
		}
		g.inside = in
	}
}

// buildingAt is the ID of the building whose room holds a local point.
func buildingAt(local vec3) string {
	if local.Y > 3.5 {
		return ""
	}
	for i, r := range rooms {
		if math.Hypot(local.X-r.center.X, local.Z-r.center.Z) < r.radius {
			return plans[i].ID
		}
	}
	return ""
}
