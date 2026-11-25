package game

import (
	"encoding/json"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"testing"
)

// onGrid sets a block down in a grid's frame, turned by quarters, if it is
// free.
func onGrid(t *testing.T, g *Game, like ecs.ID, kind string, at vec3, quarters int) ecs.ID {
	t.Helper()
	grid, ok := g.gridOf(like)
	if !ok {
		t.Fatal("no grid")
	}
	def, _ := g.Content.Entities.Get(kind)
	ax := grid.X
	for range quarters {
		ax = ax.Cross(grid.Up)
	}
	origin := grid.World(at)
	g.stepPhysics(1.0 / 60)
	if !g.ghostFits(def, origin, ax, grid.Up, true) {
		t.Errorf("%s doesn't fit at %v", kind, at)
	}
	tr := Transform{Dir: origin.Norm()}
	tr.Alt = origin.Length() - g.World.Relief.Surface(tr.Dir).Length()
	tr.SetAxes(ax, grid.Up)
	lo, hi := boxIn(grid, def, ax, grid.Up)
	id := g.add(Placed{Kind: kind, Dir: tr.Dir, Yaw: tr.Yaw, Alt: tr.Alt, Pitch: tr.Pitch, Roll: tr.Roll,
		Grid: &OnGrid{Grid: grid, Lo: at.Add(lo), Hi: at.Add(hi)}})
	g.stepPhysics(1.0 / 60)
	return id
}

// stride walks the player from a point of an entity's frame toward
// another, sec seconds at walking speed: where they end up, in that frame.
func stride(g *Game, id ecs.ID, from, to vec3, sec float64) vec3 {
	pos, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
	world := func(v vec3) vec3 { return pos.Add(ax.Scale(v.X)).Add(up.Scale(v.Y)).Add(az.Scale(v.Z)) }
	p := &g.Player
	p.Pos, p.vel, p.vr, p.onGround = world(from), vec3{}, 0, true
	d := world(to).Sub(world(from))
	for range int(sec * 60) {
		p.want = d.Sub(p.LocalUp().Scale(d.Dot(p.LocalUp()))).Norm().Scale(walkSpeed)
		g.stepPhysics(1.0 / 60)
		p.fall(1.0/60, g.World.Relief)
	}
	l := p.Pos.Sub(pos)
	return vec3{l.Dot(ax), l.Dot(up), l.Dot(az)}
}

func TestDoorway(t *testing.T) {
	g := gridGame(t)
	slab := put(g, "core:slab", ahead(g, 6), 6)
	door := onGrid(t, g, slab, "core:wall-door", vec3{2, 0.2, 2}, 0)
	for _, z := range []float64{0, 0.4} { // through it, in the middle and along its side
		if at := stride(g, door, vec3{1.5, 0, z}, vec3{-1.5, 0, z}, 0.6); at.X > -1 || math.Abs(at.Z-z) > 0.1 {
			t.Errorf("through the door at %.1f: %v", z, at)
		}
	}
	if at := stride(g, door, vec3{1.5, 0, 1.4}, vec3{-1.5, 0, 1.4}, 0.6); at.X < 0.2 {
		t.Errorf("through the wall beside the door: %v", at)
	}
	// Along the opening, its side stops them without throwing them out.
	if at := stride(g, door, vec3{0, 0, 0}, vec3{0, 0, 2}, 0.6); at.Z > 0.6 || math.Abs(at.X) > 0.3 {
		t.Errorf("against the side: %v", at)
	}
	if at := stride(g, door, vec3{1.5, 0, 0}, vec3{-1.5, 0, 0}, 0.1); at.Y > 0.1 {
		t.Errorf("on top of it: %v", at)
	}
}

func TestStairs(t *testing.T) {
	g := gridGame(t)
	slab := put(g, "core:slab-big", ahead(g, 8), 8)
	if slab == 0 {
		t.Fatal(g.Ghost.Why)
	}
	stairs := onGrid(t, g, slab, "core:stairs", vec3{4, 0.2, 4}, 0)
	onGrid(t, g, slab, "core:slab", vec3{0, 2.7, 4}, 0) // the floor above
	top := stride(g, stairs, vec3{2.6, 0, 0}, vec3{-3, 0, 0}, 1.1)
	if math.Abs(top.Y-2.7) > 0.05 || top.X > -2 {
		t.Errorf("up the stairs: %v", top)
	}
	if at := stride(g, stairs, vec3{-2.5, 2.7, 0}, vec3{3, 2.7, 0}, 1.1); at.Y > 0.6 {
		t.Errorf("down the stairs: %v", at)
	}
	// Under the floor above, jumping stops at it, and doesn't throw them out.
	at := stride(g, stairs, vec3{-4, 0, 0}, vec3{-4.01, 0, 0}, 0.02)
	g.Player.vr, g.Player.onGround = jumpSpeed, false
	g.Player.want = vec3{}
	high := 0.0
	ground := g.Player.Pos.Length()
	was := g.Player.Pos.Norm()
	for range 120 {
		g.stepPhysics(1.0 / 60)
		g.Player.fall(1.0/60, g.World.Relief)
		high = math.Max(high, g.Player.Pos.Length()-ground)
	}
	if high > 2.5-personH+0.02 || high < 0.4 || g.Player.Pos.Norm().Sub(was).Length()*ground > 0.2 {
		t.Errorf("jumped %.2f m under the floor from %v, moved %.2f m", high, at, g.Player.Pos.Norm().Sub(was).Length()*ground)
	}
}

// A storey built by aiming alone: walls on a slab's edge, a slab on them, a
// roof on the next ones.
func TestHouseByAiming(t *testing.T) {
	g := gridGame(t)
	slab := put(g, "core:slab", ahead(g, 6), 6)
	grid, _ := g.gridOf(slab)
	pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(slab))
	edge := pos.Add(ax.Scale(1.9)).Add(up.Scale(0.2)) // its near edge
	wall := put(g, "core:wall-window", edge, 5)
	if wall == 0 {
		t.Fatalf("no wall: %q", g.Ghost.Why)
	}
	lo, hi, _ := g.boxOf(grid, wall)
	slo, shi, _ := g.boxOf(grid, slab)
	if math.Abs(lo.Y-shi.Y) > 1e-6 || outside(lo.X, slo.X, shi.X)+outside(hi.X, slo.X, shi.X)+outside(lo.Z, slo.Z, shi.Z)+outside(hi.Z, slo.Z, shi.Z) > 1e-6 ||
		math.Abs(lo.X-slo.X) > 1e-6 && math.Abs(hi.X-shi.X) > 1e-6 || math.Abs(lo.Z-slo.Z) > 1e-6 && math.Abs(hi.Z-shi.Z) > 1e-6 {
		t.Fatalf("the wall fills %v %v on %v %v", lo, hi, slo, shi)
	}
	wp, wx, wu, _ := g.Frame(g.Ents.Transform.Get(wall))
	roof := put(g, "core:roof", wp.Add(wx.Scale(0.1)).Add(wu.Scale(2.3)), 5)
	if roof == 0 {
		t.Fatalf("no roof: %q", g.Ghost.Why)
	}
	rlo, rhi, _ := g.boxOf(grid, roof)
	if math.Abs(rlo.Y-hi.Y) > 1e-6 || math.Abs(rlo.Z-slo.Z) > 1e-6 || math.Abs(rlo.X-slo.X) > 1e-6 { // over the room
		t.Errorf("the roof fills %v %v on %v %v", rlo, rhi, lo, hi)
	}
	if rp, rx, ru, _ := g.Frame(g.Ents.Transform.Get(roof)); true { // its eave on the wall
		if o, _ := g.Content.Entities.Get("core:roof"); o != nil {
			d, _ := riseOn(grid, o, rx, ru)
			if w := grid.Local(wp).Sub(grid.Local(rp)); d.Dot(w) > -1 {
				t.Errorf("rises toward %v, the wall at %v", d, w)
			}
		}
	}
	// Another goes on from its top, 2 m higher.
	rp, rx, ru, _ := g.Frame(g.Ents.Transform.Get(roof))
	next := put(g, "core:roof", rp.Add(rx.Scale(-1.2)).Add(ru.Scale(1.8)), 6)
	if next == 0 {
		t.Fatalf("no second roof: %q", g.Ghost.Why)
	}
	nlo, _, _ := g.boxOf(grid, next)
	if math.Abs(nlo.Y-rlo.Y-2) > 1e-6 {
		t.Errorf("the second roof is %.2f m over the first", nlo.Y-rlo.Y)
	}
}

// Every block of the catalog can be set down, and walked up to without
// getting stuck.
func TestCatalogStands(t *testing.T) {
	for _, s := range gridGame(t).Catalog() {
		for _, it := range s.Items {
			g := gridGame(t)
			id := put(g, it.Places, ahead(g, 9), 9)
			if id == 0 {
				t.Errorf("%s doesn't fit on the plain: %q", it.ID, g.Ghost.Why)
				continue
			}
			if def, _ := g.Content.Entities.Get(it.Places); def.Build && len(def.Parts) == 0 {
				t.Errorf("%s has no parts", it.ID)
			}
			walk(g, g.Player.Heading, 4)
			if v := g.Player.Pos.Length(); math.IsNaN(v) {
				t.Errorf("%s: lost the player", it.ID)
			}
		}
	}
}

func TestCatalogPays(t *testing.T) {
	g := gridGame(t)
	it, _ := g.Content.Items.Get("core:wall")
	g.State.Credits = it.Cost + 10
	g.Player.Pitch = -0.4
	g.Take(it)
	if !g.Ghost.OK {
		t.Fatalf("doesn't fit: %q", g.Ghost.Why)
	}
	g.SetFinish("core:rust")
	g.placeGhost()
	id := g.entityOf("core:wall")
	if g.State.Credits != 10 || g.Paint(id) != "core:rust" {
		t.Errorf("%.0f credits left, painted %q", g.State.Credits, g.Paint(id))
	}
	// Too poor for another; but one carried is free.
	g.poseGhost()
	if g.Ghost.OK {
		t.Error("placed on credit")
	}
	// Saved and loaded, it keeps its finish and its price.
	raw, _ := json.Marshal(g.placed())
	var list []Placed
	if err := json.Unmarshal(raw, &list); err != nil || len(list) != 1 || list[0].Paint != "core:rust" || list[0].Paid != it.Cost {
		t.Errorf("saved as %s", raw)
	}
	use := g.entityUse(id)
	use.Use(g, vec3{})
	if g.State.Credits != it.Cost+10 || g.State.Items["core:wall"] != 0 {
		t.Errorf("taken back: %.0f credits, %d carried", g.State.Credits, g.State.Items["core:wall"])
	}
	// What was carried comes back as it was: nothing to sell.
	g.State.Credits = 0
	g.GiveItem("core:wall", 1)
	g.stepPhysics(1.0 / 60)
	g.poseGhost()
	g.placeGhost()
	id = g.entityOf("core:wall")
	use = g.entityUse(id)
	use.Use(g, vec3{})
	if g.State.Credits != 0 || g.State.Items["core:wall"] != 1 {
		t.Errorf("taken back: %.0f credits, %d carried", g.State.Credits, g.State.Items["core:wall"])
	}
}

func TestShelves(t *testing.T) {
	g := fleetGame(t, LoadContent(), map[string]int{})
	for _, it := range g.ShopItems() {
		if def, ok := g.Content.Entities.Get(it.id); ok && def.Build {
			t.Errorf("%s is in the shop", it.id)
		}
	}
	n := 0
	for _, s := range g.Catalog() {
		n += len(s.Items)
	}
	if n < 40 {
		t.Errorf("%d things in the catalog", n)
	}
	for _, it := range g.Content.Items.All() {
		if def, ok := g.Content.Entities.Get(it.Places); ok && def.Paint != "" {
			found := false
			for _, p := range def.Parts {
				found = found || content.Qualify(p.Token) == content.Qualify(def.Paint)
			}
			if !found {
				t.Errorf("%s: no part made of %s to paint", it.ID, def.Paint)
			}
		}
	}
	// R goes through what was picked, and what is carried.
	g.GiveItem("core:flag", 1)
	a, _ := g.Content.Items.Get("core:roof")
	g.State.Credits = 1e4
	g.Take(a)
	if h, n := g.HeldItem(); h != a || n != 0 {
		t.Errorf("holds %v ×%d", h, n)
	}
	if h := g.nextItem(); h.ID != "core:flag" {
		t.Errorf("then %s", h.ID)
	}
	if h := g.nextItem(); h != a {
		t.Errorf("then %s", h.ID)
	}
}
