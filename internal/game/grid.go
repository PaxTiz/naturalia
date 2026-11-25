package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
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

// Sheet is the part of the grid shown under the ghost: the point At of its
// lines nearest to the ghost, and the two axes they follow.
type Sheet struct {
	At, U, V vec3
	Step     float64
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

// nominal is the box a thing is meant to fill, in its own frame: its
// parts' but for the centimeter their trims stick out.
func nominal(def *content.EntityDef) (lo, hi vec3) {
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

// gridAt is the grid taken up at a point: the nearest thing's.
func (g *Game) gridAt(at vec3) (planet.Grid, bool) {
	best, reach, found := planet.Grid{}, gridNear, false
	for _, id := range g.EntitiesNear(buildReach + gridNear + 15) {
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
	if !gh.Free {
		at, n, on = g.aimSurface(def, eye, fwd)
	}

	// The grid: that of the block aimed at, of what is near, or a new one.
	var grid planet.Grid
	ok := false
	if on != 0 {
		if grid, ok = g.gridOf(on); !ok {
			return false
		}
	} else {
		grid, ok = g.gridAt(at)
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
	onGround := on == 0 && !gh.Free
	up := grid.Up
	front := grid.Front(face)
	if onGround && !def.Build { // upright on the ground as it is
		up = at.Norm()
		front = planet.TangentAt(up, front)
	}
	gh.frame = [3]vec3{front, up, front.Cross(up)}
	ax, top := gh.inFrame(gh.rel[0]), gh.inFrame(gh.rel[1])
	if fresh { // along the ghost's sides, as it was turned
		if own, ok := gridOn(def, at, ax, top); ok {
			grid = own
		}
	}
	lo, hi := boxIn(grid, def, ax, top)
	sides := def.Build && square(grid, ax, top)

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
	// ...then its sides on the lines.
	if !fresh {
		for _, i := range []int{0, 2} {
			if gh.Free || i != across {
				setAlong(&pos, i, along(pos, i)+snapSide(along(pos, i)+along(lo, i), along(pos, i)+along(hi, i), step, sides))
			}
		}
	}
	lift := planet.Snap(gh.lift, planet.GridRise)
	ground := false
	gh.Why = ""
	fits := true
	switch {
	case gh.Free:
		if !fresh {
			pos.Y += snapRise(pos.Y+lo.Y, pos.Y+hi.Y, g.gridEdges(grid, pos.Add(lo), pos.Add(hi)))
		}
	case on != 0:
		blo, bhi, _ := g.boxOf(grid, on)
		if across != 1 {
			pos.Y += snapRise(pos.Y+lo.Y, pos.Y+hi.Y, []float64{blo.Y, bhi.Y})
		}
		// Against the face, but for the block's trims.
		face := along(aim, across)
		for _, e := range []float64{along(blo, across), along(bhi, across)} {
			if math.Abs(e-face) < 0.06 {
				face = e
			}
		}
		setAlong(&aim, across, face)
		low := math.Inf(1)
		a, b := nominal(def)
		for _, c := range boxCorners(a, b, ax, top) {
			low = math.Min(low, c.Dot(n))
		}
		gap := pos.Sub(aim).Dot(normal) + low - lift
		setAlong(&pos, across, along(pos, across)-gap/along(normal, across))
	case def.Build:
		var level float64
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
	if !gh.OK && sides && !fresh {
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
	gh.OK = gh.OK && fits
	gh.fresh = fresh
	gh.grid = &OnGrid{Grid: grid, Lo: pos.Add(lo), Hi: pos.Add(hi), Ground: ground}
	gh.Sheet = g.sheet(gh.grid, across, step, normal)
	return true
}

// sheet is the grid's lines under the box, on the side it rests on.
func (g *Game) sheet(o *OnGrid, across int, step float64, normal vec3) *Sheet {
	mid := o.Lo.Add(o.Hi).Scale(0.5)
	for i := range 3 {
		setAlong(&mid, i, planet.Snap(along(mid, i), step))
	}
	side, off := along(o.Lo, across), 0.03
	if along(normal, across) < 0 {
		side, off = along(o.Hi, across), -0.03
	}
	setAlong(&mid, across, side+off)
	axes := o.Axes()
	return &Sheet{At: o.World(mid), U: axes[(across+1)%3], V: axes[(across+2)%3], Step: step}
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

// gridLevel is the height a block filling lo..hi (seen from above) of a
// grid stands at on the ground, raised by lift: level with the blocks
// standing on the ground near, or the level of the grid nearest to the
// ground. ground tells whether the ground is graded to it, ok whether the
// block may stand there.
func (g *Game) gridLevel(grid planet.Grid, lo, hi vec3, fresh bool, lift float64) (level float64, ground, ok bool) {
	r := g.World.Relief
	var heights []float64 // of the natural ground under it, in the grid's frame
	apart := false
	for _, kx := range []float64{0, 0.5, 1} {
		for _, kz := range []float64{0, 0.5, 1} {
			at := vec3{lo.X + (hi.X-lo.X)*kx, 0, lo.Z + (hi.Z-lo.Z)*kz}
			d := grid.World(at).Norm()
			heights = append(heights, grid.Local(d.Scale(planet.Radius+r.Natural(d))).Y)
			apart = apart || r.ByPad(d)
		}
	}
	low, high := slices.Min(heights), slices.Max(heights)
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
	ground = !apart && level-low <= planet.MaxGrade && high-level <= planet.MaxGrade
	return level, ground, ground || high-level < 0.15
}

// syncGround grades the ground under the blocks standing on it, when some
// were set down or taken off.
func (g *Game) syncGround() {
	if !g.regrade && g.graded == g.Ents.Grid.Len() {
		return
	}
	g.regrade, g.graded = false, g.Ents.Grid.Len()
	var plots []planet.Plot
	for _, o := range g.Ents.Grid.All() {
		if !o.Ground {
			continue
		}
		p := o.Plot(o.Lo, o.Hi, o.Lo.Y)
		// Left from another Mars, it would raise a mountain.
		if d := p.At.Norm(); math.Abs(p.At.Length()-planet.Radius-g.World.Relief.Natural(d)) < 3*planet.MaxGrade {
			plots = append(plots, p)
		}
	}
	if slices.Equal(plots, g.World.Relief.Flat().Plots) {
		return
	}
	g.reshape(func() { g.World.Relief.SetPlots(plots) })
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
// two rows of slabs, walls along the far edge, a crate at their end.
func (g *Game) ShotGrid(dist, step float64) {
	g.Settings.Grid, g.Settings.GridStep = true, step
	n := planet.Toward2(g.Player.LocalUp(), g.Player.Heading, dist)
	at := g.World.Relief.Surface(n)
	grid := planet.Grid{At: at, X: planet.TangentAt(n, g.Player.Heading).Scale(-1), Up: n}
	stand := func(kind string, x, y, z float64, ground bool) {
		def, ok := g.Content.Entities.Get(kind)
		if !ok {
			return
		}
		origin := grid.World(vec3{x, y, z})
		t := Transform{Dir: origin.Norm()}
		t.Alt = origin.Length() - g.World.Relief.Surface(t.Dir).Length()
		t.SetAxes(grid.X, grid.Up)
		lo, hi := boxIn(grid, def, grid.X, grid.Up)
		o := vec3{x, y, z}
		g.add(Placed{Kind: kind, Dir: t.Dir, Yaw: t.Yaw, Alt: t.Alt, Pitch: t.Pitch, Roll: t.Roll,
			Grid: &OnGrid{Grid: grid, Lo: o.Add(lo), Hi: o.Add(hi), Ground: ground}})
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
