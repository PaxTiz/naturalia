package main

import (
	"image/color"
	"math"
	"math/rand/v2"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// State is the player's progress that gets saved.
type State struct {
	Credits     float64        `json:"credits"`
	Tank        float64        `json:"tank"`
	ToolTier    int            `json:"tool_tier"`
	Levels      map[string]int `json:"levels"`
	TotalDust   float64        `json:"total_dust"`
	TotalEarned float64        `json:"total_earned"`
}

type mode int

const (
	modePaused mode = iota
	modePlay
	modeShop
)

const (
	maxParticles  = 900
	saveInterval  = 30.0
	terminalReach = 7.0 // how close to the base the terminal can be used
	droneSpeed    = 5.0
	droneHover    = 1.4
	toolViewScale = 0.6 // the held tool is drawn smaller so it doesn't hide the brush
)

type Drone struct {
	Pos      vec3
	Target   vec3
	retarget float64
	phase    float64
}

type Particle struct {
	Pos, Vel      vec3
	Life, MaxLife float64
	Size          float64
	Falls         bool // affected by gravity (debris), unlike sucked-up dust
	Additive      bool // glowing sparks rather than dust
	C             color.RGBA
}

// Floater is a "+12 cr" label rising from a point in the world.
type Floater struct {
	Pos  vec3
	Life float64
	Text string
	C    color.RGBA
}

type Game struct {
	st        State
	player    Player
	world     *World
	terrain   *Terrain
	gfx       *Graphics
	rng       *rand.Rand
	drones    []*Drone
	particles []Particle
	floaters  []Floater
	nearGems  []Gem // crystals around the player, refreshed every frame

	mode        mode
	clock       float64
	saveTimer   float64
	sellPop     float64 // timer of the "+X cr" pop next to the credits
	sellPopText string
	hover       int

	aim       vec3 // where the tool points on the ground
	aimOK     bool
	sweeping  bool
	lastSweep vec3
	toolBase  vec3 // held tool: where the hand grips it...
	toolDir   vec3 // ...and where it points
	toolTip   vec3 // world position of the tool's head, where dust is sucked to
}

func NewGame() *Game {
	g := &Game{
		rng:   rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
		hover: -1,
	}
	g.load()
	if g.st.Levels == nil {
		g.st.Levels = map[string]int{}
	}
	fresh := g.world == nil
	if fresh {
		g.world = NewWorld(g.rng.Uint64())
	}
	g.terrain = NewTerrain(g.world.Seed) // also sets basePos on the ground
	if fresh {
		g.player = Player{Pos: basePos.add(vec3{0, 0, -6}), Heading: vec3{0, 0, 1}}
	}
	g.player.Pos = g.terrain.Surface(g.player.Pos)
	terminalPos = g.terrain.Surface(basePos.add(vec3{1.8, 0, -3.4}))
	g.gfx = NewGraphics()
	g.terrain.Warm(g.player.Eye(), 5*time.Second)
	g.syncDrones()
	return g
}

func (g *Game) setMode(m mode) {
	g.mode = m
	if m == modePlay {
		rl.DisableCursor()
	} else {
		rl.EnableCursor()
	}
}

func (g *Game) nearTerminal() bool { return g.player.Pos.sub(basePos).length() < terminalReach }

func (g *Game) Update(dt float64) {
	g.clock += dt

	if rl.IsKeyPressed(rl.KeyF11) {
		rl.ToggleBorderlessWindowed()
	}
	switch g.mode {
	case modePaused:
		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			g.setMode(modePlay)
		}
	case modePlay:
		switch {
		case rl.IsKeyPressed(rl.KeyEscape) || !rl.IsWindowFocused():
			g.setMode(modePaused)
		case rl.IsKeyPressed(rl.KeyE) && g.nearTerminal():
			g.setMode(modeShop)
		}
	case modeShop:
		if rl.IsKeyPressed(rl.KeyEscape) || rl.IsKeyPressed(rl.KeyE) || rl.IsKeyPressed(rl.KeyTab) {
			g.setMode(modePlay)
		}
		if rl.IsKeyPressed(rl.KeySpace) {
			g.sell()
		}
		g.updateShop()
	}

	g.player.Update(dt, g.terrain, g.mode == modePlay)
	g.player.pushOut(basePos, 2.4)
	g.player.pushOut(terminalPos, 0.8)
	g.updateAim()
	g.updateSweeping(dt)
	g.updateDrones(dt)
	g.updateEffects(dt)
	if g.tankFull() && g.autoSell() {
		g.sell()
	}

	g.saveTimer += dt
	if g.saveTimer >= saveInterval {
		g.saveTimer = 0
		g.save()
	}
}

func (g *Game) updateAim() {
	p := &g.player
	eye, fwd := p.Eye(), p.Forward()
	g.aim, g.aimOK = g.terrain.Raycast(eye, fwd, g.tool().Reach)

	// The held tool: lower right of the view, pointing at the aimed spot.
	bob := math.Sin(p.walkPhase) * 0.015 * p.moveAmount
	base := eye.add(fwd.scale(0.3)).add(p.Right().scale(0.26)).add(p.Up().scale(-0.24 + bob))
	dir := fwd
	if g.aimOK {
		dir = g.aim.sub(base).norm()
	}
	look := g.tool().Look
	g.toolBase, g.toolDir = base, dir
	g.toolTip = base.add(dir.scale((look.HandleLen + look.HeadLen) * toolViewScale))
}

func (g *Game) updateSweeping(dt float64) {
	if g.mode != modePlay || !g.aimOK || !rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		g.sweeping = false
		return
	}
	tool := g.tool()
	amount := tool.Rate * g.powerMult() * dt

	// Brush along the whole path so fast strokes don't skip cells.
	from, steps := g.aim, 1
	if g.sweeping {
		from = g.lastSweep
		steps = min(8, int(g.aim.sub(from).length()/(tool.Radius*0.5))+1)
	}
	for s := 1; s <= steps; s++ {
		p := from.lerp(g.aim, float64(s)/float64(steps))
		g.collect(g.world.Clean(p, tool.Radius, amount, g.space()), p, tool.Radius, g.toolTip)
	}
	for _, gem := range g.world.CollectGems(g.aim, tool.Radius) {
		g.collectGem(gem)
	}
	g.lastSweep, g.sweeping = g.aim, true
}

// collect adds dust to the tank, with particles sucked from the ground
// toward sink (the tool or a drone).
func (g *Game) collect(grams float64, at vec3, radius float64, sink vec3) {
	if grams <= 0 {
		return
	}
	g.st.Tank += grams
	g.st.TotalDust += grams
	n := min(8, int(grams*4+g.rng.Float64()))
	up := at.norm()
	t1, t2 := tangentBasis(up)
	for range n {
		a := g.rng.Float64() * 2 * math.Pi
		r := math.Sqrt(g.rng.Float64()) * radius
		ground := g.terrain.Surface(at.add(t1.scale(math.Cos(a) * r)).add(t2.scale(math.Sin(a) * r)))
		start := ground.add(up.scale(0.05))
		travel := 0.35 + g.rng.Float64()*0.3
		g.addParticle(Particle{
			Pos: start, Vel: sink.sub(start).scale(1 / travel),
			MaxLife: travel, Size: 0.03 + g.rng.Float64()*0.04,
			C: color.RGBA{215, 205, 185, 255},
		})
	}
}

func (g *Game) collectGem(gem Gem) {
	v := g.gemValue()
	g.st.Credits += v
	g.st.TotalEarned += v
	pos := g.terrain.Surface(gem.Dir).add(gem.Dir.scale(0.3))
	g.floaters = append(g.floaters, Floater{Pos: pos.add(gem.Dir.scale(0.4)), Life: 1.5, Text: "+" + fmtNum(v) + " cr", C: colCyan})
	for range 20 {
		vel := vec3{g.rng.Float64() - 0.5, g.rng.Float64() - 0.5, g.rng.Float64() - 0.5}.add(gem.Dir.scale(0.6)).scale(4)
		g.addParticle(Particle{Pos: pos, Vel: vel, MaxLife: 0.7, Size: 0.06, C: colCyan, Falls: true, Additive: true})
	}
}

func (g *Game) sell() {
	if g.st.Tank <= 0 {
		return
	}
	earned := g.st.Tank * g.priceMult()
	g.st.Credits += earned
	g.st.TotalEarned += earned
	g.st.Tank = 0
	g.sellPop, g.sellPopText = 1.5, "+"+fmtNum(earned)+" cr"
}

func (g *Game) syncDrones() {
	for len(g.drones) < g.st.Levels["drone"] {
		p := basePos.add(vec3{0, 4, 0})
		g.drones = append(g.drones, &Drone{Pos: p, Target: basePos, phase: g.rng.Float64() * 10})
	}
}

// droneRange is how far from the player drones look for dust: they follow
// you around the Moon.
const droneRange = 25.0

func (g *Game) updateDrones(dt float64) {
	radius := 0.8 + float64(g.st.ToolTier)*0.5
	amount := 0.6 * g.powerMult() * dt
	for _, d := range g.drones {
		d.phase += dt
		up := d.Pos.norm()
		hover := droneHover
		if g.tankFull() {
			hover += 0.5 // tank full: drones hover in place, waiting
		} else {
			d.retarget -= dt
			arrived := up.sub(d.Target.norm()).length()*moonRadius < 0.3
			if d.retarget <= 0 || (arrived && g.world.DustAt(d.Target) < 0.05) {
				d.Target = g.terrain.Surface(g.world.FindDusty(g.rng, g.player.Pos, droneRange))
				d.retarget = 8 + g.rng.Float64()*5
			}
			if !arrived {
				delta := d.Target.sub(d.Pos)
				delta = delta.sub(up.scale(delta.dot(up))) // stay along the ground
				d.Pos = d.Pos.add(delta.norm().scale(math.Min(delta.length(), droneSpeed*dt)))
				up = d.Pos.norm()
			}
		}
		r := d.Pos.length()
		r += (g.terrain.SurfaceRadius(up) + hover - r) * math.Min(1, dt*3)
		d.Pos = up.scale(r)
		if g.tankFull() {
			continue
		}
		g.collect(g.world.Clean(up, radius, amount, g.space()), g.terrain.Surface(up), radius, d.Pos)
		for _, gem := range g.world.CollectGems(up, radius) {
			g.collectGem(gem)
		}
	}
}

func (g *Game) addParticle(p Particle) {
	if len(g.particles) >= maxParticles {
		return
	}
	p.Life = p.MaxLife
	g.particles = append(g.particles, p)
}

func (g *Game) updateEffects(dt float64) {
	g.sellPop = math.Max(0, g.sellPop-dt)

	kept := g.particles[:0]
	for _, p := range g.particles {
		p.Life -= dt
		if p.Life <= 0 {
			continue
		}
		p.Pos = p.Pos.add(p.Vel.scale(dt))
		if p.Falls {
			p.Vel = p.Vel.sub(p.Pos.norm().scale(moonGravity * dt))
		}
		kept = append(kept, p)
	}
	g.particles = kept

	keptF := g.floaters[:0]
	for _, f := range g.floaters {
		f.Life -= dt
		f.Pos = f.Pos.add(f.Pos.norm().scale(0.6 * dt))
		if f.Life > 0 {
			keptF = append(keptF, f)
		}
	}
	g.floaters = keptF
}

func (g *Game) updateShop() {
	items := g.shopItems()
	mouse := rl.GetMousePosition()
	g.hover = -1
	for i := range items {
		if rl.CheckCollisionPointRec(mouse, shopItemRect(i)) {
			g.hover = i
		}
	}
	if !rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
		return
	}
	if rl.CheckCollisionPointRec(mouse, sellButtonRect()) {
		g.sell()
	}
	if g.hover >= 0 {
		if it := items[g.hover]; it.affordable(g.st.Credits) {
			g.st.Credits -= it.cost
			it.buy()
		}
	}
}
