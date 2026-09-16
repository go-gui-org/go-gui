# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.77.0] - 2026-09-16

### Added

- **`gui.Interactive` gives a view builder its hover, press and focus state
  (#650)** — a custom look that read `IsHovered` / `IsPressed` had to be a named
  view type with its own `GenerateLayout`, call `w.EffID` itself, and spell
  "pressed and still hovered" again in every look.
  `gui.Interactive(id, func(gui.InteractionState) gui.View)` does the deferral
  and the ID resolution, and passes `Hovered`, `Pressed`, `Armed` and `Focused`.
  The caller names only the leaf ID. When the built root does not carry that ID,
  `gui.Debug` reports it under `DebugMissingIDs` instead of the state staying
  false silently. `examples/custom_buttons` now uses it.

- **`DebugLayoutInvariants` reports a frame that breaks a sizing rule (#634)** —
  wrong geometry was silent: the frame rendered, nothing errored, and the defect
  surfaced much later as something looking a few pixels off. The new category
  walks the arranged tree and reports a child positioned outside a non-clipping
  parent's bounds, a minimum left above its maximum, and any non-finite or
  negative size. It is opt-in, outside `DebugAll` for the reason
  `DebugUnscopedIDs` is: escaping a parent is sometimes the design, as a slider
  thumb overhangs its track. Ask for it by name through
  `(*Window).TestFindings(gui.DebugAll | gui.DebugLayoutInvariants)` or
  `gui.DebugCategories`. The rules it checks are written down for the first time
  in `docs/specs/layout-sizing-rules.md`, and the same checks back the layout
  fuzz targets so the document and the code cannot drift.
- **`MaskNone` names the no-mask preset** — `InputMaskPreset`'s zero value meant
  "no mask" but had no exported name, unlike every other preset. `MaskNone`
  spells it explicitly; existing code is unaffected.
- **`Checkbox` alias is back** — `Checkbox` calls `Toggle` with the same config,
  so code that uses the checkbox name builds again. The alias stays; later
  export cuts must keep it.
- **Custom input mask tokens and the remaining mask presets are public** —
  `InputCfg.MaskTokens` carries caller-defined `MaskTokenDef` tables into the
  compiled mask the keystroke path reads, `MaskTokenDef.Matcher` is exported so
  a table can actually be built outside the package, and `MaskCreditCard16`,
  `MaskCreditCardAmex` and `MaskCVC` join the preset list. A custom token
  without a `Matcher` fails `compileInputMask` instead of installing a slot no
  keystroke can ever fill.
- **`DebugLayoutInvariants` checks Fill distribution (#638)** — a Fill row that
  could not fit its minimums, or that left space undistributed, gave up
  silently: the frame rendered and nothing said so. The category now reports a
  main-axis container whose in-flow children plus spacing do not sum to its
  content box while a Fill child is there to take the slack. A row with no Fill,
  a clipping or scrolling parent, a Wrap or Overflow row, and a gap left while
  every Fill child sits at its maximum stay quiet: each is alignment slack or a
  supported outcome, not undistributed space. The rule is written down as
  invariant 4 in `docs/specs/layout-sizing-rules.md`.
- **`DebugSizing` warns when Fixed sizing discards Min/Max (#635)** — a Fixed
  axis pins `Min = Max = size`, so a stated `MinWidth`/`MaxWidth` (or height
  equivalent) never took effect and nothing said so. The new category, on by
  default in `DebugAll`, reports a conflicting stated bound at generation time;
  a bound equal to the size and a Fixed axis with no positive size stay quiet.
  The `Shape` and `ContainerCfg` bound fields now document the rule.

### Changed

- **go-glyph bumped to v1.25.2.** Brings typesetting v0.3.5 and x/text v0.42
  with no replace directive, so go-gui and glyph resolve the same text stack.
- **BREAKING: `OverflowPanel` and Overflow containers require an `ID`** — the
  overflow pass stores the visible-item count keyed by the container's ID. Two
  panels without one shared a single slot, overwrote each other's count every
  frame and forced a relayout on every frame, and each menu listed items with
  the other panel's count. `OverflowPanelCfg.ID` is now `gui:"required"`, and
  `OverflowPanel` and a `ContainerCfg` with `Overflow: true` panic on an empty
  ID. Migration: give each overflow panel or container a unique `ID`.
- **BREAKING: `SidebarCfg.Clip` is gone; sidebars always clip (#641)** — the
  sidebar animates its width from 0, so unclipped content stuck out for the
  whole slide, and the showcase sidebar drew 27.6px past its own edge at rest.
  The inner container now clips unconditionally. Migration: delete the `Clip`
  line from any `SidebarCfg`; content that must escape the panel belongs in a
  float or overlay, not in the sidebar tree.

- **`DrawRecorder` now documents that a points slice is valid for the duration
  of the call only** — under an active canvas transform the `[]float32` a
  recorder receives is a mapped copy in one shared scratch buffer, so two
  retained polylines both ended up holding the second one's coordinates. With no
  transform in force the caller's own slice is passed straight through and
  retention appeared to work, which is how an exporter ends up correct until its
  first `Translate`. No signature changed: an implementation that queues
  commands to serialize after the redraw must copy the points it is handed.

- **BREAKING: NumericInput with Min > Max now panics** — the bounds previously
  swapped silently and the field clamped the wrong way. Pass them in order; a
  NaN bound still counts as unset on its side.

### Fixed

- **A `Select` label keeps its descenders** — the closed-field label sits in a
  clipping wrapper (so a long value cannot push the disclosure arrow out) and
  takes the cap-band optical correction, which moves the text box down past the
  wrapper's bottom. The wrapper's clip then cut the tail of labels like "Cogs".
  The wrapper now grows by the applied shift, so the label stays inside the clip
  while the field's own height is unchanged.
- **A list marker no longer spills over the text beside it (#634)** — a bullet
  or number column was `Fixed` at a width computed from `prefixCharWidth`, a
  nominal per-character guess, while the marker text inside it was sized by the
  real text measurer. Any font whose glyphs were wider than the guess left the
  marker hanging outside its own column and over the item's text. The column is
  now `Fit` with that width as a `MinWidth`, so it grows to hold the marker and
  still keeps markers aligned down the list. The width is also counted in runes
  rather than bytes, which drops a hand-tuned correction that happened to be
  right for `"• "` and wrong for every other multi-byte marker.

- **`OnMouseLeave` no longer fires for a hover that ended frames ago** — the
  per-shape hover record was a flag, and a shape that stopped being walked
  (disabled, or not generated that frame) left its flag set. When the shape came
  back with the pointer elsewhere, it fired a leave for a hover that was long
  over. The record is now the frame the pointer was last inside, and only the
  current frame or the one before counts as still hovered.
- **An Overflow row counts the gap after a zero-width item** — the decision to
  reserve spacing read the width accumulated so far, so an in-flow child of zero
  width dropped one gap that positioning still applied, and the row kept a
  trailing item that did not fit.
- **A cached rich-text layout is shared, not copied** — every hit on the
  cross-frame RTF layout cache moved a copy of the shaped layout to the heap,
  once per rich-text shape per frame, which a Markdown page paid for every block
  it drew. The cache now hands out the layout it already holds.
- **An RTL scrollable row can reach the content it hides** — an RTL row runs
  leftward from its right edge, so what does not fit sits off the left, but the
  scroll offset moved the children further left still and no input could bring
  it back. The offset keeps its meaning and its range — a distance from the
  start edge, in `[maxOffset, 0]` — while the wheel, trackpad, keyboard, pan,
  thumb drag and gutter click now read mirrored for such a row, and the
  scrollbar thumb rests at the right end when unscrolled. RTL columns are
  unchanged.
- **A centered or end-aligned Wrap container places its rows correctly** — the
  rows a wrap builds were never given a cached content width, so alignment read
  0 and moved each row by its whole slack: a centered row started halfway across
  and its last item ran past the edge. Rows now align against the width of the
  items they hold.
- **A rotated widget no longer collapses an empty Canvas** — when a rotation
  changed a child's size, the re-fit of its Fit ancestors set an axis-less
  container (Canvas) with no in-flow child to size 0. The sizing pass keeps the
  current size in that case, and the re-fit now does the same.
- **MaxWidth and MaxHeight win over a larger minimum at every sizing step** —
  when a minimum was larger than the maximum, most sizing steps capped the size
  at the maximum, but the fill pass that shrinks a Fill child raised it to the
  minimum. The clamp used for Fill children could do either, depending on the
  starting size. Every step now caps at the maximum, and a container's computed
  minimum can no longer exceed its maximum on the cross axis.
- **A Fit scroll container no longer takes its children's minimum size** —
  sizing read `Clip` before the layout pass set it on scroll containers, so a
  Fit `Scrollable` container took its content's minimum and could not shrink and
  scroll. Sizing now treats a scroll container as clipped on each axis it
  scrolls. An axis that `ScrollMode` excludes keeps the content minimum, as
  before.
- **Fill children shrink beside a wider fixed sibling** — when a row was too
  narrow, the shrink pass searched for its largest child among Fixed and Fit
  siblings too. If that child was not Fill, nothing shrank and the row
  overflowed its parent: a wide label next to a Fill input in a narrow window.
  Only Fill children are now searched and share the shortfall.
- **Wrap and overflow rows align and scroll against their real content width** —
  a Fixed or Fill wrap that broke into rows kept its one-row content width, so a
  scrollable wrap scrolled sideways into empty space with a wrong-size thumb. An
  overflow row kept the width from before its hidden children were removed, so a
  centered row started left of its edge. Both now re-measure after the change.
- **An overflow row shows its last item when it fits without the trigger** —
  room for the trigger was reserved even for the last item, so a row where every
  item fit exactly hid the last one and showed an overflow menu.
- **Rotated RTL rows, and children of rotated containers, stay in place** — a
  quarter-turn RTL row moved its children in the wrong direction and drew them
  outside the container. Children of a rotated container ignored ancestor clips,
  so content scrolled out of a viewport still took clicks and hover.
- **Floats inside a disabled container are disabled** — a popover or menu lifted
  out of a disabled container took clicks, hover and Tab focus. It now inherits
  the disabled state of its host.
- **Float layout edge cases** — an auto-flipped float now keeps its offset as a
  gap on the new side instead of overlapping the anchor. Extreme `FloatZIndex`
  values no longer overflow the sort. A nested float with a lower Z than its
  host now anchors to the host's final position, not to a stale one.
- **Clip containers and Canvas refits agree with the width rules** — a Clip
  container no longer takes its children's minimum height, matching how widths
  already worked. A Fit Canvas that holds a rotated child keeps enclosing
  children placed at an offset instead of shrinking and clipping them.
- **`FindLayout` stops at the depth cap** — like every other layout walk, it no
  longer recurses without limit on a very deep tree.

- **Windows resize cursors stay visible between frames (#631)** — the frame loop
  overwrote the system resize cursor with the application's cursor, making
  window edges difficult to find. Cursor updates now apply only over app content
  or during a captured widget drag, leaving Windows in control over the native
  window frame.
- **Multiline Enter goes through the text filter** — pressing Enter in a
  multiline `Input` inserted the newline directly, bypassing the field's `Mask`
  and `PreTextChange` validator, so a validator never saw that rune and a masked
  field accepted a character no slot matches. The newline now passes through the
  same choke point as every other insertion: a veto leaves the text unchanged
  and a mask refuses it.
- **An uncompilable input mask panics instead of running unmasked** — a `Mask`
  whose custom token lacks a `Matcher` logged and fell back to a nil mask, so
  the field silently edited plain text. It now panics at construction like the
  inverted-bounds and bad-date-format checks, failing the typo loudly instead of
  dropping validation.
- **`NumericInput` treats ±Inf values as unset** — a `Value` or `Min` of ±Inf
  seeded stepping and committed text that formatted as `+Inf` (`-+Inf` with a
  sign), which no locale parses back. Non-finite seeds now fall through to the
  typed text, `Min`, and zero the way NaN already did, and a commit against one
  yields an empty value.
- **Gradient fills split at every stop when stops are out of order** — the
  gradient tessellator searched its stop breakpoints as a sorted list, but only
  sorted it for `SpreadReflect`. With pad or repeat spread and stops not in
  ascending offset order (an SVG `<linearGradient>` in document order, or a
  `CanvasGradient` built that way), triangles that crossed a stop were not
  split, and the fill smeared one color segment across them. The breakpoints are
  now sorted for every spread.
- **Overflow containers count placeholder children when hiding items** — an
  empty, floating or `OverDraw` child before the trigger made the overflow pass
  store too small an item index. When every item fit, the trigger stayed
  visible. When some did not, the `OverflowPanel` menu also listed items that
  were still in the row. The stored value is now the child index where hiding
  starts, which is the index `OverflowPanel` reads.
- **NumericInput re-parses its own output for multi-size `GroupSizes`** — with
  `GroupSizes: []int{3, 2}` the formatter repeats the last size for every
  further group and shows `1,23,45,67,890`, but the parser used size 3 past the
  end of the list and rejected that string. Commit, edit and arrow-key steps on
  a displayed value then failed with no error. Both paths now repeat the last
  configured size.
- **IME input is sanitized the same way on every backend** — the X11 preedit,
  the web composition/commit and the Android commit reached the widgets without
  the length cap and replacement-character strip the Win32 and X11 commit paths
  apply, so a hostile input source could push an unbounded string or visible
  decoding failures into the render path. All three now share the same
  strip-and-cap. The X11 preedit still emits when empty (that event ends the
  composition), and a missing web `data` value yields an empty string instead of
  the literal `"null"`. The Win32 caret pixel now rounds to nearest like
  `gui.imeCoord` and the X11 scaler instead of truncating, so one caret lands in
  one place on every backend. The X11 `IMEStart`/`IMEStop` calls are explicit
  no-ops with no input method present, matching `IMESetRect` and `drainIME`.
- **Rotated children no longer leave stale content sizes on their parents
  (#622)** — a child with `QuarterTurns` 1 or 3 swaps its width and height after
  the fill passes have cached each container's content width and height. Those
  caches were not updated, so a scrolling parent clamped its scroll range and
  sized its scrollbar thumb from the pre-swap extent, and a Fit container with
  centered or end alignment offset its children by the old difference. Every
  ancestor the swap re-fits, and the Fixed or Fill parent where the re-fit
  stops, now refreshes both caches.
- **Windows tray reports a failed window init on every call** — the tray's
  hidden message window is built once. If that build failed, only the first
  `Create` saw the error; every later `Create` returned success and registered
  an icon against no window, so its clicks and menu never arrived. The error is
  now kept on the tray, and every `Create` returns it.
- **Windows tray clicks arrive when the tray is made off the main thread
  (#616)** — Win32 gives a window's messages only to the thread that made the
  window. The tray made its window on the caller's thread but read messages on a
  different thread, so its own loop never got a message and held one OS thread
  for the life of the process. Clicks worked only because the gl backend's pump
  ran on the same thread as `OnInit`. If `SetSystemTray` was called from another
  goroutine, clicks and menu picks never arrived and no error was reported. The
  tray now makes its window and reads its messages on one thread that it owns,
  so it works from any goroutine. Each tray also registers its own window class,
  so a second tray no longer fails to register or sends its clicks to the first.
- **Windows tray reads its click events correctly and works from the keyboard
  (#617)** — the tray asked for `NOTIFYICON_VERSION_4` but decoded its callback
  in the older layout, where `wParam` is the icon ID. In version 4 `wParam` is a
  screen position and the icon ID sits in `lParam`, so clicks could miss their
  icon, a right click could fire the default action, and mouse moves could fire
  it too. The version request was also sent before the icon existed, and its
  result was not checked. The tray now sets the version after adding the icon,
  decodes the version 4 layout, fires the default action when the icon is
  selected by mouse or keyboard, and opens the menu, by mouse or keyboard, at
  the position the shell reports. If the shell refuses version 4,
  `SetSystemTray` returns an error and no icon is left behind.
- **Windows tray updates and removes the icon it added (#619)** — the tray added
  each icon by a new random GUID but changed and deleted it by its numeric ID.
  The shell matches an icon added with a GUID by that GUID, so
  `UpdateSystemTray` and `RemoveSystemTray` could miss the icon. Also, each
  launch could leave one more stale entry in the notification area settings. The
  tray now names the icon by window and numeric ID in every call.
  `UpdateSystemTray` frees the old icon handle only after the shell accepts the
  new one. An update with no icon no longer frees the icon still on screen,
  which `RemoveSystemTray` then freed a second time.
- **macOS frame pump survives a nested runloop inside a nested runloop** — the
  pump that repaints windows during a modal dialog, live resize or open menu
  shared one snapshot buffer across calls. When app code run by a pumped frame
  entered a deeper runloop, the timer called the pump again, which truncated and
  cleared that buffer while the outer call still walked it. The outer call then
  read a nil window and panicked, or pumped the wrong windows. Each call now
  holds the buffer for itself; only real re-entry allocates, so the 60 Hz tick
  stays allocation-free.

- **Linux screen readers get the correct widget states** — the AT-SPI2 bridge
  sent thirteen of its fourteen state flags at the wrong bit positions. Orca
  read every node as editable, multi-line and pressed, a focused widget as
  defunct, a busy one as checked, and a read-only field as a default button.
  Each position now matches `AtspiStateType` in at-spi2-core, and read-only uses
  `READ_ONLY` (43). A test checks each position against the numbers in the
  header.

- **`audio.Init` is safe to call from many goroutines at once** — `Init`, `quit`
  and `PlaySource` read and wrote the `initialized` flag with no lock. Two first
  calls to `Init` could both see it unset and open the output sink twice, and a
  shutdown racing `Init` could leave the flag out of step with the backend. A
  package mutex now covers the check, the backend call and the flag write, so
  the documented "call from any goroutine, idempotent" promise holds.
- **Volume and music calls no longer race the audio thread, and do nothing
  before `audio.Init`** — the audio thread read the master and music volumes and
  the music track state every buffer while the app wrote them with no
  synchronization. Volumes are now atomics, and the music track has a lock that
  both sides take. Before `Init`, `HaltMusic`, `FadeOutMusic` and the other
  music controls dereferenced a nil track and panicked; they now do nothing, and
  `Music.Play` and `Music.FadeIn` return an "audio: not initialized" error.
  `Music.Free` still closes the decoder before `Init`. Under `-race`, the
  package now runs its `TestRace*` tests instead of skipping everything.
- **Sound calls no longer race `audio.Init` or the audio thread, and do nothing
  before `Init`** — `Sound.Play`, `Sound.FadeIn` and the channel helpers read
  the mixer while `Init` built it, and a sound's volume was written by the app
  while the audio thread read it. Before `Init` they dereferenced a nil mixer
  and panicked. `Sound.Play`, `Sound.PlayOnce` and `Sound.FadeIn` now return an
  "audio: not initialized" error before `Init`, and `HaltChannel`, `IsPlaying`
  and the other channel helpers do nothing. A sound's volume and the output
  sample rate are atomics, so `LoadSoundBytes` and `SampleRate` still take no
  lock and a long decode does not block other audio calls.
- **Android accessibility getters reject negative indices** — the
  gomobile-exported `A11yNode*` getters checked only the upper bound, so a
  negative index from Kotlin (such as the `-1` root sentinel `A11yNodeParent`
  returns) panicked on the slice index and killed the app process. Every getter
  now checks both bounds through one helper and returns its fallback (`0`, `""`,
  or `-1` for `A11yNodeParent`).
- **Custom shaders render on Android** — `glesSetCustomPipeline` marked the
  bound program as "no pipeline", so `glesSetMVP` and `glesSetTM` returned early
  and never wrote the custom program's `mvp` and `tm` uniforms. `mvp` stayed
  zero, every vertex collapsed to the origin, and a custom shader drew nothing;
  its `Params` were lost too. The GLES backend now records which custom program
  is bound and writes both uniforms through its cached locations. Deleting the
  bound custom pipeline also unbinds it, so a rebuilt program that reuses the
  slot does not get stale writes.
- **Rounded clips show their content on Android** — `glesBeginStencilClip` and
  `glesEndStencilClip` bound the stencil program and drew the clip mask without
  writing its `mvp` uniform. Uniforms belong to one program, so the matrix set
  on the solid pipeline just before did not reach it. The stencil program kept a
  zero matrix, the mask covered no pixels, and every child of a rounded clip was
  clipped away. Both functions now take the frame MVP and write it after they
  bind the stencil program, as the desktop GL backend already does.
- **GPU clip rects round outward (#601)** — the GL and Metal backends converted
  a clip box to device pixels by truncating `x`, `y`, `w` and `h` on their own,
  putting the far edge at `floor(x) + floor(w)`. At a fractional DPI scale, or a
  fractional layout coordinate, that is up to a full device pixel short, so
  scrolled and clipped content lost a line of pixels along its right and bottom
  edges. Both backends now floor the near edge and ceil the far edge through a
  shared helper, the rule the software backend already used, so the device rect
  always contains the box. An empty clip stays empty.
- **`BoundedMap` keeps one ordering slot per key after `Delete`** — `Delete`
  left the key's slot in the order list on small maps, because compaction runs
  only past a size threshold. Setting the same key again appended a second slot:
  `Keys` and `Range` returned the key twice, clones copied the duplicate, and
  eviction removed the re-inserted key as the oldest entry, before keys that
  were older. `Delete` now removes the slot in place, with no allocation.
- **NumericInput inner identities join their scope** — the field and step-button
  IDs were composed from the unresolved leaf, stamping window-global identities
  that collided across scopes. They are now composed from the resolved ID (the
  datagrid pattern), the wrapper leaves the tab order to the field with the
  focus ring following it, and a click on the frame focuses the field. A step
  lost to float precision no longer sounds the refusal cue, and the step buttons
  take the `Click` color slot and consume their click.
- **Masked edits keep vertical navigation and local state stays private** —
  masked insert, paste and delete left `cursorOffset` at 0, pinning the next
  Up/Down to the left edge instead of recomputing the column from the caret;
  `formatRaw` scanned forward per literal instead of one suffix pass; and
  `numericLocaleNormalize` aliased the caller's `GroupSizes` and the package
  default instead of cloning.

- **IME preedit and commit input bounded against hostile input methods** — the
  preedit stored per window is capped at 4096 runes in `imeUpdate`, so an
  unbounded composition from any backend cannot grow memory or the per-frame
  render cost; X11 commit strings are stripped of replacement characters and
  capped the same way. X11 commits now arrive as a single `EventChar` carrying
  the whole string, matching the documented contract and every other backend, so
  a multi-rune CJK commit is one insert and one undo step instead of one event
  per rune.
- **IME candidate window follows the caret and rect reports stay finite** — the
  reported rect is anchored to the composition caret instead of the preedit
  start, `IMESetRect` rounds to nearest and clamps centrally (NaN lands on zero,
  infinities on the bound) instead of truncating, and the X11 backend caches the
  scaled rect like Win32 so the per-frame re-report costs a comparison instead
  of a D-Bus call. The web backend reports the selected clause from the hidden
  input's selection during `compositionupdate` instead of always sending a zero
  range.
- **Per-frame focus gates share one tree walk** — the render pass resolved the
  IME edit context and the caret-blink gate in two full walks; both now resolve
  off one depth-capped walk, so a focused frame pays half the traversal.

- **Faded and disabled images now fade the pixels, not just the backdrop** —
  `ImageCfg.Opacity` and the disabled dim reached only the `BgColor` fill while
  every backend painted the texels fully opaque, because the image shaders
  ignored the vertex color and `RenderCmd` carried no image alpha. Commands now
  carry `Opacity` (folded from shape opacity and the disabled dim at emit), the
  GL/Metal/GLES shaders multiply texel alpha by it, and the soft, web and PDF
  backends apply it the same way. A remote image that resolves to SVG keeps its
  ID, click handler, assistive label and sound instead of dropping them at the
  `svgView` handoff.
- **Remote image downloads land atomically with a strict type allowlist** —
  concurrent windows fetching one URL truncated each other's cache file, a
  crashed partial stayed servable under its final name, cache files were
  world-readable, and any `image/*` body (webp, gif) was stored under `.png`
  only to fail decode every frame after. Bodies now stream to a `0600` temp file
  that is renamed into place (a rename loser serves the winner), only
  PNG/JPEG/SVG content types are admitted, and an in-flight URL skips the
  per-frame filesystem probe.
- **Image validation, logging and placeholder text hardened** — `Image` warned
  on every frame for a missing file and embedded the full source in the
  `[missing: …]` layout; warnings are now once per window and the source is
  capped at 80 chars. `validateImagePath` rejects NUL and empty/dot paths like
  the backend gate, remote `.SVG` suffixes match case-insensitively, and the
  downloading placeholder allocates through the shape pool.

- **Canvas `Save` past its depth cap no longer unbalances `Restore`** — a
  `DrawContext.Save` beyond `maxXformDepth` (256) was dropped silently, so the
  matching `Restore` popped an ancestor instead and every nesting level after
  the cap drew at the wrong offset for the rest of the redraw. Dropped pushes
  are now counted and their `Restore`s are no-ops, which keeps the stack
  balanced however deep the nest goes. Nests shallower than 256 are unaffected.

- **Canvas transforms reject an overflow to infinity** — `ScaleBy` and
  `Translate` screened their arguments but not the result, so two
  `ScaleBy(1e38, 1e38)` calls left the matrix at `+Inf`. `DrawContext.Text` also
  drops an entry whose baked position or font size overflows, which a single
  finite `ScaleBy` can cause, as well as one handed non-finite coordinates or
  px-valued style fields with no transform in force. The cell and emoji-box
  widths are screened too, since the shaper reads them. This one matters beyond
  the usual non-finite screening because the canvas text emit path measures a
  style through the glyph shaper before render-command validation runs, so an
  infinite font size reached the shaper's cache and rasterizer.

- **A cached canvas no longer has its emitted geometry overwritten inside one
  render pass** — the cache entry stamped the render pass that last _redrew_ it
  but not one that merely _hit_ it, so two shapes sharing an effective ID and
  differing only in `Version` or size — the first hitting the cache, the second
  redrawing — let the redraw recycle the triangle buffers the first shape's
  already-emitted command still pointed at. A hit now claims the pass as a
  redraw does. Only reachable with duplicate effective IDs, which
  `TestDuplicateIDs` and `gui.Debug` already report.

- **A canvas no longer pins the text and images of its largest past redraw** —
  the recycled `Texts` and `Images` arrays were truncated with `[:0]`, leaving
  every entry past the new length live in the backing array for the life of the
  canvas, including each `Text` string and each image `Src` and `ImageFetcher`
  (which can close over an arbitrary graph). The entries are cleared on reuse;
  the capacity is still kept.
- **A data grid filter input stays inside a narrow column (#640)** — the filter
  `Input` took the theme field min-width floor (160), so any column narrower
  than that drew its input over the next column. The cell already sizes the
  input, so the input now opts out through the new exported
  `InputCfg.NoMinWidthFloor`, previously an in-package-only flag shared with
  `NumericInput` and `InputDate`. Leave it false on a standalone form field,
  where the floor keeps an empty field the width of a filled one.
- **Container alignment stays pinned when content overflows (#636)** — a
  centered or end-aligned row or column with content larger than the container
  moved the content off the start edge. This hid the first items. The alignment
  now treats negative leftover space as zero. This matches the cross-axis
  helpers. The content starts at the edge and clips only at the far side.
- **A horizontal-only scroll column keeps its height floor (#637)** — the Column
  main-axis reset dropped a Scrollable Fill container's minimum to 5px without
  checking `ScrollMode`, so a horizontal-only column could be squeezed below its
  content with no way to scroll to the rest. It now routes through
  `scrollFillResetMin` like every other axis, and the excluded axis keeps its
  floor.

## [v0.76.1] - 2026-09-13

### Fixed

- **Escape dismisses a dialog whose focused content holds its own key handler**
  — The per-dispatch dedup suppressed every focused target after the first, so a
  focused child with an `OnKeyDown` that declined Escape vetoed the dialog
  root's Escape handling: the dialog stayed open and `OnCancelNo` never fired.
  The dialog root now skips the dedup marks. A child that consumes Escape still
  overrides, since post-order dispatch reaches the child first and its consume
  short-circuits the dialog.

## Older releases

v0.76.0 and earlier are in [CHANGELOG-archive.md](CHANGELOG-archive.md).
