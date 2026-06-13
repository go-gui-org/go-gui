# Roadmap

Improvement items from the June 2026 codebase review.

## Quick wins

- [x] Fix or gate `cmd/buildapp` `TestInstallIconPngConversion` so `go test ./...`
  matches CI (valid iconset fixture, build tag, or skip when `iconutil` unavailable)
- [x] Enforce coverage in CI — fail or warn when total coverage drops below baseline
- [x] Run `make check` (or equivalent) on PRs to match CONTRIBUTING guidance

## Testing gaps

### Render backends

- [x] Add headless render smoke tests for `metal/`, `gl/`, and `sdl2/` — draw a
  rect + text, assert no panic and non-empty framebuffer

### Native platform

- [ ] Expand tests for `filedialog`, `printdialog`, `nativemenu`, `sni`, and
  `spellcheck` beyond stub "unsupported" paths — interface-level tests via
  `NativePlatform` mocks; platform-specific tests behind build tags

### Accessibility

- [x] Add tests for `syncA11y` — build a small layout tree and assert
  `A11yNode` structure, labels, and roles

### Examples and packages

- [ ] Add compile + one-frame render tests for the 18 examples that lack tests
- [ ] Add minimal init/lifecycle tests for `gui/audio` and `gui/shader`

## Code organization

- [ ] Split oversized files when touched — prioritize:
  - `gui/svg/tessellate.go`
  - `gui/svg/animation.go`
  - `gui/render_svg.go`
  - `gui/view_tree.go`
  - `gui/svg_load.go`
- [ ] Consider subpackages for large core subsystems over time (e.g. `gui/layout/`,
  `gui/animation/`) if compile times or discoverability become painful

## CI and developer experience

- [ ] Add coverage diff on PRs — comment or summary showing which files regressed
- [ ] Run fuzz tests in CI periodically (nightly or on main):
  `fuzz_clamp_unit`, `markdown/walker_fuzz`, `svg/fuzz_transform_decompose`
- [ ] Document PostToolUse / pre-commit hooks in CONTRIBUTING
- [ ] Document local `go-glyph` replace setup; consider a `go.work` example for
  multi-repo development

## Documentation and onboarding

- [ ] Document CI vs local test scope in CONTRIBUTING (`./gui/...` vs `./...`)
- [ ] Keep architecture docs in sync with backends (README mentions WebGPU;
  `docs/architecture.md` lists Metal/OpenGL)
- [ ] Add a "add a new widget" cookbook — Cfg struct, `requiredid` tags, test
  pattern, showcase demo
- [ ] Improve godoc on key exported types (`Window`, `Layout`, widget `*Cfg`)

## Performance and quality

- [ ] Add allocation benchmarks for hot paths: `GenerateViewLayout`,
  `renderLayout`, SVG cache hits
- [ ] Add `docs/profiling.md` for pprof workflows
- [ ] Keep golden tests when changing SVG tessellation or animation

## API and product polish

- [ ] Document native dialog platform matrix per feature
- [ ] Ensure showcase docs cover form async validation patterns
- [ ] Add a minimal time-travel debugging example and test

## Priority order

1. Fix or gate `cmd/buildapp` test
2. Backend render smoke tests
3. a11y tree tests
4. Coverage threshold / PR diff in CI
5. Split largest files when touched
6. Example test coverage for untested demos
