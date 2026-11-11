package game

// collideRocks keeps the player out of the bigger rocks.
func (g *Game) collideRocks() {
	for _, r := range g.NearRocks {
		if r.Size < 0.6 {
			continue
		}
		g.Player.pushOut(g.World.Relief.Surface(r.Dir), r.Size*0.85)
	}
}

const RockDrawDist = 150.0
