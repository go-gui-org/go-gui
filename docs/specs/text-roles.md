# Semantic text roles

Issue #734. Names theme text by purpose instead of by face and step.

## The problem

`Theme` exported a closed 6x6 grid of numbered rungs: `N1..N6`, `B1..B6`,
`I1..I6`, `BI1..BI6`, `M1..M6`, `Icon1..Icon6`. A number said how big the text
was, never what it was for. A caller who picked `B3` for a section title and
another who picked `B2` for the same purpose both had correct code, and the two
looks drifted apart. The named quiet roles (`TextStyleSecondary`,
`TextStyleLabel`, `TextStyleDisabled`, `TextStylePlaceholder`, #335) already
took the semantic approach; the grid worked against it.

## The rule

A caller rendering a title names `TextStyleTitleSmall`. The roles, all derived
in `fillTextRungs` (`gui/theme_maker_rungs.go`):

- `TextStyleDisplay`, `TextStyleTitle`, `TextStyleTitleSmall` — bold heroes,
  window titles, section titles.
- `TextStyleBodyLarge`, `TextStyleBody`, `TextStyleBodySmall`,
  `TextStyleCaption`, `TextStyleCaptionSmall` — the roman scale.
- `TextStyleCode`, `TextStyleCodeSmall`, `TextStyleCodeTiny` — mono with the +1
  optical compensation baked in (terminals).
- `TextStyleIconXLarge` … `TextStyleIconTiny` — the glyph scale, carrying
  `glyphRole` so optical centring treats runs as ink.

The tail a fixed role set cannot cover goes through face modifiers
(`gui/theme_text_modifiers.go`):

- `Bold`, `Italic`, `Roman` are methods on `TextStyle`: pure Typeface maps,
  order-independent and idempotent, leaving size and color alone. Donors
  (`Display` read only to override its size), markdown emphasis
  (`BodySmall.Italic()`), and the roman extra large (`Display.Roman()`) spell
  through them.
- `Mono` is a method on `Theme`, not `TextStyle`, because only the theme knows
  `ThemeCfg.MonoFontFamily`. It applies the same +1 the Code roles bake in;
  overriding Size afterwards discards it, so a donor that states its size is not
  compensated twice.

Decomposing a role (`role.Color`, `role.Size`) works exactly as decomposing a
rung did. The quiet roles compose unchanged: `withRoleAlpha(Body, Secondary)`
where the color is caller-supplied.

The full old-to-new mapping is the `### Changed` entry for #734 in
`CHANGELOG.md`. The golden tests pass unchanged, which is the proof the
migration moved no pixel.

## Cost

The exported text surface drops from 36 fields to 17 roles plus 4 modifier
methods. Derivation runs once per theme build, as before; modifiers allocate
nothing (value copies).

## Related

- `docs/specs/widget-visual-consistency-audit.md` — the style guide roles the
  quiet vocabulary comes from.
- `docs/specs/theme-style-single-source.md` — one derivation site;
  `fillTextRungs` stays that site.

## Rejected Approaches

- **Roles on top, grid stays.** Additive, zero migration. Rejected: two
  vocabularies keep the exact drift the issue reports, and a later removal taxes
  every caller twice (learn roles now, forced migration later). The breaking
  branch was already open, so the forced migration ships once, in the same
  release as #732/#733/#735.
- **M3's 15-role set verbatim.** Display/headline/title/body/label in three
  sizes each. Rejected: `TextStyleLabel` already means the quiet-text role
  (#335), so `labelLarge` collides; and M3 has no answer for the mono +1
  terminals depend on or the icon glyph scale. The toolkit set keeps M3's
  purpose idea with toolkit names plus Code and Icon families.
- **Roles for N/B/M only, Icon and italic carved out.** Rejected: a surviving
  grid is not a replacement, and the carved-out faces had live callers (markdown
  emphasis, ~40 icon reads). Full coverage is what lets the grid die.
- **Document the mapping, no code change.** Rejected: unenforced convention
  already failed — that failure is the issue.
