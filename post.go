package main

import (
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
	ao            rl.RenderTexture2D // ambient occlusion, half size
	down, up, img rl.Shader
	aoSh          rl.Shader
	locs          struct{ texel, first, exposure, sunUp, time, sunScreen, aoProj int32 }
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
		// A depth texture, not a renderbuffer: the ambient occlusion reads it.
		rt.Depth = rl.Texture2D{ID: rl.LoadTextureDepth(w, h, false), Width: w, Height: h}
		rl.FramebufferAttach(rt.ID, rt.Depth.ID, rl.AttachmentDepth, rl.AttachmentTexture2d, 0)
	}
	if !rl.FramebufferComplete(rt.ID) {
		panic("HDR framebuffer incomplete")
	}
	rl.DisableFramebuffer()
	return rt
}

func newPostFX() *postFX {
	p := &postFX{exposure: baseExposure}
	p.down = rl.LoadShaderFromMemory(postVS, bloomDownFS)
	p.up = rl.LoadShaderFromMemory(postVS, bloomUpFS)
	p.img = rl.LoadShaderFromMemory(postVS, imageFS)
	p.aoSh = rl.LoadShaderFromMemory(postVS, aoFS)
	p.locs.aoProj = rl.GetShaderLocation(p.aoSh, "proj")
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
		rl.UnloadRenderTexture(p.ao)
	}
	p.w, p.h = w, h
	p.scene = hdrTarget(w, h, true)
	p.ao = hdrTarget(max(1, w/2), max(1, h/2), false)
	for i := range p.bloom {
		w, h = max(1, w/2), max(1, h/2)
		p.bloom[i] = hdrTarget(w, h, false)
	}
}

// adapt moves the exposure toward what the scene needs: brighter at night,
// and higher indoors, so stepping out into the day dazzles for a moment.
func (p *postFX) adapt(dt, sunUp float64, inside bool) {
	target := baseExposure * (2.6 - 1.6*smoothstep(-0.15, 0.2, sunUp))
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
	// Ambient occlusion from the depth: corners, contacts and creases darken.
	tanY := float32(math.Tan(float64(cam.Fovy) * math.Pi / 360))
	rl.SetShaderValue(p.aoSh, p.locs.aoProj, []float32{tanY * float32(p.w) / float32(p.h), tanY, float32(math.Mod(clock, 100)), 0}, rl.ShaderUniformVec4)
	rl.BeginShaderMode(p.aoSh)
	rl.BeginTextureMode(p.ao)
	blit(p.scene.Depth, p.ao.Texture.Width, p.ao.Texture.Height)
	rl.EndTextureMode()
	rl.EndShaderMode()

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
	fwd := vec3{float64(cam.Target.X), float64(cam.Target.Y), float64(cam.Target.Z)}.sub(eye).norm()
	shafts := smoothstep(0.3, 0.85, fwd.dot(sunDir)) * smoothstep(-0.12, 0.0, sunUp) * (1 - smoothstep(0.15, 0.45, sunUp))
	sp := rl.GetWorldToScreen(eye.add(sunDir.scale(1000)).rl(), cam)
	sw, sh := float32(rl.GetScreenWidth()), float32(rl.GetScreenHeight())
	rl.SetShaderValue(p.img, p.locs.sunScreen, []float32{sp.X / sw, 1 - sp.Y/sh, float32(shafts * 0.2)}, rl.ShaderUniformVec3)
	rl.BeginShaderMode(p.img)
	rl.SetShaderValueTexture(p.img, rl.GetShaderLocation(p.img, "bloom"), p.bloom[0].Texture)
	rl.SetShaderValueTexture(p.img, rl.GetShaderLocation(p.img, "aoMap"), p.ao.Texture)
	blit(p.scene.Texture, int32(rl.GetScreenWidth()), int32(rl.GetScreenHeight()))
	rl.EndShaderMode()
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// postVS is raylib's default vertex shader.
const postVS = `#version 330
in vec3 vertexPosition;
in vec2 vertexTexCoord;
uniform mat4 mvp;
out vec2 fragTexCoord;
void main() {
    fragTexCoord = vertexTexCoord;
    gl_Position = mvp * vec4(vertexPosition, 1.0);
}`

// bloomDownFS halves the image with a 13-tap filter that avoids flicker.
// The first pass keeps only what is brighter than white, with a soft knee.
const bloomDownFS = `#version 330
in vec2 fragTexCoord;
uniform sampler2D texture0;
uniform vec2 texel;
uniform float first;
out vec4 finalColor;
vec3 s(vec2 o) { return texture(texture0, fragTexCoord + o * texel).rgb; }
void main() {
    vec3 c = s(vec2(0)) * 0.125;
    c += (s(vec2(-1, -1)) + s(vec2(1, -1)) + s(vec2(-1, 1)) + s(vec2(1, 1))) * 0.125;
    c += (s(vec2(-2, -2)) + s(vec2(2, -2)) + s(vec2(-2, 2)) + s(vec2(2, 2))) * 0.03125;
    c += (s(vec2(-2, 0)) + s(vec2(2, 0)) + s(vec2(0, -2)) + s(vec2(0, 2))) * 0.0625;
    if (first > 0.5) {
        float b = max(c.r, max(c.g, c.b));
        float knee = clamp(b - 0.6, 0.0, 0.8);
        float w = max(knee * knee / 3.2, b - 1.0) / max(b, 1e-4);
        c *= w;
    }
    finalColor = vec4(c, 1.0);
}`

// bloomUpFS blurs a level with a tent filter as it is added to the one above.
const bloomUpFS = `#version 330
in vec2 fragTexCoord;
uniform sampler2D texture0;
out vec4 finalColor;
void main() {
    vec2 t = 1.0 / vec2(textureSize(texture0, 0));
    vec3 c = texture(texture0, fragTexCoord).rgb * 4.0;
    c += (texture(texture0, fragTexCoord + vec2(t.x, 0)).rgb + texture(texture0, fragTexCoord - vec2(t.x, 0)).rgb
        + texture(texture0, fragTexCoord + vec2(0, t.y)).rgb + texture(texture0, fragTexCoord - vec2(0, t.y)).rgb) * 2.0;
    c += texture(texture0, fragTexCoord + t).rgb + texture(texture0, fragTexCoord - t).rgb
        + texture(texture0, fragTexCoord + vec2(t.x, -t.y)).rgb + texture(texture0, fragTexCoord - vec2(t.x, -t.y)).rgb;
    finalColor = vec4(c / 16.0, 1.0);
}`

// imageFS makes the final image. image() is the whole look for one pixel;
// FXAA then blends along the edges it finds in the result.
var imageFS = `#version 330
in vec2 fragTexCoord;
uniform sampler2D texture0; // HDR scene
uniform sampler2D bloom;
uniform sampler2D aoMap;    // ambient occlusion, half size
uniform float exposure;
uniform float sunUp;        // sun height where the player stands
uniform float time;
uniform vec3 sunScreen;     // xy: the sun on screen (uv), z: light shafts strength
out vec4 finalColor;

// Light shafts: the bright parts (the bloom's source) smeared toward the
// sun, so they stream out from behind the relief when the sun is low.
vec3 shafts(vec2 uv) {
    if (sunScreen.z <= 0.0) return vec3(0.0);
    vec2 stride = (sunScreen.xy - uv) / 32.0;
    vec3 sum = vec3(0.0);
    float w = 1.0;
    for (int i = 0; i < 32; i++) {
        uv += stride;
        sum += texture(bloom, uv).rgb * w;
        w *= 0.92;
    }
    return sum / 32.0 * sunScreen.z;
}

vec3 rays; // light shafts, computed once per pixel in main

// occlusion blurs the half-size ambient occlusion with a 3×3 tent, which
// cancels out its 2×2 rotation pattern exactly.
float occlusion(vec2 uv) {
    vec2 t = 1.0 / vec2(textureSize(aoMap, 0));
    // Four bilinear taps half a texel away make the tent.
    t *= 0.5;
    return 0.25 * (texture(aoMap, uv + vec2(-1, -1) * t).r + texture(aoMap, uv + vec2(1, -1) * t).r
                 + texture(aoMap, uv + vec2(-1, 1) * t).r + texture(aoMap, uv + vec2(1, 1) * t).r);
}

float occ; // computed once per pixel in main

vec3 image(vec2 uv) {
    vec3 c = texture(texture0, uv).rgb * occ + texture(bloom, uv).rgb * ` + ftoa(bloomStrength) + ` + rays;
    // Filmic curve (ACES, Narkowicz's fit): deeper shadows and a soft
    // shoulder. The 0.53 keeps the mid-tones where the old curve had them.
    c *= exposure * 0.53;
    c = clamp(c * (2.51 * c + 0.03) / (c * (2.43 * c + 0.59) + 0.14), 0.0, 1.0);

    // Grading by time of day: warm and rich in the day, cool shadows and
    // warm highlights in the blue dusk, cold and greyer at night.
    float day = smoothstep(-0.1, 0.25, sunUp);
    float dusk = smoothstep(-0.3, -0.02, sunUp) * (1.0 - smoothstep(0.02, 0.3, sunUp));
    float l = dot(c, vec3(0.2126, 0.7152, 0.0722));
    float sat = mix(0.8, 1.12, day);
    c = mix(vec3(l), c, sat);
    c *= mix(vec3(0.84, 0.94, 1.16), vec3(1.03, 1.0, 0.96), day);
    // Split toning, the style's warm light and cool shade: violet in the
    // shadows all day, stronger and bluer at dusk.
    vec3 shadows = mix(mix(vec3(1.0), vec3(0.95, 0.93, 1.07), day), vec3(0.9, 0.95, 1.12), dusk);
    vec3 lights = mix(mix(vec3(1.0), vec3(1.03, 1.0, 0.95), day), vec3(1.08, 1.0, 0.9), dusk);
    c *= mix(shadows, lights, smoothstep(0.1, 0.7, l));
    return clamp(c, 0.0, 1.0);
}

float luma(vec3 c) { return dot(c, vec3(0.299, 0.587, 0.114)); }

void main() {
    vec2 t = 1.0 / vec2(textureSize(texture0, 0));
    vec2 uv = fragTexCoord;
    rays = shafts(uv) * vec3(1.0, 0.75, 0.5);
    occ = occlusion(uv);
    vec3 c = image(uv);

    // FXAA, simplified: find the edge direction from the neighbors' luma
    // and blur along it.
    float lm = luma(c);
    float ln = luma(image(uv + vec2(-t.x, -t.y))), lne = luma(image(uv + vec2(t.x, -t.y)));
    float lsw = luma(image(uv + vec2(-t.x, t.y))), lse = luma(image(uv + vec2(t.x, t.y)));
    float lo = min(lm, min(min(ln, lne), min(lsw, lse)));
    float hi = max(lm, max(max(ln, lne), max(lsw, lse)));
    if (hi - lo > max(0.04, hi * 0.12)) {
        vec2 dir = vec2(-((ln + lne) - (lsw + lse)), (ln + lsw) - (lne + lse));
        float reduce = max((ln + lne + lsw + lse) * 0.03125, 1.0 / 128.0);
        dir = clamp(dir / (min(abs(dir.x), abs(dir.y)) + reduce), -8.0, 8.0) * t;
        vec3 a = 0.5 * (image(uv - dir / 6.0) + image(uv + dir / 6.0));
        vec3 b = a * 0.5 + 0.25 * (image(uv - dir * 0.5) + image(uv + dir * 0.5));
        float lb = luma(b);
        c = (lb < lo || lb > hi) ? a : b;
    }

    // Vignette and a fine film grain.
    vec2 q = uv - 0.5;
    c *= 1.0 - dot(q, q) * 0.45;
    float n = fract(sin(dot(uv * 1000.0 + time, vec2(12.9898, 78.233))) * 43758.5453);
    c += (n - 0.5) * 0.025;
    finalColor = vec4(c, 1.0);
}`

// aoFS: scalable ambient obscurance from the depth alone. Each pixel finds
// its view-space position and normal, then looks at a dozen neighbors on a
// spiral half a meter wide: those rising above its surface hide the sky.
var aoFS = `#version 330
in vec2 fragTexCoord;
uniform sampler2D texture0; // the scene's depth
uniform vec4 proj;          // x, y: tangent of the half field of view; z: time
out vec4 finalColor;
const float near = 0.05, far = 9000.0;
const float radius = 0.8;

vec3 viewPos(vec2 uv) {
    float d = texture(texture0, uv).r * 2.0 - 1.0;
    float z = 2.0 * near * far / (far + near - d * (far - near));
    return vec3((uv * 2.0 - 1.0) * proj.xy * z, z);
}

void main() {
    vec2 uv = fragTexCoord;
    if (texture(texture0, uv).r >= 0.99999) { finalColor = vec4(1.0); return; } // sky
    vec3 P = viewPos(uv);
    if (P.z > 40.0) { finalColor = vec4(1.0); return; }
    // Normal from the closer neighbor on each axis, so edges stay clean.
    // The depth has few bits: neighbors a few texels away give a steadier
    // slope than adjacent ones.
    vec2 t = 3.0 / vec2(textureSize(texture0, 0));
    vec3 px = viewPos(uv + vec2(t.x, 0)), nx = viewPos(uv - vec2(t.x, 0));
    vec3 py = viewPos(uv + vec2(0, t.y)), ny = viewPos(uv - vec2(0, t.y));
    vec3 dx = abs(px.z - P.z) < abs(nx.z - P.z) ? px - P : P - nx;
    vec3 dy = abs(py.z - P.z) < abs(ny.z - P.z) ? py - P : P - ny;
    vec3 N = normalize(cross(dx, dy));
    if (dot(N, P) > 0.0) N = -N;

    const int N_SAMPLES = 8;
    // The spiral turns from pixel to pixel over a 2×2 block; the blur in
    // imageFS averages the block.
    ivec2 b = ivec2(gl_FragCoord.xy) & 1;
    float spin = (float(b.x * 2 + (b.x ^ b.y)) + 0.5) / 4.0 * 6.2832 / float(N_SAMPLES) * 2.0;
    vec2 rs = radius / (P.z * 2.0 * proj.xy); // the radius on screen, in uv
    float sum = 0.0;
    for (int i = 0; i < N_SAMPLES; i++) {
        float a = 0.3 + 0.7 * (float(i) + 0.5) / float(N_SAMPLES);
        float ang = a * 6.2832 * 2.0 + spin;
        vec2 o = vec2(cos(ang), sin(ang)) * a * rs;
        vec3 v = viewPos(uv + o) - P;
        float vv = dot(v, v);
        float f = max(radius * radius - vv, 0.0);
        sum += f * f * f * max((dot(v, N) - 0.008 * P.z - 0.0008 * P.z * P.z) / (vv + 0.003), 0.0);
    }
    float ao = max(0.0, 1.0 - sum * 5.5 / (pow(radius, 6.0) * float(N_SAMPLES)));
    // Surfaces seen edge-on have the least reliable depth: fade it there.
    float facing = smoothstep(0.12, 0.4, abs(dot(N, normalize(P))));
    ao = mix(1.0, ao, (1.0 - smoothstep(20.0, 40.0, P.z)) * facing);
    finalColor = vec4(vec3(ao), 1.0);
}`
