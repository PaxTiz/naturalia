package main

import (
	"fmt"
	"log"
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/game"
	"mars_sweeper/internal/render"
	"mars_sweeper/internal/script"
	"mars_sweeper/internal/ui"
	_ "mars_sweeper/packs/core" // the base game, registered as a pack
	"os"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	flags := uint32(rl.FlagWindowResizable | rl.FlagMsaa4xHint | rl.FlagVsyncHint)
	if os.Getenv("MS_SHOT_PERF") != "" { // measuring: no wait for the screen (shot.go)
		flags &^= rl.FlagVsyncHint
	}
	rl.SetConfigFlags(flags)
	w, h := int32(1280), int32(720)
	fmt.Sscanf(os.Getenv("MS_SHOT_SIZE"), "%dx%d", &w, &h)
	rl.InitWindow(w, h, "Mars Sweeper")
	defer rl.CloseWindow()
	rl.SetWindowMinSize(800, 450)
	// Escape pauses the game instead of quitting it.
	rl.SetExitKey(0)

	content.LoadMods(game.ModDirs(), game.DisabledMods(), script.Run)
	c := game.LoadContent()
	log.Println("packs:", content.PackList())
	g := game.New(c)
	gfx := render.New(g)
	hud := ui.New()
	shot := newShotRun(g, gfx)
	for !rl.WindowShouldClose() {
		// Clamp so a window drag or a hitch doesn't teleport everything.
		dt := min(float64(rl.GetFrameTime()), 0.05)
		hud.Update(g, dt)
		g.Update(dt)
		shot.tick(g)
		gfx.Draw(g, func(cam rl.Camera3D) { hud.Draw(g, cam, gfx.Stats()) })
	}
	g.Save()
	rl.CloseAudioDevice()
}
