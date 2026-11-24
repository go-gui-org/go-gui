# go-webview: web content in a Go-Gui window

Issue #1179. Status: **planned** (design for review, no code).

## Problem

Some apps must show real web content inside a Go-Gui panel: HTML with CSS,
tables, video, iframes and JavaScript. Examples:

- the preview for an HTML editor that a mobile app renders
- an HTML e-mail or report
- a dashboard or a documentation page that exists only as a web page.

`Window.HTML` (#1158) does not cover these cases. By design it is a safe subset
with no CSS, tables, video or iframes (`docs/specs/html-subset.md`).

OAuth login is a different case, and it already has an answer. Google and other
providers block OAuth requests from embedded web views. The sanctioned flow is
the system browser, PKCE and a loopback redirect, opened with
`NativePlatform.OpenURI`. This spec does not change that flow.

## Goals

- A separate module, `go-gui-org/go-webview`, in the same family as go-term and
  go-map. Only apps that import it carry web engine code.
- The system engine on each platform: WKWebView on macOS, WebView2 on Windows.
- No cgo. The module uses purego on macOS and `syscall` on Windows, the same as
  the Go-Gui backends.
- Go-Gui menus, dialogs, toasts and tooltips stay visible over a web view.
- Go-Gui core gets only two small, optional additions. Core gets no new
  dependency.

## Non-goals

- Linux in v1. See [Platforms](#platforms).
- A bundled engine (CEF, Chromium).
- OAuth inside a web view.

## Constraints

These facts from the current source decide the design.

1. **Go-Gui draws all layers into one GPU surface.** `layoutArrange` lifts the
   floats out of the tree. Then it adds the toast layer and the dialog layer on
   top (`gui/layout_arrange.go:45-80`). `composeLayout` puts all layers under
   one root, and `buildRenderers` walks that root into one renderer list
   (`gui/window_update.go:485-516`). A native child view sits above this
   surface. Thus it covers every Go-Gui popup.
2. **No backend exposes a native handle.** The macOS window is the unexported
   `macWindow` (`gui/backend/metal/window_darwin.go:89-93`). The Win32 `hwnd` is
   in the unexported `platformState` (`gui/backend/gl/platform_win32.go:218`).
   `NativePlatform` (`gui/native_platform.go:165-187`) has no method that
   returns a handle or attaches a child view.
3. **`AmendLayout` runs too early to place a native view.** It sees the final X,
   Y, width and height. But the layout and hero transitions run after it, and
   the shape clips are set after it (`gui/layout_pipeline.go:59-65`). Floats get
   their positions later, in `layoutArrange` (`gui/window_update.go:413`).
4. **Sibling modules use only the public `gui` API.** go-term and go-map draw
   with `gui.DrawCanvas` and import nothing from `gui/backend`. A sibling cannot
   import `gui/internal/darwinffi`, so go-webview brings its own purego and objc
   bindings.
5. **A streaming image path exists.** `gui.PixelSurface`
   (`gui/pixel_surface.go:88`) takes `Update(pix, stride)` each frame and has a
   DMA-BUF zero-copy path (`gui/pixel_surface_gpu.go:86`).
6. **Go-Gui owns raw X11 and Wayland windows on Linux, not GTK windows.**
   WebKitGTK needs a GTK parent. It cannot attach to a foreign Wayland surface.

## Prior art: gowt

gowt (`github.com/haiodo/gowt`, package `webview`) embeds WKWebView, WebKitGTK
and WebView2 with no cgo:

- macOS: purego and `objc_msgSend`, `addSubview:` into the host NSView.
- Linux: purego `dlopen` of WebKitGTK 4.1 and GTK 3.
- Windows: `syscall` only, COM vtables built by hand. It finds the Evergreen
  runtime through the registry, so it needs no `WebView2Loader.dll`.

gowt attaches a native child view and lets the OS composite it. That works in
gowt because gowt is a port of SWT. Its menus and popups are native windows, and
the OS stacks them above the child view. Go-Gui draws its popups itself
(Constraint 1). gowt has no code for that problem.

gowt is licensed EPL-2.0. See [Clean-room rule](#clean-room-rule).

## Candidate designs

| #   | Design                                                                                      | Go-Gui surface added                                          | Fidelity and integration                                                                                                                           | Failure when misused                                                          |
| --- | ------------------------------------------------------------------------------------------- | ------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| A   | Native child view over the Go-Gui surface (WKWebView subview, WebView2 windowed controller) | Optional `NativeChildHost` interface and a placement callback | Full engine. IME, accessibility and scrolling are native. Covers Go-Gui popups unless it hides itself for them                                     | A popup opens under the web view. The view lags one frame behind an animation |
| B   | Offscreen engine renders into a `PixelSurface`. Input goes to the engine, as in go-term     | None                                                          | Z-order, clipping and scrolling are exact. macOS has no public offscreen WKWebView API. Input, IME and accessibility must all be forwarded by hand | Missing input paths. A per-frame copy cost where no zero-copy path exists     |
| C   | A separate top-level native window (the glaze spike in #1179)                               | None                                                          | Full engine, but not part of the layout                                                                                                            | Move, minimize and focus sync is manual and drifts                            |

## Decision

**Design A, in the separate `go-webview` repo, macOS and Windows first.**

- Only A gives full fidelity on macOS. gowt shows that A works with no cgo on
  both platforms.
- The popup problem has a solution inside Go-Gui. Each frame, Go-Gui knows the
  rectangle of every float, dialog, menu and toast. When one of them overlaps a
  web view, the web view hides its native view and Go-Gui draws a snapshot
  instead. See [Occlusion](#occlusion).
- B is kept as the candidate for Linux.

Decided 2026-11-24:

- Linux is deferred. v1 is macOS and Windows.
- The occlusion policy is a snapshot swap, not a plain hide.
- Core gets a narrow attach and detach interface, not a raw handle getter.
- The repo name is `go-webview`. The clean-room rule applies.

## Go-Gui core changes (draft)

The names and signatures below are a sketch for review. The Go-Gui PR fixes
them, with the usual design table.

### 1. `NativeChildHost`

This is an optional interface. A backend implements it, and go-webview finds it
with a type assertion on `w.NativePlatformBackend()`. Tests and the web backend
do not implement it, so go-webview reports "not supported".

```go
// NativeChildHost attaches native child views to the window's content view.
type NativeChildHost interface {
	// NativeKind names the platform: "cocoa" or "win32".
	NativeKind() string
	// AttachChild adds a native view (NSView* or HWND) above the content
	// view and returns an ID for later calls.
	AttachChild(view uintptr) (ChildID, error)
	// PlaceChild sets the frame and the clip in window logical units.
	// The backend converts to points or physical pixels.
	PlaceChild(id ChildID, frame, clip Rect, visible bool)
	DetachChild(id ChildID)
}
```

The interface gives no access to the `NSWindow` or the top-level `HWND`. The
module can add and move a child. It cannot change window state behind the
backend.

**Clipping.** A web view inside a scroll container is often partly clipped. The
backend puts each child into a clip container: an `NSView` with `masksToBounds`
on macOS, and a child `HWND` on Windows. The container takes the clip rectangle.
The web view takes the full frame, offset inside the container. Thus the web
view scrolls and clips like a Go-Gui view.

### 2. Placement callback

A widget registers a callback that runs once per frame, after `composeLayout`
(`gui/window_update.go:418`). At that point the transitions, the clips and the
float positions are final.

```go
// Placement is the final geometry of a shape for this frame.
type Placement struct {
	Frame    Rect // after transitions, window coordinates
	Clip     Rect // intersection of all ancestor clips
	Visible  bool // false if the shape is not in the tree this frame
	Occluded bool // a float, dialog, menu, toast or tooltip overlaps Clip
}
```

The callback runs under `w.mu`. It must not call `SetFocus`, `SetView` or other
window-mutating APIs (the frame lock rule in `CLAUDE.md`). `PlaceChild` is a
backend call, not a window call, so the callback can use it.

When a shape leaves the tree, the callback runs one last time with
`Visible: false`. Thus the module hides the native view in the same frame.

## Occlusion

A native view is always above the Go-Gui surface. The policy is a snapshot swap:

1. The web view shows its live native view while `Occluded` is false.
2. When `Occluded` becomes true, the module hides the native view. In the same
   frame, the widget draws the last snapshot as a `PixelSurface` image. The
   Go-Gui popup draws over the image in the usual way.
3. When `Occluded` becomes false, the module shows the native view again.

The snapshot APIs are asynchronous: `takeSnapshotWithConfiguration:` on
WKWebView and `CapturePreview` on WebView2. Thus the module cannot take the
snapshot at the moment that the popup opens. The module keeps a recent snapshot
instead. It refreshes the snapshot after each navigation finishes and when the
page goes idle. Then an occlusion shows a slightly old image, but never an empty
area. Video and animation stop moving while a popup is open. This result is
acceptable for v1.

## Module API (go-webview)

The API is a sketch. It follows the feature set that gowt shows to be possible.

| Function              | Purpose                                                                   |
| --------------------- | ------------------------------------------------------------------------- |
| `webview.View(cfg)`   | Factory. Returns a `gui.View`. Takes `ID`, sizing and the callbacks below |
| `Navigate(url)`       | Load a URL                                                                |
| `SetHTML(html)`       | Load an HTML string                                                       |
| `Eval(js, cb)`        | Run JavaScript. `cb` gets the result as JSON text                         |
| `OnMessage`           | Gets strings that the page sends with `window.gogui.postMessage`          |
| `HandleScheme(s, fn)` | Serve a custom scheme (`app://`) from Go. Set before the first load       |
| `OnNavigation*`       | Started, finished and failed events                                       |

Engine callbacks arrive on the UI thread. The module sends them to the app with
`w.QueueCommand`, so app code runs outside the frame lock.

## Platforms

| Platform    | Engine             | Bindings                        | v1       |
| ----------- | ------------------ | ------------------------------- | -------- |
| macOS arm64 | WKWebView          | purego, objc runtime            | yes      |
| Windows     | WebView2 Evergreen | `syscall`, COM vtables, no DLL  | yes      |
| Linux       | WPE WebKit         | purego, DMA-BUF to PixelSurface | deferred |
| web, mobile | —                  | —                               | no       |

**Linux.** gowt uses WebKitGTK, and that path does not transfer (Constraint 6).
X11 has XEmbed, but only with GTK 3, and Wayland has no equivalent. WPE WebKit
renders offscreen into DMA-BUF buffers. These buffers can go straight into the
`PixelSurface` DMA-BUF path, so Linux uses design B. The cost is input, IME and
accessibility forwarding. A later issue covers a WPE spike. If the spike fails,
Linux falls back to design C.

**DPI.** macOS uses points, so the backend passes logical units through.
WebView2 uses physical pixels. The Win32 backend multiplies by the window scale,
and it sets `RasterizationScale` again when the DPI changes. gowt has no
DPI-change code, so the module must test this case.

**Focus.** On Windows, the web view gets focus with `MoveFocus` when the Go-Gui
widget gets focus. A `MoveFocusRequested` event from WebView2 moves Go-Gui focus
to the next or previous focusable. On macOS, the responder chain does this.

## Clean-room rule

gowt is EPL-2.0. EPL is a weak copyleft license that applies per file. A file
copied or translated from gowt keeps the EPL obligations, also inside a separate
module. Thus:

- Contributors can read gowt to learn the approach and to find which platform
  APIs it calls.
- Contributors do not copy or translate gowt files, for example the COM vtables
  or the objc delegate class.
- Code is written from the Apple and Microsoft documentation.
- A PR that adds platform bindings says which documentation it used.

## Phasing

| Phase | Work                                                                                                                 | Repo       | Status   |
| ----- | -------------------------------------------------------------------------------------------------------------------- | ---------- | -------- |
| 1     | Spike: WKWebView as a subview of `MetalContentView`, placed from `AmendLayout`. Proves attach, resize, focus, Retina | scratch    | pending  |
| 2     | `NativeChildHost` and the placement callback (metal and Win32 backends), tests with a nil platform, CHANGELOG        | go-gui     | pending  |
| 3     | go-webview on macOS: View, Navigate, SetHTML, Eval, OnMessage, HandleScheme, snapshot swap                           | go-webview | pending  |
| 4     | go-webview on Windows (x64 and arm64)                                                                                | go-webview | pending  |
| 5     | Docs: go-webview README and example, Go-Gui `README.md` sibling entry, wiki                                          | both       | pending  |
| 6     | Linux: WPE spike (separate issue)                                                                                    | go-webview | deferred |

## Test plan

- Go-Gui: unit tests for the placement callback with a nil platform. Cover the
  clip of a scrolled parent, a layout transition, a float that overlaps, and a
  shape that leaves the tree. The golden tests do not change.
- go-webview: an example with a web view under a `Select` dropdown and a dialog.
  The popup must stay visible. The web view must follow scroll and resize. Run
  it by hand on macOS arm64, Windows x64 and Windows arm64.

## Rejected Approaches

- **CEF or Chromium offscreen.** It adds 100 MB or more and needs cgo. It breaks
  the cgo-free rule.
- **A web view inside Go-Gui core.** Every app would carry engine code and
  platform risk. The sibling module model (go-term, go-map) applies.
- **A public getter for the raw `NSWindow` or `HWND`.** Apps can change window
  state behind the backend. The narrow attach and detach interface prevents
  this.
- **Port of the gowt webview files.** EPL-2.0. See
  [Clean-room rule](#clean-room-rule).
- **WebKitGTK on Linux.** It needs a GTK parent. Go-Gui windows are raw X11 and
  Wayland windows.
- **Plain hide on occlusion.** The web view area goes empty while a popup is
  open. The snapshot swap costs little more.
- **System browser and loopback as the full answer.** It covers OAuth, not
  panels. It stays the OAuth path.
- **`AmendLayout` as the placement hook.** It runs before the transitions and
  the clips (Constraint 3).

## Open questions

1. Snapshot refresh: is "after navigation and on idle" enough, or must the
   module refresh on a timer for pages that change?
2. Does `PlaceChild` take a clip rectangle, or a list of rectangles for a view
   that a popup covers only in part? v1 uses one rectangle and the snapshot
   swap.
3. Accessibility: does the native web view join the Go-Gui accessibility tree,
   or stay a separate native element?
4. Can the Go-Gui 1.0 surface carry `NativeChildHost`, or does it stay marked
   experimental until Windows ships?
