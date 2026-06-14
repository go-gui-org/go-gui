# Roadmap

New improvement items identified in the June 2026 codebase review.

## Testing Strategy Expansion

- [x] **Event & Layout Fuzzing**: Expand fuzz testing to the event dispatch system and layout sizing engine to catch edge-case panics in the single-pass pipeline. (6 new fuzz targets in `gui/event_fuzz_test.go`, `gui/layout_sizing_fuzz_test.go`)
- [x] **Concurrency Stress Tests**: Add stress tests for `gui.StateMap` and `Animation` subsystems to verify thread safety at high refresh rates (120Hz+). (8 new stress tests in `gui/statemap_stress_test.go`, `gui/animation_stress_test.go`)

## Documentation & Onboarding

- [x] **Advanced Cookbooks**: Add guides for async data binding in `DataGrid` and custom GPU shader integration. (`docs/cookbook-datagrid-async.md`, `docs/cookbook-custom-shader.md`)
- [x] **Native Platform Matrix**: Expand `docs/native-platform-matrix.md` with a detailed feature-support table (a11y, dialogs, tray) per OS/backend.
- [x] **Architecture Sync**: Update `docs/architecture.md` to reflect recent backend evolutions (WebGPU plans, Metal/UIKit on iOS).

## Security Hardening

- [x] **Privacy Audit**: Review `log` statements in `ImageFetcher` and `NativePlatform` for potential PII leakage.
- [x] **Resource Exhaustion Caps**: Implement global memory limits for SVG and font glyph caches to prevent memory bloat in long-running apps.
- [x] **Automated Scan**: Perform a comprehensive automated security audit using gosec.

## Performance & Observability

- [ ] **CI Benchmarking**: Integrate allocation benchmarks for hot paths (`GenerateViewLayout`, `renderLayout`) into the CI pipeline.
- [ ] **SVG Gradients**: Support diagonal gradients in `gui/svg_cache.go` (once `go-glyph` adds angle support).
- [ ] **Metal Test Hardening**: Resolve threading issues in `gui/backend/metal/render_test.go` to support reliable ARM64 macOS testing.

## Code Quality & Tooling

- [ ] **Multi-Repo Workflow**: Document `go.work` setup in `CONTRIBUTING.md` for sibling project development (`go-glyph`, `go-edit`).
- [ ] **Aggressive Linting**: Enable additional linters (`gocritic`, `cyclop`) to maintain the structural integrity of complex widget logic.
