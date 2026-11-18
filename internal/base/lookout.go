package base

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/mesh"
)

// The lookout up on the great hall's mezzanine, behind the bay window: a
// bench, a low table, and a telescope on its tripod looking east.

// BuildBench: a white block on a recessed plinth under a slate cushion,
// along Z.
func BuildBench(b *building, at vec3, _ int) {
	b.in("home.trim").Box(at.Add(vec3{0, 0.04, 0}), vec3{0.36, 0.08, 1.5}, xAxis, 0.01)
	b.in("home.panel2").Box(at.Add(vec3{0, 0.23, 0}), vec3{0.48, 0.3, 1.64}, xAxis, 0.04)
	b.in("fabric.slate").Box(at.Add(vec3{0, 0.43, 0}), vec3{0.46, 0.1, 1.6}, xAxis, 0.04)
}

// BuildLowTable: a white top on an anthracite base.
func BuildLowTable(b *building, at vec3, _ int) {
	b.in("home.trim").Box(at.Add(vec3{0, 0.2, 0}), vec3{0.5, 0.4, 0.3}, xAxis, 0.02)
	b.in("home.panel").Box(at.Add(vec3{0, 0.42, 0}), vec3{0.9, 0.04, 0.6}, xAxis, 0.015)
}

// The telescope's tube, along the kit's +X: tipped up, its eyepiece at
// the back.
const (
	scopeTilt = 6 * math.Pi / 180
	scopeHead = 1.32 // its pivot over the floor
)

// BuildTelescope: a tripod, a mount, the tube looking east and up a
// little, a finder on top, the eyepiece at the back.
func BuildTelescope(b *building, at vec3, _ int) {
	steel, dark := b.Part("steel", mesh.LitIndoor, vec3{}), b.Part("steel.dark", mesh.LitIndoor, vec3{})
	head := at.Add(vec3{0, scopeHead, 0})
	for k := range 3 {
		a := float64(k)*2*math.Pi/3 + math.Pi/6
		foot := at.Add(vec3{math.Cos(a) * 0.5, 0, math.Sin(a) * 0.5})
		d := head.Sub(foot)
		dark.Prism(foot, d.Norm(), 0.025, 0.02, d.Length()-0.1, 6, 0)
		b.Part("rubber", mesh.LitIndoor, vec3{}).Prism(foot, yUp, 0.04, 0.04, 0.04, 8, 0)
	}
	dark.Prism(head.Sub(vec3{0, 0.12, 0}), yUp, 0.07, 0.06, 0.14, 12, 0.01)
	axis := vec3{math.Cos(scopeTilt), math.Sin(scopeTilt), 0}
	back := head.Add(vec3{0, 0.08, 0}).Sub(axis.Scale(0.45))
	b.Part("habitat.white", mesh.LitIndoor, vec3{}).Prism(back, axis, 0.075, 0.095, 1.15, 20, 0.01)
	steel.Prism(back.Add(axis.Scale(1.1)), axis, 0.11, 0.11, 0.16, 20, 0.01) // the dew shield
	b.Part("glass.dark", mesh.LitIndoor, vec3{}).Prism(back.Add(axis.Scale(1.255)), axis, 0.1, 0.1, 0.01, 20, 0)
	b.Part("pipe.yellow", mesh.LitIndoor, vec3{}).Prism(back.Add(axis.Scale(0.5)), axis, 0.1, 0.1, 0.05, 20, 0)
	steel.Prism(back.Add(axis.Scale(0.35)).Add(vec3{0, 0.13, 0}), axis, 0.025, 0.025, 0.3, 8, 0) // the finder
	dark.Prism(back, axis.Scale(-1), 0.03, 0.022, 0.12, 10, 0)                                   // the eyepiece
}

// TelescopeEye is where the view through a telescope standing at local
// point at, turned by yaw (degrees, as a prop's), starts: just past the
// end of its tube, which would fill it; and the way it looks, level.
func TelescopeEye(at vec3, yaw float64) (eye, facing vec3) {
	axis := vec3{math.Cos(scopeTilt), math.Sin(scopeTilt), 0}
	front := vec3{0, scopeHead + 0.08, 0}.Add(axis.Scale(1.15 - 0.45 + 0.15))
	return at.Add(content.Turn(front, yaw)), content.Turn(vec3{1, 0, 0}, yaw)
}
