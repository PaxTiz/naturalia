package game

import (
	"strconv"
	"strings"

	rl "github.com/gen2brain/raylib-go/raylib"
)

// Action is something done with a key, which the player can bind to
// another (the pause screen's settings).
type Action int

const (
	ActForward Action = iota
	ActBack
	ActLeft
	ActRight
	ActUp // jump, climb
	ActDown
	ActRun // run, boost, faster camera
	ActSlow
	ActUse
	ActPhoto
	ActDock
	ActCamera
	ActPlace
	ActNextItem
	ActTurnY
	ActTurnZ
	ActTurnX
	ActStick
	ActGrid
	ActGridStep
	ActStow
	ActTurnQuarter
	ActTurnDegree
	ActTurnBack
	ActRollLeft
	ActRollRight
	ActRollReset
	ActPhotoPanel
	ActMute
	ActFullscreen
	Actions
)

// Where an action acts: two actions may share a key only if they never act
// in the same place.
const (
	onFoot = 1 << iota
	inVehicle
	building
	inPhoto
	everywhere = onFoot | inVehicle | building | inPhoto
)

// ActionDef is an action's name in the settings, its group, its default
// key and its key in settings.json.
type ActionDef struct {
	ID, Name, Group string
	Key             int32
	where           int
}

// Keys are physical positions: the defaults name the QWERTY key, which is
// where ZQSD sits on AZERTY.
var ActionDefs = [Actions]ActionDef{
	{"forward", "Avancer", "Déplacement", rl.KeyW, onFoot | inVehicle | inPhoto},
	{"back", "Reculer", "Déplacement", rl.KeyS, onFoot | inVehicle | inPhoto},
	{"left", "Aller à gauche", "Déplacement", rl.KeyA, onFoot | inVehicle | inPhoto},
	{"right", "Aller à droite", "Déplacement", rl.KeyD, onFoot | inVehicle | inPhoto},
	{"up", "Sauter, monter", "Déplacement", rl.KeySpace, onFoot | inVehicle | inPhoto},
	{"down", "Descendre", "Déplacement", rl.KeyLeftControl, onFoot | inVehicle | inPhoto},
	{"run", "Courir, boost", "Déplacement", rl.KeyLeftShift, onFoot | inVehicle | inPhoto},
	{"slow", "Ralentir la caméra photo", "Déplacement", rl.KeyLeftAlt, inPhoto},
	{"use", "Utiliser, sortir du véhicule", "Actions", rl.KeyE, onFoot | inVehicle},
	{"photo", "Mode photo", "Actions", rl.KeyP, onFoot | inPhoto},
	{"dock", "Garer le véhicule", "Actions", rl.KeyG, inVehicle},
	{"camera", "Caméra du véhicule", "Actions", rl.KeyV, inVehicle},
	{"place", "Poser", "Construction", rl.KeyF, onFoot | building},
	{"next", "Changer d'objet", "Construction", rl.KeyR, onFoot | building},
	{"turn_y", "Tourner", "Construction", rl.KeyX, onFoot | building},
	{"turn_z", "Basculer en avant", "Construction", rl.KeyC, onFoot | building},
	{"turn_x", "Basculer de côté", "Construction", rl.KeyB, onFoot | building},
	{"stick", "Coller ou libre", "Construction", rl.KeyT, onFoot | building},
	{"grid", "Grille", "Construction", rl.KeyG, building},
	{"grid_step", "Pas de la grille", "Construction", rl.KeyH, building},
	{"stow", "Ranger, annuler", "Construction", rl.KeyBackspace, onFoot | building},
	{"turn_quarter", "Tourner d'un quart, de 15° sur la grille (maintenu)", "Construction", rl.KeyLeftControl, building},
	{"turn_degree", "Tourner d'un degré (maintenu)", "Construction", rl.KeyLeftAlt, building},
	{"turn_back", "Tourner à l'envers (maintenu)", "Construction", rl.KeyLeftShift, building},
	{"roll_left", "Pencher à gauche", "Photo", rl.KeyQ, inPhoto},
	{"roll_right", "Pencher à droite", "Photo", rl.KeyE, inPhoto},
	{"roll_reset", "Redresser", "Photo", rl.KeyR, inPhoto},
	{"photo_panel", "Masquer le panneau", "Photo", rl.KeyH, inPhoto},
	{"mute", "Couper le son", "Général", rl.KeyM, everywhere},
	{"fullscreen", "Plein écran", "Général", rl.KeyF11, everywhere},
}

// keys are the keys bound, from the settings (applySettings).
var keys [Actions]int32

// KeyOf is the key bound to a.
func KeyOf(a Action) int32 { return keys[a] }

// KeyDown tells whether a's key is held.
func KeyDown(a Action) bool { return keys[a] != 0 && rl.IsKeyDown(keys[a]) }

// KeyPressed tells whether a's key went down this frame.
func KeyPressed(a Action) bool { return keys[a] != 0 && rl.IsKeyPressed(keys[a]) }

// KeyLabel is a's key as the keyboard shows it.
func KeyLabel(a Action) string { return KeyName(keys[a]) }

// KeyLabels joins the keys of several actions, for the hints: "ZQSD".
func KeyLabels(as ...Action) string {
	var b strings.Builder
	sep := ""
	for _, a := range as {
		l := KeyLabel(a)
		if len([]rune(l)) > 1 {
			sep = " "
		}
		if b.Len() > 0 {
			b.WriteString(sep)
		}
		b.WriteString(l)
	}
	return b.String()
}

// Reserved keys stay out of the bindings: Escape pauses and cancels.
func Reserved(key int32) bool { return key == rl.KeyEscape || key == rl.KeyNull }

// Bind gives a key to an action. An action that had it and acts in the
// same places takes the old key instead, so no key does two things at once.
func (g *Game) Bind(a Action, key int32) {
	if Reserved(key) || keys[a] == key {
		return
	}
	old := keys[a]
	for b := range Actions {
		if b != a && keys[b] == key && ActionDefs[b].where&ActionDefs[a].where != 0 {
			g.setKey(b, old)
		}
	}
	g.setKey(a, key)
}

// ResetKeys brings back every default key.
func (g *Game) ResetKeys() {
	for a := range Actions {
		g.setKey(a, ActionDefs[a].Key)
	}
}

func (g *Game) setKey(a Action, key int32) {
	keys[a] = key
	if key == ActionDefs[a].Key {
		delete(g.Settings.Keys, ActionDefs[a].ID)
	} else {
		if g.Settings.Keys == nil {
			g.Settings.Keys = map[string]int32{}
		}
		g.Settings.Keys[ActionDefs[a].ID] = key
	}
	g.settingsChanged = true
}

// loadKeys reads the bindings from the settings: the defaults, then what
// the player changed.
func (g *Game) loadKeys() {
	for a := range Actions {
		keys[a] = ActionDefs[a].Key
		if k, ok := g.Settings.Keys[ActionDefs[a].ID]; ok && !Reserved(k) {
			keys[a] = k
		}
	}
}

// PollKeys reads the keys pressed this frame, once, before anything else:
// the menu waiting for a key to bind takes them from KeysPressed. When a
// single key types a single character, that character becomes its label,
// so the names follow the keyboard's layout (Z on AZERTY where QWERTY has
// W): raylib only knows the positions.
func (g *Game) PollKeys() {
	g.KeysPressed = g.KeysPressed[:0]
	for k := rl.GetKeyPressed(); k != 0; k = rl.GetKeyPressed() {
		g.KeysPressed = append(g.KeysPressed, k)
	}
	g.CharsTyped = g.CharsTyped[:0]
	for c := rl.GetCharPressed(); c != 0; c = rl.GetCharPressed() {
		g.CharsTyped = append(g.CharsTyped, c)
	}
	chars := g.CharsTyped
	mods := rl.IsKeyDown(rl.KeyLeftShift) || rl.IsKeyDown(rl.KeyRightShift) || rl.IsKeyDown(rl.KeyLeftAlt) ||
		rl.IsKeyDown(rl.KeyRightAlt) || rl.IsKeyDown(rl.KeyLeftControl) || rl.IsKeyDown(rl.KeyLeftSuper)
	if len(g.KeysPressed) != 1 || len(chars) != 1 || mods {
		return
	}
	k, c := g.KeysPressed[0], chars[0]
	if k < rl.KeyApostrophe || k > rl.KeyGrave || c <= ' ' {
		return // only the keys that type; not the keypad's
	}
	label := strings.ToUpper(string(c))
	if g.Settings.KeyNames == nil {
		g.Settings.KeyNames = map[int32]string{}
	}
	if g.Settings.KeyNames[k] != label {
		g.Settings.KeyNames[k] = label
		keyNames[k] = label
		g.settingsChanged = true
	}
}

// keyNames are the labels learned from the keyboard (PollKeys).
var keyNames = map[int32]string{}

var specialKeys = map[int32]string{
	rl.KeySpace: "ESPACE", rl.KeyEnter: "ENTRÉE", rl.KeyTab: "TAB", rl.KeyBackspace: "RETOUR",
	rl.KeyInsert: "INSER", rl.KeyDelete: "SUPPR", rl.KeyRight: "DROITE", rl.KeyLeft: "GAUCHE",
	rl.KeyDown: "BAS", rl.KeyUp: "HAUT", rl.KeyPageUp: "PAGE HAUT", rl.KeyPageDown: "PAGE BAS",
	rl.KeyHome: "DÉBUT", rl.KeyEnd: "FIN", rl.KeyCapsLock: "VERR MAJ", rl.KeyScrollLock: "DÉFIL",
	rl.KeyNumLock: "VERR NUM", rl.KeyPrintScreen: "IMPR", rl.KeyPause: "PAUSE",
	rl.KeyLeftShift: "MAJ", rl.KeyLeftControl: "CTRL", rl.KeyLeftAlt: "ALT", rl.KeyLeftSuper: "CMD",
	rl.KeyRightShift: "MAJ DR", rl.KeyRightControl: "CTRL DR", rl.KeyRightAlt: "ALT DR", rl.KeyRightSuper: "CMD DR",
	rl.KeyKbMenu: "MENU", rl.KeyKpDecimal: "PAVÉ .", rl.KeyKpDivide: "PAVÉ /", rl.KeyKpMultiply: "PAVÉ *",
	rl.KeyKpSubtract: "PAVÉ -", rl.KeyKpAdd: "PAVÉ +", rl.KeyKpEnter: "PAVÉ ENTRÉE", rl.KeyKpEqual: "PAVÉ =",
}

// KeyName is a key's label: learned from the keyboard, else its name.
func KeyName(k int32) string {
	if s, ok := keyNames[k]; ok {
		return s
	}
	if s, ok := specialKeys[k]; ok {
		return s
	}
	switch {
	case k == 0:
		return "—"
	case k >= rl.KeyF1 && k <= rl.KeyF12:
		return "F" + strconv.Itoa(int(k-rl.KeyF1+1))
	case k >= rl.KeyKp0 && k <= rl.KeyKp9:
		return "PAVÉ " + string(rune('0'+k-rl.KeyKp0))
	case k >= rl.KeyApostrophe && k <= rl.KeyGrave:
		return string(rune(k))
	}
	return "?"
}
