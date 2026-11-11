package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/mathx"
	"naturalia/internal/planet"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// padSpot is where bought vehicles wait.
var padSpot = vec3{11, 0, -12.5}

// buyVehicle sets a new vehicle on the landing pad.
func (g *Game) buyVehicle(v *content.Vehicle) {
	g.State.Levels[v.ID] = 1
	off := float64(len(g.State.Entities)%4) * 0.8
	g.spawn(v.ID, g.World.Relief.Surface(base.ToWorld(padSpot.Add(vec3{off - 1.2, 0, off - 1.2}))))
}

// Driven is the vehicle the player drives, if any.
func (g *Game) Driven() (*content.Vehicle, bool) {
	if g.Riding < 0 || g.Riding >= len(g.State.Entities) {
		return nil, false
	}
	return g.Content.Vehicles.Get(g.State.Entities[g.Riding].Kind)
}

// getIn or out of the vehicle standing as entity i.
func (g *Game) getIn(i int) {
	g.Riding = i
	g.Player.Pos = g.World.Relief.Surface(g.State.Entities[i].Dir)
	g.sfx.Beep(0.02, 0.2, 0.12, 220, 330)
	g.emit(content.Event{Name: content.EvDrive, ID: g.State.Entities[i].Kind})
}

func (g *Game) getOut() {
	v, ok := g.Driven()
	g.Riding = -1
	g.Player.speed, g.Player.eyeH = 0, 0
	if !ok {
		return
	}
	side := g.Player.Right().Scale(content.OrZero(v.Radius, 1.2) + 0.6)
	g.Player.Pos = g.World.Relief.Surface(g.Player.Pos.Add(side))
	g.sfx.Beep(0.02, 0.2, 0.12, 330, 220)
	g.emit(content.Event{Name: content.EvDrive, ID: ""})
}

// updateVehicle moves the driven vehicle with the player, and sweeps.
func (g *Game) updateVehicle(dt float64) {
	v, ok := g.Driven()
	if !ok {
		g.Riding = -1
		return
	}
	if g.Mode == ModePlay && rl.IsKeyPressed(rl.KeyE) {
		g.getOut()
		return
	}
	g.Player.speed, g.Player.eyeH = v.Speed, content.OrZero(v.Seat, 2.4)
	e := &g.State.Entities[g.Riding]
	e.Dir = g.Player.Pos.Norm()
	t1, t2 := mathx.TangentBasis(e.Dir)
	e.Yaw = math.Atan2(g.Player.Heading.Dot(t2), g.Player.Heading.Dot(t1))

	if v.Sweep <= 0 || g.Mode != ModePlay || planet.ArcDist(e.Dir, planet.BaseUp) < base.Reach {
		return
	}
	at := g.World.Relief.Surface(e.Dir)
	got := g.World.Clean(at, v.Sweep, v.Rate*g.powerMult()*dt, g.space())
	g.collect(got, at, v.Sweep, at.Add(e.Dir.Scale(0.8)))
	for _, gem := range g.World.CollectGems(at, v.Sweep) {
		g.collectGem(gem)
	}
}
