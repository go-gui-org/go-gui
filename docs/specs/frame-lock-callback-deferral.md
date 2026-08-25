# Frame-lock callback deferral

Status: implemented (issue #394)

## The invariant

**No app callback runs while the frame lock (`w.mu`) is held.**

Library code that reaches app code from inside the frame pass raises the
callback with `deferCallback` instead of calling it. `Update` and
`updateRenderOnly` run the raised callbacks through `flushDeferredCallbacks`
after releasing the lock. Both live in `gui/window_deferred.go`.

## Why

`Update` releases `w.mu` for View generation and re-takes it for `layoutArrange`
and `buildRenderers` (`gui/window_update.go`). Two things inside that locked
region reach app code:

- `layoutAmend` (`gui/layout_pipeline.go`) invokes every shape's `AmendLayout`
  hook.
- `buildRenderers` runs the render pass, which reports the caret rect and syncs
  the platform input method.

`w.mu` is a plain `sync.Mutex`. It is not reentrant, and the main goroutine _is_
the platform event loop (`runtime.LockOSThread` in
`gui/backend/metal/mainthread.go`). So an app callback reached from that region
that calls `SetFocus`, `ClearFocus`, `UpdateView`, `ClearDrawCanvasCache` or
`Window.Lock` blocks the main thread on a lock it already owns. The window
freezes permanently, with no panic and no CPU burn.

Issue #394 was exactly this. The Input's blur commit fired from
`inputAmendLayout`. `examples/todo` called `w.SetFocus` from `OnTextCommit` to
put the caret back in the field. Pressing <kbd>Tab</kbd> after typing froze the
app.

The defect was the contract, not the example. Calling `SetFocus` from a commit
handler is a reasonable thing to do, and nothing said not to.

## What is deferred and what is not

Deferred (app code):

- `OnTextCommit` with `InputCommitBlur`
- `OnBlur`
- `OnTextChanged` raised by `PostCommitNormalize` on the blur path
  (`fireTextChangedDeferred`, `gui/view_input.go`)

Not deferred (gui-internal, and needs the live tree):

- the layout echo in `fireTextChangedDeferred` — the arrange pass is still
  reading that tree
- `normalizeOnCommit`, `spellCheckClear`, the selection propagation to the inner
  text shape

A deferred callback receives `EventCtx{nil, nil, w}`. **`ctx.Layout` is nil and
a `*Layout` must never be captured**: the tree is rebuilt from pooled arenas
(`layoutChildrenArena`) before the callback runs, so the pointer dangles into
reused memory. `ctx.Window` and the value arguments are unchanged.

The Enter commit path (`gui/view_input_keys.go`) is dispatched from `EventFn`,
which holds no lock, so it is untouched and still carries a live `ctx.Layout`.

## Audit

Two routes lead from an `AmendLayout` hook into app code across `gui/`:

1. the input blur block — deferred, as above.
2. the app's own hook — `ContainerCfg.AmendLayout` (`gui/view_container.go`) and
   `ButtonCfg.AmendLayout`, reached as `bc.OnAmend` from `buttonAmendLayout`
   (`gui/view_button.go`).

The second cannot be deferred. The hook exists to mutate the arranged tree in
place, so it must run in the pass. It is covered by the fail-fast below instead.

Everything else that sets `AmendLayout` is geometry, focus-ring, hover or
window-state work.

## Fail fast, do not hang

`updateLocked` and `renderOnlyLocked` set `w.inFramePass` for the duration of
the locked region. The `w.mu`-taking window APIs go through `lockForAPI`, which
probes with `TryLock`. When the lock is held during a frame pass, the API
panics, naming the API and the remedy (`QueueCommand`) rather than blocking
forever.

The probe cannot distinguish "this goroutine self-deadlocked" from "another
goroutine is mid-`Update`". It does not need to: calling these APIs off the
frame thread is already outside the contract — `QueueCommand` is the documented
route — so the message covers both.

`PumpFrame` already used the same `TryLock` technique to decline a re-entrant
frame, so this is the established instinct in this codebase, not a new one.

## Flush shape

`flushDeferredCallbacks` swaps the slice out before running it, so a callback
that raises another (an `OnBlur` moving focus, blurring a second field) lands in
the next round rather than mutating the slice it ranges over. Rounds are bounded
by `maxDeferredCallbackBatches`. Past the bound, the remainder is dropped and
reported through `DebugCallbacks`.

`FrameFn` re-runs the refresh pass once after any pass whose flush ran a
deferred callback. Those callbacks run after the renderers were built, and a
callback that writes state or moves focus marks no refresh flag — `SetFocus` and
plain state writes are silent — so `flushDeferredCallbacks`'s own report (does
anything run?) is what re-arms the loop. Without it, the frame renders the
pre-callback state and keeps it until the next event. Two passes, not a loop: a
view that dirties itself every pass is a bug the frame loop must not amplify
into a spin, and the next `FrameFn` picks it up anyway.

## Rejected: a reentrancy flag

The obvious smaller fix is a `Window.inFrameLock` bool that makes `SetFocus`
skip the mutex when the frame pass already owns it.

It does not make the lock reentrant — it makes callers _bypass_ it based on a
flag that is only meaningful on the owning goroutine. Any other goroutine
calling `SetFocus` during a frame pass reads `true` and walks into
unsynchronized state, turning a benign lock wait into a silent data race one
arrange pass wide. It also covers only the methods someone remembers to
annotate, and cannot cover the exported `Window.Lock` at all.

Do not re-propose it.
