// Package mathx holds the vectors, noise and hashes shared by the whole
// game: pure functions, no graphics.
package mathx

import (
	"image/color"
	"math"
)

// Vec3 is the float64 vector used by the simulation; raylib only sees float32.
type Vec3 struct{ X, Y, Z float64 }

func (a Vec3) Add(b Vec3) Vec3             { return Vec3{a.X + b.X, a.Y + b.Y, a.Z + b.Z} }
func (a Vec3) Sub(b Vec3) Vec3             { return Vec3{a.X - b.X, a.Y - b.Y, a.Z - b.Z} }
func (a Vec3) Scale(s float64) Vec3        { return Vec3{a.X * s, a.Y * s, a.Z * s} }
func (a Vec3) Dot(b Vec3) float64          { return a.X*b.X + a.Y*b.Y + a.Z*b.Z }
func (a Vec3) Length() float64             { return math.Sqrt(a.Dot(a)) }
func (a Vec3) Lerp(b Vec3, t float64) Vec3 { return a.Add(b.Sub(a).Scale(t)) }

func (a Vec3) Cross(b Vec3) Vec3 {
	return Vec3{a.Y*b.Z - a.Z*b.Y, a.Z*b.X - a.X*b.Z, a.X*b.Y - a.Y*b.X}
}

func (a Vec3) Norm() Vec3 {
	if l := a.Length(); l > 0 {
		return a.Scale(1 / l)
	}
	return a
}

func Clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func Smoothstep(e0, e1, x float64) float64 {
	t := Clamp01((x - e0) / (e1 - e0))
	return t * t * (3 - 2*t)
}

func RGB(r, g, b float64) color.RGBA {
	c := func(v float64) uint8 { return uint8(math.Max(0, math.Min(255, v))) }
	return color.RGBA{c(r), c(g), c(b), 255}
}

func lerp8(a, b uint8, t float32) uint8 {
	return uint8(float32(a) + (float32(b)-float32(a))*t)
}

func Hash2(x, y int, seed uint64) float64 {
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
	a, b := Hash2(xi, yi, seed), Hash2(xi+1, yi, seed)
	c, d := Hash2(xi, yi+1, seed), Hash2(xi+1, yi+1, seed)
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

func Hash3(x, y, z int, seed uint64) float64 {
	h := uint64(x)*0x9E3779B97F4A7C15 ^ uint64(y)*0xC2B2AE3D27D4EB4F ^ uint64(z)*0x27D4EB2F165667C5 ^ seed*0x165667B19E3779F9
	h ^= h >> 31
	h *= 0xBF58476D1CE4E5B9
	h ^= h >> 29
	return float64(h>>11) / (1 << 53)
}

func valueNoise3(p Vec3, seed uint64) float64 {
	ix, iy, iz := math.Floor(p.X), math.Floor(p.Y), math.Floor(p.Z)
	fx, fy, fz := p.X-ix, p.Y-iy, p.Z-iz
	sx, sy, sz := fx*fx*(3-2*fx), fy*fy*(3-2*fy), fz*fz*(3-2*fz)
	x, y, z := int(ix), int(iy), int(iz)
	lerp := func(a, b, t float64) float64 { return a + (b-a)*t }
	layer := func(z int) float64 {
		return lerp(lerp(Hash3(x, y, z, seed), Hash3(x+1, y, z, seed), sx),
			lerp(Hash3(x, y+1, z, seed), Hash3(x+1, y+1, z, seed), sx), sy)
	}
	return lerp(layer(z), layer(z+1), sz)
}

func Fbm3(p Vec3, seed uint64, octaves int) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for o := range octaves {
		sum += amp * valueNoise3(p, seed+uint64(o)*101)
		norm += amp
		amp *= 0.5
		p = p.Scale(2)
	}
	return sum / norm
}

// TangentBasis returns two unit vectors spanning the plane orthogonal to up.
func TangentBasis(up Vec3) (Vec3, Vec3) {
	ref := Vec3{1, 0, 0}
	if math.Abs(up.X) > 0.9 {
		ref = Vec3{0, 0, 1}
	}
	t1 := up.Cross(ref).Norm()
	return t1, t1.Cross(up)
}

// Orient returns the rotation (axis, degrees) taking +Y onto up.
func Orient(up Vec3) (Vec3, float64) {
	axis := Vec3{0, 1, 0}.Cross(up)
	angle := math.Acos(math.Max(-1, math.Min(1, up.Y))) * 180 / math.Pi
	if axis.Length() < 1e-6 {
		return Vec3{1, 0, 0}, angle
	}
	return axis.Norm(), angle
}

// Mars' surface is addressed through a cube projected on the sphere
// (equiangular, so cells keep a near-constant size). Each face has a normal
// and two axes; the shaders use the same table.
type cubeFace struct{ N, U, V Vec3 }

var CubeFaces = [6]cubeFace{
	{Vec3{1, 0, 0}, Vec3{0, 0, -1}, Vec3{0, 1, 0}},
	{Vec3{-1, 0, 0}, Vec3{0, 0, 1}, Vec3{0, 1, 0}},
	{Vec3{0, 1, 0}, Vec3{1, 0, 0}, Vec3{0, 0, -1}},
	{Vec3{0, -1, 0}, Vec3{1, 0, 0}, Vec3{0, 0, 1}},
	{Vec3{0, 0, 1}, Vec3{1, 0, 0}, Vec3{0, 1, 0}},
	{Vec3{0, 0, -1}, Vec3{-1, 0, 0}, Vec3{0, 1, 0}},
}

// FaceDir maps face coordinates a, b in [-1, 1] to a unit direction.
// Values slightly outside the range spill over onto the neighboring faces.
func FaceDir(f int, a, b float64) Vec3 {
	cf := CubeFaces[f]
	x, y := math.Tan(a*math.Pi/4), math.Tan(b*math.Pi/4)
	return cf.N.Add(cf.U.Scale(x)).Add(cf.V.Scale(y)).Norm()
}

// DirToFace is the inverse of faceDir.
func DirToFace(d Vec3) (f int, a, b float64) {
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
	cf := CubeFaces[f]
	dn := d.Dot(cf.N)
	return f, math.Atan(d.Dot(cf.U)/dn) * 4 / math.Pi, math.Atan(d.Dot(cf.V)/dn) * 4 / math.Pi
}
