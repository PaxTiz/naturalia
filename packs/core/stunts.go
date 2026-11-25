package core

import (
	"fmt"
	"math"
	"naturalia/internal/content"
)

// registerStunts adds what makes driving fun: ramps to build
// with, a boost pad to set down anywhere, and credits for long jumps, paid like a mod would.
func registerStunts(c *content.Content) {
	// One module: 6 m long, 4 m wide, levels of 2 m, as high as a big crate.
	// Their tops are smooth: no trim for the wheels to jump on.
	ramp := func(id, name, desc string, cost, height float64) {
		const lip = 0.2 // m: the hazard strip at its end, flush
		low := height * (6 - lip) / 6
		c.Entities.Add(id, content.EntityDef{Name: name, Pickup: true, Prompt: "Ramasser : " + name, Build: true, Paint: "core:steel",
			Cell: vec3{6, height, 4}, Ramp: &content.Ramp{Length: 6, Width: 4, Height: height},
			Parts: []content.PartJSON{
				{Shape: "wedge", Token: "steel", At: vec3{-lip / 2, 0, 0}, Size: vec3{6 - lip, low, 4}, Lit: "outdoor"},
				box("hazard", vec3{3 - lip/2, low / 2, 0}, vec3{lip, low, 4}, 0),
				{Shape: "wedge", Token: "hazard", At: vec3{3 - lip/2, low, 0}, Size: vec3{lip, height - low, 4}, Lit: "outdoor"},
				box("neon.orange", vec3{3, height - 0.3, 0}, vec3{0.02, 0.06, 3.8}, 0),
				box("steel.dark", vec3{0.3, 0.06, -2}, vec3{5.2, 0.12, 0.02}, 0),
				box("steel.dark", vec3{0.3, 0.06, 2}, vec3{5.2, 0.12, 0.02}, 0),
			}})
		c.Items.Add(id, content.Item{Name: name, Desc: desc, Icon: "ramp", Cost: cost, Places: id, Shelf: "climbs"})
	}
	ramp("core:ramp", "Tremplin", "6 × 4 m, monte de 2 m : vises-en le haut pour en ajouter un qui le prolonge", 2_000, 2)
	ramp("core:ramp-steep", "Tremplin raide", "6 × 4 m, monte de 4 m : à mettre au bout d'un autre, pour les très grands sauts", 3_500, 4)
	c.Entities.Add("core:ramp-base", content.EntityDef{Name: "Socle", Pickup: true, Prompt: "Ramasser : Socle", Build: true,
		Cell: vec3{6, 2, 4}, Paint: "core:steel", Parts: []content.PartJSON{
			box("steel", vec3{0, 1, 0}, vec3{6, 2, 4}, 0),
			box("steel.dark", vec3{0, 0.06, -2}, vec3{6, 0.12, 0.02}, 0),
			box("steel.dark", vec3{0, 0.06, 2}, vec3{6, 0.12, 0.02}, 0),
			box("hazard", vec3{0, 1.8, -2}, vec3{6, 0.2, 0.02}, 0),
			box("hazard", vec3{0, 1.8, 2}, vec3{6, 0.2, 0.02}, 0),
		}})
	c.Items.Add("core:ramp-base", content.Item{Name: "Socle", Desc: "6 × 4 m, 2 m de haut : porte un tremplin, ou continue sa hauteur",
		Icon: "crate", Cost: 500, Places: "core:ramp-base", Shelf: "climbs"})

	pad := []content.PartJSON{
		box("steel.dark", vec3{0, 0.04, 0}, vec3{3.2, 0.08, 2.6}, 0.02),
		box("hazard", vec3{0, 0.09, -1.2}, vec3{3.2, 0.02, 0.15}, 0),
		box("hazard", vec3{0, 0.09, 1.2}, vec3{3.2, 0.02, 0.15}, 0),
	}
	for _, x := range []float64{-1, 0, 1} { // chevrons pointing forward
		pad = append(pad,
			boxYaw("neon.cyan", vec3{x, 0.1, -0.45}, vec3{0.9, 0.02, 0.14}, 40),
			boxYaw("neon.cyan", vec3{x, 0.1, 0.45}, vec3{0.9, 0.02, 0.14}, -40))
	}
	for i := range pad {
		pad[i].Lit = "outdoor"
	}
	c.Entities.Add("core:pad", content.EntityDef{Name: "Accélérateur", Pickup: true, Prompt: "Ramasser : Accélérateur",
		Kick: 15, Parts: pad})
	c.Items.Add("core:pad", content.Item{Name: "Accélérateur", Desc: "Plaque à poser (F) : lance le véhicule qui passe dessus et remplit son boost",
		Icon: "pad", Cost: 5_000, Places: "core:pad", Shelf: "climbs"})

	// A jump of over a second pays, more for dearer vehicles; what flies
	// doesn't count.
	c.On(content.EvLand, func(g content.Game, e *content.Event) {
		v, ok := c.Vehicles.Get(e.ID)
		if !ok || v.Fly != nil || e.Amount < 1 {
			return
		}
		gain := math.Round(math.Max(10, v.Cost*0.002) * e.Amount * e.Amount)
		g.Say(fmt.Sprintf("Saut de %.1f s !", e.Amount))
		g.GiveCredits(gain)
	})
}
