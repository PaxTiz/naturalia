// Package ecs keeps entities: an entity is only an ID, and what it is and
// does comes from the components attached to it, each kind of component
// kept in its own Store, packed in a slice that systems run through.
package ecs

import "iter"

// ID names an entity: its slot in the World (low 24 bits) and the slot's
// generation (high 8 bits). Once the entity is killed, the slot is reused
// under a new generation, and the old ID names nothing any more instead of
// naming someone else. 0 is no entity.
type ID uint32

const (
	indexBits = 24
	indexMask = 1<<indexBits - 1
)

// Index is the entity's slot: small, dense, stable while it lives.
func (id ID) Index() int { return int(id & indexMask) }

func (id ID) gen() uint8 { return uint8(id >> indexBits) }

func makeID(index int, gen uint8) ID { return ID(gen)<<indexBits | ID(index) }

// World hands out IDs and kills entities, with all their components.
type World struct {
	gens   []uint8 // by slot: its generation (slot 0 is never used)
	live   []bool
	free   []int
	count  int
	stores []remover
}

type remover interface{ Remove(ID) }

func NewWorld() *World {
	return &World{gens: []uint8{0}, live: []bool{false}}
}

// New makes an entity, with no component yet.
func (w *World) New() ID {
	var i int
	if n := len(w.free); n > 0 {
		i, w.free = w.free[n-1], w.free[:n-1]
	} else {
		if len(w.gens) > indexMask {
			panic("ecs: too many entities")
		}
		i = len(w.gens)
		w.gens, w.live = append(w.gens, 0), append(w.live, false)
	}
	w.live[i] = true
	w.count++
	return makeID(i, w.gens[i])
}

// Alive tells if id names an entity still in the world.
func (w *World) Alive(id ID) bool {
	i := id.Index()
	return i > 0 && i < len(w.gens) && w.live[i] && w.gens[i] == id.gen()
}

// Kill removes an entity and its components. Killing a dead one does
// nothing.
func (w *World) Kill(id ID) {
	if !w.Alive(id) {
		return
	}
	for _, s := range w.stores {
		s.Remove(id)
	}
	i := id.Index()
	w.live[i] = false
	w.gens[i]++
	w.free = append(w.free, i)
	w.count--
}

// Len is how many entities live.
func (w *World) Len() int { return w.count }

// All goes through the living entities in slot order: always the same
// for the same world, unlike a store's order.
func (w *World) All() iter.Seq[ID] {
	return func(yield func(ID) bool) {
		for i := 1; i < len(w.gens); i++ {
			if w.live[i] && !yield(makeID(i, w.gens[i])) {
				return
			}
		}
	}
}

// Store holds one kind of component, as a sparse set: the components packed
// in a slice, the IDs they belong to alongside, and by slot where each is.
// Getting, setting and removing take constant time.
//
// A pointer returned by Get or Set stays valid until the next Set or Remove
// on the same store.
type Store[T any] struct {
	at    []int32 // by slot: position in dense, +1 (0: none)
	ids   []ID
	dense []T
}

// NewStore makes a store whose components go when w kills their entity.
func NewStore[T any](w *World) *Store[T] {
	s := &Store[T]{}
	w.stores = append(w.stores, s)
	return s
}

func (s *Store[T]) pos(id ID) int {
	i := id.Index()
	if i >= len(s.at) || s.at[i] == 0 {
		return -1
	}
	p := int(s.at[i] - 1)
	if s.ids[p] != id {
		return -1 // the slot's former entity
	}
	return p
}

// Set attaches v to id, or replaces its component.
func (s *Store[T]) Set(id ID, v T) *T {
	if p := s.pos(id); p >= 0 {
		s.dense[p] = v
		return &s.dense[p]
	}
	i := id.Index()
	for len(s.at) <= i {
		s.at = append(s.at, 0)
	}
	s.ids = append(s.ids, id)
	s.dense = append(s.dense, v)
	s.at[i] = int32(len(s.dense))
	return &s.dense[len(s.dense)-1]
}

// Get is id's component, nil if it has none.
func (s *Store[T]) Get(id ID) *T {
	if p := s.pos(id); p >= 0 {
		return &s.dense[p]
	}
	return nil
}

func (s *Store[T]) Has(id ID) bool { return s.pos(id) >= 0 }

// Remove detaches id's component: the last one takes its place.
func (s *Store[T]) Remove(id ID) {
	p := s.pos(id)
	if p < 0 {
		return
	}
	last := len(s.dense) - 1
	s.dense[p], s.ids[p] = s.dense[last], s.ids[last]
	s.at[s.ids[p].Index()] = int32(p + 1)
	s.at[id.Index()] = 0
	var zero T
	s.dense[last] = zero // let go of what it pointed to
	s.dense, s.ids = s.dense[:last], s.ids[:last]
}

func (s *Store[T]) Len() int { return len(s.dense) }

// All goes through the components, last first: removing the one being
// visited, or killing its entity, is safe.
func (s *Store[T]) All() iter.Seq2[ID, *T] {
	return func(yield func(ID, *T) bool) {
		for p := len(s.dense) - 1; p >= 0; p-- {
			if p < len(s.dense) && !yield(s.ids[p], &s.dense[p]) {
				return
			}
		}
	}
}
