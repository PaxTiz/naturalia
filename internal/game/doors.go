package game

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
)

// Doors slide open as someone comes close: the bootcamps' (camps.go), and
// those of entities with leaves (content.EntityDef.Leaves).

// door is how open a door is, 0 closed to 1 open, and where it is going.
type door struct{ open, want float64 }

// slideDoor moves entity id's door a frame toward open or closed.
func (g *Game) slideDoor(id ecs.ID, open bool, dt float64) {
	if g.doors == nil {
		g.doors = map[ecs.ID]door{}
	}
	d := g.doors[id]
	want := 0.0
	if open {
		want = 1
	}
	if want != d.want {
		d.want = want
		g.sfx.Airlock()
	}
	d.open += (want - d.open) * math.Min(1, dt*4)
	if math.Abs(want-d.open) < 0.002 {
		d.open = want
	}
	g.doors[id] = d
}

const (
	doorsNear = 40.0 // how far from the player doors move
	doorReach = 2.5  // how close to a door one opens it, on foot
	carReach  = 7.0  // and in a vehicle
)

// updateDoors opens the doors near that the player or a vehicle comes to.
func (g *Game) updateDoors(dt float64) {
	var cars []vec3
	near := g.EntitiesNear(doorsNear)
	for _, id := range near {
		if _, ok := g.vehicleOf(id); ok && id != g.Riding {
			cars = append(cars, g.Ents.Transform.Get(id).Dir)
		}
	}
	for _, id := range near {
		def, ok := g.Content.Entities.Get(g.kind(id))
		if !ok || len(def.Leaves) == 0 || g.Held(id) {
			continue
		}
		reach := doorReach
		if g.Riding != 0 {
			reach = carReach
		}
		open := g.byDoor(id, def, g.Player.Pos, reach)
		for _, c := range cars {
			open = open || g.byDoor(id, def, g.World.Relief.Surface(c), 4)
		}
		g.slideDoor(id, open, dt)
	}
}

// byDoor tells whether a point of the world is within reach of a door, on
// its floor.
func (g *Game) byDoor(id ecs.ID, def *content.EntityDef, at vec3, reach float64) bool {
	pos, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
	lo, hi := content.Bounds(leafParts(def))
	d := at.Sub(pos)
	x, y, z := d.Dot(ax), d.Dot(up), d.Dot(az)
	out := func(v, lo, hi float64) float64 { return math.Max(0, math.Max(lo-v, v-hi)) }
	return math.Hypot(out(x, lo.X, hi.X), out(z, lo.Z, hi.Z)) < reach && y > lo.Y-1.5 && y < hi.Y+0.5
}

// leafParts are the parts of a door's leaves.
func leafParts(def *content.EntityDef) []content.PartJSON {
	var out []content.PartJSON
	for _, l := range def.Leaves {
		out = append(out, l.Parts...)
	}
	return out
}

// LeafOpen is how open entity id's door is, 0 closed to 1 open.
func (g *Game) LeafOpen(id ecs.ID) float64 { return g.doors[id].open }

// LeafShift is how far the leaf i of entity id's door has slid, in the
// world.
func (g *Game) LeafShift(id ecs.ID, i int) vec3 {
	def, ok := g.Content.Entities.Get(g.kind(id))
	if !ok || i >= len(def.Leaves) {
		return vec3{}
	}
	_, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
	s := def.Leaves[i].Slide.Scale(g.doors[id].open)
	return ax.Scale(s.X).Add(up.Scale(s.Y)).Add(az.Scale(s.Z))
}
