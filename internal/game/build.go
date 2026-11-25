package game

import (
	"math"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/physics"
	"naturalia/internal/planet"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Building (docs/CONSTRUCTION.md): F shows the held item where it would
// stand, see-through, on what the player looks at; X, C and B turn it, T
// lets it float free of the surfaces, the wheel moves it, F sets it down
// and right click puts it away.

const (
	buildReach = 8.0          // m: how far the ghost goes on what is aimed
	turnStep   = math.Pi / 12 // 15°: a press of X, C or B
)

// Ghost is the item being placed.
type Ghost struct {
	Kind   string
	Pose   Transform // where it would stand
	OK     bool      // nothing in the way: F sets it down
	Why    string    // or why not, when it says (a bootcamp)
	Free   bool      // floating where the player holds it, not on a surface
	Moving ecs.ID    // a placed entity being moved, out of the world meanwhile (0: a new one)
	item   string
	// rel is its front and up in the frame it stands in (front toward
	// the player, up, side), as the player turned it.
	rel        [2]vec3
	frame      [3]vec3
	grid       *OnGrid      // where it would stand on a grid (grid.go), if it does
	home       *planet.Grid // the one it is kept on, far from any (alignGhost)
	from       *planet.Grid // the one it was taken from
	fresh      bool         // the grid is its own: it founds it
	Sheet      *Sheet       // the grid's lines shown
	dist, lift float64      // free: how far ahead; on a surface: how high over it
}

// turnable: what is driven over (ramps, pads) and vehicles only turn
// around their up.
func (g *Game) turnable(def *content.EntityDef) bool {
	_, vehicle := g.Content.Vehicles.Get(def.ID)
	return def.Ramp == nil && def.Kick <= 0 && !vehicle && !isCamp(def)
}

func (g *Game) updateItems() {
	if g.Mode != ModePlay || g.Riding != 0 {
		g.Ghost = nil // what was being moved stays where it was
		return
	}
	gh := g.Ghost
	if gh != nil && gh.Moving != 0 && !g.Ents.Alive(gh.Moving) {
		g.Ghost, gh = nil, nil // gone meanwhile (a mod)
	}
	if gh != nil && (rl.IsMouseButtonPressed(rl.MouseButtonRight) || KeyPressed(ActStow)) {
		g.Ghost = nil
		return
	}
	if gh == nil || gh.Moving == 0 {
		it, n := g.HeldItem()
		if KeyPressed(ActNextItem) {
			g.itemSel++
			it, n = g.HeldItem()
		}
		switch {
		case gh == nil && rl.IsMouseButtonPressed(rl.MouseButtonRight):
			if id := g.AimedMovable(); id != 0 {
				g.grab(id)
			}
			return
		case it == nil || n == 0:
			g.Ghost = nil
			return
		case gh == nil:
			if KeyPressed(ActPlace) {
				g.Ghost = &Ghost{rel: [2]vec3{{1, 0, 0}, {0, 1, 0}}, dist: 4}
				g.Ghost.Kind, g.Ghost.item = content.Qualify(it.Places), it.ID
				g.poseGhost()
			}
			return
		}
		gh.Kind, gh.item = content.Qualify(it.Places), it.ID
	}
	def, ok := g.Content.Entities.Get(gh.Kind)
	if !ok {
		g.Ghost = nil
		return
	}
	if g.campLayout(def.ID) == nil {
		if KeyPressed(ActGrid) {
			g.ToggleGrid()
			gh.home = nil
		}
		if KeyPressed(ActAlign) {
			g.alignGhost()
		}
		if KeyPressed(ActGridStep) && g.Settings.Grid {
			g.NextGridStep()
		}
	}
	step, quarter := turnStep, math.Pi/2
	if gh.grid != nil { // on the grid, quarter turns come first
		step, quarter = quarter, step
	}
	switch {
	case KeyDown(ActTurnQuarter):
		step = quarter
	case KeyDown(ActTurnDegree):
		step = math.Pi / 180
	}
	if KeyDown(ActTurnBack) {
		step = -step
	}
	if KeyPressed(ActTurnY) {
		gh.turn(vec3{0, 1, 0}, step)
	}
	if g.turnable(def) {
		if KeyPressed(ActTurnZ) {
			gh.turn(vec3{0, 0, 1}, step)
		}
		if KeyPressed(ActTurnX) {
			gh.turn(vec3{1, 0, 0}, step)
		}
	} else {
		gh.rel[1] = vec3{0, 1, 0}
		gh.rel[0] = vec3{gh.rel[0].X, 0, gh.rel[0].Z}.Norm()
	}
	if KeyPressed(ActStick) {
		gh.Free = !gh.Free
		pos, _, _, _ := g.Frame(&gh.Pose)
		gh.dist = math.Max(1.5, pos.Sub(g.Player.Eye()).Length()) // it stays where it was
	}
	if w := float64(rl.GetMouseWheelMove()); w != 0 {
		if gh.Free {
			gh.dist = math.Max(1.5, math.Min(15, gh.dist+w*0.3))
		} else if gh.grid != nil { // a level a notch, down into the ground too
			gh.lift = math.Max(-5, math.Min(5, gh.lift+w*planet.GridRise))
		} else {
			gh.lift = math.Max(0, math.Min(5, gh.lift+w*0.1))
		}
	}
	g.poseGhost()
	if KeyPressed(ActPlace) && gh.OK {
		g.placeGhost()
	}
}

// held is the placed entity being moved, out of the world meanwhile; 0
// for none.
func (g *Game) held() ecs.ID {
	if g.Ghost == nil {
		return 0
	}
	return g.Ghost.Moving
}

// Held tells if entity id is being moved: not drawn where it was.
func (g *Game) Held(id ecs.ID) bool { return id != 0 && id == g.held() }

// movable: what right click picks up to move, a block or anything put
// back in the inventory.
func movable(def *content.EntityDef) bool { return def.Build || def.Pickup || isCamp(def) }

// AimedMovable is the entity right click would move: the nearest the
// player looks at, within reach, before the ground; 0 for none.
func (g *Game) AimedMovable() ecs.ID {
	eye, fwd := g.Player.Eye(), g.Player.Forward()
	reach := buildReach
	if hit, ok := g.World.Relief.Raycast(eye, fwd, buildReach); ok {
		reach = hit.Sub(eye).Length()
	}
	var best ecs.ID
	for _, id := range g.EntitiesNear(buildReach + 12) {
		if def, ok := g.Content.Entities.Get(g.kind(id)); !ok || !movable(def) || id == g.Riding {
			continue
		}
		if t, _, ok := g.rayEntity(id, eye, fwd, reach); ok {
			reach, best = t, id
		}
	}
	for _, c := range g.CampsNear(eye, buildReach) {
		if t, ok := rayCamp(c, eye, fwd, reach); ok {
			reach, best = t, c.Ent
		}
	}
	return best
}

// grab takes a placed entity in hand to move it, as it was turned: it
// leaves its spot (and its body) until set down again, or put back.
func (g *Game) grab(id ecs.ID) {
	e := g.Ents.Transform.Get(id)
	def, ok := g.Content.Entities.Get(g.kind(id))
	if !ok {
		return
	}
	n := e.Dir
	face := g.facing(def, n)
	var from *planet.Grid
	if grid, ok := g.gridOf(id); ok {
		from = &grid
		if g.Settings.Grid { // as it stands on its grid
			face, n = grid.Front(face), grid.Up
		}
	}
	ax, up, _ := e.Axes()
	pos, _, _, _ := g.Frame(e)
	g.Ghost = &Ghost{Kind: def.ID, Moving: id, from: from, frame: [3]vec3{face, n, face.Cross(n)}, dist: math.Max(1.5, pos.Sub(g.Player.Eye()).Length())}
	g.Ghost.setRel(ax, up)
	g.Ents.Air.Remove(id)
	if bp := g.Ents.Body.Get(id); bp != nil && g.phys != nil {
		g.phys.w.Remove(*bp)
		g.Ents.Body.Remove(id)
	}
	g.poseGhost()
	g.sfx.Beep(0.02, 0.1, 0.1, 523, 392)
}

// turn turns the ghost by angle around an axis of the frame it stands in.
func (gh *Ghost) turn(axis vec3, angle float64) {
	gh.rel[0] = rotate(gh.rel[0], axis, angle).Norm()
	gh.rel[1] = rotate(gh.rel[1], axis, angle).Norm()
}

// inFrame is a direction of the frame the ghost stands in, in the world.
func (gh *Ghost) inFrame(v vec3) vec3 {
	return gh.frame[0].Scale(v.X).Add(gh.frame[1].Scale(v.Y)).Add(gh.frame[2].Scale(v.Z))
}

// setRel takes the turns that give the front ax and the up up in the frame
// the ghost stands in.
func (gh *Ghost) setRel(ax, up vec3) {
	rel := func(v vec3) vec3 { return vec3{v.Dot(gh.frame[0]), v.Dot(gh.frame[1]), v.Dot(gh.frame[2])} }
	gh.rel = [2]vec3{rel(ax), rel(up)}
}

// aimSurface is the surface the player aims at to set a thing down: where,
// its normal, and the block it is a face of, 0 for the ground.
func (g *Game) aimSurface(def *content.EntityDef, eye, fwd vec3) (at, n vec3, on ecs.ID) {
	hit, ok := g.World.Relief.Raycast(eye, fwd, buildReach)
	reach := hit.Sub(eye).Length()
	if !ok {
		hit, reach = g.World.Relief.Surface(eye.Add(planet.TangentAt(g.Player.LocalUp(), fwd).Scale(buildReach))), buildReach
	}
	if !g.turnable(def) {
		return hit, hit.Norm(), 0
	}
	if id, bhit, bn, ok := g.aimBlock(eye, fwd, reach); ok {
		return bhit, bn, id
	}
	if !ok { // too far ahead: on what stands there, from above
		n := hit.Norm()
		if h := eye.Length() - hit.Length(); h > 0 {
			if id, bhit, bn, ok := g.aimBlock(hit.Add(n.Scale(h)), n.Scale(-1), h); ok {
				return bhit, bn, id
			}
		}
	}
	return hit, hit.Norm(), 0
}

// poseGhost sets the ghost where the player looks: on the grid (grid.go);
// or without it, resting by its lowest point on the ground, or against the
// face of a block, turned like it; or held in the air when free.
func (g *Game) poseGhost() {
	gh := g.Ghost
	def, ok := g.Content.Entities.Get(gh.Kind)
	if !ok {
		return
	}
	gh.grid, gh.Sheet, gh.fresh, gh.Why = nil, nil, false, ""
	if l := g.campLayout(def.ID); l != nil {
		g.poseCamp(gh, l)
		return
	}
	if g.Settings.Grid && g.poseOnGrid(gh, def) {
		return
	}
	p := &g.Player
	eye, fwd := p.Eye(), p.Forward()
	at := eye.Add(fwd.Scale(gh.dist))
	n := at.Norm() // the surface's up
	face := g.facing(def, n)
	gh.frame = [3]vec3{face, n, face.Cross(n)} // what the player's turns are from
	var on ecs.ID                              // the block it rests against
	if !gh.Free {
		if at, n, on = g.aimSurface(def, eye, fwd); on != 0 {
			_, ax, up, az := g.Frame(g.Ents.Transform.Get(on))
			gh.frame = [3]vec3{ax, up, az}
		}
	}
	ax, up := gh.inFrame(gh.rel[0]), gh.inFrame(gh.rel[1])
	lift := math.Max(0, gh.lift)
	var origin vec3
	switch {
	case gh.Free:
		lo, hi := content.Bounds(def.Parts)
		mid := lo.Add(hi).Scale(0.5)
		origin = at.Sub(ax.Scale(mid.X).Add(up.Scale(mid.Y)).Add(ax.Cross(up).Scale(mid.Z)))
	case on != 0:
		origin = at.Sub(n.Scale(lowest(def, ax, up, n) - lift))
	default:
		origin = g.restOnGround(def, at, n, ax, up, lift)
	}
	gh.Pose = Transform{Dir: origin.Norm()}
	gh.Pose.Alt = origin.Length() - g.World.Relief.Surface(gh.Pose.Dir).Length()
	gh.Pose.SetAxes(ax, up)
	gh.OK = g.ghostFits(def, origin, ax, up, false)
}

// lowest is how high along n the lowest point of a thing turned to ax and
// up is, from its origin.
func lowest(def *content.EntityDef, ax, up, n vec3) float64 {
	if len(def.Parts) == 0 {
		return 0
	}
	low := math.Inf(1)
	for _, c := range ghostCorners(def, ax, up, 0) {
		low = math.Min(low, c.Dot(n))
	}
	return low
}

// restOnGround is the origin of a thing turned to ax and up resting on the
// ground at at, its up n, lift over it; on a slope, up to where none of it
// goes into the ground.
func (g *Game) restOnGround(def *content.EntityDef, at, n, ax, up vec3, lift float64) vec3 {
	origin := at.Sub(n.Scale(lowest(def, ax, up, n) - lift))
	sink := 0.0
	for _, c := range ghostCorners(def, ax, up, 0) {
		sink = math.Max(sink, -g.World.Relief.Altitude(origin.Add(c)))
	}
	return origin.Add(n.Scale(sink))
}

// ghostCorners are the corners of the box around an entity's parts,
// turned to ax and up, from its origin, pulled in by in.
func ghostCorners(def *content.EntityDef, ax, up vec3, in float64) []vec3 {
	lo, hi := content.Bounds(def.Parts)
	d := vec3{in, in, in}
	return boxCorners(lo.Add(d), hi.Sub(d), ax, up)
}

// boxCorners are the corners of the box lo..hi turned to ax and up.
func boxCorners(lo, hi, ax, up vec3) []vec3 {
	az := ax.Cross(up)
	out := make([]vec3, 0, 8)
	for i := range 8 {
		c := lo
		if i&1 != 0 {
			c.X = hi.X
		}
		if i&2 != 0 {
			c.Y = hi.Y
		}
		if i&4 != 0 {
			c.Z = hi.Z
		}
		out = append(out, ax.Scale(c.X).Add(up.Scale(c.Y)).Add(az.Scale(c.Z)))
	}
	return out
}

// ghostFits tells if an entity would stand at origin, turned to ax and up,
// without going into the ground (unless it is graded under it) or into
// anything that takes room.
func (g *Game) ghostFits(def *content.EntityDef, origin, ax, up vec3, graded bool) bool {
	if len(def.Parts) == 0 {
		return true
	}
	corners := ghostCorners(def, ax, up, 0.03) // touching is fine
	var pts []physics.V2
	bottom, top := math.Inf(1), math.Inf(-1)
	for _, c := range corners {
		w := origin.Add(c)
		if !graded && g.World.Relief.Altitude(w) < -0.15 {
			return false // into the ground, more than its bumps
		}
		if p := g.phys; p != nil {
			pts = append(pts, p.plane(w))
			bottom, top = math.Min(bottom, p.height(w)), math.Max(top, p.height(w))
		}
	}
	if g.phys == nil {
		return true
	}
	shape := physics.Hull(pts, 0)
	return !g.phys.w.OverlapAny(shape, physics.V2{}, 0, bottom, top, layerStatic|layerEntity|layerVehicle|layerRobot|layerProp|layerPlayer, func(b *physics.Body) bool {
		if def.Build && g.phys.stones[b] {
			return false // a small rock: moved out of the way
		}
		// The engine's outlines and slices swell around a block that leans:
		// with another block, their boxes tell.
		if other, ok := g.Content.Entities.Get(g.kind(ecs.ID(b.ID))); b.ID != 0 && ok && other.Build {
			return g.boxesMeet(def, origin, ax, up, ecs.ID(b.ID), 0.03)
		}
		return true
	})
}

// placeGhost sets the ghost down: as a new entity, or the one moved at
// its new spot; a thing pushed around set in the air falls.
func (g *Game) placeGhost() {
	gh := g.Ghost
	e := gh.Pose
	id := gh.Moving
	if id != 0 {
		*g.Ents.Transform.Get(id) = e
		if r := g.Ents.Roam.Get(id); r != nil {
			r.Home = e.Dir // a patrolling robot works around its new spot
		}
		g.Ghost = nil
	} else {
		id = g.add(Placed{Kind: gh.Kind, Dir: e.Dir, Yaw: e.Yaw, Alt: e.Alt, Pitch: e.Pitch, Roll: e.Roll})
		g.State.Items[gh.item]--
	}
	if def, ok := g.Content.Entities.Get(gh.Kind); ok && gh.grid != nil && def.Mass == 0 {
		g.Ents.Grid.Set(id, *gh.grid)
	} else if g.Ents.Grid.Has(id) {
		g.Ents.Grid.Remove(id)
	}
	g.regrade = true
	if gh.fresh { // the next one lines up on this one as it stands
		face := gh.grid.Front(gh.frame[0])
		ax, up, _ := e.Axes()
		gh.frame = [3]vec3{face, gh.grid.Up, face.Cross(gh.grid.Up)}
		gh.setRel(ax, up)
	}
	if gh.grid != nil {
		gh.lift = 0 // the next one is level with this one
	}
	g.syncGround()
	if def, ok := g.Content.Entities.Get(gh.Kind); ok && def.Mass > 0 && e.Alt > 0.05 {
		g.Ents.Air.Set(id, Air{})
	}
	if g.campLayout(gh.Kind) != nil {
		g.standCamps() // the ground flattened under it, its walls where it stands
	}
	g.sfx.Beep(0.02, 0.15, 0.12, 392, 523)
	if gh.Moving == 0 {
		g.emit(content.Event{Name: content.EvPlace, ID: gh.Kind, At: g.World.Relief.Surface(e.Dir)})
	}
}

// HoldGhost gives one of an item and holds its ghost, tipped by deg
// degrees around the player's view, for screenshots.
func (g *Game) HoldGhost(item string, deg float64) {
	it, ok := g.Content.Items.Get(item)
	if !ok {
		return
	}
	if g.State.Items[it.ID] == 0 {
		g.GiveItem(it.ID, 1)
	}
	for g.itemSel = 0; g.itemSel < g.Content.Items.Len(); g.itemSel++ {
		if h, _ := g.HeldItem(); h == it {
			break
		}
	}
	g.Ghost = &Ghost{Kind: content.Qualify(it.Places), item: it.ID, rel: [2]vec3{{1, 0, 0}, {0, 1, 0}}, dist: 4}
	g.Ghost.turn(vec3{1, 0, 0}, deg*math.Pi/180)
	g.poseGhost()
}
