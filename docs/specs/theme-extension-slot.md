# Theme extension slot

Issue #733. Stores styles for widgets outside `gui/` in the `Theme`.

## The problem

`Theme` had no place for styles that belong to widgets outside
`gui/`. Siblings (go-edit, go-charts) read `w.Theme()` but kept a
parallel style struct and synced it with the theme by hand. That is
the two-sources problem that issue #300 removed inside `gui/`, one
layer out. The parallel struct also misses `Themed` scoping: a
subtree scoped to a light theme can draw an editor with dark styles.

## The rule

A sibling stores its style value in the theme with `WithExt` and
reads it with `Ext`. The slot keys by the exact value type. The
sibling owns the type, so `gui` never imports the sibling.

- `WithExt` returns a new theme with a fresh id. The install fast
  path (`needsInstall`) and `Themed` push/pop cover the value with
  no extra work.
- `Ext` reports a missing value as the zero value and false. A
  missing extension is a fallback, not a panic.
- Stored values are immutable. Theme copies share the backing map,
  and `WithExt` clones on write. A stored value that aliases
  writable caller memory breaks that promise.
- Every rebuild path carries the values: `WithColors`,
  `WithPadding`, `WithBorders`, `AdjustFontSize`. A sibling derives
  once at build time. `WithColors` copies the theme by value, so it
  carries them with no extra line. The `ThemeMaker` paths assign
  them after the build.
- Reads during generation use the installed theme
  (`CurrentTheme`), not `w.Theme()`. `Themed` scopes the installed
  theme, so `w.Theme()` misses the scope.

## Cost

One map-header copy per theme copy, against a 12 KB struct copy
that already happens. `Ext` costs one map lookup. `WithExt` clones
the map, but stores happen at build time, not per frame.

## Related

- `docs/specs/theme-style-single-source.md` — the same rule inside
  `gui/`.
- `docs/specs/per-window-theme.md` — install, identity, and the
  generation boundary the read rule follows.

## Rejected Approaches

- **Derive registry.** The sibling registers a derive function and
  `ThemeMaker` calls it. Rejected: a global mutable registry adds
  frame-thread races and startup ordering, and every `With*` path
  must call it or silently drop values. `WithExt` after the build
  gives the same single source with no registry.
- **Document the pattern, no API change.** Siblings keep parallel
  structs. Rejected: keeps the exact fault this entry removes,
  including frozen `SetDefault`-style caches and `Themed` scope
  misses.
- **Typed slot keyed by type without a fallback.** A missing value
  panics like `State[T]`. Rejected: extensions are optional per
  sibling, so absence is normal. The fallback is the locked
  behavior.
- **Slice storage.** A linear scan per read instead of a map
  lookup. Rejected: the map costs one header copy and reads in
  constant time. The locked decision is the map.
- **Extension `lerp`/`copyWith` (Flutter `ThemeExtension`).**
  Rejected: `gui` has no theme interpolation path, so `lerp` is
  dead surface.
