package base

import (
	"cmp"
	"iter"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/engine/ecs"
	"naturalia/internal/engine/mathx"
	"slices"
)

// The base as entities (package ecs): each building, each corridor between
// two of them, each piece of furniture is one, made by Setup from the
// blueprints of the content. The rest of the game reads them from Ents.

// Entities are the base's entities and their components.
type Entities struct {
	*ecs.World
	Building  *ecs.Store[Room] // laid out from its blueprint
	Corridor  *ecs.Store[Corridor]
	Furniture *ecs.Store[Furniture]
	Use       *ecs.Store[Interactable]       // furniture the player uses with E
	Obstacles *ecs.Store[[]content.Obstacle] // what stops the player (local): furniture, the yard's clutter
	Floor     *ecs.Store[Floor]
	Lamps     *ecs.Store[[]Lamp] // furniture's lamps (local)
	Grows     *ecs.Store[int]    // furniture that grows night after night: its stages
}

func newEntities() *Entities {
	w := ecs.NewWorld()
	return &Entities{World: w, Building: ecs.NewStore[Room](w), Corridor: ecs.NewStore[Corridor](w),
		Furniture: ecs.NewStore[Furniture](w), Use: ecs.NewStore[Interactable](w), Obstacles: ecs.NewStore[[]content.Obstacle](w), Floor: ecs.NewStore[Floor](w),
		Lamps: ecs.NewStore[[]Lamp](w), Grows: ecs.NewStore[int](w)}
}

// Ents is the base laid out by Setup.
var Ents = newEntities()

// Furniture is a kit piece standing in a building.
type Furniture struct {
	Kit  string  // its qualified ID
	At   vec3    // local, on the floor
	Yaw  float64 // in degrees, as in the blueprint
	Seed int
	In   ecs.ID // its building
}

// Buildings goes through the buildings in the order of the content's,
// the hub first.
func Buildings() iter.Seq2[ecs.ID, *Room] {
	return ordered(Ents.Building)
}

// Corridors goes through the corridors, the airlock's last.
func Corridors() iter.Seq2[ecs.ID, *Corridor] {
	return ordered(Ents.Corridor)
}

// AllFurniture goes through the furniture, building after building.
func AllFurniture() iter.Seq2[ecs.ID, *Furniture] {
	return ordered(Ents.Furniture)
}

// FurnitureOf goes through the furniture of building in, in the order of
// its blueprint's props.
func FurnitureOf(in ecs.ID) iter.Seq2[ecs.ID, *Furniture] {
	return func(yield func(ecs.ID, *Furniture) bool) {
		for id, f := range ordered(Ents.Furniture) {
			if f.In == in && !yield(id, f) {
				return
			}
		}
	}
}

// ordered goes through a store in the order its entities were made:
// always the same, unlike the store's.
func ordered[T any](s *ecs.Store[T]) iter.Seq2[ecs.ID, *T] {
	return func(yield func(ecs.ID, *T) bool) {
		for id := range Ents.All() {
			if c := s.Get(id); c != nil && !yield(id, c) {
				return
			}
		}
	}
}

// NearBuildings are the n buildings closest to a local point (by their
// walls), in the order of Buildings.
func NearBuildings(at vec3, n int) []ecs.ID {
	return nearest(Ents.Building, n, func(r *Room) float64 {
		return math.Hypot(at.X-r.Center.X, at.Z-r.Center.Z) - r.reach()
	})
}

// NearCorridors are the n corridors closest to a local point, in the order
// of Corridors.
func NearCorridors(at vec3, n int) []ecs.ID {
	return nearest(Ents.Corridor, n, func(c *Corridor) float64 {
		flat := vec3{at.X, 0, at.Z}
		d := c.To.Sub(c.From)
		t := mathx.Clamp01(flat.Sub(c.From).Dot(d) / d.Dot(d))
		return flat.Sub(c.From.Add(d.Scale(t))).Length()
	})
}

// nearest keeps the n entities of s closest by dist, in creation order.
func nearest[T any](s *ecs.Store[T], n int, dist func(*T) float64) []ecs.ID {
	type near struct {
		id ecs.ID
		d  float64
		k  int
	}
	var all []near
	for id, c := range ordered(s) {
		all = append(all, near{id, dist(c), len(all)})
	}
	if len(all) > n {
		slices.SortStableFunc(all, func(a, b near) int { return cmp.Compare(a.d, b.d) })
		all = all[:n]
		slices.SortFunc(all, func(a, b near) int { return a.k - b.k })
	}
	out := make([]ecs.ID, len(all))
	for i, a := range all {
		out[i] = a.id
	}
	return out
}

// Hub is the first building, where the airlock leads.
func Hub() (ecs.ID, *Room) {
	for id, r := range Buildings() {
		return id, r
	}
	return 0, nil
}

// Obstacles lists what stops the player in the base: the yard's clutter,
// then the furniture.
func Obstacles() []content.Obstacle {
	var out []content.Obstacle
	for _, o := range ordered(Ents.Obstacles) {
		out = append(out, *o...)
	}
	return out
}

// Interactables lists the furniture the player can use with E.
func Interactables() []Interactable {
	var out []Interactable
	for _, it := range ordered(Ents.Use) {
		out = append(out, *it)
	}
	return out
}

// TableSpot is where the map table stands, under the hologram (local).
func TableSpot() vec3 {
	for _, f := range ordered(Ents.Furniture) {
		if f.Kit == "core:maptable" {
			return f.At
		}
	}
	return vec3{}
}
