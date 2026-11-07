package main

import (
	"math"
	"math/rand/v2"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	sampleRate  = 44100
	audioBuffer = 2048 // samples per stream refill, ~46 ms
)

// Every sound is synthesized here, sample by sample: no audio file.
// One mono stream mixes the wind, the sweeping tool, footsteps and the
// interface beeps.
type synth struct {
	stream rl.AudioStream
	buf    []float32
	rng    *rand.Rand

	// What the game asks for, read when filling the buffer.
	outside  bool
	sweeping bool
	power    float64 // tool strength, raises the motor's pitch

	// Wind: low-passed noise with slow gusts.
	windLP, windLP2, gust, gustTarget float64
	gustTimer                         float64
	// Sweeping tool: a motor hum plus a hiss, with an envelope.
	toolEnv, motorPhase, hissLP float64
	// Short sounds in flight.
	voices []voice
}

// voice is a short sound: a sine tone, or a muffled thud of noise when
// freq is 0.
type voice struct {
	freq, t, dur, gain, delay float64
	lp                        float64
}

func newSynth() *synth {
	rl.InitAudioDevice()
	rl.SetAudioStreamBufferSizeDefault(audioBuffer)
	s := &synth{
		stream: rl.LoadAudioStream(sampleRate, 32, 1),
		buf:    make([]float32, audioBuffer),
		rng:    rand.New(rand.NewPCG(7, 11)),
	}
	rl.PlayAudioStream(s.stream)
	return s
}

// update refills the stream whenever it has played a buffer.
func (s *synth) update() {
	for rl.IsAudioStreamProcessed(s.stream) {
		s.fill()
		rl.UpdateAudioStream(s.stream, s.buf)
	}
}

// beep plays a sequence of notes (Hz), one every gap seconds.
func (s *synth) beep(gap, dur, gain float64, notes ...float64) {
	for i, f := range notes {
		s.voices = append(s.voices, voice{freq: f, dur: dur, gain: gain, delay: float64(i) * gap})
	}
}

// step plays a soft footstep in the dust.
func (s *synth) step() {
	s.voices = append(s.voices, voice{dur: 0.12, gain: 0.35 + 0.1*s.rng.Float64()})
}

func (s *synth) fill() {
	const dt = 1.0 / sampleRate
	windGain, windCut := 0.0, 0.0
	if s.outside {
		windGain, windCut = 0.22, 0.02
	} else {
		windGain, windCut = 0.05, 0.006 // heard through the walls
	}
	toolTarget := 0.0
	if s.sweeping {
		toolTarget = 1
	}
	for i := range s.buf {
		n := s.rng.Float64()*2 - 1

		// Wind: two low-pass stages make a soft rumble; gusts wander.
		s.gustTimer -= dt
		if s.gustTimer <= 0 {
			s.gustTarget = 0.3 + 0.7*s.rng.Float64()
			s.gustTimer = 1.5 + 3*s.rng.Float64()
		}
		s.gust += (s.gustTarget - s.gust) * dt * 0.6
		cut := windCut * (0.5 + s.gust)
		s.windLP += (n - s.windLP) * cut
		s.windLP2 += (s.windLP - s.windLP2) * cut
		out := s.windLP2 * windGain * s.gust * 6

		// Sweeping tool: quick attack, slower release.
		rate := 12.0
		if toolTarget < s.toolEnv {
			rate = 5
		}
		s.toolEnv += (toolTarget - s.toolEnv) * dt * rate
		if s.toolEnv > 0.001 {
			freq := 95 + 25*s.power
			s.motorPhase = math.Mod(s.motorPhase+freq*dt, 1)
			motor := math.Sin(2*math.Pi*s.motorPhase) + 0.35*math.Sin(4*math.Pi*s.motorPhase) + 0.15*(2*s.motorPhase-1)
			s.hissLP += (n - s.hissLP) * 0.25
			out += s.toolEnv * (motor*0.07 + s.hissLP*0.09)
		}

		// Short sounds.
		for k := range s.voices {
			v := &s.voices[k]
			if v.delay > 0 {
				v.delay -= dt
				continue
			}
			v.t += dt
			if v.t >= v.dur {
				continue
			}
			env := math.Min(1, v.t/0.005) * (1 - v.t/v.dur)
			if v.freq > 0 {
				out += math.Sin(2*math.Pi*v.freq*v.t) * env * env * v.gain
			} else {
				v.lp += (n - v.lp) * 0.04
				out += v.lp * env * env * v.gain
			}
		}
		s.buf[i] = float32(math.Tanh(out * 1.2)) // soft limit
	}
	// Forget finished sounds.
	alive := s.voices[:0]
	for _, v := range s.voices {
		if v.delay > 0 || v.t < v.dur {
			alive = append(alive, v)
		}
	}
	s.voices = alive
}
