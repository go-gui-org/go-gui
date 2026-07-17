# Dependencies

Inventory of every Go module pulled in by go-gui. Source of truth:
[`go.mod`](../go.mod) (declarations) and [`go.sum`](../go.sum)
(content checksums).

Go toolchain pin: `go 1.26.0`.

## Direct Dependencies

| Module | Version | Purpose |
| ------ | ------- | ------- |
| `github.com/alecthomas/chroma/v2` | v2.26.1 | Syntax highlighting in the markdown widget. |
| `github.com/ebitengine/purego` | v0.10.1 | cgo-free dynamic loading of libEGL for the native Linux backend (`gui/backend/gl`). |
| `github.com/go-gl/gl` | v0.0.0-20260331235117-4566fea9a276 | OpenGL bindings for `gui/backend/gl`. |
| `github.com/go-gui-org/go-glyph` | v1.17.0 | Text shaping + glyph rasterization. Required by every backend. |
| `github.com/go-pdf/fpdf` | v0.9.0 | PDF generation for the print-dialog backend. |
| `github.com/godbus/dbus/v5` | v5.2.2 | Linux native platform: notifications, portals. |
| `github.com/gopxl/beep/v2` | v2.1.1 | Audio playback (sound effects, music, mixer, fade) for `gui/audio`. |
| `github.com/jezek/xgb` | v1.3.1 | Pure-Go X11 windowing + events for the native Linux backend (`gui/backend/gl`). |
| `github.com/tdewolff/parse/v2` | v2.8.13 | CSS tokenizer for the SVG `<style>` / `style=""` cascade pipeline. |
| `github.com/yuin/goldmark` | v1.8.2 | Markdown parser (markdown widget + showcase docs). |
| `github.com/yuin/goldmark-emoji` | v1.0.6 | Goldmark extension: `:emoji:` shortcodes. |
| `golang.org/x/mod` | v0.37.0 | Module version parsing; imported by `requiredid` analyzer. |
| `golang.org/x/sys` | v0.46.0 | Win32 + WGL syscalls for the native Windows backend (`gui/backend/gl`, `winkey`). |
| `golang.org/x/tools` | v0.47.0 | `go/analysis` framework for the `requiredid` analyzer (`tools/`). |

## Indirect Dependencies

Pulled in transitively; listed for completeness.

| Module | Version | Pulled in by |
| ------ | ------- | ------------ |
| `github.com/dlclark/regexp2/v2` | v2.2.2 | chroma |
| `github.com/ebitengine/oto/v3` | v3.3.2 | gopxl/beep (audio output) |
| `github.com/go-text/typesetting` | v0.3.4 | go-glyph (pure-Go text shaping) |
| `github.com/hajimehoshi/go-mp3` | v0.3.4 | gopxl/beep (MP3 decode) |
| `github.com/icza/bitio` | v1.1.0 | mewkiz/flac (FLAC decode) |
| `github.com/jfreymuth/oggvorbis` | v1.0.5 | gopxl/beep (Vorbis decode) |
| `github.com/jfreymuth/vorbis` | v1.0.2 | jfreymuth/oggvorbis (Vorbis decode) |
| `github.com/mewkiz/flac` | v1.0.12 | gopxl/beep (FLAC decode) |
| `github.com/mewkiz/pkg` | v0.0.0-20230226050401-4010bf0fec14 | mewkiz/flac (FLAC decode) |
| `github.com/pkg/errors` | v0.9.1 | gopxl/beep (audio) |
| `github.com/rivo/uniseg` | v0.4.7 | grapheme segmentation (chroma/glyph) |
| `golang.org/x/image` | v0.44.0 | go-glyph (pure-Go rasterization) |
| `golang.org/x/mobile` | v0.0.0-20260602190626-68735029466e | gobind tool (Android AAR builds) |
| `golang.org/x/sync` | v0.22.0 | x/tools |
| `golang.org/x/text` | v0.40.0 | misc. text processing (transitive) |

## Updating

```bash
go get <module>@<version>
go mod tidy
```

Example:

```bash
go get github.com/go-gui-org/go-glyph@v1.13.0
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
