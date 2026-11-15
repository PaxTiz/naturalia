// Package assets holds the game's sound and music files, built into the
// binary. Each music folder keeps its attribution.txt next to the track.
package assets

import "embed"

//go:embed sounds musics
var FS embed.FS
