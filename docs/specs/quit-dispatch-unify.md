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

## Plan

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

### 2. Remove the wid==0 EventQuitRequested branch

`backend.go:289-303` — delete the entire `if evt.Type == gui.EventQuitRequested`
block from both `RunAppE` and `Run`. The `!cont` branch above handles quit
uniformly.

For `Run` (single-window): the `!cont` branch at `backend.go:106-108` sets
`running = false` and breaks. After the inner event loop, `w.CloseRequested()`
is checked — but the window's close handler was already invoked by
`DispatchQuitRequest`, so `Close()` has been called.

Wait — `Run`'s `!cont` branch doesn't call `DispatchQuitRequest`; it only has
`running = false; break`. For single-window mode, the window is `w`, so
`DispatchCloseRequest(w)` is the right call. But `DispatchQuitRequest` takes an
`*App`, which doesn't exist in single-window mode.

**Correction:** keep the `EventQuitRequested` handler in `Run` (single-window) —
it's already correct. Only change `RunAppE`.

### 3. Remove `performClose:` from `quit:`

`metal_window.m` `quit:` delegate method: remove the `[[NSApp keyWindow]
performClose:nil]` call. With the unified dispatch, `DispatchQuitRequest`
handles all windows uniformly — no need for a synchronous workaround.

### 4. (Optional) Verify Cmd+Q fallback still works

`events.go:71-72` — the Cmd+Q key-event fallback maps to `EventQuitRequested`
with `cont=true`. After this change, Cmd+Q goes through `performKeyEquivalent:`
→ `quit:` → `_evType = METAL_EVENT_QUIT` → `cont=false` →
`DispatchQuitRequest`. The key-event fallback is only used if
`performKeyEquivalent:` fails (unlikely with a properly wired menu). Leave it
as a safety net.

## Affected files

| File | Change |
|------|--------|
| `gui/backend/metal/events.go:24-25` | `METAL_EVENT_QUIT` → `cont=false` |
| `gui/backend/metal/backend.go:289-303` | Remove `EventQuitRequested` handler from `RunAppE` |
| `gui/backend/metal/metal_window.m` `quit:` | Remove `performClose:` call |

## Risks

- **Cmd+Q regression**: if `performKeyEquivalent:` doesn't fire (menu not wired),
  the key-event fallback still maps to `EventQuitRequested` with `cont=true`.
  The deleted wid==0 handler was the only consumer of this event in `RunAppE`.
  If the fallback fires, the event would pass through to `w.EventFn(evt)` as a
  regular event — no quit would happen. **Mitigation**: keep the key-event
  fallback but have it also return `cont=false`, matching the
  `METAL_EVENT_QUIT` path. Or convert it to inject `METAL_EVENT_QUIT` state
  and skip the key-event mapping entirely.

- **Dialog veto**: `DispatchQuitRequest` checks `DialogIsVisible()` and skips
  close for dialog windows. This is new behavior for Metal — previously, the
  `Broadcast` closed all windows regardless. If the app has a visible dialog
  during quit, the dialog stays open and the quit does nothing. This is correct
  per the gl/sdl2 convention but may surprise existing Metal users.

- **Double OnCloseRequest goes away**: currently the `quit:` delegate method
  calls `performClose:` (synchronous `OnCloseRequest`) AND the Go event loop
  dispatches it again. After this change, only `DispatchQuitRequest` fires it.
  This is the desired behavior but changes timing — the synchronous dispatch
  during menu-bar tracking is gone, replaced by event-loop dispatch.

## Testing

- `GO_GUI_MAIN_THREAD_TESTS=1 go test ./gui/backend/metal/...`
- Manual: Cmd+Q quit, menu-bar click quit, system logout quit
- Multi-window: open 2+ windows, Cmd+Q — verify all get close requests
- Veto: window with `OnCloseRequest` that doesn't call `Close()` — verify
  quit is blocked
- Dialog: open dialog then Cmd+Q — verify quit is blocked (dialog veto)
