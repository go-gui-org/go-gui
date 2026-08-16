# Color picker components

Status: implemented. Supersedes the monolithic `ColorPicker` internals.

## Problem

`gui.ColorPicker` was one welded widget. An app that wanted only a hue wheel, or
a saturation plane beside its own controls, had to copy the file. Three things
blocked splitting it up:

1. **No conic gradient, and no gradient can express an HSL plane.** The old
   saturation/value square worked only because HSV factorizes into `white → hue`
   across and `transparent → black` down. HSL's saturation × lightness plane is
   a bilinear blend, and a hue wheel is conic. Neither is reachable with the
   linear and radial gradients the renderer has.
2. **No pixel-buffer path.** Images were `Src string` only — a path, an http(s)
   URL, or a `data:` URL. There was no way to hand the renderer bytes.
3. **Hue is not recoverable from RGBA.** At `S=0`, `L=0` or `L=1` the hue is
   gone. The old widget hid an `H` in window state to paper over this. Four
   independent components each hiding their own copy would drift apart.

`gradientShaderStopLimit = 5` compounded (1): the old hue strip declared seven
stops and was silently resampled to five on every GPU backend.

## Design

### One value, owned by the app

`HSLA` (`gui/color_hsl.go`) is a plain value the caller holds. Every component
takes `Value HSLA` and reports `OnChange(HSLA, EventCtx)`; none keeps state. Two
controls stay in sync because they read the same variable, not because they talk
to each other.

This is what makes (3) a non-issue: hue lives in the value, so it survives a
drag through black. `ColorPicker` keeps a single `nsColorPicker` entry only
because its own public API is still RGBA in and RGBA out — a lossy-conversion
guard for the back-compat surface, not a second color model.

### The in-memory image registry

`gui/image_mem.go` is the primitive the components stand on, and it is useful
well beyond them:

```go
src := gui.UseImage(key, w, h, pix) // "mem:"+key; NRGBA8, straight alpha
ok := gui.HasImage(key)             // skip generating a buffer that exists
gui.DropImage(key)
gui.SetMemImageBudget(bytes)        // default 32 MiB
```

`Src` strings starting `mem:` are resolved by every backend through
`gui.LookupImage` before any path handling (`drawImage` in `metal`, `gl`, `ios`,
`android`; an offscreen canvas in `web`; a PNG-encoded embed in the PDF export).
The registry is a byte-budgeted LRU, process-global because backends resolve a
`Src` string with no `*Window` in hand.

**Content keying is the contract.** The key names the inputs the buffer was
generated from, so a buffer that must change is registered under a _new_ key and
the stale variant ages out on its own. There is deliberately no invalidation
call: backends cache uploaded textures under the same string, so an app that
mutated a buffer in place would have no way to tell them.

Keys quantize — hue to whole degrees, S/L/A to whole percent. A key tracking a
float exactly would miss on every frame of a drag and rebuild the buffer each
time, which is the failure the cache exists to prevent. A steady frame costs one
map lookup and zero allocations: the key is built into a stack scratch buffer,
and `memImageSrc` returns the string stored at registration.

Buffers are generated at `imgScale = 2` and displayed at logical size — sharp on
HiDPI, cheaply downscaled on 1×, and the scale never enters a key.

**Security.** A `mem:` buffer bypasses `AllowedImageRoots`, `MaxImageBytes` and
`MaxImagePixels`. Those bound _untrusted_ input — a path or URL an attacker
might steer. An in-process pixel buffer is trusted by construction; the
registry's byte budget is its bound.

### What each control depends on

| Control              | Buffer keyed on            | Rebuilt when          |
| -------------------- | -------------------------- | --------------------- |
| `ColorChannelSlider` | channel, size, other H/S/L | another channel moves |
| `ColorPlane`         | hue, size                  | hue moves             |
| `ColorWheel`         | lightness, size            | lightness moves       |
| `ColorSwatch`        | size                       | never (one checker)   |

A channel is never in its own strip's key — the strip sweeps it — so dragging
hue repaints the saturation and lightness tracks but not the hue track. Alpha is
in no strip's key at all: H/S/L tracks are drawn opaque, and the alpha track is
the one sweeping it.

### Shared internals

- `colorDragTrack` (`gui/view_color_drag.go`) — the whole pointer-capture
  sequence in one place. `OnClick` arrives shape-relative while `MouseLock`
  delivers window-absolute, and the dragged shape must be re-found by ID each
  move because the tree is rebuilt every frame.
- `colorMarker` (`gui/view_color_marker.go`) — the ring. It floats above the
  imagery: a channel slider floats its track to centre it in a control the thumb
  makes taller, and a marker under that track is invisible.
- Arrow keys move every control (`colorKeyDelta`); Shift takes the 10× step. A
  key a control does not handle is not consumed, so Tab still escapes.

## API

```go
gui.ColorPlane(gui.ColorPlaneCfg{ID, Value, Size, OnChange})
gui.ColorWheel(gui.ColorWheelCfg{ID, Value, Size, OnChange})
gui.ColorChannelSlider(gui.ColorChannelSliderCfg{ID, Channel, Value, Vertical, Width, Height, OnChange})
gui.ColorSwatch(gui.ColorSwatchCfg{ID, Color, Width, Height})
gui.ColorFields(gui.ColorFieldsCfg{ID, Value, ShowHSL, ShowSwatch, OnChange})
```

All are fixed-size (`FixedFixed`): their size is tied to the buffer generated
for them, so they must not stretch to fill a parent.

A channel slider set `Vertical` sweeps bottom (minimum) to top (maximum) and
takes its length from `Height`. Orientation is part of the strip's content key,
so the two orientations are separate buffers rather than one rotated at draw
time — the strip must be generated along the axis it is displayed on or the
end-cap insets land on the wrong sides.

`ColorFields` set `ShowSwatch` puts a `ColorSwatch` to the right of the hex
field, under the fields' own ID scope. Both are readouts of the same color — one
exact, one legible — so they belong on one line; a caller that wants the swatch
anywhere else leaves the flag off and places a plain `ColorSwatch`. It is drawn
twice as wide as `SwatchSize`, which stays its height: a square beside a text
field reads as a button, where a wide bar reads as a sample of the value next to
it.

`gui.ColorPicker` is now a composition of `ColorPlane`, two vertical
`ColorChannelSlider`s standing to the right of the plane, and `ColorFields`
carrying the swatch. Its `Cfg` is unchanged and every existing caller compiles
untouched.

The plane's size is derived rather than taken from the theme's `sVSize`, which
becomes an upper bound. The picker is as wide as its widest row — the four RGBA
fields — so the plane is that width less what the two sliders and the gaps
beside them occupy, and the two rows come out flush. Widening the gap therefore
narrows the plane instead of widening the picker.

`ShowHSV` still works and is deprecated in favour of `ShowHSL`, which names what
the row contains.

## What callers see change

The square is the HSL saturation × lightness plane rather than the HSV square;
the hue and alpha sliders stand vertically to the right of it instead of
stacking beneath it, which makes the picker squarer; the preview swatch sits
beside the hex field rather than left of the whole numeric column; the alpha
slider gains a real transparency checkerboard; the hue strip is exact instead of
resampled to five stops. No API break.

## Rejected

- **Keeping the HSV square as a mode.** It is free — two gradients, no buffer —
  but it would have forced `ColorPicker` to carry a permanent RGBA↔HSV↔HSLA
  adapter and two plane implementations. That is the second-source-of-truth
  pattern this repo rejects elsewhere (`ThemeMaker` vs `default*Style`).
- **Per-vertex colored triangle meshes** instead of buffers. Would have reached
  the wheel without a texture upload, but touches every backend's shader and is
  far less reusable than an image source.
- **PNG-encoded `data:` URLs** to avoid backend work. Re-encoding a 400×400 PNG
  on every hue change is a visible hitch on drag.

## Follow-up

`gradientShaderStopLimit = 5` (`gui/render_gradient.go:11`) still silently
resamples any gradient with more than five stops. This work no longer depends on
it, but it degrades every such gradient in the repo.
