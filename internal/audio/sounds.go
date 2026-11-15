package audio

import "math"

// sound is a file of the assets folder with its measured level, so every
// file plays at the level of its role whatever it was recorded at.
// TestLevels decodes each file and fails if the numbers here are wrong:
// a louder file put in place of one is caught before it is heard.
type sound struct {
	file      string
	rms, peak float64 // measured over the whole file, dBFS
	target    float64 // the role's level, dBFS RMS
}

// No file plays above this peak, whatever its target.
const ceiling = -1.0

// level is the gain bringing the file to its target, never above the ceiling.
func (s sound) level() float64 {
	db := math.Min(s.target-s.rms, ceiling-s.peak)
	return math.Pow(10, db/20)
}

// The targets, before the buses and the master volume (50 % by default).
var (
	windOut   = sound{"sounds/wind_ambiant.mp3", -29.4, -2.5, -26}
	windMars  = sound{"sounds/wind_mars.wav", -21.3, -0.0, -30} // Perseverance, under the other
	breath    = sound{"sounds/suit_breath.mp3", -48.9, -33.1, -40}
	machines  = sound{"sounds/machines.mp3", -33.7, -19.0, -30}
	vacuum    = sound{"sounds/vacuum_cleaner.mp3", -12.6, -0.1, -24}
	airlock   = sound{"sounds/suit_air_release.mp3", -31.6, -4.2, -26}
	musicTrks = []sound{
		{"musics/mesmerizing_galaxy/music.mp3", -13.6, -1.1, -21},
		{"musics/dreams_become_real/music.mp3", -26.6, -9.4, -21},
	}
)

// Footsteps: files of several steps, cut into one sound per step, each
// brought to the same peak.
var (
	stepsDust = stepFile{"sounds/footsteps_gravel.mp3", -18}
	stepsRun  = stepFile{"sounds/footsteps_gravel_fast.mp3", -18}
	stepsHard = stepFile{"sounds/footsteps_concrete.mp3", -16}
)

type stepFile struct {
	file string
	peak float64 // dBFS of each step
}

// Credits of the files played, shown in the pause screen.
const Credits = "Musique : « Mesmerizing Galaxy », « Dreams Become Real », Kevin MacLeod (incompetech.com), CC BY 4.0"
