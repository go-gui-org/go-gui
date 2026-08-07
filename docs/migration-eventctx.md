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

| Before                              | After                    |
| ----------------------------------- | ------------------------ |
| `func(*Layout, *Event, *Window)`    | `func(EventCtx)`         |
| `func(*Layout, *Window)`            | `func(EventCtx)`         |
| `func(*Layout, string, *Window)`    | `func(string, EventCtx)` |
| `func(T, *Event, *Window)`          | `func(T, EventCtx)`      |
| `func(*Window)` / `func(T,*Window)` | unchanged                |

Lifecycle callbacks with neither a layout nor an event (animation
`OnDone`/`OnValue`, native dialog and notification `OnDone`,
`NativeMenuCfg.OnAction`) are unchanged, as is `OnDraw func(*DrawContext)` and
`Window.OnEvent func(*Event, *Window)`.

## The consume / notify split

**Consume-class** events are marked handled _before_ the callback runs:

> `OnClick`, `OnChar`, `OnMouseUp`, `OnGesture`, `OnFileDrop`

Delete the trailing `e.IsHandled = true` from these — it is now the default.
Where the old body returned early meaning "not mine, pass it on", call
`ctx.Bubble()`:

```go
OnChar: func(ctx gui.EventCtx) {
    if ctx.Event.CharCode != gui.CharSpace {
        ctx.Bubble() // every other character keeps travelling
        return
    }
    toggle(ctx.Window)
},
```

**Notify-class** events are unchanged in behaviour: nothing is pre-marked, and
the callback calls `ctx.Consume()` to stop propagation.

> `OnKeyDown`, `OnKeyUp`, `OnHover`, `OnMouseMove`, `OnMouseLeave`,
> `OnMouseScroll`, `OnScroll`, `AmendLayout`, `OnIMECommit`

The carve-outs are deliberate:

- `OnKeyDown` receives every key. Auto-consuming would kill tab traversal and
  accelerators in any widget with a key handler.
- Hover, move and leave are notifications. Nested shapes legitimately all want
  them, so auto-consuming would break every hover-highlight-on-container.
- `OnMouseScroll` cascades to the enclosing scroll container **only if the
  handler leaves the event unhandled**. Scroll chaining is to scrolling what
  bubbling is to keys.

`MouseLockCfg`'s `MouseDown`/`MouseMove`/`MouseUp` take the new signature but
get no auto-consume: mouse lock already bypasses hit-testing and propagation.
Their coordinates stay window-absolute, unlike the shape-relative coordinates
everywhere else.

## What `Bubble()` does and does not do

`ctx.Bubble()` opts out of **this callback's** auto-consume. It does not
un-handle an event that an earlier handler already consumed, because the
coordinate save/restore in dispatch re-applies the incoming flag.

## Nil `ctx.Event`

`AmendLayout` and `OnScroll` have no originating event, so `ctx.Event` is nil
there. All three methods are nil-safe — `Consume()` and `Bubble()` do nothing,
`Handled()` reports false — so no guard is needed.

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

### The review list

Pass 1 writes a report of every return path in a consume-class callback that is
not dominated by a handled assignment. Each entry is a human decision — insert
`ctx.Bubble()`, or confirm consume-by-default is right. The tool cannot infer
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
