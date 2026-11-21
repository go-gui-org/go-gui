# Windows software renderer

Issue #1310 (follow-up of #1265). Status: **planned** (phase 1 of 5 in review as
#1319; phases 2 to 5 not started).

The maintainer accepted this design on the issue on 2026-11-20 (see Decisions).
Code facts are as of #1319. "Measured" with no machine named means throwaway
programs on one 16-thread AMD desktop (Windows 11 24H2, Go 1.27.4, a 1920×1080
screen, median of 10–20 runs); those figures are indicative. "The lab VM" is the
one machine without a GPU driver that was tested (see Problem).

## Problem

A Go-Gui app does not start on a Windows machine whose OpenGL cannot run the
backend. Linux got a CPU path in #1265 (`docs/specs/live-software-renderer.md`),
which left Windows out of its first round.

Without a vendor driver, `opengl32.dll` runs Microsoft's generic implementation:
`GL_RENDERER` "GDI Generic", version 1.1
([OpenGL on Windows](https://learn.microsoft.com/en-us/windows/win32/opengl/opengl)).
It has no `wglCreateContextAttribsARB`, `glAttachShader` or `glGenVertexArrays`
(measured by forcing a generic pixel format). The backend fails on it in one of
two ways, and neither message has a remedy for Windows: the
`GOGUI_RENDERER=soft` hint is Linux-only (`gpuContextError.Error`).

1. **The context is refused.** `createContext` (`gui/backend/gl/wgl_windows.go`)
   returns an error, `New` wraps it with `newGPUContextError`, and `Run` prints
   one line with advice and exits 1 (`exitNoGPUOrPanic`).
2. **A generic context is handed out.** `createContextWithFormat` keeps the
   legacy context when it cannot get 3.3 core. Binding the GL function table
   then fails at `glAttachShader`. Before #1319 that was a plain error, and
   `Run` panicked with a stack trace. Since #1319 it matches `ErrNoGPUContext`
   too, and the message names `GL_VERSION` and `GL_RENDERER`.

**Measured on a real machine** (for #1319). The lab VM is QEMU with the standard
VGA display, which Windows drives with the Microsoft Basic Display Adapter;
Windows 11 Enterprise LTSC 2024, build 26100.1742. The console session and an
RDP session gave the same result:

- The first WGL context a process asks for is refused:
  `wglCreateContext: The pixel format is invalid.` A one-window app exits with
  the advice, before and after #1319. It does not panic.
- A second attempt in the same process gets GDI Generic 1.1.0 and fails at the
  binding.

The issue said the common failure is the panic. On the one machine measured it
is the refused context. The panic needed a context from the generic
implementation: an attempt after the first on that machine, or a first attempt
where a context is given at once, as with a forced generic pixel format on a
machine with a GPU driver. So phase 1 (#1319) fixes the second way and changes
nothing for the first. Phase 3 falls back on both; on the lab VM the refused
context is the trigger, and a process that fell back makes no further GL
attempt. No test may assume which way the system OpenGL fails.

**Which machines.** Only the rows marked measured or stated are observations.
"No vendor GL expected" comes from the adapter's documentation and does not say
which of the two ways the machine fails.

| Environment                                                  | OpenGL                                                             | Basis                                                                                                                                                                                                    |
| ------------------------------------------------------------ | ------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| QEMU VM, standard VGA, console session and RDP session       | first context refused; a second attempt gets GDI Generic 1.1.0     | measured on the lab VM (#1319), one Windows version                                                                                                                                                      |
| A generic pixel format forced on a machine with a GPU driver | GDI Generic 1.1.0 at the first attempt                             | measured                                                                                                                                                                                                 |
| GitHub `windows-latest` runner (Windows Server 2025)         | GDI Generic 1.1; whether the first context is refused is not known | stated in `.github/workflows/ci.yml`, which adds Mesa to start the showcase                                                                                                                              |
| Hyper-V VM without GPU-P or DDA                              | no vendor GL expected                                              | the "Microsoft Hyper-V Video" adapter is display-only ([GPU-PV](https://learn.microsoft.com/en-us/windows-hardware/drivers/display/gpu-paravirtualization))                                              |
| QEMU with QXL or virtio-gpu                                  | no vendor GL expected                                              | the Windows virtio-gpu driver is display-only ([QEMU](https://www.qemu.org/docs/master/system/devices/virtio/virtio-gpu.html))                                                                           |
| RDP session on an RD Session Host, policy unset              | no vendor GL expected; a vendor driver in "all other cases"        | the hardware-adapter [policy](https://learn.microsoft.com/en-us/windows/client-management/mdm/policy-csp-admx-terminalserver#ts_dx_use_full_hwgpu) text                                                  |
| Windows Sandbox with vGPU disabled                           | no vendor GL expected                                              | the sandbox then renders with WARP ([.wsb docs](https://learn.microsoft.com/en-us/windows/security/application-security/application-isolation/windows-sandbox/windows-sandbox-configure-using-wsb-file)) |
| Mesa's `opengl32.dll` beside the exe                         | the driver Mesa selects: llvmpipe when forced, as CI does          | [mesa-dist-win](https://github.com/pal1000/mesa-dist-win#readme)                                                                                                                                         |

**The Mesa hook.** `initOpenGL32` loads `opengl32.dll` from the executable's
directory and falls back to System32, in a package variable initializer, so in
every process. An application ships Mesa there on purpose, typically to keep
custom shader fills, which the CPU renderer does not draw. CI does it too.

## Candidates

| Design                                           | Dependencies                                                                                                                                    | CPU and memory                                                                         | Parity                                                               | How it fails                                                                                |
| ------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- | -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| **A. `soft.Renderer` presented with GDI**        | none: `gdi32` and `user32` are already bound. +0.23 MB on a 25.1 MB exe (measured)                                                              | on Linux llvmpipe costs 2.5–3× soft's CPU and 2× its memory; Windows figures are below | every render kind; a custom shader draws its children and no fill    | nothing can be missing; a frame past the soft size limit is skipped and logged              |
| B. Bundle or auto-load Mesa                      | `opengl32.dll` and `libgallium_wgl.dll` with LLVM, per architecture; MSYS2's package is 120 MB installed                                        | llvmpipe: the 2.5–3× above                                                             | full, custom shaders included                                        | a local DLL that fails to load falls through to System32, and the app fails as with no Mesa |
| C. ANGLE (GLES 3.0 on D3D11, WARP without a GPU) | `libEGL.dll`, `libGLESv2.dll`, usually `d3dcompiler_47.dll`; an EGL platform layer for Win32 and ES variants of the backend's desktop pipelines | WARP; not measured                                                                     | custom shaders have a GLES 3 wrapper already (`BuildGLSLESFragment`) | a DLL is missing, or the D3D11 feature level is too low                                     |

All three keep `CGO_ENABLED=0`. B and C add a third-party binary to rebuild,
sign and patch.

## Decisions

**Picked: A, GDI present of `soft.Renderer` from a DIB section.** It is the
Linux design on a third window system, as #1265 planned. It adds no dependency
and no API, and cannot fail for a missing file, which is the property a no-GPU
fallback needs. It reuses a renderer that is pixel-tested against
`RenderToImage` on X11 and on Wayland.

**Within A: a DIB section in a memory DC, sent with `BitBlt`,** not #1265's
`SetDIBitsToDevice` from a Go buffer. Both copy the frame twice and cost the
same on screen (1.5 ms at 1920×1080 with a `GdiFlush` after each call,
measured). Only the DIB section gives `UpdateLayeredWindow` the source DC it
takes, which phase 4 needs. It is the `wl_shm` shape: the frame store is the
memory the window system reads.

**Decided on the issue, 2026-11-20:**

1. Branches, not a `presenter` interface.
2. Phase 1 classifies binding and pipeline failures as `ErrNoGPUContext`. Phase
   3 falls back on a context or binding failure only: a fallback on a pipeline
   failure would land a driver-specific shader bug on soft with only a log line.
3. A transparent window with a system frame is opaque on soft, with a
   `DebugWindowTransparency` report and no error.
4. A layered soft window raises alpha 0 to 1, to keep the GL path's hit-testing.
5. "Restart the app" is acceptable when a session moves between the console and
   RDP, for the first round.
6. This spec is its own file, linked from `live-software-renderer.md`.
7. The acceptance bar under Cost.
8. The `CS_OWNDC` with `WS_EX_LAYERED` conflict on the GL path is pre-existing
   and out of scope: #1316.

## Design

### Selection policy

`GOGUI_RENDERER` means on Windows what it means on Linux, through the same
`readRendererEnv`.

| `GOGUI_RENDERER` | Windows                                                                                           |
| ---------------- | ------------------------------------------------------------------------------------------------- |
| `soft`           | The CPU renderer. No pixel format, no WGL, and no `opengl32.dll` is loaded, a local Mesa included |
| `gl`             | GL. A bring-up failure is `ErrNoGPUContext`, with no fallback                                     |
| unset            | GL. If the first window's context or binding fails, the process draws on the CPU renderer         |
| anything else    | As `gl`, with one warning                                                                         |

What each bring-up failure does:

| Step                                      | Where                                            | Unset, first window | `gl`, or a later window of a GL process |
| ----------------------------------------- | ------------------------------------------------ | ------------------- | --------------------------------------- |
| Pixel format, context, make-current       | `createContext`                                  | reopen on soft      | `ErrNoGPUContext`                       |
| Binding the required GL functions         | `glbind.InitWithProcAddrFunc`, called from `New` | reopen on soft      | `ErrNoGPUContext` (since #1319)         |
| Building the built-in pipelines           | `initPipelines`, through `initGLResources`       | `ErrNoGPUContext`   | `ErrNoGPUContext` (since #1319)         |
| Window creation, `GetDC`, the text system | `New`, `initGLResources`                         | plain error         | plain error                             |

There is no version test and no renderer-name test: a machine whose GL comes up
today, a working Mesa included, makes the same calls and cannot be moved to
soft. `GL_VERSION` and `GL_RENDERER` are read only after a failure
(`glIdentity`).

**Lifetime.** The first window's outcome sets the renderer for the process, as
on Linux. If its context or binding fails, it reopens on soft and every later
window opens on soft with no GL attempt. If it is on GL, later windows open on
GL, and one whose bring-up fails returns the classified error. The fallback logs
one line: the step that failed, its cause, `GL_VERSION` and `GL_RENDERER` where
a context exists, `GOGUI_RENDERER=gl`, and that custom shaders are not drawn.
When the soft setup fails too, the error is the GPU error first, so it still
matches `ErrNoGPUContext`, with the soft cause after it, as on Linux.

### Opening a window

By the WGL attempt the `HWND` already has a registry entry, an IME association,
an icon, a DC and possibly transparency and opacity setup. A fallback therefore
discards the whole attempt and opens a second window, of another class (below).

1. `New` locks the OS thread once and reads the choice.
2. The GL attempt runs today's sequence. It publishes nothing to the
   `gui.Window` until the binding has succeeded, the last step that can fall
   back:
   - `applyVSync` reports a refused swap interval to the window and runs before
     the binding today. It moves after it.
   - `enableWindowTransparency` and `applyWindowOpacity` run before WGL and
     report degradations to the window. Each attempt collects its reports, and
     `New` publishes those of the attempt that won.
   - `SetNativePlatform` replays placement requests queued before the backend
     started, so it must run once, on the window that survives. It does:
     `initGLResources` calls it, after the binding.
3. A failed attempt that falls back is torn down with `plat.destroy`: no GL
   resource exists yet. The window was never shown (`showAtStart` is `New`'s
   last step), and its messages went to `DefWindowProc`, because `wndProc`
   routes nothing while `plat.w` is unset.
4. The soft attempt creates its window and attaches the native platform.

`OnInit` runs after `New` returns and is not affected. Step 2's changes land in
phase 3, with the fallback.

### Present path

A soft window has `platformState.softMode` set and a non-nil
`platformState.soft`, as on X11. It has no pixel format, no `hglrc` and no kept
window DC.

- **Window class.** A second class with the same window procedure and without
  `CS_OWNDC`, which excludes `WS_EX_LAYERED`
  ([extended styles](https://learn.microsoft.com/en-us/windows/win32/winmsg/extended-window-styles)).
  Window DCs are taken per use (`GetDC` and `ReleaseDC` around a present,
  `BeginPaint` and `EndPaint` in `WM_PAINT`); the memory DC is kept.
- **Frame store.** One top-down 32 bpp `BI_RGB` DIB section per window, selected
  into the memory DC. Its byte order is B, G, R, A, which is what `swizzleBGRA`
  writes. DWM composes an opaque window opaque, so the alpha byte soft leaves
  below 255 is ignored and there is no `finishFrameAlpha` step.
- **Frame.** `soft.Renderer.Draw` under the window lock, as
  `softPresent.present` does on X11; swizzle into the DIB after the unlock;
  `BitBlt(SRCCOPY)` to the window DC. A frame `Draw` refuses is skipped, logged
  once and still paced.
- **GDI boundary.** `GdiFlush` follows every blit at once, on the window's
  thread, while the destination DC is still held
  ([`CreateDIBSection`](https://learn.microsoft.com/en-us/windows/win32/api/wingdi/nf-wingdi-createdibsection)).
  A present counts only when `BitBlt` and `GdiFlush` both succeed. The next
  write to the DIB, and its disposal, therefore come after GDI has finished
  reading it, with `VSyncOff` too.
- **Resize.** The new DIB is created, then selected into the memory DC. Only
  when `SelectObject` succeeds do the bitmap, its size and its bits pointer
  change, together. If either step fails, the old three stay, so `WM_PAINT`
  still has the last valid frame; the frame is skipped and logged.
- **Repaint.** `WM_PAINT` only validates today. On a soft window it blits the
  DIB and flushes between `BeginPaint` and `EndPaint`, and never sleeps. This
  also covers a window shown after a hidden start.
- **Pacing.** GL is held to the display by `wglSwapIntervalEXT`. Soft waits for
  a deadline after a present, as `softPresent.pace` computes it;
  `WindowCfg.VSyncOff` skips the wait. The interval is the display frequency of
  the window's monitor (`MonitorFromWindow`, then `EnumDisplaySettingsW`), or 60
  Hz when that is 0, 1 or outside 20–1000 Hz. It is re-read on
  `WM_DISPLAYCHANGE` and when the monitor handle changes, checked on `WM_MOVE`
  and `WM_SIZE`. Sleep is a best-effort limiter: sixty 16.67 ms sleeps came back
  4.1 ms late at the median (measured in the design review). A waitable timer is
  the remedy if phase 5 asks for it.
- **Memory.** Two frames per window, soft's RGBA buffer and the DIB: 16.6 MB at
  1920×1080. After a fallback, the `opengl32.dll` it loaded stays mapped.

### Window and message-loop integration

Six sites branch on soft. Each is a one-line delegation to a `*Win32` wrapper,
as the X11 sites delegate to `initSoftX11`, `presentX11` and `resizeX11`.

| Site                      | GL today                                                      | Soft                                                                                                       |
| ------------------------- | ------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `New`                     | GL class, `GetDC`, `createContext`, glbind, `initGLResources` | soft class, `initSoftWin32`: `soft.NewRenderer`, `SetNativePlatform`, `SetWindowRendersShaders(w, false)`  |
| `Destroy`                 | `destroyGLResources` calls GL                                 | flush; release the renderer, the DIB and the memory DC. `plat.destroy` already skips a zero `hglrc` and DC |
| `Run`, the frame call     | `renderFrame`                                                 | `presentWin32`                                                                                             |
| `runAppE`, the frame call | `renderFrame`                                                 | `presentWin32`                                                                                             |
| `WM_SIZE`                 | `handleResize` sets the GL viewport, then `renderFrame`       | `resizeWin32`: size, `applyDPIScale`, `Renderer.SetScale`; then `presentWin32`. Live resize keeps working  |
| `WM_PAINT`                | validate                                                      | `BeginPaint`, blit the DIB, flush, `EndPaint`                                                              |

No change is needed for per-monitor DPI (the `WM_SIZE` that follows
`WM_DPICHANGED` adopts the scale), multiple windows (each backend owns its DIB
and renderer), frameless windows, window state, size limits and occlusion (style
and message handling only), IME and UI Automation (window messages and
`nativePlatform`, which soft installs), or whole-window opacity
(`SetLayeredWindowAttributes` redirects GDI painting too).

**Transparent windows** (`WindowCfg.Transparent`) are phase 4. The GL path makes
DWM honour the framebuffer's alpha with `DwmEnableBlurBehindWindow` and avoids
`WS_EX_LAYERED` because it fights the swap chain (`transparency_win32.go`). Soft
has no swap chain, and the documented route for a bitmap with per-pixel alpha is
a layered window updated with
[`UpdateLayeredWindow`](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-updatelayeredwindow)
and `AC_SRC_ALPHA`, which takes premultiplied 32 bpp: the DIB above.

- The bitmap is the whole window, so this fits a frameless window, whose client
  area is the window. With a system frame the window stays opaque (decision 3),
  as every transparent soft window does until phase 4, with the same report.
- A layered window passes mouse input through where alpha is 0 (probed in the
  design review: alpha 1, 2 and 255 do not); the GL path takes input everywhere.
  Hence the alpha floor of decision 4.
- `SetLayeredWindowAttributes` and `UpdateLayeredWindow` exclude each other, so
  opacity on a transparent window stays refused, as on GL
  (`layeredOpacityAllowed`).

### Where the code lives

All in package `gui/backend/gl`, which already imports `soft` on Linux.

| File                                                                     | Build tag                                   | Content                                                                                                                                |
| ------------------------------------------------------------------------ | ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `softpresent_win32.go` (new)                                             | `windows && !js`                            | soft class, DIB section, present, repaint, pacing, the `*Win32` wrappers                                                               |
| `renderer_pick_win32.go` (new)                                           | `windows && !js`                            | the choice, the process renderer, the fallback and its log text                                                                        |
| `renderer_env.go` (new)                                                  | `((linux && !android) \|\| windows) && !js` | moved, unchanged: `rendererEnv`, `readRendererEnv`, `softInitError`, `newGPUContextErrorNoSoft`, `swizzleBGRA`, `defaultFrameInterval` |
| `platform_win32.go`, `events_win32.go`, `wgl_windows.go`, `gpu_error.go` | as now                                      | the six branches; lazy `opengl32`; the Windows hint                                                                                    |

`glstring.go` is the precedent: #1319 moved `cGoString` and `glRenderer` there
for both platforms. **Lazy `opengl32`:** today the DLL and four WGL procedures
are package variables, and `glProc` looks names up in the same DLL. All of them
go behind one accessor that picks the DLL on first use, in today's order.

## Cost

Each component timed alone: `soft.Renderer.Draw` of a settled frame on one
thread, the swizzle, and `BitBlt` from a DIB section plus `GdiFlush` to a shown
window on a 1920×1080 screen (larger sizes were clipped and are not reported).
Layout, the pacing wait, DWM's composition and RDP's encoding are not included.

| Frame             | Draw, 100 buttons | Draw, window full of widgets | Swizzle | Blit   |
| ----------------- | ----------------- | ---------------------------- | ------- | ------ |
| 1280×800 at 1×    | 9.0 ms            | 10.5 ms                      | 1.5 ms  | 1.0 ms |
| 1920×1080 at 1×   | 11.1 ms           | 24.5 ms                      | 3.5 ms  | 1.5 ms |
| 2560×1440 at 1×   | 12.8 ms           | 46.9 ms                      | 5.8 ms  | —      |
| 3840×2160 at 1.5× | 35.7 ms           | 92.8 ms                      | 12.9 ms | —      |

- At 1920×1080 the three components sum to 16–30 ms. No frame rate is derived
  for 4K: no opaque present was measured above 1920×1080.
- Idle draws nothing, and input costs at most one frame per loop turn: the loop
  drains every pending message before it asks for a frame. A resize is the
  exception. `WM_SIZE` draws inside the message handler, on GL and on soft
  alike, so a live resize draws one full frame for each size the system reports,
  and follows the mouse no faster than the frame times above allow.
- Remote sessions are capped at 30 fps by default
  ([Microsoft](https://learn.microsoft.com/en-us/troubleshoot/windows-server/remote/frame-rate-limited-to-30-fps)).

**Acceptance bar** (decision 7), measured on a VM without a GPU, 2 vCPUs, in an
RDP session, desktop 1920×1080 at 100 %, with `examples/benchmark` under
`GOGUI_RENDERER=soft`. Every figure is recorded with the window's client size:
the benchmark opens at 1024×768, so the desktop size alone does not make a 1080p
workload.

- The showcase starts with no extra file beside the exe.
- Idle: on a page with no animation, process CPU time over 60 s, starting 10 s
  after the last input, stays under 1 % of one core.
- Benchmark, 100 buttons, running, window maximised: frame p50 at most 33 ms.
  The run at the default size is recorded beside it.
- At 4K with 150 % scaling it starts and draws correctly; its frame time is
  recorded as a documented limit.

## Phases

| Phase | Delivers                                                                                             | State             |
| ----- | ---------------------------------------------------------------------------------------------------- | ----------------- |
| 1     | A GL bring-up failure is a GPU-context failure: no panic, and the message names the GL. No soft code | in review (#1319) |
| 2     | `GOGUI_RENDERER=soft` on Windows, opaque windows                                                     | planned           |
| 3     | Automatic fallback when the first window's context or binding fails                                  | planned           |
| 4     | Transparent frameless windows on soft                                                                | planned           |
| 5     | Follow-up evidence: the wider machine matrix, the numbers recorded here, follow-up issues            | planned           |

Each phase is one pull request, carries its tests, docs and `CHANGELOG.md`
entry, and updates the table above. The steps, in execution order:

1. **Phase 1, done in #1319.** `New` reports a glbind failure and a pipeline
   failure through `glBringUpError`. `initGLResources` marks a pipeline failure
   with `pipelineInitError`, so the text-system error stays plain.
2. **Phase 2.** Move the shared helpers and their tests to `renderer_env.go`,
   and put `opengl32.dll` and its procedures behind the lazy accessor.
3. Register the soft window class. Split `New` into the thread lock and choice,
   a GL attempt and a soft attempt; the GL attempt is today's sequence.
4. Add `softpresent_win32.go` and the six branches of the integration table.
5. `GOGUI_RENDERER=soft` selects the path; a transparent window is opaque, with
   a report. Add ", or set GOGUI_RENDERER=soft" to the Windows error text.
6. Give `glrendertest` its Windows selections (see Tests).
7. **Phase 3.** Add the process renderer, the first-window fallback and its log
   line to `New`. Move `applyVSync` after the binding, and collect each
   attempt's degradation reports for `New` to publish.
8. Run the CI showcase smoke test twice with the variable unset: with no Mesa
   DLL (the real no-GPU start, which must stay up), and with Mesa (which must
   stay on GL).
9. Before phase 3 merges, run the showcase and the benchmark on a machine
   without a vendor driver. Record in the pull request: the fallback line, a
   live resize, minimise and restore, and the acceptance figures.
10. **Phase 4.** Create a transparent, frameless soft window with
    `WS_EX_LAYERED`, skip `enableWindowTransparency`, and present it with
    `UpdateLayeredWindow` from the memory DC, then flush. Raise alpha 0 to 1 on
    that path.
11. **Phase 5.** Run the matrix under Tests and record the numbers here. File
    follow-up issues from what they show (swizzle, HiDPI, timer).

Docs, in the phase that changes the behaviour: `docs/deployment.md` beside the
Linux paragraph, one sentence in `README.md`, and the doc comments that name the
platforms (`gpu_error.go`, `gui/backend/soft/doc.go`,
`gui/backend/soft/renderer.go`). `CHANGELOG.md` gets `### Fixed` in phase 1 and
`### Added` in phases 2, 3 and 4.

## Tests

**Selections.** `glrendertest` runs one list per OS, on Windows `commonTests`
plus `win32Tests`, with Mesa's DLL beside the binary and `GOGUI_RENDERER=gl`
forced for every test (`startTest`). Phase 2 gives it three selections on
Windows, each one process per test as now:

| Selection   | Mesa DLL beside the binary | `GOGUI_RENDERER`           | Runs                                               |
| ----------- | -------------------------- | -------------------------- | -------------------------------------------------- |
| Mesa GL     | yes                        | `gl`, or unset by the test | today's list, plus the keeps-GL test               |
| forced soft | yes                        | `soft`                     | the soft present tests                             |
| system GL   | no                         | unset or `gl`, by the test | phase 1's regression test, then the fallback tests |

`glrendertest` reports a skip as a failure, and the regression test skips when
its retry is refused too, so the system GL selection also shows what the
runner's own OpenGL does.

- **Phase 1 (#1319).** `TestGDIGenericIsGPUContextError`, the regression test,
  picks a generic pixel format through the `createContextFunc` seam.
  `TestPipelineFailureIsGPUContextError`, `TestTextSystemFailureStaysPlain`.
- **Phase 2, pure.** `TestDIBHeader`, `TestDisplayFrameInterval`,
  `TestPaceDeadline`; the moved `TestReadRendererEnv` and `TestSwizzleBGRA`.
- **Phase 2, forced soft.** `TestSoftWin32LoadsNoGL`: with Mesa's DLL present,
  frames draw with no `opengl32.dll` module loaded.
  `TestSoftWin32PixelsMatchRenderToImage` reads the shown window back through a
  window DC and compares it with `RenderToImage`; it then paints over a patch,
  invalidates it and requires the real `WM_PAINT` to restore the pixels.
  `TestSoftWin32ResizeKeepsHandles`: the GDI object count holds over repeated
  resizes, and a failed DIB creation or selection keeps the last frame.
  `TestSoftWin32OpacityAfterPaint`, `TestSoftWin32PresentAllocs`.
- **Phase 3, fallback.** Each trigger is forced: `TestFallbackOnRefusedContext`
  through the `createContextFunc` seam, `TestFallbackOnGDIGeneric` with the
  pixel format of phase 1's test. Both require a soft window and one log line.
  `TestPipelineFailureDoesNotFallBack`, `TestRendererGLReportsError` (with `gl`,
  no fallback), `TestFallbackSoftFailure`, `TestFallbackKeepsHiddenStart`,
  `TestFallbackKeepsQueuedPlacement`, `TestFallbackRunsOnInitOnce`,
  `TestFallbackPublishesWinningReportsOnly`. `TestSoftProcessMakesNoGLAttempt`:
  after a fallback, a second window is on soft and the seam is never called.
- **Phase 3, Mesa GL.** `TestUnsetWithMesaKeepsGL`: the window is on GL and a
  custom shader's fill is in the read-back pixels.
  `TestLaterWindowGLFailureReturnsError`: no soft window opens.
- **Phase 4.** `TestLayeredAlphaFloor` (pure: 0 becomes 1, 2 stays 2);
  `UpdateLayeredWindow` succeeding on a real window.

**Follow-up evidence (phase 5).** Each run records the GL strings, or the
fallback line and the step that failed, on: Hyper-V without GPU-P; QEMU with std
VGA and with virtio-gpu, on more than one Windows version; an RD Session Host
without the hardware-adapter policy; Windows Sandbox with and without vGPU;
VirtualBox with and without Guest Additions; RDP into a workstation with a GPU.
Each also covers monitors with different scaling and refresh rates, an IME
composition, Narrator, the tray, and the hidden-start, frameless and transparent
examples. Session moves are recorded, not fixed (decision 5).

## Rejected approaches

- **B, bundling Mesa.** A 100 MB-class binary dependency, for a renderer that
  costs more CPU and memory than the one in the tree. The exe-directory hook
  stays, for CI and for an app that ships Mesa.
- **C, ANGLE.** A binary dependency, an EGL platform layer and ES pipelines; on
  a machine without a GPU it ends on WARP.
- **A native Direct3D 11 backend, on WARP where there is no GPU.** A fourth GPU
  backend with HLSL pipelines and a translator for GLSL custom shaders, in the
  class of the rejected WebGPU backend (`CLAUDE.md`).
- **Microsoft's OpenGL Compatibility Pack** (GL 3.3 on D3D12). A Store package
  per machine, not installable where the Store is blocked, and Go-Gui is not
  shown to run on it.
- **Keeping GL only and fixing the error.** The app still does not start; the
  fix itself is phase 1.
- **Falling back on a pipeline failure.** See decision 2.
- **Picking soft because `GL_RENDERER` names a software rasterizer**, as Linux
  does. On Windows llvmpipe appears only because the application shipped Mesa
  beside the exe, usually for its shaders, which soft would remove.
- **A `GL_VERSION` gate.** A context below 3.3 can still export every entry
  point the backend needs through extensions, so a version comparison could
  reject a machine that works today. The test is what fails.
- **A later window falling back alone** in a process whose first window is on
  GL. The renderer is chosen once per process, as Linux's X11 probe chooses it.
  A mixed process would have windows of one app differ in whether they draw
  custom shaders, for a case no machine has shown. The later window returns the
  error, as today.
- **A probe window before the first window**, as Linux probes EGL. `New` creates
  the window hidden and shows it last, so the first window's own bring-up is the
  probe and a working machine pays nothing.
- **Deciding from the session or the hardware** (`SM_REMOTESESSION`, adapter
  enumeration). An RDP session can have hardware GL and a console session can
  lack it, as on the lab VM; Microsoft documents `SM_REMOTESESSION` as wrong
  under RemoteFX vGPU
  ([docs](https://learn.microsoft.com/en-us/windows/win32/termserv/detecting-the-terminal-services-environment)).
- **Switching renderer while the app runs** (on `WM_WTSSESSION_CHANGE`). Soft
  uses another window class, so it means a new `HWND`, which the IME association
  and the UI Automation provider hold. Restart the app (decision 5).
- **`DwmEnableBlurBehindWindow` plus alpha bytes written by `BitBlt`** for
  transparent soft windows. It relies on what GDI happens to leave in the alpha
  channel. `UpdateLayeredWindow` is the documented per-pixel-alpha path.
- **`DwmFlush` for pacing.** Not verified in remote sessions or with the display
  off. A follow-up.
- **A `presenter` interface** (decision 1), which `live-software-renderer.md`
  said to revisit for a Windows path. This design adds six one-line branches and
  shares almost nothing with the X11 and Wayland present code.
- **Drawing straight into BGRA, damage tracking, threaded tiles,
  `WindowCfg.Renderer`.** All rejected for Linux in `live-software-renderer.md`,
  for reasons that hold here.

## Known limits

- Full frame on every redraw, on one thread, with no damage tracking, as on
  Linux. HiDPI is slow (see Cost); the alternative today is no window.
- A custom shader draws its children and no fill; `Window.RendersShaders`
  reports it. Mesa beside the exe keeps GL.
- Sleep pacing is late by milliseconds and not locked to DWM's composition: a
  lower animation rate and possible judder, but no tearing.
- A process that fell back does not return to GL, at the console after an RDP
  start either. In a GL process a later window whose GL fails returns an error.
- A transparent window on soft is opaque until phase 4, and afterwards only a
  frameless one is transparent, with the alpha floor.
- The GL window class has `CS_OWNDC`, and `applyWindowOpacity` sets
  `WS_EX_LAYERED` on it, which the style documentation excludes. Pre-existing,
  not changed here: #1316.

## Not verified

- The soft path on any machine: it does not exist yet. The lab VM runs cover
  phase 1 only.
- Why the lab VM refuses the first context and grants the second, and whether
  another Windows version, adapter or hypervisor does the same. What the CI
  runner's own OpenGL does at the first attempt.
- Every "expected" cell of the environment table. Windows Sandbox with vGPU,
  Hyper-V with GPU-P and VirtualBox with Guest Additions have no row: unknown.
- What a running GL window does when a session connects or disconnects.
- That a window DC reads back the presented pixels on the CI runner, for a
  layered window too. The destination-pixel and opacity tests depend on it.
- That checking the monitor handle on `WM_MOVE`, `WM_SIZE` and
  `WM_DISPLAYCHANGE` catches every monitor change.
- `BitBlt` cost above 1920×1080, and all cost inside an RDP session or a VM.
  Mesa llvmpipe against soft on Windows; the 2.5–3× figure is Linux's.
- The sleep overshoot and the alpha hit-test results come from the design review
  and were not repeated.
- That `UpdateLayeredWindow`'s bitmap covers the non-client area; it follows
  from the documentation's wording.
