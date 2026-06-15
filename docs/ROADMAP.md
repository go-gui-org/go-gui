# Roadmap

Phase 2 backlog — June 2026 codebase review. Phase 1 (CI, docs hygiene,
native-platform glue, large-file tooling) is complete.

## Windows Reliability

- [x] **Close issue #8 — showcase on Windows.** Users report missing
  `libFLAC.dll` at startup. Audio is already opt-in (`-tags audio`), and
  release builds use `-tags static,audio`, but dynamic builds and incomplete
  DLL bundles still fail. Verify the release zip includes all transitive
  codec DLLs (`libFLAC.dll`, etc.), not just `SDL2*.dll`. Document the
  supported Windows build path (MSYS2 + `-tags static,audio`, or use the
  release zip as-is).
- [x] **Add Windows CI smoke test.** The Windows job currently only
  `go build` + `go vet`. Add at least `go test ./gui/...` (headless backend)
  and a post-build launch of `showcase-windows.exe` with a timeout to catch
  missing-DLL startup failures before release.
- [x] **Align local and CI Windows packaging.** `scripts/bundle-windows-dlls.sh`
  downloads upstream SDL2_mixer DLLs; the release workflow copies from MSYS2.
  Pick one source of truth so `make release` and CI produce identical zips.
- [x] **Document MinGW static-link pitfalls.** `go build -tags static,audio`
  fails on some toolchains with `undefined reference to __ms_vsscanf`
  (go-sdl2 bundled libs vs MSYS2 GCC mismatch). Add a troubleshooting section
  to `CONTRIBUTING.md` or `README.md`.

## Platform Parity

Gaps from `docs/native-platform-matrix.md`, ordered by user impact.

- [ ] **Windows accessibility (UIA bridge).** Screen readers do not work on
  Windows — no UIA or AT-SPI bridge exists. macOS (VoiceOver) and Linux
  (AT-SPI) are functional.
- [x] **Save/Discard/Cancel dialogs on Linux and Windows.** Linux: zenity
  `--question` + `--extra-button Discard`, kdialog `--warningyesnocancel`.
  Windows: `MessageBoxW` with `MB_YESNOCANCEL`. Maps: Yes→Save, No→Discard,
  Cancel→Cancel.
- [x] **Windows system tray.** `Shell_NotifyIconW` with message-only window
  for callbacks, PNG→HICON via `CreateDIBSection`+`CreateIconIndirect`,
  popup menus via `CreatePopupMenu`+`TrackPopupMenu`. Wired into SDL2 and
  GL backends via `sni.Tray` package.
- [ ] **Windows spell check.** macOS (NSSpellChecker) and Linux (hunspell) are
  functional; Windows and Web are stubs.
- [x] **Windows SDL2 notifications.** Already resolved — both SDL2 and GL
  backends delegate to `nativehost.SendNotification`, which uses PowerShell
  on Windows.
- [ ] **Dark titlebar.** No-op on all backends. Requires platform-specific
  window manager calls (macOS `NSWindow.appearance`, Windows
  `DwmSetWindowAttribute`, Linux GTK CSD).
- [ ] **Security-scoped bookmarks (macOS sandbox).** C dialog layer parses
  `bookmarkData` from `NSOpenPanel`; Go side does not consume it.
- [ ] **iOS native surface.** Dialogs, a11y, IME, and OpenURI are stubs.
  CI builds c-archive but runtime behavior is minimal.
- [ ] **Android/iOS file dialogs.** Native dialog backends are stubs on
  mobile; desktop and Web are functional.

## Backend Test Coverage

Core `gui/` coverage is ~78% (CI threshold: 70%). Backend packages are
largely untested in CI coverage reports.

| Package | Coverage | Priority |
|---------|----------|----------|
| `gui/backend/sdl2` | 0% | High — default Linux/Windows backend |
| `gui/backend/metal` | ~2% | Medium — macOS default |
| `gui/backend/gl` | ~2% | Medium — alt backend |
| `gui/backend/filedialog` | 0% | High — testable via mocks |
| `gui/backend/printdialog` | 0% | Medium |
| `gui/backend/spellcheck` | 0% | Medium |
| `gui/backend/nativemenu` | 0% | Medium |
| `gui/audio` | ~34% | Low — opt-in, SDL2_mixer dependency |

- [ ] **Extend mock-based native integration tests.** Follow the pattern in
  `app_native_test.go` and `backend/internal/nativehost/`. Prioritize
  testable behavior: URI validation, command construction, notification
  errors, dialog result mapping, menu/tray callbacks, app-level routing.
- [ ] **Consistent `*_other_test.go` stubs.** printdialog, sni, and spellcheck
  already use build-tag stubs; apply the same pattern to filedialog and
  nativemenu where missing.
- [ ] **Optional per-package coverage floors.** CI enforces 70% on
  `./gui/...` only. Consider adding floors for packages with mock-based
  tests (e.g. filedialog ≥ 50%) without requiring display hardware.

## Large-File Maintenance

Run `./scripts/large-files.sh` to monitor. Split incrementally when already
touching a file — no broad refactors.

| Lines | File | Notes |
|------:|------|-------|
| 933 | `gui/svg/css/parse.go` | CSS parser leaf subpackage |
| 891 | `gui/styles_widget.go` | Theme/style application |

### Recently split (June 2026)

| Was | Lines → | Now | Files extracted |
|----:|--------:|-----|------------------|
| 1317 | 237 | `gui/svg/style.go` | `style_attr.go`, `style_color.go`, `style_compute.go`, `style_stroke.go`, `style_transform.go` |
| 1115 | 109 | `gui/svg/animation.go` | `animation_motion.go`, `animation_parse.go`, `animation_parse_value.go` |
| 934 | 217 | `gui/datagrid/view_data_grid_events.go` | `view_data_grid_jump.go`, `view_data_grid_keys.go`, `view_data_grid_pager.go` |
| 898 | 630 | `gui/view_splitter.go` | `view_splitter_handle.go` |
| — | 454 | `gui/render_svg_animation.go` | `render_svg_anim_attr.go`, `render_svg_anim_lerp.go` |
| — | 761 | `gui/canvas_draw.go` | `canvas_curve.go` |

- [x] **Extract cohesive helpers from top hotspots.** All files that
  were >800 lines have been split. `svg_cache.go` is the next candidate
  at 427 lines.
- [x] **Add tests for extracted code.** New tests: `style_compute_test.go`
  (25 tests), `render_svg_anim_attr_test.go` (35 tests),
  `render_svg_anim_lerp_test.go` (14 tests), `style_attr_test.go`
  (12 tests). Animation parser extended with `parseAnimateDashOffsetElement`
  and `parseAnimateDashArrayElement` tests (12 tests).
- [ ] **SVG diagonal gradients.** One remaining code TODO in
  `gui/svg_cache.go` 

## Documentation And DX

- [ ] **Fix build-tag naming drift.** CHANGELOG references
  `gui_showcase_audio`; the actual tag is `audio` (`demo_audio.go`).
- [ ] **Windows developer onboarding.** Consolidate MSYS2 setup, static vs
  dynamic linking, and audio opt-in into a single doc section.
- [ ] **Refresh native-platform-matrix.md** as platform gaps close.

## Deferred (revisit only when triggered)

- **Core `gui/` subpackage split.** Compile time is ~0.28s; import cycles
  prevent moving Layout/Shape/Window. See `docs/subpackage-analysis.md`.
- **Raise global coverage threshold.** Core is at ~78%; 75% is reasonable
  when backend mock tests land.
- **Example smoke test expansion.** Many examples have `main_test.go`; only
  expand for examples that demo fragile platform features.
