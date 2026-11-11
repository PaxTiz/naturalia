package main

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Dust devils: whirlwinds of dust wandering across the plains in daytime.
// They are only scenery: they neither carry dust away nor bring it back.

const (
	maxDevils    = 3
	devilMinDist = 80.0  // they appear this far from the player at least...
	devilMaxDist = 220.0 // ...and at most
	devilGone    = 420.0 // and vanish beyond this
)

type dustDevil struct {
	dir            vec3 // ground position, from Mars' center
	vel            vec3 // drift along the ground, m/s
	age, life      float64
	height, radius float64
	seed           float64
}

// strength fades a devil in when it forms and out when it dies down.
func (d *dustDevil) strength() float64 {
	return smoothstep(0, 6, d.age) * smoothstep(d.life, d.life-6, d.age)
}

func (g *Game) updateDevils(dt float64) {
	up := g.player.LocalUp()
	kept := g.devils[:0]
	for _, d := range g.devils {
		d.age += dt
		// Drift with a slow wander, staying along the ground.
		t1, t2 := tangentBasis(d.dir)
		d.vel = d.vel.add(t1.scale((g.rng.Float64() - 0.5) * dt)).add(t2.scale((g.rng.Float64() - 0.5) * dt))
		d.dir = d.dir.add(d.vel.scale(dt / marsRadius)).norm()
		d.vel = tangentAt(d.dir, d.vel).scale(d.vel.length())
		if d.age < d.life && arcDist(d.dir, up) < devilGone {
			kept = append(kept, d)
		}
	}
	g.devils = kept

	g.devilTimer -= dt
	if g.devilTimer > 0 || len(g.devils) >= maxDevils || sunDir.dot(up) < 0.2 {
		return
	}
	g.devilTimer = 8 + g.rng.Float64()*14
	t1, t2 := tangentBasis(up)
	a := g.rng.Float64() * 2 * math.Pi
	at := toward2(up, t1.scale(math.Cos(a)).add(t2.scale(math.Sin(a))), devilMinDist+g.rng.Float64()*(devilMaxDist-devilMinDist))
	if g.terrain.Ice(at) > 0.05 || arcDist(at, baseUp) < 80 {
		return // they need warm bare ground
	}
	b := g.rng.Float64() * 2 * math.Pi
	tt1, tt2 := tangentBasis(at)
	g.devils = append(g.devils, &dustDevil{
		dir:    at,
		vel:    tt1.scale(math.Cos(b)).add(tt2.scale(math.Sin(b))).scale(2 + g.rng.Float64()*3),
		life:   45 + g.rng.Float64()*60,
		height: 35 + g.rng.Float64()*45,
		radius: 2.5 + g.rng.Float64()*3,
		seed:   g.rng.Float64() * 100,
	})
}

// drawDevils draws the whirlwinds as see-through swirling tubes, shaped
// and animated by their shaders. Drawn after the opaque scenery.
func (g *Game) drawDevils() {
	if len(g.devils) == 0 {
		return
	}
	gfx := g.gfx
	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()
	rl.DisableBackfaceCulling()
	rl.BeginBlendMode(rl.BlendAlpha)
	for _, d := range g.devils {
		up := d.dir
		t1, t2 := tangentBasis(up)
		foot := g.terrain.Surface(d.dir).sub(up.scale(1))
		rl.SetShaderValue(gfx.devilSh, gfx.devilLoc, []float32{float32(d.seed), float32(d.strength()), 0, 0}, rl.ShaderUniformVec4)
		ax, ay, az := t1.scale(d.radius), up.scale(d.height), t2.scale(d.radius)
		rl.DrawMesh(gfx.devilMesh, gfx.devilMat, rl.Matrix{
			M0: float32(ax.X), M1: float32(ax.Y), M2: float32(ax.Z),
			M4: float32(ay.X), M5: float32(ay.Y), M6: float32(ay.Z),
			M8: float32(az.X), M9: float32(az.Y), M10: float32(az.Z),
			M12: float32(foot.X), M13: float32(foot.Y), M14: float32(foot.Z), M15: 1,
		})
	}
	rl.EndBlendMode()
	rl.EnableBackfaceCulling()
	rl.EnableDepthMask()
}

// devilVS bends a flat grid into the whirlwind: u goes around, v goes up.
// The column narrows above a dusty skirt, flares at the top and sways.
const devilVS = `#version 330
in vec3 vertexPosition;
in vec2 vertexTexCoord;
uniform mat4 mvp;
uniform mat4 matModel;
uniform float time;
uniform vec4 devil; // x: seed, y: strength
uniform vec3 origin;
out vec3 fragRel;
out vec3 fragPos;
out vec3 fragNormal;
out vec2 tube;
void main() {
    float a = vertexTexCoord.x * 6.2831853;
    float t = vertexTexCoord.y;
    float r = 0.45 + 0.55 * pow(t, 1.3) + 0.7 * exp(-t * 18.0);
    vec2 sway = vec2(sin(t * 3.0 + time * 0.7 + devil.x), cos(t * 2.3 + time * 0.5 + devil.x * 1.7)) * 0.8 * t * t;
    vec3 p = vec3(cos(a) * r + sway.x, t, sin(a) * r + sway.y);
    tube = vec2(a, t);
    fragPos = vec3(matModel * vec4(p, 1.0));
    fragRel = fragPos - origin;
    fragNormal = normalize(mat3(matModel) * vec3(cos(a), 0.0, sin(a)));
    gl_Position = mvp * vec4(p, 1.0);
}`

// devilFS paints spiraling streaks of dust, denser where the tube is seen
// face-on (its middle) and fading at the bottom, the top and the edges.
const devilFS = `#version 330
in vec3 fragPos;
in vec3 fragNormal;
in vec2 tube;
uniform float time;
uniform vec4 devil;
out vec4 finalColor;
` + shaderCommon + `
void main() {
    vec3 v = normalize(viewRel - fragRel);
    vec3 n = normalize(fragNormal);
    float t = tube.y;
    float a = tube.x - time * 3.0 - t * 5.0;
    float swirl = fbm3(vec3(cos(a) * 1.5, sin(a) * 1.5, t * 7.0 - time * 0.8 + devil.x));
    float streak = smoothstep(0.35, 0.75, swirl);
    float fade = smoothstep(0.0, 0.06, t) * (1.0 - smoothstep(0.5, 1.0, t));
    float dense = mix(1.7, 0.8, smoothstep(0.0, 0.35, t)); // thickest near the ground
    float alpha = clamp(pow(abs(dot(n, v)), 1.5) * (0.25 + 0.75 * streak) * fade * dense * devil.y * 0.55, 0.0, 0.85);

    vec3 up = normalize(fragPos);
    float lit = 0.55 + 0.45 * clamp(dot(n, sunDir) * 0.5 + 0.5, 0.0, 1.0);
    vec3 col = vec3(0.62, 0.40, 0.26) * (0.75 + 0.5 * streak) * (sunCol * 0.8 * lit * daylight(up) + skyLight(up, up));
    finalColor = vec4(finish(col, length(viewRel - fragRel)), alpha);
}`
