package audio

import (
	"io/fs"
	"log"
	"math"
	"math/rand/v2"
	"naturalia/assets"

	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	sampleRate  = 44100
	audioBuffer = 2048 // frames per stream refill, ~46 ms

	maxVoices  = 32   // one-shot sounds at once; more are dropped
	limit      = 0.7  // the limiter keeps the mix under -3 dBFS...
	hardLimit  = 0.9  // ...and no sample ever goes past this
	fadeInTime = 1.0  // seconds: the sound never starts at full volume
	release    = 0.25 // seconds for the limiter to let go
)

type bus int

const (
	busAmbience bus = iota
	busEffects
	busMusic
	busCount
)

// Mixer plays the game's sound files through one stereo raylib stream:
// ambience loops, footsteps, the tool, the music and the interface beeps
// are mixed here, in Go, then go through a limiter and a hard clamp, so no
// mistake anywhere can make a loud sound.
type Mixer struct {
	stream rl.AudioStream
	fsys   fs.FS
	buf    []float32
	rng    *rand.Rand

	// What the game asks for, read at each Update.
	Outside  bool
	Sweeping bool
	Running  bool
	Power    float64 // tool strength, raises the motor's pitch

	Master  float64 // the player's volume, 0.5 by default
	BusGain [busCount]float64
	muted   bool
	mute    float64 // eased toward 0 or 1
	fade    float64 // start-up fade, 0 to 1
	limGain float64 // the limiter's current gain

	windOut, windMars, breath, machines, vacuum *stream
	loops                                       []*stream
	voices                                      []*stream // one-shots: steps, airlock
	beeps                                       []beep

	steps     map[stepFile][][]float32
	lastStep  int
	airlockCD float64 // seconds before the airlock can sound again

	music     *stream
	track     int
	musicWait float64 // seconds of silence before the next track
}

// beep is a short tone: the interface sounds that have no file yet.
type beep struct{ freq, t, dur, gain, delay float64 }

// New opens the audio device and starts the stream.
func New() *Mixer {
	rl.InitAudioDevice()
	rl.SetAudioStreamBufferSizeDefault(audioBuffer)
	m := newMixer(assets.FS)
	m.stream = rl.LoadAudioStream(sampleRate, 32, 2)
	rl.PlayAudioStream(m.stream)
	return m
}

// newMixer loads the sounds of fsys; it needs no audio device.
func newMixer(fsys fs.FS) *Mixer {
	m := &Mixer{
		fsys:      fsys,
		buf:       make([]float32, audioBuffer*2),
		rng:       rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
		Master:    0.5,
		BusGain:   [busCount]float64{1, 1, 0.8},
		mute:      1,
		limGain:   1,
		lastStep:  -1,
		musicWait: 40, // a little while in the wind first
		steps:     map[stepFile][][]float32{},
	}
	loop := func(s sound, b bus) *stream {
		d, err := open(fsys, s.file)
		if err != nil {
			log.Println("audio:", err)
			return nil
		}
		st := newStream(d, b, s.level(), true)
		m.loops = append(m.loops, st)
		return st
	}
	m.windOut = loop(windOut, busAmbience)
	m.windMars = loop(windMars, busAmbience)
	m.breath = loop(breath, busAmbience)
	m.machines = loop(machines, busAmbience)
	m.vacuum = loop(vacuum, busEffects)
	if m.vacuum != nil {
		m.vacuum.setGlide(0.15)
	}
	for _, f := range []stepFile{stepsDust, stepsRun, stepsHard} {
		s, err := decodeAll(fsys, f.file)
		if err != nil {
			log.Println("audio:", err)
			continue
		}
		m.steps[f] = sliceSteps(s)
	}
	m.track = m.rng.IntN(len(musicTrks))
	return m
}

// Update sets the loops from the game's state and refills the stream
// whenever it has played a buffer.
func (m *Mixer) Update() {
	if m == nil {
		return
	}
	for rl.IsAudioStreamProcessed(m.stream) {
		m.fill(m.buf)
		// raylib-go counts the slice's length as frames: give it one
		// element per frame, the stereo samples lie behind in the array.
		rl.UpdateAudioStream(m.stream, m.buf[:len(m.buf)/2])
	}
}

// ToggleMute cuts or brings back all sound (M), with a short fade.
func (m *Mixer) ToggleMute() {
	if m != nil {
		m.muted = !m.muted
	}
}

// Beep plays a sequence of notes (Hz), one every gap seconds.
func (m *Mixer) Beep(gap, dur, gain float64, notes ...float64) {
	if m == nil { // tests have no sound
		return
	}
	for i, f := range notes {
		if len(m.beeps) < maxVoices {
			m.beeps = append(m.beeps, beep{freq: f, dur: dur, gain: math.Min(gain, 0.3), delay: float64(i) * gap})
		}
	}
}

// Step plays a footstep: on dust, or on a hard floor (the base, a block).
func (m *Mixer) Step(hard bool) {
	if m == nil {
		return
	}
	f := stepsDust
	switch {
	case hard:
		f = stepsHard
	case m.Running:
		f = stepsRun
	}
	steps := m.steps[f]
	if len(steps) == 0 {
		return
	}
	i := m.rng.IntN(len(steps))
	if i == m.lastStep && len(steps) > 1 { // never the same twice in a row
		i = (i + 1) % len(steps)
	}
	m.lastStep = i
	st := m.play(&memDecoder{s: steps[i]}, busEffects, math.Pow(10, f.peak/20))
	if st != nil {
		st.Pitch = 0.94 + 0.12*m.rng.Float64()
		st.Gain = 0.8 + 0.2*m.rng.Float64()
		if !m.Outside && !hard {
			st.Gain *= 0.6
		}
	}
}

// Airlock plays the air rushing through the airlock as its doors move.
func (m *Mixer) Airlock() {
	if m == nil || m.airlockCD > 0 {
		return
	}
	d, err := open(m.fsys, airlock.file)
	if err != nil {
		return
	}
	m.play(d, busEffects, airlock.level())
	m.airlockCD = 3
}

// play starts a one-shot sound at full gain.
func (m *Mixer) play(d decoder, b bus, level float64) *stream {
	if len(m.voices) >= maxVoices {
		return nil
	}
	st := newStream(d, b, level, false)
	st.Gain, st.gain = 1, 1
	m.voices = append(m.voices, st)
	return st
}

// steer sets the loops' gains from what the game asked; dt is the
// duration of the buffer about to be filled.
func (m *Mixer) steer(dt float64) {
	set := func(s *stream, gain, cut, pitch float64) {
		if s != nil {
			s.Gain, s.Cut, s.Pitch = gain, cut, pitch
		}
	}
	if m.Outside {
		set(m.windOut, 1, 0, 1)
		set(m.windMars, 1, 0, 1)
		run := 0.0
		if m.Running {
			run = 1
		}
		set(m.breath, 1+0.5*run, 0, 1+0.08*run) // in the helmet
		set(m.machines, 0, 0, 1)
	} else {
		set(m.windOut, 0.3, 350, 1) // through the walls
		set(m.windMars, 0, 350, 1)
		set(m.breath, 0, 0, 1)
		set(m.machines, 1, 0, 1)
	}
	tool := 0.0
	if m.Sweeping {
		tool = 1
	}
	set(m.vacuum, tool, 0, 0.9+0.2*math.Max(0, math.Min(1, m.Power)))
	m.airlockCD -= dt

	// Music: a track, then a few minutes of silence, then the next one.
	if m.music != nil && m.music.done {
		m.music = nil
		m.track = (m.track + 1) % len(musicTrks)
		m.musicWait = 180 + 240*m.rng.Float64()
	}
	if m.music == nil {
		if m.musicWait -= dt; m.musicWait <= 0 {
			t := musicTrks[m.track]
			if d, err := open(m.fsys, t.file); err == nil {
				m.music = newStream(d, busMusic, t.level(), false)
				m.music.setGlide(4) // fades in
				m.music.Gain = 1
			} else {
				m.musicWait = 60
			}
		}
	}
}

// fill mixes one buffer of interleaved stereo samples.
func (m *Mixer) fill(out []float32) {
	frames := len(out) / 2
	m.steer(float64(frames) / sampleRate)
	const dt = 1.0 / sampleRate
	rel := 1 - math.Exp(-dt/release)
	muteTarget := 1.0
	if m.muted {
		muteTarget = 0
	}
	active := make([]*stream, 0, len(m.loops)+len(m.voices)+1)
	for _, s := range m.loops {
		if !s.idle() {
			active = append(active, s)
		}
	}
	active = append(active, m.voices...)
	if m.music != nil {
		active = append(active, m.music)
	}
	master := math.Max(0, math.Min(1, m.Master))
	for i := range frames {
		var bus [busCount][2]float64
		for _, s := range active {
			if s.done {
				continue
			}
			l, r, ok := s.next()
			if ok {
				bus[s.bus][0] += l
				bus[s.bus][1] += r
			}
		}
		for k := range m.beeps {
			b := &m.beeps[k]
			if b.delay > 0 {
				b.delay -= dt
				continue
			}
			b.t += dt
			if b.t >= b.dur {
				continue
			}
			env := math.Min(1, b.t/0.005) * (1 - b.t/b.dur)
			v := math.Sin(2*math.Pi*b.freq*b.t) * env * env * b.gain
			bus[busEffects][0] += v
			bus[busEffects][1] += v
		}

		m.fade = math.Min(1, m.fade+dt/fadeInTime)
		m.mute += (muteTarget - m.mute) * dt * 20
		g := master * m.fade * m.mute
		var l, r float64
		for b := range busCount {
			bg := math.Max(0, math.Min(1, m.BusGain[b]))
			l += bus[b][0] * bg
			r += bus[b][1] * bg
		}
		l, r = l*g, r*g

		// Limiter: drops at once to keep the peak under limit, recovers slowly.
		peak := math.Max(math.Abs(l), math.Abs(r))
		if peak*m.limGain > limit {
			m.limGain = limit / peak
		} else {
			m.limGain += (1 - m.limGain) * rel
		}
		out[i*2] = clamp(l * m.limGain)
		out[i*2+1] = clamp(r * m.limGain)
	}

	alive := m.voices[:0]
	for _, s := range m.voices {
		if !s.done {
			alive = append(alive, s)
		}
	}
	clear(m.voices[len(alive):])
	m.voices = alive
	beeps := m.beeps[:0]
	for _, b := range m.beeps {
		if b.delay > 0 || b.t < b.dur {
			beeps = append(beeps, b)
		}
	}
	m.beeps = beeps
}

// clamp is the last barrier: never past hardLimit, silence for NaN.
func clamp(v float64) float32 {
	if math.IsNaN(v) {
		return 0
	}
	return float32(math.Max(-hardLimit, math.Min(hardLimit, v)))
}
