# Go-Gui Roadmap

This document outlines the planned future directions for the Go-Gui project.

## High Priority

- **WebGPU Backend**: Implement a WebGPU backend (via `GPUCanvasContext` / `navigator.gpu`) to provide lower GPU overhead and compute shader support on the web target.
- **Trap Quit Requests**: When displaying a dialog, the request to quit the program (cmd+q on mac) should be ignored.

## Medium Priority

- **Performance Optimizations**:
    - Investigate releasing the `Window` mutex during user-provided View function execution.
    - Further optimization of scratch pools for transient layout data.
- **Text Roll Animation** for single line labels. Sometimes referred to as slot machine.

## Low Priority

- **Upstream Feature Integration**:
    - Support diagonal gradients in SVG once `go-glyph` adds angle support.
- **Improved Tooling**:
    - Enhance time-travel debugging with better state inspection.

## Maintenance

Ongoing code-health and infrastructure work. Tracked across releases rather
than tied to specific milestones.

### Code Health

- **File-Size Reduction**: ✓ Complete. All `gui/` source files now ≤800 lines.
  14 files split into 32 total (see `docs/file-size-reduction.md` for split
  log). Enforced via `scripts/large-files.sh` in CI.
- **Complexity Reduction**: Reduce `//nolint:gocyclo` suppressions (20+
  instances) across SVG parsing, event dispatch, backend event loops, drag
  reorder, tab control, and text rendering.
- **Backend Draw Consolidation**: Evaluate shared structure across 6 `draw.go`
  backends (web, gl, metal, sdl2, android, ios) for deduplication.
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

- **Race Detector**: Enable `go test -race` on macOS and Windows
  (currently Linux-only).
- **File-Size Gate**: Add CI step failing if any `gui/` source file exceeds
  800 lines, using `scripts/large-files.sh`.
- **Dead-Code Detection**: Add to CI `check` job.
- **Coverage Floors as Config**: Extract per-package coverage floor values
  from CI YAML into a standalone file so tweaks don't require YAML edits.

### Tooling

- **Dependencies Doc**: Keep `docs/dependencies.md` auto-generated via
  `make deps-doc`; CI validates freshness via `make deps-doc-check`.
- **Linter Config**: Maintain `.golangci.yml` and `.gosec.json` as the
  single source of truth for static analysis rules.
