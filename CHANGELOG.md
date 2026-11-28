# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- **Scrolling feels smoother and more responsive across desktop and web.**
  Smooth-scroll interpolation now advances synchronously inside the render frame
  instead of via a fixed 16 ms background timer, eliminating frame-pacing jitter
  on 60/120 Hz displays. Wrapped text blocks without explicit IDs are now cached
  across frames, offscreen children inside clipped scroll viewports are culled
  before rendering, the Metal backend caches cursor state and coalesces `wakeUp`
  dispatches, and the web backend detects precise trackpad wheel events so
  trackpads scroll 1:1 without artificial smoothing lag.
- **`WithTooltip` no longer moves or grows the view it wraps (#1381).** Its
  wrapper set no padding or spacing, so the theme's default container padding
  applied: 14 px on each side in the default theme. A view that got a tooltip
  moved and grew, and in a list with a fixed row height it was pushed down and
  cut off. The wrapper now adds no room. A layout that relied on those 14 px
  shrinks. To keep them, put the wrapped view in a container with that padding
  inside `Content`. Padding on the wrapped view itself does not work for a
  fixed-size view: it stays the same size, and its content gets less room.

## [v0.100.1] - 2026-11-27

### Fixed

- **A DataGrid with a pager or the CRUD toolbar fits a narrow parent (#1368).**
  The pager and the toolbar were plain rows of single-line texts and fixed-width
  buttons, so the grid could not be narrower than the row. In a narrow parent,
  such as a phone screen, the grid stuck out past the parent's end, and the
  vertical scrollbar and the end of the bar could not be reached. The width also
  changed with the text: one added row changed the status to "Draft 1 Dirty 1
  Delete 0" and made the grid wider. Now the grid takes its parent's width, and
  each bar scrolls sideways when its content does not fit. Scroll with the wheel
  or the trackpad. The bar's scrollbar shows under the pointer. Keyboard focus
  does not yet scroll a hidden button of the bar into view.
- **Tabs reorder and take arrow keys correctly under right-to-left (#1359).** In
  a `TabControl` with `Reorderable: true` under an RTL locale, a tab dragged
  past its neighbours did not move. The drop slot was found as if the tabs ran
  left to right. Now the tab lands where the pointer is, as under LTR. During a
  drag the floating copy of the tab was drawn on the opposite side of the strip,
  because its position was mirrored. It now stays under the pointer. This also
  applies to the floating row in a reorderable `ListBox` and `Tree`. The arrow
  keys now go the way they point: under RTL, Left selects the tab to the left,
  which is the next one, and Alt+Left moves the tab to the left. This also
  applies to `SegmentedControl`. Up, Down, Home and End keep reading order. A
  window resize during a drag now cancels the drag, in either direction. A drag
  records the tab positions when it starts, and a resize moves them, so the drop
  slot and the floating tab would no longer follow the pointer.
- **The End caret on a soft-wrapped right-to-left line is on the correct edge
  (#1371).** Press End on a wrapped Hebrew line and the caret now goes after the
  last letter, at the left edge of the line. Before, it went to the right edge,
  where the line starts. The caret comes from go-glyph
  `Layout.LineEndCursorPos`.
- **A Canvas child sits at its own X under every alignment and text direction
  (#1358).** A `Canvas` added its horizontal alignment to each child's X, so
  each child moved right by its own share of the free space. Children of
  different widths moved by different amounts. Under a right-to-left locale the
  default alignment resolves to the right, so this happened with no `HAlign`
  set: in `examples/2048` most tiles were drawn outside the board. The scroll
  range and the `Fit` size did not count the shift. Now `HAlign` has no effect
  on a Canvas, as `VAlign` already had none. Under RTL a Canvas also took its
  left inset from `Padding.Right`, as a column does. It now uses `Padding.Left`,
  so its children use left-to-right coordinates, as its scroll offset already
  did. An app that set `HAlign: gui.HAlignLeft` on a Canvas to work around this
  can remove it.

### Changed

- go-glyph v1.30.0.
- **iOS draws with the macOS Metal renderer (#1342).** The iOS backend's own cgo
  renderer (`metal_darwin.m`) and its copy of the draw code are gone. iOS now
  uses the purego renderer that macOS uses, so a renderer fix lands on both at
  once. On the simulator a frame takes about a third of the main-thread time it
  did, and the draw path makes no allocations (121 a frame before), because the
  frame no longer waits for the GPU to schedule it. Keychain and locale on iOS
  use the macOS purego code too. The iOS package API does not change, and an iOS
  build still needs `CGO_ENABLED=1`: purego needs cgo on iOS, and the UIKit
  shell stays cgo.
- **Bump indirect Go dependencies (regexp2, mewkiz/pkg).**
  `github.com/dlclark/regexp2/v2` goes from v2.13.2 to v2.13.3 (pulled in by
  chroma for code highlighting) and `github.com/mewkiz/pkg` to its latest commit
  (pulled in by beep's FLAC decoder). The public API does not change.
- **CI uses `actions/checkout@v7` and `actions/setup-go@v7`.** The workflows and
  the shared setup action track the new major versions; no workflow input
  changes.

## Older releases

All published releases are in [CHANGELOG-archive.md](CHANGELOG-archive.md).
