package main

import (
	"image/color"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Lighting modes of the object shader.
const (
	litSun     = 0 // outdoors
	litIndoor  = 1 // ceiling lamps
	litBack    = 2 // shells: sun outside, lamps on the back faces seen from inside
	litTowards = 3 // walls: lamps on the faces turned toward the room's center
)

var (
	matPanel     = color.RGBA{232, 234, 240, 255}
	matPanel2    = color.RGBA{218, 222, 232, 255}
	matFloor     = color.RGBA{82, 84, 94, 255}
	matFloor2    = color.RGBA{104, 106, 118, 255}
	accentCyan   = color.RGBA{80, 210, 235, 255}
	accentOrange = color.RGBA{255, 140, 50, 255}
	accentBlue   = color.RGBA{90, 140, 255, 255}
	accentPurple = color.RGBA{170, 120, 255, 255}
	lampWarm     = color.RGBA{255, 236, 200, 255}
)

// setLighting selects how the next objects are lit; room is the world point
// that litTowards walls face.
func (gfx *Graphics) setLighting(mode float32, room vec3) {
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.lighting, []float32{mode}, rl.ShaderUniformFloat)
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.room, v3f(room), rl.ShaderUniformVec3)
}

// yawFor returns the rotation around Y (radians) that turns a model's +X
// axis toward the horizontal direction dir.
func yawFor(dir vec3) float64 { return math.Atan2(-dir.Z, dir.X) }

// box draws a cube centered at the local point c, turned so its width (X)
// runs along the horizontal direction across.
func (gfx *Graphics) box(c, size, across vec3, col color.RGBA, spec, emissive float32) {
	gfx.put(gfx.cube, toWorld(c), size, vec3{0, 1, 0}, yawFor(across)*180/math.Pi, col, spec, emissive)
}

func (g *Game) drawBase() {
	gfx := g.gfx
	rl.DisableBackfaceCulling()
	defer rl.EnableBackfaceCulling()

	accents := []color.RGBA{accentCyan, accentOrange, accentBlue, accentPurple}
	for ri, r := range rooms {
		inside := toWorld(r.center.add(vec3{0, 1.2, 0}))
		gfx.setLighting(litTowards, inside)
		for i, p := range r.panels() {
			col := matPanel
			if i%2 == 1 {
				col = matPanel2
			}
			tangent := vec3{-math.Sin(p.angle), 0, math.Cos(p.angle)}
			gfx.box(p.mid.add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, wallThick}, tangent, col, 0.4, 0)
		}
		// Dome roof.
		gfx.setLighting(litBack, inside)
		gfx.put(gfx.hemi, toWorld(r.center.add(vec3{0, r.wallH, 0})), vec3{r.radius + 0.1, r.radius * 0.55, r.radius + 0.1}, vec3{0, 1, 0}, 0, matPanel, 0.5, 0)

		// Inside: floor, a glowing accent stripe along the walls, ceiling lamp.
		gfx.setLighting(litIndoor, inside)
		gfx.put(gfx.cyl, toWorld(r.center.add(vec3{0, floorTop - 0.1, 0})), vec3{r.radius, 0.1, r.radius}, vec3{0, 1, 0}, 0, matFloor, 0.2, 0)
		gfx.put(gfx.cyl, toWorld(r.center.add(vec3{0, floorTop - 0.095, 0})), vec3{r.radius * 0.45, 0.1, r.radius * 0.45}, vec3{0, 1, 0}, 0, matFloor2, 0.3, 0)
		for _, p := range r.panels() {
			if p.y0 > 0 {
				continue
			}
			out := vec3{math.Cos(p.angle), 0, math.Sin(p.angle)}
			tangent := vec3{-out.Z, 0, out.X}
			strip := p.mid.sub(out.scale(wallThick/2 + 0.012)).add(vec3{0, 1.0, 0})
			gfx.box(strip, vec3{p.width * 0.96, 0.06, 0.01}, tangent, accents[ri], 0, 0.7)
		}
		gfx.put(gfx.cyl, toWorld(r.center.add(vec3{0, r.wallH + r.radius*0.5, 0})), vec3{0.45, 0.05, 0.45}, vec3{0, 1, 0}, 0, lampWarm, 0, 1)
	}

	for _, c := range corridors {
		mid := c.from.lerp(c.to, 0.5)
		axis := c.to.sub(c.from)
		length := axis.length()
		side := c.side()
		gfx.setLighting(litTowards, toWorld(mid.add(vec3{0, 1.2, 0})))
		for _, s := range []float64{-1, 1} {
			wall := mid.add(side.scale(s * (c.width/2 + wallThick/2))).add(vec3{0, c.height / 2, 0})
			gfx.box(wall, vec3{length, c.height, wallThick}, axis, matPanel, 0.4, 0)
		}
		gfx.box(mid.add(vec3{0, c.height + 0.06, 0}), vec3{length, 0.12, c.width + 2*wallThick}, axis, matPanel2, 0.4, 0)
		gfx.setLighting(litIndoor, toWorld(mid))
		gfx.box(mid.add(vec3{0, floorTop - 0.05, 0}), vec3{length, 0.1, c.width}, axis, matFloor, 0.2, 0)
		gfx.box(mid.add(vec3{0, c.height - 0.02, 0}), vec3{length * 0.7, 0.03, 0.25}, axis, lampWarm, 0, 1)
	}

	for _, p := range junctionPlates() {
		room := toWorld(p.mid.sub(p.normal.scale(2)).add(vec3{0, 1.2, 0}))
		gfx.setLighting(litTowards, room)
		gfx.box(p.mid.add(vec3{0, (p.y0 + p.y1) / 2, 0}), vec3{p.width, p.y1 - p.y0, wallThick * 1.5}, vec3{-p.normal.Z, 0, p.normal.X}, matPanel2, 0.4, 0)
	}

	g.drawAirlock()
	g.drawFurniture()
	g.drawOutside()
	gfx.setLighting(litSun, vec3{})
}

// drawAirlock draws the outer door: a frame and two panels sliding apart
// when the player comes close.
func (g *Game) drawAirlock() {
	gfx := g.gfx
	end := airlockEnd
	gfx.setLighting(litSun, vec3{})
	across := vec3{1, 0, 0}
	half := (doorWidth+0.2)/2 + wallThick
	for _, s := range []float64{-1, 1} {
		gfx.box(end.add(vec3{s * half, 1.35, 0}), vec3{0.3, 2.7, 0.35}, across, matDark, 0.5, 0)
		w := (doorWidth + 0.2) / 2
		gfx.box(end.add(vec3{s * (w/2 + g.doorOpen*w), doorHeight / 2, 0}), vec3{w, doorHeight, 0.08}, across, matSteel, 0.6, 0)
	}
	gfx.box(end.add(vec3{0, 2.6, 0}), vec3{2*half + 0.3, 0.3, 0.35}, across, matDark, 0.5, 0)
	lamp := colGreen
	if g.doorOpen < 0.9 {
		lamp = colGold
	}
	gfx.box(end.add(vec3{0, 2.6, -0.2}), vec3{1.2, 0.08, 0.04}, across, lamp, 0, 1)
}

func (g *Game) drawFurniture() {
	gfx := g.gfx
	up := vec3{0, 1, 0}
	gfx.setLighting(litIndoor, vec3{})

	// Hub: the map table.
	t := tableSpot
	gfx.put(gfx.cyl, toWorld(t), vec3{0.85, 0.85, 0.85}, up, 0, matDark, 0.5, 0)
	gfx.put(gfx.cyl, toWorld(t.add(vec3{0, 0.85, 0})), vec3{1.2, 0.08, 1.2}, up, 0, matSteel, 0.8, 0)
	gfx.put(gfx.cyl, toWorld(t.add(vec3{0, 0.93, 0})), vec3{1.0, 0.02, 1.0}, up, 0, accentCyan, 0, 0.9)
	for _, a := range []float64{0.8, 2.4, 3.9, 5.5} {
		crate := vec3{math.Cos(a) * 5.4, 0.35, math.Sin(a) * 5.4}
		gfx.box(crate, vec3{0.7, 0.7, 0.7}, vec3{math.Cos(a + 0.4), 0, math.Sin(a + 0.4)}, color.RGBA{120, 112, 96, 255}, 0.2, 0)
	}

	// Sale: the incinerator, glowing harder while it burns dust.
	in := incineratorSpot
	glow := float32(0.55 + 0.45*clamp01(g.burn))
	gfx.put(gfx.cyl, toWorld(in), vec3{1.1, 1.9, 1.1}, up, 0, matSteel, 0.6, 0)
	gfx.put(gfx.cyl, toWorld(in.add(vec3{0, 0.25, 0})), vec3{1.14, 0.12, 1.14}, up, 0, matDark, 0.3, 0)
	gfx.put(gfx.cyl, toWorld(in.add(vec3{0, 1.6, 0})), vec3{1.14, 0.12, 1.14}, up, 0, matDark, 0.3, 0)
	gfx.segment(gfx.cone, toWorld(in.add(vec3{0, 2.75, 0})), toWorld(in.add(vec3{0, 1.9, 0})), 0.95, matSteel, 0.6, 0)
	gfx.box(in.add(vec3{1.08, 0.95, 0}), vec3{0.8, 0.55, 0.06}, vec3{0, 0, 1}, accentOrange, 0, glow)
	gfx.segment(gfx.cyl, toWorld(in.add(vec3{-0.6, 1.5, 0.6})), toWorld(in.add(vec3{-0.6, 4.9, 0.6})), 0.18, matDark, 0.4, 0)
	gfx.box(in.add(vec3{1.0, 1.0, 1.25}), vec3{0.5, 0.7, 0.3}, vec3{0, 0, 1}, matDark, 0.4, 0)
	gfx.box(in.add(vec3{1.16, 1.1, 1.25}), vec3{0.36, 0.3, 0.02}, vec3{0, 0, 1}, colGreen, 0, 0.8)

	// Research: a desk with three screens, between two server racks.
	c := consoleSpot
	gfx.box(c.add(vec3{-0.2, 0.78, 0}), vec3{0.9, 0.06, 2.2}, vec3{1, 0, 0}, matSteel, 0.6, 0)
	for _, z := range []float64{-0.95, 0.95} {
		gfx.box(c.add(vec3{-0.2, 0.38, z}), vec3{0.8, 0.76, 0.08}, vec3{1, 0, 0}, matDark, 0.3, 0)
	}
	for i, z := range []float64{-0.75, 0, 0.75} {
		yaw := vec3{0, 0, 1}
		if i != 1 {
			yaw = vec3{-z * 0.4, 0, 1}.norm()
		}
		gfx.box(c.add(vec3{0.1, 1.28, z}), vec3{0.66, 0.46, 0.05}, yaw, accentBlue, 0, 0.75)
	}
	for _, z := range []float64{-2.3, 2.3} {
		gfx.box(c.add(vec3{0, 0.95, z}), vec3{0.7, 1.9, 0.7}, vec3{1, 0, 0}, matDark, 0.4, 0)
		for k := range 5 {
			on := float32(0.3)
			if math.Sin(g.clock*3+float64(k)*1.7+z) > 0 {
				on = 1
			}
			gfx.box(c.add(vec3{-0.36, 0.4 + float64(k)*0.3, z}), vec3{0.5, 0.04, 0.02}, vec3{0, 0, 1}, colGreen, 0, on)
		}
	}

	// Sleep: a bed against the far wall and a bedside lamp.
	b := bedSpot
	gfx.box(b.add(vec3{0, 0.2, 0}), vec3{2.1, 0.4, 1.05}, vec3{1, 0, 0}, matDark, 0.3, 0)
	gfx.box(b.add(vec3{0, 0.48, 0}), vec3{2.0, 0.16, 0.98}, vec3{1, 0, 0}, matWhite, 0.2, 0)
	gfx.box(b.add(vec3{0.32, 0.58, 0}), vec3{1.36, 0.06, 1.0}, vec3{1, 0, 0}, accentPurple, 0.2, 0)
	gfx.box(b.add(vec3{-0.72, 0.62, 0}), vec3{0.42, 0.12, 0.7}, vec3{1, 0, 0}, matWhite, 0.2, 0)
	gfx.box(b.add(vec3{1.45, 0.3, 0.2}), vec3{0.5, 0.6, 0.5}, vec3{1, 0, 0}, matDark, 0.3, 0)
	gfx.put(gfx.sphere, toWorld(b.add(vec3{1.45, 0.75, 0.2})), vec3{0.13, 0.13, 0.13}, up, 0, lampWarm, 0, 1)
}

// drawOutside: an antenna and a beacon on the hub, solar panels nearby.
func (g *Game) drawOutside() {
	gfx := g.gfx
	gfx.setLighting(litSun, vec3{})
	top := hubRoom.wallH + hubRoom.radius*0.55
	gfx.segment(gfx.cyl, toWorld(vec3{1.5, top - 0.8, 1.5}), toWorld(vec3{1.5, top + 2.2, 1.5}), 0.05, matSteel, 0.5, 0)
	gfx.put(gfx.hemi, toWorld(vec3{-1.8, top - 0.5, -1.2}), vec3{0.7, 0.25, 0.7}, vec3{0, 0, 1}, 25, matWhite, 0.6, 0)
	if math.Mod(g.clock, 1.2) < 0.3 {
		gfx.put(gfx.sphere, toWorld(vec3{1.5, top + 2.25, 1.5}), vec3{0.09, 0.09, 0.09}, vec3{0, 1, 0}, 0, colRed, 0, 1)
	}
	for _, z := range []float64{-7.5, -10.5} {
		at := vec3{-12, 0, z}
		gfx.segment(gfx.cyl, toWorld(at), toWorld(at.add(vec3{0, 1.2, 0})), 0.06, matSteel, 0.5, 0)
		gfx.put(gfx.cube, toWorld(at.add(vec3{0, 1.3, 0})), vec3{2.6, 0.05, 1.5}, vec3{1, 0, 0}, -30, color.RGBA{40, 60, 120, 255}, 1, 0)
	}
}

// beaconTop is where the blinking light sits, for its glow.
func beaconTop() vec3 {
	return toWorld(vec3{1.5, hubRoom.wallH + hubRoom.radius*0.55 + 2.25, 1.5})
}

// drawGlobe draws the hologram of the Moon above the map table.
func (g *Game) drawGlobe(cam rl.Camera3D) {
	gfx := g.gfx
	center := toWorld(tableSpot.add(vec3{0, globeHeight, 0}))
	rl.SetShaderValue(gfx.holoSh, gfx.hLoc.globeCenter, v3f(center), rl.ShaderUniformVec3)
	// Turn the player's spot on the Moon toward the viewer, a bit raised,
	// with a slow sway so the hologram feels alive.
	toViewer := g.player.Eye().sub(center)
	toViewer = vec3{toViewer.X, 0, toViewer.Z}.norm()
	sway := math.Sin(g.clock*0.35) * 0.45
	toViewer = vec3{
		toViewer.X*math.Cos(sway) + toViewer.Z*math.Sin(sway), 0,
		-toViewer.X*math.Sin(sway) + toViewer.Z*math.Cos(sway),
	}
	facing := toViewer.add(vec3{0, 0.2, 0}).norm()
	rl.SetShaderValueMatrix(gfx.holoSh, gfx.hLoc.globeRot, rotationBetween(facing, g.player.LocalUp()))
	rl.SetShaderValue(gfx.holoSh, gfx.hLoc.playerDir, v3f(g.player.LocalUp()), rl.ShaderUniformVec3)
	rl.SetMaterialTexture(&gfx.holo.GetMaterials()[0], rl.MapDiffuse, g.world.Coverage())

	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()
	rl.BeginBlendMode(rl.BlendAdditive)
	rl.DrawModel(gfx.holo, center.rl(), globeRadius, rl.White)
	// Soft light rising from the table to the globe.
	for i := range 6 {
		f := float64(i) / 5
		p := toWorld(tableSpot.add(vec3{0, 1.0 + f*(globeHeight-1.0), 0}))
		rl.DrawBillboard(cam, gfx.glow, p.rl(), float32(0.9+f*1.2), color.RGBA{60, 180, 220, uint8(40 - 25*f)})
	}
	rl.EndBlendMode()
	rl.EnableDepthMask()
}

const (
	globeHeight = 2.0
	globeRadius = 0.8
)

// rotationBetween returns the rotation taking unit vector a onto unit
// vector b (Rodrigues' formula), as a raylib matrix.
func rotationBetween(a, b vec3) rl.Matrix {
	k := a.cross(b)
	c := a.dot(b)
	if c < -0.9999 {
		// Opposite vectors: half a turn around any perpendicular axis.
		p, _ := tangentBasis(a)
		k, c = p.scale(1e-6), -1+1e-12
	}
	f := 1 / (1 + c)
	m := [3][3]float64{
		{c + k.X*k.X*f, k.X*k.Y*f - k.Z, k.X*k.Z*f + k.Y},
		{k.Y*k.X*f + k.Z, c + k.Y*k.Y*f, k.Y*k.Z*f - k.X},
		{k.Z*k.X*f - k.Y, k.Z*k.Y*f + k.X, c + k.Z*k.Z*f},
	}
	return rl.Matrix{
		M0: float32(m[0][0]), M4: float32(m[0][1]), M8: float32(m[0][2]),
		M1: float32(m[1][0]), M5: float32(m[1][1]), M9: float32(m[1][2]),
		M2: float32(m[2][0]), M6: float32(m[2][1]), M10: float32(m[2][2]),
		M15: 1,
	}
}
