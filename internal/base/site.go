package base

import "math"

// Site is where buildings stand on Mars, in a frame of their own: the
// base on the north pole, a bootcamp anywhere (camp.go). Its origin is on
// the floor, its Y axis is the local up, X and Z lie along the ground; the
// buildings are laid out in it as at the pole, with y up.
type Site struct {
	Origin  vec3
	X, Y, Z vec3 // unit, X × Y = Z as for the base's axes
}

// BaseSite is the base's: its origin, the world's axes.
func BaseSite() Site { return Site{Origin, vec3{1, 0, 0}, vec3{0, 1, 0}, vec3{0, 0, 1}} }

// ToWorld takes a point of the site's frame to the world.
func (s Site) ToWorld(local vec3) vec3 { return s.Origin.Add(s.Dir(local)) }

// ToLocal takes a point of the world to the site's frame.
func (s Site) ToLocal(world vec3) vec3 { return s.DirLocal(world.Sub(s.Origin)) }

// Dir turns a direction of the site's frame to the world.
func (s Site) Dir(d vec3) vec3 { return s.X.Scale(d.X).Add(s.Y.Scale(d.Y)).Add(s.Z.Scale(d.Z)) }

// DirLocal turns a direction of the world to the site's frame.
func (s Site) DirLocal(d vec3) vec3 { return vec3{d.Dot(s.X), d.Dot(s.Y), d.Dot(s.Z)} }

// Rotation is how the site's frame is turned from the world's, as an axis
// and an angle in degrees (raylib's DrawModelEx).
func (s Site) Rotation() (axis vec3, deg float64) {
	// From the rotation matrix whose columns are the axes.
	m := [3][3]float64{{s.X.X, s.Y.X, s.Z.X}, {s.X.Y, s.Y.Y, s.Z.Y}, {s.X.Z, s.Y.Z, s.Z.Z}}
	c := (m[0][0] + m[1][1] + m[2][2] - 1) / 2
	a := math.Acos(math.Max(-1, math.Min(1, c)))
	if a < 1e-9 {
		return vec3{0, 1, 0}, 0
	}
	axis = vec3{m[2][1] - m[1][2], m[0][2] - m[2][0], m[1][0] - m[0][1]}
	if axis.Length() < 1e-9 { // half a turn: the axis is a column of m + I
		for i := range 3 {
			col := vec3{m[0][i], m[1][i], m[2][i]}
			col = col.Add([3]vec3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}[i])
			if col.Length() > 1e-6 {
				axis = col
				break
			}
		}
	}
	return axis.Norm(), a * 180 / math.Pi
}
