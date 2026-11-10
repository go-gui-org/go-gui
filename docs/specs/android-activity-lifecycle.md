# Spec: Android activity lifecycle

Issue #1246. Status: **implemented** (unreleased).

## Problem

On Android an app's screen is an activity. Android can close the activity and
open a new one while the app's process keeps running. This happens after Back on
Android 11 and earlier, after `finish()`, after a configuration change that the
manifest does not handle, and with the developer option "Don't keep activities".

go-gui's Android side lives in Go package variables, so it lives as long as the
process. But the demo's Kotlin code, its host, tore it down when the activity
closed. In `onDestroy` it queued `android.CleanUp` on the activity's GL thread.
`CleanUp` destroyed the backend and the window and set both to nil.

The new activity then had no window. The demo's `Init` keeps its first window
(since #780), so it never handed a new one to the backend. `Start` then created
an empty window, and the screen showed only the background.

Whether `CleanUp` ran at all depended on timing. It was queued on a GL thread
that was shutting down. When it did not run, the new activity worked:
`SurfaceCreated` rebuilt the GL resources (#1137), and the window kept its view
and state. When it ran after the new activity's `Start`, it destroyed the new
activity's backend from the old thread.

`CleanUp` came with the first Android backend (c2b3a857), as part of an API
"mirroring iOS". No reason for tearing anything down in `onDestroy` was
recorded. Nothing in the repo calls the iOS `CleanUp`.

## Decision

The Go window and backend live as long as the app's process.

- **A new activity reuses them.** Its `onSurfaceCreated` calls `SurfaceCreated`,
  which rebuilds the GL resources in the new context, just as after a lost EGL
  context. The window keeps its view and the app's state. `OnInit` does not run
  again.
- **Nothing has to delete the old GL objects.** They belong to the old
  activity's EGL context, and `GLSurfaceView` destroys that context when its GL
  thread ends.
- **`android.CleanUp` does nothing and is deprecated.** A host copied from the
  demo still calls it from `onDestroy`. Updating go-gui fixes such a host
  without any change to its code.
- **`Start` logs a warning when it has to create a window.** That happens only
  when the host never called `SetWindow`, and such a window draws only the
  background.

What this changes for an app:

- `w.Ctx()` is no longer cancelled when an activity closes. It lives as long as
  the process, as on iOS. Work that should pause in the background will use the
  visibility event (#1170).
- On Android 11 and earlier, Back and opening the app again show the previous
  state. Android 12 and later already keep the activity on Back.
- While Android keeps the process cached, the process keeps the window's memory.

## Testing

The tests used `examples/android_demo` on a Pixel 7 (API 37) and an Aquaris X
Pro (API 27). They opened the demo with adb (`adb shell am start -n`, Android's
command for starting an app from the computer). On Android 12 and later, Back
only moves an app to the background when it was opened from its icon. When it
was opened another way, as here, Back closes the activity, and the next start
opens a new one in the same process. That is the case that failed. On Android 11
and earlier, Back closes the activity either way.

| Check                        | Before (`main`)               | After                               |
| ---------------------------- | ----------------------------- | ----------------------------------- |
| Back and reopen, Pixel       | Blank 3 of 3                  | Drawn 3 of 3, the click count kept  |
| Back and reopen, Aquaris     | Blank 4 of 6; 2 drew (timing) | Drawn 6 of 6, the click count kept  |
| Fullscreen across a reopen   | -                             | Comes back on both (the demo asks)  |
| Typing after a reopen        | -                             | Works on both                       |
| Memory over repeated reopens | -                             | One rise in the first 10, then flat |

Two Aquaris runs drew before the fix. For one of them the event log was read,
and it shows a new activity there too (one destroyed, one created): the queued
`CleanUp` had not run yet.

Memory: on the Pixel, GL memory rose from 17.8 to 21.9 MB in the first 10
reopens and stayed there over 20 more. On the Aquaris the total stayed within
0.5 MB over the second 10.

## Rejected Approaches

- **A fresh start for each activity.** `CleanUp` would end the window, and the
  next `Init` would build a new one. The app would lose its state every time the
  system closes an activity. And a late `CleanUp` from the old GL thread would
  still have to be kept away from the new window.
- **Only letting `Init` build a new window after `CleanUp`.** A late `CleanUp`
  could still destroy the new window.
- **Running `CleanUp` on the UI thread.** GL objects can only be deleted on the
  GL thread, where their context is current. The UI thread has no GL context.
- **Keeping `CleanUp` working and only removing the demo's call.** Every host
  copied from the demo would keep the bug until someone edits it.
