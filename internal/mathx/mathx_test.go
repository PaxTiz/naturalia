package mathx

import (
	"encoding/json"
	"math"
	"testing"
)

func TestFaceRoundTrip(t *testing.T) {
	for _, d := range []Vec3{{1, 0.2, -0.3}, {-0.1, -1, 0.4}, {0.3, 0.3, 1}} {
		d = d.Norm()
		f, a, b := DirToFace(d)
		if back := FaceDir(f, a, b); back.Sub(d).Length() > 1e-9 {
			t.Errorf("%v → face %d (%f, %f) → %v", d, f, a, b, back)
		}
	}
}

func TestVec3JSON(t *testing.T) {
	var a, b Vec3
	if err := json.Unmarshal([]byte(`[1, 2, 3]`), &a); err != nil || a != (Vec3{1, 2, 3}) {
		t.Errorf("array: %v %v", a, err)
	}
	if err := json.Unmarshal([]byte(`{"X": 4, "Z": 6}`), &b); err != nil || b != (Vec3{4, 0, 6}) {
		t.Errorf("object: %v %v", b, err)
	}
	if math.Abs(Smoothstep(0, 1, 0.5)-0.5) > 1e-12 {
		t.Error("smoothstep")
	}
}
