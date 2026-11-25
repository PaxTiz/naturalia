package game

import (
	"encoding/json"
	"math"
	"naturalia/internal/ecs"
	"naturalia/internal/planet"
	"testing"
)

// gridGame is a game far from the base, the grid on.
func gridGame(t *testing.T) *Game {
	g := fleetGame(t, LoadContent(), map[string]int{})
	g.Settings.Grid, g.Settings.GridStep = true, 1
	g.stepPhysics(1.0 / 60)
	return g
}

// put aims at a point and sets a block down there: its ID, 0 if it
// doesn't fit. The player stands back m before it.
func put(g *Game, kind string, at vec3, back float64) ecs.ID {
	n := at.Norm()
	g.Player.Pos = g.World.Relief.Surface(planet.Toward2(n, g.Player.Heading, -back))
	d := at.Sub(g.Player.Eye()).Norm()
	up := g.Player.LocalUp()
	g.Player.Heading = planet.TangentAt(up, d)
	g.Player.Pitch = math.Asin(d.Dot(up))
	g.relay = true
	g.stepPhysics(1.0 / 60)
	g.GiveItem(kind, 1)
	if g.Ghost == nil {
		g.Ghost = &Ghost{rel: [2]vec3{{1, 0, 0}, {0, 1, 0}}}
	}
	g.Ghost.Kind, g.Ghost.item = kind, kind
	g.poseGhost()
	if !g.Ghost.OK {
		return 0
	}
	before := map[ecs.ID]bool{}
	for id := range g.Ents.All() {
		before[id] = true
	}
	g.placeGhost()
	g.stepPhysics(1.0 / 60)
	for id := range g.Ents.All() {
		if !before[id] {
			return id
		}
	}
	return 0
}

func TestGridSlabs(t *testing.T) {
	g := gridGame(t)
	first := put(g, "core:slab", ahead(g, 6), 6)
	if first == 0 {
		t.Fatalf("the first slab doesn't fit: %q", g.Ghost.Why)
	}
	grid, ok := g.gridOf(first)
	if !ok {
		t.Fatal("no grid")
	}
	pos, ax, _, az := g.Frame(g.Ents.Transform.Get(first))
	// Turned askew, aimed 60 cm off: the next one is still flush, level and
	// turned like it.
	g.Player.Heading = rotate(g.Player.Heading, g.Player.LocalUp(), 0.4)
	second := put(g, "core:slab", pos.Add(az.Scale(4.4)).Add(ax.Scale(0.4)), 5)
	if second == 0 {
		t.Fatalf("the second slab doesn't fit: %q", g.Ghost.Why)
	}
	p2, ax2, up2, _ := g.Frame(g.Ents.Transform.Get(second))
	if d := grid.DirLocal(p2.Sub(pos)); math.Abs(math.Abs(d.X)+math.Abs(d.Z)-4) > 1e-6 || math.Abs(d.Y) > 1e-6 {
		t.Errorf("the second slab is %v off the first", d)
	}
	if !square(grid, ax2, up2) {
		t.Error("the second slab is askew")
	}
	// The ground is level with the slabs' bottoms, under all of them.
	for _, id := range []ecs.ID{first, second} {
		p, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
		for _, off := range []vec3{{}, ax.Scale(1.9), ax.Scale(-1.9), az.Scale(1.9), az.Scale(-1.9)} {
			q := p.Add(off)
			if h := g.World.Relief.Surface(q).Sub(p).Dot(up); math.Abs(h) > 1e-6 {
				t.Errorf("the ground is %.4f m over a slab's bottom", h)
			}
		}
	}
}

func TestGridWalls(t *testing.T) {
	g := gridGame(t)
	slab := put(g, "core:slab", ahead(g, 6), 6)
	pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(slab))
	grid, _ := g.gridOf(slab)
	top := up.Scale(0.2)
	// A wall on the far edge: its outer face flush with the slab's.
	far := put(g, "core:wall", pos.Add(top).Sub(ax.Scale(1.8)), 6)
	if far == 0 {
		t.Fatalf("the wall doesn't fit: %q", g.Ghost.Why)
	}
	lo, hi, _ := g.boxOf(grid, far)
	slo, shi, _ := g.boxOf(grid, slab)
	flush := func(a, b float64) bool { return math.Abs(a-b) < 1e-6 }
	if !flush(lo.Y, shi.Y) {
		t.Errorf("the wall stands at %.4f on a slab's top at %.4f", lo.Y, shi.Y)
	}
	edges := 0
	for _, e := range [][2]float64{{lo.X, slo.X}, {hi.X, shi.X}, {lo.Z, slo.Z}, {hi.Z, shi.Z}} {
		if flush(e[0], e[1]) {
			edges++
		}
	}
	if edges != 3 {
		t.Errorf("the wall is flush with %d of the slab's edges, not 3: %v %v in %v %v", edges, lo, hi, slo, shi)
	}
	// Turned a quarter on a side: slid out of the first one's thickness.
	_, _, _, az := g.Frame(g.Ents.Transform.Get(slab))
	g.Ghost.turn(vec3{0, 1, 0}, math.Pi/2)
	side := put(g, "core:wall", pos.Add(top).Add(az.Scale(1.8)).Add(ax.Scale(0.3)), 6)
	if side == 0 {
		t.Fatalf("the second wall doesn't fit: %q", g.Ghost.Why)
	}
	lo2, hi2, _ := g.boxOf(grid, side)
	if lo2.X < hi.X-1e-6 && lo.X < hi2.X-1e-6 && lo2.Z < hi.Z-1e-6 && lo.Z < hi2.Z-1e-6 {
		t.Errorf("the walls overlap: %v %v and %v %v", lo, hi, lo2, hi2)
	}
}

// What is built stays put as the ground is graded, as it is taken off, and
// from a save to the next.
func TestGridGround(t *testing.T) {
	g := gridGame(t)
	a := put(g, "core:crate-2", ahead(g, 6), 6)
	pa, _, _, _ := g.Frame(g.Ents.Transform.Get(a))
	b := put(g, "core:slab", pa.Add(g.Player.Right().Scale(4)), 6)
	if a == 0 || b == 0 {
		t.Fatal("don't fit", a, b)
	}
	stay := func(when string, id ecs.ID, want vec3) {
		t.Helper()
		if p, _, _, _ := g.Frame(g.Ents.Transform.Get(id)); p.Sub(want).Length() > 1e-6 {
			t.Errorf("%s, moved %.4f m", when, p.Sub(want).Length())
		}
	}
	stay("a slab set beside", a, pa)
	pb, _, _, _ := g.Frame(g.Ents.Transform.Get(b))
	data, err := json.Marshal(g.saved())
	if err != nil {
		t.Fatal(err)
	}
	g.Ents.Kill(b)
	g.syncGround()
	stay("the slab taken off", a, pa)
	if h := g.World.Relief.Height(pb.Norm()) - g.World.Relief.Natural(pb.Norm()); math.Abs(h) > 0.8 {
		t.Errorf("the ground under what was taken off: %f m off", h)
	}

	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	g2 := gridGame(t)
	g2.loading = true
	for _, p := range st.Entities {
		g2.add(p)
	}
	g2.syncGround()
	g2.loading = false
	var got []vec3
	for id := range g2.Ents.All() {
		p, _, _, _ := g2.Frame(g2.Ents.Transform.Get(id))
		got = append(got, p)
	}
	for i, want := range []vec3{pa, pb} {
		if i >= len(got) || got[i].Sub(want).Length() > 1e-6 {
			t.Errorf("loaded: %v, saved %v", got, want)
		}
	}
}

// Free, a block still lines up, off the ground.
func TestGridFree(t *testing.T) {
	g := gridGame(t)
	a := put(g, "core:slab", ahead(g, 6), 6)
	grid, _ := g.gridOf(a)
	g.Ghost.Free, g.Ghost.dist = true, 6
	g.Player.Pitch = 0.25
	g.poseGhost()
	gh := g.Ghost
	if !gh.OK || gh.grid == nil || !gh.grid.Same(grid) || gh.grid.Ground {
		t.Fatalf("free: %+v", gh.grid)
	}
	for _, v := range []float64{gh.grid.Lo.X, gh.grid.Lo.Z} {
		if math.Abs(v-math.Round(v)) > 1e-6 {
			t.Errorf("off the lines: %v", gh.grid.Lo)
		}
	}
}

// With the grid off, what is set down keeps none.
func TestGridOff(t *testing.T) {
	g := gridGame(t)
	g.Settings.Grid = false
	if id := put(g, "core:crate-2", ahead(g, 6), 6); id == 0 || g.Ents.Grid.Has(id) {
		t.Errorf("block %v on a grid", id)
	}
}

// Moved on its grid, a block keeps its turn.
func TestGridMove(t *testing.T) {
	g := gridGame(t)
	a := put(g, "core:slab", ahead(g, 6), 6)
	put(g, "core:slab", ahead(g, 12), 6) // keeps the grid while the first is in hand
	pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(a))
	g.Ghost = nil
	g.grab(a)
	g.poseGhost()
	if got := g.Ghost.inFrame(g.Ghost.rel[0]); math.Abs(got.Dot(ax)-1) > 1e-6 {
		t.Errorf("moved, it turned: %v from %v", got, ax)
	}
	if math.Abs(g.Ghost.inFrame(g.Ghost.rel[1]).Dot(up)-1) > 1e-3 {
		t.Errorf("moved, it leans, at %v", pos)
	}
}

// Aimed on another's edge, a block slides edge to edge with it.
func TestGridSlides(t *testing.T) {
	g := gridGame(t)
	a := put(g, "core:slab", ahead(g, 7), 7)
	grid, _ := g.gridOf(a)
	pos, ax, _, _ := g.Frame(g.Ents.Transform.Get(a))
	b := put(g, "core:slab", pos.Add(ax.Scale(2.7)), 5)
	if b == 0 {
		t.Fatalf("doesn't fit: %q", g.Ghost.Why)
	}
	p, _, _, _ := g.Frame(g.Ents.Transform.Get(b))
	if d := grid.DirLocal(p.Sub(pos)); math.Abs(math.Abs(d.X)-4) > 1e-6 && math.Abs(math.Abs(d.Z)-4) > 1e-6 {
		t.Errorf("%v apart", d)
	}
}

// What was set down askew, taken in hand, is straightened on the grid; far
// from it too, where it is kept on its lines.
func TestGridAlign(t *testing.T) {
	g := gridGame(t)
	a := put(g, "core:slab", ahead(g, 6), 6)
	grid, _ := g.gridOf(a)
	put(g, "core:slab", ahead(g, 12), 6) // keeps the grid
	g.Ghost.turn(vec3{0, 1, 0}, 0.3)
	g.Ghost.turn(vec3{1, 0, 0}, 0.2)
	ax, up := g.Ghost.inFrame(g.Ghost.rel[0]), g.Ghost.inFrame(g.Ghost.rel[1])
	g.Ents.Transform.Get(a).SetAxes(ax, up)
	g.Ghost = nil
	g.grab(a)
	if g.Ghost.Aligned() {
		t.Fatal("askew, it says it is straight")
	}
	aligned := func(when string) {
		t.Helper()
		ax, up, _ := g.Ghost.Pose.Axes()
		if gh := g.Ghost; !gh.Aligned() || gh.grid == nil || !gh.grid.Same(grid) || !square(grid, ax, up) ||
			math.Abs(gh.grid.Lo.X-math.Round(gh.grid.Lo.X)) > 1e-6 {
			t.Errorf("%s: %+v, rel %v", when, gh.grid, gh.rel)
		}
	}
	g.alignGhost()
	aligned("beside the grid")
	// 40 m away, beyond any thing's grid, the key keeps it on its own.
	g.Player.Pos = g.World.Relief.Surface(planet.Toward2(g.Player.LocalUp(), g.Player.Heading, -40))
	g.Ghost.home = nil
	g.poseGhost()
	if g.Ghost.Aligned() {
		t.Error("far away, it stayed on the grid by itself")
	}
	g.alignGhost()
	aligned("far away")
	// With the grid off, the key brings it back.
	g.Settings.Grid = false
	g.alignGhost()
	if !g.Settings.Grid {
		t.Error("the grid is still off")
	}
}

// A small rock gives way to a block, on the grid or not, and comes back
// once it is taken off; a boulder stays.
func TestBuildOnRocks(t *testing.T) {
	for _, on := range []bool{true, false} {
		c := LoadContent()
		g := fleetGame(t, c, map[string]int{})
		set := c.Terrain
		set.Gems = 0
		g.World = planet.NewWorld(7, &set)
		g.Settings.Grid, g.Settings.GridStep = on, 1
		small, boulder := planet.Rock{}, planet.Rock{}
		for _, r := range g.World.RocksNear(g.Player.Pos, 400) {
			if planet.ArcDist(r.Dir, planet.BaseUp) < 130 {
				continue
			}
			alone := true
			for _, o := range g.World.RocksNear(r.Dir, 7) {
				alone = alone && (o.Dir == r.Dir || o.Size < 0.5)
			}
			if alone && r.Size > minRock && r.Size < planet.SmallRock {
				small = r
			} else if r.Size > 1.4 {
				boulder = r
			}
		}
		there := func(r planet.Rock) bool {
			for _, o := range g.World.RocksNear(r.Dir, 1) {
				if o.Dir == r.Dir {
					return true
				}
			}
			return false
		}
		if small.Size == 0 || boulder.Size == 0 {
			t.Fatal("no rock to build on")
		}
		g.Player.Pos = g.World.Relief.Surface(small.Dir)
		id := put(g, "core:crate-2", g.World.Relief.Surface(small.Dir), 6)
		if id == 0 || there(small) {
			t.Errorf("grid %v: block %v (%q), the small rock still there: %v", on, id, g.Ghost.Why, there(small))
			continue
		}
		g.Ents.Kill(id)
		g.syncGround()
		if !there(small) {
			t.Errorf("grid %v: the block taken off, no rock", on)
		}
		g.Ghost = nil
		g.Player.Pos = g.World.Relief.Surface(boulder.Dir)
		if id := put(g, "core:crate-2", g.World.Relief.Surface(boulder.Dir), 6); id != 0 && !there(boulder) {
			t.Errorf("grid %v: a boulder gave way", on)
		}
	}
}

// Aimed high on its side, over the player's head, a block goes on top,
// edge to edge; aimed again low on the tower's foot, higher still.
func TestGridStack(t *testing.T) {
	for _, kind := range []string{"core:crate-2", "core:wall"} {
		g := gridGame(t)
		first := put(g, kind, ahead(g, 6), 6)
		grid, _ := g.gridOf(first)
		lo, hi, _ := g.boxOf(grid, first)
		pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(first))
		tall := hi.Y - lo.Y
		high := pos.Add(up.Scale(tall * 0.8)).Add(ax.Scale(hi.X - lo.X).Scale(0.4))
		for i := 1; i <= 2; i++ {
			id := put(g, kind, high, 5)
			if id == 0 {
				t.Fatalf("%s %d doesn't fit: %q", kind, i, g.Ghost.Why)
			}
			l, h, _ := g.boxOf(grid, id)
			want := vec3{lo.X, lo.Y + tall*float64(i), lo.Z}
			if l.Sub(want).Length() > 1e-6 || h.Y-l.Y-tall > 1e-6 {
				t.Errorf("%s %d at %v, want %v", kind, i, l, want)
			}
		}
		// Low on its side, the next one is beside it, level with it.
		g.Ghost.lift = 0
		id := put(g, kind, pos.Add(up.Scale(tall*0.3)).Add(ax.Scale((hi.X-lo.X)*0.5)), 5)
		if id == 0 {
			t.Fatalf("%s beside doesn't fit: %q", kind, g.Ghost.Why)
		}
		if l, _, _ := g.boxOf(grid, id); math.Abs(l.Y-lo.Y) > 1e-6 {
			t.Errorf("%s beside it is %.3f m off its level", kind, l.Y-lo.Y)
		}
	}
}

// A roof aimed at a wall's head goes over the room it closes.
func TestGridRoof(t *testing.T) {
	g := gridGame(t)
	slab := put(g, "core:slab", ahead(g, 8), 8)
	grid, _ := g.gridOf(slab)
	pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(slab))
	wall := put(g, "core:wall", pos.Add(up.Scale(0.2)).Sub(ax.Scale(1.8)), 6)
	if wall == 0 {
		t.Fatal("no wall")
	}
	wpos, _, _, _ := g.Frame(g.Ents.Transform.Get(wall))
	roof := put(g, "core:slab", wpos.Add(up.Scale(2.1)).Add(ax.Scale(0.1)), 5)
	if roof == 0 {
		t.Fatalf("no roof: %q", g.Ghost.Why)
	}
	lo, _, _ := g.boxOf(grid, roof)
	slo, _, _ := g.boxOf(grid, slab)
	if want := slo.Add(vec3{0, 2.7, 0}); lo.Sub(want).Length() > 1e-6 {
		t.Errorf("the roof is at %v, want %v", lo, want)
	}
}

// Crates rest on blocks and on each other, and fall once what bears them
// is gone.
func TestCratesStack(t *testing.T) {
	for _, on := range []bool{true, false} {
		g := gridGame(t)
		g.Settings.Grid = on
		settle := func() {
			for range 180 {
				g.stepPhysics(1.0 / 60)
			}
		}
		slab := put(g, "core:slab", ahead(g, 6), 6)
		pos, _, up, _ := g.Frame(g.Ents.Transform.Get(slab))
		first := put(g, "core:crate", pos.Add(up.Scale(0.2)), 4)
		settle()
		p1, _, _, _ := g.Frame(g.Ents.Transform.Get(first))
		second := put(g, "core:crate", p1.Add(up.Scale(0.8)), 3)
		if first == 0 || second == 0 {
			t.Fatalf("grid %v: don't fit: %v %v %q", on, first, second, g.Ghost.Why)
		}
		settle()
		p1b, _, _, _ := g.Frame(g.Ents.Transform.Get(first))
		p2, _, _, _ := g.Frame(g.Ents.Transform.Get(second))
		if d := p1.Sub(pos).Dot(up); math.Abs(d-0.2) > 0.03 {
			t.Errorf("grid %v: the first crate is %.2f m over the slab's foot", on, d)
		}
		if d := p2.Sub(p1b); math.Abs(d.Dot(up)-0.8) > 0.03 || d.Sub(up.Scale(d.Dot(up))).Length() > 0.3 {
			t.Errorf("grid %v: the second crate is at %v from the first", on, d)
		}
		if p1b.Sub(p1).Length() > 0.05 {
			t.Errorf("grid %v: the first crate moved %.2f m", on, p1b.Sub(p1).Length())
		}
		g.Ents.Kill(first)
		settle()
		if p, _, _, _ := g.Frame(g.Ents.Transform.Get(second)); math.Abs(p.Sub(pos).Dot(up)-0.2) > 0.03 || g.Ents.Air.Has(second) {
			t.Errorf("grid %v: the second crate is %.2f m over the slab's foot", on, p.Sub(pos).Dot(up))
		}
	}
}

// Aimed just over a wall's head, a block goes on it, not behind it.
func TestGridOverHead(t *testing.T) {
	g := gridGame(t)
	wall := put(g, "core:wall", ahead(g, 5), 5)
	grid, _ := g.gridOf(wall)
	lo, hi, _ := g.boxOf(grid, wall)
	pos, _, up, _ := g.Frame(g.Ents.Transform.Get(wall))
	id := put(g, "core:wall", pos.Add(up.Scale(2.7)), 4)
	if id == 0 {
		t.Fatalf("doesn't fit: %q", g.Ghost.Why)
	}
	l, _, _ := g.boxOf(grid, id)
	if want := (vec3{lo.X, hi.Y, lo.Z}); l.Sub(want).Length() > 1e-6 {
		t.Errorf("at %v, want %v", l, want)
	}
}
