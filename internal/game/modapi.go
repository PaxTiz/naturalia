package game

import (
	"cmp"
	"image/color"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/palette"
	"naturalia/internal/physics"
	"naturalia/internal/planet"
	"slices"
)

func (g *Game) Credits() float64       { return g.State.Credits }
func (g *Game) Tank() float64          { return g.State.Tank }
func (g *Game) ToolID() string         { return g.Tool().ID }
func (g *Game) Level(id string) int    { return g.State.Levels[id] }
func (g *Game) SunHeight() float64     { return planet.SunDir.Dot(g.Player.LocalUp()) }
func (g *Game) Inside() string         { return g.inside }
func (g *Game) Clock() float64         { return g.Elapsed }
func (g *Game) PlayerPos() vec3        { return g.Player.Pos }
func (g *Game) DustDevils() int        { return len(g.Devils) }
func (g *Game) Random() float64        { return g.Rng.Float64() }
func (g *Game) Beep(freq float64)      { g.sfx.Beep(0.05, 0.25, 0.14, freq) }
func (g *Game) SpawnDustDevil()        { spawnDevil(g) }
func (g *Game) OpenMap()               { g.setMode(ModeMap) }
func (g *Game) OpenShop()              { g.Garage = false; g.setMode(ModeShop) }
func (g *Game) OpenGarage()            { g.Garage = true; g.setMode(ModeShop) }
func (g *Game) Sleep()                 { g.Mode, g.SleepT = ModeSleep, 0 }
func (g *Game) Spawn(kind string) bool { return g.spawn(kind, g.placeAt()) != 0 }

func (g *Game) Driving() string {
	if v, ok := g.Driven(); ok {
		return v.ID
	}
	return ""
}

func (g *Game) Entities(kind string) int {
	if kind == "" {
		return g.Ents.Len()
	}
	n := 0
	for _, k := range g.Ents.Kind.All() {
		if *k == kind {
			n++
		}
	}
	return n
}

func (g *Game) GiveCredits(n float64) {
	if math.IsNaN(n) || math.IsInf(n, 0) {
		return
	}
	g.State.Credits = math.Max(0, g.State.Credits+n)
	if n > 0 {
		g.State.TotalEarned += n
		g.Floaters = append(g.Floaters, Floater{Pos: g.Player.Eye().Add(g.Player.Forward().Scale(2)), Life: 2, Text: "+" + FmtNum(n) + " cr", C: palette.Gold})
	}
}

func (g *Game) GiveItem(id string, n int) {
	if g.State.Items == nil {
		g.State.Items = map[string]int{}
	}
	g.State.Items[id] = max(0, g.State.Items[id]+n)
}

func (g *Game) Burst(n int, c color.RGBA) {
	at := g.Player.Eye().Add(g.Player.Forward().Scale(2))
	for range min(n, 200) {
		vel := vec3{g.Rng.Float64() - 0.5, g.Rng.Float64() - 0.5, g.Rng.Float64() - 0.5}.Add(g.Player.LocalUp().Scale(0.6)).Scale(4)
		g.addParticle(Particle{Pos: at, Vel: vel, MaxLife: 0.9, Size: 0.06, C: c, Falls: true, Additive: true})
	}
}

func (g *Game) ModData(pack string) map[string]any {
	if g.State.Mods == nil {
		g.State.Mods = map[string]map[string]any{}
	}
	if g.State.Mods[pack] == nil {
		g.State.Mods[pack] = map[string]any{}
	}
	return g.State.Mods[pack]
}

func (g *Game) Near(kind string, dist float64) []content.EntityState {
	var out []content.EntityState
	up := g.Player.LocalUp()
	for id, e := range g.Ents.Transform.All() {
		d := planet.ArcDist(up, e.Dir)
		if d > dist || (kind != "" && g.kind(id) != kind) {
			continue
		}
		out = append(out, content.EntityState{ID: uint32(id), Kind: g.kind(id), At: g.World.Relief.Surface(e.Dir), Alt: e.Alt, Dist: d})
	}
	slices.SortFunc(out, func(a, b content.EntityState) int { return cmp.Compare(a.Dist, b.Dist) })
	return out
}

func (g *Game) Impulse(id uint32, v vec3) bool {
	e := ecs.ID(id)
	if !g.Ents.Alive(e) || math.IsNaN(v.X+v.Y+v.Z) || math.IsInf(v.X+v.Y+v.Z, 0) {
		return false
	}
	t := g.Ents.Transform.Get(e)
	up := v.Dot(t.Dir)
	along := v.Sub(t.Dir.Scale(up))
	if e == g.Riding {
		d := &g.drive
		d.vel = d.vel.Add(along)
		d.vr += up
		d.air = d.air || up > 0
		return true
	}
	bp := g.Ents.Body.Get(e)
	if bp == nil || (*bp).Kind != physics.Dynamic || g.phys == nil {
		return false
	}
	b := *bp
	b.Vel = b.Vel.Add(g.phys.vecPlane(t.Dir.Scale(g.phys.r0), along))
	b.Wake()
	if def, ok := g.Content.Entities.Get(g.kind(e)); ok && def.Mass > 0 && up > 0 {
		a := g.Ents.Air.Get(e)
		if a == nil {
			a = g.Ents.Air.Set(e, Air{})
		}
		a.vz = math.Max(a.vz, up)
		g.rubs(e, false)
	}
	return true
}
