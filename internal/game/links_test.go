package game

import (
	"encoding/json"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/planet"
	"testing"
)

// pair is a game with a building standing on flat ground, and another at a
// point of its frame, turned by yaw, its floor raised by rise (NaN: on the ground there).
func pair(t *testing.T, a, b string, at vec3, yaw, rise float64) (*Game, CampAt, CampAt) {
	t.Helper()
	g := baseGame(t)
	g.Mode = ModePlay
	g.State.Credits = 1e9
	la := g.campLayout(a)
	for i := range 600 {
		dir := planet.Toward(vec3{math.Cos(float64(i)), 0, math.Sin(float64(i))}, 300+5*float64(i))
		if ok, _ := g.campFits(la, dir, nil, 0); !ok || g.World.Relief.PadSlope(dir, la.Reach+30, g.World.Relief.Layers(dir)) > 1 {
			continue
		}
		ida := g.add(Placed{Kind: a, Dir: dir})
		ca, _ := g.campOf(ida)
		there := ca.ToWorld(at).Norm()
		level := ca.Site.Origin.Length() - planet.Radius + rise
		idb := g.add(Placed{Kind: b, Dir: there})
		e := g.Ents.Transform.Get(idb)
		e.SetAxes(planet.TangentAt(there, ca.Site.Dir(vec3{math.Cos(yaw), 0, math.Sin(yaw)})), there)
		if !math.IsNaN(rise) { // on the ground as it is
			g.Ents.Camp.Get(idb).Level = &level
		}
		g.standCamps()
		ca, _ = g.campOf(ida)
		cb, _ := g.campOf(idb)
		g.Player.Pos = g.World.Relief.Surface(ca.ToWorld(at.Scale(0.5)))
		g.phys = nil
		g.stepPhysics(1.0 / 60)
		return g, ca, cb
	}
	t.Fatal("no flat ground anywhere")
	return nil, CampAt{}, CampAt{}
}

// look stands the player 4 m out of a port, looking at its door.
func look(g *Game, r PortRef) {
	p, _ := g.portAt(r)
	g.Player.Pos = g.World.Relief.Surface(p.At.Add(p.Out.Scale(4)).Add(p.Out.Cross(p.At.Norm()).Scale(1)))
	g.Player.Heading = planet.TangentAt(g.Player.Pos.Norm(), p.At.Sub(g.Player.Pos))
	g.Player.Pitch = 0
}

// laid lays a link from a port to another as the player does; why it can't,
// "" if it did.
func laid(t *testing.T, g *Game, from, to PortRef) string {
	t.Helper()
	return laidAs(t, g, "core:sas", from, to)
}

// laidAs is laid, for a link of any kind.
func laidAs(t *testing.T, g *Game, kind string, from, to PortRef) string {
	t.Helper()
	it, _ := g.Content.Items.Get(kind)
	g.Ghost = nil
	g.hold(it)
	for _, r := range []PortRef{from, to} {
		look(g, r)
		g.poseGhost()
		if a := g.Ghost.Link.Aim; a == nil || *a != r {
			t.Fatalf("looking at %v, aims at %v", r, a)
		}
		if !g.Ghost.OK {
			return g.Ghost.Why
		}
		g.pickPort()
	}
	g.Ghost = nil
	return ""
}

// Links are laid straight, bent, sloping; walked through from a building to
// the other, never out of; their ports open, and no other building or link
// where they go.
func TestLinkWalk(t *testing.T) {
	for _, c := range []struct {
		name     string
		a, b     string
		at       vec3
		yaw      float64
		rise     float64
		from, to int
		points   int
	}{
		{"straight", "core:carrefour", "core:serre", vec3{22, 0, 0}, math.Pi, 0, 0, 0, 4},
		{"bent", "core:carrefour", "core:habitat", vec3{18, 0, 16}, -math.Pi / 2, 0, 0, 0, 5},
		{"aside", "core:carrefour", "core:bootcamp", vec3{24, 0, 5}, math.Pi, 0, 0, 0, 4},
		{"up", "core:carrefour", "core:garage", vec3{28, 0, 0}, 0, 0.6, 0, 0, 4},
		{"down", "core:serre", "core:observatoire", vec3{28, 0, 0}, math.Pi, -0.6, 0, 0, 4},
	} {
		g, a, b := pair(t, c.a, c.b, c.at, c.yaw, c.rise)
		from, to := PortRef{a.ID, c.from}, PortRef{b.ID, c.to}
		credits := g.State.Credits
		if why := laid(t, g, from, to); why != "" {
			t.Errorf("%s: %s", c.name, why)
			continue
		}
		l := g.Links()[0]
		if n := len(l.Layout.Pts); n != c.points {
			t.Errorf("%s: %d points, want %d", c.name, n, c.points)
		}
		if want := 500_000 * math.Ceil(l.Layout.Length-1e-6); credits-g.State.Credits != want || l.Paid != want {
			t.Errorf("%s: paid %v for %.1f m", c.name, credits-g.State.Credits, l.Layout.Length)
		}
		checkFights(t, c.name, l.Layout.Parts())
		if !g.Linked(a.ID, c.from) || g.CampDoor(b.Ent, c.to) != 1 {
			t.Errorf("%s: its ports aren't open", c.name)
		}
		// The ground is its floor, all along.
		l.Layout.Path(func(p vec3) {
			at := l.Layout.Site.ToWorld(p)
			if d := g.World.Relief.Altitude(at); math.Abs(d) > 0.06 {
				t.Errorf("%s: its floor is %.2f m over the ground at %v", c.name, d, p)
			}
		})
		// Through it, both ways.
		for _, way := range [][2]CampAt{{a, b}, {b, a}} {
			pts := append([]vec3{}, l.Layout.Pts...)
			pf, pt := a.Layout.Ports[c.from], b.Layout.Ports[c.to]
			start, end := a.ToWorld(pf.Airlock.From), b.ToWorld(pt.Airlock.From)
			if way[0].ID == b.ID {
				start, end = end, start
				for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
					pts[i], pts[j] = pts[j], pts[i]
				}
			}
			g.Player.Pos, g.Player.vel, g.phys = g.World.Relief.Surface(start), vec3{}, nil
			var goals []vec3
			for _, p := range pts {
				goals = append(goals, l.Layout.Site.ToWorld(p))
			}
			for _, to := range append(goals, end) {
				for range 900 {
					d := planet.TangentAt(g.Player.Pos.Norm(), to.Sub(g.Player.Pos))
					if planet.ArcDist(g.Player.Pos.Norm(), to.Norm()) < 0.15 {
						break
					}
					walk(g, d, 1.0/60)
					if !g.Indoors(g.Player.Eye()) {
						t.Fatalf("%s: out of it at %v", c.name, l.Layout.Site.ToLocal(g.Player.Pos))
					}
				}
				if d := planet.ArcDist(g.Player.Pos.Norm(), to.Norm()); d > 0.3 {
					t.Fatalf("%s: stopped %.1f m from %v, at %v", c.name, d, l.Layout.Site.ToLocal(to), l.Layout.Site.ToLocal(g.Player.Pos))
				}
			}
		}
		// Its walls stop the player, from inside and from outside.
		l.Layout.Path(func(p vec3) {
			for _, s := range []float64{-1, 1} {
				side := l.Layout.Site.Dir(vec3{0, 0, s})
				at := l.Layout.Site.ToWorld(p)
				in := l.Layout.Indoors(p.Add(vec3{0, 1, 0}))
				if math.Abs(l.Layout.Site.ToLocal(a.Site.Origin).Sub(p).Length()) < a.Layout.Reach+0.5 ||
					math.Abs(l.Layout.Site.ToLocal(b.Site.Origin).Sub(p).Length()) < b.Layout.Reach+0.5 || !in {
					continue
				}
				g.Player.Pos, g.Player.vel, g.phys = g.World.Relief.Surface(at), vec3{}, nil
				walk(g, side, 1.5)
				if !g.Indoors(g.Player.Eye()) {
					t.Errorf("%s: through its wall, from %v", c.name, p)
				}
			}
		})
	}
}

// What a link can't do, and why.
func TestLinkRefused(t *testing.T) {
	for _, c := range []struct {
		a, b string
		at   vec3
		yaw  float64
		rise float64
		to   int
		why  string
	}{
		{"core:carrefour", "core:serre", vec3{50, 0, 0}, math.Pi, 0, 0, "trop long : il faut un tunnel"},
		{"core:carrefour", "core:bootcamp", vec3{20, 0, 0}, math.Pi, 1.4, 0, "trop pentu : il faut un tunnel"},
		{"core:carrefour", "core:bootcamp", vec3{30, 0, 0}, math.Pi, 1.8, 0, "trop de dénivelé : il faut un tunnel"},
		{"core:carrefour", "core:bootcamp", vec3{20, 0, 0}, 0, 0, 0, "ces portes se tournent trop le dos"},
	} {
		g, a, b := pair(t, c.a, c.b, c.at, c.yaw, c.rise)
		it, _ := g.Content.Items.Get("core:sas")
		g.hold(it)
		g.Ghost.Link.From = &PortRef{a.ID, 0}
		look(g, PortRef{b.ID, c.to})
		g.poseGhost()
		if g.Ghost.OK || g.Ghost.Why != c.why {
			t.Errorf("%v, %v: OK %v, %q, want %q", c.at, c.rise, g.Ghost.OK, g.Ghost.Why, c.why)
		}
		if g.Ghost.Link.Aim == nil {
			t.Errorf("%v: aims at nothing", c.at)
		}
	}
	// What stands in the way, no credits, a link already there.
	g, a, b := pair(t, "core:carrefour", "core:serre", vec3{22, 0, 0}, math.Pi, 0)
	from, to := PortRef{a.ID, 0}, PortRef{b.ID, 0}
	g.State.Credits = 1000
	if why := laid(t, g, from, to); why != "crédits insuffisants" {
		t.Errorf("without credits: %q", why)
	}
	g.State.Credits = 1e9
	pf, _ := g.portAt(from)
	crate := g.add(Placed{Kind: "core:crate-2", Dir: pf.At.Add(pf.Out.Scale(5)).Norm()})
	g.phys = nil
	g.stepPhysics(1.0 / 60)
	if why := laid(t, g, from, to); why != "quelque chose gêne" {
		t.Errorf("with a crate in the way: %q", why)
	}
	g.Ents.Kill(crate)
	g.phys = nil
	g.stepPhysics(1.0 / 60)
	if why := laid(t, g, from, to); why != "" {
		t.Fatal(why)
	}
	look(g, from)
	it, _ := g.Content.Items.Get("core:sas")
	g.hold(it)
	g.poseGhost()
	if a := g.Ghost.Link.Aim; a != nil && *a == from {
		t.Error("a port already linked is aimed at")
	}
	// Nothing is set down on it, its buildings stay.
	g.Ghost = nil
	mid := g.Links()[0].Layout.Site.Origin.Norm()
	l := g.campLayout("core:habitat")
	for _, off := range []float64{0, l.Reach, l.Reach + 1.5} {
		d := planet.Toward2(mid, g.Links()[0].Layout.Site.Z, off)
		if ok, why := g.campFits(l, d, nil, 0); ok {
			t.Errorf("a building fits %.1f m from it (%s)", off, why)
		}
	}
	if !g.holdsLinks(a.Ent) {
		t.Error("a linked building is moved")
	}
	use := g.entityUse(a.Ent)
	use.Use(g, use.At)
	if _, ok := g.campOf(a.Ent); !ok {
		t.Error("a linked building is taken back")
	}
}

// A link is saved by what it links, stands again once loaded, and is taken
// back for its price, the ground as it was.
func TestLinkKept(t *testing.T) {
	g, a, b := pair(t, "core:carrefour", "core:serre", vec3{22, 0, 0}, math.Pi, 0)
	if why := laid(t, g, PortRef{a.ID, 0}, PortRef{b.ID, 0}); why != "" {
		t.Fatal(why)
	}
	paid := g.Links()[0].Paid
	key := g.Links()[0].Layout.Key
	raw, _ := json.Marshal(g.saved())
	var st State
	if err := json.Unmarshal(raw, &st); err != nil || len(st.Links) != 1 {
		t.Fatalf("saved: %v %+v", err, st.Links)
	}
	for id := range g.Ents.Camp.All() {
		g.Ents.Kill(id)
	}
	g.State.Links = nil
	g.standCamps()
	if len(g.Links()) != 0 || len(g.World.Relief.Flat().Ways.List) != 0 {
		t.Fatal("it stands without its buildings")
	}
	g.State.Links = st.Links
	for _, p := range st.Entities {
		if p.Camp != nil {
			g.add(p)
		}
	}
	if len(g.Links()) != 1 || g.Links()[0].Layout.Key != key {
		t.Fatalf("loaded: %d links", len(g.Links()))
	}
	l := g.Links()[0].Layout
	g.phys = nil
	// Aimed at from outside.
	g.Player.Pos = g.World.Relief.Surface(l.Site.ToWorld(vec3{0, 0, 4}))
	g.Player.Heading, g.Player.Pitch = l.Site.Dir(vec3{0, 0, -1}), 0
	g.stepPhysics(1.0 / 60)
	use, ok := g.NearbyUse()
	if !ok || use.Prompt != "[E] Reprendre : SAS" {
		t.Fatalf("aimed at, E says %q", use.Prompt)
	}
	if g.AimedMovable() != 0 {
		t.Error("what stands past it is aimed at")
	}
	had := g.State.Credits
	use.Use(g, use.At)
	if g.State.Credits-had != paid || len(g.Links()) != 0 || len(g.State.Links) != 0 {
		t.Errorf("taken back: %v of %v, %d left", g.State.Credits-had, paid, len(g.Links()))
	}
	if len(g.World.Relief.Flat().Ways.List) != 0 || g.Linked(a.ID, 0) {
		t.Error("the ground or its ports stay as they were with it")
	}
}

// A mod's link, its own materials, is laid as core's.
func TestLinkMod(t *testing.T) {
	g, a, b := pair(t, "core:carrefour", "core:serre", vec3{22, 0, 0}, math.Pi, 0)
	def := content.Link{ID: "t:tube", Name: "Tube", Width: 3, Max: 12, Floor: "core:steel"}
	def.Fill()
	g.Content.Links.Add("t:tube", def)
	from, _ := g.portAt(PortRef{a.ID, 0})
	to, _ := g.portAt(PortRef{b.ID, 0})
	l, _ := base.LayLink(&def, from, to, g.bareGround)
	if l == nil {
		t.Fatal("not laid")
	}
	if why := g.linkFits(l, PortRef{a.ID, 0}, PortRef{b.ID, 0}); (why == "trop long") != (l.Length > 12) {
		t.Errorf("%.1f m long: %q", l.Length, why)
	}
}

// No building is set down on a block.
func TestCampOnBlock(t *testing.T) {
	g := baseGame(t)
	g.State.Credits = 1e9
	gh := openPlain(t, g, "core:carrefour")
	at := gh.Pose.Dir
	if !gh.OK {
		t.Fatal(gh.Why)
	}
	g.add(Placed{Kind: "core:crate-2", Dir: at})
	g.phys = nil
	g.stepPhysics(1.0 / 60)
	g.poseGhost()
	if gh.OK || gh.Why != "quelque chose gêne" {
		t.Errorf("on a block: %v, %q", gh.OK, gh.Why)
	}
}
