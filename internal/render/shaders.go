package render

import (
	"embed"
	"naturalia/internal/engine/gpu"
)

// Every surface is painted by the shaders in shaders/: no image file is
// ever loaded. World positions are relative to Mars' center, so
// normalize(fragPos) is the local "up" anywhere on the surface. The image
// they make is finished by the engine's own (gpu.Post).

//go:embed shaders/*
var shaderFiles embed.FS

// shaders are the scene's, with the Go constants they read.
var shaders = gpu.Sources{FS: shaderFiles, Dir: "shaders", Consts: map[string]float64{
	"regolithTile": regolithTile,
	"dustTile":     dustTile,
}}

// glsl reads a shader of the scene (gpu.Sources).
func glsl(name string) string { return shaders.Read(name) }
