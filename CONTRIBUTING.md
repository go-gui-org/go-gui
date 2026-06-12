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

## Adding Examples

Example apps live in `examples/`. Each example should be a self-contained
`main` package that demonstrates a specific feature or pattern.

## License

Contributions are accepted under the
[PolyForm Noncommercial License 1.0.0](LICENSE).
