package main

import (
	"fmt"
	"log"
	"slices"
	"strings"
)

// Content is everything the game is made of, filled by packs (docs/MODS.md).
// The base game is itself a pack, "core", registered like any other: if it
// can do everything through this API, so can a mod.
//
// IDs are namespaced, "pack:name". A pack that adds an ID already taken
// overrides that entry, which keeps its place in the order; Patch changes
// some fields of an entry instead.
//
// A bare name ("steel") stands for the core pack's ("core:steel").
type Content struct {
	Tools     Registry[Tool]
	Upgrades  Registry[Upgrade]
	Tokens    Registry[token]     // materials of the buildings
	Kits      Registry[kitProp]   // furniture and other kit pieces
	Buildings Registry[Blueprint] // the first one is the hub
}

// Registry holds one kind of content, in registration order.
type Registry[T any] struct {
	kind  string
	ids   []string
	items map[string]*T
	owner map[string]string // pack that last set each entry
	pack  string            // pack being registered
}

func (r *Registry[T]) init(kind string) {
	r.kind, r.items, r.owner = kind, map[string]*T{}, map[string]string{}
}

// Add registers an entry, or overrides the one with the same ID.
func (r *Registry[T]) Add(id string, v T) {
	if !validID(id) {
		log.Printf("%s %q ignored: IDs are \"pack:name\"", r.kind, id)
		return
	}
	if _, ok := r.items[id]; ok {
		log.Printf("%s %s: %s overrides %s", r.kind, id, r.pack, r.owner[id])
	} else {
		r.ids = append(r.ids, id)
	}
	r.items[id], r.owner[id] = &v, r.pack
}

// Patch changes an entry registered earlier; it reports whether it exists.
func (r *Registry[T]) Patch(id string, f func(*T)) bool {
	id = qualify(id)
	v, ok := r.items[id]
	if ok {
		f(v)
		r.owner[id] = r.pack
	}
	return ok
}

// Remove takes an entry out, for a mod that replaces part of the game.
func (r *Registry[T]) Remove(id string) {
	id = qualify(id)
	if _, ok := r.items[id]; ok {
		delete(r.items, id)
		r.ids = slices.DeleteFunc(r.ids, func(s string) bool { return s == id })
	}
}

func (r *Registry[T]) Get(id string) (*T, bool) {
	v, ok := r.items[qualify(id)]
	return v, ok
}

// IDs lists the entries in registration order.
func (r *Registry[T]) IDs() []string { return r.ids }

func (r *Registry[T]) Len() int { return len(r.ids) }

// All lists the entries in registration order.
func (r *Registry[T]) All() []*T {
	out := make([]*T, len(r.ids))
	for i, id := range r.ids {
		out[i] = r.items[id]
	}
	return out
}

// qualify puts a bare name in the core pack.
func qualify(id string) string {
	if strings.Contains(id, ":") {
		return id
	}
	return "core:" + id
}

func validID(id string) bool {
	ns, name, ok := strings.Cut(id, ":")
	return ok && ns != "" && name != "" && !strings.ContainsAny(id, " /\\")
}

// Pack is a bundle of content. Packs compiled into the game register
// themselves from an init function, like database/sql drivers.
type Pack struct {
	ID       string
	Name     string
	Version  string
	Requires []string // packs loaded before this one
	Register func(c *Content)
}

var packs []Pack

func RegisterPack(p Pack) { packs = append(packs, p) }

// content is the game's content once every pack is loaded (loadContent).
var content *Content

// loadContent runs the packs, dependencies first. A pack whose requirements
// are missing is skipped.
func loadContent() {
	c := &Content{}
	c.Tools.init("tool")
	c.Upgrades.init("upgrade")
	c.Tokens.init("token")
	c.Kits.init("kit")
	c.Buildings.init("building")

	byID := map[string]Pack{}
	for _, p := range packs {
		byID[p.ID] = p
	}
	state := map[string]int{} // 1 loading, 2 loaded, 3 failed
	var load func(id string) bool
	load = func(id string) bool {
		switch state[id] {
		case 1:
			log.Printf("pack %s: circular requirement", id)
			return false
		case 2:
			return true
		case 3:
			return false
		}
		p, ok := byID[id]
		if !ok {
			return false
		}
		state[id] = 1
		for _, dep := range p.Requires {
			if !load(dep) {
				log.Printf("pack %s skipped: needs %s", id, dep)
				state[id] = 3
				return false
			}
		}
		c.setPack(p.ID)
		p.Register(c)
		state[id] = 2
		return true
	}
	// core first, then the others in the order they registered.
	load("core")
	for _, p := range packs {
		load(p.ID)
	}
	c.finish()
	content = c
	setupBase()
}

func (c *Content) setPack(id string) {
	c.Tools.pack = id
	c.Upgrades.pack = id
	c.Tokens.pack = id
	c.Kits.pack = id
	c.Buildings.pack = id
}

// finish checks the content once every pack is loaded.
func (c *Content) finish() {
	if c.Tools.Len() == 0 {
		panic("no tool registered: is the core pack missing?")
	}
	// Tools are tiers: from the cheapest up, whoever added them.
	slices.SortStableFunc(c.Tools.ids, func(a, b string) int {
		ca, cb := c.Tools.items[a].Cost, c.Tools.items[b].Cost
		switch {
		case ca < cb:
			return -1
		case ca > cb:
			return 1
		}
		return 0
	})
	for _, id := range c.Tools.ids {
		c.Tools.items[id].ID = id
	}
	for _, id := range c.Upgrades.ids {
		c.Upgrades.items[id].ID = id
	}
	for _, id := range c.Buildings.ids {
		c.Buildings.items[id].ID = id
	}
}

// packList describes the loaded packs, for the log.
func packList() string {
	var s []string
	for _, p := range packs {
		s = append(s, fmt.Sprintf("%s %s", p.ID, p.Version))
	}
	return strings.Join(s, ", ")
}
