package game

import (
	"fmt"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/mathx"
	"naturalia/internal/physics"
	"naturalia/internal/planet"
)

// Vehicles (content.Vehicle) are entities that sweep. The ones the player
// drives wait on the landing pad; automatic ones work on their own: some
// follow the player, others sweep around the spot where they were set
// down. State.Levels counts how many of each were bought.

// padSpot is where bought vehicles wait.
var padSpot = vec3{11, 0, -12.5}

const (
	// While the player is at the base, followers circle above the hub...
	dockAltitude = 7.5
	dockRadius   = 3.5
	// ...and anywhere near it, they fly over the domes, not through them.
	domesReach = base.Reach + 6
)

// buyVehicle adds one to the fleet.
func (g *Game) buyVehicle(v *content.Vehicle) {
	g.State.Levels[v.ID]++
	g.syncFleet()
}

// entityOf is the first entity of a kind, or 0.
func (g *Game) entityOf(kind string) ecs.ID {
	for id := range g.Ents.All() {
		if g.kind(id) == kind {
			return id
		}
	}
	return 0
}

// sellVehicle sells a driven vehicle back for refund credits: it leaves,
// wherever it is, unless the player is in it.
func (g *Game) sellVehicle(v *content.Vehicle, refund float64) {
	id := g.entityOf(v.ID)
	if id != 0 && id == g.Riding {
		g.Say("Descends d'abord du véhicule")
		g.sfx.Beep(0, 0.15, 0.1, 180)
		return
	}
	g.Ents.Kill(id)
	g.State.Levels[v.ID] = max(0, g.State.Levels[v.ID]-1)
	g.State.Credits += refund
	g.emit(content.Event{Name: content.EvResell, ID: v.ID, Amount: refund})
	g.Say(fmt.Sprintf("%s revendu : +%s cr", v.Name, FmtNum(refund)))
	g.sfx.Beep(0.06, 0.18, 0.14, 784, 523)
}

// owned counts the vehicles of a kind standing in the world or carried.
func (g *Game) owned(v *content.Vehicle) int {
	return g.Entities(v.ID) + g.State.Items[v.ID]
}

// syncFleet brings every vehicle bought into the world: driven ones into
// their bay in the garage (or on the landing pad without one), followers
// next to the player, patrolling ones into the inventory. It also brings back the drones of saves where they were an
// upgrade, under the same ID.
func (g *Game) syncFleet() {
	parked := 0 // driven kinds so far: each has its spot on the pad
	for _, v := range g.Content.Vehicles.All() {
		if !v.Auto() && v.Fly == nil {
			parked++
		}
		for n := g.owned(v); n < g.State.Levels[v.ID]; n++ {
			switch {
			case v.Patrol():
				g.GiveItem(v.ID, 1)
			case v.Auto():
				if id := g.spawn(v.ID, g.Player.Pos); id != 0 {
					g.Ents.Transform.Get(id).Alt = v.Hover
				}
			case g.park(v, 0):
			default:
				off := float64(n%4) * 0.8
				spot := vec3{float64(parked%3)*6 - 6, 0, float64(parked/3) * -6}
				if v.Fly != nil {
					spot = vec3{} // what flies stands on the landing pad
				}
				g.spawn(v.ID, g.World.Relief.Surface(base.ToWorld(padSpot.Add(spot).Add(vec3{off - 1.2, 0, off - 1.2}))))
			}
		}
	}
}

// yawOf is the angle of the tangent direction fwd at up, as entities
// store it.
func yawOf(up, fwd vec3) float64 {
	t1, t2 := mathx.TangentBasis(up)
	return math.Atan2(fwd.Dot(t2), fwd.Dot(t1))
}

// offLimits tells whether the ground at dir is the base's, with no dust to
// sweep: around the domes, and under the halls' floors.
func offLimits(dir vec3) bool {
	return planet.ArcDist(dir, planet.BaseUp) < base.Reach+2 || base.Covered(base.ToLocal(dir.Scale(planet.Radius)))
}

// sweepUnder cleans the dust under a vehicle into the tank, away from the
// base's ground.
func (g *Game) sweepUnder(v *content.Vehicle, e *Transform, dt float64) {
	if v.Sweep <= 0 || offLimits(e.Dir) {
		return
	}
	at := g.World.Relief.Surface(e.Dir)
	got := g.World.Clean(at, v.Sweep, v.Rate*g.powerMult()*dt, g.space())
	g.collect(got, at, v.Sweep, at.Add(e.Dir.Scale(e.Alt+0.8)))
	for _, gem := range g.World.CollectGems(at, v.Sweep) {
		g.collectGem(gem)
	}
}

// updateFleet runs the automatic vehicles. They stop while the tank is
// full, waiting for it to be emptied.
func (g *Game) updateFleet(dt float64) {
	if g.fleetHeld {
		return
	}
	// In the hangar too, they wait over the hub.
	docked := planet.ArcDist(g.Player.LocalUp(), planet.BaseUp) < base.Reach+15 || base.Covered(base.ToLocal(g.Player.Pos))
	nth := map[string]int{} // followers of each kind so far, to share the circle above the hub
	for id, e := range g.Ents.Transform.All() {
		v, ok := g.vehicleOf(id)
		if !ok || !v.Auto() || id == g.held() {
			continue
		}
		r := g.Ents.Roam.Get(id)
		if r == nil {
			r = g.Ents.Roam.Set(id, Roam{})
		}
		bob := math.Sin(g.Elapsed*3+float64(id.Index())) * 0.08
		if v.Patrol() {
			g.patrol(id, v, e, r, dt)
			alt := 0.0
			if v.Hover > 0 {
				alt = math.Max(v.Hover, g.clearance(id)) + bob // over what stands below
			}
			e.Alt += (alt - e.Alt) * math.Min(1, dt*3)
		} else {
			g.follow(id, v, e, r, dt, docked, nth[v.ID], g.Entities(v.ID))
			nth[v.ID]++
			alt := v.Hover
			switch {
			case docked:
				alt = dockAltitude
			case g.TankFull():
				alt += 0.5 // waiting
			}
			if planet.ArcDist(e.Dir, planet.BaseUp) < domesReach {
				alt = math.Max(alt, dockAltitude)
			}
			if v.Hover > 0 {
				alt = math.Max(alt, g.clearance(id)) // over what stands below
			}
			e.Alt += (alt + bob - e.Alt) * math.Min(1, dt*3)
		}
		if !docked || v.Patrol() {
			if !g.TankFull() {
				g.sweepUnder(v, e, dt)
			}
		}
	}
}

// follow: the vehicle looks for dust around the player, and keeps up with
// them; at the base it circles above the hub with the others of its kind.
func (g *Game) follow(id ecs.ID, v *content.Vehicle, e *Transform, r *Roam, dt float64, docked bool, i, n int) {
	speed := content.OrZero(v.Speed, 5)
	spread := content.OrZero(v.Range, 25)
	switch {
	case docked:
		a := float64(i)/float64(n)*2*math.Pi + g.Elapsed*0.2
		r.target = base.ToWorld(vec3{math.Cos(a) * dockRadius, 0, math.Sin(a) * dockRadius}).Norm()
	case g.TankFull():
		return
	default:
		r.retarget -= dt
		arrived := planet.ArcDist(e.Dir, r.target) < 0.3
		if r.target == (vec3{}) || r.retarget <= 0 || (arrived && g.World.DustAt(r.target) < 0.05) || offLimits(r.target) {
			r.target = g.World.FindDusty(g.Rng, g.Player.Pos, spread)
			r.retarget = 8 + g.Rng.Float64()*5
		}
	}
	// Left behind (the player drives off), it hurries back.
	if far := planet.ArcDist(e.Dir, g.Player.LocalUp()); far > spread {
		speed *= far / spread
	}
	g.moveToward(id, e, r, r.target, speed, dt)
}

// patrol: the vehicle sweeps the dustiest spots around where it was set
// down, staying off the base's ground.
func (g *Game) patrol(id ecs.ID, v *content.Vehicle, e *Transform, r *Roam, dt float64) {
	if r.Home == (vec3{}) {
		r.Home = e.Dir
	}
	if g.TankFull() {
		return
	}
	r.retarget -= dt
	arrived := planet.ArcDist(e.Dir, r.target) < 0.5
	if r.target == (vec3{}) || r.retarget <= 0 || (arrived && g.World.DustAt(r.target) < 0.05) {
		r.target = g.World.FindDusty(g.Rng, r.Home.Scale(planet.Radius), content.OrZero(v.Range, 30))
		r.retarget = 10 + g.Rng.Float64()*6
		if offLimits(r.target) {
			r.target = e.Dir // try again next time
			r.retarget = 0.5
		}
	}
	g.moveToward(id, e, r, r.target, content.OrZero(v.Speed, 3), dt)
}

// moveToward rolls or flies an entity along the ground toward the
// direction to, turning its nose that way. Near the player, it asks its
// body for that speed instead, and the collisions move it (fleetBefore).
func (g *Game) moveToward(id ecs.ID, e *Transform, r *Roam, to vec3, speed, dt float64) {
	dist := planet.ArcDist(e.Dir, to)
	if dist < 0.05 {
		return
	}
	fwd := planet.TangentAt(e.Dir, to)
	if bp := g.Ents.Body.Get(id); bp != nil && (*bp).Kind == physics.Dynamic {
		r.want = fwd.Scale(math.Min(speed, dist*3)) // slowing down to stop on it
		return
	}
	step := math.Min(dist, speed*dt)
	e.Dir = e.Dir.Scale(planet.Radius).Add(fwd.Scale(step)).Norm()
	want := yawOf(e.Dir, fwd)
	turn := math.Remainder(want-e.Yaw, 2*math.Pi)
	e.Yaw += turn * math.Min(1, dt*4)
}

// LineUp sets one of each vehicle in two rows ahead of the player, facing
// them, for screenshots; the automatic ones stand still.
func (g *Game) LineUp() {
	g.fleetHeld = true
	right := g.Player.Right()
	rows := [2][]*content.Vehicle{}
	for _, v := range g.Content.Vehicles.All() {
		big := content.OrZero(v.Radius, 1.2) > 2
		rows[btoi(big)] = append(rows[btoi(big)], v)
	}
	for r, row := range rows {
		width := 0.0
		for _, v := range row {
			width += 2*content.OrZero(v.Radius, 1) + 1.5
		}
		x := -width / 2
		for _, v := range row {
			w := 2*content.OrZero(v.Radius, 1) + 1.5
			at := g.Player.Pos.Add(g.Player.Heading.Scale(12 + 14*float64(r))).Add(right.Scale(x + w/2))
			x += w
			id := g.spawn(v.ID, g.World.Relief.Surface(at))
			if id == 0 {
				continue
			}
			e := g.Ents.Transform.Get(id)
			e.Alt = v.Hover
			e.Yaw += math.Pi / 3 // three-quarter view
			if r := g.Ents.Roam.Get(id); r != nil {
				r.Home = e.Dir
			}
		}
	}
}

func btoi(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Board gets in a new vehicle of kind id, for screenshots.
func (g *Game) Board(id string) {
	if e := g.spawn(id, g.Player.Pos); e != 0 {
		g.Ents.Transform.Get(e).Yaw = yawOf(g.Player.LocalUp(), g.Player.Heading)
		g.getIn(e)
		g.updateVehicle(0)
	}
}

// SpawnAhead sets an entity of kind id dist m ahead of the player, facing
// away like a ramp, for screenshots.
func (g *Game) SpawnAhead(id string, dist float64) {
	g.spawn(id, g.World.Relief.Surface(g.Player.Pos.Add(g.Player.Heading.Scale(dist))))
}
