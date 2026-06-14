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
- [ ] **Save/Discard/Cancel dialogs on Linux and Windows.** Underlying toolkits
  (zenity/kdialog, Win32 MessageBox) support 3-button layouts; the filedialog
  package does not expose them yet.
- [ ] **Windows system tray.** macOS (`NSStatusBar`) and Linux (SNI D-Bus) are
  functional; Windows GL and SDL2 backends are stubs.
- [ ] **Windows spell check.** macOS (NSSpellChecker) and Linux (hunspell) are
  functional; Windows and Web are stubs.
- [ ] **Windows SDL2 notifications.** GL backend uses PowerShell toast API;
  SDL2 backend returns stub — gap within the same OS.
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
| 1317 | `gui/svg/style.go` | Largest file; candidate for style resolution helpers |
| 1115 | `gui/svg/animation.go` | Already has `animation_resolve.go`; more to extract |
| 934 | `gui/datagrid/view_data_grid_events.go` | Event dispatch vs row rendering |
| 933 | `gui/svg/css/parse.go` | CSS parser leaf subpackage |
| 898 | `gui/view_splitter.go` | Drag/resize logic |
| 891 | `gui/styles_widget.go` | Theme/style application |

- [ ] **Extract cohesive helpers from top hotspots.** Follow the
  `render_text.go` extraction from `render_layout.go` (856 → 385 lines).
- [ ] **SVG diagonal gradients.** One remaining code TODO in
  `gui/svg_cache.go` — blocked on glyph angle support.

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
