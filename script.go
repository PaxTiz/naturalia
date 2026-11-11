package main

import (
	"fmt"
	"log"
	"math"
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
	g      *Game // during a call
	errors int
}

// loadScripts runs a mod's scripts while its pack registers.
func (m *modFile) loadScripts(c *Content) {
	for _, name := range m.Scripts {
		src, err := os.ReadFile(filepath.Join(m.dir, filepath.Clean("/"+name)))
		if err != nil {
			log.Printf("mod %s: %v", m.ID, err)
			continue
		}
		s := &script{mod: m.ID, rt: goja.New()}
		s.rt.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))
		s.rt.Set("game", s.api(c))
		s.rt.Set("console", map[string]any{"log": func(args ...any) { log.Println(append([]any{"mod " + m.ID + ":"}, args...)...) }})
		if _, err := s.run(nil, func() (goja.Value, error) { return s.rt.RunScript(name, string(src)) }); err != nil {
			log.Printf("mod %s: %s: %v", m.ID, name, err)
		}
	}
}

// run calls into JS with the game at hand, within the time budget.
func (s *script) run(g *Game, f func() (goja.Value, error)) (goja.Value, error) {
	s.g = g
	s.rt.ClearInterrupt()
	t := time.AfterFunc(scriptBudget, func() { s.rt.Interrupt("too slow: more than " + scriptBudget.String()) })
	defer t.Stop()
	return f()
}

// call runs a JS function; after a few errors, the mod's handler is muted.
func (s *script) call(g *Game, fn goja.Callable, args ...any) goja.Value {
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
func (s *script) game() *Game {
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

func (s *script) api(c *Content) map[string]any {
	return map[string]any{
		// Listening.
		"on": func(name string, fn goja.Callable) {
			c.On(name, func(g *Game, e *Event) {
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
			ev := RandomEvent{Every: def.Every, Run: func(g *Game) { s.call(g, def.Run) }}
			if def.When != nil {
				ev.When = func(g *Game) bool { return s.call(g, def.When).ToBoolean() }
			}
			c.Events.Add(qualifyIn(s.mod, def.ID), ev)
		},

		// Reading.
		"credits":  func() float64 { return s.game().st.Credits },
		"tank":     func() float64 { return s.game().st.Tank },
		"capacity": func() float64 { return s.game().capacity() },
		"tool":     func() string { return s.game().tool().ID },
		"level": func(id string) int {
			own := qualifyIn(s.mod, id)
			if _, ok := content.Upgrades.Get(own); !ok {
				own = qualify(id) // not the mod's: the core's
			}
			return s.game().st.Levels[own]
		},
		"isDay":  func() bool { return sunDir.dot(s.game().player.LocalUp()) > 0.05 },
		"inside": func() string { return s.game().inside },
		"clock":  func() float64 { return s.game().clock },
		// distanceToBase: in meters along the ground.
		"distanceToBase": func() float64 { return arcDist(s.game().player.LocalUp(), baseUp) },
		"player": func() jsVec {
			return jsVec(s.game().player.Pos)
		},

		// Acting.
		"say": func(text string) { s.game().say(text) },
		"giveCredits": func(n float64) {
			g := s.game()
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return
			}
			g.st.Credits = math.Max(0, g.st.Credits+n)
			if n > 0 {
				g.st.TotalEarned += n
				g.floaters = append(g.floaters, Floater{Pos: g.player.Eye().add(g.player.Forward().scale(2)), Life: 2, Text: "+" + fmtNum(n) + " cr", C: colGold})
			}
		},
		// burst throws sparks around the player: color "#rrggbb".
		"burst": func(n int, col string) {
			g := s.game()
			c, err := hexColor(col)
			if err != nil {
				c = colCyan
			}
			at := g.player.Eye().add(g.player.Forward().scale(2))
			for range min(n, 200) {
				vel := vec3{g.rng.Float64() - 0.5, g.rng.Float64() - 0.5, g.rng.Float64() - 0.5}.add(g.player.LocalUp().scale(0.6)).scale(4)
				g.addParticle(Particle{Pos: at, Vel: vel, MaxLife: 0.9, Size: 0.06, C: c, Falls: true, Additive: true})
			}
		},
		"beep":      func(freq float64) { s.game().sfx.beep(0.05, 0.25, 0.14, freq) },
		"dustDevil": func() { spawnDevil(s.game()) },
		"random":    func() float64 { return s.game().rng.Float64() },

		// Remembering, in the save: numbers, strings, lists, objects.
		"get": func(key string) any { return s.game().st.Mods[s.mod][key] },
		"set": func(key string, v any) {
			g := s.game()
			if g.st.Mods == nil {
				g.st.Mods = map[string]map[string]any{}
			}
			if g.st.Mods[s.mod] == nil {
				g.st.Mods[s.mod] = map[string]any{}
			}
			g.st.Mods[s.mod][key] = v
		},
	}
}

// qualifyIn puts a bare name in the mod's namespace.
func qualifyIn(mod, id string) string {
	if id == "" {
		return mod + ":event"
	}
	if validID(id) {
		return id
	}
	return fmt.Sprintf("%s:%s", mod, id)
}
