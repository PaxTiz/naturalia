package main

import (
	"image/color"
	"math"
)

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
