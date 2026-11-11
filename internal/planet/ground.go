package planet

import "math"

// Queries on the ground: where it is, how high above it a point stands,
// where a ray meets it.

// SunDir points toward the sun: low on the horizon for long soft shadows.
// It stays put: the sun never moves across the sky.
var SunDir = vec3{-0.5, 0.62, -0.6}.Norm()

func (t Relief) SurfaceRadius(d vec3) float64 { return Radius + t.Height(d) }

// Surface returns the ground point in direction d (any length).
func (t Relief) Surface(d vec3) vec3 {
	d = d.Norm()
	return d.Scale(t.SurfaceRadius(d))
}

// Altitude is the height of p above the ground right below it.
func (t Relief) Altitude(p vec3) float64 { return p.Length() - t.SurfaceRadius(p.Norm()) }

// Raycast marches along the ray and returns the first ground hit within
// maxDist. Steps grow with the height above ground (slopes stay gentle).
func (t Relief) Raycast(from, dir vec3, maxDist float64) (vec3, bool) {
	const minStep = 0.1
	prev := from
	for d := minStep; d <= maxDist; {
		p := from.Add(dir.Scale(d))
		alt := t.Altitude(p)
		if alt > 0 {
			prev = p
			d += math.Max(minStep, alt*0.5)
			continue
		}
		// Refine between the last point above ground and this one.
		lo, hi := prev, p
		for range 10 {
			mid := lo.Lerp(hi, 0.5)
			if t.Altitude(mid) <= 0 {
				hi = mid
			} else {
				lo = mid
			}
		}
		return hi, true
	}
	return vec3{}, false
}
