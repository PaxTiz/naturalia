package main

import (
	"image/color"
	"math"
)

// gameAPI is what packs see of the running game: their handlers, random
// events, furniture and debug lines act through it, never on the game's
// internals. Scripts get the same, in JavaScript (script.go).
type gameAPI interface {
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

func (g *Game) Credits() float64       { return g.st.Credits }
func (g *Game) Tank() float64          { return g.st.Tank }
func (g *Game) ToolID() string         { return g.tool().ID }
func (g *Game) Level(id string) int    { return g.st.Levels[id] }
func (g *Game) SunHeight() float64     { return sunDir.Dot(g.player.LocalUp()) }
func (g *Game) Inside() string         { return g.inside }
func (g *Game) Clock() float64         { return g.clock }
func (g *Game) PlayerPos() vec3        { return g.player.Pos }
func (g *Game) DustDevils() int        { return len(g.devils) }
func (g *Game) Random() float64        { return g.rng.Float64() }
func (g *Game) Beep(freq float64)      { g.sfx.beep(0.05, 0.25, 0.14, freq) }
func (g *Game) SpawnDustDevil()        { spawnDevil(g) }
func (g *Game) OpenMap()               { g.setMode(modeMap) }
func (g *Game) OpenShop()              { g.setMode(modeShop) }
func (g *Game) Sleep()                 { g.mode, g.sleepT = modeSleep, 0 }
func (g *Game) Spawn(kind string) bool { return g.spawn(kind, g.placeAt()) }

func (g *Game) Driving() string {
	if v, ok := g.driven(); ok {
		return v.ID
	}
	return ""
}

func (g *Game) Entities(kind string) int {
	n := 0
	for _, e := range g.st.Entities {
		if kind == "" || e.Kind == kind {
			n++
		}
	}
	return n
}

func (g *Game) GiveCredits(n float64) {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return
	}
	g.st.Credits = math.Max(0, g.st.Credits+n)
	if n > 0 {
		g.st.TotalEarned += n
		g.floaters = append(g.floaters, Floater{Pos: g.player.Eye().Add(g.player.Forward().Scale(2)), Life: 2, Text: "+" + fmtNum(n) + " cr", C: colGold})
	}
}

func (g *Game) GiveItem(id string, n int) {
	if g.st.Items == nil {
		g.st.Items = map[string]int{}
	}
	g.st.Items[id] = max(0, g.st.Items[id]+n)
}

func (g *Game) Burst(n int, c color.RGBA) {
	at := g.player.Eye().Add(g.player.Forward().Scale(2))
	for range min(n, 200) {
		vel := vec3{g.rng.Float64() - 0.5, g.rng.Float64() - 0.5, g.rng.Float64() - 0.5}.Add(g.player.LocalUp().Scale(0.6)).Scale(4)
		g.addParticle(Particle{Pos: at, Vel: vel, MaxLife: 0.9, Size: 0.06, C: c, Falls: true, Additive: true})
	}
}

func (g *Game) ModData(pack string) map[string]any {
	if g.st.Mods == nil {
		g.st.Mods = map[string]map[string]any{}
	}
	if g.st.Mods[pack] == nil {
		g.st.Mods[pack] = map[string]any{}
	}
	return g.st.Mods[pack]
}
