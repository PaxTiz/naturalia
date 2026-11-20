package game

import (
	"cmp"
	"fmt"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/planet"
	"slices"
)

// Bootcamps (docs/TELEPORTEURS.md) are entities whose kind lays out a
// small dome (content.EntityDef.Camp): the base's code builds it once per
// kind in its own frame (base.LayCamp), and each one stands it where it
// was placed (base.Site).

// Bootcamp is what a bootcamp keeps: an ID that stays the same from save
// to save, and its name.
type Bootcamp struct {
	ID   int       `json:"id"`
	Name string    `json:"name"`
	site base.Site // where it stands, set when it is placed (placeCamp)
}

// CampAt is a bootcamp standing on Mars: its entity, what it keeps, where
// it stands and how it is laid out.
type CampAt struct {
	Ent ecs.ID
	*Bootcamp
	Site   base.Site
	Layout *base.Camp
}

// ToWorld takes a point of the bootcamp's frame to the world.
func (c CampAt) ToWorld(local vec3) vec3 { return c.Site.ToWorld(local) }

// campLayout is how a kind of bootcamp is laid out, figured once; nil for
// what isn't one.
func (g *Game) campLayout(kind string) *base.Camp {
	def, ok := g.Content.Entities.Get(kind)
	if !ok || def.Camp == nil {
		return nil
	}
	c, ok := g.layouts[def.ID]
	if !ok {
		c = base.LayCamp(&g.Content.Kits, *def.Camp)
		if g.layouts == nil {
			g.layouts = map[string]*base.Camp{}
		}
		g.layouts[def.ID] = c
	}
	return c
}

// Camps are the bootcamps standing, in the order they were first placed;
// the one moved isn't.
func (g *Game) Camps() []CampAt {
	var out []CampAt
	for id, b := range g.Ents.Camp.All() {
		l := g.campLayout(g.kind(id))
		if l == nil || g.Held(id) {
			continue
		}
		out = append(out, CampAt{id, b, b.site, l})
	}
	slices.SortFunc(out, func(a, b CampAt) int { return cmp.Compare(a.ID, b.ID) })
	return out
}

// CampsNear are the bootcamps within dist of a point of the world, by
// their walls.
func (g *Game) CampsNear(at vec3, dist float64) []CampAt {
	var out []CampAt
	for _, c := range g.Camps() {
		if planet.ArcDist(at.Norm(), c.Site.Origin.Norm()) < dist+c.Layout.Reach {
			out = append(out, c)
		}
	}
	return out
}

// CampAround is the bootcamp a point of the world is inside, if any.
func (g *Game) CampAround(at vec3) (CampAt, bool) {
	for _, c := range g.CampsNear(at, 1) {
		if c.Layout.Indoors(c.Site.ToLocal(at)) {
			return c, true
		}
	}
	return CampAt{}, false
}

// Indoors tells whether a point of the world is inside the base or a
// bootcamp.
func (g *Game) Indoors(at vec3) bool {
	if base.Indoors(base.ToLocal(at)) {
		return true
	}
	_, ok := g.CampAround(at)
	return ok
}

// buildingAt is the ID of the building of the base whose room holds a
// local point.
func buildingAt(local vec3) string {
	if id := base.RoomAt(local); id != 0 {
		return base.Ents.Building.Get(id).Plan().ID
	}
	return ""
}

// buildingAt is the ID of the building a point of the world is in: a
// room of the base, or a bootcamp (its kind's); "" outside.
func (g *Game) buildingAt(at vec3) string {
	if in := buildingAt(base.ToLocal(at)); in != "" {
		return in
	}
	if c, ok := g.CampAround(at); ok {
		return g.kind(c.Ent)
	}
	return ""
}

// standCamp sets where bootcamp id stands, from its entity: upright on
// the ground where it was placed.
func (g *Game) standCamp(id ecs.ID) {
	b := g.Ents.Camp.Get(id)
	t := g.Ents.Transform.Get(id)
	if b == nil || t == nil {
		return
	}
	t.Alt, t.Pitch, t.Roll = 0, 0, 0
	pos, ax, up, az := g.Frame(t)
	b.site = base.Site{Origin: pos, X: ax, Y: up, Z: az}
}

// newCamp makes entity id a bootcamp, with the next ID and a name.
func (g *Game) newCamp(id ecs.ID) *Bootcamp {
	next := 1
	for _, b := range g.Ents.Camp.All() {
		next = max(next, b.ID+1)
	}
	return g.Ents.Camp.Set(id, Bootcamp{ID: next, Name: campName(next)})
}

// campName is a new bootcamp's name.
func campName(n int) string { return fmt.Sprintf("Bootcamp %d", n) }

// isCamp tells whether an entity's kind is a bootcamp.
func isCamp(def *content.EntityDef) bool { return def != nil && def.Camp != nil }
