# Windows software renderer

Issue #1310 (follow-up of #1265). Status: **phases 1 to 4 of 5 landed** (#1319,
#1333, #1338, #1344; phase 5 planned).

The maintainer accepted this design on the issue on 2026-11-20 (see Decisions).
Code facts are as of #1319, except where a sentence names phase 2, 3 or 4.
"Measured" with no machine named means throwaway programs on one 16-thread AMD
desktop (Windows 11 24H2, Go 1.27.4, a 1920×1080 screen, median of 10–20 runs);
those figures are indicative. "The lab VM" is the one machine without a GPU
driver that was tested (see Problem).

## Problem

A Go-Gui app does not start on a Windows machine whose OpenGL cannot run the
backend. Linux got a CPU path in #1265 (`docs/specs/live-software-renderer.md`),
which left Windows out of its first round.

Without a vendor driver, `opengl32.dll` runs Microsoft's generic implementation:
`GL_RENDERER` "GDI Generic", version 1.1
([OpenGL on Windows](https://learn.microsoft.com/en-us/windows/win32/opengl/opengl)).
It has no `wglCreateContextAttribsARB`, `glAttachShader` or `glGenVertexArrays`
(measured by forcing a generic pixel format). The backend fails on it in one of
two ways, and before phase 2 neither message had a remedy for Windows: the
`GOGUI_RENDERER=soft` hint was Linux-only (`gpuContextError.Error`).

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
context was the trigger until #1348, and a process that fell back makes no
further GL attempt. No test may assume which way the system OpenGL fails.

**Why the first context was refused** (found in phase 5, fixed in #1348). The
system `opengl32.dll` was loaded by the first `wglCreateContext` call, after
`ChoosePixelFormat` and `SetPixelFormat`. A DLL that is first loaded after
`SetPixelFormat` gets no context, as phase 2 measured for a Mesa beside the exe;
a second attempt worked because the DLL was loaded by then. Measured on
2026-11-22, console sessions, with a probe that loaded the system DLL before the
first attempt (upstream `main` at `0ec3b7ca`):

- On the lab VM the first attempt then gets GDI Generic 1.1.0 and fails at the
  binding. The fallback still happens; its line names
  `bind GL functions failed (...)` instead of `create WGL context failed (...)`.
- In a VirtualBox guest with Guest Additions and 3D acceleration, whose OpenGL
  4.1 works (Mesa 24.0.2, SVGA3D), the first attempt was refused the same way,
  so every window fell back to the CPU renderer. With the DLL loaded first, the
  first attempt gets the 4.1 context and the window stays on GL.

Since #1348 the GL attempt loads the system DLL before the window and the pixel
format, as it did for Mesa. Measured with that build on 2026-11-22: on the lab
VM, console session and RDP session, the first attempt of every process fails at
the binding with GDI Generic 1.1.0 and the window opens on the CPU renderer; in
the VirtualBox guest, console session, the first attempt gets the 4.1 context
and `examples/get_started` draws with OpenGL. On a workstation with an AMD
driver the first attempt got a context with either load order.

**Which machines.** Only the rows marked measured or stated are observations.
"No vendor GL expected" comes from the adapter's documentation and does not say
which of the two ways the machine fails.

| Environment                                                  | OpenGL                                                             | Basis                                                                                                                                                                                                    |
| ------------------------------------------------------------ | ------------------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| QEMU VM, standard VGA, console session and RDP session       | GDI Generic 1.1.0; first context refused before #1348              | measured on the lab VM (#1319), one Windows version; with #1348 the first attempt gets GDI Generic, measured on the console and over RDP                                                                 |
| VirtualBox 7.2 guest, VBoxSVGA, Guest Additions, 3D on       | 4.1 core (Mesa 24.0.2, SVGA3D); first context refused before #1348 | measured 2026-11-22 on the console, on `main` at `0ec3b7ca` and with #1348                                                                                                                               |
| A generic pixel format forced on a machine with a GPU driver | GDI Generic 1.1.0 at the first attempt                             | measured                                                                                                                                                                                                 |
| GitHub `windows-latest` runner (Windows Server 2025)         | GDI Generic 1.1.0 at the first attempt                             | measured in the CI run of #1348's pull request: `TestFirstGLAttemptOfProcess` logs both attempts in the system GL selection                                                                              |
| Hyper-V VM without GPU-P or DDA                              | no vendor GL expected                                              | the "Microsoft Hyper-V Video" adapter is display-only ([GPU-PV](https://learn.microsoft.com/en-us/windows-hardware/drivers/display/gpu-paravirtualization))                                              |
| QEMU with QXL or virtio-gpu                                  | no vendor GL expected                                              | the Windows virtio-gpu driver is display-only ([QEMU](https://www.qemu.org/docs/master/system/devices/virtio/virtio-gpu.html))                                                                           |
| RDP session on an RD Session Host, policy unset              | no vendor GL expected; a vendor driver in "all other cases"        | the hardware-adapter [policy](https://learn.microsoft.com/en-us/windows/client-management/mdm/policy-csp-admx-terminalserver#ts_dx_use_full_hwgpu) text                                                  |
| Windows Sandbox with vGPU disabled                           | no vendor GL expected                                              | the sandbox then renders with WARP ([.wsb docs](https://learn.microsoft.com/en-us/windows/security/application-security/application-isolation/windows-sandbox/windows-sandbox-configure-using-wsb-file)) |
| Mesa's `opengl32.dll` beside the exe                         | the driver Mesa selects: llvmpipe when forced, as CI does          | [mesa-dist-win](https://github.com/pal1000/mesa-dist-win#readme)                                                                                                                                         |

**The Mesa hook.** `initOpenGL32` loads `opengl32.dll` from the executable's
directory and falls back to System32. Until phase 2 it ran in a package variable
initializer, so in every process; since phase 2 it runs when a GL attempt starts
(`openGL32`), and never on soft. Until #1348 it loaded only Mesa there; the
system DLL was loaded by its first procedure call. An application ships Mesa
there on purpose, typically to keep custom shader fills, which the CPU renderer
does not draw. CI does it too.

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

Since phase 3 the line reads
`gl: <step> failed (<cause>); drawing with the CPU renderer (GOGUI_RENDERER=gl reports the error instead; custom shaders are not drawn)`,
and it is logged once the soft setup has succeeded, so a soft setup that fails
too logs nothing. The process renderer (`processRenderer`, in
`renderer_pick_win32.go`) is set by the first window that opens with the
variable unset. A first window that opens no window at all, through a plain
error, a pipeline failure or a soft setup that fails too, leaves it undecided,
and the next window is a first window again. `gl`, `soft` and other values are
read per window and neither read nor set it. A soft setup failure is a
`softInitError` (renderer setup); a failure to create the soft window stays a
plain error, as on GL.

**Ordering.** No window's bring-up waits on another's. The process renderer's
lock guards only its state, the choice and a `deciding` flag, and is never held
across a bring-up: application code can run inside one before the choice is made
(an invalid `IconPNG` or a font logs a warning through the app's log writer) and
when the window attaches to the `gui.Window` (`SetNativePlatform` runs an
`OnSystemAppearance` hook), and either may open a window. A window that finds
the choice made opens there. One that finds no choice and no bring-up deciding
sets `deciding` and is the deciding window: each attempt commits just before the
attach, once no later step can fail, by setting the choice, and the soft attempt
then logs the line. So a window opened from the attach, on this thread or
another, finds the choice made: GL when the first window is on GL, soft with no
GL attempt when it fell back. The deciding window clears `deciding` on every
return, so after a failure without a choice the next window decides. A window
that finds another bring-up deciding, opened from code that bring-up reached
before its commit or from another thread at the same moment, makes its own
attempt, GL with the same fallback and its own fallback line, without setting
the choice or waiting. In principle it could end on another renderer than the
deciding window, on a machine whose two attempts differ; no machine has shown
that. Until phase 3's first review the lock was held for every bring-up, and a
window opened from the hook blocked on it, on the same thread, for good; until
the second, the first window held it up to its commit, and a window opened from
the app's log writer on the icon warning blocked the same way. The icon warning
is now one of the attempt's reports, logged after the choice is made.

### Opening a window

By the WGL attempt the `HWND` already has a registry entry, an IME association,
an icon, a DC and possibly transparency and opacity setup. A fallback therefore
discards the whole attempt and opens a second window, of another class (below).

1. `New` locks the OS thread once and reads the choice.
2. The GL attempt runs today's sequence. It publishes nothing to the
   `gui.Window` until the binding has succeeded, the last step that can fall
   back:
   - `applyVSync` reports a refused swap interval to the window. It ran before
     the binding until phase 3, and runs after it since.
   - `enableWindowTransparency` and `applyWindowOpacity` run before WGL and
     report degradations to the window. Each attempt collects its reports, and
     `New` publishes those of the attempt that won.
   - `SetNativePlatform` replays placement requests queued before the backend
     started, so it must run once, on the window that survives. It does:
     `attachGLWindow`, the part of `initGLResources` that cannot fail, calls it
     after the binding and the pipelines, once the choice is committed.
3. A failed attempt that falls back is torn down with `plat.destroy`: no GL
   resource exists yet. The window was never shown (`showAtStart` is `New`'s
   last step), and its messages went to `DefWindowProc`, because `wndProc`
   routes nothing while `plat.w` is unset.
4. The soft attempt creates its window, commits the choice and attaches the
   native platform.

`OnInit` runs after `New` returns and is not affected. Step 2's changes landed
in phase 3, with the fallback: `enableWindowTransparency`,
`reportSoftTransparency`, `applyWindowOpacity`, `setWindowIcon` and the swap
interval's `applyVSync` take a report function, which `New` points at the
attempt's `windowReports`. A bad icon is logged once, and a window that fails
after the swap interval is set reports no vsync finding.

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
  below 255 is ignored and there is no `finishFrameAlpha` step. A layered window
  (phase 4, below) reads the byte as premultiplied alpha.
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
  still has the last valid frame; the frame is skipped and logged. The present
  compares the DIB's size with the client size at every frame, so the next frame
  allocates again. There is no timer-driven retry: a failed `CreateDIBSection`
  means memory or GDI handles ran out. The DIB is allocated after `Draw`, so a
  size `Draw` refuses leaves the last frame in place too.
- **Repaint.** `WM_PAINT` only validates today. On a soft window it blits the
  DIB and flushes between `BeginPaint` and `EndPaint`, and never sleeps. This
  also covers a window shown after a hidden start.
- **Pacing.** GL is held to the display by `wglSwapIntervalEXT`. Soft waits for
  a deadline after a present, as `softPresent.pace` computes it;
  `WindowCfg.VSyncOff` skips the wait. The interval is the display frequency of
  the window's monitor (`MonitorFromWindow`, then `EnumDisplaySettingsW`), or 60
  Hz when that is 0, 1 or outside 20–1000 Hz. It is re-read on
  `WM_DISPLAYCHANGE` and when the monitor handle changes, checked at every
  present (one `MonitorFromWindow` call), so a move needs no `WM_MOVE` handler.
  The per-frame GDI and user32 calls use `syscall.SyscallN`, because
  `LazyProc.Call` moves its arguments to the heap. Sleep is a best-effort
  limiter: sixty 16.67 ms sleeps came back 4.1 ms late at the median (measured
  in the design review). A waitable timer is the remedy if phase 5 asks for it.
- **Memory.** Two frames per window, soft's RGBA buffer and the DIB: 16.6 MB at
  1920×1080. After a fallback, the `opengl32.dll` it loaded stays mapped.

### Window and message-loop integration

Six sites branch on soft. Each is a one-line delegation to a `*Win32` wrapper,
as the X11 sites delegate to `initSoftX11`, `presentX11` and `resizeX11`. A
seventh, `WM_DISPLAYCHANGE`, tells the pacer to re-read the display frequency
(`displayChangedWin32`); GL ignores it.

| Site                      | GL today                                                      | Soft                                                                                                       |
| ------------------------- | ------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `New`                     | GL class, `GetDC`, `createContext`, glbind, `initGLResources` | soft class, `initSoftWin32`: `soft.NewRenderer`, `SetNativePlatform`, `SetWindowRendersShaders(w, false)`  |
| `Destroy`                 | `destroyGLResources` calls GL                                 | flush; release the renderer, the DIB and the memory DC. `plat.destroy` already skips a zero `hglrc` and DC |
| `Run`, the frame call     | `renderFrame`                                                 | `presentWin32`                                                                                             |
| `runAppE`, the frame call | `renderFrame`                                                 | `presentWin32`                                                                                             |
| `WM_SIZE`                 | `handleResize` sets the GL viewport, then `renderFrame`       | `resizeWin32`: size, `applyDPIScale`, `Renderer.SetScale`; then `presentWin32`. Live resize keeps working  |
| `WM_PAINT`                | validate                                                      | `BeginPaint`, blit the DIB, flush, `EndPaint`; a layered window only validates (phase 4)                   |

No change is needed for per-monitor DPI (the `WM_SIZE` that follows
`WM_DPICHANGED` adopts the scale), multiple windows (each backend owns its DIB
and renderer), frameless windows, window state, size limits and occlusion (style
and message handling only), IME and UI Automation (window messages and
`nativePlatform`, which soft installs), or whole-window opacity
(`SetLayeredWindowAttributes` redirects GDI painting too).

**Transparent windows** (`WindowCfg.Transparent`, phase 4). The GL path makes
DWM honour the framebuffer's alpha with `DwmEnableBlurBehindWindow` and avoids
`WS_EX_LAYERED` because it fights the swap chain (`transparency_win32.go`). Soft
has no swap chain, and the documented route for a bitmap with per-pixel alpha is
a layered window updated with
[`UpdateLayeredWindow`](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-updatelayeredwindow)
and `AC_SRC_ALPHA`, which takes premultiplied 32 bpp: the DIB above. The
measurements below are from the workstation.

- **Which windows.** A soft window that is `Transparent` and `DecorationNone` is
  created with `WS_EX_LAYERED` (`softExStyle`), on the soft class, which has no
  `CS_OWNDC`. `enableWindowTransparency` is not called for it, and it makes no
  transparency report. The bitmap is the whole window, so a transparent window
  with a system frame stays opaque (decision 3), with one report: "the CPU
  renderer on Windows draws a window with a system frame opaque; a frameless one
  (DecorationNone) is transparent". After a fallback, the GL attempt's
  `enableWindowTransparency` call and its report go with the discarded window.
- **Client area is the window.** A frameless window answers `WM_NCCALCSIZE` with
  no non-client area, but the first one comes inside `CreateWindowExW`, before
  the window is registered, and `DefWindowProc` gives a `WS_THICKFRAME` window
  its resize border there: a 320×200 client in a 334×214 window at 100 %. For a
  layered window, `fitLayeredFrame` sends a frame change and sets the window to
  the client size it was created with, and `routeMessage` answers
  `WM_NCCALCSIZE` for a layered window before `Run`. The client rect then equals
  the window rect from the start, in the normal, resized and maximized states
  (the frameless maximize clamp, `clampMaximizeToWorkArea`), for both frameless
  styles; `FixedSize`'s plain `WS_POPUP` has no border to remove. The size
  change is sent with `SWP_NOSENDCHANGING`: in `WM_WINDOWPOSCHANGING`,
  `DefWindowProc` holds a `WS_THICKFRAME` window to its minimum track size,
  14×14 at 100 %, which made an 8×80 window 14×80 and a 1×1 window 14×14
  (measured). Creating a layered window at the client size and sending only the
  frame change does not avoid it: `CreateWindowExW` applies the same minimum,
  and the 8×80 window is created 14×80 (measured). The size limits
  (`trackSizeFor`) are answered only once `Run` has started, as before. For a
  frameless window they are now client sizes with nothing added: the function
  added the border `AdjustWindowRectExForDpi` counts for `WS_THICKFRAME`, which
  a frameless window does not have as non-client area, so a `MinWidth` of 200
  held the client at 214 and a `MaxWidth` of 400 let it reach 414, on GL and on
  soft (measured; found in the review of #1344).
- **Resize before the loop.** A `WM_SIZE` that comes between `New` and `Run` (a
  `SetWindowPos` on the window, as the review did) reaches `DefWindowProc`,
  because `wndProc` routes nothing else to the backend until a run loop sets
  `plat.w`. The client size and scale `New` read, and the window's logical size,
  stayed at the creation size. On `main` that is a first frame at the old size,
  on GL and on soft (measured on `c41eb6b8`: a 304×191 client drawn as 320×200);
  on a layered window, whose present needs the frame to be the window's size, it
  was every frame skipped and no window on screen. Both loops now take the
  window through `attachWin32`, which compares the client rect and the scale
  with what `New` read and, when they differ, runs `resizeWin32` before
  `OnInit`. GL makes its context current first, because `handleResize` sets the
  viewport of the current context. The `EventResized` that `WM_SIZE` would have
  sent is held back: it runs the application's `OnEvent`, which on `c41eb6b8`
  never ran before `OnInit`, nor in `RunApp` before the window was registered
  with the app and had its wake hook. The first version of the reconcile sent it
  at once, and the review reproduced the cost: a resize handler calling
  `w.App().OpenWindow` found no app, platform ID 0 and `OnInit` still pending,
  and the request was lost. `sendStartResize` sends it after `OnInit` (in `Run`
  after the wake hook is set, in `RunApp` after the registration), before the
  first frame, with the size current then, so a `WM_SIZE` handled during
  `OnInit` is not overwritten by an older size. When the window's logical size
  already matches, after such a `WM_SIZE` or a change of scale alone, no second
  event is sent. gui has no way to set a window's logical size without
  dispatching to the application, so `OnInit` reads the creation size, as on
  `c41eb6b8`; the first frame has the real one. A minimized window is left to
  the `WM_SIZE` of its restore. A queued `Window.SetWindowSize` does not reach
  this case: `ResizeWindow` posts the request, and the loop handles it after the
  attach (measured).
- **Present.** `swizzleBGRALayered` swizzles and applies the alpha floor in one
  pass. Then `UpdateLayeredWindow` from the memory DC: no `pptDst`, `psize` the
  frame's size, `pptSrc` (0, 0), `ULW_ALPHA` with `AC_SRC_OVER`, constant alpha
  255 and `AC_SRC_ALPHA`; then `GdiFlush`, on the window's thread. A failed call
  is a skipped frame, logged once with the Win32 error and still paced; the
  frame stays in the DIB and the next present sends it. The call's structs live
  in `softPresentWin32`, so a present allocates nothing.
- **Size.** `psize` is the window's new size, and the bitmap covers the window
  rect, frame included. Without `psize` the call succeeds and shows nothing new;
  a `psize` unlike the window's resizes the window, a minimized one too
  (measured). So the present reads the window rect and sends the frame only when
  the two sizes are equal: a frame never resizes the window. When they differ, a
  minimized window, parked at 160×28, is skipped without a log line, and the
  `WM_SIZE` of the restore presents. `IsIconic` is asked only then, so a
  minimized window whose parked size equals the frame's is updated, which
  resizes nothing. Any other difference is expected for one frame and a fault
  after that: when another thread or process resizes the window, the system
  changes the window rect once this thread has answered `WM_WINDOWPOSCHANGING`
  and `WM_NCCALCSIZE`, and only then sends the `WM_WINDOWPOSCHANGED` that
  becomes `WM_SIZE`, so the loop draws one frame in between at the old size (292
  of 320 resizes of an animating window from another process did, measured).
  That frame is dropped with no log line and no pacing wait, and the loop's next
  turn handles `WM_SIZE` and presents at the new size. The next frame that does
  not fit, before one that does, is a skipped frame, logged. A window that draws
  one frame and then idles therefore logs nothing if its size were wrong for
  good; the one known cause of that, a resize before the loop starts, is
  reconciled above.
- **`WM_PAINT`** only validates on a layered window: the system composes it from
  the last bitmap it was given. It is sent no `WM_PAINT` after a present, a
  resize, or a minimize and restore, but `InvalidateRect` still sends one, with
  an empty paint rect.
- **First frame and hidden start.** A layered window that was never updated
  shows nothing. `showAtStart` shows it, and the loop's first frame puts it on
  screen. A frame sent while the window is hidden is kept, and shows when
  `Window.Show` shows the window.
- **Alpha floor** (decision 4). A layered window passes mouse input through
  where alpha is 0; the GL path takes input everywhere. A pixel of alpha 0 gets
  alpha 1, so a premultiplied (0, 0, 0, 0) becomes (0, 0, 0, 1) and darkens what
  is behind it by 1/255. A point of a margin left at alpha 0 hit-tests
  (`WindowFromPoint`) to the window behind, and with the floor to the layered
  window, which a real click there reaches. The floor costs 2.1 ms against
  swizzleBGRA's 1.7 ms at 1920×1080, and 8.4 ms against 6.2 ms at 3840×2160
  (`BenchmarkSwizzleBGRALayered`); an opaque window keeps `swizzleBGRA`.
- **Opacity.** `SetWindowOpacity` stays refused on every transparent window
  (`layeredOpacityAllowed`), the replay of a fade set before the start included,
  and `fadeRefusal` says why for each kind. On GL, the fade's `WS_EX_LAYERED`
  would fight the swap chain of a window made transparent by blur-behind. A
  layered soft window shows its alpha through `UpdateLayeredWindow`, which fails
  once `SetLayeredWindowAttributes` has been called on the window (the two
  exclude each other). A transparent soft window with a system frame is drawn
  opaque and would take the fade, as any opaque soft window does; it is refused
  so that the answer does not depend on the renderer, and a fallback from GL
  does not change it.

### Where the code lives

All in package `gui/backend/gl`, which already imports `soft` on Linux.

| File                                                                     | Build tag                                   | Content                                                                                                                                |
| ------------------------------------------------------------------------ | ------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `softpresent_win32.go` (new)                                             | `windows && !js`                            | soft class, DIB section, present, repaint, pacing, the `*Win32` wrappers                                                               |
| `renderer_pick_win32.go` (new, phase 3)                                  | `windows && !js`                            | the choice, the process renderer, the fallback and its log text                                                                        |
| `renderer_env.go` (new)                                                  | `((linux && !android) \|\| windows) && !js` | moved, unchanged: `rendererEnv`, `readRendererEnv`, `softInitError`, `newGPUContextErrorNoSoft`, `swizzleBGRA`, `defaultFrameInterval` |
| `platform_win32.go`, `events_win32.go`, `wgl_windows.go`, `gpu_error.go` | as now                                      | the six branches; lazy `opengl32`; the Windows hint                                                                                    |

Phase 2 moved four of the six shared names; `softInitError` and
`newGPUContextErrorNoSoft` moved with the fallback in phase 3, because Windows
did not call them before it and the `unused` linter rejects them under
`GOOS=windows`. `New` keeps the thread lock and the choice in
`platform_win32.go`, with `openWin32Window` and `finishWin32Window`, the window
setup both renderers share. The GL attempt (`newGLWin32`) is in
`wgl_windows.go`, the soft attempt (`newSoftWin32`) in `softpresent_win32.go`:
`platform_win32.go` was at 798 of its 800 lines. Since phase 3, `New` reads the
choice and hands it to `openWin32Backend` in `renderer_pick_win32.go`, which
runs the attempts, the fallback and the report publishing.

`glstring.go` is the precedent: #1319 moved `cGoString` and `glRenderer` there
for both platforms. **Lazy `opengl32`:** until phase 2 the DLL and four WGL
procedures were package variables, and `glProc` looks names up in the same DLL.
Since phase 2 all of them are behind one accessor, `openGL32`, that picks the
DLL on first use, in the same order. The GL attempt calls it first, before the
window and the pixel format: a Mesa beside the exe that is first loaded after
`SetPixelFormat` gets no context from `wglCreateContext` (measured with Mesa
26.3.0; the Mesa GL selection fails without the early call). The accessor loaded
only Mesa: for the system DLL it made a lazy handle, which the first
`wglCreateContext` call loaded, so the first context of a process without Mesa
was refused the same way (found in phase 5). Since #1348 `openGL32` loads either
DLL before it returns; a DLL that cannot be loaded is a GPU-context failure that
the first window falls back on, where the lazy procedures used to panic at the
first WGL call. `TestFirstGLAttemptOfProcess` checks that the DLL is loaded when
`openGL32` returns, and that the first GL attempt of a process ends where the
second does.

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
- On the lab VM (phase 2; `examples/benchmark` with 100 buttons, timed inside
  the present path by a throwaway build): a frame costs 4.0 ms of `Draw`, 1.1 ms
  of swizzle and 0.2 ms of blit at 1024×768 in the console session, and 5.2 ms,
  2.1 ms and 0.6 ms maximised at 1920×1009 in an RDP session. The loop presents
  64 to 65 frames a second on the console, whose display reports 64 Hz, and 32
  to 33 in the RDP session, which reported 32 Hz. An idle window was blocked in
  the message wait for 69.96 of 70 s. Those frame costs predate #1328 and #1329;
  with both, `Draw` costs 3.1 ms in the console run and 3.3 ms in the RDP
  session at either size.
- Two traps for whoever measures the acceptance bar. Process CPU time is no
  measure of an animating window on that VM: Windows samples it at the 64 Hz
  clock tick, the paced loop runs in step with the tick, and the same load read
  as anything from 0 % to 100 % of a core. And the benchmark's FPS figure counts
  view runs, not presented frames: it showed 64 while the loop presented 32 to
  33, because `FrameFn` runs the view a second time in a frame whose view asked
  for a layout.

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

| Phase | Delivers                                                                                             | State          |
| ----- | ---------------------------------------------------------------------------------------------------- | -------------- |
| 1     | A GL bring-up failure is a GPU-context failure: no panic, and the message names the GL. No soft code | landed (#1319) |
| 2     | `GOGUI_RENDERER=soft` on Windows, opaque windows                                                     | landed (#1333) |
| 3     | Automatic fallback when the first window's context or binding fails                                  | landed (#1338) |
| 4     | Transparent frameless windows on soft                                                                | landed (#1344) |
| 5     | Follow-up evidence: the wider machine matrix, the numbers recorded here, follow-up issues            | planned        |

Each phase is one pull request, carries its tests, docs and `CHANGELOG.md`
entry, and updates the table above. The steps, in execution order:

1. **Phase 1, done in #1319.** `New` reports a glbind failure and a pipeline
   failure through `glBringUpError`. `initGLResources` marks a pipeline failure
   with `pipelineInitError`, so the text-system error stays plain.
2. **Phase 2, done in #1333.** Move the shared helpers and their tests to
   `renderer_env.go`, and put `opengl32.dll` and its procedures behind the lazy
   accessor.
3. Register the soft window class. Split `New` into the thread lock and choice,
   a GL attempt and a soft attempt; the GL attempt is today's sequence.
4. Add `softpresent_win32.go` and the six branches of the integration table.
5. `GOGUI_RENDERER=soft` selects the path; a transparent window is opaque, with
   a report. Add ", or set GOGUI_RENDERER=soft" to the Windows error text.
6. Give `glrendertest` its Windows selections (see Tests).
7. **Phase 3, done in #1338.** Add the process renderer, the first-window
   fallback and its log line to `New`. Move `applyVSync` after the binding, and
   collect each attempt's degradation reports for `New` to publish.
8. **Phase 3, done in #1338.** Run the CI showcase smoke test twice with the
   variable unset: with no Mesa DLL (the real no-GPU start, which must stay up),
   and with Mesa (which must stay on GL). The run with Mesa keeps the old test
   of a start (alive after 10 s, or a zero exit); the run without Mesa must be
   alive at the 10 s deadline, and an earlier exit fails it whatever its status,
   with the log and the status printed. Each reads the app's log: the run
   without Mesa requires the fallback line, the run with Mesa its absence.
9. **Phase 3, done in #1338.** Before phase 3 merges, run the showcase and the
   benchmark on a machine without a vendor driver. Record in the pull request:
   the fallback line, a live resize, minimise and restore, and the acceptance
   figures.
10. **Phase 4, done in #1344.** Create a transparent, frameless soft window with
    `WS_EX_LAYERED`, skip `enableWindowTransparency`, and present it with
    `UpdateLayeredWindow` from the memory DC, then flush. Raise alpha 0 to 1 on
    that path. Make the client area the whole window before the first frame.
11. **Phase 5.** Run the matrix under Tests and record the numbers here. File
    follow-up issues from what they show (swizzle, HiDPI, timer).

Docs, in the phase that changes the behaviour: `docs/deployment.md` beside the
Linux paragraph, one sentence in `README.md`, and the doc comments that name the
platforms (`gpu_error.go`, `gui/backend/soft/doc.go`,
`gui/backend/soft/renderer.go`). `CHANGELOG.md` gets `### Fixed` in phase 1 and
`### Added` in phases 2, 3 and 4.

## Tests

**Selections.** Before phase 2, `glrendertest` ran one list per OS, on Windows
`commonTests` plus `win32Tests`, with Mesa's DLL beside the binary and
`GOGUI_RENDERER=gl` forced for every test (`startTest`). Phase 2 gives it three
selections on Windows, chosen with `-select` (`mesa-gl`, the default, `soft`,
`system-gl`), each one process per test as before:

| Selection   | Mesa DLL beside the binary | `GOGUI_RENDERER`           | Runs                                                 |
| ----------- | -------------------------- | -------------------------- | ---------------------------------------------------- |
| Mesa GL     | yes                        | `gl`, or unset by the test | today's list, plus the tests that need GL to come up |
| forced soft | yes                        | `soft`                     | the soft present tests                               |
| system GL   | no                         | unset or `gl`, by the test | phase 1's regression test, then the fallback tests   |

`glrendertest` reports a skip as a failure, and the regression test skips when
its context is refused (until #1348 it retried once, for the refused first
context), so the system GL selection also shows what the runner's own OpenGL
does.

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
  resizes, and a failed DIB creation or selection keeps the last frame (bitmap,
  size and bits unchanged, `WM_PAINT` still shows it, no object leaked), and the
  next present allocates again. `TestSoftWin32OpacityAfterPaint`: a fade after
  the first frame keeps that frame, and a later, different frame reaches the
  layered window. `TestSoftWin32PresentAllocs`: a steady-state present makes no
  allocation. In a plain `go test` run, where an earlier test may have loaded
  `opengl32.dll`, `TestSoftWin32LoadsNoGL` reruns itself alone in a subprocess.
- **Phase 3, fallback.** Each trigger is forced: `TestFallbackOnRefusedContext`
  through the `createContextFunc` seam, `TestFallbackOnGDIGeneric` with the
  pixel format of phase 1's test. Both require a soft window and one log line.
  `TestPipelineFailureDoesNotFallBack`, `TestRendererGLReportsError` (with `gl`,
  no fallback), `TestFallbackSoftFailure`, `TestFallbackKeepsHiddenStart`,
  `TestFallbackKeepsQueuedPlacement`, `TestFallbackRunsOnInitOnce`,
  `TestFallbackPublishesWinningReportsOnly`. `TestSoftProcessMakesNoGLAttempt`:
  after a fallback, a second window is on soft and the seam is never called.
  `TestPipelineFailureDoesNotFallBack` needs a GL that reaches the pipelines, so
  it runs in the Mesa GL selection; the others run in the system GL one.
  `TestFallbackPublishesWinningReportsOnly` opens its window in a subprocess and
  reads the `gui.Debug` reports from its stderr, the only place they go; the GL
  attempt's transparency and opacity reports come from the
  `enableWindowTransparencyFunc` and `applyWindowOpacityFunc` seams, so each is
  one only the discarded attempt can make. The soft setup fails through the
  `newSoftPresentWin32Func` seam. Each test resets the process renderer, so they
  also pass in one process.
- **Phase 3, Mesa GL.** `TestUnsetWithMesaKeepsGL`: the window is on GL and a
  custom shader's fill is in the read-back pixels. Its `New` is the process's
  first, the automatic attempt with no `opengl32.dll` loaded before it, where
  phase 2 had a Mesa beside the exe get no context, and the system DLL did until
  #1348; in a plain `go test` run where an earlier test loaded the DLL it reruns
  itself alone in a subprocess. With Mesa beside the binary the loaded DLL must
  be that one and the context Mesa's, and with `GOGUI_REQUIRE_GL` set Mesa must
  be there; elsewhere it may be the machine's own GL, and a machine whose GL
  does not come up skips it. Like the other GL drawing tests it is not built
  with cgo. `TestLaterWindowGLFailureReturnsError`: no soft window opens.
  `TestWindowFromAppearanceHook`: a first window that follows the system
  appearance opens a second window from its `OnSystemAppearance` hook, inside
  `New`; the second must open, on the first window's renderer, within 30 s, once
  on the machine's own outcome (GL in this selection) and once after a forced
  fallback. On GL, the first window's transparent texture must be a texture in
  its own context: the hook's window leaves its context current, and the texture
  was once created after the attach, in that context. `TestWindowFromLogWriter`:
  a first window with an invalid `IconPNG` logs a warning, and a log writer
  opens a second window from that write; the second finds the first deciding,
  opens on GL without choosing, and the first then chooses GL. It checks first,
  with `GOGUI_RENDERER=gl`, that GL comes up, since a second window that fell
  back would log from inside the write, where the log package holds its own
  lock. A New that blocks in either test ends the test binary with the reason
  and every goroutine's stack, since the stuck goroutine would also keep the
  test from ending.
- **Phase 4.** Pure: `TestLayeredAlphaFloor` (0 becomes 1, 2 stays 2, in both
  halves of a word and in the odd pixel) and
  `TestSwizzleBGRALayeredMatchesByteLoop` (a byte loop, on seeded noise and on
  every pair of alphas). Forced soft: `TestSoftWin32LayeredWindow`, for both
  frameless styles: `WS_EX_LAYERED`, a present through `UpdateLayeredWindow`,
  the stored frame equal to `RenderToImage`'s with alpha 0 raised to 1, and the
  window rect equal to the client rect and the frame after a resize, a maximize
  and a restore; a minimized window skips its frame without a log line, and so
  does one frame of another size than the window's, while a second in a row is
  logged and neither resizes the window. A third case, resizable at 8×80, checks
  that the window is the size asked for. `TestSoftWin32LayeredShowsDesktop`
  reads the screen over an opaque backdrop window: the margin shows the
  backdrop, the card the blend, and the margin hit-tests to the layered window,
  and to the backdrop once its alpha is set back to 0; once shown at the start,
  once hidden at the start and shown with `Show` after its frame. It samples the
  margin at half the physical inset, so it holds at any device scale (passed at
  0.5, 1 and 1.5). `TestWin32ResizeBeforeRun`: a window resized between `New`
  and the loop draws its first frame at the window's size, and its logical size
  follows: a layered window in `Run` and in `RunApp` (resized inside `New`
  through the present seam), and a GL window in `Run`. `OnEvent` gets the
  `EventResized` after `OnInit`, in `RunApp` with an app and a platform ID, and
  with the real size (all three cases failed on the first version, which sent it
  before `OnInit`). It is in the Mesa GL selection.
  `TestSoftWin32TransparentFramedIsOpaque`: with a system frame, no
  `WS_EX_LAYERED`, the pixels of the blit and one report.
  `TestSoftWin32PresentAllocs` runs on a layered window too. In the system GL
  selection `TestFallbackPublishesWinningReportsOnly` also falls back with a
  frameless window: it is layered and gets no transparency report. Each of these
  live tests passed 20 times in one process and 20 times one process per test on
  the workstation.
- **#1348, system GL.** `TestFirstGLAttemptOfProcess` runs in a process with no
  `opengl32.dll` loaded (in a plain `go test` run, alone in a subprocess). An
  `opengl32.dll` must be mapped when `New` reaches the context step, before the
  first pixel-format call, checked there with `GetModuleHandleW`, which loads
  nothing; before #1348 the system DLL was not, on any machine. Then two GL
  attempts with `GOGUI_RENDERER=gl` must end alike: both up, or both failing at
  the same step, and the log names both outcomes. The lab VM and the VirtualBox
  guest showed the difference; a workstation with an AMD driver got a context at
  either attempt. `TestGDIGenericIsGPUContextError` and
  `TestFallbackOnGDIGeneric` no longer retry a refused first context.
  `TestOpenGL32LoadFailure` sets the recorded load error: the first window falls
  back with one line, `GOGUI_RENDERER=gl` returns `ErrNoGPUContext`, and neither
  reaches the context step.

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
- A transparent window on soft is transparent only when it is frameless. With a
  system frame it is opaque, and black where the frame is clear: the clear color
  is `ColorTransparent`, premultiplied (0, 0, 0, 0), which an opaque window
  shows as black.
- The alpha floor darkens what is behind a fully transparent pixel by 1/255.
- Pre-existing, not changed: a resizable frameless window that is not layered
  (on GL, or opaque on soft) starts with its resize border as non-client area,
  because the first `WM_NCCALCSIZE` reaches `DefWindowProc`, and its first
  resize after `Run` makes the client area the whole window: 14 px wider and
  higher at 100 % (measured on soft; GL routes the message the same way).
- The GL window class has `CS_OWNDC`, and `applyWindowOpacity` sets
  `WS_EX_LAYERED` on it, which the style documentation excludes. Pre-existing,
  not changed here: #1316.

## Not verified

- The phase 3 fallback on a machine without a GPU driver. Both triggers were
  forced through seams on the workstation, where the phase 3 tests pass one
  process per test and all in one process, and the three glrendertest selections
  pass with Mesa 26.3.0 beside the binary. Not run: the lab VM (step 9) and the
  CI runner, whose showcase smoke test without Mesa is the first real start on
  the runner's own OpenGL.
- The soft path beyond two machines on one Windows version (Windows 11 LTSC
  2024, build 26100): a workstation with an AMD driver, and the lab VM in its
  console session and in an RDP session. On both, the phase 2 tests pass one
  process per test, as glrendertest runs them, and all in one process, and an
  example app draws, resizes, minimises, restores and closes with
  `GOGUI_RENDERER=soft` (`examples/get_started` on the workstation, the showcase
  on the lab VM). On the lab VM no OpenGL module is loaded and an idle window
  uses no CPU time. On the workstation the three glrendertest selections pass
  with Mesa 26.3.0 beside the binary. Not run: the CI runner.
- Why the lab VM refused the first context and granted the second is known since
  #1348: the system DLL was loaded late. With #1348 the system GL and soft
  selections pass on the lab VM (console and RDP) and in the VirtualBox guest
  (console), where the GL render selection also passes on the guest's own OpenGL
  4.1. On the CI runner the first attempt gets GDI Generic 1.1.0 and fails at
  the binding, as the second does (the system GL selection of #1348's run). On
  `main`, a Windows 10 22H2 VM and a Windows Server 2025 RD Session Host refused
  the first context the same way, on the console and over RDP, and a probe that
  loaded the DLL first reached the binding at the first attempt there too. Not
  verified: #1348 itself on those two, an RDP session into the VirtualBox guest,
  and why an AMD driver is not affected.
- Every "expected" cell of the environment table. Windows Sandbox with vGPU and
  Hyper-V with GPU-P have no row: unknown.
- What a running GL window does when a session connects or disconnects.
- That a window DC reads back the presented pixels on the CI runner, for a
  layered window too. The destination-pixel and opacity tests depend on it. It
  does on the workstation above.
- That checking the monitor handle at every present and re-reading on
  `WM_DISPLAYCHANGE` catches every change of display frequency. Only one monitor
  was tested.
- `BitBlt` cost above 1920×1080. Cost in a VM and over RDP beyond the one
  benchmark view under Cost; the acceptance figures of decision 7 are phase 3's
  to record. Mesa llvmpipe against soft on Windows; the 2.5–3× figure is
  Linux's.
- The sleep overshoot comes from the design review and was not repeated. The
  alpha hit-test was repeated in phase 4 for alpha 0 and 1 (`WindowFromPoint`,
  and a real click in a throwaway app); alpha 2 and 255 only in the design
  review.
- Phase 4 beyond the workstation: transparent frameless windows were tested
  there only, at 100 % scaling on one monitor, with the phase 4 tests and a
  throwaway app (see the pull request): the desktop shows through and the card
  blends, a click in the clear margin reaches the window, a hidden start, a
  minimize and restore, a maximize and a live resize keep the frame on screen,
  and the same app with a system frame is opaque. Not run: the lab VM, an RDP
  session, the CI runner, a second monitor or another scale.
- Phase 4 settles that `UpdateLayeredWindow`'s bitmap covers the whole window
  rect, non-client area included: the `psize` it is given becomes the window's
  size (measured). A layered soft window has no non-client area.
