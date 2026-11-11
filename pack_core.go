package main

import "image/color"

// The base game, as a pack: it goes through the same API as a mod.
func init() {
	RegisterPack(Pack{
		ID:       "core",
		Name:     "Mars Sweeper",
		Version:  "1.0",
		Register: registerCore,
	})
}

func registerCore(c *Content) {
	for _, t := range []Tool{
		{ID: "core:toothbrush", Name: "Brosse à dents", Radius: 0.75, Reach: 3.5, Rate: 0.8, Cost: 0,
			Look: ToolLook{0.30, 0.014, color.RGBA{80, 150, 240, 255}, 0.06, 0.022, color.RGBA{245, 245, 250, 255}, headBlock, 0}},
		{ID: "core:paintbrush", Name: "Pinceau", Radius: 1.25, Reach: 4.5, Rate: 1.0, Cost: 60,
			Look: ToolLook{0.35, 0.016, color.RGBA{190, 70, 60, 255}, 0.10, 0.03, color.RGBA{215, 175, 115, 255}, headTaper, 0}},
		{ID: "core:broom", Name: "Balai", Radius: 2.25, Reach: 6, Rate: 1.4, Cost: 500,
			Look: ToolLook{0.55, 0.02, color.RGBA{165, 115, 70, 255}, 0.24, 0.12, color.RGBA{240, 205, 95, 255}, headFlare, 0}},
		{ID: "core:vacuum", Name: "Aspirateur", Radius: 3.75, Reach: 9, Rate: 2.2, Cost: 4_000,
			Look: ToolLook{0.40, 0.025, color.RGBA{170, 175, 190, 255}, 0.22, 0.07, color.RGBA{225, 75, 80, 255}, headNozzle, 0.6}},
		{ID: "core:blower", Name: "Souffleur indus.", Radius: 6, Reach: 13, Rate: 3.5, Cost: 35_000,
			Look: ToolLook{0.30, 0.05, color.RGBA{60, 64, 80, 255}, 0.30, 0.09, color.RGBA{240, 150, 55, 255}, headNozzle, 0.8}},
		{ID: "core:vacuum-cannon", Name: "Canon à vide", Radius: 10, Reach: 22, Rate: 6, Cost: 400_000,
			Look: ToolLook{0.35, 0.06, color.RGBA{230, 232, 240, 255}, 0.26, 0.12, color.RGBA{80, 130, 235, 255}, headNozzle, 1}},
	} {
		c.Tools.Add(t.ID, t)
	}

	for _, u := range []Upgrade{
		{ID: "core:tank", Name: "Réservoir", Desc: "Stocke 60% de poussière en plus avant de devoir rentrer vendre",
			Icon: "tank", Base: 25, Growth: 1.75, Effects: []Effect{{Stat: "tank", Mult: 1.6}}},
		{ID: "core:power", Name: "Turbo", Desc: "Outils et drones aspirent 30% plus vite",
			Icon: "power", Base: 40, Growth: 1.85, Effects: []Effect{{Stat: "power", Mult: 1.3}}},
		{ID: "core:price", Name: "Négociation", Desc: "La poussière et les cristaux se vendent 20% plus cher",
			Icon: "price", Base: 150, Growth: 2.1, Effects: []Effect{{Stat: "price", Mult: 1.2}}},
		{ID: "core:drone", Name: "Drone balayeur", Desc: "Un drone autonome qui nettoie à ta place",
			Icon: "drone", Base: 300, Growth: 1.9, Effects: []Effect{{Stat: "drones", Add: 1}}},
		{ID: "core:auto", Name: "Téléporteur", Desc: "Vend à distance dès que le réservoir est plein",
			Icon: "auto", Base: 1_500, Growth: 1, Max: 1, Effects: []Effect{{Stat: "autosell", Add: 1}}},
	} {
		c.Upgrades.Add(u.ID, u)
	}
}
