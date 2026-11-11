package render

import (
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/planet"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	bloomLevels   = 6
	bloomStrength = 0.09
	baseExposure  = 1.35 // the exposure the shaders used to apply themselves
	// raylib's PIXELFORMAT_UNCOMPRESSED_R16G16B16A16, missing from raylib-go.
	pixelRGBA16F rl.PixelFormat = 13
)

// postFX renders the 3D scene in HDR into a float texture, then makes the
// image: bloom, exposure, tone curve, color grading, vignette, grain and
// FXAA (MSAA is lost when rendering into a texture).
type postFX struct {
	w, h          int32
	scene         rl.RenderTexture2D
	bloom         [bloomLevels]rl.RenderTexture2D
	ldr           rl.RenderTexture2D // the graded image, luma in alpha, before FXAA
	down, up, img rl.Shader
	fxaa          rl.Shader
	locs          struct{ texel, first, exposure, sunUp, time, sunScreen int32 }
	exposure      float64 // adapts over time, like an eye
}

// hdrTarget makes a half-float render texture, with a depth buffer if asked.
// raylib-go has no rlLoadTexture: the texture comes from a converted image.
func hdrTarget(w, h int32, depth bool) rl.RenderTexture2D {
	img := rl.GenImageColor(int(w), int(h), rl.Black)
	rl.ImageFormat(img, pixelRGBA16F)
	rt := rl.RenderTexture2D{Texture: rl.LoadTextureFromImage(img)}
	rl.UnloadImage(img)
	rl.SetTextureFilter(rt.Texture, rl.FilterBilinear)
	rl.SetTextureWrap(rt.Texture, rl.WrapClamp)
	rt.ID = rl.LoadFramebuffer()
	rl.FramebufferAttach(rt.ID, rt.Texture.ID, rl.AttachmentColorChannel0, rl.AttachmentTexture2d, 0)
	if depth {
		rt.Depth = rl.Texture2D{ID: rl.LoadTextureDepth(w, h, true), Width: w, Height: h}
		rl.FramebufferAttach(rt.ID, rt.Depth.ID, rl.AttachmentDepth, rl.AttachmentRenderbuffer, 0)
	}
	if !rl.FramebufferComplete(rt.ID) {
		panic("HDR framebuffer incomplete")
	}
	rl.DisableFramebuffer()
	return rt
}

func newPostFX() *postFX {
	p := &postFX{exposure: baseExposure}
	p.down = rl.LoadShaderFromMemory(glsl("post.vert"), glsl("bloom_down.frag"))
	p.up = rl.LoadShaderFromMemory(glsl("post.vert"), glsl("bloom_up.frag"))
	p.img = rl.LoadShaderFromMemory(glsl("post.vert"), glsl("image.frag"))
	p.fxaa = rl.LoadShaderFromMemory(glsl("post.vert"), glsl("fxaa.frag"))
	p.locs.texel = rl.GetShaderLocation(p.down, "texel")
	p.locs.first = rl.GetShaderLocation(p.down, "first")
	p.locs.exposure = rl.GetShaderLocation(p.img, "exposure")
	p.locs.sunUp = rl.GetShaderLocation(p.img, "sunUp")
	p.locs.time = rl.GetShaderLocation(p.img, "time")
	p.locs.sunScreen = rl.GetShaderLocation(p.img, "sunScreen")
	return p
}

// resize (re)creates the targets when the window changes size.
func (p *postFX) resize() {
	w, h := int32(rl.GetRenderWidth()), int32(rl.GetRenderHeight())
	if w == p.w && h == p.h {
		return
	}
	if p.w > 0 {
		rl.UnloadRenderTexture(p.scene)
		for _, b := range p.bloom {
			rl.UnloadRenderTexture(b)
		}
		rl.UnloadRenderTexture(p.ldr)
	}
	p.w, p.h = w, h
	p.scene = hdrTarget(w, h, true)
	p.ldr = rl.LoadRenderTexture(w, h)
	rl.SetTextureFilter(p.ldr.Texture, rl.FilterBilinear)
	rl.SetTextureWrap(p.ldr.Texture, rl.WrapClamp)
	for i := range p.bloom {
		w, h = max(1, w/2), max(1, h/2)
		p.bloom[i] = hdrTarget(w, h, false)
	}
}

// adapt moves the exposure toward what the scene needs: brighter at night,
// and higher indoors, so stepping out into the day dazzles for a moment.
func (p *postFX) adapt(dt, sunUp float64, inside bool) {
	target := baseExposure * (2.6 - 1.6*mathx.Smoothstep(-0.15, 0.2, sunUp))
	if inside {
		target = baseExposure * 1.25
	}
	// The eye opens slowly in the dark, closes fast in bright light.
	rate := 0.7
	if target < p.exposure {
		rate = 2.5
	}
	p.exposure += (target - p.exposure) * (1 - math.Exp(-dt*rate))
}

// blit draws src stretched over the current target (render textures are
// stored upside down, hence the negative height).
func blit(src rl.Texture2D, w, h int32) {
	rl.DrawTexturePro(src, rl.NewRectangle(0, 0, float32(src.Width), -float32(src.Height)),
		rl.NewRectangle(0, 0, float32(w), float32(h)), rl.Vector2{}, 0, rl.White)
}

// finish turns the HDR scene into the final image on the screen.
func (p *postFX) finish(cam rl.Camera3D, sunUp, clock float64) {
	// Bloom: halve again and again, then add each level back into the one
	// above, blurring as it grows.
	rl.BeginShaderMode(p.down)
	src := p.scene.Texture
	for i, dst := range p.bloom {
		rl.SetShaderValue(p.down, p.locs.texel, []float32{1 / float32(src.Width), 1 / float32(src.Height)}, rl.ShaderUniformVec2)
		rl.SetShaderValue(p.down, p.locs.first, []float32{b2f(i == 0)}, rl.ShaderUniformFloat)
		rl.BeginTextureMode(dst)
		blit(src, dst.Texture.Width, dst.Texture.Height)
		rl.EndTextureMode()
		src = dst.Texture
	}
	rl.EndShaderMode()
	rl.BeginShaderMode(p.up)
	rl.BeginBlendMode(rl.BlendAdditive)
	for i := bloomLevels - 1; i > 0; i-- {
		dst := p.bloom[i-1]
		rl.BeginTextureMode(dst)
		blit(p.bloom[i].Texture, dst.Texture.Width, dst.Texture.Height)
		rl.EndTextureMode()
	}
	rl.EndBlendMode()
	rl.EndShaderMode()

	rl.SetShaderValue(p.img, p.locs.exposure, []float32{float32(p.exposure)}, rl.ShaderUniformFloat)
	rl.SetShaderValue(p.img, p.locs.sunUp, []float32{float32(sunUp)}, rl.ShaderUniformFloat)
	rl.SetShaderValue(p.img, p.locs.time, []float32{float32(math.Mod(clock, 100))}, rl.ShaderUniformFloat)
	// Light shafts when the sun is low and ahead.
	eye := vec3{float64(cam.Position.X), float64(cam.Position.Y), float64(cam.Position.Z)}
	fwd := vec3{float64(cam.Target.X), float64(cam.Target.Y), float64(cam.Target.Z)}.Sub(eye).Norm()
	shafts := mathx.Smoothstep(0.3, 0.85, fwd.Dot(planet.SunDir)) * mathx.Smoothstep(-0.12, 0.0, sunUp) * (1 - mathx.Smoothstep(0.15, 0.45, sunUp))
	sp := rl.GetWorldToScreen(ToRL(eye.Add(planet.SunDir.Scale(1000))), cam)
	sw, sh := float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())
	rl.SetShaderValue(p.img, p.locs.sunScreen, []float32{sp.X / sw, 1 - sp.Y/sh, float32(shafts * 0.2)}, rl.ShaderUniformVec3)
	// Written as is, without blending: the alpha holds the luma for FXAA,
	// and alpha blending would mix in what the last frame left there,
	// trailing everything that moves.
	rl.BeginTextureMode(p.ldr)
	rl.BeginShaderMode(p.img)
	rl.SetBlendFactors(glOne, glZero, glAdd)
	rl.BeginBlendMode(rl.BlendCustom)
	rl.SetShaderValueTexture(p.img, rl.GetShaderLocation(p.img, "bloom"), p.bloom[0].Texture)
	blit(p.scene.Texture, p.ldr.Texture.Width, p.ldr.Texture.Height)
	rl.EndBlendMode()
	rl.EndShaderMode()
	rl.EndTextureMode()
	// Then the edges are smoothed, straight to the screen.
	rl.SetShaderValue(p.fxaa, rl.GetShaderLocation(p.fxaa, "time"), []float32{float32(math.Mod(clock, 100))}, rl.ShaderUniformFloat)
	rl.BeginShaderMode(p.fxaa)
	blit(p.ldr.Texture, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()))
	rl.EndShaderMode()
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
