package gpu

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// DepthMap keeps what casters are nearest to one or more lenses: a float
// texture, in the cells of a grid, one view each. The casters write their
// depth themselves (Renderer.Depth), blended with "min" to keep the
// nearest. The programs that are lit then look it up to know what stands
// between them and a light.
type DepthMap struct {
	Target    rl.RenderTexture2D
	Grid      int     // cells per side
	Cell      int     // texels per side of a cell
	Near, Far float64 // the lenses' depth range
}

// NewDepthMap makes a map of grid×grid cells. Clamped, it is read texel by
// texel and nothing repeats past its edges.
func NewDepthMap(grid, cell int, near, far float64, clamp bool) *DepthMap {
	d := &DepthMap{Grid: grid, Cell: cell, Near: near, Far: far}
	res := grid * cell
	img := rl.GenImageColor(res, res, rl.White)
	rl.ImageFormat(img, rl.UncompressedR32)
	d.Target.Texture = rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	if clamp {
		rl.SetTextureFilter(d.Target.Texture, rl.FilterPoint)
		rl.SetTextureWrap(d.Target.Texture, rl.WrapClamp)
	}
	d.Target.ID = rl.LoadFramebuffer()
	rl.FramebufferAttach(d.Target.ID, d.Target.Texture.ID, rl.AttachmentColorChannel0, rl.AttachmentTexture2d, 0)
	if !rl.FramebufferComplete(d.Target.ID) {
		panic("depth map framebuffer incomplete")
	}
	rl.DisableFramebuffer()
	return d
}

// Lens is a light's camera: square, perspective or orthographic (Fovy is
// then its width, in meters).
type Lens struct {
	Eye, Look, Up vec3
	Fovy          float64
	Ortho         bool
}

func (d *DepthMap) proj(l Lens) rl.Matrix {
	if l.Ortho {
		h := float32(l.Fovy / 2)
		return rl.MatrixOrtho(-h, h, -h, h, float32(d.Near), float32(d.Far))
	}
	return rl.MatrixPerspective(float32(l.Fovy*math.Pi/180), 1, float32(d.Near), float32(d.Far))
}

// Cols is a lens of the map about origin o, by columns (RelVP): the lit
// programs look the map up from fragRel. From the world position, some
// 3 km from Mars' center, float precision is a quarter millimeter, a
// grain that shifts as the camera moves and makes the shadows crawl.
func (d *DepthMap) Cols(l Lens, o vec3) []float32 { return RelVP(l.Eye, l.Look, l.Up, o, d.proj(l)) }

// Begin binds the map to draw into it, all cleared if asked.
func (d *DepthMap) Begin(r *Renderer, clear bool) {
	rl.BeginTextureMode(d.Target)
	if clear {
		rl.ClearBackground(rl.White)
	}
	rl.SetClipPlanes(d.Near, d.Far)
	r.Depth(true)
}

// End gives the view back its clip planes and the screen its colors.
func (d *DepthMap) End(r *Renderer) {
	r.Depth(false)
	rl.SetClipPlanes(r.Near, r.Far)
	rl.EndTextureMode()
}

// Draw draws into cell i what lens l sees of the casters draw draws, that
// cell cleared first if asked. Thin and open meshes cast too: no face is
// culled.
func (d *DepthMap) Draw(r *Renderer, i int, l Lens, clear bool, draw func()) {
	cell := int32(d.Cell)
	x, y := int32(i%d.Grid)*cell, int32(i/d.Grid)*cell
	if clear {
		rl.EnableScissorTest()
		rl.Scissor(x, y, cell, cell)
		rl.ClearBackground(rl.White)
	}
	proj := rl.CameraPerspective
	if l.Ortho {
		proj = rl.CameraOrthographic
	}
	rl.BeginMode3D(rl.Camera3D{Position: ToRL(l.Eye), Target: ToRL(l.Look), Up: ToRL(l.Up), Fovy: float32(l.Fovy), Projection: proj})
	rl.Viewport(x, y, cell, cell)
	r.SetView(d.Cols(l, l.Eye), l.Eye)
	rl.SetBlendFactors(glOne, glOne, glMin)
	rl.BeginBlendMode(rl.BlendCustom)
	rl.DisableBackfaceCulling()
	draw()
	rl.EnableBackfaceCulling()
	rl.EndBlendMode()
	rl.EndMode3D()
	if clear {
		rl.DisableScissorTest()
	}
}

// SunShadow is the shadows of what stands around a point, seen down the
// sun's rays; one map, moved with that point.
type SunShadow struct {
	Map    *DepthMap
	Radius float64 // meters covered around the point
	lens   Lens
}

// NewSunShadow makes a map of res texels covering radius around its point,
// depth deep along the rays.
func NewSunShadow(res int, radius, depth float64) *SunShadow {
	return &SunShadow{Map: NewDepthMap(1, res, 1, depth, false), Radius: radius}
}

// Texture is the map, for the lit programs.
func (s *SunShadow) Texture() rl.Texture2D { return s.Map.Target.Texture }

// aim looks down the sun's rays at center, which is snapped to the map's
// texels so the shadows don't shimmer as the player walks.
func (s *SunShadow) aim(center, sun vec3) {
	ref := vec3{0, 1, 0}
	if math.Abs(sun.Y) > 0.9 {
		ref = vec3{1, 0, 0}
	}
	right := sun.Cross(ref).Norm()
	up := right.Cross(sun).Norm()
	texel := 2 * s.Radius / float64(s.Map.Cell)
	snap := func(v float64) float64 { return math.Floor(v/texel) * texel }
	x, y, z := center.Dot(right), center.Dot(up), center.Dot(sun)
	center = right.Scale(snap(x)).Add(up.Scale(snap(y))).Add(sun.Scale(z))
	s.lens = Lens{Eye: center.Add(sun.Scale(s.Map.Far / 2)), Look: center, Up: up, Fovy: 2 * s.Radius, Ortho: true}
}

// Render draws the casters around center into the map.
func (s *SunShadow) Render(r *Renderer, center, sun vec3, draw func()) {
	s.aim(center, sun)
	s.Map.Begin(r, true)
	s.Map.Draw(r, 0, s.lens, false, draw)
	s.Map.End(r)
}

// Cols is the sun's camera about origin o (DepthMap.Cols).
func (s *SunShadow) Cols(o vec3) []float32 { return s.Map.Cols(s.lens, o) }

// Covers tells whether a sphere casts into the map around center.
func (s *SunShadow) Covers(center, sun, at vec3, radius float64) bool {
	d := at.Sub(center)
	return d.Sub(sun.Scale(d.Dot(sun))).Length() < s.Radius*math.Sqrt2+radius
}
