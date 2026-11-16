package base

import (
	"iter"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
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
}

func newEntities() *Entities {
	w := ecs.NewWorld()
	return &Entities{World: w, Building: ecs.NewStore[Room](w), Corridor: ecs.NewStore[Corridor](w),
		Furniture: ecs.NewStore[Furniture](w), Use: ecs.NewStore[Interactable](w), Obstacles: ecs.NewStore[[]content.Obstacle](w)}
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
