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
	Free   bool      // floating where the player holds it, not on a surface
	Moving ecs.ID    // a placed entity being moved, out of the world meanwhile (0: a new one)
	item   string
	// rel is its front and up in the frame it stands in (front toward
	// the player, up, side), as the player turned it.
	rel        [2]vec3
	dist, lift float64 // free: how far ahead; on a surface: how high over it
}

// turnable: what is driven over (ramps, pads) and vehicles only turn
// around their up.
func (g *Game) turnable(def *content.EntityDef) bool {
	_, vehicle := g.Content.Vehicles.Get(def.ID)
	return def.Ramp == nil && def.Kick <= 0 && !vehicle
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
	if gh != nil && (rl.IsMouseButtonPressed(rl.MouseButtonRight) || rl.IsKeyPressed(rl.KeyBackspace)) {
		g.Ghost = nil
		return
	}
	if gh == nil || gh.Moving == 0 {
		it, n := g.HeldItem()
		if rl.IsKeyPressed(rl.KeyR) {
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
			if rl.IsKeyPressed(rl.KeyF) {
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
	step := turnStep
	switch {
	case rl.IsKeyDown(rl.KeyLeftControl):
		step = math.Pi / 2
	case rl.IsKeyDown(rl.KeyLeftAlt):
		step = math.Pi / 180
	}
	if rl.IsKeyDown(rl.KeyLeftShift) {
		step = -step
	}
	if rl.IsKeyPressed(rl.KeyX) {
		gh.turn(vec3{0, 1, 0}, step)
	}
	if g.turnable(def) {
		if rl.IsKeyPressed(rl.KeyC) {
			gh.turn(vec3{0, 0, 1}, step)
		}
		if rl.IsKeyPressed(rl.KeyB) {
			gh.turn(vec3{1, 0, 0}, step)
		}
	} else {
		gh.rel[1] = vec3{0, 1, 0}
		gh.rel[0] = vec3{gh.rel[0].X, 0, gh.rel[0].Z}.Norm()
	}
	if rl.IsKeyPressed(rl.KeyT) {
		gh.Free = !gh.Free
		pos, _, _, _ := g.Frame(&gh.Pose)
		gh.dist = math.Max(1.5, pos.Sub(g.Player.Eye()).Length()) // it stays where it was
	}
	if w := float64(rl.GetMouseWheelMove()); w != 0 {
		if gh.Free {
			gh.dist = math.Max(1.5, math.Min(15, gh.dist+w*0.3))
		} else {
			gh.lift = math.Max(0, math.Min(5, gh.lift+w*0.1))
		}
	}
	g.poseGhost()
	if rl.IsKeyPressed(rl.KeyF) && gh.OK {
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
func movable(def *content.EntityDef) bool { return def.Build || def.Pickup }

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
	face := planet.TangentAt(n, g.Player.Heading).Scale(-1)
	if def.Ramp != nil || def.Kick > 0 {
		face = face.Scale(-1)
	}
	side := face.Cross(n)
	ax, up, _ := e.Axes()
	rel := func(v vec3) vec3 { return vec3{v.Dot(face), v.Dot(n), v.Dot(side)} }
	pos, _, _, _ := g.Frame(e)
	g.Ghost = &Ghost{Kind: def.ID, Moving: id, rel: [2]vec3{rel(ax), rel(up)}, dist: math.Max(1.5, pos.Sub(g.Player.Eye()).Length())}
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

// poseGhost sets the ghost where the player looks: resting by its lowest
// point on the ground, or against the face of a block, lined up with it
// on a 25 cm grid; or held in the air when free.
func (g *Game) poseGhost() {
	gh := g.Ghost
	def, ok := g.Content.Entities.Get(gh.Kind)
	if !ok {
		return
	}
	p := &g.Player
	eye, fwd := p.Eye(), p.Forward()
	at := eye.Add(fwd.Scale(gh.dist))
	n := at.Norm() // the surface's up
	face := planet.TangentAt(n, p.Heading).Scale(-1)
	if def.Ramp != nil || def.Kick > 0 {
		face = face.Scale(-1) // rising away from the player, toward where they drive
	}
	frame := [3]vec3{face, n, face.Cross(n)} // what the player's turns are from
	var on ecs.ID                            // the block it rests against
	if !gh.Free {
		hit, ok := g.World.Relief.Raycast(eye, fwd, buildReach)
		reach := hit.Sub(eye).Length()
		if !ok {
			hit, reach = g.World.Relief.Surface(eye.Add(planet.TangentAt(p.LocalUp(), fwd).Scale(buildReach))), buildReach
		}
		at, n = hit, hit.Norm()
		if id, bhit, bn, ok := g.aimBlock(eye, fwd, reach); ok && g.turnable(def) {
			at, n, on = bhit, bn, id
			_, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
			frame = [3]vec3{ax, up, az}
		}
	}
	world := func(v vec3) vec3 { return frame[0].Scale(v.X).Add(frame[1].Scale(v.Y)).Add(frame[2].Scale(v.Z)) }
	ax, up := world(gh.rel[0]), world(gh.rel[1])
	var origin vec3
	if gh.Free {
		lo, hi := content.Bounds(def.Parts)
		mid := lo.Add(hi).Scale(0.5)
		origin = at.Sub(ax.Scale(mid.X).Add(up.Scale(mid.Y)).Add(ax.Cross(up).Scale(mid.Z)))
	} else {
		low := 0.0
		if len(def.Parts) > 0 {
			low = math.Inf(1)
			for _, c := range ghostCorners(def, ax, up, 0) {
				low = math.Min(low, c.Dot(n))
			}
		}
		origin = at.Sub(n.Scale(low - gh.lift))
	}
	if on != 0 {
		origin = g.snapOn(on, origin, n)
	} else if !gh.Free {
		// On a slope: up to where none of it goes into the ground.
		sink := 0.0
		for _, c := range ghostCorners(def, ax, up, 0) {
			sink = math.Max(sink, -g.World.Relief.Altitude(origin.Add(c)))
		}
		origin = origin.Add(n.Scale(sink))
	}
	gh.Pose = Transform{Dir: origin.Norm()}
	gh.Pose.Alt = origin.Length() - g.World.Relief.Surface(gh.Pose.Dir).Length()
	gh.Pose.SetAxes(ax, up)
	gh.OK = g.ghostFits(def, origin, ax, up)
}

// snapOn moves a point resting on a face of block id onto a 25 cm grid in
// the block's frame, along the face only.
func (g *Game) snapOn(id ecs.ID, at, n vec3) vec3 {
	const grid = 0.25
	pos, ax, up, az := g.Frame(g.Ents.Transform.Get(id))
	off := at.Sub(pos)
	for _, a := range [3]vec3{ax, up, az} {
		if math.Abs(a.Dot(n)) > 0.9 {
			continue // across the face: it rests on it
		}
		l := off.Dot(a)
		off = off.Add(a.Scale(math.Round(l/grid)*grid - l))
	}
	return pos.Add(off)
}

// ghostCorners are the corners of the box around an entity's parts,
// turned to ax and up, from its origin, pulled in by in.
func ghostCorners(def *content.EntityDef, ax, up vec3, in float64) []vec3 {
	lo, hi := content.Bounds(def.Parts)
	az := ax.Cross(up)
	var out []vec3
	for i := range 8 {
		x, y, z := lo.X+in, lo.Y+in, lo.Z+in
		if i&1 != 0 {
			x = hi.X - in
		}
		if i&2 != 0 {
			y = hi.Y - in
		}
		if i&4 != 0 {
			z = hi.Z - in
		}
		out = append(out, ax.Scale(x).Add(up.Scale(y)).Add(az.Scale(z)))
	}
	return out
}

// ghostFits tells if an entity would stand at origin, turned to ax and up,
// without going into the ground or into anything that takes room.
func (g *Game) ghostFits(def *content.EntityDef, origin, ax, up vec3) bool {
	if len(def.Parts) == 0 {
		return true
	}
	corners := ghostCorners(def, ax, up, 0.03) // touching is fine
	var pts []physics.V2
	bottom, top := math.Inf(1), math.Inf(-1)
	for _, c := range corners {
		w := origin.Add(c)
		if g.World.Relief.Altitude(w) < -0.15 {
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
	return !g.phys.w.Overlap(shape, physics.V2{}, 0, bottom, top, layerStatic|layerEntity|layerVehicle|layerRobot|layerProp|layerPlayer, nil)
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
	if def, ok := g.Content.Entities.Get(gh.Kind); ok && def.Mass > 0 && e.Alt > 0.05 {
		g.Ents.Air.Set(id, Air{})
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
	g.GiveItem(it.ID, 1)
	for g.itemSel = 0; g.itemSel < g.Content.Items.Len(); g.itemSel++ {
		if h, _ := g.HeldItem(); h == it {
			break
		}
	}
	g.Ghost = &Ghost{Kind: content.Qualify(it.Places), item: it.ID, rel: [2]vec3{{1, 0, 0}, {0, 1, 0}}, dist: 4}
	g.Ghost.turn(vec3{1, 0, 0}, deg*math.Pi/180)
}
