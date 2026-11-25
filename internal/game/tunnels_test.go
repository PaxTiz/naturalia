package game

import (
	"encoding/json"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/planet"
	"testing"
)

// tunnelFrom digs a tunnel between two buildings, as the player does.
func tunnelFrom(t *testing.T, a, b string, at vec3, yaw, rise float64, from, to int) (*Game, CampAt, CampAt, *base.Link) {
	t.Helper()
	g, ca, cb := pair(t, a, b, at, yaw, rise)
	if why := laidAs(t, g, "core:tunnel", PortRef{ca.ID, from}, PortRef{cb.ID, to}); why != "" {
		t.Fatalf("%s to %s at %v: %s", a, b, at, why)
	}
	g.phys = nil
	return g, ca, cb, g.Links()[0].Layout
}

// follow walks the player along points, inside a tunnel all the way but for
// the buildings; how deep they went.
func follow(t *testing.T, g *Game, what string, goals []vec3) float64 {
	t.Helper()
	deep := 0.0
	for i, to := range goals {
		for range 1500 {
			if planet.ArcDist(g.Player.Pos.Norm(), to.Norm()) < 0.15 {
				break
			}
			was := g.Player.Pos.Length()
			walk(g, to.Sub(g.Player.Pos), 1.0/60)
			if !g.Indoors(g.Player.Eye()) {
				t.Fatalf("%s: out of it toward point %d, %.2f m over the ground", what, i, g.World.Relief.Altitude(g.Player.Pos))
			}
			if d := was - g.Player.Pos.Length(); math.Abs(d) > 0.25 {
				t.Fatalf("%s: jumped %.2f m down toward point %d", what, d, i)
			}
			deep = math.Max(deep, -g.World.Relief.Altitude(g.Player.Pos))
		}
		if d := planet.ArcDist(g.Player.Pos.Norm(), to.Norm()); d > 0.3 {
			t.Fatalf("%s: stopped %.1f m from point %d of %d, %.2f m over the ground", what, d, i, len(goals), g.World.Relief.Altitude(g.Player.Pos))
		}
	}
	return deep
}

// line is the way through a tunnel, from inside a building to inside the
// other, or back.
func line(l *base.Link, a, b CampAt, from, to int, back bool) []vec3 {
	out := []vec3{a.ToWorld(a.Layout.Ports[from].Airlock.From)}
	for _, p := range l.Pts {
		out = append(out, l.Site.ToWorld(p))
	}
	out = append(out, b.ToWorld(b.Layout.Ports[to].Airlock.From))
	if back {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}

// Tunnels are dug straight, turning, up and down hill; walked through both
// ways, down and up again, never out of; under the ground all along.
func TestTunnelWalk(t *testing.T) {
	for _, c := range []struct {
		name     string
		a, b     string
		at       vec3
		yaw      float64
		rise     float64
		from, to int
	}{
		{"straight", "core:carrefour", "core:serre", vec3{60, 0, 0}, math.Pi, 0, 0, 0},
		{"aside", "core:carrefour", "core:habitat", vec3{50, 0, 30}, math.Pi / 2, 0, 0, 0},
		{"backs turned", "core:carrefour", "core:bootcamp", vec3{50, 0, 8}, 0, 0, 0, 0},
		{"up", "core:serre", "core:garage", vec3{55, 0, 0}, 0, 3, 0, 0},
		{"down", "core:carrefour", "core:observatoire", vec3{48, 0, 0}, math.Pi, -3, 0, 0},
		{"far", "core:carrefour", "core:habitat", vec3{150, 0, -30}, math.Pi, math.NaN(), 0, 0},
	} {
		g, a, b, l := tunnelFrom(t, c.a, c.b, c.at, c.yaw, c.rise, c.from, c.to)
		lk := g.Links()[0]
		if want := 1_000_000 * math.Ceil(l.Length-1e-6); lk.Paid != want {
			t.Errorf("%s: paid %v for %.1f m", c.name, lk.Paid, l.Length)
		}
		checkFights(t, c.name, l.Parts())
		for _, sh := range l.Shells() {
			checkFights(t, c.name+", a shelter", sh.Parts())
		}
		l.Buried(func(at vec3, want float64) {
			if d := -g.World.Relief.Altitude(at); d < math.Min(want, 1.4) {
				t.Errorf("%s: %.2f m of ground over it, wants %.2f", c.name, d, want)
			}
		})
		for i, p := range l.Pts[:len(l.Pts)-1] {
			d := l.Site.ToWorld(l.Pts[i+1]).Sub(l.Site.ToWorld(p))
			if d.Length() > 5.5 || d.Length() < 0.05 {
				t.Errorf("%s: a stretch %.2f m long", c.name, d.Length())
			}
		}
		for _, back := range []bool{false, true} {
			pts := line(l, a, b, c.from, c.to, back)
			g.Player.Pos, g.Player.vel, g.Player.vr, g.phys = g.World.Relief.Surface(pts[0]), vec3{}, 0, nil
			if d := follow(t, g, c.name, pts); d < 5.2 {
				t.Errorf("%s: %.1f m deep at most", c.name, d)
			}
			if h := g.World.Relief.Altitude(g.Player.Pos); math.Abs(h) > 0.1 {
				t.Errorf("%s: %.2f m over the ground once through", c.name, h)
			}
		}
		// Its walls stop the player, all along.
		for i, p := range l.Pts[:len(l.Pts)-1] {
			at := l.Site.ToWorld(p).Lerp(l.Site.ToWorld(l.Pts[i+1]), 0.5)
			d := l.Site.ToWorld(l.Pts[i+1]).Sub(l.Site.ToWorld(p))
			for _, s := range []float64{-1, 1} {
				g.Player.Pos, g.Player.vel, g.Player.vr, g.phys = at, vec3{}, 0, nil
				walk(g, d.Cross(at.Norm()).Scale(s), 1)
				if !g.Under(g.Player.Eye()) {
					t.Errorf("%s: through its wall, past point %d", c.name, i)
				}
			}
		}
	}
}

// Those on the ground above walk and drive as if there was no tunnel; those
// in it bump into nothing of what stands above.
func TestTunnelLayers(t *testing.T) {
	g, a, b, l := tunnelFrom(t, "core:carrefour", "core:serre", vec3{60, 0, 0}, math.Pi, 0, 0, 0)
	mid := l.Site.ToWorld(l.Pts[len(l.Pts)/2])
	along := l.Site.Dir(l.Pts[len(l.Pts)/2+1].Sub(l.Pts[len(l.Pts)/2]))
	side := along.Cross(mid.Norm()).Norm()
	// Across it, over the ground, and along it.
	for _, d := range []vec3{side, along.Norm()} {
		g.Player.Pos, g.Player.vel, g.Player.vr, g.phys = g.World.Relief.Surface(mid.Sub(d.Scale(4))), vec3{}, 0, nil
		was := g.Player.Pos
		walk(g, d, 1.6)
		if moved := planet.ArcDist(was.Norm(), g.Player.Pos.Norm()); moved < 7.5 {
			t.Errorf("walking over it: %.1f m", moved)
		}
		if h := g.World.Relief.Altitude(g.Player.Pos); math.Abs(h) > 0.05 || g.Indoors(g.Player.Eye()) {
			t.Errorf("walking over it: %.2f m over the ground", h)
		}
	}
	// Blocks, a crate, a vehicle right above: the player below goes through.
	for i, p := range l.Pts {
		if i%3 == 0 && g.World.Relief.Altitude(l.Site.ToWorld(p)) < -5 {
			g.add(Placed{Kind: "core:crate-2", Dir: l.Site.ToWorld(p).Norm()})
		}
	}
	g.add(Placed{Kind: "core:block", Dir: mid.Norm()})
	pts := line(l, a, b, 0, 0, false)
	g.Player.Pos, g.Player.vel, g.phys = g.World.Relief.Surface(pts[0]), vec3{}, nil
	follow(t, g, "under things", pts)
	// On a shelter's roof.
	r := l.Site.ToWorld(l.Pts[0])
	p, _ := g.portAt(PortRef{a.ID, 0})
	at := p.At.Add(p.Out.Scale(4))
	top, ok := l.Roof(at)
	if !ok {
		t.Fatal("no roof", r)
	}
	g.Player.Pos, g.Player.vel, g.phys = at.Norm().Scale(top+0.3), vec3{}, nil
	walk(g, p.Out, 0.5)
	if g.Under(g.Player.Pos) {
		t.Error("through the roof of its shelter")
	}
	// Its shelter's walls, from outside.
	out := p.Out.Cross(p.At.Norm()).Norm()
	for _, s := range []float64{-1, 1} {
		g.Player.Pos, g.Player.vel, g.Player.vr, g.phys = g.World.Relief.Surface(p.At.Add(p.Out.Scale(2.5)).Add(out.Scale(4*s))), vec3{}, 0, nil
		walk(g, out.Scale(-s), 1.5)
		if g.Under(g.Player.Pos) || planet.ArcDist(g.Player.Pos.Norm(), p.At.Add(p.Out.Scale(2.5)).Norm()) < 1.6 {
			t.Error("through the wall of its shelter, from outside")
		}
	}
}

// What a tunnel can't do; saved with the player in it; taken back.
func TestTunnelKept(t *testing.T) {
	g, a, b, l := tunnelFrom(t, "core:carrefour", "core:carrefour", vec3{60, 0, 0}, math.Pi, 0, 0, 0)
	// Another across it.
	if why := laidAs(t, g, "core:tunnel", PortRef{a.ID, 1}, PortRef{b.ID, 1}); why != "croise un tunnel" && why != "" {
		t.Errorf("another: %q", why)
	}
	g.Ghost = nil
	raw, _ := json.Marshal(g.saved())
	var st State
	if err := json.Unmarshal(raw, &st); err != nil || len(st.Links) == 0 || st.Links[0].Kind != "core:tunnel" {
		t.Fatalf("saved: %v %+v", err, st.Links)
	}
	mid := l.Site.ToWorld(l.Pts[len(l.Pts)/2])
	g.Player.Pos, g.phys = mid, nil
	for range 30 {
		walk(g, vec3{}, 1.0/60)
	}
	if d := mid.Sub(g.Player.Pos).Length(); d > 0.1 || !g.Deep(g.Player.Eye()) {
		t.Errorf("standing in it, moved %.2f m", d)
	}
	g.Player.SetFly(true, g.World.Relief)
	g.Player.SetFly(false, g.World.Relief)
	if d := mid.Sub(g.Player.Pos).Length(); d > 0.1 {
		t.Errorf("after flying in it, moved %.2f m", d)
	}
	if g.updateAim(); g.AimOK {
		t.Error("the dust is swept from under the ground")
	}
	if it, ok := g.NearbyUse(); ok {
		t.Errorf("in it, E says %q", it.Prompt)
	}
	if it, _ := g.Content.Items.Get("core:block"); it != nil {
		g.hold(it)
		if g.poseGhost(); g.Ghost.OK {
			t.Error("a block is set down from under the ground")
		}
		g.Ghost = nil
	}
	// No building where its vault would come out, some far over it.
	lay := g.campLayout("core:bootcamp")
	if g.campDigs(mid.Norm(), lay.Reach, mid.Length()-planet.Radius+3) == false {
		t.Error("a building digs down to it")
	}
	// Taken back, aimed at by its shelter.
	p, _ := g.portAt(PortRef{a.ID, 0})
	side := p.Out.Cross(p.At.Norm()).Norm()
	g.Player.Pos, g.phys = g.World.Relief.Surface(p.At.Add(p.Out.Scale(2)).Add(side.Scale(4))), nil
	g.Player.Heading, g.Player.Pitch = planet.TangentAt(g.Player.Pos.Norm(), side.Scale(-1)), 0
	g.stepPhysics(1.0 / 60)
	use, ok := g.NearbyUse()
	if !ok || use.Prompt != "[E] Reprendre : Tunnel" {
		t.Fatalf("aimed at, E says %q", use.Prompt)
	}
	n, had := len(g.Links()), g.State.Credits
	use.Use(g, use.At)
	if len(g.Links()) != n-1 || g.State.Credits <= had || g.Under(mid) {
		t.Error("not taken back")
	}
}

// A mod's tunnel, wider, deeper and of its own materials, is dug as core's.
func TestTunnelMod(t *testing.T) {
	g, a, b := pair(t, "core:carrefour", "core:serre", vec3{60, 0, 0}, math.Pi, 0)
	def := content.Link{ID: "t:mine", Name: "Mine", Under: true, Width: 4, Height: 3.2, Depth: 9, Max: 40, Wall: "core:steel"}
	def.Fill()
	g.Content.Links.Add("t:mine", def)
	from, to := PortRef{a.ID, 0}, PortRef{b.ID, 0}
	pf, _ := g.portAt(from)
	pt, _ := g.portAt(to)
	l, why := base.LayLink(&def, pf, pt, g.bareGround)
	if l == nil {
		t.Fatal(why)
	}
	if why := g.linkFits(l, from, to); why != "trop long" {
		t.Errorf("%.1f m long: %q", l.Length, why)
	}
	g.State.Links = append(g.State.Links, Link{Kind: "t:mine", From: from, To: to})
	g.layLinks()
	pts := line(l, a, b, 0, 0, false)
	g.Player.Pos, g.phys = g.World.Relief.Surface(pts[0]), nil
	if d := follow(t, g, "a mod's", pts); d < 8 {
		t.Errorf("%.1f m deep", d)
	}
	checkFights(t, "a mod's", l.Parts())
	for _, sh := range l.Shells() {
		checkFights(t, "a mod's shelter", sh.Parts())
	}
}

// Tunnels laid out all over Mars, between ports turned any way: their
// stretches follow one another, gently sloping, deep enough.
func TestTunnelLayout(t *testing.T) {
	g := baseGame(t)
	def, _ := g.Content.Links.Get("core:tunnel")
	laid, why := 0, map[string]int{}
	for i := range 400 {
		k := float64(i)
		port := func(dir vec3, yaw float64) base.PortAt {
			t1 := planet.TangentAt(dir, vec3{0.3, 0.8, -0.5})
			out := t1.Scale(math.Cos(yaw)).Add(dir.Cross(t1).Scale(math.Sin(yaw)))
			return base.PortAt{At: dir.Scale(g.bareGround(dir)), Out: out, Up: dir, Landing: 1.2}
		}
		da := planet.Toward(vec3{math.Cos(k), 0, math.Sin(k)}, 300+20*k)
		db := planet.Toward2(da, planet.TangentAt(da, vec3{math.Sin(k * 3), math.Cos(k * 5), 0.2}), 25+math.Mod(k*7, 160))
		from, to := port(da, k*1.7), port(db, k*2.9)
		l, no := base.LayLink(def, from, to, g.bareGround)
		if l == nil {
			why[no]++
			continue
		}
		laid++
		h := l.Hollows()
		for _, s := range h[4:] {
			d := s.B.Sub(s.A)
			if slope := math.Abs(s.B.Length()-s.A.Length()) / d.Length(); slope > def.Slope+0.002 || d.Length() > 5.5 {
				t.Errorf("%d: a stretch %.1f m long, sloping %.3f", i, d.Length(), slope)
			}
		}
		for j := 5; j < len(h); j++ {
			if gap := h[j].A.Sub(h[j-1].B).Length(); gap > 0.01 {
				t.Errorf("%d: a gap of %.2f m", i, gap)
			}
			if c := h[j].B.Sub(h[j].A).Norm().Dot(h[j-1].B.Sub(h[j-1].A).Norm()); c < math.Cos(25*math.Pi/180) {
				t.Errorf("%d: bends by %.0f° past stretch %d of %d", i, math.Acos(c)*180/math.Pi, j, len(h))
			}
		}
		for e, p := range []base.PortAt{from, to} {
			foot := h[2*e+1].B
			end := h[4].A
			if e == 1 {
				end = h[len(h)-1].B
			}
			if gap := foot.Sub(end).Length(); gap > 0.1 || h[2*e].A.Sub(p.At).Length() > 0.01 {
				t.Errorf("%d: %.2f m from the foot of the stairs to the gallery", i, gap)
			}
		}
		l.Buried(func(at vec3, want float64) {
			if d := g.bareGround(at.Norm()) - at.Length(); want == def.Cover && d < want-0.7 {
				t.Errorf("%d: %.2f m of ground over it", i, d)
			}
		})
	}
	t.Log(laid, why)
	if laid < 250 {
		t.Errorf("%d laid out of 400: %v", laid, why)
	}
}
