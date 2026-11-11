package render

import (
	"math"
	"naturalia/internal/mathx"
	"runtime"
	"sync"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Every texture of the game is painted here, in code, at startup. They tile
// seamlessly and pack several layers in their RGBA channels; the shaders
// mix them with the object colors. A light posterization and stroke-like
// noise give them a hand-painted feel.

// tnoise is value noise that repeats every period lattice cells.
func tnoise(x, y float64, period int, seed uint64) float64 {
	ix, iy := math.Floor(x), math.Floor(y)
	fx, fy := x-ix, y-iy
	sx, sy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	wrap := func(v int) int { return ((v % period) + period) % period }
	x0, y0 := wrap(int(ix)), wrap(int(iy))
	x1, y1 := wrap(x0+1), wrap(y0+1)
	a, b := mathx.Hash2(x0, y0, seed), mathx.Hash2(x1, y0, seed)
	c, d := mathx.Hash2(x0, y1, seed), mathx.Hash2(x1, y1, seed)
	top, bottom := a+(b-a)*sx, c+(d-c)*sx
	return top + (bottom-top)*sy
}

// tfbm is tileable fractal noise; u, v in [0, 1) span one tile.
func tfbm(u, v float64, period, octaves int, seed uint64) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for o := range octaves {
		p := period << o
		sum += amp * tnoise(u*float64(p), v*float64(p), p, seed+uint64(o)*131)
		norm += amp
		amp *= 0.5
	}
	return sum / norm
}

// tnoiseXY is tileable value noise with its own period on each axis.
func tnoiseXY(x, y float64, px, py int, seed uint64) float64 {
	ix, iy := math.Floor(x), math.Floor(y)
	fx, fy := x-ix, y-iy
	sx, sy := fx*fx*(3-2*fx), fy*fy*(3-2*fy)
	x0, y0 := ((int(ix)%px)+px)%px, ((int(iy)%py)+py)%py
	x1, y1 := (x0+1)%px, (y0+1)%py
	a, b := mathx.Hash2(x0, y0, seed), mathx.Hash2(x1, y0, seed)
	c, d := mathx.Hash2(x0, y1, seed), mathx.Hash2(x1, y1, seed)
	top, bottom := a+(b-a)*sx, c+(d-c)*sx
	return top + (bottom-top)*sy
}

// stretched is tileable fractal noise whose features are stretch times
// longer along u than along v: brush strokes, brushed metal, streaks.
func stretched(u, v float64, period, stretch, octaves int, seed uint64) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for o := range octaves {
		px, py := period<<o, (period*stretch)<<o
		sum += amp * tnoiseXY(u*float64(px), v*float64(py), px, py, seed+uint64(o)*131)
		norm += amp
		amp *= 0.5
	}
	return sum / norm
}

// strokes is noise stretched along a diagonal: brush strokes. Mixing
// u+v and u-v keeps it tileable.
func strokes(u, v float64, period int, seed uint64) float64 {
	sum, amp, norm := 0.0, 1.0, 0.0
	for o := range 3 {
		px, py := period<<o, (period*5)<<o
		sum += amp * tnoiseXY((u+v)*float64(px), (u-v)*float64(py), px, py, seed+uint64(o)*131)
		norm += amp
		amp *= 0.5
	}
	return sum / norm
}

// warped is tileable fractal noise seen through a gentle swirl: organic
// paint daubs instead of grid-aligned blobs.
func warped(u, v float64, period, octaves int, seed uint64) float64 {
	wu := u + 0.04*(tfbm(u, v, 4, 3, seed+11)-0.5)
	wv := v + 0.04*(tfbm(u, v, 4, 3, seed+12)-0.5)
	return tfbm(wu, wv, period, octaves, seed)
}

// painterly pulls a value toward soft flat steps, like paint daubs.
func painterly(v float64, steps float64) float64 {
	k := v * steps
	f := k - math.Floor(k)
	stepped := (math.Floor(k) + mathx.Smoothstep(0.35, 0.65, f)) / steps
	return v*0.45 + stepped*0.55
}

// cellular scatters at most one round feature per cell of a tileable grid
// and calls fn with the distance to the nearest center (in cell units).
func cellular(u, v float64, cells int, seed uint64, fn func(d, r float64, h float64)) {
	x, y := u*float64(cells), v*float64(cells)
	cx, cy := int(math.Floor(x)), int(math.Floor(y))
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			gx, gy := cx+dx, cy+dy
			wx, wy := ((gx%cells)+cells)%cells, ((gy%cells)+cells)%cells
			px := float64(gx) + 0.15 + 0.7*mathx.Hash2(wx, wy, seed)
			py := float64(gy) + 0.15 + 0.7*mathx.Hash2(wx, wy, seed+1)
			fn(math.Hypot(x-px, y-py), mathx.Hash2(wx, wy, seed+2), mathx.Hash2(wx, wy, seed+3))
		}
	}
}

type texel struct{ r, g, b, a float64 } // all in [0, 1]

// bake renders a size×size texture in parallel, with mipmaps for smooth
// distant surfaces, repeating and trilinear filtered.
func bake(size int, fn func(u, v float64) texel) rl.Texture2D {
	pix := make([]byte, size*size*4)
	var wg sync.WaitGroup
	rows := make(chan int, size)
	for y := range size {
		rows <- y
	}
	close(rows)
	for range runtime.NumCPU() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for y := range rows {
				for x := range size {
					t := fn((float64(x)+0.5)/float64(size), (float64(y)+0.5)/float64(size))
					i := (y*size + x) * 4
					pix[i] = byte(mathx.Clamp01(t.r) * 255)
					pix[i+1] = byte(mathx.Clamp01(t.g) * 255)
					pix[i+2] = byte(mathx.Clamp01(t.b) * 255)
					pix[i+3] = byte(mathx.Clamp01(t.a) * 255)
				}
			}
		}()
	}
	wg.Wait()
	img := rl.GenImageColor(size, size, rl.White)
	tex := rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.UpdateTexture(tex, pix)
	rl.GenTextureMipmaps(&tex)
	// Anisotropic: floors and walls seen at a grazing angle stay sharp
	// without shimmering.
	rl.SetTextureFilter(tex, rl.FilterAnisotropic16x)
	rl.SetTextureWrap(tex, rl.WrapRepeat)
	return tex
}

// Regolith: R tone, G pebbles, B cavities (dark), A height.
// One tile covers regolithTile meters.
const regolithTile = 3.0

func genRegolith() rl.Texture2D {
	return bake(1024, func(u, v float64) texel {
		base := tfbm(u, v, 6, 5, 501)
		grain := tnoise(u*256, v*256, 256, 502)
		h := base*0.3 + grain*0.04

		// Stones of two sizes, plus a few big ones, with irregular outlines:
		// the lookup position wobbles a little.
		wu := u + 0.006*(tfbm(u, v, 24, 2, 503)-0.5)
		wv := v + 0.006*(tfbm(u, v, 24, 2, 504)-0.5)
		stone, stoneTone, stoneEdge := 0.0, 0.0, 0.0
		layer := func(cells int, chance, rMin, rMax, height float64, seed uint64) {
			cellular(wu, wv, cells, seed, func(d, r, tone float64) {
				if r > chance {
					return
				}
				rad := rMin + (rMax-rMin)*(r/chance)
				if d >= rad {
					return
				}
				x := d / rad
				dome := math.Sqrt(1-x*x) * height
				if dome > stone {
					stone, stoneTone, stoneEdge = dome, tone, mathx.Smoothstep(0.7, 1, x)
				}
			})
		}
		layer(34, 0.35, 0.10, 0.30, 0.35, 510)
		layer(12, 0.30, 0.12, 0.32, 0.6, 511)
		layer(4, 0.25, 0.15, 0.30, 0.9, 512)
		h += stone * 0.5

		crater := 0.0
		cellular(u, v, 5, 520, func(d, r, _ float64) {
			if r > 0.5 {
				return
			}
			rad := 0.18 + 0.25*r
			x := d / rad
			if x < 1.5 {
				bowl := math.Max(0, 1-x*x)
				lip := (x - 1) / 0.25
				crater += -bowl*bowl*0.5 + 0.18*math.Exp(-lip*lip)
			}
		})
		h += crater * 0.35

		tone := painterly(0.5+0.45*(tfbm(u, v, 4, 4, 530)-0.5)+0.25*(strokes(u, v, 8, 540)-0.5), 5)
		tone += (grain - 0.5) * 0.12
		if stone > 0 {
			// Painted stones: a lit top and a dark outline.
			tone = (0.3 + 0.35*stoneTone + stone*0.35) * (1 - 0.45*stoneEdge)
		}
		cavity := mathx.Clamp01(-crater*1.2) + mathx.Clamp01((0.4-base)*1.5)*0.4
		return texel{tone, math.Min(1, stone*5), cavity, mathx.Clamp01(h + 0.3)}
	})
}

// Dust: R tone, G clumps, A height. One tile covers dustTile meters.
const dustTile = 2.0

func genDust() rl.Texture2D {
	return bake(1024, func(u, v float64) texel {
		clumps := warped(u, v, 6, 4, 601)
		stroke := strokes(u, v, 4, 610)
		// Wind ripples: soft, wavy crests a few per tile, with a lit side
		// and a shaded side, fading in and out in patches.
		wave := v*5 + 0.35*(tfbm(u, v, 3, 3, 630)-0.5) + 0.08*math.Sin(u*2*math.Pi*2)
		ripple := math.Sin(wave * 2 * math.Pi)
		patch := mathx.Smoothstep(0.35, 0.65, tfbm(u, v, 2, 2, 640))
		tone := painterly(0.5+0.4*(clumps-0.5)+0.25*(stroke-0.5)+0.07*ripple*patch, 4)
		h := clumps*0.35 + stroke*0.2 + tfbm(u, v, 24, 2, 620)*0.08 + (0.5+0.5*ripple)*0.35*patch
		return texel{tone, clumps, 0, h}
	})
}

// Panel: one hull plate per tile, laid by the meshes' UVs so its edges
// follow the walls: a soft, shallow joint all around and a faint shade
// along the foot. Factory-clean: no rivets, no streaks.
// R tone, G grime, B joint (dark), A height.
func genPanel() rl.Texture2D {
	return bake(512, func(u, v float64) texel {
		e := math.Min(math.Min(u, 1-u), math.Min(v, 1-v)) // distance to the edge
		joint := 1 - mathx.Smoothstep(0.004, 0.02, e)
		h := 0.5 + 0.2*mathx.Smoothstep(0.004, 0.03, e)
		grime := mathx.Smoothstep(0.85, 1, v) * 0.5
		tone := 0.5 + 0.1*(tfbm(u, v, 2, 2, 740)-0.5)
		return texel{tone, grime, joint * 0.6, h}
	})
}

// Floor: diamond tread plate. R tone, G grime, A height. Tile = floorTile m.
const floorTile = 1.2

func genFloor() rl.Texture2D {
	return bake(512, func(u, v float64) texel {
		const n = 10
		x, y := u*n, v*n
		cx, cy := math.Mod(x, 1)-0.5, math.Mod(y, 1)-0.5
		if int(math.Floor(x)+math.Floor(y))%2 == 0 {
			cx, cy = cy, -cx // alternate the diamonds' direction
		}
		d := math.Abs(cx*0.9+cy*0.45) + math.Abs(cx*0.45-cy*0.9)*0.3
		bump := 1 - mathx.Smoothstep(0.12, 0.2, d)
		grime := mathx.Clamp01(tfbm(u, v, 4, 4, 801)*1.3 - 0.45)
		tone := painterly(0.5+0.15*(tfbm(u, v, 6, 3, 810)-0.5)+bump*0.12, 5)
		return texel{tone, grime, 0, 0.4 + bump*0.4}
	})
}

// Metal: brushed steel with scratches. R tone, G scratches, A height.
const metalTile = 1.0

func genMetal() rl.Texture2D {
	return bake(512, func(u, v float64) texel {
		brushed := stretched(v, u, 2, 40, 3, 901)
		// Scratches: thin contour lines of a stretched noise, here and there.
		line := stretched(u, v, 3, 6, 2, 910)
		scratch := (1 - mathx.Smoothstep(0.003, 0.009, math.Abs(line-0.5))) * mathx.Smoothstep(0.55, 0.7, tfbm(u, v, 4, 2, 920))
		tone := painterly(0.6+0.25*(brushed-0.5)+0.12*(tfbm(u, v, 3, 3, 930)-0.5), 6)
		return texel{tone + scratch*0.25, scratch, 0, 0.5 + 0.05*brushed - scratch*0.1}
	})
}

// Paint: tinted objects. R stroke tone, B chipped edges, A height
// (content.PaintTile tiles per meter).
func genPaint() rl.Texture2D {
	return bake(512, func(u, v float64) texel {
		tone := painterly(0.92+0.12*(strokes(u, v, 5, 1001)-0.5)+0.06*(tfbm(u, v, 12, 2, 1010)-0.5), 6)
		chips := mathx.Clamp01((tfbm(u, v, 16, 3, 1020) - 0.6) * 4)
		return texel{tone, 0, chips, 0.5 + 0.05*tfbm(u, v, 10, 3, 1030) - chips*0.08}
	})
}

// Fabric: a soft weave. R tone, A height (content.FabricTile tiles per
// meter).
func genFabric() rl.Texture2D {
	return bake(256, func(u, v float64) texel {
		const n = 24
		wx := math.Sin(u * n * 2 * math.Pi)
		wy := math.Sin(v * n * 2 * math.Pi)
		over := 0.5 + 0.5*wx*wy
		tone := painterly(0.85+0.1*over+0.08*(tfbm(u, v, 4, 3, 1101)-0.5), 6)
		return texel{tone, 0, 0, 0.4 + 0.3*over}
	})
}

// Hazard: diagonal yellow and black stripes, for the airlock.
func genHazard() rl.Texture2D {
	return bake(256, func(u, v float64) texel {
		stripe := mathx.Smoothstep(0.47, 0.53, math.Abs(math.Mod(u*4+v*4, 1)-0.5)*2)
		return texel{stripe, 1 - stripe, 0.6, 1}
	})
}

// Textures are all the painted textures, baked once at startup.
type Textures struct {
	regolith, dust                             rl.Texture2D
	panel, floor, metal, paint, fabric, hazard rl.Texture2D
}

func bakeTextures() Textures {
	return Textures{
		regolith: genRegolith(),
		dust:     genDust(),
		panel:    genPanel(),
		floor:    genFloor(),
		metal:    genMetal(),
		paint:    genPaint(),
		fabric:   genFabric(),
		hazard:   genHazard(),
	}
}

// GenVisor paints the helmet visor seen from inside, for a 16:9 screen:
// a thin dark rim hugging the corners, a faint glint along its edge and a
// soft reflection on the glass. It frames the view without eating it.
func GenVisor() rl.Texture2D {
	const w, h = 1280, 720
	pix := make([]byte, w*h*4)
	var wg sync.WaitGroup
	for y := range h {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for x := range w {
				nx := (float64(x)/w*2 - 1) * 1.02
				ny := (float64(y)/h*2 - 1) * 1.04
				// Superellipse: a rounded rectangle with soft corners.
				d := math.Pow(math.Pow(math.Abs(nx), 7)+math.Pow(math.Abs(ny), 5), 1.0/6)
				edge := mathx.Smoothstep(0.985, 1.045, d)
				vignette := mathx.Smoothstep(0.7, 1.0, d) * 0.1
				rim := math.Exp(-math.Pow((d-0.982)/0.006, 2)) * 0.05
				// Reflection: a faint curved band across the upper left.
				band := math.Exp(-math.Pow((nx*0.6+ny+0.95+0.25*nx*nx)/0.07, 2)) * 0.05 * mathx.Smoothstep(0.2, -0.6, nx)
				a := math.Min(1, edge*0.94+vignette)
				r, g, b := 0.03, 0.025, 0.035
				light := rim + band
				i := (y*w + x) * 4
				alpha := mathx.Clamp01(a + light)
				mix := func(c, l float64) byte {
					if alpha == 0 {
						return 0
					}
					return byte(mathx.Clamp01((c*a+l*light)/alpha) * 255)
				}
				pix[i], pix[i+1], pix[i+2], pix[i+3] = mix(r, 1), mix(g, 0.85), mix(b, 0.75), byte(alpha*255)
			}
		}()
	}
	wg.Wait()
	img := rl.GenImageColor(w, h, rl.Blank)
	tex := rl.LoadTextureFromImage(img)
	rl.UnloadImage(img)
	rl.UpdateTexture(tex, pix)
	rl.SetTextureFilter(tex, rl.FilterBilinear)
	return tex
}
