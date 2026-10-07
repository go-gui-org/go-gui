# Spec: key binding mode

Issue: #969. Status: **implemented** (unreleased).

## Problem

Text widgets treated Ctrl and Cmd as one modifier on every platform. On macOS,
Ctrl+A selected all text, but a native Cocoa text field moves the caret to the
start of the line. The other Cocoa Emacs keys did nothing. On Linux and Windows,
Super+A also selected all text, but only Ctrl must do this.

## Decision

One process-wide `KeyBindingMode` (`gui/key_binding.go`) with two values:

| Mode                | Platforms                    | Shortcuts | Word move      | Extra keys                                |
| ------------------- | ---------------------------- | --------- | -------------- | ----------------------------------------- |
| `KeyBindingCommand` | macOS, iOS, web on Apple     | Cmd       | Option+Arrow   | Cmd+Arrow line/doc edge; Cocoa Emacs keys |
| `KeyBindingControl` | Linux, Windows, Android, web | Ctrl      | Ctrl/Alt+Arrow | none                                      |

- The mode comes from `runtime.GOOS` at startup.
- The web backend runs with `GOOS=js`. It reads `navigator.userAgentData` or
  `navigator.platform` and calls `SetKeyBindingMode`
  (`gui/backend/web/key_binding.go`).
- `GOGUI_KEY_BINDING_MODE=command|control` overrides both. When it is set,
  `SetKeyBindingMode` does nothing. Use it to try the macOS keys on Linux.
- `ShortcutModifier()` returns `ModSuper` or `ModCtrl`. DataGrid uses it, and
  apps can use it.
- Inside `gui`, every call site uses `isShortcut`, `isWordMod`, `isLineMod` or
  `resolveTextKey`. A plain `HasAny(ModCtrl, ModSuper)` is the bug this spec
  fixes. Do not add it again.

The Cocoa Emacs keys are active in Command mode only, and only when Ctrl is the
one keyboard modifier held:

| Key        | Action                                    | Widgets          |
| ---------- | ----------------------------------------- | ---------------- |
| Ctrl+A / E | Go to paragraph start / end (no cycling)  | Input, Text, RTF |
| Ctrl+F / B | Move caret right / left                   | Input, Text, RTF |
| Ctrl+N / P | Move caret down / up                      | Input, Text, RTF |
| Ctrl+D / H | Delete forward / back                     | Input            |
| Ctrl+K     | Cut to paragraph end into the kill buffer | Input            |
| Ctrl+Y     | Paste the kill buffer                     | Input            |

The kill buffer is a field on `Window`, apart from the clipboard, as in Cocoa.
In a password or masked field, Ctrl+K deletes and clears the kill buffer. A
masked delete keeps the literals, so the cut range is not what was removed.

Cmd+Left/Right stop at the line edge. They do not cycle on to the paragraph and
text edge, as Home/End do.

## Rejected Approaches

- **Unexported mode only, from `runtime.GOOS`.** The web build cannot see the
  browser's OS, so Mac browser users would get Ctrl shortcuts.
- **A per-window field (`WindowCfg.KeyBindings`).** The binding is a fact about
  the platform, not one window. Two windows with different bindings is a bug.
- **A new method on `NativePlatform`.** Every backend that implements the
  interface breaks, to carry one enum.
- **Remapping `ModSuper` to `ModCtrl` in the backends.** It hides the real
  modifier from apps and breaks `Shortcut` matching.
- **A `ControlEmacs` mode (GTK Emacs key theme), as in guigui.** A niche Linux
  user setting, not a platform default. Add it later if users ask; it does not
  break the API.
- **Copying guigui's `keybindingmode.go`.** guigui is Apache-2.0. Only the idea
  is used, so go-gui stays MIT only.

## Not in scope

- `Shortcut.String()` still uses `runtime.GOOS`, so menu labels on web show Ctrl
  names on a Mac.
- DataGrid copy listens for a Ctrl+C character event (char code 3). The macOS
  backend sends no character event for a Cmd chord, so DataGrid copy does not
  work on macOS. Separate issue.
