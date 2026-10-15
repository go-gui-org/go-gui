# Web backend: SVG meshes with WebGL2

Issue #1045. Status: **implemented**.

## Problem

The web backend drew every SVG mesh as Canvas2D paths on every frame. Go
transformed each vertex and copied the mesh into JS, and the browser rebuilt and
rasterized one path per same-color run. A gradient mesh has per-vertex colors,
so it became up to one `fill` per triangle, and drew as flat triangles because
Canvas2D has no per-vertex color. A grid of SVG illustrations (~150k visible
triangles) scrolled at 3–14 fps; a page of gradient logos took ~2 s per frame.

## Design

SVG meshes are drawn with WebGL2 into an offscreen canvas and composited onto
the 2D canvas with `drawImage`. Everything else keeps its Canvas2D path, so
text, images, clips, filters and transforms are unchanged.

- **Mesh cache** (`draw_webgl.go`, `mesh_cache.go`). A mesh is uploaded once and
  drawn by id. It is keyed by its backing array and its content hash (triangles
  and vertex colors), computed each frame because DrawCanvas rewrites buffers in
  place; the entry pins the slice, so the address cannot be reused. Commands
  that share triangles but differ in colors (a tint, an opacity, a disabled
  state) are separate variants and coexist, so one never deletes a mesh another
  command has queued. A buffer rewritten in place becomes a new variant; one
  backing array keeps at most 8, dropping the least recently used that the
  current frame has not drawn. Idle meshes are evicted. A mesh is cached only
  once it is drawn in a second frame: an animated SVG path is tessellated into a
  new slice every frame, so its meshes never repeat, and cached on first sight
  each frame would add one per path, uploaded and pinned until it idled out.
  Meshes not cached are drawn from transient slots whose GPU buffers the next
  frame refills; slots a frame does not use are freed. The Path2D cache shares
  this policy (`meshCache`). Freed ids are reused: ids travel to JS in a
  `Float32Array`, exact only up to 2^24. The hash (`mesh_hash.go`) folds 8 bytes
  per step with a 64-bit xor and multiply in four lanes; `hash/maphash` takes a
  software fallback on wasm that was a third of the frame.
- **Batches.** Consecutive `RenderSvg` commands render in one GL pass into an
  MSAA canvas, and one `drawImage` composites the batch. Go writes one record
  per draw into a shared `Float32Array` and calls the JS loop (`goGuiSvgGL` in
  `gogui.js`) once per batch. Any other command ends the batch, so a batch draws
  under the 2D state it was queued under.
- **Two passes, one staging copy** (`prepassSvgGL`). Firefox snapshots a WebGL
  canvas for every `drawImage` from it, even unchanged, so a frame of many small
  batches paid one snapshot each. A frame now runs twice. The pre-pass runs the
  clip, transform, filter and SVG commands through the same batching with the 2D
  context swapped for a no-op stand-in (`goGuiSvgGL.nullContext`), renders every
  batch into the WebGL canvas, and copies the drawn area once into a 2D staging
  canvas. The drawing pass composites each batch from the copy; `drawImage` from
  a 2D canvas is cheap. A batch renders at its own place unless an earlier batch
  of the frame took part of it; then it goes on a shelf in an overflow half
  below, which the drawing buffer grows into the first time a frame needs it and
  drops after 300 display frames that do not, including while the page needs no
  repaint. A browser that cannot give the doubled buffer scales a smaller one,
  which would misplace every draw, so the buffer goes back to the canvas size
  and is not grown again until the canvas size changes. A batch with no place,
  or one the drawing pass sees differently from the pre-pass, is rendered and
  composited on its own. The pre-pass restores the 2D context and the frame
  state however it ends, and the frame releases the window lock: a panic left on
  the stand-in, or holding the lock, would leave every later frame blank.
- **Rect clips.** Each SVG sits between two `Clip` commands, so flushing on
  every clip would give one composite per SVG. With no view transform open, a
  rect clip is applied lazily: it goes into the draw's record as a GL scissor,
  the composite runs under the non-rect clips only, and the full clip stack is
  replayed before the next non-SVG command and at the end of the frame (the next
  frame clears under the last clip; a stale one let the canvas accumulate).
  `rebuildState` is `rebuildClipStack` with the rect-override and skip-rect
  parameters this needs.
- **View transforms and rotations.** A rect clip is a window rect, while the
  geometry inside a transform is in the view's frame, so a scissor there would
  cut in the wrong place. Inside a transform, clips apply eagerly: a clip change
  flushes the batch, and the batch draws without scissors and composites under
  the canvas clip and transform. A batch whose bounds in the view's frame leave
  the canvas is rendered elsewhere in the WebGL canvas (on an overflow shelf, or
  at its origin when direct) and composited at its own place; only one larger
  than the canvas is filled with Canvas2D.
- **Context loss.** `webglcontextlost` (with `preventDefault`) switches SVGs to
  Canvas2D and forgets the uploaded meshes; `webglcontextrestored` rebuilds the
  program and resumes. A batch queued when the context goes is filled instead.
  While the context is lost, SVGs draw from the Path2D cache.
- **Selection** (`initSvgMode`). WebGL2 when available, else the Path2D cache,
  else plain Canvas2D paths. A page opts out with
  `globalThis.goGuiSvgMode = "canvas2d"` before Go starts; the backend writes
  the mode in effect back to that global. No Go API change.

Gradient meshes now render with interpolated vertex colors, which is what the
SVG describes. Canvas2D has no per-vertex color, so where SVGs still fall back
to it they draw flat: while the WebGL context is lost, and for a batch under a
transform that is larger than the canvas.

## Measured

`examples/web_svg_bench`, mixed grid, same window per browser. Chrome and
Firefox on Windows 11 (Intel Iris Xe), Safari on macOS (Apple M3 Pro):

| Browser     | Canvas2D | Path2D cache | WebGL2 |
| ----------- | -------- | ------------ | ------ |
| Chrome 150  | 3.4      | 17.3         | 59.6   |
| Firefox 157 | 9.2      | 9.7          | 59.6   |
| Safari 26.6 | 13.6     | 16.4         | 59.7   |

Firefox with the staging copy, against one composite per batch from the WebGL
canvas: mixed grid 42 → 59.6, gradient logos 15.7 → 59.4, icons 54.8 (then on
Canvas2D under a light-batch threshold) → 59.5. Safari pays the same cost: its
gradient logos went 18.2 → 59.9 (composite time 41 ms → 0.05 ms a frame), and
the mixed grid, icons and tiger run at 60. Chrome stays at 60 on the mixed grid,
logos and tiger.

## Verification

- `draw_webgl_test.go` runs the draw loop against the shared fake 2D context
  (`draw_state_test.go`) and a fake `goGuiSvgGL`: batching across clips, the
  end-of-frame clip, brackets and view transforms, one staging copy per frame,
  overflow placement and shelf packing, the direct fallback, the mesh cache
  (color variants coexist, the variant cap, id reuse; the fake flags a draw of a
  deleted id), context loss and restore. `svg_mode_test.go` covers the
  selection, `mesh_hash_test.go` that any single changed float or color moves
  the hash.
- `examples/web_svg_bench` reproduces the numbers in a browser: `svg=canvas2d`
  for the baseline, `asset=`, `xf=` (cells in view transforms), `lose=1` (drives
  `WEBGL_lose_context`), `shot=1` for screenshots.

## Out of scope

- Drawing everything with WebGL. It would remove the composites, but web text is
  drawn by go-glyph with `fillText`, so it needs a glyph atlas there first.
- Culling inside view transforms. gui culls against nothing inside a transform
  (#1043), so every transformed SVG is drawn each frame, visible or not; that is
  a gui change for every backend.

## Rejected Approaches

- **Draw everything with WebGL now.** Needs a go-glyph glyph atlas first; a
  follow-up, not a prerequisite.
- **Port the desktop GL backend through a `syscall/js` glbind.** Each GL call
  becomes a JS call, and the desktop path makes about five per quad; it also
  needs the glyph atlas.
- **Stack a WebGL canvas under or over the 2D canvas.** SVG draws between rects
  and text; two layers cannot keep that order.
- **WebGPU.** Not in every browser, and rejected for the backends (`CLAUDE.md`).
- **OffscreenCanvas in a worker.** Go's wasm runtime is single-threaded; the
  command stream would cross `postMessage` every frame.
- **Cache whole SVGs as ImageBitmaps.** Breaks on animation and needs a
  re-raster at every scale.
- **The Path2D cache as the fix.** Helps simple meshes in Chrome, but the
  browser still rasterizes every path each frame, and Firefox gains nothing.
  Kept only as the no-WebGL2 and fallback path.
- **A stable mesh key on `RenderCmd`** to skip the content hash. New exported
  surface for under 1 ms per frame once the hash is word-wise.
- **One composite per SVG.** The original cut: 24 composites a frame on the
  bench, and each costs Firefox a snapshot. Lazy rect clips made it one per run.
- **Fill light batches with Canvas2D** (a triangle-count threshold). Shipped in
  the first cut against the Firefox snapshot cost. Canvas2D has no per-vertex
  color, so a gradient drew flat or differently depending on which batch it fell
  in. Every Canvas2D approximation tried (mean color, exact sub-triangles, seam
  strokes, merged runs) showed seams or stripes. The staging copy removes the
  cost the threshold avoided.
- **Render each batch, then composite it** (what the staging copy replaced).
  Simple, but it is one WebGL snapshot per batch in Firefox. Rendering all
  batches first and compositing from the unchanged WebGL canvas measured no
  better there (32 fps against 60 with the staging copy).
