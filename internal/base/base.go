package base

import (
	"log"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/planet"
	"slices"
)

// The Mars base sits on the north pole, where up is +Y and the ground is
// flat over the whole base: everything here is laid out in local
// coordinates (x, y, z) relative to Origin, with y up.
//
//	                   [ Bedroom ]
//	                       ║
//	[ Hangar ]═══( Great hall: map table, sale, lab, lounge )
//	                       ║
//	                   [ Airlock ] → outside

// Room is a round building: panel walls with openings, under a dome roof;
// or, with Hall set, a big rectangular one (hall.go).
type Room struct {
	Center   vec3    // local, on the floor
	Yaw      float64 // degrees, as its blueprint's
	Radius   float64
	WallH    float64
	Hall     *content.Hall
	openings []float64          // angles (radians, atan2(z, x)) of the doorways
	plan     *content.Blueprint // drawn from the kit when set
}

// Plan is the blueprint the room is laid out from.
func (r Room) Plan() *content.Blueprint { return r.plan }

// warm tells a room lit low and amber, like the bedroom.
func (r Room) warm() bool { return r.plan != nil && r.plan.Light == "warm" }

// Turn takes a point of the room's own frame (laid out unturned, around
// its center) to the base's: it turns it by the room's yaw.
func (r Room) Turn(p vec3) vec3 { return r.Center.Add(content.Turn(p.Sub(r.Center), r.Yaw)) }

// Unturn takes a point of the base to the room's own frame.
func (r Room) Unturn(p vec3) vec3 { return r.Center.Add(content.Turn(p.Sub(r.Center), -r.Yaw)) }

// TurnDir turns a direction of the room's own frame to the base's.
func (r Room) TurnDir(d vec3) vec3 { return content.Turn(d, r.Yaw) }

// SignOver is where a sign hangs inside the room over the doorway toward
// angle a (in the base), against the wall, and the way it faces: a dome's
// just under its dome, a hall's over the stripes above its doorways.
func (r Room) SignOver(a float64) (at, facing vec3) {
	d := vec3{math.Cos(a), 0, math.Sin(a)}
	if r.Hall == nil {
		return r.Center.Add(d.Scale(r.Radius - WallThick/2 - 0.03)).Add(vec3{0, 2.6, 0}), d.Scale(-1)
	}
	w := r.Unturn(r.Center.Add(d.Scale(edge(r.plan, d))))
	out := r.wallOut(w)
	return r.Turn(w.Sub(out.Scale(hallWall/2 + 0.03))).Add(vec3{0, DoorHeight + 1.25, 0}), r.TurnDir(out.Scale(-1))
}

// Corridor is a straight rectangular tube between two rooms (or outside).
type Corridor struct {
	From, To vec3 // local, on the floor, along its axis
	Width    float64
	Height   float64
	Rooms    [2]ecs.ID // the buildings at From and To; 0 outside
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

	AirlockEnd = vec3{0, 0, -10.5} // the hub's south door leads there (Setup)
	SpawnSpot  = vec3{0, 0, 11.6}  // wake up next to the bed (Setup)
)

// The base as laid out from the buildings in the content (Setup), besides
// its entities (Ents).
var (
	Bays   []Bay   // where each driven vehicle parks in the garage
	Extent float64 // how far from the origin the buildings reach, in m
)

// Setup lays the base out from the buildings of c, as entities: rooms,
// corridors along the links, their doors, the furniture, what blocks the
// player and what they can use.
func Setup(c *content.Content) {
	plans := c.Buildings.All()
	hub := plans[0]
	south := vec3{0, 0, -1}
	addDoor(hub, South) // to the airlock
	// The links open the doors first: a room is laid out with them.
	type link struct {
		from, to *content.Blueprint
		c        Corridor
	}
	var links []link
	linked := func(a, b *content.Blueprint) bool {
		return slices.ContainsFunc(links, func(l link) bool { return (l.from == a && l.to == b) || (l.from == b && l.to == a) })
	}
	for _, bp := range plans {
		for _, id := range bp.AllLinks() {
			to, ok := c.Buildings.Get(id)
			if !ok || !slices.Contains(plans, to) {
				log.Printf("base: %s links to %s, which isn't built", bp.ID, id)
				continue
			}
			if to == bp || linked(bp, to) {
				continue
			}
			d := to.At.Sub(bp.At)
			d = vec3{d.X, 0, d.Z}.Norm()
			addDoor(bp, math.Atan2(d.Z, d.X))
			addDoor(to, math.Atan2(-d.Z, -d.X))
			links = append(links, link{bp, to, Corridor{
				From: to.At.Sub(d.Scale(edge(to, d.Scale(-1)))), To: bp.At.Add(d.Scale(edge(bp, d))), Width: DoorWidth, Height: 2.4,
			}})
		}
	}
	Ents = newEntities()
	ids := map[*content.Blueprint]ecs.ID{}
	for _, bp := range plans {
		id := Ents.New()
		Ents.Building.Set(id, roomOf(bp))
		ids[bp] = id
	}
	for _, l := range links {
		l.c.Rooms = [2]ecs.ID{ids[l.to], ids[l.from]}
		Ents.Corridor.Set(Ents.New(), l.c)
	}
	AirlockEnd = hub.At.Add(south.Scale(edge(hub, south) + 4.2))
	Ents.Corridor.Set(Ents.New(), Corridor{hub.At.Add(south.Scale(edge(hub, south))), AirlockEnd, DoorWidth + 0.2, 2.6, [2]ecs.ID{ids[hub], 0}})
	// The yard's clutter keeps clear of the buildings and corridors.
	Ents.Obstacles.Set(Ents.New(), YardObstacles())
	Ents.Obstacles.Set(Ents.New(), groundObstacles())
	for _, bp := range plans {
		for _, p := range bp.Props {
			addFurniture(&c.Kits, ids[bp], bp, p)
		}
	}
	for _, bp := range plans {
		for _, fl := range bp.Floors {
			addFloor(ids[bp], bp, fl)
		}
	}
	// Wake up 2.4 m from the bed, toward the middle of its room.
	for _, f := range AllFurniture() {
		if f.Kit != "core:bed" {
			continue
		}
		if d := Ents.Building.Get(f.In).Center.Sub(f.At); d.Length() > 0.1 {
			SpawnSpot = f.At.Add(d.Norm().Scale(2.4))
		}
		break
	}
	Extent = Reach
	for _, r := range Buildings() {
		Extent = math.Max(Extent, math.Hypot(r.Center.X, r.Center.Z)+r.reach())
	}
	Bays = layBays(c)
}

// addFurniture puts prop p of blueprint bp in building in, from its kit:
// where it stands, what stops the player around it, what E does with it.
func addFurniture(kits *content.Registry[content.Kit], in ecs.ID, bp *content.Blueprint, p content.Prop) {
	id := Ents.New()
	f := Ents.Furniture.Set(id, Furniture{Kit: content.Qualify(p.Kit), At: bp.At.Add(content.Turn(p.At, bp.Yaw)), Yaw: p.Yaw + bp.Yaw, Seed: p.Seed, In: in})
	k, _ := kits.Get(p.Kit)
	if k == nil {
		return
	}
	if len(k.Obstacles) > 0 {
		var obs []content.Obstacle
		for _, o := range k.Obstacles {
			obs = append(obs, content.Obstacle{f.At.Add(content.Turn(o.At, f.Yaw)), o.Radius})
		}
		Ents.Obstacles.Set(id, obs)
	}
	var lamps []Lamp
	for _, l := range k.Lamps {
		lamps = append(lamps, Lamp{f.At.Add(content.Turn(l.At, f.Yaw)), l.Power, l.Range, 1})
	}
	if len(lamps) > 0 {
		Ents.Lamps.Set(id, lamps)
	}
	if k.Grow != nil && k.Stages > 0 {
		Ents.Grows.Set(id, k.Stages)
	}
	if k.Use != nil {
		Ents.Use.Set(id, Interactable{f.Kit, f.At, k.Reach, k.Use, k.Prompt})
	}
}

// edge is how far from its middle a corridor leaving bp along d starts:
// just inside a dome's round wall, in the middle of a hall's straight one.
func edge(bp *content.Blueprint, d vec3) float64 {
	if bp.Hall == nil {
		return bp.Shape.Radius - 0.2
	}
	d = content.Turn(d, -bp.Yaw)
	t := math.Inf(1)
	if d.X != 0 {
		t = bp.Hall.Size[0] / 2 / math.Abs(d.X)
	}
	if d.Z != 0 {
		t = math.Min(t, bp.Hall.Size[1]/2/math.Abs(d.Z))
	}
	return t
}

// addDoor opens a door of bp facing angle a (in the base), unless there is
// one already.
func addDoor(bp *content.Blueprint, a float64) {
	a -= bp.Yaw * math.Pi / 180
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
		a := (float64(i)+0.5)*2*math.Pi/float64(n) + r.Yaw*math.Pi/180
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
	for _, c := range Corridors() {
		axis := c.To.Sub(c.From).Norm()
		side := c.side()
		for _, r := range Buildings() {
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
					e := nearestPanelEdge(*r, a)
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

// Half is half the length of a straight wall.
func (s Segment) Half() float64 { return s.b.Sub(s.a).Length() / 2 }

// Ends are the wall's ends (local, on the floor) and how high it stands.
func (s Segment) Ends() (a, b vec3, top float64) { return s.a, s.b, s.top }

// Walls lists every wall the player bumps into.
func Walls() []Segment {
	var walls []Segment
	for _, r := range Buildings() {
		for _, p := range r.panels() {
			if p.y0 > 0 {
				continue // doorway
			}
			t := vec3{-math.Sin(p.angle), 0, math.Cos(p.angle)}.Scale(p.width / 2)
			walls = append(walls, Segment{p.mid.Sub(t), p.mid.Add(t), r.WallH + r.Radius*0.55})
		}
		walls = append(walls, r.hallWalls()...)
	}
	for _, c := range Corridors() {
		side := c.side().Scale(c.Width/2 + WallThick/2)
		top := c.Height + 0.2
		walls = append(walls, Segment{c.From.Add(side), c.To.Add(side), top}, Segment{c.From.Sub(side), c.To.Sub(side), top})
	}
	walls = append(walls, entryWalls()...)
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

// Indoors tells whether a local point is inside a room or a corridor.
func Indoors(local vec3) bool {
	if RoomAt(local) != 0 {
		return true
	}
	if local.Y > 3.5 {
		return false
	}
	for _, c := range Corridors() {
		d := c.To.Sub(c.From)
		t := vec3{local.X, 0, local.Z}.Sub(c.From).Dot(d) / d.Dot(d)
		across := math.Abs(vec3{local.X, 0, local.Z}.Sub(c.From).Dot(c.side()))
		if t >= 0 && t <= 1 && across < c.Width/2 {
			return true
		}
	}
	return false
}

// RoomAt is the building whose room holds a local point, or 0.
func RoomAt(local vec3) ecs.ID {
	for id, r := range Buildings() {
		if r.Hall != nil {
			if r.inHall(local, 0) && local.Y < r.WallH+r.Hall.Rise {
				return id
			}
			continue
		}
		if local.Y < 3.5 && math.Hypot(local.X-r.Center.X, local.Z-r.Center.Z) < r.Radius {
			return id
		}
	}
	return 0
}

// Covered tells whether a local point of the ground is under a building
// beyond the base's round footprint: its floor hides the dust.
func Covered(local vec3) bool {
	for _, r := range Buildings() {
		if r.Hall != nil && r.inHall(local, 0.5) {
			return true
		}
	}
	return false
}

// Lamp is a lamp of the base: where it hangs, how bright it is, how far
// it reaches, and how warm it is: 0 white, 1 amber (a warm room's, a
// piece of furniture's), a warm white between (a home's, homeWarm).
type Lamp struct {
	At           vec3 // local
	Power, Range float64
	Warm         float64
}

// Lamps lists the lamps of some buildings and corridors: one per room
// first, on its ceiling (they cast shadows), then one per corridor, then
// their furniture's, then the halls' other ceiling lamps.
func Lamps(rooms, corridors []ecs.ID) []Lamp {
	var lamps []Lamp
	hub, _ := Hub()
	for _, id := range rooms {
		r := Ents.Building.Get(id)
		switch {
		case r.Hall != nil:
			lamps = append(lamps, r.turnLamp(r.hallLamps()[0]))
		case r.warm():
			lamps = append(lamps, Lamp{r.Center.Add(vec3{0, 2.3, 0}), 0.6, 8, 1})
		case id == hub:
			lamps = append(lamps, Lamp{r.Center.Add(vec3{0, 2.7, 0}), 1.4, 10, r.lampWarmth()})
		default:
			lamps = append(lamps, Lamp{r.Center.Add(vec3{0, 2.3, 0}), r.lampPower(), 10, r.lampWarmth()})
		}
	}
	for _, id := range corridors {
		c := Ents.Corridor.Get(id)
		w, power := 0.0, 1.0
		if c.home() {
			w, power = homeWarm, homeDim
		}
		lamps = append(lamps, Lamp{c.From.Lerp(c.To, 0.5).Add(vec3{0, 2.1, 0}), power, 10, w})
	}
	for _, id := range rooms {
		for f := range FurnitureOf(id) {
			if l := Ents.Lamps.Get(f); l != nil {
				lamps = append(lamps, *l...)
			}
		}
	}
	for _, id := range rooms {
		if r := Ents.Building.Get(id); r.Hall != nil {
			for _, l := range r.hallLamps()[1:] {
				lamps = append(lamps, r.turnLamp(l))
			}
		}
	}
	return lamps
}

// turnLamp takes a lamp laid out in the room's own frame to the base.
func (r Room) turnLamp(l Lamp) Lamp {
	l.At = r.Turn(l.At)
	return l
}

// Interactable is something the player can use with E.
type Interactable struct {
	Kit    string // ID of the kit piece
	At     vec3   // local
	Reach  float64
	Use    func(g content.Game, at vec3)
	Prompt string
}
