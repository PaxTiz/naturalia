package game

import (
	"image/color"
	"mars_sweeper/internal/audio"
	"mars_sweeper/internal/base"
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/palette"
	"mars_sweeper/internal/planet"
	"math"
	"math/rand/v2"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// State is the player's progress that gets saved.
type State struct {
	Credits     float64                   `json:"credits"`
	Tank        float64                   `json:"tank"`
	Tool        string                    `json:"tool"`                // ID of the held tool
	Levels      map[string]int            `json:"levels"`              // by upgrade ID; a removed mod's stay
	Mods        map[string]map[string]any `json:"mods,omitempty"`      // what each mod's scripts keep
	Items       map[string]int            `json:"items,omitempty"`     // carried, by item ID
	Entities    []Placed                  `json:"entities,omitempty"`  // standing in the world
	OldTier     int                       `json:"tool_tier,omitempty"` // before IDs: index of the tool
	TotalDust   float64                   `json:"total_dust"`
	TotalEarned float64                   `json:"total_earned"`
}

type Mode int

const (
	ModePaused Mode = iota
	ModePlay
	ModeShop  // research lab: upgrades
	ModeMap   // map table: state of Mars
	ModeSleep // fading out, sleeping, waking up
)

// Sleep timeline, in seconds: fade out, sleep (the game saves), fade in.
const (
	SleepFadeOut = 1.2
	SleepSave    = 2.2
	SleepWake    = 3.2
	SleepEnd     = 4.4
)

const (
	maxParticles  = 900
	saveInterval  = 30.0
	droneSpeed    = 5.0
	droneHover    = 1.4
	ToolViewScale = 0.6 // the held tool is drawn smaller so it doesn't hide the brush
)

type Drone struct {
	Pos      vec3
	Target   vec3
	retarget float64
	Phase    float64
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

// Game is the running game: the player, the world, the progress, and
// everything moving in it. It knows nothing of how it is drawn.
type Game struct {
	Content   *content.Content // everything the packs registered
	State     State
	Player    Player
	World     *planet.World
	Rng       *rand.Rand
	Drones    []*Drone
	Particles []Particle
	Floaters  []Floater
	NearRocks []planet.Rock // boulders around the player
	sfx       *audio.Synth
	Fov       float64 // degrees, eased by updateFov
	Steps     Footprints
	Devils    []*DustDevil
	inside    string // ID of the building the player is in

	Mode        Mode
	Elapsed     float64 // seconds played
	saveTimer   float64
	SellPop     float64 // timer of the "+X cr" pop next to the credits
	SellPopText string
	Message     string  // said by a mod (say), shown on the visor...
	MessageT    float64 // ...for this long still
	itemSel     int     // which carried item F places (R cycles)
	Riding      int     // index in State.Entities of the vehicle driven, -1 on foot

	walls    []base.Segment // the base's walls, for collisions
	DoorOpen float64        // airlock doors: 0 closed, 1 open
	Burn     float64        // incinerator glow after a deposit
	SleepT   float64        // time into the sleep sequence
	wakeUp   bool           // a new game starts by waking up in bed

	Aim       vec3 // where the tool points on the ground
	AimOK     bool
	Sweeping  bool
	lastSweep vec3
	ToolBase  vec3 // held tool: where the hand grips it...
	ToolDir   vec3 // ...and where it points
	toolTip   vec3 // world position of the tool's head, where dust is sucked to
}

// New starts the game from the last save, or a fresh Mars.
func New(c *content.Content) *Game {
	g := &Game{
		Content: c,
		Rng:     rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
		Riding:  -1,
		sfx:     audio.New(),
	}
	g.load()
	if g.State.Levels == nil {
		g.State.Levels = map[string]int{}
	}
	if g.State.Tool == "" {
		g.State.Tool = g.Content.Tools.IDs()[0]
	}
	fresh := g.World == nil
	if fresh {
		g.World = planet.NewWorld(g.Rng.Uint64(), &g.Content.Terrain)
	}
	base.Origin = planet.BaseUp.Scale(planet.Radius + g.World.Relief.BaseHeight())
	if fresh {
		g.Player = Player{Pos: base.ToWorld(base.SpawnSpot), Heading: vec3{0, 0, -1}}
		g.wakeUp = true
	}
	g.Player.Pos = g.World.Relief.Surface(g.Player.Pos)
	g.walls = base.Walls()
	g.syncDrones()
	return g
}

func (g *Game) setMode(m Mode) {
	g.Mode = m
	if m == ModePlay {
		rl.DisableCursor()
	} else {
		rl.EnableCursor()
	}
}

func (g *Game) Update(dt float64) {
	g.Elapsed += dt

	if rl.IsKeyPressed(rl.KeyF11) {
		rl.ToggleBorderlessWindowed()
	}
	closeKeys := rl.IsKeyPressed(rl.KeyEscape) || rl.IsKeyPressed(rl.KeyE) || rl.IsKeyPressed(rl.KeyTab)
	switch g.Mode {
	case ModePaused:
		if rl.IsMouseButtonPressed(rl.MouseButtonLeft) {
			g.setMode(ModePlay)
			if g.wakeUp {
				g.wakeUp = false
				g.Mode, g.SleepT = ModeSleep, SleepWake
			}
		}
	case ModePlay:
		switch {
		case rl.IsKeyPressed(rl.KeyEscape) || !rl.IsWindowFocused():
			g.setMode(ModePaused)
		case rl.IsKeyPressed(rl.KeyE) && g.Riding < 0:
			if it, ok := g.NearbyUse(); ok {
				g.use(it)
			}
		}
	case ModeShop:
		if closeKeys {
			g.setMode(ModePlay)
		}
	case ModeMap:
		if closeKeys {
			g.setMode(ModePlay)
		}
	case ModeSleep:
		g.updateSleep(dt)
	}

	g.NearRocks = g.World.RocksNear(g.Player.Pos, RockDrawDist)
	g.Player.Update(dt, g.World.Relief, g.Mode == ModePlay)
	if g.Steps.update(&g.Player) {
		g.sfx.Step()
	}
	g.sfx.Outside = !base.Indoors(base.ToLocal(g.Player.Eye()))
	g.sfx.Sweeping = g.Sweeping
	g.sfx.Power = math.Min(1, (g.powerMult()-1)/3)
	g.sfx.Update()
	g.updateFov(dt)
	g.Player.Pos = base.Collide(g.Player.Pos, g.walls)
	g.collideRocks()
	g.updateVehicle(dt)
	g.collideEntities()
	g.updateItems()
	g.updateBase(dt)
	g.updateAim()
	g.updateSweeping(dt)
	g.updateDrones(dt)
	g.updateEffects(dt)
	g.updateEvents(dt)
	if g.TankFull() && g.AutoSell() {
		g.sell()
	}

	g.saveTimer += dt
	if g.saveTimer >= saveInterval {
		g.saveTimer = 0
		g.Save()
	}
}

func (g *Game) updateAim() {
	p := &g.Player
	eye, fwd := p.Eye(), p.Forward()
	g.Aim, g.AimOK = g.World.Relief.Raycast(eye, fwd, g.Tool().Reach)
	if g.AimOK && planet.ArcDist(g.Aim.Norm(), planet.BaseUp) < base.Reach {
		g.AimOK = false // the ground under the base isn't swept
	}

	// The held tool: lower right of the view, pointing at the aimed spot.
	bob := math.Sin(p.WalkPhase) * 0.015 * p.MoveAmount
	base := eye.Add(fwd.Scale(0.3)).Add(p.Right().Scale(0.26)).Add(p.Up().Scale(-0.24 + bob))
	dir := fwd
	if g.AimOK {
		dir = g.Aim.Sub(base).Norm()
	}
	look := g.Tool().Look
	g.ToolBase, g.ToolDir = base, dir
	g.toolTip = base.Add(dir.Scale((look.HandleLen + look.HeadLen) * ToolViewScale))
}

func (g *Game) updateSweeping(dt float64) {
	if g.Mode != ModePlay || !g.AimOK || !rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		g.Sweeping = false
		return
	}
	tool := g.Tool()
	amount := tool.Rate * g.powerMult() * dt

	// Brush along the whole path so fast strokes don't skip cells.
	from, steps := g.Aim, 1
	if g.Sweeping {
		from = g.lastSweep
		steps = min(8, int(g.Aim.Sub(from).Length()/(tool.Radius*0.5))+1)
	}
	for s := 1; s <= steps; s++ {
		p := from.Lerp(g.Aim, float64(s)/float64(steps))
		g.collect(g.World.Clean(p, tool.Radius, amount, g.space()), p, tool.Radius, g.toolTip)
	}
	for _, gem := range g.World.CollectGems(g.Aim, tool.Radius) {
		g.collectGem(gem)
	}
	g.lastSweep, g.Sweeping = g.Aim, true
}

// collect adds dust to the tank, with particles sucked from the ground
// toward sink (the tool or a drone).
func (g *Game) collect(grams float64, at vec3, radius float64, sink vec3) {
	if grams <= 0 {
		return
	}
	g.State.Tank += grams
	g.State.TotalDust += grams
	g.emit(content.Event{Name: content.EvDust, At: at, Amount: grams})
	n := min(8, int(grams*4+g.Rng.Float64()))
	up := at.Norm()
	t1, t2 := mathx.TangentBasis(up)
	for range n {
		a := g.Rng.Float64() * 2 * math.Pi
		r := math.Sqrt(g.Rng.Float64()) * radius
		ground := g.World.Relief.Surface(at.Add(t1.Scale(math.Cos(a) * r)).Add(t2.Scale(math.Sin(a) * r)))
		start := ground.Add(up.Scale(0.05))
		travel := 0.35 + g.Rng.Float64()*0.3
		g.addParticle(Particle{
			Pos: start, Vel: sink.Sub(start).Scale(1 / travel),
			MaxLife: travel, Size: 0.03 + g.Rng.Float64()*0.04,
			C: color.RGBA{210, 140, 95, 255},
		})
	}
}

func (g *Game) collectGem(gem planet.Gem) {
	v := g.gemValue()
	g.State.Credits += v
	g.State.TotalEarned += v
	g.emit(content.Event{Name: content.EvGem, At: gem.Dir, Amount: v})
	pos := g.World.Relief.Surface(gem.Dir).Add(gem.Dir.Scale(0.3))
	g.Floaters = append(g.Floaters, Floater{Pos: pos.Add(gem.Dir.Scale(0.4)), Life: 1.5, Text: "+" + FmtNum(v) + " cr", C: palette.Cyan})
	g.sfx.Beep(0.07, 0.3, 0.16, 880, 1175, 1568)
	for range 20 {
		vel := vec3{g.Rng.Float64() - 0.5, g.Rng.Float64() - 0.5, g.Rng.Float64() - 0.5}.Add(gem.Dir.Scale(0.6)).Scale(4)
		g.addParticle(Particle{Pos: pos, Vel: vel, MaxLife: 0.7, Size: 0.06, C: palette.Cyan, Falls: true, Additive: true})
	}
}

func (g *Game) sell() {
	if g.State.Tank <= 0 {
		return
	}
	earned := g.State.Tank * g.priceMult()
	g.State.Credits += earned
	g.State.TotalEarned += earned
	g.State.Tank = 0
	g.SellPop, g.SellPopText = 1.5, "+"+FmtNum(earned)+" cr"
	g.emit(content.Event{Name: content.EvSell, Amount: earned})
	g.sfx.Beep(0.08, 0.22, 0.14, 660, 990)
}

func (g *Game) syncDrones() {
	for len(g.Drones) < int(g.stat("drones")) {
		p := base.ToWorld(vec3{0, 8, 0})
		g.Drones = append(g.Drones, &Drone{Pos: p, Target: base.Origin, Phase: g.Rng.Float64() * 10})
	}
}

const (
	// droneRange is how far from the player drones look for dust: they
	// follow you around Mars.
	droneRange = 25.0
	// While the player is at the base, drones circle above the hub.
	dockAltitude = 7.5
	dockRadius   = 3.5
)

func (g *Game) updateDrones(dt float64) {
	radius := 0.8 + float64(g.ToolTier())*0.5
	amount := 0.6 * g.powerMult() * dt
	docked := planet.ArcDist(g.Player.LocalUp(), planet.BaseUp) < base.Reach+15
	for i, d := range g.Drones {
		d.Phase += dt
		up := d.Pos.Norm()
		hover := droneHover
		switch {
		case docked:
			a := float64(i)/float64(len(g.Drones))*2*math.Pi + g.Elapsed*0.2
			d.Target = base.ToWorld(vec3{math.Cos(a) * dockRadius, 0, math.Sin(a) * dockRadius})
			hover = dockAltitude
		case g.TankFull():
			hover += 0.5 // tank full: drones hover in place, waiting
		default:
			d.retarget -= dt
			arrived := up.Sub(d.Target.Norm()).Length()*planet.Radius < 0.3
			if d.retarget <= 0 || (arrived && g.World.DustAt(d.Target) < 0.05) || planet.ArcDist(d.Target.Norm(), planet.BaseUp) < base.Reach {
				d.Target = g.World.Relief.Surface(g.World.FindDusty(g.Rng, g.Player.Pos, droneRange))
				d.retarget = 8 + g.Rng.Float64()*5
			}
		}
		if g.TankFull() && !docked {
			d.Target = d.Pos
		}
		delta := d.Target.Sub(d.Pos)
		delta = delta.Sub(up.Scale(delta.Dot(up))) // stay along the ground
		if delta.Length() > 0.05 {
			d.Pos = d.Pos.Add(delta.Norm().Scale(math.Min(delta.Length(), droneSpeed*dt)))
			up = d.Pos.Norm()
		}
		if planet.ArcDist(up, planet.BaseUp) < base.Reach+6 {
			hover = math.Max(hover, dockAltitude) // fly over the domes, not through them
		}
		r := d.Pos.Length()
		r += (g.World.Relief.SurfaceRadius(up) + hover - r) * math.Min(1, dt*3)
		d.Pos = up.Scale(r)
		if docked || g.TankFull() || planet.ArcDist(up, planet.BaseUp) < base.Reach {
			continue
		}
		g.collect(g.World.Clean(up, radius, amount, g.space()), g.World.Relief.Surface(up), radius, d.Pos)
		for _, gem := range g.World.CollectGems(up, radius) {
			g.collectGem(gem)
		}
	}
}

// updateBase animates the airlock doors and the incinerator glow.
func (g *Game) updateBase(dt float64) {
	open := 0.0
	if base.ToLocal(g.Player.Pos).Sub(base.AirlockEnd).Length() < 4 {
		open = 1
	}
	g.DoorOpen += (open - g.DoorOpen) * math.Min(1, dt*4)
	g.Burn = math.Max(0, g.Burn-dt*0.5)
}

// use runs what the player activates with E.
func (g *Game) use(it base.Interactable) {
	it.Use(g, it.At)
	g.emit(content.Event{Name: content.EvUse, ID: it.Kit, At: it.At})
}

// Deposit feeds the tank's dust to the incinerator at the local point in,
// which turns it into credits.
func (g *Game) Deposit(in vec3) {
	hopper := base.ToWorld(in.Add(vec3{0, 2.6, 0}))
	if g.State.Tank < 1 {
		g.Floaters = append(g.Floaters, Floater{Pos: hopper.Add(vec3{0, 0.5, 0}), Life: 1.5, Text: "Réservoir vide", C: palette.Gray})
		g.sfx.Beep(0, 0.25, 0.12, 220)
		return
	}
	earned := g.State.Tank * g.priceMult()
	g.sell()
	g.Burn = 2
	g.Floaters = append(g.Floaters, Floater{Pos: hopper.Add(vec3{0, 0.6, 0}), Life: 2, Text: "+" + FmtNum(earned) + " cr", C: palette.Gold})
	// A stream of dust flying from the player into the hopper.
	from := g.Player.Eye().Sub(g.Player.LocalUp().Scale(0.6))
	for range 60 {
		start := from.Add(vec3{g.Rng.Float64() - 0.5, g.Rng.Float64() - 0.5, g.Rng.Float64() - 0.5}.Scale(0.6))
		travel := 0.4 + g.Rng.Float64()*0.6
		g.addParticle(Particle{
			Pos: start, Vel: hopper.Sub(start).Scale(1 / travel),
			MaxLife: travel, Size: 0.04 + g.Rng.Float64()*0.05,
			C: color.RGBA{210, 140, 95, 255},
		})
	}
	for range 30 {
		vel := vec3{g.Rng.Float64() - 0.5, 1 + g.Rng.Float64(), g.Rng.Float64() - 0.5}.Scale(1.5)
		g.addParticle(Particle{Pos: hopper, Vel: vel, MaxLife: 0.6 + g.Rng.Float64()*0.6, Size: 0.06, C: palette.AccentOrange, Additive: true})
	}
}

// updateSleep runs the sleep sequence: fade out, save, wake up.
func (g *Game) updateSleep(dt float64) {
	before := g.SleepT
	g.SleepT += dt
	if before < SleepSave && g.SleepT >= SleepSave {
		g.Save()
	}
	if g.SleepT >= SleepEnd {
		g.setMode(ModePlay)
		g.emit(content.Event{Name: content.EvWake})
	}
}

func (g *Game) addParticle(p Particle) {
	if len(g.Particles) >= maxParticles {
		return
	}
	p.Life = p.MaxLife
	g.Particles = append(g.Particles, p)
}

// Say shows a message on the visor for a few seconds.
func (g *Game) Say(text string) {
	g.Message, g.MessageT = text, 4.5
	g.sfx.Beep(0.03, 0.12, 0.1, 740, 988)
}

func (g *Game) updateEffects(dt float64) {
	g.SellPop = math.Max(0, g.SellPop-dt)
	g.MessageT = math.Max(0, g.MessageT-dt)

	kept := g.Particles[:0]
	for _, p := range g.Particles {
		p.Life -= dt
		if p.Life <= 0 {
			continue
		}
		p.Pos = p.Pos.Add(p.Vel.Scale(dt))
		if p.Falls {
			p.Vel = p.Vel.Sub(p.Pos.Norm().Scale(marsGravity * dt))
		}
		kept = append(kept, p)
	}
	g.Particles = kept

	keptF := g.Floaters[:0]
	for _, f := range g.Floaters {
		f.Life -= dt
		f.Pos = f.Pos.Add(f.Pos.Norm().Scale(0.6 * dt))
		if f.Life > 0 {
			keptF = append(keptF, f)
		}
	}
	g.Floaters = keptF
}

// updateFov widens the view when running and narrows it a touch while
// sweeping, smoothly.
func (g *Game) updateFov(dt float64) {
	target := 70.0
	switch {
	case g.Sweeping:
		target = 67
	case rl.IsKeyDown(rl.KeyLeftShift) && g.Player.MoveAmount > 0.5:
		target = 76
	}
	if g.Fov == 0 {
		g.Fov = target
	}
	g.Fov += (target - g.Fov) * (1 - math.Exp(-dt*5))
}

// LoadContent loads the game's content, with extra packs for tests, then
// lays out the base.
func LoadContent(extra ...content.Pack) *content.Content {
	c := content.Build(extra...)
	base.Setup(c)
	return c
}
