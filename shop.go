package main

import (
	"fmt"
	"mars_sweeper/internal/content"
	"math"
	"slices"
)

// stat is a stat with every upgrade applied.
func (g *Game) stat(name string) float64 {
	v, mult := content.Stats[name], 1.0
	for _, u := range reg.Upgrades.All() {
		lvl := float64(g.st.Levels[u.ID])
		if lvl == 0 {
			continue
		}
		for _, e := range u.Effects {
			if e.Stat != name {
				continue
			}
			v += e.Add * lvl
			if e.Mult != 0 {
				mult *= math.Pow(e.Mult, lvl)
			}
		}
	}
	return v * mult
}

// toolTier is the held tool's rank, 0 being the cheapest. A tool from a
// removed mod falls back to the first one.
func (g *Game) toolTier() int { return max(0, slices.Index(reg.Tools.IDs(), g.st.Tool)) }

func (g *Game) tool() *content.Tool { return reg.Tools.All()[g.toolTier()] }
func (g *Game) Capacity() float64   { return g.stat("tank") }
func (g *Game) powerMult() float64  { return g.stat("power") }
func (g *Game) priceMult() float64  { return g.stat("price") }
func (g *Game) autoSell() bool      { return g.stat("autosell") > 0 }
func (g *Game) space() float64      { return math.Max(0, g.Capacity()-g.st.Tank) }
func (g *Game) tankFull() bool      { return g.space() <= 1e-6 }
func (g *Game) gemValue() float64 {
	return 20 * g.priceMult() * math.Pow(2.5, float64(g.toolTier()))
}

type shopItem struct {
	id               string // of the tool or upgrade
	name, desc, info string
	icon             string // what the shop draws next to it: "tool" or an upgrade id
	cost             float64
	maxed            bool
	buy              func()
}

func (it shopItem) affordable(credits float64) bool { return !it.maxed && credits >= it.cost }

func (g *Game) shopItems() []shopItem {
	tools := reg.Tools.All()
	items := make([]shopItem, 0, 1+reg.Upgrades.Len())

	if next := g.toolTier() + 1; next < len(tools) {
		t := tools[next]
		items = append(items, shopItem{
			id:   t.ID,
			name: "Outil : " + t.Name,
			desc: fmt.Sprintf("Rayon %.1f m, portée %.0f m, vitesse %.1f g/s", t.Radius, t.Reach, t.Rate),
			info: fmt.Sprintf("Outil %d/%d", next+1, len(tools)),
			icon: "tool",
			cost: t.Cost,
			buy:  func() { g.st.Tool = t.ID },
		})
	} else {
		items = append(items, shopItem{name: "Outil : " + g.tool().Name, desc: "Le meilleur outil de la galaxie", info: "Outil max", icon: "tool", maxed: true})
	}

	for _, u := range reg.Upgrades.All() {
		lvl := g.st.Levels[u.ID]
		it := shopItem{
			id:   u.ID,
			name: u.Name,
			desc: u.Desc,
			icon: u.Icon,
			info: fmt.Sprintf("Niv %d", lvl),
			cost: u.Base * math.Pow(u.Growth, float64(lvl)),
			buy: func() {
				g.st.Levels[u.ID]++
				g.syncDrones()
			},
		}
		if u.Max > 0 && lvl >= u.Max {
			it.maxed, it.info = true, "Acquis"
		}
		items = append(items, it)
	}

	for _, v := range reg.Vehicles.All() {
		it := shopItem{
			id: v.ID, name: "Véhicule : " + v.Name, desc: v.Desc, icon: "vehicle",
			info: fmt.Sprintf("%.0f km/h", v.Speed*3.6), cost: v.Cost,
			buy: func() { g.buyVehicle(v) },
		}
		if g.st.Levels[v.ID] > 0 {
			it.maxed, it.info = true, "Sur l'aire d'atterrissage"
		}
		items = append(items, it)
	}
	for _, o := range reg.Items.All() {
		if o.Cost <= 0 {
			continue
		}
		items = append(items, shopItem{
			id: o.ID, name: o.Name, desc: o.Desc, icon: content.OrDefault(o.Icon, "flag"),
			info: fmt.Sprintf("Objet · %d sur toi", g.st.Items[o.ID]), cost: o.Cost,
			buy: func() {
				if g.st.Items == nil {
					g.st.Items = map[string]int{}
				}
				g.st.Items[o.ID]++
			},
		})
	}
	return items
}

// fmtNum formats big incremental-game numbers: 950, 1.2K, 3.4M...
func fmtNum(v float64) string {
	if v < 1000 {
		return fmt.Sprintf("%.0f", math.Floor(v))
	}
	for _, unit := range []string{"K", "M", "B", "T", "Qa", "Qi", "Sx", "Sp", "Oc"} {
		v /= 1000
		if v < 1000 {
			return fmt.Sprintf("%.1f%s", v, unit)
		}
	}
	return fmt.Sprintf("%.2e", v*1000)
}
