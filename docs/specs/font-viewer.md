# Font Viewer

Status: **implemented** — `examples/fontviewer/` (main.go + tests). Shipped
from this spec; see the feature table for P0 coverage.

Browsable system-font catalog. Type sample text; every installed font family
renders it in a scrollable card grid. Filter by name, adjust preview size,
click to copy the family name.

**What it stresses (be precise):** because the grid is virtualized (P0), it
does **not** shape all N families at once — it exercises per-family
*resolution*, wrapping, and glyph-atlas growth *as you scroll* (the windowing +
cache-warming path), not simultaneous N-family layout. An optional
`--shape-all` debug mode (P2) drops virtualization to shape **all N families in
one frame** (still main-thread — "concurrent" would be wrong; go-glyph shaping
is single-threaded). The default path deliberately never does. (shirei's
reference prewarms off-thread; this port cannot — main-thread only — so
scroll-windowing is the substitute, not a background prewarm.)

Reference: `go-shirei/examples/fontviewer` — same feature set, ported to
go-gui's widget model (immediate-mode `Layout` tree, not shirei's
closure-based DSL).

## Feature Summary

| #   | Feature                                                            | Priority |
| --- | ------------------------------------------------------------------ | -------- |
| 1   | Enumerate installed fonts (same catalog go-glyph resolves), sorted | P0       |
| 2   | Filterable card grid — type to narrow by family name               | P0       |
| 3   | Editable sample text — type or shuffle                             | P0       |
| 4   | Adjustable preview size (12–72 px slider)                          | P0       |
| 5   | Click card to copy family name to clipboard                        | P0       |
| 6   | Grid virtualization — window fixed-size cards to the visible range  | P0       |
| 7   | Visible-range / spacer-math test (grid correctness, not optional)  | P0       |
| 8   | "Copied" transient confirmation on the clicked card                | P1       |
| 9   | State tests (empty, filtered, copied, sizing)                      | P1       |
| 10  | `--shape-all` debug mode — drop virtualization, shape all N (stress) | P2       |
| 11  | Skeleton on cold scroll-in (optional cosmetic)                     | P2       |
| 12  | Headless PNG export (`--png out.png`) — speculative                | P2       |

**Phasing note (why #6 is P0):** go-gui's layout shapes text for _every_ card
in the tree, with no scroll-visibility gate (`layoutWrapTextWalk`,
`layout_pipeline.go`); scroll offset is applied later, in the positioning
phase (`layout_position.go`). Emitting all 500–800 cards would cold-shape
every family on the first frame — a hard stall. Virtualization emits only the
visible window, so per-frame cost is **O(visible), not O(N)**, and it reuses
go-gui's tested `listCoreVisibleRange` primitive (the same one behind
`ListBox`, `Tree`, and `Table`). Fixed card size is the precondition it needs.

## Non-goals (v1)

Explicit so the feature table is not read as a finished font browser:

- **Desktop only.** `ListSystemFonts` returns nil where no `FontLister` backend
  exists (WASM stub), so the web build shows the empty state permanently.
  Browser enumeration (`FontFace` / `queryLocalFonts`) is a separate future
  phase, not a v1 gap to paper over.
- **Regular weight only.** Previews resolve the bare family key, i.e. the
  Regular face; per-weight/italic preview is out of scope.
- **Mouse-first.** Cards are click-to-copy; no roving tab-stop across the grid
  and a11y is whatever the containers default to (family names are not exposed
  to assistive tech). **v1 is intentionally inaccessible**; a keyboard-navigable
  catalog with proper `AccessRole`s is a later increment.
- **"No fonts" copy is coarse.** A nil `FontLister` (WASM / not-yet-ready
  backend) and a genuinely empty catalog both show "no system fonts"; the view
  does not distinguish "not ready yet" from "backend has no lister." Acceptable
  given desktop-only scope — revisit if the web build ships.

| Topic                    | Decision                                                                                                                                                                                                                                                                               |
| ------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Enumeration source       | go-glyph's discovered font catalog (system discovery + app-registered fonts), surfaced through a **clean family set captured at registration time** — **not** a parallel CTFontManager / fontconfig / DirectWrite / AWT list, and **not** a reverse-parse of the resolution map's keys |
| App fonts                | Families from `RegisterAppFont` / `AddFontFile` flow through the same `registerFontPath` and land in the same set. **P0 contract:** fonts registered *before the window runs* (the normal demo flow) are listed. Fonts registered *after* the first successful enumeration are invisible until a refresh (`s.Loaded = false`) — that refresh is P3, and this limit must be stated, not left implied |
| Hidden / generic keys    | Excluded **at the source**: leading-`"."` families and generic aliases (`sans-serif`, `serif`, `monospace`, …) never enter the family set                                                                                                                                              |
| Style variants           | A face's `-Regular/-Bold/-Italic/-BoldItalic` resolution keys are **not** families; the set stores only the clean display name, so `Arial` appears once, not four times                                                                                                                |
| go-gui ↔ go-glyph seam   | Optional `FontLister` capability interface + type assertion — **not** a widened mandatory `TextMeasurer` interface                                                                                                                                                                     |
| `LayoutText` concurrency | **Not safe.** go-glyph `doc.go` documents `Context` / `Renderer` / `TextSystem` / `GlyphAtlas` as _"Not safe for concurrent use. Call all glyph methods from the main/render thread."_ All shaping stays on the main thread                                                            |
| Scaling strategy         | **Grid virtualization.** Read the previous frame's scroll offset (`w.ScrollY().Get(id)`), compute the visible row range, emit only those rows padded by top/bottom spacer rects. O(visible) per frame. Same pattern as `ListBox` / `Tree` / `Table`                                     |
| Visible-range primitive  | **Export `func ListVisibleRange(itemCount int, rowHeight, listHeight, scrollY float32, overscan int) (first, last int)`** from `gui`. Wraps the tested core but takes the overscan **as an argument** (the internal `listCoreVirtualBufferRows` is not stacked on top), so the caller sets one buffer number, not 2+4. The example is `package main` and cannot call the unexported original; exporting beats duplicating the arithmetic. Scope is **only** the arithmetic helper — the grid's column/spacer math stays in the example; do not let Phase 1 grow a general virtualized-grid widget |
| **Grid drives BOTH its dimensions** | The grid `Column` is **`Sizing: FixedFixed`** with explicit `Width: outerW` and `Height: listH` — the *same* numbers feed the `cols`/`ListVisibleRange` math **and** the arranged box, so neither axis can disagree with layout. Height alone was not enough: width computed from `WindowSize` but arranged by `Fill` is the identical X-axis footgun (wrong `cols` → overflow / dead space). No `FindByID` width-readback API exists, so making width explicit is the fix. `listH` is clamped `>= rowH` (`listCoreVisibleRange` yields no rows at `<= 0`) |
| Row geometry             | The two axes carry gap **differently**: the grid `Column` sets `Spacing: SomeF(0)` (vertical spacing must be 0 or it corrupts the spacer arithmetic — `rowH` alone accounts for the vertical `gap`); each `Row` sets `Spacing: SomeF(gap)` to draw the **horizontal** gutter between cards (this is where the `(cols-1)*gap` that `cardW` subtracts actually goes — zero it and cards abut with dead space on the right). Each `Row` is **exactly `rowH` tall** (`cardH` + vertical `gap` folded in) so `top + rows + bottom == totalRows*rowH`; trailing `gap` on the last row is an intentional bottom margin |
| Container chrome         | Root and grid `Column`s set `Padding: NoPadding`, `Spacing: SomeF(0)`, **and `SizeBorder: Some(0)`** — `DefaultContainerStyle` is `PaddingMedium` + `SpacingMedium` + `SizeBorderDef` (1.5), and `paddingHeight()` counts `2·SizeBorder`, so an un-zeroed box eats `2·pad + spacing + 2·border` (~6 px of border alone across root+grid) and `listH`/`outerW` go optimistic → over-height root / wrong `cols`. `sidePad` is the grid's **real** horizontal `Padding` (right side widened to clear the overlay scrollbar), folded into `contentW` |
| Hover under virtualization | **Sticky-`Copy` footgun.** `layoutMouseLeave` walks only the *current* tree and needs a non-empty `shape.ID`; a hovered card scrolled out of the window is never visited, so its `OnMouseLeave` never fires. Rule: (a) every card gets a **stable ID** `"card:"+family` (also required for click/leave identity), and (b) the retained `hoveredFam` is cleared each frame if it is **not in the emitted `[first,last]` window**. Without (b) the "Copy" affordance reappears when the family scrolls back |
| Card grid layout         | **Manual column math over fixed-size cards.** `gui.Wrap` lays out _all_ children and can't window, so it is not usable for a virtualized grid; compute `cols` from viewport width each frame instead                                                                                    |
| Family de-duplication    | Fold case when building the family set (canonical first-seen display case, keyed by `strings.ToLower`) so two faces reporting `Arial` / `arial` list once — matches shirei                                                                                                             |

## Architecture

### State

```go
type FontViewerState struct {
    Sample   string   // current sample text (init: a random pangram)
    Filter   string   // case-insensitive family-name substring
    FontSize float32  // preview size in px (12–72; init: 28)
    Families []string // all discovered family names, sorted (from ListSystemFonts); may be nil
    Loaded   bool     // families have been enumerated (backend was ready)

    CopiedFam   string  // family whose "Copied" badge is showing ("" = none)
    CopyOpacity float32 // 1→0, written by the fade tween's OnValue, read by the card view
    HoveredFam  string  // family under the pointer ("" = none); cleared on eviction
}
```

Initialize `FontSize: 28` and `Sample:` a random pangram (matching shirei) in
the `WindowCfg{State: ...}` seed. `CopiedAt` is intentionally **absent** — the
tween owns the fade's timing and `OnDone` clears `CopiedFam`, so no timestamp
is read anywhere.

Virtualization needs no per-family state: the scroll offset lives in go-gui's
scroll store (keyed by the grid's `ID`), and the visible range is derived from
it each frame. There is no `Ready` map and no batch-reveal pass — windowing
bounds measurement structurally, so nothing has to be staged in.

`CopyOpacity` is the **state channel** the badge fade needs: an animation's
`OnValue` runs outside the view, so the animated value must land in a field the
view reads (the pattern `view_toast.go` uses with `animFrac`).

Per-window via `gui.State[FontViewerState](w)`. No package-level globals.
Any deferred work uses `w.QueueCommand` to mutate state and `w.Ctx()` for
cancellation on window close. No background goroutine ever touches the text
system.

### FontFamily

Family names are just `string`s returned by `ListSystemFonts(w)`. No struct
needed — the viewer only needs the name to pass to `TextStyle.Family`.
`ListSystemFonts` returns **nil** before backend init (and on WASM stubs), so
`state.Families` may be nil; `filterFontFamilies` and every `range` over it must
be nil-safe (ranging nil is fine; the filter returns nil for a nil input).

### View function skeleton

Ties enumeration timing, the filter, and the virtualized grid together. The
view runs under `w.mu`, so it only calls lock-free reads (`ListSystemFonts`,
`w.ScrollY().Get`, `w.WindowSize`).

```go
const (
    gridID       = "font-grid"
    cardMaxW     = 380 // fixed nominal card width (px, layout units)
    gap          = 16  // gutter between cards; also folded into rowH
    sidePad      = 24  // grid horizontal padding
    scrollbarW   = 14  // deliberate slack to clear the overlay thumb, not an
                       // engine constant: DefaultScrollbarStyle Size(7)+GapEdge(3)
                       // ≈10px footprint, rounded up. Derive from the style if it changes
    nameRowH     = 28  // name row + its padding (the fixed part of cardH)
    previewPad   = 24  // preview box vertical padding (top+bottom)
    previewLines = 3   // preview clamps to this many lines
    lineFactor   = 1.4 // engine line-height factor (matches listCoreRowHeightEstimate)
    headerH      = 72  // fixed header bar height — must equal header()'s
    toolbarH     = 104 // fixed two-row toolbar height — must equal toolbar()'s
    overscanRows = 4   // grid's own overscan (shared const is only 2 — see Rendering)
    minSample    = 12
    maxSample    = 72
)

// cardHeight is uniform per FontSize. Uses the engine's 1.4× line height, not
// a flat previewLines*FontSize (which under-counts by ~40% and would clip).
func cardHeight(fontSize float32) float32 {
    return nameRowH + previewPad + previewLines*fontSize*lineFactor
}

func fontViewer(w *Window) View {
    s := State[FontViewerState](w)

    // Lazy one-time enumeration. ListSystemFonts reads a pre-built set
    // (cheap, no shaping) and sorts once. nil until the backend is ready →
    // retry next frame. Fonts registered via RegisterAppFont *after* this
    // succeeds stay invisible until a refresh sets s.Loaded = false again
    // (see open question on late registration).
    if !s.Loaded {
        s.Families = ListSystemFonts(w)
        s.Loaded = s.Families != nil
    }

    matches := filterFontFamilies(s.Families, s.Filter) // nil-safe

    // Zero ALL inherited chrome (default is PaddingMedium + SpacingMedium +
    // SizeBorderDef 1.5) so listH = winH - headerH - toolbarH is exact — not
    // off by 2*pad + spacing + 2*border.
    return Column(ContainerCfg{
        Sizing:     FillFill,
        Padding:    NoPadding,
        Spacing:    SomeF(0),
        SizeBorder: Some(float32(0)),
        Content: []View{
            header(),
            toolbar(w, s, len(matches)),
            fontGrid(w, s, matches),
        },
    })
}
```

### View Tree

```
RootView (FillFill column)
├── Header (title + subtitle bar)
├── Toolbar (sample text, shuffle, filter, size slider, match count)
└── FontGrid (Column + Scrollable + ID)   ← virtualized
    ├── top spacer rect        (height = firstRow * rowH)
    ├── Row × (visible rows)                ← only [firstRow, lastRow] emitted
    │   └── FontCard × cols (fixed W × H)
    │       ├── Name row (UI font + copy badge)
    │       └── Preview box (sample text in family's font)
    └── bottom spacer rect     (height = (rows-1-lastRow) * rowH)
```

### Widget Mapping (shirei → go-gui)

| shirei                               | go-gui equivalent                                                                                                          |
| ------------------------------------ | -------------------------------------------------------------------------------------------------------------------------- |
| `Container(Attrs(...), func(){...})` | `Row`/`Column`/`Wrap(ContainerCfg{...})` (grid rows use `Row`, not `Wrap` — see FontGrid)                                  |
| `Label(text, Fonts(name), ...)`      | `Text(TextCfg{Text: text, TextStyle: ts})` with `ts.Family = name`                                                         |
| `TextInputExt(&var, attrs)`          | `Input(InputCfg{ID: "...", Text: state.Sample, OnTextChanged: fn})` — no `&var`                                            |
| `Slider(&var, SliderAttrs{...})`     | `Slider(SliderCfg{ID: "...", Value: state.FontSize, Min: 12, Max: 72, Step: 1, OnChange: fn})`                             |
| `Button(label, "tooltip")`           | `Button(ButtonCfg{ID: "...", Content: ..., OnClick: fn})`                                                                  |
| `VirtualListView(nil, rows, ...)`    | Manual grid virtualization: `Column{Scrollable, ID}` + `listCoreVisibleRange` → emit visible rows only, padded by top/bottom spacer rects (same pattern as `ListBox` / `Table`) |
| `RequestTextCopy(text)`              | `w.SetClipboard(text)`                                                                                                     |
| `Icon(SymCopy, ...)`                 | Feather has no `IconCopy` — use a text "Copy"/"Copied" badge (add a glyph to `gui/fonts.go` only if a real icon is wanted) |
| `Icon(SymShuffle, ...)`              | `Text(TextCfg{Text: gui.IconSync, TextStyle: iconStyle})`                                                                  |
| `PressAction()`                      | `OnClick` (containers default to `ClickButton: MouseLeft`)                                                                 |
| `IsHovered()` + `ModAttrs(...)`      | `OnHover` / leave handling on `ContainerCfg`; call `w.RequestRedraw()` if needed                                           |
| `RequestNextFrame()`                 | `w.QueueCommand(...)` (wakes main) or `w.UpdateWindow()` / `w.RequestRedraw()`                                             |

**Callback signatures diverge — do not assume the generic `func(*Layout,
*Event, *Window)`:**

- `InputCfg.OnTextChanged` is `func(*Layout, string, *Window)` — the new text
  arrives as the second argument.
- `SliderCfg.OnChange` is `func(float32, *Event, *Window)` — the new value is
  the first argument.

### Font Enumeration — capture a clean family set, don't reverse-parse

go-gui has no public font list today. go-glyph **already** discovers system
fonts into `Context.fontPaths` via directory walks (`discover_darwin.go`,
`discover_linux.go`, `discover_windows.go`, `discover_android.go`) and resolves
`TextStyle.Family` against that map. It does **not** use CTFontManager,
fontconfig, DirectWrite, or AWT.

**`fontPaths` is a resolution map, not an enumeration source.**
`registerFontPath` inserts _multiple keys per face_: a style key
`family + styleSuffix` (`Arial-Regular`, `Arial-Bold`, `Arial-Italic`,
`Arial-BoldItalic`), the bare `family` key, and lowercase generic aliases.
Iterating those keys and string-deduping yields duplicate, style-mangled
"families" (`Arial` next to `Arial-Bold`) — a naive list is wrong by
construction.

**Correct approach — capture the clean name where it is already known.**
`registerFontPath` receives the clean `family` _before_ the style suffix is
appended. But it is a **free function** (`registerFontPath(fontPaths,
keyWeights, family, aspect, path)`), not a `Context` method — so thread a
`families` set through it and add the entry at both call sites (the
`AddFontFile` path and `fontScan.consider`). Aliases are inserted into
`fontPaths` **outside** `registerFontPath`, so they never reach the set — the
exclusion is structural, not a filter.

   ```go
   // Context gains a case-folded set: lower(family) -> first-seen display case.
   families map[string]string

   // registerFontPath signature gains the set; both existing call sites pass
   // ctx.families / s.ctx.families:
   func registerFontPath(fontPaths map[string]string, keyWeights map[string]font.Weight,
       families map[string]string, family string, aspect font.Aspect, path string) {

       if family != "" && !strings.HasPrefix(family, ".") {
           if lc := strings.ToLower(family); families[lc] == "" {
               families[lc] = family // fold case → "Arial" and "arial" list once
           }
       }
       // ... existing styleSuffix / considerFontKey logic unchanged ...
   }

   // ListFontFamilies returns the display-case values, sorted case-
   // insensitively. Excludes leading-"." names and generic aliases by
   // construction. Not safe for concurrent use — call from the main/UI thread.
   // TextSystem embeds *Context, so this delegates to ctx.families.
   func (ts *TextSystem) ListFontFamilies() []string
   ```

   Correct by construction: no suffix-stripping heuristics, aliases and style
   variants never enter the set, case is folded (matching shirei), and
   `RegisterAppFont` / `AddFontFile` families appear because they flow through
   the same `registerFontPath`.

2. **go-gui** — optional capability interface + type assertion. Do **not**
   widen the mandatory `TextMeasurer` interface (that would force a
   simultaneous implementation across metal/gl/web/android/ios):

   ```go
   // FontLister is an optional capability. Backends that wrap a
   // *glyph.TextSystem opt in; others need no change.
   type FontLister interface{ ListFontFamilies() []string }

   // ListSystemFonts returns family names from the active text system's
   // catalog, sorted case-insensitively. Returns nil before backend init,
   // on WASM stub backends with no fonts, or on backends that do not
   // implement FontLister. Includes RegisterAppFont families once those
   // paths have been added to the text system.
   func ListSystemFonts(w *Window) []string {
       if fl, ok := w.textMeasurer.(FontLister); ok {
           return fl.ListFontFamilies()
       }
       return nil
   }
   ```

   Incremental rollout (one backend at a time), nil-safe in tests, zero
   forced churn.

3. **Filtering is a pure function** — factored out so it is testable without a
   backend:

   ```go
   // filterFontFamilies returns the entries of all whose family name
   // contains filter (case-insensitive), preserving all's sort order.
   // Runs every frame, so an empty filter returns all as-is (no clone);
   // nil-safe (nil in → nil out).
   func filterFontFamilies(all []string, filter string) []string
   ```

**Rejected:** Android `java.awt.GraphicsEnvironment` (desktop AWT, not
Android). go-glyph already walks `/system/fonts`, `/product/fonts`, etc.

**Rejected:** Linux fontconfig in go-gui — conflicts with go-glyph's
no-fontconfig directory walk and invites dual catalogs.

**Rejected:** a parallel platform enumerator in `gui/backend/` — names from
CTFontManager / fontconfig / DirectWrite can diverge from go-glyph's family
strings, so cards would show fonts that silently fall back to Helvetica /
Roboto / etc.

### Rendering — virtualize the grid

**The constraint:** go-gui is immediate-mode. `layoutWrapTextWalk`
(`layout_pipeline.go`) recurses **every** child unconditionally — there is no
scroll-visibility gate — and scroll offset is applied later, in the
positioning phase (`layout_position.go`). So emitting all N cards shapes text
for all N on every frame they are in the tree. Windowing removes them from the
tree, so they are never measured until visible.

**The mechanism — reuse `listCoreVisibleRange`.** go-gui already virtualizes
`ListBox`, `Tree`, and `Table` through one pure, tested helper:

```go
// list_core.go — pure arithmetic, plus listCoreVirtualBufferRows overscan.
func listCoreVisibleRange(itemCount int, rowHeight, listHeight, scrollY float32) (first, last int)
```

The immediate-mode trick that makes this work: the view reads the **previous
frame's** scroll offset at generate time via `w.ScrollY().Get(id)` (exported
`BoundedMap`), so the range is one frame stale — the overscan buffer absorbs
the lag.

**Grid adaptation (2-D).** `ListBox`/`Table` are one item per row;
the font grid is a wrapped grid. Fixed card size closes the gap. Each frame:

1. `cols = max(1, floor((contentW + gap) / (cardMaxW + gap)))`, recomputed from
   the current content width so columns still reflow responsively.
2. `rows = ceil(len(matches) / cols)`, `rowH = cardH + gap`.
3. `first, last := ListVisibleRange(rows, rowH, listH, scrollY, overscanRows)` —
   the exported wrapper applies the overscan internally (no separate widening
   step, no stacked buffers).
4. Emit `Column{Scrollable: true, ID: gridID, Sizing: FixedFixed, Width: outerW, Height: listH}` containing:
   - a top spacer rect of height `first * rowH`,
   - one `Row` per visible row in `[first, last]`, each holding the `cols`
     cards from `matches[r*cols : min((r+1)*cols, len(matches))]`,
   - a bottom spacer rect of height `(rows-1-last) * rowH`.

The spacers keep the total scroll range equal to the full catalog, so the
scrollbar stays correct.

**Overscan buffer.** The internal `listCoreVirtualBufferRows = 2` is tuned for
~20–32 px list rows; at `rowH ≈ 150–200 px` that is only 300–400 px, which a
fast flick outruns, flashing blank rows. The exported `ListVisibleRange` takes
the buffer as an argument, so the grid passes one `overscanRows` (start ~4,
tune against a fast scroll on a 500+ font machine) rather than stacking its own
widening on top of a hidden 2. Overscan cards are O(1) per row.

**Viewport dimensions (`outerW`, `listH`).** `ListBox`/`Table` virtualize only
with an explicit `Height`/`MaxHeight` (`virtualize := cfg.Scrollable &&
listHeight > 0`) — so the grid does **not** try to be `FillFill`/`FillFixed`
and estimate its own size. It is **`Sizing: FixedFixed` with explicit `Width:
outerW` and `Height: listH`**, and the *same* numbers feed `cols`/spacer math
and `ListVisibleRange`: on **both** axes one number drives arrange and math, so
they cannot disagree. (Height alone was insufficient — a `Fill` width arranged
differently from the `WindowSize`-derived `cols` estimate is the identical
X-axis footgun: wrong `cols` → overflow or dead space. There is no
previous-frame width-readback API, so explicit width is the fix.) Source from
`w.WindowSize()` (callable at view time; its values seed the `FillFill` root,
so they are already in layout units — see open Q1):

- `listH = max(rowH, winH - headerH - toolbarH)`. The **clamp is mandatory**:
  `listCoreVisibleRange` returns no rows for `listHeight <= 0`, so a short
  window or overshooting chrome constants would otherwise blank the grid.
- `outerW = winW` (the grid box fills the window; root chrome is zeroed).
  `contentW = outerW - 2*sidePad - scrollbarW` after the grid's real horizontal
  `Padding`, where the right inset is widened by `scrollbarW` — an **overlay
  inset**, since the vertical scrollbar is `OverDraw: true` (reserves no layout
  space), a deliberate choice to keep cards clear of the thumb, not an
  engine-reserved gutter (`ListBox` does not do it). `cols`/`cardW` derive from
  `contentW`.

Chrome constants (`headerH`, `toolbarH`) must equal the real rendered heights,
which requires the root/grid columns to zero their default `PaddingMedium`,
`SpacingMedium`, **and `SizeBorderDef` (1.5, counted twice per box via
`paddingHeight`)**; otherwise `listH`/`outerW` are wrong on frame one.
Recomputing per frame keeps resize correct.

**Scroll offset must be reset on content-shape changes.** The scroll store
holds a raw pixel offset and is re-clamped only during user-scroll events —
never when content geometry changes. So:

- **FontSize change** alters `rowH`; the old pixel offset then points at the
  wrong row (offset −1600 at `rowH` 160 shows row 10; at `rowH` 200 it shows
  row 8). The size slider's `OnChange` must call
  `w.ScrollVerticalTo(gridID, 0)` after updating `state.FontSize`.
- **Filter change** alters the match set (and can empty it); the filter
  input's `OnTextChanged` must likewise `w.ScrollVerticalTo(gridID, 0)`.
  `listCoreVisibleRange(0, …)` already returns `(0, -1)` (no rows), so an empty
  result is safe; the reset just keeps the stored offset honest.
- **Window resize** changes `cols`/`rows` but **not** `rowH`, so vertical
  depth stays valid; no reset. A now-out-of-range offset self-corrects on the
  next scroll event. (A future refinement could anchor by first-visible family
  index instead of resetting, if resetting-to-top on size change feels abrupt.)

**Cost.** Arrange and shaping are both O(visible), independent of catalog
size. No skeleton, batch reveal, prewarm API, background thread, or
`TextMeasurer` access is required for scale.

### Skeleton on cold scroll-in (optional, P2)

Windowing means a newly-scrolled-in row shapes its ~`cols` cards (3–5) that
frame — trivial, and normally invisible. If fast scrolling ever janks on
cold-cache families, a card may render fixed-size skeleton rects for the single
frame before its font is in the glyph cache. This is cosmetic polish, not a
scaling mechanism, and carries no persistent state.

### Clipboard Copy

`w.SetClipboard(text string)` already exists on `Window`, wired to all
backends (NSPasteboard on macOS, X11 CLIPBOARD on Linux, Win32 clipboard on
Windows, Clipboard API on web). No new API needed.

### "Copied" transient — driven by a real animation

`SetClipboard` and `QueueCommand` do not self-schedule a wake at T+1.2 s, so
the confirmation needs a frame source. Two API facts drive the shape: the
`NewTweenAnimation(id, from, to, onValue)` constructor takes **no** `Duration`
or `OnDone` (its default duration is 300 ms), and `OnValue` runs outside the
view — so the animated value must be written into a field the view reads.
Construct with a struct literal and store the value in `state.CopyOpacity`:

```go
w.SetClipboard(fam)
s.CopiedFam = fam    // reset explicitly each click — do not rely on the old
s.CopyOpacity = 1    // tween's OnDone (AnimationAdd drops the prior same-ID
                     // anim without firing OnDone)
w.AnimationAdd(&TweenAnimation{
    AnimID:   "copied-fade", // stable ID: a fresh click restarts the same fade
    Duration: 1200 * time.Millisecond, // default is 300ms — must override
    Easing:   EaseOutCubic,
    From:     1,
    To:       0,
    OnValue:  func(v float32, _ *Window) { s.CopyOpacity = v }, // state channel
    OnDone:   func(_ *Window) { s.CopiedFam = "" },
})
```

The card view renders the "Copied" badge at `s.CopyOpacity` when
`s.CopiedFam == name`. Resetting `CopiedFam`/`CopyOpacity` on every click (the
toast pattern) means a rapid click on another card is correct even though
`AnimationAdd` replaces the same-ID tween and skips the previous `OnDone`. The
animation loop guarantees the redraws; no manual timer.

**Refresh cost:** `TweenAnimation.RefreshKind()` is `AnimationRefreshLayout`, so
each of the ~1.2 s of ticks rebuilds and re-shapes the layout. Virtualization
bounds that to the *visible* window (not N), so it is acceptable — but opacity
does not change geometry, so if it janks while scrolling, replace the tween with
a small custom `Animation` whose `RefreshKind()` returns
`AnimationRefreshRenderOnly` (the pattern `BlinkCursorAnimation` uses).

## UI Layout Details

### Header

Dark accent bar with title "go-gui font viewer" and subtitle. Fixed height
(feeds the `listH` derivation).

### Toolbar — Row 1

- "Sample Text" label → `Input` with `ID`, `Text: state.Sample`,
  `OnTextChanged` writing the new string back to state (wide, ~440–640 px)
- "Shuffle" button → `OnClick` sets `state.Sample` to a random entry from a
  fixed pangram pool (package var), **excluding the current sample** so a
  re-pick never silently no-ops. E.g.:

  ```go
  var pangrams = []string{
      "The quick brown fox jumps over the lazy dog",
      "Sphinx of black quartz, judge my vow",
      "Pack my box with five dozen liquor jugs",
      "How vexingly quick daft zebras jump",
      "Waltz, bad nymph, for quick jigs vex",
      "Jackdaws love my big sphinx of quartz",
  }
  ```

### Toolbar — Row 2

- "Filter Fonts" label → `Input` bound to `state.Filter`. `OnTextChanged`
  updates `state.Filter` **and** resets scroll: `w.ScrollVerticalTo(gridID, 0)`
  (the match set changed — see Rendering).
- Clear control when filter is non-empty (text "×" button is fine; also resets
  scroll).
- Spacer
- "Size" label → `Slider` `ID` + `Value: state.FontSize`, Min 12, Max 72,
  Step 1. `OnChange` updates `state.FontSize` **and** resets scroll:
  `w.ScrollVerticalTo(gridID, 0)` (`rowH` changed). Width ~170 px → "NN px"
  readout.
- Spacer
- "N / M fonts" match count label

The toolbar's height is fixed (two rows) so it can be subtracted from
`w.WindowSize()` to yield the grid viewport height.

### FontCard

**Fixed width AND uniform height** — the precondition for virtualization.
`cardH = cardHeight(FontSize)` = `nameRowH + previewPad + previewLines *
FontSize * 1.4`. This is a **uniform clip box**, deliberately *not* a per-face
fit: `1.4` approximates go-gui's list line-height (`listCoreRowHeightEstimate`),
but go-glyph's real per-face `LineHeight` varies, so a tall face clips at the
box and a short face leaves a small band. That is the accepted trade — uniform
declared height is what keeps `rowH` constant and the spacer math honest; the
Latin-only note does not cover this metric variance. If the clip/band looks bad,
measure the box **once per `FontSize`** with a reference face (never per family
— that reintroduces N measurements), not a flat multiplier.

- **Name row**: family name in UI font (13 px, medium weight). Truncate via
  single-line `Text` + parent `Clip` / `MaxWidth` (no `MaxLines`/ellipsis API
  today — clip is enough for v1). On hover: "Copy" badge at right. When
  `state.CopiedFam == name`: "Copied" badge at alpha `state.CopyOpacity`.
- **Preview box**: white rounded rect with light border. Sample text in the
  family's own face, `TextModeWrap` at width `cardW - 2*previewPad`, clipped
  with parent `MaxHeight` / `Clip` to `previewLines` lines of the current
  `FontSize`. **Latin-only contract:** sample text is Latin pangrams; non-Latin
  runes fall back per-rune to a covering face and do **not** exercise the
  card's family — do not "fix" this with Unicode samples that silently lie.

Card states:

- **Default**: subtle background, name visible
- **Hovered**: slightly brighter background, "Copy" affordance appears. Each
  card has a **stable ID** `"card:"+family`, so `OnHover` sets `hoveredFam` and
  `OnMouseLeave` clears it while the card is on-screen. Because a card evicted by
  windowing is never visited by `layoutMouseLeave`, the grid **also** clears
  `hoveredFam` each frame when it falls outside `[first,last]` (see hover
  decision) — otherwise the affordance sticks when the family scrolls back
- **Copied**: green "Copied" badge at `state.CopyOpacity` (fades 1→0 over
  ~1.2 s; the tween's `OnDone` clears `state.CopiedFam`)

Changing `FontSize` changes `cardH` (preview grows) → recompute `rowH`; the
virtualization math handles it, but every card must use the same height for a
given size.

### FontGrid

Virtualized `Column` — **not** `gui.Wrap` (Wrap lays out all children and
can't window):

```go
func fontGrid(w *Window, s *FontViewerState, matches []string) View {
    // len(Families)==0 covers both nil (backend not ready) and a non-nil empty
    // enumerate; emptyState distinguishes copy via the nil check itself.
    if len(matches) == 0 {
        return emptyState(len(s.Families) == 0)
    }
    winW, winH := w.WindowSize() // view-time; values seed FillFill root (open Q1)

    cardH := cardHeight(s.FontSize)                    // clip-height, uniform per size
    rowH := cardH + gap                                // gap folded into the row
    outerW := float32(winW)                            // grid box == window (root chrome zeroed)
    listH := max(rowH, float32(winH)-headerH-toolbarH) // clamp: never <= 0
    // contentW after the grid's own horizontal padding; right side widened to
    // clear the overlay scrollbar. Same numbers arrange the box (FixedFixed).
    contentW := outerW - 2*sidePad - scrollbarW

    cols := max(1, int((contentW+gap)/(cardMaxW+gap)))
    cardW := min(cardMaxW, (contentW-float32(cols-1)*gap)/float32(cols))
    rows := (len(matches) + cols - 1) / cols

    scrollY, _ := w.ScrollY().Get(gridID)
    // Buffer passed explicitly (overscanRows) — the exported wrapper does NOT
    // add its own, so overscan is one number, not stacked magic (2+4).
    first, last := ListVisibleRange(rows, rowH, listH, scrollY, overscanRows)

    // Clear hover if its card was evicted (layoutMouseLeave never visits
    // off-window cards → OnMouseLeave won't fire → sticky "Copy" otherwise).
    if s.HoveredFam != "" && !inWindow(matches, s.HoveredFam, first, last, cols) {
        s.HoveredFam = ""
    }

    children := []View{spacerV(float32(first) * rowH)}
    for r := first; r <= last; r++ {
        children = append(children, gridRow(w, s, matches, r, cols, cardW, cardH, rowH))
    }
    children = append(children, spacerV(float32(rows-1-last)*rowH))

    // FixedFixed with explicit Width AND Height — the SAME numbers as the
    // cols/range math, so neither axis can disagree with arrange. Padding is
    // real (sidePad + scrollbar clearance); Spacing/SizeBorder zeroed.
    return Column(ContainerCfg{
        ID: gridID, Scrollable: true,
        Sizing:     FixedFixed,
        Width:      outerW,
        Height:     listH,
        Padding:    Some(Padding{Left: sidePad, Right: sidePad + scrollbarW}),
        Spacing:    SomeF(0),
        SizeBorder: Some(float32(0)),
        Content:    children,
    })
}
```

- `gridRow` builds one `Row{Height: rowH, Spacing: SomeF(gap)}` (horizontal
  card gutter — vertical gap lives in `rowH`, not row spacing) holding up to
  `cols` cards of `cardW × cardH`, top-aligned (the `gap` slack sits below the
  card so the row measures exactly `rowH`). Each card has a **stable ID**
  `"card:"+family` so click, hover, and `OnMouseLeave` identity survive.
- `inWindow(matches, fam, first, last, cols)` reports whether `fam`'s index in
  `matches` lies in `[first*cols, (last+1)*cols)` — the emitted card range —
  used to clear `HoveredFam` on eviction.
- `RequireScrollID` enforces the non-empty `ID` at construction.
- **Narrow viewport:** when `contentW < cardMaxW`, `cols` stays 1 and `cardW`
  shrinks to fit — cards never overflow horizontally, no horizontal scrollbar.
  The preview `TextModeWrap` width is `cardW - 2*previewPad` (recomputed with
  `cardW`, so wrapping tracks the shrunk card).
- **Empty states:** `emptyState(len(s.Families) == 0)` renders "no system fonts
  found" (nil *or* empty enumerate) vs "no fonts match the filter" (families
  present, filter excludes all) — no grid, no virtualization.

## Implementation Plan

### Phase 1: Font Enumeration API (go-glyph + go-gui)

- go-glyph: case-folded `Context.families` set threaded through
  `registerFontPath` — **a signature change to a free function with direct test
  call sites**; Phase 1 owns updating both call sites (`AddFontFile` path and
  `fontScan.consider`) and their tests. `.LastResort` still enters `fontPaths`
  as before — the new set only *excludes* it, no discovery behavior changes.
  Then `(*TextSystem).ListFontFamilies() []string` returns display-case names
  sorted case-insensitively
- go-glyph test — **bidirectional** (`ResolveFontName` is the wrong API: it
  returns `resolveFontFamily(...)`, always succeeds, never consults
  `fontPaths`). White-box in `package glyph`:
  - *forward* — every `ListFontFamilies()` name has a real `ctx.fontPaths` key
    (bare family **or** a `-Regular/-Bold/…` style key) mapping to a non-empty
    path that is **not** only the generic-fallback path (the map `newFTFont` /
    `fontPaths[family]` actually resolve against, `freetype_types_puregoft.go`)
  - *reverse* — a fixture directory with a known family enumerates it **exactly
    once** (catches a scan path that never hits `registerFontPath`, and the
    case-fold dedup)
- go-gui: **export `ListVisibleRange(itemCount, rowHeight, listHeight, scrollY,
  overscan)`** so the `package main` example can call it (buffer as an arg — no
  stacked `listCoreVirtualBufferRows`)
- go-gui: `FontLister` interface + `ListSystemFonts(w *Window) []string` type
  assertion; opt in the native text backend
- No CTFontManager / fontconfig / DirectWrite / AWT enumerator

`filterFontFamilies` is **not** a `gui` export — it is example-local
(`examples/fontviewer`, tested there); it has no shared caller and does not
belong in the widget API.

**Release coupling:** go-glyph is upstream. Landing `ListFontFamilies`
requires a go-glyph tag + a go-gui `go.mod` bump before Phase 2 can build
against it.

### Phase 2: Core Font Viewer

- `examples/fontviewer/main.go` with full state (init `FontSize: 28`, random
  pangram), header, toolbar, grid
- Sample text, shuffle, filter, size slider; filter/size handlers reset scroll
  via `w.ScrollVerticalTo(gridID, 0)`
- **Virtualized grid**: `FixedFixed` grid with explicit `Width: outerW` +
  `Height: listH` (both from `w.WindowSize()`, both feeding the math);
  `Padding`/`Spacing`/`SizeBorder` zeroed on root+grid; uniform-height cards
  (`cardHeight`), `ListVisibleRange(..., overscanRows)` over rows, `rowH`-tall
  rows, top/bottom spacers, `listH` clamped `>= rowH`
- Click-to-copy + tween-driven "Copied" transient (`CopyOpacity`)
- Empty states keyed on `len(s.Families) == 0` (not `== nil`)
- Layout/state tests: empty, filtered, copied, sizing, **and the visible-range
  math** — assert emitted card count and that `topSpacer + rows*rowH +
  bottomSpacer == totalRows*rowH` for a given `(N, cols, rowH, listH, scrollY)`
  (pure, headless; mirrors `TestListCoreVisibleRange` / `TestTableVirtualization`)

### Phase 3: Polish (optional)

- `--shape-all` debug mode: drop virtualization (emit all N cards) to stress
  **all-N main-thread shaping** in one frame (not "concurrent" — go-glyph is
  single-threaded) — the honest "stress test at scale" path
- Skeleton on cold scroll-in (P2) if profiling shows fast-scroll jank
- Late-registration refresh: re-enumerate when `RegisterAppFont` fires after
  the first successful list (set `s.Loaded = false`)
- P2 PNG export only if an offscreen/render + encode path is defined (none
  exists today — skip rather than invent ad hoc)

## Remaining Open Questions

1. **Viewport-size units / DPI — Phase 2 entry criterion (a geometry spike,
   NOT a Phase 1 gate).** Phase 1 is the enumeration API (`ListFontFamilies` /
   `FontLister` / `ListVisibleRange`) and does not touch window geometry, so
   this must not block or rubber-stamp it. `w.WindowSize()` returns cached
   `windowWidth/windowHeight` that seed the `FillFill` root, so `listH`/`outerW`
   are consistent *with each other* in layout units. The landmine is
   `w.BackingScale` (real field, default 1, 2 on Retina): if `TextStyle.Size`
   (hence `cardHeight`) and window dims ever live in different scales, `rowH`
   and the arranged rows diverge and the overscan cannot hide it. **Spike this
   on a Retina display with a 500+ font catalog as the first task of Phase 2**,
   before the grid geometry is built on the assumption.

2. **Tuning constants** (empirical on a 500+ font machine, not fundamental):
   `overscanRows` (start 4 — enough to hide a fast flick at grid `rowH`);
   `nameRowH` / `previewPad` / `lineFactor` so `cardHeight(FontSize)` matches
   the actually-rendered card and rows stay uniform.

3. **Chrome geometry, not just drift**: `listH`/`contentW` subtract `headerH`,
   `toolbarH`, `sidePad`, `scrollbarW`. These hold only if the root/grid columns
   zero `Padding` **and** `SizeBorder` (default `SizeBorderDef` 1.5, counted
   twice per box) — otherwise `listH` is optimistic → blank bands the overscan
   can't cover. Bias the estimate large, and assert `headerH`/`toolbarH` against
   the rendered heights of `header()`/`toolbar()` in a layout test so they can't
   silently diverge.

4. **Headless PNG export**: no `RenderToPNG` equivalent exists. Keep P2
   speculative until an offscreen buffer + encode path lands; otherwise cut
   from v1.

5. **List API surface**: `ListSystemFonts(w *Window)` matches how other
   window-scoped text helpers are exposed. A window-free variant taking a
   `TextMeasurer` / backend handle is possible later if a caller needs it —
   same rule: list only what resolution can see.
