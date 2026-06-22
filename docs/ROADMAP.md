# Roadmap

## SDL2 elimination

**Goal:** Remove `github.com/veandco/go-sdl2` from all backends. SDL2 adds
~30 MB of C dependencies, complicates cross-compilation, and forces a CGo
toolchain on every platform. Each backend should use native windowing and
GPU APIs directly.

### Progress

| Platform  | Backend | Status  | Notes |
|-----------|---------|---------|-------|
| macOS     | Metal   | **Done** (2026-06) | Native `NSWindow` + `CAMetalLayer`. Zero SDL2. |
| Linux     | GL      | Planned | Replace SDL2 window/context with native GLX or EGL. |
| Windows   | GL      | Planned | Replace SDL2 window/context with native WGL. |
| All       | SDL2    | Planned | Archive after GL backend migrates to native windowing. |

### macOS (done)

The `gui/backend/metal` package uses AppKit + Metal directly. No SDL2
headers, no `go-sdl2` import. go-glyph v1.11.0 eliminated SDL2 from its
Metal GPU path (`metalInit` takes `CAMetalLayer*` directly).

### Remaining

- **GL backend** (`gui/backend/gl`): Currently uses SDL2 for window
  creation, OpenGL context management, and event polling. Replace with
  native equivalents (GLX/EGL on Linux, WGL on Windows). The GPU
  rendering path through go-glyph's `backend/gpu` also needs SDL2
  removed from `gl_linux.go` / `gl_windows.go` (go-glyph Phase F).

- **SDL2 backend** (`gui/backend/sdl2`): Archive once the GL backend
  has native windowing. The SDL2 renderer offers no advantage over GL
  with native windows.

### Dependencies

- go-glyph [Phase F](https://github.com/go-gui-org/go-glyph/blob/main/ROADMAP.md#phase-f--sdl2-elimination):
  native GL context creation on Linux/Windows.
