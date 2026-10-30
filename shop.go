package main

import (
	"fmt"
	"image/color"
	"math"
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
type ToolLook struct {
	HandleLen, HandleR float64
	HandleCol          color.RGBA
	HeadLen, HeadR     float64
	HeadCol            color.RGBA
	Head               headShape
	Glow               float32 // emissive strength of the nozzle ring
}

type Tool struct {
	Name   string
	Radius float64 // brush radius in meters
	Reach  float64 // max aiming distance in meters
	Rate   float64 // grams per second removed from a cell at the brush center
	Cost   float64
	Look   ToolLook
}

var tools = []Tool{
	{"Brosse à dents", 0.75, 3.5, 0.8, 0, ToolLook{0.30, 0.014, color.RGBA{80, 150, 240, 255}, 0.06, 0.022, color.RGBA{245, 245, 250, 255}, headBlock, 0}},
	{"Pinceau", 1.25, 4.5, 1.0, 60, ToolLook{0.35, 0.016, color.RGBA{190, 70, 60, 255}, 0.10, 0.03, color.RGBA{215, 175, 115, 255}, headTaper, 0}},
	{"Balai", 2.25, 6, 1.4, 500, ToolLook{0.55, 0.02, color.RGBA{165, 115, 70, 255}, 0.24, 0.12, color.RGBA{240, 205, 95, 255}, headFlare, 0}},
	{"Aspirateur", 3.75, 9, 2.2, 4_000, ToolLook{0.40, 0.025, color.RGBA{170, 175, 190, 255}, 0.22, 0.07, color.RGBA{225, 75, 80, 255}, headNozzle, 0.6}},
	{"Souffleur indus.", 6, 13, 3.5, 35_000, ToolLook{0.30, 0.05, color.RGBA{60, 64, 80, 255}, 0.30, 0.09, color.RGBA{240, 150, 55, 255}, headNozzle, 0.8}},
	{"Canon à vide", 10, 22, 6, 400_000, ToolLook{0.35, 0.06, color.RGBA{230, 232, 240, 255}, 0.26, 0.12, color.RGBA{80, 130, 235, 255}, headNozzle, 1}},
}

type upgradeDef struct {
	id, name, desc string
	base, growth   float64
	max            int // 0 means unlimited
}

var upgrades = []upgradeDef{
	{"tank", "Réservoir", "Stocke 60% de poussière en plus avant de devoir rentrer vendre", 25, 1.75, 0},
	{"power", "Turbo", "Outils et drones aspirent 30% plus vite", 40, 1.85, 0},
	{"price", "Négociation", "La poussière et les cristaux se vendent 20% plus cher", 150, 2.1, 0},
	{"drone", "Drone balayeur", "Un drone autonome qui nettoie à ta place", 300, 1.9, 0},
	{"auto", "Téléporteur", "Vend à distance dès que le réservoir est plein", 1_500, 1, 1},
	{"meteor", "Aimant à météores", "Plus de météorites, la poussière revient plus vite", 800, 2.0, 0},
}

func (g *Game) level(id string) float64 { return float64(g.st.Levels[id]) }

func (g *Game) tool() Tool              { return tools[g.st.ToolTier] }
func (g *Game) capacity() float64       { return 50 * math.Pow(1.6, g.level("tank")) }
func (g *Game) powerMult() float64      { return math.Pow(1.3, g.level("power")) }
func (g *Game) priceMult() float64      { return math.Pow(1.2, g.level("price")) }
func (g *Game) meteorInterval() float64 { return 25 / math.Pow(1.25, g.level("meteor")) }
func (g *Game) regenRate() float64      { return 0.002 * (1 + 0.2*g.level("meteor")) }
func (g *Game) autoSell() bool          { return g.level("auto") > 0 }
func (g *Game) space() float64          { return math.Max(0, g.capacity()-g.st.Tank) }
func (g *Game) tankFull() bool          { return g.space() <= 1e-6 }
func (g *Game) gemValue() float64 {
	return 20 * g.priceMult() * math.Pow(2.5, float64(g.st.ToolTier))
}

type shopItem struct {
	name, desc, info string
	cost             float64
	maxed            bool
	buy              func()
}

func (it shopItem) affordable(credits float64) bool { return !it.maxed && credits >= it.cost }

func (g *Game) shopItems() []shopItem {
	items := make([]shopItem, 0, 1+len(upgrades))

	if next := g.st.ToolTier + 1; next < len(tools) {
		t := tools[next]
		items = append(items, shopItem{
			name: "Outil : " + t.Name,
			desc: fmt.Sprintf("Rayon %.1f m, portée %.0f m, vitesse %.1f g/s", t.Radius, t.Reach, t.Rate),
			info: fmt.Sprintf("Outil %d/%d", next+1, len(tools)),
			cost: t.Cost,
			buy:  func() { g.st.ToolTier = next },
		})
	} else {
		items = append(items, shopItem{name: "Outil : " + g.tool().Name, desc: "Le meilleur outil de la galaxie", info: "Outil max", maxed: true})
	}

	for i := range upgrades {
		u := &upgrades[i]
		lvl := g.st.Levels[u.id]
		it := shopItem{
			name: u.name,
			desc: u.desc,
			info: fmt.Sprintf("Niv %d", lvl),
			cost: u.base * math.Pow(u.growth, float64(lvl)),
			buy: func() {
				g.st.Levels[u.id]++
				g.syncDrones()
			},
		}
		if u.max > 0 && lvl >= u.max {
			it.maxed, it.info = true, "Acquis"
		}
		items = append(items, it)
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
