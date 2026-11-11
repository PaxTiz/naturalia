package ui

import (
	"fmt"
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/game"
	"mars_sweeper/internal/planet"
	"mars_sweeper/internal/render"
	"math"
	"runtime"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// debugInfo gathers the measures shown by the debug overlay (F3), with
// the rendering's (Stats).
type debugInfo struct {
	on               bool
	frameMs, worstMs float64 // smoothed frame time, worst one over the last second
	worstAcc, timer  float64
	heapMB           float64
	goroutines       int
}

// updateDebug toggles the overlay and refreshes its slower measures.
func (hud *HUD) updateDebug(dt float64) {
	d := &hud.dbg
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

func (hud *HUD) drawDebug(g *game.Game, rs render.Stats) {
	d := &hud.dbg
	if !d.on {
		return
	}
	u := uiScale()
	t := g.World.Relief
	up := g.Player.Pos.Norm()
	lines := []string{
		fmt.Sprintf("FPS %d   image %.1f ms   pire %.1f ms", rl.GetFPS(), d.frameMs, d.worstMs),
		fmt.Sprintf("terrain  %d dessinés  %d chargés  %d en attente", rs.Patches, rs.Loaded, rs.Pending),
		fmt.Sprintf("cailloux %d   rochers %d", rs.Pebbles, len(g.NearRocks)),
		fmt.Sprintf("lat %.2f°  lon %.2f°", math.Asin(up.Y)*180/math.Pi, math.Atan2(up.Z, up.X)*180/math.Pi),
		fmt.Sprintf("alt sol %.1f m   relief %.1f m", t.Altitude(g.Player.Pos), t.Height(up)),
		fmt.Sprintf("soleil %.1f°   ombre %.2f   occlusion %.2f", math.Asin(up.Dot(planet.SunDir))*180/math.Pi, rs.Sun, rs.AO),
		fmt.Sprintf("poussière %.2f   tuiles %d", g.World.DustAt(up), len(g.World.Touched())),
		fmt.Sprintf("tas %.0f Mo   goroutines %d", d.heapMB, d.goroutines),
		fmt.Sprintf("packs %s   entités %d", content.PackList(), len(g.State.Entities)),
	}
	for _, line := range g.Content.Debug { // added by packs
		lines = append(lines, line(g))
	}
	size, step := 15*u, 20*u
	x, y := 40*u, 180*u
	rl.DrawRectangleRec(rl.NewRectangle(x, y, 420*u, float32(len(lines))*step+20*u), withAlpha(rl.Black, 0.6))
	for i, s := range lines {
		rl.DrawTextEx(hud.mono, s, rl.NewVector2(x+12*u, y+10*u+float32(i)*step), size, 0, visorCyan)
	}
}
