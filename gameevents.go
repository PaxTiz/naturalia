package main

import (
	"mars_sweeper/internal/content"
	"math"
)

// emit runs the handlers of an event, in the order packs registered them.
func (g *Game) emit(e content.Event) {
	for _, h := range reg.Handlers(e.Name) {
		h(g, &e)
	}
}

// updateEvents ticks the listeners, rolls the random events and follows
// the player in and out of buildings.
func (g *Game) updateEvents(dt float64) {
	if g.mode != modePlay {
		return
	}
	g.moveDevils(dt)
	g.emit(content.Event{Name: content.EvTick, Dt: dt})
	for _, ev := range reg.Events.All() {
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
			g.emit(content.Event{Name: content.EvLeave, ID: g.inside})
		}
		if in != "" {
			g.emit(content.Event{Name: content.EvEnter, ID: in})
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
