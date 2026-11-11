package main

import (
	"fmt"
	"log"
	"mars_sweeper/internal/base"
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/script"
	"os"

	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	flags := uint32(rl.FlagWindowResizable | rl.FlagMsaa4xHint | rl.FlagVsyncHint)
	if os.Getenv("MS_SHOT_PERF") != "" { // measuring: no wait for the screen (debug.go)
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

	content.LoadMods(modDirs(), disabledMods(), script.Run)
	loadContent()
	log.Println("packs:", content.PackList())
	g := NewGame()
	shot := newShotRun(g)
	for !rl.WindowShouldClose() {
		// Clamp so a window drag or a hitch doesn't teleport everything.
		dt := min(float64(rl.GetFrameTime()), 0.05)
		g.Update(dt)
		shot.tick(g)
		g.Draw()
	}
	g.save()
	rl.CloseAudioDevice()
}

// loadContent loads the game's content, with extra packs for tests, then
// lays out the base.
func loadContent(extra ...content.Pack) {
	reg = content.Build(extra...)
	base.Setup(reg)
}

// reg is the game's content once every pack is loaded (loadContent).
var reg *content.Content
