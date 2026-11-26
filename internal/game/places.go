package game

import "naturalia/internal/planet"

// Where one may stand: on the relief, or in a hollow under it (the
// tunnels' galleries, later the caves). The relief knows one height a
// direction; a hollow has its own floors and ceilings. What can be under
// the ground asks Ground for its floor, rather than the relief; the rest
// (vehicles, whirlwinds, bootcamps, dust) keeps to the relief.

// Places are the relief, and the hollows under it.
func (g *Game) Places() planet.Places {
	return planet.Places{Relief: g.World.Relief, Burrow: g.burrow}
}

// Ground is the floor at p's place and at its height: a hollow's, or the
// relief's; under tells which.
func (g *Game) Ground(p vec3) (at vec3, under bool) { return g.Places().Ground(p) }

// foot is the ground an entity stands on, under what it flies or rests on
// (Alt): the floor of a hollow if it says so (Transform.Floor), or the
// relief.
func (g *Game) foot(e *Transform) vec3 {
	if e.Floor > 0 {
		return e.Dir.Norm().Scale(e.Floor)
	}
	return g.World.Relief.Surface(e.Dir)
}

// footRadius is foot's distance from Mars' center.
func (g *Game) footRadius(e *Transform) float64 {
	if e.Floor > 0 {
		return e.Floor
	}
	return g.World.Relief.SurfaceRadius(e.Dir)
}
