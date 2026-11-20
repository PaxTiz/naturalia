package game

import (
	"math"
	"naturalia/internal/planet"
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
