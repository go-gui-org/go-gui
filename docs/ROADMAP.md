# Go-Gui Roadmap

Concrete, independently shippable improvements. Milestones map to semver
bumps from current v0.25.

---

## v0.26.0 — Quality & hygiene

### 1. Docs and tooling consistency ✅

**Trivial fix, high trust impact.**

- [x] CONTRIBUTING.md claims `go test ./...` is "~0.25s" — stale since the early
  days; actual wall clock is ~12s. Fix the number.
- [x] Lint commands sometimes say `./gui/...`, CI lints `./...`. Pick one and
  standardize across README, CONTRIBUTING.md, and CI config.
- [x] Test that all 35 examples build.

### 2. Fix macOS linker warnings ✅

**Low effort, noisy-CI hygiene.**

`ld: warning: ignoring duplicate libraries: '-lobjc'` (and occasional
`-lSDL2`) fire on every `go test ./...` run. Likely an SDL2 pkg-config or
CGO_LDFLAGS duplicate. Worth 30 minutes of investigation — noisy CI masks
real linker issues.

- [x] Root cause: multiple CGO packages each link Apple frameworks
  (`-framework AppKit`, `-framework Cocoa`, etc.); each transitively pulls
  `-lobjc`. When linked into one test binary, the linker sees duplicates.
- [x] Fix: `CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries"` on macOS builds.
  Applied in CI test job (macOS only), Makefile `build-macos` and
  `build-examples` targets (macOS-conditional).

### 3. CI benchmark regression gates

**Builds on existing infra.**

CI already runs benchmarks on main
(`.github/workflows/ci.yml:273`), but doesn't compare against a baseline.
Perf is constrained by allocations, not throughput (see
[[project_go_gui_perf_baseline]]). Add:
- Stored baseline per benchmark (or `benchstat` against previous main).
- PR comment or check-run annotation on regression beyond threshold.
- Gate: layout, SVG parse/tessellate, and render pipeline are the
  highest-ROI targets.

### 4. SVG rendering: `SvgAlignNone`

**Concrete, scoped, user-visible.**

`gui/render_svg.go:178` — `SvgAlignNone` should non-uniformly stretch
with independent scaleX/scaleY. Currently treated as xMidYMid.
Self-contained fix, clear test target.

### 5. Backend/platform test coverage

**De-risk the cross-platform surface.**

Several backend subpackages have zero test files: `android/`, `atspi/`,
`internal/`, `ios/`. Calibrate expectations:

| Package     | Priority | Rationale                                      |
|-------------|----------|------------------------------------------------|
| `internal/` | High     | Shared backend internals, broad blast radius   |
| `atspi/`    | Medium   | Accessibility bridge, complex protocol surface |
| `android/`  | Low      | Thin platform stub, low ROI                    |
| `ios/`      | Low      | Thin platform stub, low ROI                    |

Packages with one test file (`gl/`, `metal/`, `sdl2/`, `nativemenu/`,
`spellcheck/`) could also use expansion — config translation, error paths,
and capability fallbacks are good smoke-test targets.

### 6. Backend panics → returned errors

**Correct Go idiom, but calibrate the rationale.**

Several backends `panic` or `log.Fatal` on init failure (e.g.,
`gui/backend/gl/backend.go:364` has three panics, `gui/backend/web/backend.go`
has two). Codex's framing — "GUI apps should show error dialogs" — is
wrong. If GL context creation fails, there is no window to show a dialog
in.

The real value is **testability and embeddability**:
- Tests can exercise failure paths instead of crashing the test runner.
- go-term (which embeds go-gui, see [[project_go_term]]) can degrade
  gracefully on partial backend init.

Prioritize these by call site:
1. Init paths that are reachable from tests.
2. Init paths that go-term or other embedders hit.
3. Deep internal paths where recovery is genuinely impossible (these can
   stay as panics with a comment explaining why).

---

## Future

### SVG diagonal gradients

`gui/svg_load.go:361` — blocked upstream. The TODO says "when glyph adds
angle support." Until go-glyph exposes diagonal gradient direction, this
stays as-is.

### Native dark/light mode sync

Auto-switch theme to follow OS appearance preference. Requires:
- `ThemeAuto` mode in the theme system
- `NativePlatform.OSThemePreference()` on each backend
- macOS: `NSApp.effectiveAppearance`
- Linux: `gsettings get org.gnome.desktop.interface color-scheme`
- Windows: registry `AppsUseLightTheme`

### Autocomplete / suggestion list

Extend `InputCfg` with `Suggestions func(string) []string` (debounced
callback). Renders a floating dropdown below the input, navigable by
arrow keys. Partially covered by Combobox for static option lists;
autocomplete handles dynamic/suggestion scenarios.

### Charting / graphing

Separate `go-charts` package built on go-gui. All framework prerequisites
are complete (canvas view, retained geometry, text measurement, clipping,
mouse events, gradients, animation, custom shaders).

### Media

Embedded video/audio — native media playback widget. Requires platform
backends (AVPlayer on macOS, GStreamer or PipeWire on Linux, Media
Foundation on Windows).

### Community & adoption

- **Contribution guide**: update `CONTRIBUTING.md` with new Makefile targets
- **Issue templates**: add `.github/ISSUE_TEMPLATE/` forms for bugs and
  feature requests
- **GoReleaser**: evaluate for v0.26+ once Makefile release pipeline is
  stable. Right now the CGo + static SDL2 path needs explicit control;
  GoReleaser adds abstraction when it's no longer needed.
