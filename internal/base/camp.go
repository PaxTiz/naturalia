package base

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/mesh"
)

// A bootcamp (docs/TELEPORTEURS.md) is a small closed dome of the base
// standing alone anywhere on Mars: a bed, a workbench, the hologram. It is
// laid out once per blueprint by the base's own code, in a frame of its
// own (its floor's middle at the origin, y up, its door toward +X at the
// end of a short airlock), then stood where each bootcamp is (Site).
//
//	   ( dome ) ═══ airlock ▐ door → outside

// Camp is a bootcamp laid out, in its own frame.
type Camp struct {
	Room      Room
	Airlock   Corridor
	Walls     []Segment
	Lamps     []Lamp
	Uses      []Interactable
	Obstacles []content.Obstacle
	Furniture []Furniture
	Table     vec3        // where the hologram's table stands
	Wake      vec3        // where the player wakes up, by the bed
	Beacon    vec3        // the light blinking on its roof
	Reach     float64     // how far from its middle it stands: the ground is flat under it
	Parts     *mesh.Parts // its static pieces
}

// campAirlock is how far its airlock reaches out of the dome's wall.
const campAirlock = 2.2

// LayCamp lays a bootcamp out from its blueprint, a dome (Shape) and its
// props, with kits' furniture. It lays it as Setup does the base: the
// base's entities are set aside meanwhile.
func LayCamp(kits *content.Registry[content.Kit], plan content.Blueprint) *Camp {
	bp := plan // its own doors
	bp.At, bp.Yaw, bp.Hall, bp.Doors, bp.Floors = vec3{}, 0, nil, nil, nil
	ents, origin, airlock := Ents, Origin, AirlockEnd
	defer func() { Ents, Origin, AirlockEnd = ents, origin, airlock }()
	Ents, Origin = newEntities(), vec3{}

	out := vec3{1, 0, 0}
	from := out.Scale(edge(&bp, out))
	AirlockEnd = out.Scale(bp.Shape.Radius + campAirlock)
	addDoor(&bp, East, from)
	room := Ents.New()
	Ents.Building.Set(room, roomOf(&bp))
	lock := Ents.New()
	Ents.Corridor.Set(lock, Corridor{from, AirlockEnd, DoorWidth + 0.2, 2.4, [2]ecs.ID{room, 0}})
	for _, p := range bp.Props {
		addFurniture(kits, room, &bp, p)
	}

	c := &Camp{
		Room: *Ents.Building.Get(room), Airlock: *Ents.Corridor.Get(lock),
		Walls: shellWalls(false), Lamps: Lamps([]ecs.ID{room}, []ecs.ID{lock}),
		Uses: Interactables(), Obstacles: Obstacles(),
		Table: TableSpot(), Beacon: BeaconTop(), Reach: AirlockEnd.X + 0.6,
	}
	for _, f := range AllFurniture() {
		c.Furniture = append(c.Furniture, *f)
		if f.Kit == "core:bed" && c.Wake == (vec3{}) {
			c.Wake = f.At.Add(f.At.Scale(-1).Norm().Scale(1.5)) // toward the middle
		}
	}
	if c.Wake == (vec3{}) {
		c.Wake = AirlockEnd.Scale(0.4)
	}

	b := &building{Parts: mesh.NewParts(vec3{}), kits: kits}
	r := c.Room
	b.room(room, r)
	if !r.glazed() {
		b.greebleRoom(room, r, 11)
	}
	b.corridor(c.Airlock)
	b.greebleCorridor(c.Airlock, 5)
	b.plates()
	b.campDoor()
	b.outside()
	b.Unfight()
	c.Parts = b.Parts
	return c
}

// campDoor: the outer frame of the airlock's door; the leaves slide (see
// drawCampDoors).
func (b *building) campDoor() {
	end := AirlockEnd
	across := vec3{0, 0, 1}
	half := (DoorWidth+0.2)/2 + WallThick
	for _, s := range []float64{-1, 1} {
		b.Part("hazard", mesh.LitSun, vec3{}).Box(end.Add(vec3{0, 1.35, s * half}), vec3{0.3, 2.7, 0.35}, across, 0.04)
	}
	b.Part("steel.dark", mesh.LitSun, vec3{}).Box(end.Add(vec3{0, 2.6, 0}), vec3{2*half + 0.3, 0.3, 0.35}, across, 0.05)
}

// Door is the middle of the airlock's outer door, on the floor, and the
// way across it.
func (c *Camp) Door() (at, across vec3) { return c.Airlock.To, c.Airlock.side() }

// Indoors tells whether a point of the camp's frame is inside it: in the
// dome or in its airlock.
func (c *Camp) Indoors(p vec3) bool {
	if p.Y < 3.5 && math.Hypot(p.X-c.Room.Center.X, p.Z-c.Room.Center.Z) < c.Room.Radius {
		return true
	}
	a := c.Airlock
	d := a.To.Sub(a.From)
	flat := vec3{p.X, 0, p.Z}
	t := flat.Sub(a.From).Dot(d) / d.Dot(d)
	return p.Y < a.Height && t >= 0 && t <= 1 && math.Abs(flat.Sub(a.From).Dot(a.side())) < a.Width/2
}
