# Developer ergonomics: assessment and improvement plan

Status: accepted after three independent reviews; ready to implement.
One breaking phase (§4.3, §4.4, §4.7) targeting v0.53.0; 18
sibling call sites affected (§7). §4.2 is entirely non-breaking and
lands in phase 1. Phases 1–3 ship as v0.52.x. §9 Q1–Q7
resolved; Q6 is a phase-2 gate on phase 4, and Q8 — whether `requiredid`
becomes supported surface — is open and due before phase 1.
Base: `main` @ `80715d1`

## Context

Two independent reviews rated app-level ergonomics ~7/10. Both landed on
the same shape of problem: the immediate-mode core is sound, and the
friction is bookkeeping the framework pushes onto callers (IDs, state
keys, event flags, per-widget colors). This spec records what was
measured, corrects two claims that do not hold against the code, and
prioritizes fixes.

Scope is API surface only. The rendering pipeline, allocation profile,
and immediate-mode model are not in question.

## 1. What was measured

All counts taken from `main` @ `80715d1`. Counting rules in §10 —
several figures depend on dedupe and scope choices.

| Metric                                | Count | Source                            |
| ------------------------------------- | ----- | --------------------------------- |
| Widget factories taking a `*Cfg`      | ~50   | `gui/view_*.go`                   |
| `On*` callback decls, raw             | 136   | `ergoaudit -mode callbacks`       |
| — distinct (name, signature) pairs    | 70    | deduped by go/ast; see §10        |
| — of shape `func(EventCtx)`           | 16    |                                   |
| — of shape `func(T…, EventCtx)`       | 19    |                                   |
| — with a trailing `*Window`           | 27    | 14 keep it; see §4.3              |
| — exposing a raw `*Event`             | 6     |                                   |
| — neither (no `Window`/`EventCtx`)    | 2     | `OnAction`, `OnDraw`              |
| Fields on `ContainerCfg`              | 71    | `gui/view_container.go`           |
| Fields on `ButtonCfg`                 | 38    | `gui/view_button.go`              |
| Distinct `Color*` field names (approx)| 20+   | `gui/view_*.go`; see §10          |
| `Opt[T]` field decls (approx)         | 110   | `gui/view_*.go`; see §10          |
| `StateMap[string, …]` call sites      | 242   | `gui/*.go`                        |
| `RequireFocusID` call sites           | **0** | dead; deleted by §4.2             |
| Exported symbols in `gui` (go doc)    | 953   | 228 funcs, 349 types, 376 methods |
| Exported event-dispatch entry points  | **0** | `Shape.events` unexported (§4.6)  |
| Focusable-by-default `Cfg`s           | 15    | `FocusDisabled` opt-out           |
| — of those, `ID` **not** required     | 9     | unguarded; see §4.2               |
| Literals of those 9, all repos        | 408   | `go/ast` walk; see §4.2, §10      |
| — focusable but ID-less (broken)      | 126   | 12 in go-gui's own widgets        |
| — using `FocusDisabled` opt-out       | **1** | decorative case is theoretical    |
| Tests in `examples/*/main_test.go`    | 63    | 98 lines are no-panic assertions  |
| `Cfg`s exposing `Scrollable bool`     | 7     | scroll state keyed by ID (§4.9)   |
| — tag-guarded                         | 5     | Combobox, CmdPalette, ListBox,    |
|                                       |       | Table, Tree                       |
| — guarded at runtime only             | 1     | `Container`; sole `RequireScrollID`|
| — unguarded entirely                  | 1     | **`Input`**                       |
| Examples calling `WindowSize()`       | 45    | 50 sites, 108 arithmetic lines    |

## 2. What works

Not a preamble — these are the constraints any fix must preserve.

- **Small core.** `NewWindow(WindowCfg)`, a view function,
  `gui.State[T](w)`, `backend.Run(w)`. `examples/get_started/main.go` is
  a stateful app in 60 lines including comments.
- **Zero-value `Cfg` structs.** Uniform across all ~50 factories. Adding
  a field is non-breaking, and autocomplete on `gui.ButtonCfg{` is the
  documentation.
- **No closures over state, no globals.** One typed state slot per
  window kills the entire stale-closure bug class.
- **Composition is plain functions.** `cardView(w)`, `listView(w)` in
  `examples/todo/main.go` — no registration, no interface to implement.

## 3. Corrections to circulating claims

Recorded because both errors have been used to justify work.

### 3.1 "Callback signatures are consistent `func(EventCtx)`" — false

Only 16 of 70 distinct signatures use the bare form. `InputCfg` alone carries
two conventions on adjacent fields (`gui/view_input.go:24,29`):

```go
OnTextChanged func(string, EventCtx)
OnTextCommit  func(*Layout, string, InputCommitReason, *Window)
```

Six callbacks still leak a raw `*Event` (§10 — two are the qualified
`*gg.Event` in `datagrid`, which an unqualified grep misses), including
`SplitterCfg.OnChange func(float32, SplitterCollapsed, *Event, *Window)`
(`gui/view_splitter.go:125`). The v0.52 `EventCtx` refactor
(`docs/specs/eventctx-callback-refactor.md`) converted the consume-class
events and stopped.

### 3.2 "`StateMap` plumbing leaks into app code" — false

Zero `StateMap` references exist in `examples/`. `nsSelect` and
`capModerate` are unexported (`gui/layout_overflow.go:65`), so app code
cannot write that call. The encapsulation being asked for already
exists; the reviewer read an internal file as public surface. **No work
required.**

## 4. Proposals, prioritized

### 4.1 Debug gate — additive, do first

`GOGUI_FOCUS_DEBUG` (`gui/layout_query.go:10`) already establishes the
pattern for exactly one check. Generalize it into a single `gui.Debug`
gate covering:

- duplicate `ID` in one frame (not only focusable ones)
- any shape with `Focusable == true && ID == ""` (see §4.2)
- any shape with `Scrollable == true && ID == ""` (see §4.9)
- `State[T]` type mismatch, with the concrete type held vs requested

Deliberately **not** in v1 of the gate: "consume-class event reaching
dispatch with no handler". Every inert shape under the cursor trips it,
so it is noise before it is signal. Revisit scoped to shapes that are
focusable or that claim a consume-class handler.

`focusDebug` is a package-level `os.Getenv` read
(`gui/layout_query.go:10`), so the honest claim is **no-op unless
enabled**, not "compiles to nothing" — there is no build tag and no
dead-code elimination. If zero-cost-when-off matters, that is a separate
`//go:build` decision, not something the current pattern delivers.

To be precise about the cost, since it has been queried: `var focusDebug
= os.Getenv(...)` is evaluated **once** at package var-init. There is no
per-frame `os.Getenv` today, and the guarded read in `focusDupWarn`
(`gui/layout_query.go:14`) is a plain bool load.

**But make the flag `atomic.Bool`.** The proposal here is `gui.Debug(b)`
— a function, so the flag becomes *mutable*, which today's env-only
value is not. `focusDupWarn` is called from focus-candidate collection
(`gui/layout_query.go:109`), i.e. per candidate per frame, so a plain
bool read racing a `Debug(true)` write from another goroutine is a data
race that `-race` will flag. `atomic.Bool.Load` compiles to a plain load
on amd64 and arm64, so this costs nothing measurable. The mutability, not
the lookup, is the reason.

**Warn once per `(check, ID)` per window.** These checks run at
focus-candidate collection — per candidate, per frame — so an
undeduplicated warning for one ID-less button is emitted at the frame
rate. That is not a diagnostic, it is a reason to turn the gate back
off, which defeats the item. Dedup state resets on window close and on
a `false → true` transition of `Debug`, so re-enabling the gate after
fixing something reports the current state rather than staying silent.
The ID-less checks key on the shape's position in the tree, since `""`
is precisely what they are reporting.

No API breakage. This converts the current "works but subtly doesn't"
bugs into immediate errors and is the cheapest item on the list.

### 4.2 Close the focus/ID hole, and delete `RequireFocusID`

`isFocusedTarget` (`gui/event_traversal.go:17`) returns `false` on an
empty ID, so a focusable widget without an `ID` renders, clicks, and
silently never joins the tab order.

**There are two separate holes, and the uncovered one is larger.**
Widget `Cfg` types split into two focus conventions:

| Group             | Field                | Cfg types | Analyzer |
| ----------------- | -------------------- | --------- | -------- |
| Opt-in            | `Focusable bool`     | 12        | covers   |
| Focusable-by-dflt | `FocusDisabled bool` | 15        | **none** |

("Cfg types" is exact: the `Focusable bool` inventory also matches
`Shape`, `termgrid`, and `inspector`, which are not app-facing `Cfg`s
and are excluded from the 12.)

The analyzer's `checkFocusableID` keys on a literal `Focusable` field
(`tools/requiredid/requiredid.go:91`). Focusable-by-default `Cfg`s never
set one — their own testdata records this as intended: "Focusable-by-
default Cfgs never set Focusable, so the rule stays quiet"
(`tools/requiredid/testdata/src/widgets/widgets.go:117`).

So the second group is unguarded end to end. `gui/view_button.go:135`
resolves `Focusable: !cfg.FocusDisabled` — true by default — and passes
`ID: cfg.ID` through unvalidated, with no `RequireID` call. And **9 of
the 15** default-on `Cfg`s carry no `gui:"required"` tag on `ID`:
`Button`, `Input`, `NumericInput`, `InputDate`, `Radio`,
`RadioButtonGroup`, `Select`, `Switch`, `Toggle`. Six do have it:
`Combobox`, `ColorPicker`, `DatePicker`, `ListBox`, `Slider`, `Tree`.

Concretely, today:

```go
gui.Button(gui.ButtonCfg{OnClick: handler}) // compiles, vets clean,
                                            // renders, clicks —
                                            // never tab-reachable
```

That is the most-used widget in the library, failing silently, caught by
nothing. `examples/get_started/main.go:47` sets `ID: "gs_counter"`, but
by convention, not by any enforcement.

**Delete `RequireFocusID` (`gui/state_registry.go:134`).** It is called
by nothing — zero call sites in go-gui (tests included) and zero across
all five sibling repos. Fourteen files expose a `Focusable bool` field
and not one of them invokes it. It is a widget-authoring guard, not
app-facing surface, and every widget in go-gui is first-party, so it
has no external caller by construction. Removing it is preferable to
wiring it up:

- The `requiredid` analyzer (`tools/requiredid/requiredid.go:87`)
  already reports the static case, at build time rather than at first
  render, with a message naming the `Cfg` type.
- `RequireFocusID` is redundant *with the analyzer*, at a strictly worse
  moment. It is not a claim that panics are the wrong tool here — see
  the enforcement split below.
- Retaining a dead exported guard implies a check that does not run.
  That is worse than no guard: it reads like coverage.

**Enforcement split (resolves the apparent panic contradiction).**
Deleting one panic while phase 4 adds `RequireID` to `Button` looks
inconsistent. It is not, once the cases are separated by *when the
defect is knowable*:

| Case                              | Knowable at | Mechanism        |
| --------------------------------- | ----------- | ---------------- |
| Literal `Cfg{}`, `ID` absent/`""` | compile     | `gui:"required"` |
| `ID: expr` that evaluates to `""` | first call  | `RequireID`      |
| `Focusable` set dynamically       | render      | debug gate       |

`gui:"required"` catches the overwhelming majority statically — that is
where the 126 affected sites live. `RequireID` is the narrow runtime
backstop for a computed ID that turns out empty, which no analyzer can
see. The debug gate covers what neither can. `RequireFocusID` fits none
of these rows: its static case is already the analyzer's, and its
dynamic case is `RequireID`'s.

**Row 1 is compile-time only where the analyzer runs, which today is
this repo alone.** `gui:"required"` is an ordinary struct tag. No
compiler, no `go build`, and no plain `go vet` reads it; only
`requiredid` does, and `requiredid` is a standalone binary invoked
explicitly (`Makefile:104`, `.github/workflows/ci.yml:155`). Importing
go-gui does not run it. For an app author who wires up nothing, the tag
is inert and row 1 collapses into row 2:

| Case                              | go-gui gets      | Author gets, unwired |
| --------------------------------- | ---------------- | -------------------- |
| Literal `Cfg{}`, `ID` absent/`""` | analyzer, in CI  | `RequireID` panic    |
| `ID: expr` that evaluates to `""` | `RequireID`      | `RequireID`          |
| `Focusable` set dynamically       | debug gate       | debug gate, if on    |

Consumers are not unprotected — `RequireID` genuinely panics
(`gui/state_registry.go:125-129`), and phase 4 wires it into all 9
default-on factories, so a missing ID is loud on first render rather
than silent. But it is a runtime panic in a GUI app instead of a build
failure, which is the trade this section argues against everywhere
else. Stating it plainly matters because the 3 sibling sites and every
future consumer sit in the right-hand column, not the left.

Adoption is one line, which is what makes leaving it undocumented
wasteful. `requiredid.Analyzer` is exported
(`tools/requiredid/requiredid.go:27`), `tools/` is in the main module
rather than a separate one, `cmd/requiredid` is a `singlechecker`, and
`golang.org/x/tools` is already a direct dependency (`go.mod:19`):

```fish
go run github.com/go-gui-org/go-gui/tools/requiredid/cmd/requiredid ./...
```

A `go tool` directive, `go vet -vettool=`, or a golangci-lint custom
plugin all work equally. §8 carries the doc deliverable; whether the
analyzer becomes *supported* surface rather than an internal tool is
Q8 (§9), and it is the one question this spec does not resolve.

**Route the residual gap to the debug gate.** The analyzer is static, so
three cases stay dark. They belong in §4.1, not in a panic:

| Case                                     | Analyzer | Debug gate |
| ---------------------------------------- | -------- | ---------- |
| `FocusCfg{Focusable: true}`              | reports  | —          |
| `FocusCfg{Focusable: on}` (dynamic flag) | silent   | catches    |
| `FocusCfg{ID: id}` where `id == ""`      | silent   | catches    |
| `//requiredid:ignore` opt-out            | silent   | catches    |

Add one `gui.Debug` check: at focus-candidate collection, warn on any
shape with `Focusable == true && ID == ""`. It inspects `Shape`, not
`Cfg`, so it covers **both** groups above — including the 15 default-on
widgets the analyzer cannot see at all. This is the immediate mitigation
and the reason phase 1 matters more than its size suggests.

**Fix only the unguarded group. `ID` stays the single identity key.**
The two holes are not symmetric, and treating them as one problem is
what made earlier drafts of this section long:

| Group             | Guarded today          | Live defects | Fix        |
| ----------------- | ---------------------- | ------------ | ---------- |
| Opt-in (12)       | yes — `checkFocusableID`| **0**       | none       |
| Default-on (15)   | **no**                 | **126**      | tag 9 + `RequireID` |

So the whole fix is the default-on row: tag `ID` with `gui:"required"`
on the 9 that lack it, add `RequireID` to their factories. Focus stays
default-on; the ID that focus depends on becomes mandatory. That is
phase 1, non-breaking, and it closes **every defect the audit found**.

**An earlier draft also proposed collapsing the opt-in group's
`Focusable bool` + `ID` pair into `Focus: gui.Focus(id)` — a constructed
value whose only constructor takes an ID, making the invalid state
unrepresentable. That is cut.** It is a real improvement in the
abstract and the wrong trade here:

- The opt-in group is **already covered**. `checkFocusableID`
  (`tools/requiredid/requiredid.go:87-106`) reports `Focusable: true`
  without a non-empty ID today. The audit found 126 broken literals and
  **all of them are default-on** — the opt-in group has zero, because
  the analyzer catches them. The collapse converts an error that is
  already caught into one that cannot be written: real, marginal.
- It costs a breaking change, 11 sibling edits to code that is correct
  as written today, and the entire complication below.
- **It erodes the property that makes IDs worth having.** `ID` is one
  deterministic identity key: 242 `StateMap[string, …]` sites and
  §4.9's scroll offsets all key off `Shape.ID`. A `Focus` value that
  also carries an ID is a second way to write the same key.
  `ContainerCfg` is the only type that is both opt-in-focusable and
  scrollable (`Focusable` :97, `Scrollable` :102, `ID` :67), so it
  would have had to carry `Focus: gui.Focus("panel")` for focus *and*
  `ID: "panel"` for scroll keying, with nothing forcing them to agree:

```go
// Would have needed a new rule to forbid: which one keys the scroll?
gui.ContainerCfg{Focus: gui.Focus("a"), ID: "b", Scrollable: true}
```

That state does not exist today. Closing a hole that the analyzer
already covers, by introducing a defect class that does not yet exist,
is the wrong direction — and the mutual-exclusion rule, its analyzer
check, and its debug check all existed only to contain a problem the
change itself created.

**What is genuinely given up.** In a repo that does not run the
analyzer, `Focusable: true` without an ID stays silently inert. That is
bounded from both sides: the §4.1 debug gate catches it at render time
regardless of analyzer adoption, and Q8 (§9) already governs whether
the analyzer is something consumers are expected to run. It is the same
gap Q8 exists to decide, not a new one.

Revisit only if the opt-in group starts accumulating real defects, or
if a widget needs to move between the two groups.

**Follow-up: three interactive widgets are in the wrong group.** The
split above reads cleanly — input controls default on, display and
container elements opt in — with three exceptions:

| Cfg              | Group  | Ships keyboard nav               |
| ---------------- | ------ | -------------------------------- |
| `TabControlCfg`  | opt-in | `KeyLeft/Up`, `KeyRight/Down` (`view_tab_control.go:447,453`) |
| `BreadcrumbCfg`  | opt-in | `KeyLeft`, `KeyRight` (`view_breadcrumb.go:294,300`) |
| `ThemePickerCfg` | opt-in | `OnKeyDown` (`view_theme_picker.go:114`) |

All three are controls a user would expect to tab into, and all three
**already implement arrow-key navigation**. That handler is dead code
unless the author sets both `Focusable: true` and an `ID` — so the
library ships keyboard support that is off by default, for widgets whose
whole purpose is interaction.

This is a different defect class from the 126 and is why the audit does
not count it: nothing here is written incorrectly. `Focusable` unset is
a valid state that the analyzer cannot flag, because for `Container` and
`Text` it is the right default. The gap is in the default itself, not in
any call site.

Measured: of 18 literals of these three `Cfg`s outside their own
factories, **3 set `Focusable: true`**. The other 15 render a control
with working key handling that no keyboard user can reach.

Proposed, as a follow-up rather than part of phase 1: move the three to
the default-on group — replace `Focusable bool` with `FocusDisabled
bool`, tag `ID` with `gui:"required"`, wire `RequireID`. It is breaking
in the same shape as the flat-color removal, so it rides phase 4 if
taken. Deliberately **not** folded into phase 1: phase 1 is scoped to
defects that exist against the current design, and this is a change *to*
the design. Deciding it needs a look at the remaining nine opt-in
members (`Splitter`, `OverflowPanel`, `DatePickerRoller`, and `TermGrid`
are the next-closest calls) rather than these three in isolation.

**Measured migration cost.** All 408 literals of the 9 unguarded `Cfg`
types across go-gui and the five siblings, counted with a `go/ast`
walker (regex cannot bracket-match multi-line literals):

| Location             | withID | noID + opt-out | noID (affected) |
| -------------------- | ------ | -------------- | --------------- |
| go-gui tests         | —      | —              | 66              |
| go-gui examples      | —      | —              | 45              |
| **go-gui library**   | —      | —              | **12**          |
| go-charts            | 8      | 0              | 1               |
| go-map               | 1      | 0              | 2               |
| go-edit/kite/term    | 6      | 0              | 0               |
| **Total**            | 281    | **1**          | **126**         |

Three conclusions:

1. **Consumer cost is 3 sites**, all in example programs
   (`go-charts/examples/basic_line/main.go:86`,
   `go-map/examples/full-map/main.go:126,139`). Application code in
   go-edit, go-kite, and go-term already supplies IDs throughout.
2. **12 affected sites are go-gui's own widgets** — the toast action
   and dismiss buttons (`gui/view_toast.go:170,179`), the date-picker
   month toggle and prev/next arrows (`gui/view_date_picker.go:269,
   278,288`), plus `view_date_picker_calendar.go:100`,
   `view_input_date.go:121`, `view_markdown_blocks.go:166`,
   `view_overflow_panel.go:60`, `time_travel_view.go:274`, and two in
   `gui/datagrid/`. Each carries an `OnClick` and none is
   keyboard-reachable. These are live accessibility defects in shipped
   first-party widgets, and requiring `ID` is what surfaces them.
3. **The decorative-button case is theoretical.** Exactly 1 literal of
   408 uses `FocusDisabled` as an opt-out. Migration is "add an ID",
   not "audit every button for intent".

The change is therefore a bug-finder that costs three example-file
edits in repos you own — not a migration tax.

**Pull the go-gui-internal half into phase 1.** The 12 library defects
and the 9 missing `gui:"required"` tags need no API change — adding an
`ID` to `gui/view_toast.go:170` is an ordinary bug fix, and tagging the
9 `Cfg`s only breaks callers *inside* this repo, which `requiredid`
flags in CI. Doing both in phase 1 closes the live accessibility bugs
immediately and turns the analyzer into a working gate. Phase 1 as
originally written only *warns* about these defects; there is no reason
to wait to fix them.

With the opt-in collapse cut, **§4.2 is entirely phase 1** and contains
no breaking change at all. Nothing here waits for v0.53.0.

### 4.3 One combined breaking release: events + callbacks

Two changes, one migration, because every one costs the five sibling
repos (`go-charts`, `go-edit`, `go-kite`, `go-term`, `go-map`) a bump.

**Finish `EventCtx` — for event-driven callbacks only.** Convert the
`*Window`-tailed and `*Event`-leaking callbacks that actually fire from
an input event. Target for that set: two shapes — `func(EventCtx)` and
`func(T, EventCtx)` — with no `*Layout` or `*Event` parameter.

**Explicitly out of scope: 14 of the 27 distinct `*Window`-tail
signatures.** Twelve fire from a timer tick, a dialog completion, or a
lifecycle transition — `OnInit`, `OnCloseRequest`, `OnCancelNo`,
`OnOkYes`, `OnDismiss`, `OnReply`, `OnLazyLoad`, `OnValue`, and four
`OnDone` variants. **There is no event.** Giving them an `EventCtx`
means a permanently-nil `ctx.Event` — the wart `CLAUDE.md` already
documents for `AmendLayout` and `OnScroll`, multiplied by twelve.

Two more are not callbacks at all but view builders that return a
value: `OnCellFormat func(…) GridCellFormat` and
`OnDetailRowView func(…) gg.View`. They are misnamed rather than
mis-signatured; renaming them out of the `On*` space would be clearer
than converting them.

That leaves **13 genuinely event-driven** signatures to convert:
`OnAction`, `OnChange`, `OnLayoutChange`, `OnPanelClose`,
`OnPanelSelect`, both `OnReorder` variants, `OnReset`, `OnSelect`,
`OnSubmit`, `OnTextCommit`, `OnToggle`, `OnValueCommit`.

`WindowCfg.OnEvent` / `Window.OnEvent` (`gui/window_cfg.go:14`,
`gui/window.go:168`, the same field declared twice) is a raw escape
hatch by design and also stays `func(*Event, *Window)`.

See §7.1 for how the boundary was measured.

**Collapse the event model to one rule.** The consume-class /
notify-class split plus `ctx.Bubble()` as an escape hatch is the most
confusing part of the API. Adopt: every callback starts unhandled; call
`ctx.Consume()` to stop propagation; delete `Bubble()` and the
auto-handled class. Cost is real — 23 `Bubble()`/`Consume()` sites in
`examples/` alone plus all sibling call sites — which is precisely why
it bundles with the signature work rather than shipping separately.

### 4.4 Color-set collapse — highest per-app line savings

`examples/todo/main.go:139-166` sets six `Color*` fields on one button
purely to stop it changing appearance on hover, focus, and click. That
is a defaults failure presenting as API surface. Introduce a shared
sub-struct:

```go
type ColorSet struct {
    Base, Hover, Click, Focus, Border, BorderFocus Color
}

// Flat sets every state to c — the common "don't react" case.
func Flat(c Color) ColorSet
```

Unset states fall back to `Base`, `Base` falls back to theme. Removes
~6 lines per styled widget.

**Precedence, when both a flat `Color*` field and a `ColorSet` are
set: the flat field wins.** This is the only rule that makes the
transition safe — existing code sets flat fields and must keep its
current appearance when a `ColorSet` default arrives, and a
partially-migrated literal should not silently change color. The rule
is unintuitive (the newer, more specific-looking API loses), so it goes
in the doc comment on both, not only here.

**The struct does not shrink in phase 3 — it grows.** An additive
`ColorSet` sits *alongside* the flat fields, so `ContainerCfg` gains a
field rather than losing six. Shrinking requires deleting flat fields,
which is breaking and was not scheduled anywhere. Scheduling it, with
one split that the measurement forces:

- **Delete in phase 4: the five state fields** — `ColorHover`,
  `ColorFocus`, `ColorClick`, `ColorBorder`, `ColorBorderFocus`
  (`gui/view_button.go:52-61`). These are exactly what `ColorSet`
  replaces, and they are what makes the `examples/todo` literal six
  lines long. Sibling cost is **8 sites, all in go-charts**; the other
  four siblings set none.
- **Keep `Color` permanently**, as shorthand for `ColorSet.Base`. It is
  the single-color case, it is the overwhelmingly common one, and
  `ColorSet{Base: c}` is strictly worse ergonomics for it than `Color:
  c`. Retaining it is not the `InputCfg` two-conventions defect (§3.1),
  because it is not a second way to say the same thing — it is the
  degenerate case with its own name, the same relationship `Flat(c)`
  has to a fully-specified `ColorSet`.

So the shrink claim survives, at five fields per styled `Cfg` rather
than six. Phase 3 lands `ColorSet` plus the precedence rule; phase 4
removes the five it replaced.

A caution on measuring this: a naive `^\s+Color[A-Za-z]*:` grep reports
173 sibling sites, which is off by 20x. Nearly all of them are
go-charts' **own** `Color` field taking a go-gui color *value*
(`Color: gui.Blue`) — the `gui.` on the line is the palette constant,
not the `Cfg`. Match the specific state-field names.

**Consider, but do not block on:** named theme-backed style presets. The
`examples/todo` button is really asking for "the accent style", not for
six specific colors. A small preset set (primary / secondary / chrome /
danger) may remove more lines than `ColorSet` alone, and the two
compose — `ColorSet` is the mechanism, presets are the vocabulary. Ship
`ColorSet` first; presets are a separate additive proposal.

### 4.5 `Opt[T]` consistency pass and value shorthands

110 `Opt[T]` fields coexist with plain-value fields under no documented
rule.

**Decision: `Opt[T]` where the zero value is a legitimate user choice
that must be distinguishable from "unset"; a plain field everywhere
else.** This was the last §4 item still phrased as an either/or, which
meant phase 3 could not start on it. The rule is not a style
preference — it is the only thing that distinguishes the two cases,
and `SizeBorder` is the worked example already in `CLAUDE.md`: a
border width of 0 is a thing a caller means, so a plain field cannot
tell "no border" from "not specified" and silently applies the theme
default. Where zero is not meaningful (most sizes, counts, and
indices), `Opt` costs a wrapper call and buys nothing.

Applying the rule is an audit of the existing 110, not a rewrite:
fields that satisfy it stay, fields that do not become plain in phase
4 with the other signature changes. Document the rule in `CLAUDE.md`
so new `Cfg` fields are decided at authoring time rather than by
whichever neighbor was copied.

Pair either way with cheap value constructors returning the same types
the current wrappers do — these are unaffected by the decision above:

```go
gui.PadAll(12)  // == gui.SomeP(12, 12, 12, 12)
gui.PadXY(8, 4)
```

### 4.6 App-testing API — largest additive gap

Apps built on go-gui cannot test behavior. They can only test that a
view function does not panic.

The query half of the story is fine: `FindByID`, `FindLayout`,
`FindShape`, `NextFocusable`, `PreviousFocusable` are all exported
(`gui/layout_query.go`). The **dispatch half is entirely unexported**.
`Shape.events` is a lowercase field (`gui/shape.go:19`) with zero
exported accessors, and there is no public entry point that walks a
layout tree and fires handlers.

The library's own tests reach straight through the field:

```go
// gui/view_button_test.go:36 — in-package, so this compiles
layout.Shape.events.OnClick(EventCtx{&layout, e, w})
```

An app author cannot write that line. The measurable consequence: 63
tests across `examples/*/main_test.go`, and 98 of their assertion lines
are some form of "call `GenerateLayout` and see if it panics." Zero
assert that clicking a button changed state.

This is the gap most worth closing, because immediate mode plus one
typed state slot makes go-gui apps unusually testable *in principle* —
a view is a pure function of state, so `state → tree → event → state'`
is fully deterministic with no backend, no event loop, and no clock.
None of that is reachable from outside the package.

Proposed surface, additive, no breaking change:

```go
// Build a headless window with injected interfaces left nil.
func NewTestWindow(cfg WindowCfg) *Window

// Render the current view to a tree, then fire a synthetic event at
// the widget with the given ID, running its handler chain.
func (w *Window) TestRender(view ViewFn) *Layout
func (w *Window) TestClick(id string) error
func (w *Window) TestKey(id string, k Key, mods Mod) error
func (w *Window) TestType(id string, text string) error

// Focus assertions. NextFocusable / PreviousFocusable are already
// exported (gui/layout_query.go), so these are thin wrappers.
func (w *Window) TestFocus(id string) error
func (w *Window) TestTab(dir TabDirection) (focusedID string, err error)

// Scroll injection and read-back. The offset getter is the load-bearing
// half: offsets live in an internal StateMap keyed by Shape.ID
// (gui/layout_position.go:71-75) and are unreachable from app code, so
// without it a scroll test can inject but cannot assert.
func (w *Window) TestScroll(id string, dx, dy float32) error
func (w *Window) TestScrollOffset(id string) (x, y float32, err error)
```

`TestTab` matters for phase 4 specifically: the focus unification
(§4.2) and the `Scrollable` work (§4.9) both change tab-order and
state-key behavior. Without a way to assert "tab from A lands on B",
that phase ships with no test that justifies it.

`TestScroll` is not optional either — **Q6's phase gate is
undischargeable without it.** Q6 requires writing the nested-scroll
case as a test in phase 2 and changing the propagation model against
it; an API with click, key, type, focus, and tab has no way to express
that test. The same pair is what pins §7.2's silent consume-class
regressions, which by construction produce no compile error.

Errors (not panics) on unknown ID, on a widget with no matching
handler, and on a disabled widget — those are assertion failures in a
test, not programmer errors at a render site.

Two open design points, listed in §9: whether this lives in `gui` or a
`gui/guitest` subpackage, and whether `TestClick` runs full hit-testing
from coordinates or targets the ID directly. Hit-testing is the more
faithful simulation and would also catch overlay and z-order bugs;
ID-targeting is simpler and sufficient for state-transition tests.

### 4.7 Naming: `RTF` / `RtfCfg` casing split

`RTF(cfg RtfCfg)` (`gui/view_rtf.go:212`) is the only factory whose
name disagrees with its `Cfg` in casing. Go convention initialises
acronyms uniformly, so this should be `RTF(cfg RTFCfg)`. Breaking
rename; fold into phase 4 where consumers already migrate.

**Also in phase 4: rename `OnCellFormat` and `OnDetailRowView` out of
the `On*` space.** §4.3 argues they are misnamed — they are view
builders that return a value, not event callbacks — but named no phase,
which would have meant either another breaking release or keeping a
misnomer §8 already expects the next reviewer to trip on. Phase 4 is
the only bus it can ride. Cost is zero outside the declaring package:
no reference to either identifier exists in any of the five siblings,
or anywhere in go-gui outside `gui/datagrid/`. Suggested `CellFormat`
and `DetailRowView`, matching the `Cfg`-field-as-builder convention
rather than the callback one.

The other twelve factory/`Cfg` name divergences are deliberate and
should stay: the container family (`Column`, `Row`, `Wrap`, `Canvas`,
`Circle`) intentionally shares `ContainerCfg`, and
`RadioButtonGroupColumn` / `RadioButtonGroupRow` follow the same
axis-variant pattern.

### 4.8 Example audit: `FillFill` vs manual viewport math

`examples/get_started/main.go:36` documents that `FillFill` removes the
need for `WindowSize()` and manual arithmetic. **45 example files call
`WindowSize()` anyway** — 50 call sites and 108 lines of
`float32(ww)-N` arithmetic. This is not one contradictory pair; it is
the dominant idiom in the examples, teaching the opposite of what the
library recommends.

Treat it as an audit, not a file fix:

1. Convert to `FillFill` wherever the size is only used to fill the
   window or subtract padding.
2. Keep an explicit allowlist for examples that genuinely need viewport
   numbers — canvas, particle, game, and shader demos that compute
   positions in pixel space. Document *why* in each.
3. Once converted, the remaining `WindowSize()` calls are a signal
   rather than noise.

Highest-leverage change in the plan for *perceived* ergonomics: the
examples are where the idiom is learned, and right now they teach
manual layout.

Also convert two or three example tests from no-panic assertions to
real state-transition assertions once §4.6 lands, so the testing
pattern is demonstrated rather than described.

### 4.9 Close the `Scrollable`/ID hole alongside focus

Same defect class as §4.2, found by the same audit. Scroll offsets are
keyed by `Shape.ID` (`gui/layout_position.go:71-75`):

```go
if v, ok := sx.Get(layout.Shape.ID); ok { x += v }
```

An empty ID is a valid map key, so **every ID-less scrollable in a
window shares the key `""`** and they scroll in lockstep. That is worse
than the focus hole: focus without an ID is inert, but scroll without an
ID is cross-widget state bleed.

Current coverage of the 7 `Cfg`s exposing `Scrollable bool`:

| Guard                    | Cfgs                                       |
| ------------------------ | ------------------------------------------ |
| `gui:"required"` tag     | Combobox, CommandPalette, ListBox, Table,  |
|                          | Tree                                       |
| `RequireScrollID` only   | Container (`gui/view_container.go:342`)    |
| **none**                 | **Input**                                  |

`RequireScrollID` (`gui/state_registry.go:143`) is **not** dead the way
`RequireFocusID` was — it has exactly one caller. So finish wiring it
rather than deleting it. The analyzer has zero `Scrollable` references.

One live defect found: `gui/view_select.go:145` builds the dropdown
container with `Scrollable: true` and no ID. Open two `Select`s in one
window, scroll one, and the other's offset follows.

Proposed, all additive except the tag:

- Tag `ID` on `ContainerCfg` and `InputCfg`; wire `RequireScrollID`
  into `Input`.
- Add a `checkScrollableID` rule to `tools/requiredid`, mirroring
  `checkFocusableID` — same shape, small diff.
- Add the `Scrollable && ID == ""` check to the §4.1 debug gate, which
  catches internal shapes the analyzer never sees.
- Fix `gui/view_select.go:145`.

Phase 1, alongside the focus work: same mechanism, same audit, and the
`Select` defect is live today.

`ContainerCfg` is the only type that is both opt-in-focusable and
scrollable (`Focusable` :97, `Scrollable` :102, `ID` :67). With §4.2's
opt-in collapse cut, that is unremarkable: both concerns key off the
same `ID` field, `checkScrollableID` needs to know nothing about focus,
and nothing here carries into phase 4.

## 5. Rejected

### 5.1 Positional auto-generated IDs

Proposed as "derive stable IDs from tree position at layout time, like
React keys; explicit `ID` becomes an override." **Reject.**

IDs are not merely focus tokens — they are the identity key for all
cross-frame widget state. 242 `StateMap[string, …]` sites key off
`Shape.ID`: scroll offsets (`gui/layout_position.go:71`), overflow
counts (`gui/layout_overflow.go:59`), dropdown open state
(`gui/layout_overflow.go:65`). Positional identity is stable only while
tree structure is stable — and React needs explicit keys *because* that
assumption fails on insert and reorder.

Concretely: insert a row at the top of a list and every row below
inherits the previous occupant's scroll position and open-dropdown
state. That trades a loud analyzer error for silent state corruption,
contradicting the proposal's own "never a silent no-op" principle. §4.2
addresses the real complaint without touching identity semantics.

### 5.2 Variadic modifier helpers

Proposed as `gui.Row(gui.Pad(8), gui.Gap(4), children...)`, claimed to
allocate nothing at build time. **Reject — the claim is inverted.**
Variadic interface parameters allocate a backing slice plus one boxing
allocation per modifier, per widget, per frame. That is the functional-
options allocation pattern the proposal claims to avoid. The view phase
is already the sole per-frame allocator (pipeline, arrange, and render
are zero-alloc), so this would worsen the one hot spot. Struct literals
allocate nothing extra; keep them and add value constructors (§4.5).

### 5.3 `State[T]` returning an error instead of panicking

Keep the panic. A window holding the wrong state type is a programmer
error discoverable at first render, not a runtime condition worth
threading through every view function. Improve the message instead —
report the type held and the type requested (§4.1).

## 6. Sequencing

| Phase | Contents                          | Breaking | Notes               |
| ----- | --------------------------------- | -------- | ------------------- |
| 1     | §4.1 gate, §4.2 delete + tag 9 +  | no\*     | closes 13 live      |
|       | fix 12 a11y defects, §4.9 scroll  |          | defects; go-gui only|
| 2     | §4.6 test API (incl. `TestTab`,   | no       | unblocks the rest;  |
|       | `TestScroll`)                     |          | discharges Q6 gate  |
| 3     | §4.4 `ColorSet`, §4.5 `Opt` rule  | no       | additive + fallback |
| 4     | §4.3, §4.7, flat `Color*`         | **yes**  | sibling migration   |
|       | removal (§4.4)                    |          |                     |
| 5     | §4.8 example audit                | no       | 45 files            |

**Versions.** Phases 1–3 are additive and ship as `v0.52.x` point
releases — siblings pick them up on a routine bump with no action.
Phase 4 is the single breaking release, **v0.53.0**. Phase 5 touches
only `examples/`, so it rides whatever release follows. Stated
explicitly because release-consumer decisions hang off which bumps
siblings see, and the header names only v0.53.0.

§4.6 moves ahead of the cosmetic work deliberately: a color-set
refactor (§4.4) and an event-model change (§4.3) both alter behavior
that apps currently cannot assert on. Landing the test API first means
the later phases ship with regression coverage instead of hoping the
examples still look right.

\* Phase 1 is non-breaking **for consumers**. It removes one exported
symbol with zero call sites anywhere (pre-1.0, no compatibility promise
below v1), and adding `gui:"required"` to the 9 `Cfg`s breaks only
go-gui's own callers — 111 of them, all in this repo's tests and
examples, surfaced by `requiredid` in CI rather than at runtime. The 3
sibling sites wait for phase 4 with the rest of the migration.

That last claim depends on siblings not running the analyzer, so it was
checked rather than assumed. `requiredid` is not a `go vet` plugin
registered by importing go-gui; it is a standalone binary invoked
explicitly (`Makefile:104` and `.github/workflows/ci.yml:155` both run
`go run ./tools/requiredid/cmd/requiredid ./...`). All five siblings run
plain `go vet ./...`, which does not load it. Tagging the 9 `Cfg`s
therefore cannot red a sibling's CI on a routine version bump — the
tags are inert in any repo that does not invoke the tool. If a sibling
later adopts `requiredid`, it adopts the backlog at that moment by
choice.

Phase 4 must be a single release. Three breaking event refactors in
consecutive versions is worse for consumers than one larger one.

## 7. Sibling impact

All five siblings pin `go-gui v0.52.0`; `main` is v0.52.1, so these
counts reflect the current API. Measured with `go/ast` walks over each
repo (`scratchpad/siblingimpact.go`).

| Item                       | Forced sibling edits         |
| -------------------------- | ---------------------------- |
| §4.2 default-on `ID`       | 3 (go-charts 1, go-map 2)    |
| §4.3a finish `EventCtx`    | **0**                        |
| §4.3b delete `Bubble()`    | 7 (go-edit 5, go-term 2)     |
| §4.7 `RTF` rename          | **0** — no sibling uses it   |
| §4.7 datagrid builders     | **0** — no sibling reference |
| §4.4 `ColorSet` (phase 3)  | 0 — additive with fallback   |
| §4.4 drop 5 state colors   | 8 (go-charts 8)              |
| §4.6 test API              | 0 — purely additive          |
| **Total**                  | **18 call sites**            |

Eighteen edits across five repos, all mechanical. The cost of the whole
breaking release is smaller than the cost of one of its parts was
assumed to be.

Two rows moved after review. The `ColorSet` row was `0` until the
flat-field deletion was scheduled (§4.4); an additive-only reading
scored it zero and quietly deferred the real number — `Color` is
retained, so only the five state fields count. The `gui.Focus` row was
11 until §4.2's opt-in collapse was cut; those 11 sites are correct as
written today and now require no edit at all. **Only 3 of the remaining
18 come from §4.2**, and none of those 3 is a breaking API change —
they are IDs that should have been supplied anyway.

### 7.1 How the §4.3 boundary was measured

§4.3 states the conclusion; this is the derivation. A naive count finds
38 `*Window`-tailed callback literals in the siblings. None is forced
work:

| Category                              | Sites | Verdict           |
| ------------------------------------- | ----- | ----------------- |
| `OnInit` (`WindowCfg` lifecycle)      | 13    | keep `*Window`    |
| `OnDone` / `OnValue` (anim, dialog)   | 12    | keep `*Window`    |
| go-map's own `OnClick`                | 10    | not go-gui's API  |
| go-edit's own `OnFileDrop`            | 3     | not go-gui's API  |

The last two are sibling-defined fields that merely mirror go-gui's
older convention — `go-map/mapview/overlay.go:104` declares
`OnClick func(*gui.Window)`, and `go-edit/edit/editor.go:53` declares
`OnFileDrop func(path string, w *gui.Window)`. Renaming go-gui's
signatures does not touch them. They become *stylistically* out of step,
which is a follow-suit invitation, not a migration.

The first two categories are what drove the §4.3 scope correction: they
are go-gui's own callbacks, and they legitimately have no event to
carry. §4.3 now lists the 14 excluded signatures directly.

### 7.2 The §4.3b risk is silent, not loud

Deleting `Bubble()` breaks 7 sites loudly — a compile error the
consumer cannot miss. That is the safe part.

The unsafe part is unmeasurable by counting: under the one-rule
collapse, a consume-class handler that relied on dispatch's automatic
handled-marking and never called `Consume()` will start bubbling. That
is a **behavior change with no compile error**. Siblings already call
`Consume()` 71 times, so the idiom is well established, but any handler
that omitted it because the auto-mark made it unnecessary changes
meaning silently.

This is the strongest argument for landing §4.6's test API first
(phase 2): event-propagation regressions are precisely what an app-level
test can catch and a compiler cannot.

### 7.3 Reproducing these counts

Every figure in this spec comes from `tools/ergoaudit`, checked in:

```
go run ./tools/ergoaudit/ -mode focus     -gui . . ../go-charts ../go-edit ../go-kite ../go-term ../go-map
go run ./tools/ergoaudit/ -mode callbacks -gui . . ../go-charts ../go-edit ../go-kite ../go-term ../go-map
```

or `make ergo-audit` for the go-gui-only run. Mode `focus` **derives**
the unguarded `Cfg` set from the source instead of hardcoding it, so the
numbers track the code — and the per-file scan that once misreported
`ListBoxCfg` cannot recur.

## 8. Doc deliverables

- `README.md` — color-set and padding-shorthand examples, and a
  "testing your app" section (§4.6)
- `CHANGELOG.md` — per phase
- `CLAUDE.md` — event-model section rewritten for the single rule (§4.3)
- `docs/specs/eventctx-callback-refactor.md` — mark superseded by §4.3
- `docs/specs/idfocus-to-focusable.md` — decision 3 cites `RequireID`
  enforcement for `Focusable: true`; amend to record that the runtime
  guard was never wired up and is now deleted in favour of the analyzer
  plus the debug gate (§4.2)
- **Phase-4 migration guide** (new, `docs/migration-v0.53.md` or
  similar). 18 sibling sites plus ~126 ID fills are mechanical, and
  §7.2's silent half is not — a before/after for `Focus`, `Consume`,
  and `ColorSet` is cheap insurance.
- **`ergoaudit -fix`** (new, phase 1). Not "consider" — commit to it.
  Phase 1 alone rewrites 111 internal literals to add `ID` fields, and
  phase 4 adds ~15 more; that is a week
  of mechanical edits done by hand, and hand-editing 111 literals is
  how a typo'd ID reaches `main` looking like intent. The `go/ast`
  `CompositeLit` walk that finds them is already written and tested in
  `tools/ergoaudit`, so `-fix` is an insertion pass on top of the
  existing classifier, not a new tool. IDs derive from the enclosing
  file and variable name and are **written into the source literal** —
  this does not reopen §5.1, which rejects IDs *computed at runtime
  from tree position*. A generated ID in the file is an ordinary ID
  that a human can read, review, and edit; the rejected design has no
  source-level existence and changes when a sibling is inserted. If the
  codemod is worth shipping for siblings, it is worth running on the
  111 first — where its output is reviewable in the same PR that adds
  the tags.
- **Two callback families, documented as intentional** (godoc +
  `CLAUDE.md`). Event-driven takes `EventCtx`; lifecycle, animation,
  and completion take `func(T…, *Window)`. Writing this down is what
  stops the next reviewer reopening §4.3 as "unfinished `EventCtx`".
- **How to run `requiredid` in your own build** (new, `README.md` plus
  the phase-4 migration guide). Phase 1 is the release that makes
  `gui:"required"` load-bearing, and it is enforcement only in repos
  that invoke the analyzer — see §4.2. Give authors the one-line
  invocation, the `go vet -vettool=` and golangci-lint plugin
  alternatives, and one sentence on what they get without it: a
  `RequireID` panic on first render rather than a build failure. This
  is the cheapest item on the list and the one that decides whether the
  §4.2 enforcement story is true for anyone but this repo.
- affected per-example `README.md` files

## 9. Open questions

Two independent reviews agreed with every recommendation below, so Q1–Q7
are decisions rather than questions. Q6 remains a **phase gate**: agreed
in approach, but it must be discharged by work in phase 2 before phase 4
can proceed. Q8 is the one item genuinely unresolved.

| #   | Topic                    | Decision                             |
| --- | ------------------------ | ------------------------------------ |
| 1   | Test API location        | package `gui`                        |
| 2   | Click model              | ID-targeting v1; `TestClickAt` later |
| 3   | Nested focus             | unexported `Shape` helper            |
| 4   | Focusable without ID     | proceed; mandatory `ID`              |
| 5   | `ColorSet` zero value    | `Opt[Color]`                         |
| 6   | Nested `OnMouseScroll`   | **gate** — test first, in phase 2    |
| 7   | Breaking release target  | v0.53.0                              |
| 8   | `requiredid` for authors | **open** — documented vs. supported  |

Detail where the decision carries a constraint:

1. **Test API in `gui`.** A `gui/guitest` subpackage cannot reach
   `Shape.events` without exporting it, permanently widening the public
   surface to serve testing. Four exported names in an
   already-953-symbol package is the cheaper trade.
2. **ID-targeting for v1.** Answers the state-transition question that
   63 example tests cannot answer today. Hit-testing arrives later as a
   separate `TestClickAt(x, y)` — never by changing `TestClick`'s
   meaning, which would silently reinterpret existing tests.
3. **Nested focus via an unexported `Shape` helper.** Compound widgets
   that mark an internal child focusable get an internal path to
   propagate an ID derived from the parent's, so `Cfg.ID` stays the
   only public spelling.
5. **`Opt[Color]` for `ColorSet`.** `Color` has no reserved zero, so a
   plain field cannot distinguish "unset, inherit from `Base`" from
   "deliberately transparent" — and fallback is the entire point of the
   type. Keep literals short with `Flat(c)` for the all-states case and
   a `Some`-style constructor for individual fields; if
   `ColorSet{Base: gui.Some(c)}` proves noisy in practice, add a
   `gui.Col(...)` shorthand rather than dropping `Opt`.
6. **Nested scroll is the one real risk.** Under the one-rule collapse a
   nested scrollable that today relies on notify-class propagation to
   hand an unconsumed scroll to its parent changes behavior with **no
   compile error** — the silent class from §7.2. Discharge it by writing
   the nested-scroll case as a test in phase 2, then changing the model
   against it. That test is only writable if phase 2 ships `TestScroll`
   and `TestScrollOffset` (§4.6) — without an offset read-back the case
   can be injected but not asserted, and the gate cannot be discharged.
   This is also why §4.9 belongs in phase 1: scroll-state keying and
   scroll propagation should not both be in motion at once.
7. **v0.53.0** for the breaking phase; phases 1–3 as `v0.52.x`. All 29
   sibling edits land in one release; see §6.
8. **`requiredid` for app authors — open.** Phase 1 makes
   `gui:"required"` load-bearing, but it is enforcement only where the
   analyzer runs, and today that is this repo alone (§4.2). Documenting
   the one-line invocation (§8) is agreed and cheap. What is not
   decided is whether the analyzer becomes **supported** surface:

   - **Documented only.** It stays a `tools/` binary that authors may
     invoke. Its rules can tighten freely, because nothing promises
     they will not. Costs nothing; leaves most consumers on the
     `RequireID`-panic path by default.
   - **Supported.** Named in the README as part of the recommended
     setup, versioned with the library, plausibly a `go tool`
     directive. Makes §4.2's static-enforcement claim true for
     everyone — and makes the analyzer's rules API. Tightening a rule
     then breaks somebody's build on a patch bump, which is exactly
     the failure mode `deps-doc` and the alloc gates exist to avoid.

   Not resolved here because it is a support-commitment question, not a
   technical one, and it does not block any phase: the documentation
   deliverable is the same either way. Decide before phase 1 ships,
   since that is the release the tag starts mattering in.

## 10. Counting rules

§1's figures are not re-derivable without these. All against `main` @
`80715d1`, `_test.go` excluded unless stated.

**Callback declarations — the unit is a (field name, signature) pair.**
`ergoaudit -mode callbacks` walks `gui/` with `go/ast`, collects every
exported `On*` field whose type is a `func`, and keys the dedupe on the
field name joined to the `go/printer` rendering of its type. So
`OnDone func(*Window)` and `OnDone func(NativeAlertResult, *Window)`
are two entries, and an identical shape declared under two names stays
two entries. **136 raw declarations reduce to 70 distinct pairs.**
Scope is `gui/*.go` plus `gui/*/*.go`; `_test.go` excluded.

Text dedupe does not reproduce this. An earlier `grep | sort -u` pass
reported 120, because gofmt aligns field types to the widest name in
each struct, so one signature counts once per distinct indentation.
`TestRenderExprNormalises` pins the AST rendering against that.

**Shape breakdown.** Of the 70 distinct pairs: **16** bare
`func(EventCtx)`, **19** `func(T…, EventCtx)`, **27** with a trailing
`*Window`, **6** exposing a raw `*Event`, **2** in none of these
(`OnAction func(id string)` and `OnDraw func(*DrawContext)`, which take
neither a window nor a context). The two view builders `OnCellFormat`
and `OnDetailRowView` are *not* in that last bucket: they end in
`*gg.Window` and so count inside the 27, which is where §4.3 excludes
them. Of the 27 `*Window`-tailed, **14 are excluded** by §4.3 as
lifecycle/dialog callbacks that have no event to carry, leaving **13
to convert**. §4.3 lists all 14 by name; that list, not this count, is
the phase-4 work item.

The raw-`*Event` figure is 6, not the 5 a `grep '\*Event'` finds: two
are declared in the `datagrid` subpackage as the qualified `*gg.Event`
(`OnColumnPinChange`, `OnCopyRows`). `baseType` strips the package
qualifier; `TestBaseType` pins it. Counted as declaration *lines*
instead of pairs the figure is 7 — `OnEvent func(*Event, *Window)` is
declared twice (`gui/window_cfg.go:14`, `gui/window.go:168`) with an
identical signature and collapses to one pair.

**`Opt[T]` fields — approximate.** 110 counts occurrences of `Opt[` in
`gui/view_*.go` field position, which includes non-`Cfg` structs. Treat
as an order-of-magnitude figure for "how much of the surface uses
`Opt`", not an exact field census.

**`Color*` field names — approximate.** "20+" counts distinct
`^\tColor[A-Za-z]*` field names across `gui/view_*.go` and does not
separate `Cfg` fields from helpers. The load-bearing claim is the six
fields on one `ButtonCfg` literal in `examples/todo/main.go:139-166`,
which is exact.

**`Focusable` groups.** 12 and 15 count *`Cfg` types*. The raw
`Focusable bool` inventory also matches `Shape` (`gui/shape.go:120`),
`termgrid`, and `inspector`, which are excluded as not app-facing.

**Required-tag audit.** Per-struct, not per-file: `awk` scoped to the
struct containing the `FocusDisabled` field. A file-level `grep -m1`
gives the wrong answer where a file declares several `Cfg` types —
that error is what originally misclassified `ListBoxCfg` as untagged.

**Literal audit (§4.2, §7).** `go/ast` `CompositeLit` walk over each
repo, skipping `vendor/`, `.git/`, `testdata/`. `ID` counts as present
unless absent or the literal `""`; a computed `ID` that evaluates empty
at runtime counts as present, so 126 is a floor. Tests included and
labelled separately.

**`WindowSize()` audit.** Call *expressions* under `examples/`,
excluding `_test.go` and excluding the one occurrence in prose — the
comment at `examples/get_started/main.go:36` that this section quotes.
Counting that line as a call gives 46/51 instead of 45/50.
