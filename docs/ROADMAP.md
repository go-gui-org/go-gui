# Go-Gui Roadmap

This document outlines the planned future directions for the Go-Gui project.

## High Priority

- **WebGPU Backend** ~explored, rejected~: Prototype proved the architecture works
  (12 WGSL pipelines, solid rect + gradient + shadow rendering), but text
  rendering requires Canvas2D for font measurement. A hybrid backend adds
  complexity without addressing the actual bottleneck (heap allocations).
  See CLAUDE.md § Rejected Approaches.
- **Trap Quit Requests** ~done~: When displaying a dialog, the request to quit the program (cmd+q on mac) should be ignored.

## Medium Priority

- **Performance Optimizations**: ~done, Spec at [docs/perf-optimizations.md](perf-optimizations.md).
    - Release `Window` mutex during View function execution (add `animMu`, atomic `inputCursorOn`).
    - Scratch pool cleanup (remove 4 unused `scratchSlice` fields).
- **Text Roll Animation** for single line labels. Sometimes referred to as slot machine.

## Low Priority

- **Improved Tooling**:
    - Enhance time-travel debugging with better state inspection.

### Code Health

- **Complexity Reduction**: Reduce `//nolint:gocyclo` suppressions (20+
  instances) across SVG parsing, event dispatch, backend event loops, drag
  reorder, tab control, and text rendering.
- **Backend Draw Consolidation** ~done~: Extracted shared GPU types/functions
  and draw helpers from 6 backends. Eliminated ~500 duplicated lines.

  **Completed**:
  - `gui/backend/internal/gpu/` — shared types and pure-Go functions for GPU
    backends: `Vertex`, `BuildQuad`, `NormColor`, `PackParams`, `ApplyRotation`,
    `Mat4Mul`, `Ortho`, `IdentityTM`, `PackGradientUniforms`.
  - `gui/render_draw_common.go` — `ComputeTextPathPlacements` (55-line core
    extracted from all 6 `drawTextPath` implementations) and
    `GradientBorderRects` (4-edge rect computation shared by GPU backends).
  - `drawRtf` reduced to one-line `drawLayout` delegate in all 6 backends.
  - 4 copies of rotation/matrix/buffer/quad functions → 1 shared `gpu` package.
  - 6 copies of text-path placement math → 1 shared function.

  **Deferred**:
  - `DrawBackend` interface — a 23-method interface with single compile-time
    implementation is an anti-pattern. Per-backend dispatch loops (~60 lines
    each) are clear, fast, and rarely change. Not worth the abstraction.
  - 400-line `draw.go` target — remaining code is genuinely backend-specific
    draw primitives. Not targetable without moving GPU API calls out of
    backends, which defeats their purpose.

  **Results** (draw.go lines):

  | Backend | Before | After |
  |---|---|---|
  | gl | 779 | 669 |
  | metal | 760 | 675 |
  | ios | 744 | 660 |
  | android | 717 | 633 |
  | sdl2 | 602 | 536 |
  | web | 682 | 624 |

  Plus `gpu/` (361 lines) replacing ~300 lines of duplicated code across 12
  `buffers.go`/`rotation.go` files (now stubs). Net reduction: ~500 lines.
- **Build-Tag Surface**: 60+ build-tag-conditioned files create combinatorial
  testing burden. Reduce platform-specific code where backends share behavior.

### Test Infrastructure

- **Expanded Coverage**:
  - Dedicated unit tests for `view_input_keys.go` and `view_input_layout.go`.
  - Increased coverage for SVG animation rendering (`render_svg_animation.go`).
  - Backend-specific test suites for web, gl, metal, sdl2, android, ios
    (currently only `filedialog` and `internal/` packages have coverage floors).
  - Test execution on Android and iOS CI jobs (currently vet-only).
- **Fuzz Targets**: Maintain and expand 19 fuzz targets across `gui/`,
  `gui/markdown/`, `gui/svg/`.

### CI Hardening

- **CI Hardening** ~spec~: Spec at [docs/specs/ci-hardening.md](specs/ci-hardening.md).
  - **Race Detector**: Enable `go test -race` on macOS and Windows
    (currently Linux-only).
  - **File-Size Gate**: Add CI step failing if any `gui/` source file exceeds
    800 lines, using `scripts/large-files.sh`.
  - **Dead-Code Detection**: Add `golang.org/x/tools/cmd/deadcode` to CI.
  - **Makefile Security Targets**: Add `make gosec`, `make govulncheck`,
    `make security` for local CI parity.
  - **Coverage Floor Improvements**: Set non-zero floors for backend
    packages with existing tests.
  - **Test Timeout Enforcement**: Add explicit `-timeout` flags.
  - **golangci-lint Version Pinning**: Pin version in CI and Makefile.
  - **Go Module Cache**: Enable module caching (`cache: false` → `true` on
    11 `setup-go` steps).
  - **Generated Code Freshness**: Verify `go generate` idempotence in CI.
  - **CI Job Deduplication**: `check` job re-runs `test`/`vet` redundantly;
    restructure as a lightweight gate.
  - **`go mod tidy` Gate**: Fail CI if `go.mod`/`go.sum` are untidy.
  - **Workflow Pinning**: Pin GitHub Actions to commit SHAs.
  - **Test Flake Detection**: `-count=2` or `gotestsum --rerun-fails`.

### Tooling

- **Dependencies Doc**: Keep `docs/dependencies.md` auto-generated via
  `make deps-doc`; CI validates freshness via `make deps-doc-check`.
- **Linter Config**: Maintain `.golangci.yml` and `.gosec.json` as the
  single source of truth for static analysis rules.
