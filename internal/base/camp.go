package base

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/engine/ecs"
	"naturalia/internal/engine/mesh"
	"slices"
)

// A camp is a building of the base standing alone anywhere on Mars: a
// bootcamp (a small closed dome: a bed, a workbench, the hologram), a
// greenhouse, a garage... It is laid out once per blueprint by the base's
// own code, in a frame of its own (its floor's middle at the origin, y up,
// its first port toward +X), then stood where each one is (Site). Its ports
// are doors at the end of short airlocks, that links lead from (link.go).
//
//	   ( dome ) ═══ airlock ▐ port → outside

// Camp is a building laid out, in its own frame.
type Camp struct {
	Room      Room
	Ports     []Port
	Walls     []Segment
	Lamps     []Lamp
	Uses      []Interactable
	Obstacles []content.Obstacle
	Furniture []Furniture
	Lights    []content.Light // what lights its glass rooms, at night
	Table     vec3            // where the hologram's table stands, if it has one (Hologram)
	Wake      vec3            // where the player wakes up, by the bed
	Sign      vec3            // the middle of its name's sign, standing on the door's header, facing out
	Beacon    vec3            // the light blinking on its roof
	Reach     float64         // how far from its middle it stands: the ground is flat under it
	Parts     *mesh.Parts     // its static pieces
	Bays      []Bay           // a garage's
	hologram  bool
}

// Port is one of a building's ways out: its airlock, from the room to its
// outer door.
type Port struct{ Airlock Corridor }

// At is the middle of the port's outer door, on the floor.
func (p Port) At() vec3 { return p.Airlock.To }

// Out is the way out of it.
func (p Port) Out() vec3 { return p.Airlock.To.Sub(p.Airlock.From).Norm() }

// Across is the way across its door.
func (p Port) Across() vec3 { return p.Airlock.side() }

// campAirlock is how far its airlock reaches out of the dome's wall.
const campAirlock = 2.2

// LayCamp lays a building out from its blueprint, a dome (Shape) or a hall
// and its props, with kits' furniture, its ports facing ways (degrees from
// +X toward +Z; none: one, toward +X). It lays it as Setup does the base:
// the base's entities are set aside meanwhile.
func LayCamp(kits *content.Registry[content.Kit], plan content.Blueprint, ways []float64) *Camp {
	bp := plan // its own doors
	bp.At, bp.Yaw, bp.Doors, bp.Floors = vec3{}, 0, nil, nil
	if len(ways) == 0 {
		ways = []float64{0}
	}
	ents, origin, airlock := Ents, Origin, AirlockEnd
	defer func() { Ents, Origin, AirlockEnd = ents, origin, airlock }()
	Ents, Origin = newEntities(), vec3{}

	var locks []Corridor
	for i, w := range ways {
		out := content.Turn(vec3{1, 0, 0}, w)
		from := out.Scale(edge(&bp, out))
		end := out.Scale(edge(&bp, out) + 0.2 + campAirlock)
		if i == 0 {
			AirlockEnd = end
		}
		addDoor(&bp, math.Atan2(out.Z, out.X), from)
		locks = append(locks, Corridor{From: from, To: end, Width: DoorWidth + 0.2, Height: 2.4})
	}
	room := Ents.New()
	laid := roomOf(&bp)
	laid.apron = 2.4
	Ents.Building.Set(room, laid)
	var corrs []ecs.ID
	for _, l := range locks {
		l.Rooms = [2]ecs.ID{room, 0}
		id := Ents.New()
		Ents.Corridor.Set(id, l)
		corrs = append(corrs, id)
	}
	for _, p := range bp.Props {
		addFurniture(kits, room, &bp, p)
	}

	c := &Camp{
		Room: *Ents.Building.Get(room), Walls: shellWalls(false), Lamps: Lamps([]ecs.ID{room}, corrs),
		Uses: Interactables(), Obstacles: Obstacles(),
		Table: TableSpot(), Reach: Ents.Building.Get(room).reach() + 0.6,
	}
	if r := c.Room; r.Hall == nil && r.Radius < 4 { // as bright as in the others, closer to its walls
		c.Lamps[0].Power *= r.Radius * r.Radius / 16 * 0.65
	}
	for _, l := range locks {
		c.Ports = append(c.Ports, Port{l})
		c.Reach = math.Max(c.Reach, l.To.Length()+0.6)
	}
	if c.Room.skylight() { // its lamp hangs by the door, clear of what stands in the middle
		c.Lamps[0].At = c.Lamps[0].At.Add(c.Ports[0].Out().Scale(c.Room.Radius * 0.55))
	}
	if r := c.Room; r.Hall != nil { // its seven lamps hang low, over a small floor
		for i := range c.Lamps {
			if i == 0 || i >= len(c.Lamps)-len(r.hallLamps())+1 {
				c.Lamps[i].Power *= 0.45
			}
		}
	}
	for _, f := range AllFurniture() {
		c.Furniture = append(c.Furniture, *f)
		if f.Kit == "core:bed" && c.Wake == (vec3{}) {
			c.Wake = f.At.Add(f.At.Scale(-1).Norm().Scale(1.5)) // toward the middle
		}
	}
	c.Sign = AirlockEnd.Add(vec3{0, 3.0, 0})
	c.hologram = slices.ContainsFunc(c.Furniture, func(f Furniture) bool { return f.Kit == "core:maptable" })
	if c.Wake == (vec3{}) {
		c.Wake = AirlockEnd.Scale(0.4)
	}
	if c.Room.Hall != nil && bp.Garage {
		c.Bays = campBays(c.Room)
	}
	if c.Room.skylight() {
		c.Beacon = c.Room.Center.Add(vec3{0, c.Room.WallH + c.Room.Radius*0.55 + 0.15, 0})
	} else if c.Room.glazed() {
		top := c.Room.WallH + c.Room.Radius*0.55
		c.Beacon = c.Room.Center.Add(vec3{0, top + 0.15, 0})
		c.Lights = append(c.Lights, content.Light{At: vec3{0, top - 0.5, 0}, Power: 3, Range: c.Room.Radius + top, Night: true})
	} else {
		c.Beacon = BeaconTop()
	}

	b := &building{Parts: mesh.NewParts(vec3{}), kits: kits, bays: c.Bays}
	r := c.Room
	if r.Hall != nil {
		b.hall(room, r)
	} else {
		b.room(room, r)
		if !r.glazed() {
			b.greebleRoom(room, r, 11)
		}
	}
	for i, p := range c.Ports {
		b.corridor(p.Airlock)
		b.greebleCorridor(p.Airlock, 5+i)
	}
	b.plates()
	for _, p := range c.Ports {
		b.campDoor(p)
	}
	if !r.glazed() && !r.skylight() {
		b.outside()
	}
	b.Unfight()
	c.Parts = b.Parts
	return c
}

// campDoor: the outer frame of a port's door; the leaves slide (see
// drawCampDoors).
func (b *building) campDoor(p Port) {
	end, across := p.At(), p.Across()
	half := (DoorWidth+0.2)/2 + WallThick
	for _, s := range []float64{-1, 1} {
		b.Part("hazard", mesh.LitSun, vec3{}).Box(end.Add(across.Scale(s*half)).Add(vec3{0, 1.35, 0}), vec3{0.3, 2.7, 0.35}, across, 0.04)
	}
	b.Part("steel.dark", mesh.LitSun, vec3{}).Box(end.Add(vec3{0, 2.6, 0}), vec3{2*half + 0.3, 0.3, 0.35}, across, 0.05)
}

// Door is the middle of the first port's outer door, on the floor, and
// the way across it.
func (c *Camp) Door() (at, across vec3) { return c.Ports[0].At(), c.Ports[0].Across() }

// BigDoor tells whether the camp is a hall with a big door: its middle on
// the floor, and how wide it is.
func (c *Camp) BigDoor() (mid vec3, width float64, ok bool) {
	if !c.Room.bigDoor() {
		return vec3{}, 0, false
	}
	mid, _ = c.Room.door()
	return mid, c.Room.Hall.Door[0], true
}

// BigDoorWalls are what its big door, open from 0 to 1, stops; BigDoorLeaves
// where its leaves stand.
func (c *Camp) BigDoorWalls(open float64) []Segment { return c.Room.doorWalls(open) }
func (c *Camp) BigDoorLeaves(open float64) []Leaf   { return c.Room.turnedLeaves(open) }

// Hologram tells whether the camp has a table to travel from.
func (c *Camp) Hologram() bool { return c.hologram }

// Indoors tells whether a point of the camp's frame is inside it: in its
// room or in one of its airlocks.
func (c *Camp) Indoors(p vec3) bool {
	r := c.Room
	if r.Hall != nil {
		if r.inHall(p, 0) && p.Y < r.WallH+r.Hall.Rise {
			return true
		}
	} else if p.Y < 3.5 && math.Hypot(p.X-r.Center.X, p.Z-r.Center.Z) < r.Radius {
		return true
	}
	for _, port := range c.Ports {
		a := port.Airlock
		d := a.To.Sub(a.From)
		flat := vec3{p.X, 0, p.Z}
		t := flat.Sub(a.From).Dot(d) / d.Dot(d)
		if p.Y < a.Height && t >= 0 && t <= 1 && math.Abs(flat.Sub(a.From).Dot(a.side())) < a.Width/2 {
			return true
		}
	}
	return false
}

// Ray is how far along a ray of the camp's frame from o along d (length
// 1), within max, it meets the walls or the roof of its room, seen as a
// round tower or a box.
func (c *Camp) Ray(o, d vec3, max float64) (float64, bool) {
	r := c.Room
	if r.Hall == nil {
		return rayTower(o, d, max, r.Radius, r.WallH+r.Radius*0.55)
	}
	hx, hz := r.half()
	in, out := 0.0, max
	for _, s := range [][4]float64{{o.X, d.X, -hx, hx}, {o.Z, d.Z, -hz, hz}, {o.Y, d.Y, 0, r.WallH + r.Hall.Rise}} {
		if math.Abs(s[1]) < 1e-9 {
			if s[0] < s[2] || s[0] > s[3] {
				return 0, false
			}
			continue
		}
		a, b := (s[2]-s[0])/s[1], (s[3]-s[0])/s[1]
		in, out = math.Max(in, math.Min(a, b)), math.Min(out, math.Max(a, b))
	}
	return in, in < out && in > 0
}

// rayTower is how far along a ray it meets a round tower standing on the
// origin.
func rayTower(o, d vec3, max, radius, top float64) (float64, bool) {
	// x² + z² = R² along the ray, seen from above.
	a := d.X*d.X + d.Z*d.Z
	b := 2 * (o.X*d.X + o.Z*d.Z)
	k := o.X*o.X + o.Z*o.Z - radius*radius
	if a < 1e-12 || b*b-4*a*k < 0 {
		return 0, false
	}
	t := (-b - math.Sqrt(b*b-4*a*k)) / (2 * a)
	if y := o.Y + t*d.Y; t < 0 || t > max || y < 0 || y > top {
		return 0, false
	}
	return t, true
}
