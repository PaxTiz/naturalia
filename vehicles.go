package main

import (
	"mars_sweeper/internal/base"
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/planet"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// padSpot is where bought vehicles wait.
var padSpot = vec3{11, 0, -12.5}

// buyVehicle sets a new vehicle on the landing pad.
func (g *Game) buyVehicle(v *content.Vehicle) {
	g.st.Levels[v.ID] = 1
	off := float64(len(g.st.Entities)%4) * 0.8
	g.spawn(v.ID, g.world.Relief.Surface(base.ToWorld(padSpot.Add(vec3{off - 1.2, 0, off - 1.2}))))
}

// driven is the vehicle the player drives, if any.
func (g *Game) driven() (*content.Vehicle, bool) {
	if g.driving < 0 || g.driving >= len(g.st.Entities) {
		return nil, false
	}
	return reg.Vehicles.Get(g.st.Entities[g.driving].Kind)
}

// getIn or out of the vehicle standing as entity i.
func (g *Game) getIn(i int) {
	g.driving = i
	g.player.Pos = g.world.Relief.Surface(g.st.Entities[i].Dir)
	g.sfx.Beep(0.02, 0.2, 0.12, 220, 330)
	g.emit(content.Event{Name: content.EvDrive, ID: g.st.Entities[i].Kind})
}

func (g *Game) getOut() {
	v, ok := g.driven()
	g.driving = -1
	g.player.speed, g.player.eyeH = 0, 0
	if !ok {
		return
	}
	side := g.player.Right().Scale(content.OrZero(v.Radius, 1.2) + 0.6)
	g.player.Pos = g.world.Relief.Surface(g.player.Pos.Add(side))
	g.sfx.Beep(0.02, 0.2, 0.12, 330, 220)
	g.emit(content.Event{Name: content.EvDrive, ID: ""})
}

// updateVehicle moves the driven vehicle with the player, and sweeps.
func (g *Game) updateVehicle(dt float64) {
	v, ok := g.driven()
	if !ok {
		g.driving = -1
		return
	}
	if g.mode == modePlay && rl.IsKeyPressed(rl.KeyE) {
		g.getOut()
		return
	}
	g.player.speed, g.player.eyeH = v.Speed, content.OrZero(v.Seat, 2.4)
	e := &g.st.Entities[g.driving]
	e.Dir = g.player.Pos.Norm()
	t1, t2 := mathx.TangentBasis(e.Dir)
	e.Yaw = math.Atan2(g.player.Heading.Dot(t2), g.player.Heading.Dot(t1))

	if v.Sweep <= 0 || g.mode != modePlay || planet.ArcDist(e.Dir, planet.BaseUp) < base.Reach {
		return
	}
	at := g.world.Relief.Surface(e.Dir)
	got := g.world.Clean(at, v.Sweep, v.Rate*g.powerMult()*dt, g.space())
	g.collect(got, at, v.Sweep, at.Add(e.Dir.Scale(0.8)))
	for _, gem := range g.world.CollectGems(at, v.Sweep) {
		g.collectGem(gem)
	}
}
