# Unify metal quit dispatch with gl/sdl2 backends

## Problem

The Metal backend has two quit paths, one dead, one inconsistent:

| Path | Location | Behavior | Status |
|------|----------|----------|--------|
| `!cont` → `DispatchQuitRequest(app)` | `backend.go:277-282` | Dialog veto, per-window hooks, `running` flag | **Dead code** — `METAL_EVENT_QUIT` returns `cont=true` (`events.go:25`) |
| `EventQuitRequested` (wid==0) → `app.Broadcast` | `backend.go:293-297` | No veto, no dialog check, no `running` update | **Live**, divergent from gl/sdl2 |

The gl and sdl2 backends use a single path: the quit event maps to `cont=false`,
which triggers `DispatchQuitRequest(app)` and exits the event loop when not
vetoed (`running = false`).

The Metal backend's wid==0 handler also has a workaround in the ObjC `quit:`
delegate method (`metal_window.m`) — `performClose:` on the key window is
needed because the Go-side dispatch doesn't reach all windows and can't be
vetoed.

## Goal

Single quit dispatch path in Metal that matches gl/sdl2: `METAL_EVENT_QUIT` →
`cont=false` → `DispatchQuitRequest(app)` → veto-or-exit.

Remove the workaround `performClose:` from `quit:`.

## Implementation

### 1. Change `METAL_EVENT_QUIT` to return `cont=false`

`events.go:24-25`:
```go
// Before:
case C.METAL_EVENT_QUIT:
    return gui.Event{Type: gui.EventQuitRequested}, true

// After:
case C.METAL_EVENT_QUIT:
    return gui.Event{}, false
```

Returning `cont=false` with an empty event triggers the existing `!cont` branch
at `backend.go:277-282`, which already calls `gui.DispatchQuitRequest(app)` and
sets `running` from its veto result.

### 2. Update Cmd+Q fallback to return `cont=false`

`events.go:68-73` — the Cmd+Q key-event fallback (used when
`performKeyEquivalent:` fails) now also returns `cont=false` instead of
`(EventQuitRequested, true)`, matching the primary `METAL_EVENT_QUIT` path.

### 3. Fix single-window `Run` `!cont` branch

`backend.go:106-108` — `Run`'s `!cont` branch now calls
`gui.DispatchCloseRequest(w)` before `running = false; break`. Previously it
only set `running = false` without dispatching the close request, which would
skip `OnCloseRequest` hooks.

### 4. Remove `EventQuitRequested` handler from both `Run` and `RunAppE`

`backend.go:110-114` and `backend.go:289-303` — deleted. The `!cont` branch
handles quit uniformly now.

### 5. Remove `performClose:` from `quit:`

`metal_window.m` `quit:` delegate method — removed the synchronous
`[[NSApp keyWindow] performClose:nil]` call. The Go-side `DispatchQuitRequest`
handles all windows.

### 6. Add `_quitRequested` flag for out-of-band quit events

**Deviation from original spec.** The spec assumed menu-bar quit clicks would
be dequeued by `metalPollEvent` like keyboard events. In practice, menu-bar
tracking runs in `NSEventTrackingRunLoopMode`, while `metalPollEvent` only
dequeues from `NSDefaultRunLoopMode`. Without `performClose:`, `quit:` would
set `_evType = METAL_EVENT_QUIT` but `metalPollEvent` would return 0 (no event
in default mode), so Go never saw the quit.

Added `static int _quitRequested` to the ObjC event state, set by `quit:` and
`applicationShouldTerminate:`. Checked at the top of `metalPollEvent` before
any dequeue (same pattern as the IME generation check). When set, it
synthesizes `_evType = METAL_EVENT_QUIT` and returns 1 immediately. Consumed on
first read so vetoed quits don't re-fire.

This covers both inline cases (Cmd+Q via `performKeyEquivalent:` during
`sendEvent:`) and out-of-band cases (menu-bar click, system logout).

## Affected files

| File | Change |
|------|--------|
| `gui/backend/metal/events.go:24-25` | `METAL_EVENT_QUIT` → `cont=false` |
| `gui/backend/metal/events.go:68-73` | Cmd+Q fallback → `cont=false` |
| `gui/backend/metal/backend.go:106-108` | `Run` `!cont` branch now calls `DispatchCloseRequest(w)` |
| `gui/backend/metal/backend.go:110-114` | Removed `EventQuitRequested` handler from `Run` |
| `gui/backend/metal/backend.go:289-303` | Removed `EventQuitRequested` handler from `RunAppE` |
| `gui/backend/metal/metal_window.m` | Added `_quitRequested` flag + poll check; removed `performClose:` from `quit:` |
| `gui/backend/metal/events_test.go:206-222` | Updated Cmd+Q test to expect `cont=false` |

## Risks

- **Cmd+Q regression**: mitigated by keeping the key-event fallback in
  `events.go` with `cont=false` and the `_quitRequested` flag.

- **Dialog veto**: `DispatchQuitRequest` checks `DialogIsVisible()` and skips
  close for dialog windows. This is new behavior for Metal — previously, the
  `Broadcast` closed all windows regardless. If the app has a visible dialog
  during quit, the dialog stays open and the quit does nothing. This is correct
  per the gl/sdl2 convention.

- **Double OnCloseRequest goes away**: previously `quit:` called `performClose:`
  (synchronous `OnCloseRequest`) AND the Go event loop dispatched it again.
  Now only `DispatchQuitRequest` fires it. This changes timing — the
  synchronous dispatch during menu-bar tracking is gone, replaced by
  event-loop dispatch.

## Testing

- `GO_GUI_MAIN_THREAD_TESTS=1 go test ./gui/backend/metal/...`
- Manual: Cmd+Q quit, menu-bar click quit, system logout quit
- Multi-window: open 2+ windows, Cmd+Q — verify all get close requests
- Veto: window with `OnCloseRequest` that doesn't call `Close()` — verify
  quit is blocked
- Dialog: open dialog then Cmd+Q — verify quit is blocked (dialog veto)
