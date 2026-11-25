package game

import (
	"math"
	"naturalia/internal/base"
	"naturalia/internal/content"
	"naturalia/internal/ecs"
	"naturalia/internal/mathx"
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
		if grid, ok = g.gridOf(on); !ok { // a crate's, only to stack on it
			t := g.Ents.Transform.Get(on)
			under, _ := g.Content.Entities.Get(g.kind(on))
			pos, ax, up, _ := g.Frame(t)
			if grid, ok = gridOn(under, pos, ax, up); !ok {
				return false
			}
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
	// High on a block's side, it goes on top: the player doesn't see the top
	// of what is taller than them.
	stacked := false
	var blo, bhi vec3
	if on != 0 {
		blo, bhi, _ = g.boxOf(grid, on)
		if tall := bhi.Y - blo.Y; across != 1 && tall >= 0.6 && aim.Y > blo.Y+0.6*tall {
			stacked, across = true, 1
			n, normal, aim.Y = grid.Up, vec3{0, 1, 0}, bhi.Y
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
		switch {
		case across == 1 && normal.Y > 0:
			g.stackOver(grid, &pos, lo, hi, blo, bhi, stacked)
		case across != 1 && hi.Y-lo.Y >= (bhi.Y-blo.Y)/2: // beside it, level with it
			pos.Y = blo.Y - lo.Y
		case across != 1: // a shelf on its side
			pos.Y += snapRise(pos.Y+lo.Y, pos.Y+hi.Y, []float64{blo.Y, bhi.Y})
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
		pos.Y += lift
		// Beside a block on the ground: on the ground too.
		if o := g.Ents.Grid.Get(on); def.Build && across != 1 && o != nil && o.Ground && o.Same(grid) && math.Abs(pos.Y+lo.Y-o.Lo.Y) < 1e-6 {
			low, high, apart := g.groundUnder(grid, pos.Add(lo), pos.Add(hi))
			ground = !apart && o.Lo.Y-low <= planet.MaxGrade && high-o.Lo.Y <= planet.MaxGrade
		}
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
	if !gh.OK && sides && !fresh && !stacked {
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
	if !gh.OK && !fresh && !gh.Free {
		p := pos
		for range 12 {
			top, ok := g.topOver(grid, p.Add(lo), p.Add(hi), def.Mass > 0)
			if !ok {
				break
			}
			p.Y = top - lo.Y
			graded := ground
			ground = false
			if set(p) {
				pos, gh.OK, fits, gh.Why = p, true, true, ""
				break
			}
			ground = graded
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
	gh.lift = 0
	g.poseGhost()
	g.sfx.Beep(0.02, 0.1, 0.1, 523, 659)
}

// Aligned tells whether the ghost is straight on a grid: alignGhost would
// change nothing.
func (gh *Ghost) Aligned() bool {
	if gh.grid == nil || gh.fresh && gh.from != nil && gh.home == nil {
		return false
	}
	for _, v := range gh.rel {
		if math.Max(math.Abs(v.X), math.Max(math.Abs(v.Y), math.Abs(v.Z))) < 1-1e-6 {
			return false
		}
	}
	return true
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
// fills blo..bhi: edge to edge with it where they are as long; within it
// where it is shorter; and where it is longer (a roof on a wall), if the
// block was aimed at from its side, from one of its edges, over the most
// of what else would bear it.
func (g *Game) stackOver(grid planet.Grid, pos *vec3, lo, hi, blo, bhi vec3, side bool) {
	for _, i := range []int{0, 2} {
		long, under := along(hi, i)-along(lo, i), along(bhi, i)-along(blo, i)
		at := along(*pos, i)
		first, last := along(blo, i)-along(lo, i), along(bhi, i)-along(hi, i) // edge to edge
		switch {
		case math.Abs(long-under) < 0.02:
			if side || under < g.Settings.GridStep || math.Abs(first-at) < 0.3 {
				at = first
			}
		case long < under:
			if side {
				at = math.Max(first, math.Min(last, at))
			}
		case side && under < 1: // across a wall
			me := along(grid.Local(g.Player.Pos), i)
			best := -1.0
			for _, c := range []float64{first, last} {
				p := *pos
				setAlong(&p, i, c)
				score := g.borne(grid, p.Add(lo), p.Add(hi), bhi.Y)
				mid := c + (along(lo, i)+along(hi, i))/2
				if (mid > along(blo, i)) == (me > along(blo, i)) { // toward the player, all else equal
					score += 1e-3
				}
				if score > best {
					at, best = c, score
				}
			}
		}
		setAlong(pos, i, at)
	}
}

// borne is how much of the rectangle lo..hi of a grid is over blocks no
// higher than y.
func (g *Game) borne(grid planet.Grid, lo, hi vec3, y float64) float64 {
	sum := 0.0
	g.onGridNear(grid, lo, hi, func(_ ecs.ID, blo, bhi vec3) {
		x := math.Min(hi.X, bhi.X) - math.Max(lo.X, blo.X)
		z := math.Min(hi.Z, bhi.Z) - math.Max(lo.Z, blo.Z)
		if bhi.Y <= y+0.01 && x > 0 && z > 0 {
			sum += x * z
		}
	})
	return sum
}

// topOver is the highest top of the blocks (with things, the crates too)
// that meet the box lo..hi of a grid.
func (g *Game) topOver(grid planet.Grid, lo, hi vec3, things bool) (top float64, ok bool) {
	const room = 0.03
	top = math.Inf(-1)
	for _, id := range g.EntitiesNear(buildReach + gridNear + 15) {
		def, known := g.Content.Entities.Get(g.kind(id))
		if !known || g.Held(id) || !(def.Build || things && def.Mass > 0) {
			continue
		}
		blo, bhi, _ := g.boxOf(grid, id)
		if blo.X < hi.X-room && lo.X < bhi.X-room && blo.Y < hi.Y-room && lo.Y < bhi.Y-room && blo.Z < hi.Z-room && lo.Z < bhi.Z-room {
			top, ok = math.Max(top, bhi.Y), true
		}
	}
	return top, ok
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
	ground = !apart && level-low <= planet.MaxGrade && high-level <= planet.MaxGrade
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
