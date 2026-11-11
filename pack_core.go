package main

import (
	"image/color"
	"maps"
	"mars_sweeper/internal/base"
	"mars_sweeper/internal/content"
	"math"
	"slices"
)

// The base game, as a pack: it goes through the same API as a mod.
func init() {
	content.RegisterPack(content.Pack{
		ID:       "core",
		Name:     "Mars Sweeper",
		Version:  "1.0",
		Register: registerCore,
	})
}

func registerCore(c *content.Content) {
	for _, t := range []content.Tool{
		{ID: "core:toothbrush", Name: "Brosse à dents", Radius: 0.75, Reach: 3.5, Rate: 0.8, Cost: 0,
			Look: content.ToolLook{0.30, 0.014, color.RGBA{80, 150, 240, 255}, 0.06, 0.022, color.RGBA{245, 245, 250, 255}, content.HeadBlock, 0}},
		{ID: "core:paintbrush", Name: "Pinceau", Radius: 1.25, Reach: 4.5, Rate: 1.0, Cost: 60,
			Look: content.ToolLook{0.35, 0.016, color.RGBA{190, 70, 60, 255}, 0.10, 0.03, color.RGBA{215, 175, 115, 255}, content.HeadTaper, 0}},
		{ID: "core:broom", Name: "Balai", Radius: 2.25, Reach: 6, Rate: 1.4, Cost: 500,
			Look: content.ToolLook{0.55, 0.02, color.RGBA{165, 115, 70, 255}, 0.24, 0.12, color.RGBA{240, 205, 95, 255}, content.HeadFlare, 0}},
		{ID: "core:vacuum", Name: "Aspirateur", Radius: 3.75, Reach: 9, Rate: 2.2, Cost: 4_000,
			Look: content.ToolLook{0.40, 0.025, color.RGBA{170, 175, 190, 255}, 0.22, 0.07, color.RGBA{225, 75, 80, 255}, content.HeadNozzle, 0.6}},
		{ID: "core:blower", Name: "Souffleur indus.", Radius: 6, Reach: 13, Rate: 3.5, Cost: 35_000,
			Look: content.ToolLook{0.30, 0.05, color.RGBA{60, 64, 80, 255}, 0.30, 0.09, color.RGBA{240, 150, 55, 255}, content.HeadNozzle, 0.8}},
		{ID: "core:vacuum-cannon", Name: "Canon à vide", Radius: 10, Reach: 22, Rate: 6, Cost: 400_000,
			Look: content.ToolLook{0.35, 0.06, color.RGBA{230, 232, 240, 255}, 0.26, 0.12, color.RGBA{80, 130, 235, 255}, content.HeadNozzle, 1}},
	} {
		c.Tools.Add(t.ID, t)
	}

	for _, u := range []content.Upgrade{
		{ID: "core:tank", Name: "Réservoir", Desc: "Stocke 60% de poussière en plus avant de devoir rentrer vendre",
			Icon: "tank", Base: 25, Growth: 1.75, Effects: []content.Effect{{Stat: "tank", Mult: 1.6}}},
		{ID: "core:power", Name: "Turbo", Desc: "Outils et drones aspirent 30% plus vite",
			Icon: "power", Base: 40, Growth: 1.85, Effects: []content.Effect{{Stat: "power", Mult: 1.3}}},
		{ID: "core:price", Name: "Négociation", Desc: "La poussière et les cristaux se vendent 20% plus cher",
			Icon: "price", Base: 150, Growth: 2.1, Effects: []content.Effect{{Stat: "price", Mult: 1.2}}},
		{ID: "core:drone", Name: "Drone balayeur", Desc: "Un drone autonome qui nettoie à ta place",
			Icon: "drone", Base: 300, Growth: 1.9, Effects: []content.Effect{{Stat: "drones", Add: 1}}},
		{ID: "core:auto", Name: "Téléporteur", Desc: "Vend à distance dès que le réservoir est plein",
			Icon: "auto", Base: 1_500, Growth: 1, Max: 1, Effects: []content.Effect{{Stat: "autosell", Add: 1}}},
	} {
		c.Upgrades.Add(u.ID, u)
	}

	for _, name := range slices.Sorted(maps.Keys(coreTokens)) {
		c.Tokens.Add("core:"+name, coreTokens[name])
	}
	for _, name := range slices.Sorted(maps.Keys(coreKits)) {
		c.Kits.Add("core:"+name, coreKits[name])
	}
	for _, bp := range coreBuildings {
		c.Buildings.Add(bp.ID, bp)
	}

	c.Entities.Add("core:flag", content.EntityDef{Name: "Drapeau", Pickup: true, Radius: 0.15, Parts: []content.PartJSON{
		{Shape: "prism", Token: "steel.dark", R0: 0.16, R1: 0.12, H: 0.08, Sides: 8, Lit: "outdoor"},
		{Shape: "prism", Token: "steel", R0: 0.025, H: 2.3, Sides: 6, Lit: "outdoor"},
		{Shape: "prism", Token: "steel", At: vec3{0, 2.3, 0}, R0: 0.045, H: 0.05, Sides: 8, Lit: "outdoor"},
		{Shape: "box", Token: "hazard", At: vec3{0.42, 2.0, 0}, Size: vec3{0.8, 0.5, 0.015}, Lit: "outdoor"},
		{Shape: "box", Token: "neon.cyan", At: vec3{0.42, 1.73, 0}, Size: vec3{0.8, 0.04, 0.02}, Lit: "outdoor"},
	}})
	c.Items.Add("core:flag", content.Item{Name: "Drapeau", Desc: "À planter où tu veux : un repère visible de loin (F)",
		Icon: "flag", Cost: 40, Places: "core:flag"})

	wheels := []content.PartJSON{}
	for _, x := range []float64{-0.85, 0.85} {
		for _, z := range []float64{-0.8, 0.8} {
			wheels = append(wheels,
				content.PartJSON{Shape: "prism", Token: "rubber", At: vec3{x, 0.4, z - 0.15}, Axis: vec3{0, 0, 1}, R0: 0.4, H: 0.3, Sides: 12, Bevel: 0.04, Lit: "outdoor"},
				content.PartJSON{Shape: "prism", Token: "steel", At: vec3{x, 0.4, z + 0.16*math.Copysign(1, z) - 0.01}, Axis: vec3{0, 0, 1}, R0: 0.18, H: 0.02, Sides: 8, Lit: "outdoor"})
		}
	}
	c.Vehicles.Add("core:rover", content.Vehicle{
		Name: "Rover balayeur", Desc: "Roule à 45 km/h et aspire la poussière sous ses roues",
		Cost: 15_000, Speed: 12.5, Sweep: 2.2, Rate: 2.5, Seat: 2.3, Radius: 1.5,
		Parts: append(wheels,
			content.PartJSON{Shape: "box", Token: "steel.dark", At: vec3{0, 0.55, 0}, Size: vec3{2.3, 0.25, 1.3}, Bevel: 0.04, Lit: "outdoor"},
			content.PartJSON{Shape: "box", Token: "habitat.white", At: vec3{-0.25, 0.9, 0}, Size: vec3{1.7, 0.5, 1.4}, Bevel: 0.08, Lit: "outdoor"},
			content.PartJSON{Shape: "box", Token: "hazard", At: vec3{-0.25, 0.72, 0}, Size: vec3{1.72, 0.12, 1.42}, Lit: "outdoor"},
			content.PartJSON{Shape: "box", Token: "habitat.white2", At: vec3{0.75, 1.15, 0}, Size: vec3{0.7, 0.75, 1.2}, Bevel: 0.08, Lit: "outdoor"},
			content.PartJSON{Shape: "box", Token: "glass.dark", At: vec3{1.1, 1.3, 0}, Size: vec3{0.04, 0.4, 1.0}, Lit: "outdoor"},
			content.PartJSON{Shape: "box", Token: "steel", At: vec3{1.3, 0.3, 0}, Size: vec3{0.35, 0.2, 1.6}, Bevel: 0.04, Lit: "outdoor"},
			content.PartJSON{Shape: "box", Token: "neon.cyan", At: vec3{1.45, 0.28, 0}, Size: vec3{0.03, 0.05, 1.4}, Lit: "outdoor"},
			content.PartJSON{Shape: "prism", Token: "steel", At: vec3{-0.8, 1.15, 0.45}, R0: 0.02, H: 1.1, Sides: 5, Lit: "outdoor"},
			content.PartJSON{Shape: "prism", Token: "led.green", At: vec3{-0.8, 2.25, 0.45}, R0: 0.04, H: 0.06, Sides: 6, Lit: "outdoor"},
		),
	})

	// Dust devils rise in daytime, a few at once.
	c.Events.Add("core:dust-devil", content.RandomEvent{Every: 15,
		When: func(g content.Game) bool { return g.DustDevils() < maxDevils && g.SunHeight() > 0.2 },
		Run:  content.Game.SpawnDustDevil})
}

// Off-whites, steel greys, cyan and yellow/black accents: they stand out
// against the orange ground.
var coreTokens = map[string]content.Token{
	"habitat.white":  {color.RGBA{232, 230, 224, 255}, content.MtPanel, 1, 0.55, 0},
	"habitat.white2": {color.RGBA{218, 217, 212, 255}, content.MtPanel, 1, 0.55, 0},
	"steel":          {color.RGBA{140, 146, 158, 255}, content.MtMetal, 1.2, 0.6, 0},
	"steel.dark":     {color.RGBA{50, 54, 66, 255}, content.MtPlain, 1, 0.4, 0},
	"rack.slate":     {color.RGBA{58, 64, 80, 255}, content.MtPanel, 1, 0.4, 0},
	"hazard":         {colWhite, content.MtHazard, 2.5, 0.3, 0},
	"neon.cyan":      {accentCyan, content.MtPlain, 1, 0, 0.8},
	"neon.orange":    {accentOrange, content.MtPlain, 1, 0, 0.8},
	"neon.blue":      {accentBlue, content.MtPlain, 1, 0, 0.8},
	"neon.purple":    {accentPurple, content.MtPlain, 1, 0, 0.8},
	"lamp.warm":      {lampWarm, content.MtPlain, 1, 0, 1},
	"floor.slate":    {color.RGBA{50, 54, 66, 255}, content.MtPanel, 1, 0.25, 0},
	"wall.lower":     {color.RGBA{86, 100, 118, 255}, content.MtPanel, 1, 0.3, 0},
	"floor.inlay":    {color.RGBA{68, 74, 90, 255}, content.MtPlain, 1, 0.3, 0},
	"solar.blue":     {color.RGBA{40, 62, 130, 255}, content.MtPlain, 1, 0.9, 0},
	"crate.ochre":    {color.RGBA{196, 132, 60, 255}, content.MtPanel, 1, 0.2, 0},
	"crate.slate":    {color.RGBA{88, 116, 146, 255}, content.MtPanel, 1, 0.2, 0},
	"crate.sand":     {color.RGBA{158, 148, 118, 255}, content.MtPanel, 1, 0.2, 0},
	"wood.brown":     {color.RGBA{110, 80, 60, 255}, content.MtPlain, 1, 0.3, 0},
	"fabric.white":   {matWhite, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.purple":  {accentPurple, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.rug":     {color.RGBA{150, 70, 60, 255}, content.MtFabric, 1 / content.FabricTile, 0, 0},
	// Greebles and the yard (greeble.go).
	"glass.dark":    {color.RGBA{26, 38, 54, 255}, content.MtPlain, 1, 1.0, 0},
	"pipe.yellow":   {color.RGBA{214, 164, 52, 255}, content.MtPlain, 1, 0.5, 0},
	"pipe.blue":     {color.RGBA{62, 112, 168, 255}, content.MtPlain, 1, 0.5, 0},
	"pipe.red":      {color.RGBA{178, 58, 48, 255}, content.MtPlain, 1, 0.5, 0},
	"barrel.blue":   {color.RGBA{54, 92, 150, 255}, content.MtPaint, 1 / content.PaintTile, 0.4, 0},
	"barrel.ochre":  {color.RGBA{196, 132, 60, 255}, content.MtPaint, 1 / content.PaintTile, 0.4, 0},
	"rubber":        {color.RGBA{34, 34, 38, 255}, content.MtPlain, 1, 0.2, 0},
	"locker":        {color.RGBA{72, 88, 108, 255}, content.MtPanel, 1, 0.4, 0},
	"locker.ochre":  {color.RGBA{176, 124, 60, 255}, content.MtPanel, 1, 0.4, 0},
	"pad.concrete":  {color.RGBA{130, 120, 110, 255}, content.MtPanel, 1, 0.2, 0},
	"screen.dim":    {accentCyan, content.MtPlain, 1, 0, 0.4},
	"led.green":     {colGreen, content.MtPlain, 1, 0, 1},
	"fabric.seat":   {color.RGBA{58, 74, 98, 255}, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"soil":          {color.RGBA{60, 40, 30, 255}, content.MtPlain, 1, 0, 0},
	"planter.green": {color.RGBA{72, 150, 76, 255}, content.MtPlain, 1, 0.2, 0},
	"planter.lime":  {color.RGBA{138, 186, 70, 255}, content.MtPlain, 1, 0.2, 0},
	"lamp.grow":     {color.RGBA{230, 120, 220, 255}, content.MtPlain, 1, 0, 1},
	"paper":         {color.RGBA{236, 234, 226, 255}, content.MtPlain, 1, 0, 0},
	"visor.gold":    {color.RGBA{200, 150, 60, 255}, content.MtPlain, 1, 1.2, 0},
}

// coreBuildings: the hub first, the rooms around it. The hub's south door
// leads to the airlock.
var coreBuildings = []content.Blueprint{
	content.Blueprint{
		ID:     "core:hub",
		Accent: "neon.cyan",
		Shape:  content.Dome{Radius: 6.5, WallH: 2.8},
		Style:  "habitat.white",
		Doors:  []content.Door{{base.East}, {base.West}, {base.North}, {base.South}},
		Props: []content.Prop{
			{Kit: "maptable"},
			{Kit: "crate.stack", At: content.Polar(5.4, 0.8), Seed: 1},
			{Kit: "crate.stack", At: content.Polar(5.4, 2.4), Seed: 2},
			{Kit: "crate.stack", At: content.Polar(5.4, 3.9), Seed: 4},
			{Kit: "crate.stack", At: content.Polar(5.4, 5.5), Seed: 5},
		},
	},
	content.Blueprint{
		ID:     "core:sale",
		Link:   "core:hub",
		Sign:   "VENTE",
		Accent: "neon.orange",
		At:     vec3{-13, 0, 0},
		Shape:  content.Dome{Radius: 4, WallH: 2.6},
		Style:  "habitat.white",
		Doors:  []content.Door{{base.East}},
		Props: []content.Prop{
			{Kit: "incinerator", At: vec3{-1.6, 0, 0}},
			{Kit: "crate.stack", At: vec3{1.2, 0, 2.4}, Seed: 3},
			{Kit: "crate.stack", At: vec3{0.6, 0, -2.7}, Seed: 8},
		},
	},
	content.Blueprint{
		ID:     "core:research",
		Link:   "core:hub",
		Sign:   "RECHERCHE",
		Accent: "neon.blue",
		At:     vec3{13, 0, 0},
		Shape:  content.Dome{Radius: 4, WallH: 2.6},
		Style:  "habitat.white",
		Doors:  []content.Door{{base.West}},
		Props: []content.Prop{
			{Kit: "console", At: vec3{2.3, 0, 0}},
			{Kit: "chair", At: vec3{1.2, 0, 0.5}, Seed: 0},
			{Kit: "workbench", At: vec3{-0.6, 0, 3.0}, Seed: 6},
			{Kit: "planter", At: vec3{-0.6, 0, -3.0}, Seed: 2},
		},
	},
	content.Blueprint{
		ID:     "core:sleep",
		Link:   "core:hub",
		Sign:   "QUARTIERS",
		Accent: "neon.purple",
		At:     vec3{0, 0, 13},
		Shape:  content.Dome{Radius: 4, WallH: 2.6},
		Style:  "habitat.white",
		Doors:  []content.Door{{base.South}},
		Props: []content.Prop{
			{Kit: "bed", At: vec3{0, 0, 2.1}},
			{Kit: "desk", At: vec3{2.75, 0, 0.6}, Seed: 4},
			{Kit: "chair", At: vec3{2.0, 0, 0.6}, Seed: 4},
			{Kit: "planter", At: vec3{-2.8, 0, 0.6}, Seed: 2},
		},
	},
}

var coreKits = map[string]content.Kit{
	"maptable": {Build: base.KitBuild(base.BuildMapTable), Obstacles: []content.Obstacle{{vec3{}, 1.25}},
		Reach: 2.6, Use: func(g content.Game, _ vec3) { g.OpenMap() }, Prompt: "[E] Consulter l'état de Mars"},
	"incinerator": {Build: base.KitBuild(base.BuildIncinerator), Obstacles: []content.Obstacle{{vec3{}, 1.4}},
		Reach: 2.8, Use: content.Game.Deposit, Prompt: "[E] Déposer la poussière dans l'incinérateur"},
	"console": {Build: base.KitBuild(base.BuildConsole),
		Obstacles: []content.Obstacle{{vec3{-0.3, 0, 0}, 1.0}, {vec3{0, 0, 2.3}, 0.6}, {vec3{0, 0, -2.3}, 0.6}},
		Reach:     2.8, Use: func(g content.Game, _ vec3) { g.OpenShop() }, Prompt: "[E] Ouvrir le laboratoire de recherche"},
	"bed": {Build: base.KitBuild(base.BuildBed), Obstacles: []content.Obstacle{{vec3{-0.55, 0, 0}, 0.6}, {vec3{0.55, 0, 0}, 0.6}},
		Reach: 2.6, Use: func(g content.Game, _ vec3) { g.Sleep() }, Prompt: "[E] Dormir"},
	"crate.stack": {Build: base.KitBuild(base.BuildCrates), Obstacles: []content.Obstacle{{vec3{}, 0.55}}},
	"chair":       {Build: base.KitBuild(base.BuildChair), Obstacles: []content.Obstacle{{vec3{}, 0.35}}},
	"workbench":   {Build: base.KitBuild(base.BuildWorkbench), Obstacles: []content.Obstacle{{vec3{}, 0.8}}},
	"planter":     {Build: base.KitBuild(base.BuildPlanter), Obstacles: []content.Obstacle{{vec3{}, 0.85}}},
	"desk":        {Build: base.KitBuild(base.BuildDesk), Obstacles: []content.Obstacle{{vec3{}, 0.65}}},
}
