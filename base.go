package main

import (
	"math"
)

// The moon base sits on the north pole, where up is +Y and the ground is
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
	openings []float64 // angles (radians, atan2(z, x)) of the doorways
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

	hubRoom      = room{vec3{0, 0, 0}, 6.5, 2.8, []float64{east, west, north, south}}
	saleRoom     = room{vec3{-13, 0, 0}, 4, 2.6, []float64{east}}
	researchRoom = room{vec3{13, 0, 0}, 4, 2.6, []float64{west}}
	sleepRoom    = room{vec3{0, 0, 13}, 4, 2.6, []float64{south}}
	rooms        = []room{hubRoom, saleRoom, researchRoom, sleepRoom}

	airlockEnd = vec3{0, 0, -10.5}
	corridors  = []corridor{
		{vec3{-6.3, 0, 0}, vec3{-9.2, 0, 0}, doorWidth, 2.4},
		{vec3{6.3, 0, 0}, vec3{9.2, 0, 0}, doorWidth, 2.4},
		{vec3{0, 0, 6.3}, vec3{0, 0, 9.2}, doorWidth, 2.4},
		{vec3{0, 0, -6.3}, airlockEnd, doorWidth + 0.2, 2.6},
	}
)

// Furniture spots, local.
var (
	tableSpot       = vec3{0, 0, 0}
	incineratorSpot = vec3{-14.6, 0, 0}
	consoleSpot     = vec3{15.3, 0, 0}
	bedSpot         = vec3{0, 0, 15.1}
	spawnSpot       = vec3{0, 0, 11.6} // wake up next to the bed
)

func toWorld(local vec3) vec3 { return basePos.add(local) }

func toLocal(world vec3) vec3 { return world.sub(basePos) }

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
		mid := r.center.add(vec3{math.Cos(a) * r.radius, 0, math.Sin(a) * r.radius})
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
		axis := c.to.sub(c.from).norm()
		side := c.side()
		for _, r := range rooms {
			for _, end := range []vec3{c.from, c.to} {
				if math.Abs(end.sub(r.center).length()-r.radius) > 0.6 {
					continue // this end doesn't open into this room
				}
				wall := r.center.add(end.sub(r.center).norm().scale(r.radius))
				half := c.width/2 + wallThick
				for _, s := range []float64{-1, 1} {
					out = append(out, plate{wall.add(side.scale(s * (half + 0.4))), axis, 0.8, 0, r.wallH})
				}
				out = append(out, plate{wall, axis, 2*half + 1.6, c.height, r.wallH})
			}
		}
	}
	return out
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
			t := vec3{-math.Sin(p.angle), 0, math.Cos(p.angle)}.scale(p.width / 2)
			walls = append(walls, segment2{p.mid.sub(t), p.mid.add(t)})
		}
	}
	for _, c := range corridors {
		side := c.side().scale(c.width/2 + wallThick/2)
		walls = append(walls, segment2{c.from.add(side), c.to.add(side)}, segment2{c.from.sub(side), c.to.sub(side)})
	}
	for _, p := range junctionPlates() {
		if p.y0 > 0 {
			continue // above the corridor
		}
		across := vec3{-p.normal.Z, 0, p.normal.X}.scale(p.width / 2)
		walls = append(walls, segment2{p.mid.sub(across), p.mid.add(across)})
	}
	return walls
}

// side is the horizontal unit vector across the corridor.
func (c corridor) side() vec3 {
	d := c.to.sub(c.from).norm()
	return vec3{-d.Z, 0, d.X}
}

// obstacle is a round piece of furniture the player walks around.
type obstacle struct {
	at     vec3
	radius float64
}

var baseObstacles = []obstacle{
	{tableSpot, 1.25},
	{incineratorSpot, 1.4},
	{consoleSpot.add(vec3{-0.3, 0, 0}), 1.0},
	{consoleSpot.add(vec3{0, 0, 2.3}), 0.6},
	{consoleSpot.add(vec3{0, 0, -2.3}), 0.6},
	{bedSpot.add(vec3{-0.55, 0, 0}), 0.6},
	{bedSpot.add(vec3{0.55, 0, 0}), 0.6},
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
		ab := w.b.sub(w.a)
		t := clamp01(flat.sub(w.a).dot(ab) / ab.dot(ab))
		closest := w.a.add(ab.scale(t))
		d := flat.sub(closest)
		if l := d.length(); l < radius+wallThick/2 && l > 1e-9 {
			flat = closest.add(d.scale((radius + wallThick/2) / l))
		}
	}
	for _, o := range baseObstacles {
		d := flat.sub(vec3{o.at.X, 0, o.at.Z})
		if l := d.length(); l < radius+o.radius && l > 1e-9 {
			flat = vec3{o.at.X, 0, o.at.Z}.add(d.scale((radius + o.radius) / l))
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
		d := c.to.sub(c.from)
		t := vec3{local.X, 0, local.Z}.sub(c.from).dot(d) / d.dot(d)
		across := math.Abs(vec3{local.X, 0, local.Z}.sub(c.from).dot(c.side()))
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
		lights = append(lights, r.center.add(vec3{0, 2.3, 0}))
	}
	for _, c := range corridors {
		lights = append(lights, c.from.lerp(c.to, 0.5).add(vec3{0, 2.1, 0}))
	}
	return lights
}

// interaction is something the player can use with E.
type interaction int

const (
	useNone interaction = iota
	useMap
	useIncinerator
	useResearch
	useBed
)

type interactable struct {
	at     vec3 // local
	reach  float64
	use    interaction
	prompt string
}

var interactables = []interactable{
	{tableSpot, 2.6, useMap, "[E] Consulter l'état de la Lune"},
	{incineratorSpot, 2.8, useIncinerator, "[E] Déposer la poussière dans l'incinérateur"},
	{consoleSpot, 2.8, useResearch, "[E] Ouvrir le laboratoire de recherche"},
	{bedSpot, 2.6, useBed, "[E] Dormir"},
}

// nearbyUse returns what the player can use right now: the closest
// interactable within reach that they roughly face.
func (g *Game) nearbyUse() (interactable, bool) {
	local := toLocal(g.player.Pos)
	flat := vec3{local.X, 0, local.Z}
	best, found := interactable{}, false
	bestDist := math.Inf(1)
	for _, it := range interactables {
		to := vec3{it.at.X, 0, it.at.Z}.sub(flat)
		d := to.length()
		if d > it.reach || d >= bestDist {
			continue
		}
		heading := toLocal(g.player.Pos.add(g.player.Heading)).sub(local)
		if d > 0.8 && heading.dot(to.norm()) < 0.3 {
			continue // standing next to it but looking away
		}
		best, found, bestDist = it, true, d
	}
	return best, found
}
