package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"time"
)

type saveFile struct {
	Version int    `json:"version"`
	SavedAt int64  `json:"saved_at"`
	State   State  `json:"state"`
	Player  Player `json:"player"`
	Seed    uint64 `json:"seed"`
	Dust    []byte `json:"dust"`
	Gems    []Gem  `json:"gems"`
}

// Bumped when the format changes incompatibly: older saves are ignored.
const saveVersion = 2

func savePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "moon_sweeper", "save.json"), nil
}

func (g *Game) save() {
	path, err := savePath()
	if err != nil {
		log.Println("save:", err)
		return
	}
	data, err := json.Marshal(saveFile{
		Version: saveVersion,
		SavedAt: time.Now().Unix(),
		State:   g.st,
		Player:  g.player,
		Seed:    g.world.Seed,
		Dust:    g.world.DustBytes(),
		Gems:    g.world.Gems,
	})
	if err != nil {
		log.Println("save:", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Println("save:", err)
		return
	}
	// Write then rename, so a crash mid-save never corrupts the previous save.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		log.Println("save:", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		log.Println("save:", err)
	}
}

func (g *Game) load() bool {
	path, err := savePath()
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	var sf saveFile
	if err := json.Unmarshal(data, &sf); err != nil {
		log.Println("load: corrupted save, starting fresh:", err)
		return false
	}
	if sf.Version != saveVersion {
		log.Println("load: save from an older version, starting fresh")
		return false
	}
	g.st = sf.State
	if g.st.Levels == nil {
		g.st.Levels = map[string]int{}
	}
	g.st.ToolTier = max(0, min(g.st.ToolTier, len(tools)-1))
	g.player = sf.Player
	g.world = NewWorld(sf.Seed)
	g.world.SetDustBytes(sf.Dust)
	g.world.Gems = sf.Gems
	return true
}
