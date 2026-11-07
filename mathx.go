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

func hash3(x, y, z int, seed uint64) float64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F ^ uint64(z)*0x27D4EB2F165667C5 ^ seed*0x165667B19E3779F9
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return float64(h>>11) / (1 << 53)
}

func valueNoise3(p vec3, seed uint64) float64 {
	ix, iy, iz := math.Floor(p.X), math.Floor(p.Y), math.Floor(p.Z)
	fx, fy, fz := p.X-ix, p.Y-iy, p.Z-iz
	sx, sy, sz := fx*fx*(3-2*fx), fy*fy*(3-2*fy), fz*fz*(3-2*fz)
	x, y, z := int(ix), int(iy), int(iz)
	lerp := func(a, b, t float64) float64 { return a + (b-a)*t }
	layer := func(z int) float64 {
		return lerp(lerp(hash3(x, y, z, seed), hash3(x+1, y, z, seed), sx),
			lerp(hash3(x, y+1, z, seed), hash3(x+1, y+1, z, seed), sx), sy)
	}
	return lerp(layer(z), layer(z+1), sz)
}

func fbm3(p vec3, seed uint64, octaves int) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for o := range octaves {
		sum += amp * valueNoise3(p, seed+uint64(o)*101)
		norm += amp
		amp *= 0.5
		p = p.scale(2)
	}
	return sum / norm
}

// tangentBasis returns two unit vectors spanning the plane orthogonal to up.
func tangentBasis(up vec3) (vec3, vec3) {
	ref := vec3{1, 0, 0}
	if math.Abs(up.X) > 0.9 {
		ref = vec3{0, 0, 1}
	}
	t1 := up.cross(ref).norm()
	return t1, t1.cross(up)
}

// orient returns the rotation (axis, degrees) taking +Y onto up.
func orient(up vec3) (vec3, float64) {
	axis := vec3{0, 1, 0}.cross(up)
	angle := math.Acos(math.Max(-1, math.Min(1, up.Y))) * 180 / math.Pi
	if axis.length() < 1e-6 {
		return vec3{1, 0, 0}, angle
	}
	return axis.norm(), angle
}

// Mars' surface is addressed through a cube projected on the sphere
// (equiangular, so cells keep a near-constant size). Each face has a normal
// and two axes; the shaders use the same table.
type cubeFace struct{ n, u, v vec3 }

var cubeFaces = [6]cubeFace{
	{vec3{1, 0, 0}, vec3{0, 0, -1}, vec3{0, 1, 0}},
	{vec3{-1, 0, 0}, vec3{0, 0, 1}, vec3{0, 1, 0}},
	{vec3{0, 1, 0}, vec3{1, 0, 0}, vec3{0, 0, -1}},
	{vec3{0, -1, 0}, vec3{1, 0, 0}, vec3{0, 0, 1}},
	{vec3{0, 0, 1}, vec3{1, 0, 0}, vec3{0, 1, 0}},
	{vec3{0, 0, -1}, vec3{-1, 0, 0}, vec3{0, 1, 0}},
}

// faceDir maps face coordinates a, b in [-1, 1] to a unit direction.
// Values slightly outside the range spill over onto the neighboring faces.
func faceDir(f int, a, b float64) vec3 {
	cf := cubeFaces[f]
	x, y := math.Tan(a*math.Pi/4), math.Tan(b*math.Pi/4)
	return cf.n.add(cf.u.scale(x)).add(cf.v.scale(y)).norm()
}

// dirToFace is the inverse of faceDir.
func dirToFace(d vec3) (f int, a, b float64) {
	ax, ay, az := math.Abs(d.X), math.Abs(d.Y), math.Abs(d.Z)
	switch {
	case ax >= ay && ax >= az:
		f = 0
		if d.X < 0 {
			f = 1
		}
	case ay >= az:
		f = 2
		if d.Y < 0 {
			f = 3
		}
	default:
		f = 4
		if d.Z < 0 {
			f = 5
		}
	}
	cf := cubeFaces[f]
	dn := d.dot(cf.n)
	return f, math.Atan(d.dot(cf.u)/dn) * 4 / math.Pi, math.Atan(d.dot(cf.v)/dn) * 4 / math.Pi
}
