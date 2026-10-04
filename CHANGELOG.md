# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Showcase has a DX Cheat Sheet page** — the cheat sheet in
  `docs/dx-cheat-sheet.md` lists the places where the obvious reading of the API
  is wrong, but people who use the showcase did not see it. It is now under
  **Welcome → DX Cheat Sheet**. The showcase embeds a copy, because `go:embed`
  cannot read outside its package. `TestDocMirrors` fails when the copy and
  `docs/` differ; to update the copy, run
  `go test ./examples/showcase/ -run TestDocMirrors -update`.

- **`LoadSecret` / `SaveSecret` / `DeleteSecret` keep secrets in the OS
  credential store (#920)** — `SaveSettings` writes plain JSON, so an app that
  kept an access token had to write its own store. The new functions use the
  Keychain on macOS and iOS, Credential Manager on Windows, and the Secret
  Service (GNOME Keyring, KWallet) on Linux, with the app ID as the service
  name. A value is 1 to 2560 bytes on every platform. Where no store exists
  (web, Android, a Linux session with no Secret Service daemon) they return
  `ErrSecretsUnsupported` and never fall back to a plain file. A missing key
  returns `ErrSecretNotFound`. `NewTestWindow` uses a memory store. See
  `docs/specs/secure-storage.md`.

- **Experimental native Wayland backend behind `GOGUI_WAYLAND=1` (#919)** — on a
  Wayland desktop go-gui ran only through XWayland, which blurs at fractional
  scales and is unavailable on devices with no X11 EGL platform (#916). With
  `GOGUI_WAYLAND=1` a Linux app (amd64 or arm64) opens native Wayland windows.
  They render with the same GL renderer at fractional and integer scales, take
  pointer, keyboard, scroll, touch and input method (text-input-v3) input, share
  the clipboard and primary selection, set cursors, and move and resize. The
  frame comes from the compositor (KDE, wlroots) or from libdecor (GNOME,
  Cinnamon, weston), and a warning names the missing package when neither can
  draw one. Without the variable nothing changes. If Wayland setup fails (no
  compositor, or no libwayland-client, libwayland-egl or libxkbcommon), go-gui
  prints a warning and uses X11. The build stays cgo-free: the libraries are
  opened at run time. Tested under sway, weston, KWin and Cinnamon 6.6; reports
  from other desktops and GPUs are wanted on #919. See
  `docs/specs/wayland-backend.md`.

- **`DialogCfg.EscapeDisabled` stops Escape from closing a dialog (#909)** — a
  progress dialog had no way to block the Escape path, so the user could close
  it before the work finished. Set `EscapeDisabled` to keep the dialog open. A
  blocked Escape stays unhandled, so content in the dialog can still use the
  key. `DialogDismiss()` still closes the dialog, and `OnCancelNo` does not fire
  for a blocked Escape.

- **`DrawContext.Rotate` turns canvas drawing by an angle (#904)** — the canvas
  transform stack (`Translate` / `ScaleBy` / `Save` / `Restore`) was scale and
  translate only, so a rotated shape, text or image could not be drawn; the
  qcpainterbench Flower port now draws its cached outline under a
  rotate-about-center instead of rotating 193 points on the CPU every frame. The
  transform is now the full 2×3 affine, composed in call order (positive angles
  turn clockwise, as in `TextStyle.RotationRadians`), and geometry still rides
  unbaked: batches and `RenderCmd` carry all six floats (`XformXY` / `XformYX`
  join the existing fields) for every backend to apply per vertex. Text maps its
  anchor through the matrix and gains the angle on `RotationRadians` (or a
  composed `AffineTransform`, which keeps precedence); images record their
  rotated frame for a backend rotation bracket; the lowered radial-quad path
  survives rotation (a circle stays a circle) and still falls back to the ring
  mesh under a non-uniform scale. `DrawRecorder` exporters see baked coordinates
  with no interface change: rotated rects arrive as outline polygons, rotated
  ellipses flatten at the arc tolerance. `DrawCanvasTriBatch.Transform` keeps
  its shape but returns `ok=false` for a rotated batch, whose vertices are still
  in local space; a caller that reads `ok=false` as "already in canvas space"
  misplaces rotated geometry. Move `Transform` callers to the new
  `TransformAffine`, whose `ok=false` means untransformed only.

- **`WindowCfg.VSyncOff` presents without waiting for vsync (#907)** — a
  renderer benchmark that redraws every frame read the display refresh rate
  until a frame took longer than one refresh, so it could not report raw
  throughput. The flag is opt-in and fixed when the window is made; the default
  keeps vsync on. X11 sets EGL swap interval 0 and Windows sets WGL swap
  interval 0. macOS, iOS, Android and web ignore the flag: on macOS the window
  compositor hands back a Metal drawable once per refresh even with the layer's
  display sync off, in a window and in full screen. An idle window still sleeps,
  and animations still tick every 16 ms: frames run past the refresh rate only
  when the app invalidates every frame. `gui.Debug` reports a refused or ignored
  flag under `DebugWindowDegraded`, through the new `Window.DebugWindowVSync`.
  `examples/qcpainterbench` takes `-novsync`.
- **`examples/qcpainterbench`, a canvas benchmark ported from Qt (#723)** — it
  draws the six workloads of Qt's qcpainterbench (ruler, circles, bezier lines,
  bars, icons and text, flower) through `DrawCanvas`, N times per frame. It
  shows FPS and go-gui's CPU time per frame. `-sweep 1,2,4,…,512` prints one CSV
  row per render count. No Qt code is copied: the workloads follow the behavior
  written in `docs/specs/qcpainterbench.md`. Vsync stays on, so FPS stops at the
  display refresh rate; round caps ride `StrokeStyle` since #905, and concave
  path fill is emulated in the example and tracked in #906 (rotation since #904
  rides `DrawContext.Rotate`).

- **Canvas strokes take caps and joins through `StrokeStyle` (#905)** — `Line`,
  `Polyline`, `PolylineJoined`, `Arc` and the beziers (plus `Circle` and
  `RoundedRect`) each gain a `*Styled` twin that draws round and square caps and
  round joins on top of the one stroker `gui/svg` shares. The zero style draws
  exactly what the unstyled call draws, so existing code sees no change in
  pixels, triangle counts or recorder output. A recorder that wants the style
  implements the optional `DrawStrokeRecorder` extension; all others still get
  the stroke as the equivalent unstyled primitive. The qcpainterbench Circles
  port drops its 34 half-disc fills per pass, and the curves draw with round
  joins like Qt.

- **Canvas paths fill concave shapes through `CanvasPath` (#906)** — a reusable
  path value built with `MoveTo` / `LineTo` / `QuadTo` / `CubicTo` / `ArcTo` /
  `Close`, drawn with `FillPath` (solid color), `FillPathGradient` (gradient)
  and `StrokePath` (color, width, `StrokeStyle`). Fills honor `FillNonzero` (the
  zero value, as in SVG) and `FillEvenOdd`, so holes carve and overlaps follow
  the rule instead of fanning from the first point the way `FilledPolygon` does.
  The tessellator is the one `gui/svg` shares, moved to `gui/internal/pathfill`;
  SVG output is unchanged. `Reset` rebuilds a path without allocating, and a
  redrawn canvas fills its paths with zero allocations. A recorder that wants
  the path implements the optional `DrawPathRecorder` extension; all others
  still get the fill as one flat polygon per triangle and the stroke as one
  joined polyline per contour. The qcpainterbench Lines port fills path cubics
  closed to the baseline and the Flower port fills its cached outline as one
  closed contour, dropping both hand-built triangulations.

### Fixed

- **Linux: no double scale under XWayland when `Xft.dpi` is unset (#918)** — on
  a Wayland desktop that does not publish `Xft.dpi` (Phosh), the X11 path took
  its scale from the monitor's physical DPI. The compositor also scales XWayland
  windows by the output scale, so the scale was applied twice: on a FuriLabs
  FLX1s at 174%, go-gui drew at 1.81 and the compositor enlarged that by 1.74,
  and at 100% go-gui was still about twice the size of other apps. Under
  XWayland with no `Xft.dpi`, the scale is now 1.0, the same as GTK and Qt, and
  the compositor applies the desktop scale. `Xft.dpi`, when set, still wins.
  Bare X11 sessions keep the per-monitor RandR scale.
- **Closing one window no longer blanks the others' GL resources (#921)** — each
  window owns an unshared GL context, and object names repeat across them, so
  destroying a window freed the surviving windows' same-numbered textures,
  buffers, VAOs and shader programs instead of its own. The window now makes its
  own context current before deleting its objects. Single-window apps were never
  affected.
- **Linux: GL starts when the default EGL driver has OpenGL ES only (#916)** —
  on a libhybris phone (FuriLabs, Phosh on Wayland) the default EGL display is
  the Android GPU driver, which has no desktop-OpenGL config, so every app
  failed with `eglChooseConfig: no matching config (egl error 0x3000)`. When the
  default display has no desktop-GL config, the backend now tries the Mesa X11
  platform display (`eglGetPlatformDisplay`). On that phone Mesa renders in
  software (llvmpipe), so the app runs but is slow. When no display has desktop
  GL, the error now says the driver offers OpenGL ES only, in place of only the
  old "update the GPU driver" hint. Desktop systems are not affected: their
  default display succeeds and the fallback never runs.

- **macOS: event loop no longer leaks one `NSEvent` per frame** — the Metal
  backend polled AppKit events with no autorelease pool, and the Go event loop
  has no Cocoa run loop to drain one. Every dequeued event and every wake event
  stayed in memory, so an app with a repeating animation (the `solar_system`
  example) grew by about 60 events, ~40 KB, a second for as long as it ran. The
  poll now drains its own pool, and memory stays flat.

- **SVG round joins and square caps draw on the correct side (#905)** — the
  round-join fan swept the long way around the vertex, painting the inner disc
  the segment quads already cover and leaving the outer corner open, and the
  square cap extended sideways along the normal instead of along the path.
  Moving the stroker into `gui` exposed both, confirmed with coverage probes
  before the fix. Round caps, miter joins and bevel joins were already correct
  and draw byte-identical output.

## [v0.84.0] - 2026-10-01

### Added

- **`buildapp` compiles, images and signs for distribution (#852)** — four new
  steps on the one tool, so release scripts stop hand-running them: `-build-pkg`
  compiles the package for `-platform`/`-arch` first (`-H windowsgui` is added
  on Windows automatically, `CGO_ENABLED=0` on Linux/Windows); `-dmg` wraps the
  macOS `.app` in a UDZO disk image; `-entitlements` switches `codesign` to the
  hardened-runtime form; `-notarize` submits the artefact with `notarytool` from
  a keychain profile (`-notary-profile`, never flags) and staples the ticket.
  `-notarize` without `-entitlements` is an error. `make release` uses `-dmg` in
  place of the hand-run `hdiutil` step, and the `.dmg` takes the same
  slug-version name form as the `.zip` and `.tar.gz`. Windows Authenticode
  signing stays out of scope in its own issue.

### Fixed

- **Ctrl+C / Ctrl+V / Ctrl+A no longer type their letter on Linux (#896)** — the
  X11 backend sent a character event with the plain letter after every key
  press, Ctrl and Super chords included, while `Input` drops only control
  characters (what Win32 delivers for them). The shortcut ran on key down and
  the letter was then inserted: copy looked like a cut leaving `c`, paste
  appended `v`, select all replaced the text with `a`. Ctrl and Super chords now
  produce a key down only, on the direct path and on keys an IBus engine
  forwards. Ctrl+Alt still types, for layouts that use it as AltGr. A chord also
  leaves a pending dead key alone: Ctrl+C after a dead acute used to type `ć`,
  and now the accent waits for the next plain key.
- **Linux frame rate no longer drops on pages with SVGs (#892)** — since v0.79.0
  the GL backend ran `gsettings get` for the reduce-motion setting on every
  call, and the render pass asks once per visible SVG per frame. The process
  spawns held the frame lock: the showcase SVG Spinner page fell from 60 to ~20
  fps and the Thinking Orb page to ~5 fps. The reading is now cached, read once
  up front and refreshed in the background at most every 2 s. A frame never
  waits on `gsettings` again, and a change to the desktop animation setting
  still takes effect within about 2 s.
- **GL backend draws a run of SVG meshes in one call (#895)** — on Linux and
  Windows every `RenderSvg` command paid its own pipeline bind, vertex upload
  and draw call through purego. A ThinkingOrb draws each dot as its own small
  mesh in its own color, so the showcase Thinking Orb page issued ~1,900 draws a
  frame and ran at ~40 fps on an Intel UHD 620. Consecutive SVG meshes now queue
  into one vertex buffer and draw together; any other command draws the queued
  run first, so paint order, clips, stencils, filters and rotations are
  unchanged. That page now issues 18 SVG draws a frame, and its backend draw
  time on the same GPU fell from ~18 ms to ~3 ms. Any SVG-heavy view (icons,
  charts, `DrawCanvas`) benefits the same way. A run is capped at the largest
  single command, so peak memory does not grow.
- **GitHub releases now ship with release notes (#893)** — publishing a tag
  extracts that version's `CHANGELOG.md` section as the release body and appends
  GitHub's auto-generated commit/PR list below it, so the release page shows
  highlights, fixes, breaking changes and migration steps instead of an empty
  body. The generated list is grouped by the `type/*` labels via
  `.github/release.yml`.

## [v0.83.0] - 2026-09-30

### Added

- **`InputGroup` joins form controls into one shape (#820)** — a Bootstrap-style
  input group: an `Input`, `Select`, `Button` or text addon segment side by
  side, with one rounded border around the group and a 1px divider at each seam.
  Build segments with `InputGroupText`, `InputGroupInput`, `InputGroupSelect`
  and `InputGroupButton`; each takes the widget's ordinary Cfg and removes its
  border, radius and focus glow, so a doubled seam cannot be built by mistake.
  Segments keep their own tab stops, and the group shows the focus border and
  ring while any segment holds focus. The group ID scopes the segment IDs
  (`price:amount`). Showcase page: Input Group.

- **Widgets work without an `ID` (#881)** — a widget whose `ID` is empty now
  takes a generated ID, such as `~input3`, from three parts: the nearest
  ancestor with an explicit ID, the widget kind, and the count of earlier
  widgets of that kind there. It joins Tab order, keeps its cursor, scroll
  offset and state, and needs no ID for a view whose structure does not change.
  This covers every input control, the composites (Table, Tree, VirtualList,
  Form, Combobox, DatePicker, ListBox, ColorPicker, Menubar, ContextMenu,
  CommandPalette, OverflowPanel, ExpandPanel), and a scrolling or overflowing
  container. These `ID` fields are now tagged `gui:"auto"`. A widget of another
  kind that appears does not move the key, and an explicit ID on a container
  keeps the count inside it from moving. Set an explicit ID when code names the
  widget (`SetFocus`, `FindByID`, tests), or when a widget of the same kind can
  appear before it. The new debug category `DebugAutoIDs`, in `DebugAll`,
  reports a focused widget whose generated key moved to another widget, and an
  app ID that starts with the reserved `~`. `InputGroupCfg.ID` and
  `DataGridCfg.ID` are still required. See `docs/specs/auto-widget-identity.md`.

### Changed

- **BREAKING: `TextButton` drops its `ID` parameter (#881)** —
  `TextButton(label, onClick)` and `TextButtonVariant(label, variant, onClick)`
  no longer take an `ID`; the button takes a generated ID like `Button` with an
  empty `ID` does. A button nobody names needs no identity. Migrate by deleting
  the first argument; a button named by code (`SetFocus`, `FindByID`, tests)
  becomes `Button(ButtonCfg{ID: id, Label: label, OnClick: onClick})` (or
  `Content` with a `Text` child for the plain form).

- **An empty widget `ID` no longer panics (#881)** — the factories that panicked
  with "requires a non-empty Cfg.ID" (and "with Scrollable:true" or "with
  Overflow:true") now give the widget a generated ID. Code that sets IDs does
  not change. The `requiredid` analyzer no longer reports a `Scrollable: true`
  literal without an ID. `RequireID` is still exported for widgets outside
  `gui`.

- **`ClipContents` clips to the area inside the border (#820)** — the stencil
  mask of a `ClipContents` container is now inset by its `SizeBorder`, with the
  radius less the border. Before, the mask used the outer edge, so a child's
  fill could paint over the container's border at the rounded corners. A
  container with no border is not affected.

- **GL backend reports a missing GPU context instead of panicking (#828)** —
  when EGL (Linux) or WGL (Windows) cannot create a context, as on Windows under
  RDP or in a VM with no GPU driver, `Run`/`RunApp` now print one line naming
  the failed step and what to try (update the GPU driver, leave RDP/VM, or
  install Mesa on Linux) and exit non-zero, instead of dumping a Go stack trace.
  The failure is typed as `gl.ErrNoGPUContext` (matched with `errors.Is`;
  `errors.Unwrap` gives the EGL/WGL cause). Other init failures still panic.

### Fixed

- **Two `ExpandPanel`s or `InputDate`s without an `ID` no longer share IDs
  (#881)** — both widgets name inner parts from their own ID. With the `ID`
  empty, every copy took the same bare inner ID: `"head"` for an `ExpandPanel`
  header, and `"input"` and `"calendar"` for an `InputDate` with
  `FocusDisabled`. The copies shared one tab stop and one state slot, and a key
  press reached only the first. Both widgets now take a generated ID when `ID`
  is empty, also with `FocusDisabled`.

- **`InputDate` with `FocusDisabled` keeps its calendar button out of Tab order
  (#881)** — before, only the text field opted out, and Tab still stopped on the
  calendar button. `FocusDisabled` now covers the whole control.

- **`NumericInput` without steppers keeps `Label` (#878)** — a `NumericInput`
  with the default `StepCfg` (no step buttons) no longer drops `Label`: it now
  stacks the visible label above the field and uses it as the accessible name,
  the same as the stepper path fixed in #876. Before, the early-return path
  rendered the inner field directly, so there was no label shape and no a11y
  node from `Label`.

- **Accessible names keep a colon in `Label` and display text (#876)** — a
  `Label`, `Placeholder`, menu item text, progress text, badge label or shown
  `Text` with a colon (`Price: USD`) no longer loses everything up to the last
  colon in the screen reader name. The fallback helper treated prose as an ID
  path and kept only the last scope segment; prose now falls back unchanged,
  while `cfg.ID` fallbacks still strip the scope.

- **Icon-only buttons no longer load a system fallback font (#872)** — optical
  centring of an icon-font label no longer measures the cap probe "H" in the
  icon face. The face has no "H", so the text stack loaded a system fallback
  font to measure it and kept it in memory: on Fedora that is Noto Sans CJK, and
  the live heap went from about 15 MB to 44 MB on the first frame. An icon glyph
  now centres on its own ink with no cap-band limit, which is also the correct
  position for a glyph.

- **Linux windows follow the desktop scale (`Xft.dpi`) (#871)** — the gl backend
  now takes its UI scale from `Xft.dpi` when the desktop sets it, and uses RandR
  physical DPI only when `Xft.dpi` is not set. Before, RandR came first. Under
  GNOME Wayland with fractional scaling, XWayland reports a 2x virtual screen
  with the real panel size, so RandR gave 2.62 where the desktop asked for 2.0,
  and go-gui windows were about 31% larger than GTK and Qt apps and than their
  own title bar. With `Xft.dpi` set, the scale is the same on every monitor, as
  in GTK on X11. Per-monitor rescaling now applies only to sessions without
  `Xft.dpi` (bare X). A HiDPI panel whose desktop leaves `Xft.dpi: 96` now draws
  at 1.0, where RandR gave about 2.0 before. This matches GTK and Qt on the same
  session. To get 2.0, set `Xft.dpi: 192` (the desktop's scale setting, or
  `xrdb`). An `Xft.dpi` outside 48–768 (scale 0.5–8) counts as unset.

## [v0.82.0] - 2026-09-29

### Added

- **`ergonomics-audit -mode spacing` flags gap and inset numbers (#851)** — a
  `Spacing: SomeF(n)` with n > 0, or a `PadAll` / `NewPadding` / `PadVH` built
  only from numbers, is now a finding. Use a theme step instead:
  `SpacingTight/Small/Medium/Large` (2/6/14/28) for a gap, and
  `PaddingSmall/Medium/Large` (6/14/22) or the `Pad*` constants for an inset. A
  `ButtonCfg.Padding` number is a finding with its own message: delete the
  field, and `Theme.PaddingButton` applies. `DrawCanvasCfg` padding is a plot
  margin and passes. A number that is not a gap (a 1px hairline, a bevel edge)
  takes the same-line marker `// ergonomics-audit:spacing` with a reason. In
  go-gui the mode scans `gui/view_*.go` and `examples/`. In any other repo it
  scans every non-test file, so a sibling can gate on it with
  `go run github.com/go-gui-org/go-gui/tools/ergonomics-audit@<tag> -mode spacing .`.
  `make ergonomics-audit` runs it.

- **Contrast floors for every text role, and `Theme.TextStyleLink` (#863)** —
  each text role now has a minimum contrast in one table: 4.5:1 (WCAG AA) on
  `ColorBackground` and `ColorPanel`, and 3:1 for a placeholder on
  `ColorInterior`. Disabled text has no floor. `ThemeMaker` and `WithColors`
  raise a derived color that falls short: the quiet roles (secondary, label,
  placeholder) by alpha, the status and link roles by lightness, with the hue
  kept. The presets already meet every floor except link text. A theme with a
  lighter body color or a custom background now gets readable quiet text instead
  of text under AA. A color the app states (`TextStyleDef.Color`, a
  `ThemeCfg.ColorText*` field, a role set by hand) is never moved; the new
  `DebugLowContrast` category (in `DebugAll`) reports it, and
  `w.TestFindings(gui.DebugLowContrast)` asserts it. Link text was drawn in the
  `ColorSelect` fill color, at 3.4 to 4.4 on six of the eight presets.
  `RichLink` and markdown links now draw in `TextStyleLink.Color`, which reads
  at 4.5:1; `ColorSelect` itself is unchanged. `TextStyleLink` is Body text,
  underlined, for a standalone link label.

- **Status text roles: `Theme.TextStyleError`, `TextStyleSuccess`,
  `TextStyleWarning` (#861)** — body text in the theme's error, success or
  warning hue, for a validation message, a saved notice or a caution line. Same
  size and face as `TextStyleBody`, so a status line sits in running text
  without a size jump. The color is the status color moved on lightness only
  (hue and saturation kept) until it reads at 4.5:1 (WCAG AA body text) on
  `ColorBackground` and `ColorPanel`. The status colors are tuned as fills: as
  text they measured 3.1 to 4.0 on the light theme and 3.8 for dark-theme error.
  `WithColors` moves each role with its status color unless the app changed it
  by hand. Before, apps copied `Cfg.ColorError` onto a body style themselves;
  that text was below AA, and on a theme with `ColorError` unset it copied an
  empty color. The showcase form and the `virtual_list` example now use
  `TextStyleError`; the `virtual_list` jump error is one step larger (Body, was
  BodySmall).

- **Button inset is a public theme role: `Theme.PaddingButton` /
  `ThemeCfg.PaddingButton` (#850)** — the inset a Button puts around its label
  was the unexported `paddingButton` (5/12). A custom button could not read it,
  so it could not match the theme's buttons. It is now a theme role like
  `PaddingField`: a theme sets it once, and all four variants (secondary,
  primary, ghost, danger) use it, so they still align in a row. Unset keeps
  5/12, so existing themes do not change.

- **Public command-golden helpers for apps and siblings (#847)** —
  `(*Window).TestGolden` with `GoldenCfg` pins a window's appearance as text
  from any package: one frame under `ThemeDark` and one under `ThemeLight`,
  serialized from the emitted `[]RenderCmd` with floats rounded to two decimals,
  diffed against `testdata/<name>.dark.golden` and
  `testdata/<name>.light.golden`. Before, the serializer lived in a `_test.go`
  file, so only `gui/` itself could pin appearance; a visual regression in a
  sibling was caught only by a person looking at a screen. Re-record with
  `GOGUI_UPDATE_GOLDEN=1` (an env var, since a flag in a library package
  collides with the consumer's own flags); a mismatch writes the recording under
  `testdata/failures/` for review. Files carry a `# go-gui-golden v1` header, so
  a future format change fails with a re-record hint instead of a wall of diffs.
  The clock is pinned and the window's theme and pointer state are restored
  afterwards. See `docs/dx-cheat-sheet.md`.

- **Theme text roles are a stable contract, with a gate (#846)** —
  `docs/theme-tokens.md` now lists the 4 de-emphasis roles and 16 purpose roles
  on `Theme`, the release that added each, and the rules: adding a role is free;
  removing, renaming or changing the purpose of one is an incompatible change; a
  rename keeps the old field one release as `// Deprecated:`. The new
  `make theme-surface-check` (in `make check` and CI) enforces the removal part.
  `gui/testdata/theme_surface.golden` lists every exported field of `Theme`,
  `ThemeCfg` and `TextStyle`; a branch that removes a line from it fails unless
  it adds a migration entry under `### Changed` in Unreleased. Before, #734 and
  #764 renamed roles that apps and custom themes use, and no gate saw it.
  `docs/style-guide.md` no longer names the removed `N1`..`N6` grid.

- **App settings store: `gui.LoadSettings` / `gui.SaveSettings` (#848)** — an
  app can now save one typed struct of settings, JSON-encoded, without choosing
  a file location itself. Before, each sibling picked its own rule (go-edit,
  go-term and go-kite used three), and none worked on web or Android. The key is
  the app ID from `WindowCfg.AppInfo`, so every window of the app shares the
  data; with no ID both return the new `gui.ErrNoAppID`. `LoadSettings` decodes
  into a struct the caller has already filled with defaults, so a field that the
  saved data does not have keeps its default and adding a field needs no
  migration. Desktop and iOS write `os.UserConfigDir()/<app ID>/settings.json`
  (XDG on Linux), replaced atomically so a crash cannot leave a partial file.
  Web uses `localStorage`. Android uses the app's files directory, which the
  host passes to the new `android.SetFilesDir` before `Start`. A window from
  `gui.NewTestWindow` uses an in-memory store, so tests never touch disk. Loads
  and saves over 1 MiB fail. The store is for state the app writes, not for
  hand-edited config or secrets. See `examples/settings` and
  `docs/dx-cheat-sheet.md`.

- **App manifest `appinfo.toml` (#849)** — an app can now keep its ID, name,
  version, build number and icon in one `appinfo.toml` beside `main.go`.
  `buildapp` reads it from the working directory (or `-manifest path`), so a
  Makefile no longer repeats `-name`, `-id`, `-version` and `-icon` for each
  platform. A flag given on the command line still wins over the file, and with
  no file buildapp behaves as before. The app embeds the same file and passes
  `appinfo.MustParse(manifest)` to the new `gui.WindowCfg.AppInfo`. The window
  then takes its title, and the app its file-access ID, X11 `WM_CLASS` and
  native menubar name, from the file; a value set in Go still wins. This matters
  on macOS: when the bundle ID and the runtime app ID differ, preferences and
  permission grants split across two identities with no error. Also new:
  `(*Window).AppInfo()`, buildapp's `-build` flag (`CFBundleVersion`), and the
  `[darwin] category` and `[linux] categories` keys. The format is a strict
  subset of TOML with no new dependency. See `examples/app_manifest` and
  `docs/deployment.md`.

### Changed

- **BREAKING: radius and border-width fields take `gui.Radius` and `gui.Border`,
  and follow the theme (#867)** — every Cfg `Radius*` field (`Radius`,
  `RadiusBorder`, `RadiusTab`, `RadiusHeader`, `RadiusContent`, `RadiusCrumb`,
  `RadiusMenuItem`, `RadiusSubmenu`) and every `Size*Border` field
  (`SizeBorder`, `SizeHeaderBorder`, `SizeContentBorder`, `SizeTabBorder`)
  changes from `Opt[float32]` to a self-flagging type, and so do the
  `Radius`/`SizeBorder` fields of the theme patches (`ButtonPatch` and the
  others) and `DataGridCfg`. The roles `gui.RadiusSmall/Medium/Large` read the
  active theme's radius ladder when the widget is built, and `gui.BorderThin`
  reads the theme's `SizeBorder`. Before, a call site wrote `gui.SomeF(4)`, so a
  platform theme or a custom `ThemeCfg` radius never reached it, and a
  `gui.SomeF(1)` border kept drawing under `Theme.WithBorders(false)`. A role in
  a theme patch resolves against the patched theme. The default themes keep
  their values, so no default widget moves. Examples snap control corners to the
  ladder (2/3 → small, 8 → medium, 10/14 → large); circles, decorative radii and
  heavier emphasis borders keep a fixed width. Migration:
  - `gui.NoBorder` / `gui.NoRadius` → unchanged (now typed values)
  - `gui.SomeF(1)` on a border → `gui.BorderThin`; `gui.SomeF(n)` →
    `gui.BorderPx(n)` for a stroke that must not follow the theme
  - `gui.SomeF(n)` on a radius → a role (`gui.RadiusMedium`), or
    `gui.RadiusPx(n)` for fixed geometry such as a circle or `RadiusPx(h / 2)`
  - `cfg.Radius.Get(def)` / `cfg.SizeBorder.Get(def)` → `.Or(def)`

  `ergonomics-audit -mode spacing` now flags `RadiusPx(n)` and `BorderPx(n)`
  with a number n > 0; mode `literals` flags a raw `gui.Radius{}` or
  `gui.Border{}`.

- **BREAKING: spacing fields take a `gui.Spacing`, and the steps follow the
  theme (#866)** — every Cfg spacing field (`Spacing`, `SpacingHeader`,
  `SpacingTrail`, `SpacingSubmenu`, `CellSpacing`, `RowSpacing`) changes from
  `Opt[float32]` to the new self-flagging `gui.Spacing` type. The four steps
  `SpacingTight/Small/Medium/Large` are now `gui.Spacing` roles, not `float32`
  constants. A role reads its value from the active theme when the widget is
  built. Before, `gui.SomeF(gui.SpacingLarge)` copied the fixed 28 into the Cfg,
  so a `ThemeCfg` that changed `SpacingLarge` did not move that gap. Now
  `Spacing: gui.SpacingLarge` follows it. Widgets in `gui/` that named a step
  follow the theme the same way. The default themes keep 2/6/14/28, so no
  default layout moves. Migration:
  - `gui.SomeF(gui.SpacingMedium)` or `gui.Some(gui.SpacingMedium)` →
    `gui.SpacingMedium`
  - `gui.SomeF(n)` or `gui.Some[float32](n)` → `gui.SpacingPx(n)` (a fixed gap
    that does not follow the theme)
  - `gui.SomeF(0)` → `gui.NoSpacing` (still an explicit zero)
  - `cfg.Spacing.Get(def)` → `cfg.Spacing.Or(def)`
  - arithmetic on a step, such as `gui.SpacingLarge * 2` →
    `gui.SpacingPx(2 * w.Theme().SpacingLarge)`

  `ergonomics-audit -mode spacing` now flags `SpacingPx(n)` with a number n > 0
  anywhere, not only in a `Spacing:` field. Mode `literals` flags a raw
  `Spacing{...}` literal, and mode `opt` accepts a plain `Spacing` field.

- **Examples use the spacing and padding steps (#851)** — about 450 gap and
  inset numbers in `examples/` now name a theme step. Before, 8, 12 and 16 were
  the usual gaps, and none of them is a step. Each site took the step that fits
  its meaning, not the nearest number: a label and its value take
  `SpacingSmall`, sibling controls take `SpacingMedium`, sections take
  `SpacingLarge`. Gaps and insets in the examples move by a few pixels. Plain
  example buttons no longer set their own inset, so they now match the height of
  an input in the same row. Styled buttons in the games keep a larger inset,
  built from the `Pad*` steps. The scale itself (2/6/14/28) does not change, and
  no `gui/` widget moves.

- **`TextButton` uses the theme button inset (#850)** — `TextButton` hardcoded
  an 8/16 inset, which made it taller than an `Input` or `Select` in the same
  row. It now sets no padding and takes `Theme.PaddingButton` (5/12 by default),
  like a plain `Button`. A caller that wants the old inset passes
  `Padding: gui.NewPadding(8, 16, 8, 16)` to `Button`. Also,
  `Theme.WithPadding(false)` now removes the button inset too. Before, a
  stripped theme kept 5/12 on every button.

### Fixed

- **`Test*` helpers run `QueueCommand` callbacks (#829)** — `TestClick`,
  `TestKey`, `TestType` and `TestScroll` now run the commands a handler queued
  before they return, and rebuild the frame after them. Before, only the
  backend's frame loop ran queued commands, so a test that clicked a button
  whose `OnClick` queued `SetFocus` or `SetView` saw the state from before the
  command. That is the pattern the frame-lock rule requires, so those flows
  could not be tested. Each helper also rebuilds again when a deferred callback
  (such as a blur commit) ran, and repeats until the window is quiet, up to 8
  passes. A command that queues itself again stops at that bound. Animation time
  does not advance: a `Test*` call is not a way to run an animation to its end.

## Older releases

v0.81.0 and earlier are in [CHANGELOG-archive.md](CHANGELOG-archive.md).
