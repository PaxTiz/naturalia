package main

import (
	"fmt"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/game"
	"naturalia/internal/mathx"
	"naturalia/internal/planet"
	"naturalia/internal/render"
	"naturalia/internal/ui"
	"os"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// shotRun checks the rendering without playing. With NA_SHOT=file.png the
// game takes a screenshot after a few seconds and quits without saving;
// NA_SHOT_UP=x,y,z first stands the player at that direction from Mars'
// center, facing the sun; NA_SHOT_TURN (radians, to the right) and
// NA_SHOT_PITCH then adjust the view.
type shotRun struct {
	file     string
	frames   int
	placed   bool
	heading  vec3 // the view, held against stray mouse motion
	pitch    float64
	mode     game.Mode
	perf     float64 // summed frame times (NA_SHOT_PERF)
	hud      *ui.HUD
	traveled bool // NA_SHOT_TRAVEL: the trip started
	tab      int  // the settings' tab shown, or -1 (NA_SHOT_MODE=settings)
}

func newShotRun(g *game.Game, gfx *render.Graphics, hud *ui.HUD) *shotRun {
	s := &shotRun{file: os.Getenv("NA_SHOT"), mode: game.ModePlay, hud: hud, tab: -1}
	g.NoSave = s.file != ""
	// The clock stands still for a shot, at NA_SHOT_HOUR (the base's hour)
	// or at the hour of the art direction's reference sun.
	if s.file != "" {
		hour := planet.RefHour
		fmt.Sscanf(os.Getenv("NA_SHOT_HOUR"), "%f", &hour)
		g.Content.Terrain.Sol = 0
		g.State.Sol = 0
		g.SetLocalHour(planet.BaseUp, hour)
	}
	// NA_SHOT_MODE opens a screen for the shot: shop, garage, map, pause,
	// or settings (NA_SHOT_TAB=n picks the tab).
	switch os.Getenv("NA_SHOT_MODE") {
	case "settings":
		s.mode, s.tab = game.ModePaused, 0
		fmt.Sscanf(os.Getenv("NA_SHOT_TAB"), "%d", &s.tab)
	case "shop":
		s.mode = game.ModeShop
	case "garage":
		s.mode, g.Garage = game.ModeShop, true
	case "map":
		s.mode = game.ModeMap
	case "pause":
		s.mode = game.ModePaused
	case "photo":
		s.mode = game.ModePhoto
	case "scope":
		s.mode = game.ModeScope
	}
	// NA_SHOT_TOOL=n holds tool n (0 is the toothbrush) for the shot.
	var tool int
	if _, err := fmt.Sscanf(os.Getenv("NA_SHOT_TOOL"), "%d", &tool); err == nil && s.file != "" {
		g.State.Tool = g.Content.Tools.IDs()[max(0, min(g.Content.Tools.Len()-1, tool))]
	}
	if v := os.Getenv("NA_SHOT_UP"); s.file != "" && v != "" {
		var d vec3
		fmt.Sscanf(v, "%f,%f,%f", &d.X, &d.Y, &d.Z)
		up := d.Norm()
		g.Player.Pos = gfx.Terrain.Surface(up)
		h := planet.SunDir.Sub(up.Scale(planet.SunDir.Dot(up)))
		if h.Length() < 0.1 { // the sun is overhead or right underfoot
			h, _ = mathx.TangentBasis(up)
		}
		g.Player.Heading = h.Norm()
		g.Player.Pitch = max(-0.25, math.Asin(up.Dot(planet.SunDir))-0.12)
		var turn float64
		if _, err := fmt.Sscanf(os.Getenv("NA_SHOT_TURN"), "%f", &turn); err == nil {
			r := g.Player.Right()
			g.Player.Heading = g.Player.Heading.Scale(math.Cos(turn)).Add(r.Scale(math.Sin(turn)))
		}
		fmt.Sscanf(os.Getenv("NA_SHOT_PITCH"), "%f", &g.Player.Pitch)
		// NA_SHOT_ALT=meters flies that high, in creative flight.
		var alt float64
		if _, err := fmt.Sscanf(os.Getenv("NA_SHOT_ALT"), "%f", &alt); err == nil {
			g.Player.SetFly(true, g.World.Relief)
			g.Player.Pos = g.Player.Pos.Add(up.Scale(alt))
		}
		s.placed, s.heading, s.pitch = true, g.Player.Heading, g.Player.Pitch
		gfx.Terrain.Warm(g.Player.Eye(), 20*time.Second)
	}
	// NA_SHOT_FLEET=1 lines up every vehicle ahead; NA_SHOT_DRIVE=id
	// sits in one, NA_SHOT_LIFT=alt,speed lifts it, NA_SHOT_CHASE=1 looks
	// at it from behind. NA_SHOT_RAMP=id sets that entity 15 m ahead.
	if s.file != "" && os.Getenv("NA_SHOT_FLEET") != "" {
		g.LineUp()
	}
	if id := os.Getenv("NA_SHOT_RAMP"); s.file != "" && id != "" {
		g.SpawnAhead(id, 15)
	}

	if id := os.Getenv("NA_SHOT_DRIVE"); s.file != "" && id != "" {
		g.Board(id)
		var alt, speed float64
		fmt.Sscanf(os.Getenv("NA_SHOT_LIFT"), "%f,%f", &alt, &speed)
		g.Lift(alt, speed, os.Getenv("NA_SHOT_CHASE") != "")
		s.heading = g.Player.Heading
	}
	// NA_SHOT_AT=x,z,yaw stands in the base (local coordinates, yaw in
	// radians from +X toward +Z), for views of the buildings.
	if v := os.Getenv("NA_SHOT_AT"); s.file != "" && v != "" {
		var x, z, yaw float64
		fmt.Sscanf(v, "%f,%f,%f", &x, &z, &yaw)
		// NA_SHOT_Y=meters stands that high, on a mezzanine;
		// NA_SHOT_ALT=meters flies that high over the base.
		var y float64
		fmt.Sscanf(os.Getenv("NA_SHOT_Y"), "%f", &y)
		if _, err := fmt.Sscanf(os.Getenv("NA_SHOT_ALT"), "%f", &y); err == nil {
			g.Player.SetFly(true, g.World.Relief)
		}
		g.Player.Pos = base.ToWorld(vec3{x, base.FloorTop + y, z})
		g.Player.Heading = vec3{math.Cos(yaw), 0, math.Sin(yaw)}
		g.Player.Pitch = 0.05
		fmt.Sscanf(os.Getenv("NA_SHOT_PITCH"), "%f", &g.Player.Pitch)
		s.placed, s.heading, s.pitch = true, g.Player.Heading, g.Player.Pitch
		gfx.Terrain.Warm(g.Player.Eye(), 20*time.Second)
	}
	// NA_SHOT_CAMP=dist,bearing sets a bootcamp that far from the base
	// (bearing in degrees from +X toward +Z) and looks at it from 14 m
	// before its door; NA_SHOT_CAMP_AT=x,z,yaw stands in its own frame
	// instead (yaw in radians from +X toward +Z), NA_SHOT_Y that high.
	if v := os.Getenv("NA_SHOT_CAMP"); s.file != "" && v != "" {
		var dist, bearing float64
		fmt.Sscanf(v, "%f,%f", &dist, &bearing)
		if c, ok := g.ShotCamp("core:bootcamp", dist, bearing); ok {
			x, z, yaw := 14.0, 6.0, math.Pi*1.1
			fmt.Sscanf(os.Getenv("NA_SHOT_CAMP_AT"), "%f,%f,%f", &x, &z, &yaw)
			var y float64
			fmt.Sscanf(os.Getenv("NA_SHOT_Y"), "%f", &y)
			g.Player.Pos = c.Site.ToWorld(vec3{x, base.FloorTop + y, z})
			g.Player.Heading = c.Site.Dir(vec3{math.Cos(yaw), 0, math.Sin(yaw)})
			g.Player.Pitch = 0.05
			fmt.Sscanf(os.Getenv("NA_SHOT_PITCH"), "%f", &g.Player.Pitch)
			s.placed, s.heading, s.pitch = true, g.Player.Heading, g.Player.Pitch
			gfx.Terrain.Warm(g.Player.Eye(), 20*time.Second)
		}
	}
	// NA_SHOT_NIGHTS=n counts n nights slept: what grows has grown.
	if s.file != "" {
		fmt.Sscanf(os.Getenv("NA_SHOT_NIGHTS"), "%d", &g.State.Nights)
	}
	// Through the telescope, NA_SHOT_SCOPE=fov,yaw,pitch (degrees) aims it.
	if s.mode == game.ModeScope && s.file != "" {
		for _, it := range base.Interactables() {
			if it.Kit == "core:telescope" {
				g.LookThrough(it.At)
			}
		}
		var fov, yaw, pitch float64
		if _, err := fmt.Sscanf(os.Getenv("NA_SHOT_SCOPE"), "%f,%f,%f", &fov, &yaw, &pitch); err == nil {
			g.Scope.Fov, g.Scope.Yaw, g.Scope.Pitch = fov, yaw*math.Pi/180, pitch*math.Pi/180
		}
	}
	// In the photo mode, NA_SHOT_PHOTO=key=value,... sets its settings
	// (game.PhotoKeys), and NA_SHOT_PHOTO_HIDE=1 hides its panel.
	if s.mode == game.ModePhoto && s.file != "" {
		g.EnterPhoto()
		for _, kv := range strings.Split(os.Getenv("NA_SHOT_PHOTO"), ",") {
			k, v, _ := strings.Cut(kv, "=")
			for i, name := range game.PhotoKeys {
				if name == k {
					fmt.Sscanf(v, "%f", &g.Photo.Set[i])
				}
			}
		}
		if os.Getenv("NA_SHOT_PHOTO_HIDE") != "" {
			hud.HidePhoto()
		}
	}
	return s
}

// tick runs after Update: it keeps the game playing even though the window
// is not focused, and takes the shot once the exposure has settled.
func (s *shotRun) tick(g *game.Game) {
	if s.file == "" {
		return
	}
	g.Mode = s.mode
	// NA_SHOT_MAP=tab,hover opens the map table's tab (1: travel) with
	// destination hover under the mouse; NA_SHOT_TRAVEL=t shows the trip
	// to the first bootcamp t seconds in.
	if v := os.Getenv("NA_SHOT_MAP"); v != "" && s.mode == game.ModeMap {
		fmt.Sscanf(v, "%d,%d", &g.MapTab, &g.MapHover)
	}
	if v := os.Getenv("NA_SHOT_TRAVEL"); v != "" && s.frames > 30 {
		if d := g.Destinations(); g.Mode != game.ModeTravel && len(d) > 1 && !s.traveled {
			g.State.Credits += game.TravelCost
			g.Travel(d[1])
			s.traveled = true
		}
		if s.traveled {
			fmt.Sscanf(v, "%f", &g.TravelT)
			g.Mode, s.mode = game.ModeTravel, game.ModeTravel
		}
	}
	if s.tab >= 0 {
		s.hud.OpenSettings(s.tab)
	}
	if s.placed {
		g.Player.Heading, g.Player.Pitch = s.heading, s.pitch
	}
	s.frames++
	// NA_SHOT_GHOST=item,degrees holds that item's ghost, tipped over,
	// once playing.
	if v := os.Getenv("NA_SHOT_GHOST"); v != "" && s.frames == 60 {
		item, deg, _ := strings.Cut(v, ",")
		var d float64
		fmt.Sscanf(deg, "%f", &d)
		g.HoldGhost(item, d)
	}
	// NA_SHOT_PERF=1 prints the mean frame time over frames 120 to 239,
	// without vsync (main.go). NA_SHOT_SIZE=WxH sets the window.
	if os.Getenv("NA_SHOT_PERF") != "" && s.frames >= 120 && s.frames < 240 {
		s.perf += float64(rl.GetFrameTime())
		if s.frames == 239 {
			fmt.Printf("frame %.2f ms\n", s.perf/120*1000)
		}
	}
	// NA_SHOT_JITTER=radians takes a second shot after turning that much,
	// to find what flickers as the camera moves (compare the two).
	var jit float64
	fmt.Sscanf(os.Getenv("NA_SHOT_JITTER"), "%f", &jit)
	if jit != 0 && s.frames == 241 {
		if os.Getenv("NA_SHOT_STEP") != "" { // a step forward instead of a turn
			g.Player.Pos = g.Player.Pos.Add(g.Player.Heading.Scale(jit))
		} else {
			s.heading = s.heading.Scale(math.Cos(jit)).Add(g.Player.Right().Scale(math.Sin(jit)))
		}
	}
	// NA_SHOT_WALK=meters walks that much forward each frame and shoots
	// every frame, w00_file to w23_file: what jumps shows in their diffs.
	var walk float64
	fmt.Sscanf(os.Getenv("NA_SHOT_WALK"), "%f", &walk)
	// NA_SHOT_SPIN=radians turns that much each frame while walking.
	var spin float64
	fmt.Sscanf(os.Getenv("NA_SHOT_SPIN"), "%f", &spin)
	if walk != 0 && s.frames > 240 {
		g.Player.Pos = g.Player.Pos.Add(g.Player.Heading.Scale(walk))
		s.heading = s.heading.Scale(math.Cos(spin)).Add(g.Player.Right().Scale(math.Sin(spin)))
		rl.TakeScreenshot(fmt.Sprintf("w%02d_%s", s.frames-241, s.file))
		if s.frames == 264 {
			os.Exit(0)
		}
		return
	}
	if jit != 0 && s.frames == 244 {
		rl.TakeScreenshot("b_" + s.file)
		os.Exit(0)
	}
	if s.frames == 240 {
		rl.TakeScreenshot(s.file)
		if jit == 0 && walk == 0 {
			os.Exit(0) // no save: the player was moved
		}
	}
}
