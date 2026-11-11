package main

import (
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/planet"
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
	return mathx.Smoothstep(0, 6, d.age) * mathx.Smoothstep(d.life, d.life-6, d.age)
}

// moveDevils drifts the devils and lets them die down.
func (g *Game) moveDevils(dt float64) {
	up := g.player.LocalUp()
	kept := g.devils[:0]
	for _, d := range g.devils {
		d.age += dt
		// Drift with a slow wander, staying along the ground.
		t1, t2 := mathx.TangentBasis(d.dir)
		d.vel = d.vel.Add(t1.Scale((g.rng.Float64() - 0.5) * dt)).Add(t2.Scale((g.rng.Float64() - 0.5) * dt))
		d.dir = d.dir.Add(d.vel.Scale(dt / planet.Radius)).Norm()
		d.vel = planet.TangentAt(d.dir, d.vel).Scale(d.vel.Length())
		if d.age < d.life && planet.ArcDist(d.dir, up) < devilGone {
			kept = append(kept, d)
		}
	}
	g.devils = kept
}

// spawnDevil raises a devil somewhere around the player, on warm bare
// ground.
func spawnDevil(g *Game) {
	up := g.player.LocalUp()
	t1, t2 := mathx.TangentBasis(up)
	a := g.rng.Float64() * 2 * math.Pi
	at := planet.Toward2(up, t1.Scale(math.Cos(a)).Add(t2.Scale(math.Sin(a))), devilMinDist+g.rng.Float64()*(devilMaxDist-devilMinDist))
	if g.world.Relief.Ice(at) > 0.05 || planet.ArcDist(at, planet.BaseUp) < 80 {
		return // they need warm bare ground
	}
	b := g.rng.Float64() * 2 * math.Pi
	tt1, tt2 := mathx.TangentBasis(at)
	g.devils = append(g.devils, &dustDevil{
		dir:    at,
		vel:    tt1.Scale(math.Cos(b)).Add(tt2.Scale(math.Sin(b))).Scale(2 + g.rng.Float64()*3),
		life:   45 + g.rng.Float64()*60,
		height: 35 + g.rng.Float64()*45,
		radius: 2.5 + g.rng.Float64()*3,
		seed:   g.rng.Float64() * 100,
	})
}

// drawDevils draws the whirlwinds as see-through swirling tubes, shaped
// and animated by their shaders. Drawn after the opaque scenery.
func (gfx *Graphics) drawDevils(g *Game) {
	if len(g.devils) == 0 {
		return
	}
	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()
	rl.DisableBackfaceCulling()
	rl.BeginBlendMode(rl.BlendAlpha)
	for _, d := range g.devils {
		up := d.dir
		t1, t2 := mathx.TangentBasis(up)
		foot := g.world.Relief.Surface(d.dir).Sub(up.Scale(1))
		rl.SetShaderValue(gfx.devilSh, gfx.devilLoc, []float32{float32(d.seed), float32(d.strength()), 0, 0}, rl.ShaderUniformVec4)
		ax, ay, az := t1.Scale(d.radius), up.Scale(d.height), t2.Scale(d.radius)
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
