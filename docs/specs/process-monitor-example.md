# Spec: `process_monitor` example

Status: **implemented** — `examples/process_monitor/` (main, view, store, chart,
collect + sysmem platform files, tests). Author: spec generated for go-gui
Target: `examples/process_monitor/`

## Goal

Port the
[go-shirei `process_monitor`](../../../go-shirei/examples/process_monitor)
example to go-gui as a **functional** equivalent — a live task manager: process
list, filter, flat/tree view, sortable columns, per-process CPU/RAM history
charts. NOT a visual reproduction. Layout and styling take cues from existing
go-gui examples (`todo`, `data_grid_data_source`, `scroll_demo`, `animations`)
and use the **standard go-gui theme tokens** (no hand-picked HSL colors like the
shirei original).

## Non-goals

- Pixel/visual parity with shirei's HSL-tinted look.
- Headless one-frame PNG render (`-png`): go-gui exposes no public render-to-PNG
  API. Dropped. See Open Questions.
- New core go-gui dependencies. The example must stay dependency-free (stdlib
  only) because examples live in the **main go-gui module** — there is no
  per-example `go.mod`, so any import lands in the root `go.mod`.
- Windows-grade CPU% fidelity in v1 (see Data Collection).

## Reference behavior (feature checklist)

From shirei's README + `main.go`. Port each:

1. Live process list: PID, CPU%, RSS, MEM%, USER, STATE, THREADS, NAME.
2. Click a column header to sort. Click again to reverse.
3. Row selection → detail panel + ~60s rolling CPU and RAM bar charts.
4. Filter box: match name, command line, user, or PID (substring,
   case-insensitive).
5. Flat list OR parent/child tree (collapse with ▸/▾. Filter keeps ancestors
   visible).
6. Sample interval selector: 0.5s / 1s / 2s / 5s.
7. Unreadable metrics render `--`, never a fake `0`.
8. Header stats: active/kept process counts, last-updated time, system memory
   bar.
9. Terminal mode (`-once`): print a sorted report and exit, no window.
10. Charts built from ordinary containers (rects) — no canvas, no chart lib.

## Architecture

### File layout

```
examples/process_monitor/
  main.go            # window setup, flags, RootView, sampler goroutine launch
  view.go            # header / toolbar / table / detail views (theme-driven)
  chart.go           # UsageChart + resampleHistory (container-bar chart)
  store.go           # ProcessStore, Process, history ring buffer
  collect.go         # ProcInfo/Snapshot types + Collect() dispatch + platform iface
  collect_unix.go    # //go:build darwin || linux  — ps-based collector
  collect_windows.go # //go:build windows          — tasklist-based collector
  collect_other.go   # //go:build !darwin && !linux && !windows — stub
  sysmem_darwin.go   # //go:build darwin — total/used memory via sysctl + vm_stat
  sysmem_linux.go    # //go:build linux  — /proc/meminfo
  sysmem_other.go    # fallback: report 0 (UI hides the memory bar)
  once.go            # runOnce terminal report
  store_test.go      # ProcessStore.Update, PID-reuse identity, ring buffer
  tree_test.go       # treeRows: hierarchy, collapse, ancestor-keep on filter
  chart_test.go      # resampleHistory bucketing + interpolation
  view_test.go       # headless smoke: build the view tree, assert no panic
  README.md
```

Mirrors shirei's separation (sampler / store / model / main) but split along
go-gui/Go build-tag lines for the platform collectors.

### Data collection (dependency-free, cross-platform)

shirei uses `go.hasen.dev/procinfo`. go-gui cannot: adding it (or gopsutil)
pollutes the root module. Instead, a tiny self-contained collector behind a
platform build tag, mirroring shirei's `procinfo` abstraction.

Types (in `collect.go`, unchanged from shirei semantics):

```go
type ProcInfo struct {
    PID, PPID      int
    Name, Cmdline  string
    User, State    string
    RSSBytes       uint64
    MemPercent     float64
    CPUPercent     float64   // CPUPercentUnknown (-1) when unreadable
    StartTime      time.Time
    Threads        int
    MetricsUnknown bool
}
type Snapshot struct {
    Time             time.Time
    Processes        []ProcInfo
    TotalMemoryBytes uint64
    UsedMemoryBytes  uint64
}
const CPUPercentUnknown float64 = -1
```

Per-platform `collect()`:

- **darwin + linux** (`collect_unix.go`): shell out via `os/exec` to
  `ps -axo pid=,ppid=,pcpu=,rss=,user=,state=,nlwp=,comm=,args=` (drop `nlwp` on
  darwin, which lacks it → threads = `MetricsUnknown`/`--`). Parse fixed leading
  numeric columns, then `comm`/`args` as the trailing free text. RSS is in KiB →
  ×1024. `StartTime` from `ps -o lstart=` (second field set) or left zero if not
  collected.
- **windows** (`collect_windows.go`): `tasklist /fo csv /nh /v` → image name,
  PID, session, mem usage, status, user, CPU time. No live per-interval CPU%
  from a single call. Report `CPUPercentUnknown` in v1 (renders `--`), or
  compute a delta from two `tasklist` CPU-time reads if cheap. RSS from the "Mem
  Usage" column.
- **other** (`collect_other.go`): return an error. The UI shows the sample error
  (matches shirei's error path).

CPU% decision (v1): use `ps pcpu` directly. Caveat: on Unix `pcpu` is a
**lifetime average**, not an interval rate. This is functionally present and
keeps the collector to one `exec` per sample. A delta-based interval CPU%
(re-read cumulative CPU time each sample and divide by wall-clock, like shirei's
`computeSnapshot`) is a documented enhancement, not v1. History charts work
regardless — they plot whatever CPU% is reported, over time.

System memory (`sysmem_*.go`): `/proc/meminfo` (`MemTotal`,
`MemTotal-MemAvailable`) on linux. `sysctl hw.memsize` + `vm_stat` on darwin. On
failure/other, return `(0, 0)`. The header omits the memory bar rather than
inventing a value (shirei's "-- not fake zero" philosophy).

### Sampler goroutine + refresh model

go-gui has **no `WithFrameLock`/`RequestNextFrame`**. The equivalent:

- The backend event loop calls `w.FrameFn()` each iteration and, when idle,
  waits at most ~100ms (`platform_*.go` Run loops: `waitMessage(100)` /
  `time.After(100ms)`). So a background mutation is picked up within ~100ms
  without any explicit wake. `w.wakeMainFn` is private (not callable from an
  example), and none is needed at 0.5–5s sample intervals.
- Sampling spawns a subprocess (`ps`), so it MUST run off the frame thread.

Pattern (in `main.go`, launched from `WindowCfg.OnInit`):

```go
func startSampler(w *gui.Window) {
    go func() {
        var prev *Snapshot // for optional CPU-delta enhancement
        for {
            app := gui.State[App](w)          // read interval (see note)
            snap, err := collectSnapshot(prev) // blocking exec, OFF frame thread
            prev = snap

            w.Lock()
            app.Snapshot = snap
            app.Err = err
            app.LastRefresh = time.Now()
            app.Store.Update(snap, app.Selected)
            if snap != nil && app.Selected == nil { /* auto-select top row */ }
            w.UpdateWindow()  // full layout refresh; re-runs the view fn next
                              // frame WITHOUT clearing the state registry, so
                              // the filter input keeps focus/caret.
            w.Unlock()

            time.Sleep(app.Interval) // read under lock in practice
        }
    }()
}
```

Critical detail: use `w.UpdateWindow()` (alias `markLayoutRefresh`), **NOT**
`w.UpdateView(fn)`. `UpdateView` clears the view-state registry every call,
which drops input focus and scroll position on every sample. `RequestRedraw()`
is render-only (no view re-run) so new rows do not appear — also wrong.
`UpdateWindow` re-runs the registered view generator against fresh state while
preserving the registry. Register the generator once in `OnInit` via
`w.UpdateView(rootView)`.

Reads of shared state during sampling and all mutations happen under `w.Lock()`
/ `w.Unlock()`. The view function itself runs under `w.mu` already (per
CLAUDE.md), so it reads state consistently.

### Process store

Port `ProcessStore` / `Process` / `ProcessPoint` from shirei nearly verbatim —
it is pure Go, no shirei API:

- Key by identity surviving PID reuse: `{PID, StartTime}`. When `StartTime` is
  unavailable (darwin `ps` without `lstart`), fall back to `{PID}` only and
  document the reduced PID-reuse safety.
- Ring buffer history (`maxHistoryPoints = 240`), `appendHistory`.
- Keep stopped processes 60s so their charts linger, then evict (unless
  selected).
- `Processes()`, `ActiveCount()`, `ByKey` for the header counts.

## UI (standard theme, immediate-mode)

Root: a `gui.Column` filling the window (`FixedFixed`, `w.WindowSize()`),
`Color: theme.ColorBackground`. Sections stacked: Header, Toolbar, Table
(fills), DetailPanel.

Theme tokens (from `gui.CurrentTheme()`) replace all shirei HSL literals:

| shirei intent              | go-gui token                                                    |
| -------------------------- | --------------------------------------------------------------- |
| page background            | `theme.ColorBackground`                                         |
| panel / card background    | `theme.ColorPanel`, `theme.ColorInterior`                       |
| row hover / selection      | `theme.ColorHover`, `theme.ColorSelect`                         |
| borders                    | `theme.ColorBorder`                                             |
| title / heading text       | `theme.B2`, `theme.B3`                                          |
| body / cell text           | `theme.N4`, `theme.N5`                                          |
| muted secondary text       | `theme.N5`/`N6`                                                 |
| stopped / error accent     | `theme.ColorError`, `theme.ColorWarning`                        |
| running / ok accent        | `theme.ColorSuccess`                                            |
| padding / spacing / radius | `theme.PaddingSmall`, `theme.SpacingSmall`, `theme.RadiusSmall` |

Default theme: `gui.SetTheme(gui.ThemeDark.WithBorders(true))` (matches
`data_grid_data_source`). A `ThemePicker` widget can be added to the toolbar to
demo light/dark switching (optional, see Open Questions).

### Header (`view.go`)

`gui.Row` of: title `gui.Text{TextStyle: theme.B2}`, spacer, and stat chips
(`gui.Row`+`gui.Text` in a rounded `ColorInterior` container) for "active/kept"
counts and last-updated time. System memory shown as a labeled `UsageBar` (see
below) + "used / total" text. If `Snapshot == nil`: "Collecting…". If
`Err != nil`: error text in `theme.ColorError`.

### Toolbar

- Filter: `gui.Input` (like `todo`'s composer), fixed width ~300, bound to
  `app.Filter` via `OnTextChanged`. Placeholder "Filter by name, cmd, user,
  PID".
- View mode: `gui.RadioButtonGroupRow` or `gui.Toggle` → Flat / Tree, bound to
  `app.TreeMode`.
- Sample interval: `gui.RadioButtonGroupRow` or `gui.Select` → 0.5s/1s/2s/5s,
  bound to `app.Interval`.

### Process table

The go-gui `Table` widget (`view_table.go`) takes text `TableCellCfg`s and does
not support custom cell content (CPU usage bars, tree chevrons). So build the
table **immediate-mode**, like shirei does, for full cell control:

- A header `gui.Row` of clickable column-title cells. Each is a `gui.Button` (or
  a container with `OnClick`) that sets `app.Sort.Column` / toggles
  `app.Sort.Desc`. Show a ▲/▼ marker on the active column.
- Body: a **scrollable** `gui.Column` (`Scrollable: true` + `ScrollbarCfgY`, per
  `scroll_demo`) containing one `gui.Row` per visible process.
- Each row: fixed-width cells matching the header. Cells:
  - PID/USER/STATE/MEM%/RSS/THR: `gui.Text` (`--` when `MetricsUnknown`).
  - CPU: `gui.Text` + a `UsageBar` (see below).
  - NAME: in tree mode, leading indent (`Width = depth*14`) + a ▸/▾ chevron
    button toggling `p.Collapsed`, then the name.
  - Row background: alternate `ColorPanel`/`ColorInterior`, `ColorSelect` when
    `app.Selected == p`, and `ColorHover` on hover. Row `OnClick` sets
    `app.Selected = p`. `OnClick` is consume-class, so the click is marked
    handled by dispatch.
- Row ordering, filtering, and tree flattening are computed by the app (port
  shirei's `visibleRows` / `treeRows` / `orderProcesses` — pure Go), NOT by the
  table widget. The active sort column supplies the comparator.

`UsageBar(value, max)`: a fixed-size rounded container (`ColorInterior`) holding
an inner `gui.Rectangle` whose width = `ratio * fullWidth`, filled with an
accent color (`ColorSuccess` scaling toward `ColorWarning`/`ColorError` at high
load, or a single accent for simplicity).

### Detail panel + history charts (`chart.go`)

Selected-process panel: name, pid, ppid, cpu, rss, start time. "Stopped …" shows
in `ColorError` when the process is not running. The full command line shows in
muted text. Then two `UsageChart`s (CPU, RAM) sit side by side.

`UsageChart` — port shirei's container-bar chart, restyled with theme tokens:

- `resampleHistory(hist, 60s, 1s, valueFn)` →
  `[]HistBucket{Value, HasData, Interpolated}`. Port verbatim (pure Go,
  unit-tested).
- Render: a fixed-size rounded panel with a `gui.Row` of per-bucket columns.
  Each bucket is a `gui.Column` with a bottom-anchored `gui.Rectangle` whose
  height = `ratio * chartHeight`. Empty buckets → 1px baseline. Interpolated
  buckets → dimmer fill. A floating title label over the bars.
- CPU chart pins the y-scale floor at 100%. RAM chart auto-scales to the max
  seen (port `ramHistoryScale`).

go-gui note: verify bottom-anchoring bars. shirei uses `Filler(1)` to push the
bar down. In go-gui, achieve the same with a spacer child (empty `FillFill`
container) above a fixed-height bar in a `Column`, or `VAlign: VAlignBottom` on
the bucket column. Verify during implementation.

## CLI flags (`main.go`)

Keep the subset that maps cleanly:

- `-once` : print one sorted terminal report and exit (no window). Port
  `runOnce` + `formatBytes`/`truncate` helpers. Pure Go, testable.
- `-limit N` : rows in `-once` (default 10).
- `-sort cpu|mem|pid|name|user|state|threads` : starting/`-once` sort column.
- `-refresh D` : GUI sample interval default (default 1s).

Drop `-png` (no headless render API). Drop `-samples`/`-period` (those drive
shirei's multi-sample burst window for interval CPU%, unused with `ps pcpu`).
Reintroduce only if the CPU-delta enhancement lands.

## Testing (headless, no backend)

Follows CLAUDE.md: rebuild + `go test ./examples/process_monitor/...`.

- `store_test.go`: `Update` adds/updates/evicts. PID-reuse produces a distinct
  key. Ring buffer caps at `maxHistoryPoints`. Stopped processes linger then
  evict. Selected process is not evicted.
- `tree_test.go`: `treeRows` builds correct depth/child-count. Collapse hides
  subtree. Filter keeps matched nodes' ancestors and ignores collapse.
- `chart_test.go`: `resampleHistory` — same-slot averaging, gap interpolation
  flagged `Interpolated`, pre-first-sample slots stay `HasData=false`, fixed
  wall-clock bucket boundaries (port shirei's intent).
- `view_test.go`: seed `App` with a synthetic `Snapshot`, build the root view
  via `rootView(w).GenerateLayout(w)` (nil backend, per CLAUDE.md test pattern),
  assert it returns without panic and produces child shapes. Do NOT call the
  real `ps` collector in tests (inject a synthetic snapshot).

Collectors that `exec` `ps`/`tasklist` are not unit-tested (environment
dependent). Keep them thin and behind the `collect()` seam so tests use
synthetic snapshots.

## Docs deliverables

Per `feedback_doc_sync`:

- `examples/process_monitor/README.md`: what it does, how to run
  (`go run ./examples/process_monitor`), `-once` usage, the container-chart and
  background-sampler techniques, and the known CPU%-is-lifetime-average caveat.
- Root `README.md` example list: add `process_monitor` with a one-line blurb.
- `CHANGELOG.md`: new-example entry.

## Open questions

1. **CPU%**: accept `ps pcpu` (lifetime average, one exec/sample) for v1, or
   implement shirei-style interval CPU% from two cumulative-CPU-time reads
   (needs per-platform CPU-time + a `prev` snapshot)? Recommendation: `ps pcpu`
   for v1, note the caveat, enhance later.
2. **Windows fidelity**: ship a reduced `tasklist` collector (CPU% = `--`), or
   mark Windows out of scope for v1 with the `collect_other.go` stub?
   Recommendation: ship the reduced collector. It still lists processes.
3. **StartTime on darwin**: pull `ps -o lstart=` (extra parse cost) for stronger
   PID-reuse keys, or fall back to `{PID}` only? Recommendation: `{PID}`-only
   fallback in v1. Document it.
4. **ThemePicker in toolbar** to demo light/dark, or fixed `ThemeDark`?
   Recommendation: fixed `ThemeDark.WithBorders(true)`. Add the picker only if
   it does not crowd the toolbar.
5. **Refresh latency**: rely on the ~100ms idle poll (simple, no wake), accept
   up to ~100ms lag between sample-ready and repaint? Recommendation: yes. The
   lag is imperceptible at these intervals.
