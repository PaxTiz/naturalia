package render

import (
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
		up := e.Dir
		pos := g.World.Relief.Surface(e.Dir).Add(up.Scale(e.Alt))
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
		if v, ok := g.Content.Vehicles.Get(e.Kind); ok && v.Light != (vec3{}) && math.Sin(g.Elapsed*6+float64(i)) > 0 {
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
