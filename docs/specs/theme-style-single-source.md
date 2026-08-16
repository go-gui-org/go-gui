# Theme styling has one source: ThemeMaker

Issue #300. Landed on top of per-window themes (#296), which found the problem
and deliberately left it alone.

## The problem

Go-Gui had two sources of default widget styling, maintained by hand:

- About 30 `default*Style` package-var literals in `gui/styles.go`,
  `gui/styles_widget.go`, `gui/styles_widget_control.go` and
  `gui/styles_widget_overlay.go`.
- `ThemeDark`, built by `ThemeMaker(themeDarkCfg)`.

Only `applyTheme` writes the literals, and `init` never called it — it seeded
`installedThemeID` so the first frame's `installTheme` would be a no-op. An app
that never called `SetTheme` therefore ran on a **mixture**: widgets that read
`guiTheme.xStyle` directly (badge, expand panel, slider, progress bar,
inspector) got `ThemeDark`, while widgets that read the `default*Style` mirror
(button, input, container, listbox, tree, menubar, ...) got the literals. The
first `SetTheme` call replaced every literal, so the shipped look and the
after-one-theme-switch look already differed.

Two further styles were unreachable by any theme, because `applyTheme` never
touched them: `defaultNumericInputStyle` and `defaultRadioGroupStyle`. Both kept
a 1.5px border under every theme, light included.

## The rule

`ThemeMaker` is the only source of default styling. The `default*Style` package
vars are **mirrors**: declared with no initializer, filled by
`applyTheme(ThemeDark)` from `init`, and refilled by `(*Window).installTheme`
whenever the active theme changes.

Never give a mirror an initializer. A package-var initializer is evaluated
before `init` runs, so `applyTheme` overwrites it immediately — the literal is
dead weight whose only effect is to drift from `ThemeMaker` and mislead the next
reader. `TestDefaultStylesMirrorThemeDark`
(`gui/styles_widget_defaults_test.go`) fails if one reappears.

Two literals in `gui/styles.go` are **not** mirrors and keep their values:

- `DefaultTextStyle` — `baseDarkCfg` reads it while building `ThemeDark`, which
  happens during `init` before any theme exists.
- `defaultInspectorStyle` — `ThemeMaker` copies it into every theme.

Both are still re-assigned by `applyTheme`; for `ThemeDark` that assignment is a
no-op, and for any other theme it is the correct behaviour.

## Why `ThemeDark` carries the 1.5 border

Folding `SizeBorder = sizeBorderDef` into `baseCfg` makes `dark-bordered`
an exact duplicate of `dark`. That is accepted, not an accident: the
call-site count behind issue #325 found 90 of 104 example files calling
`SetTheme(ThemeDark.WithBorders(true))` explicitly, so bordered is what
applications actually use, and `ThemeDark` is the implicit default for
apps that never call `SetTheme`. The `dark-bordered` preset stays
registered for name-based theme selection, and `Theme.WithBorders(false)`
restores the old borderless look. `light` and `blue-dark` remain
borderless by omission.

Every other delta was a literal that had simply fallen behind: an unstyled
`dataGrid` placeholder, a dialog with no width bounds, a badge with no text
style.

## Appearance changes

Borders, revisited 2026-08: issue #300 removed the literal borders and
left `ThemeDark` borderless (`SizeBorder` 0 on every widget). The
call-site count behind issue #325 then showed the borderless default
missed the audience — 90 of 104 example files re-opted in with
`Theme.WithBorders(true)` — so `baseDarkCfg` carries `sizeBorderDef`
again. Widgets render with their 1.5 (slider 1, radio 2) borders under
`ThemeDark` and the app default; `Theme.WithBorders(false)` restores the
borderless look, and `light`/`blue-dark` remain borderless.

Everything else:

| Widget          | Change                                         |
| --------------- | ---------------------------------------------- |
| button          | `colorClick` 104 → 94                          |
| input           | `ColorFocus` 104 → 74; `Padding` 5 → 10        |
| input           | spell-error color → the theme's error red      |
| listbox         | `Padding` 6 → 10                               |
| switch, toggle  | `ColorFocus` → 74; toggle `Radius` 3.5 → 5.5   |
| badge           | `Color` blue → grey 104; gains bold white text |
| toast           | `TitleStyle` bold → normal                     |
| dialog          | gains `MinWidth` 200 / `MaxWidth` 300          |
| menubar         | `spacingSubmenu` 0 → 1                         |
| command palette | detail text → 225,225,225,140                  |
| data grid       | unstyled placeholder → full dark styling       |
| numeric input   | follows the active theme                       |
| radio group     | follows the active theme                       |

To get the old borderless look:

```go
gui.SetTheme(gui.ThemeDark.WithBorders(false))
```

## Related

- `docs/specs/per-window-theme.md` — how a theme is installed per window and per
  subtree.
