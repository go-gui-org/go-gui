# Spec: interaction-state color precedence (`ColorSet.pick`)

Issue: #690

Status: **implemented** on branch `feat/690-colorset-pick`, not yet released.

## Motivation

About 15 commits fixed the same class of bug one widget at a time. A disabled
widget still reacted to the pointer: `e5ed61d9` (button), `cd9a8842` (toggle,
radio), `6274186c` (switch), `89cbf85a` (listbox). A pressed state was missing
on one input path (#658). Hover leaked or went stale: `873470af` (tooltip),
`f1b054bd` (toast), `ffb8ae8a` (menu).

Each widget wrote its own precedence and its own disabled guard, so the same
defect was found again in the next widget. #716 removed one cause by giving the
theme style structs a single `Colors ColorSet`. This spec records the other
half: one rule for which color a state shows, written once.

## The rule

Two rules, one per channel. `ColorSet.pick` is the only place either is written.

```
fill:   disabled > pressed > hovered > focused > base
border: disabled > focused > base
```

**The fill follows the pointer.** A mouse user clicks a control, the click
focuses it, and the pointer is still over it. Under a focus-first fill the
control holds its focus color until the pointer leaves, which reads as stuck.

**The border does not follow the pointer**, because the border is how a focused
control says so. A single hover-first rule over both channels would drop the
focus border exactly while someone is pointing at the control, and for `Input`
the border _is_ the focus affordance.

Most toolkits split the same way. Fluent draws its `PointerOver` fill and its
focus rect at once; AppKit puts the ring outside the control; the web design
systems pair a hover background with a focus ring. Material ranks focus above
hover instead, but it has to — its state layer is one channel and cannot show
both.

`pick` takes a set that has already been through `resolved()`. It does no
fallback of its own, so an unresolved set answers with unset colors rather than
with a plausible wrong one. Every widget resolves in its `apply*Defaults`.

There is no "did anything change" return. Every caller assigns unconditionally,
which is the point: a reported change would put back the per-site `if` that
produced the bug history.

## What `ColorSet.Focus` means now

Under a hover-first fill, the focus fill shows only where a control is focused
and the pointer is elsewhere — in practice, a control reached by the keyboard.
That is a real state and worth painting, but a rare one. It is not dead code; a
hover test never reaches it.

## The two passes

State arrives in two places, and neither sees all of it.

|             | `AmendLayout`                 | `OnHover`                                    |
| ----------- | ----------------------------- | -------------------------------------------- |
| `ctx.Event` | nil                           | carries `MouseButton`                        |
| focus       | yes                           | yes                                          |
| held Space  | yes                           | yes                                          |
| pointer     | **no**                        | yes, implicitly — it only fires when hovered |
| held mouse  | **no**                        | yes                                          |
| runs        | first (`layoutPipeline`)      | second (`layoutArrange`)                     |
| disabled    | guard, after `layoutDisables` | `layoutHoverDepth` returns early             |

Both passes build a full `stateFlags` and assign both channels. The amend pass
leaves `hovered` false and writes a complete, self-consistent appearance for a
control the pointer is not over. The hover pass knows strictly more, so it
re-picks outright rather than patching what amend left.

`TestAmendRunsBeforeHover` pins the ordering the design rests on.

The three-clause widget guard
(`Shape.Disabled || !hasEvents() || events.OnClick == nil`) stays at each call
site. It also covers a shape with no events and a nil `OnClick`, which `pick`
knows nothing about. `pick`'s `disabled` term is the belt to that braces.

## Scope

Routed through `pick`: `Button`, `Toggle`, `Switch`, `Radio`.

`Radio` is the one widget whose channels are crossed on purpose. Its fill is its
selection, so every interaction state lands on the circle's border: it builds a
border-valued set and takes `pick`'s **fill** return. `pick`'s border return is
unused there.

Not routed, but following the same rule: `Input` and `Tree` rows had their
`!IsFocus` hover guard deleted, so `Button` and `Input` do not diverge.

Not in scope:

- The 12 widgets that call `Colors.applyTo` to fan a set out into flat `Cfg`
  fields and then read those. `applyTo` exists so a flat field the caller
  assigned wins over the set; a widget with both that called `pick` on the set
  would silently ignore the caller's field. Several also pass `nil` for channels
  they lack, so a set rebuilt from them would be half-empty. That needs the
  `Cfg` flat-field inversion first, which #716 deferred.
- Row widgets whose dominant state is _selected_ (`view_listbox_items.go`,
  `view_table_build.go`). `ColorSet` has no slot for selected, and those rows
  express it as "the resting fill is not transparent".
- Menus, which have no per-state fill: hover writes a hovered index into a
  `StateMap` and the next frame paints `colorSelect`.

## Testing

Before #690 no golden recorded a hovered, pressed-by-mouse or
disabled-and-hovered appearance: `renderGolden` left the pointer at the origin,
so `layoutHover` fired in no case. "The goldens re-record with an empty diff"
was therefore true and worthless as evidence.

`goldenCase` now takes `hoverX`/`hoverY`, `mousePressed` and `hoverInert`. Two
positions are set, on purpose: `mousePosX/Y` is what `layoutHover` dispatches
from, and `pointerAt` is what `recordHoverTarget` reads. A case whose point
misses its widget fails rather than recording a resting appearance under a hover
name — `hoverInert` is the opt-out for a disabled widget, which is never a hover
target.

`mousePressed` is a bool rather than a `MouseButton` because `MouseLeft` is 0: a
`MouseButton` field would mark every case that did not mention it as pressed.
`NewWindow` dodges the same trap by seeding `mouseButtonHeld` to `MouseInvalid`.

The recorded files pin the appearance **after** this change, not before it. The
cases were added and recorded against the shipped behavior first, so the
precedence change had to land as a diff that was read — `button_focus_hover` and
`input_focus_hover` moved from the focus fill to the hover fill — and were then
re-recorded. The evidence was that diff. What the files hold now is the new
state only, so they would not have reddened on the change that created them;
what they guard is the next change.

## Rejected Approaches

- **Build-time state (`IsHovered` / `IsPressed`).** #588 closed report-only:
  nearly every built-in `OnHover` also sets a mouse cursor, which is
  window-level state reset on each mouse move. `IsHovered` has no leave-reset
  and different blocking rules, so those bodies cannot move. Cursor would need
  new API surface. Do not re-propose.
- **A single hover-first rule over both channels.** Drops the focus border while
  the pointer is on the control. The border is the affordance.
- **Focus-first for the fill.** The stuck fill described above. The argument for
  it was that `Button` and `Input` already did it — an argument from incumbency,
  and the incumbent behavior is the defect.
- **Each pass picks only its half** (amend takes focus and press, hover takes
  hover and press). That is the old structure with a helper bolted on: the hover
  pass has to consult focus anyway, at which point it is the two-pass design
  with extra steps.
- **A single pass after hover.** Needs a new pipeline stage between
  `layoutHover` and render, and `setMouseCursor` would still have to stay in
  `OnHover`. Adds a tree walk per frame.
- **A `changed bool` return from `pick`.** Reintroduces the per-site `if`.
- **`Disabled` / `Checked` / `Selected` fields on `ColorSet`.** No bug history
  asks for them, they add exported surface, and most widgets would carry slots
  they never set. Revisit from evidence, not ahead of it.
- **Per-component theme token tables** (dxui's Primitive → Semantic →
  Component). Go-Gui already has semantic `Theme` roles (`docs/style-guide.md`).
  A token table moves the per-widget explosion into a map and loses the type
  check.
- **A retained style cascade** like dxui's `StylePatch`. Several large patches
  per widget per frame conflict with zero allocations on the view path. A
  retained tree can absorb that; the immediate-mode pipeline cannot.
