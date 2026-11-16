package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The telescope on the lookout: E puts the eye to it. The mouse turns it
// within a cone around where it points, the wheel zooms, the photo key
// takes the photo mode from that framing, E or Escape steps back.

const (
	scopeYaw     = 60 * math.Pi / 180 // how far it turns either side
	scopePitchLo = -15 * math.Pi / 180
	scopePitchHi = 30 * math.Pi / 180
	scopeFovMin  = 4.0
	scopeFovMax  = 40.0
)

// Scope is the view through the telescope.
type Scope struct {
	Eye        vec3 // world
	Facing     vec3 // local, level: where the telescope points
	Yaw, Pitch float64
	Fov        float64 // vertical, degrees
}

// Forward is the way the view looks (local axes are the world's at the
// pole).
func (s *Scope) Forward() vec3 {
	side := content.Turn(s.Facing, 90)
	h := s.Facing.Scale(math.Cos(s.Yaw)).Add(side.Scale(math.Sin(s.Yaw)))
	return h.Scale(math.Cos(s.Pitch)).Add(vec3{0, math.Sin(s.Pitch), 0})
}

// LookThrough puts the eye to the telescope standing at local point at.
func (g *Game) LookThrough(at vec3) {
	for _, f := range base.AllFurniture() {
		if f.At != at {
			continue
		}
		eye, facing := base.TelescopeEye(f.At, f.Yaw)
		// Aimed at the horizon, 3° down from 4 m up on Mars at 1/1000.
		g.Scope = Scope{Eye: base.ToWorld(eye), Facing: facing, Pitch: -3 * math.Pi / 180, Fov: 24}
		g.Sweeping = false
		g.setMode(ModeScope)
		return
	}
}

// updateScope turns and zooms the telescope.
func (g *Game) updateScope() {
	s := &g.Scope
	switch {
	case KeyPressed(ActUse) || rl.IsKeyPressed(rl.KeyEscape):
		g.setMode(ModePlay)
		return
	case KeyPressed(ActPhoto):
		f := s.Forward()
		g.EnterPhoto()
		ph := &g.Photo
		ph.Pos, ph.Heading, ph.Pitch, ph.Fov = s.Eye, vec3{f.X, 0, f.Z}.Norm(), s.Pitch, s.Fov
		return
	}
	if !rl.IsWindowFocused() {
		return
	}
	d := rl.GetMouseDelta()
	sens := s.Fov / 70 // slower zoomed in
	s.Yaw = math.Max(-scopeYaw, math.Min(scopeYaw, s.Yaw+float64(d.X)*lookX*sens))
	s.Pitch = math.Max(scopePitchLo, math.Min(scopePitchHi, s.Pitch-float64(d.Y)*lookY*sens))
	if w := rl.GetMouseWheelMove(); w != 0 {
		s.Fov = math.Max(scopeFovMin, math.Min(scopeFovMax, s.Fov*math.Pow(0.9, float64(w))))
	}
}
