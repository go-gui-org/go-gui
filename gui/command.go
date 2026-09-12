package gui

import (
	"errors"
	"fmt"
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
// Nil-safe: a nil event never matches.
func (s Shortcut) matches(e *Event) bool {
	if e == nil {
		return false
	}
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
// Read-only after init; never mutate (concurrent reads).
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
//
// Execute may receive a nil *Event when invoked off the key path
// (native menubar, command palette), so it must nil-check e
// before reading key fields. CanExecute must be pure and fast:
// it runs on every key press and palette build, possibly twice
// per frame, so it must not block or mutate state. Both run
// synchronously on the event path; a panic propagates, like
// every other event handler, and is not recovered.
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
// Returns an error on empty ID, duplicate ID, or duplicate
// shortcut. Safe for concurrent use with dispatch; user
// callbacks run without the registry lock held.
func (w *Window) RegisterCommand(cmd Command) error {
	if cmd.ID == "" {
		return errors.New("gui: command ID must not be empty")
	}
	w.cmdMu.Lock()
	defer w.cmdMu.Unlock()
	for i := range w.cmdRegistry {
		if w.cmdRegistry[i].ID == cmd.ID {
			return fmt.Errorf("gui: duplicate command ID: %q", cmd.ID)
		}
		if cmd.Shortcut.IsSet() &&
			w.cmdRegistry[i].Shortcut == cmd.Shortcut {
			return fmt.Errorf(
				"gui: duplicate shortcut for commands: %q and %q",
				w.cmdRegistry[i].ID, cmd.ID)
		}
	}
	w.cmdRegistry = append(w.cmdRegistry, cmd)
	return nil
}

// RegisterCommands adds multiple commands atomically: either all
// are registered or none are. Returns the first validation error
// without mutating the registry.
func (w *Window) RegisterCommands(cmds ...Command) error {
	if len(cmds) == 0 {
		return nil
	}
	w.cmdMu.Lock()
	defer w.cmdMu.Unlock()
	// Index the existing registry once so validation is linear
	// in len(cmds) rather than quadratic: a large batch against
	// a large registry must not cost O(batch*registry).
	existingIDs := make(map[string]struct{}, len(w.cmdRegistry)+len(cmds))
	existingShortcuts := make(map[Shortcut]string, len(w.cmdRegistry)+len(cmds))
	for i := range w.cmdRegistry {
		existing := &w.cmdRegistry[i]
		existingIDs[existing.ID] = struct{}{}
		if existing.Shortcut.IsSet() {
			existingShortcuts[existing.Shortcut] = existing.ID
		}
	}
	for _, cmd := range cmds {
		if cmd.ID == "" {
			return errors.New("gui: command ID must not be empty")
		}
		if _, dup := existingIDs[cmd.ID]; dup {
			return fmt.Errorf("gui: duplicate command ID: %q", cmd.ID)
		}
		existingIDs[cmd.ID] = struct{}{}
		if cmd.Shortcut.IsSet() {
			if prev, dup := existingShortcuts[cmd.Shortcut]; dup {
				return fmt.Errorf(
					"gui: duplicate shortcut for commands: %q and %q",
					prev, cmd.ID)
			}
			existingShortcuts[cmd.Shortcut] = cmd.ID
		}
	}
	w.cmdRegistry = append(w.cmdRegistry, cmds...)
	return nil
}

// UnregisterCommand removes a command by ID. No-op if
// not found.
func (w *Window) UnregisterCommand(id string) {
	w.cmdMu.Lock()
	defer w.cmdMu.Unlock()
	for i := range w.cmdRegistry {
		if w.cmdRegistry[i].ID == id {
			copy(w.cmdRegistry[i:], w.cmdRegistry[i+1:])
			// Clear the orphaned tail so its closures can be
			// collected rather than lingering in spare capacity.
			w.cmdRegistry[len(w.cmdRegistry)-1] = Command{}
			w.cmdRegistry = w.cmdRegistry[:len(w.cmdRegistry)-1]
			return
		}
	}
}

// canExecute reports whether the command is currently enabled.
// Nil CanExecute means always enabled.
func (c *Command) canExecute(w *Window) bool {
	return c.CanExecute == nil || c.CanExecute(w)
}

// snapshotCommands copies the registry so user callbacks
// (CanExecute/Execute) run without the registry lock held.
func (w *Window) snapshotCommands() []Command {
	w.cmdMu.RLock()
	defer w.cmdMu.RUnlock()
	snapshot := make([]Command, len(w.cmdRegistry))
	copy(snapshot, w.cmdRegistry)
	return snapshot
}

// CommandByID returns a registered command by ID. The returned
// value is a copy; mutating it does not update the registry.
func (w *Window) CommandByID(id string) (Command, bool) {
	w.cmdMu.RLock()
	defer w.cmdMu.RUnlock()
	for i := range w.cmdRegistry {
		if w.cmdRegistry[i].ID == id {
			return w.cmdRegistry[i], true
		}
	}
	return Command{}, false
}

// CommandCanExecute checks if a command's CanExecute
// returns true (nil CanExecute = always true).
func (w *Window) commandCanExecute(id string) bool {
	cmd, ok := w.CommandByID(id)
	if !ok {
		return false
	}
	return cmd.canExecute(w)
}

// CommandPaletteItems returns palette items from
// registered commands. Excludes commands with empty Label.
// Snapshots the registry so CanExecute callbacks run without
// the registry lock held.
func (w *Window) CommandPaletteItems() []CommandPaletteItem {
	snapshot := w.snapshotCommands()
	items := make([]CommandPaletteItem, 0, len(snapshot))
	for i := range snapshot {
		cmd := &snapshot[i]
		if cmd.Label == "" {
			continue
		}
		// Read CanExecute from the snapshot, not via a second
		// registry lookup: the entry may have been unregistered
		// after the snapshot, and a per-item lookup is O(n^2).
		disabled := !cmd.canExecute(w)
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
// Returns true if a command was executed. Nil events never
// match. Commands with nil Execute never consume the event,
// so an action-less binding cannot swallow a keypress.
// The registry is snapshotted so Execute/CanExecute run
// without the lock held (they may register commands).
func (w *Window) commandDispatch(
	e *Event, globalOnly bool,
) bool {
	if e == nil {
		return false
	}
	snapshot := w.snapshotCommands()
	if len(snapshot) == 0 {
		return false
	}
	for i := range snapshot {
		cmd := &snapshot[i]
		if cmd.Global != globalOnly {
			continue
		}
		if !cmd.Shortcut.IsSet() {
			continue
		}
		if !cmd.Shortcut.matches(e) {
			continue
		}
		if !cmd.canExecute(w) {
			continue
		}
		if cmd.Execute == nil {
			continue
		}
		cmd.Execute(e, w)
		e.IsHandled = true
		return true
	}
	return false
}
