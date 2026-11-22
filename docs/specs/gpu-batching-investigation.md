# GPU Batching Investigation

Issue #1303. Status: **investigation**. Metal measured. GL baselines are still
needed from a machine with a real GPU.

## Question

Does go-gui lose frame time to draw-call volume, state changes or buffer
uploads? Would instanced quads, merged geometry, atlas batching or cached
display lists give a measurable gain? Before this work, no backend could answer
this question. `WindowCfg.Timings` measures only CPU phases, and
`docs/specs/qcpainterbench.md` already notes that about half of a heavy frame
was "GPU work, present, or both", with no way to measure which.

## What was added to measure it

- `GOGUI_RENDER_STATS=1`. The Metal and GL backends print one line per second.
  The line gives draws, quads, mesh vertices, binds, uploaded bytes, CPU submit
  time, GPU time, and the cause that ended each quad batch. The code is in
  `gui/backend/internal/rstats`. With the variable unset, every counter call is
  a method on a nil pointer and returns at once. With it set, Metal adds 0
  allocations (`TestRecorderDoesNotAllocate`). Field meanings are in
  `docs/profiling.md`.
- GPU time:
  - Metal reads `GPUStartTime`/`GPUEndTime` from the command buffer of the ring
    slot that `beginFrame` already waits on. This adds no wait.
  - GL uses a ring of 3 `GL_TIME_ELAPSED` queries (GL 3.3 core). It reads only
    results that are available.
- `tools/renderbaseline` runs a fixed set of workloads, each 3 times for 8 s. It
  drops the first 2 lines of each run and prints the median of each field.
- `examples/benchmark` has `-type`, `-count`, `-seconds` and `-warmup`.
  `examples/virtual_list` and `examples/data_grid_data_source` have
  `-autoscroll`.

## Current batching strategy

| Backend         | SDF quads (rect, border, shadow…)                                                                          | Glyphs                                                                      | SVG / meshes                                       | State cache                                    |
| --------------- | ---------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- | -------------------------------------------------- | ---------------------------------------------- |
| Metal (macOS)   | One shared stream. One draw per run of the same pipeline, MVP, `tm`, `frag0`, scissor, texture and sampler | In the same stream, but a different pipeline, so text ↔ rect is a new batch | Each mesh is its own draw                          | Yes (`encState`): a bound value is not re-sent |
| GL (Linux, Win) | **Not batched.** Each quad is a 4-vertex `BufferSubData` and `DrawElements` (`gl/buffers.go`)              | Batched per atlas page (#816). Flushed at the end of each text command      | Runs of meshes merged into one `DrawArrays` (#895) | No: `UseProgram` and the MVP go on every draw  |
| Web             | Canvas2D calls                                                                                             | Canvas2D `fillText`                                                         | WebGL2 batch, composited with `drawImage` (#1045)  | n/a                                            |
| iOS, Android    | One draw per quad                                                                                          | One draw per glyph                                                          | One draw per mesh                                  | No                                             |

The limits of the current strategy:

- Metal ends a batch on every pipeline change. Widget scenes alternate between
  the solid and glyph pipelines, so almost every batch ends this way.
- Metal ends a batch on every scissor change. A grid clips each cell, so each
  cell is a batch.
- Metal does not merge mesh runs. A canvas-heavy scene is one draw per mesh.
- GL has no quad batching and no state cache. That is the largest structural gap
  in any desktop backend, but it is not measured yet.

## Baselines (Metal)

Machine: Apple M5, built-in Retina display at 120 Hz, macOS, 4x MSAA. Command:
`go run ./tools/renderbaseline -runs 3 -seconds 8`. Each value is the median of
the per-second means, per presented frame.

| Workload                      | frames/s | draws  | quads | mesh verts | upload KiB | submit µs p50 | GPU µs p50 | GPU µs p99 | Batch ends by cause      |
| ----------------------------- | -------- | ------ | ----- | ---------- | ---------- | ------------- | ---------- | ---------- | ------------------------ |
| benchmark Mixed 5000          | 54       | 306    | 1463  | 0          | 188        | 287           | 602        | 1356       | pipeline 299, scissor 7  |
| benchmark Button 1000         | 121      | 439    | 1781  | 0          | 230        | 385           | 1436       | 1884       | pipeline 432, scissor 7  |
| virtual_list `-autoscroll 40` | 121      | 29     | 968   | 0          | 121        | 504           | 642        | 1266       | pipeline 24, scissor 5   |
| data_grid `-autoscroll 40`    | 121      | 284    | 1193  | 0          | 151        | 1323          | 2531       | 3174       | scissor 258, pipeline 26 |
| digital_rain                  | 34       | 2      | 482   | 0          | 60         | 648           | 427        | 1203       | pipeline 2               |
| animation_stress              | 121      | 8      | 39    | 60         | 7          | 134           | 286        | 1086       | pipeline 4, scissor 2    |
| svg_spinners                  | 60       | 56     | 119   | 3164       | 126        | 394           | 2488       | 5303       | scissor 12               |
| custom_shader                 | 121      | 7      | 45    | 0          | 6          | 152           | 183        | 1550       | pipeline 7               |
| qcpainterbench `-count 64`    | 48       | 13,795 | 4077  | 1,341,292  | 47,685     | 4517          | 18,655     | 19,733     | pipeline 15, scissor 4   |

There are no rows for `markdown`, `shadow_demo`, `gradient_demo` and
`blur_demo`. Once idle they present no frames, so they print no lines. To
measure a static scene, a workload must force a redraw each frame.

## What the numbers mean

**Widget, list and text scenes are not GPU-bound and not draw-bound on Metal.**

- Submit time is at most 1.3 ms and GPU time at most 2.5 ms. The 120 Hz budget
  is 8.3 ms.
- Mixed 5000 runs at 54 frames/s because of CPU view and layout time, not
  rendering (see the finding below).
- 300–430 draws per frame are cheap on Metal. Merging the solid and glyph
  pipelines would remove most of those draws, but it can save at most the
  0.3–0.4 ms submit time. That is not worth an uber-shader.

**The data grid is scissor-bound in draw count, but within budget.**

- 258 of its 284 batches end on a scissor change, one per clipped cell.
- A CPU-side test could keep the open batch when a quad falls fully inside the
  current clip. That would remove most of these batches.
- The gain is under 1.3 ms of submit time, in a scene that already runs at the
  display rate.

**The canvas stress test is GPU-bound, and its mesh path is draw- and
upload-heavy.**

- qcpainterbench at 64 passes takes 18.7 ms of GPU time in a 20.8 ms frame.
- It makes about 13,800 draws per frame. All but about 20 of them are single
  meshes, because Metal does not merge mesh runs.
- It uploads 47 MB of vertices each frame: 1.34 M vertices × 36 bytes.
- The data does not show whether the GPU time comes from the vertex volume, from
  the per-draw cost, or from fill and 4x MSAA over overlapping triangles.
  Merging mesh runs would cut the draws and part of the 4.5 ms submit time. It
  would not cut the upload. GPU time would fall only if per-draw cost is a large
  part of it.

**Finding outside batching: the benchmark counts passes, not frames.**

- `FrameFn` runs a second pass while a refresh is pending
  (`gui/window_update.go`, the 2-pass loop).
- `examples/benchmark` calls `InvalidateLayout()` from its view on every run. So
  each presented frame builds the view twice.
- The result: its readout shows 108 "FPS" while the backend presents 54 frames.
  Its view and layout means are per pass, so the real CPU cost per frame is
  about twice what it shows.
- `rstats` counts presented frames, so its `frames` value is the right one.
  `qcpainterbench` uses the same self-invalidate pattern, so its recorded
  numbers need the same check.

## Ranked opportunities

| Rank | Change                                                                                    | Backend   | Measured or expected benefit                                                                                          | Complexity | Risk                                          |
| ---- | ----------------------------------------------------------------------------------------- | --------- | --------------------------------------------------------------------------------------------------------------------- | ---------- | --------------------------------------------- |
| 1    | Measure GL on a real GPU (this tool, unchanged)                                           | GL        | Decides rank 2. GL makes one draw per non-glyph quad, so its widget-scene draw counts are higher than Metal's 300–440 | None       | None                                          |
| 2    | Port Metal's quad stream and state cache to GL                                            | GL        | Expected: large on widget scenes, if the GL baselines show submit time as a large share                               | Medium     | Text and filter state on GL; real-GL tests    |
| 3    | Fix the benchmark's double pass (separate issue)                                          | examples  | Correct numbers; the view and layout cost per frame is twice the readout                                              | Low        | None                                          |
| 4    | Experiment: merge mesh runs on Metal, and test qcpainter with MSAA off / a smaller window | Metal     | Cuts about 13,700 draws and part of 4.5 ms submit. GPU gain unknown until fill is ruled out                           | Low–Medium | Mesh color handling (#895 did this on GL)     |
| 5    | Keep the batch open when a quad is inside the current clip                                | Metal, GL | Under 1.3 ms submit on the data grid; scene already at display rate                                                   | Low        | Clip edge cases; must not paint outside clips |
| 6    | Solid + glyph uber-pipeline                                                               | Metal     | At most 0.3–0.4 ms submit on widget scenes                                                                            | Medium     | Shader complexity, text quality               |

## Recommendation

- **Metal: leave the batching as it is for widget, list and text workloads.**
  The numbers show no draw-call or GPU limit there.
- **Run one focused experiment for canvas and mesh-heavy scenes (rank 4).**
  First find out whether GPU time follows vertex count, draw count or fill.
  Merge mesh runs only if draw count is a measurable part of it.
- **GL: get the baselines before any change (rank 1).** If submit time is a
  large share of the frame on real hardware, rank 2 is the targeted change. It
  ports a design that is already proven on Metal, and it does not import a new
  architecture.

## Verification

- `go test ./gui/backend/internal/rstats ./tools/renderbaseline` covers the
  counters, the report line, the 0-alloc rule and the CSV and median logic.
- `TestStatsCountFlushCauses` (Metal) checks the per-cause counts with a real
  encoder.
- `TestRenderStatsCounts` and `TestGPUTimerReportsFrames` (GL) run under
  `tools/glrendertest` with Mesa llvmpipe. They check per-quad draws, the
  glyph-batch causes, the SVG run merge and the timer-query ring.

## Not done

- **GL baselines on a real GPU.** On Linux or Windows, run:
  `go run ./tools/renderbaseline -label "<GPU>" -csv gl.csv`. Llvmpipe timings
  mean nothing, because it rasterizes on the CPU.
- **Web counters.** The web backend draws with Canvas2D, so GPU batching does
  not apply to it. The existing `goGuiOnFrame` hook reports per-frame command,
  SVG and composite counts. To count each Canvas2D call, about 80 call sites
  would need a wrapper. There is no batching decision that needs those counts.
- **iOS and Android.** They have no batching and no measurement setup. This
  investigation does not rank them.
- **Static scenes.** These are the workloads with no rows in the baseline table.

## Rejected Approaches

- **Instanced quads.** With a static index buffer, a run of quads is already one
  draw. Instancing saves only vertex bytes, and widget scenes upload 120–230 KiB
  per frame.
- **Cached display lists or layers.** They conflict with the immediate-mode
  design, which has no diffing, and they bring invalidation bugs. Command build
  (`RenderBuild`) is not the limit in any measured scene.
- **A rewrite to a small set of SDF pipelines in the style of Gowaku.** #1303
  says not to copy another renderer's architecture, and no measured scene points
  to it.
