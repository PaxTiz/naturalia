package game

import (
	"fmt"
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/engine/ecs"
	"naturalia/internal/engine/physics"
	"naturalia/internal/palette"
	"naturalia/internal/planet"
	"slices"
)

// Links (docs/BATIMENTS.md) are passages from a building's port to
// another's (base.Link). What is kept is which ports: the rest is laid out
// from where their buildings stand. One is laid by aiming a port, then
// another, paid by the metre, and taken back by aiming at it.

// PortRef is a port of a building, by its Bootcamp.ID.
type PortRef struct {
	Building int `json:"building"`
	Port     int `json:"port"`
}

// Link is a link as it is kept.
type Link struct {
	Kind string  `json:"kind"`
	From PortRef `json:"from"`
	To   PortRef `json:"to"`
	Paid float64 `json:"paid,omitempty"`
}

// LinkAt is a link standing on Mars.
type LinkAt struct {
	Link
	Layout *base.Link
}

// LinkGhost is a link being laid.
type LinkGhost struct {
	Def    *content.Link
	From   *PortRef   // the first port, once chosen
	Aim    *PortRef   // the port aimed at
	Layout *base.Link // the way it would take, between the two
	Cost   float64
	why    string     // why there is none
	key    [2]PortRef // what Layout goes between
}

const (
	linkAim    = 40.0 // m: how far away a port is aimed at
	linkGround = 2.0  // m of ground a link may make up
	linkClear  = 0.3  // m kept around it
)

// campByID is the building with that ID, if it stands.
func (g *Game) campByID(id int) (CampAt, bool) {
	for e, b := range g.Ents.Camp.All() {
		if l := g.campLayout(g.kind(e)); b.ID == id && l != nil {
			return CampAt{e, b, b.site, l}, true
		}
	}
	return CampAt{}, false
}

// portAt is where a port is.
func (g *Game) portAt(r PortRef) (base.PortAt, bool) {
	c, ok := g.campByID(r.Building)
	if !ok || r.Port < 0 || r.Port >= len(c.Layout.Ports) {
		return base.PortAt{}, false
	}
	return c.Layout.PortAt(c.Site, r.Port, padMargin), true
}

// layLinks lays the links out again between the buildings as they stand,
// and the ground under them. Those whose building is gone (a mod's) stay in
// the save.
func (g *Game) layLinks() {
	was := g.links
	g.links, g.linked = nil, map[PortRef]bool{}
	var ways []planet.Plot
	var digs [][]planet.Hollow
	for _, k := range g.State.Links {
		def, ok := g.Content.Links.Get(k.Kind)
		from, okFrom := g.portAt(k.From)
		to, okTo := g.portAt(k.To)
		if !ok || !okFrom || !okTo || g.linked[k.From] || g.linked[k.To] {
			continue
		}
		l, _ := base.LayLink(def, from, to, g.bareGround)
		if l == nil {
			continue
		}
		if i := slices.IndexFunc(was, func(o LinkAt) bool { return o.Layout.Key == l.Key && o.Layout.Site == l.Site }); i >= 0 {
			l = was[i].Layout // as it was
		}
		g.links = append(g.links, LinkAt{k, l})
		g.linked[k.From], g.linked[k.To] = true, true
		digs = append(digs, l.Hollows())
		l.Ways(func(at, x, up vec3, half [2]float64) {
			ways = append(ways, planet.Plot{At: at, X: x, Up: up, Half: half})
		})
	}
	if len(ways) > 0 || len(g.World.Relief.Flat().Ways.List) > 0 {
		g.reshape(func() { g.World.Relief.SetWays(ways) })
	}
	g.burrow = planet.NewBurrow(digs)
	g.Player.burrow = g.burrow
	g.relay = true
}

// bareGround is how far from Mars' center the ground is in a direction, as
// it was before anything was built on it.
func (g *Game) bareGround(dir vec3) float64 { return planet.Radius + g.World.Relief.Layers(dir) }

// Links are the links standing.
func (g *Game) Links() []LinkAt { return g.links }

// LinksNear are the links within dist of a point of the world.
func (g *Game) LinksNear(at vec3, dist float64) []LinkAt {
	var out []LinkAt
	for _, l := range g.links {
		if planet.ArcDist(at.Norm(), l.Layout.Site.Origin.Norm()) < dist+l.Layout.Length/2+l.Layout.Def.Width {
			out = append(out, l)
		}
	}
	return out
}

// Linked tells whether a building's port is: always open.
func (g *Game) Linked(building, port int) bool { return g.linked[PortRef{building, port}] }

// holdsLinks tells, and says, whether an entity is a building with links:
// it stays where it stands.
func (g *Game) holdsLinks(id ecs.ID) bool {
	b := g.Ents.Camp.Get(id)
	if b == nil || !slices.ContainsFunc(g.links, func(l LinkAt) bool { return l.From.Building == b.ID || l.To.Building == b.ID }) {
		return false
	}
	g.Say(b.Name + " est relié : retire d'abord ses liens")
	g.sfx.Beep(0, 0.15, 0.1, 180)
	return true
}

// FreePorts are the ports near that no link leads to, and where they are.
func (g *Game) FreePorts(see func(r PortRef, at base.PortAt)) {
	for _, c := range g.CampsNear(g.Player.Pos, linkAim) {
		for i := range c.Layout.Ports {
			if r := (PortRef{c.ID, i}); !g.linked[r] {
				see(r, c.Layout.PortAt(c.Site, i, padMargin))
			}
		}
	}
}

// aimedPort is the free port the player looks at, from outside or from the
// building it is of.
func (g *Game) aimedPort() *PortRef {
	eye, fwd := g.Player.Eye(), g.Player.Forward()
	in, inside := g.CampAround(eye)
	var best *PortRef
	near := linkAim
	if hit, ok := g.World.Relief.Raycast(eye, fwd, linkAim); ok {
		near = hit.Sub(eye).Length() + 1.5
	}
	g.FreePorts(func(r PortRef, p base.PortAt) {
		mid := p.At.Add(p.At.Norm().Scale(base.DoorHeight / 2))
		to := mid.Sub(eye)
		t := to.Dot(fwd)
		if t < 0 || t > near || to.Sub(fwd.Scale(t)).Length() > 1.6+t*0.02 {
			return
		}
		if inside && in.ID != r.Building || !inside && to.Dot(p.Out) > 0 {
			return // through a wall, or from behind
		}
		best, near = &r, t
	})
	return best
}

// poseLink is the link being laid, from the port chosen to the one aimed
// at.
func (g *Game) poseLink(gh *Ghost) {
	k := gh.Link
	gh.OK, gh.Why = false, ""
	k.Aim = g.aimedPort()
	if k.From != nil {
		if _, ok := g.portAt(*k.From); !ok || g.linked[*k.From] {
			k.From = nil
		}
	}
	switch {
	case k.Aim == nil || k.From != nil && *k.Aim == *k.From:
		k.Layout, k.Cost = nil, 0
		gh.Why = "vise une porte"
		if k.From != nil {
			gh.Why = "vise une autre porte"
		}
		return
	case k.From == nil:
		k.Layout, k.Cost = nil, 0
		gh.OK = true
		return
	}
	if k.Layout == nil || k.key != [2]PortRef{*k.From, *k.Aim} {
		k.Layout, k.why, k.key = nil, "", [2]PortRef{*k.From, *k.Aim}
		from, _ := g.portAt(*k.From)
		to, _ := g.portAt(*k.Aim)
		if k.From.Building != k.Aim.Building {
			k.Layout, k.why = base.LayLink(k.Def, from, to, g.bareGround)
		}
	}
	if k.From.Building == k.Aim.Building {
		gh.Why = "c'est le même bâtiment"
		return
	}
	if k.Layout == nil {
		gh.Why = k.why
		return
	}
	k.Cost = 0
	it, _ := g.Content.Items.Get(gh.item)
	if it != nil {
		k.Cost = it.Cost * math.Ceil(k.Layout.Length-1e-6)
	}
	gh.Why = g.linkFits(k.Layout, *k.From, *k.Aim)
	if slices.Contains([]string{"trop long", "trop de dénivelé", "trop pentu", "sol trop accidenté"}, gh.Why) && !k.Def.Under {
		gh.Why += " : il faut un tunnel"
	}
	if gh.Why == "" && g.State.Credits < k.Cost {
		gh.Why = "crédits insuffisants"
	}
	gh.OK = gh.Why == ""
}

// linkFits is why a link laid out between two ports can't be laid, "" if it
// can.
func (g *Game) linkFits(l *base.Link, from, to PortRef) string {
	def := l.Def
	switch {
	case l.Length < def.Min:
		return "trop court"
	case l.Length > def.Max:
		return "trop long"
	case l.Under():
		if why := g.tunnelFits(l); why != "" {
			return why
		}
	case l.Rise > def.Rise:
		return "trop de dénivelé"
	case l.Slope > def.Slope:
		return "trop pentu"
	}
	why := ""
	a, _ := g.campByID(from.Building)
	b, _ := g.campByID(to.Building)
	camps := g.CampsNear(l.Site.Origin, def.Max+10)
	room := def.Width/2 + linkClear
	l.Path(func(p vec3) {
		if why != "" {
			return
		}
		at := l.Site.ToWorld(p)
		for _, c := range camps {
			skip := -1
			switch c.ID {
			case a.ID:
				skip = from.Port
			case b.ID:
				skip = to.Port
			}
			if c.Layout.Blocks(c.Site.ToLocal(at), room, skip) {
				why = "un bâtiment gêne"
				return
			}
		}
		for _, o := range g.links {
			o.Layout.Path(func(q vec3) {
				if o.Layout.Site.ToWorld(q).Sub(at).Length() < (def.Width+o.Layout.Def.Width)/2+linkClear {
					why = "un autre " + o.Layout.Def.Name + " gêne"
				}
			})
		}
		if planet.ArcDist(at.Norm(), planet.BaseUp) < base.Extent+4 {
			why = "trop près de la base"
		}
		if why == "" && math.Abs(g.World.Relief.Altitude(at)) > linkGround {
			why = "sol trop accidenté"
		}
	})
	for _, r := range g.World.RocksNear(l.Site.Origin, l.Length/2+10) { // the small ones are cleared
		if r.Size < planet.SmallRock || why != "" {
			continue
		}
		l.Path(func(p vec3) {
			if planet.ArcDist(r.Dir, l.Site.ToWorld(p).Norm()) < room+r.Size {
				why = "un rocher gêne"
			}
		})
	}
	if p := g.phys; why == "" && p != nil && planet.ArcDist(p.up, l.Site.Origin.Norm()) < physReach {
		l.Legs(func(c [4]vec3) {
			var pts []physics.V2
			bottom, top := math.Inf(1), math.Inf(-1)
			for _, q := range c {
				w := l.Site.ToWorld(q)
				pts = append(pts, p.plane(w))
				bottom, top = math.Min(bottom, p.height(w)), math.Max(top, p.height(w)+def.Height)
			}
			if p.stands(physics.Hull(pts, 0), physics.V2{}, bottom, top, 0) {
				why = "quelque chose gêne"
			}
		})
	}
	return why
}

// pickPort chooses the port aimed at: the first, or the second, the link
// laid.
func (g *Game) pickPort() {
	gh := g.Ghost
	k := gh.Link
	if !gh.OK {
		g.sfx.Beep(0, 0.12, 0.08, 180)
		return
	}
	if k.From == nil {
		k.From = k.Aim
		g.sfx.Beep(0.02, 0.1, 0.1, 523, 659)
		return
	}
	g.State.Credits -= k.Cost
	g.State.Links = append(g.State.Links, Link{k.Def.ID, *k.From, *k.Aim, k.Cost})
	g.emit(content.Event{Name: content.EvBuy, ID: gh.item, Amount: k.Cost})
	at := k.Layout.Site.Origin
	k.From, k.Aim, k.Layout, k.Cost = nil, nil, nil, 0
	g.layLinks()
	g.sfx.Beep(0.02, 0.15, 0.12, 392, 523)
	g.emit(content.Event{Name: content.EvPlace, ID: k.Def.ID, At: at})
}

// aimedLink is the link the player looks at from outside, within reach, and
// how far.
func (g *Game) aimedLink(reach float64) (LinkAt, float64, bool) {
	eye, fwd := g.Player.Eye(), g.Player.Forward()
	var best LinkAt
	found := false
	for _, l := range g.LinksNear(eye, reach) {
		s := l.Layout.Site
		if t, ok := l.Layout.Ray(s.ToLocal(eye), s.DirLocal(fwd), reach); ok {
			best, reach, found = l, t, true
		}
	}
	if found && g.Indoors(eye) {
		return LinkAt{}, 0, false // through a wall
	}
	return best, reach, found
}

// linkUse takes a link back.
func (g *Game) linkUse(l LinkAt) base.Interactable {
	return base.Interactable{
		Kit: l.Kind, At: base.ToLocal(l.Layout.Site.Origin), Prompt: "[E] Reprendre : " + l.Layout.Def.Name,
		Use: func(content.Game, vec3) {
			i := slices.Index(g.State.Links, l.Link)
			if i < 0 {
				return
			}
			g.State.Links = slices.Delete(g.State.Links, i, i+1)
			g.State.Credits += l.Paid
			g.Floaters = append(g.Floaters, Floater{Pos: g.Player.Eye().Add(g.Player.Forward().Scale(2)), Life: 2, Text: "+" + FmtNum(l.Paid) + " cr", C: palette.Gold})
			g.layLinks()
			g.sfx.Beep(0.02, 0.15, 0.12, 523, 392)
		},
	}
}

// LinkLabel is what the link being laid is, its length and price once known.
func (k *LinkGhost) LinkLabel() string {
	if k.Layout == nil {
		return k.Def.Name
	}
	return fmt.Sprintf("%s · %.0f m · %s cr", k.Def.Name, math.Ceil(k.Layout.Length-1e-6), FmtNum(k.Cost))
}

// ShotLink sets another building at a point of one's frame, turned by yaw
// (radians), and links their closest ports, for screenshots; or is about to,
// the first port chosen.
func (g *Game) ShotLink(c CampAt, link, kind string, at vec3, yaw float64, lay bool) (PortRef, bool) {
	dir := c.ToWorld(at).Norm()
	id := g.add(Placed{Kind: kind, Dir: dir})
	t := g.Ents.Transform.Get(id)
	t.SetAxes(planet.TangentAt(dir, c.Site.Dir(vec3{math.Cos(yaw), 0, math.Sin(yaw)})), dir)
	if l := g.campLayout(kind); l != nil {
		g.Ents.Camp.Get(id).Level = g.campLevel(l, dir, id)
	}
	g.standCamps()
	o, ok := g.campOf(id)
	if !ok {
		return PortRef{}, false
	}
	var from, to PortRef
	near := math.Inf(1)
	for i := range c.Layout.Ports {
		for j := range o.Layout.Ports {
			a, b := PortRef{c.ID, i}, PortRef{o.ID, j}
			pa, _ := g.portAt(a)
			pb, _ := g.portAt(b)
			if d := pa.At.Sub(pb.At).Length(); d < near && !g.linked[a] && !g.linked[b] {
				near, from, to = d, a, b
			}
		}
	}
	if !lay {
		return from, true
	}
	g.State.Links = append(g.State.Links, Link{link, from, to, 0})
	n := len(g.links)
	g.layLinks()
	return from, len(g.links) > n
}

// HoldLink holds a link from a port, for screenshots.
func (g *Game) HoldLink(link string, from PortRef) {
	if it, ok := g.Content.Items.Get(link); ok {
		g.State.Credits = math.Max(g.State.Credits, 1e8)
		g.State.Recent = append(g.State.Recent, it.ID)
		g.hold(it)
		g.Ghost.Link.From = &from
		g.poseGhost()
	}
}
