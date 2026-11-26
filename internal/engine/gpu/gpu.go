// Package gpu is the engine's drawing: shader programs, the view and its
// passes, the depth maps that make shadows, and the final image. It knows
// nothing of the game; the game hands it, each frame, a list of things to
// draw (Frame).
//
// Every position is in double precision until the last moment: the
// programs take them about an origin near the eye (Renderer.Look), the
// projections about the eye itself (RelVP). Far from it, float32 rounding
// would make flush surfaces fight and shadows crawl.
package gpu

import (
	"naturalia/internal/engine/mathx"

	rl "github.com/gen2brain/raylib-go/raylib"
)

type vec3 = mathx.Vec3

const (
	// MapBrdf is raylib's MATERIAL_MAP_BRDF, a material's slot for a map
	// of the engine's own (the shadows). raylib-go's MapBrdf is 11,
	// shifted by a stray MapBrdg in its enum.
	MapBrdf = 10

	// OpenGL's blend equations and factors, for rl.SetBlendFactors.
	glMin, glAdd, glOne, glZero = 0x8007, 0x8006, 1, 0
)

// ToRL is a vector as raylib takes it, in float32.
func ToRL(a vec3) rl.Vector3 { return rl.NewVector3(float32(a.X), float32(a.Y), float32(a.Z)) }

// F3 is a vector as a uniform takes it.
func F3(v vec3) []float32 { return []float32{float32(v.X), float32(v.Y), float32(v.Z)} }

// Basis sends a model's X, Y and Z axes to ax, ay and az, at pos: any
// orientation, any proportions.
func Basis(pos, ax, ay, az vec3) rl.Matrix {
	return rl.Matrix{
		M0: float32(ax.X), M1: float32(ax.Y), M2: float32(ax.Z),
		M4: float32(ay.X), M5: float32(ay.Y), M6: float32(ay.Z),
		M8: float32(az.X), M9: float32(az.Y), M10: float32(az.Z),
		M12: float32(pos.X), M13: float32(pos.Y), M14: float32(pos.Z), M15: 1,
	}
}

// RelVP is a camera's view and projection about o, by columns: passing a
// Matrix by value is unreliable without cgo.
func RelVP(eye, look, up, o vec3, proj rl.Matrix) []float32 {
	view := rl.MatrixLookAt(ToRL(eye.Sub(o)), ToRL(look.Sub(o)), ToRL(up))
	m := rl.MatrixMultiply(view, proj)
	return []float32{m.M0, m.M1, m.M2, m.M3, m.M4, m.M5, m.M6, m.M7, m.M8, m.M9, m.M10, m.M11, m.M12, m.M13, m.M14, m.M15}
}

// Model makes a model of a mesh, drawn by a program.
func Model(m rl.Mesh, p *Program) rl.Model {
	mod := rl.LoadModelFromMesh(m)
	mod.GetMaterials()[0].Shader = p.Shader
	return mod
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

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
