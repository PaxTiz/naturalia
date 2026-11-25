package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/physics"
	"naturalia/internal/planet"
)

// Tunnels (docs/BATIMENTS.md) are links under the ground (base.Link, its
// Under). The relief doesn't know of them: the player in one stands on its
// floor, under its vault, between its walls, which are far under what
// stands on the ground.

// Under tells whether a point of the world is in a tunnel: its shelters,
// its stairs, its gallery.
func (g *Game) Under(at vec3) bool {
	_, _, ok := g.burrow.At(at)
	return ok
}

// Deep tells whether a point of the world is in a tunnel, under the ground.
func (g *Game) Deep(at vec3) bool { return g.Under(at) && g.World.Relief.Altitude(at) < -0.3 }

// standPlayer finds what the walking player stands on, and what is over
// their head.
func (g *Game) standPlayer() {
	pl := &g.Player
	if f, c, ok := g.burrow.At(pl.Pos); ok {
		ground := g.World.Relief.SurfaceRadius(pl.Pos.Norm())
		pl.floor, pl.ceiling, pl.under = f-ground, c-ground, true
		return
	}
	pl.under = false
	pl.floor = g.floorUnder(pl.Pos)
	pl.ceiling = g.ceilingOver(pl.Pos)
}

// roofUnder is the top of a tunnel's shelter under feet at pos, as high
// over the ground, no higher than a step up: 0 for none.
func (g *Game) roofUnder(pos vec3) float64 {
	top := 0.0
	for _, l := range g.LinksNear(pos, 1) {
		if r, ok := l.Layout.Roof(pos); ok && r <= pos.Length()+stepUp {
			top = math.Max(top, r-g.World.Relief.SurfaceRadius(pos.Norm()))
		}
	}
	return top
}

// addSide adds a wall of a tunnel, between two radii.
func (g *Game) addSide(a, b vec3, low, high float64) {
	p := g.phys
	if planet.ArcDist(p.up, a.Norm()) > physReach+reanchor {
		return
	}
	pa, pb := p.plane(a), p.plane(b)
	mid := pa.Lerp(pb, 0.5)
	p.w.Add(&physics.Body{Kind: physics.Static, Shape: physics.Capsule(pa.Sub(mid), pb.Sub(mid), base.WallThick/2), Pos: mid,
		Bottom: low - p.r0, Top: high - p.r0, Layer: layerStatic, Mask: ^uint32(0), Friction: frictionWall, Restitution: bounceWall})
}

const (
	tunnelEarth = 1.5 // m of ground over a vault at the least, as the ground is now
	tunnelOver  = 1.5 // m between two tunnels, one over the other
)

// tunnelFits is why a tunnel can't go under the ground there, "" if it can.
func (g *Game) tunnelFits(l *base.Link) string {
	why := ""
	l.Buried(func(at vec3, want float64) {
		if -g.World.Relief.Altitude(at) < math.Min(want, tunnelEarth) {
			why = "pas assez de terre au-dessus"
		}
	})
	if why != "" {
		return why
	}
	var mine []vec3
	l.Deep(func(at vec3) { mine = append(mine, at) })
	for _, o := range g.links {
		if !o.Layout.Under() || planet.ArcDist(o.Layout.Site.Origin.Norm(), l.Site.Origin.Norm()) > (o.Layout.Length+l.Length)/2+20 {
			continue
		}
		side := (o.Layout.Def.Width+l.Def.Width)/2 + 1
		o.Layout.Deep(func(q vec3) {
			for _, p := range mine {
				d := p.Sub(q)
				up := d.Dot(q.Norm())
				high := l.Def.Height
				if up < 0 {
					high = o.Layout.Def.Height
				}
				if math.Abs(up) < high+tunnelOver && d.Length() < math.Hypot(side, up) {
					why = "croise un tunnel"
					return
				}
			}
		})
		if why != "" {
			return why
		}
	}
	return ""
}

// campDigs tells whether a building's ground, flat out to a radius at an
// altitude, would come down to a tunnel.
func (g *Game) campDigs(dir vec3, radius, level float64) bool {
	bad := false
	for _, l := range g.LinksNear(dir, radius+planet.PadBlend) {
		l.Layout.Buried(func(at vec3, want float64) {
			d := at.Norm()
			if planet.ArcDist(d, dir) < radius+planet.PadBlend/2 && (planet.Radius+level)/d.Dot(dir)-at.Length() < math.Min(want, tunnelEarth)-0.1 {
				bad = true
			}
		})
	}
	return bad
}

// ShotTunnel is a point of the first tunnel's floor, that far along it, and
// the way there, for screenshots.
func (g *Game) ShotTunnel(along float64) (at, heading vec3, ok bool) {
	for _, l := range g.links {
		if !l.Layout.Under() {
			continue
		}
		var pts []vec3
		for _, p := range l.Layout.Pts {
			pts = append(pts, l.Layout.Site.ToWorld(p))
		}
		for i := range len(pts) - 1 {
			d := pts[i+1].Sub(pts[i])
			if long := d.Length(); along <= long || i == len(pts)-2 {
				at = pts[i].Add(d.Scale(math.Min(1, along/long)))
				return at, planet.TangentAt(at.Norm(), d), true
			} else {
				along -= long
			}
		}
	}
	return at, heading, false
}
