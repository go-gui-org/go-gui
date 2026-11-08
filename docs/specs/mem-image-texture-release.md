# Releasing the textures of in-memory images

Issue #1230. Status: **implemented**.

## Problem

`DropImage` removed an image from the in-memory registry, but every backend kept
the texture it had uploaded for it. The texture caches are bounded by count (128
textures on GL, Metal, iOS and Android, 256 canvases on web), so the texture
left only when enough other images pushed it out: up to 128 dropped images per
window stayed on the GPU, about 2 GiB at 2048×2048. Since #1228 this matters
more, because `RegisterEncodedImage` images are never evicted and `DropImage` is
the only way to free them.

A probe on the real GL backend showed more than the leak:

- **Stale content.** Registering the same key again, after a drop or at a new
  size, drew the old pixels. The backend cache is keyed by the `Src` string,
  which names a key, not a registration.
- **Path fall-through.** A dropped `mem:` source was handed to the file loader,
  which logged an error and cached an empty texture under it in a second slot.
- **Web panic.** A dropped key found its canvas in the web image cache, whose
  other entries are `Image` elements, and reading `complete` from the canvas
  panicked in `js.Value.Bool`.
- **Every removal, not only `DropImage`.** Budget eviction and re-registration
  at a new size left textures behind in the same way.

## Why the registry cannot delete the texture

`DropImage` cannot reach a texture:

- `gui` cannot call the backends. They import `gui`, not the other way round.
- `DropImage` runs on whatever goroutine the app calls it from. A texture must
  be deleted on its window's render thread with that window's context current; a
  delete under another window's context frees that window's object of the same
  number (#921).
- Every window has its own cache, and a minimized window may not draw for
  minutes.

So the registry leaves a note and each backend reads it on its own thread.

## Decision

The note is a counter, and each texture remembers which registration it came
from.

1. Every registration gets a `serial`: unique for the process, never reused,
   also under the same key. Repeating a key at the same size or pinning it keeps
   the serial; a drop followed by a new registration, or a registration at a new
   size, gets a new one.
2. `removeLocked`, the one function every removal goes through (drop, budget
   eviction, re-registration at a new size), increments an atomic generation.
3. `LookupImage` returns the pixels, dimensions and serial as a `MemImage` from
   one locked lookup. A backend stores the serial with the texture it uploads,
   and a draw that finds a different serial uploads again.
4. At frame start, next to the `PixelSurface` prune, a backend whose remembered
   generation differs from `ImageRegistryGen` sweeps its cache: every `mem:`
   texture whose key `ImageSerial` no longer finds, or finds under another
   serial, is deleted. File and URL textures are left to the count LRU.

The shared part lives in `gui/backend/internal/memtex`: `Sweeper.Due` (has the
generation moved) and `Current` (may this texture stay). `texcache.Sweep` walks
a cache and destroys the entries a predicate rejects. Each backend keeps only
its upload code. Web holds its `mem:` canvases in a `texcache` of their own,
apart from the `Image` elements, and releases a canvas by shrinking it to 0×0.

A dropped `mem:` source now draws nothing: backends test `IsMemImage` first and
never pass a `mem:` source to their file or URL handling.

### Cost

A frame in which nothing left the registry costs one atomic load per window. A
frame after a removal walks at most the cache size (128 entries, 256 on web),
with one registry lock and map lookup per `mem:` entry, and allocates nothing:
the sweep predicates are plain functions. Ten removals between two frames cause
one sweep. A color picker drag evicts variants on most frames, so the sweep runs
on most frames of the drag, at a few microseconds each.

### Concurrency

`gen` is the only field read without the registry lock, so it is the only atomic
one. `lastSerial` is read and incremented in the same locked section that
inserts the entry carrying the new value; an atomic counter would not make that
pair indivisible. The texture's serial and the backend's remembered generation
are confined to one render thread.

Two windows close by construction:

- Pixels and serial come from one locked lookup, so a re-registration between
  two calls cannot pair old pixels with a new serial. Such a pairing would keep
  a stale texture for good, because its serial would look current.
- The backend reads the generation before it walks its cache. A removal after
  that read moves the generation again, so the next frame sweeps again. A
  removal that moved it just before the read is complete when `ImageSerial`
  runs, because the increment happens inside the locked section `ImageSerial`
  must wait for.

`TestImageSerialConcurrentSweep` runs registration, drop and eviction on four
goroutines against a renderer that sweeps and draws. Under `-race` it pins the
locking; on every run it checks that a serial always comes with the same buffer
and that a final sweep leaves only textures matching the registry. A registry
without the generation bump fails it.

## What it does not do

- **Mutating a registered buffer in place** still never reaches the backends.
  Content keying stays the contract: a changed buffer gets a new key, or the key
  is registered again.
- **Images that stay registered** keep their textures, bounded only by count. A
  byte budget for the texture caches, which also covers file and URL images, is
  #1239.
- **The soft backend** needs nothing: its renderer lives for one `RenderToImage`
  call and caches nothing across frames.

## Rejected approaches

- **A drop log.** The registry records dropped keys and backends drain them.
  `gui` cannot know when every window, including a paused one, has read an
  entry, so the log is either unbounded or a slow window misses entries and
  needs a full check as a fallback. That fallback is this design; the log would
  only be added on top.
- **A drop callback registered by backends.** It runs on the caller's goroutine,
  and a GL delete needs the render thread, so the callback could only enqueue:
  the drop log again.
- **A unique `Src` per registration** (`mem:logo#<id>`). It stops the stale draw
  but frees nothing, and apps write `mem:` sources by hand: markdown accepts
  `![logo](mem:logo)`, which would point at nothing after a re-registration.
- **Freeing `mem:` textures not drawn for N frames**, as `PixelSurface` textures
  are. It bounds GPU memory to what is on screen but uploads every image that
  comes back again (16 MiB for 2048×2048), and go-gui skips idle frames, so "N
  frames" is not a span of time.
- **Recency release for every texture.** It decodes file images again whenever
  they scroll back into view, which the count LRU exists to avoid.
- **Comparing pixel buffer addresses** instead of a serial. Once the old buffer
  is collected, a new one can take its address, and the stale texture would look
  current.
