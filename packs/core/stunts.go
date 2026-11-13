package core

import (
	"fmt"
	"math"
	"naturalia/internal/content"
)

// registerStunts adds what makes driving fun: a ramp and a boost pad to
// set down anywhere, and credits for long jumps, paid like a mod would.
func registerStunts(c *content.Content) {
	c.Entities.Add("core:ramp", content.EntityDef{Name: "Tremplin", Pickup: true, Prompt: "Ramasser : Tremplin",
		Ramp: &content.Ramp{Length: 6, Width: 4, Height: 1.8},
		Parts: []content.PartJSON{
			{Shape: "wedge", Token: "steel", Size: vec3{6, 1.8, 4}, Lit: "outdoor"},
			box("hazard", vec3{2.9, 1.8, 0}, vec3{0.2, 0.06, 4.02}, 0),
			box("neon.orange", vec3{2.99, 1.65, 0}, vec3{0.03, 0.06, 3.8}, 0),
			box("steel.dark", vec3{0, 0.04, -2.05}, vec3{6, 0.08, 0.1}, 0),
			box("steel.dark", vec3{0, 0.04, 2.05}, vec3{6, 0.08, 0.1}, 0),
		}})
	c.Items.Add("core:ramp", content.Item{Name: "Tremplin", Desc: "À poser (F) devant toi : prends-le en véhicule, à fond",
		Icon: "ramp", Cost: 2_000, Places: "core:ramp"})

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
		Icon: "pad", Cost: 5_000, Places: "core:pad"})

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
