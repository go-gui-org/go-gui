# Per-window themes

Status: implemented. Issue #296.

## Problem

`Window.SetTheme` looked like a per-window API but delegated to package globals,
so every window in an `App` shared one theme. The global surface is large:
`guiTheme` plus about 30 `default*Style` mirror variables that `SetTheme`
rewrote in one pass.

Widget factories resolve their defaults when they are called, not when the
layout is generated. Roughly 220 read sites inside `gui/` and 200
`CurrentTheme()` calls in `examples/` read theme state this way.

## Rejected: resolve defaults during GenerateLayout

Issue #296 proposed reading `w.Theme()` inside `GenerateLayout`. Factories build
eagerly, so this needs each factory body wrapped in a `viewFunc` closure. That
adds one closure and one `Cfg` heap escape per widget per frame, against a
baseline of 5000 widgets in 4-5 ms, and rewrites about 220 call sites. The
observable result is the same as the design below.

## Design

The window owns the theme. The frame pass installs that theme into the existing
globals before anything reads them.

```
FrameFn
  installTheme()      <- window theme becomes the installed theme
  flushCommands()
  Update()
    view fn -> generateViewLayout -> layoutArrange -> renderLayout
  (backend clear color reads CurrentTheme here)
```

The globals stop being app state. They are a frame-scoped cache of the theme of
the window under generation.

### Why this is correct

Every window's frame pass runs on the one main OS thread. Both desktop backends
call `runtime.LockOSThread` in package `init` and drive all windows from a
single sequential loop (`gui/backend/metal/backend.go`,
`gui/backend/gl/runapp_x11.go`). Two windows can never generate layouts at the
same time, and no layout is generated off that thread. A factory-time read
therefore always resolves against the window under generation.

### Layers

| State                | Meaning                           | Written by                 |
| -------------------- | --------------------------------- | -------------------------- |
| `guiTheme` + mirrors | installed theme for this frame    | `applyTheme`, frame thread |
| `Window.theme`       | the window's own theme, if pinned | `(*Window).SetTheme`       |
| `defaultTheme`       | app default for unpinned windows  | `gui.SetTheme`             |

`(*Window).Theme()` returns the pinned theme, or the app default.

`gui.SetTheme` sets the app default and requests a rebuild on every window that
has not pinned one, so existing code that calls it from `main` or from a handler
keeps working. Both setters also install eagerly, so callers outside a frame
pass (tests, `main` before `Run`) see the change at once. The next frame
re-establishes the correct per-window theme regardless.

### Theme identity

`Theme.id` is stamped by `ThemeMaker` and re-stamped by every `with*Style`
helper. `installTheme` compares ids and returns without writing when the theme
is already installed, which is the steady state for a single-window app. Zero
means "built outside `ThemeMaker`" and forces a re-install rather than a wrong
fast-path hit.

The per-window text-layout caches (`viewState.rtfLayoutTheme`,
`viewState.markdownTheme`) key on `Theme.id` for the same reason: two themes can
share a `Name` and differ in text styles.

## Scoped subtree themes

`Themed(t, build)` installs `t` for the duration of `build`'s subtree generation
and restores the enclosing theme afterwards. It nests.

```go
gui.Themed(light, func(w *gui.Window) gui.View {
    return gui.Column(gui.ContainerCfg{Content: []gui.View{
        gui.Button(gui.ButtonCfg{ID: "ok", Text: "OK"}),
    }})
})
```

The builder callback is required, not sugar. A signature taking ready-made child
views receives children the caller already built under the enclosing theme,
because factories resolve defaults when they are called.

`Themed` scopes generation only. Reads that happen after generation stay
window-scoped: scroll metrics on the event path, and the theme picker's report
of the active theme.

## Override precedence

Unchanged. Factories fill only the fields a caller left unset, so an assigned
`Cfg` field beats the theme at every level, inside a `Themed` subtree included.

## One source of truth

`init` calls `applyTheme(ThemeDark)`, so the `default*Style` package vars in
`styles*.go` are filled from `ThemeMaker` before the first frame. They carry no
literals of their own.

This was not true when per-window themes landed. `init` seeded
`installedThemeID` instead, leaving the literals in place, so an app that never
called `SetTheme` ran on a mixture: widgets reading `guiTheme.xStyle` got
`ThemeDark` while widgets reading the mirror got the literals. Issue #300
removed the literals and resolved every delta in `ThemeDark`'s favor — a visible
change to the default appearance, chiefly the loss of the 1.5px border on
buttons, inputs and containers. See `docs/specs/theme-style-single-source.md`.

## The generation boundary (issue #301)

Reads split by **phase**, not by whether a window happens to be reachable.

**During generation** — widget factories and `GenerateLayout` — the bare
`guiTheme` / `default*Style` read stays. It is not a compromise: `Themed` scopes
a theme by push/pop of the _installed_ theme, so a generation-time read that
calls `w.Theme()` silently ignores the scope. The ~420 sites are also the hot
path. Deferring one means a closure plus a `Cfg` heap escape per widget per
frame.

**Outside generation** — event handlers, post-arrange injection, public window
methods reached from handlers, and the backends — the read names its window.
Before #301 those resolved against whichever window generated last: right by
timing in a one-window app, wrong with two. `themePickerSyncHighlight` was
already wrong that way.

Migrated in #301:

| File                                    | Function                                       |
| --------------------------------------- | ---------------------------------------------- |
| `gui/event_handlers.go`                 | `keyDownScrollHandler`                         |
| `gui/scroll.go`                         | `scrollHorizontal`, `scrollVertical`           |
| `gui/scroll_smooth.go`                  | `scrollSmoothBy`                               |
| `gui/native_print.go`                   | `ExportPrintJob`                               |
| `gui/view_theme_picker.go`              | `themePickerSyncHighlight`                     |
| `gui/view_toast.go`                     | `toastEnforceMaxVisible`                       |
| `gui/view_select.go`                    | `selectScrollTo`                               |
| `gui/inspector.go`                      | `inspectorInjectWireframe`                     |
| `gui/backend/{metal,gl,web}/backend.go` | `renderFrame`                                  |
| `gui/backend/internal/framestate`       | `FrameBg`                                      |
| `gui/backend/web/custom_shader.go`      | `drawCustomShaderFallback` (via `Backend.win`) |

### The copy, and why the store is a pointer

`Theme` is a large value — ~40 style structs plus ~40 text styles — so
`w.Theme()`, which returns it by value, is not free. The scroll path reads it
per wheel or arrow event, and the first cut of #301 cost 36 ns → 1083 ns per
event on `BenchmarkEventFnMouseScrollFocused`: a 30x regression, all of it
memcpy.

So both stores hold a pointer to an immutable value: `Window.theme` and the
package `defaultTheme`. A setter publishes a new value instead of writing
through the pointer, so a reader that took the pointer under `RLock` can keep
using it after dropping the lock. `w.Theme()` is unchanged for callers — it
dereferences — and internal hot reads call the unexported `w.themeRef()`
(`gui/theme_install.go`), which returns the pointer and copies nothing. With
that, the scroll benchmark is back at ~39 ns/op, 0 allocs.

Per-frame readers (the backends' clear color) keep `w.Theme()`. The copy is
irrelevant once per frame and the value form is the safer default.

### Gate

`make ergonomics-audit` runs mode `theme`, which flags a `guiTheme`,
`CurrentTheme()` or `default*Style` read inside a function with a `*Window` /
`*gui.Window` receiver or parameter — but only in paths that are post-generation
by construction: `gui/backend/**`, `gui/scroll*.go`, `gui/event*.go`,
`gui/native_*.go`, `gui/window_*.go`. The phase cannot be decided from one
function's syntax, so the mode does not guess. Handlers living in mixed-phase
`view_*.go` files are covered by the convention only. A deliberate exception
carries `ergonomics-audit:theme-global` on its line.

### Rejected: `w.Button(...)` receiver sugar

Raised as #296 proposal item 5 and declined again in #301:

1. It does not fix the phase problem. `w.Button(cfg)` still runs eagerly and
   still resolves the theme at factory time — the same error with a window name
   attached.
2. It breaks reusability. A package factory builds window-agnostic view intent.
   A receiver binds a fragment to a window at construction, so every sub-tree
   helper threads `w`, and closing over the wrong window becomes easy — the bug
   class per-window themes set out to close.
3. It fights `Themed`, for the reason above: the subtree scope lives in the
   installed theme, not in the window.
4. It is all cost. ~60 factories, every example, and the sibling consumers
   churn, with no perf win — the closure cost that ruled out the blanket
   migration comes from deferring, not from the receiver.
5. The access already exists where it is needed: `GenerateLayout(w)`,
   `EventCtx.Window`, `viewFunc(func(w *Window) View)`, `w.EffID` / `ctx.EffID`.

The pattern is not banned — the boundary is a receiver for window-level
singletons (`(*Window).Sidebar`, `(*Window).Toast`) and a package factory for
reusable view fragments.
