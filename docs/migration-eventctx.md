# Migrating to `EventCtx`

Breaking change. Every event callback now takes a single `EventCtx` instead of
`(*Layout, *Event, *Window)`, and the events a widget normally consumes are
marked handled by dispatch instead of by the callback body.

Run the migration tool first, then work the review list it prints.

## The new signature

```go
// Before
OnClick: func(l *gui.Layout, e *gui.Event, w *gui.Window) {
    gui.State[App](w).Clicks++
    e.IsHandled = true
},

// After
OnClick: func(ctx gui.EventCtx) {
    gui.State[App](ctx.Window).Clicks++
},
```

`EventCtx` is three pointers passed by value:

```go
type EventCtx struct {
    Layout *Layout
    Event  *Event // nil for AmendLayout and OnScroll
    Window *Window
}
```

Payload-carrying callbacks keep the payload as a leading argument — a `GridRow`
is data, not context:

| Before                               | After                         |
| ------------------------------------ | ----------------------------- |
| `func(*Layout, *Event, *Window)`     | `func(EventCtx)`              |
| `func(*Layout, *Window)`             | `func(EventCtx)`              |
| `func(*Layout, string, *Window)`     | `func(string, EventCtx)`      |
| `func(T, *Event, *Window)`           | `func(T, EventCtx)`           |
| `func(*Window)`                      | `func(EventCtx)`              |
| `func(T, U, *Window)`                | `func(T, U, EventCtx)`        |
| `func(T, *Event, *Window) (R, bool)` | `func(T, EventCtx) (R, bool)` |

The rule is uniform: `EventCtx` replaces the `*Layout`, `*Event` and `*Window`
parameters and lands last. Everything else keeps its position — payloads in
their original order, and results untouched, which is why `GridCfg.OnCopyRows`
is now `func([]GridRow, EventCtx) (string, bool)`.

The last three rows arrived in v0.54.0; the rest shipped in v0.52.0.

### What did not convert

Callbacks that fire from a timer tick, a dialog completion, or a lifecycle
transition keep their `*Window`. **There is no event** at those points, so an
`EventCtx` would only promise a permanently-nil `ctx.Event`:

> `OnInit`, `OnCloseRequest`, `OnOkYes`, `OnCancelNo`, `OnDismiss`, `OnReply`,
> `OnLazyLoad`, `OnValue`, and the four `OnDone` variants (animation, native
> alert, native dialog, native notification)

Also unchanged: `OnDraw func(*DrawContext)`,
`NativeMenuCfg.OnAction func(string)`, and
`Window.OnEvent func(*Event, *Window)` — a raw escape hatch by design.

## One rule (v0.55.0)

**Nothing is marked handled for you.** A callback that acts on an event calls
`ctx.Consume()`. A callback that does not, lets the event travel on. That is the
whole model, and it applies to every callback.

v0.52.0 through v0.54.0 split callbacks into two classes: `OnClick`, `OnChar`,
`OnMouseUp`, `OnGesture` and `OnFileDrop` were "consume-class" and dispatch
marked their events handled before the callback ran, while everything else was
"notify-class" and had to consume explicitly. Which class you had written was
invisible in the signature. v0.55.0 deletes the split.

**What to change.** Add `ctx.Consume()` to any of those five that means to
absorb the event:

```go
OnChar: func(ctx gui.EventCtx) {
    if ctx.Event.CharCode != gui.CharSpace {
        return // every other character keeps travelling
    }
    toggle(ctx.Window)
    ctx.Consume()
},
```

**The case to search for is the empty handler.** An `OnClick` with an empty body
used to be a working click-blocker — the pre-mark did the absorbing. It now
absorbs nothing. Overlays, backdrops, popups and cards that stopped clicks
reaching what they cover all need a real `ctx.Consume()`.

Nothing changes for `OnKeyDown`, `OnKeyUp`, `OnHover`, `OnMouseMove`,
`OnMouseLeave`, `OnMouseScroll`, `OnScroll`, `AmendLayout` and `OnIMECommit` —
they always worked this way. The behaviours that made them the model are why:

- `OnKeyDown` receives every key. Consuming unasked would kill tab traversal and
  accelerators in any widget with a key handler.
- Hover, move and leave are notifications. Nested shapes legitimately all want
  them, so consuming unasked would break every hover-highlight-on-container.
- `OnMouseScroll` cascades to the enclosing scroll container **only if the
  handler leaves the event unhandled**. Scroll chaining is to scrolling what
  bubbling is to keys.

`MouseLockCfg`'s `MouseDown`/`MouseMove`/`MouseUp` are unaffected: mouse lock
already bypasses hit-testing and propagation. Their coordinates stay
window-absolute, unlike the shape-relative coordinates everywhere else.

## Finding the handlers you missed

The compile-time half of the v0.55.0 change is loud: `ctx.Bubble()` is gone, so
every call site stops building, and deleting the call is the fix. The other half
is silent — a handler that should consume and does not simply lets the event
carry on, and the symptom is a click that fires twice or a popup that dismisses
itself.

`gui.Debug(true)` reports those sites as they are dispatched, and
`TestUnconsumedEvents` sweeps a whole window for them:

```go
w := gui.NewTestWindow(gui.WindowCfg{State: &App{}})
w.TestRender(mainView)
for _, f := range w.TestUnconsumedEvents() {
    t.Log(f)
}
```

Each finding names a handler that did not consume and the ancestor that will
therefore also run — either because it has its own handler for the event, or
because it is focusable and will take focus on the way past.

**Read the findings; do not just drive them to zero.** A handler that inspects
an event, decides it is not its own and passes it on is now the ordinary way to
decline, and it looks exactly like one that forgot.

The sweep fires every hit-tested callback in the window, so give it a window
built for the purpose. It also sees only the frame in front of it — a site
behind a tab or a dialog needs the app driven into that state first.

## Nil `ctx.Event`

`AmendLayout` and `OnScroll` have no originating event, so `ctx.Event` is nil
there. Both methods are nil-safe — `Consume()` does nothing, `Handled()` reports
false — so no guard is needed.

## `Window.OnEvent` sees less

`Window.OnEvent` is the app-level last-resort hook and fires only when the event
is unhandled. Clicks, characters, mouse-ups, file drops and gestures that any
widget handles no longer reach it, where previously a widget that forgot
`IsHandled = true` let them through. That is the intended semantics, but it is a
visible change for apps using `OnEvent` as a global sniffer.

## State access

`EventCtx` deliberately carries no typed state accessor: Go methods cannot take
type parameters, and a generic `EventCtx[T]` would force every `Cfg` struct and
widget factory to become generic. Declare a one-line helper instead:

```go
func st(ctx gui.EventCtx) *AppState { return gui.State[AppState](ctx.Window) }
```

## Running the migration tool

`tools/eventctx` does the mechanical work. It runs in three passes, in order:

```fish
# 1. Closures and bare type expressions.
go run ./tools/eventctx/cmd/eventctx -w -report /tmp/review.txt ./path

# 2. Named callback functions and their call sites.
go run ./tools/eventctx/cmd/eventctx -w -decls ./path

# 3. Calls through callback-typed variables and struct fields, which
#    only the compiler can locate. Repeat until it folds nothing.
go build -gcflags=-e ./... 2>&1 | go run ./tools/eventctx/cmd/eventctxfold
```

For test files, compile them with the same raised error limit and feed that to
the folder:

```fish
for pkg in (go list ./...)
    go test -c -gcflags=-e -o /dev/null $pkg
end 2>&1 | go run ./tools/eventctx/cmd/eventctxfold
```

### Converting a named set of callbacks

The three passes above match by signature alone, which is safe only for the
shapes that carry a `*Layout` or an `*Event` — nothing else in the codebase
looks like those. The v0.54.0 round widened the target to any signature ending
in `*Window`, and that shape is indistinguishable from ordinary internal
plumbing, so the wide matcher is gated on an explicit list of field names:

```fish
set -l fields OnSelect,OnTextCommit,OnValueCommit,OnLayoutChange
go run ./tools/eventctx/cmd/eventctx -w -fields=$fields ./path
go run ./tools/eventctx/cmd/eventctx -w -decls -fields=$fields ./path
```

Under `-fields` a callback converts only where its owning struct field,
assignment target, or parameter is one of the named ones — matched ignoring the
leading case, so the internal spelling `onSelect` matches the field `OnSelect`.
A closure nested _inside_ a converted callback does not inherit the name: a
`w.QueueCommand(func(w *gui.Window) {…})` argument stays as it is.

`-fields` also drops the "returns nothing" restriction, which is how
`OnCopyRows func([]GridRow, *Event, *Window) (string, bool)` converts while
keeping its results.

### The review list

Pass 1 writes a report of every return path in a consume-class callback that is
not dominated by a handled assignment. Each entry is a human decision — insert
`ctx.Consume()`, or confirm passing the event on is right. The tool cannot infer
which, because the old encoding wrote nothing in both cases.

Do not batch-approve it. The recurring shapes worth care are `OnChar` filtering
(a focused input that ignores non-text characters used to let them fall
through), hit-test subregions, and disabled-state guards.

### Known limitations

- Method declarations (`func (c *T) handleKey(...)`) are not converted; the
  compiler finds them.
- Call sites of a converted closure stored in a local variable are not
  rewritten; the compiler finds those too.
- A body that already uses the name `ctx` for something else gets `ectx` as its
  context parameter.
