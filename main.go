package main

import (
	rl "github.com/gen2brain/raylib-go/raylib"
)

func main() {
	rl.SetConfigFlags(rl.FlagWindowResizable | rl.FlagMsaa4xHint | rl.FlagVsyncHint)
	rl.InitWindow(1280, 720, "Mars Sweeper")
	defer rl.CloseWindow()
	rl.SetWindowMinSize(800, 450)
	// Escape pauses the game instead of quitting it.
	rl.SetExitKey(0)

	g := NewGame()
	for !rl.WindowShouldClose() {
		// Clamp so a window drag or a hitch doesn't teleport everything.
		dt := min(float64(rl.GetFrameTime()), 0.05)
		g.Update(dt)
		g.Draw()
	}
	g.save()
}
