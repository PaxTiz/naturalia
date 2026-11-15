package game

import (
	"encoding/json"
	"log"
	"math"
	"os"
	"path/filepath"

	rl "github.com/gen2brain/raylib-go/raylib"
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
	// Mouse is the mouse's sensitivity, 1 by default; InvertY looks down
	// when the mouse goes up.
	Mouse      float64 `json:"mouse"`
	InvertY    bool    `json:"invert_y,omitempty"`
	Fullscreen bool    `json:"fullscreen,omitempty"`
	// Keys are the keys the player changed, by action (keys.go); KeyNames
	// the labels learned from the keyboard.
	Keys     map[string]int32 `json:"keys,omitempty"`
	KeyNames map[int32]string `json:"key_names,omitempty"`
}

var defaultSettings = Settings{Volume: 0.7, Music: 0.9, Vacuum: 1, Mouse: 1}

// The mouse's sensitivity's range, and its steps.
const (
	MouseMin, MouseMax, MouseStep = 0.25, 3, 0.05
)

// lookX and lookY turn the view by the mouse's motion (applySettings).
var lookX, lookY = mouseSens, mouseSens

// SetMouse changes the mouse's sensitivity.
func (g *Game) SetMouse(v float64) {
	v = math.Round(math.Max(MouseMin, math.Min(MouseMax, v))/MouseStep) * MouseStep
	if g.Settings.Mouse != v {
		g.Settings.Mouse = v
		g.settingsChanged = true
		g.applySettings()
	}
}

// SetInvertY turns the vertical look upside down, or back.
func (g *Game) SetInvertY(on bool) {
	if g.Settings.InvertY != on {
		g.Settings.InvertY = on
		g.settingsChanged = true
		g.applySettings()
	}
}

// ToggleFullscreen switches between a window and the whole screen.
func (g *Game) ToggleFullscreen() {
	rl.ToggleBorderlessWindowed()
	g.Settings.Fullscreen = !g.Settings.Fullscreen
	g.settingsChanged = true
}

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
	lookX = mouseSens * s.Mouse
	lookY = lookX
	if s.InvertY {
		lookY = -lookY
	}
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
	if g.Settings.Mouse == 0 { // a settings.json from before
		g.Settings.Mouse = 1
	}
	g.Settings.Mouse = math.Max(MouseMin, math.Min(MouseMax, g.Settings.Mouse))
	for k, s := range g.Settings.KeyNames {
		keyNames[k] = s
	}
	g.loadKeys()
	g.applySettings()
	if g.Settings.Fullscreen {
		rl.ToggleBorderlessWindowed()
	}
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
