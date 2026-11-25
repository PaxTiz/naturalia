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

// Two buildings stand a few meters apart, level with each other, and each
// keeps its flat ground; closer, they don't fit.
func TestNeighbours(t *testing.T) {
	g := baseGame(t)
	l := g.campLayout("core:bootcamp")
	var a CampAt
	dir := func(gap float64) vec3 {
		d := a.Site.Origin.Norm()
		return planet.Toward2(d, planet.TangentAt(d, a.Site.Dir(vec3{0, 0, 1})), 2*l.Reach+gap)
	}
	for i := 0; ; i++ { // somewhere flat enough
		if i == 400 {
			t.Fatal("no flat ground anywhere")
		}
		id := g.add(Placed{Kind: "core:bootcamp", Dir: planet.Toward(vec3{math.Cos(float64(i)), 0, math.Sin(float64(i))}, 300+5*float64(i))})
		a, _ = g.campOf(id)
		if ok, _ := g.campFits(l, dir(4), g.campLevel(l, dir(4), 0), 0); ok && g.campLevel(l, dir(4), 0) != nil {
			break
		}
		g.Ents.Kill(id)
	}
	if ok, why := g.campFits(l, dir(2), g.campLevel(l, dir(2), 0), 0); ok || why != "trop près d'un bâtiment" {
		t.Errorf("2 m away: fits %v (%s)", ok, why)
	}
	level := g.campLevel(l, dir(4), 0)
	if d := math.Abs(*level - g.World.Relief.Layers(dir(4))); d < 0.001 {
		t.Errorf("the ground is level already: the test sees nothing")
	}
	if ok, why := g.campFits(l, dir(4), level, 0); !ok {
		t.Fatalf("4 m away: %s", why)
	}
	id := g.add(Placed{Kind: "core:bootcamp", Dir: dir(4), Yaw: 2})
	g.Ents.Camp.Get(id).Level = level
	g.standCamps()
	b, _ := g.campOf(id)
	if d := b.Site.Origin.Length() - a.Site.Origin.Length(); math.Abs(d) > 1e-6 {
		t.Errorf("%f m over the other", d)
	}
	for _, c := range []CampAt{a, b} {
		for a := 0.0; a < 360; a += 15 {
			p := content.Turn(vec3{c.Layout.Reach, 0, 0}, a)
			if y := c.Site.ToLocal(g.World.Relief.Surface(c.ToWorld(p))).Y; math.Abs(y) > 0.005 {
				t.Errorf("%s, at %v: the ground is %.3f m off its floor", c.Name, p, y)
			}
		}
	}
	var saved *Placed
	for _, p := range g.placed() {
		if p.Camp != nil && p.Camp.ID == b.ID {
			saved = &p
		}
	}
	if saved == nil || saved.Camp.Level == nil || *saved.Camp.Level != *level {
		t.Errorf("its level isn't saved")
	}
}

// openPlain stands the player on a plain flat enough to set a building of
// that kind down before them, its ghost in hand.
func openPlain(t *testing.T, g *Game, item string) *Ghost {
	t.Helper()
	it, ok := g.Content.Items.Get(item)
	if !ok {
		t.Fatalf("no item %s", item)
	}
	g.Mode = ModePlay
	for i := range 400 {
		g.Player.Pos = g.World.Relief.Surface(planet.Toward(vec3{math.Cos(float64(i)), 0, math.Sin(float64(i))}, 300+5*float64(i)))
		g.Player.Heading = planet.TangentAt(g.Player.Pos.Norm(), vec3{1, 0, 0.3})
		g.phys = nil
		g.stepPhysics(1.0 / 60)
		g.Ghost = nil
		g.hold(it)
		if g.Ghost.OK {
			return g.Ghost
		}
	}
	t.Fatalf("nowhere to set %s down: %s", item, g.Ghost.Why)
	return nil
}

// A building is picked in the catalog, paid for as it is set down, moved,
// and taken back for its price by aiming at it.
func TestBuildingCatalog(t *testing.T) {
	g := baseGame(t)
	for _, item := range []string{"core:carrefour", "core:habitat", "core:serre"} {
		it, _ := g.Content.Items.Get(item)
		found := false
		for _, s := range g.Catalog() {
			for _, o := range s.Items {
				found = found || o.ID == item && s.Name == "Bâtiments"
			}
		}
		if !found {
			t.Errorf("%s isn't in the catalog", item)
		}
		g.State.Credits = it.Cost - 1
		g.Ghost = nil
		g.hold(it)
		if g.Ghost.OK {
			t.Errorf("%s: set down without the credits", item)
		}
		g.State.Credits = it.Cost
		dir := openPlain(t, g, item).Pose.Dir
		g.placeGhost()
		g.Ghost = nil
		if g.State.Credits != 0 || len(g.Camps()) != 1 {
			t.Fatalf("%s: %v credits left, %d standing", item, g.State.Credits, len(g.Camps()))
		}
		c := g.Camps()[0]
		flat := g.World.Relief.Height(dir)
		// From outside, aiming at its wall.
		out := content.Turn(vec3{1, 0, 0}, 45)
		g.Player.Pos = g.World.Relief.Surface(c.ToWorld(out.Scale(c.Layout.Reach)))
		g.Player.Heading, g.Player.Pitch = c.Site.Dir(out.Scale(-1)), 0
		use, ok := g.NearbyUse()
		if !ok || use.Prompt != "[E] Reprendre : "+it.Name {
			t.Fatalf("%s: aimed at, E says %q", item, use.Prompt)
		}
		use.Use(g, use.At)
		if g.State.Credits != it.Cost || len(g.Camps()) != 0 {
			t.Errorf("%s: taken back, %v credits, %d standing", item, g.State.Credits, len(g.Camps()))
		}
		if h := g.World.Relief.Height(dir); math.Abs(h-g.World.Relief.Layers(dir)) > 1e-9 || len(g.World.Relief.Flat().Pads) != 0 {
			t.Errorf("%s: the ground stays flat (%v, %v)", item, flat, h)
		}
		// From inside, its own things are used, not taken back.
	}
}
