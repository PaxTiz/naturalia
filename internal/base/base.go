package base

import (
	"log"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/mathx"
	"naturalia/internal/planet"
	"slices"
)

// The Mars base sits on the north pole, where up is +Y and the ground is
// flat over the whole base: everything here is laid out in local
// coordinates (x, y, z) relative to Origin, with y up.
//
//	              [ Sleep ]
//	                  ║
//	[ Sale ]═══( Hub + map table )═══[ Research ]
//	                  ║
//	              [ Airlock ] → outside

// Room is a round building: panel walls with openings, under a dome roof;
// or, with Hall set, a big rectangular one (hall.go).
type Room struct {
	Center   vec3 // local, on the floor
	Radius   float64
	WallH    float64
	Hall     *content.Hall
	openings []float64          // angles (radians, atan2(z, x)) of the doorways
	plan     *content.Blueprint // drawn from the kit when set
}

// Corridor is a straight rectangular tube between two rooms (or outside).
type Corridor struct {
	From, To vec3 // local, on the floor, along its axis
	Width    float64
	Height   float64
}

const (
	DoorWidth  = 2.2
	DoorHeight = 2.3
	panelSize  = 1.6 // target width of a wall panel
	WallThick  = 0.15
	FloorTop   = 0.02 // floors sit just above the flattened ground
	Reach      = 22.0 // the base's footprint: no sweeping inside it
)

var (
	East, West, North, South = 0.0, math.Pi, math.Pi / 2, -math.Pi / 2

	AirlockEnd = vec3{0, 0, -10.5} // the hub's south door leads there
	SpawnSpot  = vec3{0, 0, 11.6}  // wake up next to the bed
)

// The base as laid out from the buildings in the content (Setup).
var (
	Plans         []*content.Blueprint // the hub first
	Rooms         []Room               // in the order of plans
	HubRoom       Room
	Corridors     []Corridor // one per linked building, then the airlock's
	TableSpot     vec3       // the map table, under the hologram
	Obstacles     []content.Obstacle
	Interactables []Interactable
	Bays          []Bay   // where each driven vehicle parks in the garage
	extent        float64 // how far from the origin the buildings reach, in m
)

// The shaders hold this many rooms, and as many corridors (baseAO, lamps):
// more buildings are left out.
const maxRooms = 8

// Setup lays the base out from the buildings of c: rooms, doors and
// corridors along the links, what blocks the player, what they can use.
func Setup(c *content.Content) {
	Plans = c.Buildings.All()
	if len(Plans) > maxRooms {
		log.Printf("base: %d buildings, only the first %d are built", len(Plans), maxRooms)
		Plans = Plans[:maxRooms]
	}
	Corridors = nil
	for _, bp := range Plans {
		if bp.Link == "" {
			continue
		}
		to, ok := c.Buildings.Get(bp.Link)
		if !ok || !slices.Contains(Plans, to) {
			log.Printf("base: %s links to %s, which isn't built", bp.ID, bp.Link)
			continue
		}
		d := to.At.Sub(bp.At)
		d = vec3{d.X, 0, d.Z}.Norm()
		addDoor(bp, math.Atan2(d.Z, d.X))
		addDoor(to, math.Atan2(-d.Z, -d.X))
		Corridors = append(Corridors, Corridor{
			to.At.Sub(d.Scale(edge(to, d.Scale(-1)))), bp.At.Add(d.Scale(edge(bp, d))), DoorWidth, 2.4,
		})
	}
	Corridors = append(Corridors, Corridor{vec3{0, 0, -(Plans[0].Shape.Radius - 0.2)}, AirlockEnd, DoorWidth + 0.2, 2.6})
	Rooms = nil
	for _, bp := range Plans {
		Rooms = append(Rooms, roomOf(bp))
	}
	HubRoom = Rooms[0]
	TableSpot = vec3{0, 0, 0}
	for _, bp := range Plans {
		for _, p := range bp.Props {
			if content.Qualify(p.Kit) == "core:maptable" {
				TableSpot = bp.At.Add(p.At)
			}
		}
	}
	Obstacles = planObstacles(&c.Kits)
	Interactables = planInteractables(&c.Kits)
	extent = Reach
	for _, r := range Rooms {
		extent = math.Max(extent, math.Hypot(r.Center.X, r.Center.Z)+r.reach())
	}
	Bays = layBays(c)
}

// edge is how far from its middle a corridor leaving bp along d starts:
// just inside a dome's round wall, in the middle of a hall's straight one.
func edge(bp *content.Blueprint, d vec3) float64 {
	if bp.Hall == nil {
		return bp.Shape.Radius - 0.2
	}
	t := math.Inf(1)
	if d.X != 0 {
		t = bp.Hall.Size[0] / 2 / math.Abs(d.X)
	}
	if d.Z != 0 {
		t = math.Min(t, bp.Hall.Size[1]/2/math.Abs(d.Z))
	}
	return t
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

// Origin is where the base stands on the ground, set once the relief is known
// (NewGame).
var Origin = planet.BaseUp.Scale(planet.Radius)

func ToWorld(local vec3) vec3 { return Origin.Add(local) }

func ToLocal(world vec3) vec3 { return world.Sub(Origin) }

// panel is one wall section of a room: its middle at floor level, the angle
// it faces, and its width. Doorways get a short panel above the door.
type panel struct {
	mid          vec3
	angle, width float64
	y0, y1       float64 // vertical extent
}

func (r Room) panels() []panel {
	if r.Hall != nil {
		return nil // its walls are straight: hall.go
	}
	n := int(math.Ceil(2 * math.Pi * r.Radius / panelSize))
	width := 2 * r.Radius * math.Sin(math.Pi/float64(n)) * 1.02
	var out []panel
	for i := range n {
		a := (float64(i) + 0.5) * 2 * math.Pi / float64(n)
		mid := r.Center.Add(vec3{math.Cos(a) * r.Radius, 0, math.Sin(a) * r.Radius})
		p := panel{mid: mid, angle: a, width: width, y0: 0, y1: r.WallH}
		for _, o := range r.openings {
			if angleDist(a, o) < (DoorWidth/2)/r.Radius+math.Pi/float64(n)*0.9 {
				p.y0 = DoorHeight + 0.1 // lintel above the doorway
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
	for _, c := range Corridors {
		axis := c.To.Sub(c.From).Norm()
		side := c.side()
		for _, r := range Rooms {
			if r.Hall != nil {
				continue
			}
			for _, end := range []vec3{c.From, c.To} {
				if math.Abs(end.Sub(r.Center).Length()-r.Radius) > 0.6 {
					continue // this end doesn't open into this room
				}
				wall := r.Center.Add(end.Sub(r.Center).Norm().Scale(r.Radius))
				half := c.Width/2 + WallThick
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
					if nrm.Dot(mid.Sub(r.Center)) < 0 {
						nrm = nrm.Scale(-1) // facing out of the room, like the others
					}
					out = append(out, plate{mid, nrm, l + 0.12, 0, r.WallH})
					reach = math.Max(reach, math.Abs(e.Sub(wall).Dot(side))+0.1)
				}
				out = append(out, plate{wall, axis, 2 * reach, c.Height, r.WallH})
			}
		}
	}
	return out
}

// nearestPanelEdge is the end of a full-height panel of r closest to p, at
// floor level.
func nearestPanelEdge(r Room, p vec3) vec3 {
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

// Segment is a wall seen from above, for collisions (x, z of local coords),
// and how high it stands: what flies over it isn't stopped.
type Segment struct {
	a, b vec3
	top  float64
}

// Walls lists every wall the player bumps into.
func Walls() []Segment {
	var walls []Segment
	for _, r := range Rooms {
		for _, p := range r.panels() {
			if p.y0 > 0 {
				continue // doorway
			}
			t := vec3{-math.Sin(p.angle), 0, math.Cos(p.angle)}.Scale(p.width / 2)
			walls = append(walls, Segment{p.mid.Sub(t), p.mid.Add(t), r.WallH + r.Radius*0.55})
		}
		walls = append(walls, r.hallWalls()...)
	}
	for _, c := range Corridors {
		side := c.side().Scale(c.Width/2 + WallThick/2)
		top := c.Height + 0.2
		walls = append(walls, Segment{c.From.Add(side), c.To.Add(side), top}, Segment{c.From.Sub(side), c.To.Sub(side), top})
	}
	for _, p := range junctionPlates() {
		if p.y0 > 0 {
			continue // above the corridor
		}
		across := vec3{-p.normal.Z, 0, p.normal.X}.Scale(p.width / 2)
		walls = append(walls, Segment{p.mid.Sub(across), p.mid.Add(across), p.y1})
	}
	return walls
}

// side is the horizontal unit vector across the corridor.
func (c Corridor) side() vec3 {
	d := c.To.Sub(c.From).Norm()
	return vec3{-d.Z, 0, d.X}
}

// Collide keeps the player standing at pos out of walls and furniture:
// where they end up. Flying h m over the ground, they pass over what
// stands lower.
func Collide(pos vec3, walls []Segment, h float64) vec3 {
	const radius = 0.35
	local := ToLocal(pos)
	if math.Hypot(local.X, local.Z) > extent+2 || local.Y > 6 {
		return pos
	}
	flat := vec3{local.X, 0, local.Z}
	for _, w := range walls {
		if h >= w.top {
			continue
		}
		ab := w.b.Sub(w.a)
		t := mathx.Clamp01(flat.Sub(w.a).Dot(ab) / ab.Dot(ab))
		closest := w.a.Add(ab.Scale(t))
		d := flat.Sub(closest)
		if l := d.Length(); l < radius+WallThick/2 && l > 1e-9 {
			flat = closest.Add(d.Scale((radius + WallThick/2) / l))
		}
	}
	for _, o := range Obstacles {
		if h > 2 {
			break
		}
		d := flat.Sub(vec3{o.At.X, 0, o.At.Z})
		if l := d.Length(); l < radius+o.Radius && l > 1e-9 {
			flat = vec3{o.At.X, 0, o.At.Z}.Add(d.Scale((radius + o.Radius) / l))
		}
	}
	return ToWorld(vec3{flat.X, local.Y, flat.Z})
}

// Indoors tells whether a local point is inside a room or a corridor.
func Indoors(local vec3) bool {
	if RoomAt(local) >= 0 {
		return true
	}
	if local.Y > 3.5 {
		return false
	}
	for _, c := range Corridors {
		d := c.To.Sub(c.From)
		t := vec3{local.X, 0, local.Z}.Sub(c.From).Dot(d) / d.Dot(d)
		across := math.Abs(vec3{local.X, 0, local.Z}.Sub(c.From).Dot(c.side()))
		if t >= 0 && t <= 1 && across < c.Width/2 {
			return true
		}
	}
	return false
}

// RoomAt is the index of the room holding a local point, or -1.
func RoomAt(local vec3) int {
	for i, r := range Rooms {
		if r.Hall != nil {
			if r.inHall(local, 0) && local.Y < r.WallH+r.Hall.Rise {
				return i
			}
			continue
		}
		if local.Y < 3.5 && math.Hypot(local.X-r.Center.X, local.Z-r.Center.Z) < r.Radius {
			return i
		}
	}
	return -1
}

// Covered tells whether a local point of the ground is under a building
// beyond the base's round footprint: its floor hides the dust.
func Covered(local vec3) bool {
	for _, r := range Rooms {
		if r.Hall != nil && r.inHall(local, 0.5) {
			return true
		}
	}
	return false
}

// Lamp is a ceiling lamp: where it hangs, how bright it is, how far it
// reaches.
type Lamp struct {
	At           vec3 // local
	Power, Range float64
}

// Lamps lists the ceiling lamps: one per room first (they cast shadows),
// then one per corridor, then the halls' others.
func Lamps() []Lamp {
	lamps := []Lamp{{vec3{0, 2.7, 0}, 1.4, 10}}
	for _, r := range Rooms[1:] {
		if r.Hall != nil {
			lamps = append(lamps, r.hallLamps()[0])
			continue
		}
		lamps = append(lamps, Lamp{r.Center.Add(vec3{0, 2.3, 0}), 1, 10})
	}
	for _, c := range Corridors {
		lamps = append(lamps, Lamp{c.From.Lerp(c.To, 0.5).Add(vec3{0, 2.1, 0}), 1, 10})
	}
	for _, r := range Rooms {
		if r.Hall != nil {
			lamps = append(lamps, r.hallLamps()[1:]...)
		}
	}
	return lamps
}

// Interactable is something the player can use with E.
type Interactable struct {
	Kit    string // ID of the kit piece
	At     vec3   // local
	Reach  float64
	Use    func(g content.Game, at vec3)
	Prompt string
}
