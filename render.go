package main

import (
	"image/color"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
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
	sunDir   = vec3{-0.5, 0.62, -0.6}.norm()
	earthDir = vec3{0.35, 0.38, 0.86}.norm()
)

const (
	maxCasters = 16
)

type shaderLocs struct {
	viewPos, sunDir, time       int32
	brush, casters, casterCount int32
	surface, earthDirLc         int32
}

// Graphics holds the GPU resources. Everything is generated at startup:
// shapes are meshes built in code and every surface is painted by shaders.
type Graphics struct {
	terrainSh, objectSh, skySh rl.Shader
	tLoc, oLoc, sLoc           shaderLocs

	sphere, hemi, cyl, cone, cube, crystal, sky rl.Model
	glow                                        rl.Texture2D
	font, bold                                  rl.Font
}

func loadShader(fs string) (rl.Shader, shaderLocs) {
	sh := rl.LoadShaderFromMemory(meshVS, fs)
	loc := func(name string) int32 { return rl.GetShaderLocation(sh, name) }
	return sh, shaderLocs{
		viewPos: loc("viewPos"), sunDir: loc("sunDir"), time: loc("time"),
		brush: loc("brush"), casters: loc("casters"), casterCount: loc("casterCount"),
		surface:    loc("surface"),
		earthDirLc: loc("earthDir"),
	}
}

func NewGraphics() *Graphics {
	gfx := &Graphics{}
	gfx.terrainSh, gfx.tLoc = loadShader(terrainFS)
	gfx.objectSh, gfx.oLoc = loadShader(objectFS)
	gfx.skySh, gfx.sLoc = loadShader(skyFS)

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
	rl.SetTextureFilter(gfx.font.Texture, rl.FilterBilinear)
	rl.SetTextureFilter(gfx.bold.Texture, rl.FilterBilinear)

	rl.SetShaderValue(gfx.skySh, gfx.sLoc.earthDirLc, v3f(earthDir), rl.ShaderUniformVec3)
	for _, s := range gfx.shaders() {
		rl.SetShaderValue(s.sh, s.loc.sunDir, v3f(sunDir), rl.ShaderUniformVec3)
	}
	return gfx
}

type shaderWithLocs struct {
	sh  rl.Shader
	loc shaderLocs
}

func (gfx *Graphics) shaders() []shaderWithLocs {
	return []shaderWithLocs{{gfx.terrainSh, gfx.tLoc}, {gfx.objectSh, gfx.oLoc}, {gfx.skySh, gfx.sLoc}}
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

func (g *Game) camera() rl.Camera3D {
	eye := g.player.Eye()
	return rl.Camera3D{
		Position:   eye.rl(),
		Target:     eye.add(g.player.Forward()).rl(),
		Up:         rl.NewVector3(0, 1, 0),
		Fovy:       70,
		Projection: rl.CameraPerspective,
	}
}

func (g *Game) setUniforms(cam rl.Camera3D) {
	gfx := g.gfx
	eye := []float32{cam.Position.X, cam.Position.Y, cam.Position.Z}
	t := []float32{float32(g.clock)}
	for _, s := range gfx.shaders() {
		rl.SetShaderValue(s.sh, s.loc.viewPos, eye, rl.ShaderUniformVec3)
		rl.SetShaderValue(s.sh, s.loc.time, t, rl.ShaderUniformFloat)
	}

	brush := []float32{0, 0, 0, 0}
	if g.aimOK && g.mode == modePlay {
		brush = []float32{float32(g.aim.X), float32(g.aim.Y), float32(g.aim.Z), float32(g.tool().Radius)}
	}
	rl.SetShaderValue(gfx.terrainSh, gfx.tLoc.brush, brush, rl.ShaderUniformVec4)

	// Soft shadows cast by the base and the drones.
	casters := []float32{
		float32(basePos.X), float32(basePos.Y + 2.6), float32(basePos.Z), 3.4,
		float32(terminalPos.X), float32(terminalPos.Y + 0.7), float32(terminalPos.Z), 0.8,
	}
	for _, d := range g.drones {
		casters = append(casters, float32(d.Pos.X), float32(d.Pos.Y), float32(d.Pos.Z), 0.55)
	}
	n := min(len(casters)/4, maxCasters)
	rl.SetShaderValueV(gfx.terrainSh, gfx.tLoc.casters, casters[:n*4], rl.ShaderUniformVec4, int32(n))
	rl.SetShaderValue(gfx.terrainSh, gfx.tLoc.casterCount, []float32{float32(n)}, rl.ShaderUniformFloat)
}

func (g *Game) Draw() {
	gfx := g.gfx
	cam := g.camera()

	mats := g.terrain.model.GetMaterials()
	mats[0].Shader = gfx.terrainSh
	rl.SetMaterialTexture(&mats[0], rl.MapDiffuse, g.world.Upload())
	g.setUniforms(cam)

	rl.BeginDrawing()
	rl.ClearBackground(rl.Black)
	rl.BeginMode3D(cam)

	// Sky: a big sphere around the camera, seen from inside, behind everything.
	rl.DisableBackfaceCulling()
	rl.DisableDepthMask()
	rl.DrawModel(gfx.sky, cam.Position, 400, rl.White)
	rl.EnableDepthMask()
	rl.EnableBackfaceCulling()

	rl.DrawModel(g.terrain.model, rl.NewVector3(terrainOrigin, 0, terrainOrigin), 1, rl.White)
	g.drawBase()
	g.drawDrones()
	g.drawGems()
	g.drawTool()
	g.drawGlows(cam)
	rl.EndMode3D()

	g.drawHUD(cam)
	rl.EndDrawing()
}

var terminalPos = basePos.add(vec3{1.8, 0, -3.4})

var (
	matGold  = color.RGBA{222, 170, 70, 255}
	matWhite = color.RGBA{232, 234, 240, 255}
	matSteel = color.RGBA{150, 155, 170, 255}
	matDark  = color.RGBA{48, 52, 66, 255}
)

func (g *Game) drawBase() {
	gfx, p := g.gfx, basePos
	up := vec3{0, 1, 0}

	for i := range 4 {
		a := math.Pi/4 + float64(i)*math.Pi/2
		ca, sa := math.Cos(a), math.Sin(a)
		foot := p.add(vec3{ca * 3.3, 0.12, sa * 3.3})
		gfx.segment(gfx.cyl, p.add(vec3{ca * 1.3, 2.0, sa * 1.3}), foot, 0.07, matSteel, 0.5, 0)
		gfx.put(gfx.hemi, p.add(vec3{ca * 3.3, 0, sa * 3.3}), vec3{0.5, 0.18, 0.5}, up, 0, matSteel, 0.4, 0)
	}
	gfx.put(gfx.cyl, p.add(vec3{0, 1.7, 0}), vec3{1.75, 1.35, 1.75}, up, 0, matGold, 0.9, 0)
	gfx.put(gfx.cyl, p.add(vec3{0, 3.0, 0}), vec3{1.82, 0.14, 1.82}, up, 0, matDark, 0.3, 0)
	gfx.put(gfx.sphere, p.add(vec3{0, 4.0, 0}), vec3{1.35, 1.15, 1.35}, up, 0, matWhite, 0.6, 0)
	gfx.put(gfx.sphere, p.add(vec3{0, 4.2, -1.18}), vec3{0.42, 0.3, 0.2}, up, 0, color.RGBA{70, 190, 230, 255}, 1, 0.55)
	gfx.segment(gfx.cyl, p.add(vec3{0.45, 4.9, 0.4}), p.add(vec3{0.45, 6.5, 0.4}), 0.03, matSteel, 0.5, 0)
	gfx.put(gfx.hemi, p.add(vec3{-0.75, 5.0, 0.2}), vec3{0.55, 0.2, 0.55}, vec3{0, 0, 1}, 30, matWhite, 0.6, 0)
	if math.Mod(g.clock, 1.2) < 0.3 {
		gfx.put(gfx.sphere, p.add(vec3{0.45, 6.55, 0.4}), vec3{0.09, 0.09, 0.09}, up, 0, colRed, 0, 1)
	}

	t := terminalPos
	gfx.put(gfx.cube, t.add(vec3{0, 0.65, 0}), vec3{0.8, 1.3, 0.5}, up, 0, matDark, 0.4, 0)
	gfx.put(gfx.cube, t.add(vec3{0, 1.36, -0.05}), vec3{0.95, 0.12, 0.7}, vec3{1, 0, 0}, -12, matSteel, 0.5, 0)
	gfx.put(gfx.cube, t.add(vec3{0, 0.95, -0.26}), vec3{0.62, 0.42, 0.02}, up, 0, color.RGBA{70, 210, 230, 255}, 0, 0.8)
}

func (g *Game) drawDrones() {
	gfx, up := g.gfx, vec3{0, 1, 0}
	for _, d := range g.drones {
		p := d.Pos.add(vec3{0, math.Sin(d.phase*3) * 0.08, 0})
		gfx.put(gfx.sphere, p, vec3{0.32, 0.15, 0.32}, up, 0, matWhite, 0.8, 0)
		gfx.put(gfx.hemi, p.add(vec3{0, -0.06, 0}), vec3{0.15, 0.12, 0.15}, vec3{1, 0, 0}, 180, matDark, 1, 0)
		for i := range 4 {
			a := math.Pi/4 + float64(i)*math.Pi/2
			arm := p.add(vec3{math.Cos(a) * 0.45, 0.05, math.Sin(a) * 0.45})
			gfx.segment(gfx.cyl, p, arm, 0.025, matSteel, 0.4, 0)
			gfx.put(gfx.cyl, arm.add(vec3{0, 0.04, 0}), vec3{0.19, 0.012, 0.19}, up, 0, color.RGBA{60, 64, 80, 150}, 0.2, 0)
		}
		light := colGreen
		if g.tankFull() {
			light = colRed
		}
		if math.Sin(d.phase*6) > 0 {
			gfx.put(gfx.sphere, p.add(vec3{0, 0.15, 0}), vec3{0.045, 0.045, 0.045}, up, 0, light, 0, 1)
		}
	}
}

// gemVisibility is how much of a crystal pokes out of the dust (0..1).
func (g *Game) gemVisibility(gem Gem) float64 {
	return clamp01(1 - g.world.DustAt(gem.X, gem.Z)*2.5)
}

func (g *Game) gemPos(i int, gem Gem, s float64) vec3 {
	return vec3{gem.X, g.terrain.Height(gem.X, gem.Z) + s*1.4 + math.Sin(g.clock*2+float64(i))*0.05, gem.Z}
}

func (g *Game) drawGems() {
	gfx := g.gfx
	for i, gem := range g.world.Gems {
		vis := g.gemVisibility(gem)
		if vis <= 0 {
			continue
		}
		s := 0.06 + 0.16*vis
		p := g.gemPos(i, gem, s)
		spin := g.clock*50 + float64(i)*40
		gfx.put(gfx.crystal, p, vec3{s, s * 1.7, s}, vec3{0, 1, 0}, spin, colCyan, 1, 0.45)
		gfx.put(gfx.crystal, p, vec3{s, s * 1.2, s}, vec3{1, 0, 0}, 180, color.RGBA{60, 170, 220, 255}, 1, 0.45)
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

	gfx.segment(gfx.cyl, base, neck, hr, look.HandleCol, 0.5, 0)
	gfx.segment(gfx.cyl, neck.sub(dir.scale(0.02*k)), neck.add(dir.scale(0.02*k)), math.Max(hr, r)*1.15, matSteel, 0.8, 0)
	switch look.Head {
	case headBlock:
		gfx.segment(gfx.cyl, neck, tip, r, look.HeadCol, 0.2, 0)
	case headTaper:
		gfx.segment(gfx.cone, neck, tip, r, look.HeadCol, 0.2, 0)
	case headFlare:
		gfx.segment(gfx.cone, tip, neck, r, look.HeadCol, 0.2, 0)
	case headNozzle:
		gfx.put(gfx.sphere, neck, vec3{r * 1.3, r * 1.3, r * 1.3}, vec3{0, 1, 0}, 0, look.HeadCol, 0.7, 0)
		gfx.segment(gfx.cyl, neck, tip, r*0.7, look.HeadCol, 0.7, 0)
		gfx.segment(gfx.cyl, tip.sub(dir.scale(0.015*k)), tip, r*0.85, colCyan, 0, look.Glow)
	}
}

// drawGlows draws the soft sprites: dust particles and crystal glows.
func (g *Game) drawGlows(cam rl.Camera3D) {
	gfx := g.gfx
	rl.DrawRenderBatchActive()
	rl.DisableDepthMask()

	for _, p := range g.particles {
		if p.Additive {
			continue
		}
		c := p.C
		c.A = uint8(200 * clamp01(p.Life/p.MaxLife*1.5))
		rl.DrawBillboard(cam, gfx.glow, p.Pos.rl(), float32(p.Size*3), c)
	}

	rl.BeginBlendMode(rl.BlendAdditive)
	for _, p := range g.particles {
		if !p.Additive {
			continue
		}
		c := p.C
		c.A = uint8(255 * clamp01(p.Life/p.MaxLife*1.5))
		rl.DrawBillboard(cam, gfx.glow, p.Pos.rl(), float32(p.Size*4), c)
	}
	for i, gem := range g.world.Gems {
		vis := g.gemVisibility(gem)
		if vis <= 0 {
			continue
		}
		pulse := 0.75 + 0.25*math.Sin(g.clock*3+float64(i))
		c := color.RGBA{80, 200, 230, uint8(160 * vis * pulse)}
		rl.DrawBillboard(cam, gfx.glow, g.gemPos(i, gem, 0.06+0.16*vis).rl(), float32(1.4*vis), c)
	}
	if math.Mod(g.clock, 1.2) < 0.3 {
		rl.DrawBillboard(cam, gfx.glow, basePos.add(vec3{0.45, 6.55, 0.4}).rl(), 0.9, color.RGBA{255, 80, 60, 200})
	}
	rl.EndBlendMode()
	rl.EnableDepthMask()
}
