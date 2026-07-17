# process_monitor

A small live task manager: a filterable process list with flat/tree views,
sortable columns, and per-process CPU/RAM history charts — built on go-gui's
immediate-mode pipeline and styled entirely from the standard theme tokens.

It is a functional port of the [go-shirei `process_monitor`][shirei] example.
The goal is feature parity, not a pixel-for-pixel visual match: the layout takes
cues from other go-gui examples and the colors come from `gui.ThemeDark` /
`gui.ThemeLight`, not hand-picked HSL values.

[shirei]: https://go.hasen.dev

## Features

- Live process list: PID, CPU%, RSS, MEM%, USER, STATE, THREADS, NAME.
- Click a column header to sort; click again to reverse.
- Select a row for a detail panel with ~60s rolling CPU and RAM charts.
- Filter box matching name, command line, user, or PID (substring, case-insensitive).
- Flat list **or** parent/child tree (collapse a subtree with ▸/▾; an active
  filter keeps matched processes' ancestors visible).
- Sample-interval selector: 0.5s / 1s / 2s / 5s.
- Metrics the OS won't report render `--`, never a fake `0`.
- System memory bar in the header (shown only when the platform reports totals).

## Run it

```shell
go run ./examples/process_monitor              # open the GUI
go run ./examples/process_monitor -once        # print one report and exit
go run ./examples/process_monitor -once -limit 20 -sort mem
```

Flags: `-once`, `-limit N`, `-sort cpu|mem|pid|name|user|state|threads`,
`-refresh D` (GUI interval, snapped to the nearest preset).

## How it works

### Dependency-free data collection

go-gui examples share the root module, so this example pulls in no third-party
process library. It shells out to the OS instead:

- **macOS / Linux** — parse `ps` (`collect_unix.go`). Linux also reports the
  thread count via `nlwp`; macOS `ps` has no thread column, so THREADS shows
  `--` there.
- **Windows** — parse `tasklist` CSV (`collect_windows.go`). No live CPU% is
  available from one call, so CPU shows `--`.
- System memory totals come from `/proc/meminfo` (Linux) or `sysctl` + `vm_stat`
  (macOS); other platforms omit the memory bar.

**CPU% caveat:** on Unix the value is `ps`'s `%cpu`, which is a *lifetime
average*, not an instantaneous interval rate. It is a real, useful number and
needs only one sample; a delta-based interval CPU% (two cumulative-CPU-time
reads) would be the enhancement.

### Background sampling, UI only reads

Sampling spawns a subprocess, so it runs on a background goroutine, off the
frame path (`startSampler` in `main.go`). Each pass takes a snapshot, then
publishes it under the window lock and asks for a layout refresh:

```go
snap, err := Collect()
w.Lock()
app.Snapshot = snap
app.Store.Update(snap, app.Selected)
w.UpdateWindow() // re-run the view against fresh state, preserving focus/scroll
w.Unlock()
```

`UpdateWindow` (not `UpdateView`) re-runs the registered view without clearing
the state registry, so the filter input keeps focus and the list keeps its
scroll position across refreshes. The backend's idle poll repaints within
~100 ms, so no explicit wake is needed at these intervals.

### Stable processes + rolling history

`ProcessStore` (`store.go`) keeps a stable `*Process` per identity so table rows
and chart history survive across refreshes. Exited processes linger for 60 s
(so their charts stay visible) and are then evicted — unless they are selected.

### Charts from plain containers

Each history chart (`chart.go`) is a row of bottom-anchored `Rectangle` bars in
fixed 2-second time buckets — no canvas widget, no chart library.
`resampleHistory` folds the irregularly-sampled history into those fixed buckets
(averaging within a slot, linearly interpolating gaps) so the x-axis is always
"last 60 s," independent of the sample rate.
