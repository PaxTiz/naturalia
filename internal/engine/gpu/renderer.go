package gpu

import (
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Renderer draws a frame: the programs, the view and its clip planes, the
// image it is made into.
type Renderer struct {
	Post      *Post
	View      View    // this frame's (Look)
	Origin    vec3    // the programs' origin this frame (Look)
	Near, Far float64 // the view's clip planes (Clip)
	programs  []*Program
	casters   []*Program
}

// View is where the camera stands, looks and which way its top points,
// in double precision, and its vertical field of view in degrees.
type View struct {
	Eye, Look, Up vec3
	Fovy          float64
}

// Camera is the view as raylib takes it.
func (v View) Camera() rl.Camera3D {
	return rl.Camera3D{Position: ToRL(v.Eye), Target: ToRL(v.Look), Up: ToRL(v.Up), Fovy: float32(v.Fovy), Projection: rl.CameraPerspective}
}

func NewRenderer() *Renderer { return &Renderer{Post: newPost()} }

// Add has the renderer set a program's view, origin and clock; a caster
// also writes its depth into the depth maps (its shadowPass uniform).
func (r *Renderer) Add(p *Program, caster bool) *Program {
	r.programs = append(r.programs, p)
	if caster {
		r.casters = append(r.casters, p)
	}
	return p
}

// Programs are those added, in their order.
func (r *Renderer) Programs() []*Program { return r.programs }

// Clip sets the view's depth range.
func (r *Renderer) Clip(near, far float64) {
	r.Near, r.Far = near, far
	rl.SetClipPlanes(near, far)
}

// Look takes the view of the frame, and sets the programs' origin
// (common.glsl): the eye snapped to a 32 m grid, so the tile offsets they
// derive from it stay put between snaps. Every pass projects from it, the
// shadows' too.
func (r *Renderer) Look(v View) {
	r.View = v
	snap := func(v float64) float64 { return float64(float32(math.Floor(v/32) * 32)) }
	r.Origin = vec3{snap(v.Eye.X), snap(v.Eye.Y), snap(v.Eye.Z)}
	for _, p := range r.programs {
		p.Vec3("origin", r.Origin)
	}
}

// Uniforms gives the programs what every one of them reads: the eye, the
// clock and the sun's direction.
func (r *Renderer) Uniforms(clock float64, sun vec3) {
	eye := ToRL(r.View.Eye)
	for _, p := range r.programs {
		rl.SetShaderValue(p.Shader, p.Loc("viewPos"), []float32{eye.X, eye.Y, eye.Z}, rl.ShaderUniformVec3)
		p.Float("time", float32(clock))
		p.Vec3("sunDir", sun)
	}
	for _, p := range r.programs {
		p.Vec3("viewRel", r.View.Eye.Sub(r.Origin))
	}
}

// SetView gives every program the camera of the next draws, about its
// eye at (RelVP): the shaders project fragRel less viewAt. About origin,
// up to 50 m off, the sums giving depth would cancel down to a few meters
// and keep the rounding of the big terms, more than a depth layer
// (mesh.Unfight): flush surfaces fought a few meters away.
func (r *Renderer) SetView(cols []float32, at vec3) {
	for _, p := range r.programs {
		p.Vec4s("viewProj", cols)
		p.Vec3("viewAt", at.Sub(r.Origin))
	}
}

// ViewCols is the view's camera about its eye (SetView), as BeginMode3D
// sets it for the scene's image, between the clip planes near and far.
func (r *Renderer) ViewCols(near, far float64) []float32 {
	t := r.Post.scene.Texture
	v := r.View
	proj := rl.MatrixPerspective(float32(v.Fovy)*math.Pi/180, float32(t.Width)/float32(t.Height), float32(near), float32(far))
	return RelVP(v.Eye, v.Look, v.Up, v.Eye, proj)
}

// ToolCols is the view's camera for what is held: its own near plane, a
// centimeter from the eye, and its depth squeezed into the front hundredth
// of the range, where the world isn't (it starts at the view's near
// plane). It is never clipped, nor sinks into a wall.
func (r *Renderer) ToolCols() []float32 {
	const k = 0.01
	c := r.ViewCols(0.01, 10)
	for i := 0; i < 16; i += 4 {
		c[i+2] = k*c[i+2] - (1-k)*c[i+3] // z: from [-w, w] to [-w, (2k-1)w]
	}
	return c
}

// Depth has the casters write only their depth (shadowPass), or color
// again.
func (r *Renderer) Depth(on bool) {
	for _, p := range r.casters {
		p.Float("shadowPass", b2f(on))
	}
}

// Frame is what is drawn: layers in order, then the image is made, then
// the overlay is drawn over it, on the screen.
type Frame struct {
	Layers  []Layer
	Sun     vec3    // the sun's direction
	SunUp   float64 // its height where the eye is: 1 overhead, 0 on the horizon
	Indoors bool    // the eye adapts to rooms
	Clock   float64 // seconds
	Look    Look
	Overlay func(cam rl.Camera3D)
}

// Layer is a thing to draw.
type Layer struct {
	Name string
	Draw func()
	Sky  bool // seen from inside, behind everything: no face culled, no depth written
	Tool bool // held before the eye: its own depth range (ToolCols)
}

// Draw draws a frame. The view is the last one Look took.
func (r *Renderer) Draw(f Frame) {
	cam := r.View.Camera()
	r.Post.adapt(float64(rl.GetFrameTime()), f.SunUp, f.Indoors)
	r.Post.resize()
	rl.BeginTextureMode(r.Post.scene)
	rl.ClearBackground(rl.Black)
	rl.BeginMode3D(cam)
	view := r.ViewCols(r.Near, r.Far)
	r.SetView(view, r.View.Eye)
	for _, l := range f.Layers {
		switch {
		case l.Sky:
			rl.DisableBackfaceCulling()
			rl.DisableDepthMask()
			l.Draw()
			rl.EnableDepthMask()
			rl.EnableBackfaceCulling()
		case l.Tool:
			r.SetView(r.ToolCols(), r.View.Eye)
			l.Draw()
			r.SetView(view, r.View.Eye)
		default:
			l.Draw()
		}
	}
	rl.EndMode3D()
	rl.EndTextureMode()

	rl.BeginDrawing()
	f.Look.Near, f.Look.Far = r.Near, r.Far
	r.Post.finish(cam, f.Sun, f.SunUp, f.Clock, f.Look)
	if f.Overlay != nil {
		f.Overlay(cam)
	}
	rl.EndDrawing()
}
