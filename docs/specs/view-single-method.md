# Single-method `View`: collapse the two child mechanisms

- Status: **ready to implement** — all open questions closed 2026-08-14
- Area: `gui/` core pipeline
- Blocked on: go-gui-org/go-charts#41 (before the release tag, not before
  step 1)
- Related: #306 (resolved — Form scopes its children. See §5)

## Summary

`View` has two ways to express children. Nearly every widget uses only one of
them, and the one they use is not the one the interface advertises. Fold
`Content() []View` into `GenerateLayout`, leaving:

```go
type View interface {
	GenerateLayout(w *Window) Layout
}
```

The child walk, the ID-scope push/pop, the arena reservation, and the
`maxEventChildren` cap move from `generateViewLayout` into a shared helper that
the three container-shaped views call.

**No changes to the declarative surface.** `ContainerCfg.Content []View` — the
field — is untouched, so `examples/get_started/main.go` is byte-for-byte
identical before and after. Only the interface method of the same name goes
away.

**That is still a breaking API change**, in two directions:

- _implementing_ `View` — trivial, 24 sibling impls are all `{ return nil }`
- _calling_ `Content()` as an accessor — **not** trivial. 21 call sites exist
  downstream, and two of them cannot be mechanically migrated. See "Downstream
  impact".

## Motivation

### The two mechanisms

`generateViewLayout` (`gui/view.go:59`) calls `view.GenerateLayout(w)` for the
node, then walks `view.Content()` for children. A widget can therefore supply
children two ways:

1. return them from `Content()`, and let the framework recurse
2. ignore `Content()`, build the subtree inline, and self-recurse by calling
   `generateViewLayout(...)` on the assembled tree

Mechanism 2 is what every non-trivial widget does, because `Content()` is called
_after_ `GenerateLayout` (`view.go:60` then `:62`) and takes no `*Window`. A
widget whose children depend on state — `isOpen`, a filtered list, a selected
tab — cannot express them through `Content()` at all.

### The count

In `gui/`, 27 types implement `Content() []View`. **21 are the literal one-liner
`{ return nil }`.** Of the six that return something, three are test stubs
(`stubView`, `nilShapeStubView`, `benchView`). Three are production:

| Type             | Site                      | Returns                         |
| ---------------- | ------------------------- | ------------------------------- |
| `containerView`  | `gui/view_container.go`   | `cv.content`                    |
| `formView`       | `gui/view_form.go:192`    | `fv.content`, then truncates it |
| `rotatedBoxView` | `gui/view_rotated_box.go` | `[]View{v.content}` — allocates |

`viewFunc` (`view.go:23`) returns `nil` and self-recurses, making it a third
deferral primitive layered on the same seam.

So the framework-recursion path exists to serve three types, and one of those
allocates a one-element slice per node per frame to use it.

### `formView` is a third mechanism that defeats the other two

Verified by inspection of `gui/view_form.go:194-258`. It does not merely inject
children before returning — it opts out of the framework walk by mutating
itself:

```go
	layout := inner.GenerateLayout(w)   // :246 — bypasses generateViewLayout
	// Clear content so outer generateViewLayout does not
	// double-process children.
	fv.content = fv.content[:0]         // :249
	for _, child := range children {
		layout.Children = append(layout.Children, generateViewLayout(child, w))
	}
```

Two things follow, and only the first is documented:

1. Calling `inner.GenerateLayout(w)` rather than `generateViewLayout(inner, w)`
   skips `ensureLayoutShape` and the scope push for that node.
2. Truncating `fv.content` makes the later `view.Content()` read at `view.go:62`
   return length 0, so `scoped := w != nil && len(children) > 0` (`view.go:87`)
   is **false** and no scope is pushed at all.

`inner`'s shape carries `ID: formLayoutID(formID)` — `"form:" + formID`
(`view_form.go:138,320`), always non-empty — so on the framework path
`childScopeID` pushes, and form children resolve as `form:myform:fieldname`. On
this path they resolve in the form's **enclosing** scope instead.

#### The scope suppression is accidental, and protects nothing

An earlier draft of this spec asserted the flattening was required because
"field registration keys on plain field IDs." **That is wrong.** Two separate
namespaces are in play, and they never meet:

| Namespace                     | Example                    | Keyed by                  | Goes through `resolveShapeIDs`? |
| ----------------------------- | -------------------------- | ------------------------- | ------------------------------- |
| `FormFieldAdapterCfg.FieldID` | `"username"`               | `formRuntimeState.fields` | **no**                          |
| child widget `Shape.ID`       | `"showcase-form-username"` | effective-ID resolution   | yes                             |

`examples/showcase/demo_input.go:373-390` uses both, and they are different
strings: the `Input` carries `ID: "showcase-form-username"` while its adapter
carries `FieldID: "username"`. Fields enter the registry only through explicit
`FormRegisterFieldByID(w, formID, cfg)` / `FormOnFieldEvent` calls
(`view_form_process.go:326,339`) and are read back by
`w.FormFieldState(formID, fieldID)`. **No form field key is ever produced from a
`Shape.ID`, so no amount of scoping can affect the field registry.**

What the truncation actually suppresses is the `form:<formID>` prefix on the
**child widgets'** effective IDs — reaching `SetFocus`, `FindByID`, scroll
offsets, and the `Test*` APIs, not forms at all.

And that suppression is accidental. Git dates it:

- the truncation is present by **2026-06-08** (`2fda77b7`), when
  `generateViewLayout` had no scope push to suppress
- `viewState.idScope` and `childScopeID` arrive **2026-08-09** in `70a7a399`
  (#228, per-scope widget ID uniqueness) — two months later

So a line written to prevent double-processing silently acquired a second
effect. Its comment still mentions only the first, and no test covers the
second: `gui/view_form_test.go` contains zero `SetFocus` / `FindByID` / `idKey`
assertions.

Note also that `formLayoutID(formID)` returns `"form:" + formID`, which contains
`IDSep` and is therefore **absolute** — `resolveLeaf` (`id_resolve.go:52`) and
`joinLeaf` (`:85-87`) pass it through unjoined. That is deliberate, so
`formDecodeLayoutID` can reverse-parse it out of `Shape.ID` during
`formFindAncestorID`'s parent walk — the same documented pattern as
`gui/datagrid`. It means that if the scope push runs, children resolve under
`form:<formID>` regardless of where the form itself sits.

#### Separately: the truncation is a latent bug

Independent of scoping, `fv.content = fv.content[:0]` is destructive
self-mutation on a value the interface documents as stateless (`view.go:5`). It
is survivable today only because `Form()` deep-copies `cfg.Content` into
`fv.content` (`:187-188`) and the caller rebuilds the `formView` every frame. A
cached `formView` — the pattern `comboboxView` and `commandPaletteView` already
use for dropdown views — renders empty children from frame 2 onward. This
refactor removes the line entirely, which is reason enough to do it regardless
of how the scoping question lands.

The other two direct `.GenerateLayout(w)` callers, `view_image.go:65,157` and
`view_svg.go:184`, are childless leaf views and are unaffected.

### The costs already being paid

- **`containerView` is a shim.** Widgets that self-recurse (`comboboxView` at
  `view_combobox.go:296`, `commandPaletteView` at `view_command_palette.go:177`,
  `tabControlView` at `view_tab_control.go:363`, `themePickerView`,
  `selectView`, `colorPickerView`) all funnel into `containerView` purely to get
  back onto the `Content()` path they just left.
- **`maxEventChildren` truncates silently, and only on one path.**
  `view.go:63-65` caps `Content()` children at 10000. Children injected directly
  into `layout.Children` are uncapped.
- **The merge is load-bearing but undocumented.**
  `wantCap := len(layout.Children) + len(children)` (`view.go:70`) exists
  because `containerView.GenerateLayout` calls `addGroupBoxTitle`, which appends
  floating eraser + text children to `layout.Children` before `Content()`
  children are appended after them. Order matters and nothing states it.
- **One interface dispatch per node per frame** (~2.5 ns, measured) for a method
  that returns `nil` 21 times out of 27. A dispatch, not an allocation — see
  Unresolved #3.

This is a mechanism cleanup, not a performance change. The only concrete
allocation removed is `rotatedBoxView`'s per-node slice.

## Non-goals

Explicitly **not** proposed: eliminating `View` in favor of building `Layout`
directly from factories. That variant was evaluated and rejected. It requires
`w` threaded through every factory call (`gui.Column(w, ...)`,
`gui.Text(w, ...)`), forces every user theme read from `gui.CurrentTheme()` to
`w.Theme()`, breaks `Themed(t, build)` — whose contract is that `build` runs at
generation time — and unships the per-window theming landed in #296/#302. See
"Rejected alternative" below.

## Design

### Interface

```go
// View is a user-defined view. Views are never displayed directly; a
// Layout is generated from the View. GenerateLayout returns the node's
// complete subtree — a view with children generates them itself, via
// appendChildViews.
type View interface {
	GenerateLayout(w *Window) Layout
}
```

### `generateViewLayout` shrinks to the invariant it owns

```go
func generateViewLayout(view View, w *Window) Layout {
	layout := view.GenerateLayout(w)
	ensureLayoutShape(&layout)
	return layout
}
```

Its remaining job is the `ensureLayoutShape` normalization that every pipeline
pass depends on. `GenerateViewLayout` (exported) keeps delegating to it.

### `appendChildViews` — the single child-append path

New in `gui/view.go`. Everything `generateViewLayout` used to do for children
lives here, unchanged in behavior:

```go
// appendChildViews generates children under parent's ID scope and appends
// them to parent.Children, after whatever the parent already put there.
//
// Ordering matters: the parent's own shape must be built before this is
// called, because the parent's ID resolves in the *enclosing* scope while
// its children resolve under it. Children the parent injected directly
// (addGroupBoxTitle's eraser + label) keep their position ahead of these.
func appendChildViews(w *Window, parent *Layout, children []View) {
	appendChildViewsScoped(w, parent, children, true)
}

// appendChildViewsFlat appends children WITHOUT pushing the parent's ID
// scope, so they resolve in the parent's enclosing scope.
//
// Exactly one caller: Form, and only to preserve today's observable
// effective IDs. Form's own field registry is NOT affected either way —
// FormFieldAdapterCfg.FieldID never passes through ID resolution. What
// scoping would change is the effective ID of every widget the caller
// put inside the form, and therefore SetFocus/FindByID against it.
//
// Form is therefore the one container whose children do not take its
// scope. That is deliberate here only in the sense that this refactor
// preserves it; whether it should hold at all is issue #306. This helper
// is the single place that decision would change.
//
// Introduced in step 3b, not 3a — see § Implementation order.
func appendChildViewsFlat(w *Window, parent *Layout, children []View) {
	appendChildViewsScoped(w, parent, children, false)
}

func appendChildViewsScoped(
	w *Window, parent *Layout, children []View, scope bool,
) {
	// PRECONDITION: parent.Shape is non-nil. All three callers build it
	// before calling; childScopeID reads it immediately below, so a
	// normalization here would only move the nil deref one line down.
	if len(children) == 0 {
		return
	}
	if len(children) > maxEventChildren {
		children = children[:maxEventChildren]
	}
	// Pre-size so append never reallocates; the reservation comes from the
	// frame-scoped arena (reset in resetViewPools) to avoid a per-node heap
	// allocation.
	wantCap := len(parent.Children) + len(children)
	if cap(parent.Children) < wantCap {
		var grown []Layout
		if w != nil {
			grown = w.scratch.takeLayoutChildren(wantCap)
		} else {
			grown = make([]Layout, 0, wantCap)
		}
		grown = grown[:len(parent.Children)]
		copy(grown, parent.Children)
		parent.Children = grown
	}
	// Saved and restored rather than recomputed on the way out: a sibling
	// must not inherit a child's scope.
	saved := ""
	pushed := scope && w != nil
	if pushed {
		saved = w.viewState.idScope
		w.viewState.idScope = childScopeID(w, saved, parent.Shape)
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		parent.Children = append(
			parent.Children,
			generateViewLayout(child, w),
		)
	}
	if pushed {
		w.viewState.idScope = saved
	}
}
```

`childScopeID` (`gui/id_resolve.go:146`) and the arena are used exactly as
today. The scope push now happens inside the node that owns the children rather
than one frame up the recursion, which is the same push at the same point in the
walk.

The `scope bool` exists solely to reproduce the flat behavior `formView`
currently obtains by self-truncation. Pushing unconditionally prefixes every
widget inside a form with `form:<formID>` in its effective ID — a visible change
to `SetFocus` / `FindByID` callers, though **not** to form field lookup, which
never touches ID resolution. Keeping the flag makes this refactor
behavior-preserving. The scoping question is issue #306.

### Call-site changes

Three production types, plus `viewFunc`:

**`containerView`** — delete `Content()`. Append at the end of `GenerateLayout`,
after `addGroupBoxTitle`:

```go
	addGroupBoxTitle(cv.cfg.Title, cv.cfg.TitleBG, cv.cfg.ColorBorder,
		cv.cfg.Disabled, w, &layout)
	appendChildViews(w, &layout, cv.content)
	return layout
```

**`formView`** — carries all the ID-scoping risk, and is therefore **deferred to
step 3b** (see "Implementation order"). In step 3a it needs only its `Content()`
method deleted:

```go
	// 3a: unchanged except the deleted Content() method.
	layout := inner.GenerateLayout(w)
	fv.content = fv.content[:0]   // now dead, but harmless — see below
	for _, child := range children { ... }
```

This is behavior-preserving with no special handling, and the reason is worth
stating because it is not obvious: `inner` is built as
`Column(ContainerCfg{...})` with **no `Content` field set**, so
`containerView`'s new `appendChildViews` call sees an empty slice and returns
before pushing any scope. `formView`'s manual loop then appends children exactly
as today, still flat. The truncation becomes dead code the moment `Content()` is
gone — nothing reads `fv.content` afterward — but leaving it in place for one
step costs nothing and keeps 3a purely mechanical.

Step **3b** then does the cleanup:

```go
	layout := generateViewLayout(inner, w)   // was inner.GenerateLayout(w)
	appendChildViewsFlat(w, &layout, children)
	return layout
```

Three things land together there:

- `fv.content = fv.content[:0]` (`:249`) is deleted as dead code. `formView`
  becomes stateless as the interface has always claimed, and a cached `formView`
  stops being a frame-2 bug waiting to happen.
- `generateViewLayout(inner, w)` replaces the direct call, putting the node back
  on the one path. `formView` currently skips `ensureLayoutShape`. `inner` is a
  `containerView` and always produces a shape, so this is a correctness tidy
  rather than a fix.
- routing through `appendChildViewsFlat` gives form children the arena
  reservation and the `maxEventChildren` cap that the hand-rolled loop does not
  have today. **This is the only reason `appendChildViewsFlat` needs to exist**
  — without it, 3b can delete the dead line and stop.

**`rotatedBoxView`** — delete `Content()`. The singular child avoids the current
per-node slice allocation:

```go
	if v.content != nil {
		appendChildViews(w, &layout, v.contentSlice)
	}
```

where `contentSlice []View` is built once in the `RotatedBox` factory rather
than per frame. (Alternative: an `appendChildView` singular helper. Either is
fine. The factory-side slice keeps one helper.)

**`viewFunc`** — delete `Content()`. `GenerateLayout` already self-recurses.

**The other 24** (`svgView`, `imageView`, `splitterView`, `selectView`,
`comboboxView`, `commandPaletteView`, `tabControlView`, `themePickerView`,
`drawCanvasView`, `termGridView`, `datePickerRollerView`, `rtfView`,
`colorPickerView`, …) — delete the one-line `Content() []View { return nil }`.
No other change. They already self-recurse.

## Downstream impact

`Content()` is an interface method, so removing it is a breaking change for
external implementors. Surveyed:

| Repo        | `Content()` impls | All return `nil`? |
| ----------- | ----------------- | ----------------- |
| `go-charts` | 20                | yes               |
| `go-map`    | 4                 | yes               |
| `go-edit`   | 0                 | —                 |
| `go-kite`   | 0                 | —                 |
| `go-term`   | 0                 | —                 |

**All 24 sibling implementations are `{ return nil }`.** `gui/datagrid`,
`gui/markdown`, and `gui/svg` implement it zero times.

### `Content()` is also a live accessor — this is the real cost

An earlier draft said migration was "deletion of a dead method." **Wrong.** That
survey counted implementations and never searched call sites. `Content()` is
read as a tree-enumeration accessor in three sibling repos:

| Repo        | Call sites | Where                                                     | Mechanical? |
| ----------- | ---------: | --------------------------------------------------------- | ----------- |
| `go-map`    |         18 | `legend_test.go`, `gallery_test.go`, `fullwindow_test.go` | yes         |
| `go-charts` |          2 | `examples/showcase/gallery_main.go:150`, `helpers.go:162` | **no**      |
| `go-term`   |          1 | `term/widget_test.go:2689`                                | yes         |

All break at compile time.

**The 19 mechanical ones** are structural assertions and shape lookups. They
migrate to generating first and walking `Layout.Children`:

```go
	// was: for _, child := range v.Content()
	for i := range gui.GenerateViewLayout(v, w).Children { ... }
```

Not a pure rename — the counts differ where a container sets `Title`
(`addGroupBoxTitle` adds two children), where a child is nil (skipped), and at
the `maxEventChildren` cap. `go-map`'s relative assertions
(`len(withTitle.Content()) != len(without.Content())+1`) are the ones to verify
individually.

**The two `go-charts` sites cannot migrate this way**, and this is the finding
that matters:

```go
	// go-charts/examples/showcase/gallery_main.go:150
	if _, ok := v.(chart.Drawer); ok { out = append(out, v); return }
	for _, c := range v.Content() { walk(c) }
```

Both `findCharts` and `collectExportable` walk the tree to **type-assert
children to `chart.Drawer` and collect them as `[]gui.View`**. `Layout.Children`
carries `Shape`, not `View` — the View identity is gone, so the assertion is
impossible on the generated tree.

This is not a migration gap. It is a **capability the refactor removes**. After
the change, a `View`'s children live in `containerView.content`, which is
unexported, and there is no interface method to enumerate them. Nothing can walk
a View tree without generating it, and generating it discards View identity.

Options for `go-charts`, none of which this spec can decide unilaterally:

1. **Restructure at the build site** — `findCharts` walks a tree
   `gallery_main.go` itself just constructed, so it can record `Drawer`s while
   building instead of rediscovering them. Cleanest, and arguably what the code
   was meant to do. It costs go-charts a real refactor.
2. **Hold the cfg** — `ContainerCfg.Content` stays public, so go-charts can
   thread its own `[]gui.View` alongside. Works, slightly redundant.
3. ~~**Provide a replacement seam in `gui/`**~~ — **rejected**, see Unresolved
   #4. Option 1 or 2 it is.

**This is a prerequisite, not a follow-up.** `go-charts` must land its side
before go-gui tags a release carrying this change, or the sibling bump breaks.
Tracked as **go-gui-org/go-charts#41**, which carries the two call sites, why
the `Layout.Children` migration fails for them, and three suggested directions.

`go-map` (18 sites) and `go-term` (1) are mechanical and can migrate with the
bump rather than ahead of it — but note `go-map`'s relative assertions
(`len(withTitle.Content()) != len(without.Content())+1`) need verifying against
the `addGroupBoxTitle` child-count difference rather than blind translation.

`GenerateViewLayout` (exported) is called from `gui/datagrid` tests,
`examples/showcase`, and `examples/process_monitor`. Its signature and semantics
are unchanged. Only its doc comment needs the "walks `Content()`" sentence
replaced.

## Implementation order

### 0. ~~Decide Unresolved #4~~ — done

**Decided: no enumeration accessor** (2026-08-14). `go-charts`#41 is therefore a
blocking prerequisite rather than a follow-up, and step 2 must merge before the
release tag. All open questions in this spec are now closed. Implementation can
start at step 1.

### 1. go-gui pre-work — no behavior change, lands independently

1. **Pin `BenchmarkGenerateViewLayout`** to a production-shaped tree (`Column` +
   `Text`). `benchView.Content()` allocates a `[]View` per node, so without this
   the before/after alloc delta measures the harness, not production.
2. **Add the two missing assertions** — group-box child ordering, and form
   children's flat effective IDs with the companion `FormFieldState` check.
   Green before and after. They exist so that a mistake in step 3 fails for the
   right reason rather than silently.

### 2. go-charts#41 — parallel with step 1

Depends only on the step 0 decision, not on any go-gui code, so it can proceed
concurrently. **Must be merged before go-gui tags a release** carrying step 3.

### 3. The refactor — two PRs, not one

**3a — interface collapse.** Add `appendChildViews`. Shrink
`generateViewLayout`. Delete all 27 `Content()` implementations. Update
`containerView`, `rotatedBoxView`, `viewFunc`, and the three test stubs. Fix the
doc comments in `view.go`, `doc.go:80`, and `layout.go:4,26`.

Per the Unresolved #4 decision, `gui/doc.go` also states the new constraint
outright: **a View tree cannot be walked without generating it, and generating
it discards View identity — record Views at construction time if you need to
find them by type later.** That is the one thing this change takes away from
users, so it belongs in the docs rather than in a compile error.

`formView` loses only its `Content()` method here — see "Call-site changes"
above for why that is behavior-preserving without any helper.

**3b — `formView` cleanup.** Delete the dead truncation, switch to
`generateViewLayout(inner, w)`, and introduce `appendChildViewsFlat`.

The split exists because **3b carries all of the ID-scoping risk and 3a carries
none**. Keeping them apart means a focus or ID regression bisects to a small,
single-purpose PR instead of one that touched 27 files. 3a is large but
mechanical. 3b is three lines and the whole reason to be careful.

### 4. Release and sibling bumps

Tag go-gui, then propagate. `go-map` (18 call sites) and `go-term` (1) migrate
**with** the bump rather than ahead of it. `go-edit` and `go-kite` have neither
implementations nor call sites.

### 5. Issue #306 — resolved: Form scopes its children

Landed 2026-08-14, after the refactor settled. `Form` now routes through the
single `appendChildViews` path like every other container.
`appendChildViewsFlat` is deleted. Form children's effective IDs changed from
the flat leaf to `form:<id>:<leaf>` (absolute prefix — a form inside an
ID-bearing panel still scopes under `form:<id>`, never the panel's scope). That
is a breaking change for `SetFocus` / `FindByID` callers that used the flat
name. The field registry never participated (`FieldID` is a separate namespace)
and is unaffected. What it fixes: two forms in one window can each hold an
`Input{ID: "email"}` without colliding on one effective ID — the flat behavior
made such a window fail `TestDuplicateIDs`.

## Verification

Existing gates that must stay green unmodified in intent:

- `TestGenerateViewLayoutFlat`, `TestGenerateViewLayoutWithChildren`,
  `TestGenerateViewLayoutNested`, `TestGenerateViewLayoutExported`
  (`gui/view_test.go`)
- `TestGenerateViewLayoutNormalizesNilShape` (`gui/view_test.go:246`) — pins the
  `ensureLayoutShape` contract that `generateViewLayout` retains
- `TestViewFuncGenerateLayout` (`:281`) and `TestViewFuncGenerateLayoutNested`
  (`:291`)
- `TestGenerateViewLayout_ExcessiveChildren` (`gui/view_test.go:1140`) — must
  now exercise the cap through a container, since the framework no longer walks
  a raw `Content()`
- `BenchmarkGenerateViewLayout` (`gui/view_bench_test.go:75`) — **the baseline
  shifts under this change and the numbers are not comparable as-is.**
  `benchView.Content()` (`:13-22`) itself allocates a `[]View` per node per
  call. Moving it to `appendChildViews` drops allocations from the harness
  rather than from production code. Either pin the benchmark to a
  production-shaped tree (`Column` + `Text`) before the refactor and compare
  that, or record both numbers and state which is drift. Do not report the raw
  delta as a production gain.
- `TestDuplicateIDs` / `TestUnconsumedEvents` sweeps, and any test asserting
  effective IDs — these are the real regression surface, since the scope
  push/pop moved

Test stubs `stubView`, `nilShapeStubView` (`view_test.go:144,153`) and
`benchView` (`view_bench_test.go:13`) move their children into `GenerateLayout`
via `appendChildViews`.

Then: `make check-all`, `make export-audit` (the `View` interface is exported
surface), and `make ergonomics-audit`.

Two behaviors currently relied on with nothing pinning them down. Both gain an
assertion in **step 1**, before either refactor PR, so the tests fail for the
right reason if step 3 gets them wrong:

1. **Group-box ordering.** A container with `Title` set _and_ `Content` children
   must keep the `addGroupBoxTitle` eraser + label ahead of the content
   children.
2. **Form children's effective IDs stay flat.** A `Form{ID: "myform"}`
   containing an `Input{ID: "email"}` must resolve that input's effective ID to
   `email`, **not** `form:myform:email`. This is what `fv.content[:0]` buys
   today and what `appendChildViewsFlat` buys after. Nothing asserts it now —
   `gui/view_form_test.go` has no `SetFocus` / `FindByID` / `idKey` coverage at
   all.

   Note this is about **widget** identity, not form fields.
   `FormFieldAdapterCfg.FieldID` is a separate namespace that never passes
   through ID resolution, so a companion assertion that
   `w.FormFieldState("myform", "username")` still resolves is worth adding
   precisely to document that scoping cannot break it.

A form nested inside an ID-bearing panel is worth asserting alongside (2). Since
`formLayoutID` is absolute, "flat" there means the form's enclosing scope, not
window-global — worth pinning down explicitly.

## Rejected alternative: eliminate `View`, build `Layout` directly

Originally motivated by removing "one interface box per node per frame."
**Measurement kills that premise: the box is 0 B and 0 allocs** (Unresolved #3).
The per-node cost is the view struct, and a layouts-only design still allocates
a comparable `Layout` + `Shape` in its place — so there is no allocation win to
trade against. Costs, in the smallest app in the repo
(`examples/get_started/main.go`, 11 lines of tree):

- four `w` threadings, scaling linearly with nesting depth
- `gui.CurrentTheme()` → `w.Theme()` in every app, because factories run at call
  time rather than under `installTheme`'s frame cache — issue #301 again, but in
  user code instead of 13 internal sites
- evaluation order stops being deferred: a `[]Layout` literal evaluates its
  elements before the enclosing call, so the idScope push cannot be expressed
  around it
- `Themed(t, build)` loses its meaning, and per-window theming (#296, #302) goes
  with it
- API break across five sibling repos

A large API break across six repos, in exchange for a saving that measures zero.

## Unresolved

1. ~~Does `formView.GenerateLayout` put children into `layout.Children` before
   its `return layout`?~~ **Resolved: yes, and by a worse mechanism than
   assumed** — see "`formView` is a third mechanism" above. It bypasses
   `generateViewLayout` and truncates its own `Content()` slice to suppress the
   framework walk, which also suppresses the ID-scope push. Handled by
   `appendChildViewsFlat`.
2. ~~Is the flat scoping for form children a deliberate contract?~~ **Resolved:
   no — it is accidental.** The truncation predates ID scoping by two months
   (`2fda77b7`, 2026-06-08 vs `70a7a399`/#228, 2026-08-09), its comment claims
   only double-processing, and no test covers the effect. The earlier rationale
   in this spec — that field registration breaks — was wrong. Field IDs are a
   separate namespace. See "The scope suppression is accidental" above.

   **Decided (2026-08-14): keep flat, superseded by #306 the same day.** This
   change was behavior-preserving — `appendChildViewsFlat` reproduced today's
   effective IDs exactly, keeping any resulting focus bug unambiguous between
   the two causes. Issue #306 then resolved the question on its own merits:
   `Form` scopes its children like every other container, and
   `appendChildViewsFlat` is gone. See §5.

3. ~~Is the per-frame `View` interface box actually measurable?~~ **Resolved: it
   is exactly zero.** Measured on an Apple M5, `go test -benchmem`:

   | Benchmark                            |  ns/op |    B/op | allocs/op |
   | ------------------------------------ | -----: | ------: | --------: |
   | box `*containerView` into `View`     |    2.5 |   **0** |     **0** |
   | allocate `&containerView{}` alone    |   58.3 |     512 |         1 |
   | build 301-node tree (factories only) | 36,659 | 184,707 |       402 |
   | build + generate the same tree       | 54,308 | 195,003 |       603 |

   Boxing a pointer into an interface allocates nothing — the pointer fits in
   the interface's data word. The per-node cost is the **view struct itself**
   (`&containerView{}` is 512 B, dominated by the embedded `ContainerCfg`), and
   a layouts-only design still pays a comparable cost for the `Layout` + `Shape`
   it builds instead.

   The `Content()` call remains a real **dispatch** (~2.5 ns/node/frame), which
   is why collapsing it is still worth doing — but it is not an allocation, and
   this spec does not sell itself as an allocation win. The only allocation this
   change removes is `rotatedBoxView`'s per-node slice.

4. **Reopened by the call-site survey — but the seams need separating first.**
   The original framing — whether to export `appendChildViews` — conflates two
   distinct things:

   - **Producer seam** — a composite widget declaring children and wanting the
     framework to walk them. That is what `appendChildViews` is. **No external
     consumer needs it**: all 24 sibling implementations return `nil`, so
     nothing outside `gui/` produces children through the framework today.
     Exporting it serves nobody, and the original "conservative answer is no"
     stands for this seam.
   - **Consumer seam** — code enumerating an existing View tree. This is what
     the 21 downstream call sites use, and it is what the refactor actually
     removes. `appendChildViews` does not serve it. Exporting it does not
     unblock a single one of those sites.

   So the live question was narrower and different: whether `gui/` offers a
   replacement for View-tree enumeration at all.

   **Decided (2026-08-14): no accessor.** `GenerateViewLayout(v, w).Children`
   covers 19 of the 21 downstream sites. The other two want View identity
   through a walk, which is a design smell — `go-charts` walks a tree it built
   itself and can record `Drawer`s during construction (Downstream option 1).
   The interface stays at one method with no exceptions.

   Two consequences follow, both accepted:

   - **`go-charts`#41 is a blocking prerequisite** for the release tag, not a
     follow-up. Step 2 of the implementation order must merge first.
   - **View trees become permanently non-enumerable without generating**, and
     generating discards View identity. Anything needing to find Views by type
     must record them at construction time from here on. Worth stating in
     `gui/doc.go` alongside the `View` docs so the constraint is discoverable
     rather than learned by compile error.

   Rejected: an exported `ChildViews(v View) []View` backed by an unexported
   `childViewer` interface. It unblocks `go-charts` with no restructuring, but
   reintroduces on every `View` exactly the second mechanism this spec exists to
   remove.
