# Cancelling an in-flight `MouseLock`

Status: implemented. Issue #110.

## Problem

`MouseLock` is cleared only by an explicit `MouseUnlock`, and for every
widget that call lives in the lock's `MouseUp`. That is correct exactly as
long as a press is always followed by a release the app can see.

Windows does not guarantee that. `SetCapture`/`ReleaseCapture` are paired
with the button state in `mouseButton` (`gui/backend/gl/events_win32.go`),
but Win32 can revoke capture on its own — another process calls
`SetCapture`, a system modal or UAC prompt appears, Ctrl+Alt+Del or Win+L
fires, a debugger breaks in. The window then gets `WM_CAPTURECHANGED` and
no `WM_LBUTTONUP`. The message was unhandled, so the lock survived and
`mouseMoveHandler` kept routing every later move into the drag callback:
text selection tracking a cursor with no button held, cleared only by a
fresh click.

macOS had the same symptom from a different cause (a resize-border press
whose mouse-up AppKit swallowed), fixed separately in
`metal_window_darwin.m`. X11 is not affected: reparenting WMs own the
resize borders on the frame window, and X's implicit passive grab
guarantees the matching `ButtonRelease`.

## Why not synthesise a `MouseUp`

`MouseUp` is the *commit*. Dock drops call `onLayoutChange`, drag-reorder
calls `onReorder`. Feeding a fake release on capture loss would docks a
panel or reorder a list the user never dropped — a data mutation from an
event that did not happen. Cancellation is a distinct outcome and needs a
distinct hook.

## Design

`MouseLockCfg.Cancel func(*Window)`, plus `(*Window).MouseCancel()`:

```go
func (w *Window) MouseCancel() {
	if !w.mouseIsLocked() {
		return
	}
	cancel := w.viewState.mouseLock.Cancel
	w.MouseUnlock()
	if cancel != nil {
		cancel(w)
	}
	w.UpdateWindow()
}
```

Three properties the tests pin (`gui/mouse_cancel_test.go`):

- **Unlocks unconditionally.** `Cancel` is optional — a widget whose only
  drag state *is* the lock (scrollbar, splitter, markdown selection) needs
  no hook, and the default already restores it.
- **Clears the lock before the hook runs.** The existing escape-key
  unwinds (`dockDragCancel`, `dragReorderCancel`) call `MouseUnlock`
  themselves; clearing first makes that a no-op instead of a race.
- **Runs the hook at most once.** The guard makes a second cancel inert,
  so a duplicate capture-loss report cannot double-unwind.

`Cancel` takes `*Window`, not `EventCtx`, unlike the other three callbacks.
There is no event to carry, and every other lock callback reads
`ctx.Event` — handing them a nil one would be a footgun rather than
consistency.

## Backend contract

A backend calls `MouseCancel` when the platform ends a drag without a
release. On Win32 that is `WM_CAPTURECHANGED`, with one wrinkle: our own
`ReleaseCapture` raises it too, so an unguarded handler would cancel after
every normal mouse-up. `platformState.capturing` tracks ownership and is
cleared *before* `ReleaseCapture` — that call reenters the wndproc
synchronously, so a flag cleared afterwards would still read as an
involuntary loss.

## Wired hooks

| Site | Cancel does |
| --- | --- |
| text / RTF / input selection | remove the edge-scroll animation, which nothing else stops once the lock is gone |
| slider | clear the pressed flag; the value keeps what the last move set |
| color picker (SV area) | restore the cursor the drag hid |
| datagrid column resize | clear the active flag; the column keeps its dragged width, focus is not moved |
| dock drag | `dockDragCancel` — drop nothing, hide the zone overlay |
| drag-reorder | `dragReorderCancel` — hide ghost and gap, leave the order alone |

Scrollbar, splitter and markdown selection are deliberately hookless: the
default unlock is their whole teardown.
