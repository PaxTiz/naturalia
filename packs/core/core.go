// Package core is the base game, as a pack: it registers its content
// through the same API as a mod, from an init function.
package core

import (
	"image/color"
	"maps"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/palette"
	"slices"
)

func init() {
	content.RegisterPack(content.Pack{
		ID:       "core",
		Name:     "Naturalia",
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
		{ID: "core:power", Name: "Turbo", Desc: "Outils et véhicules aspirent 30% plus vite",
			Icon: "power", Base: 40, Growth: 1.85, Effects: []content.Effect{{Stat: "power", Mult: 1.3}}},
		{ID: "core:price", Name: "Négociation", Desc: "La poussière et les cristaux se vendent 20% plus cher",
			Icon: "price", Base: 150, Growth: 2.1, Effects: []content.Effect{{Stat: "price", Mult: 1.2}}},
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
	// The hub and the sale room were joined in the great hall: what was
	// linked to them is linked to it.
	c.Buildings.Alias("core:hub", "core:hall")
	c.Buildings.Alias("core:sale", "core:hall")

	c.Entities.Add("core:flag", content.EntityDef{Name: "Drapeau", Pickup: true, Radius: 0.15, Parts: []content.PartJSON{
		{Shape: "prism", Token: "steel.dark", R0: 0.16, R1: 0.12, H: 0.08, Sides: 8, Lit: "outdoor"},
		{Shape: "prism", Token: "steel", R0: 0.025, H: 2.3, Sides: 6, Lit: "outdoor"},
		{Shape: "prism", Token: "steel", At: vec3{0, 2.3, 0}, R0: 0.045, H: 0.05, Sides: 8, Lit: "outdoor"},
		{Shape: "box", Token: "hazard", At: vec3{0.42, 2.0, 0}, Size: vec3{0.8, 0.5, 0.015}, Lit: "outdoor"},
		{Shape: "box", Token: "neon.cyan", At: vec3{0.42, 1.73, 0}, Size: vec3{0.8, 0.04, 0.02}, Lit: "outdoor"},
	}})
	c.Items.Add("core:flag", content.Item{Name: "Drapeau", Desc: "À planter où tu veux : un repère visible de loin (F)",
		Icon: "flag", Cost: 40, Places: "core:flag"})
	c.Entities.Add("core:crate", content.EntityDef{Name: "Caisse", Pickup: true, Mass: 40, Parts: crateParts(0.8, "crate.ochre")})
	c.Items.Add("core:crate", content.Item{Name: "Caisse", Desc: "Du matériel à poser (F) : on la pousse, les véhicules la bousculent",
		Icon: "crate", Cost: 25, Places: "core:crate"})

	registerVehicles(c)
	registerStunts(c)
	registerBuild(c)

	// Dust devils rise in daytime, a few at once.
	const maxDevils = 3
	c.Events.Add("core:dust-devil", content.RandomEvent{Every: 15,
		When: func(g content.Game) bool { return g.DustDevils() < maxDevils && g.SunHeight() > 0.2 },
		Run:  content.Game.SpawnDustDevil})
}

// crateParts: a crate s m wide, painted tok, with steel corners and rims
// and a white label.
func crateParts(s float64, tok string) []content.PartJSON {
	h, k := s/2, s/0.8 // its middle, and how much bigger than the small one
	parts := []content.PartJSON{
		{Shape: "box", Token: tok, At: vec3{0, h, 0}, Size: vec3{s * 0.94, s * 0.94, s * 0.94}, Bevel: 0.03 * k, Lit: "outdoor"},
		{Shape: "box", Token: "rubber", At: vec3{0, h, 0}, Size: vec3{0.08 * k, s * 0.95, s * 0.95}, Lit: "outdoor"},
		{Shape: "box", Token: "habitat.white", At: vec3{s * 0.2, h + s*0.1, s * 0.475}, Size: vec3{s * 0.3, s * 0.2, 0.01 * k}, Lit: "outdoor"},
	}
	for _, x := range []float64{-1, 1} {
		for _, z := range []float64{-1, 1} {
			parts = append(parts, content.PartJSON{Shape: "box", Token: "steel.dark", At: vec3{x * s * 0.47, h, z * s * 0.47}, Size: vec3{0.07 * k, s, 0.07 * k}, Lit: "outdoor"})
		}
	}
	for _, y := range []float64{0.03 * k, s - 0.03*k} {
		for _, side := range []float64{-1, 1} {
			parts = append(parts,
				content.PartJSON{Shape: "box", Token: "steel.dark", At: vec3{0, y, side * s * 0.47}, Size: vec3{s, 0.06 * k, 0.06 * k}, Lit: "outdoor"},
				content.PartJSON{Shape: "box", Token: "steel.dark", At: vec3{side * s * 0.47, y, 0}, Size: vec3{0.06 * k, 0.06 * k, s}, Lit: "outdoor"})
		}
	}
	return parts
}

// Off-whites, steel greys, cyan and yellow/black accents: they stand out
// against the orange ground.
var coreTokens = map[string]content.Token{
	"habitat.white":  {color.RGBA{232, 230, 224, 255}, content.MtPanel, 1, 0.55, 0},
	"habitat.white2": {color.RGBA{218, 217, 212, 255}, content.MtPanel, 1, 0.55, 0},
	"steel":          {color.RGBA{140, 146, 158, 255}, content.MtMetal, 1.2, 0.6, 0},
	"steel.dark":     {color.RGBA{50, 54, 66, 255}, content.MtPlain, 1, 0.4, 0},
	"door.plate":     {color.RGBA{168, 172, 178, 255}, content.MtMetal, 1.2, 0.45, 0}, // the hangar's door leaves
	"door.plate2":    {color.RGBA{154, 159, 166, 255}, content.MtMetal, 1.2, 0.45, 0},
	"rack.slate":     {color.RGBA{58, 64, 80, 255}, content.MtPanel, 1, 0.4, 0},
	"hazard":         {palette.White, content.MtHazard, 2.5, 0.3, 0},
	"neon.cyan":      {palette.AccentCyan, content.MtPlain, 1, 0, 0.8},
	"neon.orange":    {palette.AccentOrange, content.MtPlain, 1, 0, 0.8},
	"neon.blue":      {palette.AccentBlue, content.MtPlain, 1, 0, 0.8},
	"neon.purple":    {palette.AccentPurple, content.MtPlain, 1, 0, 0.8},
	"lamp.warm":      {palette.LampWarm, content.MtPlain, 1, 0, 1},
	"floor.slate":    {color.RGBA{50, 54, 66, 255}, content.MtPanel, 1, 0.25, 0},
	"wall.lower":     {color.RGBA{86, 100, 118, 255}, content.MtPanel, 1, 0.3, 0},
	"floor.inlay":    {color.RGBA{68, 74, 90, 255}, content.MtPlain, 1, 0.3, 0},
	"solar.blue":     {color.RGBA{40, 62, 130, 255}, content.MtPlain, 1, 0.9, 0},
	"crate.ochre":    {color.RGBA{196, 132, 60, 255}, content.MtPanel, 1, 0.2, 0},
	"crate.slate":    {color.RGBA{88, 116, 146, 255}, content.MtPanel, 1, 0.2, 0},
	"crate.sand":     {color.RGBA{158, 148, 118, 255}, content.MtPanel, 1, 0.2, 0},
	"wood.brown":     {color.RGBA{110, 80, 60, 255}, content.MtPlain, 1, 0.3, 0},
	"fabric.white":   {palette.MatWhite, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.purple":  {palette.AccentPurple, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.rug":     {color.RGBA{150, 70, 60, 255}, content.MtFabric, 1 / content.FabricTile, 0, 0},
	// Greebles and the yard (internal/base/greeble.go).
	"glass.dark":    {color.RGBA{26, 38, 54, 255}, content.MtPlain, 1, 1.0, 0},
	"glass.pane":    {color.RGBA{150, 205, 220, 255}, content.MtPlain, 1, 1.0, 0},
	"pipe.yellow":   {color.RGBA{214, 164, 52, 255}, content.MtPlain, 1, 0.5, 0},
	"pipe.blue":     {color.RGBA{62, 112, 168, 255}, content.MtPlain, 1, 0.5, 0},
	"pipe.red":      {color.RGBA{178, 58, 48, 255}, content.MtPlain, 1, 0.5, 0},
	"barrel.blue":   {color.RGBA{54, 92, 150, 255}, content.MtPaint, 1 / content.PaintTile, 0.4, 0},
	"barrel.ochre":  {color.RGBA{196, 132, 60, 255}, content.MtPaint, 1 / content.PaintTile, 0.4, 0},
	"rubber":        {color.RGBA{34, 34, 38, 255}, content.MtPlain, 1, 0.2, 0},
	"locker":        {color.RGBA{72, 88, 108, 255}, content.MtPanel, 1, 0.4, 0},
	"locker.ochre":  {color.RGBA{176, 124, 60, 255}, content.MtPanel, 1, 0.4, 0},
	"pad.concrete":  {color.RGBA{130, 120, 110, 255}, content.MtPanel, 1, 0.2, 0},
	"screen.dim":    {palette.AccentCyan, content.MtPlain, 1, 0, 0.4},
	"led.green":     {palette.Green, content.MtPlain, 1, 0, 1},
	"fabric.seat":   {color.RGBA{58, 74, 98, 255}, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"soil":          {color.RGBA{60, 40, 30, 255}, content.MtPlain, 1, 0, 0},
	"planter.green": {color.RGBA{72, 150, 76, 255}, content.MtPlain, 1, 0.2, 0},
	"planter.lime":  {color.RGBA{138, 186, 70, 255}, content.MtPlain, 1, 0.2, 0},
	"lamp.grow":     {color.RGBA{230, 120, 220, 255}, content.MtPlain, 1, 0, 1},
	"paper":         {color.RGBA{236, 234, 226, 255}, content.MtPlain, 1, 0, 0},
	"visor.gold":    {color.RGBA{200, 150, 60, 255}, content.MtPlain, 1, 1.2, 0},
	// The bedroom: wood, soft fabrics, amber light.
	"wood.light":   {color.RGBA{168, 122, 82, 255}, content.MtPlain, 1, 0.3, 0},
	"fabric.teal":  {color.RGBA{58, 108, 116, 255}, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.duvet": {color.RGBA{200, 112, 70, 255}, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.wool":  {color.RGBA{226, 212, 186, 255}, content.MtFabric, 1 / content.FabricTile, 0.05, 0},
	"lamp.shade":   {color.RGBA{255, 214, 160, 255}, content.MtPlain, 1, 0, 0.55},
	"lamp.amber":   {color.RGBA{255, 180, 100, 255}, content.MtPlain, 1, 0, 1},
	"neon.amber":   {color.RGBA{255, 170, 80, 255}, content.MtPlain, 1, 0, 0.9},
	"rock.red":     {color.RGBA{150, 72, 48, 255}, content.MtPlain, 1, 0.1, 0},
	"earth.blue":   {color.RGBA{50, 110, 205, 255}, content.MtPlain, 1, 0.5, 0.15},
	"earth.green":  {color.RGBA{96, 150, 80, 255}, content.MtPlain, 1, 0.5, 0.1},
	"clay":         {color.RGBA{176, 98, 64, 255}, content.MtPlain, 1, 0.15, 0},
}

// coreBuildings: the great hall first, the rooms around it, then the
// hangar. The hall's south door leads to the airlock; its east wall is
// kept for the bay window. The research room and the hangar stand west,
// turned half a turn (Yaw): they keep the layout they had on the other
// side.
var coreBuildings = []content.Blueprint{
	// The hub and the sale room in one: the map table in the middle, the
	// incinerator in the south-west corner under its sign.
	content.Blueprint{
		ID:     "core:hall",
		Accent: "neon.cyan",
		Hall:   &content.Hall{Size: [2]float64{24, 16}, WallH: 6, Rise: 2},
		Style:  "habitat.white",
		// The bay window, all along the east wall: the plain, the sunrise.
		Windows: []content.Window{{At: vec3{12, 0.4, 0}, Width: 14, Height: 5.2}},
		// The lookout: a gallery 3 m up along the bay window, stairs
		// rising east to it along the south wall.
		Floors: []content.Floor{
			{From: vec3{9.75, 3, -7.8}, To: vec3{9.75, 3, 7.8}, Width: 4, Thick: 0.3},
			{From: vec3{2.2, 0, -6.9}, To: vec3{7.75, 3, -6.9}, Width: 1.3},
		},
		Signs: []content.Sign{{Text: "VENTE", At: vec3{-11.82, 3.6, -6.3}, Facing: base.East, Width: 2.6, Height: 0.6, Accent: "neon.orange"}},
		Props: []content.Prop{
			{Kit: "maptable"},
			{Kit: "incinerator", At: vec3{-10, 0, -5}},
			{Kit: "crate.stack", At: vec3{-10.6, 0, 6.6}, Seed: 1},
			{Kit: "crate.stack", At: vec3{10.6, 0, 6.6}, Seed: 2},
			{Kit: "crate.stack", At: vec3{10.6, 0, -6.6}, Seed: 4},
			{Kit: "crate.stack", At: vec3{-7.2, 0, -7.0}, Seed: 3},
			{Kit: "crate.stack", At: vec3{-10.9, 0, -7.2}, Seed: 8},
			{Kit: "telescope", At: vec3{10.5, 3, -3.5}}, // between two mullions
			{Kit: "bench", At: vec3{9.4, 3, 2.4}},
			{Kit: "lowtable", At: vec3{10.5, 3, 2.4}},
		},
	},
	content.Blueprint{
		ID:     "core:research",
		Link:   "core:hall",
		Sign:   "RECHERCHE",
		Accent: "neon.blue",
		At:     vec3{-20, 0, 0},
		Yaw:    180,
		Shape:  content.Dome{Radius: 4, WallH: 2.6},
		Style:  "habitat.white",
		Doors:  []content.Door{{base.West}},
		// The console against the south wall, facing north: the way from
		// the hall to the hangar runs across the room.
		Props: []content.Prop{
			{Kit: "console", At: vec3{0, 0, -2.3}, Yaw: -90},
			{Kit: "chair", At: vec3{0.5, 0, -1.2}, Seed: 0, Yaw: -90},
			{Kit: "workbench", At: vec3{-0.6, 0, 3.0}, Seed: 6},
			{Kit: "planter", At: vec3{2.3, 0, 2.6}, Seed: 2},
		},
	},
	// The bedroom wing, at the end of a corridor north of the hall: one
	// room, warm and lived in, where the player wakes up. The bed's head
	// against the north wall under a porthole, the desk and the shelf
	// east, the lockers, the guitar and the armchair west.
	content.Blueprint{
		ID:      "core:sleep",
		Link:    "core:hall",
		Sign:    "CHAMBRE",
		Accent:  "neon.amber",
		At:      vec3{0, 0, 22.5},
		Hall:    &content.Hall{Size: [2]float64{7, 5}, WallH: 2.6, Rise: 1},
		Style:   "habitat.white",
		Light:   "warm",
		Windows: []content.Window{{At: vec3{0, 1.35, 2.5}, Width: 1.5, Height: 0.8}},
		Props: []content.Prop{
			{Kit: "rug", At: vec3{0, 0, -0.4}},
			{Kit: "bed", At: vec3{0, 0, 1.2}, Yaw: -90},
			{Kit: "nightstand", At: vec3{-1.15, 0, 2.0}, Yaw: -90},
			{Kit: "bedlamp", At: vec3{-1.25, 0.56, 2.1}},
			{Kit: "photo.earth", At: vec3{-1.02, 0.56, 1.9}, Yaw: -60},
			{Kit: "nightstand", At: vec3{1.15, 0, 2.0}, Yaw: -90},
			{Kit: "books", At: vec3{1.15, 0.56, 2.0}, Seed: 2},
			{Kit: "writing.desk", At: vec3{2.95, 0, -0.2}, Yaw: 180},
			{Kit: "plant", At: vec3{2.97, 0.76, 0.25}}, // grows toward the shelf, night after night
			{Kit: "chair", At: vec3{2.25, 0, -0.2}},
			{Kit: "shelf", At: vec3{3.12, 0, 1.35}, Yaw: 180},
			{Kit: "lockers", At: vec3{-3.03, 0, 1.4}},
			{Kit: "guitar", At: vec3{-2.98, 0, 0.05}},
			{Kit: "armchair", At: vec3{-2.45, 0, -1.5}, Yaw: 45},
			{Kit: "books", At: vec3{-1.8, 0, -2.0}, Seed: 4},
			{Kit: "garland", At: vec3{0, 2.45, 2.2}, Seed: 60},
		},
	},
	// The vehicles' hangar, west of the base, turned so its door (east in
	// its own frame) faces away; a corridor leads there from the research
	// room.
	content.Blueprint{
		ID:     "core:hangar",
		Link:   "core:research",
		Sign:   "HANGAR 01",
		Accent: "neon.orange",
		At:     vec3{-46, 0, 0},
		Yaw:    180,
		Hall:   &content.Hall{Size: [2]float64{26, 64}, WallH: 11, Rise: 5, Door: [2]float64{52, 10}},
		Garage: true,
		Style:  "habitat.white",
		// By the corridor's door, the terminal to buy and sell vehicles.
		Props: []content.Prop{{Kit: "garage.terminal", At: vec3{-12.4, 0, -2.25}}},
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
	"garage.terminal": {Build: base.KitBuild(base.BuildGarageTerminal), Obstacles: []content.Obstacle{{vec3{}, 0.6}},
		Reach: 2.6, Use: func(g content.Game, _ vec3) { g.OpenGarage() }, Prompt: "[E] Acheter ou revendre un véhicule"},
	"bed": {Build: base.KitBuild(base.BuildBed), Obstacles: []content.Obstacle{{vec3{-0.55, 0, 0}, 0.8}, {vec3{0.55, 0, 0}, 0.8}},
		Reach: 2.6, Use: func(g content.Game, _ vec3) { g.Sleep() }, Prompt: "[E] Dormir"},
	"nightstand":   {Build: base.KitBuild(base.BuildNightstand), Obstacles: []content.Obstacle{{vec3{}, 0.3}}},
	"bedlamp":      {Build: base.KitBuild(base.BuildBedLamp), Lamps: []content.KitLamp{{base.BedLampLight, 0.45, 4}}},
	"writing.desk": {Build: base.KitBuild(base.BuildWritingDesk), Obstacles: []content.Obstacle{{vec3{0, 0, -0.35}, 0.42}, {vec3{0, 0, 0.35}, 0.42}}, Lamps: []content.KitLamp{{base.DeskLight, 0.35, 3.5}}},
	"shelf":        {Build: base.KitBuild(base.BuildShelf), Obstacles: []content.Obstacle{{vec3{0, 0, -0.3}, 0.3}, {vec3{0, 0, 0.3}, 0.3}}},
	"armchair":     {Build: base.KitBuild(base.BuildArmchair), Obstacles: []content.Obstacle{{vec3{}, 0.5}}},
	"lockers":      {Build: base.KitBuild(base.BuildLockers), Obstacles: []content.Obstacle{{vec3{0, 0, -0.23}, 0.33}, {vec3{0, 0, 0.23}, 0.33}}},
	"rug":          {Build: base.KitBuild(base.BuildRug)},
	"garland":      {Build: base.KitBuild(base.BuildGarland), Lamps: []content.KitLamp{{base.GarlandLight, 0.3, 4.5}}},
	"guitar":       {Build: base.KitBuild(base.BuildGuitar), Obstacles: []content.Obstacle{{vec3{-0.05, 0, 0}, 0.22}}},
	"photo.earth":  {Build: base.KitBuild(base.BuildPhotoEarth)},
	"books":        {Build: base.KitBuild(base.BuildBooks)},
	"plant": {Build: base.KitBuild(base.BuildPlantPot), Stages: base.PlantStages,
		Grow: base.KitBuild(base.BuildPlantStage)},
	"crate.stack": {Build: base.KitBuild(base.BuildCrates), Obstacles: []content.Obstacle{{vec3{}, 0.55}}},
	"chair":       {Build: base.KitBuild(base.BuildChair), Obstacles: []content.Obstacle{{vec3{}, 0.35}}},
	"workbench":   {Build: base.KitBuild(base.BuildWorkbench), Obstacles: []content.Obstacle{{vec3{}, 0.8}}},
	"planter":     {Build: base.KitBuild(base.BuildPlanter), Obstacles: []content.Obstacle{{vec3{}, 0.85}}},
	"desk":        {Build: base.KitBuild(base.BuildDesk), Obstacles: []content.Obstacle{{vec3{}, 0.65}}},
	"bench":       {Build: base.KitBuild(base.BuildBench), Obstacles: []content.Obstacle{{vec3{0, 0, -0.5}, 0.35}, {vec3{0, 0, 0.5}, 0.35}}},
	"lowtable":    {Build: base.KitBuild(base.BuildLowTable), Obstacles: []content.Obstacle{{vec3{}, 0.45}}},
	"telescope": {Build: base.KitBuild(base.BuildTelescope), Obstacles: []content.Obstacle{{vec3{}, 0.5}},
		Reach: 2.2, Use: content.Game.LookThrough, Prompt: "[E] Regarder dans la longue-vue"},
}
