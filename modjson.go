package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"mars_sweeper/internal/mesh"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// External mods (docs/MODS.md): a folder in mods/ with a mod.json, which
// holds data (tools, upgrades, materials) and compositions (furniture made
// of kit shapes, buildings made of furniture). Each mod becomes a pack, so
// it loads exactly like the core one. See examples/mods/serre.
//
// Inside a mod, a bare name refers to the mod's own entry when it has one,
// to the core's otherwise.

type modFile struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Requires []string `json:"requires"`

	Tools     []Tool               `json:"tools"`
	Upgrades  []Upgrade            `json:"upgrades"`
	Tokens    map[string]token     `json:"tokens"`
	Kits      map[string]kitJSON   `json:"kits"`
	Buildings []Blueprint          `json:"buildings"`
	Entities  map[string]EntityDef `json:"entities"` // things standing in the world (entities.go)
	Items     []Item               `json:"items"`    // carried, placed as entities with F
	Vehicles  []Vehicle            `json:"vehicles"` // driven (vehicles.go)
	Scripts   []string             `json:"scripts"`  // JavaScript files, run in order (script.go)
	// Terrain changes the generation: only the settings given, features
	// added to those of other packs (terrain_mod.go).
	Terrain json.RawMessage `json:"terrain"`

	// Patch changes some fields of existing entries, by ID; Remove takes
	// entries out.
	Patch struct {
		Tools     map[string]json.RawMessage `json:"tools"`
		Upgrades  map[string]json.RawMessage `json:"upgrades"`
		Tokens    map[string]json.RawMessage `json:"tokens"`
		Buildings map[string]json.RawMessage `json:"buildings"`
		Vehicles  map[string]json.RawMessage `json:"vehicles"`
		Items     map[string]json.RawMessage `json:"items"`
	} `json:"patch"`
	Remove struct {
		Tools     []string `json:"tools"`
		Upgrades  []string `json:"upgrades"`
		Buildings []string `json:"buildings"`
		Events    []string `json:"events"`
		Entities  []string `json:"entities"`
		Items     []string `json:"items"`
		Vehicles  []string `json:"vehicles"`
	} `json:"remove"`

	dir string
}

// modDirs are where mods are looked for: mods/ in the current folder, then
// in the game's settings folder, next to the save.
func modDirs() []string {
	dirs := []string{"mods"}
	if d, err := saveDir(); err == nil {
		dirs = append(dirs, filepath.Join(d, "mods"))
	}
	return dirs
}

// disabledMods lists the mods turned off in mods.json, in the settings
// folder: {"disabled": ["serre"]}.
func disabledMods() map[string]bool {
	off := map[string]bool{}
	d, err := saveDir()
	if err != nil {
		return off
	}
	var cfg struct {
		Disabled []string `json:"disabled"`
	}
	if data, err := os.ReadFile(filepath.Join(d, "mods.json")); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Println("mods.json:", err)
		}
	}
	for _, id := range cfg.Disabled {
		off[id] = true
	}
	return off
}

// loadMods registers a pack for every mod found, before loadContent.
func loadMods() {
	off := disabledMods()
	seen := map[string]bool{}
	for _, root := range modDirs() {
		entries, _ := os.ReadDir(root)
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			m, err := readMod(filepath.Join(root, e.Name()))
			switch {
			case err != nil:
				log.Printf("mod %s skipped: %v", e.Name(), err)
			case off[m.ID]:
				log.Printf("mod %s disabled", m.ID)
			case seen[m.ID]:
				log.Printf("mod %s: already loaded from another folder", m.ID)
			default:
				seen[m.ID] = true
				RegisterPack(m.pack())
			}
		}
	}
}

func readMod(dir string) (*modFile, error) {
	data, err := os.ReadFile(filepath.Join(dir, "mod.json"))
	if err != nil {
		return nil, err
	}
	m := &modFile{dir: dir}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields() // a typo in a field name says so
	if err := dec.Decode(m); err != nil {
		return nil, fmt.Errorf("mod.json: %w", err)
	}
	if m.ID == "" {
		m.ID = filepath.Base(dir)
	}
	if m.ID == "core" || strings.ContainsAny(m.ID, ": /\\") {
		return nil, fmt.Errorf("invalid id %q", m.ID)
	}
	return m, nil
}

func (m *modFile) pack() Pack {
	requires := m.Requires
	if len(requires) == 0 {
		requires = []string{"core"}
	}
	return Pack{ID: m.ID, Name: m.Name, Version: m.Version, Requires: requires, Register: m.register}
}

// own puts a name the mod defines in its namespace.
func (m *modFile) own(id string) string {
	if strings.Contains(id, ":") {
		return id
	}
	return m.ID + ":" + id
}

// ref resolves a reference: the mod's own entry first, then the core's.
func (m *modFile) ref(id string, mine func(string) bool) string {
	if id == "" || strings.Contains(id, ":") {
		return id
	}
	if mine(m.ID + ":" + id) {
		return m.ID + ":" + id
	}
	return "core:" + id
}

func (m *modFile) register(c *Content) {
	isTok := func(id string) bool { _, ok := c.Tokens.Get(id); return ok }
	isKit := func(id string) bool { _, ok := c.Kits.Get(id); return ok }
	isBld := func(id string) bool { _, ok := c.Buildings.Get(id); return ok }

	for _, id := range m.Remove.Tools {
		c.Tools.Remove(id)
	}
	for _, id := range m.Remove.Upgrades {
		c.Upgrades.Remove(id)
	}
	for _, id := range m.Remove.Buildings {
		c.Buildings.Remove(id)
	}
	for _, id := range m.Remove.Events {
		c.Events.Remove(id)
	}
	for _, id := range m.Remove.Entities {
		c.Entities.Remove(id)
	}
	for _, id := range m.Remove.Items {
		c.Items.Remove(id)
	}
	for _, id := range m.Remove.Vehicles {
		c.Vehicles.Remove(id)
	}

	for name, t := range m.Tokens {
		c.Tokens.Add(m.own(name), t)
	}
	for name, k := range m.Kits {
		for i := range k.Parts {
			k.Parts[i].Token = m.ref(k.Parts[i].Token, isTok)
		}
		c.Kits.Add(m.own(name), k.kit())
	}
	refParts := func(parts []partJSON) {
		for i := range parts {
			parts[i].Token = m.ref(parts[i].Token, isTok)
		}
	}
	isEnt := func(id string) bool { _, ok := c.Entities.Get(id); return ok }
	for name, e := range m.Entities {
		refParts(e.Parts)
		e.Item = ""
		c.Entities.Add(m.own(name), e)
	}
	for _, v := range m.Vehicles {
		v.ID = m.own(v.ID)
		refParts(v.Parts)
		c.Vehicles.Add(v.ID, v)
	}
	for _, it := range m.Items {
		it.ID = m.own(it.ID)
		it.Places = m.ref(orDefault(it.Places, it.ID), isEnt)
		c.Items.Add(it.ID, it)
	}
	for _, t := range m.Tools {
		t.ID = m.own(t.ID)
		c.Tools.Add(t.ID, t)
	}
	for _, u := range m.Upgrades {
		u.ID = m.own(u.ID)
		u.Icon = orDefault(u.Icon, "power")
		for _, e := range u.Effects {
			if _, ok := stats[e.Stat]; !ok {
				log.Printf("mod %s: upgrade %s acts on an unknown stat %q", m.ID, u.ID, e.Stat)
			}
		}
		c.Upgrades.Add(u.ID, u)
	}
	for _, bp := range m.Buildings {
		bp.ID = m.own(bp.ID)
		bp.Style = m.ref(orDefault(bp.Style, "habitat.white"), isTok)
		bp.Accent = m.ref(orDefault(bp.Accent, "neon.cyan"), isTok)
		if bp.Shape.Radius == 0 {
			bp.Shape = Dome{Radius: 4, WallH: 2.6}
		}
		for i := range bp.Props {
			bp.Props[i].Kit = m.ref(bp.Props[i].Kit, isKit)
		}
		c.Buildings.Add(bp.ID, bp)
	}
	// Links once all are added: a mod's building may link to another.
	for _, bp := range m.Buildings {
		if b, _ := c.Buildings.Get(m.own(bp.ID)); b != nil && b.Link != "" {
			b.Link = m.ref(b.Link, isBld)
		}
	}

	patch := func(kind string, ids map[string]json.RawMessage, apply func(id string, raw json.RawMessage) bool) {
		for id, raw := range ids {
			if !apply(id, raw) {
				log.Printf("mod %s: no %s %s to patch", m.ID, kind, id)
			}
		}
	}
	unmarshal := func(raw json.RawMessage, v any) {
		if err := json.Unmarshal(raw, v); err != nil {
			log.Printf("mod %s: patch: %v", m.ID, err)
		}
	}
	patch("tool", m.Patch.Tools, func(id string, raw json.RawMessage) bool {
		return c.Tools.Patch(id, func(t *Tool) { unmarshal(raw, t) })
	})
	patch("upgrade", m.Patch.Upgrades, func(id string, raw json.RawMessage) bool {
		return c.Upgrades.Patch(id, func(u *Upgrade) { unmarshal(raw, u) })
	})
	patch("token", m.Patch.Tokens, func(id string, raw json.RawMessage) bool {
		return c.Tokens.Patch(id, func(t *token) { unmarshal(raw, t) })
	})
	patch("building", m.Patch.Buildings, func(id string, raw json.RawMessage) bool {
		return c.Buildings.Patch(id, func(b *Blueprint) { unmarshal(raw, b) })
	})
	patch("vehicle", m.Patch.Vehicles, func(id string, raw json.RawMessage) bool {
		return c.Vehicles.Patch(id, func(v *Vehicle) { unmarshal(raw, v) })
	})
	patch("item", m.Patch.Items, func(id string, raw json.RawMessage) bool {
		return c.Items.Patch(id, func(it *Item) { unmarshal(raw, it) })
	})

	if len(m.Terrain) > 0 {
		before := c.Terrain.Features
		c.Terrain.Features = nil
		unmarshal(m.Terrain, &c.Terrain)
		c.Terrain.Features = append(before, c.Terrain.Features...)
	}

	m.loadScripts(c)
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// kitJSON is a piece of furniture made of kit shapes, in its own frame:
// what it is made of, what the player bumps into, what E does.
type kitJSON struct {
	Parts     []partJSON `json:"parts"`
	Obstacles []struct {
		At     vec3    `json:"at"`
		Radius float64 `json:"radius"`
	} `json:"obstacles"`
	Use    string  `json:"use"` // "map", "shop", "sleep", "deposit"; any other only sends "use"
	Prompt string  `json:"prompt"`
	Reach  float64 `json:"reach"`
}

// partJSON is one shape: a bevelled box, or a prism (a cylinder or a
// truncated cone with that many sides) standing on At along Axis.
type partJSON struct {
	Shape string  `json:"shape"` // "box" or "prism"
	Token string  `json:"token"`
	At    vec3    `json:"at"`
	Bevel float64 `json:"bevel"`
	// box
	Size vec3    `json:"size"`
	Yaw  float64 `json:"yaw"` // degrees, turning its width (X) toward Z
	// prism
	Axis  vec3    `json:"axis"` // default up
	R0    float64 `json:"r0"`   // radius at the base...
	R1    float64 `json:"r1"`   // ...and at the top, r0 if left out
	H     float64 `json:"h"`
	Sides int     `json:"sides"`
	Lit   string  `json:"lit"` // "indoor" (default) or "outdoor"
}

func (k kitJSON) kit() kitProp {
	kp := kitProp{reach: orZero(k.Reach, 2.6), prompt: k.Prompt}
	for _, o := range k.Obstacles {
		kp.obstacles = append(kp.obstacles, obstacle{o.At, o.Radius})
	}
	parts := k.Parts
	kp.build = func(b *building, at vec3, _ int) {
		for _, p := range parts {
			lit := float32(mesh.LitIndoor)
			if p.Lit == "outdoor" {
				lit = mesh.LitSun
			}
			m := b.Part(p.Token, lit, vec3{})
			switch p.Shape {
			case "prism":
				axis := p.Axis
				if axis == (vec3{}) {
					axis = vec3{0, 1, 0}
				}
				m.Prism(at.Add(p.At), axis.Norm(), p.R0, orZero(p.R1, p.R0), p.H, max(3, orInt(p.Sides, 12)), p.Bevel)
			default:
				a := p.Yaw * math.Pi / 180
				m.Box(at.Add(p.At), p.Size, vec3{math.Cos(a), 0, math.Sin(a)}, p.Bevel)
			}
		}
	}
	use := map[string]func(g *Game, at vec3){
		"map":     func(g *Game, _ vec3) { g.setMode(modeMap) },
		"shop":    func(g *Game, _ vec3) { g.setMode(modeShop) },
		"sleep":   func(g *Game, _ vec3) { g.mode, g.sleepT = modeSleep, 0 },
		"deposit": (*Game).deposit,
	}
	if f, ok := use[k.Use]; ok {
		kp.use = f
	} else if k.Use != "" || k.Prompt != "" {
		kp.use = func(*Game, vec3) {} // only the "use" event, for scripts
	}
	return kp
}

func orZero(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}

func orInt(v, def int) int {
	if v == 0 {
		return def
	}
	return v
}

// JSON forms of the engine's types.

// hexColor reads "#rrggbb" or "#rrggbbaa".
func hexColor(s string) (color.RGBA, error) {
	var c color.RGBA
	c.A = 255
	s = strings.TrimPrefix(s, "#")
	var err error
	switch len(s) {
	case 6:
		_, err = fmt.Sscanf(s, "%02x%02x%02x", &c.R, &c.G, &c.B)
	case 8:
		_, err = fmt.Sscanf(s, "%02x%02x%02x%02x", &c.R, &c.G, &c.B, &c.A)
	default:
		err = fmt.Errorf("color %q is not #rrggbb", s)
	}
	return c, err
}

var headShapes = map[string]headShape{"block": headBlock, "taper": headTaper, "flare": headFlare, "nozzle": headNozzle}

// A look patched from JSON keeps the fields it doesn't mention.
func (l *ToolLook) UnmarshalJSON(b []byte) error {
	var j struct {
		HandleLen   *float64 `json:"handle_len"`
		HandleR     *float64 `json:"handle_r"`
		HandleColor *string  `json:"handle_color"`
		HeadLen     *float64 `json:"head_len"`
		HeadR       *float64 `json:"head_r"`
		HeadColor   *string  `json:"head_color"`
		Head        *string  `json:"head"`
		Glow        *float32 `json:"glow"`
	}
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	set := func(dst *float64, v *float64) {
		if v != nil {
			*dst = *v
		}
	}
	set(&l.HandleLen, j.HandleLen)
	set(&l.HandleR, j.HandleR)
	set(&l.HeadLen, j.HeadLen)
	set(&l.HeadR, j.HeadR)
	if j.Glow != nil {
		l.Glow = *j.Glow
	}
	for _, c := range []struct {
		dst *color.RGBA
		v   *string
	}{{&l.HandleCol, j.HandleColor}, {&l.HeadCol, j.HeadColor}} {
		if c.v != nil {
			col, err := hexColor(*c.v)
			if err != nil {
				return err
			}
			*c.dst = col
		}
	}
	if j.Head != nil {
		h, ok := headShapes[*j.Head]
		if !ok {
			return fmt.Errorf("head %q: block, taper, flare or nozzle", *j.Head)
		}
		l.Head = h
	}
	return nil
}

var materials = map[string]int{
	"plain": mtPlain, "paint": mtPaint, "panel": mtPanel, "metal": mtMetal,
	"fabric": mtFabric, "hazard": mtHazard,
}

// A token reads {"color": "#rrggbb", "material": "panel", "scale": 1,
// "spec": 0.5, "emissive": 0}; patched, it keeps what isn't mentioned.
func (t *token) UnmarshalJSON(b []byte) error {
	var j struct {
		Color    *string  `json:"color"`
		Material *string  `json:"material"`
		Scale    *float32 `json:"scale"`
		Spec     *float32 `json:"spec"`
		Emissive *float32 `json:"emissive"`
	}
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	if t.scale == 0 {
		t.scale = 1 // a new token
	}
	if j.Color != nil {
		c, err := hexColor(*j.Color)
		if err != nil {
			return err
		}
		t.col = c
	}
	if j.Material != nil {
		mt, ok := materials[*j.Material]
		if !ok {
			return fmt.Errorf("material %q: plain, paint, panel, metal, fabric or hazard", *j.Material)
		}
		t.mat = mt
		if mt == mtFabric {
			t.scale = 1 / fabricTile
		} else if mt == mtPaint {
			t.scale = 1 / paintTile
		}
	}
	for _, f := range []struct {
		dst *float32
		v   *float32
	}{{&t.scale, j.Scale}, {&t.spec, j.Spec}, {&t.emissive, j.Emissive}} {
		if f.v != nil {
			*f.dst = *f.v
		}
	}
	return nil
}
