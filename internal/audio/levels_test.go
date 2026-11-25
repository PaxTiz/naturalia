package audio

import (
	"io/fs"
	"math"
	"naturalia/assets"
	"path"
	"testing"
)

// measure returns a decoded file's RMS and peak, in dBFS.
func measure(s []float32) (rms, peak float64) {
	var sum float64
	for _, v := range s {
		sum += float64(v * v)
		peak = math.Max(peak, math.Abs(float64(v)))
	}
	return 20 * math.Log10(math.Sqrt(sum/float64(len(s)))), 20 * math.Log10(peak)
}

// TestLevels decodes every file of the sounds table: a file louder than
// its measure, or a file of the assets folder played without one, fails.
func TestLevels(t *testing.T) {
	listed := map[string]bool{}
	for _, s := range append([]sound{windOut, windMars, breath, machines, vacuum, airlock}, musicTrks...) {
		listed[s.file] = true
		d, err := decodeAll(assets.FS, s.file)
		if err != nil {
			t.Fatal(err)
		}
		rms, peak := measure(d)
		if math.Abs(rms-s.rms) > 0.5 || math.Abs(peak-s.peak) > 0.5 {
			t.Errorf("%s measures rms %.1f peak %.1f dB, the table says %.1f %.1f", s.file, rms, peak, s.rms, s.peak)
		}
		if out := 20 * math.Log10(s.level()); peak+out > ceiling+0.1 { // the table is rounded to 0.1 dB
			t.Errorf("%s peaks at %.1f dB once leveled", s.file, peak+out)
		}
	}
	for _, f := range []stepFile{stepsDust, stepsRun, stepsHard} {
		listed[f.file] = true
		d, err := decodeAll(assets.FS, f.file)
		if err != nil {
			t.Fatal(err)
		}
		steps := sliceSteps(d)
		if len(steps) < 3 {
			t.Errorf("%s: %d steps cut, want several", f.file, len(steps))
		}
		for i, s := range steps {
			if _, p := measure(s); p > 0.01 {
				t.Errorf("%s step %d peaks at %.1f dB", f.file, i, p)
			}
		}
	}
	fs.WalkDir(assets.FS, ".", func(p string, d fs.DirEntry, err error) error {
		if ext := path.Ext(p); (ext == ".mp3" || ext == ".wav") && !listed[p] {
			t.Errorf("%s is not in the sounds table", p)
		}
		return nil
	})
}

// TestMixerCeiling plays everything at once, at full volume, and checks
// that no sample goes past the limiter.
func TestMixerCeiling(t *testing.T) {
	m := newMixer(assets.FS)
	m.Master = 1
	m.BusGain = [busCount]float64{1, 1, 1}
	m.Outside, m.Sweeping, m.Running, m.Power = true, true, true, 1
	m.musicWait = 0
	buf := make([]float32, audioBuffer*2)
	peak := 0.0
	for k := range 200 { // ~9 s
		if k%3 == 0 {
			m.Step(k%2 == 0)
			m.Airlock()
			m.Under = k > 100
			m.Beep(0.01, 0.2, 10, 440, 880, 1760)
		}
		m.fill(buf)
		for _, v := range buf {
			if math.IsNaN(float64(v)) {
				t.Fatal("NaN in the mix")
			}
			peak = math.Max(peak, math.Abs(float64(v)))
		}
	}
	if peak > limit+1e-3 {
		t.Errorf("mix peaks at %.3f, limit %.2f", peak, limit)
	}
	t.Logf("peak %.3f (%.1f dBFS)", peak, 20*math.Log10(peak))
}
