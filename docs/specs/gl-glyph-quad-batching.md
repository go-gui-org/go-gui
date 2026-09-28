# GL Glyph Quad Batching

Issue: #816. Follows #812 and #815, which fixed the cost of each GL call. This
spec fixes how many calls are made.

## Problem

go-glyph draws text through `glyph.DrawBackend`, one `DrawTexturedQuad` per
glyph. The GL backend (`gui/backend/gl/text.go`) turned each call into 7 GL
calls: `ActiveTexture`, `BindTexture`, `BindVertexArray`, `BindBuffer`,
`BufferSubData`, `DrawArrays`, `BindVertexArray(0)`. For consecutive glyphs from
one atlas page, 4 of those re-bind state that is already bound. A text command
of N glyphs cost `4 + 7N + 1` calls, where the 4 are `useGlyphPipeline` and the
1 is `restoreAfterGlyph`.

After #815 a call allocates nothing, so the remaining cost is CPU and driver
time. An alloc gate cannot see it, and timing gates are advisory. **The success
criterion is stated as draw counts.**

## Design

Glyph quads queue in a batch owned by `glyphBackend`. The batch draws with one
indexed `DrawElements(TRIANGLES)` per run of quads that sample the same texture.

- **Storage.** `batch [maxGlyphQuads][4][8]float32` sits inline in
  `glyphBackend`, which is allocated once per backend. Queueing a quad is a
  128-byte copy. A frame allocates nothing on the heap. `maxGlyphQuads = 1024`,
  so the VBO is 128 KiB and the largest index, 4·1024−1, fits in `uint16`.
- **Indices.** At init a static `ELEMENT_ARRAY_BUFFER` is filled with
  `(4i, 4i+1, 4i+2, 4i, 4i+2, 4i+3)` for every quad. This is the same triangle
  split `TRIANGLE_FAN` made of a single quad, so each quad covers the same
  pixels as before.
- **Scope.** A batch never outlives the text command that filled it.
  `Backend.restoreAfterGlyph` flushes it before the next `RenderCmd`. Scissor,
  stencil, blend, framebuffer and MVP are therefore constant across every quad
  in a batch. GL rasterizes the primitives of one draw call in order, so
  overlapping glyphs blend exactly as they did when drawn one by one.

### Flush triggers

Each trigger below is required for correctness:

| Trigger                                                  | Why                                                                                                                                          |
| -------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| Quad from another texture                                | One draw samples one bound page.                                                                                                             |
| Batch full (`batchN == batchCap`)                        | Capacity. The flush happens right away, so `batchCap = 1` reproduces the old draw-per-quad sequence exactly.                                 |
| `UpdateTexture` / `UpdateTextureRect` on the queued page | go-glyph evicts a page mid-command (`atlas.go` `resetPage`) and re-uploads it before the next quad. Queued quads must sample the old texels. |
| `DeleteTexture` of the queued page                       | Queued quads would otherwise sample a deleted texture name.                                                                                  |
| `DrawFilledRect`                                         | It draws immediately, and it must stay behind the glyphs queued before it.                                                                   |
| `restoreAfterGlyph`                                      | End of the text command. This is the scope rule above.                                                                                       |

An upload to a page other than the queued one leaves the batch alone, because it
cannot change what the queued quads sample. `NewTexture` does not flush: it
touches no existing texture, and `flush` re-binds its own texture anyway.

`renderersDraw`'s deferred cleanup discards any quads still queued, so a
truncated command stream cannot draw them into the next frame. A balanced stream
never has queued quads at that point.

### Result

A text command of N glyphs over P page runs costs `4 + 7P + 1` calls, plus 7
more for each run longer than 1024 quads. For a 40-glyph label on one page that
is 12 calls, down from 285.

## Verification

Golden tests (`gui/golden_test.go`) record `[]RenderCmd` before the backend
runs, so they cannot see this change. The verification is done with real-GL
tests in `gui/backend/gl/glyph_batch_test.go`. CI runs them under Mesa llvmpipe
on Linux and Windows through `scripts/gl-render-test.sh`:

- `TestGlyphBatchFlushCounts`: 40 same-page quads take 1 draw. Pages A,A,B,A
  take 3. An empty command takes 0. 1025 quads take 2.
- `TestGlyphBatchFlushesBeforeUpload`: a quad queued before a page re-upload
  renders from the old contents. An upload to another page does not flush.
- `TestGlyphBatchFlushesBeforeDelete`: a quad queued before its page is deleted
  still renders.
- `TestGlyphBatchMatchesUnbatched`: a frame with overlapping text and a clip
  that cuts through a line is drawn batched on a cold atlas, then with
  `batchCap = 1`. The pixels must be byte-identical.

Two mutation checks were run by hand. Removing the upload/delete flush fails the
upload and delete tests. Removing the command-end flush fails the pixel
comparison, because the batch then leaks across the clip change.

## Out of scope

- `DrawFilledRect` samples whatever texture happens to be bound, and
  `glyphBackend` has no `DrawFilledRectTransformed`. Both are tracked in #835.
  This change keeps the existing behavior.
- Other backends. Metal has its own text path. Web was batched separately in
  f289e6b9.

## Rejected Approaches

- **Global GL state cache** (mirroring program, VAO, buffer and texture
  bindings, and skipping redundant binds). It would need every bind site in
  `draw.go`, `buffers.go`, `text.go`, `textures.go` and `msaa.go` to go through
  the cache. One missed raw bind leaves the cache out of sync, and the symptom
  is a wrong texture, not a crash. It cuts 7 calls per quad to 2, while batching
  cuts them to about 7 per page run. It is larger in scope and smaller in
  payoff. A narrow skip-if-current in `usePipeline` could be considered later if
  a profile asks for it.
- **Batches spanning consecutive text commands**, flushed by the next non-text
  command. It saves only the fixed cost of each command (about 12 calls), and it
  brings back the cache's failure mode: every draw path, and every scissor,
  stencil, FBO or rotation change, would have to flush. A missed one reorders
  draws or ignores a clip.
- **Golden renders as verification.** They cannot observe backend output. See
  Verification.
- **A hook contract for custom pipelines.** Issue #816 asked whether custom
  shader hooks bind their own GL state. They do not. `RenderCustomShader`
  supplies only a GLSL fragment body. The backend compiles it into a program it
  owns (`getOrBuildCustomPipeline`) and draws it with `drawQuad`. `glbind` is
  `internal`, so no consumer can issue GL calls. There is no contract to define,
  and consumers are not affected.
