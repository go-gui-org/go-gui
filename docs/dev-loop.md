# Dev loop: rebuild-and-relaunch

`scripts/dev-loop.sh` runs a Go GUI app and relaunches it whenever the module
changes. The edit → recompile → relaunch cycle is one save away.

This is a convenience wrapper, not in-process reload. Rebuilding and relaunching
is the honest approach for a native Go GUI. Recompiling the app package covers
its whole dependency tree (including `gui/`).

## Usage

```bash
./scripts/dev-loop.sh <app path> [args...]
```

- `<app path>` is a Go package, for example `./examples/get_started/`.
- Extra args are preserved and passed to every relaunched instance.

The script is bash, but you can call it directly from Fish or any other shell:

```fish
./scripts/dev-loop.sh ./examples/get_started/
```

## Behavior

1. **Initial build.** The script builds the app into `build/dev-loop/`
   (gitignored) and launches it. If the initial build fails, the script prints
   the error and exits. There is no previous binary to keep running.
2. **Watch.** Every 0.5 s (override with `DEV_LOOP_POLL_SECONDS`), the script
   scans the module for changes to `*.go`, `go.mod`, or `go.sum` files. It
   ignores `build/`, `.git/`, docs, and scripts.
3. **On save.** The script rebuilds the app. Success: it kills the old process
   and launches the new binary with the original args. Failure: it prints the
   error, the watcher keeps running, and the last good binary keeps running. The
   next save retries the build.
4. **Ctrl-C** kills the running app and removes the temp artifacts.

## Limits

- Changes are picked up only when the binary relaunches. There is no in-process
  reload. State in the app (windows, focused widgets, running goroutines) is
  lost on each relaunch.
- Only Go source and module files trigger a rebuild. Assets read at runtime
  (SVGs, images, fonts) are not watched. After you replace such assets,
  relaunch, or run the app with a manual asset reload.
