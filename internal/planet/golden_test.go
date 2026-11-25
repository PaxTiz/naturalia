package planet

import (
	"encoding/json"
	"math"
	"math/rand/v2"
	"naturalia/internal/mathx"
	"os"
	"testing"
)

// goldenDirs are directions all over Mars, on each landmark and around
// the base.
func goldenDirs() []vec3 {
	rng := rand.New(rand.NewPCG(3, 4))
	var out []vec3
	around := func(c vec3, spread float64, n int) {
		t1, t2 := mathx.TangentBasis(c)
		for range n {
			a, r := rng.Float64()*2*math.Pi, rng.Float64()*spread
			out = append(out, Toward2(c, t1.Scale(math.Cos(a)).Add(t2.Scale(math.Sin(a))), r))
		}
	}
	around(BaseUp, 5000, 60)
	around(BaseUp, 200, 30)
	around(volcanoCenter, 1600, 30)
	around(capCenter, 800, 30)
	around(canyonCenter, 400, 30)
	around(BaseUp.Scale(-1), 5000, 40)
	return out
}

// TestGolden keeps Mars as it is: what is saved stands on it. With
// NA_GOLDEN=1 it writes the reference again, when the relief changes on
// purpose.
func TestGolden(t *testing.T) {
	const file = "testdata/heights.json"
	set := DefaultSettings()
	set.Prepare()
	r := NewWorld(7, &set).Relief
	var got []float64
	for _, d := range goldenDirs() {
		got = append(got, r.Height(d))
	}
	if os.Getenv("NA_GOLDEN") != "" {
		data, _ := json.Marshal(got)
		if err := os.WriteFile(file, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var want []float64
	if err := json.Unmarshal(data, &want); err != nil || len(want) != len(got) {
		t.Fatal(err, len(want), len(got))
	}
	for i := range want {
		if math.Abs(got[i]-want[i]) > 1e-9 {
			t.Errorf("height %d: %.12f, want %.12f", i, got[i], want[i])
		}
	}
}

func BenchmarkHeight(b *testing.B) {
	set := DefaultSettings()
	r := NewWorld(7, &set).Relief
	dirs := goldenDirs()
	b.ReportAllocs()
	for i := range b.N {
		r.Height(dirs[i%len(dirs)])
	}
}
