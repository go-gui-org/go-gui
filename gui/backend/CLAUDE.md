# Debugging native backends

## CGo boundary blindness

Reading source cannot show whether a value survives the C↔Go boundary. A
variable set in ObjC (e.g. `_evType`) and read in Go (`C.metalEventType()`)
looks correct on both sides and can still be lost in the crossing. When a bug
sits on this boundary:

- Add debug logging on **both** sides simultaneously in the first pass. stderr
  in C (`fprintf(stderr, ...); fflush(stderr)`), stderr in Go
  (`fmt.Fprintf(os.Stderr, ...)` or `log.Printf`).
- If the log on one side never appears, the value was lost or the code path was
  never reached.

## Platform knowledge asymmetry

Cocoa APIs interact with AppKit internals (event masks, tracking areas,
responder chain, run-loop modes) in ways the API docs do not show. Standard
desktop behaviors that "just work" in a normal Cocoa app require correct
participation in these systems. When platform-native behavior is broken
(cursors, menus, title bar, window management):

- Ask for a **minimal native test program** (10-30 lines of ObjC) before
  modifying any go-gui code. Compare against a known-working variant (e.g.
  `[NSApp run]` vs custom event loop, plain NSView vs CAMetalLayer,
  `NSEventMaskAny` vs explicit mask). The test programs in
  `gui/backend/metal/test_*.m` (untracked, build with
  `clang -fobjc-arc -framework AppKit -framework Metal -framework QuartzCore`)
  are the pattern to copy.
- Ask "what would a Cocoa developer check first?" — the answer is usually event
  masks, tracking areas, or run-loop configuration, not application-level cursor
  or menu logic.

## IME activation contract

`NativePlatform.IMEStart` / `IMEStop` mean "an **editable text** widget has
focus", not "something has focus". The gui layer decides this per frame from the
arranged tree (`gui/ime_context.go`); a backend must not widen it.

The reason is not cosmetic. An active input context routes keystrokes through
the platform's text-input machinery, which on macOS turns Option+printable into
a dead key and swallows the app's shortcut (issue #393); on Windows it attaches
the IMM context, and on web it moves DOM focus into a hidden `<input>`. A
preedit that appears with no edit context is a dead key nobody asked for and
must be discarded, or it composes into the next keystroke. See
`docs/specs/ime-text-context.md`.
