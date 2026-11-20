package planet

import "math"

// The sun's course: Mars turns on an axis that puts the base 20° from its
// equator, so the sun rises in the east (toward the canyon), climbs to 70°
// at noon and sets in the west, behind the volcano. The hours are the
// base's: elsewhere, the sun is ahead or behind by the distance walked.
//
// The sun stays on the equator (no seasons), so its course seen from a
// point is a great circle: at hour angle h from the local noon, it stands
// cos(latitude)·cos(h) above the horizon.

// SunDir points toward the sun: set every frame by SetSun.
var SunDir = refSun

// refSun is the validated look of the art direction (docs), low in the
// afternoon: the course goes through it, at RefHour.
var refSun = vec3{-0.5, 0.62, -0.6}.Norm()

const baseLatitude = 20 * math.Pi / 180

var (
	sunAxis    vec3    // Mars' spin axis
	sunAcross  vec3    // sunAxis × refSun: the course is refSun·cos a + sunAcross·sin a
	RefHour    float64 // the base's hour when the sun stands at refSun
	baseNoonAt float64 // the course's angle at the base's noon
)

func init() {
	n := refSun.Cross(BaseUp).Norm()
	m := n.Cross(refSun) // refSun, n, m: the axis lies in the plane of n and m
	beta := math.Sin(baseLatitude) / BaseUp.Dot(m)
	sunAxis = m.Scale(beta).Sub(n.Scale(math.Sqrt(1 - beta*beta))) // turning on it, refSun goes down
	sunAcross = sunAxis.Cross(refSun)
	baseNoonAt = noonAt(BaseUp)
	RefHour = 12 - baseNoonAt*12/math.Pi
}

// SunAxis is Mars' spin axis.
func SunAxis() vec3 { return sunAxis }

// noonAt is the course's angle when the sun is highest above up.
func noonAt(up vec3) float64 { return math.Atan2(sunAcross.Dot(up), refSun.Dot(up)) }

// SunAt is the sun's direction at the base's hour h.
func SunAt(h float64) vec3 {
	a := (h - RefHour) * math.Pi / 12
	return refSun.Scale(math.Cos(a)).Add(sunAcross.Scale(math.Sin(a)))
}

// SunAtHourAngle is the sun's direction seen from up, h radians after its
// local noon. Pure: the terrain's workers use it.
func SunAtHourAngle(up vec3, h float64) vec3 {
	a := noonAt(up) + h
	return refSun.Scale(math.Cos(a)).Add(sunAcross.Scale(math.Sin(a)))
}

// SetSun moves the sun to the base's hour h.
func SetSun(h float64) { SunDir = SunAt(h) }

// LocalHour is the hour at up (0..24) when it is h at the base.
func LocalHour(up vec3, h float64) float64 {
	return math.Mod(h+(baseNoonAt-noonAt(up))*12/math.Pi+48, 24)
}

// Latitude is how far up is from Mars' equator, in radians.
func Latitude(up vec3) float64 { return math.Asin(max(-1, min(1, sunAxis.Dot(up.Norm())))) }
