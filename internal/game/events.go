package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
)

// emit runs the handlers of an event, in the order packs registered them.
func (g *Game) emit(e content.Event) {
	for _, h := range g.Content.Handlers(e.Name) {
		h(g, &e)
	}
}

// updateEvents ticks the listeners, rolls the random events and follows
// the player in and out of buildings.
func (g *Game) updateEvents(dt float64) {
	if g.Mode != ModePlay {
		return
	}
	g.moveDevils(dt)
	g.emit(content.Event{Name: content.EvTick, Dt: dt})
	for _, ev := range g.Content.Events.All() {
		if ev.Every <= 0 || (ev.When != nil && !ev.When(g)) {
			continue
		}
		if g.Rng.Float64() < -math.Expm1(-dt/ev.Every) {
			ev.Run(g)
		}
	}
	in := buildingAt(base.ToLocal(g.Player.Pos))
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
	for i, r := range base.Rooms {
		if math.Hypot(local.X-r.Center.X, local.Z-r.Center.Z) < r.Radius {
			return base.Plans[i].ID
		}
	}
	return ""
}
