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
	target  rl.RenderTexture2D
	lightVP rl.Matrix
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
	right := sunDir.cross(ref).norm()
	up := right.cross(sunDir).norm()
	texel := 2 * shadowRadius / shadowRes
	snap := func(v float64) float64 { return math.Floor(v/texel) * texel }
	x, y, z := center.dot(right), center.dot(up), center.dot(sunDir)
	center = right.scale(snap(x)).add(up.scale(snap(y))).add(sunDir.scale(z))
	return rl.Camera3D{
		Position:   center.add(sunDir.scale(shadowDepth / 2)).rl(),
		Target:     center.rl(),
		Up:         up.rl(),
		Fovy:       2 * shadowRadius,
		Projection: rl.CameraOrthographic,
	}
}

const glMin, glOne = 0x8007, 1

// render draws the casters into the map and keeps the light's matrix.
// The object shader is switched to writing depth (shadowPass) meanwhile.
func (s *shadowMap) render(gfx *Graphics, center vec3, draw func()) {
	rl.BeginTextureMode(s.target)
	rl.ClearBackground(rl.White)
	rl.SetClipPlanes(1, shadowDepth)
	cam := s.camera(center)
	rl.BeginMode3D(cam)
	// The same matrices BeginMode3D uses, computed here: reading them back
	// from rlgl is unreliable without cgo.
	view := rl.MatrixLookAt(cam.Position, cam.Target, cam.Up)
	proj := rl.MatrixOrtho(-shadowRadius, shadowRadius, -shadowRadius, shadowRadius, 1, shadowDepth)
	s.lightVP = rl.MatrixMultiply(view, proj)
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
