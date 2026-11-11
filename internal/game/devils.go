package game

import (
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/planet"
	"math"
)

// Dust devils: whirlwinds of dust wandering across the plains in daytime.
// They are only scenery: they neither carry dust away nor bring it back.

const (
	devilMinDist = 80.0  // they appear this far from the player at least...
	devilMaxDist = 220.0 // ...and at most
	devilGone    = 420.0 // and vanish beyond this
)

type DustDevil struct {
	Dir            vec3 // ground position, from Mars' center
	vel            vec3 // drift along the ground, m/s
	age, life      float64
	Height, Radius float64
	Seed           float64
}

// Strength fades a devil in when it forms and out when it dies down.
func (d *DustDevil) Strength() float64 {
	return mathx.Smoothstep(0, 6, d.age) * mathx.Smoothstep(d.life, d.life-6, d.age)
}

// moveDevils drifts the devils and lets them die down.
func (g *Game) moveDevils(dt float64) {
	up := g.Player.LocalUp()
	kept := g.Devils[:0]
	for _, d := range g.Devils {
		d.age += dt
		// Drift with a slow wander, staying along the ground.
		t1, t2 := mathx.TangentBasis(d.Dir)
		d.vel = d.vel.Add(t1.Scale((g.Rng.Float64() - 0.5) * dt)).Add(t2.Scale((g.Rng.Float64() - 0.5) * dt))
		d.Dir = d.Dir.Add(d.vel.Scale(dt / planet.Radius)).Norm()
		d.vel = planet.TangentAt(d.Dir, d.vel).Scale(d.vel.Length())
		if d.age < d.life && planet.ArcDist(d.Dir, up) < devilGone {
			kept = append(kept, d)
		}
	}
	g.Devils = kept
}

// spawnDevil raises a devil somewhere around the player, on warm bare
// ground.
func spawnDevil(g *Game) {
	up := g.Player.LocalUp()
	t1, t2 := mathx.TangentBasis(up)
	a := g.Rng.Float64() * 2 * math.Pi
	at := planet.Toward2(up, t1.Scale(math.Cos(a)).Add(t2.Scale(math.Sin(a))), devilMinDist+g.Rng.Float64()*(devilMaxDist-devilMinDist))
	if g.World.Relief.Ice(at) > 0.05 || planet.ArcDist(at, planet.BaseUp) < 80 {
		return // they need warm bare ground
	}
	b := g.Rng.Float64() * 2 * math.Pi
	tt1, tt2 := mathx.TangentBasis(at)
	g.Devils = append(g.Devils, &DustDevil{
		Dir:    at,
		vel:    tt1.Scale(math.Cos(b)).Add(tt2.Scale(math.Sin(b))).Scale(2 + g.Rng.Float64()*3),
		life:   45 + g.Rng.Float64()*60,
		Height: 35 + g.Rng.Float64()*45,
		Radius: 2.5 + g.Rng.Float64()*3,
		Seed:   g.Rng.Float64() * 100,
	})
}
