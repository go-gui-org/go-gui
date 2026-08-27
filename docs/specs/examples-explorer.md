# Examples explorer

Issue #438. Status: **planned** — not yet implemented.

## Goal

Build an `examples/explorer` (name bikesheddable: `example_gallery`) app that
lists every example in `examples/*`, shows its description, framework area, and
screenshot, and offers a **Run** button (`go run ./examples/<name>`) with
duplicate-run guard. Content is discovered from files on disk, not compiled in
as code assets:

- `examples/<name>/README.md` — single source of truth for the explorer card
  text and framework tags
- `examples/<name>/screenshot.png` — scanned at startup; generated headlessly
  where missing

## Context

- Repo has 62 runnable examples under `examples/` (bin excluded). Inventory from
  `ls -d examples/*/` on 2026-08-26: `2048`, `android_demo`, `animation_stress`,
  `animations`, `benchmark`, `blur_demo`, `calculator`, `color_picker`,
  `command_demo`, `context_menu`, `cursor_demo`, `custom_shader`,
  `data_grid_data_source`, `date_picker_options`, `dialogs`, `digital_rain`,
  `dock_layout`, `draw_canvas`, `floating_layout`, `fontviewer`, `get_started`,
  `gradient_demo`, `headless_png`, `ios_demo`, `key_up_demo`, `listbox`,
  `markdown`, `menu_demo`, `minesweeper`, `multi_window`, `multiline_input`,
  `native_menu`, `optical_centring`, `particles`, `process_monitor`,
  `rotated_box`, `rtf`, `scroll_demo`, `shadow_demo`, `showcase`, `snake`,
  `solar_system`, `solitaire`, `svg`, `svg_a11y`, `svg_aspect`,
  `svg_css_selectors`, `svg_css_states`, `svg_css_vars`, `svg_flatness`,
  `svg_gradient_spread`, `svg_hittest`, `svg_radial`, `svg_spinners`,
  `svg_use_symbol`, `system_tray`, `termgrid`, `time_travel`, `todo`,
  `vibrancy`, `virtual_list`, `web_demo`.

- Only 15 already have a `README.md`: `android_demo`, `headless_png`,
  `process_monitor`, `solar_system`, `svg_a11y`, `svg_aspect`,
  `svg_css_selectors`, `svg_css_states`, `svg_css_vars`, `svg_flatness`,
  `svg_gradient_spread`, `svg_hittest`, `svg_radial`, `svg_use_symbol`,
  `virtual_list`. Their READMEs are long-form docs, not gallery cards.

- Only 2 screenshot-like images exist on disk: `solar_system/screenshot1.png`
  `+ screenshot2.png` and `system_tray/icon.png`. No other example stores a
  preview.

- `examples/headless_png` and `gui/backend/soft` already prove the screenshot
  path: `soft.RenderToPNG(w, 2, out)` settles one frame with a software
  `TextMeasurer` + SVG parser and rasterizes `[]gui.RenderCmd` on CPU, no GPU or
  window. `soft` covers every render kind except `RenderCustomShader` (by design
  — GLSL has no CPU equivalent) — see
  `docs/specs/headless-software-rendering.md`. That matters for `custom_shader`,
  possibly `system_tray`/`native_menu`/`web_demo` which depend on OS chrome.

- `gui.Image` (`gui/view_image.go`) takes a local file path via `ImageCfg.Src`
  and validates with `os.Stat` — so `examples/<name>/screenshot.png` is a
  natural preview source with no embed.

- `examples/showcase` is the precedent for a two-pane catalog layout (300 px
  catalog + detail, scrollable lists, searchable, themed).

## Success Criteria

1. `go run ./examples/explorer` launches a go-gui window with a left pane
   listing all examples and a right pane showing selected example's title,
   `Framework:` tags, description, screenshot image and a **Run** button.
2. Data is not hard-coded: explorer scans `examples/*` at startup, reads each
   `README.md`'s explorer block and probes `screenshot.png` (with fallbacks) —
   adding/changing a README updates the gallery without code change.
3. Every example folder has a `README.md` whose first section is the explorer
   block. Existing 15 READMEs retain their full body below the new top block.
   Each README passes `make fmt-md`.
4. Every example folder that can render headlessly has a `screenshot.png`
   committed. Examples that cannot render headlessly have a placeholder image
   plus a README note explaining why (`custom_shader`, platform-guards). The
   explorer shows a graceful missing-image state rather than a log-spam error.
5. **Run** executes `go run ./examples/<name>` as a child process, reports
   starting/running/error states, disables itself while that example is already
   running, and cleans up on process exit. Works for `android_demo`/`ios_demo`
   by disabling Run with an explanatory note (gomobile/Xcode flow, not
   `go run`).
6. No new global state, no `replace` directive, `make check` + `make vet` clean,
   `go build ./examples/...` still reaches all examples.

## Architecture & Key Decisions

### Discovery vs. generated manifest

Two options were considered:

| Option                      | Mechanism                                                                                      | Tradeoff                                                                                                                                  |
| --------------------------- | ---------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| A — File discovery (chosen) | Explorer scans `examples/` at startup, parses `README.md` top section, checks `screenshot.png` | Zero build step for the gallery; adding an example is just adding its folder + README. Slightly more I/O at launch (< 63 reads, trivial). |
| B — Generated manifest      | `go generate` emits `explorer/manifest.go` from the READMEs                                    | Faster startup by ~1 ms, but every README edit needs a regenerate step; drifts unless enforced by CI.                                     |

Chosen: **A — file discovery**. Matches user's stated preference and keeps the
explorer stateless. A helper `discoverExamples(root string) []ExampleMeta` does
the scan — unit-testable without a Window.

### README schema — where the explorer reads, how authors write

Explorer block is **at the top**, before any existing prose, so `markdown`
rendering and humans both see it first (user requirement).

Proposed structure (for new files and as the prepended block on existing 15):

```markdown
# <Title>

> **Framework:** <comma-separated areas, e.g. "layout — dock, flex">
> **Description:** Single paragraph (2–3 sentences) — what the example shows and
> which framework part it demos. Simple English, 25 words max per sentence.

![Preview](screenshot.png)

<!-- explorer: tags=layout,animation category=graphics run=go -->

---

<existing README body follows, untouched except wrapped below the rule>
```

Rationale:

- `> **Framework:**` and `> **Description:**` are human-readable and trivial to
  parse with a prefix scan — no YAML front-matter parser, no `gopkg.in/yaml`
  dependency. The `<!-- explorer: ... -->` comment carries machine tags for
  filtering (`tags=`, `category=`, `run=...`) without affecting rendered
  markdown. `ergonomics-audit` ignores markdown; no new analyzer needed.
- Title comes from the `H1`; tags from the comment or the `Framework:` line as
  fallback. Description is the `Description:` line (fallback: first paragraph
  after H1).
- Screenshot path is the first `![](...)` after the description. Explorer
  resolves it relative to the example dir and feeds it to `gui.Image`.
- Existing 15 READMEs keep their full bodies verbatim after a `---` rule — the
  explorer stops parsing at the rule. `process_monitor`'s long doc stays; only
  the top block is added.

Alternative rejected: YAML front-matter (`---\ntags: ...\n---`) — breaks
`make fmt-md` assumptions and renders as a horizontal rule in GitHub, and
requires a YAML dep.

### Screenshot location & naming

Standard name: `examples/<name>/screenshot.png` (one file, retina scale `2`).

Scan order (first hit wins, to preserve `solar_system`'s existing files):

1. `screenshot.png`
2. `preview.png`
3. `screenshot1.png` (legacy — `solar_system` currently has 2)
4. `icon.png` / `*.png` fallback — only for legacy examples, not for new ones

Explorer maps `screenshot.png` to `gui.ImageCfg{ Src: absPath }`. Missing file
shows a themed placeholder (`gui.Label("No preview yet")` + border), not a
backend error toast.

Generation: `go run ./examples/explorer/cmd/screenshot` (or a script at
`examples/explorer/screenshot.go` invoked via `go run`) that iterates
`examples/*`, constructs each example's Window headlessly the way
`headless_png/main.go` does — `gui.SimpleWindow` with the example's `mainView`,
then `soft.RenderToPNG(w, 2, out)`. Each example's main is not imported
directly; instead the tool drives a per-example `render.RenderToImage` helper
that reuses the example's exported `MainView` where available or falls back to
importing the example package. Simpler variant: a shell loop that runs a tiny
generator per example that calls the example's own `mainView` after refactoring
each `main.go` to expose `MainView` — but the lowest-risk start is a standalone
`soft`-based snapshotting binary that imports each example package as a library
(examples are `package main`, so this needs a `view.go` extract or an `exec` of
a headless subcommand). The plan below picks the pragmatic path — see
Implementation Phase 1.

### Explorer app layout

```
examples/explorer/
  main.go          — NewWindow, App state, layout, discovery, Run button logic
  discover.go      — scan examples/, parse README top block, probe screenshot
  discover_test.go — table-driven tests with fake example dirs in /tmp
  runners.go       — exec tracking (map[string]*exec.Cmd, sync.Mutex)
  screenshot/      — headless capture tool (see Phase 1)
  README.md        — its own explorer block + how to run
```

Window structure (mirrors `showcase/catalog.go`):

- `State`:
  `{ Examples []ExampleMeta, Filter string, Selected string, Running map[string]bool, StatusMsg string }`
- Left pane: `gui.Column` 300–340 px, `ColorPanel`, `Input` for filter,
  `Column(Scrollable)` with selectable rows (`gui.Button` or `ListBox` rows).
  Shows count + filtered count. Rows keyed via `gui.ScopeIDN`.
- Right pane: `gui.Column(Scrollable)` detail — title `B3`, framework chips
  (`gui.Wrap` pill buttons), `gui.Markdown` for description (from the
  `Description:` paragraph), `gui.Image` for screenshot (fixed aspect, max
  height 420), `gui.TextButton` **Run** + `Stop` when running, status `Text`
  line, full README body below a `Separator` rendered via `gui.Markdown`.
- Theming via `gui.ThemeDark/Light` + `ThemePicker` as in `showcase`.

### Running an example

```
Run click → runners.Start(name) → exec.Command("go", "run", "./examples/"+name)
           → track in state.Running[name]=true → button becomes disabled + shows "Running…"
           → goroutine waits for cmd.Wait(), then clears Running, sets StatusMsg
Stop       → cmd.Process.Signal(os.Interrupt) / Kill fallback, then same cleanup
```

- Detection of "already running" is process-local: `Running` map, not a global
  lockfile. Two separate explorer instances may each run the same example; that
  is acceptable. Within one explorer window, second click is blocked and shows
  "Already running (pid X)".
- `android_demo`/`ios_demo` → button disabled with tooltip text explaining the
  gomobile/Xcode flow and linking to their READMEs.
- `custom_shader` → disabled when headless screenshot also fails, with note
  about GPU requirement — running itself still works on desktop; only the
  screenshot is headless-limited.
- Process output is not streamed into the explorer UI (scope cut); errors set
  `StatusMsg`. Future enhancement: pipe stdout to a scrollable log view.

### Screenshot generation specifics

`soft.RenderToPNG` requires a `*gui.Window` with a view generator. Each example
currently is `package main` with an unexported `mainView`. Options:

1. **Per-example export (preferred):** Move view construction to `view.go` with
   exported `BuildView(w *gui.Window) gui.View` or reuse existing `mainView` by
   renaming to `MainView` where trivial. Then the screenshot tool can import
   `examples/animations` etc. — but `package main` cannot be imported. So each
   example would need to become `package animations` with a `cmd` wrapper, which
   is invasive.

2. **Subprocess headless mode (pragmatic, chosen for Phase 1):** Each example
   gains a `-screenshot <path>` flag that, when present, builds its normal
   window and calls `soft.RenderToPNG` instead of `backend.Run`. The screenshot
   tool then simply runs `go run ./examples/<name> -screenshot screenshots.png`
   per example. This is additive, touches only `main.go` in each example, and
   works for `package main`. Examples that cannot run headlessly exit with a
   sentinel error and leave a placeholder.

3. **Recorded manual screenshots:** For phase 1, run each example interactively
   on macOS, capture with `screencapture`, resize to ~800 px width, commit.
   Slower but zero code churn. Use as fallback for examples where (2) proves
   fiddly.

Plan picks **(2) as primary, (3) as fallback**. The screenshot generation step
is a one-time upfront task per user request: "Currently, not all the examples
have screenshots so those have to be created up front."

### Categories / framework areas

Tags derived from existing code + README intent. Starter taxonomy (from
showcase's groups + example names):

`layout`, `animation`, `graphics/canvas`, `input`, `selection`, `data`, `text`,
`svg`, `theme`, `system` (dialogs, tray, native menu), `performance`, `platform`
(android/ios/web), `games`.

Each example gets 1–3 tags stored in `<!-- explorer: tags=... -->`. Explorer
offers a `Wrap` chip filter (All + tag chips) plus a text filter over title and
tags.

## Implementation Phases

### Phase 0 — Inventory (1–2 hours, no code)

- Confirm the 62-name inventory and mark un-runnable as examples: `bin`
  excluded, `android_demo`/`ios_demo` require mobile toolchains, `web_demo` is
  wasm.
- Draft the README explorer-block template and screenshot naming convention; get
  approval before mass edits.
- Decide explorer app name (`explorer` vs `gallery` vs `example_browser`);
  wireframes are a two-pane layout identical to `showcase`, so no new design doc
  needed.

### Phase 1 — Content: READMEs + screenshots (2–3 days, parallelizable)

**Step 1a — README authoring**

- Create a small Go program `scripts/explorer_readme.go` that scans
  `examples/*/main.go`, extracts a one-line package comment and imports to infer
  framework tags, and prints a draft `<!-- explorer: -->` block. Human reviews
  each draft — do not auto-commit tags.
- For the 47 examples with no README: create `README.md` with explorer block
  - 2–3 sentence description in simple English + placeholder sections (Run, What
    it demonstrates). Keep each under ~60 lines; detailed docs stay in code.
- For the 15 existing READMEs: prepend explorer block + screenshot markdown
  - `---` rule before the first existing line. Preserve all existing content
    verbatim. Run `make fmt-md` and verify `prettier` does not reflow the
    `explorer:` comment.

**Step 1b — Screenshot generation**

- Add `-screenshot` flag to each example's `main.go` (guarded, 8 lines):

  ```go
  var screenshot = flag.String("screenshot", "", "write screenshot and exit")
  flag.Parse()
  if *screenshot != "" {
      w := gui.SimpleWindow(title, width, height, state, func(w *gui.Window){ w.UpdateView(mainView) })
      if err := soft.RenderToPNG(w, 2, *screenshot); err != nil { log.Fatal(err) }
      os.Exit(0)
  }
  ```

  Alternatively centralize in a `headless` helper to avoid per-example imports
  of `soft`. Keep changes minimal and vet-clean.

- Build the capture script:
  `go run ./examples/explorer/cmd/capture --out screenshot.png --scale 2` which
  loops `examples/*/`, runs each example's screenshot mode, handles failures
  (writes `placeholder.png` from a checked-in 1×1 asset), and reports a summary
  table.

- Run capture on macOS (where Metal backend builds) or in headless CI; commit
  resulting `screenshot.png` files. Expected failures: `custom_shader` (needs
  GPU shader compilation in soft — correctly skipped),
  `system_tray`/`native_menu` (needs OS chrome, blank but valid), `multi_window`
  (captures main window only, note in README).

- Verify each PNG opens and is < 800 KB (resize if needed).

**Step 1c — Verification**

- `ls examples/*/README.md | wc -l` == 62
- `ls examples/*/screenshot.png | wc -l` == ~58–60 (with documented exceptions)
- `make fmt-md-check` passes.

### Phase 2 — Explorer app (3–4 days)

**Step 2a — Discovery layer**

- `discover.go`:
  `type ExampleMeta struct { Name, Title, Description, Framework string; Tags []string; ScreenshotPath, ReadmePath string; Runnable bool }`
- `Discover(root string) ([]ExampleMeta, error)`: reads `os.ReadDir(root)`,
  skips `bin`, sorts by name, for each dir reads `README.md`, parses explorer
  block (H1, `Framework:` line, `Description:` line, first image path,
  `<!-- explorer:` comment), probes screenshot candidates in order, sets
  `Runnable` false for `android_demo`/`ios_demo`/`web_demo` pattern. Caches
  nothing; call once at startup, re-call on a Refresh button.

- Tests: `discover_test.go` creates temp dirs with synthetic READMEs and PNGs,
  asserts parsing, missing-file fallbacks, and that `solar_system`'s legacy
  `screenshot1.png` is found.

**Step 2b — Runner layer**

- `runners.go`:
  `type Runner struct { mu sync.Mutex; cmds map[string]*exec.Cmd }` with
  `Start(name string) error` (checks `cmds[name]` still running via
  `ProcessState`), `Stop(name string) error`, `IsRunning(name string) bool`,
  `WaitAsync(name string, onDone func(error))`.
- Unit tests with a fake command (`sleep 0.1`) to verify guard semantics.

**Step 2c — UI**

- `main.go` assembles the two-pane layout described above, wires
  `discover.Discover("examples")` at `OnInit`, and renders:
  - Filter input (text + tag chips) — filters the left list.
  - Left list: `Column(Scrollable)` with rows; click sets `Selected`.
  - Right detail: title, framework chips, description text, `Image` (with
    placeholder on missing), `Button` Run/Stop, status label, and `gui.Markdown`
    rendering the rest of the README body (loaded on selection).

- Image path handling: explorer is run as `go run ./examples/explorer` so
  `examples/<name>/screenshot.png` is relative to repo root. Resolve via
  `filepath.Join("examples", meta.Name, filepath.Base(meta.ScreenshotPath))` or
  absolute via `os.Getwd()` — prefer repo-root-relative, test with
  `go run ./examples/explorer -root .`.

- Focus/ID discipline: each selectable row gets
  `ID: gui.ScopeID("explorer", "row", name)`, left and right scroll containers
  get distinct `ID`s, inputs get `ID`s — satisfies `requiredid` analyzer and tab
  order.

- Theme picker + `gui.SetTheme` as in `showcase`.

**Step 2d — Polish**

- Empty filter state: "No examples match".
- Missing screenshot: themed `Container` with `Label("Preview not available")`
  and the reason from `ExampleMeta`.
- Keyboard: up/down to change selection, Enter to Run.

### Phase 3 — Integration & CI (half day)

- Add `examples/explorer` to `build-examples` Makefile loop's allowlist and to
  `examples/bin` ignore (or explicitly include explorer binary but not its own
  screenshot).
- `go vet ./examples/explorer/...` and `golangci-lint run ./...` clean.
- Update root `README.md` to mention the explorer.
- Optional: `make explorer-screenshots` target that re-runs the capture tool.

## Testing Strategy

- **Discovery parsing:** table tests over synthetic `README.md` contents (happy
  path, missing explorer block, missing screenshot, malformed comment).
- **Runner guard:** start twice without stop → second returns `already running`;
  stop then start again → succeeds; non-runnable example → `Start` returns
  descriptive error.
- **Headless screenshots:** manual visual check of 3–4 captured PNGs for text
  and shape presence; CI cannot assert pixels beyond "file exists and > 5 KB".
- **Explorer smoke:** `go run ./examples/explorer -smoke` flag that runs
  `Discover`, asserts len == 62 and every meta has non-empty Title, then exits 0
  — usable in CI without opening a window.
- **No golden `TestGolden` impact:** explorer adds no `gui/` changes.

## Risks & Mitigations

| Risk                                                                    | Impact                                                   | Mitigation                                                                                                                                                                                                          |
| ----------------------------------------------------------------------- | -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Headless render misses GPU-only visuals (shadows, blur, custom shaders) | Screenshots look slightly different from interactive run | Documented in headless spec; capture still valid as preview. Flag truly GPU-dependent examples and use placeholder + note rather than misleading blank.                                                             |
| `package main` cannot be imported for screenshot tool                   | Per-example `-screenshot` flag touches 60+ files         | Keep flag change to 8 lines per file behind `if *screenshot != ""`; optionally centralize via a shared `explorer/screenshot` helper to reduce churn. Alternative is manual screencaps without code change.          |
| `go run` child processes outlive explorer if it crashes                 | Orphaned example windows                                 | Track `*exec.Cmd` and on `WindowCleanup` iterate `cmd.Process.Kill()`. Document that explorer does not sandbox child windows.                                                                                       |
| README block drifts from code (stale tags)                              | Explorer shows wrong framework area                      | The block is short; tags come from a fixed taxonomy and are reviewed like code. `discover` tests catch empty tags, not semantic drift.                                                                              |
| Screenshot files bloat repo (62 × ~400 KB ≈ 25 MB)                      | Clones slower, LFS debate                                | Compress PNGs (level 9, ~150–300 KB each → ~12 MB total). `large-files.sh` threshold is 800 lines for Go, not assets; still run a size audit and note in PR. No `go:embed` of screenshots into the explorer binary. |
| Missing images cause noisy `log.Printf` from `gui.Image`                | Log spam on detail pane                                  | Explorer probes existence before constructing `Image`; missing shows placeholder `Container`, no `Image` view emitted.                                                                                              |

## Open Questions

1. **App name:** `examples/explorer`, `examples/gallery`, or
   `examples/example_browser`? `explorer` is short but collides with OS
   terminology. Recommendation: `examples/explorer`.

2. **Screenshot scale:** `soft.RenderToPNG` scale `2` doubles pixels (Retina).
   Confirm desired committed size vs. runtime `ImageCfg` display width. Suggest
   capture at `2` and display with `Width` constrained to ~560, height auto —
   sharp on Retina, not huge on disk.

3. **README tag vocabulary:** Final tag set and whether to allow free-form
   framework text in addition to structured `tags=`. Proposal: both — human
   `Framework:` line for prose, machine `tags=` for filtering.

4. **Child process I/O:** Should explorer capture and display child
   stdout/stderr in a collapsible log pane? Cut from v1 to stay minimal; v2 can
   add.

5. **Scheduling:** Screenshots upfront means Phase 1 must finish before Phase 2
   can render images. If headless capture proves noisy, approve Phase 2 to start
   with placeholders so phases overlap.

6. **Mobile examples:** Should `android_demo`/`ios_demo` remain listed with a
   disabled Run button plus a "See README for build steps" link, or hidden
   entirely? Proposal: listed but disabled — keeps the inventory complete.

## What Is Not Included

- No changes to `gui/` core, no new linter rules, no backend switches.
- No embedding of screenshots into the binary; they stay as files under
  `examples/<name>/`.
- No web or mobile port of the explorer itself (desktop-only first).

## Verification Checklist

- [ ] `ls examples/*/README.md` → 62 files, each starting with explorer block
- [ ] `ls examples/*/screenshot.png` → present or placeholder with note
- [ ] `make fmt-md` / `make fmt-md-check` pass
- [ ] `go vet ./...` / `make lint` pass
- [ ] `go run ./examples/explorer -smoke` exits 0, reports 62 examples
- [ ] Manual run: filter, select, screenshot visible, Run launches example,
      second Run blocked, Stop kills it
- [ ] `go run ./examples/calculator -screenshot /tmp/x.png` produces a PNG for a
      sample example

---
