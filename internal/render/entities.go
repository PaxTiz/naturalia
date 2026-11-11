package render

import (
	"mars_sweeper/internal/base"
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/game"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/mesh"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type entityModel struct {
	model rl.Model
	tok   content.Token
}

// drawEntities draws the entities around the player, standing up from the
// ground.
func (gfx *Graphics) drawEntities(g *game.Game) {
	for _, i := range g.EntitiesNear(game.EntityDrawDist) {
		e := g.State.Entities[i]
		def, ok := g.Content.Entities.Get(e.Kind)
		if !ok {
			continue // its mod was removed: it stays in the save
		}
		models, ok := gfx.entities[def.ID]
		if !ok {
			models = buildEntity(def, gfx)
			gfx.entities[def.ID] = models
		}
		pos := g.World.Relief.Surface(e.Dir)
		up := e.Dir
		t1, t2 := mathx.TangentBasis(up)
		ax := t1.Scale(math.Cos(e.Yaw)).Add(t2.Scale(math.Sin(e.Yaw)))
		az := ax.Cross(up)
		lit := float32(mesh.LitSun)
		if base.Indoors(base.ToLocal(pos)) {
			lit = mesh.LitIndoor
		}
		gfx.setLighting(lit, vec3{})
		for _, m := range models {
			gfx.setMat(m.tok.Mat, m.tok.Scale, false)
			gfx.putBasis(m.model, pos, ax, up, az, m.tok.Col, m.tok.Spec, m.tok.Emissive)
		}
	}
	gfx.setMat(content.MtPlain, 1, false)
	gfx.setLighting(mesh.LitSun, vec3{})
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
