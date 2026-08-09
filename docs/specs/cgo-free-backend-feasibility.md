# CGo-free desktop backend — feasibility assessment

Assessment of [issue #137](https://github.com/go-gui-org/go-gui/issues/137).
Date: 2026-07-31. Status: **Phase 1 implemented**; Phase 2 (macOS) not started.

## Phase 1 outcome (2026-07-31)

`github.com/go-gl/gl` is gone. It was replaced by
`gui/backend/internal/glbind`, a purego binding for the 55 GL entry points and
45 constants the backend uses, resolved through the proc-address functions that
already existed (`eglProc` on Linux; a new `glProc` on Windows reproducing
go-gl's wglGetProcAddress-then-opengl32 fallback). Call sites changed by import
swap only.

Measured result:

| Target | `CGO_ENABLED=0 go build` |
|---|---|
| windows/amd64, windows/arm64 | `./...` — **entire module** |
| linux/amd64, linux/arm64, linux/386 | `./gui/backend/...` |

Two corrections to the assessment below:

- The constant count is **45**, not 47. The original `grep` used
  `\b(gogl|gl)\.` where `go?gl` was intended, so bare `gl.`-qualified
  references in `backend.go` were miscounted.
- **`github.com/go-gl/gl` was not the only CGo dependency on Linux.**
  `gui/audio` pulls in `github.com/ebitengine/oto/v3`, whose Linux driver is
  cgo (`#cgo pkg-config: alsa` in `driver_unix.go`). The original verification
  missed it because `go build` stopped at the go-gl load error. Windows is
  unaffected — oto's Windows driver is pure syscall. Full `CGO_ENABLED=0
  GOOS=linux go build ./...` therefore still fails, on `gui/audio` alone.
  That is a separate dependency needing its own decision (gate `gui/audio`
  behind a build tag, or replace oto on Linux) and was left out of Phase 1.

32-bit was **not** dropped: purego supports 386/arm via `syscall_32bit.go`,
and `linux/386` builds.

## Summary

Issue #137 proposes a CGo-free desktop backend built on goffi + wgpu-native +
GLFW, scoped as "roughly a full backend rewrite — 4–8k lines Go + ~12 WGSL
shaders."

Both of the issue's premises verify. Its scoping does not.

Linux and Windows are already CGo-free apart from **one** dependency
(`github.com/go-gl/gl`), whose surface area in this repo is 55 functions and 47
constants — and whose proc-address loader is already written in pure Go. The
wgpu path is a superset of that work, does not address macOS (the only real CGo
backend), and trades a build-time C toolchain for a runtime shared-library
distribution burden.

**Recommendation: close the wgpu/goffi path. Do the 55-function swap instead.**

## 1. The stated blockers are genuinely resolved

The issue is correct on both counts.

**Pure-Go text.** The go-glyph root package has zero `import "C"`.
`bitmap_puregoft.go`, `context_puregoft.go`, `layout_ft.go`, and
`grapheme_ft.go` all build under `//go:build linux || darwin || windows` with no
cgo tag gate — the pure-Go path is the *default*, not an opt-in.
`go-glyph/go.mod` requires only go-text/typesetting, x/image, uniseg, and x/text.
CGo in go-glyph is confined to `backend/gpu/`, `backend/android/`,
`backend/ios/`, and examples.

**Zero-CGo FFI.** goffi is real: 8 desktop targets, crosscall2 C-thread
callbacks, full struct ABI, 88–114 ns/call.

## 2. Linux and Windows are already almost CGo-free

`gui/backend/gl/` contains no `import "C"` in any file. It already uses:

| Concern | Mechanism | File |
|---|---|---|
| X11 windowing / events | `github.com/jezek/xgb` (pure Go) | `platform_x11.go` |
| EGL context creation    | `github.com/ebitengine/purego` | `egl_linux.go:65` |
| Win32 / WGL             | `golang.org/x/sys` syscall     | `platform_win32.go`, `wgl_windows.go` |
| GL dispatch             | **`github.com/go-gl/gl` (cgo)** | 8 files |

The last row is the only CGo dependency. Confirmed by build — both targets fail
with exactly one error, and it is that import:

```
package .../gui/backend
    imports .../gui/backend/gl
    imports github.com/go-gl/gl/v3.3-core/gl: build constraints exclude all Go files
```

Surface area to port: **55 distinct GL functions, 47 constants**. Crucially, the
proc-address plumbing needed to bind them is already written and already pure
Go — `platform_x11.go:287` calls `gl.InitWithProcAddrFunc(eglProc)`, where
`eglProc` comes from a purego-loaded `eglGetProcAddress`; `wgl_windows.go` has
the Windows equivalent.

Phase 1 is therefore a swap of one dispatch layer: roughly 600 lines of purego
bindings. No shader rewrite, no wgpu, no GLFW, no windowing changes.

## 3. macOS is the entire remaining problem

- `gui/backend/metal/` — 7,202 lines total; 9 Go files with `import "C"`; 3,311
  lines of `.m`/`.h`. Across the whole tree (metal, ios, android, filedialog):
  5,924 lines of ObjC/C.
- macOS-only CGo outside the backend: `nativemenu/menu_darwin.go`,
  `filedialog/dialog_darwin.go`, `printdialog/print_darwin.go`,
  `spellcheck/spellcheck_darwin.go`, `sysbeep/sysbeep_darwin.go`,
  `gui/locale_detect_darwin.go`.
- The Linux/Windows counterparts of those services are *already* pure Go:
  `dialog_linux.go` (396 lines, portal/zenity), `dialog_windows.go` (369 lines,
  syscall), `spellcheck_linux.go` (244 lines), godbus for SNI and AT-SPI.

Nothing about the wgpu proposal touches any of this.

## 4. Why wgpu + goffi is the wrong instrument

**It does not supply native services.** `gui/native_platform.go` defines
`NativePlatform` as 10 sub-interfaces and ~30 methods: dialogs, notifier,
printer, bookmarks, a11y, IME, spellcheck, menubar, system tray, sound, plus
`OpenURI`, `TitlebarDark`, `SetWindowVibrancy`. wgpu-native and GLFW supply none
of it. The issue's claim that "existing pure-Go fallbacks (Android/iOS prove the
pattern)" does not hold: `android/native_platform.go` no-ops only menubar, tray,
and beep, and the android/ios backends are themselves cgo (`import "C"` in
`backend.go`, `draw.go`, `text.go`, `textures.go`).

**CGo-free is not dependency-free.** Today's Linux/Windows build bundles no
native library — it dlopens the system's `libEGL`/`libGL`/`opengl32`. A
wgpu-native + GLFW backend replaces a *build-time* C toolchain requirement with a
*runtime* obligation to ship and load ~10–40 MB of platform-specific shared
objects. That is a net regression for distribution, not an improvement.

**goffi is pre-1.0.** v0.6.0 is in progress; API stability is planned for v1.0.0.
It also has a documented duplicate-`_cgo_init` symbol conflict with purego, which
go-gui already depends on (`go.mod`: `github.com/ebitengine/purego v0.10.1`).
Adopting goffi would force `-tags nofakecgo` on every downstream consumer.

**The cost is strictly larger.** 12 WGSL shaders, device/surface init, and
windowing glue, on top of the same GL-dispatch problem — and macOS still
unsolved at the end of it.

## 5. Recommended path

### Phase 1 — Linux + Windows CGo-free (high value, low risk)

Replace `github.com/go-gl/gl/v3.3-core/gl` with an internal purego-backed GL
binding.

- New package `gui/backend/internal/glbind/`: 55 function pointers plus the 47
  referenced constants, bound through the existing proc-address callbacks.
- Reuse the `purego.Dlopen` / `purego.RegisterLibFunc` pattern already in
  `gui/backend/gl/egl_linux.go`, and the existing `eglProc` / WGL proc lookups.
  Do not introduce a second loader.
- Touch points: the 8 files importing `go-gl/gl` (`backend.go`, `draw.go`,
  `pipeline.go`, `buffers.go`, `textures.go`, `text.go`, `platform_x11.go`,
  `platform_win32.go`). Import swap only — call sites keep their signatures.
- Watch items:
  - `gogl.Strs` returns a free func; reimplement as a null-terminated
    byte-slice helper.
  - `VertexAttribPointerWithOffset` — offset marshaling.
  - `GetShaderiv` / `GetShaderInfoLog` / `GetProgramiv` / `GetProgramInfoLog`
    out-params need explicit pointer marshaling under purego.
- Outcome: `CGO_ENABLED=0` cross-compilation to Linux and Windows from any host.

### Phase 2 — macOS: time-boxed spike, no commitment

Evaluate `github.com/ebitengine/purego/objc` for hosting an `NSView` /
`CAMetalLayer` subclass with delegate callbacks via `objc_msgSend` and runtime
class registration. Gate the decision on two questions:

1. Does purego work under `CGO_ENABLED=0` on darwin/arm64 for the
   *class-registration* path, not just plain function calls?
2. Can the ObjC be ported service-by-service, or does the NSApplication/NSView
   delegate graph force an all-or-nothing rewrite?

If either answer is bad, macOS stays cgo. That is an acceptable outcome: macOS is
the one platform where a C toolchain (Xcode CLT) is universally present anyway.

### Out of scope

wgpu-native, GLFW/SDL2, WGSL shaders, goffi.

## Verification

Every number above is reproducible:

```fish
# 1. go-glyph root package has no cgo
cd ~/Documents/github/go-glyph; grep -rl 'import "C"' --include='*.go' .

# 2. gl backend has no cgo; one dependency blocks CGO_ENABLED=0
cd ~/Documents/github/go-gui
grep -rl 'import "C"' --include='*.go' gui/backend/gl/   # empty
env CGO_ENABLED=0 GOOS=linux go build -tags gl ./gui/...
env CGO_ENABLED=0 GOOS=windows go build ./gui/...

# 3. GL surface area to port (55 functions, 47 constants)
grep -rhoE '\b(gogl|gl)\.[A-Z][A-Za-z0-9_]*\(' gui/backend/gl/ --include='*.go' | sort -u | wc -l
grep -rhoE '\b(gogl|gl)\.[A-Z][A-Z0-9_]+\b'    gui/backend/gl/ --include='*.go' | sort -u | wc -l

# 4. macOS native surface
grep -rl 'import "C"' --include='*.go' gui/backend/metal/ | wc -l   # 9
find gui -name '*.m' -o -name '*.h' | xargs wc -l | tail -1         # 5924
```

Acceptance test if Phase 1 is approved: both `CGO_ENABLED=0` builds above
succeed, `go test ./gui/...` passes, and `go run ./examples/get_started/` on
Linux renders unchanged.
