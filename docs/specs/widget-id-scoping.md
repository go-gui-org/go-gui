# Widget ID scoping

Status: proposal, not implemented. Written 2026-08-09 after a
`GOGUI_DEBUG=1` run of the showcase reported 39 duplicate IDs.

## Problem

`Shape.ID` is the identity key for focus, scroll offsets, per-widget
state, `FindByID`, and hero animations. It must be unique per window.
Nothing enforces that at compile time, and the failure is silent: two
widgets sharing an ID collapse onto one tab stop and one state slot,
with no error and no visual difference.

The showcase run produced 39 reports. Only two were application
mistakes. The rest came from the framework:

| Count | Cause                                                       |
| ----- | ----------------------------------------------------------- |
| 32    | `Input` put `cfg.ID` on both its container and its inner text |
| 5     | every markdown paragraph claimed the markdown widget's ID     |
| 2     | showcase: a constant ID in a loop; a nested widget copy-paste |

Both framework causes are now fixed (`Shape.focusOwner` for the first,
container-owned ID for the second), and the check is available as
`(*Window).TestDuplicateIDs`. This document is about the ergonomics
question those bugs exposed.

## Evidence that scoping is the missing primitive

Composite widgets already build scoped IDs by hand, 36 times in `gui/`,
with three different separators:

| Separator | Example                                                    |
| --------- | ---------------------------------------------------------- |
| `/`       | `cfg.ID + "/" + strconv.Itoa(i)` (`view_radio_button_group.go:99`) |
| `.`       | `cfg.ID + ".day." + strconv.Itoa(d)` (`view_date_picker_calendar.go:147`) |
| `:`       | `cfg.ID + ":handle"` (`view_splitter_handle.go:41`)        |
| prefix    | `cmdbtn:` namespacing in `CommandButton`                   |

Every composite widget reimplements an ID stack through string
concatenation. `mdRenderParagraph` is the one that forgot, and that is
the whole of the second cause above. A primitive the framework itself
needs 36 times is a primitive, not a convenience.

Two further consequences of having no scope:

- Uniqueness is window-global. The showcase reuses `"input-text"` in two
  different panels and gets away with it only because the panels never
  appear at once. Every ID in a large application is a global name.
- Heading blocks key off `block.AnchorSlug`
  (`view_markdown_blocks.go:356`), so two headings with identical text
  in one document collide. A scope keyed to the document would remove
  the collision without inventing a disambiguator.

## Failure modes explicit IDs actually produce

1. **A constant ID inside a loop.** Eight overflow-panel buttons built
   from one literal (`examples/showcase/demo_layout.go`). The ID must be
   derived from the item, and nothing in the type system says so.
2. **Copy-paste into a nested widget.** A `Button` and the
   `ProgressBar` inside it sharing one ID
   (`examples/showcase/demo_feedback.go`).

Both are cheap to catch once the duplicate check is trustworthy, which
is what the framework fixes bought. Neither is an argument for dropping
explicit IDs.

## Why implicit IDs are the wrong fix

The obvious reaction — derive IDs from tree position and let the author
skip them — trades a loud failure for a silent one. In an immediate-mode
tree, identity must survive reordering, insertion, and conditional
rendering. A position-derived key migrates state when a list changes:
insert a row at the top and every row below it inherits the row above's
cursor, selection, scroll offset and focus. There is no warning for
that, because from the framework's view nothing is wrong.

Prior art agrees. Dear ImGui uses an explicit ID stack (`PushID`/`PopID`)
precisely so loop iterations disambiguate; egui derives an `Id` from the
parent scope plus a caller-supplied `id_salt`. Both are explicit
identity plus a scope, not implicit identity.

## Proposal to evaluate

1. **An ID-scope primitive.** A way to push a namespace for a subtree so
   that inner IDs are resolved relative to it — replacing the 36
   hand-rolled concatenations with one mechanism, and giving loops a
   structural answer (`scope(i)`) instead of a naming convention.
2. **One separator.** Whatever the scope uses, applied consistently, so
   composed IDs are parseable and predictable.
3. **Per-container uniqueness.** Decide whether the invariant should be
   "unique within the enclosing scope" rather than "unique per window".
   This is the change that would make large applications composable, and
   the one with the largest blast radius: `FindByID`, focus traversal,
   scroll keys, `StateMap` keys and the debug audit all assume a flat
   namespace today.

Open questions:

- Does a scope apply to every ID-keyed subsystem, or only to focus and
  state? Hero animations match IDs across frames and across subtrees,
  which a scope could break.
- Do public IDs stay stable? Applications call `ScrollVerticalTo(id)`
  and the test helpers take IDs; scoping changes what string an
  application must pass.
- Is the scope explicit at the call site or implicit in the container?
  Implicit is less typing and harder to reason about when a widget moves.
