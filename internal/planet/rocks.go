package planet

import (
	"math"
	"naturalia/internal/mathx"
)

// RockShapes is how many rock models there are to pick from.
const RockShapes = 5

// Rock is a boulder lying on the ground.
type Rock struct {
	Dir   vec3 // from Mars' center
	Size  float64
	Shape int
	Yaw   float64
	tone  float64
}

// rocksIn lists the rocks of a dust tile: placed procedurally, mostly small
// stones, now and then a big boulder. None around the base
// nor on the polar ice.
func (w *World) rocksIn(k TileKey) []Rock {
	f, tx, ty := k.Split()
	count := int(mathx.Hash3(f, tx, ty, w.Seed+201) * 8 * w.Relief.set.Rocks)
	var rocks []Rock
	for i := range count {
		s := w.Seed + 210 + uint64(i)*7
		ci := tx*TileCells + int(mathx.Hash3(f, tx, ty, s)*TileCells)
		cj := ty*TileCells + int(mathx.Hash3(f, tx, ty, s+1)*TileCells)
		size := 0.2 + 0.7*math.Pow(mathx.Hash3(f, tx, ty, s+2), 3)
		if mathx.Hash3(f, tx, ty, s+3) < 0.1 {
			size = 1.4 + 1.3*mathx.Hash3(f, tx, ty, s+4) // a big boulder
		}
		d := cellDir(f, ci, cj)
		if ArcDist(d, BaseUp) < padRadius+size+4 || w.Relief.Ice(d) > 0.3 {
			continue
		}
		rocks = append(rocks, Rock{d, size, int(mathx.Hash3(f, tx, ty, s+5) * RockShapes), mathx.Hash3(f, tx, ty, s+6) * 2 * math.Pi, mathx.Hash3(f, tx, ty, s+7)})
	}
	return rocks
}

// RocksNear lists the rocks within dist meters of c.
func (w *World) RocksNear(c vec3, dist float64) []Rock {
	c = c.Norm()
	var out []Rock
	TilesNear(c, dist, func(k TileKey) {
		for _, r := range w.rocksIn(k) {
			if r.Dir.Sub(c).Length()*Radius <= dist {
				out = append(out, r)
			}
		}
	})
	return out
}
