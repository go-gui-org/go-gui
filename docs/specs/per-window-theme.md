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

The globals stop being app state. They are a frame-scoped cache of the theme
belonging to the window currently being generated.

### Why this is correct

Every window's frame pass runs on the one main OS thread. Both desktop backends
call `runtime.LockOSThread` in package `init` and drive all windows from a
single sequential loop (`gui/backend/metal/backend.go`,
`gui/backend/gl/runapp_x11.go`). Two windows can never generate layouts at the
same time, and no layout is generated off that thread. A factory-time read
therefore always resolves against the window being generated.

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
pass (tests, `main` before `Run`) see the change at once; the next frame
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
views would receive children the caller already built under the enclosing theme,
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
removed the literals and resolved every delta in `ThemeDark`'s favour — a
visible change to the default appearance, chiefly the loss of the 1.5px border
on buttons, inputs and containers. See
`docs/specs/theme-style-single-source.md`.

## Follow-up

Migrating factory-time reads to `w.Theme()` is optional and only pays where a
window is already in hand and no closure is added: the event path
(`gui/event_handlers.go`, `gui/scroll.go`), the backends, and new widgets.
