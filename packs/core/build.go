package core

import "naturalia/internal/content"

// registerBuild adds the blocks to build with (docs/CONSTRUCTION.md):
// set down anywhere, turned any way, they stay put; the player walks and
// jumps on them, vehicles bounce off them.
func registerBuild(c *content.Content) {
	block := func(id, name, desc, icon string, cost float64, parts []content.PartJSON) {
		c.Entities.Add(id, content.EntityDef{Name: name, Pickup: true, Prompt: "Ramasser : " + name, Build: true, Parts: parts})
		c.Items.Add(id, content.Item{Name: name, Desc: desc, Icon: icon, Cost: cost, Places: id})
	}
	block("core:crate-2", "Grosse caisse", "2 m de côté, fixe une fois posée : on monte dessus, on empile", "crate", 120,
		crateParts(2, "crate.slate"))
	block("core:crate-4", "Très grosse caisse", "4 m de côté, fixe : de quoi bâtir vite", "crate", 400,
		crateParts(4, "crate.sand"))
	block("core:slab", "Dalle", "4 × 4 m, 20 cm d'épaisseur : sols, toits, ponts", "crate", 300, []content.PartJSON{
		box("pad.concrete", vec3{0, 0.1, 0}, vec3{4, 0.2, 4}, 0.02),
		box("steel.dark", vec3{0, 0.1, -1.97}, vec3{4.02, 0.22, 0.08}, 0),
		box("steel.dark", vec3{0, 0.1, 1.97}, vec3{4.02, 0.22, 0.08}, 0),
		box("steel.dark", vec3{-1.97, 0.1, 0}, vec3{0.08, 0.22, 4.02}, 0),
		box("steel.dark", vec3{1.97, 0.1, 0}, vec3{0.08, 0.22, 4.02}, 0),
	})
	// Its front faces the player: a wall shows them its face, a beam lies
	// across, a slope rises away from them.
	block("core:wall", "Mur", "4 m de long, 2,5 m de haut : parois, abris", "crate", 300, []content.PartJSON{
		box("habitat.white", vec3{0, 1.25, 0}, vec3{0.2, 2.5, 4}, 0.02),
		box("steel.dark", vec3{0, 0.06, 0}, vec3{0.22, 0.12, 4.02}, 0),
		box("steel.dark", vec3{0, 2.44, 0}, vec3{0.22, 0.12, 4.02}, 0),
		box("neon.cyan", vec3{0.105, 1.6, 0}, vec3{0.01, 0.04, 3.2}, 0),
		box("neon.cyan", vec3{-0.105, 1.6, 0}, vec3{0.01, 0.04, 3.2}, 0),
	})
	block("core:beam", "Poutre", "4 m d'acier : piliers, garde-corps, charpentes", "crate", 80, []content.PartJSON{
		box("steel", vec3{0, 0.125, 0}, vec3{0.25, 0.25, 4}, 0.02),
		box("hazard", vec3{0, 0.125, -1.9}, vec3{0.26, 0.26, 0.2}, 0),
		box("hazard", vec3{0, 0.125, 1.9}, vec3{0.26, 0.26, 0.2}, 0),
	})
	block("core:slope", "Pente", "4 × 2 m, monte de 1 m : à pied comme en véhicule", "ramp", 250, []content.PartJSON{
		{Shape: "wedge", Token: "pad.concrete", Size: vec3{4, 1, 2}, Yaw: 180, Lit: "outdoor"},
		box("hazard", vec3{-1.95, 1, 0}, vec3{0.1, 0.04, 2.02}, 0),
		box("steel.dark", vec3{0, 0.03, -1.02}, vec3{4, 0.06, 0.06}, 0),
		box("steel.dark", vec3{0, 0.03, 1.02}, vec3{4, 0.06, 0.06}, 0),
	})
}
