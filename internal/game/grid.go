package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/engine/ecs"
	"naturalia/internal/engine/mathx"
	"naturalia/internal/planet"
	"slices"
)

// Building on a grid (docs/CONSTRUCTION.md): the first thing set down
// founds a grid (planet.Grid), what is set down near it lines up on it,
// and the ground is graded level under the blocks that stand on it
// (planet.Plot). Near the base, grids follow its axes.

// GridSteps are the steps of the grid's lines, in m.
var GridSteps = []float64{0.25, 0.5, 1, 2, 4}

const gridNear = 8.0 // m: how far from a thing its grid is taken up

// OnGrid is where a thing stands on a grid: the grid, and the box it
// fills there, in the grid's frame. Ground is for a block on the ground,
// which is made level under it.
type OnGrid struct {
	planet.Grid
	Lo     vec3 `json:"lo"`
	Hi     vec3 `json:"hi"`
	Ground bool `json:"ground,omitempty"`
}

// Sheet is the part of the grid shown around the ghost: the lines every
// Step along its axes from At, out to Reach. Feet go from what is under
// the ghost's corners up to them, where it hangs over it.
type Sheet struct {
	At, X, Up, Z vec3
	Step, Reach  float64
	Feet         [4][2]vec3
}

// ToggleGrid switches the grid on or off.
func (g *Game) ToggleGrid() {
	g.Settings.Grid = !g.Settings.Grid
	g.settingsChanged = true
}

// NextGridStep takes the next of the grid's steps.
func (g *Game) NextGridStep() {
	i := slices.Index(GridSteps, g.Settings.GridStep)
	g.Settings.GridStep = GridSteps[(i+1)%len(GridSteps)]
	g.settingsChanged = true
}

// facing is the way a thing set down at up n is turned to: its front
// toward the player, or rising away from them, toward where they drive.
func (g *Game) facing(def *content.EntityDef, n vec3) vec3 {
	face := planet.TangentAt(n, g.Player.Heading)
	if def.Ramp != nil || def.Kick > 0 {
		return face
	}
	return face.Scale(-1)
}

// nominal is the box a thing is meant to fill, in its own frame: its cell,
// or its parts' but for the centimeter their trims stick out.
func nominal(def *content.EntityDef) (lo, hi vec3) {
	if c := def.Cell; c != (vec3{}) {
		return vec3{-c.X / 2, 0, -c.Z / 2}, vec3{c.X / 2, c.Y, c.Z / 2}
	}
	lo, hi = content.Bounds(def.Parts)
	for _, v := range []*float64{&lo.X, &lo.Y, &lo.Z, &hi.X, &hi.Y, &hi.Z} {
		*v = planet.Snap(*v, 0.025)
	}
	return lo, hi
}

// boxIn is the box a thing turned to ax and up fills in a grid's frame,
// from its origin.
func boxIn(grid planet.Grid, def *content.EntityDef, ax, up vec3) (lo, hi vec3) {
	a, b := nominal(def)
	inf := math.Inf(1)
	lo, hi = vec3{inf, inf, inf}, vec3{-inf, -inf, -inf}
	for _, c := range boxCorners(a, b, ax, up) {
		l := grid.DirLocal(c)
		lo = vec3{math.Min(lo.X, l.X), math.Min(lo.Y, l.Y), math.Min(lo.Z, l.Z)}
		hi = vec3{math.Max(hi.X, l.X), math.Max(hi.Y, l.Y), math.Max(hi.Z, l.Z)}
	}
	return lo, hi
}

// boxOf is the box entity id fills in a grid's frame.
func (g *Game) boxOf(grid planet.Grid, id ecs.ID) (lo, hi vec3, ok bool) {
	def, ok := g.Content.Entities.Get(g.kind(id))
	if !ok {
		return lo, hi, false
	}
	pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(id))
	lo, hi = boxIn(grid, def, ax, up)
	at := grid.Local(pos)
	return at.Add(lo), at.Add(hi), true
}

// gridOn is the grid a block turned to ax and up at pos would found, from
// its lowest corner: along its sides, if one stands nearly upright.
func gridOn(def *content.EntityDef, pos, ax, up vec3) (planet.Grid, bool) {
	n := pos.Norm()
	axes := [3]vec3{ax, up, ax.Cross(up)}
	for i, a := range axes {
		if math.Abs(a.Dot(n)) < math.Cos(10*math.Pi/180) {
			continue
		}
		grid := planet.Grid{At: pos, X: axes[(i+1)%3], Up: a.Scale(math.Copysign(1, a.Dot(n)))}
		lo, _ := boxIn(grid, def, ax, up)
		grid.At = grid.World(lo)
		return grid, true
	}
	return planet.Grid{}, false
}

// gridOf is the grid of entity id: the one it was set down on, or a
// block's own.
func (g *Game) gridOf(id ecs.ID) (planet.Grid, bool) {
	def, ok := g.Content.Entities.Get(g.kind(id))
	if !ok {
		return planet.Grid{}, false
	}
	if o := g.Ents.Grid.Get(id); o != nil {
		return o.Grid, true
	}
	if !def.Build {
		return planet.Grid{}, false
	}
	pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(id))
	return gridOn(def, pos, ax, up)
}

// gridAt is the grid taken up at a point: the nearest thing's, within
// reach.
func (g *Game) gridAt(at vec3, reach float64) (planet.Grid, bool) {
	best, found := planet.Grid{}, false
	for _, id := range g.EntitiesNear(buildReach + reach + 15) {
		if g.Held(id) {
			continue
		}
		grid, ok := g.gridOf(id)
		if !ok || !grid.Reaches(at) {
			continue
		}
		lo, hi, ok := g.boxOf(grid, id)
		if !ok {
			continue
		}
		l := grid.Local(at)
		if d := math.Hypot(outside(l.X, lo.X, hi.X), outside(l.Z, lo.Z, hi.Z)); d < reach {
			best, reach, found = grid, d, true
		}
	}
	return best, found
}

// outside is how far v is from the span lo..hi.
func outside(v, lo, hi float64) float64 { return math.Max(0, math.Max(lo-v, v-hi)) }

// along is the component of v along axis i.
func along(v vec3, i int) float64 { return [3]float64{v.X, v.Y, v.Z}[i] }

func setAlong(v *vec3, i int, x float64) { *[3]*float64{&v.X, &v.Y, &v.Z}[i] = x }

// square tells whether a thing turned to ax and up has its sides along the
// grid's axes.
func square(grid planet.Grid, ax, up vec3) bool {
	for _, v := range []vec3{ax, up} {
		l := grid.DirLocal(v)
		if math.Max(math.Abs(l.X), math.Max(math.Abs(l.Y), math.Abs(l.Z))) < 1-1e-6 {
			return false
		}
	}
	return true
}

// snapSide is where to move the span lo..hi along a grid's axis: a
// block's side on a line; another thing's middle, or that of what is
// askew, on a line or halfway between two.
func snapSide(lo, hi, step float64, sides bool) float64 {
	if sides {
		return planet.SnapSpan(lo, hi, step)
	}
	mid := (lo + hi) / 2
	return planet.Snap(mid, step/2) - mid
}

// snapRise is where to move the span lo..hi up or down to: level with
// another block's bottom or top, or a level of the grid.
func snapRise(lo, hi float64, edges []float64) float64 {
	best, near := planet.Snap(lo, planet.GridRise)-lo, 0.3
	for _, e := range edges {
		for _, d := range [2]float64{e - lo, e - hi} {
			if math.Abs(d) < near {
				best, near = d, math.Abs(d)
			}
		}
	}
	return best
}

// rise is the way a ramp rises, in its own frame: one by its Ramp, or by
// a wedge that makes up most of it.
func rise(def *content.EntityDef) (vec3, bool) {
	if def.Ramp != nil {
		return vec3{1, 0, 0}, true
	}
	if def.Rise != (vec3{}) {
		return def.Rise, true
	}
	lo, hi := nominal(def)
	for _, p := range def.Parts {
		if p.Shape == "wedge" && p.Size.X*p.Size.Z > 0.6*(hi.X-lo.X)*(hi.Z-lo.Z) {
			a := p.Yaw * math.Pi / 180
			return vec3{math.Round(math.Cos(a)), 0, math.Round(math.Sin(a))}, true
		}
	}
	return vec3{}, false
}

// riseOn is the way a ramp standing upright, turned to ax and up, rises
// along a grid's axes.
func riseOn(grid planet.Grid, def *content.EntityDef, ax, up vec3) (vec3, bool) {
	l, ok := rise(def)
	if !ok || up.Dot(grid.Up) < 1-1e-6 {
		return vec3{}, false
	}
	d := grid.DirLocal(ax.Scale(l.X).Add(ax.Cross(up).Scale(l.Z)))
	d = vec3{math.Round(d.X), 0, math.Round(d.Z)}
	return d, d.Length() == 1
}

// join is where what fills lo..hi around its origin goes on from the ramp
// that fills blo..bhi and rises along to: from its top (high) or its foot,
// in its line, its edge meeting the ramp's: another ramp goes on rising,
// anything else has its top there. own is the way it rises itself.
func join(blo, bhi, to vec3, high bool, lo, hi, own vec3) vec3 {
	i, j := 0, 2
	if to.X == 0 {
		i, j = 2, 0
	}
	s := along(to, i)
	if !high {
		s = -s
	}
	var pos vec3
	if s > 0 {
		setAlong(&pos, i, along(bhi, i)-along(lo, i))
	} else {
		setAlong(&pos, i, along(blo, i)-along(hi, i))
	}
	setAlong(&pos, j, (along(blo, j)+along(bhi, j)-along(lo, j)-along(hi, j))/2)
	tall := hi.Y - lo.Y
	edge := tall // how high its own edge there is
	if own.Dot(to)*s*along(to, i) > 0.5 {
		edge = 0
	}
	bottom := bhi.Y - edge
	if !high {
		bottom = blo.Y - edge
		if edge > 0.5 {
			bottom = blo.Y
		}
	}
	pos.Y = math.Max(bottom, blo.Y-0.5) - lo.Y
	if high && bottom < blo.Y {
		pos.Y = blo.Y - lo.Y
	}
	return pos
}

// Level is a height chosen on a grid: a thing's bottom at y there.
type Level struct {
	planet.Grid
	y float64
}

// straight tells whether the ghost's sides are along its frame's axes.
func (gh *Ghost) straight() bool {
	for _, v := range gh.rel {
		if math.Max(math.Abs(v.X), math.Max(math.Abs(v.Y), math.Abs(v.Z))) < 1-1e-6 {
			return false
		}
	}
	return true
}

// straighten turns the ghost along its frame's axes, the least.
func (g *Game) straighten(gh *Ghost, def *content.EntityDef) {
	axes := [3]vec3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}
	nearest := func(v vec3, but int) (vec3, int) {
		best := -1
		for i := range 3 {
			if i != but && (best < 0 || math.Abs(along(v, i)) > math.Abs(along(v, best))+1e-9) {
				best = i
			}
		}
		return axes[best].Scale(math.Copysign(1, along(v, best))), best
	}
	ax, up := gh.rel[0], gh.rel[1]
	if !g.turnable(def) {
		gh.rel[1] = axes[1]
		gh.rel[0], _ = nearest(ax, 1)
	} else if math.Max(math.Abs(up.X), math.Max(math.Abs(up.Y), math.Abs(up.Z))) >= math.Max(math.Abs(ax.X), math.Max(math.Abs(ax.Y), math.Abs(ax.Z)))-1e-9 {
		var i int // the straighter of the two first
		gh.rel[1], i = nearest(up, -1)
		gh.rel[0], _ = nearest(ax, i)
	} else {
		var i int
		gh.rel[0], i = nearest(ax, -1)
		gh.rel[1], _ = nearest(up, i)
	}
}

// gridUnder is the grid to line up on over or against entity id: its own,
// if it was turned askew on the one it was set down on.
func (g *Game) gridUnder(id ecs.ID) (planet.Grid, bool) {
	def, ok := g.Content.Entities.Get(g.kind(id))
	if !ok {
		return planet.Grid{}, false
	}
	pos, ax, up, _ := g.Frame(g.Ents.Transform.Get(id))
	if grid, ok := g.gridOf(id); ok && square(grid, ax, up) {
		return grid, true
	}
	return gridOn(def, pos, ax, up)
}

// levelOn is the ghost's chosen level in a grid: the same height at at, on
// a level of that grid if it was chosen on another.
func (g *Game) levelOn(gh *Ghost, grid planet.Grid, at vec3) float64 {
	l := gh.level
	if l.Same(grid) {
		return l.y
	}
	p := l.Local(at)
	p.Y = l.y
	q := grid.Local(l.World(p))
	return q.Y + snapRise(q.Y, q.Y, g.gridEdges(grid, q, q))
}

// aimLevel is where the player aims at the ghost's chosen level: where
// their sight crosses it, unless a block is in the way; otherwise over or
// under what they aim at.
func (g *Game) aimLevel(gh *Ghost, def *content.EntityDef, eye, fwd vec3) (at, n vec3, on ecs.ID) {
	at, n, on = g.aimSurface(def, eye, fwd)
	l := gh.level
	e, f := l.Local(eye), l.DirLocal(fwd)
	if math.Abs(f.Y) < 1e-6 {
		return at, n, on
	}
	t := (l.y - e.Y) / f.Y
	if t < 0.3 || t*math.Hypot(f.X, f.Z) > buildReach {
		return at, n, on
	}
	if id, _, _, ok := g.aimBlock(eye, fwd, t+0.02, def.Mass > 0); ok {
		return at, n, id
	}
	if hit, ok := g.World.Relief.Raycast(eye, fwd, buildReach); ok && hit.Sub(eye).Length() < t {
		return at, n, 0
	}
	return eye.Add(fwd.Scale(t)), l.Up, 0
}

// Raise moves the ghost's height on the grid a stop up (by > 0) or down: a
// level of the grid, or that of the blocks near; with far, straight to the
// next of theirs. From there on it stays at that height, whatever is aimed
// at, until alignGhost. What founds its own grid goes over the ground.
func (g *Game) Raise(by int, far bool) {
	gh := g.Ghost
	if gh == nil || gh.grid == nil || gh.Free || by == 0 {
		return
	}
	dir := math.Copysign(1, float64(by))
	for range max(by, -by) {
		if gh.fresh {
			gh.lift = math.Max(-5, math.Min(5, gh.lift+dir*planet.GridRise))
		} else {
			y := gh.grid.Lo.Y
			if gh.level != nil && gh.level.Same(gh.grid.Grid) {
				y = gh.level.y
			}
			gh.level = &Level{gh.grid.Grid, g.nextLevel(gh.grid, y, dir, far)}
		}
		g.poseGhost()
	}
}

// nextLevel is the next stop over y (dir 1) or under it (-1).
func (g *Game) nextLevel(o *OnGrid, y, dir float64, far bool) float64 {
	const near, same = 4.0, 0.12
	tall := o.Hi.Y - o.Lo.Y
	var stops []float64 // on the blocks near, or level with their tops
	g.onGridNear(o.Grid, o.Lo, o.Hi, func(_ ecs.ID, blo, bhi vec3) {
		if math.Max(math.Max(blo.X-o.Hi.X, o.Lo.X-bhi.X), math.Max(blo.Z-o.Hi.Z, o.Lo.Z-bhi.Z)) < near {
			stops = append(stops, blo.Y, bhi.Y, bhi.Y-tall)
		}
	})
	step := planet.GridRise
	if far {
		step = 1
	}
	next := planet.Snap(y, step) + dir*step
	if prev := planet.Snap(y, step); (prev-y)*dir > same {
		next = prev
	}
	best, found := next, false
	for _, e := range stops {
		d := (e - y) * dir
		if d < 0.01 || !far && d > (next-y)*dir+same {
			continue
		}
		if !found || d < (best-y)*dir {
			best, found = e, true
		}
	}
	return best
}

// poseOnGrid sets the ghost on the grid where the player looks; false
// when it doesn't go on one: a vehicle, or against a block set askew.
func (g *Game) poseOnGrid(gh *Ghost, def *content.EntityDef) bool {
	if _, vehicle := g.Content.Vehicles.Get(def.ID); vehicle || len(def.Parts) == 0 {
		return false
	}
	step := g.Settings.GridStep
	p := &g.Player
	eye, fwd := p.Eye(), p.Forward()
	at := eye.Add(fwd.Scale(gh.dist))
	n := at.Norm()
	var on ecs.ID
	switch {
	case gh.Free:
	case gh.level != nil:
		at, n, on = g.aimLevel(gh, def, eye, fwd)
	default:
		at, n, on = g.aimSurface(def, eye, fwd)
	}
	if !gh.askew && !gh.straight() {
		g.straighten(gh, def)
	}

	// The grid: that of the block aimed at, of what is near, or a new one.
	var grid planet.Grid
	ok := false
	if on != 0 {
		if grid, ok = g.gridUnder(on); !ok {
			return false
		}
	} else {
		grid, ok = g.gridAt(at, gridNear)
		if !ok && gh.home != nil && gh.home.Reaches(at) {
			grid, ok = *gh.home, true
		}
	}
	fresh := !ok
	face := g.facing(def, at.Norm())
	if fresh {
		if planet.ArcDist(at.Norm(), planet.BaseUp) < base.Extent+60 {
			square := planet.Grid{X: vec3{1, 0, 0}, Up: vec3{0, 1, 0}}
			face = planet.TangentAt(at.Norm(), square.Front(face))
		}
		grid = planet.Grid{At: at, X: face, Up: at.Norm()}
	}
	chosen := gh.level != nil && !fresh && !gh.Free // its height is
	onGround := on == 0 && !gh.Free && !chosen
	up := grid.Up
	front := grid.Front(face)
	if onGround && !def.Build { // upright on the ground as it is
		up = at.Norm()
		front = planet.TangentAt(up, front)
	}
	// Aimed at a ramp's slope or back: it goes on from there, turned like it.
	var ramp vec3
	joins, high := false, false
	if on != 0 && !chosen && !gh.Free && (def.Build || def.Ramp != nil) {
		under, _ := g.Content.Entities.Get(g.kind(on))
		_, rax, rup, _ := g.Frame(g.Ents.Transform.Get(on))
		if to, ok := riseOn(grid, under, rax, rup); ok {
			l := grid.DirLocal(n)
			if back := l.Dot(to) > 0.9; back || l.Dot(to) < -0.1 && l.Y > 0.1 {
				blo, bhi, _ := g.boxOf(grid, on)
				mid, half := blo.Add(bhi).Scale(0.5), bhi.Sub(blo).Scale(0.5)
				ramp, joins = to, true
				high = back || grid.Local(at).Sub(mid).Dot(to) > -0.3*math.Abs(half.Dot(to))
				if own, ok := rise(def); ok && own.Z == 0 {
					front = grid.Dir(to.Scale(own.X))
				}
			}
		}
	}
	gh.frame = [3]vec3{front, up, front.Cross(up)}
	ax, top := gh.inFrame(gh.rel[0]), gh.inFrame(gh.rel[1])
	if fresh { // along the ghost's sides, as it was turned
		if own, ok := gridOn(def, at, ax, top); ok {
			grid = own
		}
	}
	lo, hi := boxIn(grid, def, ax, top)
	sides := (def.Build || def.Ramp != nil) && gh.straight()

	// Its origin in the grid's frame: its middle where the player aims...
	aim := grid.Local(at)
	pos := aim.Sub(lo.Add(hi).Scale(0.5))
	normal := grid.DirLocal(n)
	across := 1 // the axis it rests along
	for i := range 3 {
		if math.Abs(along(normal, i)) > math.Abs(along(normal, across)) {
			across = i
		}
	}
	// High on a block's side, it goes on top: the player doesn't see the top
	// of what is taller than them.
	stacked := false
	var blo, bhi vec3
	level := 0.0
	if chosen {
		level = g.levelOn(gh, grid, at)
		if on == 0 { // in the air, over a block or not
			on = g.blockAt(grid, aim, level)
		}
	}
	if on != 0 {
		blo, bhi, _ = g.boxOf(grid, on)
		tall := bhi.Y - blo.Y
		over := across != 1 && tall >= 0.6 && aim.Y > blo.Y+0.6*tall
		if chosen { // over it if it is as high, beside it otherwise
			over = across != 1 && level > bhi.Y-0.02
		}
		if over {
			stacked, across = true, 1
			n, normal, aim.Y = grid.Up, vec3{0, 1, 0}, bhi.Y
		}
	}
	onTop := on != 0 && across == 1 && normal.Y > 0 && !gh.Free && !joins
	// ...then its sides on the lines.
	if !fresh {
		for _, i := range []int{0, 2} {
			if gh.Free || i != across && !(onTop && sides) {
				setAlong(&pos, i, along(pos, i)+snapSide(along(pos, i)+along(lo, i), along(pos, i)+along(hi, i), step, sides))
			}
		}
	}
	lift := planet.Snap(gh.lift, planet.GridRise)
	ground, column := false, false // column: on another as wide, edge to edge
	gh.Why = ""
	fits := true
	switch {
	case gh.Free:
		if !fresh {
			pos.Y += snapRise(pos.Y+lo.Y, pos.Y+hi.Y, g.gridEdges(grid, pos.Add(lo), pos.Add(hi)))
		}
	case joins:
		own, _ := riseOn(grid, def, ax, top)
		pos, column = join(blo, bhi, ramp, high, lo, hi, own), true
		low, up, apart := g.groundUnder(grid, pos.Add(lo), pos.Add(hi))
		ground = def.Build && !apart && graded(pos.Y+lo.Y, low, up, true)
	case on != 0:
		switch {
		case onTop:
			column = g.stackOver(grid, &pos, lo, hi, blo, bhi, stacked, sides)
		case across != 1 && hi.Y-lo.Y > 0.5: // beside it, level with it
			pos.Y = blo.Y - lo.Y
		case across != 1: // a shelf on its side
			pos.Y += snapRise(pos.Y+lo.Y, pos.Y+hi.Y, []float64{blo.Y, bhi.Y})
		}
		if k := 2 - across; across != 1 && sides { // flush with what is as long
			if long := along(hi, k) - along(lo, k); math.Abs(long-along(bhi, k)+along(blo, k)) < 0.02 {
				if flush := along(blo, k) - along(lo, k); math.Abs(flush-along(pos, k)) < 1.01 {
					setAlong(&pos, k, flush)
				}
			}
		}
		// Against the cell's face, whatever its trims and hollows.
		face := along(aim, across)
		for _, e := range []float64{along(blo, across), along(bhi, across)} {
			if math.Abs(e-face) < 0.15 {
				face = e
			}
		}
		setAlong(&aim, across, face)
		low := math.Inf(1)
		a, b := nominal(def)
		for _, c := range boxCorners(a, b, ax, top) {
			low = math.Min(low, c.Dot(n))
		}
		gap := pos.Sub(aim).Dot(normal) + low
		setAlong(&pos, across, along(pos, across)-gap/along(normal, across))
		if chosen {
			pos.Y = level - lo.Y
		}
		// Beside a block on the ground: on the ground too.
		if o := g.Ents.Grid.Get(on); def.Build && across != 1 && o != nil && o.Ground && o.Same(grid) && math.Abs(pos.Y+lo.Y-o.Lo.Y) < 1e-6 {
			low, high, apart := g.groundUnder(grid, pos.Add(lo), pos.Add(hi))
			ground = !apart && o.Lo.Y-low <= planet.MaxGrade && high-o.Lo.Y <= planet.MaxGrade
		}
	case chosen:
		pos.Y = level - lo.Y
		if low, high, apart := g.groundUnder(grid, pos.Add(lo), pos.Add(hi)); def.Build {
			ground = !apart && graded(level, low, high, true)
		}
	case def.Build:
		level, ground, fits = g.gridLevel(grid, pos.Add(lo), pos.Add(hi), fresh, lift)
		if !fits {
			gh.Why = "sol trop pentu"
		}
		pos.Y = level - lo.Y
	default:
		at := g.World.Relief.Surface(grid.World(pos))
		pos = grid.Local(g.restOnGround(def, at, up, ax, top, math.Max(0, gh.lift)))
	}
	if fresh { // its origin is a block's lowest corner, another thing's foot
		corner := pos.Add(lo)
		if !def.Build {
			mid := pos.Add(lo.Add(hi).Scale(0.5))
			corner.X, corner.Z = mid.X, mid.Z
		}
		grid.At = grid.World(corner)
		pos = pos.Sub(corner)
	}

	set := func(pos vec3) bool {
		origin := grid.World(pos)
		gh.Pose = Transform{Dir: origin.Norm()}
		gh.Pose.Alt = origin.Length() - g.World.Relief.Surface(gh.Pose.Dir).Length()
		gh.Pose.SetAxes(ax, top)
		return g.ghostFits(def, origin, ax, top, ground)
	}
	gh.OK = set(pos)
	// Aimed too near another block, or a wall's end in another's thickness:
	// slid out of it.
	if !gh.OK && sides && !fresh && !stacked && !column {
		for _, d := range g.slides(grid, pos.Add(lo), pos.Add(hi), across) {
			if set(pos.Add(d)) {
				pos, gh.OK = pos.Add(d), true
				break
			}
		}
		if !gh.OK {
			set(pos)
		}
	}
	// No room there: on top of what takes it, as high as it goes.
	if !gh.OK && !fresh && !gh.Free && !chosen {
		p := pos
		for range 12 {
			blo, bhi, ok := g.topOver(grid, p.Add(lo), p.Add(hi), def.Mass > 0)
			if !ok {
				break
			}
			p.Y = bhi.Y - lo.Y
			g.stackOver(grid, &p, lo, hi, blo, bhi, true, sides)
			was := ground
			ground = false
			if set(p) {
				pos, gh.OK, fits, gh.Why = p, true, true, ""
				break
			}
			ground = was
		}
		if !gh.OK {
			set(pos)
		}
	}
	gh.OK = gh.OK && fits
	gh.fresh = fresh
	gh.grid = &OnGrid{Grid: grid, Lo: pos.Add(lo), Hi: pos.Add(hi), Ground: ground}
	gh.Sheet = g.sheet(gh.grid, step)
	return true
}

// alignGhost straightens the ghost on the grid: its sides along the lines,
// by the smallest turn. Away from any grid, it is kept on the one it was
// taken from, or else the nearest; without one in reach, it is turned like
// the one it will found.
func (g *Game) alignGhost() {
	gh := g.Ghost
	def, ok := g.Content.Entities.Get(gh.Kind)
	if !ok {
		return
	}
	if !g.Settings.Grid {
		g.ToggleGrid()
	}
	eye, fwd := g.Player.Eye(), g.Player.Forward()
	at := eye.Add(fwd.Scale(gh.dist))
	if !gh.Free {
		at, _, _ = g.aimSurface(def, eye, fwd)
	}
	if grid, ok := g.gridAt(at, gridNear); ok {
		gh.home = &grid
	} else if gh.from != nil && gh.from.Reaches(at) {
		gh.home = gh.from
	} else if grid, ok := g.gridAt(at, planet.GridSpan); ok {
		gh.home = &grid
	}
	g.poseGhost() // in the frame of that grid
	g.straighten(gh, def)
	gh.askew, gh.lift, gh.level = false, 0, nil
	g.poseGhost()
	g.sfx.Beep(0.02, 0.1, 0.1, 523, 659)
}

// OnGrid tells whether the ghost is on a grid, where its height is chosen.
func (gh *Ghost) OnGrid() bool { return gh.grid != nil }

// Height is how high over the ground the ghost's bottom is, at its middle;
// chosen if the player set it.
func (g *Game) Height() (h float64, chosen bool) {
	gh := g.Ghost
	if gh == nil || gh.grid == nil {
		return 0, false
	}
	mid := gh.grid.Lo.Add(gh.grid.Hi).Scale(0.5)
	mid.Y = gh.grid.Lo.Y
	return g.World.Relief.Altitude(gh.grid.World(mid)), !gh.Free && (gh.level != nil && !gh.fresh || gh.lift != 0 && gh.fresh)
}

// Aligned tells whether the ghost is straight on a grid.
func (gh *Ghost) Aligned() bool {
	if gh.grid == nil || gh.fresh && gh.from != nil && gh.home == nil {
		return false
	}
	return gh.straight()
}

// sheet is the grid's lines around the box, and how far over what is
// under them its corners hang.
func (g *Game) sheet(o *OnGrid, step float64) *Sheet {
	mid := o.Lo.Add(o.Hi).Scale(0.5)
	at := vec3{planet.Snap(mid.X, step), o.Lo.Y, planet.Snap(mid.Z, step)}
	axes := o.Axes()
	s := &Sheet{At: o.World(at), X: axes[0], Up: axes[1], Z: axes[2], Step: step,
		Reach: math.Hypot(o.Hi.X-o.Lo.X, o.Hi.Z-o.Lo.Z)/2 + 2 + 2*step}
	for i, x := range []float64{o.Lo.X + 0.03, o.Hi.X - 0.03} {
		for j, z := range []float64{o.Lo.Z + 0.03, o.Hi.Z - 0.03} {
			foot := o.World(vec3{x, o.Lo.Y, z})
			r := foot.Length()
			under := math.Max(g.World.Relief.SurfaceRadius(foot.Norm()), g.topUnder(foot, r+0.05, true, 0))
			if r-under > 0.1 {
				s.Feet[i*2+j] = [2]vec3{foot.Norm().Scale(under), foot}
			}
		}
	}
	return s
}

// stackOver lines up a thing filling lo..hi around pos over a block that
// fills blo..bhi: exactly on it where it is as wide, a clean column (it
// returns true); otherwise on lines from that block's edges, without
// hanging over them where it is shorter, unless what is beside bears it;
// and where it is longer (a roof on a wall), if the block was aimed at
// from its side, from one of its edges, over the most of what else would
// bear it. Without sides, what is shorter stays where it is aimed.
func (g *Game) stackOver(grid planet.Grid, pos *vec3, lo, hi, blo, bhi vec3, side, sides bool) bool {
	step := g.Settings.GridStep
	first := blo.Sub(lo)
	width := hi.Sub(lo).Sub(bhi.Sub(blo))
	if math.Abs(width.X) < 0.02 && math.Abs(width.Z) < 0.02 {
		pos.X, pos.Z = first.X, first.Z
		return true
	}
	free := *pos // on the lines, wherever that is
	for _, i := range []int{0, 2} {
		long, under, at := along(hi, i)-along(lo, i), along(bhi, i)-along(blo, i), along(*pos, i)
		first, last := along(blo, i)-along(lo, i), along(bhi, i)-along(hi, i) // edge to edge
		if side && under < 1 && long > under+0.02 {                           // across a wall
			me := along(grid.Local(g.Player.Pos), i)
			best, score := at, -1.0
			for _, c := range []float64{first, last} {
				p := *pos
				setAlong(&p, i, c)
				s := g.borne(grid, p.Add(lo), p.Add(hi), math.Inf(-1), bhi.Y)
				mid := c + (along(lo, i)+along(hi, i))/2
				if (mid > along(blo, i)) == (me > along(blo, i)) { // toward the player, all else equal
					s += 1e-3
				}
				if s > score {
					best, score = c, s
				}
			}
			setAlong(pos, i, best)
			setAlong(&free, i, best)
			continue
		}
		if sides || long > under+0.02 { // the nearest line, or edge to edge, or in the middle
			rel := at - first
			best := at + planet.SnapSpan(rel, rel+long, step)
			also := []float64{last}
			if long > under+0.02 {
				also = append(also, first, (first+last)/2)
			}
			for _, c := range also {
				if math.Abs(c-at) < math.Abs(best-at) {
					best = c
				}
			}
			at = best
		}
		setAlong(&free, i, at)
		if long <= under+0.02 {
			at = math.Max(first, math.Min(last, at))
		}
		setAlong(pos, i, at)
	}
	if !side && free != *pos {
		l, h := free.Add(lo), free.Add(hi)
		if g.borne(grid, l, h, bhi.Y-0.02, bhi.Y) > 0.99*(h.X-l.X)*(h.Z-l.Z) {
			*pos = free
		}
	}
	return false
}

// borne is how much of the rectangle lo..hi of a grid is over blocks with
// their tops from low to high.
func (g *Game) borne(grid planet.Grid, lo, hi vec3, low, high float64) float64 {
	sum := 0.0
	g.onGridNear(grid, lo, hi, func(_ ecs.ID, blo, bhi vec3) {
		x := math.Min(hi.X, bhi.X) - math.Max(lo.X, blo.X)
		z := math.Min(hi.Z, bhi.Z) - math.Max(lo.Z, blo.Z)
		if bhi.Y >= low && bhi.Y <= high+0.01 && x > 0 && z > 0 {
			sum += x * z
		}
	})
	return sum
}

// blockAt is the block straight under a point of a grid, its top at y; 0
// for none.
func (g *Game) blockAt(grid planet.Grid, at vec3, y float64) ecs.ID {
	var on ecs.ID
	g.onGridNear(grid, at, at, func(id ecs.ID, blo, bhi vec3) {
		if math.Abs(bhi.Y-y) < 0.02 && outside(at.X, blo.X, bhi.X) == 0 && outside(at.Z, blo.Z, bhi.Z) == 0 {
			on = id
		}
	})
	return on
}

// topOver is the box of the highest of the blocks (with things, the crates
// too) that meet the box lo..hi of a grid.
func (g *Game) topOver(grid planet.Grid, lo, hi vec3, things bool) (tlo, thi vec3, ok bool) {
	const room = 0.03
	for _, id := range g.EntitiesNear(buildReach + gridNear + 15) {
		def, known := g.Content.Entities.Get(g.kind(id))
		if !known || g.Held(id) || !(def.Build || things && def.Mass > 0) {
			continue
		}
		blo, bhi, _ := g.boxOf(grid, id)
		if blo.X < hi.X-room && lo.X < bhi.X-room && blo.Y < hi.Y-room && lo.Y < bhi.Y-room && blo.Z < hi.Z-room && lo.Z < bhi.Z-room && (!ok || bhi.Y > thi.Y) {
			tlo, thi, ok = blo, bhi, true
		}
	}
	return tlo, thi, ok
}

// onGridNear calls fn for the blocks within gridNear of the box lo..hi of
// a grid, with their own boxes there.
func (g *Game) onGridNear(grid planet.Grid, lo, hi vec3, fn func(id ecs.ID, blo, bhi vec3)) {
	for _, id := range g.EntitiesNear(buildReach + gridNear + 15) {
		def, ok := g.Content.Entities.Get(g.kind(id))
		if !ok || !def.Build || g.Held(id) {
			continue
		}
		blo, bhi, _ := g.boxOf(grid, id)
		if math.Max(math.Max(blo.X-hi.X, lo.X-bhi.X), math.Max(blo.Z-hi.Z, lo.Z-bhi.Z)) < gridNear {
			fn(id, blo, bhi)
		}
	}
}

// gridEdges are the bottoms and tops of the blocks near.
func (g *Game) gridEdges(grid planet.Grid, lo, hi vec3) []float64 {
	var out []float64
	g.onGridNear(grid, lo, hi, func(_ ecs.ID, blo, bhi vec3) { out = append(out, blo.Y, bhi.Y) })
	return out
}

// slides are the ways out of the blocks the box lo..hi overlaps, edge to
// edge with them, the shortest first.
func (g *Game) slides(grid planet.Grid, lo, hi vec3, across int) []vec3 {
	const far = 2.0
	var out []vec3
	g.onGridNear(grid, lo, hi, func(_ ecs.ID, blo, bhi vec3) {
		for _, i := range []int{0, 2} {
			if i == across {
				continue
			}
			for _, d := range []float64{along(bhi, i) - along(lo, i), along(blo, i) - along(hi, i)} {
				if math.Abs(d) <= far {
					var v vec3
					setAlong(&v, i, d)
					out = append(out, v)
				}
			}
		}
	})
	slices.SortStableFunc(out, func(a, b vec3) int {
		switch la, lb := a.Length(), b.Length(); {
		case la < lb-1e-9:
			return -1
		case la > lb+1e-9:
			return 1
		}
		return 0
	})
	return out
}

// groundUnder is the lowest and the highest of the natural ground under
// the rectangle lo..hi of a grid, in its frame; apart if the base's pad or
// a bootcamp's shapes it: it isn't graded.
func (g *Game) groundUnder(grid planet.Grid, lo, hi vec3) (low, high float64, apart bool) {
	r := g.World.Relief
	low, high = math.Inf(1), math.Inf(-1)
	for _, kx := range []float64{0, 0.5, 1} {
		for _, kz := range []float64{0, 0.5, 1} {
			at := vec3{lo.X + (hi.X-lo.X)*kx, 0, lo.Z + (hi.Z-lo.Z)*kz}
			d := grid.World(at).Norm()
			h := grid.Local(d.Scale(planet.Radius + r.Natural(d))).Y
			low, high = math.Min(low, h), math.Max(high, h)
			apart = apart || r.ByPad(d)
		}
	}
	return low, high, apart
}

// graded tells whether the ground, between low and high, is made level for
// a block at level; chosen by the player, over the ground is in the air,
// not on a mound.
func graded(level, low, high float64, chosen bool) bool {
	return level-low <= planet.MaxGrade && high-level <= planet.MaxGrade &&
		(!chosen || level-(low+high)/2 < planet.GridRise+0.01)
}

// gridLevel is the height a block filling lo..hi (seen from above) of a
// grid stands at on the ground, raised by lift: level with the blocks
// standing on the ground near, or the level of the grid nearest to the
// ground. ground tells whether the ground is graded to it, ok whether the
// block may stand there.
func (g *Game) gridLevel(grid planet.Grid, lo, hi vec3, fresh bool, lift float64) (level float64, ground, ok bool) {
	low, high, apart := g.groundUnder(grid, lo, hi)
	rest := (low + high) / 2
	if apart { // not graded: on top of the ground as it is
		rest = high
	}
	level = rest
	if !fresh {
		level = planet.Snap(rest, planet.GridRise)
		if apart && level < rest-0.1 {
			level += planet.GridRise
		}
		near := 1.0
		g.onGridNear(grid, lo, hi, func(id ecs.ID, blo, _ vec3) {
			o := g.Ents.Grid.Get(id)
			if o == nil || !o.Ground || !o.Same(grid) {
				return
			}
			if d := math.Abs(o.Lo.Y - rest); d < near {
				level, near = o.Lo.Y, d
			}
		})
	}
	level += lift
	ground = !apart && graded(level, low, high, lift != 0)
	return level, ground, ground || high-level < 0.15
}

// syncGround grades the ground under the blocks standing on it, and clears
// it of small rocks under the others, when some were set down or taken off.
func (g *Game) syncGround() {
	if !g.regrade && g.graded == g.Ents.Len() {
		return
	}
	g.regrade, g.graded = false, g.Ents.Len()
	relief := g.World.Relief
	var plots []planet.Plot
	for _, o := range g.Ents.Grid.All() {
		if !o.Ground {
			continue
		}
		p := o.Plot(o.Lo, o.Hi, o.Lo.Y)
		// Left from another Mars, it would raise a mountain.
		if d := p.At.Norm(); math.Abs(p.At.Length()-planet.Radius-relief.Natural(d)) < 3*planet.MaxGrade {
			plots = append(plots, p)
		}
	}
	if !slices.Equal(plots, relief.Flat().Plots.List) {
		g.reshape(func() { relief.SetPlots(plots) })
	}
	var bare []planet.Plot
	for id, t := range g.Ents.Transform.All() {
		def, ok := g.Content.Entities.Get(g.kind(id))
		if o := g.Ents.Grid.Get(id); !ok || !def.Build || o != nil && o.Ground {
			continue
		}
		pos, ax, up, _ := g.Frame(t)
		grid, ok := gridOn(def, pos, ax, up)
		if !ok { // tipped over: the box around it
			n := pos.Norm()
			t1, _ := mathx.TangentBasis(n)
			grid = planet.Grid{At: pos, X: t1, Up: n}
		}
		lo, hi := boxIn(grid, def, ax, up)
		at := grid.Local(pos)
		lo, hi = at.Add(lo), at.Add(hi)
		if p := grid.Plot(lo, hi, lo.Y); relief.Altitude(p.At) < planet.SmallRock {
			bare = append(bare, p)
		}
	}
	if !slices.Equal(bare, relief.Flat().Bare.List) {
		relief.SetBare(bare)
		g.relay = true
	}
}

// reshape changes the ground: what is built, and what stands over the
// ground, stays where it was; what lies on it goes with it.
func (g *Game) reshape(change func()) {
	g.relay = true // the collisions, laid again: rocks gone, or back
	if g.loading { // saved over the ground as it will be
		change()
		return
	}
	was := map[ecs.ID]float64{}
	for id, t := range g.Ents.Transform.All() {
		def, ok := g.Content.Entities.Get(g.kind(id))
		if _, vehicle := g.vehicleOf(id); !ok || vehicle || isCamp(def) {
			continue
		}
		if def.Build || t.Alt > 0.05 && !g.Ents.Air.Has(id) {
			was[id] = g.World.Relief.Height(t.Dir) + t.Alt
		}
	}
	change()
	for id, h := range was {
		t := g.Ents.Transform.Get(id)
		t.Alt = h - g.World.Relief.Height(t.Dir)
	}
}

// blocksMeet tells whether a block turned to ax and up from origin meets
// block id, by more than room: their boxes, or the parts of what has a
// Shape.
func (g *Game) blocksMeet(def *content.EntityDef, origin, ax, up vec3, id ecs.ID, room float64) bool {
	other, ok := g.Content.Entities.Get(g.kind(id))
	if !ok || !g.boxesMeet(def, origin, ax, up, id, room) {
		return false
	}
	if len(def.Shape) == 0 && len(other.Shape) == 0 {
		return true
	}
	pos, bx, bu, _ := g.Frame(g.Ents.Transform.Get(id))
	as, bs := g.pieces(def, vec3{}, ax, up), g.pieces(other, pos.Sub(origin), bx, bu)
	for _, a := range as {
		for _, b := range bs {
			if a.meets(b, room) {
				return true
			}
		}
	}
	return false
}

// pieces are what a block takes up, turned to ax and up from at: the parts
// of its Shape, or the box it is meant to fill.
func (g *Game) pieces(def *content.EntityDef, at, ax, up vec3) []solid {
	az := ax.Cross(up)
	dir := func(v vec3) vec3 { return ax.Scale(v.X).Add(up.Scale(v.Y)).Add(az.Scale(v.Z)) }
	list := g.solidsOf(def)
	if len(def.Shape) == 0 {
		lo, hi := nominal(def)
		list = []solid{boxSolid(lo.Add(hi).Scale(0.5), [3]vec3{{1, 0, 0}, {0, 1, 0}, {0, 0, 1}}, hi.Sub(lo).Scale(0.5))}
	}
	out := make([]solid, len(list))
	for i, s := range list {
		for _, p := range s.planes {
			n := dir(p.n)
			out[i].planes = append(out[i].planes, plane{n, p.d + n.Dot(at)})
		}
		for _, p := range s.pts {
			out[i].pts = append(out[i].pts, at.Add(dir(p)))
		}
	}
	return out
}

// meets tells whether two solids overlap by more than room every way.
func (s solid) meets(o solid, room float64) bool {
	apart := func(v vec3) bool {
		l := v.Length()
		if l < 1e-6 {
			return false
		}
		v = v.Scale(1 / l)
		a0, a1 := math.Inf(1), math.Inf(-1)
		for _, p := range s.pts {
			a0, a1 = math.Min(a0, p.Dot(v)), math.Max(a1, p.Dot(v))
		}
		b0, b1 := math.Inf(1), math.Inf(-1)
		for _, p := range o.pts {
			b0, b1 = math.Min(b0, p.Dot(v)), math.Max(b1, p.Dot(v))
		}
		return math.Min(a1, b1)-math.Max(a0, b0) < room
	}
	for _, a := range s.planes {
		if apart(a.n) {
			return false
		}
	}
	for _, b := range o.planes {
		if apart(b.n) {
			return false
		}
	}
	// The edges of a convex shape are along the crossings of its faces.
	for i, a := range s.planes {
		for _, a2 := range s.planes[:i] {
			for j, b := range o.planes {
				for _, b2 := range o.planes[:j] {
					if apart(a.n.Cross(a2.n).Cross(b.n.Cross(b2.n))) {
						return false
					}
				}
			}
		}
	}
	return true
}

// boxesMeet tells whether a box turned to ax and up from origin meets the
// nominal box of entity id, by more than room.
func (g *Game) boxesMeet(def *content.EntityDef, origin, ax, up vec3, id ecs.ID, room float64) bool {
	other, ok := g.Content.Entities.Get(g.kind(id))
	if !ok {
		return false
	}
	pos, bx, bu, bz := g.Frame(g.Ents.Transform.Get(id))
	a, b := [3]vec3{ax, up, ax.Cross(up)}, [3]vec3{bx, bu, bz}
	mid := func(def *content.EntityDef, at vec3, axes [3]vec3) (c, half vec3) {
		lo, hi := nominal(def)
		m := lo.Add(hi).Scale(0.5)
		return at.Add(axes[0].Scale(m.X)).Add(axes[1].Scale(m.Y)).Add(axes[2].Scale(m.Z)), hi.Sub(lo).Scale(0.5)
	}
	ca, ha := mid(def, origin, a)
	cb, hb := mid(other, pos, b)
	off := cb.Sub(ca)
	span := func(axes [3]vec3, h, along vec3) float64 {
		return math.Abs(axes[0].Dot(along))*h.X + math.Abs(axes[1].Dot(along))*h.Y + math.Abs(axes[2].Dot(along))*h.Z
	}
	apart := func(v vec3) bool {
		l := v.Length()
		if l < 1e-6 {
			return false
		}
		v = v.Scale(1 / l)
		return math.Abs(off.Dot(v)) > span(a, ha, v)+span(b, hb, v)-room
	}
	for i := range 3 {
		if apart(a[i]) || apart(b[i]) {
			return false
		}
		for j := range 3 {
			if apart(a[i].Cross(b[j])) {
				return false
			}
		}
	}
	return true
}

// ShotGrid builds a small house on a grid dist m ahead, for screenshots:
// two rows of slabs, walls along the far edge, a crate at their end; or
// (1) ramps on two levels, (2) a two-storey house, (3) every block of the
// catalog in rows.
func (g *Game) ShotGrid(dist, step float64, what int) {
	g.Settings.Grid, g.Settings.GridStep = true, step
	n := planet.Toward2(g.Player.LocalUp(), g.Player.Heading, dist)
	at := g.World.Relief.Surface(n)
	grid := planet.Grid{At: at, X: planet.TangentAt(n, g.Player.Heading).Scale(-1), Up: n}
	quarters, paint := 0, ""
	stand := func(kind string, x, y, z float64, ground bool) {
		def, ok := g.Content.Entities.Get(kind)
		if !ok {
			return
		}
		origin := grid.World(vec3{x, y, z})
		t := Transform{Dir: origin.Norm()}
		t.Alt = origin.Length() - g.World.Relief.Surface(t.Dir).Length()
		ax := grid.X
		for range quarters {
			ax = ax.Cross(grid.Up)
		}
		t.SetAxes(ax, grid.Up)
		lo, hi := boxIn(grid, def, ax, grid.Up)
		o := vec3{x, y, z}
		g.add(Placed{Kind: def.ID, Dir: t.Dir, Yaw: t.Yaw, Alt: t.Alt, Pitch: t.Pitch, Roll: t.Roll, Paint: paint,
			Grid: &OnGrid{Grid: grid, Lo: o.Add(lo), Hi: o.Add(hi), Ground: ground}})
	}
	if what == 2 {
		turned := func(q int, kind string, x, y, z float64, ground bool) {
			quarters = q
			stand(kind, x, y, z, ground)
			quarters = 0
		}
		for _, x := range []float64{-2, 2} {
			for _, z := range []float64{-4, 0, 4} {
				stand("core:slab", x, 0, z, true)
				if x > 0 || z < 4 {
					stand("core:slab", x, 2.7, z, false)
				}
			}
		}
		turned(0, "core:slab-half", -3, 2.7, 4, false)
		stand("core:slab-round", 6, 0, 4, true)
		turned(1, "core:slab-round", 6, 0, -4, true)
		stand("core:slab", 6, 0, 0, true)
		turned(3, "core:stairs", -2, 0.2, 5, false)
		for i, z := range []float64{-4, 0, 4} { // the front and the back
			for j, y := range []float64{0.2, 2.9} {
				kinds := [][]string{{"core:wall-window", "core:door", "core:wall-glass"}, {"core:wall-glass", "core:wall-window", "core:wall-window"}}
				paint = []string{"core:coral", "core:white"}[j]
				stand(kinds[j][i], 3.9, y, z, false)
				stand("core:wall", -3.9, y, z, false)
			}
			paint = "core:plum"
			turned(0, "core:roof", 2, 5.4, z, false)
			turned(2, "core:roof", -2, 5.4, z, false)
		}
		for j, y := range []float64{0.2, 2.9} { // the ends
			paint = []string{"core:coral", "core:white"}[j]
			for _, x := range []float64{-2, 2} {
				turned(1, []string{"core:wall", "core:wall-window"}[j], x, y, -5.9, false)
				turned(1, "core:wall", x, y, 5.9, false)
			}
		}
		paint = "core:white"
		turned(3, "core:gable", -2, 5.4, -5.9, false)
		turned(1, "core:gable", 2, 5.4, -5.9, false)
		turned(3, "core:gable", -2, 5.4, 5.9, false)
		turned(1, "core:gable", 2, 5.4, 5.9, false)
		paint = ""
		stand("core:rail", 7.9, 0.2, 0, false)
		stand("core:gate-door", 4, 0, 14, true)
		stand("core:lamp-street", 9, 0, 9, true)
		stand("core:lamp-street", 9, 0, -9, true)
		for _, z := range []float64{-4, 0, 4} {
			stand("core:lamp-ceiling", 0, 2.6, z, false)
			if z < 4 {
				stand("core:lamp-ceiling", 0, 5.3, z, false)
			}
		}
		turned(0, "core:lamp-wall", 4.075, 2, 2.5, false)
		stand("core:lamp-post", 8, 0.2, 4.5, false)
		turned(2, "core:lamp-flood", 7, 0, 18, true)
		stand("core:arch", 4, 0, -14, true)
		g.regrade = true
		g.syncGround()
		return
	}
	if what == 3 {
		paint = g.State.Paint
		row := 0.0
		for _, s := range g.Catalog() {
			z := -18.0
			for _, it := range s.Items {
				def, ok := g.Content.Entities.Get(it.Places)
				if !ok || s.ID == carriedShelf {
					continue
				}
				lo, hi := nominal(def)
				z += (hi.Z - lo.Z) / 2
				stand(def.ID, -row, 0, z, true)
				z += (hi.Z-lo.Z)/2 + 1
			}
			row += 9
		}
		g.regrade = true
		g.syncGround()
		return
	}
	if what == 1 {
		grid.X = grid.X.Scale(-1)
		stand("core:ramp", 0, 0, 0, true)
		stand("core:ramp-base", 6, 0, 0, true)
		stand("core:ramp-steep", 6, 2, 0, false)
		for i := range 4 {
			stand("core:crate-1", -2, float64(i%2), 4+float64(i/2), i%2 == 0)
		}
		g.regrade = true
		g.syncGround()
		return
	}
	for i := range 3 {
		z := float64(i)*4 - 4
		stand("core:slab", -2, 0, z, true)
		stand("core:slab", 2, 0, z, true)
		stand("core:wall", -3.9, 0.2, z, false)
	}
	stand("core:crate-2", -1, 0, 7, true)
	g.regrade = true
	g.syncGround()
}
