package main

import (
	"image/color"
	"maps"
	"slices"
)

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

	for _, name := range slices.Sorted(maps.Keys(coreTokens)) {
		c.Tokens.Add("core:"+name, coreTokens[name])
	}
	for _, name := range slices.Sorted(maps.Keys(coreKits)) {
		c.Kits.Add("core:"+name, coreKits[name])
	}
	for _, bp := range coreBuildings {
		c.Buildings.Add(bp.ID, bp)
	}
}

// Off-whites, steel greys, cyan and yellow/black accents: they stand out
// against the orange ground.
var coreTokens = map[string]token{
	"habitat.white":  {color.RGBA{232, 230, 224, 255}, mtPanel, 1, 0.55, 0},
	"habitat.white2": {color.RGBA{218, 217, 212, 255}, mtPanel, 1, 0.55, 0},
	"steel":          {color.RGBA{140, 146, 158, 255}, mtMetal, 1.2, 0.6, 0},
	"steel.dark":     {color.RGBA{50, 54, 66, 255}, mtPlain, 1, 0.4, 0},
	"rack.slate":     {color.RGBA{58, 64, 80, 255}, mtPanel, 1, 0.4, 0},
	"hazard":         {colWhite, mtHazard, 2.5, 0.3, 0},
	"neon.cyan":      {accentCyan, mtPlain, 1, 0, 0.8},
	"neon.orange":    {accentOrange, mtPlain, 1, 0, 0.8},
	"neon.blue":      {accentBlue, mtPlain, 1, 0, 0.8},
	"neon.purple":    {accentPurple, mtPlain, 1, 0, 0.8},
	"lamp.warm":      {lampWarm, mtPlain, 1, 0, 1},
	"floor.slate":    {color.RGBA{50, 54, 66, 255}, mtPanel, 1, 0.25, 0},
	"wall.lower":     {color.RGBA{86, 100, 118, 255}, mtPanel, 1, 0.3, 0},
	"floor.inlay":    {color.RGBA{68, 74, 90, 255}, mtPlain, 1, 0.3, 0},
	"solar.blue":     {color.RGBA{40, 62, 130, 255}, mtPlain, 1, 0.9, 0},
	"crate.ochre":    {color.RGBA{196, 132, 60, 255}, mtPanel, 1, 0.2, 0},
	"crate.slate":    {color.RGBA{88, 116, 146, 255}, mtPanel, 1, 0.2, 0},
	"crate.sand":     {color.RGBA{158, 148, 118, 255}, mtPanel, 1, 0.2, 0},
	"wood.brown":     {color.RGBA{110, 80, 60, 255}, mtPlain, 1, 0.3, 0},
	"fabric.white":   {matWhite, mtFabric, 1 / fabricTile, 0.1, 0},
	"fabric.purple":  {accentPurple, mtFabric, 1 / fabricTile, 0.1, 0},
	"fabric.rug":     {color.RGBA{150, 70, 60, 255}, mtFabric, 1 / fabricTile, 0, 0},
	// Greebles and the yard (greeble.go).
	"glass.dark":    {color.RGBA{26, 38, 54, 255}, mtPlain, 1, 1.0, 0},
	"pipe.yellow":   {color.RGBA{214, 164, 52, 255}, mtPlain, 1, 0.5, 0},
	"pipe.blue":     {color.RGBA{62, 112, 168, 255}, mtPlain, 1, 0.5, 0},
	"pipe.red":      {color.RGBA{178, 58, 48, 255}, mtPlain, 1, 0.5, 0},
	"barrel.blue":   {color.RGBA{54, 92, 150, 255}, mtPaint, 1 / paintTile, 0.4, 0},
	"barrel.ochre":  {color.RGBA{196, 132, 60, 255}, mtPaint, 1 / paintTile, 0.4, 0},
	"rubber":        {color.RGBA{34, 34, 38, 255}, mtPlain, 1, 0.2, 0},
	"locker":        {color.RGBA{72, 88, 108, 255}, mtPanel, 1, 0.4, 0},
	"locker.ochre":  {color.RGBA{176, 124, 60, 255}, mtPanel, 1, 0.4, 0},
	"pad.concrete":  {color.RGBA{130, 120, 110, 255}, mtPanel, 1, 0.2, 0},
	"screen.dim":    {accentCyan, mtPlain, 1, 0, 0.4},
	"led.green":     {colGreen, mtPlain, 1, 0, 1},
	"fabric.seat":   {color.RGBA{58, 74, 98, 255}, mtFabric, 1 / fabricTile, 0.1, 0},
	"soil":          {color.RGBA{60, 40, 30, 255}, mtPlain, 1, 0, 0},
	"planter.green": {color.RGBA{72, 150, 76, 255}, mtPlain, 1, 0.2, 0},
	"planter.lime":  {color.RGBA{138, 186, 70, 255}, mtPlain, 1, 0.2, 0},
	"lamp.grow":     {color.RGBA{230, 120, 220, 255}, mtPlain, 1, 0, 1},
	"paper":         {color.RGBA{236, 234, 226, 255}, mtPlain, 1, 0, 0},
	"visor.gold":    {color.RGBA{200, 150, 60, 255}, mtPlain, 1, 1.2, 0},
}

// coreBuildings: the hub first, the rooms around it. The hub's south door
// leads to the airlock.
var coreBuildings = []Blueprint{
	Blueprint{
		ID:     "core:hub",
		Accent: "neon.cyan",
		Shape:  Dome{Radius: 6.5, WallH: 2.8},
		Style:  "habitat.white",
		Doors:  []Door{{east}, {west}, {north}, {south}},
		Props: []Prop{
			{Kit: "maptable"},
			{Kit: "crate.stack", At: polar(5.4, 0.8), Seed: 1},
			{Kit: "crate.stack", At: polar(5.4, 2.4), Seed: 2},
			{Kit: "crate.stack", At: polar(5.4, 3.9), Seed: 4},
			{Kit: "crate.stack", At: polar(5.4, 5.5), Seed: 5},
		},
	},
	Blueprint{
		ID:     "core:sale",
		Link:   "core:hub",
		Sign:   "VENTE",
		Accent: "neon.orange",
		At:     vec3{-13, 0, 0},
		Shape:  Dome{Radius: 4, WallH: 2.6},
		Style:  "habitat.white",
		Doors:  []Door{{east}},
		Props: []Prop{
			{Kit: "incinerator", At: vec3{-1.6, 0, 0}},
			{Kit: "crate.stack", At: vec3{1.2, 0, 2.4}, Seed: 3},
			{Kit: "crate.stack", At: vec3{0.6, 0, -2.7}, Seed: 8},
		},
	},
	Blueprint{
		ID:     "core:research",
		Link:   "core:hub",
		Sign:   "RECHERCHE",
		Accent: "neon.blue",
		At:     vec3{13, 0, 0},
		Shape:  Dome{Radius: 4, WallH: 2.6},
		Style:  "habitat.white",
		Doors:  []Door{{west}},
		Props: []Prop{
			{Kit: "console", At: vec3{2.3, 0, 0}},
			{Kit: "chair", At: vec3{1.2, 0, 0.5}, Seed: 0},
			{Kit: "workbench", At: vec3{-0.6, 0, 3.0}, Seed: 6},
			{Kit: "planter", At: vec3{-0.6, 0, -3.0}, Seed: 2},
		},
	},
	Blueprint{
		ID:     "core:sleep",
		Link:   "core:hub",
		Sign:   "QUARTIERS",
		Accent: "neon.purple",
		At:     vec3{0, 0, 13},
		Shape:  Dome{Radius: 4, WallH: 2.6},
		Style:  "habitat.white",
		Doors:  []Door{{south}},
		Props: []Prop{
			{Kit: "bed", At: vec3{0, 0, 2.1}},
			{Kit: "desk", At: vec3{2.75, 0, 0.6}, Seed: 4},
			{Kit: "chair", At: vec3{2.0, 0, 0.6}, Seed: 4},
			{Kit: "planter", At: vec3{-2.8, 0, 0.6}, Seed: 2},
		},
	},
}

var coreKits = map[string]kitProp{
	"maptable": {build: buildMapTable, obstacles: []obstacle{{vec3{}, 1.25}},
		reach: 2.6, use: func(g *Game, _ vec3) { g.setMode(modeMap) }, prompt: "[E] Consulter l'état de Mars"},
	"incinerator": {build: buildIncinerator, draw: drawIncinerator, obstacles: []obstacle{{vec3{}, 1.4}},
		reach: 2.8, use: (*Game).deposit, prompt: "[E] Déposer la poussière dans l'incinérateur"},
	"console": {build: buildConsole, draw: drawConsole,
		obstacles: []obstacle{{vec3{-0.3, 0, 0}, 1.0}, {vec3{0, 0, 2.3}, 0.6}, {vec3{0, 0, -2.3}, 0.6}},
		reach:     2.8, use: func(g *Game, _ vec3) { g.setMode(modeShop) }, prompt: "[E] Ouvrir le laboratoire de recherche"},
	"bed": {build: buildBed, draw: drawBed, obstacles: []obstacle{{vec3{-0.55, 0, 0}, 0.6}, {vec3{0.55, 0, 0}, 0.6}},
		reach: 2.6, use: func(g *Game, _ vec3) { g.mode, g.sleepT = modeSleep, 0 }, prompt: "[E] Dormir"},
	"crate.stack": {build: buildCrates, obstacles: []obstacle{{vec3{}, 0.55}}},
	"chair":       {build: buildChair, obstacles: []obstacle{{vec3{}, 0.35}}},
	"workbench":   {build: buildWorkbench, obstacles: []obstacle{{vec3{}, 0.8}}},
	"planter":     {build: buildPlanter, obstacles: []obstacle{{vec3{}, 0.85}}},
	"desk":        {build: buildDesk, obstacles: []obstacle{{vec3{}, 0.65}}},
}
