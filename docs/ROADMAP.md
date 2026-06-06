# Go-Gui Roadmap

Concrete, milestone-driven initiatives. Each independently shippable.
Milestones map to semver bumps from current v0.21.

---

## Initiative 1: Single-Binary Deploy — v0.22 ✅ (done)

**Problem:** Linux binaries need `libSDL2.so` at runtime — users must
`apt install libsdl2-dev` before running a go-gui app. This is the #1
adoption friction point for the Go community, which expects `go build` to
produce a self-contained binary.

**Goal:** True single-binary on all platforms. macOS already produces one
(Metal backend). Linux and Windows achieve it via go-sdl2's built-in static
linking (`-tags static`), which uses pre-compiled `.a` files bundled in the
go-sdl2 module — no SDL2 installation required.

### Current state (after v0.22)

| Platform | Backend  | Type          | Runtime deps              |
| -------- | -------- | ------------- | ------------------------- |
| macOS    | Metal    | Static CGo    | System frameworks only    |
| Linux    | GL       | Static CGo    | None (`-tags static`)     |
| Windows  | GL       | Static CGo    | None (`-tags static`)     |
| WASM     | Canvas2D | Single binary | None                      |

### 1.1 Static linking via go-sdl2

`veandco/go-sdl2` v0.4.40 ships pre-compiled static libraries in its
`_libs/` directory — `libSDL2_linux_amd64.a`, `libSDL2_windows_amd64.a`,
plus all transitive deps (mixer, ogg, vorbis, png, freetype, etc.).
The `static` build tag switches CGo directives from `pkg-config: sdl2`
(dynamic) to explicit `-lSDL2_linux_amd64` (static).

No SDL2 compilation, no env vars, no system packages:

```sh
# Linux — single static binary
go build -tags static ./examples/showcase/

# Windows — static .exe (cross-compile from Linux/macOS)
CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc \
  go build -tags static ./examples/showcase/
```

### 1.2 Root Makefile

Created `/Makefile`:

```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS  = -X github.com/go-gui-org/go-gui/gui.Version=$(VERSION) \
           -X github.com/go-gui-org/go-gui/gui.Commit=$(COMMIT)

CC_WINDOWS ?= x86_64-w64-mingw32-gcc
STATIC_TAG  = static

.PHONY: build-linux build-windows build-macos build-wasm release clean

build-linux:
	CGO_ENABLED=1 \
	go build -tags $(STATIC_TAG) -ldflags "$(LDFLAGS)" \
	  -o build/showcase-linux ./examples/showcase/

build-windows:
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 CC=$(CC_WINDOWS) \
	go build -tags $(STATIC_TAG) -ldflags "$(LDFLAGS)" \
	  -o build/showcase-windows.exe ./examples/showcase/

build-macos:
	go build -ldflags "$(LDFLAGS)" \
	  -o build/showcase-macos ./examples/showcase/

build-wasm:
	GOOS=js GOARCH=wasm \
	go build -ldflags "$(LDFLAGS)" \
	  -o build/showcase.wasm ./examples/showcase/
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" build/

release: build-linux build-windows build-macos build-wasm
	tar czf build/go-gui-showcase-$(VERSION)-linux-amd64.tar.gz \
	  -C build showcase-linux
	cd build && zip go-gui-showcase-$(VERSION)-windows-amd64.zip \
	  showcase-windows.exe
	cd build && go run ../cmd/buildapp -version $(VERSION) \
	  -name "Go-Gui Showcase" showcase-macos
	hdiutil create -srcfolder "build/Go-Gui Showcase.app" \
	  -volname "Go-Gui Showcase $(VERSION)" \
	  -format UDZO "build/Go-Gui-Showcase-$(VERSION).dmg"

clean:
	rm -rf build/
```

Key points:
- **`-tags static`** on Linux and Windows — zero setup, no SDL2 installation.
- **Windows .exe is fully static** — no `SDL2.dll` bundled.
- **macOS** needs no tag (Metal backend, system frameworks).
- **`go run ./cmd/buildapp`** for macOS `.app` bundles.
- **`CC_WINDOWS`** defaults to mingw cross-compiler; override with `gcc` on
  MSYS2.
- Website deploy step (from original roadmap) deferred to v0.24.

### 1.3 Version injection

New file `/gui/version.go`:

```go
package gui

// Build-time values injected via -ldflags.
var (
    Version = "dev"
    Commit  = "unknown"
)
```

Showcase window title includes `gui.Version`.

### 1.4 CI release workflow

New file `/.github/workflows/release.yml`:

Triggers: `v*` tags + `workflow_dispatch`. Matrix across `ubuntu-latest`,
`macos-latest`, `windows-latest`.

- **Linux**: checkout → `make build-linux` → `ldd` verify no `libSDL2.so` →
  package `.tar.gz` → upload.
- **macOS**: checkout → `make build-macos` → build `.app` via `cmd/buildapp`
  → create `.dmg` → upload.
- **Windows**: MSYS2 setup → `make build-windows CC_WINDOWS=gcc` → package
  `.zip` → upload.

No SDL2 compilation step needed — go-sdl2's `_libs/` handles it.

### 1.5 Files

| File | Action | Purpose |
| ---- | ------ | ------- |
| `/Makefile` | Create | Root build + release targets |
| `/gui/version.go` | Create | `Version`/`Commit` for ldflags |
| `/.github/workflows/release.yml` | Create | Multi-platform release workflow |
| `/examples/showcase/main.go` | Modify | Window title includes `gui.Version` |
| `/.gitignore` | Modify | Add `build/` |
| `/README.md` | Modify | Add "Building the Showcase" section |

### 1.6 Verification

```
make build-linux && ldd build/showcase-linux   # no libSDL2.so references
make build-windows && file build/showcase-windows.exe  # PE32+ static .exe
make build-macos && ls build/*.app build/*.dmg  # bundle + disk image
```

---

## Initiative 2: Stdlib Type Binding — v0.23 ✅ (done)

**Problem:** Most widgets require manual conversion from Go application
data to widget-specific structs. A `[]string` needs a loop to become
`[]ListBoxOption`. This is boilerplate Go devs resent — they want the
framework to accept their types directly.

**Goal:** Accept Go stdlib types (`[]string`, `[][]string`,
`[]map[string]string`) directly in widget Cfg structs. The existing typed
fields remain for complex cases (custom IDs, per-row styling, lazy
loading). When both a stdlib field and a typed field are set, the stdlib
field wins.

### 2.1 Selection widgets: `Items []string`

**Widgets:** ListBox, RadioButtonGroup, Select, Combobox

Select and Combobox already accept `Options []string` — no code change,
documentation only.

**ListBoxCfg** (`gui/view_listbox.go`):
```go
type ListBoxCfg struct {
    // ...existing fields...

    // Items is a convenience field for simple string lists. Each string
    // becomes a ListBoxOption with ID==Name==Value. When set, Items
    // takes precedence over Data.
    Items []string
}
```

**RadioButtonGroupCfg** (`gui/view_radio_button_group.go`):
```go
type RadioButtonGroupCfg struct {
    // ...existing fields...

    // Items is a convenience field for simple string lists. Each string
    // becomes a RadioOption with Label==Value. When set, Items takes
    // precedence over Options.
    Items []string
}
```

### 2.2 Table: `RawData [][]string`

**TableCfg** (`gui/view_table.go`):

`TableCfgFromData([][]string)` already exists at line 672. Add a field
that delegates to it:

```go
type TableCfg struct {
    // ...existing fields...

    // RawData is a convenience field for CSV-style data. First row is
    // treated as the header. When set, RawData takes precedence over Data.
    // Equivalent to calling TableCfgFromData(RawData) and assigning to Data.
    RawData [][]string
}
```

### 2.3 Tree: `ItemPaths []string`

**TreeCfg** (`gui/view_tree.go`):

```go
type TreeCfg struct {
    // ...existing fields...

    // ItemPaths is a convenience field for flat path strings. Each
    // string is slash-separated ("a/b/c") and auto-expanded into nested
    // TreeNodeCfg nodes. Duplicate path prefixes are merged. When set,
    // ItemPaths takes precedence over Nodes.
    ItemPaths []string
}
```

Internal helper `itemPathsToNodes(paths []string) []TreeNodeCfg`:
- Split each path on `/`
- Walk or create nodes for each segment
- Deduplicate common prefixes (`"a/b/c"` + `"a/b/d"` → single `"a"` → `"b"` node with two children)

### 2.4 DataGrid: `RowsData []map[string]string`

**DataGridCfg** (`gui/view_data_grid.go`):

```go
type DataGridCfg struct {
    // ...existing fields...

    // RowsData is a convenience field for key-value row data. Map keys
    // must match Column IDs. When set, RowsData takes precedence over
    // Rows. If Columns is empty, column definitions are auto-generated
    // from sorted keys of the first map entry (default width 150px).
    RowsData []map[string]string
}
```

Column order uses `slices.Sorted(maps.Keys(data[0]))` for determinism.
Users override width/editor by setting `Columns` explicitly alongside
`RowsData`.

### 2.5 Documentation

Add to widget doc comments and README a "Stdlib Data Binding" section:

> ### Stdlib Data Binding
>
> Data widgets accept Go stdlib types via convenience fields. This is the
> zero-configuration path:
>
> - `Items []string` for list-type widgets when display text equals the value
> - `RawData [][]string` for Table when data comes from CSV or a spreadsheet
> - `ItemPaths []string` for Tree when data is a flat list of path strings
> - `RowsData []map[string]string` for DataGrid when data is a slice of
>   key-value maps
>
> When both the stdlib field and the typed struct field are set, the stdlib
> field takes precedence. Use the typed field when you need IDs different
> from display text, per-row styling, lazy loading, or custom cell editors.

### 2.6 Files

| File | Action | Purpose |
| ---- | ------ | ------- |
| `gui/view_listbox.go` | Modify | `Items []string` field + conversion |
| `gui/view_radio_button_group.go` | Modify | `Items []string` field + conversion |
| `gui/view_table.go` | Modify | `RawData [][]string` field |
| `gui/view_tree.go` | Modify | `ItemPaths []string` + `itemPathsToNodes()` |
| `gui/view_data_grid.go` | Modify | `RowsData []map[string]string` + auto-columns |
| `gui/view_select.go` | No change | Already `Options []string` — doc only |
| `gui/view_combobox.go` | No change | Already `Options []string` — doc only |

### 2.7 Verification

```
go test ./gui/... -run "TestListBox|TestRadio|TestTable|TestTree|TestDataGrid"
```

New test cases per widget: verify display text, event handling (select/click
returns expected values), and that typed field is ignored when stdlib field
is set.

---

## Initiative 3: Downloadable Showcase Binaries — v0.24 ✅ (done)

**Problem:** There is no way to try go-gui without cloning the repo and
installing SDL2. No pre-built binaries, no live demo, no download links.
This is the single biggest adoption barrier — Go devs won't invest in a
GUI framework they can't evaluate in 30 seconds.

**Goal:** One-click access to the showcase on all four platforms. WASM in
the browser (zero install), native binaries for macOS/Linux/Windows.

### 3.1 WASM Showcase on GitHub Pages

Highest priority — zero install, works in any browser, instant evaluation.

**New workflow** `/.github/workflows/deploy-showcase.yml`:

```yaml
name: Deploy Showcase
on:
  push:
    branches: [main]
    paths: ['examples/showcase/**', 'gui/**', '.github/workflows/deploy-showcase.yml']
  workflow_dispatch:

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.26'
      - name: Build WASM
        run: |
          cd examples/showcase
          GOOS=js GOARCH=wasm go build -ldflags \
            "-X main.version=$(git describe --tags --always)" \
            -o main.wasm .
          cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" .
      - name: Deploy to org website
        uses: peaceiris/actions-gh-pages@v4
        with:
          deploy_key: ${{ secrets.WEBSITE_DEPLOY_KEY }}
          external_repository: go-gui-org/go-gui-org.github.io
          publish_dir: ./examples/showcase
          destination_dir: showcase
```

Result: `https://go-gui-org.github.io/showcase/`

Desktop downloads at `https://go-gui-org.github.io/dl/`.

### 3.2 Desktop downloads

Built by the release workflow from Initiative 1, triggered on `v*` tags.
Artifacts copied to the website repo and served at
`https://go-gui-org.github.io/dl/`.

| Platform | Format | Contents |
| -------- | ------ | -------- |
| macOS | `.dmg` | `Go-Gui Showcase.app` bundle (Metal backend) |
| Linux | `.tar.gz` | Statically-linked `showcase-linux` binary |
| Windows | `.zip` | `showcase-windows.exe` (static, no DLL) |

Also published as GitHub Release artifacts on the go-gui repo.

### 3.3 Download table in README

Add to `README.md`:

| Platform | Download |
| -------- | -------- |
| Browser (WASM) | [**Open Showcase**](https://go-gui-org.github.io/showcase/) |
| macOS | [Go-Gui-Showcase-\<version\>.dmg](https://go-gui-org.github.io/dl/) |
| Linux | [Go-Gui-Showcase-\<version\>-linux-amd64.tar.gz](https://go-gui-org.github.io/dl/) |
| Windows | [Go-Gui-Showcase-\<version\>-windows-amd64.zip](https://go-gui-org.github.io/dl/) |

### 3.4 Website download page

Add a downloads section to `../go-gui-org.github.io/index.html` listing
each platform with a direct link to the artifact in `/dl/`. The page is
hand-authored; the Makefile only copies artifacts into the repo — the
website itself is committed and pushed separately.

### 3.5 Showcase smoke test

Extend `examples/showcase/showcase_test.go`:

```go
func TestAllDemosRender(t *testing.T) {
    // Iterate every catalog entry, render one frame, verify no panic
    // and non-nil Layout tree.
}
```

Catches regressions before a release ships broken binaries.

### 3.6 Files

| File | Action | Purpose |
| ---- | ------ | ------- |
| `/.github/workflows/deploy-showcase.yml` | Create | GitHub Pages WASM deployment |
| `/README.md` | Modify | Download table + demo badge |
| `/examples/showcase/main.go` | Modify | Version in window title (from ldflags) |
| `/examples/showcase/showcase_test.go` | Extend | Smoke tests |
| `../go-gui-org.github.io/index.html` | Modify | Download links section |
| `../go-gui-org.github.io/dl/` | Create | Directory for release artifacts |

### 3.7 Verification

- Push to `main` → GitHub Actions builds and deploys WASM → open browser
  at `go-gui-org.github.io/showcase/` → showcase loads and renders.
- Run `make release` → artifacts land in `../go-gui-org.github.io/dl/` and
  `../go-gui-org.github.io/showcase/` → commit + push website repo →
  downloads available at `go-gui-org.github.io/dl/`.
- Create tag `v0.24.0` → release workflow builds all platforms → GitHub
  Release has `.dmg`, `.tar.gz`, `.zip` artifacts → each runs on its
  target OS.

---

## Future

Items from the old roadmap not yet implemented, plus new directions.

### Media

- **Embedded video/audio** — native media playback widget. Requires
  platform backends (AVPlayer on macOS, GStreamer or PipeWire on Linux,
  Media Foundation on Windows).

### Autocomplete / suggestion list

Extend `InputCfg` with `Suggestions func(string) []string` (debounced
callback). Renders a floating dropdown below the input, navigable by
arrow keys. Partially covered by Combobox for static option lists;
autocomplete handles dynamic/suggestion scenarios.

### Native dark/light mode sync

Auto-switch theme to follow OS appearance preference. Requires:
- `ThemeAuto` mode in the theme system
- `NativePlatform.OSThemePreference()` on each backend
- macOS: `NSApp.effectiveAppearance`
- Linux: `gsettings get org.gnome.desktop.interface color-scheme`
- Windows: registry `AppsUseLightTheme`

### Charting / graphing

Separate `go-charts` package built on go-gui. All framework prerequisites
are complete (canvas view, retained geometry, text measurement, clipping,
mouse events, gradients, animation, custom shaders).

### Community & adoption

- **Contribution guide**: update `CONTRIBUTING.md` with new Makefile targets
- **Issue templates**: add `.github/ISSUE_TEMPLATE/` forms for bugs and
  feature requests
- **GoReleaser**: evaluate for v0.25+ once Makefile release pipeline is
  stable. Right now the CGo + static SDL2 path needs explicit control;
  GoReleaser adds abstraction when it's no longer needed.

---

## Dependency Graph

```
v0.22 (Single-Binary Deploy) ✅
  ├── go-sdl2 -tags static (no SDL2 install needed)
  ├── Root Makefile
  ├── Version injection
  └── CI release workflow
        │
        ├── unblocks ── v0.24 (Showcase Binaries) ✅
        │                 ├── macOS .dmg
        │                 ├── Linux .tar.gz
        │                 ├── Windows .zip
        │                 └── WASM on GitHub Pages
        │
        └── parallel with ── v0.23 (Stdlib Binding) ✅
                              ├── ListBox / RadioButtonGroup Items
                              ├── Table RawData
                              ├── Tree ItemPaths
                              └── DataGrid RowsData
```

