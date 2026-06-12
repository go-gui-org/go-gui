## Code Quality Improvements

Concrete follow-ups from the June 2026 code review. Current baseline:
`go test ./...`, `go vet ./...`, `golangci-lint run`, `go test -cover ./gui/...`,
and `go test -race ./gui/...` pass locally.

### Datagrid public API cleanup

- [x] Export or replace the unexported pagination request types used by
  `GridDataRequest.Page`, so external `DataGridDataSource` implementations can
  inspect cursor/offset pagination without package-private type assertions.
- [x] Export or replace the unexported mutation kind in `GridMutationRequest`,
  or add accessor helpers, so external mutation implementations can switch on
  create/update/delete cleanly.
- [x] Pass `GridAbortSignal` and `RequestID` through CRUD save mutations, not
  just fetch requests, so slow create/update/delete operations can observe
  cancellation promptly.

### Async fetch hardening

- [x] Fix remote image max-byte handling to allow responses exactly equal to
  `MaxImageBytes`: read `maxSize+1`, reject only `> maxSize`, and handle file
  close errors after writes.
- [x] Add injectable markdown diagram fetchers or endpoint configuration for
  CodeCogs/Kroki rendering, similar to `WindowCfg.ImageFetcher`, to support
  private deployments, tests, and offline use.
- [x] Keep external markdown APIs disabled by default; document the privacy
  boundary and make opt-in behavior visible in examples that use diagrams.

### Core maintainability

- [x] Continue splitting `Window` behavior into narrower internal subsystems.
  The struct already embeds concern groups; move behavior behind those groups
  where it reduces coupling in layout, render, event, animation, dialog, IME,
  inspector, and history code.
- [x] Replace `gui/datagrid` dot imports with an explicit package alias, then
  remove the linter exclusion for staticcheck `ST1001`.
- [x] Add `make test`, `make vet`, `make lint`, and `make check` targets that
  mirror CI, so local pre-PR validation is obvious and repeatable.

### Backend coverage expansion

- [x] Add focused smoke tests for backend capability fallbacks, config
  translation, and error paths in low-coverage packages.
- [x] Prioritize shared backend internals first, then platform bridges such as
  AT-SPI, native menus, file/print dialogs, spellcheck, and tray integration.

## Future

### Media

Embedded video/audio — native media playback widget. Requires platform
backends (AVPlayer on macOS, GStreamer or PipeWire on Linux, Media
Foundation on Windows).

### Community & adoption

- **Issue templates**: add `.github/ISSUE_TEMPLATE/` forms for bugs and
  feature requests
- **GoReleaser**: evaluate for v0.26+ once Makefile release pipeline is
  stable. Right now the CGo + static SDL2 path needs explicit control;
  GoReleaser adds abstraction when it's no longer needed.
