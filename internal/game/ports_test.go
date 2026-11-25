package game

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/planet"
	"testing"
)

// standing is baseGame with a building of that kind placed 400 m east of
// the base, turned some.
func standing(t *testing.T, g *Game, kind string) CampAt {
	t.Helper()
	id := g.add(Placed{Kind: kind, Dir: planet.Toward(vec3{1, 0, 0.3}, 400), Yaw: 0.7})
	g.standCamps()
	c, ok := g.campOf(id)
	if !ok {
		t.Fatalf("%s isn't standing", kind)
	}
	return c
}

// atPort stands the player d m out of a building's port, facing it.
func atPort(g *Game, c CampAt, i int, d float64) {
	p := c.Layout.Ports[i]
	g.Player.Pos = g.World.Relief.Surface(c.ToWorld(p.At().Add(p.Out().Scale(d))))
	g.Player.Heading = c.Site.Dir(p.Out().Scale(-1))
	g.Player.vel = vec3{}
	g.phys = nil
}

// Every building is walked into through each of its ports, up to its
// middle or to what stands there, and its walls stop the player elsewhere.
func TestPorts(t *testing.T) {
	g := baseGame(t)
	g.Content.Entities.Add("test:cross", content.EntityDef{ID: "test:cross", Name: "Cross", Ports: []float64{0, 90, 180, 270},
		Camp: &content.Blueprint{Shape: content.Dome{Radius: 3, WallH: 2.6}, Style: "habitat.white", Accent: "neon.cyan"}})
	g.Content.Entities.Add("test:hall", content.EntityDef{ID: "test:hall", Name: "Hall", Ports: []float64{0, 90},
		Camp: &content.Blueprint{Hall: &content.Hall{Size: [2]float64{10, 12}, WallH: 4, Rise: 1.5}, Style: "habitat.white", Accent: "neon.cyan"}})
	for _, def := range g.Content.Entities.All() {
		if def.Camp == nil {
			continue
		}
		c := standing(t, g, def.ID)
		l := c.Layout
		if want := max(1, len(def.Ports)); len(l.Ports) != want {
			t.Errorf("%s: %d ports, want %d", def.ID, len(l.Ports), want)
		}
		for i, p := range l.Ports {
			atPort(g, c, i, 2)
			walk(g, c.Site.Dir(p.Out().Scale(-1)), 2.5)
			at := c.Site.ToLocal(g.Player.Pos)
			if in := p.Airlock.From.Dot(p.Out()) - at.Dot(p.Out()); in < 0.5 || !g.Indoors(g.Player.Eye()) {
				t.Errorf("%s: in through port %d, ended %.2f m in, at %v", def.ID, i, in, at)
			}
			g.Player.want, g.Player.vel = vec3{}, vec3{}
			walk(g, c.Site.Dir(p.Out()), 3)
			if g.Indoors(g.Player.Eye()) {
				t.Errorf("%s: never out again by port %d", def.ID, i)
			}
		}
		for a := 0.0; a < 360; a += 7.5 {
			out := content.Turn(vec3{1, 0, 0}, a)
			free := true
			for _, p := range l.Ports {
				free = free && math.Acos(math.Min(1, out.Dot(p.Out()))) > 0.75
			}
			if !free {
				continue
			}
			g.Player.Pos = g.World.Relief.Surface(c.ToWorld(out.Scale(l.Reach + 1)))
			g.Player.vel, g.phys = vec3{}, nil
			walk(g, c.Site.Dir(out.Scale(-1)), 3)
			if g.Indoors(g.Player.Eye()) {
				t.Errorf("%s: in through the wall at %v°", def.ID, a)
			}
		}
		g.Ents.Kill(c.Ent)
	}
}
