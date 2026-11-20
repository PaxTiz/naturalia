package base

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
