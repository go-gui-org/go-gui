# Dependencies

Inventory of every Go module pulled in by go-gui. Source of truth:
[`go.mod`](../go.mod) (declarations) and [`go.sum`](../go.sum)
(content checksums).

Go toolchain pin: `go 1.26.0`.

## Direct Dependencies

| Module                            | Version                            | Purpose                                                                  |
| --------------------------------- | ---------------------------------- | ------------------------------------------------------------------------ |
| `github.com/alecthomas/chroma/v2` | v2.23.1                            | Syntax highlighting in the markdown widget.                              |
| `github.com/go-gl/gl`             | v0.0.0-20260331235117-4566fea9a276 | OpenGL bindings for `gui/backend/gl`.                                    |
| `github.com/go-gui-org/go-glyph`  | v1.9.0                             | Text shaping + glyph rasterization. Required by every backend.           |
| `github.com/go-pdf/fpdf`          | v0.9.0                             | PDF generation for the print-dialog backend.                             |
| `github.com/godbus/dbus/v5`       | v5.2.2                             | Linux native platform: notifications, portals.                           |
| `github.com/tdewolff/parse/v2`    | v2.8.12                            | CSS tokenizer for the SVG `<style>` / `style=""` cascade pipeline.       |
| `github.com/veandco/go-sdl2`      | v0.4.40                            | SDL2 backend (`gui/backend/sdl2`). Window, input, GL/Metal context glue. |
| `github.com/yuin/goldmark`        | v1.8.2                             | Markdown parser (markdown widget + showcase docs).                       |
| `github.com/yuin/goldmark-emoji`  | v1.0.6                             | Goldmark extension: `:emoji:` shortcodes.                                |
| `golang.org/x/mod`                | v0.36.0                            | Module version parsing; imported by `requiredid` analyzer.               |
| `golang.org/x/tools`              | v0.45.0                            | `go/analysis` framework for the `requiredid` analyzer (`tools/`).        |

## Indirect Dependencies

Pulled in transitively; listed for completeness.

| Module                       | Version                            | Pulled in by                         |
| ---------------------------- | ---------------------------------- | ------------------------------------ |
| `github.com/dlclark/regexp2` | v1.11.5                            | chroma                               |
| `github.com/rivo/uniseg`     | v0.4.7                             | grapheme segmentation (chroma/glyph) |
| `golang.org/x/mobile`        | v0.0.0-20260602190626-68735029466e | gobind tool (Android AAR builds)     |
| `golang.org/x/sync`          | v0.20.0                            | x/tools                              |
| `golang.org/x/sys`           | v0.44.0                            | sdl2 / godbus                        |
| `golang.org/x/text`          | v0.34.0                            | misc. text processing (transitive)   |

## Updating

```bash
go get <module>@<version>
go mod tidy
```

Example:

```bash
go get github.com/go-gui-org/go-glyph@v1.9.0
go mod tidy
```

After updating dependencies, regenerate this file:

```bash
make deps-doc
```

## Local go-glyph Workflow

Day-to-day text work often runs against a sibling checkout at
`~/Documents/github/go-glyph`. Wire it in via a `replace` directive:

```bash
go mod edit -replace github.com/go-gui-org/go-glyph=../go-glyph
```

Drop the replace before tagging:

```bash
go mod edit -dropreplace github.com/go-gui-org/go-glyph
go mod tidy
```

`go.mod` on `main` must not carry a local replace — CI fetches the
tagged module.

## Verification

Pre-commit checks (PostToolUse hook runs lint-fix + tests on every
`.go` edit; reproduce manually with):

```bash
go build ./...
go vet ./...
golangci-lint run ./...
go test ./...
```

Commit `go.mod` and `go.sum` together whenever either changes.
