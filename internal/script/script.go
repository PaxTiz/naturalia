package script

import (
	"fmt"
	"image/color"
	"log"
	"naturalia/internal/content"
	"naturalia/internal/planet"
	"os"
	"path/filepath"
	"time"

	"github.com/dop251/goja"
)

// Scripts (docs/MODS.md): behaviors and random events in JavaScript, run
// by goja, a JS engine in pure Go. Each mod gets its own runtime, with no
// access to files or the network: only the game object below. A call that
// runs too long is cut short, so a broken mod can't freeze the game.
//
//	game.on("sell", e => game.say("Vendu : " + e.amount));
//	game.randomEvent({id: "storm", every: 300, when: () => game.isDay(), run: () => ...});
//
// See examples/mods/serre/main.js for the whole API.

const scriptBudget = 50 * time.Millisecond // per call

type script struct {
	mod    string
	rt     *goja.Runtime
	g      content.Game // during a call
	errors int
}

// Run runs a mod's scripts while its pack registers (a ScriptRunner).
func Run(c *content.Content, mod, dir string, names []string) {
	for _, name := range names {
		src, err := os.ReadFile(filepath.Join(dir, filepath.Clean("/"+name)))
		if err != nil {
			log.Printf("mod %s: %v", mod, err)
			continue
		}
		s := &script{mod: mod, rt: goja.New()}
		s.rt.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
		s.rt.Set("game", s.api(c))
		s.rt.Set("console", map[string]any{"log": func(args ...any) { log.Println(append([]any{"mod " + mod + ":"}, args...)...) }})
		if _, err := s.run(nil, func() (goja.Value, error) { return s.rt.RunScript(name, string(src)) }); err != nil {
			log.Printf("mod %s: %s: %v", mod, name, err)
		}
	}
}

// run calls into JS with the game at hand, within the time budget.
func (s *script) run(g content.Game, f func() (goja.Value, error)) (goja.Value, error) {
	s.g = g
	s.rt.ClearInterrupt()
	t := time.AfterFunc(scriptBudget, func() { s.rt.Interrupt("too slow: more than " + scriptBudget.String()) })
	defer t.Stop()
	return f()
}

// call runs a JS function; after a few errors, the mod's handler is muted.
func (s *script) call(g content.Game, fn goja.Callable, args ...any) goja.Value {
	if s.errors >= 5 {
		return goja.Undefined()
	}
	vals := make([]goja.Value, len(args))
	for i, a := range args {
		vals[i] = s.rt.ToValue(a)
	}
	v, err := s.run(g, func() (goja.Value, error) { return fn(goja.Undefined(), vals...) })
	if err != nil {
		s.errors++
		log.Printf("mod %s: %v", s.mod, err)
		if s.errors == 5 {
			log.Printf("mod %s: too many errors, its scripts are muted", s.mod)
		}
		return goja.Undefined()
	}
	return v
}

// game is what g is during a call; outside of one (while loading), it
// throws.
func (s *script) game() content.Game {
	if s.g == nil {
		panic(s.rt.NewTypeError("the game isn't running yet: call this from a handler"))
	}
	return s.g
}

type jsVec struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type jsEvent struct {
	Name   string  `json:"name"`
	ID     string  `json:"id"`
	At     jsVec   `json:"at"`
	Amount float64 `json:"amount"`
	Dt     float64 `json:"dt"`
}

func (s *script) api(c *content.Content) map[string]any {
	isEntity := func(id string) bool { _, ok := c.Entities.Get(id); return ok }
	return map[string]any{
		// Listening.
		"on": func(name string, fn goja.Callable) {
			c.On(name, func(g content.Game, e *content.Event) {
				s.call(g, fn, jsEvent{e.Name, e.ID, jsVec(e.At), e.Amount, e.Dt})
			})
		},
		"randomEvent": func(def struct {
			ID    string        `json:"id"`
			Every float64       `json:"every"`
			When  goja.Callable `json:"when"`
			Run   goja.Callable `json:"run"`
		}) {
			if def.Run == nil || def.Every <= 0 {
				panic(s.rt.NewTypeError("randomEvent needs every > 0 and run"))
			}
			ev := content.RandomEvent{Every: def.Every, Run: func(g content.Game) { s.call(g, def.Run) }}
			if def.When != nil {
				ev.When = func(g content.Game) bool { return s.call(g, def.When).ToBoolean() }
			}
			c.Events.Add(qualifyIn(s.mod, def.ID), ev)
		},

		// Reading.
		"credits":  func() float64 { return s.game().Credits() },
		"tank":     func() float64 { return s.game().Tank() },
		"capacity": func() float64 { return s.game().Capacity() },
		"tool":     func() string { return s.game().ToolID() },
		"level": func(id string) int {
			return s.game().Level(refIn(s.mod, id, func(id string) bool { _, ok := c.Upgrades.Get(id); return ok }))
		},
		"isDay":  func() bool { return s.game().SunHeight() > 0.05 },
		"inside": func() string { return s.game().Inside() },
		"clock":  func() float64 { return s.game().Clock() },
		// distanceToBase: in meters along the ground.
		"distanceToBase": func() float64 { return planet.ArcDist(s.game().PlayerPos().Norm(), planet.BaseUp) },
		"player":         func() jsVec { return jsVec(s.game().PlayerPos()) },

		// Acting.
		"say":         func(text string) { s.game().Say(text) },
		"giveCredits": func(n float64) { s.game().GiveCredits(n) },
		// burst throws sparks around the player: color "#rrggbb".
		"burst": func(n int, col string) {
			c, err := content.HexColor(col)
			if err != nil {
				c = color.RGBA{120, 235, 245, 255} // cyan
			}
			s.game().Burst(n, c)
		},
		"beep":      func(freq float64) { s.game().Beep(freq) },
		"dustDevil": func() { s.game().SpawnDustDevil() },
		"random":    func() float64 { return s.game().Random() },
		// spawn sets an entity where the player aims (or right ahead).
		"spawn": func(kind string) bool { return s.game().Spawn(refIn(s.mod, kind, isEntity)) },
		// entities counts the placed entities of a kind, all of them for "".
		"entities": func(kind string) int {
			if kind != "" {
				kind = refIn(s.mod, kind, isEntity)
			}
			return s.game().Entities(kind)
		},
		"giveItem": func(id string, n int) {
			s.game().GiveItem(refIn(s.mod, id, func(id string) bool { _, ok := c.Items.Get(id); return ok }), n)
		},
		"driving": func() string { return s.game().Driving() },
		// debugLine adds a line to the debug overlay (F3): fn returns it.
		"debugLine": func(fn goja.Callable) {
			c.Debug = append(c.Debug, func(g content.Game) string { return s.call(g, fn).String() })
		},

		// Remembering, in the save: numbers, strings, lists, objects.
		"get": func(key string) any { return s.game().ModData(s.mod)[key] },
		"set": func(key string, v any) { s.game().ModData(s.mod)[key] = v },
	}
}

// refIn resolves a name used by a mod's script: its own entry first, then
// the core's.
func refIn(mod, id string, exists func(string) bool) string {
	if content.ValidID(id) {
		return id
	}
	if own := mod + ":" + id; exists(own) {
		return own
	}
	return content.Qualify(id)
}

// qualifyIn puts a bare name in the mod's namespace.
func qualifyIn(mod, id string) string {
	if id == "" {
		return mod + ":event"
	}
	if content.ValidID(id) {
		return id
	}
	return fmt.Sprintf("%s:%s", mod, id)
}
