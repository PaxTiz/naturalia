package core

import (
	"math"
	"naturalia/internal/content"
)

// registerVehicles adds the vehicles, cheapest first: drones and robots
// that sweep on their own, then what the player drives, each one slower
// but wider than the last, the stunt buggy and the shuttle aside. Shapes
// face +X, y up; a flying one is centered on its hover height.
func registerVehicles(c *content.Content) {
	for _, v := range []content.Vehicle{
		{ID: "core:drone", Name: "Drone balayeur", Desc: "Un drone autonome qui te suit et nettoie autour de toi",
			Icon: "drone", Cost: 300, Growth: 1.9, Drive: "auto", AI: "follow",
			Speed: 5, Hover: 1.4, Sweep: 1.5, Rate: 0.6, Range: 25, Light: vec3{0, 0.15, 0}, Parts: droneParts()},
		{ID: "core:crawler", Name: "Robot rampeur", Desc: "Pose-le avec F : il nettoie seul à 30 m à la ronde, même sans toi",
			Icon: "robot", Cost: 6_000, Growth: 1.45, Drive: "auto", AI: "patrol",
			Speed: 2.5, Sweep: 3, Rate: 1.3, Range: 30, Radius: 0.6, Light: vec3{-0.4, 1.03, 0}, Parts: crawlerParts()},
		{ID: "core:rover", Name: "Rover balayeur", Desc: "Roule à 45 km/h et aspire la poussière sous ses roues ; saute (Espace) et accélère (Maj)",
			Cost: 15_000, Speed: 12.5, Turn: 1.3, Sweep: 5, Rate: 2.5, Hopper: 150, Seat: 2.3, Radius: 1.5,
			Jump: 5.5, Boost: 1.6, Air: 0.4, Flame: vec3{-1.5, 0.55, 0}, FlameR: 0.25, Parts: roverParts()},
		{ID: "core:buggy", Name: "Buggy cascadeur", Desc: "Léger et nerveux : double saut avec salto, gros boost, vue de derrière (V)",
			Cost: 25_000, Speed: 18, Turn: 1.9, Sweep: 2.5, Rate: 1.5, Hopper: 60, Seat: 1.5, Radius: 1.3,
			Jump: 7.5, Jumps: 2, Boost: 1.8, Fuel: 4, Air: 0.8, Gravity: 1.6, Bounce: 0.4, View: "chase",
			Flame: vec3{-1.35, 0.75, 0}, FlameR: 0.22, Parts: buggyParts()},
		{ID: "core:drone-heavy", Name: "Drone lourd", Desc: "Hexacoptère qui te suit de plus haut et balaie bien plus large",
			Icon: "drone", Cost: 40_000, Growth: 1.7, Drive: "auto", AI: "follow",
			Speed: 8, Hover: 3, Sweep: 4.5, Rate: 1.4, Range: 40, Light: vec3{0, 0.3, 0}, Parts: heavyDroneParts()},
		{ID: "core:hover", Name: "Glisseur", Desc: "Aéroglisseur à 80 km/h : il dérape dans les virages",
			Cost: 120_000, Speed: 22, Turn: 1.7, Grip: 1.2, Hover: 0.5, Sweep: 8, Rate: 3, Hopper: 800, Seat: 1.9, Radius: 1.9,
			Jump: 6.5, Jumps: 2, Boost: 1.7, Air: 0.6, Gravity: 1.4, Flame: vec3{-1.7, 0.6, 0}, FlameR: 0.4, Parts: hoverParts()},
		{ID: "core:tracker", Name: "Chenillard autonome", Desc: "Robot à chenilles à poser avec F : rase 90 m à la ronde",
			Icon: "robot", Cost: 350_000, Growth: 1.5, Drive: "auto", AI: "patrol",
			Speed: 4, Sweep: 9, Rate: 3, Range: 90, Radius: 1.6, Light: vec3{0.7, 1.9, 0.5}, Parts: trackerParts()},
		{ID: "core:harvester", Name: "Moissonneuse", Desc: "Une barre de coupe de 30 m : lente, mais rien ne lui échappe",
			Cost: 900_000, Speed: 8, Turn: 0.8, Sweep: 15, Rate: 5, Hopper: 6_000, Seat: 2.9, Radius: 2.8,
			Jump: 3, Boost: 1.5, Parts: harvesterParts()},
		{ID: "core:titan", Name: "Titan", Desc: "Aspirateur minier à chenilles : 50 m de large d'un seul passage",
			Cost: 12_000_000, Speed: 5.5, Turn: 0.5, Sweep: 26, Rate: 8, Hopper: 60_000, Seat: 4.6, Radius: 4.5,
			Boost: 1.4, Parts: titanParts()},
		// Mars is 21 km around: the shuttle goes round it in under half a
		// minute, high up.
		{ID: "core:shuttle", Name: "Navette", Desc: "Se conduit comme un rover : elle décolle en accélérant, monte avec la vitesse et file à 3 200 km/h ; le tour de Mars en 25 s",
			Icon: "shuttle", Cost: 2_500_000, Speed: 900, Turn: 0.9, Hover: 1.2, Seat: 2.0, Radius: 3.5, View: "chase", Chase: 16,
			Fly: &content.Flight{Ceiling: 1200, Climb: 150}, Boost: 1.5, Fuel: 6,
			Flame: vec3{-3.6, 1.2, 0}, FlameR: 0.5, Parts: shuttleParts()},
	} {
		c.Vehicles.Add(v.ID, v)
	}
}

// Shapes, all lit by the sun.

func box(tok string, at, size vec3, bevel float64) content.PartJSON {
	return content.PartJSON{Shape: "box", Token: tok, At: at, Size: size, Bevel: bevel, Lit: "outdoor"}
}

// boxYaw is a box turned by yaw degrees, its width (X) toward Z.
func boxYaw(tok string, at, size vec3, yaw float64) content.PartJSON {
	p := box(tok, at, size, 0)
	p.Yaw = yaw
	return p
}

// prism goes from at along axis (up if zero), radius r0 then r1.
func prism(tok string, at, axis vec3, r0, r1, h float64, sides int) content.PartJSON {
	return content.PartJSON{Shape: "prism", Token: tok, At: at, Axis: axis, R0: r0, R1: r1, H: h, Sides: sides, Lit: "outdoor"}
}

var alongZ = vec3{0, 0, 1}

// wheel is a tire centered on at, w wide, with its hub on the outer side.
func wheel(at vec3, r, w float64) []content.PartJSON {
	side := math.Copysign(1, at.Z)
	hub := at.Z + side*w/2
	if side < 0 {
		hub -= 0.02
	}
	tire := prism("rubber", vec3{at.X, at.Y, at.Z - w/2}, alongZ, r, r, w, 14)
	tire.Bevel = math.Min(0.05, r*0.1)
	return []content.PartJSON{tire, prism("steel", vec3{at.X, at.Y, hub}, alongZ, r*0.45, r*0.45, 0.02, 8)}
}

// rotor is a motor and its blades' disc at the end of an arm.
func rotor(at vec3, r float64) []content.PartJSON {
	return []content.PartJSON{
		prism("steel.dark", at, vec3{}, 0.05+r*0.1, 0.05+r*0.1, 0.08, 8),
		prism("steel.dark", at.Add(vec3{0, 0.085, 0}), vec3{}, r, r, 0.012, 16),
	}
}

func droneParts() []content.PartJSON {
	p := []content.PartJSON{
		prism("habitat.white", vec3{0, -0.07, 0}, vec3{}, 0.3, 0.22, 0.14, 10),
		prism("crate.ochre", vec3{0, 0.07, 0}, vec3{}, 0.2, 0.12, 0.06, 10),
		prism("steel.dark", vec3{0, -0.17, 0}, vec3{}, 0.08, 0.15, 0.1, 10),
	}
	for i := range 4 {
		a := math.Pi/4 + float64(i)*math.Pi/2
		end := vec3{math.Cos(a) * 0.45, 0, math.Sin(a) * 0.45}
		p = append(p, boxYaw("steel", vec3{end.X / 2, 0.02, end.Z / 2}, vec3{0.45, 0.04, 0.05}, a*180/math.Pi))
		p = append(p, rotor(end, 0.19)...)
	}
	return p
}

func heavyDroneParts() []content.PartJSON {
	p := []content.PartJSON{
		prism("habitat.white", vec3{0, -0.12, 0}, vec3{}, 0.5, 0.36, 0.24, 6),
		prism("neon.blue", vec3{0, 0.12, 0}, vec3{}, 0.37, 0.37, 0.03, 6),
		prism("steel.dark", vec3{0, 0.15, 0}, vec3{}, 0.34, 0.2, 0.1, 6),
		// The hopper hanging below.
		prism("crate.slate", vec3{0, -0.55, 0}, vec3{}, 0.25, 0.38, 0.4, 8),
		prism("steel", vec3{0, -0.62, 0}, vec3{}, 0.08, 0.08, 0.07, 8),
	}
	for i := range 6 {
		a := float64(i) * math.Pi / 3
		end := vec3{math.Cos(a) * 0.85, 0.05, math.Sin(a) * 0.85}
		p = append(p, boxYaw("steel", vec3{end.X / 2, 0.05, end.Z / 2}, vec3{0.85, 0.06, 0.08}, a*180/math.Pi))
		p = append(p, rotor(end, 0.3)...)
	}
	return p
}

func crawlerParts() []content.PartJSON {
	var p []content.PartJSON
	for _, x := range []float64{-0.45, 0, 0.45} {
		for _, z := range []float64{-0.38, 0.38} {
			p = append(p, wheel(vec3{x, 0.18, z}, 0.18, 0.12)...)
		}
	}
	brush := prism("hazard", vec3{0.7, 0.12, -0.45}, alongZ, 0.12, 0.12, 0.9, 10)
	return append(p,
		box("habitat.white", vec3{0, 0.38, 0}, vec3{1.2, 0.28, 0.62}, 0.06),
		box("hazard", vec3{0, 0.3, 0}, vec3{1.22, 0.06, 0.64}, 0),
		box("solar.blue", vec3{-0.05, 0.53, 0}, vec3{0.9, 0.02, 0.52}, 0),
		box("steel.dark", vec3{0.7, 0.25, 0}, vec3{0.3, 0.12, 1.0}, 0.03),
		brush,
		box("steel", vec3{0.5, 0.28, 0}, vec3{0.25, 0.06, 0.3}, 0),
		prism("steel", vec3{-0.4, 0.52, 0}, vec3{}, 0.02, 0.02, 0.4, 6),
		box("glass.dark", vec3{-0.4, 0.95, 0}, vec3{0.12, 0.1, 0.25}, 0.02),
	)
}

func roverParts() []content.PartJSON {
	var p []content.PartJSON
	for _, x := range []float64{-0.85, 0.85} {
		for _, z := range []float64{-0.8, 0.8} {
			p = append(p, wheel(vec3{x, 0.4, z}, 0.4, 0.3)...)
		}
	}
	return append(p,
		box("steel.dark", vec3{0, 0.55, 0}, vec3{2.3, 0.25, 1.3}, 0.04),
		box("habitat.white", vec3{-0.25, 0.9, 0}, vec3{1.7, 0.5, 1.4}, 0.08),
		box("hazard", vec3{-0.25, 0.72, 0}, vec3{1.72, 0.12, 1.42}, 0),
		box("habitat.white2", vec3{0.75, 1.15, 0}, vec3{0.7, 0.75, 1.2}, 0.08),
		box("glass.dark", vec3{1.1, 1.3, 0}, vec3{0.04, 0.4, 1.0}, 0),
		box("steel", vec3{1.3, 0.3, 0}, vec3{0.35, 0.2, 1.6}, 0.04),
		box("neon.cyan", vec3{1.45, 0.28, 0}, vec3{0.03, 0.05, 1.4}, 0),
		prism("steel", vec3{-0.8, 1.15, 0.45}, vec3{}, 0.02, 0.02, 1.1, 5),
		prism("led.green", vec3{-0.8, 2.25, 0.45}, vec3{}, 0.04, 0.04, 0.06, 6),
	)
}

func hoverParts() []content.PartJSON {
	p := []content.PartJSON{
		box("rubber", vec3{0, 0.18, 0}, vec3{3.4, 0.36, 2.0}, 0.15),
		box("neon.cyan", vec3{0, 0.03, 0}, vec3{3.0, 0.04, 1.7}, 0),
		box("habitat.white", vec3{-0.1, 0.5, 0}, vec3{3.0, 0.35, 1.8}, 0.12),
		box("habitat.white2", vec3{1.45, 0.45, 0}, vec3{0.5, 0.25, 1.4}, 0.1),
		box("hazard", vec3{-0.1, 0.68, 0}, vec3{3.02, 0.06, 1.82}, 0),
		box("habitat.white2", vec3{-0.35, 0.88, 0}, vec3{0.6, 0.35, 1.2}, 0.08),
		box("glass.dark", vec3{0.35, 0.9, 0}, vec3{1.1, 0.42, 1.0}, 0.15),
		box("habitat.white", vec3{-1.35, 1.55, 0}, vec3{0.5, 0.55, 0.06}, 0.02),
	}
	for _, z := range []float64{-0.5, 0.5} {
		duct := prism("steel.dark", vec3{-1.3, 1.1, z}, vec3{-1, 0, 0}, 0.42, 0.42, 0.35, 14)
		duct.Bevel = 0.03
		p = append(p, duct,
			prism("neon.orange", vec3{-1.65, 1.1, z}, vec3{-1, 0, 0}, 0.3, 0.3, 0.01, 14),
			prism("steel", vec3{-1.25, 1.1, z}, vec3{-1, 0, 0}, 0.12, 0.12, 0.45, 8),
			box("steel", vec3{-1.25, 0.75, z}, vec3{0.2, 0.4, 0.12}, 0))
	}
	return p
}

func trackerParts() []content.PartJSON {
	var p []content.PartJSON
	for _, z := range []float64{-0.85, 0.85} {
		p = append(p, box("rubber", vec3{0, 0.35, z}, vec3{2.6, 0.7, 0.5}, 0.15))
		for _, x := range []float64{-0.9, 0, 0.9} {
			p = append(p, prism("steel.dark", vec3{x, 0.35, z - 0.28}, alongZ, 0.25, 0.25, 0.56, 10))
		}
	}
	return append(p,
		box("barrel.blue", vec3{0, 0.95, 0}, vec3{2.2, 0.6, 1.4}, 0.1),
		box("hazard", vec3{0, 0.7, 0}, vec3{2.22, 0.1, 1.42}, 0),
		box("solar.blue", vec3{-0.2, 1.3, 0}, vec3{1.8, 0.04, 1.5}, 0),
		box("steel.dark", vec3{1.5, 0.35, 0}, vec3{0.5, 0.5, 2.4}, 0.05),
		box("neon.cyan", vec3{1.76, 0.5, 0}, vec3{0.03, 0.05, 2.2}, 0),
		box("steel", vec3{1.15, 0.75, 0}, vec3{0.3, 0.15, 0.8}, 0),
		prism("steel", vec3{0.7, 1.25, 0.5}, vec3{}, 0.03, 0.03, 0.6, 6),
		box("glass.dark", vec3{0.7, 1.82, 0.5}, vec3{0.14, 0.12, 0.3}, 0.02),
	)
}

func harvesterParts() []content.PartJSON {
	var p []content.PartJSON
	for _, z := range []float64{-1.35, 1.35} {
		p = append(p, wheel(vec3{-1.4, 0.95, z}, 0.95, 0.6)...)
		p = append(p, wheel(vec3{1.3, 0.7, z}, 0.7, 0.5)...)
	}
	p = append(p,
		box("steel.dark", vec3{0, 0.95, 0}, vec3{3.6, 0.4, 2.2}, 0.05),
		box("pipe.yellow", vec3{-0.9, 1.65, 0}, vec3{2.4, 1.1, 2.0}, 0.12),
		box("hazard", vec3{-0.9, 1.15, 0}, vec3{2.42, 0.12, 2.02}, 0),
		// The hopper, lying behind the cab.
		prism("habitat.white", vec3{-1.3, 2.75, -0.95}, alongZ, 0.7, 0.7, 1.9, 14),
		prism("steel", vec3{-0.5, 2.2, 0.75}, vec3{}, 0.08, 0.08, 1.1, 8),
		// The cab, around the driver's seat.
		box("pipe.yellow", vec3{0.1, 2.2, 0}, vec3{1.3, 0.25, 1.5}, 0.04),
		box("glass.dark", vec3{0.1, 2.78, 0}, vec3{1.0, 0.9, 1.3}, 0.08),
		box("habitat.white", vec3{0.1, 3.28, 0}, vec3{1.25, 0.12, 1.55}, 0.04),
		prism("neon.orange", vec3{0.1, 3.34, 0.55}, vec3{}, 0.08, 0.08, 0.12, 8),
		// The header, as wide as a road, with its drum.
		box("steel", vec3{1.9, 0.9, -0.8}, vec3{0.9, 0.15, 0.15}, 0),
		box("steel", vec3{1.9, 0.9, 0.8}, vec3{0.9, 0.15, 0.15}, 0),
		box("pipe.yellow", vec3{2.6, 0.6, 0}, vec3{0.9, 0.7, 6.4}, 0.08),
		prism("hazard", vec3{3.05, 0.4, -3.0}, alongZ, 0.32, 0.32, 6.0, 12),
		box("neon.orange", vec3{3.07, 0.88, 0}, vec3{0.04, 0.05, 6.0}, 0),
	)
	for _, z := range []float64{-3.25, 3.25} {
		p = append(p, box("steel.dark", vec3{2.7, 0.6, z}, vec3{1.1, 0.8, 0.08}, 0))
	}
	return p
}

func titanParts() []content.PartJSON {
	var p []content.PartJSON
	for _, z := range []float64{-2.4, 2.4} {
		p = append(p, box("rubber", vec3{0, 0.75, z}, vec3{8, 1.5, 1.3}, 0.3))
		for _, x := range []float64{-3, -1.5, 0, 1.5, 3} {
			p = append(p, prism("steel.dark", vec3{x, 0.75, z - 0.7}, alongZ, 0.55, 0.55, 1.4, 12))
		}
	}
	p = append(p,
		box("habitat.white", vec3{0, 2.1, 0}, vec3{7.2, 1.4, 3.8}, 0.2),
		box("hazard", vec3{0, 1.45, 0}, vec3{7.24, 0.18, 3.84}, 0),
		box("habitat.white2", vec3{-1.8, 3.3, 0}, vec3{3.4, 1.0, 3.4}, 0.15),
		box("solar.blue", vec3{-1.8, 3.83, 0}, vec3{3.0, 0.06, 3.0}, 0),
		// The tower, around the driver's seat.
		box("habitat.white", vec3{0.4, 3.3, 0}, vec3{2.0, 1.0, 2.4}, 0.1),
		box("glass.dark", vec3{0.4, 4.4, 0}, vec3{1.8, 1.2, 2.2}, 0.1),
		box("habitat.white", vec3{0.4, 5.06, 0}, vec3{2.1, 0.14, 2.5}, 0.05),
		// The suction hood, its duct and its glowing rim.
		prism("steel.dark", vec3{4.6, 0.1, 0}, vec3{}, 1.9, 0.9, 1.6, 16),
		prism("neon.cyan", vec3{4.6, 0.1, 0}, vec3{}, 1.93, 1.93, 0.08, 16),
		prism("pipe.yellow", vec3{4.6, 1.5, 0}, vec3{-0.6, 0.6, 0}, 0.55, 0.55, 1.6, 12),
	)
	for _, z := range []float64{-1.1, 1.1} {
		p = append(p,
			prism("steel", vec3{-3.0, 2.8, z}, vec3{}, 0.25, 0.25, 1.9, 10),
			prism("steel.dark", vec3{-3.0, 4.7, z}, vec3{}, 0.28, 0.28, 0.1, 10),
			prism("neon.orange", vec3{0.4, 5.13, z * 0.8}, vec3{}, 0.1, 0.1, 0.15, 8),
		)
	}
	return p
}

func buggyParts() []content.PartJSON {
	var p []content.PartJSON
	for _, x := range []float64{-0.8, 0.85} {
		for _, z := range []float64{-0.85, 0.85} {
			p = append(p, wheel(vec3{x, 0.45, z}, 0.45, 0.35)...)
			// Long-travel struts out to the wheels.
			p = append(p, box("steel", vec3{x, 0.6, z * 0.6}, vec3{0.12, 0.08, 0.6}, 0))
		}
	}
	p = append(p,
		box("steel.dark", vec3{0, 0.55, 0}, vec3{2.2, 0.2, 0.9}, 0.04),
		box("pipe.red", vec3{0.55, 0.75, 0}, vec3{1.1, 0.25, 0.95}, 0.08),
		box("hazard", vec3{1.15, 0.7, 0}, vec3{0.1, 0.2, 1.1}, 0),
		box("fabric.seat", vec3{-0.2, 0.9, 0}, vec3{0.5, 0.5, 0.5}, 0.06),
		// The engine behind the seat, its exhaust pointing back.
		box("steel", vec3{-0.85, 0.85, 0}, vec3{0.55, 0.45, 0.7}, 0.05),
		prism("steel.dark", vec3{-1.1, 0.75, 0}, vec3{-1, 0, 0}, 0.13, 0.17, 0.25, 8),
		prism("neon.orange", vec3{-1.35, 0.75, 0}, vec3{-1, 0, 0}, 0.12, 0.12, 0.01, 8),
		// The roll cage.
		box("pipe.yellow", vec3{-0.45, 1.55, 0}, vec3{0.07, 0.07, 0.9}, 0),
		box("pipe.yellow", vec3{0.25, 1.4, 0}, vec3{0.07, 0.07, 0.9}, 0),
		box("pipe.yellow", vec3{-0.1, 1.5, 0.45}, vec3{0.72, 0.07, 0.07}, 0),
		box("pipe.yellow", vec3{-0.1, 1.5, -0.45}, vec3{0.72, 0.07, 0.07}, 0),
		box("neon.cyan", vec3{1.11, 0.62, 0}, vec3{0.02, 0.05, 0.8}, 0),
	)
	for _, z := range []float64{-0.45, 0.45} {
		p = append(p,
			prism("pipe.yellow", vec3{-0.45, 0.65, z}, vec3{}, 0.035, 0.035, 0.9, 6),
			prism("pipe.yellow", vec3{0.25, 0.75, z}, vec3{}, 0.035, 0.035, 0.65, 6),
		)
	}
	return p
}

// shuttleParts: a small lifting-body ship with delta wings and two
// engines at the back, standing on skids; it hovers over them.
func shuttleParts() []content.PartJSON {
	p := []content.PartJSON{
		box("habitat.white", vec3{0, 1.3, 0}, vec3{6, 1.4, 2.1}, 0.25),
		prism("habitat.white2", vec3{3, 1.25, 0}, vec3{1, 0, 0}, 1.0, 0.2, 2.2, 10),
		box("glass.dark", vec3{2.5, 1.95, 0}, vec3{1.8, 0.4, 1.5}, 0.15),
		box("steel.dark", vec3{0.2, 0.6, 0}, vec3{6.2, 0.12, 2.0}, 0.04),
		box("hazard", vec3{0.6, 1.5, 0}, vec3{0.5, 1.42, 2.14}, 0),
		box("neon.cyan", vec3{-0.5, 2.02, 0}, vec3{3, 0.03, 0.2}, 0),
		// The tail fin.
		box("habitat.white", vec3{-2.4, 2.6, 0}, vec3{1.8, 1.6, 0.15}, 0.05),
		box("pipe.red", vec3{-2.85, 3.3, 0}, vec3{0.9, 0.25, 0.17}, 0),
	}
	for _, z := range []float64{-1, 1} {
		yaw := -25 * z
		p = append(p,
			// A delta wing, its glowing edge and a red tip.
			boxYaw("habitat.white2", vec3{-0.9, 0.95, z * 2.2}, vec3{3.4, 0.16, 2.6}, yaw),
			boxYaw("neon.cyan", vec3{-0.35, 0.95, z * 2.55}, vec3{3.0, 0.05, 0.05}, yaw),
			box("pipe.red", vec3{-2.2, 1.0, z * 3.45}, vec3{0.9, 0.2, 0.15}, 0),
			// An engine.
			prism("steel.dark", vec3{-2.9, 1.2, z * 0.6}, vec3{-1, 0, 0}, 0.45, 0.5, 0.7, 12),
			prism("neon.orange", vec3{-3.6, 1.2, z * 0.6}, vec3{-1, 0, 0}, 0.38, 0.38, 0.01, 12),
			// A skid and its struts.
			box("steel", vec3{0, 0.06, z * 0.9}, vec3{4.2, 0.12, 0.15}, 0.03),
			box("steel", vec3{1.2, 0.35, z * 0.9}, vec3{0.12, 0.5, 0.12}, 0),
			box("steel", vec3{-1.2, 0.35, z * 0.9}, vec3{0.12, 0.5, 0.12}, 0),
		)
	}
	return p
}
