# Dev loop: rebuild-and-relaunch

`scripts/dev-loop.sh` runs a Go GUI app and relaunches it whenever the module
changes, so the edit → recompile → relaunch cycle is one save away.

This is a convenience wrapper, not in-process reload: rebuilding and relaunching
is the honest approach for a native Go GUI, and recompiling the app package
covers its whole dependency tree (including `gui/`).

## Usage

```bash
./scripts/dev-loop.sh <app path> [args...]
```

- `<app path>` is a Go package, e.g. `./examples/get_started/`.
- Extra args are preserved and passed to every relaunched instance.

The script is bash, but you can call it directly from Fish or any other shell:

```fish
./scripts/dev-loop.sh ./examples/get_started/
```

## Behavior

1. **Initial build.** The app is built into `build/dev-loop/` (gitignored) and
   launched. If the initial build fails, the script prints the error and exits —
   there is no previous binary to keep running.
2. **Watch.** Every 0.5 s (override with `DEV_LOOP_POLL_SECONDS`) the module is
   scanned for changes to `*.go`, `go.mod`, or `go.sum` files. `build/`,
   `.git/`, docs, and scripts are ignored.
3. **On save.** The app is rebuilt. Success: the old process is killed and the
   new binary launched with the original args. Failure: the error is printed,
   the watcher keeps running, and the last good binary keeps running; the next
   save retries the build.
4. **Ctrl-C** kills the running app and removes the temp artifacts.

## Limits

- Changes are picked up only when the binary relaunches; there is no in-process
  reload. State in the app (windows, focused widgets, running goroutines) is
  lost on each relaunch.
- Only Go source and module files trigger a rebuild. Assets read at runtime
  (SVGs, images, fonts) are not watched — relaunch after replacing those, or run
  the app with a manual asset reload.
