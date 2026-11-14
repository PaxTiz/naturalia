package render

import (
	"image/color"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/game"
	"naturalia/internal/mathx"
	"naturalia/internal/mesh"
	"naturalia/internal/palette"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type entityModel struct {
	model rl.Model
	tok   content.Token
}

// drawEntities draws the entities around the player, standing up from the
// ground.
func (gfx *Graphics) drawEntities(g *game.Game) {
	for _, id := range g.EntitiesNear(game.EntityDrawDist) {
		kind := *g.Ents.Kind.Get(id)
		def, ok := g.Content.Entities.Get(kind)
		if !ok || g.Held(id) {
			continue // its mod was removed (it stays in the save), or it is being moved
		}
		models := gfx.entityModels(def)
		pos, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
		lit := float32(mesh.LitSun)
		if base.Indoors(base.ToLocal(pos)) {
			lit = mesh.LitIndoor
		}
		gfx.setLighting(lit, vec3{})
		for _, m := range models {
			gfx.setMat(m.tok.Mat, m.tok.Scale, false)
			gfx.putBasis(m.model, pos, ax, up, az, m.tok.Col, m.tok.Spec, m.tok.Emissive)
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

// entityModels are an entity's models, built on first use.
func (gfx *Graphics) entityModels(def *content.EntityDef) []entityModel {
	models, ok := gfx.entities[def.ID]
	if !ok {
		models = buildEntity(def, gfx)
		gfx.entities[def.ID] = models
	}
	return models
}

// drawGhost draws the item being placed see-through, glowing cyan where
// it fits, red where it doesn't. Last, over the scene, not hiding it.
func (gfx *Graphics) drawGhost(g *game.Game) {
	gh := g.Ghost
	if gh == nil {
		return
	}
	def, ok := g.Content.Entities.Get(gh.Kind)
	if !ok {
		return
	}
	col := color.RGBA{110, 235, 245, 90}
	if !gh.OK {
		col = color.RGBA{255, 90, 70, 100}
	}
	pos, ax, up, az := g.Frame(&gh.Pose)
	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()
	gfx.setLighting(mesh.LitSun, vec3{})
	gfx.setMat(content.MtPlain, 1, false)
	for _, m := range gfx.entityModels(def) {
		gfx.putBasis(m.model, pos, ax, up, az, col, 0, 0.5)
	}
	rl.DrawRenderBatchActive()
	rl.EnableDepthMask()
}

// buildEntity merges an entity's parts into one model per material, in
// its own frame.
func buildEntity(def *content.EntityDef, gfx *Graphics) []entityModel {
	b := mesh.NewParts(vec3{})
	content.KitJSON{Parts: def.Parts}.Kit().Build(b, vec3{}, 0)
	var out []entityModel
	for _, k := range b.Keys {
		m := b.Mesh(k)
		if len(m.V) == 0 {
			continue
		}
		model := upload(m, gfx.objectSh)
		gfx.bindMaps(model)
		out = append(out, entityModel{model, gfx.tok(k.Tok)})
	}
	return out
}
