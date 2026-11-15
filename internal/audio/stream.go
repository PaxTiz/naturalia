package audio

import "math"

// stream is a sound being played: read from its decoder, resampled to the
// mixer's rate at a pitch, low-passed and faded to the gain asked.
type stream struct {
	dec  decoder
	bus  bus
	loop bool
	done bool

	Gain  float64 // asked by the game; the stream glides to it
	Pitch float64 // 1 plays at the file's speed
	Cut   float64 // low-pass cutoff (Hz), 0 for none

	level float64 // file normalization, from the sounds table
	gain  float64 // current gain
	glide float64 // per-sample step toward Gain
	lpA   float64 // current low-pass coefficient, 1 is open
	lp    [2]float64

	buf  []float32 // native frames, interleaved
	n, i int       // frames in buf, next one to read
	a, b [2]float32
	pos  float64 // between a (0) and b (1)
	step float64 // native frames per output frame, before the pitch
}

func newStream(d decoder, b bus, level float64, loop bool) *stream {
	s := &stream{dec: d, bus: b, loop: loop, level: level, Pitch: 1, lpA: 1,
		buf: make([]float32, 2048), step: float64(d.rate()) / sampleRate}
	s.setGlide(0.08)
	s.pos = 1 // read the first frame into b at the first sample
	return s
}

// setGlide sets how long a gain change takes, in seconds.
func (s *stream) setGlide(sec float64) { s.glide = 1 / (sec * sampleRate) }

// idle streams are not decoded: a loop at gain 0 stays where it was.
func (s *stream) idle() bool { return s.Gain <= 0 && s.gain <= 1e-5 }

// next returns the next output frame, false when the sound has ended.
func (s *stream) next() (l, r float64, ok bool) {
	for s.pos >= 1 {
		s.pos--
		s.a = s.b
		if !s.advance() {
			s.done = true
			return 0, 0, false
		}
	}
	f := float32(s.pos)
	l = float64(s.a[0] + (s.b[0]-s.a[0])*f)
	r = float64(s.a[1] + (s.b[1]-s.a[1])*f)
	s.pos += s.step * math.Max(0.25, math.Min(4, s.Pitch))

	// Gain: a linear glide, never a jump (no click).
	target := math.Max(0, math.Min(4, s.Gain))
	if math.IsNaN(target) {
		target = 0
	}
	if d := target - s.gain; math.Abs(d) <= s.glide {
		s.gain = target
	} else {
		s.gain += math.Copysign(s.glide, d)
	}
	// Low-pass: one pole, its coefficient eased to the cutoff asked.
	a := 1.0
	if s.Cut > 0 {
		a = 1 - math.Exp(-2*math.Pi*s.Cut/sampleRate)
	}
	s.lpA += (a - s.lpA) * 0.0005
	s.lp[0] += (l - s.lp[0]) * s.lpA
	s.lp[1] += (r - s.lp[1]) * s.lpA
	g := s.gain * s.level
	return s.lp[0] * g, s.lp[1] * g, true
}

// advance loads the next native frame into b.
func (s *stream) advance() bool {
	if s.i >= s.n {
		s.n, s.i = s.dec.read(s.buf), 0
		if s.n == 0 && s.loop {
			if s.dec.rewind() != nil {
				return false
			}
			s.n = s.dec.read(s.buf)
		}
		if s.n == 0 {
			return false
		}
	}
	s.b = [2]float32{s.buf[s.i*2], s.buf[s.i*2+1]}
	s.i++
	return true
}

// memDecoder plays a sound decoded in memory, at the mixer's rate.
type memDecoder struct {
	s []float32
	i int
}

func (m *memDecoder) rate() int     { return sampleRate }
func (m *memDecoder) rewind() error { m.i = 0; return nil }
func (m *memDecoder) read(dst []float32) int {
	n := copy(dst, m.s[m.i*2:]) / 2
	m.i += n
	return n
}
