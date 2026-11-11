package main

import (
	"fmt"
	"image/color"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/gofont/gosmallcaps"
)

var (
	colWhite = color.RGBA{235, 237, 245, 255}
	colGray  = color.RGBA{150, 155, 178, 255}
	colGold  = color.RGBA{250, 210, 90, 255}
	colGreen = color.RGBA{110, 220, 120, 255}
	colRed   = color.RGBA{235, 90, 90, 255}
	colBlue  = color.RGBA{90, 160, 240, 255}
	colCyan  = color.RGBA{120, 235, 245, 255}

	// Direction toward the sun: low on the horizon for long soft shadows.
	sunDir = vec3{-0.5, 0.62, -0.6}.norm()
)

const (
	maxCasters = 16
)

type shaderLocs struct {
	viewPos, sunDir, time       int32
	brush, casters, casterCount int32
	surface, up, instanced      int32
	ground                      int32
	steps, stepDirs, stepCount  int32
	lightVP, shadowPass         int32
	lighting, room              int32
	mat, wear                   int32
	globeCenter, globeRot       int32
	playerDir                   int32
}

// Graphics holds the GPU resources. Everything is generated at startup:
// shapes are meshes built in code and every surface is painted by shaders.
type Graphics struct {
	terrainSh, objectSh, skySh, holoSh rl.Shader
	tLoc, oLoc, sLoc, hLoc             shaderLocs
	devilSh                            rl.Shader
	dLoc                               shaderLocs
	devilLoc                           int32
	devilMesh                          rl.Mesh
	devilMat                           rl.Material

	sphere, hemi, cyl, cone, cube, crystal, sky, holo rl.Model
	terrainMat                                        *rl.Material // own allocation: see NewGraphics
	signBox                                           rl.Model
	rocks, pebbles                                    []rl.Model
	post                                              *postFX
	shadow                                            *shadowMap
	lampShadow                                        *lampShadow
	base                                              *building
	doorPanel                                         rl.Model
	screens                                           []rl.Texture2D
	signs                                             []sign
	smallcaps                                         rl.Font
	mono                                              rl.Font
	visor                                             rl.Texture2D
	tex                                               Textures
	objectMaps                                        []rl.Texture2D // the painted textures, in the object shader's order
	glow                                              rl.Texture2D
	font, bold                                        rl.Font
}

func loadShader(fs string) (rl.Shader, shaderLocs) { return loadShaderVS(meshVS, fs) }

func loadShaderVS(vs, fs string) (rl.Shader, shaderLocs) {
	sh := rl.LoadShaderFromMemory(vs, fs)
	loc := func(name string) int32 { return rl.GetShaderLocation(sh, name) }
	return sh, shaderLocs{
		viewPos: loc("viewPos"), sunDir: loc("sunDir"), time: loc("time"),
		brush: loc("brush"), casters: loc("casters"), casterCount: loc("casterCount"),
		surface: loc("surface"), up: loc("up"), instanced: loc("instanced"), ground: loc("ground"),
		steps: loc("steps"), stepDirs: loc("stepDirs"), stepCount: loc("stepCount"),
		lightVP: loc("lightVP"), shadowPass: loc("shadowPass"),
		lighting: loc("lighting"), room: loc("room"),
		mat: loc("matInfo"), wear: loc("wear"),
		globeCenter: loc("globeCenter"), globeRot: loc("globeRot"), playerDir: loc("playerDir"),
	}
}

func NewGraphics() *Graphics {
	gfx := &Graphics{}
	gfx.post = newPostFX()
	gfx.terrainSh, gfx.tLoc = loadShaderVS(terrainVS, terrainFS)
	gfx.objectSh, gfx.oLoc = loadShaderVS(objectVS, objectFS)
	gfx.objectSh.UpdateLocation(rl.ShaderLocVertexInstancetransform, rl.GetShaderLocationAttrib(gfx.objectSh, "instanceTransform"))
	gfx.skySh, gfx.sLoc = loadShader(skyFS)
	gfx.holoSh, gfx.hLoc = loadShader(holoFS)
	gfx.devilSh, gfx.dLoc = loadShaderVS(devilVS, devilFS)
	gfx.devilLoc = rl.GetShaderLocation(gfx.devilSh, "devil")
	gfx.devilMesh = rl.GenMeshPlane(1, 1, 32, 16)
	gfx.devilMat = rl.LoadMaterialDefault()
	gfx.devilMat.Shader = gfx.devilSh

	model := func(m rl.Mesh, sh rl.Shader) rl.Model {
		mod := rl.LoadModelFromMesh(m)
		mod.GetMaterials()[0].Shader = sh
		return mod
	}
	gfx.sphere = model(rl.GenMeshSphere(1, 24, 32), gfx.objectSh)
	gfx.hemi = model(rl.GenMeshHemiSphere(1, 16, 32), gfx.objectSh)
	gfx.cyl = model(rl.GenMeshCylinder(1, 1, 32), gfx.objectSh)
	gfx.cone = model(rl.GenMeshCone(1, 1, 32), gfx.objectSh)
	gfx.cube = model(rl.GenMeshCube(1, 1, 1), gfx.objectSh)
	gfx.crystal = model(rl.GenMeshCone(1, 1, 6), gfx.objectSh)
	gfx.sky = model(rl.GenMeshSphere(1, 24, 32), gfx.skySh)
	gfx.holo = model(rl.GenMeshSphere(1, 32, 48), gfx.holoSh)

	// Soft round sprite for particles and glows, generated in code.
	img := rl.GenImageGradientRadial(64, 64, 0, rl.White, rl.Blank)
	gfx.glow = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.SetTextureFilter(gfx.glow, rl.FilterBilinear)

	// Latin-1 covers French accents; the default raylib font is ASCII only.
	var runes []rune
	for r := rune(32); r < 256; r++ {
		runes = append(runes, r)
	}
	runes = append(runes, '…', '’', '→', '►', '◄')
	gfx.font = rl.LoadFontFromMemory(".ttf", goregular.TTF, 64, runes)
	gfx.bold = rl.LoadFontFromMemory(".ttf", gobold.TTF, 64, runes)
	gfx.smallcaps = rl.LoadFontFromMemory(".ttf", gosmallcaps.TTF, 96, runes)
	gfx.mono = rl.LoadFontFromMemory(".ttf", gomonobold.TTF, 64, runes)
	rl.SetTextureFilter(gfx.mono.Texture, rl.FilterBilinear)
	gfx.visor = genVisor()
	rl.SetTextureFilter(gfx.smallcaps.Texture, rl.FilterBilinear)
	rl.SetTextureFilter(gfx.font.Texture, rl.FilterBilinear)
	rl.SetTextureFilter(gfx.bold.Texture, rl.FilterBilinear)

	for _, s := range gfx.shaders() {
		rl.SetShaderValue(s.sh, s.loc.sunDir, v3f(sunDir), rl.ShaderUniformVec3)
	}
	for name, v := range map[string]float32{
		"faceCells": faceCells, "tileCells": tileCells, "tilesPerFace": tilesPerFace,
		"atlasTiles": atlasTiles, "marsRadius": marsRadius,
	} {
		rl.SetShaderValue(gfx.terrainSh, rl.GetShaderLocation(gfx.terrainSh, name), []float32{v}, rl.ShaderUniformFloat)
	}
	// One material shared by every terrain patch. Patches are freed with
	// UnloadMesh only: unloading a model would also free this shader.
	gfx.tex = bakeTextures()
	// Kept apart from Graphics: with cgo, passing a pointer into a Go object
	// makes the runtime check that whole object for Go pointers, and
	// Graphics holds slices.
	mat := rl.LoadMaterialDefault()
	gfx.terrainMat = &mat
	gfx.terrainMat.Shader = gfx.terrainSh
	gfx.terrainSh.UpdateLocation(rl.ShaderLocMapNormal, rl.GetShaderLocation(gfx.terrainSh, "texture2"))
	gfx.terrainSh.UpdateLocation(rl.ShaderLocMapRoughness, rl.GetShaderLocation(gfx.terrainSh, "texture3"))
	rl.SetMaterialTexture(gfx.terrainMat, rl.MapNormal, gfx.tex.regolith)
	rl.SetMaterialTexture(gfx.terrainMat, rl.MapRoughness, gfx.tex.dust)
	gfx.shadow = newShadowMap()
	gfx.lampShadow = newLampShadow()
	gfx.terrainSh.UpdateLocation(rl.ShaderLocMapBrdf, rl.GetShaderLocation(gfx.terrainSh, "shadowMap"))
	rl.SetMaterialTexture(gfx.terrainMat, mapShadow, gfx.shadow.target.Texture)

	gfx.signBox = model(rl.GenMeshCube(1, 1, 1), gfx.objectSh)
	for i, c := range []color.RGBA{accentCyan, accentBlue, accentCyan, accentOrange} {
		gfx.screens = append(gfx.screens, paintScreen(i, c))
	}
	for _, s := range baseSigns() {
		s.tex = paintSign(s, gfx.smallcaps)
		gfx.signs = append(gfx.signs, s)
	}

	// Every object model gets the painted textures; the shader picks one.
	gfx.objectMaps = []rl.Texture2D{gfx.tex.paint, gfx.tex.panel, gfx.tex.metal, gfx.shadow.target.Texture, gfx.tex.fabric, gfx.tex.regolith, gfx.tex.hazard}
	objectMaps := gfx.objectMaps
	for i := range objectMaps {
		gfx.objectSh.UpdateLocation(rl.ShaderLocMapAlbedo+int32(i), rl.GetShaderLocation(gfx.objectSh, fmt.Sprintf("texture%d", i)))
	}
	gfx.objectSh.UpdateLocation(rl.ShaderLocMapBrdf, rl.GetShaderLocation(gfx.objectSh, "lampShadowMap"))
	for _, m := range []rl.Model{gfx.sphere, gfx.hemi, gfx.cyl, gfx.cone, gfx.cube, gfx.crystal} {
		gfx.bindMaps(m)
	}
	gfx.rocks = rockModels(gfx.objectSh, objectMaps, rockShapes, 1300, 14, 18)
	gfx.pebbles = rockModels(gfx.objectSh, objectMaps, pebbleShapes, 1700, 5, 7)

	// Mars is big: see mountains far away, but keep the held tool, very
	// close to the eye, from being clipped.
	rl.SetClipPlanes(0.05, 9000)

	// The base's ceiling lamps never move.
	var lights []float32
	for _, l := range baseLights() {
		p := toWorld(l)
		lights = append(lights, float32(p.X), float32(p.Y), float32(p.Z), 1)
	}
	lights[3] = 1.4 // the hub is bigger
	rl.SetShaderValueV(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lights"), lights, rl.ShaderUniformVec4, int32(len(lights)/4))
	rl.SetShaderValue(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lightCount"), []float32{float32(len(lights) / 4)}, rl.ShaderUniformFloat)
	rl.SetShaderValue(gfx.holoSh, rl.GetShaderLocation(gfx.holoSh, "tilesPerFace"), []float32{tilesPerFace}, rl.ShaderUniformFloat)
	return gfx
}

type shaderWithLocs struct {
	sh  rl.Shader
	loc shaderLocs
}

func (gfx *Graphics) shaders() []shaderWithLocs {
	return []shaderWithLocs{{gfx.terrainSh, gfx.tLoc}, {gfx.objectSh, gfx.oLoc}, {gfx.skySh, gfx.sLoc}, {gfx.holoSh, gfx.hLoc}, {gfx.devilSh, gfx.dLoc}}
}

func v3f(v vec3) []float32 { return []float32{float32(v.X), float32(v.Y), float32(v.Z)} }

// put draws a unit model scaled, then rotated (angle in degrees around axis).
func (gfx *Graphics) put(m rl.Model, pos, scale, axis vec3, angle float64, c color.RGBA, spec, emissive float32) {
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.surface, []float32{spec, emissive}, rl.ShaderUniformVec2)
	rl.DrawModelEx(m, pos.rl(), axis.rl(), float32(angle), scale.rl(), c)
}

// segment stretches a unit cylinder or cone (base at y=0, top at y=1) from a to b.
func (gfx *Graphics) segment(m rl.Model, a, b vec3, radius float64, c color.RGBA, spec, emissive float32) {
	d := b.sub(a)
	l := d.length()
	if l < 1e-6 {
		return
	}
	dir := d.scale(1 / l)
	axis := vec3{0, 1, 0}.cross(dir)
	angle := math.Acos(math.Max(-1, math.Min(1, dir.Y))) * 180 / math.Pi
	if axis.length() < 1e-6 {
		axis = vec3{1, 0, 0}
	}
	gfx.put(m, a, vec3{radius, l, radius}, axis.norm(), angle, c, spec, emissive)
}

// camera follows the eye with a little life: a gentle bob and sway when
// walking, a slight tremor while the tool runs, and the field of view
// eased by updateFov.
func (g *Game) camera() rl.Camera3D {
	p := &g.player
	eye := p.Eye()
	walk := p.moveAmount
	right, up := p.Right(), p.LocalUp()
	eye = eye.add(up.scale(math.Sin(2*p.walkPhase) * 0.02 * walk)).add(right.scale(math.Sin(p.walkPhase) * 0.012 * walk))
	camUp := up.add(right.scale(math.Sin(p.walkPhase) * 0.008 * walk))
	if g.sweeping {
		eye = eye.add(right.scale(math.Sin(g.clock*37) * 0.002)).add(up.scale(math.Sin(g.clock*23+1) * 0.002))
	}
	g.camEye = eye
	return rl.Camera3D{
		Position:   eye.rl(),
		Target:     eye.add(p.Forward()).rl(),
		Up:         camUp.rl(),
		Fovy:       float32(g.fov),
		Projection: rl.CameraPerspective,
	}
}

// updateFov widens the view when running and narrows it a touch while
// sweeping, smoothly.
func (g *Game) updateFov(dt float64) {
	target := 70.0
	switch {
	case g.sweeping:
		target = 67
	case rl.IsKeyDown(rl.KeyLeftShift) && g.player.moveAmount > 0.5:
		target = 76
	}
	if g.fov == 0 {
		g.fov = target
	}
	g.fov += (target - g.fov) * (1 - math.Exp(-dt*5))
}

func (g *Game) setUniforms(cam rl.Camera3D) {
	gfx := g.gfx
	eye := []float32{cam.Position.X, cam.Position.Y, cam.Position.Z}
	t := []float32{float32(g.clock)}
	for _, s := range gfx.shaders() {
		rl.SetShaderValue(s.sh, s.loc.viewPos, eye, rl.ShaderUniformVec3)
		rl.SetShaderValue(s.sh, s.loc.time, t, rl.ShaderUniformFloat)
	}
	// The shaders' origin (shaderCommon): the eye snapped to a 32 m grid,
	// so the tile offsets they derive from it stay put between snaps.
	e := g.camEye
	snap := func(v float64) float64 { return float64(float32(math.Floor(v/32) * 32)) }
	o := vec3{snap(e.X), snap(e.Y), snap(e.Z)}
	for _, s := range gfx.shaders() {
		rl.SetShaderValue(s.sh, rl.GetShaderLocation(s.sh, "origin"), v3f(o), rl.ShaderUniformVec3)
		rl.SetShaderValue(s.sh, rl.GetShaderLocation(s.sh, "viewRel"), v3f(e.sub(o)), rl.ShaderUniformVec3)
	}
	rl.SetShaderValue(gfx.skySh, gfx.sLoc.up, v3f(g.player.LocalUp()), rl.ShaderUniformVec3)
	// Objects take the relief's shadow from where the player stands: those
	// that matter are close, far ones are too small to tell.
	sun, ao := g.terrain.light(g.terrain.Surface(g.player.Pos))
	g.dbg.sun, g.dbg.ao = sun, ao
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.ground, []float32{float32(sun), float32(ao)}, rl.ShaderUniformVec2)

	brush := []float32{0, 0, 0, 0}
	if g.aimOK && g.mode == modePlay {
		brush = []float32{float32(g.aim.X), float32(g.aim.Y), float32(g.aim.Z), float32(g.tool().Radius)}
	}
	rl.SetShaderValue(gfx.terrainSh, gfx.tLoc.brush, brush, rl.ShaderUniformVec4)
	g.steps.upload(gfx)

	// Soft round shadows of the base's domes, for when the base is beyond
	// the shadow map.
	var casters []float32
	for _, r := range rooms {
		c := toWorld(r.center.add(vec3{0, r.wallH * 0.7, 0}))
		casters = append(casters, float32(c.X), float32(c.Y), float32(c.Z), float32(r.radius*1.05))
	}
	n := min(len(casters)/4, maxCasters)
	rl.SetShaderValueV(gfx.terrainSh, gfx.tLoc.casters, casters[:n*4], rl.ShaderUniformVec4, int32(n))
	rl.SetShaderValue(gfx.terrainSh, gfx.tLoc.casterCount, []float32{float32(n)}, rl.ShaderUniformFloat)
	// As four columns: passing a Matrix by value is unreliable without cgo.
	m := gfx.shadow.lightVP
	cols := []float32{m.M0, m.M1, m.M2, m.M3, m.M4, m.M5, m.M6, m.M7, m.M8, m.M9, m.M10, m.M11, m.M12, m.M13, m.M14, m.M15}
	rl.SetShaderValueV(gfx.terrainSh, gfx.tLoc.lightVP, cols, rl.ShaderUniformVec4, 4)
	rl.SetShaderValueV(gfx.objectSh, gfx.oLoc.lightVP, cols, rl.ShaderUniformVec4, 4)
}

func (g *Game) Draw() {
	gfx := g.gfx
	cam := g.camera()

	atlas, lookup := g.world.Upload(g.player.Pos)
	rl.SetMaterialTexture(gfx.terrainMat, rl.MapDiffuse, atlas)
	rl.SetMaterialTexture(gfx.terrainMat, rl.MapSpecular, lookup)
	patches := g.terrain.Visible(g.player.Eye())
	g.dbg.patches = len(patches)
	g.nearGems = g.world.GemsNear(g.player.Pos, gemDrawDist)
	// Objects cast their shadows first, then the scene is lit with them.
	gfx.shadow.render(gfx, g.player.Pos, func() {
		g.drawRocks()
		g.drawBase()
		g.drawDrones()
	})
	// The lamp of the room the player is in casts shadows too.
	room := -1
	eye := toLocal(g.player.Eye())
	for i, r := range rooms {
		if d := eye.sub(r.center); math.Hypot(d.X, d.Z) < r.radius {
			room = i
		}
	}
	if room >= 0 && gfx.base != nil {
		gfx.lampShadow.update(gfx, room, toWorld(baseLights()[room]).sub(vec3{0, 0.12, 0}), func() { g.drawBase() })
	}
	g.setUniforms(cam)
	m := gfx.lampShadow.lightVP
	rl.SetShaderValueV(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lampVP"), []float32{m.M0, m.M1, m.M2, m.M3, m.M4, m.M5, m.M6, m.M7, m.M8, m.M9, m.M10, m.M11, m.M12, m.M13, m.M14, m.M15}, rl.ShaderUniformVec4, 4)
	lampIdx := float32(-1)
	if room >= 0 && gfx.lampShadow.room == room {
		lampIdx = float32(room)
	}
	rl.SetShaderValue(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lampShadowIdx"), []float32{lampIdx}, rl.ShaderUniformFloat)

	sunUp := g.player.Pos.norm().dot(sunDir)
	gfx.post.adapt(float64(rl.GetFrameTime()), sunUp, indoors(toLocal(g.player.Eye())))
	gfx.post.resize()
	rl.BeginTextureMode(gfx.post.scene)
	rl.ClearBackground(rl.Black)
	rl.BeginMode3D(cam)

	// Sky: a big sphere around the camera, seen from inside, behind everything.
	rl.DisableBackfaceCulling()
	rl.DisableDepthMask()
	rl.DrawModel(gfx.sky, cam.Position, 400, rl.White)
	rl.EnableDepthMask()
	rl.EnableBackfaceCulling()

	// Culling is off: patch winding differs from one cube face to the next.
	rl.DisableBackfaceCulling()
	for _, n := range patches {
		rl.DrawMesh(n.mesh, *gfx.terrainMat, rl.MatrixIdentity())
	}
	rl.EnableBackfaceCulling()
	g.drawRocks()
	g.drawPebbles()
	g.drawBase()
	g.drawDrones()
	g.drawGems()
	g.drawDevils()
	if indoors(toLocal(g.player.Eye())) {
		gfx.setLighting(litIndoor, vec3{})
	}
	g.drawTool()
	gfx.setLighting(litSun, vec3{})
	g.drawGlows(cam)
	g.drawGlobe(cam)
	rl.EndMode3D()
	rl.EndTextureMode()

	rl.BeginDrawing()
	gfx.post.finish(cam, sunUp, g.clock)
	g.drawHUD(cam)
	rl.EndDrawing()
}

var (
	matWhite = color.RGBA{232, 234, 240, 255}
	matSteel = color.RGBA{150, 155, 170, 255}
	matDark  = color.RGBA{48, 52, 66, 255}
)

func (g *Game) drawDrones() {
	gfx := g.gfx
	for _, d := range g.drones {
		up := d.Pos.norm()
		axis, angle := orient(up)
		downAxis, downAngle := orient(up.scale(-1))
		t1, t2 := tangentBasis(up)
		p := d.Pos.add(up.scale(math.Sin(d.phase*3) * 0.08))
		gfx.setMat(mtPaint, 2, true)
		gfx.put(gfx.sphere, p, vec3{0.32, 0.15, 0.32}, axis, angle, matWhite, 0.8, 0)
		gfx.put(gfx.sphere, p.add(up.scale(0.05)), vec3{0.2, 0.09, 0.2}, axis, angle, accentOrange, 0.6, 0)
		gfx.setMat(mtMetal, 3, true)
		gfx.put(gfx.hemi, p.sub(up.scale(0.06)), vec3{0.15, 0.12, 0.15}, downAxis, downAngle, matDark, 1, 0)
		for i := range 4 {
			a := math.Pi/4 + float64(i)*math.Pi/2
			arm := p.add(t1.scale(math.Cos(a) * 0.45)).add(t2.scale(math.Sin(a) * 0.45)).add(up.scale(0.05))
			gfx.segment(gfx.cyl, p, arm, 0.025, matSteel, 0.4, 0)
			gfx.setMat(mtPlain, 1, false)
			gfx.segment(gfx.cyl, arm.add(up.scale(0.04)), arm.add(up.scale(0.052)), 0.19, color.RGBA{60, 64, 80, 150}, 0.2, 0)
			gfx.setMat(mtMetal, 3, true)
		}
		gfx.setMat(mtPlain, 1, false)
		light := colGreen
		if g.tankFull() {
			light = colRed
		}
		if math.Sin(d.phase*6) > 0 {
			gfx.put(gfx.sphere, p.add(up.scale(0.15)), vec3{0.045, 0.045, 0.045}, axis, angle, light, 0, 1)
		}
	}
}

// gemDrawDist: crystals are only drawn this close; they are tiny anyway.
const gemDrawDist = 80.0

// gemVisibility is how much of a crystal pokes out of the dust (0..1).
func (g *Game) gemVisibility(gem Gem) float64 {
	return clamp01(1 - g.world.DustAt(gem.Dir)*2.5)
}

func (g *Game) gemPos(gem Gem, s float64) vec3 {
	lift := s*1.4 + math.Sin(g.clock*2+float64(gem.Key))*0.05
	return g.terrain.Surface(gem.Dir).add(gem.Dir.scale(lift))
}

func (g *Game) drawGems() {
	gfx := g.gfx
	for _, gem := range g.nearGems {
		vis := g.gemVisibility(gem)
		if vis <= 0 {
			continue
		}
		s := 0.06 + 0.16*vis
		p := g.gemPos(gem, s)
		axis, angle := orient(gem.Dir)
		downAxis, downAngle := orient(gem.Dir.scale(-1))
		gfx.put(gfx.crystal, p, vec3{s, s * 1.7, s}, axis, angle, colCyan, 1, 0.45)
		gfx.put(gfx.crystal, p, vec3{s, s * 1.2, s}, downAxis, downAngle, color.RGBA{60, 170, 220, 255}, 1, 0.45)
	}
}

// drawTool draws the held tool, in the lower right corner of the view.
func (g *Game) drawTool() {
	if g.mode == modePaused {
		return
	}
	gfx, look := g.gfx, g.tool().Look
	base, dir := g.toolBase, g.toolDir
	if g.sweeping {
		base = base.add(vec3{(g.rng.Float64() - 0.5) * 0.006, (g.rng.Float64() - 0.5) * 0.006, 0})
	}
	const k = toolViewScale
	neck := base.add(dir.scale(look.HandleLen * k))
	tip := neck.add(dir.scale(look.HeadLen * k))
	hr, r := look.HandleR*k, look.HeadR*k

	gfx.setMat(mtPaint, 8, true)
	gfx.segment(gfx.cyl, base, neck, hr, look.HandleCol, 0.5, 0)
	// A dark rubber grip where the hand holds it, with a cyan ring.
	grip := base.add(dir.scale(look.HandleLen * 0.35 * k))
	gfx.setMat(mtPlain, 1, false)
	gfx.segment(gfx.cyl, base.sub(dir.scale(0.01*k)), grip, hr*1.3, matDark, 0.2, 0)
	gfx.segment(gfx.cyl, grip, grip.add(dir.scale(0.012*k)), hr*1.32, accentCyan, 0.3, 0)
	gfx.setMat(mtMetal, 6, true)
	collar := math.Max(hr, r) * 1.15
	if look.Head == headFlare || look.Head == headBlock {
		collar = hr * 1.5 // flat heads: a slim collar
	}
	if look.Head != headNozzle {
		gfx.segment(gfx.cyl, neck.sub(dir.scale(0.02*k)), neck.add(dir.scale(0.02*k)), collar, matSteel, 0.8, 0)
	}
	right := g.player.Right()
	across := right.sub(dir.scale(right.dot(dir))).norm()
	top := dir.cross(across)
	if top.dot(g.player.LocalUp()) < 0 {
		top = top.scale(-1)
	}
	switch look.Head {
	case headBlock:
		// A toothbrush: a flat plastic head, a white tuft of bristles
		// standing under it.
		at := neck.add(dir.scale(look.HeadLen * 0.5 * k))
		gfx.setMat(mtPaint, 8, true)
		gfx.putBasis(gfx.cube, at, across.scale(r*2.2), dir.scale(look.HeadLen*k), top.scale(r*0.9), look.HandleCol, 0.5, 0)
		gfx.setMat(mtFabric, 12, true)
		gfx.putBasis(gfx.cube, at.sub(top.scale(r*0.9)), across.scale(r*1.9), dir.scale(look.HeadLen*0.85*k), top.scale(r*1.2), look.HeadCol, 0.1, 0)
		gfx.setMat(mtPlain, 1, false)
		gfx.putBasis(gfx.cube, at.add(top.scale(r*0.46)), across.scale(r*0.5), dir.scale(look.HeadLen*0.6*k), top.scale(0.004*k), accentCyan, 0, 0.3)
	case headTaper:
		// A paintbrush: a steel ferrule, then a tuft of bristles narrowing
		// to a tip dipped in paint.
		ferrule := neck.add(dir.scale(look.HeadLen * 0.3 * k))
		body := ferrule.add(dir.scale(look.HeadLen * 0.35 * k))
		gfx.segment(gfx.cyl, neck, ferrule, r*0.85, matSteel, 0.9, 0)
		gfx.setMat(mtFabric, 12, true)
		gfx.segment(gfx.cyl, ferrule, body, r, look.HeadCol, 0.2, 0)
		gfx.segment(gfx.cone, body, tip, r, look.HeadCol, 0.2, 0)
		gfx.setMat(mtPlain, 1, false)
		gfx.segment(gfx.cone, body.lerp(tip, 0.55), tip.add(dir.scale(0.002*k)), r*0.45, accentOrange, 0.4, 0)
	case headFlare:
		// A push broom: a painted head across the handle with steel end
		// brackets and a hazard band, a thick row of bristles below.
		width := look.HeadR * 3.2 * k
		at := neck.add(dir.scale(0.03 * k))
		gfx.setMat(mtPaint, 8, true)
		gfx.putBasis(gfx.cube, at, across.scale(width), dir.scale(0.06*k), depth(across, dir).scale(0.09*k), look.HandleCol, 0.3, 0)
		gfx.setMat(mtHazard, 12, false)
		gfx.putBasis(gfx.cube, at.add(top.scale(0.046*k)), across.scale(width*0.6), dir.scale(0.05*k), depth(across, dir).scale(0.004*k), colWhite, 0.3, 0)
		gfx.setMat(mtMetal, 6, true)
		for _, sd := range []float64{-1, 1} {
			gfx.putBasis(gfx.cube, at.add(across.scale(sd*width*0.5)), across.scale(0.02*k), dir.scale(0.08*k), depth(across, dir).scale(0.1*k), matSteel, 0.8, 0)
		}
		gfx.setMat(mtFabric, 10, true)
		gfx.putBasis(gfx.cube, neck.add(dir.scale(0.12*k)), across.scale(width*0.96), dir.scale(0.13*k), depth(across, dir).scale(0.07*k), look.HeadCol, 0.1, 0)
	case headNozzle:
		// A blower in the kit's language: a painted body with a dark rear
		// cap and a hazard band, a steel nozzle in two steps, a glowing ring
		// at the mouth and a status light on top.
		body := neck.add(dir.scale(look.HeadLen * 0.5 * k))
		mid := body.add(dir.scale(look.HeadLen * 0.28 * k))
		gfx.setMat(mtPlain, 1, false)
		gfx.segment(gfx.cyl, neck.sub(dir.scale(0.04*k)), neck.sub(dir.scale(0.02*k)), r*0.9, matDark, 0.3, 0)
		gfx.setMat(mtPaint, 6, true)
		gfx.segment(gfx.cyl, neck.sub(dir.scale(0.02*k)), body, r, look.HeadCol, 0.6, 0)
		gfx.setMat(mtHazard, 3, true)
		gfx.segment(gfx.cyl, body.sub(dir.scale(0.04*k)), body.sub(dir.scale(0.01*k)), r*1.03, colWhite, 0.3, 0)
		gfx.setMat(mtMetal, 6, true)
		gfx.segment(gfx.cyl, body, mid, r*0.74, matSteel, 0.8, 0)
		gfx.segment(gfx.cyl, mid, tip, r*0.54, matSteel, 0.8, 0)
		gfx.setMat(mtPlain, 1, false)
		gfx.segment(gfx.cyl, mid.sub(dir.scale(0.008*k)), mid, r*0.8, matDark, 0.3, 0)
		gfx.segment(gfx.cyl, tip.sub(dir.scale(0.012*k)), tip.add(dir.scale(0.002*k)), r*0.58, colCyan, 0, look.Glow)
		light := colGreen
		if g.tankFull() {
			light = colRed
		}
		gfx.putBasis(gfx.cube, neck.add(dir.scale(look.HeadLen*0.22*k)).add(top.scale(r)), across.scale(r*0.45), dir.scale(look.HeadLen*0.22*k), top.scale(0.012*k), light, 0, 0.9)
	}
	gfx.setMat(mtPlain, 1, false)
}

// depth is the third axis of a flat head lying across the handle.
func depth(across, dir vec3) vec3 { return across.cross(dir) }

// drawGlows draws the soft sprites: dust particles and crystal glows.
// softness fades a particle as it reaches the ground, so its sprite
// doesn't get cut by the terrain.
func (g *Game) softness(p Particle) float64 {
	return smoothstep(0, p.Size*3, g.terrain.Altitude(p.Pos))
}

func (g *Game) drawGlows(cam rl.Camera3D) {
	gfx := g.gfx
	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()

	for _, p := range g.particles {
		if p.Additive {
			continue
		}
		c := p.C
		c.A = uint8(200 * clamp01(p.Life/p.MaxLife*1.5) * g.softness(p))
		rl.DrawBillboard(cam, gfx.glow, p.Pos.rl(), float32(p.Size*3), c)
	}

	rl.BeginBlendMode(rl.BlendAdditive)
	for _, p := range g.particles {
		if !p.Additive {
			continue
		}
		c := p.C
		c.A = uint8(255 * clamp01(p.Life/p.MaxLife*1.5) * g.softness(p))
		rl.DrawBillboard(cam, gfx.glow, p.Pos.rl(), float32(p.Size*4), c)
	}
	for _, gem := range g.nearGems {
		vis := g.gemVisibility(gem)
		if vis <= 0 {
			continue
		}
		pulse := 0.75 + 0.25*math.Sin(g.clock*3+float64(gem.Key))
		c := color.RGBA{80, 200, 230, uint8(160 * vis * pulse)}
		rl.DrawBillboard(cam, gfx.glow, g.gemPos(gem, 0.06+0.16*vis).rl(), float32(1.4*vis), c)
	}
	if math.Mod(g.clock, 1.2) < 0.3 {
		rl.DrawBillboard(cam, gfx.glow, beaconTop().rl(), 0.9, color.RGBA{255, 80, 60, 200})
	}
	rl.EndBlendMode()
	rl.EnableDepthMask()
}

// bindMaps gives a model of the object shader the painted textures and the
// lamp's shadow map.
func (gfx *Graphics) bindMaps(m rl.Model) {
	for i, t := range gfx.objectMaps {
		rl.SetMaterialTexture(&m.GetMaterials()[0], int32(i), t)
	}
	rl.SetMaterialTexture(&m.GetMaterials()[0], mapShadow, gfx.lampShadow.target.Texture)
}

// putBasis draws a unit model with its X, Y and Z axes sent to ax, ay and az
// (their lengths scale it): any orientation, any proportions.
func (gfx *Graphics) putBasis(m rl.Model, pos, ax, ay, az vec3, c color.RGBA, spec, emissive float32) {
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.surface, []float32{spec, emissive}, rl.ShaderUniformVec2)
	mat := m.GetMaterials()[0]
	mat.GetMap(rl.MapAlbedo).Color = c
	rl.DrawMesh(m.GetMeshes()[0], mat, basisMatrix(pos, ax, ay, az))
	mat.GetMap(rl.MapAlbedo).Color = rl.White
}
