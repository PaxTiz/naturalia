package render

import (
	"cmp"
	"fmt"
	"image/color"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/engine/ecs"
	"naturalia/internal/engine/gpu"
	"naturalia/internal/engine/mathx"
	"naturalia/internal/engine/mesh"
	"naturalia/internal/game"
	"naturalia/internal/palette"
	"naturalia/internal/planet"
	"slices"
	"strings"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"golang.org/x/image/font/gofont/gosmallcaps"
)

const (
	maxCasters = 16
	maxLamps   = 24 // lights in object.frag
	maxRooms   = 8  // rooms and corridors in the shaders (baseAO), lamps casting shadows
)

// Graphics is the game's scene, drawn by the engine (gpu): the terrain,
// the base and the buildings, the lamps, the objects. Everything is
// generated at startup: shapes are meshes built in code and every surface
// is painted by shaders.
type Graphics struct {
	eng                                                  *gpu.Renderer
	terrainSh, holedSh, objectSh, skySh, holoSh, devilSh *gpu.Program // holed: the terrain's, open over the tunnels (tunnels.go)
	devilMesh                                            rl.Mesh
	devilMat                                             rl.Material

	sphere, hemi, cyl, cone, cube, crystal, sky, holo rl.Model
	terrainMat                                        *rl.Material // own allocation: see New
	holedMat                                          *rl.Material
	holes                                             []vec3 // where it is open
	signBox                                           rl.Model
	rocks, pebbles                                    []rl.Model
	shadow                                            *gpu.SunShadow
	lampShadow                                        *lampShadow
	spots                                             *spots                 // the player's lamps (spots.go)
	still                                             bool                   // drawing only what stays put
	shade                                             shading                // the buildings in the shaders (selectBase)
	site                                              base.Site              // the building whose pieces are drawn, none: the base (frame)
	links                                             map[string]*baseModels // the links', by shape (camps.go)
	campSigns                                         map[ecs.ID]campSign
	dust                                              dustGPU // the dust tiles near the player, on the GPU
	base                                              *baseModels
	growth                                            map[grown]*baseModels  // what grows in the base, by stage
	camps                                             map[string]*baseModels // each kind of bootcamp's
	doorPanel                                         rl.Model
	hallLeaf                                          rl.Model                 // a door leaf of the halls, scaled to size
	reach                                             map[string]float64       // of entities, by kind (entityReach)
	entities                                          map[string][]entityModel // by entity ID and finish, built on first draw
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
	pebbleFlat  *planet.Flat                // the flattened ground they were placed around
	stats       Stats
	camEye      vec3 // the camera's position in double precision (camera)
	camLook     vec3 // where it looks, and its up, in double precision too
	camUp       vec3
	camFov      float64
	gallery     *base.Link // the tunnel whose gallery the camera is in (tunnels.go)
	under       float64    // how far under the ground it is, 0 to 1
	casting     bool       // drawing a shadow map
	hazeThin    float32    // and the haze's thinning with altitude (common.glsl)
	skyAir      float32    // the sky fading to space with altitude (sky.frag)
}

const (
	shadowRes    = 2048
	shadowRadius = 80.0  // meters covered around the player
	shadowDepth  = 600.0 // the light camera's depth range
)

// program loads a program the engine sets up each frame; the object
// shader writes into the depth maps.
func (gfx *Graphics) program(vs, fs string, caster bool) *gpu.Program {
	return gfx.eng.Add(gpu.NewProgram(glsl(vs), fs), caster)
}

// New loads everything the game is drawn with, and the ground
// around the player.
func New(g *game.Game) *Graphics {
	gfx := &Graphics{content: g.Content, entities: map[string][]entityModel{}, eng: gpu.NewRenderer()}
	gfx.Terrain = NewTerrain(g.World.Relief)
	gfx.terrainSh = gfx.program("terrain.vert", glsl("terrain.frag"), false)
	gfx.holedSh = gfx.program("terrain.vert", strings.Replace(glsl("terrain.frag"), "#version 330\n", "#version 330\n#define HOLES\n", 1), false)
	gfx.objectSh = gfx.program("object.vert", glsl("object.frag"), true)
	gfx.objectSh.UpdateLocation(rl.ShaderLocVertexInstancetransform, rl.GetShaderLocationAttrib(gfx.objectSh.Shader, "instanceTransform"))
	gfx.skySh = gfx.program("mesh.vert", glsl("sky.frag"), false)
	gfx.holoSh = gfx.program("mesh.vert", glsl("holo.frag"), false)
	gfx.devilSh = gfx.program("devil.vert", glsl("devil.frag"), false)
	gfx.devilMesh = rl.GenMeshPlane(1, 1, 32, 16)
	gfx.devilMat = rl.LoadMaterialDefault()
	gfx.devilMat.Shader = gfx.devilSh.Shader

	model := gpu.Model
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

	gfx.smallcaps = gpu.LoadFont(gosmallcaps.TTF, 96) // for the signs

	for _, t := range gfx.grounds() {
		t.Vec3("sunAxis", planet.SunAxis())
		for name, v := range map[string]float32{
			"faceCells": planet.FaceCells, "tileCells": planet.TileCells, "tilesPerFace": planet.TilesPerFace,
			"atlasTiles": atlasTiles, "marsRadius": planet.Radius,
		} {
			t.Float(name, v)
		}
	}
	// One material shared by every terrain patch. Patches are freed with
	// UnloadMesh only: unloading a model would also free this shader.
	gfx.tex = bakeTextures()
	// Kept apart from Graphics: with cgo, passing a pointer into a Go object
	// makes the runtime check that whole object for Go pointers, and
	// Graphics holds slices.
	mat := rl.LoadMaterialDefault()
	gfx.terrainMat = &mat
	gfx.terrainMat.Shader = gfx.terrainSh.Shader
	for _, t := range gfx.grounds() {
		t.UpdateLocation(rl.ShaderLocMapNormal, t.Loc("texture2"))
		t.UpdateLocation(rl.ShaderLocMapRoughness, t.Loc("texture3"))
		t.UpdateLocation(rl.ShaderLocMapBrdf, t.Loc("shadowMap"))
	}
	rl.SetMaterialTexture(gfx.terrainMat, rl.MapNormal, gfx.tex.regolith)
	rl.SetMaterialTexture(gfx.terrainMat, rl.MapRoughness, gfx.tex.dust)
	gfx.shadow = gpu.NewSunShadow(shadowRes, shadowRadius, shadowDepth)
	gfx.lampShadow = newLampShadow(min(max(base.Ents.Building.Len(), maxShadeCamps), maxRooms))
	gfx.spots = newSpots()
	rl.SetMaterialTexture(gfx.terrainMat, gpu.MapBrdf, gfx.shadow.Texture())
	holed := mat // the same textures
	gfx.holedMat = &holed
	gfx.holedMat.Shader = gfx.holedSh.Shader

	gfx.signBox = model(rl.GenMeshCube(1, 1, 1), gfx.objectSh)
	for i, c := range []color.RGBA{palette.AccentCyan, palette.AccentBlue, palette.AccentCyan, palette.AccentOrange} {
		gfx.screens = append(gfx.screens, paintScreen(i, c))
	}
	for _, s := range gfx.baseSigns() {
		s.tex = paintSign(s, gfx.smallcaps)
		gfx.signs = append(gfx.signs, s)
	}

	// Every object model gets the painted textures; the shader picks one.
	gfx.objectMaps = []rl.Texture2D{gfx.tex.paint, gfx.tex.panel, gfx.tex.metal, gfx.shadow.Texture(), gfx.tex.fabric, gfx.tex.regolith, gfx.tex.hazard}
	objectMaps := gfx.objectMaps
	for i := range objectMaps {
		gfx.objectSh.UpdateLocation(rl.ShaderLocMapAlbedo+int32(i), gfx.objectSh.Loc(fmt.Sprintf("texture%d", i)))
	}
	gfx.objectSh.UpdateLocation(rl.ShaderLocMapBrdf, gfx.objectSh.Loc("lampShadowMap"))
	for _, m := range []rl.Model{gfx.sphere, gfx.hemi, gfx.cyl, gfx.cone, gfx.cube, gfx.crystal} {
		gfx.bindMaps(m)
	}
	gfx.rocks = rockModels(gfx.objectSh.Shader, objectMaps, planet.RockShapes, 1300, 14, 18)
	gfx.pebbles = rockModels(gfx.objectSh.Shader, objectMaps, pebbleShapes, 1700, 5, 7)

	gfx.eng.Clip(0.15, 9000)

	gfx.objectSh.Float("lampGrid", float32(gfx.lampShadow.Grid))
	gfx.holoSh.Float("tilesPerFace", planet.TilesPerFace)
	gfx.Terrain.Warm(g.Player.Eye(), 5*time.Second)
	return gfx
}

// grounds are the terrain's: whole, and open over the tunnels.
func (gfx *Graphics) grounds() []*gpu.Program { return []*gpu.Program{gfx.terrainSh, gfx.holedSh} }

// lit are those the sun, the lamps and the shadows light.
func (gfx *Graphics) lit() []*gpu.Program {
	return []*gpu.Program{gfx.terrainSh, gfx.holedSh, gfx.objectSh}
}

// put draws a unit model scaled, then rotated (angle in degrees around axis).
func (gfx *Graphics) put(m rl.Model, pos, scale, axis vec3, angle float64, c color.RGBA, spec, emissive float32) {
	gfx.objectSh.Vec2("surface", spec, emissive)
	rl.DrawModelEx(m, gpu.ToRL(pos.Sub(gfx.eng.Origin)), gpu.ToRL(axis), float32(angle), gpu.ToRL(scale), c)
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
func (gfx *Graphics) camera(g *game.Game) gpu.View {
	v := gfx.view(g)
	gfx.camEye, gfx.camLook, gfx.camUp, gfx.camFov = v.Eye, v.Look, v.Up, v.Fovy
	return v
}

func (gfx *Graphics) view(g *game.Game) gpu.View {
	if ph := &g.Photo; g.Mode == game.ModePhoto {
		return gpu.View{Eye: ph.Pos, Look: ph.Pos.Add(ph.Forward()), Up: ph.CamUp(), Fovy: ph.Fov}
	}
	if s := &g.Scope; g.Mode == game.ModeScope {
		return gpu.View{Eye: s.Eye, Look: s.Eye.Add(s.Forward()), Up: vec3{0, 1, 0}, Fovy: s.Fov}
	}
	p := &g.Player
	eye := p.Eye()
	walk := p.MoveAmount
	right, up := p.Right(), p.LocalUp()
	eye = eye.Add(up.Scale(math.Sin(2*p.WalkPhase) * 0.02 * walk)).Add(right.Scale(math.Sin(p.WalkPhase) * 0.012 * walk))
	camUp := up.Add(right.Scale(math.Sin(p.WalkPhase) * 0.008 * walk))
	if g.Sweeping {
		eye = eye.Add(right.Scale(math.Sin(g.Elapsed*37) * 0.002)).Add(up.Scale(math.Sin(g.Elapsed*23+1) * 0.002))
	}
	target := eye.Add(p.Forward())
	if ce, at, ok := g.ChaseCam(); ok {
		eye, target, camUp = ce, at, up
	}
	// Hard landings and boosts shake the view.
	if k := g.Shake(); k > 0 {
		k *= 0.06
		eye = eye.Add(right.Scale(math.Sin(g.Elapsed*61) * k)).Add(up.Scale(math.Sin(g.Elapsed*47+2) * k))
	}
	return gpu.View{Eye: eye, Look: target, Up: camUp, Fovy: g.Fov}
}

// shading is what the shaders light and shade: the base's buildings and
// corridors closest to the camera, or those set down around it.
type shading struct {
	picked       bool
	rooms, corrs []ecs.ID      // the base's
	camps        []game.CampAt // or those set down, the closest first
	site         base.Site     // the frame of the base, or of the closest
	lamps        []lampView    // the lamps casting shadows
	lights       []base.Lamp   // all of them, in the world
	tunnels      bool          // the last lights set had tunnels' too
}

const (
	// campShadeDist is how close to a building the camera comes for the
	// shaders to take it rather than the base.
	campShadeDist = 40.0
	maxShadeCamps = 4 // how many of them
)

// sameCamps tells whether two lists hold the same buildings, where they
// were, in any order.
func sameCamps(a, b []game.CampAt) bool {
	in := func(c game.CampAt, list []game.CampAt) bool {
		return slices.ContainsFunc(list, func(o game.CampAt) bool { return o.Ent == c.Ent && o.Site == c.Site })
	}
	for _, c := range a {
		if !in(c, b) {
			return false
		}
	}
	return len(a) == len(b)
}

// selectBase picks what the shaders light and shade, and its ceiling
// lamps. The lamps' shadows are drawn again when the pick changes.
func (gfx *Graphics) selectBase(g *game.Game) {
	var next shading
	if next.camps = gfx.nearCamps(g); len(next.camps) > 0 {
		next.site = next.camps[0].Site
	} else {
		at := base.ToLocal(gfx.camEye)
		next.rooms, next.corrs, next.site = base.NearBuildings(at, maxRooms), base.NearCorridors(at, maxRooms), base.BaseSite()
	}
	sh := &gfx.shade
	defer gfx.setLights(g)
	if sh.picked && slices.Equal(next.rooms, sh.rooms) && slices.Equal(next.corrs, sh.corrs) && sameCamps(next.camps, sh.camps) && (next.site == sh.site || len(next.camps) > 0) {
		sh.camps, sh.site = next.camps, next.site // maybe in another order
		return
	}
	next.picked = true
	*sh = next
	gfx.lampShadow.reset()
	lamps, casting := base.Lamps(sh.rooms, sh.corrs), len(sh.rooms) // the rooms' first: one each
	sites := []base.Site{sh.site}                                   // those of the lamps, the last for all that follow
	if len(sh.camps) > 0 {
		lamps, casting, sites = nil, len(sh.camps), nil
		for _, c := range sh.camps {
			lamps, sites = append(lamps, c.Layout.Lamps[0]), append(sites, c.Site)
		}
		for _, c := range sh.camps {
			for _, l := range c.Layout.Lamps[1:] {
				lamps, sites = append(lamps, l), append(sites, c.Site)
			}
		}
	}
	for i, l := range lamps[:min(len(lamps), maxLamps)] {
		site := sites[min(i, len(sites)-1)]
		l.At = site.ToWorld(l.At)
		sh.lights = append(sh.lights, l)
		if i < casting {
			sh.lamps = append(sh.lamps, lampView{l.At.Sub(site.Y.Scale(0.12)), site.Y, site.X})
		}
	}
	sh.tunnels = true // set them
}

// setLights gives the shaders the lamps picked, and the tunnels' around:
// those only, from a gallery.
func (gfx *Graphics) setLights(g *game.Game) {
	sh := &gfx.shade
	list, casting := sh.lights, len(sh.lamps)
	if gfx.gallery != nil {
		list, casting = nil, 0
	}
	more := gfx.tunnelLamps(g, maxLamps-min(len(list), maxLamps*2/3))
	if len(more) == 0 && !sh.tunnels {
		return
	}
	sh.tunnels = len(more) > 0
	list = append(list[:min(len(list), maxLamps-len(more)):min(len(list), maxLamps-len(more))], more...)
	var lights, ranges, warm []float32
	for _, l := range list {
		lights = append(lights, float32(l.At.X), float32(l.At.Y), float32(l.At.Z), float32(l.Power))
		ranges = append(ranges, float32(l.Range))
		warm = append(warm, float32(l.Warm))
	}
	if len(list) == 0 { // raylib takes no empty array
		lights, ranges, warm = make([]float32, 4), make([]float32, 1), make([]float32, 1)
	}
	gfx.objectSh.Floats("lightRange", ranges)
	gfx.objectSh.Floats("lightWarm", warm)
	gfx.objectSh.Float("lampCount", float32(casting))
	gfx.objectSh.Vec4s("lights", lights)
	gfx.objectSh.Float("lightCount", float32(len(list)))
}

// nearCamps are the buildings set down closest to the camera, within
// campShadeDist. None stands near the base (game.campFits).
func (gfx *Graphics) nearCamps(g *game.Game) []game.CampAt {
	list := g.CampsNear(gfx.camEye, campShadeDist)
	dist := func(c game.CampAt) float64 {
		at := c.Site.ToLocal(gfx.camEye)
		return math.Hypot(at.X, at.Z) - c.Layout.Reach
	}
	slices.SortFunc(list, func(a, b game.CampAt) int { return cmp.Compare(dist(a), dist(b)) })
	return list[:min(len(list), maxShadeCamps)]
}

func (gfx *Graphics) setUniforms(g *game.Game) {
	gfx.selectBase(g)
	gfx.eng.Uniforms(g.Elapsed, planet.SunDir)
	o := gfx.eng.Origin
	for _, s := range gfx.eng.Programs() {
		s.Float("hazeThin", gfx.hazeThin)
	}
	// The base's shapes, for its ambient occlusion (baseAO); or the
	// bootcamp's, in its own frame.
	sh := &gfx.shade
	var rms, crs, csz []float32
	room := func(r *base.Room) {
		if r.Hall == nil && !r.Plan().Glazed { // halls aren't round, glass doesn't occlude
			rms = append(rms, float32(r.Center.X), float32(r.Center.Z), float32(r.Radius), float32(r.WallH))
		}
	}
	corridor := func(c *base.Corridor) {
		crs = append(crs, float32(c.From.X), float32(c.From.Z), float32(c.To.X), float32(c.To.Z))
		csz = append(csz, float32(c.Width), float32(c.Height), 0, 0)
	}
	for _, id := range sh.rooms {
		room(base.Ents.Building.Get(id))
	}
	for _, id := range sh.corrs {
		corridor(base.Ents.Corridor.Get(id))
	}
	obs := base.Obstacles()
	if len(sh.camps) > 0 {
		obs = nil
		for _, c := range sh.camps { // in the frame of the closest
			in := func(p vec3) vec3 { return sh.site.ToLocal(c.ToWorld(p)) }
			r := c.Layout.Room
			r.Center = in(r.Center)
			room(&r)
			for _, p := range c.Layout.Ports {
				if a := p.Airlock; len(crs) < maxRooms*4 {
					a.From, a.To = in(a.From), in(a.To)
					corridor(&a)
				}
			}
			for _, o := range c.Layout.Obstacles {
				o.At = in(o.At)
				obs = append(obs, o)
			}
		}
	}
	// The slabs over the floors: no lamp shines through them.
	var slabs, slabSize []float32
	for _, f := range base.Ents.Floor.All() {
		if !f.Level() || f.Thick <= 0 || !slices.Contains(sh.rooms, f.In) || len(slabs) >= 8*4 {
			continue
		}
		mid, along, half, width := f.Slab()
		slabs = append(slabs, float32(mid.X), float32(mid.Z), float32(along.X), float32(along.Z))
		slabSize = append(slabSize, float32(half), float32(width/2), float32(f.Bottom()), float32(f.Top()))
	}
	var blobs []float32
	for _, ob := range obs {
		if ob.At.Y < 0.5 && len(blobs) < 48*4 { // on the ground floor
			blobs = append(blobs, float32(ob.At.X), float32(ob.At.Z), float32(ob.Radius), 0)
		}
	}
	for _, sh := range gfx.lit() {
		sh.Vec4s("baseBlobs", blobs)
		sh.Float("blobCount", float32(len(blobs)/4))
		if len(slabs) > 0 { // raylib takes no empty array
			sh.Vec4s("baseSlabs", slabs)
			sh.Vec4s("baseSlabSize", slabSize)
		}
		sh.Float("slabCount", float32(len(slabs)/4))
		sh.Vec3("baseRel", gfx.shade.site.Origin.Sub(o))
		for name, ax := range map[string]vec3{"baseX": gfx.shade.site.X, "baseY": gfx.shade.site.Y, "baseZ": gfx.shade.site.Z} {
			sh.Vec3(name, ax)
		}
		if len(rms) > 0 { // the core base has no dome left
			sh.Vec4s("baseRooms", rms)
		}
		if len(crs) > 0 {
			sh.Vec4s("baseCorrs", crs)
			sh.Vec4s("baseCorrSize", csz)
		}
		sh.Float("roomCount", float32(len(rms)/4))
		sh.Float("corrCount", float32(len(crs)/4))
	}
	gfx.skySh.Vec3("up", gfx.camEye.Norm())
	gfx.skySh.Float("air", gfx.skyAir)
	// Objects take the relief's shadow from where the player stands: those
	// that matter are close, far ones are too small to tell.
	sun, hid := gfx.Terrain.light(gfx.Terrain.Surface(g.Player.Pos))
	gfx.stats.Sun, gfx.stats.AO = sun, hid
	gfx.objectSh.Vec2("ground", float32(sun), float32(hid))

	brush := []float32{0, 0, 0, 0}
	if g.AimOK && g.Mode == game.ModePlay && g.Ghost == nil {
		brush = []float32{float32(g.Aim.X), float32(g.Aim.Y), float32(g.Aim.Z), float32(g.Tool().Radius)}
	}
	for _, t := range gfx.grounds() {
		t.Vec4("brush", brush[0], brush[1], brush[2], brush[3])
	}
	gfx.uploadSteps(&g.Steps)
	gfx.setGrid(g)
	gfx.setHoles(g)
	ao := float32(1)
	if gfx.gallery != nil {
		ao = 0
	}
	for _, sh := range gfx.lit() {
		sh.Float("baseNear", ao)
	}

	// Soft round shadows of the base's domes, for when the base is beyond
	// the shadow map.
	var casters []float32
	for _, id := range base.NearBuildings(base.ToLocal(gfx.camEye), maxCasters) {
		r := base.Ents.Building.Get(id)
		if r.Plan().Glazed {
			continue // the sun goes through
		}
		c := base.ToWorld(r.Center.Add(vec3{0, r.WallH * 0.7, 0}))
		radius := r.Radius * 1.05
		if r.Hall != nil {
			radius = r.Hall.Size[0] * 0.55
		}
		casters = append(casters, float32(c.X), float32(c.Y), float32(c.Z), float32(radius))
	}
	for _, c := range g.CampsNear(gfx.camEye, campDrawDist) {
		r := c.Layout.Room
		if r.Plan().Glazed {
			continue
		}
		at := c.ToWorld(r.Center.Add(vec3{0, r.WallH * 0.7, 0}))
		radius := r.Radius * 1.05
		if r.Hall != nil {
			radius = r.Hall.Size[0] * 0.55
		}
		casters = append(casters, float32(at.X), float32(at.Y), float32(at.Z), float32(radius))
	}
	n := min(len(casters)/4, maxCasters)
	cols := gfx.shadow.Cols(o)
	for _, t := range gfx.grounds() {
		t.Vec4s("casters", casters[:n*4])
		t.Float("casterCount", float32(n))
		t.Vec4s("lightVP", cols)
	}
	gfx.objectSh.Vec4s("lightVP", cols)
}

// Draw draws a frame of the game; overlay draws over it, on the screen
// (the interface).
func (gfx *Graphics) Draw(g *game.Game, overlay func(cam rl.Camera3D)) {
	view := gfx.camera(g)
	cam := view.Camera()
	gfx.lookUnder(g)
	gfx.clipPlanes(g)
	gfx.eng.Look(view)

	atlas, lookup := gfx.dust.Upload(g.World, g.Player.Pos)
	rl.SetMaterialTexture(gfx.terrainMat, rl.MapDiffuse, atlas)
	rl.SetMaterialTexture(gfx.terrainMat, rl.MapSpecular, lookup)
	patches := gfx.Terrain.Visible(gfx.camEye)
	gfx.stats.Patches = len(patches)
	gfx.nearGems = g.World.GemsNear(g.Player.Pos, gemDrawDist)
	// Objects cast their shadows first, then the scene is lit with them.
	outside := gfx.gallery == nil // what is outside may be seen
	gfx.casting = true
	if outside {
		gfx.shadow.Render(gfx.eng, g.Player.Pos, planet.SunDir, func() {
			gfx.drawRocks(g)
			gfx.drawBase(g)
			gfx.drawCamps(g, gfx.inShadowMap(g.Player.Pos))
			gfx.drawEntities(g, gfx.inShadowMap(g.Player.Pos))
		})
	}
	// The rooms' lamps cast shadows too, drawn once.
	gfx.setUniforms(g)
	if gfx.base != nil && outside {
		gfx.lampShadow.render(gfx, gfx.shade.lamps, func() {
			gfx.drawBase(g)
			gfx.drawCamps(g, func(at vec3, r float64) bool { return at.Sub(gfx.camEye).Length() < 2*campShadeDist+r })
		})
		gfx.objectSh.Vec4s("lampVP", gfx.lampShadow.cols(gfx.eng.Origin))
	}

	gfx.drawSpots(g)
	gfx.casting = false

	sunUp := gfx.camEye.Norm().Dot(planet.SunDir)
	sunUp += (1 - sunUp) * gfx.under // the same light at any hour, under the ground
	indoors := g.Indoors(gfx.camEye)
	gfx.eng.Draw(gpu.Frame{
		Layers:  gfx.layers(g, cam, outside, indoors, patches),
		Sun:     planet.SunDir,
		SunUp:   sunUp,
		Indoors: indoors,
		Clock:   g.Elapsed,
		Look:    gfx.look(g),
		Overlay: overlay,
	})
}

// layers are what is drawn of the scene, in order.
func (gfx *Graphics) layers(g *game.Game, cam rl.Camera3D, outside, indoors bool, patches []*lodNode) []gpu.Layer {
	held := gpu.Layer{Name: "tool", Tool: true, Draw: func() {
		if indoors {
			gfx.setLighting(mesh.LitIndoor, vec3{})
		}
		gfx.drawTool(g)
		gfx.setLighting(mesh.LitSun, vec3{})
	}}
	over := []gpu.Layer{
		held,
		{Name: "glows", Draw: func() { gfx.drawGlows(g, cam) }},
		{Name: "globe", Draw: func() { gfx.drawGlobe(g, cam) }},
	}
	if !outside {
		return append([]gpu.Layer{{Name: "tunnel", Draw: func() { gfx.drawTunnel(gfx.gallery, gfx.inView(cam)) }}}, over...)
	}
	return append([]gpu.Layer{
		// A big sphere around the camera, seen from inside.
		{Name: "sky", Sky: true, Draw: func() { rl.DrawModel(gfx.sky, cam.Position, 400, rl.White) }},
		// The base first: the terrain it hides then fails the depth test
		// before its costly shader runs (most of the screen, indoors). Its
		// depth is laid first: a wall behind a home's lining isn't shaded
		// for nothing.
		{Name: "base", Draw: func() {
			gfx.baseDepth()
			gfx.drawBase(g)
			gfx.drawCamps(g, gfx.inView(cam))
			gfx.drawEntities(g, gfx.inView(cam))
		}},
		{Name: "terrain", Draw: func() {
			// Culling is off: patch winding differs from one cube face to the next.
			rl.DisableBackfaceCulling()
			for _, n := range patches {
				mat := gfx.terrainMat
				if gfx.holed(n) {
					mat = gfx.holedMat
				}
				rl.DrawMesh(n.mesh, *mat, rl.MatrixIdentity())
			}
			rl.EnableBackfaceCulling()
		}},
		{Name: "ground", Draw: func() {
			gfx.drawRocks(g)
			gfx.drawPebbles(g)
			gfx.drawGems(g)
		}},
		{Name: "devils", Draw: func() { gfx.drawDevils(g) }},
		{Name: "ghost", Draw: func() { gfx.drawGhost(g) }},
		{Name: "glass", Draw: func() {
			gfx.drawGlass()
			gfx.drawCampGlass(g, gfx.inView(cam))
			gfx.drawPanes(g, gfx.inView(cam))
		}},
	}, over...)
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
// big: see mountains far away. The depth buffer's grain grows as the
// square of the distance over the near plane: it stays as far out as the
// player's body allows, 35 cm wide (the held tool has its own, gpu.Renderer.ToolCols).
// High up in creative flight, see the whole visible half of Mars.
func (gfx *Graphics) clipPlanes(g *game.Game) {
	alt := g.World.Relief.Altitude(gfx.camEye)
	gfx.hazeThin = float32(1/(1+max(0, alt-20)/150)) * gfx.hazeUnder()
	gfx.skyAir = float32(1 - mathx.Smoothstep(300, 5000, alt))
	gfx.eng.Clip(math.Max(0.15, math.Min(20, alt*0.002)), math.Max(9000, gfx.camEye.Length()+planet.Radius))
}

// drawTool draws the held tool, in the lower right corner of the view.
func (gfx *Graphics) drawTool(g *game.Game) {
	if g.Mode == game.ModePaused || g.Mode == game.ModePhoto || g.Mode == game.ModeScope || g.Player.Fly || g.Chase() {
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
		rl.DrawBillboard(cam, gfx.glow, gpu.ToRL(p.Pos), float32(p.Size*3), c)
	}

	rl.BeginBlendMode(rl.BlendAdditive)
	for _, p := range g.Particles {
		if !p.Additive {
			continue
		}
		c := p.C
		c.A = uint8(255 * mathx.Clamp01(p.Life/p.MaxLife*1.5) * gfx.softness(g, p))
		rl.DrawBillboard(cam, gfx.glow, gpu.ToRL(p.Pos), float32(p.Size*4), c)
	}
	for _, gem := range gfx.nearGems {
		vis := gfx.gemVisibility(g, gem)
		if vis <= 0 {
			continue
		}
		pulse := 0.75 + 0.25*math.Sin(g.Elapsed*3+float64(gem.Key))
		c := color.RGBA{80, 200, 230, uint8(160 * vis * pulse)}
		rl.DrawBillboard(cam, gfx.glow, gpu.ToRL(gfx.gemPos(g, gem, 0.06+0.16*vis)), float32(1.4*vis), c)
	}
	if math.Mod(g.Elapsed, 1.2) < 0.3 {
		rl.DrawBillboard(cam, gfx.glow, gpu.ToRL(beaconTop()), 0.9, color.RGBA{255, 80, 60, 200})
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
	rl.SetMaterialTexture(&m.GetMaterials()[0], gpu.MapBrdf, gfx.lampShadow.Target.Texture)
}

// putBasis draws a unit model with its X, Y and Z axes sent to ax, ay and az
// (their lengths scale it): any orientation, any proportions.
func (gfx *Graphics) putBasis(m rl.Model, pos, ax, ay, az vec3, c color.RGBA, spec, emissive float32) {
	gfx.objectSh.Vec2("surface", spec, emissive)
	mat := m.GetMaterials()[0]
	mat.GetMap(rl.MapAlbedo).Color = c
	rl.DrawMesh(m.GetMeshes()[0], mat, gpu.Basis(pos.Sub(gfx.eng.Origin), ax, ay, az))
	mat.GetMap(rl.MapAlbedo).Color = rl.White
}

// Stats are what the debug overlay shows of the rendering.
type Stats struct {
	Patches, Pebbles int     // drawn this frame
	Loaded, Pending  int     // terrain meshes on the GPU, being built
	Lamps            int     // the player's, lighting the scene
	Sun, AO          float64 // relief light under the player
}

func (gfx *Graphics) Stats() Stats {
	s := gfx.stats
	s.Loaded, s.Pending = len(gfx.Terrain.loaded), gfx.Terrain.pending
	return s
}
