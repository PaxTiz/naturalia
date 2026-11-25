package core

import (
	"naturalia/internal/base"
	"naturalia/internal/content"
)

// registerCamps adds the bootcamp (docs/TELEPORTEURS.md): a small closed
// dome of the base, its door east (+X) at the end of a short airlock. The
// bed's head against the back wall, the hologram's table toward the door,
// the workbench along the north wall facing in, a plant and a lamp
// across.
func registerCamps(c *content.Content) {
	c.Kits.Add("core:camp.bench", content.Kit{Build: base.KitBuild(base.BuildWorkbench), Obstacles: []content.Obstacle{{vec3{}, 0.8}},
		Reach: 2.4, Use: func(g content.Game, _ vec3) { g.OpenShop() }, Prompt: "[E] Ouvrir le laboratoire de recherche"})
	c.Items.Add("core:bootcamp", content.Item{Name: "Bootcamp", Icon: "camp", Cost: 100_000_000, Max: 32, Places: "core:bootcamp",
		Desc: "Un petit dôme à poser loin (F) : y dormir, ouvrir le labo, voyager par l'hologramme"})
	c.Entities.Add("core:bootcamp", content.EntityDef{Name: "Bootcamp", Camp: &content.Blueprint{
		ID:       "core:bootcamp",
		Shape:    content.Dome{Radius: 4, WallH: 2.6},
		Style:    "habitat.white",
		Accent:   "neon.cyan",
		Interior: "home",
		Props: []content.Prop{
			{Kit: "rug", At: vec3{-2.2, 0, 0}},
			{Kit: "bed", At: vec3{-2.55, 0, 0}},
			{Kit: "maptable", At: vec3{0.9, 0, 0}},
			{Kit: "camp.bench", At: vec3{-0.6, 0, 2.95}, Seed: 6},
			{Kit: "plant.big", At: vec3{-1.9, 0, -2.9}, Seed: 3},
			{Kit: "lamp.floor", At: vec3{-0.4, 0, -3.35}},
		},
	}})

	registerBuildings(c)
}

// registerBuildings adds the buildings set down anywhere and linked
// together (docs/BATIMENTS.md): bought in the catalog, taken back there.
func registerBuildings(c *content.Content) {
	c.Kits.Add("core:kitchenette", content.Kit{Build: base.KitBuild(base.BuildKitchenette),
		Obstacles: obstacles([]vec3{{0, 0, -0.75}, {0, 0, 0}, {0, 0, 0.75}}, 0.42), Lamps: []content.KitLamp{{base.KitchenetteLight, 0.4, 3.5}}})
	c.Kits.Add("core:table.meal", content.Kit{Build: base.KitBuild(base.BuildMealTable),
		Obstacles: append(obstacles(base.MealObstacles, 0.2), content.Obstacle{Radius: 0.55}), Lamps: []content.KitLamp{{base.MealLight, 0.35, 3}}})
	c.Kits.Add("core:serre.plants", content.Kit{Build: base.KitBuild(base.BuildSerre), Obstacles: plantsObstacles(base.SerreBeds)})

	add := func(id, name, desc string, cost float64, ports []float64, bp content.Blueprint) {
		bp.ID = id
		c.Items.Add(id, content.Item{Name: name, Desc: desc, Icon: "camp", Cost: cost, Shelf: "buildings", Places: id})
		c.Entities.Add(id, content.EntityDef{Name: name, Item: id, Pickup: true, Prompt: "[E] Reprendre : " + name, Ports: ports, Camp: &bp})
	}
	// The crossroads: four ports around the hologram, benches and a plant
	// between them.
	add("core:carrefour", "Carrefour", "Un petit dôme à quatre prises, pour relier des bâtiments ; on y voyage par l'hologramme", 20_000_000,
		[]float64{0, 90, 180, 270}, content.Blueprint{
			Shape: content.Dome{Radius: 3, WallH: 2.6}, Style: "habitat.white", Accent: "neon.cyan", Interior: "home",
			Props: []content.Prop{
				{Kit: "maptable"},
				{Kit: "pouf", At: vec3{1.72, 0, 1.72}},
				{Kit: "plant.big", At: vec3{-1.78, 0, 1.78}, Seed: 2},
				{Kit: "pouf", At: vec3{-1.72, 0, -1.72}, Seed: 1},
				{Kit: "lamp.floor", At: vec3{1.78, 0, -1.78}},
			},
		})
	// The dwelling: the way through it along X, the bed on one side, the
	// kitchen and the table on the other.
	add("core:habitat", "Habitat", "Un dôme pour vivre : un lit, une cuisine, une table ; on s'y réveille après y avoir dormi", 60_000_000,
		[]float64{0, 180}, content.Blueprint{
			Shape: content.Dome{Radius: 4.5, WallH: 2.6}, Style: "habitat.white", Accent: "neon.amber", Interior: "home",
			Props: []content.Prop{
				{Kit: "rug", At: vec3{0, 0, -2.6}, Yaw: 90},
				{Kit: "bed", At: vec3{0, 0, -3.0}, Yaw: 90},
				{Kit: "lamp.floor", At: vec3{-1.35, 0, -3.75}},
				{Kit: "armchair", At: vec3{2.3, 0, -2.35}, Yaw: 135},
				{Kit: "plant.big", At: vec3{1.5, 0, -3.65}, Seed: 4},
				{Kit: "shelf", At: vec3{-2.75, 0, -2.75}, Yaw: 45, Seed: 2},
				{Kit: "kitchenette", At: vec3{0, 0, 3.6}, Yaw: -90, Seed: 3},
				{Kit: "table.meal", At: vec3{2.35, 0, 2.2}, Seed: 1},
				{Kit: "lockers", At: vec3{-2.65, 0, 2.75}, Yaw: -45, Seed: 1},
				{Kit: "books", At: vec3{1.5, 0, -2.9}, Seed: 2},
			},
		})
	// The greenhouse.
	add("core:serre", "Serre", "Un dôme de verre plein de plantes, que l'on traverse", 80_000_000,
		[]float64{0, 180}, content.Blueprint{
			Shape: content.Dome{Radius: 5, WallH: 2.8}, Style: "habitat.white", Accent: "led.green", Glazed: true,
			Props: []content.Prop{{Kit: "serre.plants", Seed: 5}},
		})
	// The links between them.
	c.Links.Add("core:sas", content.Link{Name: "SAS"})
	c.Items.Add("core:sas", content.Item{Name: "SAS transparent", Desc: "Un passage vitré d'un bâtiment à un autre : vise une porte, puis l'autre (3 à 30 m)", Icon: "crate",
		Cost: 500_000, Link: "sas", Shelf: "links"})

	// The garage: its big door toward whoever sets it down, two bays, the
	// terminal by the port at the back.
	add("core:garage", "Garage", "Un hall à grande porte et deux places : on y gare et on y fait venir ses véhicules", 120_000_000,
		[]float64{180}, content.Blueprint{
			Hall:  &content.Hall{Size: [2]float64{10, 12}, WallH: 4.2, Rise: 1.4, Door: [2]float64{8, 3.6}},
			Style: "clad.sand", Accent: "neon.orange", Garage: true,
			Props: []content.Prop{
				{Kit: "garage.terminal", At: vec3{-4.0, 0, 2.3}},
				{Kit: "workbench", At: vec3{-4.1, 0, -2.6}},
				{Kit: "crate.stack", At: vec3{-4.1, 0, 5.1}, Seed: 2},
				{Kit: "crate.stack", At: vec3{-4.1, 0, -5.1}, Seed: 5},
			},
		})
	// The observatory: the great telescope through its dome of glass, a
	// desk, a reading corner.
	c.Kits.Add("core:telescope.big", content.Kit{Build: base.KitBuild(base.BuildBigTelescope), Obstacles: []content.Obstacle{{vec3{0.1, 0, 0}, 0.62}},
		Reach: 2.3, Use: content.Game.LookThrough, Prompt: "[E] Regarder dans la lunette"})
	add("core:observatoire", "Observatoire", "Un dôme à coupole de verre et sa grande lunette, qui voit tout autour", 90_000_000,
		[]float64{0}, content.Blueprint{
			Shape: content.Dome{Radius: 4, WallH: 2.6}, Style: "habitat.white", Accent: "neon.blue", Interior: "home", Skylight: true,
			Props: []content.Prop{
				{Kit: "telescope.big", Yaw: 180},
				{Kit: "desk.wall", At: vec3{0, 0, 3.2}, Yaw: -90},
				{Kit: "chair", At: vec3{0, 0, 2.45}, Yaw: -90},
				{Kit: "shelf", At: vec3{-1.9, 0, 2.75}, Yaw: -55, Seed: 4},
				{Kit: "armchair", At: vec3{-0.4, 0, -2.85}, Yaw: 90},
				{Kit: "lamp.floor", At: vec3{-1.35, 0, -3.1}},
				{Kit: "books", At: vec3{0.65, 0, -3.0}, Seed: 1},
				{Kit: "plant.big", At: vec3{-3.1, 0, -1.1}, Seed: 6},
			},
		})
}
