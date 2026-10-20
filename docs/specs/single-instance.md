# Spec: single-instance mode and open events

Issue: #1106. Status: **implemented** (unreleased).

## Problem

A second launch of a go-gui app started a second process and a second window.
There was also no way to receive "open with" requests. On macOS a file opened
from Finder and a custom URL scheme (`myapp://…`) come as Apple Events (`odoc`,
`GURL`), not as arguments, and go-gui had no handler. Editors such as go-edit
need both: a double-click on a file while the app runs must open it in the
running window.

## Decision

Design A from the issue: a leaf package for the election, plus one `App`
callback.

- **`gui/instance`.** `Acquire(id) (*Lock, primary bool, err error)`. The
  primary keeps the `Lock` until exit. A secondary sends `os.Args[1:]` to the
  primary, waits for a one-byte acknowledgement, and gets `primary=false`. When
  the election itself fails, `Acquire` returns `primary=true` with the error, so
  the app runs without single-instance mode instead of exiting. `Lock` methods
  are nil-safe.
- **`App.OnOpen func(*App, OpenEvent)`.** `OpenEvent` holds `Files` (absolute
  paths) and `URLs`. `App.DispatchOpen` is the one entry point: safe from any
  goroutine, it appends the event to an App-level buffer of 64 and queues a
  drain command on the main window. The drain runs on the next frame and hands
  every buffered event to `OnOpen` in arrival order. Before a main window
  exists, `Register` queues the drain on the first window. The events stay on
  the App, not in a window's queue, so a main window that closes before its next
  frame does not take them along: `Unregister` queues a drain on the new main
  window. After `OnOpen` runs, the window is marked for a layout rebuild: a
  queued command sets no refresh flag of its own, and the live app showed the
  old frame until the next input.
- **`Lock.Attach(app)`.** Dispatches this launch's own arguments once, then each
  forwarded launch, through `gui.ParseOpenArgs`. `gui/instance` imports `gui`,
  as `gui/datagrid` does. One call wires both, so the primary's own arguments
  cannot be forgotten.
- **`gui.ParseOpenArgs`.** Skips flags until `--` (this also drops `-psn_…`). A
  scheme of two or more letters is a URL; `file:` URLs become paths. A `file:`
  URL with a host is a UNC share: kept as `\\host\…` on Windows, skipped
  elsewhere. A scheme with no `//` (`mailto:x`) is a URL only when no file of
  that name exists, so `notes:v2.txt` stays a file. Anything else is a path,
  made absolute. `C:\…` and `C:/…` are paths: `url.Parse` reads them as scheme
  `c`.
- **macOS.** `GoGuiAppDelegate` implements `application:openURLs:`, which AppKit
  calls for both `odoc` and registered-scheme `GURL` since 10.13. The Metal
  `RunApp` stores its `App` before `metalAppInit`, because a launch with
  documents delivers them in the first `[NSApp run]` of `metalAppFinishLaunch`.

### Election and forwarding

| Platform     | Election                            | Forwarding                                  |
| ------------ | ----------------------------------- | ------------------------------------------- |
| macOS, Linux | `flock(LOCK_EX\|LOCK_NB)` on a file | Unix socket beside it, 0600                 |
| Windows      | `CreateMutex` in `Local\`           | named pipe, DACL for the current user's SID |
| Web, mobile  | none: every process is the primary  | none                                        |

- Names are `gogui-` plus 16 hex digits of SHA-256 of the app ID. This keeps a
  macOS socket path under its 104-byte limit.
- Unix directory: `$XDG_RUNTIME_DIR`, else `os.TempDir()/gogui-<uid>`,
  created 0700. A directory that is a symlink, has another owner, or has group
  or other bits is refused: on a shared `/tmp` another user could make it first.
- The lock file opens with `O_NOFOLLOW` and is never removed. Removing it would
  let a second process lock a new file at the same path while the first holds
  the old one.
- A primary that holds the lock removes a stale socket before `bind`. Holding
  the lock proves that the process that made the socket is dead.
- A secondary that finds the lock taken retries the dial for 2 s: the primary
  may be between `flock` and `listen`.
- Windows: the pipe name includes a hash of the user SID, because the pipe
  namespace is machine-wide. Each instance is made with
  `FILE_FLAG_FIRST_PIPE_INSTANCE` and `PIPE_REJECT_REMOTE_CLIENTS`, so another
  process that owns the name makes the primary fail instead of sitting between
  it and its secondaries.
- Message: a `uint32` count, then a `uint32` length and the bytes of each
  argument, big-endian. Limits are 1024 arguments, 64 KiB per argument and 1 MiB
  per message. Every length is checked before memory is allocated. Unix
  connections have a 2 s deadline. The accept loop serves one client at a time,
  so a burst takes the memory of one message.

### The macOS event mask

The Metal poll used an explicit `NSEventMask` list. AppKit dispatches Apple
Events from `nextEventMatchingMask:` only under `NSEventMaskAny`. A list that
names every defined mask bit is not enough. With the old list, a document opened
while the app ran never reached the delegate. A launch-time document did,
because it arrives in `[NSApp run]`. A 40-line native loop showed this before
the change. The poll now uses `NSEventMaskAny`, as `[NSApp run]` does. Event
types that `storeEvent` does not know take its `default` branch and still go to
`sendEvent:`.

## Verification

- `gui/instance` tests: the second acquirer in the same process and in a child
  process forwards; arguments that arrive before `Attach` are kept; eight
  concurrent racers elect one primary; the socket left by a `kill -9` primary is
  replaced; an oversized message is refused and the primary keeps serving; a
  shared lock directory and a symlink are refused; the full path to `OnOpen`.
- Metal: `deliverOpen` unit test, and a main-thread test that calls the real
  delegate's `application:openURLs:` (`GO_GUI_MAIN_THREAD_TESTS=1`).
- By hand, on a `buildapp` bundle with document and URL types added: `open -a`
  with a file before launch and while running, a `GURL` Apple Event, and a
  second launch from the command line. All four reached `OnOpen`.

## Limits

- Single-window `backend.Run` has no `App`. Its Apple Events are dropped, with a
  log line under `GOGUI_DEBUG`.
- The primary does not raise its window on a forward. On Linux and Windows,
  focus-stealing rules decide whether `OnOpen` can.
- Windows pipe I/O is synchronous: a client of the same user that connects and
  stalls holds the accept loop. That user can stop the primary in other ways.
- Packaging metadata (Info.plist `CFBundleDocumentTypes` / `CFBundleURLTypes`,
  `.desktop` `MimeType`, the Windows registry) and Android or iOS intents are
  not part of this change.

## Rejected Approaches

- **`SingleInstance bool` on a Cfg, with `Run` calling `os.Exit` in a
  secondary.** The exit is hidden in `Run`: deferred cleanup in `main` is
  skipped, and the caller cannot log or choose.
- **Election on `NativePlatform`.** The portable flock and pipe code would be
  copied into every backend, `NativePlatform` would grow, and tests would need a
  fake platform.
- **Election by "socket exists".** It races. `flock` and a named mutex are
  atomic, and the kernel frees them on a crash.
- **Turning on single instance when `OnOpen` is set.** It would forward a
  secondary's arguments without giving the app a chance to exit cleanly.
- **`WindowCfg.OnOpen` for single-window `Run`.** A second field on a second
  type and a second delivery path. Apps that need open events use `RunApp`.
- **A `gui`-free `gui/instance` with a channel of argument lists.** Every app
  would write the same loop, and could forget to dispatch its own arguments.
