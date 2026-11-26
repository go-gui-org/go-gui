# CGo-free desktop backend — feasibility assessment

Status: **done** — desktop is cgo-free. Phase 1 (Linux + Windows, 2026-07-31)
and the `gui/audio` follow-up closed the Linux CGo dependency. Phase 2 (macOS)
was closed 2026-08-12 by decision (not pursued — macOS stays cgo), **reopened
and decided 2026-11-20 (#1304), and the port is done (#1323): the macOS backend
has no cgo.** iOS shares the purego Metal renderer since #1342 (see § iOS). What
follows is the historical assessment, kept for the rationale and the rejected
approaches. For the current state see `docs/architecture.md` and
`docs/deployment.md`.

## Phase 1 outcome (2026-07-31)

`github.com/go-gl/gl` is gone. It was replaced by `gui/backend/internal/glbind`,
a purego binding for the 55 GL entry points and 45 constants the backend uses.
The binding resolves through the proc-address functions that already existed:
`eglProc` on Linux, and a new `glProc` on Windows that reproduces go-gl's
wglGetProcAddress-then-opengl32 fallback. Call sites changed by import swap
only.

Measured result:

| Target                              | `CGO_ENABLED=0 go build`    |
| ----------------------------------- | --------------------------- |
| windows/amd64, windows/arm64        | `./...` — **entire module** |
| linux/amd64, linux/arm64, linux/386 | `./gui/backend/...`         |

Two corrections to the assessment below:

- The constant count is **45**, not 47. The original `grep` used `\b(gogl|gl)\.`
  where `go?gl` was intended, so bare `gl.`-qualified references in `backend.go`
  were miscounted.
- **`github.com/go-gl/gl` was not the only CGo dependency on Linux.**
  `gui/audio` pulls in `github.com/ebitengine/oto/v3`, whose Linux driver is cgo
  (`#cgo pkg-config: alsa` in `driver_unix.go`). The original verification
  missed it because `go build` stopped at the go-gl load error. Windows is
  unaffected — oto's Windows driver is pure syscall. Full
  `CGO_ENABLED=0 GOOS=linux go build ./...` therefore still fails, on
  `gui/audio` alone. That is a separate dependency that needs its own decision
  (gate `gui/audio` behind a build tag, or replace oto on Linux). It was left
  out of Phase 1.

32-bit was **not** dropped: purego supports 386/arm via `syscall_32bit.go`, and
`linux/386` builds.

## Audio outcome (2026-08-04)

The `gui/audio` dependency left out of Phase 1 (see the second correction above)
is resolved. [Issue #141](https://github.com/go-gui-org/go-gui/issues/141) put
the output driver behind a 3-function seam — `outputInit` / `outputPlay` /
`outputClose`, split across `output_oto.go` and `output_pulse.go`.

The default Linux sink is now a pure-Go PulseAudio client
(`github.com/jfreymuth/pulse`), so `CGO_ENABLED=0 go build ./...` is green on
Linux for the **entire module**, not just `./gui/backend/...`. oto/ALSA remains
available on Linux, opt-in via `-tags otoaudio`. Windows and macOS still use
oto. beep decode/mix was already pure Go and did not change.

## Why not WebGPU (2026-06 → 2026-07-31)

Recorded here because it is the question this assessment supersedes.

A WebGPU backend was explored on branch `webgpu-backend` (since deleted): 12
WGSL shader pipelines, device init, and the render loop all worked. It was
rejected at the time because WebGPU has no native text rendering. Font
measurement and glyph rasterization required Canvas2D, and a hybrid backend
defeats the purpose. GPU acceleration also does not address this project's
actual bottleneck, which is heap allocation rather than throughput.

By 2026-07 both original blockers were gone: go-glyph gained a pure-Go text
pipeline (`bitmap_puregoft.go` — go-text/typesetting harfbuzz shaping plus
`golang.org/x/image/vector` rasterization, no CGo), and
[goffi](https://github.com/go-webgpu/goffi) supplies zero-CGo FFI for calling
wgpu-native. A CGo-free WebGPU desktop backend became technically viable.

It remains the wrong instrument, for the reasons in §4: wgpu is a superset that
ships 10–40 MB of runtime shared libraries, supplies none of `NativePlatform`
(10 sub-interfaces), and leaves macOS — the only real CGo backend, 5.9k lines of
ObjC — untouched. Phase 1 got CGo-free Linux and Windows for ~600 lines instead.

## Summary

Issue #137 proposes a CGo-free desktop backend built on goffi + wgpu-native +
GLFW, scoped as "roughly a full backend rewrite — 4–8k lines Go + ~12 WGSL
shaders."

Both of the issue's premises hold. Its scoping does not.

Linux and Windows are already CGo-free apart from **one** dependency
(`github.com/go-gl/gl`), whose surface area in this repo is 55 functions and 45
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
cgo tag gate — the pure-Go path is the _default_, not an opt-in.
`go-glyph/go.mod` requires only go-text/typesetting, x/image, uniseg, and
x/text. CGo in go-glyph is confined to `backend/gpu/`, `backend/android/`,
`backend/ios/`, and examples.

**Zero-CGo FFI.** goffi is real: 8 desktop targets, crosscall2 C-thread
callbacks, full struct ABI, 88–114 ns/call.

## 2. Linux and Windows are already almost CGo-free

`gui/backend/gl/` contains no `import "C"` in any file. It already uses:

| Concern                | Mechanism                        | File                                  |
| ---------------------- | -------------------------------- | ------------------------------------- |
| X11 windowing / events | `github.com/jezek/xgb` (pure Go) | `platform_x11.go`                     |
| EGL context creation   | `github.com/ebitengine/purego`   | `egl_linux.go:65`                     |
| Win32 / WGL            | `golang.org/x/sys` syscall       | `platform_win32.go`, `wgl_windows.go` |
| GL dispatch            | **`github.com/go-gl/gl` (cgo)**  | 8 files                               |

The last row is the only CGo dependency. Confirmed by build — both targets fail
with exactly one error, and it is that import:

```
package .../gui/backend
    imports .../gui/backend/gl
    imports github.com/go-gl/gl/v3.3-core/gl: build constraints exclude all Go files
```

Surface area to port: **55 distinct GL functions, 45 constants**. Crucially, the
proc-address plumbing needed to bind them is already written and already pure Go
— `platform_x11.go:287` calls `gl.InitWithProcAddrFunc(eglProc)`, where
`eglProc` comes from a purego-loaded `eglGetProcAddress`. `wgl_windows.go` has
the Windows equivalent.

Phase 1 is therefore a swap of one dispatch layer: roughly 600 lines of purego
bindings. No shader rewrite, no wgpu, no GLFW, no windowing changes.

## 3. macOS is the entire remaining problem

- `gui/backend/metal/` — 7,202 lines total, 9 Go files with `import "C"`, 3,311
  lines of `.m`/`.h`. Across the whole tree (metal, ios, android, filedialog):
  5,924 lines of ObjC/C.
- macOS-only CGo outside the backend: `nativemenu/menu_darwin.go`,
  `filedialog/dialog_darwin.go`, `printdialog/print_darwin.go`,
  `spellcheck/spellcheck_darwin.go`, `sysbeep/sysbeep_darwin.go`,
  `gui/locale_detect_darwin.go`.
- The Linux/Windows counterparts of those services are _already_ pure Go:
  `dialog_linux.go` (396 lines, portal/zenity), `dialog_windows.go` (369 lines,
  syscall), `spellcheck_linux.go` (244 lines), godbus for SNI and AT-SPI.

Nothing about the wgpu proposal touches any of this.

## 4. Why wgpu + goffi is the wrong instrument

**It does not supply native services.** `gui/native_platform.go` defines
`NativePlatform` as 10 sub-interfaces and ~30 methods: dialogs, notifier,
printer, bookmarks, a11y, IME, spellcheck, menubar, system tray, sound, plus
`OpenURI`, `TitlebarDark`, `SetWindowVibrancy`. wgpu-native and GLFW supply none
of it. The issue's claim that "existing pure-Go fallbacks (Android/iOS prove the
pattern)" does not hold. `android/native_platform.go` no-ops only menubar, tray,
and beep. The android/ios backends are themselves cgo (`import "C"` in
`backend.go`, `draw.go`, `text.go`, `textures.go`).

**CGo-free is not dependency-free.** Today's Linux/Windows build bundles no
native library — it dlopens the system's `libEGL`/`libGL`/`opengl32`. A
wgpu-native + GLFW backend replaces a _build-time_ C toolchain requirement with
a _runtime_ obligation to ship and load ~10–40 MB of platform-specific shared
objects. That is a net regression for distribution, not an improvement.

**goffi is pre-1.0.** v0.6.0 is in progress. API stability is planned for
v1.0.0. It also has a documented duplicate-`_cgo_init` symbol conflict with
purego, which go-gui already depends on (`go.mod`:
`github.com/ebitengine/purego v0.10.1`). Adopting goffi forces `-tags nofakecgo`
on every downstream consumer.

**The cost is strictly larger.** 12 WGSL shaders, device/surface init, and
windowing glue, on top of the same GL-dispatch problem — and macOS still
unsolved at the end of it.

## 5. Recommended path

### Phase 1 — Linux + Windows CGo-free (high value, low risk)

Replace `github.com/go-gl/gl/v3.3-core/gl` with an internal purego-backed GL
binding.

- New package `gui/backend/internal/glbind/`: 55 function pointers plus the 45
  referenced constants, bound through the existing proc-address callbacks.
- Reuse the `purego.Dlopen` / `purego.RegisterLibFunc` pattern already in
  `gui/backend/gl/egl_linux.go`, and the existing `eglProc` / WGL proc lookups.
  Do not introduce a second loader.
- Touch points: the 8 files importing `go-gl/gl` (`backend.go`, `draw.go`,
  `pipeline.go`, `buffers.go`, `textures.go`, `text.go`, `platform_x11.go`,
  `platform_win32.go`). Import swap only — call sites keep their signatures.
- Watch items:
  - `gogl.Strs` returns a free func. Reimplement it as a null-terminated
    byte-slice helper.
  - `VertexAttribPointerWithOffset` — offset marshaling.
  - `GetShaderiv` / `GetShaderInfoLog` / `GetProgramiv` / `GetProgramInfoLog`
    out-params need explicit pointer marshaling under purego.
- Outcome: `CGO_ENABLED=0` cross-compilation to Linux and Windows from any host.

### Phase 2 — macOS: decided 2026-08-12, not pursued

~~Evaluate `github.com/ebitengine/purego/objc` for hosting an `NSView` /
`CAMetalLayer` subclass with delegate callbacks via `objc_msgSend` and runtime
class registration. Gate the decision on two questions:~~

1. ~~Does purego work under `CGO_ENABLED=0` on darwin/arm64 for the
   _class-registration_ path, not just plain function calls?~~
2. ~~Can the ObjC be ported service-by-service, or does the NSApplication/NSView
   delegate graph force an all-or-nothing rewrite?~~

~~If either answer is bad, macOS stays cgo. That is an acceptable outcome: macOS
is the one platform where a C toolchain (Xcode CLT) is universally present
anyway.~~

**Decision: macOS keeps cgo. Do not reopen without a concrete trigger.**

The value of a cgo-free build is build-time portability: `CGO_ENABLED=0`
cross-compiles from any host, no clang/SDK install for consumers, smaller CI.
Every one of those is weakest on macOS:

- Cross-compiling a macOS GUI binary from another host is hollow: a GUI app must
  be run and signed/notarized on a Mac anyway.
- Xcode CLT is universally present on macOS. There is no toolchain-friction
  story to fix for consumers.
- The spike's hard part was never the call ABI but ObjC the language:
  subclassing `NSView`/`CAMetalLayer` needs runtime class registration (the
  shaky zone of `purego/objc`) and hand-managed retain/release and autorelease
  pools where clang does it for free today. High bug surface, no user-visible
  payoff.
- The macOS-only CGo outside the backend (`nativemenu/menu_darwin.go`,
  `filedialog/dialog_darwin.go`, `printdialog/print_darwin.go`,
  `spellcheck/spellcheck_darwin.go`, `sysbeep/sysbeep_darwin.go`,
  `gui/locale_detect_darwin.go`) each needs a port that is strictly worse than
  its Linux/syscall counterpart.

The Linux/Windows phase earned its keep because those hosts lack a guaranteed
toolchain. The macOS phase inverts the argument. The issue
(go-gui-org/go-gui#137) was closed when Phase 1 shipped and stays closed.

Reopening conditions (any one, evidenced, not speculative):

- Apple stops shipping a C toolchain with macOS / Xcode CLT becomes optional.
- A consumer reports a real build blocker caused by the ObjC cgo (not a
  hypothetical), and `purego/objc` class registration is proven working at
  `CGO_ENABLED=0` on darwin/arm64.
- A one-shot proof that the `NSView`/`CAMetalLayer` subclass graph ports
  service-by-service rather than all-or-nothing, with the retain/release
  management plan written down.

**Re-check (2026-10-05): the cost went down; the benefit did not change. The
decision stands.**

`github.com/egoist/mygo` (checked at `2cf4327`) has a macOS backend with no cgo.
It uses `purego` and `purego/objc` for AppKit, Metal, CoreText, dialogs, menus,
the tray and accessibility. What it shows:

- `objc.RegisterClass` works with `CGO_ENABLED=0` on darwin/arm64. mygo
  registers 11 classes this way (`internal/darwin/objc.go`), among them an
  `NSWindow` subclass, an `NSView` that is a text input client, and an
  accessibility element. A `CGO_ENABLED=0` build of `examples/native` opened a
  window and ran.
- It needs no `CAMetalLayer` subclass. It adds a plain `CAMetalLayer` to the
  view's layer (`internal/gpu/metal/metal.go`).
- Retain, release and autorelease pools are a few small helpers. They are not a
  rewrite of the whole object graph.

Most of go-gui's macOS classes (`GUIWindow`, `MetalContentView`,
`GUIAccessibilityElement`, the delegates and the action handlers) have a working
match in mygo. So a port service by service looks possible. The class
registration half of the second reopening condition is met. The third condition
is not met yet: it needs that proof in go-gui's own classes, not in another
project's.

The reasons for the decision were about benefit, and they still hold. Xcode CLT
is still on every Mac. No consumer has a build blocker. go-term, the main macOS
consumer, gains nothing. iOS and Android stay cgo in any case, so the module
never becomes fully cgo-free. One reason is weaker than it was written:
`rcodesign` is said to sign and notarize from Linux, so "a GUI app must be
signed on a Mac" may not be true. This was not checked.

If a benefit trigger shows up (for example, building macOS release binaries on
Linux CI), start with a spike that is not merged: a window, a Metal clear and
one key event in go-gui, and a count of the heap allocations per frame from the
`objc_msgSend` calls made through purego. Use mygo as the reference. Do not
start with a second backend behind a `!cgo` tag: two macOS backends would have
to be kept in step.

**Spike (2026-10-25, #1160): `purego.SyscallN` with a package-level argument
array sends with 0 allocations, and the spike's frame loop runs at 0 allocations
per frame. The trigger is user adoption.** Some users reject go-gui because the
macOS backend uses cgo. Their test is `CGO_ENABLED=0 go build` on darwin. The
spike code is not merged. It is on branch `spike/1160-purego-macos`, in
`spike/puregomac/`.

What the spike did, with `CGO_ENABLED=0` on darwin/arm64 (`otool -L` shows no
AppKit or Metal link; the frameworks load at run time through `purego.Dlopen`):

1. It opened an `NSWindow`. The content view is an `NSView` subclass made with
   `objc.RegisterClass`. The view hosts a plain `CAMetalLayer`.
2. It cleared the layer with Metal each frame: `nextDrawable`, one render pass,
   `setViewport:`, `presentDrawable:`, `commit`.
3. A key-down went through `sendEvent:` into a Go `keyDown:` method. That method
   made a `gui.Event` with the `macKeyCodeTable` and `mapMetalModifiers` code
   from `gui/backend/metal/events.go`. Shift+S (key code 1, not 0, so a lost
   argument cannot pass) gave `KeyS` and `ModShift`.
4. It counted heap allocations (`runtime.ReadMemStats`, 600 frames after a
   warm-up) for three ways to send a message.

| Send style                                  | allocs/send | ns/send | allocs/frame |
| ------------------------------------------- | ----------- | ------- | ------------ |
| `objc.ID.Send` (variadic `...any`)          | 5–8         | 168     | 63           |
| `purego.RegisterFunc` (typed func)          | 4–6         | 149     | 49           |
| `purego.SyscallN`, arguments listed in call | 1           | 51      | —            |
| `purego.SyscallN`, package-level `argBuf`   | 0           | 46      | **0**        |
| cgo baseline (one `objc_msgSend` through C) | 0           | 22.5    | —            |

The darwin/amd64 build, run under Rosetta, gave the same allocation counts. A
key-down costs 1 allocation per event. That is not per frame.

What the numbers mean:

- **Typed `RegisterFunc` pointers do not pass the gate.** The issue expected
  them on the hot path. purego builds their calls with reflection, so every call
  allocates. The metal backend's gate (`TestDrawPathsDoNotAllocate`) wants 0 per
  draw.
- **`SyscallN` with a package-level argument array makes no allocation.** The
  spike measured single sends and its own frame loop (clear, viewport, present).
  It did not port or run the draw paths that `TestDrawPathsDoNotAllocate`
  covers; a port must pass that test itself. `SyscallN` is `go:uintptrescapes`,
  so the slice of a listed-argument call goes to the heap. A slice of a
  package-level array does not. One array is safe because AppKit and Metal run
  on the main thread only. `SyscallN` copies the arguments before the call, so a
  Go callback that sends again does not damage them. The cost: a pointer
  argument reaches `SyscallN` as a `uintptr`, so nothing keeps what it points to
  alive or in place. It must point to a global, or to a field of an object that
  is surely on the heap, such as a backend scratch field (the metal backend
  already keeps its per-draw arrays there). A local, even one made with `new`,
  stays on the stack when only its `uintptr` leaves the function, and the stack
  can move when it grows.
- **`SyscallN` takes integers and pointers only.** On arm64 a struct over 16
  bytes that is not a homogeneous float aggregate (`MTLViewport`,
  `MTLScissorRect`) goes as a pointer. On amd64 it goes on the stack: four zero
  words fill the last argument registers, then the struct words follow.
  `TestLargeStructSend` proves both ways with a `CGAffineTransform` round trip.
  A float aggregate (`MTLClearColor`, `NSRect`) needs `RegisterFunc`. Send it
  only when the value changes, not per frame.
- **Each draw costs more time.** Today one cgo call per draw sends about two
  ObjC messages inside C (`metalDrawQuad`: `setVertexBytes:`,
  `drawIndexedPrimitives:`), about 25 ns. Through `SyscallN` it is about 92 ns.
  At 20,000 glyph quads a frame that adds about 1.3 ms. Batching quads into one
  vertex buffer would remove most of this.

Not tested by the spike: `NSTextInputClient` (IME) callbacks that return
`NSRange` or `NSRect` to AppKit, accessibility, menus and dialogs.

The port was decided on 2026-11-20 (see § Investigation below). Port service by
service, with `SyscallN` and the argument array on every per-frame path. Start
with the plain-C services (keyring, locale). The renderer (`metal_darwin.m`)
goes last. § Port order below gives the full sequence.

### Investigation (2026-11, #1304)

**Decision (2026-11-20): port the macOS backend to purego. The target is
darwin/arm64 only. darwin/amd64 (Intel) is dropped.** The maintainer accepted
the Intel drop. This section is the port decision that the #1160 spike left
open.

#1304 asked for a proof of concept, an inventory of APIs, a comparison with the
cgo backend and a recommendation. The spike code is on branch
`spike/1160-purego-macos`, in `spike/puregomac/`. It is not for merge. All
numbers are from one Apple Silicon Mac, macOS 27, Go 1.27.4, `CGO_ENABLED=0`
unless the row says cgo.

#### Prior art

Two MIT-licensed projects ship a macOS backend with no cgo. Both use the same
purego version as go-gui (v0.11.x).

- **gowaku** (`github.com/bankai-tensei/gowaku`, checked at `e4d27a2`). It is a
  full port: AppKit, Metal, IME, accessibility, menus, tray, notifications, file
  dialogs, drag and drop and clipboard. It is darwin/arm64 only. On amd64,
  `gowaku.Run` returns `ErrUnsupported`. Its ADR 0082, 0083 and 0085 record the
  design. Its `docs/BENCH.md` gives 145 ms startup and 0% idle CPU, against 151
  ms for Gio, which uses cgo for Metal.
- **mygo** (`github.com/egoist/mygo`, checked at `2dd01b4a`). It supports amd64
  too. On amd64 it sends a struct return of more than 16 bytes through
  `objc_msgSend_stret` (`internal/darwin/objc.go:120-140`).

The spike copies three patterns from gowaku:

- A `send(obj, sel, args ...uintptr)` helper that copies its arguments into one
  argument array (`gpu/metal/runtime_darwin.go:63`). The variadic slice stays on
  the stack, so a send makes no allocation.
- An autorelease pool around each public entry point, with
  `runtime.LockOSThread` for the life of the pool. ADR 0082 reports intermittent
  crashes without the lock. The spike hit the same fault: a sub-benchmark ran on
  a new goroutine, the pool pop ran on a different OS thread, and the process
  crashed with SIGSEGV.
- `NSTextInputClient` and `NSAccessibilityElement` methods as
  `objc.RegisterClass` Go methods that return `NSRange` and `NSRect`
  (`platform/cocoa/classes_darwin.go:88-113`).

gowaku also records two lessons for the port. When a window is hidden,
`CAMetalLayer` stops waiting for vsync, so a window that animates runs at about
120 fps. gowaku stops drawing on `windowDidChangeOcclusionState:`. Also,
`[NSApp run]` must run once and stop in `applicationDidFinishLaunching:`, or
AppKit does not finish the launch.

#### What the spike proves

| Area                         | Result                                                                                                                                                                                        | Test                                                |
| ---------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------- |
| Window, layer, present, keys | Pass (#1160)                                                                                                                                                                                  | `-selftest`                                         |
| go-gui shader library        | `msl.Source` compiles with the backend's pinned MSL 3.0. `vs_solid`/`fs_solid` build with `mainVertexDesc` and the backend's blend state. A drawn quad reads back red, the clear reads black. | `TestPipelineDrawsQuad`                             |
| Texture upload               | `replaceRegion:` and `getBytes:` round-trip a 2x2 texture. `MTLRegion` (48 bytes) goes by pointer.                                                                                            | `TestTextureUploadReadback`                         |
| Allocations                  | A full frame of 1000 quads makes 0 allocations, per-quad and batched.                                                                                                                         | `TestDrawAllocs`                                    |
| IME struct returns           | `markedRange`, `selectedRange` (NSRange) and `firstRectForCharacterRange:actualRange:` (NSRect, out-parameter) return correct values to a purego caller and to a compiled C caller.           | `TestIMEStructReturns`, `TestIMEStructReturnsFromC` |
| Blocks and target/action     | `dispatch_async_f` on the main queue, an `NSTimer` with target/action, an `NSTimer` with an `objc.NewBlock` block, and an `NSMenu` item with a Go action all fire.                            | `-selftest`                                         |

The C-caller IME test matters. The purego test has purego on both sides of the
call, so the same ABI mistake on both sides can pass. AppKit is a compiled C
caller, and the C test uses that caller.

Not tested: IME with a real input source (a person must type), VoiceOver, the
file, print and alert dialogs, and the stencil and MSAA passes. gowaku ships
each of these with purego, so the spike did not repeat them.

#### Comparison with the cgo backend

Frame encode time on the CPU, for N solid quads, without GPU time
(`BenchmarkQuads`, `BenchmarkQuadsCgo`, 3 runs each):

| Shape                                       | 1000 quads | 20,000 quads | allocs/frame |
| ------------------------------------------- | ---------- | ------------ | ------------ |
| cgo, one call per quad (today's backend)    | 67 µs      | 1230 µs      | 0            |
| purego, two sends per quad                  | 155 µs     | 2960 µs      | 0            |
| purego, one vertex buffer and one draw call | 15 µs      | 164 µs       | 0            |

A purego send costs about 2.4 times a send inside C. With the backend's current
draw shape, a frame of 20,000 quads costs 1.7 ms more. That is more than the 1.3
ms that #1160 estimated. If the port writes quads into one shared vertex buffer,
the frame costs 7.5 times less than today's cgo backend. **The port must
therefore batch quads on the draw path.** Batching also helps the cgo backend,
but cgo does not need it to keep today's speed. A real frame changes pipeline
and scissor state between draws, so a batch holds one run of draws with the same
state. The gain in a real frame is less than in this benchmark.

Startup, memory and size (`spike/puregomac/measure.sh`, the same spike built
both ways):

| Measure                             | cgo off   | cgo on    |
| ----------------------------------- | --------- | --------- |
| Launch to first finished frame, p50 | 105 ms    | 97 ms     |
| Peak RSS after 600 frames           | 107.5 MB  | 107.0 MB  |
| Binary size, `-s -w`                | 10.95 MiB | 10.73 MiB |
| GUI frameworks linked at load time  | 0         | 0         |

The spike opens the frameworks with `purego.Dlopen` in both builds, so these
rows measure the cgo runtime only. The startup difference is less than the
spread between runs (91 ms to 686 ms). For reference, `examples/get_started` on
the real cgo backend is 16.27 MiB and links 4 GUI frameworks. The two binaries
contain different code, so the size difference does not measure cgo.

#### Inventory

"Proven" means the spike or a reference project runs the API with no cgo.

| go-gui service (file)                                             | purego status                                                                             | Evidence                                                       |
| ----------------------------------------------------------------- | ----------------------------------------------------------------------------------------- | -------------------------------------------------------------- |
| Renderer (`metal_darwin.m`, 1642 lines)                           | Proven. Batch quads (see above).                                                          | Spike pipeline tests, gowaku `gpu/metal/`                      |
| Window, events, live resize (`metal_window_darwin.m`, 2740 lines) | Proven                                                                                    | Spike `-selftest`, gowaku `window_darwin.go`                   |
| IME (`MetalContentView <NSTextInputClient>`)                      | Proven for the ABI. Not tested with a real input source.                                  | Spike IME tests, gowaku `classes_darwin.go`, mygo `surface.go` |
| Accessibility (`a11y_darwin.m`, 395 lines)                        | Proven                                                                                    | gowaku `accessibility_darwin.go`, mygo `access.go`             |
| Menus and tray (`menu_darwin.m`, 505 lines)                       | Proven                                                                                    | Spike menu action, gowaku `tray_darwin.go`, mygo `menu.go`     |
| File dialogs and alerts (`dialog_darwin.m`, 273 lines)            | Proven                                                                                    | gowaku `filedialog_darwin.go`, mygo `dialog.go`                |
| Print (`print_darwin.m`, 81 lines)                                | Proven                                                                                    | mygo `window.go` (`NSPrintOperation`)                          |
| Spell check (`spellcheck_darwin.m`, 172 lines)                    | Not tested. Plain `NSSpellChecker` sends with no blocks or subclass, so no new risk.      | None                                                           |
| Appearance observer (`appearance_darwin.m`)                       | Proven                                                                                    | gowaku `platform_darwin.go`, mygo `system.go`                  |
| Keep awake (`keep_awake_darwin.m`, `NSProcessInfo` activity)      | Not tested. Two plain sends.                                                              | None                                                           |
| System beep (`sysbeep_*.m`, `NSSound`)                            | Proven for `NSBeep`. `NSSound` is plain sends.                                            | mygo `cf.go`                                                   |
| Keyring (`keyring_darwin.go`, Security C API)                     | Proven                                                                                    | mygo `secrets.go`                                              |
| Locale (`gui/locale_detect_darwin.go`, CoreFoundation)            | Proven                                                                                    | gowaku `platform_darwin.go`, mygo `locale.go`                  |
| Blocks: `dispatch_async`, `dispatch_once`, `NSTimer …block:` (15) | Proven. Use `dispatch_async_f` with one callback, `sync.Once`, and target/action timers.  | Spike `-selftest`                                              |
| iOS backend (`gui/backend/ios/`)                                  | Out of scope here. #1342 moved its renderer to purego (§ iOS). The UIKit shell stays cgo. | —                                                              |

No service needs a small cgo boundary on darwin/arm64.

#### Costs of the port

- **Build and distribution.** After the last port step, `CGO_ENABLED=0 go build`
  will work on darwin/arm64. Today an app that calls `backend.Run` with cgo off
  gets the link-time error from `gui/backend/run_darwin_nocgo.go`. After the
  port, a Linux host can cross-compile a macOS binary, and Xcode Command Line
  Tools are not necessary to build. Signing and notarization still need a Mac,
  or `rcodesign`, which this investigation did not test.
- **Intel.** darwin/amd64 loses the backend. It gets the link-time error that
  `gui/backend/run_darwin_nocgo.go` gives today. The port PR must carry a
  `**BREAKING:**` CHANGELOG entry. mygo's `_stret` path can add Intel back
  later.
- **Safety.** Today, clang does retain/release and checks selector types at
  compile time. After the port, a selector typo is a run-time "unrecognized
  selector" crash, and a lost release is a leak. The port must cache every
  selector at startup and test that each target class responds to each selector.
- **Pointer rule.** A pointer argument reaches `SyscallN` as a `uintptr`. It
  must point to a global, or to a field of an object that stays reachable for
  the whole native call. It must never point to a stack local. The spike puts
  every scratch value (`MTLRegion`, NSError out-parameter, vertex bytes) in a
  heap struct.
- **Size of the work.** About 6100 lines of Objective-C become Go with `unsafe`.
  gowaku needs about 7200 lines of Go for a similar scope.

#### Rejected

- **A second macOS backend behind a `!cgo` build tag.** Two backends must stay
  the same in every feature. This stays rejected (see the 2026-10-05 re-check).
- **Keep cgo.** The spike and the reference projects remove every technical
  reason. The users who need `CGO_ENABLED=0` on darwin are the trigger, and the
  port has no startup or memory cost.
- **Keep Intel through mygo's `_stret` path now.** It doubles the struct ABI
  work and the test matrix for a platform that Apple no longer sells. It can
  come back later.

#### Port order

Port service by service inside the one backend. The backend keeps cgo until the
last service moves. Each step is one PR.

1. Plain C and plain-send services: keyring, locale, keep awake, system beep,
   appearance. **Done in #1307.** The shared helpers are in
   `gui/internal/darwinffi`. iOS keeps cgo copies of keyring and locale
   (`keyring_ios.go`, `locale_detect_ios.go`), because purego needs cgo on iOS.
   #1342 removed them: iOS now uses the purego files too. Keep awake and
   appearance have no cgo of their own, but keep the `cgo` build term until step
   6, because the rest of `gui/backend/metal` needs cgo.
2. Dialogs, print, spell check. **Done in #1312.** No test shows a panel or an
   alert: AppKit aborts when one is made off the main thread, and `go test` runs
   each test on its own goroutine. The tests cover the parts that need no
   window: content types, file paths, print info and spell check. The port also
   corrects two faults in the cgo code. NSString drops leading byte order marks,
   so every spell check range after them was 3 bytes early per mark. The print
   copy count was set after the operation took its own copy of the print info,
   so it was lost.
3. Menus and tray. **Done in #1314.** One target class, `GoGuiMenuTarget`,
   serves the menubar and every tray. A tray item carries its tray ID in its
   tag, and a menubar item has tag 0. The cgo code had one class for each kind.
   The `dispatch_async` block that removes the items macOS adds to the Edit menu
   is now `performSelectorOnMainThread:withObject:waitUntilDone:`, which the
   same main run loop runs. The tests build menus but do not install them. The
   port also corrects one fault in the cgo code: a title that was not valid
   UTF-8 gave a nil NSString, and NSMenuItem stopped the app.
4. Accessibility. **Done in #1317.** Go keeps the whole tree. Each node has one
   `GoGuiA11yElement`, a subclass of NSAccessibilityElement with no state of its
   own. Its methods find their node through a Go map and answer from the Go
   copy. A sync computes the screen offset of the view once, not with two sends
   for each node, and keeps the CFString of a text that did not change. The
   tests build a tree with no window and ask the elements what VoiceOver asks.
   The port also corrects four faults in the cgo code. An announcement with text
   that was not valid UTF-8 put nil in a dictionary, and the exception stopped
   the app. The root element had node index 0, so it reported the focus of node
   0 and sent its actions as actions of node 0. The last posted focus did not
   change while there was no focus, so a focus that came back to the same node
   was not posted. A node that named itself, or a later node, as its parent made
   a cycle that VoiceOver walks for ever.
5. Window, events and IME. **Done in #1321.** `metal_window_darwin.m` and
   `metal_window.h` are gone. The four classes keep their names (`GUIWindow`,
   `MetalContentView`, `GUIApplication`, `GoGuiAppDelegate`), and their state is
   in Go. The `dispatch_async` blocks are `dispatch_async_f` with one purego
   callback each, the `NSTimer` blocks are target/action timers, and the
   run-loop observer of the frame pump is a purego callback. The event path must
   not allocate, and `objc.Send` allocates 4 to 9 times for each call. A mouse
   event reads about ten values, some of them doubles and an NSPoint, which
   `purego.SyscallN` cannot return. `darwinffi.FCaller` is a small arm64
   trampoline that gives D0-D3 back, through `runtime.cgocall` as purego does.
   `TestStoreEventDoesNotAllocate` and `TestWakeUpDoesNotAllocate` hold the
   event path and the per-frame wake at 0 allocations. darwin/amd64 keeps
   building until step 6, with `objc.Send` in place of the trampoline. The C
   test hooks are Go functions in a test file now. The port found one purego
   limit: on darwin/arm64, `objc.Send` puts each stack argument (an integer
   argument after the eighth) in an 8-byte slot, but Apple packs stack arguments
   by size. `keyEventWithType:…isARepeat:keyCode:` then gets key code 0. No
   production send has such arguments.
6. Renderer (`metal_darwin.m`), with quad batching. **Done in #1323.**
   `metal_darwin.m` and `metal_darwin.h` are gone, and the renderer is
   `renderer_darwin.go`. The backend has no cgo, and the darwin files build for
   arm64 only. darwin/amd64 stops at link with
   `macOS_requires_arm64__Intel_Macs_are_not_supported`
   (`gui/backend/run_darwin_amd64.go`), and `TestDarwinArm64BuildsWithoutCgo`
   and `TestDarwinAmd64LinkFails` hold both. `make cross-compile` and CI now
   build darwin/arm64 with `CGO_ENABLED=0`, and the release `.dmg` is arm64
   only. Two changes keep the draw path at today's cost or less. First, quads
   are batched: `drawQuad` copies the vertices into a shared vertex buffer (4
   MiB chunks, three frames in a ring) and sends nothing. A run of quads goes in
   one `drawIndexedPrimitives` call when the encoder state must change. Second,
   the setters cache the bound state. The draw code sets the pipeline and the
   MVP before each draw, and a setter whose value is bound sends nothing and
   keeps the batch open. A frame of 20,000 solid quads encodes in about 400 µs
   with 0 allocations (`BenchmarkBatchedQuads`, CPU only). The figure includes
   `gpu.BuildQuad` for each quad, so it is not the same shape as the 164 µs of
   the spike. The cgo backend took 1230 µs for the same count.
   `TestBatchFlushesOnStateChange` reads back a frame to prove that a state
   change draws the open batch first, and `TestBatchSpansLimits` crosses the
   batch and buffer limits. The port found one fault that the cgo build hid:
   with no cgo, nothing links AppKit at load time, so a class lookup before
   `loadObjc` gets nil. `GoGuiA11yElement` then had no superclass and stopped
   the app on its first message. Each lookup that can come first now calls
   `loadObjc`.

### iOS (2026-11, #1342)

Issue #1342. Status: **done for the renderer** (option 1 below). Device
performance is not measured.

The iOS backend kept its own cgo renderer (`metal_darwin.m`, 1213 lines) and its
own copy of the draw code after #1323. Each renderer fix had to be made twice.

#### Options

| Option                                  | What changes                                                                 | Cost                                                                                      |
| --------------------------------------- | ---------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| 1. Share the renderer only (**chosen**) | iOS draws with the macOS renderer. The UIKit shell (`ios_app.m`) stays cgo.  | About 2,500 lines removed. The iOS build keeps `CGO_ENABLED=1`.                           |
| 2. Full port, UIKit shell included      | Option 1, and the UIKit view, view controller and events move to purego too. | About 700 more lines of Objective-C become Go. The iOS build still needs cgo (see below). |
| 3. Do nothing                           | Two renderers stay.                                                          | Every renderer change is made and tested twice. The iOS copy allocates on every frame.    |

Option 2 is rejected. purego needs cgo on iOS (`is_ios.go` in purego), so a full
port does not give `CGO_ENABLED=0` on iOS. The shell has no duplicate on macOS,
so moving it removes no duplicate code. The renderer is where the two backends
had the same code twice.

Option 3 is rejected. The iOS copy had fallen behind: it waited for the GPU on
every frame and made 121 allocations a frame (see Results).

#### What changed

- The renderer and the draw code moved from `gui/backend/metal` to
  `gui/backend/internal/mtldraw` (`darwin && arm64`, which includes iOS). The
  macOS and iOS backends both make a `mtldraw.Renderer` on their own
  CAMetalLayer. The package knows nothing about windows or views.
- `gui/internal/darwinffi` builds for iOS too. `fcall_arm64.s` must carry the
  same build tag as `fcall_arm64.go`: without it, the first send stops with
  `cgocall nil`.
- `gui/backend/ios/metal_darwin.m`, `metal_darwin.h` and the iOS copies of
  `draw.go`, `text.go`, `textures.go`, `shader_channels.go`, `pixel_surface.go`,
  `buffers.go` and `rotation.go` are gone. The public API of the iOS package
  (`Start`, `Render`, `Resize`, `CleanUp`, `TouchInput`) is the same.
- The cgo copies of keyring and locale (`keyring_ios.go`,
  `locale_detect_ios.go`) are gone. One iOS rule stays: a new Keychain item is
  `kSecAttrAccessibleAfterFirstUnlockThisDeviceOnly`.

#### Results (iOS simulator)

`examples/ios_demo`, same scene, 600-frame windows, main-thread time of one
render:

| Renderer               | Time per render | Allocations per render |
| ---------------------- | --------------: | ---------------------: |
| cgo (`metal_darwin.m`) |         1100 µs |                    121 |
| purego (`mtldraw`)     |          350 µs |                      0 |

Most of the time difference is not the call cost. The cgo renderer waited in
`waitUntilScheduled` on every frame. The shared renderer presents with
`presentDrawable:` and does not wait, except during a live resize on macOS.
`TestDrawPathsDoNotAllocate` and `TestBatchedDrawDoesNotAllocate` pass on the
simulator.

#### Tests on the simulator

A test binary for iOS runs on a booted simulator with no Xcode project:

```fish
set SDK (xcrun --sdk iphonesimulator --show-sdk-path)
env CGO_ENABLED=1 GOOS=ios GOARCH=arm64 \
  CC=(xcrun --sdk iphonesimulator -f clang) \
  CGO_CFLAGS="-isysroot $SDK -arch arm64 -mios-simulator-version-min=17.0" \
  CGO_LDFLAGS="-isysroot $SDK -arch arm64 -mios-simulator-version-min=17.0" \
  go test -c -o /tmp/mtldraw.test ./gui/backend/internal/mtldraw
env SIMCTL_CHILD_MTL_DEBUG_LAYER=1 xcrun simctl spawn booted /tmp/mtldraw.test -test.v
```

`gui/internal/darwinffi/spike_ios_test.go` holds the first proof: it clears a
render target on iOS through purego and reads every pixel back. A Keychain test
cannot run this way. A bare binary gets `errSecMissingEntitlement` (-34018), and
the simulator does not start a bare binary that claims keychain entitlements.
`keyring_ios_test.go` skips then. The Keychain rule was checked inside the
signed demo app.

#### Open

- **Device performance.** No device was available. The simulator runs on the
  Mac's GPU, so the table above shows the CPU side only. Measure
  `examples/ios_demo` on a device before a claim about device frame time.

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

# 3. GL surface area to port (55 functions, 45 constants)
# Historical: run before Phase 1, when gui/backend/gl/ still imported go-gl.
# The regex is go?gl\. — the original used \b(gogl|gl)\. and over-counted
# constants as 47; see the correction under "Phase 1 outcome" above.
grep -rhoE '\bgo?gl\.[A-Z][A-Za-z0-9_]*\(' gui/backend/gl/ --include='*.go' | sort -u | wc -l
grep -rhoE '\bgo?gl\.[A-Z][A-Z0-9_]+\b'    gui/backend/gl/ --include='*.go' | sort -u | wc -l

# 4. macOS native surface: none since #1323 (9 cgo files and 5924
#    lines of .m/.h before the port). The .m/.h left are iOS and Android.
grep -rl 'import "C"' --include='*.go' gui/backend/metal/ | wc -l   # 0
env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build ./...           # green
```

Acceptance test if Phase 1 is approved: both `CGO_ENABLED=0` builds above
succeed. `go test ./gui/...` passes, and `go run ./examples/get_started/` on
Linux renders unchanged.
