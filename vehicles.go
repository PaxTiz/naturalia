package main

import (
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/planet"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Vehicles are entities the player drives: bought in the shop, they wait
// on the landing pad. E gets in and out; driving, the player goes faster,
// sits higher, and the vehicle sweeps the dust it rolls over.
type Vehicle struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Desc   string     `json:"desc"`
	Cost   float64    `json:"cost"`
	Speed  float64    `json:"speed"`  // m/s
	Sweep  float64    `json:"sweep"`  // radius it sweeps under it; 0 doesn't
	Rate   float64    `json:"rate"`   // grams per second, like a tool's
	Seat   float64    `json:"seat"`   // eye height when driving
	Radius float64    `json:"radius"` // what the player bumps into on foot
	Parts  []partJSON `json:"parts"`  // its shape, front toward +X
}

// padSpot is where bought vehicles wait.
var padSpot = vec3{11, 0, -12.5}

// vehicleEntities gives each vehicle its entity, for drawing and E.
func (c *Content) vehicleEntities() {
	for _, v := range c.Vehicles.All() {
		if _, ok := c.Entities.Get(v.ID); ok {
			continue
		}
		c.Entities.pack = c.Vehicles.owner[v.ID]
		c.Entities.Add(v.ID, EntityDef{ID: v.ID, Name: v.Name, Parts: v.Parts, Radius: orZero(v.Radius, 1.2), Prompt: "Conduire : " + v.Name})
	}
}

// buyVehicle sets a new vehicle on the landing pad.
func (g *Game) buyVehicle(v *Vehicle) {
	g.st.Levels[v.ID] = 1
	off := float64(len(g.st.Entities)%4) * 0.8
	g.spawn(v.ID, g.terrain.Surface(toWorld(padSpot.Add(vec3{off - 1.2, 0, off - 1.2}))))
}

// driven is the vehicle the player drives, if any.
func (g *Game) driven() (*Vehicle, bool) {
	if g.driving < 0 || g.driving >= len(g.st.Entities) {
		return nil, false
	}
	return reg.Vehicles.Get(g.st.Entities[g.driving].Kind)
}

// getIn or out of the vehicle standing as entity i.
func (g *Game) getIn(i int) {
	g.driving = i
	g.player.Pos = g.terrain.Surface(g.st.Entities[i].Dir)
	g.sfx.beep(0.02, 0.2, 0.12, 220, 330)
	g.emit(Event{Name: evDrive, ID: g.st.Entities[i].Kind})
}

func (g *Game) getOut() {
	v, ok := g.driven()
	g.driving = -1
	g.player.speed, g.player.eyeH = 0, 0
	if !ok {
		return
	}
	side := g.player.Right().Scale(orZero(v.Radius, 1.2) + 0.6)
	g.player.Pos = g.terrain.Surface(g.player.Pos.Add(side))
	g.sfx.beep(0.02, 0.2, 0.12, 330, 220)
	g.emit(Event{Name: evDrive, ID: ""})
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
	g.player.speed, g.player.eyeH = v.Speed, orZero(v.Seat, 2.4)
	e := &g.st.Entities[g.driving]
	e.Dir = g.player.Pos.Norm()
	t1, t2 := mathx.TangentBasis(e.Dir)
	e.Yaw = math.Atan2(g.player.Heading.Dot(t2), g.player.Heading.Dot(t1))

	if v.Sweep <= 0 || g.mode != modePlay || planet.ArcDist(e.Dir, planet.BaseUp) < baseReach {
		return
	}
	at := g.terrain.Surface(e.Dir)
	got := g.world.Clean(at, v.Sweep, v.Rate*g.powerMult()*dt, g.space())
	g.collect(got, at, v.Sweep, at.Add(e.Dir.Scale(0.8)))
	for _, gem := range g.world.CollectGems(at, v.Sweep) {
		g.collectGem(gem)
	}
}
