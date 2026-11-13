package game

import (
	"fmt"
	"math"
	"naturalia/internal/content"
	"naturalia/internal/mathx"
	"os"
	"path/filepath"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The photo mode's settings, indexes in Photo.Set.
const (
	PhotoExposure   = iota // stops added to the eye's exposure
	PhotoFocus             // index in FocusDistances, 0 is autofocus
	PhotoAperture          // depth of field blur, 0 is sharp everywhere
	PhotoFilter            // index in PhotoFilters
	PhotoContrast          // -1 to 1
	PhotoSaturation        // -1 to 1
	PhotoVignette          // darkening of the corners
	PhotoGrain             // film grain
	PhotoFrame             // index in PhotoFrames
	PhotoGrid              // rule of thirds, never in the picture
	PhotoTime              // 0 frozen, 1 running
	PhotoSettings
)

// PhotoSetting describes one line of the photo mode's panel: Labels name
// the steps of a choice, otherwise the value runs from Min to Max by Step
// and Format shows it.
type PhotoSetting struct {
	Name           string
	Min, Max, Step float64
	Default        float64
	Labels         []string
	Format         func(v float64) string
}

func percent(v float64) string { return fmt.Sprintf("%.0f %%", v*100) }
func signed(v float64) string  { return fmt.Sprintf("%+.0f", v*100) }

// FocusDistances are the manual focus steps, in meters (the first is auto).
var FocusDistances = []float64{0, 0.5, 0.75, 1, 1.5, 2, 3, 4, 6, 8, 12, 16, 25, 35, 50, 75, 100, 150, 250, 500}

// PhotoFilters are the looks graded in image.frag, in its order.
var PhotoFilters = []string{"Naturel", "Doré", "Crépuscule", "Ciné", "Noir et blanc", "Argentique"}

// PhotoFrames are the picture's proportions (width / height), 0 for the
// window's own: the view is cropped to them by black bars, and so is the
// saved picture.
var PhotoFrames = []struct {
	Name   string
	Aspect float64
}{{"Écran", 0}, {"16:9", 16.0 / 9}, {"2,39:1", 2.39}, {"1,85:1", 1.85}, {"3:2", 1.5}, {"4:3", 4.0 / 3}, {"1:1", 1}, {"4:5", 0.8}, {"9:16", 9.0 / 16}}

var PhotoSettingDefs = func() [PhotoSettings]PhotoSetting {
	var d [PhotoSettings]PhotoSetting
	focus := make([]string, len(FocusDistances))
	focus[0] = "Auto (centre)"
	for i, m := range FocusDistances[1:] {
		focus[i+1] = fmt.Sprintf("%g m", m)
	}
	frames := make([]string, len(PhotoFrames))
	for i, f := range PhotoFrames {
		frames[i] = f.Name
	}
	d[PhotoExposure] = PhotoSetting{Name: "Exposition", Min: -3, Max: 3, Step: 1.0 / 3, Format: func(v float64) string { return fmt.Sprintf("%+.1f IL", v) }}
	d[PhotoFocus] = PhotoSetting{Name: "Mise au point", Labels: focus}
	d[PhotoAperture] = PhotoSetting{Name: "Flou d'arrière-plan", Max: 1, Step: 0.1, Format: percent}
	d[PhotoFilter] = PhotoSetting{Name: "Filtre", Labels: PhotoFilters}
	d[PhotoContrast] = PhotoSetting{Name: "Contraste", Min: -1, Max: 1, Step: 0.1, Format: signed}
	d[PhotoSaturation] = PhotoSetting{Name: "Saturation", Min: -1, Max: 1, Step: 0.1, Format: signed}
	d[PhotoVignette] = PhotoSetting{Name: "Vignettage", Max: 1.5, Step: 0.15, Default: 0.45, Format: func(v float64) string { return percent(v / 1.5) }}
	d[PhotoGrain] = PhotoSetting{Name: "Grain", Max: 1, Step: 0.1, Default: 0.2, Format: percent}
	d[PhotoFrame] = PhotoSetting{Name: "Cadrage", Labels: frames}
	d[PhotoGrid] = PhotoSetting{Name: "Grille des tiers", Labels: []string{"Non", "Oui"}}
	d[PhotoTime] = PhotoSetting{Name: "Temps", Labels: []string{"Figé", "En marche"}}
	for i := range d {
		if d[i].Labels != nil {
			d[i].Max, d[i].Step = float64(len(d[i].Labels)-1), 1
		}
	}
	return d
}()

const (
	photoTether = 60.0 // the camera stays this close to the player, where the world is detailed
	photoSpeed  = 4.0  // m/s, Shift ×4, Alt ×¼
	photoRoll   = 0.6  // rad/s
)

// Photo is the photo mode: a free camera, detached from the player, and
// the picture's settings. The settings are kept from one photo to the next.
type Photo struct {
	Pos         vec3 // the camera's eye
	Heading     vec3 // tangent to the ground, like the player's
	Pitch, Roll float64
	Fov         float64 // vertical, degrees
	Set         [PhotoSettings]float64
	ready       bool // Set holds the defaults
}

func (p *Photo) up() vec3 { return p.Pos.Norm() }

func (p *Photo) Forward() vec3 {
	return p.Heading.Scale(math.Cos(p.Pitch)).Add(p.up().Scale(math.Sin(p.Pitch)))
}

func (p *Photo) Right() vec3 { return p.Heading.Cross(p.up()).Norm() }

// CamUp is the camera's up, tilted by the roll.
func (p *Photo) CamUp() vec3 {
	r := p.Right()
	u := r.Cross(p.Forward())
	return u.Scale(math.Cos(p.Roll)).Add(r.Scale(math.Sin(p.Roll)))
}

// FocalMM is the field of view as a full frame lens's focal length.
func (p *Photo) FocalMM() float64 { return 12 / math.Tan(p.Fov*math.Pi/360) }

// Adjust steps setting i by n steps, within its range.
func (p *Photo) Adjust(i, n int) {
	d := PhotoSettingDefs[i]
	v := p.Set[i] + float64(n)*d.Step
	if d.Labels != nil && i != PhotoFocus { // choices wrap around
		k := float64(len(d.Labels))
		v = math.Mod(v+k, k)
	}
	p.Set[i] = math.Max(d.Min, math.Min(d.Max, math.Round(v/d.Step)*d.Step))
}

// Reset puts setting i back to its default.
func (p *Photo) Reset(i int) { p.Set[i] = PhotoSettingDefs[i].Default }

// Frozen tells if the world stands still for the photo.
func (p *Photo) Frozen() bool { return p.Set[PhotoTime] == 0 }

// Aspect is the picture's proportions, 0 for the window's.
func (p *Photo) Aspect() float64 { return PhotoFrames[int(p.Set[PhotoFrame])].Aspect }

// EnterPhoto starts the photo mode from the player's eye.
func (g *Game) EnterPhoto() {
	ph := &g.Photo
	if !ph.ready {
		for i := range ph.Set {
			ph.Reset(i)
		}
		ph.ready = true
	}
	ph.Pos, ph.Heading, ph.Pitch, ph.Roll = g.Player.Eye(), g.Player.Heading, g.Player.Pitch, 0
	ph.Fov = content.OrZero(g.Fov, 70)
	g.Sweeping = false
	g.setMode(ModePhoto)
}

// updatePhoto flies the photo camera: like the creative flight, plus the
// roll (A and E on AZERTY) and the zoom (wheel). The panel's settings are
// the interface's (ui). It returns whether the world skips this frame:
// frozen, or just back to playing (the key that left would act again).
func (g *Game) updatePhoto(dt float64) bool {
	ph := &g.Photo
	if rl.IsKeyPressed(rl.KeyP) || rl.IsKeyPressed(rl.KeyEscape) {
		g.setMode(ModePlay)
		return true
	}
	if rl.IsWindowFocused() {
		up := ph.up()
		d := rl.GetMouseDelta()
		// Slower turns when zoomed in, so framing stays precise.
		sens := mouseSens * ph.Fov / 70
		turn := -float64(d.X) * sens
		ph.Heading = ph.Heading.Scale(math.Cos(turn)).Add(up.Cross(ph.Heading).Scale(math.Sin(turn)))
		ph.Pitch = math.Max(-1.55, math.Min(1.55, ph.Pitch-float64(d.Y)*sens))
		if w := rl.GetMouseWheelMove(); w != 0 {
			ph.Fov = math.Max(8, math.Min(110, ph.Fov*math.Pow(0.92, float64(w))))
		}
		if rl.IsKeyDown(rl.KeyQ) {
			ph.Roll -= photoRoll * dt
		}
		if rl.IsKeyDown(rl.KeyE) {
			ph.Roll += photoRoll * dt
		}
		if rl.IsKeyPressed(rl.KeyR) {
			ph.Roll = 0
		}
		var move vec3
		right := ph.Right()
		for _, k := range []struct {
			key int32
			dir vec3
		}{{rl.KeyW, ph.Forward()}, {rl.KeyS, ph.Forward().Scale(-1)}, {rl.KeyD, right}, {rl.KeyA, right.Scale(-1)},
			{rl.KeySpace, up}, {rl.KeyLeftControl, up.Scale(-1)}} {
			if rl.IsKeyDown(k.key) {
				move = move.Add(k.dir)
			}
		}
		speed := photoSpeed
		switch {
		case rl.IsKeyDown(rl.KeyLeftShift):
			speed *= 4
		case rl.IsKeyDown(rl.KeyLeftAlt):
			speed /= 4
		}
		ph.Pos = ph.Pos.Add(move.Norm().Scale(speed * dt))
	}
	// On a leash around the player, and above the ground.
	if off := ph.Pos.Sub(g.Player.Eye()); off.Length() > photoTether {
		ph.Pos = g.Player.Eye().Add(off.Norm().Scale(photoTether))
	}
	if g.World.Relief.Altitude(ph.Pos) < 0.15 {
		ph.Pos = g.World.Relief.Surface(ph.Pos).Add(ph.up().Scale(0.15))
	}
	up := ph.up()
	h := ph.Heading.Sub(up.Scale(ph.Heading.Dot(up)))
	if h.Length() < 1e-6 {
		h, _ = mathx.TangentBasis(up)
	}
	ph.Heading = h.Norm()
	return ph.Frozen()
}

// PhotoKeys name the settings for the screenshot tool (NA_SHOT_PHOTO).
var PhotoKeys = [PhotoSettings]string{"exp", "focus", "blur", "filter", "contrast", "sat", "vignette", "grain", "frame", "grid", "time"}

// PhotoPath is where the next photo is saved: in the settings folder,
// photos/, named by the date.
func PhotoPath() (string, error) {
	dir, err := saveDir()
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, "photos")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := "Naturalia " + time.Now().Format("2006-01-02 15.04.05")
	path := filepath.Join(dir, name+".png")
	for i := 2; ; i++ { // two in the same second
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return path, nil
		}
		path = filepath.Join(dir, fmt.Sprintf("%s (%d).png", name, i))
	}
}

// Shutter plays the camera's click.
func (g *Game) Shutter() { g.sfx.Beep(0.035, 0.05, 0.14, 1800, 1200) }
