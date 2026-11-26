package render

import (
	"cmp"
	"fmt"
	"image/color"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/engine/mathx"
	"naturalia/internal/engine/mesh"
	"naturalia/internal/game"
	"naturalia/internal/palette"
	"naturalia/internal/planet"
	"slices"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type entityModel struct {
	model rl.Model
	tok   content.Token
	glass bool // seen through: drawn last (drawPanes)
}

// sees tells whether a sphere may be in a picture.
type sees func(at vec3, radius float64) bool

// inView: what the camera sees, a cone around its picture.
func (gfx *Graphics) inView(cam rl.Camera3D) sees {
	fwd := gfx.camLook.Sub(gfx.camEye).Norm()
	aspect := float64(rl.GetScreenWidth()) / math.Max(1, float64(rl.GetScreenHeight()))
	sin, cos := math.Sincos(math.Atan(math.Tan(float64(cam.Fovy)*math.Pi/360) * math.Hypot(1, aspect)))
	return func(at vec3, radius float64) bool {
		d := at.Sub(gfx.camEye)
		z := d.Dot(fwd)
		return math.Sqrt(math.Max(0, d.Dot(d)-z*z))*cos-z*sin < radius
	}
}

// inShadowMap: what casts its shadow into the sun's map around center.
func (gfx *Graphics) inShadowMap(center vec3) sees {
	return func(at vec3, radius float64) bool { return gfx.shadow.Covers(center, planet.SunDir, at, radius) }
}

// entityReach is how far from its origin an entity's parts go, figured
// once by kind.
func (gfx *Graphics) entityReach(def *content.EntityDef) float64 {
	r, ok := gfx.reach[def.ID]
	if !ok {
		lo, hi := content.Bounds(def.Parts)
		r = vec3{math.Max(-lo.X, hi.X), math.Max(-lo.Y, hi.Y), math.Max(-lo.Z, hi.Z)}.Length()
		if gfx.reach == nil {
			gfx.reach = map[string]float64{}
		}
		gfx.reach[def.ID] = r
	}
	return r
}

// drawEntities draws the entities around the player, standing up from the
// ground: those seen.
func (gfx *Graphics) drawEntities(g *game.Game, seen sees) {
	for _, id := range g.EntitiesNear(game.EntityDrawDist) {
		kind := *g.Ents.Kind.Get(id)
		def, ok := g.Content.Entities.Get(kind)
		if !ok || g.Held(id) {
			continue // its mod was removed (it stays in the save), or it is being moved
		}
		if _, car := g.Content.Vehicles.Get(kind); car && gfx.still {
			continue
		}
		pos, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
		if !seen(pos, gfx.entityReach(def)+0.5) {
			continue
		}
		models := gfx.entityModels(def, g.Paint(id))
		lit := float32(mesh.LitSun)
		if g.Indoors(pos) {
			lit = mesh.LitIndoor
		}
		gfx.setLighting(lit, vec3{})
		put := func(models []entityModel, pos vec3) {
			for _, m := range models {
				if m.glass {
					continue
				}
				gfx.setMat(m.tok.Mat, m.tok.Scale, false)
				glow := m.tok.Emissive
				if def.Light != nil && def.Light.Night {
					glow *= 0.15 + 0.85*float32(nightAt(pos))
				}
				gfx.putBasis(m.model, pos, ax, up, az, m.tok.Col, m.tok.Spec, glow)
			}
		}
		put(models, pos)
		for i := range def.Leaves {
			put(gfx.leafModels(def, i, g.Paint(id)), pos.Add(g.LeafShift(id, i)))
		}
		if v, ok := g.Content.Vehicles.Get(kind); ok && v.Light != (vec3{}) && math.Sin(g.Elapsed*6+float64(id.Index())) > 0 {
			light := palette.Green
			if g.TankFull() {
				light = palette.Red
			}
			at := pos.Add(ax.Scale(v.Light.X)).Add(up.Scale(v.Light.Y)).Add(az.Scale(v.Light.Z))
			axis, angle := mathx.Orient(up)
			gfx.setMat(content.MtPlain, 1, false)
			gfx.put(gfx.sphere, at, vec3{0.045, 0.045, 0.045}, axis, angle, light, 0, 1)
		}
	}
	gfx.setMat(content.MtPlain, 1, false)
	gfx.setLighting(mesh.LitSun, vec3{})
}

// drawPanes draws the entities' glass over the scene, as drawGlass the
// base's, the farthest first.
func (gfx *Graphics) drawPanes(g *game.Game, seen sees) {
	type pane struct {
		m               entityModel
		pos, ax, up, az vec3
		dist            float64
	}
	var panes []pane
	for _, id := range g.EntitiesNear(game.EntityDrawDist) {
		def, ok := g.Content.Entities.Get(*g.Ents.Kind.Get(id))
		if !ok || g.Held(id) {
			continue
		}
		pos, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
		if !seen(pos, gfx.entityReach(def)+0.5) {
			continue
		}
		add := func(models []entityModel, pos vec3) {
			for _, m := range models {
				if m.glass {
					panes = append(panes, pane{m, pos, ax, up, az, pos.Sub(gfx.camEye).Length()})
				}
			}
		}
		add(gfx.entityModels(def, g.Paint(id)), pos)
		for i := range def.Leaves {
			add(gfx.leafModels(def, i, g.Paint(id)), pos.Add(g.LeafShift(id, i)))
		}
	}
	if len(panes) == 0 {
		return
	}
	slices.SortFunc(panes, func(a, b pane) int { return cmp.Compare(b.dist, a.dist) })
	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()
	gfx.setMat(content.MtPlain, 1, false)
	gfx.setLighting(mesh.LitGlass, vec3{})
	for _, p := range panes {
		gfx.putBasis(p.m.model, p.pos, p.ax, p.up, p.az, p.m.tok.Col, p.m.tok.Spec, 0)
	}
	rl.DrawRenderBatchActive()
	rl.EnableDepthMask()
	gfx.setLighting(mesh.LitSun, vec3{})
}

// entityModels are an entity's models, painted with a finish ("" for its
// own), built on first use.
func (gfx *Graphics) entityModels(def *content.EntityDef, finish string) []entityModel {
	return gfx.partModels(def, finish, "", def.Parts)
}

// leafModels are the models of the leaf i of an entity's door.
func (gfx *Graphics) leafModels(def *content.EntityDef, i int, finish string) []entityModel {
	return gfx.partModels(def, finish, fmt.Sprintf(" leaf %d", i), def.Leaves[i].Parts)
}

// partModels are the models of some of an entity's parts, kept under its
// ID and what they are of it.
func (gfx *Graphics) partModels(def *content.EntityDef, finish, what string, parts []content.PartJSON) []entityModel {
	key := def.ID + what
	if def.Paint != "" {
		key += " " + finish
	}
	models, ok := gfx.entities[key]
	if !ok {
		models = buildEntity(def, finish, parts, gfx)
		gfx.entities[key] = models
	}
	return models
}

// setGrid gives the shaders the building grid's lines, around the ghost on
// one (grid.glsl).
func (gfx *Graphics) setGrid(g *game.Game) {
	size := []float32{0, 0}
	var s game.Sheet
	if g.Ghost != nil && g.Ghost.Sheet != nil {
		s = *g.Ghost.Sheet
		size = []float32{float32(s.Step), float32(s.Reach)}
	}
	for _, sh := range gfx.lit() {
		sh.Vec2("gridSize", size[0], size[1])
		if s.Reach == 0 {
			continue
		}
		for name, v := range map[string]vec3{"gridRel": s.At.Sub(gfx.eng.Origin), "gridX": s.X, "gridUp": s.Up, "gridZ": s.Z} {
			sh.Vec3(name, v)
		}
	}
}

// drawGhost draws the item being placed see-through, glowing cyan where
// it fits, red where it doesn't. Last, over the scene. Its depth is laid
// first: only its faces turned to the player show, it reads as a solid.
// Where it hangs over something, its corners' feet go down to it.
func (gfx *Graphics) drawGhost(g *game.Game) {
	gh := g.Ghost
	if gh == nil {
		return
	}
	def, ok := g.Content.Entities.Get(gh.Kind)
	if !ok && gh.Link == nil {
		return
	}
	col := color.RGBA{95, 225, 238, 150}
	if !gh.OK {
		col = color.RGBA{255, 90, 70, 150}
	}
	pos, ax, up, az := g.Frame(&gh.Pose)
	draw := func() {
		if gh.Link != nil {
			if l := gh.Link.Layout; l != nil && l.Under() {
				rl.DisableBackfaceCulling()
				gfx.drawTunnelGhost(l, col)
				rl.EnableBackfaceCulling()
			} else if l != nil {
				rl.DisableBackfaceCulling()
				m := gfx.linkModels(l)
				for i, md := range m.models {
					if m.Keys[i].Lit == mesh.LitGlass {
						continue // its frames tell enough
					}
					gfx.putBasis(md, l.Site.Origin, l.Site.X, l.Site.Y, l.Site.Z, col, 0, 0.22)
				}
				rl.EnableBackfaceCulling()
			}
			rl.DrawRenderBatchActive()
			return
		}
		all := gfx.entityModels(def, gh.Paint)
		for i := range def.Leaves {
			all = append(all[:len(all):len(all)], gfx.leafModels(def, i, gh.Paint)...)
		}
		for _, m := range all {
			gfx.putBasis(m.model, pos, ax, up, az, col, 0, 0.22)
		}
		if l := g.CampLayout(def.ID); l != nil { // a bootcamp: its meshes, all alike
			rl.DisableBackfaceCulling()
			for _, md := range gfx.campModels(def.ID, l).models {
				gfx.putBasis(md, pos, ax, up, az, col, 0, 0.22)
			}
			rl.EnableBackfaceCulling()
		}
		rl.DrawRenderBatchActive()
	}
	rl.DrawRenderBatchActive()
	gfx.setLighting(mesh.LitSun, vec3{})
	gfx.setMat(content.MtPlain, 1, false)
	// No lines on the ghost itself.
	gfx.objectSh.Vec2("gridSize", 0, 0)
	if gh.Sheet != nil {
		for _, f := range gh.Sheet.Feet {
			gfx.segment(gfx.cyl, f[0], f[1], 0.012, color.RGBA{col.R, col.G, col.B, 255}, 0, 0.5)
		}
	}
	rl.ColorMask(false, false, false, false)
	gfx.eng.Depth(true)
	draw()
	gfx.eng.Depth(false)
	rl.ColorMask(true, true, true, true)
	draw()
	if gh.Link != nil {
		if l := gh.Link.Layout; l != nil && l.Under() {
			gfx.drawTunnelWay(l, col)
		}
		gfx.drawPorts(g, gh.Link, col)
		rl.DrawRenderBatchActive()
		return
	}
	for _, e := range g.GhostEdges() {
		gfx.segment(gfx.cyl, e[0], e[1], 0.014, color.RGBA{col.R, col.G, col.B, 255}, 0, 0.45)
	}
	rl.DrawRenderBatchActive()
}

// drawPorts outlines the free ports around, those a link being laid goes
// between brighter and thicker.
func (gfx *Graphics) drawPorts(g *game.Game, k *game.LinkGhost, col color.RGBA) {
	g.FreePorts(func(r game.PortRef, p base.PortAt) {
		thick, glow, c := 0.02, float32(0.25), color.RGBA{95, 225, 238, 255}
		if k.Aim != nil && *k.Aim == r || k.From != nil && *k.From == r {
			thick, glow, c = 0.06, 0.8, color.RGBA{col.R, col.G, col.B, 255}
		}
		up := p.At.Norm()
		side := p.Out.Cross(up).Norm()
		at := p.At.Add(p.Out.Scale(0.3))
		pt := func(x, y float64) vec3 { return at.Add(side.Scale(x)).Add(up.Scale(y)) }
		c4 := [4]vec3{pt(-1.6, 0.05), pt(1.6, 0.05), pt(1.6, 2.85), pt(-1.6, 2.85)}
		for i := range c4 {
			gfx.segment(gfx.cyl, c4[i], c4[(i+1)%4], thick, c, 0, glow)
		}
	})
}

// buildEntity merges some of an entity's parts into one model per
// material, in its own frame.
func buildEntity(def *content.EntityDef, finish string, parts []content.PartJSON, gfx *Graphics) []entityModel {
	b := gfx.content.Painted(def, finish, parts)
	b.Unfight()
	var out []entityModel
	for _, k := range b.Keys {
		m := b.Mesh(k)
		if len(m.V) == 0 {
			continue
		}
		model := upload(m, gfx.objectSh.Shader)
		gfx.bindMaps(model)
		out = append(out, entityModel{model, gfx.tok(k.Tok), k.Lit == mesh.LitGlass})
	}
	return out
}
