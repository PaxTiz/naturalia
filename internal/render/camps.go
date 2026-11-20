package render

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/game"
	"naturalia/internal/mesh"
	"naturalia/internal/palette"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The bootcamps are drawn from their layout's meshes, built once per kind
// in its own frame and stood where each one stands (base.Site).

// campDrawDist is how far the bootcamps are drawn: a small dome, its
// beacon seen from afar.
const campDrawDist = 1500.0

// campModels are a kind of bootcamp's meshes, uploaded on first use.
func (gfx *Graphics) campModels(kind string, l *base.Camp) *baseModels {
	if gfx.camps == nil {
		gfx.camps = map[string]*baseModels{}
	}
	m := gfx.camps[kind]
	if m == nil {
		m = gfx.uploadBase(l.Parts, gfx.objectSh)
		for _, md := range m.models {
			gfx.bindMaps(md)
		}
		gfx.camps[kind] = m
	}
	return m
}

// drawCamps draws the bootcamps around the camera: their meshes, their
// door's leaves, their beacon.
func (gfx *Graphics) drawCamps(g *game.Game) {
	rl.DisableBackfaceCulling()
	defer rl.EnableBackfaceCulling()
	for _, c := range g.CampsNear(gfx.camEye, campDrawDist) {
		m := gfx.campModels(g.Kind(c.Ent), c.Layout)
		s := c.Site
		rl.SetShaderValue(gfx.objectSh, gfx.oLoc.wear, []float32{1, float32(s.Origin.Dot(s.Y))}, rl.ShaderUniformVec2)
		rl.SetShaderValue(gfx.objectSh, gfx.oLoc.wearUp, v3f(s.Y), rl.ShaderUniformVec3)
		frame := basisMatrix(s.Origin, s.X, s.Y, s.Z)
		for i, k := range m.Keys {
			if k.Lit == mesh.LitGlass {
				continue
			}
			t := m.toks[i]
			room := k.At
			if room != (vec3{}) {
				room = s.ToWorld(room)
			}
			gfx.setMat(t.Mat, t.Scale, false)
			gfx.setLighting(k.Lit, room)
			rl.SetShaderValue(gfx.objectSh, gfx.oLoc.surface, []float32{t.Spec, t.Emissive}, rl.ShaderUniformVec2)
			mat := m.models[i].GetMaterials()[0]
			mat.GetMap(rl.MapAlbedo).Color = t.Col
			rl.DrawMesh(m.models[i].GetMeshes()[0], mat, frame)
			mat.GetMap(rl.MapAlbedo).Color = rl.White
		}
		gfx.drawCampDoor(g, c)
		if math.Mod(g.Elapsed+float64(c.ID)*0.37, 1.2) < 0.3 {
			gfx.setLighting(mesh.LitSun, vec3{})
			gfx.setMat(content.MtPlain, 1, false)
			gfx.putBasis(gfx.sphere, c.ToWorld(c.Layout.Beacon), s.X.Scale(0.09), s.Y.Scale(0.09), s.Z.Scale(0.09), palette.Red, 0, 1)
		}
	}
	gfx.setMat(content.MtPlain, 1, false)
	gfx.setLighting(mesh.LitSun, vec3{})
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.wear, []float32{0, 0}, rl.ShaderUniformVec2)
}

// drawCampDoor draws the two leaves of a bootcamp's door, sliding apart as
// the player comes close, and its light.
func (gfx *Graphics) drawCampDoor(g *game.Game, c game.CampAt) {
	s := c.Site
	end, across := c.Layout.Door()
	open := g.CampDoor(c.Ent)
	steel := gfx.tok("steel")
	gfx.setLighting(mesh.LitSun, vec3{})
	gfx.setMat(steel.Mat, steel.Scale, false)
	w := (base.DoorWidth + 0.2) / 2
	ax := s.Dir(across) // the leaf's width, along the door
	az := ax.Cross(s.Y)
	for _, side := range []float64{-1, 1} {
		at := end.Add(across.Scale(side * (w/2 + open*w))).Add(vec3{0, base.DoorHeight / 2, 0})
		gfx.putBasis(gfx.doorPanel, s.ToWorld(at), ax, s.Y, az, steel.Col, steel.Spec, 0)
	}
	gfx.setMat(content.MtPlain, 1, false)
	lamp := palette.Green
	if open < 0.9 {
		lamp = palette.Gold
	}
	out := vec3{0, 1, 0}.Cross(across) // out of the door
	at := end.Add(vec3{0, 2.6, 0}).Add(out.Scale(0.2))
	gfx.putBasis(gfx.cube, s.ToWorld(at), ax.Scale(1.2), s.Y.Scale(0.08), az.Scale(0.04), lamp, 0, 1)
}
