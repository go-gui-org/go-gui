# InputGroup

Issue #820.

Status: **implemented** — `InputGroup(InputGroupCfg)`, unreleased.

## The problem

An app that wants a Bootstrap-style input group (a field with a `$` prefix, a
`.00` suffix, or a button attached) had no way to join controls into one shape.
A titled container is a group box: it draws a frame around separate controls,
each with its own border. A row of controls shows a double border at each seam,
round corners on every control, and a focus ring on each one.

## The design

`InputGroup` is a new factory with its own Cfg. It is a `Row` that:

- draws the only border and radius, from the theme's input style;
- sets `ClipContents`, so the segments are clipped to the rounded area inside
  the border. An end segment's square fill cannot cover the rounded corner, so
  no per-corner radius is needed;
- puts a 1px `Rectangle` divider (`max(1, SizeBorder)`, border color) between
  each pair of segments, as `SegmentedControl` does;
- is not a tab stop. While the focus ID lies under the group's effective ID
  (`targetWithin`), its AmendLayout sets the focus border color and hangs the
  theme's focus ring on the group.

Segments are the typed `InputGroupSegment`, made only by constructors:
`InputGroupText(InputGroupTextCfg)`, `InputGroupInput(InputCfg)`,
`InputGroupSelect(SelectCfg)` and `InputGroupButton(ButtonCfg)`. Each takes the
widget's ordinary Cfg and sets `SizeBorder: NoBorder`, `Radius: NoRadius`, a
fill height, and the unexported `noFocusRing`. A Label becomes the A11YLabel,
because `labelledField` would stack it above the segment. A button with no
Padding takes `Theme.PaddingField`, so the group keeps the height of an Input.
The text addon has the `ColorBackground` fill: `ColorPanel` equals
`ColorInterior` in the light theme, so a panel addon disappears.

`noFocusRing` is needed because a segment's outset glow is clipped at the group
edge and spills over the segments beside it. It is unexported on `InputCfg`,
`ButtonCfg` and `SelectCfg`, like `ButtonCfg.selected`.

### Renderer change

The `ClipContents` stencil mask was the outer rect with the outer radius. The
container's own border is drawn before its children, so a child fill painted
over the border inside each rounded corner. The mask is now inset by
`SizeBorder`, with radius `Radius - SizeBorder`. A container with no border is
not affected. No production code set `ClipContents` before this change.

## Cost

One `[]View` for the segments and dividers per frame, the same as
`SegmentedControl`. A segment is one pointer (its view). No new theme struct:
the group reads `defaultInputStyle`, `Theme.ColorBackground` and
`Theme.PaddingField`.

## Rejected Approaches

- **Per-corner radius on `Shape`** — changes every rect draw path, clip radius,
  shadows, and the Metal and GL shaders. It still leaves doubled borders at the
  seams, clipped focus rings, and corner flags that the caller sets by hand on
  each child.
- **Free `[]View` children with a theme patch** (`Themed` +
  `Theme.With(InputPatch{...})`) — makes a new theme id per group per frame, and
  an explicit `SizeBorder` on a child Cfg silently beats the patch.
- **Free `[]View` children that the caller strips** — this is the failure the
  issue shows: one missed field on one child breaks the look.
- **A "grouped" flag in the generation context that each widget reads** — hidden
  coupling across every widget factory for one container.
- **Restyle children from the group's AmendLayout** — border space is reserved
  at layout, before amend runs, and the children's own amend hooks overwrite the
  change.
- **Rebuild `SegmentedControl` on `InputGroup`** — a segmented control is one
  value and one tab stop with arrow-key selection; an input group is a layout of
  independent controls. They stay separate widgets.
