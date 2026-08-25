# Contributing to Go-Gui

## Build and Test

Run the full local validation gate before pushing a branch:

```bash
make prepush
```

`make prepush` approximates the CI matrix from one host: race-enabled tests,
vet, lint (linux + the cross-GOOS `//go:build` files via `make lint-cross`),
cgo-free cross-compiles of the whole module (`make cross-compile`), the coverage
gate (`make coverage-gate`: 70% total + per-package floors), and the export
audit. CI and the Makefile share the coverage thresholds via
`scripts/coverage-gate.sh`.

When only the fast gate checks are wanted, `make check` (vet, deps-doc,
large-files, generate-check, tidy-check) is the quick subset. The tracked
`.githooks/pre-push` hook runs `make check-all` (test + lint + check) on every
push — enable it with `git config core.hooksPath .githooks`. Tools like
golangci-lint and gosec must be installed. `make lint` pins the golangci-lint
version.

For a tight edit → rebuild → relaunch loop while iterating on an example app,
see [docs/dev-loop.md](docs/dev-loop.md)
(`./scripts/dev-loop.sh ./examples/get_started/`).

Build artifacts never land in the repo root: `make build-examples` writes each
example to `examples/bin/`. `make build-macos`, `make build-windows`, and the
other build targets write to `build/`. `scripts/dev-loop.sh` writes to
`build/dev-loop/`. A bare `go build ./examples/<name>/` or
`go build ./tools/<name>/` drops a binary in the repo root, so always pass `-o`
with an explicit output path (for example, `-o build/<name>`).

`go vet` does not run the `requiredid` analyzer — it is a standalone
framework-owned analyzer. CI runs it on every push, and `make vet` includes it.
Adopter projects can invoke it the same way, standalone, without the Makefile:

```bash
go run github.com/go-gui-org/go-gui/tools/requiredid/cmd/requiredid ./...
```

It flags focusable/scrollable widgets created without an `ID` (a silent no-op
that never joins the tab order). Escalate with a `//requiredid:ignore` comment
on the offending line if an ID is genuinely not needed.

Tests exercise layout and widget logic without a display. On macOS, suppress
harmless duplicate-library warnings with
`export CGO_LDFLAGS="-Wl,-no_warn_duplicate_libraries"` (or use the repo's
`.envrc` with [direnv](https://direnv.net/)).

CI also enforces race detector and benchmark regression gates. Run
`make test-race` locally before pushing (prepush includes it).

### CI-only validation

These CI checks have no local Makefile equivalent — they either need a different
OS runner or a baseline from `main` that only CI can supply:

- OS-matrix test runs on Windows and macOS runners. Windows also runs the
  showcase smoke test with Mesa's software GL
- Coverage diff on PRs and the benchmark regression gate — both compare against
  a baseline cached from `main` (`scripts/cov-diff.sh` can run them locally if
  you supply two profiles)
- WASM build/vet/test (`GOOS=js` needs a node `wasm_exec` wrapper).
  `make build-wasm` covers the build half
- iOS and Android vet+lint — need an Xcode iphoneos sysroot / Android NDK.
  `make build-ios` and `make build-android` cover the build half
- Release packaging (`release.yml`)

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

Markdown must pass Prettier. Run `make fmt-md` before committing. `make check`
runs `make fmt-md-check` and fails on drift. The wrap width lives in
`.prettierrc`, so no flags are needed at the call site.

Theme reads follow the generation boundary. Code that runs **outside**
generation — event handlers, post-arrange work, backends — and has a `*Window`
in hand calls `w.Theme()`. The bare `guiTheme` / `default*Style` read is for
widget factories and `GenerateLayout`, which have no window at hand and where
the bare read is also what makes `gui.Themed` subtree scoping work.
`make ergonomics-audit` (mode `theme`) gates the post-generation paths. See
[docs/specs/per-window-theme.md](docs/specs/per-window-theme.md).

## Submitting Changes

1. Fork, create a feature branch, make focused commits.
2. Add or update tests.
3. Run `make prepush` (fast subset: `make check`).
4. Open a pull request against `main`.

## Claude Code hooks

`.claude/settings.json` auto-runs `golangci-lint run --fix` and
`go test -count=1 -short` after `.go` edits. Customize in
`~/.claude/settings.json`. See
[docs](https://docs.anthropic.com/en/docs/claude-code/hooks).

## Adding Examples

Example apps live in `examples/`. Each example is a self-contained `main`
package that demonstrates a specific feature or pattern.

## License

Contributions are accepted under the [MIT License](LICENSE).
