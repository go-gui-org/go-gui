# EventCtx: reduce event-handling ceremony

Status: **implemented and superseded.** The signature collapse shipped in
v0.52.0–v0.54.0 (`EventCtx`, `Consume()`, `tools/eventctx`); the handled-default
flip (consume-class pre-mark + `ctx.Bubble()`) shipped with it and was then
**deliberately reversed in v0.55.0** (`fe404f6`, #206): nothing is marked
handled for you, `ctx.Bubble()` is deleted, `ctx.Consume()` is the one rule for
every callback. The `class` argument survived only to name events for the
`debugUnconsumed` check (`gui/event_traversal.go`). The one-event-rule design is
authoritative; do not re-implement the consume-class default from this spec.
Item 3 (typed `ctx.State`) was dropped for the documented Go reasons. Breaking
change.

## Context

Event handling in `go-gui` carries three kinds of friction:

1. **Manual handled-marking.** Every consuming callback must write
   `e.IsHandled = true` or the event keeps bubbling. There are 371 such
   assignments across the repo. Forgetting one is a silent bug.
2. **Three-argument callbacks.** ~57 callback fields declare
   `func(*Layout, *Event, *Window)`, plus ~25 payload-carrying variants. Every
   closure repeats the same three parameters.
3. **State access.** `gui.State[T](w)` is verbose at the call site.

Only the five sibling repos (`go-charts`, `go-edit`, `go-kite`, `go-term`,
`go-map`) consume this library, so a large breaking change is affordable now and
will not be later.

Intended outcome: one breaking release that collapses the callback signature to
`func(EventCtx)`, flips the handled default for consuming events, and leaves
keyboard/hover propagation semantics intact.

**Item 3 is dropped.** Go cannot give a typed `ctx.State` — methods cannot take
type parameters, and a generic `EventCtx[T]` would force every `Cfg` struct and
widget factory to become generic, which the heterogeneous `Layout` callback
storage forbids. The achievable version saves ~10 characters. Instead, document
the one-line app-side helper:

```go
func st(ctx gui.EventCtx) *AppState { return gui.State[AppState](ctx.Window) }
```

## Design

### The type

New file `gui/event_ctx.go`. `EventCtx` is passed **by value**. It is three
pointers (24 bytes), so Go's register ABI passes it in registers with no heap
allocation even through an indirect call. Value semantics also makes nested
dispatch inherently safe — see [Reentrancy](#reentrancy) below. Mutation still
works because `Event` is a pointer.

```go
// EventCtx bundles the three values every input-event callback needs.
// Passed by value. Methods use value receivers and mutate through the
// Event pointer, so they match the alloc story and avoid addressability
// footguns on non-addressable EventCtx values.
type EventCtx struct {
	Layout *Layout
	Event  *Event // nil for OnAmend and OnScroll (no originating event)
	Window *Window
}

// Consume marks the event handled so ancestors do not receive it.
// Needed only in notify-class callbacks; consume-class callbacks are
// already marked handled before they run.
func (c EventCtx) Consume() {
	if c.Event != nil {
		c.Event.IsHandled = true
	}
}

// Bubble marks the event unhandled so ancestors receive it. This is the
// explicit opt-out for consume-class callbacks.
func (c EventCtx) Bubble() {
	if c.Event != nil {
		c.Event.IsHandled = false
	}
}

// Handled reports whether the event has been consumed.
func (c EventCtx) Handled() bool {
	return c.Event != nil && c.Event.IsHandled
}
```

All three methods are nil-`Event` safe, so they are callable from `OnAmend` /
`OnScroll` without a guard. No `HasEvent()` helper: plain `ctx.Event != nil`
needs no wrapper, and the nil case is documented per callback.

`ShapeCallback` (`gui/event_traversal.go:4`) becomes
`type ShapeCallback = func(EventCtx)`. Keeping it a type **alias** means untyped
closure literals in `Cfg` structs continue to compile without a conversion.

### Handled-by-default, per event class

The flip happens in dispatch, not globally. Two classes:

The class list below is exhaustive over the real fields in `eventHandlers`
(`gui/shape.go:380-395`). There is **no `OnMouseDown` callback** — mouse-down
dispatches to `OnClick` via `handleMouseDownEvent` (`gui/window_event.go:151`).

**Consume — marked handled before the callback runs.** `OnClick`, `OnChar`,
`OnMouseUp`, `OnGesture`, `OnFileDrop`.

**Notify — unchanged; callback must consume explicitly.** `OnKeyDown`,
`OnKeyUp`, `OnHover`, `OnMouseMove`, `OnMouseLeave`, `OnMouseScroll`,
`OnScroll`, `AmendLayout`, `OnDraw`, `OnIMECommit`. Also
`shapeButtonColors.OnHover` / `OnAmend` (`gui/shape.go:409-410`).

Rationale for the carve-outs, since this is the one non-mechanical decision in
the change:

- `OnKeyDown` receives _every_ key. A widget handling only Enter must leave Tab,
  Escape, and accelerators to bubble — which is why the `ClickOnEnter` dispatch
  (`gui/event_handlers.go:93-101`) consumes only on match. Auto-consuming would
  silently kill tab traversal in any widget with a key handler.
- Hover/move/leave are notifications, not consumption. Nested shapes
  legitimately all want hover; auto-consuming breaks every
  hover-highlight-on-container pattern and reintroduces the same boilerplate
  inverted.
- **`OnMouseScroll` is notify, not consume.** `mouseScrollFallbackHandler`
  (`gui/event_handlers.go:329-337`) calls the user handler and then falls
  through to the scroll container _only if the handler left the event
  unhandled_. Cascade-on-unhandled is the designed contract, asserted by
  `TestMouseScrollUnhandledCascadesToScrollContainer`
  (`gui/event_handlers_test.go:444`). Auto-consuming would invert it and
  silently break nested scroll containers. Scroll chaining is to scroll what
  bubbling is to keys — same carve-out, same reason.

**`MouseLockCfg`** (`gui/window.go:282-284`) — its `MouseDown`, `MouseMove`,
`MouseUp` convert to the new signature but get **no auto-consume**. Mouse lock
already bypasses hit-testing and normal propagation, so handled-marking is moot
there. Note its coordinates stay window-absolute, unlike the shape-relative
coords everywhere else.

This classification is **internal** — encoded at the dispatch sites and
described in prose in the docs. It is not exposed or configurable.

### Reentrancy

Nested synchronous dispatch is routine in this codebase: an `OnClick` that
scrolls a container reaches `fireOnScroll` → `OnScroll` while the click callback
is still on the stack, and `AmendLayout` recurses through children from
`gui/layout_pipeline.go:49`.

A single reusable `EventCtx` owned by `*Window` would therefore be corrupted by
any nested callback overwriting `Layout` or `Event` under the outer frame. Value
semantics removes the failure mode: each frame gets its own copy, and there is
no shared buffer for a user to retain past the callback.

### Allocation

`AmendLayout` (`gui/layout_pipeline.go:49`) and `fireOnScroll` (12 internal call
sites) run inside phases that are currently zero-allocation
(`layout_pipeline_bench_test.go`, `render_layout_bench_test.go`). Passing
`EventCtx` by value keeps them there — nothing escapes, because no pointer to
the struct is formed. This is the reason for value semantics over `*EventCtx`;
confirm with the alloc gates rather than assuming.

### Pre-mark ordering inside `callRelative`

`callRelative` (`gui/event_traversal.go:37-50`) does `saved := *e`, translates
coordinates, calls back, then `*e = saved` and re-applies the post-callback
handled flag. The auto-consume mark **must be set after `saved := *e`**, not
before:

```go
saved := *e
e.MouseX = saved.MouseX - layout.Shape.X
e.MouseY = saved.MouseY - layout.Shape.Y
if consume { // class parameter; see the next section
	e.IsHandled = true // pre-mark AFTER the save
}
callback(EventCtx{layout, e, w})
handled := e.IsHandled // Bubble() inside the callback shows up here
*e = saved
if handled {
	e.IsHandled = true
}
```

Marking before the save would copy `IsHandled = true` into `saved`, and the
restore would then undo any `ctx.Bubble()` the callback performed — a silent,
hard-to-trace failure. `EventCtx.Event` stays a pointer to the same `Event`, so
the save/restore is otherwise unaffected.

Semantics to document: **`Bubble()` opts out of _this_ callback's auto-consume
only.** It does not un-handle an event some earlier handler already consumed,
because the restore re-applies the incoming flag.

### Where the pre-mark goes — class is a parameter, not a helper

**The three shared traversal helpers each serve both classes.** A blanket
pre-mark inside them would silently destroy the carve-outs this spec spends its
length defending:

| Helper                 | Consume-class callers                                           | Notify-class callers                    |
| ---------------------- | --------------------------------------------------------------- | --------------------------------------- |
| `executeFocusCallback` | `OnChar` (`:38`)                                                | `OnKeyDown` (`:89`), `OnKeyUp` (`:137`) |
| `executeMouseCallback` | `OnClick` (`:219`), `OnMouseUp` (`:281`), `OnFileDrop` (`:384`) | `OnMouseMove` (`:252`)                  |
| `callRelative`         | via `executeMouseCallback`                                      | `OnMouseScroll` (`:306`)                |

(Line numbers in `gui/event_handlers.go`.) Marking inside `callRelative` would
make `if callRelative(...) { return }` at `:306` always true and kill
focused-target scroll cascading; marking inside `executeMouseCallback` would
auto-consume `OnMouseMove` and kill hover-on-container.

**Therefore: add an explicit class argument** to `executeFocusCallback`,
`executeMouseCallback`, and `callRelative` — `consume bool`, or a two-valued
`evClass` type if it reads better. The pre-mark then happens inside
`callRelative` (after `saved := *e`, per the ordering above) but only when the
flag is set. Every call site is forced to state its class, and the compiler
catches a new dispatch path that forgets to.

This **replaces** any single `invokeConsuming(...)` helper: such a helper would
have to duplicate the coordinate translation, and the classes are not separable
by call path.

**Direct call sites bypassing all three helpers** — each needs the class
decision applied by hand:

- `mouseScrollFallbackHandler` (`gui/event_handlers.go:329-337`) calls
  `OnMouseScroll` directly. Notify: no pre-mark, and the cascade-on-unhandled
  behaviour must be preserved verbatim.
- `ClickOnSpace` (`:41-50`) and `ClickOnEnter` (`:93-101`) call `OnClick`
  directly, bypassing the mouse path. Consume: both need the pre-mark.
- `OnGesture` dispatches directly at `gui/gesture.go:520`, through none of the
  three helpers. Consume: needs the pre-mark.

### Interaction with `Window.OnEvent`

`Window.OnEvent func(*Event, *Window)` (`gui/window.go:168`) is the app-level
last-resort hook, fired at `gui/window_event.go:69` **only when
`!e.IsHandled`**. It has no `Layout`, so it stays `func(*Event, *Window)` —
unchanged, and outside the signature table by design.

But auto-consume changes when it fires: clicks, chars, mouse-ups, file drops and
gestures that any widget handled will no longer reach `OnEvent`, where
previously a widget that forgot `IsHandled = true` let them through. That is the
intended semantics, not a regression — but it is a visible behaviour change for
apps using `OnEvent` as a global sniffer. Call it out in the migration guide.

### Signature groups

| Current                             | Count | New                          |
| ----------------------------------- | ----: | ---------------------------- |
| `func(*Layout, *Event, *Window)`    |    54 | `func(EventCtx)`             |
| `func(*Layout, *Window)`            |   ≥6† | `func(EventCtx)` (nil Event) |
| `func(*Layout, string, *Window)`    |     5 | `func(string, EventCtx)`     |
| `func(T, *Event, *Window)`          |   ~25 | `func(T, EventCtx)`          |
| `func(*Window)` / `func(T,*Window)` |   ~24 | **unchanged**                |

† Counts are Shape/Cfg **field declarations**, not textual occurrences; the
latter are higher because of factory helpers returning these types. The
`func(*Layout, *Window)` set is at least: `eventHandlers.OnScroll` and
`.AmendLayout` (`gui/shape.go:388-389`), `shapeButtonColors.OnAmend` (`:410`),
`InputCfg.OnBlur` (`gui/view_input.go:34`), `ButtonCfg.AmendLayout`
(`gui/view_button.go:23`), `ContainerCfg.OnScroll` / `.AmendLayout`
(`gui/view_container.go:48,53`), and the tooltip callback
(`gui/view_tooltip.go:180`). Enumerate exhaustively before writing the tool's
match list.

`OnBlur` fires from `gui/view_input.go:537` with no event in hand, so it
converts with a nil `ctx.Event` like the other lifecycle callbacks. It is
arguable that blur should carry the click that moved focus; that is a separate
change and explicitly out of scope here.

Payload carriers keep their payload as a leading argument rather than being
stuffed into `EventCtx` — a `GridRow` is not context, and hiding it in a struct
field loses type safety at the call site.

Lifecycle callbacks (animation `OnDone`/`OnValue`, native dialog and
notification `OnDone`, `NativeMenuCfg.OnAction`) are **not** converted: they
have no `Layout` and no `Event`, so an `EventCtx` would carry two nil fields and
dilute what the type means.

`OnDraw func(*DrawContext)` is unchanged — `DrawContext` is already its own
context type.

Parameter naming convention: `ctx EventCtx`. Note the adjacency to
`Window.Ctx() context.Context` (`gui/window.go:327`), which is untouched and
unrelated.

## Work

### Phase 0 — Issues

- File tracking issue for the refactor; add to org Project board (`projects/1`)
  with Kind/Area/Status set.
- File a **separate issue to update the GitHub wiki**
  (`github.com/go-gui-org/go-gui/wiki`). The wiki is linked from `README.md` as
  the primary documentation surface and is outside the repo, so it cannot be
  updated in-tree. Blocked-by the tracking issue.

### Phase 1 — Core

`gui/event_ctx.go` (new), `gui/event_traversal.go`, `gui/event.go`,
`gui/event_handlers.go`, `gui/shape.go` (callback field types in `ShapeEvents`).

Thread the class argument through `executeFocusCallback`,
`executeMouseCallback`, and `callRelative`, and apply the pre-mark at the direct
dispatch sites — see "Where the pre-mark goes" above. The blanket "pre-mark
inside the shared helpers" shortcut is wrong; those helpers serve both classes.

**Keyboard-activation dispatch.** The live click-on-key semantics are the
`eventHandlers` struct fields, not wrappers: `ClickOnSpace` fires through
**OnChar** dispatch (`gui/event_handlers.go:41-50`), `ClickOnEnter` through
**OnKeyDown** dispatch (`:93-101`), and `ClickButton` filters `OnClick`
(`:214`). The two key paths fall on opposite sides of the consume/notify split —
`OnChar` is auto-handled, `OnKeyDown` is not — so the Enter path must
`Consume()` explicitly and the space path must not double-handle. Test both.

**Internal handlers keep their three-arg form.** `charHandler`,
`keydownHandler`, `keyupHandler`, `mouseDownHandler`, `mouseMoveHandler`,
`mouseUpHandler`, `mouseScrollHandler`, `mouseScrollFallbackHandler`, and
`fileDropHandler` (`gui/event_handlers.go`) are package-internal dispatch
plumbing, not user-facing callbacks. Only the `ShapeCallback` boundary converts.
Leaving them as `(layout, e, w)` keeps the diff to the public surface.

**Delete the deprecated wrappers.** `spacebarToClick`, `enterToClick`, and
`leftClickOnly` (the three `Deprecated:` wrappers at the end of `gui/event.go`)
have no production call sites — only `gui/event_test.go:141-210` exercises them.
They were superseded by the `eventHandlers` fields above to avoid per-frame
closure allocation. Remove them and their tests rather than porting them; a
breaking release is the right moment. Update the referring comments in
`gui/view_container.go:33-43` and `gui/shape.go:397-399` — the latter cites
`docs/specs/perf-optimizations.md` §6, which does not exist. Do not propagate
that citation; drop it.

### Phase 2 — Migration tool

Built **before** the mechanical rewrite, since phases 3 and 5 are driven by it.
`tools/eventctx/` (beside `tools/requiredid/`), using `go/ast` +
`golang.org/x/tools/go/ast/astutil`, with `testdata/` golden tests matching the
`tools/requiredid` layout. Regex is not viable — the rewrite restructures
parameter lists and conditionally deletes statements inside nested closures.

**Rewrite rules, in order:**

1. **Signature.** `func(l *Layout, e *Event, w *Window)` → `func(ctx EventCtx)`.
   Rewrite body references: `l`→`ctx.Layout`, `e`→`ctx.Event`, `w`→`ctx.Window`,
   honouring the actual parameter names and any `_` blanks. Payload carriers
   keep the leading argument.
2. **Notify-class callbacks:** rewrite `e.IsHandled = true` → `ctx.Consume()`.
   Never delete it.
3. **Consume-class callbacks:** delete `e.IsHandled = true` when it is the last
   statement of the function or of a terminal branch. This is the safe subset.
4. **Everything else in a consume-class callback: report, do not rewrite.**

**Rule 4 is the migration's real risk and it is not automatable.** A
consume-class callback that today returns early _without_ setting handled is
relying on the old bubble-by-default, and after the flip it must call
`ctx.Bubble()` on that path. The tool cannot infer whether a given early return
meant "not mine, pass it on" or "done, nothing to do" — those are
indistinguishable in the old encoding, because both wrote nothing.

So the tool **emits a report** rather than guessing: for every consume-class
callback, list each `return` (explicit or implicit fall-off-the-end) that is not
dominated by an `IsHandled = true` assignment. Each entry is a human decision:
insert `ctx.Bubble()` or confirm consume-by-default is correct. Expect this list
to be short now that `OnMouseScroll` is notify-class — the remaining consume set
is `OnClick`, `OnChar`, `OnMouseUp`, `OnGesture`, `OnFileDrop`, where
conditional consumption is uncommon but real. The sharpest case is **`OnChar`
filtering**: a focused input whose `OnChar` ignores non-text characters
previously let them fall through to container-level ancestors, and now blocks
them by default. Hit-test subregions and disabled-state guards are the other
recurring shapes. Budget review time for this list; do not batch-approve.

The report is also the tool's contract for the sibling repos, which get the same
treatment in Phase 7 without a maintainer who knows this spec.

### Phase 3 — Mechanical rewrite of `gui/`

~205 callback literals plus ~371 `IsHandled = true` sites across
`gui/view_*.go`, `gui/datagrid/`, `gui/markdown/`, driven by the Phase 2 tool.
Representative files: `gui/view_button.go`, `gui/view_input.go`,
`gui/view_scrollbar.go`, `gui/view_tab_control.go`, `gui/view_select.go`,
`gui/list_core.go`, `gui/datagrid/view_data_grid.go`.

Work the rule-4 report to zero before moving on.

### Phase 4 — Non-standard signatures

`OnScroll` and `AmendLayout` (`gui/shape.go:388-389`) and
`shapeButtonColors.OnAmend` (`:410`) take an `EventCtx` with a nil `Event`;
update `fireOnScroll` and `gui/layout_pipeline.go:49`. `OnIMECommit` becomes
`func(string, EventCtx)`. `MouseLockCfg`'s three fields
(`gui/window.go:282-284`) convert with no auto-consume.

### Phase 5 — Examples

59 example programs, 29 of which contain `IsHandled` or three-arg callbacks.
Same tool. `examples/showcase` is the largest consumer.

### Phase 6 — Docs

- `CLAUDE.md` — the "callbacks share sig `func(*Layout, *Event, *Window)`" and
  "must set `e.IsHandled = true`" rules both become wrong.
- `README.md`, `docs/architecture.md`, `docs/commands.md`,
  `docs/cookbook-add-widget.md`, `CHANGELOG.md`.
- 21 markdown files reference the old signature or `IsHandled`, incl. ~15 of the
  71 files in `examples/showcase/docs/`.
- 12 per-example READMEs.
- Migration guide in `docs/` covering: the new signature, the consume/notify
  split, `ctx.Bubble()` vs `ctx.Consume()`, the fact that `Bubble()` opts out
  only of this callback's auto-consume, the nil `ctx.Event` in
  `AmendLayout`/`OnScroll`, and the `st(ctx)` state helper pattern.

### Phase 7 — Siblings

Run the tool across `go-charts`, `go-edit`, `go-kite`, `go-term`, `go-map` after
go-gui tags. Follow the existing `sync-siblings` skill order. Verify each with
`GOWORK=off` per the CI-drift note.

## Verification

1. `go build ./...` and `CGO_ENABLED=0 go build ./...` (Linux/Windows parity
   must not regress).
2. `golangci-lint run ./...`, `gofmt -l .`, `go vet ./...` (exercises the
   `requiredid` analyzer — confirm it does not pattern-match callback types).
3. `go test ./...` — full suite.
4. **Alloc gates, the real risk:** `go test -run '^$' -bench . -benchmem ./gui/`
   focusing on `layout_pipeline_bench_test.go`, `render_layout_bench_test.go`,
   `event_bench_test.go`, `view_frame_bench_test.go`. Zero-alloc phases must
   stay at 0 allocs. Compare against `main` with `benchstat`.
5. New tests: consume-class callback bubbles nothing by default; notify-class
   callback still bubbles; `ctx.Bubble()` restores propagation; `ctx.Consume()`
   stops it from a notify-class callback; `OnKeyDown` on a focused widget still
   lets Tab reach the traversal handler; nested hover fires on both child and
   ancestor; `ClickOnSpace` and `ClickOnEnter` both activate exactly once and
   both consume; all three `EventCtx` methods are no-ops on a nil `Event`.
6. Reentrancy: an `OnClick` that triggers a synchronous `OnScroll` must leave
   the outer callback's `EventCtx` usable. `ctx.Layout` stability is trivial
   under value semantics, so assert the stronger property — after the nested
   dispatch returns, the outer `ctx.Event` is the same pointer, its coordinates
   are still shape-relative to the outer layout, and `ctx.Consume()` from the
   outer frame still takes effect.
7. Class-parameter correctness — the shared helpers serve both classes, so
   assert each side explicitly: `OnMouseMove` through `executeMouseCallback`
   does **not** auto-consume (hover-on-container survives); `OnMouseScroll`
   through `callRelative` at `event_handlers.go:306` does **not** auto-consume
   (focused-target scroll still cascades); `OnChar` through
   `executeFocusCallback` does, while `OnKeyDown`/`OnKeyUp` through the same
   helper do not.
8. `Window.OnEvent` reachability: a click consumed by a widget no longer reaches
   `OnEvent`, and an unhandled event still does.
9. Scroll chaining regression:
   `TestMouseScrollUnhandledCascadesToScroll\ Container`
   (`gui/event_handlers_test.go:444`) must pass unmodified — it encodes the
   cascade contract that keeps `OnMouseScroll` in the notify class. Treat any
   need to edit it as a design error, not a test to update.
10. Run `go run ./examples/showcase/` and exercise click, keyboard traversal,
    hover, scroll chaining, and a data grid by hand — the scroll-chaining and
    tab-traversal changes are the ones unit tests are most likely to miss.

## Decisions

1. **Single PR.** Phases 1–6 land together. Split PRs would leave `main`
   uncompilable between them, which is worse than one unbisectable commit — and
   the repo has no external consumers to strand. Phase 0 (issues) precedes it;
   Phase 7 (siblings) follows the go-gui tag.
2. **Wiki rewrite is out of scope** for this PR. Phase 0 still files the issue;
   the rewrite happens immediately after the PR merges, against the shipped API
   rather than a moving target.
