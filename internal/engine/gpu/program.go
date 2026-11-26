package gpu

import (
	"fmt"
	"io/fs"
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Program is a shader, with its uniforms' locations looked up once.
type Program struct {
	rl.Shader
	locs map[string]int32
}

// NewProgram compiles a vertex and a fragment shader.
func NewProgram(vs, fs string) *Program {
	return &Program{Shader: rl.LoadShaderFromMemory(vs, fs), locs: map[string]int32{}}
}

// Loc is a uniform's location, -1 when the program has none of that name
// (setting it then does nothing).
func (p *Program) Loc(name string) int32 {
	l, ok := p.locs[name]
	if !ok {
		l = rl.GetShaderLocation(p.Shader, name)
		p.locs[name] = l
	}
	return l
}

func (p *Program) Float(name string, v float32) {
	rl.SetShaderValue(p.Shader, p.Loc(name), []float32{v}, rl.ShaderUniformFloat)
}

// Int sets an int uniform, a sampler's slot for one.
func (p *Program) Int(name string, v int32) {
	// raylib-go takes floats: the int in a float's bits.
	rl.SetShaderValue(p.Shader, p.Loc(name), []float32{math.Float32frombits(uint32(v))}, rl.ShaderUniformInt)
}

func (p *Program) Vec2(name string, x, y float32) {
	rl.SetShaderValue(p.Shader, p.Loc(name), []float32{x, y}, rl.ShaderUniformVec2)
}

func (p *Program) Vec3(name string, v vec3) {
	rl.SetShaderValue(p.Shader, p.Loc(name), F3(v), rl.ShaderUniformVec3)
}

func (p *Program) Vec4(name string, x, y, z, w float32) {
	rl.SetShaderValue(p.Shader, p.Loc(name), []float32{x, y, z, w}, rl.ShaderUniformVec4)
}

// Floats sets an array of floats; nothing if it is empty (raylib takes no
// empty array).
func (p *Program) Floats(name string, v []float32) {
	if len(v) > 0 {
		rl.SetShaderValueV(p.Shader, p.Loc(name), v, rl.ShaderUniformFloat, int32(len(v)))
	}
}

// Vec4s sets an array of vec4, or a mat4 by columns; nothing if it is
// empty.
func (p *Program) Vec4s(name string, v []float32) {
	if len(v) >= 4 {
		rl.SetShaderValueV(p.Shader, p.Loc(name), v, rl.ShaderUniformVec4, int32(len(v)/4))
	}
}

func (p *Program) Matrix(name string, m rl.Matrix) {
	rl.SetShaderValueMatrix(p.Shader, p.Loc(name), m)
}

func (p *Program) Texture(name string, t rl.Texture2D) {
	rl.SetShaderValueTexture(p.Shader, p.Loc(name), t)
}

// Sources reads shaders through a small preprocessor: `#include "file"`
// pastes a shared file, `${name}` a constant of the Go code, so both sides
// keep the same value.
type Sources struct {
	FS     fs.FS
	Dir    string
	Consts map[string]float64
}

var (
	includeRe = regexp.MustCompile(`(?m)^#include "([^"]+)"$`)
	constRe   = regexp.MustCompile(`\$\{(\w+)\}`)
)

// Read reads a shader, includes and constants resolved. A missing file or
// constant is a bug: it panics at startup.
func (s Sources) Read(name string) string {
	data, err := fs.ReadFile(s.FS, path.Join(s.Dir, name))
	if err != nil {
		panic(err)
	}
	src := includeRe.ReplaceAllStringFunc(string(data), func(line string) string {
		return strings.TrimRight(s.Read(includeRe.FindStringSubmatch(line)[1]), "\n")
	})
	return constRe.ReplaceAllStringFunc(src, func(ref string) string {
		v, ok := s.Consts[constRe.FindStringSubmatch(ref)[1]]
		if !ok {
			panic(fmt.Sprintf("shader %s: unknown constant %s", name, ref))
		}
		return strconv.FormatFloat(v, 'f', 3, 64)
	})
}
