# Spec: Android activity lifecycle

Issue #1246. Status: **implemented** (unreleased).

## Problem

Android can end an activity and start a new one in the same process: after Back
on Android 11 and earlier, `finish()`, a configuration change the manifest does
not handle, or "Don't keep activities". The Go side is package state that lives
with the process, but the demo host tore it down in `onDestroy` by queueing
`android.CleanUp` on the activity's GL thread. `CleanUp` destroyed the backend
and the window and set both to nil. The demo's `Init` keeps its first window
(since #780), so the next activity never handed a window over, and `Start` made
an empty one: only the background was drawn.

Whether `CleanUp` ran at all was a race. It was queued on a GL thread that was
shutting down. When it lost, the next activity worked: `SurfaceCreated` rebuilt
the GL resources (#1137) and the window kept its view and state. When it ran
after the new activity's `Start`, it destroyed the live backend from the old
thread.

`CleanUp` came with the first Android backend (c2b3a857) as part of a Pattern B
API "mirroring iOS". No reason for tearing down in `onDestroy` was recorded, and
nothing in the repo calls the iOS `CleanUp`.

## Decision

The Go window and backend live as long as the process.

- A new activity reuses them. `onSurfaceCreated` calls `SurfaceCreated`, which
  rebuilds the GL resources in the new context, as after a lost EGL context. The
  window keeps its view and the app's state; `OnInit` does not run again.
- The ended activity's GL objects go with its EGL context, which `GLSurfaceView`
  destroys when its GL thread ends. Nothing has to delete them.
- `android.CleanUp` does nothing and is deprecated. A host copied from the demo,
  which still calls it from `onDestroy`, is fixed by updating go-gui.
- `Start` logs once when it has to make a window because the host never called
  `SetWindow`, since such a window draws only the background.

What changes for an app:

- `w.Context()` is no longer cancelled when an activity ends; it lives with the
  process, as on iOS. Work that should pause in the background waits for the
  visibility event (#1170).
- On Android 11 and earlier, Back and a relaunch show the previous state.
  Android 12 and later already keep the activity on Back.
- The process keeps the window's memory while Android keeps it cached.

## Testing

`examples/android_demo` on a Pixel 7 (API 37) and an Aquaris X Pro (API 27).
Back finishes the activity when it was not started from the launcher, so
`am start -n` after Back makes a new activity in the same process.

| Check                        | Before (`main`)                 | After                               |
| ---------------------------- | ------------------------------- | ----------------------------------- |
| Back and reopen, Pixel       | Blank 3 of 3                    | Drawn 3 of 3, the click count kept  |
| Back and reopen, Aquaris     | Blank 4 of 6; 2 drew (the race) | Drawn 6 of 6, the click count kept  |
| Fullscreen across a reopen   | -                               | Comes back on both (the demo asks)  |
| Typing after a reopen        | -                               | Works on both                       |
| Memory over repeated reopens | -                               | One rise in the first 10, then flat |

Of the two Aquaris runs that drew before the fix, the one with event counts
shows a new activity as well (one destroyed, one created): the queued `CleanUp`
had not run. Memory: on the Pixel GL memory rose from 17.8 to 21.9 MB in the
first 10 reopens and stayed there over 20 more; on the Aquaris the total stayed
within 0.5 MB over the second 10.

## Rejected Approaches

- **A fresh start per activity.** `CleanUp` ends the window and the next `Init`
  builds a new one. App state would be lost on every activity the system ends,
  and a late `CleanUp` from the old GL thread would still have to be fenced off
  from the new one, whose context is already gone by then.
- **Only teaching `Init` to build a new window after `CleanUp`.** The late
  `CleanUp` could still destroy the new window.
- **Running `CleanUp` on the UI thread.** Deleting GL objects needs the GL
  thread's context, which is gone after `onPause`.
- **Leaving `CleanUp` working and only removing the demo's call.** Every host
  copied from the demo would keep the bug until edited.
