package main

import (
	"image/color"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// vec3 is the float64 vector used by the simulation; raylib only sees float32.
type vec3 struct{ X, Y, Z float64 }

func (a vec3) add(b vec3) vec3             { return vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a vec3) sub(b vec3) vec3             { return vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a vec3) scale(s float64) vec3        { return vec3{a.X * s, a.Y * s, a.Z * s} }
func (a vec3) dot(b vec3) float64          { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a vec3) length() float64             { return math.Sqrt(a.dot(a)) }
func (a vec3) lerp(b vec3, t float64) vec3 { return a.add(b.sub(a).scale(t)) }
func (a vec3) rl() rl.Vector3              { return rl.NewVector3(float32(a.X), float32(a.Y), float32(a.Z)) }

func (a vec3) cross(b vec3) vec3 {
	return vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}

func (a vec3) norm() vec3 {
	if l := a.length(); l > 0 {
		return a.scale(1 / l)
	}
	return a
}

func distXZ(a, b vec3) float64 { return math.Hypot(a.X-b.X, a.Z-b.Z) }

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func smoothstep(e0, e1, x float64) float64 {
	t := clamp01((x - e0) / (e1 - e0))
	return t * t * (3 - 2*t)
}

func rgb(r, g, b float64) color.RGBA {
	c := func(v float64) uint8 { return uint8(math.Max(0, math.Min(255, v))) }
	return color.RGBA{c(r), c(g), c(b), 255}
}

func lerp8(a, b uint8, t float32) uint8 {
	return uint8(float32(a) + (float32(b)-float32(a))*t)
}

func hash2(x, y int, seed uint64) float64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F ^ seed*0x165667B19E3779F9
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return float64(h>>11) / (1 << 53)
}

func valueNoise(x, y float64, seed uint64) float64 {
	ix, iy := math.Floor(x), math.Floor(y)
	fx, fy := x-ix, y-iy
	sx, sy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	xi, yi := int(ix), int(iy)
	a, b := hash2(xi, yi, seed), hash2(xi+1, yi, seed)
	c, d := hash2(xi, yi+1, seed), hash2(xi+1, yi+1, seed)
	top, bottom := a+(b-a)*sx, c+(d-c)*sx
	return top + (bottom-top)*sy
}

func fbm(x, y float64, seed uint64, octaves int) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for o := range octaves {
		sum += amp * valueNoise(x, y, seed+uint64(o)*101)
		norm += amp
		amp *= 0.5
		x, y = x*2, y*2
	}
	return sum / norm
}
