# Per-widget theme overrides

Issue #754. An app says "all buttons use radius 2" through the theme.

## The problem

Widget styles went private in #735. That left two override paths. Global
`ThemeCfg` tokens move every widget at once. A `Cfg` value on each call site
breaks the rule that a call site never states a visual value. The middle case
had no path: one widget class, every instance, stated once in the theme.

## The rule

A patch states the geometry of one widget class. Five patches exist, one per
class: Button, Input, Select, Dialog, Container. Each holds padding, border and
radius. Each field is unset by default. Colors stay with `WithColors` and are
not part of a patch.

Apply a patch with `Theme.With`. The result carries a fresh theme id. A patch of
the same type replaces the stored one. A patch of another type coexists with it.
Any other type panics. A panic beats a silent no-op: an ignored patch looks
applied while it changes nothing.

The patch applies once, at build time, to the private styles that `ThemeMaker`
owns. Frames read the styles, never the patch. The steady state costs nothing:
no copy and no map lookup on the scroll path.

Patches ride the ext slot, so every rebuild path carries them with no extra
field. `WithPadding`, `WithBorders` and `AdjustFontSize` carry the slot across
the rebuild and then re-apply the stored patches. A strip followed by a restore
keeps the patch. `WithColors` touches colors only, so patched geometry survives
it by value copy. The fade copies the whole target theme and blends colors only,
so patched geometry takes effect on the first frame by design.

A stored patch is immutable like any ext value. It holds no pointer and no alias
to caller memory.

## Cost

One map-header copy per theme copy, against a 12 KB struct copy that already
happens. `Theme.With` clones the map, but stores happen at build time, not per
frame. `ThemeMaker` itself does no extra lookup: rebuild paths re-apply from the
carried slot after the build.

## Related

- `docs/specs/theme-style-single-source.md` — the same rule inside `gui/`.
- `docs/specs/theme-extension-slot.md` — the slot the patches ride.
- `docs/specs/per-window-theme.md` — install, identity, and the generation
  boundary the read rule follows.

## Rejected Approaches

- **Closure over the style.** A `WithStyle` callback that edits the private
  style re-exports every style that #735 made private. Each field of each style
  becomes public and frozen.
- **More `ThemeCfg` tokens.** A `ButtonRadius` token per role grows the flat
  struct that #755 must already slim. The token count grows with each widget,
  past 40. Patches add no `ThemeCfg` field, so #755 stays independent of this
  change.
- **Mutable `Theme` fields.** Direct assignment breaks the rule that a published
  theme is never written through (`themeRef`).
- **Apply-once with no retained patch.** A patch that only mutates the styles is
  dropped by the next `WithBorders` rebuild. Retention in the slot is what makes
  composition hold.
- **Silent no-op on unknown type.** An unknown patch that returns the theme
  unchanged hides a misspelled override. The panic names it at the call site.
