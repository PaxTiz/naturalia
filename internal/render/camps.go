package render

import (
	"cmp"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/game"
	"naturalia/internal/mesh"
	"naturalia/internal/palette"
	"slices"
	"strings"

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

// campRoof is how far above its floor a building may reach.
const campRoof = 10.0

// campSeen tells whether a building may be in a picture.
func campSeen(c game.CampAt, seen sees) bool {
	return seen(c.ToWorld(vec3{0, campRoof / 2, 0}), math.Hypot(c.Layout.Reach, campRoof/2))
}

// campFrame places a building's meshes.
func (gfx *Graphics) campFrame(s base.Site, m *baseModels) rl.Matrix {
	return basisMatrix(s.Origin.Add(s.Dir(m.Origin())).Sub(gfx.origin), s.X, s.Y, s.Z) // about the origin (object.vert)
}

// drawCampGlass draws the buildings' panes over the scene, as drawGlass
// the base's, the farthest first.
func (gfx *Graphics) drawCampGlass(g *game.Game, seen sees) {
	list := g.CampsNear(gfx.camEye, campDrawDist)
	dist := func(c game.CampAt) float64 { return c.Site.Origin.Sub(gfx.camEye).Length() }
	slices.SortFunc(list, func(a, b game.CampAt) int { return cmp.Compare(dist(b), dist(a)) })
	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()
	gfx.setMat(content.MtPlain, 1, false)
	for _, c := range list {
		if !campSeen(c, seen) {
			continue
		}
		m := gfx.campModels(g.Kind(c.Ent), c.Layout)
		frame := gfx.campFrame(c.Site, m)
		for i, k := range m.Keys {
			if k.Lit != mesh.LitGlass {
				continue
			}
			room := k.At
			if room != (vec3{}) {
				room = c.ToWorld(room)
			}
			t := m.toks[i]
			gfx.setLighting(k.Lit, room)
			rl.SetShaderValue(gfx.objectSh, gfx.oLoc.surface, []float32{t.Spec, t.Emissive}, rl.ShaderUniformVec2)
			mat := m.models[i].GetMaterials()[0]
			mat.GetMap(rl.MapAlbedo).Color = t.Col
			rl.DrawMesh(m.models[i].GetMeshes()[0], mat, frame)
			mat.GetMap(rl.MapAlbedo).Color = rl.White
		}
	}
	rl.DrawRenderBatchActive()
	rl.EnableDepthMask()
	gfx.setLighting(mesh.LitSun, vec3{})
}

// drawCamps draws the bootcamps around the camera seen: their meshes,
// their doors' leaves, their beacon.
func (gfx *Graphics) drawCamps(g *game.Game, seen sees) {
	rl.DisableBackfaceCulling()
	defer rl.EnableBackfaceCulling()
	for _, c := range g.CampsNear(gfx.camEye, campDrawDist) {
		if !campSeen(c, seen) {
			continue
		}
		m := gfx.campModels(g.Kind(c.Ent), c.Layout)
		s := c.Site
		rl.SetShaderValue(gfx.objectSh, gfx.oLoc.wear, []float32{1, float32(s.Origin.Dot(s.Y))}, rl.ShaderUniformVec2)
		rl.SetShaderValue(gfx.objectSh, gfx.oLoc.wearUp, v3f(s.Y), rl.ShaderUniformVec3)
		frame := gfx.campFrame(s, m)
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
		gfx.site = s
		for _, f := range c.Layout.Furniture {
			if draw := kitDraws[f.Kit]; draw != nil {
				draw(gfx, g, kitSpot{f.At, f.Yaw})
			}
		}
		if _, _, ok := c.Layout.BigDoor(); ok {
			gfx.drawLeaves(c.Layout.Room, c.Layout.BigDoorLeaves(g.BigDoor(c.Ent)))
		}
		gfx.site = base.Site{}
		for i := range c.Layout.Ports {
			gfx.drawCampDoor(g, c, i)
		}
		if c.Layout.Hologram() {
			gfx.drawCampSign(c)
		}
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

// drawCampDoor draws the two leaves of the door of a bootcamp's port i,
// sliding apart as the player comes close, and its light.
func (gfx *Graphics) drawCampDoor(g *game.Game, c game.CampAt, i int) {
	s := c.Site
	port := c.Layout.Ports[i]
	end, across, out := port.At(), port.Across(), port.Out()
	open := g.CampDoor(c.Ent, i)
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
	at := end.Add(vec3{0, 2.6, 0}).Add(out.Scale(0.2))
	gfx.putBasis(gfx.cube, s.ToWorld(at), ax.Scale(1.2), s.Y.Scale(0.08), az.Scale(0.04), lamp, 0, 1)
}

// campSign is the texture painted with a bootcamp's name, kept until it
// is renamed.
type campSign struct {
	name string
	tex  rl.Texture2D
}

const campSignW, campSignH = 2.4, 0.45

// drawCampSign draws a bootcamp's name on the sign standing on its door's
// header, facing out.
func (gfx *Graphics) drawCampSign(c game.CampAt) {
	if gfx.campSigns == nil {
		gfx.campSigns = map[ecs.ID]campSign{}
	}
	sg, ok := gfx.campSigns[c.Ent]
	if !ok || sg.name != c.Name {
		if ok {
			rl.UnloadTexture(sg.tex)
		}
		sg = campSign{c.Name, paintSign(sign{width: campSignW, height: campSignH, text: strings.ToUpper(c.Name), accent: palette.AccentCyan}, gfx.smallcaps)}
		gfx.campSigns[c.Ent] = sg
	}
	s := c.Site
	gfx.setMat(content.MtDecal, 1, false)
	gfx.setLighting(mesh.LitSun, vec3{})
	rl.SetMaterialTexture(&gfx.signBox.GetMaterials()[0], rl.MapAlbedo, sg.tex)
	// The box's front (+z) shows the texture: turned to face out.
	port := c.Layout.Ports[0]
	gfx.putBasis(gfx.signBox, c.ToWorld(c.Layout.Sign), s.Dir(port.Across().Scale(-campSignW)), s.Y.Scale(campSignH), s.Dir(port.Out()).Scale(0.06), rl.White, 0.2, 0.25)
	gfx.setMat(content.MtPlain, 1, false)
}
