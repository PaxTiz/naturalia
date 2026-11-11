package game

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"mars_sweeper/internal/content"
	"mars_sweeper/internal/planet"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type saveFile struct {
	Version   int                       `json:"version"`
	SavedAt   int64                     `json:"saved_at"`
	State     State                     `json:"state"`
	Player    Player                    `json:"player"`
	Seed      uint64                    `json:"seed"`
	Tiles     map[planet.TileKey][]byte `json:"tiles"`     // cleaned tiles only, uint16 little-endian
	Collected []planet.TileKey          `json:"collected"` // tiles whose crystal was picked up
}

// Bumped when the world format changes: older saves keep the progress
// (credits, upgrades) but start on a fresh Mars.
const saveVersion = 5

func saveDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mars_sweeper"), nil
}

func (g *Game) Save() {
	dir, err := saveDir()
	if err != nil {
		log.Println("save:", err)
		return
	}
	data, err := json.Marshal(saveFile{
		Version:   saveVersion,
		SavedAt:   time.Now().Unix(),
		State:     g.State,
		Player:    g.Player,
		Seed:      g.World.Seed,
		Tiles:     g.World.TileBytes(),
		Collected: g.World.CollectedKeys(),
	})
	if err != nil {
		log.Println("save:", err)
		return
	}
	// The dust layer is mostly long runs of equal values: it compresses well.
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	zw.Write(data)
	zw.Close()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Println("save:", err)
		return
	}
	// Write then rename, so a crash mid-save never corrupts the previous save.
	path := filepath.Join(dir, "save.json.gz")
	if err := os.WriteFile(path+".tmp", buf.Bytes(), 0o644); err != nil {
		log.Println("save:", err)
		return
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		log.Println("save:", err)
	}
}

// load restores the last save. g.world stays nil when there is no world to
// restore, including for saves from older versions of which only the
// progress is kept.
func (g *Game) load() {
	dir, err := saveDir()
	if err != nil {
		return
	}
	var sf saveFile
	if data, err := readGzip(filepath.Join(dir, "save.json.gz")); err == nil {
		if err := json.Unmarshal(data, &sf); err != nil {
			log.Println("load: corrupted save, starting fresh:", err)
			return
		}
	} else if data, err := os.ReadFile(filepath.Join(dir, "save.json")); err == nil {
		// Flat-world save from the very first version.
		if err := json.Unmarshal(data, &sf); err != nil {
			return
		}
	} else {
		return
	}

	g.State = sf.State
	migrateState(&g.State, g.Content)
	if sf.Version != saveVersion {
		log.Println("load: save from an older version, keeping progress on a new Mars")
		return
	}
	g.Player = sf.Player
	g.World = planet.NewWorld(sf.Seed, &g.Content.Terrain)
	g.World.SetTileBytes(sf.Tiles)
	g.World.SetCollected(sf.Collected)
}

func readGzip(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(zr)
}

// coreTools are the tools in their order before IDs, when saves stored an
// index (tool_tier).
var coreTools = []string{"core:toothbrush", "core:paintbrush", "core:broom", "core:vacuum", "core:blower", "core:vacuum-cannon"}

// migrateState turns indexes and bare names from older saves into IDs.
func migrateState(st *State, c *content.Content) {
	if st.Tool == "" {
		st.Tool = coreTools[max(0, min(st.OldTier, len(coreTools)-1))]
	}
	st.OldTier = 0
	for k, v := range st.Levels {
		if !strings.Contains(k, ":") {
			delete(st.Levels, k)
			st.Levels["core:"+k] = v
		}
	}
	if _, ok := c.Tools.Get(st.Tool); !ok {
		log.Printf("load: tool %s no longer exists (mod removed?), back to the first one", st.Tool)
	}
}

// ModDirs are where mods are looked for: mods/ in the current folder, then
// in the game's settings folder, next to the save.
func ModDirs() []string {
	dirs := []string{"mods"}
	if d, err := saveDir(); err == nil {
		dirs = append(dirs, filepath.Join(d, "mods"))
	}
	return dirs
}

// DisabledMods lists the mods turned off in mods.json, in the settings
// folder: {"disabled": ["serre"]}.
func DisabledMods() map[string]bool {
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
