package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

type saveFile struct {
	Version   int                `json:"version"`
	SavedAt   int64              `json:"saved_at"`
	State     State              `json:"state"`
	Player    Player             `json:"player"`
	Seed      uint64             `json:"seed"`
	Tiles     map[tileKey][]byte `json:"tiles"`     // cleaned tiles only, uint16 little-endian
	Collected []tileKey          `json:"collected"` // tiles whose crystal was picked up
}

// Bumped when the world format changes: older saves keep the progress
// (credits, upgrades) but start on a fresh Moon.
const saveVersion = 4

func saveDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "moon_sweeper"), nil
}

func (g *Game) save() {
	dir, err := saveDir()
	if err != nil {
		log.Println("save:", err)
		return
	}
	data, err := json.Marshal(saveFile{
		Version:   saveVersion,
		SavedAt:   time.Now().Unix(),
		State:     g.st,
		Player:    g.player,
		Seed:      g.world.Seed,
		Tiles:     g.world.TileBytes(),
		Collected: g.world.CollectedKeys(),
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
		// Flat-world save from before the Moon became a sphere.
		if err := json.Unmarshal(data, &sf); err != nil {
			return
		}
	} else {
		return
	}

	g.st = sf.State
	g.st.ToolTier = max(0, min(g.st.ToolTier, len(tools)-1))
	if sf.Version != saveVersion {
		log.Println("load: save from an older version, keeping progress on a new Moon")
		return
	}
	g.player = sf.Player
	g.world = NewWorld(sf.Seed)
	g.world.SetTileBytes(sf.Tiles)
	g.world.SetCollected(sf.Collected)
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
