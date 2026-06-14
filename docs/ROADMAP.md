# Roadmap

Improvement backlog from the June 2026 codebase review.

## CI And Quality Gates

- [x] **Fail fuzz workflow on discovered crashes.** The fuzz workflow currently
  records failing targets as warnings but does not fail the job. Keep the summary
  table, then exit non-zero when any fuzz target fails.
- [x] **Fix coverage-diff regression counting.** The PR coverage diff increments
  `regressions` inside a shell pipeline subshell, so the final count can be
  wrong. Rewrite the loop to preserve state in the parent shell.
- [x] **Refresh coverage and benchmark baseline keys.** The cache keys for
  coverage and benchmark baselines are fixed names, which can make baselines
  stale. Include the main-branch commit SHA or an intentional rolling key.
- [x] **Add recurring vulnerability/security scans.** Add `govulncheck ./...`
  and a targeted `gosec` run to CI so security checks are enforced rather than
  one-time manual audit notes.

## Documentation Hygiene

- [x] **Resolve license drift.** `LICENSE` and `README.md` identify the project
  as MIT, while `CONTRIBUTING.md` says PolyForm Noncommercial. Pick the intended
  license and make all references consistent.
- [x] **Update dependency documentation.** `docs/dependencies.md` has stale module
  versions compared with `go.mod`. Add a lightweight check or generation step so
  this file does not drift.
- [x] **Sync profiling docs with current CI.** `docs/profiling.md` still says to
  consider adding benchmark comparison to PR CI, but the benchmark gate already
  exists.

## Platform And Native Surface

- [ ] **Raise coverage on native platform behavior.** Core `gui` coverage is
  healthy, but backend/native packages remain low. Prioritize testable behavior:
  URI validation, command construction, notification errors, dialog result
  mapping, menu callbacks, tray callbacks, and app-level routing.
- [ ] **Share duplicated native-platform glue.** The GL and SDL2 backends
  duplicate OpenURI, dialog forwarding, notification, spellcheck, and IME logic.
  Extract shared non-rendering behavior into a small backend-internal helper.
- [ ] **Add App native integration tests.** Cover `App.SetNativeMenubar`,
  `ClearNativeMenubar`, `SetSystemTray`, `UpdateSystemTray`, and
  `RemoveSystemTray` using `NoopNativePlatform`-based mocks.

## Architecture And Maintenance

- [ ] **Keep core `gui` flat unless compile time becomes a real problem.** The
  layout/render/event/window/widget trunk is tightly coupled by design. Continue
  moving only leaf subsystems into subpackages.
- [ ] **Track large-file hotspots.** The largest files are mostly SVG,
  DataGrid, layout/render, and tests. Avoid broad refactors, but split files
  when a cohesive helper can be extracted without changing public API.
- [ ] **Preserve benchmark coverage for hot paths.** Keep benchmarks focused on
  `GenerateViewLayout`, `layoutArrange`, `renderLayout`, event dispatch, SVG
  parse/render, and list/grid preparation paths.
