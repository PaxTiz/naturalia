package audio

import "math"

// sliceSteps cuts a recording of footsteps into one sound per step: a step
// starts where the loudness rises above a quarter of the loudest, at least
// 180 ms after the previous one. Each step is faded at both ends and
// brought to a peak of 1; the faint ones are dropped.
func sliceSteps(s []float32) [][]float32 {
	const win = sampleRate / 100 // 10 ms
	frames := len(s) / 2
	env := make([]float64, frames/win)
	top := 0.0
	for k := range env {
		for i := k * win; i < (k+1)*win; i++ {
			env[k] = math.Max(env[k], math.Abs(float64(s[i*2]+s[i*2+1]))/2)
		}
		top = math.Max(top, env[k])
	}
	var onsets []int
	armed := true
	for k, e := range env {
		if armed && e > 0.25*top && (len(onsets) == 0 || k-onsets[len(onsets)-1] >= 18) {
			onsets = append(onsets, k)
			armed = false
		}
		if e < 0.12*top {
			armed = true
		}
	}
	var steps [][]float32
	for j, k := range onsets {
		from := max(0, k-1) * win
		to := min(frames, from+sampleRate*45/100)
		if j+1 < len(onsets) {
			to = min(to, (onsets[j+1]-1)*win)
		}
		step := append([]float32(nil), s[from*2:to*2]...)
		peak := 0.0
		for _, v := range step {
			peak = math.Max(peak, math.Abs(float64(v)))
		}
		if peak < 0.3*top || to-from < sampleRate/10 {
			continue
		}
		n := len(step) / 2
		fadeIn, fadeOut := sampleRate/500, min(n/3, sampleRate/20)
		for i := range n {
			g := 1 / peak
			if i < fadeIn {
				g *= float64(i) / float64(fadeIn)
			}
			if r := n - 1 - i; r < fadeOut {
				g *= float64(r) / float64(fadeOut)
			}
			step[i*2] *= float32(g)
			step[i*2+1] *= float32(g)
		}
		steps = append(steps, step)
	}
	return steps
}
