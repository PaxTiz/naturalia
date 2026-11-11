// Package palette holds the colors shared by the game, its packs and its
// rendering (docs/DIRECTION_ARTISTIQUE.md).
package palette

import "image/color"

// Interface and effects: text, credits, sparks.
var (
	White = color.RGBA{235, 237, 245, 255}
	Gray  = color.RGBA{150, 155, 178, 255}
	Gold  = color.RGBA{250, 210, 90, 255}
	Green = color.RGBA{110, 220, 120, 255}
	Red   = color.RGBA{235, 90, 90, 255}
	Cyan  = color.RGBA{120, 235, 245, 255}
)

// The base's accents: they stand out against the orange ground.
var (
	AccentCyan   = color.RGBA{80, 210, 235, 255}
	AccentOrange = color.RGBA{255, 140, 50, 255}
	AccentBlue   = color.RGBA{90, 140, 255, 255}
	AccentPurple = color.RGBA{170, 120, 255, 255}
	LampWarm     = color.RGBA{255, 236, 200, 255}
)

// Painted surfaces: drones, fabrics.
var (
	MatWhite = color.RGBA{232, 234, 240, 255}
	MatSteel = color.RGBA{150, 155, 170, 255}
	MatDark  = color.RGBA{48, 52, 66, 255}
)
