package main

import (
	"fmt"
	"image/color"
	"math"
	"slices"
)

type headShape int

const (
	headBlock  headShape = iota // straight block (toothbrush bristles)
	headTaper                   // narrows to a point (paintbrush)
	headFlare                   // widens toward the ground (broom)
	headNozzle                  // motor bulb and nozzle (powered tools)
)

// ToolLook describes the model held in first person: a handle, a metal
// collar, then the head.
// In mods it reads (modjson.go):
//
//	{"handle_len": 0.5, "handle_r": 0.02, "handle_color": "#a57346",
//	 "head_len": 0.24, "head_r": 0.12, "head_color": "#f0cd5f",
//	 "head": "block|taper|flare|nozzle", "glow": 0}
type ToolLook struct {
	HandleLen, HandleR float64
	HandleCol          color.RGBA
	HeadLen, HeadR     float64
	HeadCol            color.RGBA
	Head               headShape
	Glow               float32 // emissive strength of the nozzle ring
}

type Tool struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Radius float64  `json:"radius"` // brush radius in meters
	Reach  float64  `json:"reach"`  // max aiming distance in meters
	Rate   float64  `json:"rate"`   // grams per second removed from a cell at the brush center
	Cost   float64  `json:"cost"`
	Look   ToolLook `json:"look"`
}

// Upgrade is bought level after level; each level applies its effects to
// the game's stats.
type Upgrade struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Desc    string   `json:"desc"`
	Icon    string   `json:"icon"`   // pictogram drawn by the shop (icon)
	Base    float64  `json:"base"`   // price of the first level...
	Growth  float64  `json:"growth"` // ...times this for each level bought
	Max     int      `json:"max"`    // 0 means unlimited
	Effects []Effect `json:"effects"`
}

// Effect changes a stat for each level: (base + Add×level) × Mult^level.
type Effect struct {
	Stat string  `json:"stat"`
	Mult float64 `json:"mult"`
	Add  float64 `json:"add"`
}

// stats are what upgrades act on, with their value before any upgrade.
var stats = map[string]float64{
	"tank":     50, // grams the tank holds
	"power":    1,  // multiplies how fast tools and drones sweep
	"price":    1,  // multiplies what dust and crystals sell for
	"drones":   0,  // sweeping drones
	"autosell": 0,  // above 0, the tank sells itself when full
}

// stat is a stat with every upgrade applied.
func (g *Game) stat(name string) float64 {
	v, mult := stats[name], 1.0
	for _, u := range content.Upgrades.All() {
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
func (g *Game) toolTier() int { return max(0, slices.Index(content.Tools.IDs(), g.st.Tool)) }

func (g *Game) tool() *Tool        { return content.Tools.All()[g.toolTier()] }
func (g *Game) capacity() float64  { return g.stat("tank") }
func (g *Game) powerMult() float64 { return g.stat("power") }
func (g *Game) priceMult() float64 { return g.stat("price") }
func (g *Game) autoSell() bool     { return g.stat("autosell") > 0 }
func (g *Game) space() float64     { return math.Max(0, g.capacity()-g.st.Tank) }
func (g *Game) tankFull() bool     { return g.space() <= 1e-6 }
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
	tools := content.Tools.All()
	items := make([]shopItem, 0, 1+content.Upgrades.Len())

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

	for _, u := range content.Upgrades.All() {
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

	for _, v := range content.Vehicles.All() {
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
	for _, o := range content.Items.All() {
		if o.Cost <= 0 {
			continue
		}
		items = append(items, shopItem{
			id: o.ID, name: o.Name, desc: o.Desc, icon: orDefault(o.Icon, "flag"),
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
