# SDL3 shim evaluation

Source: [cataggar/SDL#1](https://github.com/cataggar/SDL/pull/1) +
[cataggar/go-gui#1](https://github.com/cataggar/go-gui/pull/1)

## Summary

Two coordinated PRs that let go-gui (and go-glyph) target SDL3 with zero
source changes, via a cgo drop-in replacement for the `go-sdl2` API surface
go-gui uses. The shim lives in a fork of SDL's zig-build port and is consumed
with a single `go.mod` replace directive.

## Architecture

```
go-gui backend code (unchanged)
  → github.com/veandco/go-sdl2/sdl  (import path)
    → replace → ../SDL/go-sdl2-sdl3  (go.mod)
      → cgo → SDL3 C API (zig-built, dynamic or static)
```

Three layers in the shim:

1. **`go-sdl2-sdl3/sdl`** — Hand-written cgo shim. Reshapes SDL3's split
   window-event model and float coordinates back into the SDL2-style API.
   Event conversion folds SDL3's distinct window event types into
   SDL2-style `WindowEvent` subtypes. A `cbits.c`/`cbits.h` C layer avoids
   cgo struct-field mangling by exposing field accessor functions.
   ~382 SDL2 API calls across `gui/backend/sdl2/`, `gui/backend/gl/`, and
   `sdlkey` compile unchanged.

2. **`go-sdl2-sdl3/mix`** — SDL_mixer-compatible audio engine on SDL3's
   native audio API (SDL3 doesn't bundle SDL_mixer). Per-channel
   `SDL_AudioStream`s auto-mixed by SDL + dedicated music stream.
   Looping/fades via get-callback, volume via per-stream gain. WAV decoded
   natively; MP3/OGG via vendored dr_mp3 / stb_vorbis (public domain).

3. **`gosdl3/`** — Experimental. Zig comptime bindgen emits full cgo SDL3
   Go bindings. Separate from the hand-written shim; useful as a generated
   reference surface.

## Benefits

- **Zero go-gui code changes.** The API shim absorbs all SDL2→SDL3
  differences.
- **Eliminates MSYS2/MinGW on Windows.** `zig build` + `zig cc` replaces the
  entire SDL2 toolchain. Relevant given recent CI churn pinning MSYS2 to a
  stable release (v0.27.1).
- **Static linking.** `-tags sdl3static` produces a fully static binary with
  no SDL3.dll/dylib. The static link file enumerates Win32 system libs
  (kernel32, user32, gdi32, etc.) that can't be carried by a static archive.
- **Audio without SDL_mixer.** Eliminates a separate build dependency.
- **Covers go-glyph too.** go-glyph's SDL backend (3 files) works with the
  same replace directive.
- **SDL3 is the current stable.** SDL2 is maintenance-mode. Moving now is
  right timing.

## Concerns

### zig as a new toolchain dependency

Every contributor and CI pipeline needs zig installed. zig is increasingly
popular for C/C++ cross-compilation and the `build.zig` already exists in
the forked repo, but it's a non-trivial addition. On the upside, it
*replaces* MSYS2, not adds to it — net toolchain count stays flat or drops.

### Shim location

The shim currently lives in `cataggar/SDL` (a fork of a fork of SDL's zig
port). Long-term, the shim should be its own standalone repo (e.g.,
`go-gui-org/go-sdl3-shim`) with its own lifecycle, tests, and versioning.
Tying it to a personal SDL fork creates an unclear maintenance dependency.

### Partial API coverage

The shim covers only the go-sdl2 API surface go-gui actually uses. If
go-gui later needs an API not in the shim, someone has to add it. Acceptable
tradeoff — the full go-sdl2 surface is large and mostly unused — but must be
acknowledged.

### Metal backend CGo dependency

**Decision: Required, scope is small (~20 lines in `backend.go`).**

The Metal backend (`gui/backend/metal/backend.go`) directly includes
`<SDL.h>` and `<SDL_metal.h>` via CGo and calls SDL2 C functions
(`SDL_Metal_CreateView`, `SDL_Metal_GetLayer`, `SDL_Metal_GetDrawableSize`,
`SDL_Metal_DestroyView`). These are raw CGo calls, not Go-level `sdl.*`
calls — the shim only covers the Go→cgo path through `go-sdl2/sdl`.

Without updating these, macOS compilation fails under the shim. The .m file
(`metal_darwin.m`) is unaffected — it receives a `void*` CAMetalLayer
pointer regardless of how it's obtained.

Changes needed in the shim's `cbits.h`/`cbits.c`:
- Expose SDL3's `SDL_GetPointerProperty(SDL_PROP_WINDOW_METAL_CAMetalLayer)`
  to replace `SDL_Metal_CreateView` + `SDL_Metal_GetLayer`.
- Expose `SDL_GetWindowSizeInPixels` to replace
  `SDL_Metal_GetDrawableSize`.

These are SDL3-native APIs with no SDL2 equivalent — they belong in the
shim's C layer, not in go-gui. The callers in `backend.go` then switch from
`C.SDL_Metal_*` to the shim's C accessors.

### Downstream replace directive leakage

**Decision: Acceptable for transition. File follow-up issue for module-based
solution before v1.0.**

The `replace` is fundamental to the "zero source changes" design. go-gui
has `import "github.com/veandco/go-sdl2/sdl"` in ~100+ files. The replace
redirects that import path to the shim without editing any source.
There is no way to eliminate the replace without changing every import
statement in go-gui (which defeats the purpose).

Go modules documents `replace` directives as explicitly non-transitive.
Any downstream project importing go-gui must manually copy the replace
directive into their own `go.mod` to build against SDL3.

Short-term this is tolerable — go-gui's primary consumers are Mike's own
repos (go-glyph, go-charts, go-edit, go-kite), which can be updated in
lockstep. Third-party consumers omitting the replace continue to build
against standard go-sdl2 (SDL2) as before — the shim is opt-in.

A standalone shim repo (`go-gui-org/go-sdl3-shim`) improves the replace
target from a local filesystem path to a versioned module URL, but does
not remove the replace itself. The only path to truly eliminating it is
changing go-gui's import paths (e.g., to native SDL3 bindings via the
`gosdl3/` experiment), which is a separate, larger effort.

Ship v0.28 with replace. Accept that it's the design, not a temporary
wart.

### Audio format coverage

**Decision: Non-issue. Comment fix only.**

The `sound.go` doc comment lists FLAC, AIFF, and VOC as supported formats.
The actual `Init()` default format mask is `mix.INIT_OGG | mix.INIT_MP3`
(audio.go:65). FLAC requires `mix.INIT_FLAC`; AIFF/VOC require
`mix.INIT_MOD`. These are never enabled by default — the comment documents
SDL_mixer's theoretical capabilities, not go-gui's actual behavior.

The shim's WAV/MP3/OGG coverage matches the default config exactly.
No code change needed. Update the `LoadSound` doc comment to read
"Supports WAV, MP3, OGG" to reflect reality.

### Test divergence

`TestBackendRenderSmoke` reads backbuffer after `Present()`. Defined in SDL2,
undefined in SDL3 with hardware-accelerated flip-model renderers (Direct3D11).
Passes on software renderers (CI), may read empty pixels on accelerated
Windows. Production paths (blur/color-matrix filter readback to texture
targets) are unaffected.

Additional Metal-specific gap: the Metal backend sets
`CAMetalLayer.presentsWithTransaction = YES` (metal_darwin.m:763) to
eliminate content shift during live resize. SDL3's Metal integration may
handle presentsWithTransaction differently or expose its own controls via
the Properties API. A macOS Metal smoke test variant should verify live
resize behavior under SDL3 before shipping.

### go-glyph coordination

Both repos need the same replace directive. Works with sibling-directory
layout (`../SDL/go-sdl2-sdl3`) but doesn't compose well across independent
projects. A standalone shim repo with its own `go.mod` module path solves
this.

## Build chain comparison

| | Current | Proposed |
|---|---|---|
| C toolchain | MSYS2/MinGW (Windows), system cc (macOS/Linux) | zig cc (all platforms) |
| SDL | SDL2 via pkg-config | SDL3 via zig build |
| Audio | SDL_mixer | vendored decoders + SDL3 audio API |
| Platform libs | implicit via pkg-config | explicit for static builds |

## Recommendation

**Adopt the approach, repackage first.**

The architectural idea (API shim → zero go-gui changes) is excellent and
the engineering is solid. Before merging into go-gui:

1. Extract `go-sdl2-sdl3/` into a standalone repo with its own module path,
   build instructions, and test suite.
2. Add SDL3 Metal property accessors (`SDL_GetPointerProperty`,
   `SDL_GetWindowSizeInPixels`) to the shim's `cbits.h`/`cbits.c`.
   Update `backend.go` to use them instead of `SDL_Metal_*` CGo calls.
3. Document `zig` as a build dependency in README and CI.
4. Apply the same replace directive to go-glyph.
5. Acknowledge `TestBackendRenderSmoke` caveat in backend test docs.
   Add macOS Metal-specific smoke test for live-resize behavior under SDL3
   (verifies `presentsWithTransaction` equivalent).
6. Fix `LoadSound` doc comment: "Supports WAV, MP3, OGG" to match the
   default format mask.
7. Accept `replace` as a permanent part of the design. Document in README
   that downstream consumers must copy the directive to opt into SDL3.
