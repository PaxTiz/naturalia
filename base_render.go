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
	if gfx.base == nil {
		gfx.base = buildBase(gfx.objectSh)
		gfx.doorPanel = doorPanelModel(gfx.objectSh)
	}

	// Red dust settles on the faces turned up and at the foot of the walls.
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.wear, []float32{1, float32(basePos.Y)}, rl.ShaderUniformVec2)
	gfx.base.draw(gfx)
	for _, bp := range plans {
		for _, p := range bp.Props {
			if k := kitProps[p.Kit]; k.draw != nil {
				k.draw(g, bp.At.add(p.At))
			}
		}
	}

	g.drawAirlock()
	g.drawBeacon()
	g.drawSigns()
	gfx.setMat(mtPlain, 1, false)
	gfx.setLighting(litSun, vec3{})
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.wear, []float32{0, 0}, rl.ShaderUniformVec2)
}

// drawAirlock draws the moving parts of the outer door: two leaves sliding
// apart when the player comes close, and the status light.
func (g *Game) drawAirlock() {
	gfx := g.gfx
	end := airlockEnd
	gfx.setLighting(litSun, vec3{})
	steel := tokens["steel"]
	gfx.setMat(steel.mat, steel.scale, false)
	w := (doorWidth + 0.2) / 2
	for _, s := range []float64{-1, 1} {
		at := end.add(vec3{s * (w/2 + g.doorOpen*w), doorHeight / 2, 0})
		gfx.put(gfx.doorPanel, toWorld(at), vec3{1, 1, 1}, vec3{0, 1, 0}, 0, steel.col, steel.spec, 0)
	}
	gfx.setMat(mtPlain, 1, false)
	lamp := colGreen
	if g.doorOpen < 0.9 {
		lamp = colGold
	}
	gfx.box(end.add(vec3{0, 2.6, -0.2}), vec3{1.2, 0.08, 0.04}, vec3{1, 0, 0}, lamp, 0, 1)
}

// drawBeacon blinks the red light on top of the hub's antenna.
func (g *Game) drawBeacon() {
	if math.Mod(g.clock, 1.2) < 0.3 {
		g.gfx.setLighting(litSun, vec3{})
		g.gfx.setMat(mtPlain, 1, false)
		g.gfx.put(g.gfx.sphere, beaconTop(), vec3{0.09, 0.09, 0.09}, vec3{0, 1, 0}, 0, colRed, 0, 1)
	}
}

// beaconTop is where the blinking light sits, for its glow.
func beaconTop() vec3 {
	return toWorld(vec3{1.5, hubRoom.wallH + hubRoom.radius*0.55 + 2.25, 1.5})
}

// drawGlobe draws the hologram of Mars above the map table.
func (g *Game) drawGlobe(cam rl.Camera3D) {
	gfx := g.gfx
	center := toWorld(tableSpot.add(vec3{0, globeHeight, 0}))
	rl.SetShaderValue(gfx.holoSh, gfx.hLoc.globeCenter, v3f(center), rl.ShaderUniformVec3)
	// Turn the player's spot on Mars toward the viewer, a bit raised,
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

// Painted materials of the object shader (the mat* names are colors).
const (
	mtPlain  = 0
	mtPaint  = 1
	mtPanel  = 2
	mtMetal  = 3
	mtFloor  = 4
	mtFabric = 5
	mtRock   = 6
	mtHazard = 7
	mtDecal  = 8 // the model's own texture, by UVs (signs)
)

// setMat selects the painted material of the next objects: scale is in
// texture tiles per meter; objSpace makes the texture follow a moving object
// (its unit mesh coordinates are used instead of world ones).
func (gfx *Graphics) setMat(kind int, scale float32, objSpace bool) {
	o := float32(0)
	if objSpace {
		o = 1
	}
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.mat, []float32{float32(kind), scale, o}, rl.ShaderUniformVec3)
}
