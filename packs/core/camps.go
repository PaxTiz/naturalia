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
}
