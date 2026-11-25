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
	// The research lab moved into a corner of the great hall.
	c.Buildings.Alias("core:research", "core:hall")

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
	c.Items.Add("core:crate", content.Item{Name: "Caisse", Desc: "Du matériel à poser (F) : les véhicules la bousculent. Pour bâtir, prends plutôt le Bloc",
		Icon: "crate", Cost: 25, Places: "core:crate"})

	registerVehicles(c)
	registerStunts(c)
	registerBuild(c)
	registerCamps(c)

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
	// The working buildings' cladding, a warm steel grey that sinks into
	// the plain where the living spaces stay white.
	"clad.sand":     {color.RGBA{148, 140, 126, 255}, content.MtPanel, 1, 0.45, 0},
	"clad.sand2":    {color.RGBA{138, 131, 118, 255}, content.MtPanel, 1, 0.45, 0},
	"steel":         {color.RGBA{140, 146, 158, 255}, content.MtMetal, 1.2, 0.6, 0},
	"steel.dark":    {color.RGBA{50, 54, 66, 255}, content.MtPlain, 1, 0.4, 0},
	"door.plate":    {color.RGBA{168, 172, 178, 255}, content.MtMetal, 1.2, 0.45, 0}, // the hangar's door leaves
	"door.plate2":   {color.RGBA{154, 159, 166, 255}, content.MtMetal, 1.2, 0.45, 0},
	"rack.slate":    {color.RGBA{58, 64, 80, 255}, content.MtPanel, 1, 0.4, 0},
	"hazard":        {palette.White, content.MtHazard, 2.5, 0.3, 0},
	"neon.cyan":     {palette.AccentCyan, content.MtPlain, 1, 0, 0.8},
	"neon.orange":   {palette.AccentOrange, content.MtPlain, 1, 0, 0.8},
	"neon.blue":     {palette.AccentBlue, content.MtPlain, 1, 0, 0.8},
	"neon.purple":   {palette.AccentPurple, content.MtPlain, 1, 0, 0.8},
	"lamp.warm":     {palette.LampWarm, content.MtPlain, 1, 0, 1},
	"floor.slate":   {color.RGBA{50, 54, 66, 255}, content.MtPanel, 1, 0.25, 0},
	"wall.lower":    {color.RGBA{86, 100, 118, 255}, content.MtPanel, 1, 0.3, 0},
	"floor.inlay":   {color.RGBA{68, 74, 90, 255}, content.MtPlain, 1, 0.3, 0},
	"solar.blue":    {color.RGBA{40, 62, 130, 255}, content.MtPlain, 1, 0.9, 0},
	"crate.ochre":   {color.RGBA{196, 132, 60, 255}, content.MtPanel, 1, 0.2, 0},
	"crate.slate":   {color.RGBA{88, 116, 146, 255}, content.MtPanel, 1, 0.2, 0},
	"crate.sand":    {color.RGBA{158, 148, 118, 255}, content.MtPanel, 1, 0.2, 0},
	"wood.brown":    {color.RGBA{110, 80, 60, 255}, content.MtPlain, 1, 0.3, 0},
	"fabric.white":  {palette.MatWhite, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.purple": {palette.AccentPurple, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.rug":    {color.RGBA{150, 70, 60, 255}, content.MtFabric, 1 / content.FabricTile, 0, 0},
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
	"lamp.cold":    {color.RGBA{214, 232, 255, 255}, content.MtPlain, 1, 0, 0.9},
	"neon.amber":   {color.RGBA{255, 170, 80, 255}, content.MtPlain, 1, 0, 0.9},
	"rock.red":     {color.RGBA{150, 72, 48, 255}, content.MtPlain, 1, 0.1, 0},
	"earth.blue":   {color.RGBA{50, 110, 205, 255}, content.MtPlain, 1, 0.5, 0.15},
	"earth.green":  {color.RGBA{96, 150, 80, 255}, content.MtPlain, 1, 0.5, 0.1},
	"clay":         {color.RGBA{176, 98, 64, 255}, content.MtPlain, 1, 0.15, 0},
	// The greenhouse: more greens, and flowers.
	"leaf.dark":    {color.RGBA{46, 110, 62, 255}, content.MtPlain, 1, 0.2, 0},
	"leaf.olive":   {color.RGBA{118, 136, 62, 255}, content.MtPlain, 1, 0.2, 0},
	"bloom.coral":  {color.RGBA{232, 108, 86, 255}, content.MtPlain, 1, 0.2, 0},
	"bloom.yellow": {color.RGBA{238, 198, 78, 255}, content.MtPlain, 1, 0.2, 0},
	// The living spaces' finishes (interior: "home"): off-white composite
	// panels, smooth white ribs, a warm grey matte floor and its lighter
	// paths.
	"home.panel":  {color.RGBA{230, 226, 218, 255}, content.MtPlain, 1, 0.35, 0},
	"home.panel2": {color.RGBA{218, 214, 206, 255}, content.MtPlain, 1, 0.35, 0},
	"home.rib":    {color.RGBA{246, 245, 240, 255}, content.MtPlain, 1, 0.45, 0},
	"home.floor":  {color.RGBA{104, 94, 86, 255}, content.MtPaint, 1 / content.PaintTile, 0.2, 0},
	"home.path":   {color.RGBA{104, 99, 94, 255}, content.MtPanel, 1, 0.15, 0},
	// Its joinery: rounded anthracite profiles, glass guard rails, the
	// warm white of the lights built in.
	"home.trim":      {color.RGBA{56, 58, 64, 255}, content.MtPlain, 1, 0.5, 0},
	"home.led":       {color.RGBA{255, 228, 194, 255}, content.MtPlain, 1, 0, 1},
	"home.led.amber": {color.RGBA{255, 178, 100, 255}, content.MtPlain, 1, 0, 0.9},
	// The accent of the living spaces, round their doorways: a warm amber,
	// where the base has its cyan.
	"home.accent": {color.RGBA{255, 160, 96, 255}, content.MtPlain, 1, 0, 0.9},
	// Their warm materials: a ceramic fired on site from the regolith's
	// clay, in three tones; acoustic felt panels; a woollen carpet.
	"ceramic":         {color.RGBA{168, 110, 88, 255}, content.MtPaint, 1 / content.PaintTile, 0.35, 0},
	"ceramic.dark":    {color.RGBA{146, 96, 80, 255}, content.MtPaint, 1 / content.PaintTile, 0.35, 0},
	"ceramic.sand":    {color.RGBA{184, 140, 112, 255}, content.MtPaint, 1 / content.PaintTile, 0.35, 0},
	"felt.terracotta": {color.RGBA{168, 92, 68, 255}, content.MtFabric, 1 / content.FabricTile, 0, 0},
	"felt.sand":       {color.RGBA{198, 172, 136, 255}, content.MtFabric, 1 / content.FabricTile, 0, 0},
	"felt.sage":       {color.RGBA{124, 140, 116, 255}, content.MtFabric, 1 / content.FabricTile, 0, 0},
	"carpet.oat":      {color.RGBA{170, 154, 132, 255}, content.MtFabric, 1 / content.FabricTile, 0, 0},
	// Its soft furnishings: warm charcoal and oat cushions, cushions and
	// throws of ochre and rust, a rug of terracotta round sand.
	"crystal.blue": {color.RGBA{120, 200, 255, 255}, content.MtPlain, 1, 0.8, 0.35},
	"fabric.slate": {color.RGBA{84, 80, 78, 255}, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.oat":   {color.RGBA{196, 182, 160, 255}, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.ochre": {color.RGBA{204, 146, 70, 255}, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.rust":  {color.RGBA{164, 82, 58, 255}, content.MtFabric, 1 / content.FabricTile, 0.1, 0},
	"fabric.felt":  {color.RGBA{156, 90, 68, 255}, content.MtFabric, 1 / content.FabricTile, 0, 0},
	"fabric.felt2": {color.RGBA{196, 170, 136, 255}, content.MtFabric, 1 / content.FabricTile, 0, 0},
	"glass.rail":   {color.RGBA{185, 212, 218, 255}, content.MtPlain, 1, 1.0, 0},
}

// coreBuildings: the great hall first, the bedroom, then the hangar. The
// hall's south door leads to the airlock; its east wall is kept for the
// bay window. The hangar stands west, turned half a turn (Yaw): its door
// faces away.
var coreBuildings = []content.Blueprint{
	// The hub, the sale room and the research lab in one: the map table
	// in the middle, the incinerator in the south-west corner and the lab
	// in the north-west one, under their signs, the lounge north-east.
	content.Blueprint{
		ID:     "core:hall",
		Accent: "neon.cyan",
		// Its roof rises toward the bay window, toward the sunrise.
		Hall:  &content.Hall{Size: [2]float64{24, 16}, WallH: 5.5, Rise: 3.5, Roof: "shed"},
		Style: "habitat.white",
		// A living space: finished like a home inside.
		Interior: "home",
		// The bay window, all along the east wall, nearly up to the roof:
		// the plain, the sunrise.
		Windows: []content.Window{{At: vec3{12, 0.4, 0}, Width: 14, Height: 7.8}},
		// The lookout: a gallery 3 m up along the bay window, stairs
		// rising east to it along the south wall.
		Floors: []content.Floor{
			{From: vec3{9.75, 3, -7.8}, To: vec3{9.75, 3, 7.8}, Width: 4, Thick: 0.3},
			{From: vec3{2.2, 0, -6.9}, To: vec3{7.75, 3, -6.9}, Width: 1.3},
		},
		Signs: []content.Sign{
			{Text: "VENTE", At: vec3{-11.74, 3.6, -6.3}, Facing: base.East, Width: 2.6, Height: 0.6, Accent: "neon.orange"},
			{Text: "RECHERCHE", At: vec3{-8.2, 3.95, 7.74}, Facing: base.South, Width: 3.4, Height: 0.6, Accent: "neon.blue"},
		},
		Props: []content.Prop{
			{Kit: "maptable"},
			{Kit: "incinerator", At: vec3{-10, 0, -5}},
			// Built-in storage along the walls, either side of the
			// bedroom's door.
			// Its zones, by their floors and walls: the entry by the
			// airlock's door tiled in the ceramic fired on site, a wall of
			// it beside; the lab on a technical floor, sage felt over the
			// console; terracotta felt behind the lounge.
			{Kit: "floor.ceramic", At: vec3{-1.5, 0, -6.28}, Seed: 3},
			{Kit: "ceramic.wall", At: vec3{-3.2, 0, -7.78}, Yaw: 90, Seed: 5},
			{Kit: "entry", At: vec3{-3.2, 0, -7.76}, Yaw: 90},
			{Kit: "floor.tech", At: vec3{-9.28, 0, 5.28}},
			{Kit: "felt.wall", At: vec3{-8.2, 1.0, 7.78}, Yaw: -90, Seed: 2},
			{Kit: "felt.wall", At: vec3{6, 0.15, 7.78}, Yaw: -90},
			{Kit: "storage", At: vec3{-3, 0, 7.53}, Yaw: -90, Seed: 2},
			{Kit: "storage", At: vec3{3, 0, 7.53}, Yaw: -90, Seed: 3},
			// The research lab, in the north-west corner: the console
			// against the north wall, the bench and the samples' showcase
			// against the west one, a planter between it and the way to
			// the hangar.
			{Kit: "console", At: vec3{-8.2, 0, 7.38}, Yaw: -90},
			{Kit: "chair", At: vec3{-7.7, 0, 6.3}, Yaw: -90},
			{Kit: "workbench", At: vec3{-11.5, 0, 4.2}},
			{Kit: "showcase", At: vec3{-11.6, 0, 6.2}, Seed: 1},
			{Kit: "planter.home", At: vec3{-7, 0, 2.0}, Yaw: 90, Seed: 2},
			// The lounge, in the north-east corner, open toward the bay
			// window.
			{Kit: "rug.felt", At: vec3{5, 0, 3.8}},
			{Kit: "sofa.arc", At: vec3{5, 0, 3.8}, Yaw: -45},
			{Kit: "coffee.table", At: vec3{5, 0, 3.8}},
			{Kit: "pouf", At: vec3{6.22, 0, 3.36}},
			{Kit: "pouf", At: vec3{5.44, 0, 2.58}, Seed: 1},
			{Kit: "lamp.arc", At: vec3{3.3, 0, 5.55}, Yaw: -45},
			// A canopy over it lowers the ceiling there; low bookcases
			// close it from the map table and the lab from the middle.
			{Kit: "canopy", At: vec3{5, 3.0, 3.8}},
			{Kit: "divider", At: vec3{2.0, 0, 3.6}, Seed: 1},
			{Kit: "divider", At: vec3{-6.3, 0, 4.2}, Seed: 3},
			// Lamps low down: by the entry, by the lookout's bench.
			{Kit: "lamp.floor", At: vec3{-4.75, 0, -7.3}},
			{Kit: "lamp.floor", At: vec3{11.35, 3, 1.4}},
			// Hydroponic planters under the lookout, along the bay window.
			{Kit: "planter.home", At: vec3{10.8, 0, -3}, Seed: 1},
			{Kit: "planter.home", At: vec3{10.8, 0, -1}, Seed: 4},
			{Kit: "planter.home", At: vec3{10.8, 0, 1}, Seed: 7},
			{Kit: "planter.home", At: vec3{10.8, 0, 3}, Seed: 2},
			// Life: curtains drawn back at the bay's ends, plants hanging
			// under the lookout and standing in corners, pictures going up
			// along the stairs, and on the way to the bedroom.
			{Kit: "curtain", At: vec3{11.72, 2.62, -6.55}, Yaw: 180},
			{Kit: "curtain", At: vec3{11.72, 2.62, 6.55}, Yaw: 180},
			{Kit: "curtain.tall", At: vec3{11.72, 8.15, -6.55}, Yaw: 180},
			{Kit: "curtain.tall", At: vec3{11.72, 8.15, 6.55}, Yaw: 180},
			{Kit: "plant.hanging", At: vec3{8.3, 2.3, -4.6}, Seed: 1},
			{Kit: "plant.hanging", At: vec3{8.3, 2.3, -0.4}, Seed: 2},
			{Kit: "plant.hanging", At: vec3{8.3, 2.3, 4.2}, Seed: 3},
			{Kit: "plant.big", At: vec3{7.3, 0, 7.1}, Seed: 1},
			{Kit: "plant.big", At: vec3{-11.25, 0, 2.3}, Seed: 2},
			{Kit: "frame", At: vec3{3.6, 2.35, -7.78}, Yaw: 90, Seed: 0},
			{Kit: "frame", At: vec3{5.0, 3.1, -7.78}, Yaw: 90, Seed: 2},
			{Kit: "frame", At: vec3{6.4, 3.85, -7.78}, Yaw: 90, Seed: 1},
			{Kit: "frame", At: vec3{-1.03, 1.55, 11.2}, Seed: 3},
			{Kit: "frame", At: vec3{1.03, 1.55, 13.8}, Yaw: 180, Seed: 1},
			{Kit: "telescope", At: vec3{10.5, 3, -3.5}}, // between two mullions
			{Kit: "bench", At: vec3{9.4, 3, 2.4}},
			{Kit: "lowtable", At: vec3{10.5, 3, 2.4}},
			// On the roof's low half, solar panels facing the setting sun,
			// and two extractors.
			{Kit: "solar.rack", At: vec3{-8, 6.32, -3.6}},
			{Kit: "solar.rack", At: vec3{-8, 6.32, 2.2}},
			{Kit: "roof.vent", At: vec3{-3, 7.0, 5.6}},
			{Kit: "roof.vent", At: vec3{-3, 7.0, -6.0}, Seed: 1},
			// Outside, the life support's annex, between the greenhouse
			// and the hangar.
			{Kit: "annex.tech", At: vec3{-16.5, 0, -12}},
		},
	},
	// The bedroom wing, at the end of a corridor north of the hall: one
	// room, warm and lived in, where the player wakes up. The bed's head
	// against the north wall under a porthole, the desk and the shelf
	// east, the lockers, the guitar and the armchair west.
	content.Blueprint{
		ID:     "core:sleep",
		Link:   "core:hall",
		Sign:   "CHAMBRE",
		Accent: "neon.amber",
		At:     vec3{0, 0, 19.5}, // 9 m of corridor from the hall
		// A roof of its own, a small slope rising east like the hall's.
		Hall:     &content.Hall{Size: [2]float64{7, 5}, WallH: 2.6, Rise: 1.4, Roof: "shed"},
		Style:    "habitat.white",
		Light:    "warm",
		Interior: "home",
		Windows:  []content.Window{{At: vec3{0, 1.35, 2.5}, Width: 1.5, Height: 0.8}},
		Props: []content.Prop{
			// Built in, like the rest of the habitat: the bed in a niche
			// between two wardrobes, a desk shelf and a bookcase east, a
			// reading seat west. From Earth, a few things only: the
			// photo, the guitar, real books, the plant, the rug, the
			// throw, the garland.
			{Kit: "rug", At: vec3{0, 0, -0.4}},
			{Kit: "bed.niche", At: vec3{0, 0, 1.97}, Yaw: -90},
			{Kit: "bed", At: vec3{0, 0, 1.2}, Yaw: -90},
			{Kit: "photo.earth", At: vec3{-1.08, 0.56, 2.1}, Yaw: -60},
			{Kit: "books", At: vec3{1.0, 0.56, 2.06}, Seed: 2},
			{Kit: "desk.wall", At: vec3{2.95, 0, -0.2}, Yaw: 180},
			{Kit: "plant", At: vec3{2.97, 0.76, 0.25}}, // grows toward the shelf, night after night
			{Kit: "chair", At: vec3{2.25, 0, -0.2}},
			{Kit: "shelf", At: vec3{3.12, 0, 1.35}, Yaw: 180},
			{Kit: "banquette", At: vec3{-2.97, 0, -0.9}},
			{Kit: "felt.low", At: vec3{-3.28, 1.0, -0.9}, Seed: 1},
			{Kit: "guitar", At: vec3{-2.98, 0, 0.9}},
			{Kit: "books", At: vec3{-1.8, 0, -2.0}, Seed: 4},
			{Kit: "garland", At: vec3{0, 2.45, 1.62}, Seed: 44},
		},
	},
	// The greenhouse, on the south side where one arrives, west of the
	// entry: a geodesic dome of glass full of plants, green seen from
	// afar, a short corridor from the great hall.
	content.Blueprint{
		ID:     "core:greenhouse",
		Link:   "core:hall",
		Sign:   "SERRE",
		Accent: "led.green",
		At:     vec3{-8, 0, -14.5},
		Shape:  content.Dome{Radius: 4.5, WallH: 2.8},
		Style:  "habitat.white",
		Glazed: true,
		Props:  []content.Prop{{Kit: "greenhouse.plants", Seed: 2}},
	},
	// The vehicles' hangar, west of the base, turned so its door (east in
	// its own frame) faces away; a corridor leads there from the great
	// hall. Clad in steel grey, just long and high enough for the fleet in
	// one row: the great hall stands in front of it.
	content.Blueprint{
		ID:     "core:hangar",
		Link:   "core:hall",
		Sign:   "HANGAR 01",
		Accent: "neon.orange",
		At:     vec3{-33, 0, 0}, // 10 m of corridor from the hall
		Yaw:    180,
		Hall:   &content.Hall{Size: [2]float64{22, 58}, WallH: 7.5, Rise: 3, Door: [2]float64{50, 7}},
		Garage: true,
		Style:  "clad.sand",
		// By the corridor's door, the terminal to buy and sell vehicles.
		Props: []content.Prop{{Kit: "garage.terminal", At: vec3{-12.4, 0, -2.25}}},
	},
}

var coreKits = map[string]content.Kit{
	"maptable": {Build: base.KitBuild(base.BuildMapTable), Obstacles: []content.Obstacle{{vec3{}, 1.25}},
		Reach: 2.6, Use: func(g content.Game, _ vec3) { g.OpenMap() }, Prompt: "[E] Consulter l'état de Mars"},
	"incinerator": {Build: base.KitBuild(base.BuildIncinerator), Obstacles: []content.Obstacle{{vec3{0, 0, -0.2}, 1.3}, {vec3{0, 0, 0.8}, 1.3}},
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
	"lamp.arc":     {Build: base.KitBuild(base.BuildArcLamp), Obstacles: []content.Obstacle{{vec3{}, 0.25}}, Lamps: []content.KitLamp{{base.ArcLight, 1.1, 6}}},
	"lamp.floor":   {Build: base.KitBuild(base.BuildFloorLamp), Obstacles: []content.Obstacle{{vec3{}, 0.22}}, Lamps: []content.KitLamp{{base.FloorLight, 0.7, 5}}},
	"canopy":       {Build: base.KitBuild(base.BuildCanopy)},
	"divider":      {Build: base.KitBuild(base.BuildDivider), Obstacles: obstacles(base.DividerObstacles, 0.3)},
	"entry":        {Build: base.KitBuild(base.BuildEntry), Obstacles: obstacles(base.EntryObstacles, 0.32)},
	"pouf":         {Build: base.KitBuild(base.BuildPouf), Obstacles: []content.Obstacle{{vec3{}, 0.3}}},
	"frame":        {Build: base.KitBuild(base.BuildFrame)},
	"curtain":      {Build: base.KitBuild(base.BuildCurtain)},
	"curtain.tall": {Build: base.KitBuild(base.BuildTallCurtain)},
	// Outside: on the roofs and around the buildings.
	"solar.rack":        {Build: base.KitBuild(base.BuildSolarRack)},
	"roof.vent":         {Build: base.KitBuild(base.BuildRoofVent)},
	"greenhouse.plants": {Build: base.KitBuild(base.BuildGreenhousePlants), Obstacles: plantsObstacles()},
	"annex.tech":        {Build: base.KitBuild(base.BuildAnnex), Obstacles: obstacles(base.AnnexObstacles, 1.9)},
	"plant.hanging":     {Build: base.KitBuild(base.BuildHangingPlant)},
	"plant.big":         {Build: base.KitBuild(base.BuildBigPlant), Obstacles: []content.Obstacle{{vec3{}, 0.3}}},
	"felt.wall":         {Build: base.KitBuild(base.BuildFeltWall)},
	"felt.low":          {Build: base.KitBuild(base.BuildFeltLow)},
	"ceramic.wall":      {Build: base.KitBuild(base.BuildCeramicWall)},
	"floor.ceramic":     {Build: base.KitBuild(base.BuildCeramicFloor)},
	"floor.tech":        {Build: base.KitBuild(base.BuildTechFloor)},
	"bed.niche": {Build: base.KitBuild(base.BuildBedNiche), Obstacles: obstacles(base.NicheObstacles, 0.33),
		Lamps: []content.KitLamp{{base.NicheLights[0], 0.3, 3}, {base.NicheLights[1], 0.3, 3}}},
	"desk.wall":   {Build: base.KitBuild(base.BuildWallDesk), Obstacles: []content.Obstacle{{vec3{0, 0, -0.35}, 0.38}, {vec3{0, 0, 0.35}, 0.38}}, Lamps: []content.KitLamp{{base.DeskLight, 0.35, 3.5}}},
	"banquette":   {Build: base.KitBuild(base.BuildBanquette), Obstacles: obstacles(base.BanquetteObstacles, 0.35)},
	"rug":         {Build: base.KitBuild(base.BuildRug)},
	"garland":     {Build: base.KitBuild(base.BuildGarland), Lamps: []content.KitLamp{{base.GarlandLight, 0.3, 4.5}}},
	"guitar":      {Build: base.KitBuild(base.BuildGuitar), Obstacles: []content.Obstacle{{vec3{-0.05, 0, 0}, 0.22}}},
	"photo.earth": {Build: base.KitBuild(base.BuildPhotoEarth)},
	"books":       {Build: base.KitBuild(base.BuildBooks)},
	"plant": {Build: base.KitBuild(base.BuildPlantPot), Stages: base.PlantStages,
		Grow: base.KitBuild(base.BuildPlantStage)},
	"crate.stack":  {Build: base.KitBuild(base.BuildCrates), Obstacles: []content.Obstacle{{vec3{}, 0.55}}},
	"chair":        {Build: base.KitBuild(base.BuildChair), Obstacles: []content.Obstacle{{vec3{}, 0.35}}},
	"workbench":    {Build: base.KitBuild(base.BuildWorkbench), Obstacles: []content.Obstacle{{vec3{}, 0.8}}},
	"planter":      {Build: base.KitBuild(base.BuildPlanter), Obstacles: []content.Obstacle{{vec3{}, 0.85}}},
	"desk":         {Build: base.KitBuild(base.BuildDesk), Obstacles: []content.Obstacle{{vec3{}, 0.65}}},
	"bench":        {Build: base.KitBuild(base.BuildBench), Obstacles: []content.Obstacle{{vec3{0, 0, -0.5}, 0.35}, {vec3{0, 0, 0.5}, 0.35}}},
	"lowtable":     {Build: base.KitBuild(base.BuildLowTable), Obstacles: []content.Obstacle{{vec3{}, 0.45}}},
	"showcase":     {Build: base.KitBuild(base.BuildShowcase), Obstacles: []content.Obstacle{{vec3{0, 0, -0.3}, 0.4}, {vec3{0, 0, 0.3}, 0.4}}},
	"sofa.arc":     {Build: base.KitBuild(base.BuildSofaArc), Obstacles: obstacles(base.SofaObstacles(), 0.45)},
	"coffee.table": {Build: base.KitBuild(base.BuildCoffeeTable), Obstacles: []content.Obstacle{{vec3{}, 0.6}}},
	"rug.felt":     {Build: base.KitBuild(base.BuildFeltRug)},
	"storage":      {Build: base.KitBuild(base.BuildStorage), Obstacles: obstacles(base.StorageObstacles, 0.45)},
	"planter.home": {Build: base.KitBuild(base.BuildHomePlanter), Obstacles: []content.Obstacle{{vec3{0, 0, -0.5}, 0.45}, {vec3{0, 0, 0.5}, 0.45}},
		Lamps: []content.KitLamp{{base.PlanterLight, 0.3, 3.5}}},
	"telescope": {Build: base.KitBuild(base.BuildTelescope), Obstacles: []content.Obstacle{{vec3{}, 0.5}},
		Reach: 2.2, Use: content.Game.LookThrough, Prompt: "[E] Regarder dans la longue-vue"},
}

// obstacles are round obstacles of radius r at the points at.
// plantsObstacles: the greenhouse's beds.
func plantsObstacles() []content.Obstacle {
	at, radius := base.PlantsObstacles()
	var out []content.Obstacle
	for i, p := range at {
		out = append(out, content.Obstacle{At: p, Radius: radius[i]})
	}
	return out
}

func obstacles(at []vec3, r float64) []content.Obstacle {
	var out []content.Obstacle
	for _, p := range at {
		out = append(out, content.Obstacle{At: p, Radius: r})
	}
	return out
}
