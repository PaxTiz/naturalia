package render

import (
	"image/color"
	"mars_sweeper/internal/base"
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/game"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/mesh"
	"mars_sweeper/internal/palette"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// setLighting selects how the next objects are lit; room is the world point
// that litTowards walls face.
func (gfx *Graphics) setLighting(mode float32, room vec3) {
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.lighting, []float32{mode}, rl.ShaderUniformFloat)
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.room, v3f(room), rl.ShaderUniformVec3)
}

// yawFor returns the rotation around Y (radians) that turns a model's +X
// axis toward the horizontal direction dir.
func yawFor(dir vec3) float64 { return math.Atan2(-dir.Z, dir.X) }

// box draws a cube centered at the local point c, turned so its width (X)
// runs along the horizontal direction across.
func (gfx *Graphics) box(c, size, across vec3, col color.RGBA, spec, emissive float32) {
	gfx.put(gfx.cube, base.ToWorld(c), size, vec3{0, 1, 0}, yawFor(across)*180/math.Pi, col, spec, emissive)
}

// kitDraws draw the moving and glowing parts of kit pieces, each frame.
var kitDraws = map[string]func(gfx *Graphics, g *game.Game, at vec3){
	"core:incinerator": drawIncinerator,
	"core:console":     drawConsole,
	"core:bed":         drawBed,
}

func (gfx *Graphics) drawBase(g *game.Game) {
	rl.DisableBackfaceCulling()
	defer rl.EnableBackfaceCulling()
	if gfx.base == nil {
		gfx.base = gfx.uploadBase(base.Build(&g.Content.Kits), gfx.objectSh)
		gfx.doorPanel = doorPanelModel(gfx.objectSh)
		for _, m := range append(gfx.base.models, gfx.doorPanel) {
			gfx.bindMaps(m)
		}
	}

	// Red dust settles on the faces turned up and at the foot of the walls.
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.wear, []float32{1, float32(base.Origin.Y)}, rl.ShaderUniformVec2)
	gfx.base.draw(gfx)
	for _, bp := range base.Plans {
		for _, p := range bp.Props {
			if draw := kitDraws[content.Qualify(p.Kit)]; draw != nil {
				draw(gfx, g, bp.At.Add(p.At))
			}
		}
	}

	gfx.drawAirlock(g)
	gfx.drawBeacon(g)
	gfx.drawSigns(g)
	gfx.setMat(content.MtPlain, 1, false)
	gfx.setLighting(mesh.LitSun, vec3{})
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.wear, []float32{0, 0}, rl.ShaderUniformVec2)
}

// drawAirlock draws the moving parts of the outer door: two leaves sliding
// apart when the player comes close, and the status light.
func (gfx *Graphics) drawAirlock(g *game.Game) {
	end := base.AirlockEnd
	gfx.setLighting(mesh.LitSun, vec3{})
	steel := gfx.tok("steel")
	gfx.setMat(steel.Mat, steel.Scale, false)
	w := (base.DoorWidth + 0.2) / 2
	for _, s := range []float64{-1, 1} {
		at := end.Add(vec3{s * (w/2 + g.DoorOpen*w), base.DoorHeight / 2, 0})
		gfx.put(gfx.doorPanel, base.ToWorld(at), vec3{1, 1, 1}, vec3{0, 1, 0}, 0, steel.Col, steel.Spec, 0)
	}
	gfx.setMat(content.MtPlain, 1, false)
	lamp := palette.Green
	if g.DoorOpen < 0.9 {
		lamp = palette.Gold
	}
	gfx.box(end.Add(vec3{0, 2.6, -0.2}), vec3{1.2, 0.08, 0.04}, vec3{1, 0, 0}, lamp, 0, 1)
}

// drawBeacon blinks the red light on top of the hub's antenna.
func (gfx *Graphics) drawBeacon(g *game.Game) {
	if math.Mod(g.Elapsed, 1.2) < 0.3 {
		gfx.setLighting(mesh.LitSun, vec3{})
		gfx.setMat(content.MtPlain, 1, false)
		gfx.put(gfx.sphere, beaconTop(), vec3{0.09, 0.09, 0.09}, vec3{0, 1, 0}, 0, palette.Red, 0, 1)
	}
}

// beaconTop is where the blinking light sits, for its glow.
func beaconTop() vec3 {
	return base.ToWorld(vec3{1.5, base.HubRoom.WallH + base.HubRoom.Radius*0.55 + 2.25, 1.5})
}

// drawGlobe draws the hologram of Mars above the map table.
func (gfx *Graphics) drawGlobe(g *game.Game, cam rl.Camera3D) {
	center := base.ToWorld(base.TableSpot.Add(vec3{0, globeHeight, 0}))
	rl.SetShaderValue(gfx.holoSh, gfx.hLoc.globeCenter, v3f(center), rl.ShaderUniformVec3)
	// Turn the player's spot on Mars toward the viewer, a bit raised,
	// with a slow sway so the hologram feels alive.
	toViewer := g.Player.Eye().Sub(center)
	toViewer = vec3{toViewer.X, 0, toViewer.Z}.Norm()
	sway := math.Sin(g.Elapsed*0.35) * 0.45
	toViewer = vec3{
		toViewer.X*math.Cos(sway) + toViewer.Z*math.Sin(sway), 0,
		-toViewer.X*math.Sin(sway) + toViewer.Z*math.Cos(sway),
	}
	facing := toViewer.Add(vec3{0, 0.2, 0}).Norm()
	rl.SetShaderValueMatrix(gfx.holoSh, gfx.hLoc.globeRot, rotationBetween(facing, g.Player.LocalUp()))
	rl.SetShaderValue(gfx.holoSh, gfx.hLoc.playerDir, v3f(g.Player.LocalUp()), rl.ShaderUniformVec3)
	rl.SetMaterialTexture(&gfx.holo.GetMaterials()[0], rl.MapDiffuse, gfx.dust.Coverage())

	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()
	rl.BeginBlendMode(rl.BlendAdditive)
	rl.DrawModel(gfx.holo, ToRL(center), globeRadius, rl.White)
	// Soft light rising from the table to the globe.
	for i := range 6 {
		f := float64(i) / 5
		p := base.ToWorld(base.TableSpot.Add(vec3{0, 1.0 + f*(globeHeight-1.0), 0}))
		rl.DrawBillboard(cam, gfx.glow, ToRL(p), float32(0.9+f*1.2), color.RGBA{60, 180, 220, uint8(40 - 25*f)})
	}
	rl.EndBlendMode()
	rl.EnableDepthMask()
}

const (
	globeHeight = 2.0
	globeRadius = 0.8
)

// rotationBetween returns the rotation taking unit vector a onto unit
// vector b (Rodrigues' formula), as a raylib matrix.
func rotationBetween(a, b vec3) rl.Matrix {
	k := a.Cross(b)
	c := a.Dot(b)
	if c < -0.9999 {
		// Opposite vectors: half a turn around any perpendicular axis.
		p, _ := mathx.TangentBasis(a)
		k, c = p.Scale(1e-6), -1+1e-12
	}
	f := 1 / (1 + c)
	m := [3][3]float64{
		{c + k.X*k.X*f, k.X*k.Y*f - k.Z, k.X*k.Z*f + k.Y},
		{k.Y*k.X*f + k.Z, c + k.Y*k.Y*f, k.Y*k.Z*f - k.X},
		{k.Z*k.X*f - k.Y, k.Z*k.Y*f + k.X, c + k.Z*k.Z*f},
	}
	return rl.Matrix{
		M0: float32(m[0][0]), M4: float32(m[0][1]), M8: float32(m[0][2]),
		M1: float32(m[1][0]), M5: float32(m[1][1]), M9: float32(m[1][2]),
		M2: float32(m[2][0]), M6: float32(m[2][1]), M10: float32(m[2][2]),
		M15: 1,
	}
}

// setMat selects the painted material of the next objects: scale is in
// texture tiles per meter; objSpace makes the texture follow a moving object
// (its unit mesh coordinates are used instead of world ones).
func (gfx *Graphics) setMat(kind int, scale float32, objSpace bool) {
	o := float32(0)
	if objSpace {
		o = 1
	}
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.mat, []float32{float32(kind), scale, o}, rl.ShaderUniformVec3)
}

// tok is a material by ID; an unknown one shows up magenta.
func (gfx *Graphics) tok(id string) content.Token {
	if t, ok := gfx.content.Tokens.Get(id); ok {
		return *t
	}
	return content.Token{color.RGBA{255, 0, 255, 255}, content.MtPlain, 1, 0, 0.5}
}

// baseModels are the base's merged meshes, uploaded: one model per piece.
type baseModels struct {
	*mesh.Parts
	models []rl.Model
	toks   []content.Token // of each model, looked up once
}

func (gfx *Graphics) uploadBase(parts *mesh.Parts, sh rl.Shader) *baseModels {
	b := &baseModels{Parts: parts}
	for _, k := range b.Keys {
		b.models = append(b.models, upload(b.Mesh(k), sh))
		b.toks = append(b.toks, gfx.tok(k.Tok))
	}
	return b
}

func (b *baseModels) draw(gfx *Graphics) {
	for i, k := range b.Keys {
		t := b.toks[i]
		gfx.setMat(t.Mat, t.Scale, false)
		gfx.setLighting(k.Lit, k.At)
		rl.SetShaderValue(gfx.objectSh, gfx.oLoc.surface, []float32{t.Spec, t.Emissive}, rl.ShaderUniformVec2)
		rl.DrawModel(b.models[i], rl.Vector3{}, 1, t.Col)
	}
}

// drawIncinerator: the window glows harder while it burns dust.
func drawIncinerator(gfx *Graphics, g *game.Game, in vec3) {
	gfx.setLighting(mesh.LitIndoor, vec3{})
	gfx.setMat(content.MtPlain, 1, false)
	glow := float32(0.55 + 0.45*mathx.Clamp01(g.Burn))
	gfx.box(in.Add(vec3{1.08, 0.95, 0}), vec3{0.8, 0.55, 0.06}, vec3{0, 0, 1}, palette.AccentOrange, 0, glow)
	gfx.drawScreen(in.Add(vec3{1.16, 1.1, 1.25}), vec3{1, 0, 0}, 0.38, 0.28, gfx.screens[3], 0.8)
}

// drawConsole: three screens and the blinking lights of the racks.
func drawConsole(gfx *Graphics, g *game.Game, c vec3) {
	gfx.setLighting(mesh.LitIndoor, vec3{})
	gfx.setMat(content.MtPlain, 1, false)
	for i, z := range []float64{-0.75, 0, 0.75} {
		facing := vec3{-1, 0, z * 0.4}.Norm()
		gfx.drawScreen(c.Add(vec3{0.1, 1.28, z}), facing, 0.66, 0.42, gfx.screens[i], 0.8)
	}
	gfx.setLighting(mesh.LitIndoor, vec3{})
	gfx.setMat(content.MtPlain, 1, false)
	for _, z := range []float64{-2.3, 2.3} {
		for k := range 5 {
			on := float32(0.3)
			if math.Sin(g.Elapsed*3+float64(k)*1.7+z) > 0 {
				on = 1
			}
			gfx.box(c.Add(vec3{-0.36, 0.4 + float64(k)*0.3, z}), vec3{0.5, 0.04, 0.02}, vec3{0, 0, 1}, palette.Green, 0, on)
		}
	}
}

func drawBed(gfx *Graphics, g *game.Game, at vec3) {
	gfx.setLighting(mesh.LitIndoor, vec3{})
	gfx.setMat(content.MtPlain, 1, false)
	gfx.put(gfx.sphere, base.ToWorld(at.Add(vec3{1.45, 0.75, 0.2})), vec3{0.13, 0.13, 0.13}, vec3{0, 1, 0}, 0, palette.LampWarm, 0, 1)
}

// upload sends the triangles to the GPU. The slices stay referenced by the builder:
// the mesh is never unloaded.
func upload(m *mesh.Builder, sh rl.Shader) rl.Model {
	mesh := rl.Mesh{VertexCount: int32(len(m.V) / 3), TriangleCount: int32(len(m.V) / 9)}
	mesh.Vertices, mesh.Normals, mesh.Texcoords = &m.V[0], &m.N[0], &m.UV[0]
	rl.UploadMesh(&mesh, false)
	mod := rl.LoadModelFromMesh(mesh)
	mod.GetMaterials()[0].Shader = sh
	return mod
}

// doorPanelModel is one sliding leaf of the airlock.
func doorPanelModel(sh rl.Shader) rl.Model {
	m := base.DoorPanel()
	keep = append(keep, m)
	return upload(m, sh)
}

// keep holds builders whose slices back GPU meshes.
var keep []*mesh.Builder
