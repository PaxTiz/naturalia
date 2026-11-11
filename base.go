package main

import (
	"log"
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/planet"
	"math"
	"slices"
)

// The Mars base sits on the north pole, where up is +Y and the ground is
// flat over the whole base: everything here is laid out in local
// coordinates (x, y, z) relative to basePos, with y up.
//
//	              [ Sleep ]
//	                  ║
//	[ Sale ]═══( Hub + map table )═══[ Research ]
//	                  ║
//	              [ Airlock ] → outside

// room is a round building: panel walls with openings, under a dome roof.
type room struct {
	center   vec3 // local, on the floor
	radius   float64
	wallH    float64
	openings []float64          // angles (radians, atan2(z, x)) of the doorways
	plan     *content.Blueprint // drawn from the kit when set
}

// corridor is a straight rectangular tube between two rooms (or outside).
type corridor struct {
	from, to vec3 // local, on the floor, along its axis
	width    float64
	height   float64
}

const (
	doorWidth  = 2.2
	doorHeight = 2.3
	panelSize  = 1.6 // target width of a wall panel
	wallThick  = 0.15
	floorTop   = 0.02 // floors sit just above the flattened ground
	baseReach  = 22.0 // the base's footprint: no sweeping inside it
)

var (
	east, west, north, south = 0.0, math.Pi, math.Pi / 2, -math.Pi / 2

	airlockEnd = vec3{0, 0, -10.5} // the hub's south door leads there
	spawnSpot  = vec3{0, 0, 11.6}  // wake up next to the bed
)

// The base as laid out from the buildings in the content (setupBase).
var (
	plans         []*content.Blueprint // the hub first
	rooms         []room               // in the order of plans
	hubRoom       room
	corridors     []corridor // one per linked building, then the airlock's
	tableSpot     vec3       // the map table, under the hologram
	baseObstacles []content.Obstacle
	interactables []interactable
)

// The shaders hold this many rooms, and as many corridors (baseAO, lamps):
// more buildings are left out.
const maxRooms = 8

// setupBase lays the base out from the buildings: rooms, doors and
// corridors along the links, what blocks the player, what they can use.
func setupBase() {
	plans = reg.Buildings.All()
	if len(plans) > maxRooms {
		log.Printf("base: %d buildings, only the first %d are built", len(plans), maxRooms)
		plans = plans[:maxRooms]
	}
	corridors = nil
	for _, bp := range plans {
		if bp.Link == "" {
			continue
		}
		to, ok := reg.Buildings.Get(bp.Link)
		if !ok || !slices.Contains(plans, to) {
			log.Printf("base: %s links to %s, which isn't built", bp.ID, bp.Link)
			continue
		}
		d := to.At.Sub(bp.At)
		d = vec3{d.X, 0, d.Z}.Norm()
		addDoor(bp, math.Atan2(d.Z, d.X))
		addDoor(to, math.Atan2(-d.Z, -d.X))
		corridors = append(corridors, corridor{
			to.At.Sub(d.Scale(to.Shape.Radius - 0.2)), bp.At.Add(d.Scale(bp.Shape.Radius - 0.2)), doorWidth, 2.4,
		})
	}
	corridors = append(corridors, corridor{vec3{0, 0, -(plans[0].Shape.Radius - 0.2)}, airlockEnd, doorWidth + 0.2, 2.6})
	rooms = nil
	for _, bp := range plans {
		rooms = append(rooms, roomOf(bp))
	}
	hubRoom = rooms[0]
	tableSpot = vec3{0, 0, 0}
	for _, bp := range plans {
		for _, p := range bp.Props {
			if content.Qualify(p.Kit) == "core:maptable" {
				tableSpot = bp.At.Add(p.At)
			}
		}
	}
	baseObstacles = planObstacles()
	interactables = planInteractables()
}

// addDoor opens a door of bp facing angle a, unless there is one already.
func addDoor(bp *content.Blueprint, a float64) {
	for _, d := range bp.Doors {
		if angleDist(d.Facing, a) < 0.05 {
			return
		}
	}
	bp.Doors = append(bp.Doors, content.Door{a})
}

// basePos is the base's origin on the ground, set once the relief is known
// (NewGame).
var basePos = planet.BaseUp.Scale(planet.Radius)

func toWorld(local vec3) vec3 { return basePos.Add(local) }

func toLocal(world vec3) vec3 { return world.Sub(basePos) }

// panel is one wall section of a room: its middle at floor level, the angle
// it faces, and its width. Doorways get a short panel above the door.
type panel struct {
	mid          vec3
	angle, width float64
	y0, y1       float64 // vertical extent
}

func (r room) panels() []panel {
	n := int(math.Ceil(2 * math.Pi * r.radius / panelSize))
	width := 2 * r.radius * math.Sin(math.Pi/float64(n)) * 1.02
	var out []panel
	for i := range n {
		a := (float64(i) + 0.5) * 2 * math.Pi / float64(n)
		mid := r.center.Add(vec3{math.Cos(a) * r.radius, 0, math.Sin(a) * r.radius})
		p := panel{mid: mid, angle: a, width: width, y0: 0, y1: r.wallH}
		for _, o := range r.openings {
			if angleDist(a, o) < (doorWidth/2)/r.radius+math.Pi/float64(n)*0.9 {
				p.y0 = doorHeight + 0.1 // lintel above the doorway
			}
		}
		out = append(out, p)
	}
	return out
}

func angleDist(a, b float64) float64 {
	d := math.Mod(math.Abs(a-b), 2*math.Pi)
	return math.Min(d, 2*math.Pi-d)
}

// plate is a flat wall facing along normal: it fills the gap between a
// corridor and the round wall it opens into.
type plate struct {
	mid    vec3 // at floor level
	normal vec3 // horizontal, along the corridor
	width  float64
	y0, y1 float64
}

func junctionPlates() []plate {
	var out []plate
	for _, c := range corridors {
		axis := c.to.Sub(c.from).Norm()
		side := c.side()
		for _, r := range rooms {
			for _, end := range []vec3{c.from, c.to} {
				if math.Abs(end.Sub(r.center).Length()-r.radius) > 0.6 {
					continue // this end doesn't open into this room
				}
				wall := r.center.Add(end.Sub(r.center).Norm().Scale(r.radius))
				half := c.width/2 + wallThick
				// Each side plate runs from the corridor's wall to the edge of
				// the first round panel left standing: the round wall curves
				// back from the doorway, a plate along its tangent would leave
				// a slit open to the outside.
				reach := half
				for _, s := range []float64{-1, 1} {
					a := wall.Add(side.Scale(s * half))
					e := nearestPanelEdge(r, a)
					d := e.Sub(a)
					l := d.Length()
					d = d.Scale(1 / l)
					mid := a.Add(e).Scale(0.5)
					nrm := vec3{d.Z, 0, -d.X}
					if nrm.Dot(mid.Sub(r.center)) < 0 {
						nrm = nrm.Scale(-1) // facing out of the room, like the others
					}
					out = append(out, plate{mid, nrm, l + 0.12, 0, r.wallH})
					reach = math.Max(reach, math.Abs(e.Sub(wall).Dot(side))+0.1)
				}
				out = append(out, plate{wall, axis, 2 * reach, c.height, r.wallH})
			}
		}
	}
	return out
}

// nearestPanelEdge is the end of a full-height panel of r closest to p, at
// floor level.
func nearestPanelEdge(r room, p vec3) vec3 {
	best, bestD := p, math.Inf(1)
	for _, q := range r.panels() {
		if q.y0 > 0 {
			continue
		}
		t := vec3{-math.Sin(q.angle), 0, math.Cos(q.angle)}.Scale(q.width / 2)
		for _, e := range []vec3{q.mid.Sub(t), q.mid.Add(t)} {
			if d := e.Sub(p).Length(); d < bestD {
				best, bestD = e, d
			}
		}
	}
	return best
}

// segment2 is a wall seen from above, for collisions (x, z of local coords).
type segment2 struct{ a, b vec3 }

// baseWalls lists every wall the player bumps into.
func baseWalls() []segment2 {
	var walls []segment2
	for _, r := range rooms {
		for _, p := range r.panels() {
			if p.y0 > 0 {
				continue // doorway
			}
			t := vec3{-math.Sin(p.angle), 0, math.Cos(p.angle)}.Scale(p.width / 2)
			walls = append(walls, segment2{p.mid.Sub(t), p.mid.Add(t)})
		}
	}
	for _, c := range corridors {
		side := c.side().Scale(c.width/2 + wallThick/2)
		walls = append(walls, segment2{c.from.Add(side), c.to.Add(side)}, segment2{c.from.Sub(side), c.to.Sub(side)})
	}
	for _, p := range junctionPlates() {
		if p.y0 > 0 {
			continue // above the corridor
		}
		across := vec3{-p.normal.Z, 0, p.normal.X}.Scale(p.width / 2)
		walls = append(walls, segment2{p.mid.Sub(across), p.mid.Add(across)})
	}
	return walls
}

// side is the horizontal unit vector across the corridor.
func (c corridor) side() vec3 {
	d := c.to.Sub(c.from).Norm()
	return vec3{-d.Z, 0, d.X}
}

// collide keeps a player of the given radius out of walls and furniture.
func collideBase(p *Player, walls []segment2) {
	const radius = 0.35
	local := toLocal(p.Pos)
	if math.Hypot(local.X, local.Z) > baseReach+2 || local.Y > 6 {
		return
	}
	flat := vec3{local.X, 0, local.Z}
	for _, w := range walls {
		ab := w.b.Sub(w.a)
		t := mathx.Clamp01(flat.Sub(w.a).Dot(ab) / ab.Dot(ab))
		closest := w.a.Add(ab.Scale(t))
		d := flat.Sub(closest)
		if l := d.Length(); l < radius+wallThick/2 && l > 1e-9 {
			flat = closest.Add(d.Scale((radius + wallThick/2) / l))
		}
	}
	for _, o := range baseObstacles {
		d := flat.Sub(vec3{o.At.X, 0, o.At.Z})
		if l := d.Length(); l < radius+o.Radius && l > 1e-9 {
			flat = vec3{o.At.X, 0, o.At.Z}.Add(d.Scale((radius + o.Radius) / l))
		}
	}
	p.Pos = toWorld(vec3{flat.X, local.Y, flat.Z})
}

// indoors tells whether a local point is inside a room or a corridor.
func indoors(local vec3) bool {
	if local.Y > 3.5 {
		return false
	}
	for _, r := range rooms {
		if math.Hypot(local.X-r.center.X, local.Z-r.center.Z) < r.radius {
			return true
		}
	}
	for _, c := range corridors {
		d := c.to.Sub(c.from)
		t := vec3{local.X, 0, local.Z}.Sub(c.from).Dot(d) / d.Dot(d)
		across := math.Abs(vec3{local.X, 0, local.Z}.Sub(c.from).Dot(c.side()))
		if t >= 0 && t <= 1 && across < c.width/2 {
			return true
		}
	}
	return false
}

// baseLights are the ceiling lamps: local position and intensity.
func baseLights() []vec3 {
	lights := []vec3{{0, 2.7, 0}}
	for _, r := range rooms[1:] {
		lights = append(lights, r.center.Add(vec3{0, 2.3, 0}))
	}
	for _, c := range corridors {
		lights = append(lights, c.from.Lerp(c.to, 0.5).Add(vec3{0, 2.1, 0}))
	}
	return lights
}

// interactable is something the player can use with E.
type interactable struct {
	kit    string // ID of the kit piece
	at     vec3   // local
	reach  float64
	use    func(g *Game, at vec3)
	prompt string
}

// nearbyUse returns what the player can use right now: the closest
// interactable within reach that they roughly face.
func (g *Game) nearbyUse() (interactable, bool) {
	if i, _ := g.nearbyEntity(); i >= 0 {
		return g.entityUse(i), true
	}
	local := toLocal(g.player.Pos)
	flat := vec3{local.X, 0, local.Z}
	best, found := interactable{}, false
	bestDist := math.Inf(1)
	for _, it := range interactables {
		to := vec3{it.at.X, 0, it.at.Z}.Sub(flat)
		d := to.Length()
		if d > it.reach || d >= bestDist {
			continue
		}
		heading := toLocal(g.player.Pos.Add(g.player.Heading)).Sub(local)
		if d > 0.8 && heading.Dot(to.Norm()) < 0.3 {
			continue // standing next to it but looking away
		}
		best, found, bestDist = it, true, d
	}
	return best, found
}
