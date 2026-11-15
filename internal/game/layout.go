package game

import (
	"os"
	"os/exec"
	"runtime"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// azerty names the keys that move on a French keyboard, until a press
// teaches the real label (PollKeys).
var azerty = map[int32]string{
	rl.KeyQ: "A", rl.KeyA: "Q", rl.KeyW: "Z", rl.KeyZ: "W",
	rl.KeyM: ",", rl.KeySemicolon: "M", rl.KeyComma: ";", rl.KeyPeriod: ":",
}

// guessLayout fills the labels not learned yet from the keyboard's layout,
// as the system tells it: the input source on macOS, the language
// elsewhere.
func guessLayout() {
	french := false
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("defaults", "read", "com.apple.HIToolbox", "AppleCurrentKeyboardLayoutInputSourceID").Output()
		id := string(out)
		french = err == nil && (strings.Contains(id, "French") || strings.Contains(id, "Belgian")) && !strings.Contains(id, "Swiss")
	} else {
		for _, v := range []string{"LC_ALL", "LC_CTYPE", "LANG"} {
			if l := os.Getenv(v); l != "" {
				french = strings.HasPrefix(l, "fr_FR") || strings.HasPrefix(l, "fr_BE")
				break
			}
		}
	}
	if !french {
		return
	}
	for k, s := range azerty {
		if _, ok := keyNames[k]; !ok {
			keyNames[k] = s
		}
	}
}
