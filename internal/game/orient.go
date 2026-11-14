package game

import "math"

// An entity's orientation is its yaw around the ground's up, then its
// pitch (nose up), then its roll (right side down), as the save keeps
// them. Building turns it around any axis, through its axes.

// Axes are the entity's front, up and side.
func (e *Transform) Axes() (ax, up, az vec3) {
	ax, up, az = entityAxes(e.Dir, e.Yaw)
	if e.Pitch != 0 {
		c, s := math.Cos(e.Pitch), math.Sin(e.Pitch)
		ax, up = ax.Scale(c).Add(up.Scale(s)), up.Scale(c).Sub(ax.Scale(s))
	}
	if e.Roll != 0 {
		c, s := math.Cos(e.Roll), math.Sin(e.Roll)
		up, az = up.Scale(c).Add(az.Scale(s)), az.Scale(c).Sub(up.Scale(s))
	}
	return ax, up, az
}

// SetAxes sets the angles that give the front ax and the up up (both of
// length 1, at right angles). Nose straight up or down, the roll is 0.
func (e *Transform) SetAxes(ax, up vec3) {
	d := e.Dir
	sp := math.Max(-1, math.Min(1, ax.Dot(d)))
	e.Pitch = math.Asin(sp)
	ax0 := ax.Sub(d.Scale(sp))
	if ax0.Length() < 1e-6 { // its front is the ground's up: its up gives its yaw
		ax0, e.Roll = up.Scale(-sp), 0
	}
	ax0 = ax0.Sub(d.Scale(ax0.Dot(d))).Norm()
	e.Yaw = yawOf(d, ax0)
	cp := math.Cos(e.Pitch)
	up1, az0 := d.Scale(cp).Sub(ax0.Scale(sp)), ax0.Cross(d)
	e.Roll = math.Atan2(up.Dot(az0), up.Dot(up1))
}

// Turn turns the entity by angle radians around axis (length 1),
// counter-clockwise looking down it.
func (e *Transform) Turn(axis vec3, angle float64) {
	ax, up, _ := e.Axes()
	e.SetAxes(rotate(ax, axis, angle).Norm(), rotate(up, axis, angle).Norm())
}

// rotate turns v by angle around axis (Rodrigues).
func rotate(v, axis vec3, angle float64) vec3 {
	c, s := math.Cos(angle), math.Sin(angle)
	return v.Scale(c).Add(axis.Cross(v).Scale(s)).Add(axis.Scale(axis.Dot(v) * (1 - c)))
}
