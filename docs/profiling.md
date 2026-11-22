# Profiling

Practical pprof workflows for go-gui. It covers CPU, heap, and benchmark-driven
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

`-benchmem` prints B/op and allocs/op per benchmark. Use it to spot regressions
before you examine the pprof output.

## Hot paths

The go-gui render pipeline has well-defined hot paths. When investigating
performance, profile these first:

| Path                  | Function                              | Benchmark                       |
| --------------------- | ------------------------------------- | ------------------------------- |
| View → Layout tree    | `generateViewLayout`                  | `BenchmarkGenerateViewLayout`   |
| Layout → RenderCmd    | `renderLayout`                        | `BenchmarkRenderLayout`         |
| Layout arrangement    | `layoutArrange`                       | `BenchmarkLayoutArrange`        |
| SVG cache hit         | `LoadSvg` (cached)                    | `BenchmarkSvgLoadCacheHit`      |
| SVG cache miss        | `LoadSvg` (uncached)                  | `BenchmarkSvgLoadCacheMiss`     |
| SVG render            | `renderSvg`                           | `BenchmarkRenderSvgAnimated`    |
| Wrap container layout | `layoutWrapContainers`                | `BenchmarkLayoutWrapContainers` |
| Focus traversal       | `nextFocusable` (method on `*Layout`) | `BenchmarkFocusTraversal`       |

## GPU submission counters

pprof shows CPU time and allocations. It does not show what the backend sends to
the GPU. Set `GOGUI_RENDER_STATS=1` and the Metal (macOS) and GL (Linux,
Windows) backends print one line per second to stderr (#1303):

```text
rstats backend=metal frames=120 draws=29.1 quads=967.9 ... submit_us_p50=431 gpu_us_p50=635 ... flush_pipeline=24.1 flush_scissor=5.0 ...
```

Each count is a mean per frame over that second:

- `draws`: draw calls. `quads` and `verts`: quads and triangle-mesh vertices
  drawn.
- `pipe_binds`, `tex_binds`, `uniforms`: state calls sent to the driver while
  drawing. Metal skips a bind whose state is already set, so it counts state
  changes. GL has no such cache: it sends the program and its projection on
  every draw, and the count shows that cost. Texture uploads are not counted.
- `upload_kib`: vertex data copied to the GPU. `grows`: GPU buffers created or
  grown.
- `flush_<cause>`: quad batches drawn, by the state change that ended each one.
  The causes are `pipeline`, `mvp`, `uniform`, `frag0`, `scissor`, `texture`,
  `sampler`, `stride`, `cap`, `chunk`, `triangles`, `pass`, `text` and `other`.
  GL batches only glyphs, so its other quads add to `draws` and to no cause.
- `submit_us_p50`/`_p99`: CPU time from the start of the backend's frame to the
  commit. It does not include the swap, which can wait for vsync.
- `gpu_us_p50`/`_p99`: GPU time per frame. Metal reads it from the command
  buffer. GL uses `GL_TIME_ELAPSED` queries. `-1` means that no time was
  reported.

The counters cost nothing when the variable is not set. When it is set, Metal
adds no allocations. GL's timer queries go through purego and allocate a few
times per frame.

`go run ./tools/renderbaseline` runs a fixed set of example workloads with the
counters on and prints the median of each field per workload. Add `-csv out.csv`
to write a CSV file too. `docs/specs/gpu-batching-investigation.md` has the
baselines and what they mean.

The web backend draws with Canvas2D and does not print these lines. Its
`goGuiOnFrame` page hook (`examples/web_svg_bench`) reports per-frame command
and composite counts.

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

The most common allocation source in hot paths is slice growth. When the upper
bound is known, pre-size with `make([]T, 0, cap)`. Check `renderLayout` and
`generateViewLayout` call trees for slice append without pre-allocation.

### Scratch pool review

`scratchPools` in `gui/scratch_pools.go` holds reusable per-frame buffers for
frequently allocated types. When you add new hot-path allocations, consider
pooling them.
