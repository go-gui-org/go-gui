# Contributing to Go-Gui

## Prerequisites

- Go 1.26+
- SDL2 development libraries (for running examples)
- [golangci-lint](https://golangci-lint.run/)

## Build and Test

```bash
go build ./...                        # build all packages
go test ./...                         # run all tests (headless, ~12s)
go test ./gui/... -run TestFoo        # run a single test
go vet ./...                          # static analysis
golangci-lint run ./...               # full lint
```

Tests use a headless backend (`gui/backend/test/`) — no display needed.

### macOS: suppress duplicate library warnings

On macOS, `go build`/`go test`/`go run` emit `ld: warning: ignoring duplicate
libraries: '-lobjc'`. This is harmless — multiple CGO packages each link Apple
frameworks which transitively pull `-lobjc`. To suppress:

```bash
export CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries"
```

Or use [direnv](https://direnv.net/) — the repo includes a `.envrc` file.

### CI vs local testing

`go test ./...` runs **all** packages — core gui, backends, examples, cmd
tools, internal packages. This is what you should run locally before pushing.
Some backend packages require CGO and system libraries (SDL2, etc.).

CI splits testing across dedicated jobs:

| Job | Scope | Notes |
|-----|-------|-------|
| Test | `go test ./gui/...` | Core gui only; coverage + threshold (70%) |
| Check | `make check` | `go test ./...` + vet + lint |
| Examples | `make build-examples` | Compile-only, no runtime |
| WASM | `go test ./gui/...` | Under `GOOS=js GOARCH=wasm` |
| Fuzz | nightly, `workflow_dispatch` | 12 fuzz targets, 60s each |

CI runs `./gui/...` for coverage because backend and example packages have
conditional build constraints and platform-specific dependencies best tested
in their dedicated jobs.

CI also enforces:
- **Coverage threshold**: 70% total across `./gui/...`. PRs compare per-package
  coverage against the main-branch baseline and flag drops > 2%.
- **Race detector**: enabled on Linux test runs (`-race`).
- **Benchmark regression gate**: compares `Benchmark(Layout|GenerateViewLayout|...|RenderSvg)`
  against the main-branch baseline using `benchstat`.

## Coding Conventions

- **No variable shadowing.** Use `=` to reassign existing variables, not `:=`.
- **Clean lint and format.** All code must pass `golangci-lint run ./...` and
  `gofmt` with zero issues before committing.
- Prefer reducing heap allocations when optimizing performance.

## Submitting Changes

1. Fork the repository and create a feature branch.
2. Make focused, single-purpose commits.
3. Add or update tests for any changed behavior.
4. Run the full check suite before pushing:
   ```bash
   go test ./... && go vet ./... && golangci-lint run ./...
   ```
5. Open a pull request against `main`.

## Pre-commit / Post-edit hooks

The repo includes [Claude Code](https://claude.ai/code) hooks in
`.claude/settings.json`. These fire automatically after every `.go` file
edit when using Claude Code:

- **`golangci-lint run --fix`** — auto-fixes lint issues in the edited
  package directory.
- **`go test -count=1 -short`** — runs the edited package's tests to catch
  regressions immediately.
- **`go.sum` edit guard** — blocks direct edits to `go.sum`; use
  `go mod tidy` instead.

These hooks are optional but recommended. To customize them, edit
`.claude/settings.json` or your global `~/.claude/settings.json`. See the
[Claude Code hooks documentation](https://docs.anthropic.com/en/docs/claude-code/hooks)
for details.

## Adding Examples

Example apps live in `examples/`. Each example should be a self-contained
`main` package that demonstrates a specific feature or pattern.

## License

Contributions are accepted under the
[PolyForm Noncommercial License 1.0.0](LICENSE).
