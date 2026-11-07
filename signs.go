package main

import (
	"image/color"
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

func baseSigns() []sign {
	inHub := func(angle float64, text string, accent color.RGBA) sign {
		dir := vec3{math.Cos(angle), 0, math.Sin(angle)}
		at := dir.scale(hubRoom.radius - wallThick/2 - 0.03).add(vec3{0, 2.6, 0})
		return sign{at: at, facing: dir.scale(-1), width: 1.9, text: text, accent: accent}
	}
	return []sign{
		inHub(west, "VENTE", accentOrange),
		inHub(east, "RECHERCHE", accentBlue),
		inHub(north, "QUARTIERS", accentPurple),
		inHub(south, "SAS 01 - SORTIE", colGold),
		{at: airlockEnd.add(vec3{0, 3.05, -0.2}), facing: vec3{0, 0, -1}, width: 2.6, text: "BASE MARTIENNE - SAS 01", accent: colGold},
	}
}

// paintSign draws a sign's texture: dark plate, accent stripe, worn edges
// and the text in small capitals.
func paintSign(s sign, font rl.Font) rl.Texture2D {
	const h = 128
	w := int(math.Round(s.width / signHeight * h))
	img := rl.GenImageColor(w, h, color.RGBA{34, 37, 46, 255})
	// Painted wear: lighter specks near the edges.
	for i := range w * h / 30 {
		x, y := int(hash2(i, 0, 77)*float64(w)), int(hash2(i, 1, 77)*float64(h))
		edge := math.Min(math.Min(float64(x), float64(w-x)), math.Min(float64(y), float64(h-y)))
		if edge < 14 || hash2(i, 2, 77) > 0.85 {
			rl.ImageDrawPixel(img, int32(x), int32(y), color.RGBA{70, 74, 86, 255})
		}
	}
	rl.ImageDrawRectangleLines(img, rl.NewRectangle(6, 6, float32(w-12), h-12), 3, color.RGBA{90, 95, 110, 255})
	rl.ImageDrawRectangle(img, 14, h-26, int32(w-28), 8, s.accent)
	size := float32(64)
	tw := rl.MeasureTextEx(font, s.text, size, 2).X
	if tw > float32(w-40) {
		size *= float32(w-40) / tw
		tw = rl.MeasureTextEx(font, s.text, size, 2).X
	}
	rl.ImageDrawTextEx(img, rl.NewVector2((float32(w)-tw)/2, (h-26-size)/2+4), font, s.text, size, 2, color.RGBA{236, 232, 220, 255})
	// The cube's front face shows the image upside down.
	rl.ImageFlipVertical(img)
	tex := rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.GenTextureMipmaps(&tex)
	rl.SetTextureFilter(tex, rl.FilterTrilinear)
	return tex
}

func (g *Game) drawSigns() {
	gfx := g.gfx
	gfx.setMat(mtDecal, 1, false)
	for _, s := range gfx.signs {
		rl.SetMaterialTexture(&gfx.signBox.GetMaterials()[0], rl.MapAlbedo, s.tex)
		if indoors(s.at.sub(s.facing.scale(0.3))) {
			gfx.setLighting(litIndoor, vec3{})
		} else {
			gfx.setLighting(litSun, vec3{})
		}
		yaw := math.Atan2(s.facing.X, s.facing.Z) * 180 / math.Pi
		gfx.put(gfx.signBox, toWorld(s.at), vec3{s.width, signHeight, 0.03}, vec3{0, 1, 0}, yaw, color.RGBA{255, 255, 255, 255}, 0.2, 0.25)
	}
	gfx.setMat(mtPlain, 1, false)
}

// paintScreen draws a fake computer display: a header, lines of text-like
// bars and a small chart, in the accent color.
func paintScreen(seed int, accent color.RGBA) rl.Texture2D {
	const w, h = 256, 160
	dim := color.RGBA{accent.R / 3, accent.G / 3, accent.B / 3, 255}
	img := rl.GenImageColor(w, h, color.RGBA{8, 14, 24, 255})
	rl.ImageDrawRectangle(img, 0, 0, w, 18, dim)
	rl.ImageDrawRectangle(img, 8, 6, 60, 6, accent)
	for i := range 7 {
		y := int32(28 + i*12)
		length := int32(30 + hash2(seed, i, 91)*110)
		rl.ImageDrawRectangle(img, 10, y, length, 5, dim)
		if hash2(seed, i, 92) > 0.6 {
			rl.ImageDrawRectangle(img, 14+length, y, 24, 5, accent)
		}
	}
	// A little line chart in the right part.
	prev := int32(0)
	for x := int32(0); x < 90; x += 3 {
		y := int32(120 - 50*tnoise(float64(x)/20, float64(seed), 64, 93))
		if x > 0 {
			rl.ImageDrawLine(img, 155+x-3, prev, 155+x, y, accent)
		}
		prev = y
	}
	rl.ImageDrawRectangleLines(img, rl.NewRectangle(150, 60, 100, 90), 1, dim)
	rl.ImageFlipVertical(img)
	tex := rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.GenTextureMipmaps(&tex)
	rl.SetTextureFilter(tex, rl.FilterTrilinear)
	return tex
}

// drawScreen shows a display texture on a thin panel facing the given way.
func (gfx *Graphics) drawScreen(at, facing vec3, w, h float64, tex rl.Texture2D, glow float32) {
	gfx.setMat(mtDecal, 1, false)
	rl.SetMaterialTexture(&gfx.signBox.GetMaterials()[0], rl.MapAlbedo, tex)
	yaw := math.Atan2(facing.X, facing.Z) * 180 / math.Pi
	gfx.put(gfx.signBox, toWorld(at), vec3{w, h, 0.03}, vec3{0, 1, 0}, yaw, color.RGBA{255, 255, 255, 255}, 0, glow)
	gfx.setMat(mtPlain, 1, false)
}
