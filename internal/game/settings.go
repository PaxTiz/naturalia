package game

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"path/filepath"
)

// The sound settings, in the order of the pause screen's panel.
const (
	SoundMaster = iota
	SoundMusic
	SoundVacuum
	SoundSettings
)

var SoundNames = [SoundSettings]string{"Volume général", "Musique", "Aspirateur"}

// Settings are the player's preferences, kept in settings.json next to the
// save. A volume is the slider's position, 0 to 1; the gain is its square,
// so the low end of the slider goes very quiet.
type Settings struct {
	Volume float64 `json:"volume"`
	Music  float64 `json:"music"`
	Vacuum float64 `json:"vacuum"`
}

var defaultSettings = Settings{Volume: 0.7, Music: 0.9, Vacuum: 1}

func (s *Settings) sound(i int) *float64 {
	return [SoundSettings]*float64{&s.Volume, &s.Music, &s.Vacuum}[i]
}

// Sound is a sound setting's slider position.
func (g *Game) Sound(i int) float64 { return *g.Settings.sound(i) }

// SetSound moves a sound slider, by steps of 5 %.
func (g *Game) SetSound(i int, v float64) {
	v = math.Round(math.Max(0, math.Min(1, v))*20) / 20
	if p := g.Settings.sound(i); *p != v {
		*p = v
		g.settingsChanged = true
		g.applySettings()
	}
}

func (g *Game) applySettings() {
	s := g.Settings
	g.sfx.SetVolumes(s.Volume*s.Volume, s.Music*s.Music, s.Vacuum*s.Vacuum)
}

func (g *Game) loadSettings() {
	g.Settings = defaultSettings
	if dir, err := saveDir(); err == nil {
		if data, err := os.ReadFile(filepath.Join(dir, "settings.json")); err == nil {
			if err := json.Unmarshal(data, &g.Settings); err != nil {
				log.Println("settings.json:", err)
				g.Settings = defaultSettings
			}
		}
	}
	for i := range SoundSettings {
		p := g.Settings.sound(i)
		*p = math.Max(0, math.Min(1, *p))
	}
	g.applySettings()
}

// SaveSettings writes settings.json if a setting changed.
func (g *Game) SaveSettings() {
	if !g.settingsChanged {
		return
	}
	dir, err := saveDir()
	if err == nil {
		err = os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		log.Println("settings:", err)
		return
	}
	data, _ := json.MarshalIndent(g.Settings, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), data, 0o644); err != nil {
		log.Println("settings:", err)
		return
	}
	g.settingsChanged = false
}
