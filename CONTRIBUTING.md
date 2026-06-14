# Contributing to Go-Gui

## Build and Test

```bash
go test ./... && go vet ./... && golangci-lint run ./...
```

Tests use a headless backend (`gui/backend/test/`) — no display needed.
On macOS, suppress harmless duplicate-library warnings with
`export CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries"` (or use the repo's
`.envrc` with [direnv](https://direnv.net/)).

CI enforces 70% coverage, race detector, and benchmark regression gates.
Run `go test ./...` locally before pushing.

### Local development with sibling repos

Sibling repos: [go-glyph](https://github.com/go-gui-org/go-glyph) (text),
[go-edit](https://github.com/go-gui-org/go-edit) (code editor),
[go-charts](https://github.com/go-gui-org/go-charts) (charts),
[go-kite](https://github.com/go-gui-org/go-kite) (tiling).

Use a `go.work` file (recommended, don't commit):

```bash
cd ~/Documents/github/
go work init ./go-gui ./go-glyph
go work use ./go-edit ./go-charts  # add as needed
```

Or `go.mod` replace directives (revert before committing):

```bash
go mod edit -replace=github.com/go-gui-org/go-glyph=../go-glyph
```

## Coding Conventions

Code must pass `golangci-lint run ./...` and `gofmt`. No variable shadowing.

## Submitting Changes

1. Fork, create a feature branch, make focused commits.
2. Add or update tests.
3. Run `go test ./... && go vet ./... && golangci-lint run ./...`.
4. Open a pull request against `main`.

## Claude Code hooks

`.claude/settings.json` auto-runs `golangci-lint run --fix` and
`go test -count=1 -short` after `.go` edits. Customize in
`~/.claude/settings.json`. See [docs](https://docs.anthropic.com/en/docs/claude-code/hooks).

## Adding Examples

Example apps live in `examples/`. Each example should be a self-contained
`main` package that demonstrates a specific feature or pattern.

## License

Contributions are accepted under the [MIT License](LICENSE).
