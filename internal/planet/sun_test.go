package planet

import (
	"math"
	"testing"
)

// The sun's course goes through the validated look, rises in the east of
// the base, climbs to 70° at its noon and is gone at midnight.
func TestSunCourse(t *testing.T) {
	if d := SunAt(RefHour).Sub(refSun).Length(); d > 1e-9 {
		t.Fatalf("the course misses the reference sun by %g", d)
	}
	elev := func(h float64) float64 { return math.Asin(SunAt(h).Dot(BaseUp)) * 180 / math.Pi }
	if e := elev(12); math.Abs(e-70) > 0.01 {
		t.Fatalf("noon at %.2f°, want 70°", e)
	}
	if e := elev(0); e > -60 {
		t.Fatalf("midnight at %.2f°", e)
	}
	if e := elev(6); math.Abs(e) > 0.01 {
		t.Fatalf("6 h at %.2f°, want the horizon", e)
	}
	if elev(9) < elev(8) || elev(15) > elev(14) {
		t.Fatal("the sun goes the wrong way")
	}
	if east := SunAt(7).Dot(vec3{1, 0, 0}); east <= 0 {
		t.Fatalf("the sun rises toward %v, not the east", SunAt(7))
	}
	if h := LocalHour(BaseUp, 9.5); math.Abs(h-9.5) > 1e-9 {
		t.Fatalf("local hour at the base %g", h)
	}
	far := Toward(vec3{-1, 0, 0}, Radius*math.Pi/2) // a quarter of Mars to the west
	if h := LocalHour(far, 12); h > 9 {
		t.Fatalf("a quarter of Mars west, it is %g h at the base's noon", h)
	}
	up := Toward(vec3{0.3, 0, 1}, 900)
	if d := SunAtHourAngle(up, 0).Sub(SunAt(12 + 12 - LocalHour(up, 12))).Length(); d > 1e-9 {
		t.Fatalf("the local noon is off by %g", d)
	}
}
