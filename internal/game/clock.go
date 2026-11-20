package game

import (
	"math"
	"naturalia/internal/planet"
)

// The clock: a sol, a day and a night, lasts Content.Terrain.Sol seconds
// of play (20 minutes by default). It runs while playing, in the menus of
// the world (shop, map, telescope), not on the pause screen. The hours are
// the base's; elsewhere on Mars it is earlier or later (planet.LocalHour).

// Dawn is the hour of the first morning, and of waking up.
const Dawn = 7.0

// BaseHour is the hour at the base, 0 to 24.
func (g *Game) BaseHour() float64 { return math.Mod(Dawn+24*g.State.Sol, 24) }

// LocalHour is the hour where p stands.
func (g *Game) LocalHour(p vec3) float64 { return planet.LocalHour(p.Norm(), g.BaseHour()) }

// SetLocalHour moves the clock forward to hour h where p stands (sleeping,
// the shots).
func (g *Game) SetLocalHour(p vec3, h float64) {
	g.State.Sol += math.Mod(h-g.LocalHour(p)+24, 24) / 24
	planet.SetSun(g.BaseHour())
}

// tick runs the clock and moves the sun.
func (g *Game) tick(dt float64) {
	if g.Mode != ModePaused && g.Mode != ModeSleep && g.Content.Terrain.Sol > 0 {
		g.State.Sol += dt / g.Content.Terrain.Sol
	}
	planet.SetSun(g.BaseHour())
}

// wakeAtDawn moves the clock to the next morning where the player sleeps.
func (g *Game) wakeAtDawn() { g.SetLocalHour(g.Player.Pos, Dawn) }
