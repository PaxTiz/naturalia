package render

import (
	"fmt"
	"image/color"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/game"
	"naturalia/internal/mathx"
	"naturalia/internal/mesh"
	"naturalia/internal/palette"
	"naturalia/internal/planet"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"golang.org/x/image/font/gofont/gosmallcaps"
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
	terrainMat                                        *rl.Material // own allocation: see New
	signBox                                           rl.Model
	rocks, pebbles                                    []rl.Model
	post                                              *postFX
	shadow                                            *shadowMap
	lampShadow                                        *lampShadow
	dust                                              dustGPU // the dust tiles near the player, on the GPU
	base                                              *baseModels
	doorPanel                                         rl.Model
	entities                                          map[string][]entityModel // by entity ID, built on first draw
	screens                                           []rl.Texture2D
	signs                                             []sign
	smallcaps                                         rl.Font
	tex                                               Textures
	objectMaps                                        []rl.Texture2D // the painted textures, in the object shader's order
	glow                                              rl.Texture2D

	content     *content.Content            // materials, entities
	Terrain     *Terrain                    // the ground as drawn, in levels of detail
	nearGems    []planet.Gem                // crystals around the player, refreshed every frame
	nearPebbles map[planet.TileKey][]pebble // per tile, around the player
	stats       Stats
	camEye      vec3    // the camera's position in double precision (camera)
	near, far   float64 // the view's clip planes, set each frame by clipPlanes
	hazeThin    float32 // and the haze's thinning with altitude (common.glsl)
	skyAir      float32 // the sky fading to space with altitude (sky.frag)
	origin      vec3    // the shaders' origin this frame (setUniforms)
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

// New loads everything the game is drawn with, and the ground
// around the player.
func New(g *game.Game) *Graphics {
	gfx := &Graphics{content: g.Content, entities: map[string][]entityModel{}}
	gfx.Terrain = NewTerrain(g.World.Relief)
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

	gfx.smallcaps = LoadFont(gosmallcaps.TTF, 96) // for the signs

	for _, s := range gfx.shaders() {
		rl.SetShaderValue(s.sh, s.loc.sunDir, v3f(planet.SunDir), rl.ShaderUniformVec3)
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
	gfx.lampShadow = newLampShadow(len(base.Rooms))
	gfx.terrainSh.UpdateLocation(rl.ShaderLocMapBrdf, rl.GetShaderLocation(gfx.terrainSh, "shadowMap"))
	rl.SetMaterialTexture(gfx.terrainMat, mapShadow, gfx.shadow.target.Texture)

	gfx.signBox = model(rl.GenMeshCube(1, 1, 1), gfx.objectSh)
	for i, c := range []color.RGBA{palette.AccentCyan, palette.AccentBlue, palette.AccentCyan, palette.AccentOrange} {
		gfx.screens = append(gfx.screens, paintScreen(i, c))
	}
	for _, s := range gfx.baseSigns() {
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

	gfx.near, gfx.far = 0.05, 9000
	rl.SetClipPlanes(gfx.near, gfx.far)

	// The base's ceiling lamps never move.
	var lights []float32
	for _, l := range base.Lights() {
		p := base.ToWorld(l)
		lights = append(lights, float32(p.X), float32(p.Y), float32(p.Z), 1)
	}
	lights[3] = 1.4 // the hub is bigger
	rl.SetShaderValue(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lampCount"), []float32{float32(len(base.Rooms))}, rl.ShaderUniformFloat)
	rl.SetShaderValue(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lampGrid"), []float32{float32(gfx.lampShadow.grid)}, rl.ShaderUniformFloat)
	rl.SetShaderValueV(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lights"), lights, rl.ShaderUniformVec4, int32(len(lights)/4))
	rl.SetShaderValue(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lightCount"), []float32{float32(len(lights) / 4)}, rl.ShaderUniformFloat)
	rl.SetShaderValue(gfx.holoSh, rl.GetShaderLocation(gfx.holoSh, "tilesPerFace"), []float32{planet.TilesPerFace}, rl.ShaderUniformFloat)
	gfx.Terrain.Warm(g.Player.Eye(), 5*time.Second)
	return gfx
}

// ToRL is a vector as raylib takes it, in float32.
func ToRL(a vec3) rl.Vector3 { return rl.NewVector3(float32(a.X), float32(a.Y), float32(a.Z)) }

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
	rl.DrawModelEx(m, ToRL(pos), ToRL(axis), float32(angle), ToRL(scale), c)
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
func (gfx *Graphics) camera(g *game.Game) rl.Camera3D {
	p := &g.Player
	eye := p.Eye()
	walk := p.MoveAmount
	right, up := p.Right(), p.LocalUp()
	eye = eye.Add(up.Scale(math.Sin(2*p.WalkPhase) * 0.02 * walk)).Add(right.Scale(math.Sin(p.WalkPhase) * 0.012 * walk))
	camUp := up.Add(right.Scale(math.Sin(p.WalkPhase) * 0.008 * walk))
	if g.Sweeping {
		eye = eye.Add(right.Scale(math.Sin(g.Elapsed*37) * 0.002)).Add(up.Scale(math.Sin(g.Elapsed*23+1) * 0.002))
	}
	gfx.camEye = eye
	return rl.Camera3D{
		Position:   ToRL(eye),
		Target:     ToRL(eye.Add(p.Forward())),
		Up:         ToRL(camUp),
		Fovy:       float32(g.Fov),
		Projection: rl.CameraPerspective,
	}
}

func (gfx *Graphics) setUniforms(g *game.Game, cam rl.Camera3D) {
	eye := []float32{cam.Position.X, cam.Position.Y, cam.Position.Z}
	t := []float32{float32(g.Elapsed)}
	for _, s := range gfx.shaders() {
		rl.SetShaderValue(s.sh, s.loc.viewPos, eye, rl.ShaderUniformVec3)
		rl.SetShaderValue(s.sh, s.loc.time, t, rl.ShaderUniformFloat)
	}
	// The shaders' origin (common.glsl): the eye snapped to a 32 m grid,
	// so the tile offsets they derive from it stay put between snaps.
	e := gfx.camEye
	snap := func(v float64) float64 { return float64(float32(math.Floor(v/32) * 32)) }
	o := vec3{snap(e.X), snap(e.Y), snap(e.Z)}
	gfx.origin = o
	for _, s := range gfx.shaders() {
		rl.SetShaderValue(s.sh, rl.GetShaderLocation(s.sh, "origin"), v3f(o), rl.ShaderUniformVec3)
		rl.SetShaderValue(s.sh, rl.GetShaderLocation(s.sh, "viewRel"), v3f(e.Sub(o)), rl.ShaderUniformVec3)
		rl.SetShaderValue(s.sh, rl.GetShaderLocation(s.sh, "hazeThin"), []float32{gfx.hazeThin}, rl.ShaderUniformFloat)
	}
	// The base's shapes, for its ambient occlusion (baseAO).
	var rms, crs, csz []float32
	for _, r := range base.Rooms {
		rms = append(rms, float32(r.Center.X), float32(r.Center.Z), float32(r.Radius), float32(r.WallH))
	}
	for _, c := range base.Corridors {
		crs = append(crs, float32(c.From.X), float32(c.From.Z), float32(c.To.X), float32(c.To.Z))
		csz = append(csz, float32(c.Width), float32(c.Height), 0, 0)
	}
	var blobs []float32
	for _, ob := range base.Obstacles[:min(len(base.Obstacles), 48)] {
		blobs = append(blobs, float32(ob.At.X), float32(ob.At.Z), float32(ob.Radius), 0)
	}
	for _, sh := range []rl.Shader{gfx.terrainSh, gfx.objectSh} {
		rl.SetShaderValueV(sh, rl.GetShaderLocation(sh, "baseBlobs"), blobs, rl.ShaderUniformVec4, int32(len(blobs)/4))
		rl.SetShaderValue(sh, rl.GetShaderLocation(sh, "blobCount"), []float32{float32(len(blobs) / 4)}, rl.ShaderUniformFloat)
		rl.SetShaderValue(sh, rl.GetShaderLocation(sh, "baseRel"), v3f(base.Origin.Sub(o)), rl.ShaderUniformVec3)
		rl.SetShaderValueV(sh, rl.GetShaderLocation(sh, "baseRooms"), rms, rl.ShaderUniformVec4, int32(len(rms)/4))
		rl.SetShaderValueV(sh, rl.GetShaderLocation(sh, "baseCorrs"), crs, rl.ShaderUniformVec4, int32(len(crs)/4))
		rl.SetShaderValueV(sh, rl.GetShaderLocation(sh, "baseCorrSize"), csz, rl.ShaderUniformVec4, int32(len(csz)/4))
		rl.SetShaderValue(sh, rl.GetShaderLocation(sh, "roomCount"), []float32{float32(len(rms) / 4)}, rl.ShaderUniformFloat)
		rl.SetShaderValue(sh, rl.GetShaderLocation(sh, "corrCount"), []float32{float32(len(crs) / 4)}, rl.ShaderUniformFloat)
	}
	rl.SetShaderValue(gfx.skySh, gfx.sLoc.up, v3f(g.Player.LocalUp()), rl.ShaderUniformVec3)
	rl.SetShaderValue(gfx.skySh, rl.GetShaderLocation(gfx.skySh, "air"), []float32{gfx.skyAir}, rl.ShaderUniformFloat)
	// Objects take the relief's shadow from where the player stands: those
	// that matter are close, far ones are too small to tell.
	sun, ao := gfx.Terrain.light(gfx.Terrain.Surface(g.Player.Pos))
	gfx.stats.Sun, gfx.stats.AO = sun, ao
	rl.SetShaderValue(gfx.objectSh, gfx.oLoc.ground, []float32{float32(sun), float32(ao)}, rl.ShaderUniformVec2)

	brush := []float32{0, 0, 0, 0}
	if g.AimOK && g.Mode == game.ModePlay {
		brush = []float32{float32(g.Aim.X), float32(g.Aim.Y), float32(g.Aim.Z), float32(g.Tool().Radius)}
	}
	rl.SetShaderValue(gfx.terrainSh, gfx.tLoc.brush, brush, rl.ShaderUniformVec4)
	gfx.uploadSteps(&g.Steps)

	// Soft round shadows of the base's domes, for when the base is beyond
	// the shadow map.
	var casters []float32
	for _, r := range base.Rooms {
		c := base.ToWorld(r.Center.Add(vec3{0, r.WallH * 0.7, 0}))
		casters = append(casters, float32(c.X), float32(c.Y), float32(c.Z), float32(r.Radius*1.05))
	}
	n := min(len(casters)/4, maxCasters)
	rl.SetShaderValueV(gfx.terrainSh, gfx.tLoc.casters, casters[:n*4], rl.ShaderUniformVec4, int32(n))
	rl.SetShaderValue(gfx.terrainSh, gfx.tLoc.casterCount, []float32{float32(n)}, rl.ShaderUniformFloat)
	cols := gfx.shadow.cols(o)
	rl.SetShaderValueV(gfx.terrainSh, gfx.tLoc.lightVP, cols, rl.ShaderUniformVec4, 4)
	rl.SetShaderValueV(gfx.objectSh, gfx.oLoc.lightVP, cols, rl.ShaderUniformVec4, 4)
}

// Draw draws a frame of the game; overlay draws over it, on the screen
// (the interface).
func (gfx *Graphics) Draw(g *game.Game, overlay func(cam rl.Camera3D)) {
	cam := gfx.camera(g)
	gfx.clipPlanes(g)

	atlas, lookup := gfx.dust.Upload(g.World, g.Player.Pos)
	rl.SetMaterialTexture(gfx.terrainMat, rl.MapDiffuse, atlas)
	rl.SetMaterialTexture(gfx.terrainMat, rl.MapSpecular, lookup)
	patches := gfx.Terrain.Visible(g.Player.Eye())
	gfx.stats.Patches = len(patches)
	gfx.nearGems = g.World.GemsNear(g.Player.Pos, gemDrawDist)
	// Objects cast their shadows first, then the scene is lit with them.
	gfx.shadow.render(gfx, g.Player.Pos, func() {
		gfx.drawRocks(g)
		gfx.drawBase(g)
		gfx.drawDrones(g)
		gfx.drawEntities(g)
	})
	// The rooms' lamps cast shadows too, drawn once.
	gfx.setUniforms(g, cam)
	if gfx.base != nil {
		var lamps []vec3
		for _, l := range base.Lights()[:len(base.Rooms)] {
			lamps = append(lamps, base.ToWorld(l).Sub(vec3{0, 0.12, 0}))
		}
		gfx.lampShadow.render(gfx, lamps, func() { gfx.drawBase(g) })
		vp := gfx.lampShadow.cols(gfx.origin)
		rl.SetShaderValueV(gfx.objectSh, rl.GetShaderLocation(gfx.objectSh, "lampVP"), vp, rl.ShaderUniformVec4, int32(len(vp)/4))
	}

	sunUp := g.Player.Pos.Norm().Dot(planet.SunDir)
	gfx.post.adapt(float64(rl.GetFrameTime()), sunUp, base.Indoors(base.ToLocal(g.Player.Eye())))
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
	gfx.drawBase(g)
	gfx.drawDrones(g)
	gfx.drawEntities(g)
	// Culling is off: patch winding differs from one cube face to the next.
	rl.DisableBackfaceCulling()
	for _, n := range patches {
		rl.DrawMesh(n.mesh, *gfx.terrainMat, rl.MatrixIdentity())
	}
	rl.EnableBackfaceCulling()
	gfx.drawRocks(g)
	gfx.drawPebbles(g)
	gfx.drawGems(g)
	gfx.drawDevils(g)
	if base.Indoors(base.ToLocal(g.Player.Eye())) {
		gfx.setLighting(mesh.LitIndoor, vec3{})
	}
	gfx.drawTool(g)
	gfx.setLighting(mesh.LitSun, vec3{})
	gfx.drawGlows(g, cam)
	gfx.drawGlobe(g, cam)
	rl.EndMode3D()
	rl.EndTextureMode()

	rl.BeginDrawing()
	gfx.post.finish(cam, sunUp, g.Elapsed)
	overlay(cam)
	rl.EndDrawing()
}

func (gfx *Graphics) drawDrones(g *game.Game) {
	for _, d := range g.Drones {
		up := d.Pos.Norm()
		axis, angle := mathx.Orient(up)
		downAxis, downAngle := mathx.Orient(up.Scale(-1))
		t1, t2 := mathx.TangentBasis(up)
		p := d.Pos.Add(up.Scale(math.Sin(d.Phase*3) * 0.08))
		gfx.setMat(content.MtPaint, 2, true)
		gfx.put(gfx.sphere, p, vec3{0.32, 0.15, 0.32}, axis, angle, palette.MatWhite, 0.8, 0)
		gfx.put(gfx.sphere, p.Add(up.Scale(0.05)), vec3{0.2, 0.09, 0.2}, axis, angle, palette.AccentOrange, 0.6, 0)
		gfx.setMat(content.MtMetal, 3, true)
		gfx.put(gfx.hemi, p.Sub(up.Scale(0.06)), vec3{0.15, 0.12, 0.15}, downAxis, downAngle, palette.MatDark, 1, 0)
		for i := range 4 {
			a := math.Pi/4 + float64(i)*math.Pi/2
			arm := p.Add(t1.Scale(math.Cos(a) * 0.45)).Add(t2.Scale(math.Sin(a) * 0.45)).Add(up.Scale(0.05))
			gfx.segment(gfx.cyl, p, arm, 0.025, palette.MatSteel, 0.4, 0)
			gfx.setMat(content.MtPlain, 1, false)
			gfx.segment(gfx.cyl, arm.Add(up.Scale(0.04)), arm.Add(up.Scale(0.052)), 0.19, color.RGBA{60, 64, 80, 150}, 0.2, 0)
			gfx.setMat(content.MtMetal, 3, true)
		}
		gfx.setMat(content.MtPlain, 1, false)
		light := palette.Green
		if g.TankFull() {
			light = palette.Red
		}
		if math.Sin(d.Phase*6) > 0 {
			gfx.put(gfx.sphere, p.Add(up.Scale(0.15)), vec3{0.045, 0.045, 0.045}, axis, angle, light, 0, 1)
		}
	}
}

// gemDrawDist: crystals are only drawn this close; they are tiny anyway.
const gemDrawDist = 80.0

// gemVisibility is how much of a crystal pokes out of the dust (0..1).
func (gfx *Graphics) gemVisibility(g *game.Game, gem planet.Gem) float64 {
	return mathx.Clamp01(1 - g.World.DustAt(gem.Dir)*2.5)
}

func (gfx *Graphics) gemPos(g *game.Game, gem planet.Gem, s float64) vec3 {
	lift := s*1.4 + math.Sin(g.Elapsed*2+float64(gem.Key))*0.05
	return gfx.Terrain.Surface(gem.Dir).Add(gem.Dir.Scale(lift))
}

func (gfx *Graphics) drawGems(g *game.Game) {
	for _, gem := range gfx.nearGems {
		vis := gfx.gemVisibility(g, gem)
		if vis <= 0 {
			continue
		}
		s := 0.06 + 0.16*vis
		p := gfx.gemPos(g, gem, s)
		axis, angle := mathx.Orient(gem.Dir)
		downAxis, downAngle := mathx.Orient(gem.Dir.Scale(-1))
		gfx.put(gfx.crystal, p, vec3{s, s * 1.7, s}, axis, angle, palette.Cyan, 1, 0.45)
		gfx.put(gfx.crystal, p, vec3{s, s * 1.2, s}, downAxis, downAngle, color.RGBA{60, 170, 220, 255}, 1, 0.45)
	}
}

// clipPlanes sets the view's depth range for the eye's altitude. Mars is
// big: see mountains far away, but keep the held tool, very close to the
// eye, from being clipped. High up in creative flight, see the whole
// visible half of Mars.
func (gfx *Graphics) clipPlanes(g *game.Game) {
	alt := g.World.Relief.Altitude(gfx.camEye)
	gfx.near = math.Max(0.05, math.Min(20, alt*0.002))
	gfx.hazeThin = float32(1 / (1 + max(0, alt-20)/150))
	gfx.skyAir = float32(1 - mathx.Smoothstep(300, 5000, alt))
	gfx.far = math.Max(9000, gfx.camEye.Length()+planet.Radius)
	rl.SetClipPlanes(gfx.near, gfx.far)
}

// drawTool draws the held tool, in the lower right corner of the view.
func (gfx *Graphics) drawTool(g *game.Game) {
	if g.Mode == game.ModePaused || g.Player.Fly {
		return
	}
	look := g.Tool().Look
	base, dir := g.ToolBase, g.ToolDir
	if g.Sweeping {
		base = base.Add(vec3{(g.Rng.Float64() - 0.5) * 0.006, (g.Rng.Float64() - 0.5) * 0.006, 0})
	}
	const k = game.ToolViewScale
	neck := base.Add(dir.Scale(look.HandleLen * k))
	tip := neck.Add(dir.Scale(look.HeadLen * k))
	hr, r := look.HandleR*k, look.HeadR*k

	gfx.setMat(content.MtPaint, 8, true)
	gfx.segment(gfx.cyl, base, neck, hr, look.HandleCol, 0.5, 0)
	// A dark rubber grip where the hand holds it, with a cyan ring.
	grip := base.Add(dir.Scale(look.HandleLen * 0.35 * k))
	gfx.setMat(content.MtPlain, 1, false)
	gfx.segment(gfx.cyl, base.Sub(dir.Scale(0.01*k)), grip, hr*1.3, palette.MatDark, 0.2, 0)
	gfx.segment(gfx.cyl, grip, grip.Add(dir.Scale(0.012*k)), hr*1.32, palette.AccentCyan, 0.3, 0)
	gfx.setMat(content.MtMetal, 6, true)
	collar := math.Max(hr, r) * 1.15
	if look.Head == content.HeadFlare || look.Head == content.HeadBlock {
		collar = hr * 1.5 // flat heads: a slim collar
	}
	if look.Head != content.HeadNozzle {
		gfx.segment(gfx.cyl, neck.Sub(dir.Scale(0.02*k)), neck.Add(dir.Scale(0.02*k)), collar, palette.MatSteel, 0.8, 0)
	}
	right := g.Player.Right()
	across := right.Sub(dir.Scale(right.Dot(dir))).Norm()
	top := dir.Cross(across)
	if top.Dot(g.Player.LocalUp()) < 0 {
		top = top.Scale(-1)
	}
	switch look.Head {
	case content.HeadBlock:
		// A toothbrush: a flat plastic head, a white tuft of bristles
		// standing under it.
		at := neck.Add(dir.Scale(look.HeadLen * 0.5 * k))
		gfx.setMat(content.MtPaint, 8, true)
		gfx.putBasis(gfx.cube, at, across.Scale(r*2.2), dir.Scale(look.HeadLen*k), top.Scale(r*0.9), look.HandleCol, 0.5, 0)
		gfx.setMat(content.MtFabric, 12, true)
		gfx.putBasis(gfx.cube, at.Sub(top.Scale(r*0.9)), across.Scale(r*1.9), dir.Scale(look.HeadLen*0.85*k), top.Scale(r*1.2), look.HeadCol, 0.1, 0)
		gfx.setMat(content.MtPlain, 1, false)
		gfx.putBasis(gfx.cube, at.Add(top.Scale(r*0.46)), across.Scale(r*0.5), dir.Scale(look.HeadLen*0.6*k), top.Scale(0.004*k), palette.AccentCyan, 0, 0.3)
	case content.HeadTaper:
		// A paintbrush: a steel ferrule, then a tuft of bristles narrowing
		// to a tip dipped in paint.
		ferrule := neck.Add(dir.Scale(look.HeadLen * 0.3 * k))
		body := ferrule.Add(dir.Scale(look.HeadLen * 0.35 * k))
		gfx.segment(gfx.cyl, neck, ferrule, r*0.85, palette.MatSteel, 0.9, 0)
		gfx.setMat(content.MtFabric, 12, true)
		gfx.segment(gfx.cyl, ferrule, body, r, look.HeadCol, 0.2, 0)
		gfx.segment(gfx.cone, body, tip, r, look.HeadCol, 0.2, 0)
		gfx.setMat(content.MtPlain, 1, false)
		gfx.segment(gfx.cone, body.Lerp(tip, 0.55), tip.Add(dir.Scale(0.002*k)), r*0.45, palette.AccentOrange, 0.4, 0)
	case content.HeadFlare:
		// A push broom: a painted head across the handle with steel end
		// brackets and a hazard band, a thick row of bristles below.
		width := look.HeadR * 3.2 * k
		at := neck.Add(dir.Scale(0.03 * k))
		gfx.setMat(content.MtPaint, 8, true)
		gfx.putBasis(gfx.cube, at, across.Scale(width), dir.Scale(0.06*k), depth(across, dir).Scale(0.09*k), look.HandleCol, 0.3, 0)
		gfx.setMat(content.MtHazard, 12, false)
		gfx.putBasis(gfx.cube, at.Add(top.Scale(0.046*k)), across.Scale(width*0.6), dir.Scale(0.05*k), depth(across, dir).Scale(0.004*k), palette.White, 0.3, 0)
		gfx.setMat(content.MtMetal, 6, true)
		for _, sd := range []float64{-1, 1} {
			gfx.putBasis(gfx.cube, at.Add(across.Scale(sd*width*0.5)), across.Scale(0.02*k), dir.Scale(0.08*k), depth(across, dir).Scale(0.1*k), palette.MatSteel, 0.8, 0)
		}
		gfx.setMat(content.MtFabric, 10, true)
		gfx.putBasis(gfx.cube, neck.Add(dir.Scale(0.12*k)), across.Scale(width*0.96), dir.Scale(0.13*k), depth(across, dir).Scale(0.07*k), look.HeadCol, 0.1, 0)
	case content.HeadNozzle:
		// A blower in the kit's language: a painted body with a dark rear
		// cap and a hazard band, a steel nozzle in two steps, a glowing ring
		// at the mouth and a status light on top.
		body := neck.Add(dir.Scale(look.HeadLen * 0.5 * k))
		mid := body.Add(dir.Scale(look.HeadLen * 0.28 * k))
		gfx.setMat(content.MtPlain, 1, false)
		gfx.segment(gfx.cyl, neck.Sub(dir.Scale(0.04*k)), neck.Sub(dir.Scale(0.02*k)), r*0.9, palette.MatDark, 0.3, 0)
		gfx.setMat(content.MtPaint, 6, true)
		gfx.segment(gfx.cyl, neck.Sub(dir.Scale(0.02*k)), body, r, look.HeadCol, 0.6, 0)
		gfx.setMat(content.MtHazard, 3, true)
		gfx.segment(gfx.cyl, body.Sub(dir.Scale(0.04*k)), body.Sub(dir.Scale(0.01*k)), r*1.03, palette.White, 0.3, 0)
		gfx.setMat(content.MtMetal, 6, true)
		gfx.segment(gfx.cyl, body, mid, r*0.74, palette.MatSteel, 0.8, 0)
		gfx.segment(gfx.cyl, mid, tip, r*0.54, palette.MatSteel, 0.8, 0)
		gfx.setMat(content.MtPlain, 1, false)
		gfx.segment(gfx.cyl, mid.Sub(dir.Scale(0.008*k)), mid, r*0.8, palette.MatDark, 0.3, 0)
		gfx.segment(gfx.cyl, tip.Sub(dir.Scale(0.012*k)), tip.Add(dir.Scale(0.002*k)), r*0.58, palette.Cyan, 0, look.Glow)
		light := palette.Green
		if g.TankFull() {
			light = palette.Red
		}
		gfx.putBasis(gfx.cube, neck.Add(dir.Scale(look.HeadLen*0.22*k)).Add(top.Scale(r)), across.Scale(r*0.45), dir.Scale(look.HeadLen*0.22*k), top.Scale(0.012*k), light, 0, 0.9)
	}
	gfx.setMat(content.MtPlain, 1, false)
}

// depth is the third axis of a flat head lying across the handle.
func depth(across, dir vec3) vec3 { return across.Cross(dir) }

// drawGlows draws the soft sprites: dust particles and crystal glows.
// softness fades a particle as it reaches the ground, so its sprite
// doesn't get cut by the terrain.
func (gfx *Graphics) softness(g *game.Game, p game.Particle) float64 {
	return mathx.Smoothstep(0, p.Size*3, gfx.Terrain.Altitude(p.Pos))
}

func (gfx *Graphics) drawGlows(g *game.Game, cam rl.Camera3D) {
	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()

	for _, p := range g.Particles {
		if p.Additive {
			continue
		}
		c := p.C
		c.A = uint8(200 * mathx.Clamp01(p.Life/p.MaxLife*1.5) * gfx.softness(g, p))
		rl.DrawBillboard(cam, gfx.glow, ToRL(p.Pos), float32(p.Size*3), c)
	}

	rl.BeginBlendMode(rl.BlendAdditive)
	for _, p := range g.Particles {
		if !p.Additive {
			continue
		}
		c := p.C
		c.A = uint8(255 * mathx.Clamp01(p.Life/p.MaxLife*1.5) * gfx.softness(g, p))
		rl.DrawBillboard(cam, gfx.glow, ToRL(p.Pos), float32(p.Size*4), c)
	}
	for _, gem := range gfx.nearGems {
		vis := gfx.gemVisibility(g, gem)
		if vis <= 0 {
			continue
		}
		pulse := 0.75 + 0.25*math.Sin(g.Elapsed*3+float64(gem.Key))
		c := color.RGBA{80, 200, 230, uint8(160 * vis * pulse)}
		rl.DrawBillboard(cam, gfx.glow, ToRL(gfx.gemPos(g, gem, 0.06+0.16*vis)), float32(1.4*vis), c)
	}
	if math.Mod(g.Elapsed, 1.2) < 0.3 {
		rl.DrawBillboard(cam, gfx.glow, ToRL(beaconTop()), 0.9, color.RGBA{255, 80, 60, 200})
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

// LoadFont loads a TrueType font at size, with Latin-1 for the French
// accents (raylib's default font is ASCII only) and a few symbols.
func LoadFont(ttf []byte, size int32) rl.Font {
	var runes []rune
	for r := rune(32); r < 256; r++ {
		runes = append(runes, r)
	}
	runes = append(runes, '…', '’', '→', '►', '◄')
	f := rl.LoadFontFromMemory(".ttf", ttf, size, runes)
	rl.SetTextureFilter(f.Texture, rl.FilterBilinear)
	return f
}

// Stats are what the debug overlay shows of the rendering.
type Stats struct {
	Patches, Pebbles int     // drawn this frame
	Loaded, Pending  int     // terrain meshes on the GPU, being built
	Sun, AO          float64 // relief light under the player
}

func (gfx *Graphics) Stats() Stats {
	s := gfx.stats
	s.Loaded, s.Pending = len(gfx.Terrain.loaded), gfx.Terrain.pending
	return s
}
