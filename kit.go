package main

import (
	"image/color"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The buildings' design system (docs/MODS.md): tokens say what a surface
// is made of, kit pieces are shapes built once in code, and blueprints are
// data that put pieces together. A blueprint is described once: drawing,
// collisions and interactions are deduced from it.

// token is a material of the base: color, painted material, shine, glow.
type token struct {
	col      color.RGBA
	mat      int
	scale    float32 // texture tiles per meter
	spec     float32
	emissive float32
}

// Off-whites, steel greys, cyan and yellow/black accents: they stand out
// against the orange ground.
var tokens = map[string]token{
	"habitat.white":  {color.RGBA{228, 225, 216, 255}, mtPlain, 1, 0.35, 0},
	"habitat.white2": {color.RGBA{212, 210, 203, 255}, mtPlain, 1, 0.35, 0},
	"steel":          {color.RGBA{140, 146, 158, 255}, mtMetal, 1.2, 0.6, 0},
	"steel.dark":     {color.RGBA{50, 54, 66, 255}, mtPlain, 1, 0.4, 0},
	"hazard":         {colWhite, mtHazard, 2.5, 0.3, 0},
	"neon.cyan":      {accentCyan, mtPlain, 1, 0, 0.8},
	"crate.ochre":    {color.RGBA{196, 132, 60, 255}, mtPlain, 1, 0.2, 0},
	"crate.slate":    {color.RGBA{88, 116, 146, 255}, mtPlain, 1, 0.2, 0},
	"crate.sand":     {color.RGBA{158, 148, 118, 255}, mtPlain, 1, 0.2, 0},
}

// Blueprint is a building as data.
type Blueprint struct {
	ID    string
	At    vec3 // local, center of the floor
	Shape Dome
	Style string // token of the walls and roof
	Doors []Door
	Props []Prop
}

type Dome struct{ Radius, WallH float64 }

type Door struct{ Facing float64 } // angle, atan2(z, x)

// Prop is a kit piece placed in a building, relative to its center; Seed
// varies pieces that are built by rules.
type Prop struct {
	Kit  string
	At   vec3
	Seed int
}

var salePlan = Blueprint{
	ID:    "core:sale",
	At:    vec3{-13, 0, 0},
	Shape: Dome{Radius: 4, WallH: 2.6},
	Style: "habitat.white",
	Doors: []Door{{Facing: east}},
	Props: []Prop{
		{Kit: "incinerator", At: vec3{-1.6, 0, 0}},
		{Kit: "crate.stack", At: vec3{1.2, 0, 2.4}, Seed: 3},
		{Kit: "crate.stack", At: vec3{0.6, 0, -2.7}, Seed: 8},
	},
}

var plans = []*Blueprint{&salePlan}

func (bp *Blueprint) room() room {
	var openings []float64
	for _, d := range bp.Doors {
		openings = append(openings, d.Facing)
	}
	return room{bp.At, bp.Shape.Radius, bp.Shape.WallH, openings, bp}
}

// kitProp is a piece of furniture: what it adds to the merged meshes, what
// it draws each frame (moving or glowing parts), how the player bumps into
// it and uses it.
type kitProp struct {
	build    func(b *building, at vec3, seed int)
	draw     func(g *Game, at vec3)
	obstacle float64
	reach    float64
	use      interaction
	prompt   string
}

var kitProps = map[string]kitProp{
	"incinerator": {draw: drawIncinerator, obstacle: 1.4, reach: 2.8, use: useIncinerator,
		prompt: "[E] Déposer la poussière dans l'incinérateur"},
	"crate.stack": {build: buildCrates, obstacle: 0.55},
}

// spot returns where the first prop of a kit sits in the base.
func (bp *Blueprint) spot(kit string) vec3 {
	for _, p := range bp.Props {
		if p.Kit == kit {
			return bp.At.add(p.At)
		}
	}
	return bp.At
}

func planObstacles() (out []obstacle) {
	for _, bp := range plans {
		for _, p := range bp.Props {
			if r := kitProps[p.Kit].obstacle; r > 0 {
				out = append(out, obstacle{bp.At.add(p.At), r})
			}
		}
	}
	return out
}

func planInteractables() (out []interactable) {
	for _, bp := range plans {
		for _, p := range bp.Props {
			if k := kitProps[p.Kit]; k.use != useNone {
				out = append(out, interactable{bp.At.add(p.At), k.reach, k.use, k.prompt})
			}
		}
	}
	return out
}

// building holds a blueprint's merged meshes: one per token and lighting.
type building struct {
	bp     *Blueprint
	inside vec3 // world point that walls face
	parts  map[partKey]*meshBuilder
	keys   []partKey
	models []rl.Model
}

type partKey struct {
	tok string
	lit float32
}

func (b *building) part(tok string, lit float32) *meshBuilder {
	k := partKey{tok, lit}
	if b.parts[k] == nil {
		b.parts[k] = &meshBuilder{}
		b.keys = append(b.keys, k)
	}
	return b.parts[k]
}

func buildPlan(bp *Blueprint, sh rl.Shader) *building {
	b := &building{bp: bp, parts: map[partKey]*meshBuilder{}}
	r := bp.room()
	b.inside = toWorld(r.center.add(vec3{0, 1.2, 0}))
	// Walls: bevelled panels, a dark plinth, a steel collar under the dome.
	for i, p := range r.panels() {
		tok := bp.Style
		if i%2 == 1 {
			tok += "2"
		}
		out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
		tangent := vec3{-out.Z, 0, out.X}
		b.part(tok, litTowards).box(p.mid.add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, wallThick}, tangent, 0.04)
		b.part("steel", litTowards).box(p.mid.add(vec3{0, p.y1 - 0.1, 0}).add(out.scale(0.03)), vec3{p.width, 0.2, wallThick + 0.08}, tangent, 0.035)
		if p.y0 == 0 {
			b.part("steel.dark", litTowards).box(p.mid.add(vec3{0, 0.15, 0}).add(out.scale(0.03)), vec3{p.width, 0.3, wallThick + 0.08}, tangent, 0.035)
		}
	}
	// Door frames: steel jambs and a hazard header.
	for _, d := range bp.Doors {
		out := vec3{math.Cos(d.Facing), 0, math.Sin(d.Facing)}
		tangent := vec3{-out.Z, 0, out.X}
		w := r.center.add(out.scale(r.radius))
		for _, s := range []float64{-1, 1} {
			b.part("steel", litTowards).box(w.add(tangent.scale(s*(doorWidth/2+0.13))).add(vec3{0, (doorHeight + 0.1) / 2, 0}), vec3{0.26, doorHeight + 0.1, 0.42}, tangent, 0.05)
		}
		b.part("hazard", litTowards).box(w.add(vec3{0, doorHeight + 0.2, 0}), vec3{doorWidth + 0.52, 0.22, 0.42}, tangent, 0.05)
		b.part("neon.cyan", litIndoor).box(w.add(vec3{0, doorHeight + 0.2, 0}).sub(out.scale(0.22)), vec3{doorWidth * 0.6, 0.05, 0.02}, tangent, 0)
	}
	for _, p := range bp.Props {
		if k := kitProps[p.Kit]; k.build != nil {
			k.build(b, bp.At.add(p.At), p.Seed)
		}
	}
	for _, k := range b.keys {
		b.models = append(b.models, b.parts[k].model(sh))
	}
	return b
}

func (b *building) draw(gfx *Graphics) {
	for i, k := range b.keys {
		t := tokens[k.tok]
		gfx.setMat(t.mat, t.scale, false)
		gfx.setLighting(k.lit, b.inside)
		rl.SetShaderValue(gfx.objectSh, gfx.oLoc.surface, []float32{t.spec, t.emissive}, rl.ShaderUniformVec2)
		rl.DrawModel(b.models[i], rl.Vector3{}, 1, t.col)
	}
	// The dome stays a smooth shell: dust settles on its top.
	r := b.bp.room()
	t := tokens[b.bp.Style]
	gfx.setMat(t.mat, t.scale, false)
	gfx.setLighting(litBack, b.inside)
	gfx.put(gfx.hemi, toWorld(r.center.add(vec3{0, r.wallH, 0})), vec3{r.radius + 0.12, r.radius * 0.55, r.radius + 0.12}, vec3{0, 1, 0}, 0, t.col, t.spec, 0)
}

// buildCrates stacks two or three bevelled crates, chosen by the seed.
func buildCrates(b *building, at vec3, seed int) {
	cols := []string{"crate.ochre", "crate.slate", "crate.sand"}
	rnd := func(k int) float64 { return math.Mod(math.Abs(math.Sin(float64(seed*7+k)*12.9898))*43758.5453, 1) }
	y := 0.0
	for i := range 2 + seed%2 {
		s := 0.75 - 0.15*float64(i) - 0.1*rnd(i)
		a := rnd(i+10) * math.Pi
		off := vec3{(rnd(i+20) - 0.5) * 0.15, 0, (rnd(i+30) - 0.5) * 0.15}
		b.part(cols[(seed+i)%3], litIndoor).box(at.add(off).add(vec3{0, y + s/2, 0}), vec3{s, s, s}, vec3{math.Cos(a), 0, math.Sin(a)}, 0.05)
		y += s
	}
}

// drawIncinerator draws the incinerator, glowing harder while it burns dust.
func drawIncinerator(g *Game, in vec3) {
	gfx := g.gfx
	up := vec3{0, 1, 0}
	steel, dark := tokens["steel"], tokens["steel.dark"]
	glow := float32(0.55 + 0.45*clamp01(g.burn))
	gfx.setLighting(litIndoor, vec3{})
	gfx.setMat(steel.mat, steel.scale, false)
	gfx.put(gfx.cyl, toWorld(in), vec3{1.1, 1.9, 1.1}, up, 0, steel.col, steel.spec, 0)
	gfx.segment(gfx.cone, toWorld(in.add(vec3{0, 2.75, 0})), toWorld(in.add(vec3{0, 1.9, 0})), 0.95, steel.col, steel.spec, 0)
	gfx.setMat(dark.mat, dark.scale, false)
	gfx.put(gfx.cyl, toWorld(in.add(vec3{0, 0.25, 0})), vec3{1.14, 0.12, 1.14}, up, 0, dark.col, dark.spec, 0)
	gfx.segment(gfx.cyl, toWorld(in.add(vec3{-0.6, 1.5, 0.6})), toWorld(in.add(vec3{-0.6, 4.9, 0.6})), 0.18, dark.col, dark.spec, 0)
	gfx.box(in.add(vec3{1.0, 1.0, 1.25}), vec3{0.5, 0.7, 0.3}, vec3{0, 0, 1}, dark.col, dark.spec, 0)
	gfx.setMat(mtHazard, 3, false)
	gfx.put(gfx.cyl, toWorld(in.add(vec3{0, 1.6, 0})), vec3{1.14, 0.14, 1.14}, up, 0, colWhite, 0.3, 0)
	gfx.setMat(mtPlain, 1, false)
	gfx.box(in.add(vec3{1.08, 0.95, 0}), vec3{0.8, 0.55, 0.06}, vec3{0, 0, 1}, accentOrange, 0, glow)
	gfx.drawScreen(in.add(vec3{1.16, 1.1, 1.25}), vec3{1, 0, 0}, 0.38, 0.28, gfx.screens[3], 0.8)
}

// meshBuilder collects flat-shaded triangles in world coordinates.
type meshBuilder struct{ v, n, uv []float32 }

// tri adds a triangle facing away from center.
func (m *meshBuilder) tri(a, b, c, center vec3) {
	nrm := b.sub(a).cross(c.sub(a))
	if nrm.dot(a.add(b).add(c).scale(1.0/3).sub(center)) < 0 {
		b, c = c, b
		nrm = nrm.scale(-1)
	}
	nrm = nrm.norm()
	for _, p := range []vec3{a, b, c} {
		w := toWorld(p)
		m.v = append(m.v, float32(w.X), float32(w.Y), float32(w.Z))
		m.n = append(m.n, float32(nrm.X), float32(nrm.Y), float32(nrm.Z))
		m.uv = append(m.uv, 0, 0)
	}
}

func (m *meshBuilder) quad(a, b, c, d, center vec3) {
	m.tri(a, b, c, center)
	m.tri(a, c, d, center)
}

// box adds a box centered at the local point c, its width (X) along the
// horizontal direction across, with edges and corners chamfered by bevel:
// they catch the light instead of being sharp.
func (m *meshBuilder) box(c, size, across vec3, bevel float64) {
	ex := across.norm()
	ax := [3]vec3{ex, {0, 1, 0}, ex.cross(vec3{0, 1, 0})}
	h := [3]float64{size.X / 2, size.Y / 2, size.Z / 2}
	bevel = math.Min(bevel, 0.45*math.Min(h[0], math.Min(h[1], h[2])))
	// pt is the corner s pushed out along one axis: the corner of that face.
	pt := func(axis int, s [3]float64) vec3 {
		p := c
		for k := range 3 {
			q := s[k] * (h[k] - bevel)
			if k == axis {
				q = s[k] * h[k]
			}
			p = p.add(ax[k].scale(q))
		}
		return p
	}
	signs := [2]float64{-1, 1}
	for a := range 3 {
		u, v := (a+1)%3, (a+2)%3
		for _, sa := range signs {
			var q [4]vec3
			for i, uv := range [4][2]float64{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
				var s [3]float64
				s[a], s[u], s[v] = sa, uv[0], uv[1]
				q[i] = pt(a, s)
			}
			m.quad(q[0], q[1], q[2], q[3], c)
			if bevel == 0 {
				continue
			}
			// The bevel along axis v between faces a and u.
			for _, su := range signs {
				var s0, s1 [3]float64
				s0[a], s0[u], s0[v] = sa, su, -1
				s1[a], s1[u], s1[v] = sa, su, 1
				m.quad(pt(a, s0), pt(a, s1), pt(u, s1), pt(u, s0), c)
			}
		}
	}
	if bevel == 0 {
		return
	}
	for _, sx := range signs {
		for _, sy := range signs {
			for _, sz := range signs {
				s := [3]float64{sx, sy, sz}
				m.tri(pt(0, s), pt(1, s), pt(2, s), c)
			}
		}
	}
}

// model uploads the triangles. The slices stay referenced by the builder:
// the mesh is never unloaded.
func (m *meshBuilder) model(sh rl.Shader) rl.Model {
	mesh := rl.Mesh{VertexCount: int32(len(m.v) / 3), TriangleCount: int32(len(m.v) / 9)}
	mesh.Vertices, mesh.Normals, mesh.Texcoords = &m.v[0], &m.n[0], &m.uv[0]
	rl.UploadMesh(&mesh, false)
	mod := rl.LoadModelFromMesh(mesh)
	mod.GetMaterials()[0].Shader = sh
	return mod
}
