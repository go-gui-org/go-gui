# CLAUDE.md

Guidance for Claude Code (claude.ai/code) in this repo.

## Commands

```
go run ./examples/get_started/  # run the example app
make prepush                    # full gate (race, cross-lint, cross-compile, coverage, export audit)
                                # .githooks/pre-push runs prepush
make check-all                  # test + lint + check, one step after another
make check                      # fast gate (vet, deps-doc, large-files, generate/tidy/fmt-md/changelog checks, widget-ID audit)
make test / lint / vet          # individually
make fmt-md                     # Prettier over tracked .md (.prettierrc holds the flags)
make ergonomics-audit           # focus/callbacks/opt/ids/literals/theme/a11y/visual/deadcfg gates
make export-audit               # exported surface (advisory in-repo)
git config core.hooksPath .githooks  # enable tracked hooks
```

## Architecture

Immediate-mode pipeline. No virtual DOM, no diffing:

```
View fn → generateViewLayout() → Layout tree
  → layoutArrange() (Fit/Fixed/Fill sizing)
  → renderLayout() (emits into w.renderers)
  → Backend (Metal on macOS; native GL on Linux/Windows)
```

- **Packages stay flat.** Only leaf subsystems (`svg/`, `datagrid/`,
  `markdown/`, `backend/`, …) get subpackages; `gui/` holds widget factories,
  layout, theme, animation, event dispatch, state. No test backend package —
  tests run with nil injected interfaces.
- **`View`** is an interface (`Content() []View`,
  `GenerateLayout(*Window) Layout`). Factories return `View`, not `*Layout`;
  `*Layout` does not implement it. Tests reach a widget's shape via
  `v.GenerateLayout(w).Shape`.
- **State:** one typed slot per window, no globals or closures.
  `gui.State[T](w)` type-asserts and **panics** on a type mismatch.
- **Sizing:** a `FillFill` root fills the window (min=max seed in
  `updateLayout`).
- **Injected interfaces** (nil in tests): `TextMeasurer` (glyph metrics),
  `SvgParser`, `NativePlatform` (dialogs, notifications, print, a11y, IME,
  titlebar).
- **`glyph`** (text shaping) is a versioned module; a `go.work`
  (`use (. ../go-glyph)`) points local builds at `~/Documents/github/go-glyph`.
  No `replace` directive. For text work check glyph first.

## Widget and API rules (short form)

These rules apply to all code that calls `gui`, including `examples/` and the
sibling repos. The full rules, internals and failure modes are in
`gui/CLAUDE.md`. Claude Code loads that file when you work under `gui/`. Read it
before you change a widget, identity, theme, event or layout-hook code.

- **Cfg and callbacks.** Every widget takes a zero-initializable `*Cfg`.
  Callbacks are `func(EventCtx)`. Examples: `docs/dx-cheat-sheet.md`.
- **Focus.** Focus needs `Focusable` **and** a non-empty `ID`. Input controls
  are focusable by default. To opt out, set `FocusDisabled`. Never set
  `Focusable: false`.
- **Identity.** Public APIs (`SetFocus`, `FindByID`, `Test*`) take the
  **effective** ID: the leaf joined to its ID-bearing ancestors. Compose IDs
  with `gui.ScopeID` / `gui.ScopeIDN`, never by hand. Read an effective ID back
  with `w.ResolveID`; do not spell it.
- **Accessibility.** Set `A11YLabel` / `A11YDescription` through the embedded
  `A11YCfg`. Never redeclare them on a Cfg.
- **Opt, colors, literals.** Use `Opt[T]` only for a primitive whose zero value
  is a real choice (`SizeBorder`). `Color`, `Padding` and `Sizing` flag
  themselves: build them with their constructors (`RGB`, `PadAll`, `FillFit`),
  never raw literals. On a Cfg, use plain `Color`, not `Opt[Color]`.
- **Visual roles.** Never write a de-emphasis alpha, a label size step or a
  control text inset at a call site. Use the `Theme` roles
  (`docs/style-guide.md`). A structural wrapper sets `SizeBorder: NoBorder`.
- **Events.** Nothing is consumed for you. A callback that acts on an event
  calls `ctx.Consume()`.
- **Frame lock.** `AmendLayout` runs under `w.mu`. Do not call `SetFocus`,
  `SetView` or other window-mutating APIs from it; use
  `ctx.Window.QueueCommand`. To move an element that has children, use the float
  fields, not `AmendLayout`.
- **Theme.** Outside generation, read `w.Theme()`. Key on `Theme.id`, never
  `Theme.Name`. Never add an initializer to a `default*Style` var.
- **Debug first.** Before you audit a layout by hand, run with `GOGUI_DEBUG=1`.
  In tests, assert `w.TestFindings(mask)`.

## Design before code

**For any change that adds or reshapes API surface, present candidate designs
first and stop.** Applies to a new widget, a new `Cfg` field or exported
function, a change to layout/event/identity semantics, anything warranting a
`docs/specs` entry, and any task whose issue does not already fix the approach.
A bug fix at a known site, a test, or a doc edit skips this.

The pre-code deliverable is:

1. A table of 2-3 candidate designs, one row each, with the tradeoffs that
   separate them (allocation cost, exported surface added, migration burden on
   the siblings, how it fails when misused).
2. The one to pick, named, with the reason.
3. What is **explicitly rejected** and why — the alternatives discarded, not
   only the ones carried forward.

Then stop and wait. Do not write code, do not scaffold, do not "start with the
uncontroversial part". A rejected direction already built is wasted work.
Rejections belong in the issue and later in the `docs/specs` entry, so the next
reader does not re-propose them (`## Rejected Approaches` is the long-lived
form).

**Number implementation steps explicitly.** Never "in reverse order", "the
opposite of the above", or "same as before but backwards" — spell out step 1, 2,
3 in execution order.

## Definition of done

**Every user-facing change carries a `CHANGELOG.md` entry under
`## [Unreleased]`, written in the same pass as the code — not as cleanup before
a release.** Match the existing format: a Keep a Changelog section (`### Added`
/ `### Changed` / `### Fixed`), a bolded one-line summary, the issue or PR
number, and prose on what now happens and why it matters. A breaking change goes
under `### Changed` as `- **BREAKING: <what> (#N)**` with the migration spelled
out. User-facing means the exported surface, observable behaviour, or a
documented default. `make check` runs `changelog-entry-check`, which fails a
branch that changes exported declarations or release packaging without touching
`CHANGELOG.md`; the escape hatch is `changelog: skip` in a commit message.

**Code quoted in a doc is wrapped in `doc:snippet` markers and pinned by a
test.** A guide that pastes Go so a reader can copy it compiling must quote a
marked region byte-for-byte, never paraphrase it:

```go
// doc:snippet-begin <name>
...
// doc:snippet-end <name>
```

Enforcement is per-guide, not automatic.
`examples/showcase/sound_snippet_test.go` is the working example — it hardcodes
one source file, one marker name and the guides that quote it, and fails when
the two drift either way. **A new snippet needs its own test, or that test
generalized to take a table of (source, marker, guides).** Markers alone check
nothing.

## Coding Conventions

- **No variable shadowing.** Never `:=` redeclare a var from an outer scope. Use
  `=`, or pick a distinct name.
- Committed code must pass `golangci-lint run ./...` and `gofmt`. A PostToolUse
  hook auto-runs lint-fix + tests on every .go edit.
- **Minimal scoped diffs.** Touch only what the request needs. No cosmetic
  comment/formatting churn, no drive-by edits. Rename/regex passes must not
  alter comment prose (for example, apostrophes in possessives).

## Verification

- Rebuild AND run the relevant tests before claiming a fix works. Never report
  success on an unverified change. State failures plainly with the output; if a
  step was skipped, say so.
- **Visual claims get recorded, not read.** `gui/golden_test.go` builds a
  widget, drives the real frame pipeline and diffs the emitted `[]RenderCmd`
  against `gui/testdata/`, in both `ThemeDark` and `ThemeLight`. Re-record with
  `go test ./gui/ -run TestGolden -update` **after reading the diff**. Reading
  source is not equivalent: `GenerateLayout` output is taken before
  `layoutDisables` and before arrange, so it shows neither inherited `Disabled`
  nor resolved geometry. Add a case for any widget whose appearance a change can
  move; set `focusID` on it to record a focus state.
- After touching the exported surface of `gui/`, run `make export-audit`: every
  export must be referenced from outside `gui/` or carry a `// exportaudit:keep`
  marker. The consumer scan is authoritative, the in-repo run advisory;
  `internal`-class exports are accepted by policy.
  (`docs/specs/exportaudit-surface-policy.md`)
- Native/CGo or focus/activation bugs: confirm root cause with instrumented
  logging (evidence) before editing. Reproduce before, verify the symptom gone
  after. Never leave the app non-launching. See `gui/backend/CLAUDE.md` for the
  two-sided logging technique.
- CI signals: distinguish runner noise (CPU variance, ns/op jitter) from real
  regressions. Alloc gates stay hard, timing gates are advisory.
- Before reviewing or editing a branch, confirm it is rebased on the current
  base branch. If stale, update first.

### Cross-repo root-causing

**A fix that does not hold is a signal the state lives in another module.**
Before proposing a second fix at the same site, dispatch a subagent to trace
where the state actually lives across the chain — `go-glyph` (upstream),
`go-gui`, and the consumers (`go-charts`, `go-map`, `go-edit`, `go-kite`,
`go-term`, `go-speedtest`). This is one of the few sanctioned uses of the Agent
tool in this repo.

The subagent reports and edits nothing: the **owning module** and the
`file:line` the state is declared and mutated at, whether the defect is
**upstream** or in the **consumer**, and what a fix at each layer would change.

Two failure shapes motivate this. State persisting in a dependency's `StateMap`
survives a reset the consumer performs, so patching the consumer looks correct
and changes nothing. And a `go.work` build resolves siblings from local working
trees, so a trace run without `GOWORK=off` can name a module version CI never
compiles — see the `sync-siblings` skill.

## Rejected Approaches

- **WebGPU backend** — explored and rejected. Do not re-propose.
  `gui/backend/gl/` already has no cgo (X11 via xgb, EGL via purego, Win32 via
  syscall).
- **Current CGo state:** `CGO_ENABLED=0 go build ./...` is green on Linux and
  Windows for the whole module. macOS (5.9k lines ObjC) stays cgo **by
  decision** (2026-08-12); do not re-open without a trigger.

Full history and rationale in `docs/specs/cgo-free-backend-feasibility.md`.

## Specs

Specs go in `docs/specs/`; issue first, spec after.
