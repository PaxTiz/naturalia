package render

import (
	"embed"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Every surface is painted by the shaders in shaders/: no image file is
// ever loaded. World positions are relative to Mars' center, so
// normalize(fragPos) is the local "up" anywhere on the surface.
//
// The files go through a small preprocessor: `#include "file.glsl"` pastes
// a shared file, `${name}` a constant of the Go code (shaderConsts), so
// both sides keep the same value.

//go:embed shaders/*
var shaderFiles embed.FS

// shaderConsts are the Go constants the shaders read.
var shaderConsts = map[string]float64{
	"regolithTile":  regolithTile,
	"dustTile":      dustTile,
	"bloomStrength": bloomStrength,
}

var (
	includeRe = regexp.MustCompile(`(?m)^#include "([^"]+)"$`)
	constRe   = regexp.MustCompile(`\$\{(\w+)\}`)
)

// glsl reads a shader, includes and constants resolved. A missing file or
// constant is a bug in the game: it panics at startup.
func glsl(name string) string {
	data, err := shaderFiles.ReadFile("shaders/" + name)
	if err != nil {
		panic(err)
	}
	src := includeRe.ReplaceAllStringFunc(string(data), func(line string) string {
		return strings.TrimRight(glsl(includeRe.FindStringSubmatch(line)[1]), "\n")
	})
	return constRe.ReplaceAllStringFunc(src, func(ref string) string {
		v, ok := shaderConsts[constRe.FindStringSubmatch(ref)[1]]
		if !ok {
			panic(fmt.Sprintf("shader %s: unknown constant %s", name, ref))
		}
		return strconv.FormatFloat(v, 'f', 3, 64)
	})
}
