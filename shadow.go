package main

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	shadowRes    = 2048
	shadowRadius = 80.0  // meters covered around the player
	shadowDepth  = 600.0 // the light camera's depth range
	// The terrain material's slot for the map: raylib's MATERIAL_MAP_BRDF.
	// raylib-go's MapBrdf is 11, shifted by a stray MapBrdg in its enum.
	mapShadow = 10
)

// shadowMap renders the objects around the player as seen from the sun,
// keeping only their depth: the terrain then knows what stands between it
// and the sun. The relief's own shadow is baked in the terrain (lod.go),
// so only objects are drawn here.
//
// The objects write their depth themselves into a float color texture,
// blended with "min" to keep the nearest.
type shadowMap struct {
	target    rl.RenderTexture2D
	eye, look vec3 // the light's camera, kept in double for lightCols
	up        vec3
}

func newShadowMap() *shadowMap {
	img := rl.GenImageColor(shadowRes, shadowRes, rl.White)
	rl.ImageFormat(img, rl.UncompressedR32)
	s := &shadowMap{}
	s.target.Texture = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	s.target.ID = rl.LoadFramebuffer()
	rl.FramebufferAttach(s.target.ID, s.target.Texture.ID, rl.AttachmentColorChannel0, rl.AttachmentTexture2d, 0)
	if !rl.FramebufferComplete(s.target.ID) {
		panic("shadow framebuffer incomplete")
	}
	rl.DisableFramebuffer()
	return s
}

// camera looks down the sun's rays at center, which is snapped to the
// map's texels so the shadows don't shimmer as the player walks.
func (s *shadowMap) camera(center vec3) rl.Camera3D {
	ref := vec3{0, 1, 0}
	if math.Abs(sunDir.Y) > 0.9 {
		ref = vec3{1, 0, 0}
	}
	right := sunDir.Cross(ref).Norm()
	up := right.Cross(sunDir).Norm()
	texel := 2 * shadowRadius / shadowRes
	snap := func(v float64) float64 { return math.Floor(v/texel) * texel }
	x, y, z := center.Dot(right), center.Dot(up), center.Dot(sunDir)
	center = right.Scale(snap(x)).Add(up.Scale(snap(y))).Add(sunDir.Scale(z))
	s.eye, s.look, s.up = center.Add(sunDir.Scale(shadowDepth/2)), center, up
	return rl.Camera3D{
		Position:   toRL(center.Add(sunDir.Scale(shadowDepth / 2))),
		Target:     toRL(center),
		Up:         toRL(up),
		Fovy:       2 * shadowRadius,
		Projection: rl.CameraOrthographic,
	}
}

const glMin, glAdd, glOne, glZero = 0x8007, 0x8006, 1, 0

// cols gives the sun's camera, as BeginMode3D sets it, but about origin o:
// the shaders look the shadows up from fragRel. From the world position,
// some 3 km from Mars' center, float precision is a quarter millimeter, a
// grain that shifts as the camera moves and makes the shadows crawl.
func (s *shadowMap) cols(o vec3) []float32 {
	proj := rl.MatrixOrtho(-shadowRadius, shadowRadius, -shadowRadius, shadowRadius, 1, shadowDepth)
	return relVP(s.eye, s.look, s.up, o, proj)
}

// relVP is a camera's view and projection about o, by columns: passing a
// Matrix by value is unreliable without cgo.
func relVP(eye, look, up, o vec3, proj rl.Matrix) []float32 {
	view := rl.MatrixLookAt(toRL(eye.Sub(o)), toRL(look.Sub(o)), toRL(up))
	m := rl.MatrixMultiply(view, proj)
	return []float32{m.M0, m.M1, m.M2, m.M3, m.M4, m.M5, m.M6, m.M7, m.M8, m.M9, m.M10, m.M11, m.M12, m.M13, m.M14, m.M15}
}

// render draws the casters into the map and keeps the light's matrix.
// The object shader is switched to writing depth (shadowPass) meanwhile.
func (s *shadowMap) render(gfx *Graphics, center vec3, draw func()) {
	rl.BeginTextureMode(s.target)
	rl.ClearBackground(rl.White)
	rl.SetClipPlanes(1, shadowDepth)
	rl.BeginMode3D(s.camera(center))
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.shadowPass, []float32{1}, rl.ShaderUniformFloat)
	rl.SetBlendFactors(glOne, glOne, glMin)
	rl.BeginBlendMode(rl.BlendCustom)
	rl.DisableBackfaceCulling() // thin and open meshes cast too
	draw()
	rl.EnableBackfaceCulling()
	rl.EndBlendMode()
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.shadowPass, []float32{0}, rl.ShaderUniformFloat)
	rl.EndMode3D()
	rl.SetClipPlanes(0.05, 9000)
	rl.EndTextureMode()
}

// lampShadow holds the shadows of the rooms' ceiling lamps: one wide
// perspective view straight down from each lamp, in the cells of a grid
// on one texture (2×2 for the core base). The base never moves: they are drawn once. Every room keeps
// its shadows, so nothing pops in when the player walks in.
type lampShadow struct {
	target rl.RenderTexture2D
	lamps  []vec3 // where the cameras stand, looking down
	grid   int    // cells per side
	done   bool
}

const (
	lampShadowCell = 1024 // each lamp's view; the rooms' lamps are the first lights of baseLights
	lampFov        = 150.0
)

func newLampShadow(lamps int) *lampShadow {
	s := &lampShadow{grid: int(math.Ceil(math.Sqrt(float64(max(1, lamps)))))}
	res := s.grid * lampShadowCell
	img := rl.GenImageColor(res, res, rl.White)
	rl.ImageFormat(img, rl.UncompressedR32)
	s.target.Texture = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.SetTextureFilter(s.target.Texture, rl.FilterPoint)
	rl.SetTextureWrap(s.target.Texture, rl.WrapClamp)
	s.target.ID = rl.LoadFramebuffer()
	rl.FramebufferAttach(s.target.ID, s.target.Texture.ID, rl.AttachmentColorChannel0, rl.AttachmentTexture2d, 0)
	if !rl.FramebufferComplete(s.target.ID) {
		panic("lamp shadow framebuffer incomplete")
	}
	rl.DisableFramebuffer()
	return s
}

// cols gives the lamps' cameras about origin o, four columns each (see
// shadowMap.cols).
func (s *lampShadow) cols(o vec3) []float32 {
	proj := rl.MatrixPerspective(lampFov*math.Pi/180, 1, 0.1, 30)
	var c []float32
	for _, l := range s.lamps {
		c = append(c, relVP(l, l.Sub(vec3{0, 1, 0}), vec3{1, 0, 0}, o, proj)...)
	}
	return c
}

// render draws every lamp's view into its quarter, once.
func (s *lampShadow) render(gfx *Graphics, lamps []vec3, draw func()) {
	if s.done {
		return
	}
	s.done = true
	cell := int32(lampShadowCell)
	grid := int32(s.grid)
	rl.BeginTextureMode(s.target)
	rl.ClearBackground(rl.White)
	rl.SetClipPlanes(0.1, 30)
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.shadowPass, []float32{1}, rl.ShaderUniformFloat)
	for i, lamp := range lamps[:min(len(lamps), s.grid*s.grid)] {
		cam := rl.Camera3D{
			Position: toRL(lamp), Target: toRL(lamp.Sub(vec3{0, 1, 0})), Up: rl.Vector3{X: 1},
			Fovy: lampFov, Projection: rl.CameraPerspective,
		}
		rl.BeginMode3D(cam)
		rl.Viewport(int32(i)%grid*cell, int32(i)/grid*cell, cell, cell)
		s.lamps = append(s.lamps, lamp)
		rl.SetBlendFactors(glOne, glOne, glMin)
		rl.BeginBlendMode(rl.BlendCustom)
		rl.DisableBackfaceCulling()
		draw()
		rl.EnableBackfaceCulling()
		rl.EndBlendMode()
		rl.EndMode3D()
	}
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.shadowPass, []float32{0}, rl.ShaderUniformFloat)
	rl.SetClipPlanes(0.05, 9000)
	rl.EndTextureMode()
}
