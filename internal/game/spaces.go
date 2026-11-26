package game

import (
	"naturalia/internal/base"
	"naturalia/internal/engine/mathx"
)

// A space is the kind of place an eye is in: out on the ground, in a room,
// in a tunnel's gallery, later in a cave. Each tells what is seen from it,
// how it is lit, how the eye adapts, the haze, the wind and the echo: the
// drawing and the sound ask it, rather than each finding out on its own.

// SpaceKind is what kind of place it is.
type SpaceKind int

const (
	SpaceOut     SpaceKind = iota // on the ground, under the sky
	SpaceIndoors                  // in a room of the base, a bootcamp, a link's shelter
	SpaceGallery                  // in a tunnel's gallery, shut in
	SpaceCave                     // in a cave (docs/SOUTERRAIN.md): none yet
)

// Space is where an eye is, and what goes with it.
type Space struct {
	Kind SpaceKind
	// Hollow is the gallery the eye is shut in: nothing of the outside is
	// seen, only it. Nil anywhere else.
	Hollow *base.Link
	// Indoors: in a room, a shelter or a gallery. The eye adapts as in a
	// room; no wind; steps sound on a floor.
	Indoors bool
	// Depth is how far under the ground, from 0 at the surface to 1 three
	// meters down: the light is the same at any hour, the haze gone.
	Depth float64
	// Deep: under the ground, in a hollow. Sounds come back, muffled.
	Deep bool
}

// SpaceAt is the space of an eye at a point of the world.
func (g *Game) SpaceAt(eye vec3) Space {
	s := Space{Kind: SpaceOut, Indoors: g.Indoors(eye)}
	if s.Indoors {
		s.Kind = SpaceIndoors
	}
	if g.Under(eye) {
		down := -g.World.Relief.Altitude(eye)
		s.Depth = mathx.Smoothstep(0.5, 3, down)
		s.Deep = down > 0.3
	}
	for _, l := range g.LinksNear(eye, 5) {
		if l.Layout.InGallery(eye) {
			s.Kind, s.Hollow = SpaceGallery, l.Layout
		}
	}
	return s
}

// Open tells whether the outside may be seen from the space: the relief,
// the sky, the sun and its shadows, what stands on the ground.
func (s Space) Open() bool { return s.Hollow == nil }

// Light is the sun's height, 1 overhead and 0 on the horizon, as the space
// lights things: under the ground, the same at any hour.
func (s Space) Light(sunUp float64) float64 { return sunUp + (1-sunUp)*s.Depth }

// Haze is how much of the outside's haze is left: none under the ground.
func (s Space) Haze() float64 { return max(0, 1-s.Depth) }

// Wind tells whether the wind is heard.
func (s Space) Wind() bool { return !s.Indoors }

// Echo tells whether the space gives sounds back.
func (s Space) Echo() bool { return s.Deep }
