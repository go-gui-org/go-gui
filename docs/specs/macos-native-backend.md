# Native macOS backend (SDL2 elimination)

Status: **implemented** — SDL2 eliminated from macOS (`bead449`, #11): sdl2 is
gone from go.mod and the Metal backend speaks Cocoa/AppKit directly. macOS stays
cgo by decision (2026-08-12); see `cgo-free-backend-feasibility.md` § Phase 2.

## Summary

Replace SDL2's window creation, event loop, clipboard, cursors, and IME in the
macOS Metal backend with direct Cocoa/AppKit calls. The existing Metal rendering
pipeline (`metal_darwin.m`, ~800 lines) is unchanged — it already takes a raw
`CAMetalLayer*` and has zero SDL dependency. All existing native feature bridges
(file drops, scroll phases, accessibility, dock icon, menus, print dialog,
spellcheck) are already SDL-free — they use system frameworks directly.

Net result: zero SDL on macOS. `go build` works with no
Homebrew/pkg-config/MSYS2.

## Motivation

SDL2 is the #1 friction point for new users (issue #10). It requires:

| Platform | SDL2 install                                             |
| -------- | -------------------------------------------------------- |
| macOS    | `brew install sdl2`                                      |
| Windows  | MSYS2/MinGW (recently pinned to stable release, v0.27.1) |
| Linux    | `apt install libsdl2-dev` or equivalent                  |

The shim approach (sdl3-shim.md) eliminates MSYS2 by swapping to zig-built SDL3,
but keeps a C build-toolchain dependency (zig). A native macOS backend
eliminates ALL build-toolchain C dependencies on macOS. System framework CGo
(Metal, AppKit, CoreFoundation) requires zero installs — they ship with macOS.

SDL3 is the current SDL stable, so the shim is the right answer for Linux and
eventually Windows. But for macOS specifically, the Metal backend already does
native rendering — SDL is only providing window + events + clipboard + cursors

- IME. All of these have direct AppKit equivalents with smaller API surfaces.

## Architecture

```
Before:                              After:
┌──────────────┐                    ┌──────────────┐
│  backend.go  │                    │  backend.go  │
│  events.go   │                    │  events.go   │  (rewritten)
│  (SDL2)      │                    │  (NSEvent)   │
├──────────────┤                    ├──────────────┤
│  SDL window  │                    │  NSWindow +  │
│  + Metal     │                    │  CAMetalLayer│
│  view        │                    │              │
├──────────────┤                    ├──────────────┤
│metal_darwin.m│ ← (unchanged)      │metal_darwin.m│ ← (unchanged)
│  GPU render  │                    │  GPU render  │
├──────────────┤                    ├──────────────┤
│  filedrop.m  │ ← (simplified)     │  filedrop.m  │ ← (no SDL reach-through)
│  scroll.m    │ ← (simplified)     │  scroll.m    │ ← (no SDL reach-through)
│  a11y.m      │ ← (simplified)     │  a11y.m      │ ← (no SDL reach-through)
│  icon.m      │ ← (unchanged)      │  icon.m      │ ← (unchanged)
└──────────────┘                    └──────────────┘
```

## What SDL provides today (and its native replacement)

| Capability      | SDL2 API                                             | Cocoa replacement                                                  | Lines  |
| --------------- | ---------------------------------------------------- | ------------------------------------------------------------------ | ------ |
| Window creation | `sdl.CreateWindow(WINDOW_METAL)`                     | `NSWindow` + `MetalContentView` with `CAMetalLayer`                | ~150   |
| Window title    | `win.SetTitle()`                                     | `[nsWindow setTitle:]`                                             | 1 line |
| Window size     | `win.GetSize()`                                      | `[nsWindow frame].size`                                            | 1 line |
| Event pump      | `sdl.WaitEventTimeout` / `sdl.PollEvent`             | `[NSApp nextEventMatchingMask:…]`                                  | ~80    |
| Mouse events    | `sdl.MouseButtonEvent`, etc.                         | `NSEventTypeLeftMouseDown`, etc.                                   | ~100   |
| Keyboard events | `sdl.KeyboardEvent`                                  | `NSEventTypeKeyDown` + key code mapping                            | ~80    |
| Scroll events   | `sdl.MouseWheelEvent`                                | `NSEventTypeScrollWheel`                                           | ~30    |
| Window events   | `sdl.WindowEvent` (resize, focus, close)             | `NSWindowDelegate` callbacks                                       | ~50    |
| Touch events    | `sdl.FingerEvent`                                    | `NSTouch` via touch tracking                                       | ~60    |
| Text input      | `sdl.TextInputEvent` / `sdl.TextEditingEvent`        | `NSTextInputClient` protocol                                       | ~120   |
| Clipboard get   | `sdl.GetClipboardText()`                             | `[NSPasteboard generalPasteboard] stringForType:`                  | ~5     |
| Clipboard set   | `sdl.SetClipboardText()`                             | `[NSPasteboard generalPasteboard] clearContents` + `setString:`    | ~8     |
| Cursors         | `[11]*sdl.Cursor` with `CreateSystemCursor`          | `[NSCursor arrowCursor]` etc.                                      | ~15    |
| Live resize     | `sdl.AddEventWatchFunc` + `WINDOWEVENT_SIZE_CHANGED` | `NSWindowDelegate windowDidResize:`                                | ~20    |
| App quit        | `sdl.QuitEvent`                                      | `NSApplicationDelegate applicationShouldTerminate:`                | ~8     |
| Wake from timer | `sdl.RegisterEvents` + `sdl.PushEvent(UserEvent)`    | `dispatch_async(dispatch_get_main_queue(), …)` or CFRunLoop source | ~15    |

## Files changed

### New files

```
gui/backend/metal/metal_window.h       (~40 lines)
gui/backend/metal/metal_window.m       (~200 lines)
```

`metal_window.h` exposes:

```c
// Opaque handle to a GoGuiWindow + its CAMetalLayer.
typedef void* GoGuiNSWindow;

// Create a window. Returns nil on failure.
GoGuiNSWindow metalWindowCreate(const char *title, int width, int height,
                                int fixedSize);

// Window properties.
void metalWindowSetTitle(GoGuiNSWindow w, const char *title);
void metalWindowGetSize(GoGuiNSWindow w, int *width, int *height);
void metalWindowGetFramebufferSize(GoGuiNSWindow w, int *w, int *h);

// Metal layer accessor for the rendering pipeline.
void *metalWindowGetLayer(GoGuiNSWindow w);

// Window ID for multi-window tracking (matches gui.App registration).
unsigned int metalWindowGetID(GoGuiNSWindow w);

// Close and destroy.
void metalWindowDestroy(GoGuiNSWindow w);
```

`metal_window.m` implements:

- `GoGuiNSWindow` → wraps `NSWindow*` + `MetalContentView*`
- `MetalContentView` → `NSView` subclass with `CAMetalLayer` backing, implements
  `NSTextInputClient` for IME
- Window delegate for resize, focus, close callbacks

### Modified files

```
gui/backend/metal/backend.go           (~300 changed, ~200 deleted)
gui/backend/metal/events.go            (~200 new, ~250 deleted)
gui/backend/metal/native_platform.go   (~50 new, ~20 deleted)
gui/backend/metal/text.go              (~15 new IME methods)
gui/backend/metal/a11y_darwin.h        (~4 lines: SDL_Window → GoGuiNSWindow)
gui/backend/metal/a11y_darwin.m        (~10 lines: drop SDL_GetWindowWMInfo)
gui/backend/metal/filedrop_darwin.h    (~4 lines: SDL_Window → GoGuiNSWindow)
gui/backend/metal/filedrop_darwin.m    (~10 lines: drop SDL_GetWindowWMInfo)
gui/backend/metal/scroll_phase_darwin.h (~4 lines: SDL_Window → GoGuiNSWindow)
gui/backend/metal/scroll_phase_darwin.m (~10 lines: drop SDL_GetWindowWMInfo)
```

### Unchanged files

```
gui/backend/metal/metal_darwin.{h,m}   — GPU rendering (no SDL used)
gui/backend/metal/icon.go              — dock icon (no SDL used)
gui/backend/metal/icon_darwin.m        — dock icon (pure Cocoa)
gui/backend/metal/draw.go             — render command dispatch
gui/backend/metal/textures.go         — texture cache
gui/backend/metal/rotation.go         — screen rotation
```

### Deleted files

None. The Metal backend is gated by `//go:build darwin && !ios`. All SDL2 and GL
backend files continue to serve Linux and Windows. `gui/compat_mingw.go` stays —
it's gated `//go:build windows && cgo` and remains needed for SDL2 on Windows.

### Shared files (no impact)

No shared Go files are modified. The changes are entirely within
`gui/backend/metal/` (Darwin-only build tag) plus the two new `.h/.m` files.
Linux and Windows compilation paths are untouched — `go build` on Linux
continues to select the SDL2 or GL backend as before.

## Design details

### 1. Event loop

The SDL `WaitEventTimeout` / `PollEvent` pattern is replaced with manual NSEvent
polling. This mirrors what SDL does internally on macOS and gives Go full
control over the loop timing.

```go
// backend.go
func (b *Backend) Run(w *gui.Window) {
    // ... setup ...

    for running {
        for ev := pollNSEvent(waitMs); ev != nil; ev = pollNSEvent(0) {
            mapped, cont := mapNSEvent(ev, b)
            if !cont {
                running = false
                break
            }
            if mapped.Type != gui.EventInvalid {
                w.EventFn(mapped)
            }
        }
        // frame + render (unchanged from current)
    }
}
```

`pollNSEvent` in the `.m` file delegates to `[NSApp nextEventMatchingMask:…]`.
Events are sent to `[NSApp sendEvent:]` first so AppKit handles window
management (close button, miniaturize, key window, main window). Go callbacks
fire from the window delegate during `sendEvent:`.

The wake mechanism replaces `sdl.PushEvent(&sdl.UserEvent{…})` with
`dispatch_async(dispatch_get_main_queue(), ^{ [NSApp postEvent:… atStart:NO] })`
— or more simply, a CFRunLoop source that Go signals.

### 2. Event mapping

`events.go` is rewritten. The mapping NSEvent → `gui.Event` is simpler than the
SDL path because NSEvent already carries more structured data (e.g., button
number, click count, pressure, precise scrolling deltas):

```go
func mapNSEvent(event C.NSEventRef, b *Backend) (gui.Event, bool) {
    switch C.nsEventType(event) {
    case C.NSEventTypeLeftMouseDown:
        return gui.Event{
            Type:        gui.EventMouseDown,
            MouseButton: gui.MouseButtonLeft,
            MouseX:      float32(C.nsEventX(event)),
            MouseY:      float32(C.nsEventY(event)),
            Modifiers:   mapNSModifiers(C.nsEventModifiers(event)),
        }, true
    // ... etc
    }
}
```

A thin C layer in `metal_window.m` extracts fields from NSEvent (location in
window, button number, modifier flags, etc.) as simple C functions callable from
cgo. This avoids exposing NSEvent's ObjC interface to Go.

Key mappings:

| NSEvent                                                 | gui.Event                                    |
| ------------------------------------------------------- | -------------------------------------------- |
| `NSEventTypeLeftMouseDown/Up`                           | `EventMouseDown/Up` with `MouseButtonLeft`   |
| `NSEventTypeRightMouseDown/Up`                          | `EventMouseDown/Up` with `MouseButtonRight`  |
| `NSEventTypeOtherMouseDown/Up`                          | `EventMouseDown/Up` with `MouseButtonMiddle` |
| `NSEventTypeMouseMoved` / `NSEventTypeLeftMouseDragged` | `EventMouseMove`                             |
| `NSEventTypeScrollWheel`                                | `EventMouseScroll`                           |
| `NSEventTypeKeyDown/Up`                                 | `EventKeyDown/Up` (with key code mapping)    |
| `NSEventTypeFlagsChanged`                               | Modifier-only key events                     |
| Window delegate `windowDidResize:`                      | `EventResized`                               |
| Window delegate `windowDidBecomeKey:`                   | `EventFocused`                               |
| Window delegate `windowDidResignKey:`                   | `EventUnfocused`                             |
| `NSTextInputClient insertText:`                         | `EventChar`                                  |
| `NSTextInputClient setMarkedText:`                      | `EventIMEComposition`                        |

Key code mapping replaces `sdlkey.MapKeyCode(e.Keysym.Sym)`. macOS key codes
(`e.keyCode`) are hardware-scancode-based, similar to SDL scancodes. A new
`gui/backend/internal/nskey` mapping table (or inlined in events.go for the
first iteration) translates `UInt16` key codes to `gui.KeyCode`.

Where SDL uses string-based key symbols (`SDLK_a`), NSEvent uses virtual key
codes. The `gui.KeyCode` constants already map to logical keys (letters,
function keys, arrows) — the NSEvent translation is a direct table lookup:

```go
var nsKeyToGui = [128]gui.KeyCode{
    kVK_ANSI_A: gui.KeyA,
    kVK_ANSI_B: gui.KeyB,
    // ...
    kVK_Return:   gui.KeyEnter,
    kVK_Escape:   gui.KeyEscape,
    kVK_Space:    gui.KeySpace,
    kVK_LeftArrow:  gui.KeyLeft,
    kVK_RightArrow: gui.KeyRight,
    kVK_UpArrow:    gui.KeyUp,
    kVK_DownArrow:  gui.KeyDown,
}
```

### 3. Cursors

SDL creates 11 `*sdl.Cursor` objects, one for each `gui.MouseCursor` value.
NSCursor has class methods for all standard shapes — no pre-creation needed:

```go
var nsCursorSelectors = [11]string{
    gui.CursorDefault:      "arrowCursor",
    gui.CursorArrow:        "arrowCursor",
    gui.CursorIBeam:        "IBeamCursor",
    gui.CursorCrosshair:    "crosshairCursor",
    gui.CursorPointingHand: "pointingHandCursor",
    gui.CursorResizeEW:     "resizeLeftRightCursor",
    gui.CursorResizeNS:     "resizeUpDownCursor",
    gui.CursorResizeNWSE:   "_windowResizeNorthWestSouthEastCursor",
    gui.CursorResizeNESW:   "_windowResizeNorthEastSouthWestCursor",
    gui.CursorResizeAll:    "closedHandCursor",
    gui.CursorNotAllowed:   "operationNotAllowedCursor",
}
```

Usage: `[[NSCursor className] performSelector:NSSelectorFromString(name)]`. This
avoids pre-defining all 11 cursors. The cursor is set once per frame in the
event loop (existing pattern unchanged). The `Backend.cursors` array field is
removed.

Note: `_windowResizeNorthWestSouthEastCursor` is a private selector. If App
Store submission is ever a goal, replace with
`[[NSCursor alloc] initWithImage: hotSpot:]` from bundled PNG. Not needed now.

### 4. Clipboard

Three lines of ObjC each direction:

```c
// metal_window.m
const char *metalClipboardGet(void) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSString *s = [pb stringForType:NSPasteboardTypeString];
    return s ? strdup([s UTF8String]) : NULL;
}

void metalClipboardSet(const char *text) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    [pb setString:[NSString stringWithUTF8String:text]
          forType:NSPasteboardTypeString];
}
```

Go side calls these via cgo. The returned string from `metalClipboardGet` is
freed by Go via `C.free()`.

### 5. IME (text input)

The most substantial new piece. `MetalContentView` implements
`NSTextInputClient`. The Go side sets cursor position / composition state via
simple C setters:

```c
// IME state (in metal_window.h)
void metalWindowIMESetCursorRect(GoGuiNSWindow w, float x, float y, float w, float h);
void metalWindowIMESetActive(GoGuiNSWindow w, int active);
```

When IME is active and the user types, `NSTextInputClient insertText:` fires,
which calls a Go callback. `setMarkedText:` fires for in-progress compositions
(Chinese, Japanese, Korean). Both are bridged to `gui.EventChar` and
`gui.EventIMEComposition` respectively, matching the current SDL behavior.

The `NSTextInputClient` implementation in `MetalContentView`:

```objc
@interface MetalContentView : NSView <NSTextInputClient>
@property (nonatomic) NSRange markedRange;
@property (nonatomic) NSMutableAttributedString *markedText;
@property (nonatomic) NSRect imeCursorRect;  // set from Go
@end
```

`firstRectForCharacterRange:` returns `imeCursorRect` converted to screen
coordinates. `insertText:` and `setMarkedText:` call the Go callbacks.

### 6. Filedrop, scroll phases, accessibility

These three bridges currently use `SDL_GetWindowWMInfo` to extract the NSWindow
from SDL's window handle. With a native NSWindow, the indirection disappears:

**Before:**

```c
SDL_SysWMinfo info;
SDL_GetWindowWMInfo(win, &info);
NSWindow *nsWin = info.info.cocoa.window;
```

**After:**

```c
NSWindow *nsWin = ((GoGuiWindow *)win)->nsWindow;
```

The `GoGuiWindow` struct is defined in the `.m` file:

```objc
typedef struct {
    NSWindow           *nsWindow;
    MetalContentView   *contentView;
    uint32_t            windowID;
    // Go callback function pointers
    void (*onResize)(uint32_t windowID, int width, int height);
    void (*onClose)(uint32_t windowID);
    void (*onFocus)(uint32_t windowID, int gained);
} GoGuiWindow;
```

The callback function pointers are set by Go via cgo and called from the
NSWindowDelegate. This eliminates the `goFileDrop(SDL_Window*, char*)` pattern —
all bridges use the same `GoGuiWindow` struct and its callbacks.

### 7. Multi-window support

The existing `RunApp` / `windowState` pattern maps cleanly. Each `GoGuiWindow`
has a `windowID` that `gui.App` uses for registration. `NSApp` natively manages
multiple windows — no SDL hacks needed.

The `onClose` callback fires for each window. `NSApp terminate:` only fires when
the last window closes (configurable via
`NSWindowDelegate applicationShouldTerminateAfterLastWindowClosed:`).

## Concerns

### Key code mapping correctness

SDL's `SDLK_*` values differ from macOS virtual key codes. The mapping table
(`nsKeyToGui`) must be exhaustively verified against all keys go-gui supports:
letters, digits, function keys (F1–F12), navigation (Home, End, PgUp, PgDn),
modifiers, media keys, numpad.

Testing strategy: write a key-event echo example that displays key codes, then
run it side-by-side with the SDL backend's keyboard output. Diff until zero.

### IME edge cases

`NSTextInputClient` has subtle behavior around marked ranges, insertion points,
and dead keys. The current SDL `TextEditingEvent` path handles these but through
SDL's abstraction. Native IME needs:

- Correct `firstRectForCharacterRange:` for CJK candidate windows
- `attributedSubstringForProposedRange:` for inline composition
- Dead key sequences (Option+E then E → é)

The existing `gui.EventIMEComposition` with `IMEStart`/`IMELength` mirrors
`-[NSTextInputClient markedRange]`, so the data model is compatible.

### Touch/trackpad events

SDL provides `FINGERDOWN`/`FINGERMOTION`/`FINGERUP` touch events. macOS exposes
touch via `NSTouch` on `NSTouchBar`-capable devices and via `NSEventTypeGesture`
for trackpad gestures. Direct `NSTouch` access requires `allowedTouchTypes` on
the NSView. Multi-touch (e.g., for a drawing canvas) is usable but maps
differently than SDL's finger abstraction.

For the initial implementation, trackpad scroll (already handled via
`NSEventTypeScrollWheel`) and mouse events cover the primary use cases. Full
touch support can follow.

### Dark mode / appearance

`TitlebarDark(bool)` in `native_platform.go` is currently a no-op under Metal.
The native backend can implement it via `NSAppearance`:

```objc
if (dark) {
    nsWindow.appearance = [NSAppearance appearanceNamed:NSAppearanceNameDarkAqua];
} else {
    nsWindow.appearance = nil; // system default
}
```

### Fullscreen

SDL has `WINDOW_FULLSCREEN` / `WINDOW_FULLSCREEN_DESKTOP` flags. NSWindow has
`toggleFullScreen:` and `NSWindowStyleMaskFullScreen`. The `WindowCfg`
fullscreen options map to these. Not needed for v0.28 but should be noted.

### CGo framework dependencies

All system frameworks in this design are CGo (`import "C"`) but require zero
build-toolchain installs:

```
-framework Metal
-framework QuartzCore
-framework AppKit
-framework Cocoa
-framework Foundation
```

These ship with macOS. `go build` works on a fresh macOS install with Xcode CLT
only.

### go-glyph CGo

go-glyph continues to use CoreText via CGo for font shaping and rasterization.
This is a system framework, not a build dependency. The goal is "no build
toolchain," not "zero CGo." Pure-Go text is a separate effort tracked in
go-glyph (see CLAUDE.md rejected WebGPU approach note).

### Test divergence: TestBackendRenderSmoke

The existing `TestBackendRenderSmoke` reads backbuffer after `Present()`. SDL3's
hardware-accelerated flip-model renderers can return empty pixels on this read.
Under the native Metal backend, the test either:

- Skipped on Metal (software-readback of Metal drawables is possible but slow),
  or
- Moved to a Metal-specific pixel-readback path (GPU→CPU texture copy).

Production blur/color-matrix filter paths render to offscreen textures and are
unaffected.

## Build comparison

|                 | Current (SDL2)            | Proposed (Native Metal)            |
| --------------- | ------------------------- | ---------------------------------- |
| C toolchain     | Homebrew sdl2             | None (Xcode CLT only)              |
| Window/events   | SDL2 via pkg-config       | Cocoa/AppKit via system frameworks |
| Rendering       | SDL Metal or direct Metal | Metal (unchanged)                  |
| Audio           | SDL_mixer                 | SDL_mixer (unchanged for now)      |
| Clipboard       | SDL2                      | NSPasteboard                       |
| Cursors         | SDL2                      | NSCursor                           |
| IME             | SDL2                      | NSTextInputClient                  |
| Text shaping    | go-glyph CoreText CGo     | go-glyph CoreText CGo (unchanged)  |
| `go build` deps | Homebrew                  | None                               |
| Static linking  | Partial (SDL dylib)       | Full (no external dylibs)          |

## Non-goals for v0.28

- Linux/Windows SDL elimination (they continue using SDL2 or the SDL3 shim)
- Audio backend replacement (SDL_mixer still used)
- go-glyph pure-Go text (separate project)
- GPU pixel readback for TestBackendRenderSmoke
- iOS backend (already has its own native UIKit/Metal backend)
- Android backend (already has its own native GLES backend)

## Implementation order

1. **`metal_window.h/.m`** — NSWindow + MetalContentView + CAMetalLayer
   creation, window delegate with callbacks, NSTextInputClient stub
2. **`backend.go` rewrite** — replace `New()` to use `metalWindowCreate`,
   replace `Run()` event loop with NSEvent polling, replace cursor/clipboard/IME
   glue
3. **`events.go` rewrite** — NSEvent → `gui.Event` mapping, key code translation
4. **`native_platform.go`** — IME delegates to `metalWindowIME*`, remove SDL
   syscalls
5. **Bridge updates** — `a11y_darwin`, `filedrop_darwin`, `scroll_phase_darwin`:
   replace `SDL_Window*` with `GoGuiNSWindow`
6. **Key code table validation** — side-by-side diff against SDL backend output
7. **CI update** — macOS CI drops `brew install sdl2`, runs `go build ./...` and
   `go test ./...` directly
8. **Documentation** — README docs/sdl-support.md updated with macOS "zero
   dependencies" note

## Related

- [sdl3-shim.md](sdl3-shim.md) — SDL3 shim evaluation (Linux/Windows path)
- [Issue #10](https://github.com/go-gui-org/go-gui/issues/10) — static
  compilation using Zig & SDL 3.4 (user feedback driving this)
