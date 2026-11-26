package render

import (
	"cmp"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/engine/ecs"
	"naturalia/internal/engine/gpu"
	"naturalia/internal/engine/mathx"
	"naturalia/internal/game"
	"naturalia/internal/planet"
	"slices"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The lamps built by the player (content.Light): the closest to the camera
// light the scene, each with its shadows, one wide view along its cone in a
// cell of one texture. A view is drawn again only when something around its
// lamp changes.

const (
	maxSpots     = 16 // in spots.glsl
	spotGrid     = 4
	spotCell     = 512
	spotFov      = 150.0
	spotDist     = 130.0 // how far from the camera lamps light, fading from spotFade on
	spotFade     = 95.0
	spotSlot     = 13 // the texture slot of their map: past those a material binds
	spotNearClip = 0.05
	spotFarClip  = 40.0
)

type spots struct {
	*gpu.DepthMap
	seen [maxSpots]uint64 // what each cell shows (spotKey), 0 for nothing yet
	last []spotID         // the lamps of the cells, kept in their cell as long as they light
}

// spotID tells lamps apart: the one of an entity, or the n-th of a building
// or a link.
type spotID struct {
	ent ecs.ID
	n   int
}

func newSpots() *spots {
	return &spots{DepthMap: gpu.NewDepthMap(spotGrid, spotCell, spotNearClip, spotFarClip, true), last: make([]spotID, maxSpots)}
}

// nightAt is how much the lamps of the night are lit at a point: 0 by day.
func nightAt(at vec3) float64 {
	return 1 - mathx.Smoothstep(-0.06, 0.1, at.Norm().Dot(planet.SunDir))
}

// spot is a lamp lighting the scene.
type spot struct {
	id          spotID
	at, dir, up vec3 // where, where to, and the way its view's top points
	power       float64
	reach, edge float64
	col         vec3
	in          bool
	dist        float64
}

// lens is its view.
func (s *spot) lens() gpu.Lens {
	return gpu.Lens{Eye: s.at, Look: s.at.Add(s.dir), Up: s.up, Fovy: spotFov}
}

// spotsNear are the lamps that light the scene, the closest first.
func (gfx *Graphics) spotsNear(g *game.Game) []spot {
	if !gfx.space.Open() {
		return nil
	}
	var out []spot
	// A lamp standing in a frame; outdoors whatever it is in, if open.
	add := func(id spotID, l *content.Light, pos, ax, up, az vec3, open bool) {
		world := func(v vec3) vec3 { return ax.Scale(v.X).Add(up.Scale(v.Y)).Add(az.Scale(v.Z)) }
		s := spot{id: id, at: pos.Add(world(l.At)), power: content.OrZero(l.Power, 1), reach: math.Min(content.OrZero(l.Range, 10), spotFarClip), col: vec3{1.0, 0.86, 0.68}}
		s.dist = s.at.Sub(gfx.camEye).Length()
		if s.dist > spotDist {
			return
		}
		s.power *= 1 - mathx.Smoothstep(spotFade, spotDist, s.dist)
		if l.Night {
			s.power *= nightAt(pos)
		}
		if s.power < 0.004 {
			return
		}
		d := l.Dir
		if d == (vec3{}) {
			d = vec3{0, -1, 0}
		}
		s.dir = world(d.Norm())
		s.up = az
		s.edge = math.Cos(math.Min(content.OrZero(l.Cone, 70), 74) * math.Pi / 180)
		if t, ok := g.Content.Tokens.Get(l.Tint); ok && l.Tint != "" {
			// The painted color, straightened: light adds up.
			lin := func(c uint8) float64 { return math.Pow(float64(c)/255, 1.6) }
			s.col = vec3{lin(t.Col.R), lin(t.Col.G), lin(t.Col.B)}
		}
		s.in = !open && g.Indoors(s.at)
		out = append(out, s)
	}
	for _, id := range g.EntitiesNear(spotDist + 20) {
		def, ok := g.Content.Entities.Get(g.Kind(id))
		if !ok || def.Light == nil || g.Held(id) {
			continue
		}
		pos, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
		add(spotID{ent: id}, def.Light, pos, ax, up, az, false)
	}
	for _, c := range g.CampsNear(gfx.camEye, spotDist) {
		for i := range c.Layout.Lights {
			add(spotID{c.Ent, 1 + i}, &c.Layout.Lights[i], c.Site.Origin, c.Site.X, c.Site.Y, c.Site.Z, true)
		}
	}
	for _, l := range g.LinksNear(gfx.camEye, spotDist) {
		s := l.Layout.Site
		for i := range l.Layout.Lights {
			add(spotID{0, (l.From.Building*16+l.From.Port)*64 + 1 + i}, &l.Layout.Lights[i], s.Origin, s.X, s.Y, s.Z, true)
		}
	}
	slices.SortFunc(out, func(a, b spot) int { return cmp.Compare(a.dist, b.dist) })
	if len(out) > maxSpots { // those that make room fade out before they go
		edge := out[maxSpots].dist
		out = out[:maxSpots]
		for i := range out {
			out[i].power *= 1 - mathx.Smoothstep(edge*0.8, edge, out[i].dist)
		}
	}
	return out
}

// casts are the blocks and other still things in a lamp's reach.
func casts(g *game.Game, s *spot) []ecs.ID {
	var out []ecs.ID
	for id, t := range g.Ents.Transform.All() {
		if planet.ArcDist(s.at.Norm(), t.Dir) > s.reach+6 || g.Held(id) {
			continue
		}
		if _, ok := g.Content.Vehicles.Get(g.Kind(id)); ok {
			continue
		}
		out = append(out, id)
	}
	return out
}

// spotKey tells what a lamp's view shows: it changes when something in it
// does.
func spotKey(g *game.Game, s *spot, ids []ecs.ID) uint64 {
	h := uint64(14695981039346656037)
	add := func(v uint64) { h = (h ^ v) * 1099511628211 }
	for _, v := range []float64{s.at.X, s.at.Y, s.at.Z, s.dir.X, s.dir.Y, s.dir.Z} {
		add(math.Float64bits(v))
	}
	for _, id := range ids {
		t := g.Ents.Transform.Get(id)
		for _, v := range []float64{t.Dir.X, t.Dir.Y, t.Dir.Z, t.Yaw, t.Alt, t.Pitch, t.Roll, g.LeafOpen(id)} {
			add(math.Float64bits(v))
		}
		add(uint64(id))
	}
	return h | 1
}

// drawSpots picks the lamps, draws the views that changed, and gives the
// shaders all of it.
func (gfx *Graphics) drawSpots(g *game.Game) {
	sp := gfx.spots
	list := gfx.spotsNear(g)
	// Each keeps its cell while it lights: only a new one is drawn.
	cells := make([]*spot, maxSpots)
	for i := range list {
		if at := slices.Index(sp.last, list[i].id); at >= 0 {
			cells[at] = &list[i]
		}
	}
	for i := range list {
		if slices.Index(sp.last, list[i].id) < 0 {
			at := slices.Index(cells, nil)
			cells[at], sp.seen[at] = &list[i], 0
		}
	}
	for i, s := range cells {
		sp.last[i] = spotID{}
		if s == nil {
			continue
		}
		sp.last[i] = s.id
		ids := casts(g, s)
		if key := spotKey(g, s, ids); key != sp.seen[i] {
			sp.seen[i] = key
			gfx.drawSpot(g, i, s)
		}
	}

	var at, dir, col, in, vp []float32
	n := 0
	for i, s := range cells {
		if s == nil {
			at, dir, col, in = append(at, 0, 0, 0, 0), append(dir, 0, 0, 0, 0), append(col, 0, 0, 0, 0), append(in, 0)
			vp = append(vp, make([]float32, 16)...)
			continue
		}
		n = i + 1
		p := s.at.Sub(gfx.eng.Origin)
		at = append(at, float32(p.X), float32(p.Y), float32(p.Z), float32(s.power))
		dir = append(dir, float32(s.dir.X), float32(s.dir.Y), float32(s.dir.Z), float32(s.edge))
		col = append(col, float32(s.col.X), float32(s.col.Y), float32(s.col.Z), float32(s.reach))
		k := float32(0)
		if s.in {
			k = 1
		}
		in = append(in, k)
		vp = append(vp, gfx.spots.Cols(s.lens(), gfx.eng.Origin)...)
	}
	gfx.stats.Lamps = len(list)
	for _, sh := range gfx.lit() {
		sh.Float("spotCount", float32(n))
		// An int, as the shader takes it, in a float's bits.
		sh.Int("spotMap", spotSlot)
		if n == 0 {
			continue
		}
		sh.Vec4s("spotAt", at)
		sh.Vec4s("spotDir", dir)
		sh.Vec4s("spotCol", col)
		sh.Floats("spotIn", in)
		sh.Vec4s("spotVP", vp)
	}
	rl.ActiveTextureSlot(spotSlot)
	rl.EnableTexture(sp.Target.Texture.ID)
	rl.ActiveTextureSlot(0)
}

// drawSpot draws lamp s's view into cell i, over what it held.
func (gfx *Graphics) drawSpot(g *game.Game, i int, s *spot) {
	sp := gfx.spots
	sp.Begin(gfx.eng, false)
	sp.Draw(gfx.eng, i, s.lens(), true, func() {
		gfx.still = true // what moves isn't in the key
		gfx.drawEntities(g, func(at vec3, r float64) bool { return at.Sub(s.at).Length() < s.reach+r })
		gfx.still = false
	})
	sp.End(gfx.eng)
}

// lampShadow holds the shadows of the rooms' ceiling lamps: one wide
// perspective view straight down from each lamp, in the cells of a grid
// (3×3 at most). The base never moves: they are drawn once, and again only
// when other rooms come closer (selectBase). Every room kept keeps its
// shadows, so nothing pops in when the player walks in.
type lampShadow struct {
	*gpu.DepthMap
	lamps []lampView // where the cameras stand, looking down
	done  bool
}

// lampView is a lamp's camera: where it stands, the up it looks down
// from (a bootcamp's isn't the world's), and the way the top of its view
// points.
type lampView struct{ at, up, side vec3 }

func (l lampView) lens() gpu.Lens {
	return gpu.Lens{Eye: l.at, Look: l.at.Sub(l.up), Up: l.side, Fovy: lampFov}
}

const (
	lampShadowCell = 1024 // each lamp's view; the rooms' lamps are the first lights of base.Lamps
	lampFov        = 150.0
)

func newLampShadow(lamps int) *lampShadow {
	grid := int(math.Ceil(math.Sqrt(float64(max(1, lamps)))))
	return &lampShadow{DepthMap: gpu.NewDepthMap(grid, lampShadowCell, 0.1, 30, true)}
}

// cols gives the lamps' cameras about origin o, four columns each.
func (s *lampShadow) cols(o vec3) []float32 {
	var c []float32
	for _, l := range s.lamps {
		c = append(c, s.Cols(l.lens(), o)...)
	}
	return c
}

// reset has the lamps' views drawn again.
func (s *lampShadow) reset() { s.done, s.lamps = false, nil }

// render draws every lamp's view into its cell, once.
func (s *lampShadow) render(gfx *Graphics, lamps []lampView, draw func()) {
	if s.done {
		return
	}
	s.done = true
	s.Begin(gfx.eng, true)
	for i, lamp := range lamps[:min(len(lamps), s.Grid*s.Grid)] {
		s.lamps = append(s.lamps, lamp)
		s.Draw(gfx.eng, i, lamp.lens(), false, draw)
	}
	s.End(gfx.eng)
}
