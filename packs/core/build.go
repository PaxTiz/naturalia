package core

import (
	"image/color"
	"math"
	"naturalia/internal/content"
)

// registerBuild adds the blocks to build with (docs/CONSTRUCTION.md):
// set down anywhere, turned any way, they stay put; the player walks and
// jumps on them, vehicles bounce off them.
//
// They go together on a 4 m module: walls 2.5 m high under slabs 20 cm
// thick, a storey of 2.7 m; roofs rising 2 m over 4; volumes of 1, 2 and
// 4 m. A block's front (+X) faces the player: a wall shows them its face,
// what climbs rises away from them (toward -X).
func registerBuild(c *content.Content) {
	registerFinishes(c)
	defer registerLamps(c)
	// What a block is made of, how much of the grid it fills, and the token
	// its finish replaces. What isn't a plain box has a shape.
	block := func(shelf, id, name, desc, icon string, cost float64, cell vec3, paint string, parts []content.PartJSON, more ...func(e *content.EntityDef)) {
		e := content.EntityDef{Name: name, Pickup: true, Prompt: "Ramasser : " + name, Build: true, Cell: cell, Paint: "core:" + paint, Parts: parts}
		for _, f := range more {
			f(&e)
		}
		c.Entities.Add(id, e)
		c.Items.Add(id, content.Item{Name: name, Desc: desc, Icon: icon, Cost: cost, Places: id, Shelf: shelf})
	}
	shape := func(parts ...content.PartJSON) func(e *content.EntityDef) {
		return func(e *content.EntityDef) { e.Shape = parts }
	}
	rises := func(e *content.EntityDef) { e.Rise = vec3{-1, 0, 0} }

	// Volumes.
	crate := func(id, name, desc string, s, cost float64, tok string) {
		block("blocks", id, name, desc, "crate", cost, vec3{s, s, s}, tok, crateParts(s, tok))
	}
	crate("core:crate-1", "Bloc", "1 m de côté, fixe une fois posé : marches, socles, murets", 1, 40, "crate.ochre")
	crate("core:crate-2", "Grosse caisse", "2 m de côté, fixe une fois posée : on monte dessus, on empile", 2, 120, "crate.slate")
	crate("core:crate-4", "Très grosse caisse", "4 m de côté, fixe : de quoi bâtir vite", 4, 400, "crate.sand")
	plain := func(id, name, desc string, cost float64, size vec3) {
		block("blocks", id, name, desc, "crate", cost, size, "habitat.white", []content.PartJSON{
			box("habitat.white", vec3{0, size.Y / 2, 0}, size, 0.04),
		})
	}
	plain("core:cube-half", "Demi-bloc", "1 × 1 m, 50 cm de haut : marches, socles, bancs", 25, vec3{1, 0.5, 1})
	plain("core:cube-long", "Bloc long", "1 × 1 m, 2 m de long", 60, vec3{1, 1, 2})
	plain("core:cube-2", "Bloc lisse", "2 m de côté, tout uni", 110, vec3{2, 2, 2})
	plain("core:cube-low", "Bloc plat", "2 × 2 m, 1 m de haut", 70, vec3{2, 1, 2})
	volume := func(id, name, desc string, cost float64, size vec3, pts ...vec3) {
		p := hull("habitat.white", pts...)
		block("blocks", id, name, desc, "ramp", cost, size, "habitat.white", []content.PartJSON{p}, shape(p))
	}
	volume("core:wedge-1", "Petit coin", "1 m de côté, coupé en biais à 45°", 30, vec3{1, 1, 1},
		vec3{0.5, 0, -0.5}, vec3{0.5, 0, 0.5}, vec3{-0.5, 0, -0.5}, vec3{-0.5, 0, 0.5}, vec3{-0.5, 1, -0.5}, vec3{-0.5, 1, 0.5})
	volume("core:wedge-2", "Coin", "2 m de côté, coupé en biais à 45°", 80, vec3{2, 2, 2},
		vec3{1, 0, -1}, vec3{1, 0, 1}, vec3{-1, 0, -1}, vec3{-1, 0, 1}, vec3{-1, 2, -1}, vec3{-1, 2, 1})
	volume("core:corner-2", "Coin d'angle", "2 m de côté : l'angle saillant entre deux coins", 70, vec3{2, 2, 2},
		vec3{1, 0, -1}, vec3{1, 0, 1}, vec3{-1, 0, -1}, vec3{-1, 0, 1}, vec3{-1, 2, -1})
	inner := []content.PartJSON{
		hull("habitat.white", vec3{1, 0, -1}, vec3{1, 0, 1}, vec3{-1, 0, -1}, vec3{-1, 0, 1}, vec3{-1, 2, -1}, vec3{-1, 2, 1}),
		hull("habitat.white", vec3{1, 0, -1}, vec3{1, 0, 1}, vec3{-1, 0, -1}, vec3{-1, 0, 1}, vec3{-1, 2, -1}, vec3{1, 2, -1}),
	}
	block("blocks", "core:valley-2", "Coin rentrant", "2 m de côté : l'angle creux entre deux coins", "ramp", 100, vec3{2, 2, 2}, "habitat.white", inner, shape(inner...))
	volume("core:prism-2", "Bloc triangle", "2 m de haut, un triangle vu de dessus", 70, vec3{2, 2, 2},
		vec3{1, 0, -1}, vec3{-1, 0, -1}, vec3{-1, 0, 1}, vec3{1, 2, -1}, vec3{-1, 2, -1}, vec3{-1, 2, 1})
	volume("core:round-2", "Quart de rond", "2 m de haut, un quart de disque vu de dessus : les angles arrondis", 90, vec3{2, 2, 2},
		quarter(2, 0, 2)...)

	// Floors. Their edges are steel, flush with their sides.
	floor := func(id, name, desc string, cost, x, z float64) {
		block("floors", id, name, desc, "slab", cost, vec3{x, 0.2, z}, "pad.concrete", []content.PartJSON{
			box("pad.concrete", vec3{0, 0.1, 0}, vec3{x, 0.2, z}, 0.02),
			box("steel.dark", vec3{0, 0.1, -z/2 + 0.03}, vec3{x + 0.02, 0.22, 0.08}, 0),
			box("steel.dark", vec3{0, 0.1, z/2 - 0.03}, vec3{x + 0.02, 0.22, 0.08}, 0),
			box("steel.dark", vec3{-x/2 + 0.03, 0.1, 0}, vec3{0.08, 0.22, z + 0.02}, 0),
			box("steel.dark", vec3{x/2 - 0.03, 0.1, 0}, vec3{0.08, 0.22, z + 0.02}, 0),
		})
	}
	floor("core:slab", "Dalle", "4 × 4 m, 20 cm d'épaisseur : sols, toits, ponts", 300, 4, 4)
	floor("core:slab-half", "Demi-dalle", "2 × 4 m : couloirs, balcons, autour d'un escalier", 160, 2, 4)
	floor("core:slab-quarter", "Petite dalle", "2 × 2 m", 90, 2, 2)
	floor("core:slab-big", "Grande dalle", "8 × 8 m : places, pistes, grands sols", 1_100, 8, 8)
	tri := hull("pad.concrete", vec3{2, 0, -2}, vec3{-2, 0, -2}, vec3{-2, 0, 2}, vec3{2, 0.2, -2}, vec3{-2, 0.2, -2}, vec3{-2, 0.2, 2})
	block("floors", "core:slab-tri", "Dalle triangle", "4 × 4 m coupée en deux : les pans en biais", "slab", 170, vec3{4, 0.2, 4}, "pad.concrete", []content.PartJSON{tri}, shape(tri))
	disc := hull("pad.concrete", quarter(4, 0, 0.2)...)
	block("floors", "core:slab-round", "Dalle arrondie", "Un quart de disque de 4 m : terrasses, plateformes rondes", "slab", 260, vec3{4, 0.2, 4}, "pad.concrete", []content.PartJSON{disc}, shape(disc))
	block("floors", "core:slab-glass", "Dalle vitrée", "4 × 4 m de verre dans un cadre : on voit dessous", "slab", 520, vec3{4, 0.2, 4}, "steel.dark", append(
		frame("steel.dark", 4, 4, 0.2, 0.16),
		glass(vec3{0, 0.12, 0}, vec3{3.7, 0.08, 3.7}),
		box("steel.dark", vec3{0, 0.06, 0}, vec3{3.7, 0.08, 0.1}, 0),
		box("steel.dark", vec3{0, 0.06, 0}, vec3{0.1, 0.08, 3.7}, 0),
	), shape(box("", vec3{0, 0.1, 0}, vec3{4, 0.2, 4}, 0)))
	grate := frame("steel", 2, 4, 0.2, 0.1)
	for i := range 18 { // bars across
		grate = append(grate, box("steel.dark", vec3{0, 0.14, -1.8 + float64(i)*3.6/17}, vec3{1.84, 0.1, 0.05}, 0))
	}
	block("floors", "core:grate", "Caillebotis", "2 × 4 m d'acier ajouré : passerelles", "slab", 140, vec3{2, 0.2, 4}, "steel", grate,
		shape(box("", vec3{0, 0.1, 0}, vec3{2, 0.2, 4}, 0)))

	// Walls, on a base and under a cap of steel.
	trims := func(z, y float64) []content.PartJSON {
		return []content.PartJSON{
			box("steel.dark", vec3{0, 0.06, 0}, vec3{0.22, 0.12, z + 0.02}, 0),
			box("steel.dark", vec3{0, y - 0.06, 0}, vec3{0.22, 0.12, z + 0.02}, 0),
		}
	}
	wall := func(id, name, desc string, cost, z, y float64) {
		parts := []content.PartJSON{box("habitat.white", vec3{0, y / 2, 0}, vec3{0.2, y, z}, 0.02)}
		parts = append(parts, trims(z, y)...)
		if y > 2 {
			parts = append(parts,
				box("neon.cyan", vec3{0.105, 1.6, 0}, vec3{0.01, 0.04, z * 0.8}, 0),
				box("neon.cyan", vec3{-0.105, 1.6, 0}, vec3{0.01, 0.04, z * 0.8}, 0))
		}
		block("walls", id, name, desc, "wall", cost, vec3{0.2, y, z}, "habitat.white", parts)
	}
	wall("core:wall", "Mur", "4 m de long, 2,5 m de haut : parois, abris", 300, 4, 2.5)
	wall("core:wall-half", "Demi-mur", "2 m de long, 2,5 m de haut", 160, 2, 2.5)
	wall("core:wall-low", "Muret", "4 m de long, 1 m de haut : parapets, bords de piste", 130, 4, 1)
	// A wall with an opening from z0 to z1, y0 to y1: pieces around it.
	opened := func(y, half, y0, y1 float64) []content.PartJSON {
		side := 2 - half
		out := []content.PartJSON{
			box("habitat.white", vec3{0, y / 2, -2 + side/2}, vec3{0.2, y, side}, 0.02),
			box("habitat.white", vec3{0, y / 2, 2 - side/2}, vec3{0.2, y, side}, 0.02),
			box("habitat.white", vec3{0, (y1 + y) / 2, 0}, vec3{0.2, y - y1, 2 * half}, 0),
		}
		if y0 > 0 {
			out = append(out, box("habitat.white", vec3{0, y0 / 2, 0}, vec3{0.2, y0, 2 * half}, 0))
		}
		return out
	}
	// Its frame, sticking out a little.
	lining := func(half, y0, y1 float64) []content.PartJSON {
		mid, tall := (y0+y1)/2, y1-y0
		out := []content.PartJSON{
			box("steel.dark", vec3{0, mid, -half + 0.03}, vec3{0.26, tall, 0.06}, 0),
			box("steel.dark", vec3{0, mid, half - 0.03}, vec3{0.26, tall, 0.06}, 0),
			box("steel.dark", vec3{0, y1 - 0.03, 0}, vec3{0.26, 0.06, 2 * half}, 0),
		}
		if y0 > 0 {
			out = append(out, box("steel.dark", vec3{0, y0 + 0.03, 0}, vec3{0.26, 0.06, 2 * half}, 0))
		}
		return out
	}
	caps := box("steel.dark", vec3{0, 2.44, 0}, vec3{0.22, 0.12, 4.02}, 0)
	foot := func(half float64) []content.PartJSON {
		side := 2 - half
		return []content.PartJSON{caps,
			box("steel.dark", vec3{0, 0.06, -2.01 + (side+0.01)/2}, vec3{0.22, 0.12, side + 0.01}, 0),
			box("steel.dark", vec3{0, 0.06, 2.01 - (side+0.01)/2}, vec3{0.22, 0.12, side + 0.01}, 0)}
	}
	join := func(lists ...[]content.PartJSON) (out []content.PartJSON) {
		for _, l := range lists {
			out = append(out, l...)
		}
		return out
	}
	door := opened(2.5, 0.8, 0, 2.2)
	block("walls", "core:wall-door", "Mur à porte", "4 m de long : un passage de 1,6 m au milieu", "door", 320, vec3{0.2, 2.5, 4}, "habitat.white",
		join(door, foot(0.8), lining(0.8, 0, 2.2), []content.PartJSON{box("neon.cyan", vec3{0.105, 2.29, 0}, vec3{0.01, 0.04, 1.4}, 0), box("neon.cyan", vec3{-0.105, 2.29, 0}, vec3{0.01, 0.04, 1.4}, 0)}),
		shape(door...))
	// Doors: their leaves slide into the wall on both sides.
	sliding := func(id, name, desc string, cost float64, leaf func(side float64) []content.PartJSON) {
		parts := join(door, foot(0.8), lining(0.8, 0, 2.2), []content.PartJSON{
			box("neon.cyan", vec3{0.105, 2.29, 0}, vec3{0.01, 0.04, 1.4}, 0), box("neon.cyan", vec3{-0.105, 2.29, 0}, vec3{0.01, 0.04, 1.4}, 0)})
		block("walls", id, name, desc, "door", cost, vec3{0.2, 2.5, 4}, "habitat.white", parts, shape(door...), func(e *content.EntityDef) {
			for _, s := range []float64{-1, 1} {
				e.Leaves = append(e.Leaves, content.Leaf{Parts: leaf(s), Slide: vec3{0, 0, s * 0.72}})
			}
		})
	}
	sliding("core:door", "Porte coulissante", "4 m de long : s'ouvre toute seule quand on approche", 520, func(s float64) []content.PartJSON {
		return []content.PartJSON{
			box("steel", vec3{0, 1.07, s * 0.37}, vec3{0.08, 2.14, 0.73}, 0),
			box("steel.dark", vec3{0, 1.07, s * 0.04}, vec3{0.1, 2.12, 0.06}, 0),
			box("neon.cyan", vec3{0, 1.25, s * 0.4}, vec3{0.1, 0.04, 0.4}, 0),
		}
	})
	sliding("core:door-glass", "Porte vitrée", "4 m de long : s'ouvre toute seule, laisse passer la lumière", 560, func(s float64) []content.PartJSON {
		return []content.PartJSON{
			box("steel.dark", vec3{0, 0.06, s * 0.37}, vec3{0.08, 0.12, 0.73}, 0),
			box("steel.dark", vec3{0, 2.1, s * 0.37}, vec3{0.08, 0.08, 0.73}, 0),
			box("steel.dark", vec3{0, 1.09, s * 0.035}, vec3{0.08, 1.94, 0.06}, 0),
			box("steel.dark", vec3{0, 1.09, s * 0.705}, vec3{0.08, 1.94, 0.06}, 0),
			glass(vec3{0, 1.09, s * 0.37}, vec3{0.04, 1.94, 0.61}),
		}
	})
	window := opened(2.5, 1.4, 0.9, 2.1)
	block("walls", "core:wall-window", "Mur à fenêtre", "4 m de long : une vitre de 2,8 m", "window", 380, vec3{0.2, 2.5, 4}, "habitat.white",
		join(window, trims(4, 2.5), lining(1.4, 0.9, 2.1), []content.PartJSON{glass(vec3{0, 1.5, 0}, vec3{0.04, 1.08, 2.68})}),
		shape(box("", vec3{0, 1.25, 0}, vec3{0.2, 2.5, 4}, 0)))
	block("walls", "core:wall-glass", "Baie vitrée", "4 m de long, 2,5 m de haut : du verre du sol au plafond", "window", 450, vec3{0.2, 2.5, 4}, "steel.dark",
		join(trims(4, 2.5), []content.PartJSON{
			box("steel.dark", vec3{0, 1.25, -1.95}, vec3{0.2, 2.26, 0.1}, 0),
			box("steel.dark", vec3{0, 1.25, 1.95}, vec3{0.2, 2.26, 0.1}, 0),
			box("steel.dark", vec3{0, 1.25, 0}, vec3{0.12, 2.26, 0.06}, 0),
			glass(vec3{0, 1.25, -0.965}, vec3{0.04, 2.26, 1.87}), glass(vec3{0, 1.25, 0.965}, vec3{0.04, 2.26, 1.87})}),
		shape(box("", vec3{0, 1.25, 0}, vec3{0.2, 2.5, 4}, 0)))
	gable := hull("habitat.white", vec3{-0.1, 0, -2}, vec3{0.1, 0, -2}, vec3{-0.1, 0, 2}, vec3{0.1, 0, 2}, vec3{-0.1, 2, 2}, vec3{0.1, 2, 2})
	block("walls", "core:gable", "Pignon", "4 m de long : le triangle de mur sous un toit en pente", "wall", 170, vec3{0.2, 2, 4}, "habitat.white", []content.PartJSON{gable}, shape(gable))
	gate := []content.PartJSON{
		box("habitat.white", vec3{0, 2, -2.75}, vec3{0.4, 4, 0.5}, 0.03),
		box("habitat.white", vec3{0, 2, 2.75}, vec3{0.4, 4, 0.5}, 0.03),
		box("habitat.white", vec3{0, 3.7, 0}, vec3{0.4, 0.6, 5}, 0),
	}
	block("walls", "core:gate", "Portail", "6 m de large, 4 m de haut : les véhicules passent dessous", "door", 700, vec3{0.4, 4, 6}, "habitat.white",
		join(gate, []content.PartJSON{
			box("hazard", vec3{0, 0.5, -2.49}, vec3{0.42, 1, 0.04}, 0), box("hazard", vec3{0, 0.5, 2.49}, vec3{0.42, 1, 0.04}, 0),
			box("neon.cyan", vec3{0.205, 3.7, 0}, vec3{0.01, 0.06, 4.6}, 0), box("neon.cyan", vec3{-0.205, 3.7, 0}, vec3{0.01, 0.06, 4.6}, 0)}),
		shape(gate...))
	// Its shutter: slats that all roll up behind the header.
	block("walls", "core:gate-door", "Portail à rideau", "6 m de large, 4 m de haut : le rideau monte quand on approche, à pied ou en véhicule", "door", 1_100, vec3{0.4, 4, 6}, "habitat.white",
		join(gate, []content.PartJSON{
			box("hazard", vec3{0, 0.5, -2.49}, vec3{0.42, 1, 0.04}, 0), box("hazard", vec3{0, 0.5, 2.49}, vec3{0.42, 1, 0.04}, 0),
			box("neon.orange", vec3{0.205, 3.7, 0}, vec3{0.01, 0.06, 4.6}, 0), box("neon.orange", vec3{-0.205, 3.7, 0}, vec3{0.01, 0.06, 4.6}, 0)}),
		shape(gate...), func(e *content.EntityDef) {
			const n, h = 6, 3.4 / 6
			for i := range n {
				y := h * (float64(i) + 0.5)
				slat := []content.PartJSON{box("steel", vec3{0, y, 0}, vec3{0.08, h - 0.02, 4.94}, 0), box("steel.dark", vec3{0, y, 0}, vec3{0.05, h, 4.9}, 0)}
				if i == 0 {
					slat = append(slat, box("hazard", vec3{0, 0.08, 0}, vec3{0.1, 0.14, 4.92}, 0))
				}
				e.Leaves = append(e.Leaves, content.Leaf{Parts: slat, Slide: vec3{0, 3.7 - y, 0}})
			}
		})

	// Roofs: 20 cm thick, their underside rising 2 m over 4.
	const thick = 0.2
	slope := func(z float64) content.PartJSON {
		return over("crate.slate", thick, vec3{2, 0, -z}, vec3{2, 0, z}, vec3{-2, 2, -z}, vec3{-2, 2, z})
	}
	roof := func(id, name, desc string, cost, z float64) {
		p := slope(z / 2)
		block("roofs", id, name, desc, "ramp", cost, vec3{4, 2, z}, "crate.slate", []content.PartJSON{p}, shape(p), rises)
	}
	roof("core:roof", "Toit", "4 × 4 m, monte de 2 m : se pose sur un mur, se prolonge", 320, 4)
	roof("core:roof-half", "Demi-toit", "2 m de large, monte de 2 m", 170, 2)
	hip := []content.PartJSON{
		over("crate.slate", thick, vec3{2, 0, -2}, vec3{2, 0, 2}, vec3{-2, 2, -2}),
		over("crate.slate", thick, vec3{2, 0, 2}, vec3{-2, 0, 2}, vec3{-2, 2, -2}),
	}
	block("roofs", "core:roof-corner", "Angle de toit", "4 × 4 m : l'angle saillant entre deux pentes", "ramp", 300, vec3{4, 2, 4}, "crate.slate", hip, shape(hip...))
	valley := []content.PartJSON{
		over("crate.slate", thick, vec3{2, 0, 2}, vec3{-2, 2, 2}, vec3{-2, 2, -2}),
		over("crate.slate", thick, vec3{2, 0, 2}, vec3{2, 2, -2}, vec3{-2, 2, -2}),
	}
	block("roofs", "core:roof-valley", "Noue de toit", "4 × 4 m : l'angle creux entre deux pentes", "ramp", 340, vec3{4, 2, 4}, "crate.slate", valley, shape(valley...))
	pane := over("glass.pane", 0.06, vec3{1.9, 0.11, -1.9}, vec3{1.9, 0.11, 1.9}, vec3{-1.9, 2.01, -1.9}, vec3{-1.9, 2.01, 1.9})
	pane.Lit = "glass"
	skylight := []content.PartJSON{
		over("steel.dark", thick, vec3{2, 0, -2}, vec3{2, 0, 2}, vec3{1.9, 0.05, -2}, vec3{1.9, 0.05, 2}),
		over("steel.dark", thick, vec3{-1.9, 1.95, -2}, vec3{-1.9, 1.95, 2}, vec3{-2, 2, -2}, vec3{-2, 2, 2}),
		over("steel.dark", thick, vec3{1.9, 0.05, -2}, vec3{1.9, 0.05, -1.9}, vec3{-1.9, 1.95, -2}, vec3{-1.9, 1.95, -1.9}),
		over("steel.dark", thick, vec3{1.9, 0.05, 1.9}, vec3{1.9, 0.05, 2}, vec3{-1.9, 1.95, 1.9}, vec3{-1.9, 1.95, 2}),
	}
	block("roofs", "core:roof-glass", "Verrière", "4 × 4 m, monte de 2 m : un toit de verre", "window", 560, vec3{4, 2, 4}, "steel.dark",
		append(skylight, pane), shape(slope(2)), rises)

	// What climbs.
	block("climbs", "core:slope", "Pente", "4 × 2 m, monte de 1 m : à pied comme en véhicule", "ramp", 250, vec3{4, 1, 2}, "pad.concrete", []content.PartJSON{
		{Shape: "wedge", Token: "pad.concrete", At: vec3{0.05, 0, 0}, Size: vec3{3.9, 0.975, 2}, Yaw: 180, Lit: "outdoor"},
		box("hazard", vec3{-1.95, 0.4875, 0}, vec3{0.1, 0.975, 2}, 0),
		{Shape: "wedge", Token: "hazard", At: vec3{-1.95, 0.975, 0}, Size: vec3{0.1, 0.025, 2}, Yaw: 180, Lit: "outdoor"},
		box("steel.dark", vec3{-0.2, 0.03, -1}, vec3{3.4, 0.06, 0.02}, 0),
		box("steel.dark", vec3{-0.2, 0.03, 1}, vec3{3.4, 0.06, 0.02}, 0),
	})
	var steps []content.PartJSON
	for i := range 5 {
		k := float64(i)
		steps = append(steps, box("pad.concrete", vec3{0.8 - 0.4*k, 0.1 * (k + 1), 0}, vec3{0.4, 0.2 * (k + 1), 2}, 0),
			box("hazard", vec3{0.96 - 0.4*k, 0.2*(k+1) - 0.02, 0}, vec3{0.1, 0.05, 1.8}, 0))
	}
	block("climbs", "core:steps", "Marches", "2 × 2 m, montent de 1 m", "stairs", 110, vec3{2, 1, 2}, "pad.concrete", steps,
		shape(hull("", vec3{1, 0, -1}, vec3{1, 0, 1}, vec3{-1, 0, -1}, vec3{-1, 0, 1}, vec3{-1, 1, -1}, vec3{-1, 1, 1})), rises)
	// A storey's stairs: a slab under the steps, to walk underneath.
	const storey, flights = 2.7, 14
	flight := hull("pad.concrete", vec3{2, 0, -1}, vec3{2, 0, 1}, vec3{1.4, 0, -1}, vec3{1.4, 0, 1},
		vec3{-2, storey - 0.5, -1}, vec3{-2, storey - 0.5, 1}, vec3{-2, storey, -1}, vec3{-2, storey, 1})
	under := hull("pad.concrete", vec3{2, 0, -1}, vec3{2, 0, 1}, vec3{1.4, 0, -1}, vec3{1.4, 0, 1},
		vec3{-2, storey - 0.5, -1}, vec3{-2, storey - 0.5, 1}, vec3{-2, storey - 0.19, -1}, vec3{-2, storey - 0.19, 1})
	stairs := []content.PartJSON{under}
	for i := range flights {
		const run, riser = 4.0 / flights, storey / flights
		x, y := 2-(float64(i)+0.5)*run, float64(i+1)*riser
		stairs = append(stairs, box("pad.concrete", vec3{x, y - 0.12, 0}, vec3{run, 0.24, 2}, 0),
			box("steel.dark", vec3{x + run/2 - 0.03, y - 0.02, 0}, vec3{0.08, 0.05, 1.8}, 0))
	}
	block("climbs", "core:stairs", "Escalier", "4 × 2 m, monte d'un étage (2,7 m : un mur et une dalle)", "stairs", 360, vec3{4, storey, 2}, "pad.concrete", stairs,
		shape(flight), rises)

	// Structure.
	block("frames", "core:beam", "Poutre", "4 m d'acier : piliers, garde-corps, charpentes", "crate", 80, vec3{0.25, 0.25, 4}, "steel", []content.PartJSON{
		box("steel", vec3{0, 0.125, 0}, vec3{0.25, 0.25, 4}, 0.02),
		box("hazard", vec3{0, 0.125, -1.9}, vec3{0.26, 0.26, 0.2}, 0),
		box("hazard", vec3{0, 0.125, 1.9}, vec3{0.26, 0.26, 0.2}, 0),
	})
	block("frames", "core:beam-half", "Petite poutre", "2 m d'acier", "crate", 45, vec3{0.25, 0.25, 2}, "steel", []content.PartJSON{
		box("steel", vec3{0, 0.125, 0}, vec3{0.25, 0.25, 2}, 0.02),
	})
	block("frames", "core:pillar", "Pilier", "50 cm de côté, 2,5 m de haut : porte une dalle", "pillar", 90, vec3{0.5, 2.5, 0.5}, "habitat.white", []content.PartJSON{
		box("habitat.white", vec3{0, 1.25, 0}, vec3{0.4, 2.5, 0.4}, 0.04),
		box("steel.dark", vec3{0, 0.1, 0}, vec3{0.5, 0.2, 0.5}, 0.02),
		box("steel.dark", vec3{0, 2.4, 0}, vec3{0.5, 0.2, 0.5}, 0.02),
	})
	column := prism("habitat.white", vec3{0, 0, 0}, vec3{}, 0.42, 0.42, 2.5, 12)
	block("frames", "core:column", "Colonne", "1 m de large, ronde, 2,5 m de haut", "pillar", 120, vec3{1, 2.5, 1}, "habitat.white", []content.PartJSON{
		column,
		prism("steel.dark", vec3{0, 0, 0}, vec3{}, 0.5, 0.5, 0.15, 12),
		prism("steel.dark", vec3{0, 2.35, 0}, vec3{}, 0.5, 0.5, 0.15, 12),
	}, shape(column))
	rail := func(id, name, desc string, cost, z float64, posts int) {
		parts := []content.PartJSON{
			box("steel", vec3{0, 0.97, 0}, vec3{0.1, 0.06, z}, 0.01),
			box("steel", vec3{0, 0.55, 0}, vec3{0.05, 0.04, z}, 0),
			box("steel.dark", vec3{0, 0.06, 0}, vec3{0.04, 0.12, z}, 0),
		}
		for i := range posts {
			at := -z/2 + 0.05 + (z-0.1)*float64(i)/float64(posts-1)
			parts = append(parts, box("steel", vec3{0, 0.48, at}, vec3{0.08, 0.96, 0.08}, 0))
		}
		block("frames", id, name, desc, "rail", cost, vec3{0.1, 1, z}, "steel", parts, shape(box("", vec3{0, 0.5, 0}, vec3{0.1, 1, z}, 0)))
	}
	rail("core:rail", "Garde-corps", "4 m de long, 1 m de haut : balcons, passerelles", 90, 4, 4)
	rail("core:rail-half", "Petit garde-corps", "2 m de long, 1 m de haut", 50, 2, 3)
	// An arch: two legs 1 m high under half a ring.
	arch := []content.PartJSON{
		box("habitat.white", vec3{0, 0.5, -2.7}, vec3{0.6, 1, 0.6}, 0),
		box("habitat.white", vec3{0, 0.5, 2.7}, vec3{0.6, 1, 0.6}, 0),
	}
	const bows = 10
	at := func(r, k float64) (y, z float64) {
		s, c := math.Sincos(k * math.Pi / bows)
		return 1 + r*s, -r * c
	}
	for i := range bows {
		var pts []vec3
		for _, k := range []float64{float64(i), float64(i + 1)} {
			for _, r := range []float64{2.4, 3} {
				y, z := at(r, k)
				pts = append(pts, vec3{-0.3, y, z}, vec3{0.3, y, z})
			}
		}
		arch = append(arch, hull("habitat.white", pts...))
	}
	block("frames", "core:arch", "Arche", "6 m de large, 4 m de haut : les véhicules passent dessous", "door", 650, vec3{0.6, 4, 6}, "habitat.white",
		append(arch, box("steel.dark", vec3{0, 0.06, -2.7}, vec3{0.64, 0.12, 0.64}, 0), box("steel.dark", vec3{0, 0.06, 2.7}, vec3{0.64, 0.12, 0.64}, 0)),
		shape(arch...))
}

// registerLamps adds the lamps to build with. Those for outside light up
// at dusk.
func registerLamps(c *content.Content) {
	lamp := func(id, name, desc string, cost float64, cell vec3, l content.Light, paint string, parts []content.PartJSON, shape ...content.PartJSON) {
		c.Entities.Add(id, content.EntityDef{Name: name, Pickup: true, Prompt: "Ramasser : " + name, Build: true, Cell: cell, Paint: "core:" + paint, Parts: parts, Shape: shape, Light: &l})
		c.Items.Add(id, content.Item{Name: name, Desc: desc, Icon: "lamp", Cost: cost, Places: id, Shelf: "lights"})
	}
	lamp("core:lamp-ceiling", "Plafonnier", "60 cm, sous une dalle : éclaire une pièce", 120, vec3{0.6, 0.1, 0.6},
		content.Light{At: vec3{0, -0.06, 0}, Cone: 74, Power: 0.75, Range: 9}, "steel.dark", []content.PartJSON{
			box("steel.dark", vec3{0, 0.065, 0}, vec3{0.6, 0.07, 0.6}, 0.02),
			box("lamp.warm", vec3{0, 0.02, 0}, vec3{0.5, 0.04, 0.5}, 0),
		})
	lamp("core:lamp-strip", "Réglette", "2 m de lumière blanche, sous une dalle", 110, vec3{0.2, 0.1, 2},
		content.Light{At: vec3{0, -0.06, 0}, Cone: 74, Power: 0.85, Range: 10, Tint: "core:lamp.cold"}, "steel.dark", []content.PartJSON{
			box("steel.dark", vec3{0, 0.07, 0}, vec3{0.2, 0.06, 2}, 0.01),
			box("lamp.cold", vec3{0, 0.025, 0}, vec3{0.12, 0.05, 1.9}, 0),
		})
	lamp("core:lamp-wall", "Applique", "Contre un mur, dedans ou dehors : une lumière douce", 90, vec3{0.15, 0.4, 0.25},
		content.Light{At: vec3{0.14, 0.2, 0}, Dir: vec3{0.6, -1, 0}, Cone: 72, Power: 0.5, Range: 7, Tint: "core:lamp.amber"}, "steel.dark", []content.PartJSON{
			box("steel.dark", vec3{-0.05, 0.2, 0}, vec3{0.05, 0.4, 0.25}, 0.01),
			box("lamp.amber", vec3{0.01, 0.2, 0}, vec3{0.08, 0.3, 0.15}, 0.02),
		})
	lamp("core:lamp-post", "Lampe sur pied", "1,6 m, pour l'intérieur : une lumière chaude", 150, vec3{0.5, 1.6, 0.5},
		content.Light{At: vec3{0, 1.22, 0}, Cone: 74, Power: 0.5, Range: 7, Tint: "core:lamp.amber"}, "steel.dark", []content.PartJSON{
			prism("steel.dark", vec3{}, vec3{}, 0.16, 0.16, 0.04, 12),
			prism("steel.dark", vec3{0, 0.04, 0}, vec3{}, 0.02, 0.02, 1.21, 6),
			prism("lamp.amber", vec3{0, 1.25, 0}, vec3{}, 0.24, 0.15, 0.35, 10),
		}, box("", vec3{0, 0.8, 0}, vec3{0.1, 1.6, 0.1}, 0))
	lamp("core:lamp-bollard", "Borne lumineuse", "1 m de haut, s'allume la nuit : bords de piste, allées", 130, vec3{0.25, 1, 0.25},
		content.Light{At: vec3{0, 0.84, 0}, Cone: 74, Power: 0.6, Range: 6, Night: true}, "steel.dark", []content.PartJSON{
			box("steel.dark", vec3{0, 0.4, 0}, vec3{0.1, 0.8, 0.1}, 0),
			box("lamp.warm", vec3{0, 0.89, 0}, vec3{0.2, 0.07, 0.2}, 0),
			box("steel.dark", vec3{0, 0.96, 0}, vec3{0.25, 0.08, 0.25}, 0.01),
		}, box("", vec3{0, 0.5, 0}, vec3{0.2, 1, 0.2}, 0))
	pole := box("habitat.white", vec3{-0.8, 2, 0}, vec3{0.16, 4, 0.16}, 0.02)
	lamp("core:lamp-street", "Lampadaire", "4 m de haut, s'allume la nuit : places et pistes", 320, vec3{2, 4, 0.5},
		content.Light{At: vec3{0.6, 3.72, 0}, Cone: 74, Power: 2.6, Range: 16, Night: true}, "habitat.white", []content.PartJSON{
			pole,
			box("steel.dark", vec3{-0.8, 0.1, 0}, vec3{0.4, 0.2, 0.4}, 0.02),
			box("habitat.white", vec3{0.1, 3.92, 0}, vec3{1.7, 0.1, 0.12}, 0.01),
			box("steel.dark", vec3{0.6, 3.86, 0}, vec3{0.7, 0.1, 0.4}, 0.01),
			box("lamp.warm", vec3{0.6, 3.8, 0}, vec3{0.6, 0.04, 0.3}, 0),
		}, pole)
	lamp("core:lamp-flood", "Projecteur", "Sur pied, s'allume la nuit : un large faisceau, loin devant lui", 380, vec3{1, 2, 1},
		content.Light{At: vec3{-0.3, 1.65, 0}, Dir: vec3{-1, -0.28, 0}, Cone: 42, Power: 5, Range: 30, Night: true, Tint: "core:lamp.cold"}, "steel.dark", []content.PartJSON{
			box("steel.dark", vec3{0, 0.03, 0}, vec3{0.8, 0.06, 0.8}, 0.01),
			box("steel.dark", vec3{0, 0.75, 0}, vec3{0.1, 1.4, 0.1}, 0),
			box("steel.dark", vec3{0, 1.65, 0}, vec3{0.3, 0.45, 0.8}, 0.03),
			box("lamp.cold", vec3{-0.16, 1.65, 0}, vec3{0.03, 0.37, 0.7}, 0),
		}, box("", vec3{0, 0.95, 0}, vec3{0.4, 1.9, 0.8}, 0))
}

// hull is the convex hull of points.
func hull(tok string, pts ...vec3) content.PartJSON {
	return content.PartJSON{Shape: "hull", Token: tok, Points: pts, Lit: "outdoor"}
}

// over is a sloped panel: its underside's corners, and how thick it is,
// straight up.
func over(tok string, thick float64, pts ...vec3) content.PartJSON {
	for _, p := range pts {
		pts = append(pts, p.Add(vec3{0, thick, 0}))
	}
	return hull(tok, pts...)
}

// quarter is a quarter of a disc, r wide, from y0 to y1, its corner at
// (-r/2, -r/2).
func quarter(r, y0, y1 float64) []vec3 {
	var out []vec3
	for _, y := range []float64{y0, y1} {
		out = append(out, vec3{-r / 2, y, -r / 2})
		for i := range 9 {
			s, c := math.Sincos(float64(i) / 8 * math.Pi / 2)
			out = append(out, vec3{-r/2 + r*c, y, -r/2 + r*s})
		}
	}
	return out
}

// glass is a pane of glass.
func glass(at, size vec3) content.PartJSON {
	p := box("glass.pane", at, size, 0)
	p.Lit = "glass"
	return p
}

// frame is the four bars around a floor x by z, h high and w wide.
func frame(tok string, x, z, h, w float64) []content.PartJSON {
	return []content.PartJSON{
		box(tok, vec3{0, h / 2, -z/2 + w/2}, vec3{x, h, w}, 0),
		box(tok, vec3{0, h / 2, z/2 - w/2}, vec3{x, h, w}, 0),
		box(tok, vec3{-x/2 + w/2, h / 2, 0}, vec3{w, h, z - 2*w}, 0),
		box(tok, vec3{x/2 - w/2, h / 2, 0}, vec3{w, h, z - 2*w}, 0),
	}
}

// registerFinishes adds what blocks may be painted with: flat tones of the
// base and of Mars, and a few colors.
func registerFinishes(c *content.Content) {
	finish := func(id, name, token string) {
		c.Finishes.Add("core:"+id, content.Finish{Name: name, Token: "core:" + token})
	}
	paint := func(id, name string, col color.RGBA) {
		c.Tokens.Add("core:paint."+id, content.Token{Col: col, Mat: content.MtPanel, Scale: 1, Spec: 0.3})
		finish(id, name, "paint."+id)
	}
	finish("white", "Blanc", "habitat.white")
	finish("sand", "Sable", "clad.sand")
	finish("concrete", "Béton", "pad.concrete")
	finish("slate", "Ardoise", "rack.slate")
	finish("steel", "Acier", "steel")
	finish("ochre", "Ocre", "crate.ochre")
	paint("rust", "Rouille", color.RGBA{158, 74, 46, 255})
	paint("coral", "Corail", color.RGBA{222, 112, 84, 255})
	paint("plum", "Prune", color.RGBA{94, 62, 96, 255})
	paint("teal", "Canard", color.RGBA{52, 108, 116, 255})
	paint("sky", "Ciel", color.RGBA{120, 164, 190, 255})
	paint("moss", "Mousse", color.RGBA{98, 124, 72, 255})
	paint("night", "Nuit", color.RGBA{44, 48, 62, 255})
}
