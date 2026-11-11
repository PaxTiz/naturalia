package game

const (
	MaxSteps   = 48   // footprints kept: a trail of about 35 m
	stepLength = 0.75 // meters between two prints
)

// Footprint is a boot print pressed in the dust, drawn by the terrain
// shader. The newest ones are the sharpest; the oldest fade out.
type Footprint struct {
	Pos, Dir vec3
}

// Footprints records the player's trail, a print left then right.
type Footprints struct {
	Ring [MaxSteps]Footprint
	N    int // prints made so far; the newest is at (n-1) % maxSteps
	last vec3
}

// update reports whether a new step was taken.
func (f *Footprints) update(p *Player) bool {
	if !p.onGround {
		return false
	}
	if f.N > 0 && p.Pos.Sub(f.last).Length() < stepLength {
		return false
	}
	f.last = p.Pos
	side := 0.12
	if f.N%2 == 1 {
		side = -side
	}
	f.Ring[f.N%MaxSteps] = Footprint{p.Pos.Add(p.Right().Scale(side)), p.Heading}
	f.N++
	return true
}
