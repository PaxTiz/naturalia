package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/planet"
	"strings"
	"testing"
)

// campGame is baseGame with a bootcamp placed 400 m east of the base,
// turned some, and the player standing before its door.
func campGame(t *testing.T) (*Game, CampAt) {
	g := baseGame(t)
	g.add(Placed{Kind: "core:bootcamp", Dir: planet.Toward(vec3{1, 0, 0.3}, 400), Yaw: 0.7})
	camps := g.Camps()
	if len(camps) != 1 {
		t.Fatalf("%d bootcamps standing", len(camps))
	}
	c := camps[0]
	door, _ := c.Layout.Door()
	g.Player.Pos = g.World.Relief.Surface(c.ToWorld(door.Add(vec3{3, 0, 0})))
	g.phys = nil
	return g, c
}

// The bootcamp's dome holds its furniture, its bed, workbench and table
// are used with E, and its inside is its dome and its airlock.
func TestCampLayout(t *testing.T) {
	g, c := campGame(t)
	l := c.Layout
	if len(l.Walls) < 10 || len(l.Lamps) == 0 || len(l.Parts.Keys) == 0 {
		t.Fatalf("%d walls, %d lamps, %d meshes", len(l.Walls), len(l.Lamps), len(l.Parts.Keys))
	}
	for _, f := range l.Furniture {
		if d := math.Hypot(f.At.X, f.At.Z); d > l.Room.Radius-0.3 {
			t.Errorf("%s stands %.2f m from the middle, out of the dome", f.Kit, d)
		}
	}
	uses := map[string]bool{}
	for _, u := range l.Uses {
		uses[u.Kit] = true
	}
	for _, k := range []string{"core:bed", "core:maptable", "core:camp.bench"} {
		if !uses[k] {
			t.Errorf("no %s to use", k)
		}
	}
	door, _ := l.Door()
	for _, p := range []struct {
		at     vec3
		inside bool
	}{{vec3{0, 1, 0}, true}, {door.Add(vec3{-0.5, 1, 0}), true}, {door.Add(vec3{1, 1, 0}), false}, {vec3{0, 1, 4.5}, false}} {
		if got := g.Indoors(c.ToWorld(p.at)); got != p.inside {
			t.Errorf("at %v: indoors %v, want %v", p.at, got, p.inside)
		}
	}
	// The ground lies flat under it, up to the airlock's door.
	for _, p := range []vec3{{0, 0, 0}, {-3.9, 0, 0}, {0, 0, 3.9}, door} {
		if y := c.Site.ToLocal(g.World.Relief.Surface(c.ToWorld(p))).Y; math.Abs(y) > 0.005 {
			t.Errorf("at %v, the ground is %.3f m off its floor", p, y)
		}
	}
	if b := g.buildingAt(c.ToWorld(vec3{0, 1, 0})); b != "core:bootcamp" {
		t.Errorf("in the dome: building %q", b)
	}
}

// The player walks in through the door, and is stopped by the walls
// elsewhere.
func TestWalkIntoCamp(t *testing.T) {
	g, c := campGame(t)
	door, _ := c.Layout.Door()
	walk(g, c.Site.Dir(vec3{-1, 0, 0}), 4)
	if at := c.Site.ToLocal(g.Player.Pos); !g.Indoors(g.Player.Eye()) || at.X > door.X-1 {
		t.Errorf("walking in through the door, ended at %v", at)
	}
	// From behind, toward the middle: the wall stops them.
	for _, a := range []float64{math.Pi, math.Pi * 0.75, math.Pi * 1.3} {
		out := vec3{math.Cos(a), 0, math.Sin(a)}
		g.Player.Pos = g.World.Relief.Surface(c.ToWorld(out.Scale(c.Layout.Room.Radius + 1.5)))
		g.Player.vel = vec3{}
		walk(g, c.Site.Dir(out.Scale(-1)), 2)
		if at := c.Site.ToLocal(g.Player.Pos); math.Hypot(at.X, at.Z) < c.Layout.Room.Radius {
			t.Errorf("walking in from %.0f°, went through the wall to %v", a*180/math.Pi, at)
		}
	}
	// Inside, the bed is used with E.
	bed := c.Layout.Wake
	g.Player.Pos = g.World.Relief.Surface(c.ToWorld(bed))
	g.Player.Heading = c.Site.Dir(vec3{-1, 0, 0})
	if it, ok := g.NearbyUse(); !ok || it.Kit != "core:bed" {
		t.Errorf("by the bed, E uses %q", it.Kit)
	}
}

// A bootcamp is bought at the lab, set down by its ghost away from the
// base and the others, and no more than 32 of them.
func TestCampShop(t *testing.T) {
	g := baseGame(t)
	g.State.Credits = 1e12
	buy := func() (ShopItem, bool) {
		for _, it := range g.ShopItems() {
			if it.id == "core:bootcamp" {
				return it, true
			}
		}
		return ShopItem{}, false
	}
	it, ok := buy()
	if !ok || it.Tab != "build" || it.Cost != 100_000_000 {
		t.Fatalf("in the shop: %+v", it)
	}
	g.Buy(it)
	ghost := func() *Ghost {
		g.Ghost = &Ghost{Kind: "core:bootcamp", item: "core:bootcamp", rel: [2]vec3{{1, 0, 0}, {0, 1, 0}}}
		g.poseGhost()
		return g.Ghost
	}
	// By the base: no.
	g.Player.Pos = g.World.Relief.Surface(planet.Toward(vec3{0, 0, -1}, 40))
	g.Player.Heading = planet.TangentAt(g.Player.Pos.Norm(), vec3{0, 0, -1})
	g.phys = nil
	g.stepPhysics(1.0 / 60)
	if gh := ghost(); gh.OK || gh.Why != "trop près de la base" {
		t.Errorf("by the base: fits %v (%s)", gh.OK, gh.Why)
	}
	// Out on a flat plain: yes, its door toward the player.
	for _, d := range []float64{500, 700, 900, 1100, 1300} {
		g.Player.Pos = g.World.Relief.Surface(planet.Toward(vec3{0.2, 0, 1}, d))
		g.Player.Heading = planet.TangentAt(g.Player.Pos.Norm(), vec3{0, 0, 1})
		if gh := ghost(); gh.OK {
			break
		}
	}
	if !g.Ghost.OK {
		t.Fatalf("nowhere to set it down: %s", g.Ghost.Why)
	}
	g.placeGhost()
	camps := g.Camps()
	if len(camps) != 1 || g.State.Items["core:bootcamp"] != 0 {
		t.Fatalf("%d standing, %d carried", len(camps), g.State.Items["core:bootcamp"])
	}
	c := camps[0]
	if to := c.Site.DirLocal(g.Player.Pos.Sub(c.Site.Origin)); to.X < 0 || math.Abs(to.Z) > to.X {
		t.Errorf("its door doesn't face the player: they are toward %v", to)
	}
	if gh := ghost(); gh.OK {
		t.Errorf("a second one right next to it fits")
	}
	g.Ghost = nil
	// 31 more: the shop has none left.
	for i := range 31 {
		g.add(Placed{Kind: "core:bootcamp", Dir: planet.Toward(vec3{math.Cos(float64(i)), 0, math.Sin(float64(i))}, 2000)})
	}
	if it, _ := buy(); !it.Maxed || it.Info != "32 bootcamps posés" {
		t.Errorf("with 32 standing: %+v", it)
	}
}

// From the base's hologram the player travels to a bootcamp, for a price,
// and comes out before its hologram, facing it, inside; and back.
func TestTravel(t *testing.T) {
	g, c := campGame(t)
	g.State.Credits = 2 * TravelCost
	dests := g.Destinations()
	if len(dests) != 2 || dests[0].Name != "Base" || dests[1].Camp != c.Ent {
		t.Fatalf("destinations: %+v", dests)
	}
	g.Travel(dests[1])
	for g.Mode == ModeTravel {
		g.updateTravel(1.0 / 60)
	}
	if g.State.Credits != TravelCost {
		t.Errorf("%v credits left", g.State.Credits)
	}
	at, ok := g.CampAround(g.Player.Eye())
	if !ok || at.Ent != c.Ent {
		t.Fatalf("not in the bootcamp: %v", c.Site.ToLocal(g.Player.Pos))
	}
	table := c.ToWorld(c.Layout.Table).Sub(g.Player.Pos)
	if table.Norm().Dot(g.Player.Heading) < 0.95 {
		t.Errorf("not facing the hologram")
	}
	for range 30 { // standing still: nothing pushes them out
		g.stepPhysics(1.0 / 60)
	}
	if !g.Indoors(g.Player.Eye()) {
		t.Errorf("pushed out: %v", c.Site.ToLocal(g.Player.Pos))
	}
	if d := g.Destinations(); !d[1].Here || g.CanTravel(d[1]) {
		t.Errorf("can travel where they are")
	}
	g.Travel(g.Destinations()[0])
	for g.Mode == ModeTravel {
		g.updateTravel(1.0 / 60)
	}
	if l := base.ToLocal(g.Player.Pos); !base.Indoors(l) {
		t.Errorf("not back in the base: %v", l)
	}
	if g.Travel(g.Destinations()[1]); g.Mode == ModeTravel {
		t.Errorf("traveled without the credits")
	}
}

// A bootcamp is renamed, not to nothing nor to a name taken; a mod sends
// the player there by that name, for free, and hears of it.
func TestRenameAndTeleport(t *testing.T) {
	g, c := campGame(t)
	if g.RenameCamp(c.Ent, "  ") || g.RenameCamp(c.Ent, "base") {
		t.Errorf("renamed to nothing, or to the base")
	}
	if !g.RenameCamp(c.Ent, "  Olympus, au pied du volcan géant  ") {
		t.Fatalf("not renamed")
	}
	name := g.Bootcamps()[0].Name
	if name != "Olympus, au pied du volc" {
		t.Errorf("named %q", name)
	}
	var heard []string
	g.Content.On(content.EvTeleport, func(_ content.Game, e *content.Event) { heard = append(heard, e.By+" > "+e.ID) })
	credits := g.State.Credits
	if g.Teleport(name) {
		t.Errorf("teleported from the pause screen")
	}
	g.Mode = ModePlay
	if g.Teleport("nowhere") || !g.Teleport(strings.ToUpper(name)) {
		t.Fatalf("teleport by name")
	}
	for g.Mode == ModeTravel {
		g.updateTravel(1.0 / 60)
	}
	if at, ok := g.CampAround(g.Player.Eye()); !ok || at.Ent != c.Ent || g.State.Credits != credits {
		t.Errorf("not there for free")
	}
	if len(heard) != 1 || heard[0] != "Base > "+name {
		t.Errorf("heard %v", heard)
	}
}

// Setting a bootcamp down while something slides nearby lays the
// collisions again, carrying its speed over.
func TestCampSetWhileMoving(t *testing.T) {
	g, c := campGame(t)
	g.Mode = ModePlay
	g.stepPhysics(1.0 / 60)
	crate := g.add(Placed{Kind: "core:crate", Dir: planet.Toward2(g.Player.Pos.Norm(), planet.TangentAt(g.Player.Pos.Norm(), g.Player.Heading), 3)})
	g.stepPhysics(1.0 / 60)
	if !g.Impulse(uint32(crate), g.Player.Heading.Scale(4)) {
		t.Fatalf("the crate won't move")
	}
	g.stepPhysics(1.0 / 60)
	g.Ents.Transform.Get(c.Ent).Yaw += 0.3 // moved
	g.standCamps()
	for range 10 {
		g.stepPhysics(1.0 / 60)
	}
}
