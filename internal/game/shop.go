package game

import (
	"fmt"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"slices"
)

// stat is a stat with every upgrade applied.
func (g *Game) stat(name string) float64 {
	v, mult := content.Stats[name], 1.0
	for _, u := range g.Content.Upgrades.All() {
		lvl := float64(g.State.Levels[u.ID])
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

// ToolTier is the held tool's rank, 0 being the cheapest. A tool from a
// removed mod falls back to the first one.
func (g *Game) ToolTier() int { return max(0, slices.Index(g.Content.Tools.IDs(), g.State.Tool)) }

func (g *Game) Tool() *content.Tool { return g.Content.Tools.All()[g.ToolTier()] }
func (g *Game) Capacity() float64 {
	if v, ok := g.Driven(); ok {
		return g.stat("tank") + v.Hopper
	}
	return g.stat("tank")
}
func (g *Game) powerMult() float64 { return g.stat("power") }
func (g *Game) priceMult() float64 { return g.stat("price") }
func (g *Game) AutoSell() bool     { return g.stat("autosell") > 0 }
func (g *Game) space() float64     { return math.Max(0, g.Capacity()-g.State.Tank) }
func (g *Game) TankFull() bool     { return g.space() <= 1e-6 }
func (g *Game) gemValue() float64 {
	return 20 * g.priceMult() * math.Pow(2.5, float64(g.ToolTier()))
}

type ShopItem struct {
	id               string // of the tool or upgrade
	Name, Desc, Info string
	Icon             string // what the shop draws next to it: "tool" or an upgrade id
	Tab              string // the shop's tab it is in (content.ShopTabs); "" at the garage
	Cost             float64
	Maxed            bool
	buy              func()
	// A vehicle owned, at the garage: what selling it back gives.
	Refund float64
	sell   func()
}

// resale is the share of its price a vehicle sells back for.
const resale = 0.5

func (it ShopItem) Affordable(credits float64) bool { return !it.Maxed && credits >= it.Cost }

func (g *Game) ShopItems() []ShopItem {
	if g.Garage {
		return g.garageItems()
	}
	tools := g.Content.Tools.All()
	items := make([]ShopItem, 0, 1+g.Content.Upgrades.Len())

	if next := g.ToolTier() + 1; next < len(tools) {
		t := tools[next]
		items = append(items, ShopItem{
			id:   t.ID,
			Name: "Outil : " + t.Name,
			Desc: fmt.Sprintf("Rayon %.1f m, portée %.0f m, vitesse %.1f g/s", t.Radius, t.Reach, t.Rate),
			Info: fmt.Sprintf("Outil %d/%d", next+1, len(tools)),
			Icon: "tool",
			Tab:  "tools",
			Cost: t.Cost,
			buy:  func() { g.State.Tool = t.ID },
		})
	} else {
		items = append(items, ShopItem{Name: "Outil : " + g.Tool().Name, Desc: "Le meilleur outil de la galaxie", Info: "Outil max", Icon: "tool", Tab: "tools", Maxed: true})
	}

	for _, u := range g.Content.Upgrades.All() {
		lvl := g.State.Levels[u.ID]
		it := ShopItem{
			id:   u.ID,
			Name: u.Name,
			Desc: u.Desc,
			Icon: u.Icon,
			Tab:  content.OrDefault(u.Category, "upgrades"),
			Info: fmt.Sprintf("Niv %d", lvl),
			Cost: u.Base * math.Pow(u.Growth, float64(lvl)),
			buy:  func() { g.State.Levels[u.ID]++ },
		}
		if u.Max > 0 && lvl >= u.Max {
			it.Maxed, it.Info = true, "Acquis"
		}
		items = append(items, it)
	}

	for _, v := range g.Content.Vehicles.All() {
		if !v.Auto() {
			continue // at the garage
		}
		bought := g.State.Levels[v.ID]
		it := ShopItem{
			id: v.ID, Name: v.Name, Desc: v.Desc, Icon: content.OrDefault(v.Icon, "vehicle"),
			Tab:  content.OrDefault(v.Category, "fleet"),
			Cost: v.Cost * math.Pow(content.OrZero(v.Growth, 1), float64(bought)),
			buy:  func() { g.buyVehicle(v) },
		}
		switch {
		case v.Patrol():
			it.Info = fmt.Sprintf("Robot · %d en service, %d sur toi", g.Entities(v.ID), g.State.Items[v.ID])
		default:
			it.Info = fmt.Sprintf("Drone · %d en service", bought)
		}
		if v.Max > 0 && bought >= v.Max {
			it.Maxed, it.Info = true, "Acquis"
		}
		items = append(items, it)
	}
	for _, o := range g.Content.Items.All() {
		if o.Cost <= 0 {
			continue
		}
		tab, kind := o.Category, "Objet"
		if def, ok := g.Content.Entities.Get(o.Places); ok && def.Build {
			tab, kind = content.OrDefault(tab, "build"), "Bloc"
		}
		items = append(items, ShopItem{
			id: o.ID, Name: o.Name, Desc: o.Desc, Icon: content.OrDefault(o.Icon, "flag"),
			Tab:  content.OrDefault(tab, "items"),
			Info: fmt.Sprintf("%s · %d sur toi", kind, g.State.Items[o.ID]), Cost: o.Cost,
			buy: func() {
				if g.State.Items == nil {
					g.State.Items = map[string]int{}
				}
				g.State.Items[o.ID]++
			},
		})
	}
	return items
}

// ShopTab is a tab of the shop: its category, and its name.
type ShopTab struct{ ID, Name string }

// ShopTabs are the tabs items are in, in content.ShopTabs' order, then
// the mods' own categories as they come.
func ShopTabs(items []ShopItem) []ShopTab {
	var tabs []ShopTab
	for _, t := range content.ShopTabs {
		if slices.ContainsFunc(items, func(it ShopItem) bool { return it.Tab == t.ID }) {
			tabs = append(tabs, ShopTab(t))
		}
	}
	for _, it := range items {
		if it.Tab != "" && !slices.ContainsFunc(tabs, func(t ShopTab) bool { return t.ID == it.Tab }) {
			tabs = append(tabs, ShopTab{it.Tab, it.Tab})
		}
	}
	return tabs
}

// garageItems are the vehicles to drive: those to buy, and those owned,
// to sell back.
func (g *Game) garageItems() []ShopItem {
	var items []ShopItem
	for _, v := range g.Content.Vehicles.All() {
		if v.Auto() {
			continue
		}
		it := ShopItem{
			id: v.ID, Name: v.Name, Desc: v.Desc, Icon: content.OrDefault(v.Icon, "vehicle"), Cost: v.Cost,
			Info: fmt.Sprintf("%.0f km/h · rayon %.0f m", v.Speed*3.6, v.Sweep),
			buy:  func() { g.buyVehicle(v) },
		}
		if g.State.Levels[v.ID] > 0 {
			it.Maxed, it.Refund = true, math.Floor(v.Cost*resale)
			it.Info = "Au garage"
			if id := g.entityOf(v.ID); id != 0 && !base.Covered(base.ToLocal(g.World.Relief.Surface(g.Ents.Transform.Get(id).Dir))) {
				it.Info = "Sorti"
			}
			if id := g.entityOf(v.ID); id != 0 && id == g.Riding {
				it.Info = "Conduit"
			}
			it.sell = func() { g.sellVehicle(v, it.Refund) }
		}
		items = append(items, it)
	}
	return items
}

// Sell sells back a vehicle owned, at the garage.
func (g *Game) Sell(it ShopItem) {
	if it.sell != nil {
		it.sell()
	}
}

// FmtNum formats big incremental-game numbers: 950, 1.2K, 3.4M...
func FmtNum(v float64) string {
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

// Buy pays for a shop item and gets it, when there is enough credit.
func (g *Game) Buy(it ShopItem) {
	if !it.Affordable(g.State.Credits) {
		g.sfx.Beep(0, 0.15, 0.1, 180)
		return
	}
	g.State.Credits -= it.Cost
	it.buy()
	g.emit(content.Event{Name: content.EvBuy, ID: it.id, Amount: it.Cost})
	g.sfx.Beep(0.06, 0.18, 0.14, 523, 784)
}
