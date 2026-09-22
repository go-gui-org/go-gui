# ThemeCfg grouping

Issue #755. `ThemeCfg` holds 71 fields in one flat struct. Material 3 groups the
same tokens into color scheme, type, and shape. The flat struct is easy to
search but hard to learn.

## The problem

A new reader sees 71 names with no map of which ones relate. `ThemeCfg` is built
as a struct literal by `theme_defaults.go`, `theme_macos.go`, `theme_winui.go`,
`theme_gnome.go`, and by each sibling repo that makes a theme. A regroup is a
breaking change.

## The rule

The struct stays flat. Section banners in `gui/theme.go` name the group of each
field. `docs/theme-tokens.md` is the same map as an index. No field moves. No
exported name changes. The zero-value rule holds: the grouping adds comments
only.

The groups are type, shape, spacing, control sizes, color scheme, elevation,
focus, and window chrome. Cross-cutting tokens stay with their consumer: the
focus ring stays under focus, elevation stays under elevation, borders stay
under shape and color scheme.

A nested `Color`/`Type`/`Shape` regroup waits for a release that already breaks
`ThemeCfg` for another reason. Issue #754 landed without such a break
(per-widget patches add no `ThemeCfg` field), so the wait continues.

## Cost

Zero. Comments and docs add no allocation and no exported surface.

## Related

- `docs/theme-tokens.md` — the field index.
- `docs/specs/per-widget-theme-overrides.md` — per-widget patches add no
  `ThemeCfg` field, so this entry stays independent of that change.

## Rejected Approaches

- **Nested groups now (`ThemeCfg{Color: ..., Type: ..., Shape: ...}`).**
  Rejected: it is the correct end state but it breaks each literal in go-gui and
  five siblings for a learnability gain alone. Group edges are also arguable
  (focus ring, elevation, borders). Do it once, with another `ThemeCfg` break,
  not on its own.
- **Deprecated aliases (new groups plus flat alias fields for one release).**
  Rejected: two ways to set one value need a precedence rule, the surface
  doubles for a release, and broad deprecation markers drown first-party code in
  warnings.
