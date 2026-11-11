package main

import (
	"image/color"
	"mars_sweeper/internal/base"
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/game"
	"mars_sweeper/internal/mathx"
	"mars_sweeper/internal/mesh"
	"mars_sweeper/internal/palette"
	"math"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// sign is a painted label on a wall of the base.
type sign struct {
	at     vec3 // local, center of the sign
	facing vec3 // horizontal direction the text faces
	width  float64
	text   string
	accent color.RGBA
	tex    rl.Texture2D
}

const signHeight = 0.34

func (gfx *Graphics) baseSigns() []sign {
	inHub := func(angle float64, text string, accent color.RGBA) sign {
		dir := vec3{math.Cos(angle), 0, math.Sin(angle)}
		at := dir.Scale(base.HubRoom.Radius - base.WallThick/2 - 0.03).Add(vec3{0, 2.6, 0})
		return sign{at: at, facing: dir.Scale(-1), width: 1.9, text: text, accent: accent}
	}
	// Over each door of the hub, the name of the building it leads to.
	var signs []sign
	for _, bp := range base.Plans[1:] {
		if bp.Sign != "" && bp.Link == base.Plans[0].ID {
			d := bp.At.Sub(base.Plans[0].At)
			signs = append(signs, inHub(math.Atan2(d.Z, d.X), bp.Sign, gfx.tok(bp.Accent).Col))
		}
	}
	return append(signs,
		inHub(base.South, "SAS 01 - SORTIE", palette.Gold),
		sign{at: base.AirlockEnd.Add(vec3{0, 3.05, -0.2}), facing: vec3{0, 0, -1}, width: 2.6, text: "BASE MARTIENNE - SAS 01", accent: palette.Gold},
	)
}

// paintSign draws a sign's texture in flat colors: a dark steel plate, an
// accent tab on the left with a hazard notch, and the text in small capitals.
func paintSign(s sign, font rl.Font) rl.Texture2D {
	const h = 128
	w := int(math.Round(s.width / signHeight * h))
	img := rl.GenImageColor(w, h, color.RGBA{44, 48, 60, 255})
	rl.ImageDrawRectangle(img, 0, 0, int32(w), 6, color.RGBA{64, 70, 86, 255})
	rl.ImageDrawRectangle(img, 0, h-6, int32(w), 6, color.RGBA{30, 33, 42, 255})
	rl.ImageDrawRectangle(img, 0, 0, 34, h, s.accent)
	rl.ImageDrawRectangle(img, 44, h-22, int32(w-58), 5, s.accent)
	size := float32(64)
	tw := rl.MeasureTextEx(font, s.text, size, 2).X
	if tw > float32(w-90) {
		size *= float32(w-90) / tw
		tw = rl.MeasureTextEx(font, s.text, size, 2).X
	}
	rl.ImageDrawTextEx(img, rl.NewVector2(34+(float32(w-34)-tw)/2, (h-22-size)/2+2), font, s.text, size, 2, color.RGBA{236, 232, 220, 255})
	// The cube's front face shows the image upside down.
	rl.ImageFlipVertical(img)
	tex := rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.GenTextureMipmaps(&tex)
	rl.SetTextureFilter(tex, rl.FilterTrilinear)
	return tex
}

func (gfx *Graphics) drawSigns(g *game.Game) {
	gfx.setMat(content.MtDecal, 1, false)
	for _, s := range gfx.signs {
		rl.SetMaterialTexture(&gfx.signBox.GetMaterials()[0], rl.MapAlbedo, s.tex)
		if base.Indoors(s.at.Sub(s.facing.Scale(0.3))) {
			gfx.setLighting(mesh.LitIndoor, vec3{})
		} else {
			gfx.setLighting(mesh.LitSun, vec3{})
		}
		yaw := math.Atan2(s.facing.X, s.facing.Z) * 180 / math.Pi
		gfx.put(gfx.signBox, base.ToWorld(s.at), vec3{s.width, signHeight, 0.03}, vec3{0, 1, 0}, yaw, color.RGBA{255, 255, 255, 255}, 0.2, 0.25)
	}
	gfx.setMat(content.MtPlain, 1, false)
}

// paintScreen draws a fake computer display in flat colors: a header,
// rows of readouts and a filled chart, in the accent color.
func paintScreen(seed int, accent color.RGBA) rl.Texture2D {
	const w, h = 256, 160
	dim := color.RGBA{accent.R / 3, accent.G / 3, accent.B / 3, 255}
	img := rl.GenImageColor(w, h, color.RGBA{10, 20, 30, 255})
	rl.ImageDrawRectangle(img, 0, 0, w, 20, dim)
	rl.ImageDrawRectangle(img, 0, 0, 8, 20, accent)
	rl.ImageDrawRectangle(img, 16, 7, 56, 6, accent)
	for i := range 6 {
		y := int32(32 + i*14)
		length := int32(30 + mathx.Hash2(seed, i, 91)*90)
		rl.ImageDrawRectangle(img, 10, y, length, 6, dim)
		if mathx.Hash2(seed, i, 92) > 0.55 {
			rl.ImageDrawRectangle(img, 112, y, 22, 6, accent)
		}
	}
	// A filled chart in the right part.
	for x := int32(0); x < 96; x += 6 {
		top := int32(140 - 60*tnoise(float64(x)/24, float64(seed), 64, 93))
		rl.ImageDrawRectangle(img, 150+x, top, 5, 146-top, dim)
		rl.ImageDrawRectangle(img, 150+x, top, 5, 3, accent)
	}
	rl.ImageFlipVertical(img)
	tex := rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.GenTextureMipmaps(&tex)
	rl.SetTextureFilter(tex, rl.FilterTrilinear)
	return tex
}

// drawScreen shows a display texture on a thin panel facing the given way.
func (gfx *Graphics) drawScreen(at, facing vec3, w, h float64, tex rl.Texture2D, glow float32) {
	gfx.setMat(content.MtDecal, 1, false)
	rl.SetMaterialTexture(&gfx.signBox.GetMaterials()[0], rl.MapAlbedo, tex)
	yaw := math.Atan2(facing.X, facing.Z) * 180 / math.Pi
	gfx.put(gfx.signBox, base.ToWorld(at), vec3{w, h, 0.03}, vec3{0, 1, 0}, yaw, color.RGBA{255, 255, 255, 255}, 0, glow)
	gfx.setMat(content.MtPlain, 1, false)
}
