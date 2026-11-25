package render

import (
	"cmp"
	"image/color"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/game"
	"naturalia/internal/mathx"
	"naturalia/internal/mesh"
	"slices"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// The tunnels: their shelters stand on the ground, which is cut open over
// their stairs (terrain.frag); from their galleries nothing is seen of the
// outside, and nothing of it is drawn.

const (
	maxHoles    = 8    // in terrain.frag
	tunnelSight = 60.0 // m from its shelters, a tunnel's inside may be seen
	lampSight   = 75.0 // m: how far a tunnel's lamps light it
)

// lookUnder finds where the camera is, before anything is drawn: in a
// tunnel's gallery, or not.
func (gfx *Graphics) lookUnder(g *game.Game) {
	gfx.gallery = nil
	for _, l := range g.LinksNear(gfx.camEye, 5) {
		if l.Layout.InGallery(gfx.camEye) {
			gfx.gallery = l.Layout
		}
	}
	gfx.under = 0
	if g.Under(gfx.camEye) {
		gfx.under = mathx.Smoothstep(0.5, 3, -g.World.Relief.Altitude(gfx.camEye))
	}
}

// setHoles gives the terrain the holes over the closest tunnels' stairs.
func (gfx *Graphics) setHoles(g *game.Game) {
	type hole struct {
		s            base.Site
		x0, x1, half float64
		dist         float64
	}
	var list []hole
	for _, l := range g.LinksNear(gfx.camEye, campDrawDist) {
		l.Layout.Holes(func(s base.Site, x0, x1, half float64) {
			list = append(list, hole{s, x0, x1, half, s.Origin.Sub(gfx.camEye).Length()})
		})
	}
	slices.SortFunc(list, func(a, b hole) int { return cmp.Compare(a.dist, b.dist) })
	var at, x, z []float32
	gfx.holes = gfx.holes[:0]
	for _, h := range list[:min(len(list), maxHoles)] {
		gfx.holes = append(gfx.holes, h.s.Origin)
		o := h.s.Origin.Sub(gfx.origin)
		at = append(at, float32(o.X), float32(o.Y), float32(o.Z), float32(h.x0))
		x = append(x, float32(h.s.X.X), float32(h.s.X.Y), float32(h.s.X.Z), float32(h.x1))
		z = append(z, float32(h.s.Z.X), float32(h.s.Z.Y), float32(h.s.Z.Z), float32(h.half))
	}
	n := int32(len(at) / 4)
	rl.SetShaderValue(gfx.holedSh, rl.GetShaderLocation(gfx.holedSh, "holeCount"), []float32{float32(n)}, rl.ShaderUniformFloat)
	if n > 0 {
		rl.SetShaderValueV(gfx.holedSh, rl.GetShaderLocation(gfx.holedSh, "holeAt"), at, rl.ShaderUniformVec4, n)
		rl.SetShaderValueV(gfx.holedSh, rl.GetShaderLocation(gfx.holedSh, "holeX"), x, rl.ShaderUniformVec4, n)
		rl.SetShaderValueV(gfx.holedSh, rl.GetShaderLocation(gfx.holedSh, "holeZ"), z, rl.ShaderUniformVec4, n)
	}
}

// holed tells whether a patch of terrain may be open over a tunnel's stairs.
func (gfx *Graphics) holed(n *lodNode) bool {
	for _, h := range gfx.holes {
		if n.centerPos.Sub(h).Length() < n.arc*0.8+12 {
			return true
		}
	}
	return false
}

// tunnelLamps are the lamps of the tunnels that may be seen, the closest
// first, fading out before they go.
func (gfx *Graphics) tunnelLamps(g *game.Game, most int) []base.Lamp {
	type near struct {
		base.Lamp
		d float64
	}
	var list []near
	for _, l := range g.LinksNear(gfx.camEye, tunnelSight) {
		if !gfx.tunnelSeen(l.Layout) {
			continue
		}
		for _, lamp := range l.Layout.Lamps() {
			d := lamp.At.Sub(gfx.camEye).Length()
			if d < lampSight {
				list = append(list, near{lamp, d})
			}
		}
	}
	slices.SortFunc(list, func(a, b near) int { return cmp.Compare(a.d, b.d) })
	edge := lampSight
	if len(list) > most {
		edge = list[most].d
		list = list[:most]
	}
	var out []base.Lamp
	for _, n := range list {
		n.Power *= 1 - mathx.Smoothstep(edge*0.8, edge, n.d)
		out = append(out, n.Lamp)
	}
	return out
}

// tunnelSeen tells whether the inside of a tunnel may be seen.
func (gfx *Graphics) tunnelSeen(l *base.Link) bool {
	if !l.Under() || gfx.gallery != nil {
		return l == gfx.gallery
	}
	return l.Near(gfx.camEye) < tunnelSight || l.Indoors(l.Site.ToLocal(gfx.camEye))
}

// drawTunnel draws a tunnel's shelters, and its inside if it may be seen;
// its inside casts no shadow.
func (gfx *Graphics) drawTunnel(l *base.Link, seen sees) {
	if gfx.gallery == nil {
		for _, sh := range l.Shells() {
			if seen(sh.Site.Origin, 9) {
				gfx.wearAt(sh.Site)
				gfx.drawSited(gfx.keptModels(sh.Key, sh.Parts), sh.Site, false)
			}
		}
	}
	if !gfx.casting && gfx.tunnelSeen(l) {
		gfx.drawSited(gfx.linkModels(l), l.Site, false)
	}
}

// drawTunnelGhost draws a tunnel being dug: its shelters, and its way under
// the ground, through it.
func (gfx *Graphics) drawTunnelGhost(l *base.Link, col color.RGBA) {
	for _, sh := range l.Shells() {
		for _, md := range gfx.keptModels(sh.Key, sh.Parts).models {
			gfx.putBasis(md, sh.Site.Origin, sh.Site.X, sh.Site.Y, sh.Site.Z, col, 0, 0.22)
		}
	}
}

// drawTunnelWay draws the way of a tunnel being dug, as if the ground was
// not there.
func (gfx *Graphics) drawTunnelWay(l *base.Link, col color.RGBA) {
	rl.DrawRenderBatchActive()
	rl.DisableDepthTest()
	pts := l.Plan()
	c := color.RGBA{col.R, col.G, col.B, 255}
	if len(pts) > 1 {
		sh := l.Shells()
		pts = append(append([]vec3{sh[0].Site.ToWorld(vec3{1.2, 0, 0})}, pts...), sh[1].Site.ToWorld(vec3{1.2, 0, 0}))
	}
	for i := range len(pts) - 1 {
		a, b := pts[i], pts[i+1]
		side := b.Sub(a).Cross(a.Norm()).Norm().Scale(l.Def.Width / 2)
		for _, s := range []float64{-1, 1} {
			gfx.segment(gfx.cyl, a.Add(side.Scale(s)), b.Add(side.Scale(s)), 0.03, c, 0, 0.5)
		}
		gfx.segment(gfx.cyl, a.Sub(side), a.Add(side), 0.02, c, 0, 0.5)
	}
	rl.DrawRenderBatchActive()
	rl.EnableDepthTest()
}

// keptModels are meshes uploaded on first use, and kept under a key.
func (gfx *Graphics) keptModels(key string, parts func() *mesh.Parts) *baseModels {
	if gfx.links == nil {
		gfx.links = map[string]*baseModels{}
	}
	m := gfx.links[key]
	if m == nil {
		m = gfx.uploadBase(parts(), gfx.objectSh)
		for _, md := range m.models {
			gfx.bindMaps(md)
		}
		gfx.links[key] = m
	}
	return m
}

// hazeUnder is how much of the haze is left for the camera: none under the
// ground.
func (gfx *Graphics) hazeUnder() float32 { return float32(math.Max(0, 1-gfx.under)) }
