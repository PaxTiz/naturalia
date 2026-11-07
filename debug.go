package main

import (
	"fmt"
	"math"
	"runtime"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// debugInfo gathers the measures shown by the debug overlay (F3).
type debugInfo struct {
	on               bool
	frameMs, worstMs float64 // smoothed frame time, worst one over the last second
	worstAcc, timer  float64
	patches, pebbles int     // drawn this frame
	sun, ao          float64 // relief light under the player (Terrain.light)
	heapMB           float64
	goroutines       int
}

// updateDebug toggles the overlay and refreshes its slower measures.
func (g *Game) updateDebug(dt float64) {
	d := &g.dbg
	if rl.IsKeyPressed(rl.KeyF3) {
		d.on = !d.on
	}
	ms := dt * 1000
	d.frameMs += (ms - d.frameMs) * 0.1
	d.worstAcc = max(d.worstAcc, ms)
	d.timer += dt
	if d.timer >= 1 {
		d.worstMs, d.worstAcc, d.timer = d.worstAcc, 0, 0
		if d.on { // reading the memory stats briefly stops the world
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			d.heapMB = float64(m.HeapAlloc) / (1 << 20)
			d.goroutines = runtime.NumGoroutine()
		}
	}
}

func (g *Game) drawDebug() {
	d := &g.dbg
	if !d.on {
		return
	}
	gfx, u := g.gfx, uiScale()
	t := g.terrain
	up := g.player.Pos.norm()
	lines := []string{
		fmt.Sprintf("FPS %d   image %.1f ms   pire %.1f ms", rl.GetFPS(), d.frameMs, d.worstMs),
		fmt.Sprintf("terrain  %d dessinés  %d chargés  %d en attente", d.patches, len(t.loaded), t.pending),
		fmt.Sprintf("cailloux %d   rochers %d", d.pebbles, len(g.nearRocks)),
		fmt.Sprintf("lat %.2f°  lon %.2f°", math.Asin(up.Y)*180/math.Pi, math.Atan2(up.Z, up.X)*180/math.Pi),
		fmt.Sprintf("alt sol %.1f m   relief %.1f m", t.Altitude(g.player.Pos), t.Height(up)),
		fmt.Sprintf("soleil %.1f°   ombre %.2f   occlusion %.2f", math.Asin(up.dot(sunDir))*180/math.Pi, d.sun, d.ao),
		fmt.Sprintf("poussière %.2f   tuiles %d", g.world.DustAt(up), len(g.world.tiles)),
		fmt.Sprintf("tas %.0f Mo   goroutines %d", d.heapMB, d.goroutines),
	}
	size, step := 15*u, 20*u
	x, y := 40*u, 180*u
	rl.DrawRectangleRec(rl.NewRectangle(x, y, 420*u, float32(len(lines))*step+20*u), withAlpha(rl.Black, 0.6))
	for i, s := range lines {
		rl.DrawTextEx(gfx.mono, s, rl.NewVector2(x+12*u, y+10*u+float32(i)*step), size, 0, visorCyan)
	}
}
