# Profiling

Practical pprof workflows for go-gui. Covers CPU, heap, and benchmark-driven
profiling.

## Quick start

```bash
# CPU profile of running all tests
go test -cpuprofile=cpu.prof -count=1 ./gui/...
go tool pprof -http=:8080 cpu.prof

# Heap profile (allocation count)
go test -memprofile=mem.prof -count=1 ./gui/...
go tool pprof -http=:8080 mem.prof

# Benchmark with memory profile
go test -bench=. -benchmem -memprofile=bench.prof -count=1 ./gui/
go tool pprof -http=:8080 bench.prof
```

Open `http://localhost:8080` — flame graph, top list, and source view.

## CPU profiling

### Profile a single benchmark

```bash
go test -bench='BenchmarkRenderLayout/nested_4x10' \
  -cpuprofile=cpu.prof -count=5 ./gui/
go tool pprof -http=:8080 cpu.prof
```

`-count=5` increases sample count for statistically meaningful profiles.

### Profile an example app

```bash
# Build with profiling hooks (add to your main.go):
#   import _ "net/http/pprof"
#   import "net/http"
#   go func() { http.ListenAndServe(":6060", nil) }()

go run ./examples/get_started/ &
# Interact with the app, then:
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/profile?seconds=30
```

### Profile a 30-second render loop

```bash
go tool pprof -http=:8080 \
  http://localhost:6060/debug/pprof/profile?seconds=30
```

## Heap profiling

### Allocation count vs in-use

```bash
# Allocation count (default — what was allocated)
go test -memprofile=mem.prof -count=1 ./gui/...
go tool pprof -http=:8080 mem.prof

# In-use memory (live objects at profile time)
go test -memprofile=mem.prof -count=1 ./gui/...
go tool pprof -sample_index=inuse_space -http=:8080 mem.prof
```

### Benchmark with allocation breakdown

```bash
go test -bench='BenchmarkRenderLayout' -benchmem -count=1 ./gui/
```

`-benchmem` prints B/op and allocs/op per benchmark. Use to spot regressions
before diving into pprof.

## Hot paths

The go-gui render pipeline has well-defined hot paths. Profile these first when
investigating performance:

| Path                  | Function               | Benchmark                       |
| --------------------- | ---------------------- | ------------------------------- |
| View → Layout tree    | `generateViewLayout`   | `BenchmarkGenerateViewLayout`   |
| Layout → RenderCmd    | `renderLayout`         | `BenchmarkRenderLayout`         |
| Layout arrangement    | `layoutArrange`        | `BenchmarkLayoutArrange`        |
| SVG cache hit         | `LoadSvg` (cached)     | `BenchmarkSvgLoadCacheHit`      |
| SVG cache miss        | `LoadSvg` (uncached)   | `BenchmarkSvgLoadCacheMiss`     |
| SVG render            | `renderSvg`            | `BenchmarkRenderSvgAnimated`    |
| Wrap container layout | `layoutWrapContainers` | `BenchmarkLayoutWrapContainers` |
| Focus traversal       | `NextFocusable`        | `BenchmarkFocusTraversal`       |

## Allocation targets

Zero-allocation is the goal on frame-critical hot paths.

| Path                        | Current allocs | Target |
| --------------------------- | -------------- | ------ |
| SVG cache hit               | 0              | 0 ✓    |
| layoutFillWidths/Heights    | 2 per node     | 0      |
| StateMap (hot namespaces)   | 1 per shape    | 0      |
| Event wrapper closures      | 2–3 per widget | 0      |
| generateViewLayout children | log₂(n) grows  | 0–1    |
| renderLayout (flat)         | varies         | ↓      |

Run benchmarks before and after changes to detect allocation regressions:

```bash
# Before
go test -bench='BenchmarkRenderLayout' -benchmem -count=5 \
  -cpuprofile=before.prof ./gui/ > before.txt

# After
go test -bench='BenchmarkRenderLayout' -benchmem -count=5 \
  -cpuprofile=after.prof ./gui/ > after.txt

# Compare
go tool pprof -base=before.prof after.prof
benchstat before.txt after.txt
```

## Golden tests

SVG tessellation and animation changes must keep golden tests passing. Golden
files live in `gui/svg/testdata/`:

| Golden file               | Covers                   | Regenerate flag  |
| ------------------------- | ------------------------ | ---------------- |
| `phase0_smil_goldens.txt` | SMIL animation output    | `-phase0-update` |
| `phaseG_css_goldens.txt`  | CSS spinner fingerprints | `-phaseG-update` |

Run goldens before and after SVG changes:

```bash
go test ./gui/svg/ -run 'TestPhase0SmilSpinnerFingerprint|TestPhaseGCssSpinnerFingerprint' -count=1
```

If a tessellation or animation change intentionally alters output, regenerate:

```bash
go test ./gui/svg/ -run TestPhase0SmilSpinnerFingerprint -phase0-update
go test ./gui/svg/ -run TestPhaseGCssSpinnerFingerprint -phaseG-update
```

Review the diff in `testdata/` before committing regenerated goldens.

## Common patterns

### Finding allocation hot spots

```bash
go test -bench='BenchmarkRenderLayout' -benchmem -memprofile=mem.prof ./gui/
go tool pprof -alloc_space -http=:8080 mem.prof
```

In the flame graph view, focus on thick stacks — those are your allocation
volume.

### Reducing slice growth allocations

The most common allocation source in hot paths is slice growth. Pre-size with
`make([]T, 0, cap)` when the upper bound is known. Check `renderLayout` and
`generateViewLayout` call trees for slice append without pre-allocation.

### Scratch pool review

`scratchPools` in `gui/scratch_pools.go` holds reusable per-frame buffers
for frequently allocated types. When adding new hot-path allocations,
consider pooling.
