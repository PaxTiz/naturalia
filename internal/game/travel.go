package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/planet"
	"strings"
)

// Travel (docs/TELEPORTEURS.md): the hologram of the base or of any
// bootcamp lists every place with one; the player picks one and is
// teleported there, on foot, before its hologram, for a price.

// TravelCost is what a trip costs, in credits.
const TravelCost = 500_000

// The trip's timeline, in seconds: a cyan fade out, the jump, the fade in.
const (
	TravelOut  = 0.7
	TravelJump = 0.75
	TravelEnd  = 1.5
)

// Destination is somewhere to travel to: the base, or a bootcamp.
type Destination struct {
	Name    string
	Camp    ecs.ID  // 0 for the base
	Dir     vec3    // where it stands, from Mars' center
	Dist    float64 // how far from the player, in m
	Bearing string  // which way from the player, on the compass (N is the base)
	Here    bool    // the player is at its hologram
}

// Destinations are where the player can travel to: the base first, then
// the bootcamps, in the order they were placed.
func (g *Game) Destinations() []Destination {
	from := g.Player.Pos
	here := g.travelFrom()
	out := []Destination{{Name: "Base", Dir: planet.BaseUp, Here: here == 0}}
	for _, c := range g.Camps() {
		if !c.Layout.Hologram() {
			continue
		}
		out = append(out, Destination{Name: c.Name, Camp: c.Ent, Dir: c.Site.Origin.Norm(), Here: here == c.Ent})
	}
	for i := range out {
		d := &out[i]
		d.Dist = planet.ArcDist(from.Norm(), d.Dir)
		d.Bearing = bearing(from, d.Dir)
	}
	return out
}

// travelFrom is the bootcamp the player stands in, or 0 at the base (or
// anywhere else: the map table is at the base).
func (g *Game) travelFrom() ecs.ID {
	if c, ok := g.CampAround(g.Player.Eye()); ok {
		return c.Ent
	}
	return 0
}

// bearing is the compass point toward dir from a point of the world: the
// base is north (it stands on the pole).
func bearing(from, dir vec3) string {
	up := from.Norm()
	north := planet.BaseUp.Sub(up.Scale(planet.BaseUp.Dot(up)))
	to := dir.Sub(up.Scale(dir.Dot(up)))
	if north.Length() < 1e-9 || to.Length() < 1e-9 {
		return "—"
	}
	north = north.Norm()
	east := north.Cross(up)
	a := math.Atan2(to.Dot(east), to.Dot(north)) * 180 / math.Pi
	points := []string{"N", "NE", "E", "SE", "S", "SO", "O", "NO"}
	return points[int(math.Round(math.Mod(a+360, 360)/45))%8]
}

// CanTravel tells whether the player can travel to d: somewhere else,
// with the credits.
func (g *Game) CanTravel(d Destination) bool { return !d.Here && g.State.Credits >= TravelCost }

// Travel pays for the trip to d and starts it: the view fades to cyan,
// the player jumps (updateTravel) and comes out before its hologram.
func (g *Game) Travel(d Destination) {
	if !g.CanTravel(d) || g.Riding != 0 {
		g.sfx.Beep(0, 0.15, 0.1, 180)
		return
	}
	g.State.Credits -= TravelCost
	g.startTravel(d)
}

func (g *Game) startTravel(d Destination) {
	g.travelTo, g.TravelT = d, 0
	g.Mode = ModeTravel
	g.sfx.Beep(0.07, 0.35, 0.14, 330, 440, 587, 784, 1047)
}

// Teleport sends the player to the base ("Base") or the bootcamp of that
// name, for free: a mod's doing. False if there is none, or the player
// can't go now (driving, or not playing).
func (g *Game) Teleport(name string) bool {
	if g.Riding != 0 || g.Mode != ModePlay && g.Mode != ModeMap {
		return false
	}
	for _, d := range g.Destinations() {
		if strings.EqualFold(d.Name, name) {
			g.startTravel(d)
			return true
		}
	}
	return false
}

// Bootcamps are the bootcamps standing, for packs: their name, where they
// stand and how far from the player.
func (g *Game) Bootcamps() []content.CampState {
	var out []content.CampState
	for _, d := range g.Destinations()[1:] {
		out = append(out, content.CampState{Name: d.Name, At: g.World.Relief.Surface(d.Dir), Dist: d.Dist})
	}
	return out
}

// maxCampName is how long a bootcamp's name may be, in letters.
const maxCampName = 24

// RenameCamp names bootcamp id: trimmed, cut to maxCampName letters; an
// empty name or one another place has already is refused.
func (g *Game) RenameCamp(id ecs.ID, name string) bool {
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > maxCampName {
		name = strings.TrimSpace(string(r[:maxCampName]))
	}
	b := g.Ents.Camp.Get(id)
	if b == nil || name == "" {
		return false
	}
	for _, d := range g.Destinations() {
		if d.Camp != id && strings.EqualFold(d.Name, name) {
			return false
		}
	}
	b.Name = name
	return true
}

// TravelingTo is the name of where the player is traveling to.
func (g *Game) TravelingTo() string { return g.travelTo.Name }

// updateTravel runs the trip: halfway, the player is where they go.
func (g *Game) updateTravel(dt float64) {
	before := g.TravelT
	g.TravelT += dt
	if before < TravelJump && g.TravelT >= TravelJump {
		g.jump(g.travelTo)
	}
	if g.TravelT >= TravelEnd {
		g.setMode(ModePlay)
	}
}

// jump stands the player before the hologram of d, facing it.
func (g *Game) jump(d Destination) {
	from := "Base"
	if c, ok := g.CampAround(g.Player.Eye()); ok {
		from = c.Name
	}
	site, table, door := base.BaseSite(), base.TableSpot(), base.AirlockEnd
	if c, ok := g.campOf(d.Camp); ok {
		site, table = c.Site, c.Layout.Table
		door, _ = c.Layout.Door()
	}
	out := door.Sub(table)
	out = vec3{out.X, 0, out.Z}.Norm() // toward the door: the way in
	at := table.Add(out.Scale(travelGap))
	g.Player.Pos = site.ToWorld(vec3{at.X, table.Y + base.FloorTop, at.Z})
	g.Player.Heading = site.Dir(out.Scale(-1))
	g.Player.Pitch, g.Player.vr, g.Player.vel, g.Player.want = -0.1, 0, vec3{}, vec3{}
	g.relay = true // the collisions, laid again where they arrive
	g.Steps = Footprints{}
	g.NearRocks = g.World.RocksNear(g.Player.Pos, RockDrawDist)
	g.sfx.Beep(0.06, 0.4, 0.12, 1047, 784, 1175, 1568)
	g.emit(content.Event{Name: content.EvTeleport, ID: d.Name, By: from, At: g.Player.Pos})
}

// travelGap is how far from the table's middle the player comes out.
const travelGap = 2.1

// campOf is bootcamp id standing, if it is one.
func (g *Game) campOf(id ecs.ID) (CampAt, bool) {
	for _, c := range g.Camps() {
		if c.Ent == id {
			return c, true
		}
	}
	return CampAt{}, false
}
