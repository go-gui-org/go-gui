# Texture caches bounded by bytes

Issue #1239. Status: **implemented**.

## Problem

Each backend kept its decoded images in a cache bounded by count only: 128
textures on GL, Metal, iOS and Android, and 256 Image elements on web (with
random batch eviction). One image can be up to 40 million pixels
(`DefaultMaxImagePixels`), which is 160 MiB as RGBA. A gallery of large photos
could keep gigabytes on the GPU per window. On iOS and Android the OS kills an
app that uses too much memory.

#1230 deletes the texture of a `mem:` image when the image leaves the registry.
It does not bound images that stay registered, or file and URL images.

## Design

### Where an app sets the cap

`WindowCfg.MaxTextureBytes int64`. Zero or negative selects the default:

| Backend        | Default                          |
| -------------- | -------------------------------- |
| GL, Metal, web | 256 MiB (`DesktopTextureBudget`) |
| iOS, Android   | 128 MiB (`MobileTextureBudget`)  |

The caches belong to a window, so the cap is per window. It sits beside
`MaxImageBytes` and `MaxImagePixels`, and every backend reads it at init with
them. The count bounds (128, 256) stay as a second bound.

### How the cache evicts

`texcache.NewSized(maxCount, maxBytes, size, destroy)` makes a cache bounded by
count and by bytes. `size` gives the bytes of one value: `w × h × 4` for a
texture, `naturalWidth × naturalHeight × 4` for a web image. A failed load is
cached at size 0.

The backend calls `BeginFrame` once per frame. A `Get` or `Set` marks the entry
with the current frame. **An entry used in the current frame or the previous one
is never evicted.** Eviction runs in `Set`, after the insert, and in
`BeginFrame`, so the textures of an over-budget frame go once they leave the
screen even when no new image follows. It removes the oldest entries until the
cache is within both bounds, and stops at the first entry in use. The LRU order
is also the order of last use, so all entries after that one are in use too.

This gives two results:

- **No upload thrash.** When one frame draws more bytes than the cap, a strict
  LRU deletes each texture just before the frame draws it again, so every frame
  uploads every texture. Here the cache keeps the textures in use and goes over
  the cap by that amount. The frame needs that memory anyway.
- **An image larger than the whole cap** is kept while it is drawn. When it
  stops being drawn, the next upload deletes it.

Two frames and not one: a draw order that changes between frames (a scroll, a
reorder) would otherwise let a texture drawn late in the last frame be evicted
by an upload early in this one.

A frame that uploads nothing deletes nothing. Images that leave the screen stay
until a later upload needs the space.

On web, an Image element is cached at size 0 while it loads. On the first draw
that finds it complete, the backend sets it again with its real size, which can
evict older images. The `mem:` canvases are not in this cache: each one copies a
registry buffer, and `SetMemImageBudget` bounds those already.

## Rejected Approaches

- **A process-wide setter, like `SetMemImageBudget`.** The registry is global,
  so its budget is global. The texture caches belong to windows; a global cap
  over many windows has no clear meaning (per window, or the sum?).
- **No API, defaults only.** An app that shows large photos cannot raise the
  cap, and a small kiosk app cannot lower it.
- **A strict byte LRU.** It thrashes as described above whenever the visible
  images need more than the cap.
- **Trimming to the cap at the start of each frame, with no protection.** It
  deletes textures that the frame is about to draw, so it thrashes the same way.
  `BeginFrame` trims too, but skips the entries used in the previous frame.
- **Recording a web image's size in its `onload` handler.** A browser decodes an
  `Image` when it is first drawn, not when it loads, so an image that leaves the
  screen while it loads holds no decoded bitmap. The size is recorded at that
  first draw, when the bitmap exists.
- **A budget that can change at run time.** No use case yet. A setter can be
  added later without breaking the field.
