package main

import (
	"fmt"
	"image/color"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/mesh"
	"mars_sweeper/internal/planet"
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
	sunDir = vec3{-0.5, 0.62, -0.6}.Norm()
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
	dust                                              dustGPU // the dust tiles near the player, on the GPU
	base                                              *building
	doorPanel                                         rl.Model
	entities                                          map[string][]entityModel // by entity ID, built on first draw
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

func loadShader(fs string) (rl.Shader, shaderLocs) { return loadShaderVS(glsl("mesh.vert"), fs) }

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
	gfx := &Graphics{entities: map[string][]entityModel{}}
	gfx.post = newPostFX()
	gfx.terrainSh, gfx.tLoc = loadShaderVS(glsl("terrain.vert"), glsl("terrain.frag"))
	gfx.objectSh, gfx.oLoc = loadShaderVS(glsl("object.vert"), glsl("object.frag"))
	gfx.objectSh.UpdateLocation(rl.ShaderLocVertexInstancetransform, rl.GetShaderLocationAttrib(gfx.objectSh, "instanceTransform"))
	gfx.skySh, gfx.sLoc = loadShader(glsl("sky.frag"))
	gfx.holoSh, gfx.hLoc = loadShader(glsl("holo.frag"))
	gfx.devilSh, gfx.dLoc = loadShaderVS(glsl("devil.vert"), glsl("devil.frag"))
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
		"faceCells": planet.FaceCells, "tileCells": planet.TileCells, "tilesPerFace": planet.TilesPerFace,
		"atlasTiles": atlasTiles, "marsRadius": planet.Radius,
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
	gfx.lampShadow = newLampShadow(len(rooms))
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
	gfx.rocks = rockModels(gfx.objectSh, objectMaps, planet.RockShapes, 1300, 14, 18)
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
	rl.SetShaderValue(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lampCount"), []float32{float32(len(rooms))}, rl.ShaderUniformFloat)
	rl.SetShaderValue(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lampGrid"), []float32{float32(gfx.lampShadow.grid)}, rl.ShaderUniformFloat)
	rl.SetShaderValueV(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lights"), lights, rl.ShaderUniformVec4, int32(len(lights)/4))
	rl.SetShaderValue(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lightCount"), []float32{float32(len(lights) / 4)}, rl.ShaderUniformFloat)
	rl.SetShaderValue(gfx.holoSh, rl.GetShaderLocation(gfx.holoSh, "tilesPerFace"), []float32{planet.TilesPerFace}, rl.ShaderUniformFloat)
	return gfx
}

// toRL is a vector as raylib takes it, in float32.
func toRL(a vec3) rl.Vector3 { return rl.NewVector3(float32(a.X), float32(a.Y), float32(a.Z)) }

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
	rl.DrawModelEx(m, toRL(pos), toRL(axis), float32(angle), toRL(scale), c)
}

// segment stretches a unit cylinder or cone (base at y=0, top at y=1) from a to b.
func (gfx *Graphics) segment(m rl.Model, a, b vec3, radius float64, c color.RGBA, spec, emissive float32) {
	d := b.Sub(a)
	l := d.Length()
	if l < 1e-6 {
		return
	}
	dir := d.Scale(1 / l)
	axis := vec3{0, 1, 0}.Cross(dir)
	angle := math.Acos(math.Max(-1, math.Min(1, dir.Y))) * 180 / math.Pi
	if axis.Length() < 1e-6 {
		axis = vec3{1, 0, 0}
	}
	gfx.put(m, a, vec3{radius, l, radius}, axis.Norm(), angle, c, spec, emissive)
}

// camera follows the eye with a little life: a gentle bob and sway when
// walking, a slight tremor while the tool runs, and the field of view
// eased by updateFov.
func (g *Game) camera() rl.Camera3D {
	p := &g.player
	eye := p.Eye()
	walk := p.moveAmount
	right, up := p.Right(), p.LocalUp()
	eye = eye.Add(up.Scale(math.Sin(2*p.walkPhase) * 0.02 * walk)).Add(right.Scale(math.Sin(p.walkPhase) * 0.012 * walk))
	camUp := up.Add(right.Scale(math.Sin(p.walkPhase) * 0.008 * walk))
	if g.sweeping {
		eye = eye.Add(right.Scale(math.Sin(g.clock*37) * 0.002)).Add(up.Scale(math.Sin(g.clock*23+1) * 0.002))
	}
	g.camEye = eye
	return rl.Camera3D{
		Position:   toRL(eye),
		Target:     toRL(eye.Add(p.Forward())),
		Up:         toRL(camUp),
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
	// The shaders' origin (common.glsl): the eye snapped to a 32 m grid,
	// so the tile offsets they derive from it stay put between snaps.
	e := g.camEye
	snap := func(v float64) float64 { return float64(float32(math.Floor(v/32) * 32)) }
	o := vec3{snap(e.X), snap(e.Y), snap(e.Z)}
	g.origin = o
	for _, s := range gfx.shaders() {
		rl.SetShaderValue(s.sh, rl.GetShaderLocation(s.sh, "origin"), v3f(o), rl.ShaderUniformVec3)
		rl.SetShaderValue(s.sh, rl.GetShaderLocation(s.sh, "viewRel"), v3f(e.Sub(o)), rl.ShaderUniformVec3)
	}
	// The base's shapes, for its ambient occlusion (baseAO).
	var rms, crs, csz []float32
	for _, r := range rooms {
		rms = append(rms, float32(r.center.X), float32(r.center.Z), float32(r.radius), float32(r.wallH))
	}
	for _, c := range corridors {
		crs = append(crs, float32(c.from.X), float32(c.from.Z), float32(c.to.X), float32(c.to.Z))
		csz = append(csz, float32(c.width), float32(c.height), 0, 0)
	}
	var blobs []float32
	for _, ob := range baseObstacles[:min(len(baseObstacles), 48)] {
		blobs = append(blobs, float32(ob.at.X), float32(ob.at.Z), float32(ob.radius), 0)
	}
	for _, sh := range []rl.Shader{gfx.terrainSh, gfx.objectSh} {
		rl.SetShaderValueV(sh, rl.GetShaderLocation(sh, "baseBlobs"), blobs, rl.ShaderUniformVec4, int32(len(blobs)/4))
		rl.SetShaderValue(sh, rl.GetShaderLocation(sh, "blobCount"), []float32{float32(len(blobs) / 4)}, rl.ShaderUniformFloat)
		rl.SetShaderValue(sh, rl.GetShaderLocation(sh, "baseRel"), v3f(basePos.Sub(o)), rl.ShaderUniformVec3)
		rl.SetShaderValueV(sh, rl.GetShaderLocation(sh, "baseRooms"), rms, rl.ShaderUniformVec4, int32(len(rms)/4))
		rl.SetShaderValueV(sh, rl.GetShaderLocation(sh, "baseCorrs"), crs, rl.ShaderUniformVec4, int32(len(crs)/4))
		rl.SetShaderValueV(sh, rl.GetShaderLocation(sh, "baseCorrSize"), csz, rl.ShaderUniformVec4, int32(len(csz)/4))
		rl.SetShaderValue(sh, rl.GetShaderLocation(sh, "roomCount"), []float32{float32(len(rms) / 4)}, rl.ShaderUniformFloat)
		rl.SetShaderValue(sh, rl.GetShaderLocation(sh, "corrCount"), []float32{float32(len(crs) / 4)}, rl.ShaderUniformFloat)
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
		c := toWorld(r.center.Add(vec3{0, r.wallH * 0.7, 0}))
		casters = append(casters, float32(c.X), float32(c.Y), float32(c.Z), float32(r.radius*1.05))
	}
	n := min(len(casters)/4, maxCasters)
	rl.SetShaderValueV(gfx.terrainSh, gfx.tLoc.casters, casters[:n*4], rl.ShaderUniformVec4, int32(n))
	rl.SetShaderValue(gfx.terrainSh, gfx.tLoc.casterCount, []float32{float32(n)}, rl.ShaderUniformFloat)
	cols := gfx.shadow.cols(o)
	rl.SetShaderValueV(gfx.terrainSh, gfx.tLoc.lightVP, cols, rl.ShaderUniformVec4, 4)
	rl.SetShaderValueV(gfx.objectSh, gfx.oLoc.lightVP, cols, rl.ShaderUniformVec4, 4)
}

func (g *Game) Draw() {
	gfx := g.gfx
	cam := g.camera()

	atlas, lookup := gfx.dust.Upload(g.world, g.player.Pos)
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
		g.drawEntities()
	})
	// The rooms' lamps cast shadows too, drawn once.
	g.setUniforms(cam)
	if gfx.base != nil {
		var lamps []vec3
		for _, l := range baseLights()[:len(rooms)] {
			lamps = append(lamps, toWorld(l).Sub(vec3{0, 0.12, 0}))
		}
		gfx.lampShadow.render(gfx, lamps, func() { g.drawBase() })
		vp := gfx.lampShadow.cols(g.origin)
		rl.SetShaderValueV(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lampVP"), vp, rl.ShaderUniformVec4, int32(len(vp)/4))
	}

	sunUp := g.player.Pos.Norm().Dot(sunDir)
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

	// The base first: the terrain it hides then fails the depth test before
	// its costly shader runs (most of the screen, indoors).
	g.drawBase()
	g.drawDrones()
	g.drawEntities()
	// Culling is off: patch winding differs from one cube face to the next.
	rl.DisableBackfaceCulling()
	for _, n := range patches {
		rl.DrawMesh(n.mesh, *gfx.terrainMat, rl.MatrixIdentity())
	}
	rl.EnableBackfaceCulling()
	g.drawRocks()
	g.drawPebbles()
	g.drawGems()
	g.drawDevils()
	if indoors(toLocal(g.player.Eye())) {
		gfx.setLighting(mesh.LitIndoor, vec3{})
	}
	g.drawTool()
	gfx.setLighting(mesh.LitSun, vec3{})
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
		up := d.Pos.Norm()
		axis, angle := mathx.Orient(up)
		downAxis, downAngle := mathx.Orient(up.Scale(-1))
		t1, t2 := mathx.TangentBasis(up)
		p := d.Pos.Add(up.Scale(math.Sin(d.phase*3) * 0.08))
		gfx.setMat(mtPaint, 2, true)
		gfx.put(gfx.sphere, p, vec3{0.32, 0.15, 0.32}, axis, angle, matWhite, 0.8, 0)
		gfx.put(gfx.sphere, p.Add(up.Scale(0.05)), vec3{0.2, 0.09, 0.2}, axis, angle, accentOrange, 0.6, 0)
		gfx.setMat(mtMetal, 3, true)
		gfx.put(gfx.hemi, p.Sub(up.Scale(0.06)), vec3{0.15, 0.12, 0.15}, downAxis, downAngle, matDark, 1, 0)
		for i := range 4 {
			a := math.Pi/4 + float64(i)*math.Pi/2
			arm := p.Add(t1.Scale(math.Cos(a) * 0.45)).Add(t2.Scale(math.Sin(a) * 0.45)).Add(up.Scale(0.05))
			gfx.segment(gfx.cyl, p, arm, 0.025, matSteel, 0.4, 0)
			gfx.setMat(mtPlain, 1, false)
			gfx.segment(gfx.cyl, arm.Add(up.Scale(0.04)), arm.Add(up.Scale(0.052)), 0.19, color.RGBA{60, 64, 80, 150}, 0.2, 0)
			gfx.setMat(mtMetal, 3, true)
		}
		gfx.setMat(mtPlain, 1, false)
		light := colGreen
		if g.tankFull() {
			light = colRed
		}
		if math.Sin(d.phase*6) > 0 {
			gfx.put(gfx.sphere, p.Add(up.Scale(0.15)), vec3{0.045, 0.045, 0.045}, axis, angle, light, 0, 1)
		}
	}
}

// gemDrawDist: crystals are only drawn this close; they are tiny anyway.
const gemDrawDist = 80.0

// gemVisibility is how much of a crystal pokes out of the dust (0..1).
func (g *Game) gemVisibility(gem planet.Gem) float64 {
	return mathx.Clamp01(1 - g.world.DustAt(gem.Dir)*2.5)
}

func (g *Game) gemPos(gem planet.Gem, s float64) vec3 {
	lift := s*1.4 + math.Sin(g.clock*2+float64(gem.Key))*0.05
	return g.terrain.Surface(gem.Dir).Add(gem.Dir.Scale(lift))
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
		axis, angle := mathx.Orient(gem.Dir)
		downAxis, downAngle := mathx.Orient(gem.Dir.Scale(-1))
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
		base = base.Add(vec3{(g.rng.Float64() - 0.5) * 0.006, (g.rng.Float64() - 0.5) * 0.006, 0})
	}
	const k = toolViewScale
	neck := base.Add(dir.Scale(look.HandleLen * k))
	tip := neck.Add(dir.Scale(look.HeadLen * k))
	hr, r := look.HandleR*k, look.HeadR*k

	gfx.setMat(mtPaint, 8, true)
	gfx.segment(gfx.cyl, base, neck, hr, look.HandleCol, 0.5, 0)
	// A dark rubber grip where the hand holds it, with a cyan ring.
	grip := base.Add(dir.Scale(look.HandleLen * 0.35 * k))
	gfx.setMat(mtPlain, 1, false)
	gfx.segment(gfx.cyl, base.Sub(dir.Scale(0.01*k)), grip, hr*1.3, matDark, 0.2, 0)
	gfx.segment(gfx.cyl, grip, grip.Add(dir.Scale(0.012*k)), hr*1.32, accentCyan, 0.3, 0)
	gfx.setMat(mtMetal, 6, true)
	collar := math.Max(hr, r) * 1.15
	if look.Head == headFlare || look.Head == headBlock {
		collar = hr * 1.5 // flat heads: a slim collar
	}
	if look.Head != headNozzle {
		gfx.segment(gfx.cyl, neck.Sub(dir.Scale(0.02*k)), neck.Add(dir.Scale(0.02*k)), collar, matSteel, 0.8, 0)
	}
	right := g.player.Right()
	across := right.Sub(dir.Scale(right.Dot(dir))).Norm()
	top := dir.Cross(across)
	if top.Dot(g.player.LocalUp()) < 0 {
		top = top.Scale(-1)
	}
	switch look.Head {
	case headBlock:
		// A toothbrush: a flat plastic head, a white tuft of bristles
		// standing under it.
		at := neck.Add(dir.Scale(look.HeadLen * 0.5 * k))
		gfx.setMat(mtPaint, 8, true)
		gfx.putBasis(gfx.cube, at, across.Scale(r*2.2), dir.Scale(look.HeadLen*k), top.Scale(r*0.9), look.HandleCol, 0.5, 0)
		gfx.setMat(mtFabric, 12, true)
		gfx.putBasis(gfx.cube, at.Sub(top.Scale(r*0.9)), across.Scale(r*1.9), dir.Scale(look.HeadLen*0.85*k), top.Scale(r*1.2), look.HeadCol, 0.1, 0)
		gfx.setMat(mtPlain, 1, false)
		gfx.putBasis(gfx.cube, at.Add(top.Scale(r*0.46)), across.Scale(r*0.5), dir.Scale(look.HeadLen*0.6*k), top.Scale(0.004*k), accentCyan, 0, 0.3)
	case headTaper:
		// A paintbrush: a steel ferrule, then a tuft of bristles narrowing
		// to a tip dipped in paint.
		ferrule := neck.Add(dir.Scale(look.HeadLen * 0.3 * k))
		body := ferrule.Add(dir.Scale(look.HeadLen * 0.35 * k))
		gfx.segment(gfx.cyl, neck, ferrule, r*0.85, matSteel, 0.9, 0)
		gfx.setMat(mtFabric, 12, true)
		gfx.segment(gfx.cyl, ferrule, body, r, look.HeadCol, 0.2, 0)
		gfx.segment(gfx.cone, body, tip, r, look.HeadCol, 0.2, 0)
		gfx.setMat(mtPlain, 1, false)
		gfx.segment(gfx.cone, body.Lerp(tip, 0.55), tip.Add(dir.Scale(0.002*k)), r*0.45, accentOrange, 0.4, 0)
	case headFlare:
		// A push broom: a painted head across the handle with steel end
		// brackets and a hazard band, a thick row of bristles below.
		width := look.HeadR * 3.2 * k
		at := neck.Add(dir.Scale(0.03 * k))
		gfx.setMat(mtPaint, 8, true)
		gfx.putBasis(gfx.cube, at, across.Scale(width), dir.Scale(0.06*k), depth(across, dir).Scale(0.09*k), look.HandleCol, 0.3, 0)
		gfx.setMat(mtHazard, 12, false)
		gfx.putBasis(gfx.cube, at.Add(top.Scale(0.046*k)), across.Scale(width*0.6), dir.Scale(0.05*k), depth(across, dir).Scale(0.004*k), colWhite, 0.3, 0)
		gfx.setMat(mtMetal, 6, true)
		for _, sd := range []float64{-1, 1} {
			gfx.putBasis(gfx.cube, at.Add(across.Scale(sd*width*0.5)), across.Scale(0.02*k), dir.Scale(0.08*k), depth(across, dir).Scale(0.1*k), matSteel, 0.8, 0)
		}
		gfx.setMat(mtFabric, 10, true)
		gfx.putBasis(gfx.cube, neck.Add(dir.Scale(0.12*k)), across.Scale(width*0.96), dir.Scale(0.13*k), depth(across, dir).Scale(0.07*k), look.HeadCol, 0.1, 0)
	case headNozzle:
		// A blower in the kit's language: a painted body with a dark rear
		// cap and a hazard band, a steel nozzle in two steps, a glowing ring
		// at the mouth and a status light on top.
		body := neck.Add(dir.Scale(look.HeadLen * 0.5 * k))
		mid := body.Add(dir.Scale(look.HeadLen * 0.28 * k))
		gfx.setMat(mtPlain, 1, false)
		gfx.segment(gfx.cyl, neck.Sub(dir.Scale(0.04*k)), neck.Sub(dir.Scale(0.02*k)), r*0.9, matDark, 0.3, 0)
		gfx.setMat(mtPaint, 6, true)
		gfx.segment(gfx.cyl, neck.Sub(dir.Scale(0.02*k)), body, r, look.HeadCol, 0.6, 0)
		gfx.setMat(mtHazard, 3, true)
		gfx.segment(gfx.cyl, body.Sub(dir.Scale(0.04*k)), body.Sub(dir.Scale(0.01*k)), r*1.03, colWhite, 0.3, 0)
		gfx.setMat(mtMetal, 6, true)
		gfx.segment(gfx.cyl, body, mid, r*0.74, matSteel, 0.8, 0)
		gfx.segment(gfx.cyl, mid, tip, r*0.54, matSteel, 0.8, 0)
		gfx.setMat(mtPlain, 1, false)
		gfx.segment(gfx.cyl, mid.Sub(dir.Scale(0.008*k)), mid, r*0.8, matDark, 0.3, 0)
		gfx.segment(gfx.cyl, tip.Sub(dir.Scale(0.012*k)), tip.Add(dir.Scale(0.002*k)), r*0.58, colCyan, 0, look.Glow)
		light := colGreen
		if g.tankFull() {
			light = colRed
		}
		gfx.putBasis(gfx.cube, neck.Add(dir.Scale(look.HeadLen*0.22*k)).Add(top.Scale(r)), across.Scale(r*0.45), dir.Scale(look.HeadLen*0.22*k), top.Scale(0.012*k), light, 0, 0.9)
	}
	gfx.setMat(mtPlain, 1, false)
}

// depth is the third axis of a flat head lying across the handle.
func depth(across, dir vec3) vec3 { return across.Cross(dir) }

// drawGlows draws the soft sprites: dust particles and crystal glows.
// softness fades a particle as it reaches the ground, so its sprite
// doesn't get cut by the terrain.
func (g *Game) softness(p Particle) float64 {
	return mathx.Smoothstep(0, p.Size*3, g.terrain.Altitude(p.Pos))
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
		c.A = uint8(200 * mathx.Clamp01(p.Life/p.MaxLife*1.5) * g.softness(p))
		rl.DrawBillboard(cam, gfx.glow, toRL(p.Pos), float32(p.Size*3), c)
	}

	rl.BeginBlendMode(rl.BlendAdditive)
	for _, p := range g.particles {
		if !p.Additive {
			continue
		}
		c := p.C
		c.A = uint8(255 * mathx.Clamp01(p.Life/p.MaxLife*1.5) * g.softness(p))
		rl.DrawBillboard(cam, gfx.glow, toRL(p.Pos), float32(p.Size*4), c)
	}
	for _, gem := range g.nearGems {
		vis := g.gemVisibility(gem)
		if vis <= 0 {
			continue
		}
		pulse := 0.75 + 0.25*math.Sin(g.clock*3+float64(gem.Key))
		c := color.RGBA{80, 200, 230, uint8(160 * vis * pulse)}
		rl.DrawBillboard(cam, gfx.glow, toRL(g.gemPos(gem, 0.06+0.16*vis)), float32(1.4*vis), c)
	}
	if math.Mod(g.clock, 1.2) < 0.3 {
		rl.DrawBillboard(cam, gfx.glow, toRL(beaconTop()), 0.9, color.RGBA{255, 80, 60, 200})
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
