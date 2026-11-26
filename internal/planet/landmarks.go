package planet

import (
	"math"
	"naturalia/internal/engine/mathx"
)

// The three landmarks of Mars, placed around the base so that the volcano
// rises ahead when leaving the airlock (toward -Z), the polar cap shines
// behind the base and the canyon opens to the east.
var (
	volcanoCenter = Toward(vec3{-0.6, 0, -1}, 1750)
	capCenter     = Toward(vec3{0, 0, 1}, 760)
	canyonCenter  = Toward(vec3{1, 0, 0}, 1050)
	// The canyon follows the great circle through its center along this
	// direction: its axis, then the axis across it.
	canyonAlong  = TangentAt(canyonCenter, vec3{0.3, 0, 1})
	canyonNormal = canyonCenter.Cross(canyonAlong)
)

const (
	volcanoRadius  = 1500.0
	volcanoHeight  = 380.0
	calderaRadius  = 190.0
	calderaDepth   = 40.0
	capRadius      = 640.0
	capThickness   = 26.0
	canyonDepth    = 80.0
	canyonHalfW    = 260.0
	canyonFrom     = -1100.0 // along the axis, from its center
	canyonTo       = 1700.0
	canyonEndTaper = 500.0
)

// volcano: a wide shield with a caldera on top and lava flows running down
// its flanks. It also returns how much d is on it (0..1).
func volcano(seed uint64, d, p vec3) (h, on float64) {
	dist := ArcDist(d, volcanoCenter)
	if dist > volcanoRadius*1.15 {
		return 0, 0
	}
	warp := (mathx.Fbm3(p.Scale(1.0/450), seed+81, 2) - 0.5) * 140
	x := (dist + warp) / volcanoRadius
	on = mathx.Smoothstep(1.05, 0.6, x)
	if x >= 1 {
		return 0, on
	}
	bump := 1 - x*x
	h = volcanoHeight * bump * bump

	// Lava flows: radial ridges, strongest mid-flank.
	t1, t2 := mathx.TangentBasis(volcanoCenter)
	angle := math.Atan2(d.Dot(t2), d.Dot(t1))
	flows := math.Sin(angle*28+6*mathx.Fbm3(p.Scale(1.0/300), seed+82, 2)) * 0.5
	flows += math.Sin(angle*61+4*mathx.Fbm3(p.Scale(1.0/200), seed+83, 2)) * 0.25
	h += 4 * flows * mathx.Smoothstep(0.3, 0.5, x) * (1 - mathx.Smoothstep(0.75, 1, x))

	// Caldera: a flat-floored pit, plus a smaller one nested off-center.
	h -= calderaDepth * (1 - mathx.Smoothstep(calderaRadius*0.75, calderaRadius*1.2, dist))
	inner := ArcDist(d, Toward2(volcanoCenter, t1, 60))
	h -= 20 * (1 - mathx.Smoothstep(45, 95, inner))
	return h, on
}

// canyon: a long winding trench with a flat floor and walls in two steps.
// It also returns how much d is in it (0..1).
func canyon(seed uint64, d, p vec3) (h, in float64) {
	along := math.Atan2(d.Dot(canyonAlong), d.Dot(canyonCenter)) * Radius
	if along < canyonFrom-200 || along > canyonTo+200 {
		return 0, 0
	}
	across := math.Asin(math.Max(-1, math.Min(1, d.Dot(canyonNormal)))) * Radius
	if math.Abs(across) > canyonHalfW*2.2 {
		return 0, 0
	}
	taper := mathx.Smoothstep(canyonFrom, canyonFrom+canyonEndTaper, along) * (1 - mathx.Smoothstep(canyonTo-canyonEndTaper, canyonTo, along))
	center := 70*math.Sin(along/380) + 80*(mathx.Fbm3(p.Scale(1.0/900), seed+91, 2)-0.5)
	width := canyonHalfW * (0.85 + 0.3*mathx.Fbm3(p.Scale(1.0/600), seed+92, 2)) * (0.5 + 0.5*taper)
	x := (math.Abs(across-center) + 30*(mathx.Fbm3(p.Scale(1.0/120), seed+93, 2)-0.5)) / width
	depth := 0.5*(1-mathx.Smoothstep(0.35, 0.65, x)) + 0.5*(1-mathx.Smoothstep(0.7, 1, x))
	return -canyonDepth * taper * depth, mathx.Smoothstep(1.4, 1, x) * taper
}

// capInside is how far inside the polar cap's ragged edge d is, in meters
// (negative outside).
func capInside(seed uint64, d vec3) float64 {
	dist := ArcDist(d, capCenter)
	if dist > capRadius*1.3 {
		return -dist
	}
	return capRadius + 70*(mathx.Fbm3(d.Scale(Radius/260), seed+101, 2)-0.5) - dist
}

// Ice is how much of the polar cap covers direction d (0..1). The white
// starts right at the foot of the ice cliff.
func (r Relief) Ice(d vec3) float64 {
	if r.set.IceCap == 0 {
		return 0
	}
	return mathx.Smoothstep(-12, 12, capInside(r.seed, d))
}

// iceCap: a low dome of ice, carved by spiral troughs like the real one.
func iceCap(s Spot) float64 {
	if s.Ice <= 0 {
		return 0
	}
	dist := ArcDist(s.Dir, capCenter)
	h := capThickness * mathx.Smoothstep(-10, 70, capInside(s.Seed, s.Dir)) * (0.6 + 0.4*(1-math.Pow(dist/capRadius, 2)))
	t1, t2 := mathx.TangentBasis(capCenter)
	angle := math.Atan2(s.Dir.Dot(t2), s.Dir.Dot(t1))
	x := angle/(2*math.Pi)*3 + dist/250 + 0.3*mathx.Fbm3(s.P.Scale(1.0/150), s.Seed+102, 2)
	f := x - math.Floor(x)
	trough := mathx.Smoothstep(0, 0.15, f) * (1 - mathx.Smoothstep(0.25, 0.4, f))
	return h - 4*trough*s.Ice*mathx.Smoothstep(80, 200, dist)
}

// Basalt is how much of the volcano's dark rock shows at d (0..1).
func (r Relief) Basalt(d vec3) float64 {
	if r.set.Volcano == 0 {
		return 0
	}
	dist := ArcDist(d, volcanoCenter)
	return mathx.Smoothstep(volcanoRadius*1.05, volcanoRadius*0.55, dist)
}

// landmarkClear says whether a feature of the given radius at dir stays
// away from the base and from the three landmarks.
func landmarkClear(dir vec3, radius float64) bool {
	if ArcDist(dir, BaseUp) < radius+plainRadius*2 ||
		ArcDist(dir, volcanoCenter) < radius+volcanoRadius+100 ||
		ArcDist(dir, capCenter) < radius+capRadius+100 {
		return false
	}
	along := math.Atan2(dir.Dot(canyonAlong), dir.Dot(canyonCenter)) * Radius
	across := math.Abs(math.Asin(math.Max(-1, math.Min(1, dir.Dot(canyonNormal)))) * Radius)
	return along < canyonFrom-radius-100 || along > canyonTo+radius+100 || across > radius+canyonHalfW*1.6
}
