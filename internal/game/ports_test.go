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
	for _, item := range []string{"core:carrefour", "core:habitat", "core:serre", "core:garage", "core:observatoire"} {
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

// A garage's big door opens to whoever comes; a vehicle drives in and is
// parked in a bay, another is brought to the other by the terminal, a third
// takes a place, and the one there goes back to the base.
func TestCampGarage(t *testing.T) {
	g := baseGame(t)
	g.Mode = ModePlay
	c := standing(t, g, "core:garage")
	if len(c.Layout.Bays) != 2 {
		t.Fatalf("%d bays, want 2", len(c.Layout.Bays))
	}
	mid, _, ok := c.Layout.BigDoor()
	if !ok {
		t.Fatal("no big door")
	}
	// Closed, it stops whoever walks into it.
	g.Player.Pos = g.World.Relief.Surface(c.ToWorld(mid.Add(vec3{3, 0, 0})))
	g.phys = nil
	walk(g, c.Site.Dir(vec3{-1, 0, 0}), 3)
	if g.Indoors(g.Player.Eye()) {
		t.Error("in through the big door, closed")
	}
	for range 180 {
		g.updateCampHalls(1.0 / 60)
	}
	if o := g.BigDoor(c.Ent); o != 1 {
		t.Fatalf("the big door is %v open", o)
	}
	walk(g, c.Site.Dir(vec3{-1, 0, 0}), 2)
	if !g.Indoors(g.Player.Eye()) {
		t.Error("not in through the big door, open")
	}

	own := func(id string) (*content.Vehicle, int) {
		v, _ := g.Content.Vehicles.Get(id)
		g.State.Levels[id] = 1
		g.syncFleet()
		return v, g.Entities(id)
	}
	rover, n := own("core:rover")
	if n != 1 {
		t.Fatalf("%d rovers", n)
	}
	id := g.entityOf(rover.ID)
	g.getIn(id)
	g.Player.Pos = g.World.Relief.Surface(c.ToWorld(vec3{1, 0, 0}))
	g.drive.fwd = c.Site.Dir(vec3{-1, 0, 0})
	if !g.CanDock() {
		t.Fatal("no bay to park in")
	}
	g.startDock()
	for range 600 {
		if g.drive.dock == nil {
			break
		}
		g.glideDock(rover, 1.0/60)
	}
	g.getOut()
	in := 0
	for _, b := range c.Layout.Bays {
		if g.inBay(c, b) == id {
			in++
		}
	}
	if in != 1 || g.Riding != 0 {
		t.Fatalf("the rover is in %d bays", in)
	}
	if !g.Indoors(g.Player.Eye()) {
		t.Error("stepped out of the garage")
	}

	item := func(id string) ShopItem {
		g.Garage, g.garageIn = true, c.Ent
		for _, it := range g.garageItems() {
			if it.id == id {
				return it
			}
		}
		t.Fatalf("no %s at the terminal", id)
		return ShopItem{}
	}
	if it := item("core:rover"); it.Info != "Ici" || it.Call || it.Refund == 0 {
		t.Errorf("the rover at the terminal: %+v", it)
	}
	own("core:buggy")
	own("core:hover")
	g.Call(item("core:buggy"))
	g.Call(item("core:hover"))
	here := 0
	for _, v := range []string{"core:rover", "core:buggy", "core:hover"} {
		if item(v).Info == "Ici" {
			here++
		}
	}
	if here != 2 || item("core:hover").Info != "Ici" {
		t.Errorf("%d vehicles in the garage, hover %q", here, item("core:hover").Info)
	}
	own("core:harvester")
	if it := item("core:titan"); it.Call {
		t.Error("the titan goes in the garage")
	}
}

// The observatory's telescope sees from out of the dome, the way its own
// building faces.
func TestObservatory(t *testing.T) {
	g := baseGame(t)
	g.Mode = ModePlay
	c := standing(t, g, "core:observatoire")
	g.Player.Pos = g.World.Relief.Surface(c.ToWorld(vec3{1.4, 0, 0}))
	g.Player.Heading, g.Player.Pitch = c.Site.Dir(vec3{-1, 0, 0}), 0
	use, ok := g.NearbyUse()
	if !ok || use.Kit != "core:telescope.big" {
		t.Fatalf("by the telescope, E is %+v", use)
	}
	use.Use(g, use.At)
	if g.Mode != ModeScope {
		t.Fatal("not looking through it")
	}
	f := g.Scope.Forward()
	if up := g.Scope.Eye.Norm(); math.Abs(f.Dot(up)+math.Sin(3*math.Pi/180)) > 0.01 || f.Dot(c.Site.Dir(vec3{-1, 0, 0})) < 0.99 {
		t.Errorf("it looks %v", f)
	}
	if g.Indoors(g.Scope.Eye) {
		t.Error("it sees from inside")
	}
}
