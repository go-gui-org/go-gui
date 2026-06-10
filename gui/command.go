package gui

import (
	"errors"
	"runtime"
	"strconv"
)

// Shortcut binds a key + modifier combination.
type Shortcut struct {
	Key       KeyCode
	Modifiers Modifier
}

// IsSet returns true if the shortcut has a key assigned.
func (s Shortcut) IsSet() bool { return s.Key != KeyInvalid }

// matches returns true if the event matches this shortcut.
func (s Shortcut) matches(e *Event) bool {
	return s.Key != KeyInvalid &&
		e.KeyCode == s.Key && e.Modifiers == s.Modifiers
}

// String returns a human-readable shortcut label.
// Uses macOS glyphs on darwin, text labels elsewhere.
func (s Shortcut) String() string {
	if s.Key == KeyInvalid {
		return ""
	}
	buf := make([]byte, 0, 16)
	if runtime.GOOS == "darwin" {
		buf = shortcutStringDarwin(s.Modifiers, buf)
	} else {
		buf = shortcutStringOther(s.Modifiers, buf)
	}
	buf = append(buf, keyName(s.Key)...)
	return string(buf)
}

func shortcutStringDarwin(m Modifier, buf []byte) []byte {
	if m.Has(ModCtrl) {
		buf = append(buf, "⌃"...)
	}
	if m.Has(ModAlt) {
		buf = append(buf, "⌥"...)
	}
	if m.Has(ModShift) {
		buf = append(buf, "⇧"...)
	}
	if m.Has(ModSuper) {
		buf = append(buf, "⌘"...)
	}
	return buf
}

func shortcutStringOther(m Modifier, buf []byte) []byte {
	if m.Has(ModCtrl) {
		buf = append(buf, "Ctrl+"...)
	}
	if m.Has(ModAlt) {
		buf = append(buf, "Alt+"...)
	}
	if m.Has(ModShift) {
		buf = append(buf, "Shift+"...)
	}
	if m.Has(ModSuper) {
		buf = append(buf, "Super+"...)
	}
	return buf
}

// keyNameMap maps individual key codes to display names.
var keyNameMap = map[KeyCode]string{
	KeySpace:        "Space",
	KeyEnter:        "Enter",
	KeyTab:          "Tab",
	KeyBackspace:    "Backspace",
	KeyDelete:       "Del",
	KeyInsert:       "Insert",
	KeyEscape:       "Esc",
	KeyUp:           "Up",
	KeyDown:         "Down",
	KeyLeft:         "Left",
	KeyRight:        "Right",
	KeyHome:         "Home",
	KeyEnd:          "End",
	KeyPageUp:       "PgUp",
	KeyPageDown:     "PgDn",
	KeyMinus:        "-",
	KeyEqual:        "=",
	KeyComma:        ",",
	KeyPeriod:       ".",
	KeySlash:        "/",
	KeyBackslash:    "\\",
	KeyLeftBracket:  "[",
	KeyRightBracket: "]",
	KeyApostrophe:   "'",
	KeySemicolon:    ";",
	KeyGraveAccent:  "`",
}

// keyName returns a display name for a key code.
func keyName(k KeyCode) string {
	switch {
	case k >= KeyA && k <= KeyZ:
		return string(rune('A' + (k - KeyA)))
	case k >= Key0 && k <= Key9:
		return string(rune('0' + (k - Key0)))
	case k >= KeyF1 && k <= KeyF25:
		n := int(k - KeyF1 + 1)
		return "F" + strconv.Itoa(n)
	case k >= KeyKP0 && k <= KeyKP9:
		return "KP" + string(rune('0'+(k-KeyKP0)))
	}
	if s, ok := keyNameMap[k]; ok {
		return s
	}
	return "?"
}

// Command bundles an action with its identity, shortcut,
// and enable/disable logic.
type Command struct {
	Execute    func(*Event, *Window)
	CanExecute func(*Window) bool // nil = always enabled
	ID         string
	Label      string
	Icon       string
	Group      string
	Shortcut   Shortcut
	Global     bool // fires before focus dispatch
}

// RegisterCommand adds a command to the window registry.
// Returns an error on duplicate ID or duplicate shortcut.
func (w *Window) RegisterCommand(cmd Command) error {
	for i := range w.cmdRegistry {
		if w.cmdRegistry[i].ID == cmd.ID {
			return errors.New(
				"gui: duplicate command ID: " + cmd.ID)
		}
		if cmd.Shortcut.IsSet() &&
			w.cmdRegistry[i].Shortcut == cmd.Shortcut {
			return errors.New(
				"gui: duplicate shortcut for commands: " +
					w.cmdRegistry[i].ID + " and " + cmd.ID)
		}
	}
	w.cmdRegistry = append(w.cmdRegistry, cmd)
	return nil
}

// RegisterCommands adds multiple commands. Stops and returns
// the first error encountered.
func (w *Window) RegisterCommands(cmds ...Command) error {
	for _, cmd := range cmds {
		if err := w.RegisterCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

// UnregisterCommand removes a command by ID. No-op if
// not found.
func (w *Window) UnregisterCommand(id string) {
	for i := range w.cmdRegistry {
		if w.cmdRegistry[i].ID == id {
			w.cmdRegistry = append(
				w.cmdRegistry[:i], w.cmdRegistry[i+1:]...)
			return
		}
	}
}

// CommandByID returns a registered command by ID.
func (w *Window) CommandByID(id string) (Command, bool) {
	for i := range w.cmdRegistry {
		if w.cmdRegistry[i].ID == id {
			return w.cmdRegistry[i], true
		}
	}
	return Command{}, false
}

// CommandCanExecute checks if a command's CanExecute
// returns true (nil CanExecute = always true).
func (w *Window) CommandCanExecute(id string) bool {
	cmd, ok := w.CommandByID(id)
	if !ok {
		return false
	}
	if cmd.CanExecute == nil {
		return true
	}
	return cmd.CanExecute(w)
}

// CommandPaletteItems returns palette items from
// registered commands. Excludes commands with empty Label.
func (w *Window) CommandPaletteItems() []CommandPaletteItem {
	items := make([]CommandPaletteItem, 0, len(w.cmdRegistry))
	for i := range w.cmdRegistry {
		cmd := &w.cmdRegistry[i]
		if cmd.Label == "" {
			continue
		}
		disabled := cmd.CanExecute != nil && !cmd.CanExecute(w)
		items = append(items, CommandPaletteItem{
			ID:       cmd.ID,
			Label:    cmd.Label,
			Detail:   cmd.Shortcut.String(),
			Icon:     cmd.Icon,
			Group:    cmd.Group,
			Disabled: disabled,
		})
	}
	return items
}

// commandDispatch scans commands matching the event's
// key+modifiers. If globalOnly is true, only Global=true
// commands are checked; if false, only Global=false.
// Returns true if a command was executed.
func (w *Window) commandDispatch(
	e *Event, globalOnly bool,
) bool {
	for i := range w.cmdRegistry {
		cmd := &w.cmdRegistry[i]
		if cmd.Global != globalOnly {
			continue
		}
		if !cmd.Shortcut.IsSet() {
			continue
		}
		if !cmd.Shortcut.matches(e) {
			continue
		}
		if cmd.CanExecute != nil && !cmd.CanExecute(w) {
			continue
		}
		if cmd.Execute != nil {
			cmd.Execute(e, w)
		}
		e.IsHandled = true
		return true
	}
	return false
}
