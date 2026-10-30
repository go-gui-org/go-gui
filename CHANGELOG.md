# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **Key codes for every printable ASCII character, and `ModAltGr` (#1188).** The
  keys that had no `KeyCode` constant now have one, valued at their character
  code like the existing ones: `KeyExclamation`, `KeyQuotation`,
  `KeyNumberSign`, `KeyDollar`, `KeyPercent`, `KeyAmpersand`, `KeyLeftParen`,
  `KeyRightParen`, `KeyAsterisk`, `KeyPlus`, `KeyColon`, `KeyLess`,
  `KeyGreater`, `KeyQuestion`, `KeyAt`, `KeyCircumflex`, `KeyUnderscore`,
  `KeyLeftBrace`, `KeyBar`, `KeyRightBrace` and `KeyTilde`. `KeyScrollLock`,
  `KeyPrintScreen`, `KeyPause` and `KeyF21`–`KeyF24` are exported. `ModAltGr`
  names ISO level 3 (AltGr), which is not Ctrl+Alt; `Shortcut.String` shows it
  as `AltGr+`. Linux sends them (see Fixed); other platforms do not yet.

- **Shortcuts match the key a press typed: `Event.TypedKey` and
  `Event.TypedModifiers` (#1188).** A key down now also carries the key the
  press produced with Shift, AltGr and NumLock applied, and the modifiers that
  translation did not use up. On a German layout Shift+7 types `/`: `KeyCode`
  stays `Key7` with Shift, `TypedKey` is `KeySlash` with no modifiers. Command
  dispatch tries an exact `KeyCode`/`Modifiers` match first and, only when no
  command matches, `TypedKey`/`TypedModifiers`. So `Shortcut{Key: KeySlash}`
  fires on German Shift+7, `Ctrl+/` on Ctrl+Shift+7, `KeyLeftBracket` on AltGr+8
  and `Ctrl++` on US Ctrl+Shift+=, while a command bound to the exact keys still
  wins. Ctrl+Shift+A stays Ctrl+Shift+A: a press that types the same key keeps
  all its modifiers. On the default US layout every existing shortcut behaves as
  before. The Linux backends (Wayland and X11) set the fields; on Windows, macOS
  and web they are zero and matching is exact, as before.
  [`examples/key_inspector/`](examples/key_inspector/) shows, for every key
  press, its `KeyCode`, `TypedKey`, modifiers and character, and the shortcut to
  bind it with; its terminal output is what a keyboard bug report needs.

- **Custom shaders can read textures: `Shader.Surface` binds a `PixelSurface`'s
  planes as `iChannel0`-`iChannel3` (#1168).** A surface can now hold up to four
  planes in `R8`, `RG8` or `RGBA8`, each with its own size
  (`PixelSurfaceCfg.Planes`), and `UpdatePlanes` publishes all of them as one
  frame, so a shader never sees one plane's new pixels with another's old ones.
  A shader with a `Surface` reads plane _i_ as `iChannel`_i_, Shadertoy style,
  with `iChannelResolution`; sampling is linear and clamps to the edge. This is
  what video needs: NV12 frames go to the GPU as an R8 and an RG8 plane and the
  shader converts them, with no YUV code in go-gui. A new frame redraws without
  rebuilding layout or `Shader.Animate`. Shaders without a `Surface` compile
  exactly as before. `Window.RendersShaders` reports whether the backend runs
  shaders (not the soft renderer or PDF), so a library can fall back to an RGBA8
  `Image`. On Metal an upload into a surface texture waits for an earlier frame
  still sampling it, as `replaceRegion` does not. See
  [`examples/shader_channels/`](examples/shader_channels/).

- **`PixelSurface`: an image the app can update every frame (#1168).** Create
  one with `NewPixelSurface`, pass it as `ImageCfg.Surface`, and call `Update`
  with RGBA8 pixels from any goroutine. `Update` copies the pixels, so the
  producer can reuse its buffer, and keeps only the newest frame. Every window
  that draws the surface redraws without rebuilding its layout, and each
  renderer uploads only when there is a new frame, reusing its texture at a
  fixed size. This is what a video player, camera preview or emulator screen
  needs; until now an image could only change by registering a new one. All
  backends draw it (OpenGL, Metal, WebGL/Canvas2D, Android, iOS, software) and
  PDF export embeds the current frame. `Resize` changes the size and `Close`
  stops it and releases the textures. Images without a `Surface` are unchanged.
  [`examples/pixel_surface/`](examples/pixel_surface/) animates one at 60 Hz.

- **`InputCfg.OnPasteRejected` reports a paste the field refused (#1187).** It
  is called with the whole paste, before any cap, when Ctrl+V, the context
  menu's Paste or the macOS Ctrl+Y yank leaves the field unchanged: the text
  would exceed the field's 65,536-rune budget, the `Mask` has no room for it, or
  `PreTextChange` refused it. Until now only the reject cue marked a refusal,
  and that is silent unless the app installs a sound player. The app can now say
  why ("Paste too large: 92,359 characters"), keep the text, or open it in a
  view that can hold it. `ctx.Layout` is the input's own layout and `ctx.Event`
  is nil, as in `OnTextChanged`.

### Fixed

- **A Wayland window with a libdecor frame no longer quits on its own (#1194).**
  On a compositor that leaves the frame to the client (GNOME, weston), the event
  loop sometimes ended after logging `gl: wayland: libdecor_dispatch: -4`, with
  no user action, and the app quit. libdecor's dispatch ends in a `poll()`,
  which a signal fails with `EINTR`, and the Go runtime sends `SIGURG` to
  preempt goroutines, so a busy app gets one now and then. The backend took the
  interrupted poll for a dead connection. It now counts as a dispatch that found
  nothing, and the loop's next pass picks up the events. Other libdecor errors
  still end the loop. The `GODEBUG=asyncpreemptoff=1` workaround is no longer
  needed.

- **A `ContextMenu` opened near the bottom or right edge of the window opens
  next to the cursor (#1193).** Right-clicking near the bottom of a tall menu
  area, such as a table filling the window, opened the menu at the top of the
  window. The flip that keeps a popup in the window treated the click position
  as a gap from the area's top edge and mirrored it to the area's bottom edge,
  which for a tall area is far above the cursor. The popup now hangs from the
  click point and flips around it: with no room below it opens above the cursor,
  with no room to the right it opens to the left. The same fix applies to the
  link menu on a right-clicked link in `RTF` and `Markdown`, which had the same
  fault in long documents. Where the menu fits below and to the right of the
  cursor, it opens exactly where it did before. The tooltip on an `RTF` or
  `Markdown` run had the same fault: hovered near the top of the window, it
  flipped to the bottom of the text block, far from the word. It now opens 3px
  below the word when there is no room above it.

- **Keys on non-US keyboard layouts get a `KeyCode` on Linux (#1188).** A key's
  `KeyCode` came from its first-level symbol through a US-only table, so on
  German, French, Russian and most other layouts many keys reported
  `KeyInvalid`: `#`, `+`, `<`, `^`, the numpad, NumLock, Menu, the AZERTY number
  row, and every letter on a Cyrillic layout. Shortcuts on those keys never
  fired, numpad arrows did nothing in a text field, and Ctrl+C/V/X/Z did nothing
  on a Russian layout. On Wayland and X11 the code now follows the rules of
  `docs/specs/keyboard-layouts.md`: named and keypad keys by their symbol (the
  numpad navigates with NumLock off), ASCII letters and digits, the digit Shift
  types on AZERTY, on non-Latin layouts the letter of the first configured Latin
  layout (else the US one at that position), and every printable ASCII
  character. AltGr is reported as `ModAltGr`, so AltGr+8 typing `[` no longer
  fires a `Key8` shortcut, and an AltGr chord no longer acts as the plain key
  for focus traversal (Tab) or keyboard scrolling. A key's release reports the
  code of its press. On the default US layout nothing that worked changes; a
  test pins this.

- **On X11, AltGr, NumLock, Caps Lock and non-Latin layouts type the right text
  (#1188).** The X11 backend chose a key's symbol from Shift alone, so on a
  German layout AltGr+Q typed `q` instead of `@` (and `[`, `]`, `{`, `}`, `\`,
  `|`, `~`, `€` could not be typed at all), numpad digits typed nothing with
  NumLock on, Caps Lock typed lowercase letters, and Cyrillic, Greek and other
  non-Latin letters typed nothing. It now applies the AltGr level, the NumLock
  rule for keypad keys, Caps Lock as XKB does (Shift+Caps Lock types the
  lowercase letter), and libxkbcommon's character table when libxkbcommon is
  installed, as the Wayland backend does. Without libxkbcommon, `€` and the
  numpad's characters type too; only non-Latin text needs it. A layout changed
  while the window is open (setxkbmap, the desktop settings) applies to the next
  key press; the keyboard was read only at start. AltGr is recognized on
  whichever modifier the keyboard mapping puts it, and a key an IBus input
  method forwards is reported as the same key typed directly. go-gui now enables
  the XKB extension on X11, as GTK and Qt do: without it Xorg reported a second
  layout as a held AltGr. Key symbols come from the server's XKB map, so a third
  and fourth layout configured together type their own characters too. Where
  only XKB's key types say how a key behaves (a layout's own level order, as on
  some keypads and on `cm(azerty)`'s digit row, ISO level 5, a layout's own Caps
  Lock type, as Turkish declares for i/İ), the X11 result can differ from
  Wayland; the spec lists these.

- **A paste too large for an `Input` is refused whole instead of cut short
  (#1187).** A paste that would take the field past 65,536 runes inserted only
  the first 65,536, with no sign anything was dropped, so pasted JSON, CSV or
  logs ended mid-document. Two smaller pastes that together crossed the budget
  were kept until the next keystroke cut the text silently. The field now stays
  unchanged, sounds the reject cue and calls `OnPasteRejected`. The budget
  counts the text after the paste, so replacing a selection frees room. Typed
  text and IME commits are unchanged. Two cases that used to keep part of a
  paste now refuse it: a `Mask` field handed more than 65,536 runes, which took
  its first matching characters, and a field whose app-set text is already over
  the budget, which now takes a paste only over a selection.

- **Typing or pasting the same digit over itself in a masked `Input` no longer
  counts as refused (#1187).** In a full field such as `111`, typing `1` at the
  start sounded the reject cue and left the caret where it was, so typing over a
  filled mask stalled on every digit that matched the one already there. Pasting
  `123` over a selected `123` did the same and now would also have called
  `OnPasteRejected`. The mask accepted the input; only the text came out the
  same. The caret now moves on and the selection clears, as for any accepted
  edit, with no undo step, since the text did not change.

- **A press anywhere in an `Input` now places the caret and opens its context
  menu (#1186).** In a `Scrollable` multiline `Input`, only the lines holding
  text took a press: a left click below the last line placed no caret, and a
  right click there opened no Cut/Copy/Paste menu. A field scrolled sideways
  past a long word took no press at all, not even on the visible text. The
  handler sat on the inner row that holds the text, which is only as tall as the
  text and moves with the scroll. It now sits on the field, which is where the
  I-beam cursor already shows. So a press in the padding of any `Input` places
  the caret too, at the nearest character, as in a native text field; before, it
  only focused the field. Scrolling is unchanged, and a press on a scrollbar
  still goes to the scrollbar, a right press included. A screen reader's
  activate action (VoiceOver press, AT-SPI press, TalkBack double tap) now
  focuses the field and leaves the caret where it was; before, it did nothing.

- **An open `Select` dropdown now closes when focus leaves the field (#1181).**
  Before, a press on an `Input` or another focusable widget moved focus there
  but left the list open over the form, and a `SetFocus` from app code did the
  same. This is the check `Combobox` got in #1150; both widgets now share it. A
  press on the dropdown scrollbar keeps the list open, a pick in a multi-select
  still leaves it open, and a `FocusDisabled` select skips the check. Opening
  the list now also focuses the field, as `Combobox` does, so a screen reader's
  press opens a list that stays open.

- **A `FocusDisabled` `Select` or `Combobox` now opens on a click (#1183).**
  Before, the click did nothing: the field takes no focus, so the press counted
  as unhandled and the window closed the dropdown again in the same press. A
  `Select` click also took focus away from the widget that held it. The field
  now consumes the press, so the dropdown opens, an option can be picked with
  the pointer, and focus stays where it was, as `FocusDisabled` promises. A
  `FocusDisabled` `Combobox` no longer moves focus to the first tab stop when it
  opens. Picking a `Combobox` row with the pointer now keeps focus where it was,
  as `Select` already did: on the field, or for `FocusDisabled` on the widget
  that held it. Before, the pick cleared focus.

## [v0.93.0] - 2026-10-25

### Added

- **`Window.HTML` renders a small, safe subset of HTML (#1158).** Use it to
  preview HTML that something else shows, such as a promo snippet an admin
  writes for a mobile app. It takes the same `MarkdownCfg` as `Window.Markdown`
  and draws through the same renderer, so styles, selection, links and the image
  policy work the same. Supported: `p`, `div`, `br`, `hr`, `h1`–`h6`,
  `b`/`strong`, `i`/`em`, `u`, `s`/`del`, `mark`, `sup`, `sub`, `code`, `pre`,
  `blockquote`, `ul`/`ol`/`li`, `a href` and `img src alt width height`. Other
  tags are dropped and their text kept. `script`, `style` and similar elements
  are dropped with their content. Links and image sources pass the same safety
  checks as Markdown, and remote images stay blocked unless `AllowRemoteImages`
  is set. The text goes straight into the document, so `*`, `_`, `#` and `|`
  stay literal; converting HTML to Markdown text in the app changed their
  meaning. There is no CSS and no script: this is not a browser. The parser is
  `markdown.ParseHTML`. See `examples/html_preview/` and the HTML page in the
  showcase.

- **`Table` gets an optional row header column and a select-all corner, like the
  WinForms `DataGridView` (#1156).** Set `TableCfg.RowHeader` to put a narrow
  selector cell in front of every row. A click on it selects that row. Under
  `MultiSelect`, Ctrl+click (Cmd+click on macOS) toggles the row and Shift+click
  selects a range. On the header row the cell is the corner: a click there
  selects every data row. The row that was clicked or moved to last shows a ▶
  marker. The cell is not part of `Data`, so column widths, alignments, and your
  sort and copy code do not see it. Selection still goes through `Selected` and
  `OnSelect`, and it keys on row index, so it stays stable while virtualization
  scrolls rows out of view. `RowHeaderWidth` sets the width (default 24). A
  focusable table with a row header also takes Shift+Space (select only the
  current row) and Escape (clear the selection; with nothing selected, Escape
  travels on). Any focusable `MultiSelect` table now takes Ctrl+A (Cmd+A on
  macOS) to select every data row. The showcase table demo has a "Row header"
  toggle.

- **A right press on an `Input` opens a Cut, Copy, Paste and Select All menu
  (#1151).** Before, a right press in a text field did nothing. Copy and Cut act
  on the current selection, which the press keeps. Cut and Paste are off in a
  `ReadOnly` field, Copy and Cut are off in a password field, and Paste is off
  when the clipboard is empty. A paste from the menu goes through the field's
  mask and `PreTextChange`, the same as Ctrl+V. The field keeps its focus look
  while the menu is open, and it does not fire `OnBlur` or a blur
  `OnTextCommit`. Escape closes the menu and gives focus back to the field. The
  menu also works in an `Input` inside a dialog. To use your own menu, set
  `InputCfg.ContextMenuDisabled` and wrap the field in a `ContextMenu`. A
  `FocusDisabled` field gets no menu.

- **`examples/solar_system` gets pause, speed and mute controls, a close button,
  nav-dot names and a phone-width layout.** Space pauses the orbits, Up and Down
  double and halve their speed (0.25x to 16x), and M mutes the ambience track.
  Buttons for mute and pause, and a speed readout, sit beside the zoom buttons.
  The camera keeps running while the orbits are paused. The fact sheet has a ×
  button, because a touch screen has no Escape key. Hovering a nav dot names its
  body with the canvas tooltip. Below 792 px wide, the fact sheet shrinks to the
  window and puts its six cards in two rows of three. Before, it ran off the
  edge of a narrow window. The embedded track is now FLAC (3.05 MB, was a 4.08
  MB WAV), lossless so the loop stays seamless, and low-passed at 3.5 kHz to cut
  the 8-bit source's hiss by 13 dB.

- **`KrokiMermaidFetcher(baseURL)` renders Mermaid diagrams through a
  self-hosted Kroki server (#1147).** Set
  `MarkdownCfg.MermaidFetcher: gui.KrokiMermaidFetcher("http://localhost:8000")`.
  Before, the Kroki address was fixed to the public `https://kroki.io`. When
  that service was slow or down, diagrams did not render. To use another server,
  an app had to write its own fetcher and copy the size limits and error
  handling. The new fetcher keeps all of them and uses the shared diagram HTTP
  client. A self-hosted server also keeps diagram source on your own network.
  The default is unchanged.

- **`cmd/shaderport` ports a Shadertoy shader to a `*gui.Shader` (#573).** Run
  `shaderport -name myShader -o my_shader.go my.frag` on a shader with
  `mainImage(out vec4, in vec2)`. The tool writes a Go file whose `*gui.Shader`
  has both bodies: GLSL for OpenGL, WebGL2 and Android, and MSL for macOS and
  iOS. Before, a copied shader was GLSL only and drew nothing on Metal. The tool
  is a token rewriter, so the output keeps your comments and macros and reads as
  a diff to review. It handles helper functions, structs, `const` globals,
  macros, `out` parameters, and the GLSL names that MSL spells differently. It
  translates `mod` with GLSL's floor rule, because MSL `fmod` truncates. It sets
  `Animate` when the shader reads `iTime`, `iTimeDelta` or `iFrame`. A construct
  it cannot translate (`iMouse`, `iChannel*`, `texture()`, a mutable global)
  stops it with the line and column, and it writes no file.
  `examples/custom_shader` adds a ported tile, and `docs/shader-porting.md`
  lists what the tool supports and rejects.

- **`Shader.GLSLDecls` and `Shader.MetalDecls` hold program-scope declarations
  (#573).** A body runs inside the fragment function, so it cannot define a
  helper function or a struct. The two new fields go before that function, on
  every backend. A GLSL helper can read the built-in inputs, which are globals
  there. In MSL they are locals of the fragment function, so a helper takes them
  as parameters. `gui.BuildGLSLFragmentDecls` and `gui.BuildGLSLESFragmentDecls`
  wrap a body together with its decls. `ShaderHash` now covers all four strings.

- **`gui.ShaderHashCache` memoizes `ShaderHash` per `*Shader` (#573).** Each
  backend calls `ShaderHash` on every custom-shader draw of every frame, and it
  reads every byte of the source. For a ported shader of 32 KB that took about
  30 µs per draw. Every GPU backend now keeps one cache per window: a hit takes
  about 4 ns and allocates nothing. The cache compares each string's data
  pointer and length, so a field set to new text gets a new hash.

### Changed

- **BREAKING: an `Input` inside a `ContextMenu` must now set
  `ContextMenuDisabled` to keep the app's menu (#1151).** A right press on an
  `Input` now opens the built-in Cut, Copy, Paste and Select All menu and
  consumes the press, so a `ContextMenu` wrapped around the field no longer gets
  it. To keep your own menu, set `InputCfg.ContextMenuDisabled: true` on the
  field.

- **A menu item now runs when the mouse button comes up, not when it goes
  down.** This applies to every menu: `ContextMenu`, `OverflowPanel`, `Menubar`
  dropdowns, submenus and the Input menu. The item runs only when the press and
  the release are both on it, so a press dragged from another item, or the
  release of the right press that opened the menu, runs nothing. A press inside
  an open menu no longer closes it: a press on a disabled item, a separator or
  the menu padding leaves the menu open, as in a native menu. An item with a
  submenu still opens the submenu on the press. A right click on an item no
  longer runs it. A test that sends only a mouse down to a menu item must now
  send the release too; `Window.TestClick` already sends both.

- **Touch: a long press that no handler consumed lets the finger go when it
  drags, so a finger that rests before it drags scrolls (#1117).** A touch held
  still for 500 ms became a long press, and from then on its moves were ignored:
  the page did not scroll until the finger lifted. A reader who placed a finger
  carefully, for example aiming at the thin scrollbar and missing it by a few
  pixels, saw the page freeze. Now a move past the pan distance cancels the long
  press (`GesturePhaseCancelled`, not `Ended`), releases the press it sent where
  the finger now is, and scrolls. A long press still keeps the finger when its
  press took the mouse lock (a reorderable row, a text selection, a scrollbar
  thumb, which the finger then drags) or when an `OnGesture` handler consumed
  its `Began`. An app whose `OnGesture` acts on `GestureLongPress` without
  `ctx.Consume()` changes behaviour: a finger that drags after the long press
  now gets a `Cancelled` phase and pans. Call `ctx.Consume()` in that handler to
  keep the finger, as the showcase gesture pad now does.

### Fixed

- **The open list of a Select or Combobox is at least as wide as the field
  (#1155).** Before, a Select list was only as wide as its longest option, so a
  `FillFit` field in a wide column opened a narrow list. A Combobox list took
  the field's width, but its rows kept their own width, so the hover color
  stopped short of the edge, and an option wider than the field was cut off. Now
  both lists are as wide as the field, or wider when an option needs more space.
  The rows fill the list. `MaxWidth` is still the upper limit.

- **A macOS app built with `CGO_ENABLED=0` fails to link instead of panicking at
  launch (#1159).** The macOS backend needs cgo. Before, a darwin build with cgo
  off linked, and the app panicked at start with "no native backend available".
  Now the link fails and the error names the cause:
  `relocation target .../gui/backend.macOS_requires_cgo__set_CGO_ENABLED_1 not defined`.
  Set `CGO_ENABLED=1` to build for macOS. `go vet` and lint with cgo off still
  pass, and builds for other platforms do not change.

- **Touch: a vertical pan over a horizontally scrolling child scrolls the page
  (#1118).** Before, a page barely moved while the finger was over a child that
  scrolls only sideways, such as a wide Markdown table: the table took each
  move's sideways drift, and the vertical part never reached the page. A pan now
  locks to one axis when it starts more than twice as far along it as across, as
  on iOS and Android, and scrolls only on that axis until the lift. A diagonal
  start scrolls both axes. Each axis goes to the innermost scrollable that moves
  on it, so a diagonal pan over the table scrolls the table sideways and the
  page vertically, and the lift flings each container on its own axis. The lock
  applies to scrolling only: an `OnGesture` handler still reads the finger's raw
  deltas, less any axis a scrollable inside it took. Note for apps: an
  ancestor's `OnGesture` now runs on a pan move that an inner scrollable took
  only one axis of, with that axis zeroed. Before, the inner scroll stopped the
  walk. A handler that must stay quiet while an inner scrollable moves (a
  swipe-to-dismiss) checks for a zero delta on its axis.

- **An open menu no longer passes hover to the widgets under it.** Before, the
  pointer over a disabled item, a separator or the edge of a `ContextMenu`,
  `OverflowPanel`, submenu or Input menu also hovered what was below the menu,
  so a button under it showed its hover color. The menu panel now takes the
  hover. Enter, Space or a click on a menu item now keeps a focus that either
  action set, the item's `MenuItemCfg.Action` or the menu's `MenubarCfg.Action`.
  Before, Enter and Space always cleared focus after the action, and a click
  kept only a focus set by `MenubarCfg.Action`. While a modal dialog is open,
  the layers above it (the Input menu, the inspector) now take hover too;
  before, only the dialog's own shapes did. Found while testing #1151.

- **A widget shows its pressed color only for a press that began on it.**
  `Button`, `Input`, `InputNumeric`, `Radio`, `Switch`, `Toggle`, `ExpandPanel`
  and `Breadcrumb` read the held left button to draw their pressed color, so a
  press that began on another shape and dragged over them made them look
  pressed. A menu item picked over a button left that button pressed until the
  release. They now use the shape the press went down on, the same rule
  `Window.IsPressed` uses. `GoldenCfg.MousePressed` now records its press on the
  shape under `HoverX`, `HoverY`. Found while testing #1151.

- **An open `Combobox` dropdown now closes when focus leaves the field
  (#1150).** Before, a press on an `Input` or another focusable widget moved
  focus there but left the dropdown open over the form. The dropdown now also
  closes on a press in empty space, which clears focus. A press on the dropdown
  scrollbar keeps it open, and a press on a row still picks that row.

- **`audio.LoadMusicBytes`: embedded MP3, OGG and FLAC tracks play.** `Play` and
  `FadeIn` seek the track to its start, but in-memory MP3, OGG and FLAC data
  reached the decoder through a reader that could not seek. FLAC and OGG
  returned `seek music: ... not enabled` and stayed silent. MP3 panicked with a
  divide by zero inside go-mp3. Only WAV worked. All four formats now get a
  seekable reader. `LoadMusic` from a file path was not affected.

- **`examples/solar_system`: motion stays smooth on long runs, keeps pace when
  frames drop, and `-screenshot` plays no audio.** The simulation clock was
  `float32`. After about a day, one float32 step was larger than a tick: Earth
  froze on some ticks and jumped on others. The clock is now `float64`, and
  every orbit, spin and twinkle angle is reduced to a fraction of a turn before
  float32 math. Each tick also advanced a fixed 16 ms, so a slow frame slowed
  the orrery. A tick now measures the real time since the last one, capped at
  100 ms so a laptop wake does not jump the planets. And `-screenshot` started
  the ambience track, because `soft.RenderToPNG` runs `OnInit`. The track now
  starts in `main`, on the interactive path only.

- **SVG: geometry from CSS (`r`, `cx`, `cy`, ...) renders, so the showcase
  `var() + calc()` sample is no longer blank (#1145).** A circle whose radius
  came only from a stylesheet (`.v { r: calc(var(--r) - 4); }`) parsed to zero
  paths: the cascade dropped the unknown property and the shape parsers read
  only the `r` attribute. The SVG2 geometry properties now ride the normal
  cascade (presentation attribute < author rule < inline style, with
  `var()`/`calc()` resolved), and the circle/rect/ellipse/line parsers use the
  cascade winner. A bogus rule value is ignored instead of clobbering the
  attribute, a negative `r` still disables rendering, and geometry never
  inherits from the parent.

- **Touch: a slow drag no longer jumps the content where it starts (#1117).** A
  pan is recognized after 10 px of travel, and its first move caught up those 10
  px in one step, so the content sat under the finger. A fast swipe hid it; a
  slow drag, such as one that rested first, showed a 10 px jump. The pan now
  starts where the finger crossed the threshold and drops the distance, as
  Android does and `DragScroll` containers already did: the content stays 10 px
  behind the finger. The pan's `GestureDX` / `GestureDY` drop it too, so an
  `OnGesture` handler that sums them lags the finger by the same 10 px, and a
  short, slow flick reads a little slower in `VelocityX` / `VelocityY` and is
  less often a `GestureSwipe`. The fling after a lift reads the finger's real
  positions and is unchanged.

- **Android: temp files land in the app's own directory, so icons and other
  temp-file assets load (#1140).** Go's `os.TempDir()` fell back to
  `/data/local/tmp`, which an app cannot write: a Go shared library reads its
  environment once when it loads and never sees the `TMPDIR` Android sets later
  through libc (from Android 13), and before Android 13 Android sets none. The
  icon font failed to load, so every icon showed as a box, and anything else
  written with `os.CreateTemp` failed too: app fonts added as bytes, the diagram
  cache, PDF printing, the showcase's images. `TMPDIR` is now the app's cache
  directory on every Android version: the backend takes it from libc where
  Android set it, and below Android 13 uses the directory the host passes from
  `Context.getCacheDir()` to `SetCacheDir`. Android trims that directory when
  storage runs low, so the remote image cache persists across launches, and all
  processes of one app share it. A host calls
  `android.SetCacheDir(cacheDir.absolutePath)` before `Start`, as
  `examples/android_demo` now does; without it, temp files still fail below
  Android 13. Android fonts loaded from bytes use stable cache files rather than
  leaking new temporary files on every launch. The remote image cache now
  resolves its directory on first use instead of at package init, so it follows
  the same `TMPDIR`.

- **Web: a NaN, infinite or out-of-range event field reads as 0, and a fake
  touch list cannot hang the page (#1131).** A script can dispatch an event
  whose `offsetX` or `deltaY` is NaN, ±Inf or too large for a `float32`. The web
  backend passed the value on to `Event.MouseX` or `ScrollY`, and from there
  into hit tests and scroll offsets. It now reads such a value as 0, as it
  already did for a missing field. Integer fields (`keyCode`, `deltaMode`, a
  touch `identifier`) do the same. A touch list that claims a huge `length` made
  the backend scan it without end; it now looks at no more than 64 entries.
- **Touch: a long press, then a drag, reorders a list on a scrolling page
  (#1134).** Since #1116, a touch on a reorderable `ListBox` or `Tree` row on a
  page that scrolls vertically waited to see a tap or a scroll, and a vertical
  drag counted as a scroll: the row could not be reordered by touch. A long
  press armed the drag, but its moves were dropped. Now the moves after a long
  press go to the drag, so holding a row for half a second and then dragging
  reorders it, as on iOS and Android. A quick swipe still scrolls the page. A
  touch drag that the platform cancels, or that a second finger takes over, is
  now cancelled through the lock's `Cancel` hook instead of released, so it no
  longer drops a reordered row, a docked panel or any other drag where the
  finger happened to be. The moves after a long press drive any drag this
  touch's press started, so a row held and then dragged on a list that does not
  scroll reorders too, and a held text selection grows as the finger moves. A
  drag already held when the finger landed is another drag's: it gets none of
  the moves, and an abort of this touch does not cancel it.
- **Android: an app returning from the background is drawn again, not left black
  (#1136).** `GLSurfaceView` destroys its GL context while the app is in the
  background, and the next frame ran in a new context with the old programs,
  buffers and textures, so only the background color appeared. The new
  `android.SurfaceCreated` tells the backend about the new context. It then
  rebuilds its GL resources without deleting the dead ones: images and custom
  shaders are uploaded again on first use, and the glyph atlas is sent again
  from its CPU copy before the first frame draws, so text does not flash blank.
  The window, its view and app state, and the loaded fonts stay as they were. If
  the rebuild fails, the backend logs it once and tries again each frame; the
  app no longer has to crash. A host must call `SurfaceCreated` from
  `GLSurfaceView.Renderer.onSurfaceCreated`, as `examples/android_demo` now
  does. A host built from that demo should also set
  `preserveEGLContextOnPause = true` on its `GLSurfaceView`: Android then
  usually keeps the context, and a resume rebuilds nothing.
- **Android: rotating the device lays the window out at the new size (#1138).**
  A host calls `Start` from every `onSurfaceChanged`, and on a running backend
  `Start` only resized the GL viewport. The window was never told, so after a
  rotation, a split-screen change or a fold it kept laying out at the size it
  started with: in landscape the content stayed portrait-wide with the rest of
  the screen empty. `Start` now sends the window the new size, as `Resize`
  already did. A host needs no change.

## [v0.92.0] - 2026-10-21

### Added

- **Content transitions on `Text` (#1021).** The new `TextCfg.Transition`
  animates a change of `Text`. `ContentTransitionNumeric` rolls only the
  characters that change, compared by grapheme from the right, so "41" to "42"
  rolls the "1" and leaves the "4" still, and "99" to "100" rolls three columns.
  A larger number comes up from below and a smaller one comes down from above;
  `Down` forces down. Each rolling character is clipped to the line, and the
  text asks for tabular figures, so where the font has them its width does not
  change with the digits. `ContentTransitionFade` fades the old text out and the
  new text in. `ContentTransitionReplace` shrinks an old icon away while the new
  one grows in. Under reduced motion, and for wrapped or multiline text, every
  kind fades. A focused text, or one with a selection, shows the new text at
  once, so its caret and selection stay. Every frame is render-only: no view
  function and no layout runs while the text moves. The text needs an `ID`, as
  `TextCfg.Anim` does. See `examples/content_transition/` and the showcase's
  Content Transition page.

- **Single-instance mode and open events (#1106).** `instance.Acquire(id)` in
  the new `gui/instance` package elects one primary process per app ID and user.
  A later launch sends its arguments to the primary and gets `primary=false`, so
  it can exit. `Lock.Attach(app)` turns this launch's arguments and every
  forwarded launch into a `gui.OpenEvent` (files and URLs) for the new
  `App.OnOpen`, which runs on the main thread and rebuilds the view after. On
  macOS, files opened from Finder or the Dock and links in a scheme the bundle
  registers reach `App.OnOpen` too, at launch and while the app runs.
  `App.DispatchOpen` and `gui.ParseOpenArgs` are exported for apps that feed
  their own events. The election uses `flock` on Unix and a named mutex on
  Windows. The kernel frees it when the primary dies, also on `kill -9`.
  Single-window `backend.Run` has no `App`, so it gets no open events; use
  `backend.RunApp`. See `examples/single_instance/`.

### Fixed

- **iOS Safari opens the soft keyboard when a text field is tapped (#1126).**
  The web backend focused its hidden input from the next frame
  (`requestAnimationFrame`), after the touch handler had returned. iOS opens a
  keyboard only for a focus made while it handles the touch, so a tapped field
  showed a blinking caret but no keyboard came up. A press that focuses an
  editable field now starts the input method and shows the keyboard inside the
  press itself: at the touch-down, or at the lift where a scrollable holds the
  press until it knows the touch is a tap. A press that leaves the field, or
  that focuses a widget the next frame builds, is still settled in the frame.
  Other platforms get the same calls one frame sooner.
- **Web: a saved password no longer stops the app (#1128).** Picking a saved
  password from Chrome's autofill froze the page: no tap, scroll or key reached
  the app again. Autofill sends plain `keydown` and `keyup` events that carry
  none of the keyboard fields, and the backend read them with `js.Value.Bool`,
  which panics on a missing value. The panic stopped the Go program. Every event
  listener now reads a missing field as its zero value: key, mouse, wheel and
  touch events and the dark-mode `change` event, so an event that a page script
  or an extension builds with `new Event()` cannot stop the app either. A mouse
  press or release with no button is dropped, not read as a left click at the
  top-left corner, and a touch list entry that is not a touch is skipped.

- **macOS: Apple Events reach a running app (#1106).** The event loop polled
  with a fixed list of event types. AppKit dispatches Apple Events only when the
  poll asks for every type, so a document opened while the app ran, a Dock
  reopen and other Apple Events were lost. The loop now polls with
  `NSEventMaskAny`, as `[NSApp run]` does.

- **A Wayland window without a frame provider falls back to X11 (#1121).**
  v0.91.0 made Wayland the default. GNOME draws no window frames, so go-gui
  needs libdecor and its GTK plugin there. Without them the window had no title
  bar, no close button and could not be moved. Now the first window falls back
  to X11, and XWayland draws its frame, as in v0.90.0. A warning names
  `libdecor-0-plugin-1-gtk`, which gives native Wayland with a frame.
  `GOGUI_WAYLAND=1` keeps the frameless Wayland window. KDE and wlroots
  compositors draw the frame themselves and are not affected.

- **Wayland windows no longer show dark edges on borders, text and switches
  (#1121).** The blend function also blended the alpha channel, so a translucent
  draw over an opaque pixel left the pixel's alpha below 1. On X11 the window
  has no alpha channel and this did not show. A Wayland buffer carries the alpha
  to the compositor, so anti-aliased edges, text and translucent borders let the
  desktop, or black in a screenshot, show through. An opaque Wayland window now
  sets its alpha to 1 at the end of each frame. A `Transparent` window is
  unchanged.

- **Scrolling a list by touch no longer clicks the row under the finger
  (#1116).** A touch-down sent the mouse press at once, and a press fires
  `OnClick`, so a scroll that started on a row picked the row; in the showcase
  catalog on a phone it also closed the drawer. Inside a scrollable that can
  scroll, the press now waits for the finger: a lift before the 10 px pan
  threshold is a tap and clicks, at the lift; a pan the scrollable can follow,
  diagonal ones included, drops the press; a pan clearly along the other axis,
  or a long press, sends it. A tap that stops a fling only stops it and picks
  nothing, as on iOS and Android. On a scrollbar the bar's axis decides: a pan
  along it drags the thumb, a pan across it scrolls the content, and a tap still
  jumps the track. Outside a scrollable, in content that fits and in a
  `DragScroll` container the press goes out at touch-down, as before. Mouse
  input is unchanged. This applies on Android, iOS and the web backend on
  phones.

- **The scrollbar thumb follows a finger (#1116).** A touch drag on the thumb
  did not move it: mouse moves synthesized from touch carried no `MouseDX/DY`.
  Past the pan threshold the content panned with the finger instead, so pulling
  the thumb down scrolled up. Touch moves now carry their delta, and a pan no
  longer scrolls the content while a press holds the mouse lock, as a thumb or
  slider drag does. A `DragScroll` container's lock is the exception, so a
  vertical swipe on a horizontal carousel still scrolls the page, and a sideways
  swipe on it does not move the page.

## [v0.91.0] - 2026-10-19

### Added

- **A touch flick keeps the content gliding (#1110).** A scroll container panned
  with a finger used to stop dead when the finger lifted. A pan that lifts while
  the finger still moves now flings: the content starts at the finger's speed,
  slows exponentially (time constant 0.4 s) and stops, or stops on the scroll
  edge it reaches. A touch on gliding content catches it there, and a wheel
  notch or a key takes over from where it is. This applies to the touch-pan
  fallback of every `Scrollable` container on Android, iOS, the web backend on
  phones and Wayland touchscreens.

- **Custom shaders get Shadertoy's built-in inputs for size, time and frame
  (#573).** Each `RenderCustomShader` command now carries a built-in block,
  filled in while the frame's commands are built: `iResolution`, `iTime`,
  `iTimeDelta` and `iFrame`. Units follow Shadertoy, so a copied shader reads
  them unchanged: sizes are physical pixels. A backend reads the block with
  `RenderCmd.ShaderBuiltinsAt(scale)`, which converts sizes with the scale of
  the frame being drawn, so the first frame on a HiDPI display is right as well.
  Time and frame advance once per displayed frame and read zero in headless
  captures and for a user who asked for reduced motion. `iTimeDelta` is capped
  at 0.25 s, so the first frame after a pause does not jump. `iMouse` is not
  tracked: it reads zero, Shadertoy's value for a canvas never clicked. The GLSL
  wrapper (`BuildGLSLFragment`, `BuildGLSLESFragment`) declares the five names
  as globals, so a GLSL body can use them as written; a body that declares its
  own `iTime` hides the built-in and still compiles. The Metal wrapper, shared
  by macOS and iOS, declares them as locals and runs the body in a nested block,
  so the same holds for an MSL body. Every GPU backend uploads the block on
  every custom-shader draw: desktop GL (Linux, Windows), WebGL2, Android, macOS
  and iOS.

- **`Shader.Animate` keeps an animated shader moving without a view rebuild
  (#573).** An animated shader used to need a repeating `Animate` with an empty
  callback, which rebuilt the view and the layout on every frame, and an app had
  to remove it again when the shader left the screen. With
  `Shader.Animate: true` the window keeps frames coming while such a shader is
  drawn, and each frame rebuilds only the render commands, where `iTime` is
  filled in. Frames stop in the first frame that draws no `Animate` shader, so
  an idle window stays idle. For a user who asked for reduced motion the shader
  holds still, as in a headless capture: its clock reads zero and no frames
  start. `examples/custom_shader` and the showcase's shader page now use `iTime`
  and `Animate`, with no clock or animation of their own.

- **`llms.txt` at the repository root gives AI coding tools a map of Go-Gui
  (#1092).** The file follows the llms.txt format. It states the API rules that
  compile when broken but give wrong behavior: `Cfg` structs, `ctx.Consume()`,
  focus and IDs, constructors for colors and sizes, theme roles and
  `GOGUI_DEBUG`. It also links to the README, the cheat sheet, the guides, the
  examples and the API reference. An assistant that reads it writes app code
  that follows these rules on the first try, and does not copy patterns from
  other GUI toolkits.

- **`LocaleFormatNumber` and `LocaleFormatCurrency` format values with the
  active locale (#1087).** `Locale` holds number and currency rules, but until
  now only `NumericInput` and the data grid's row and page labels applied them,
  so an app that showed a price in a `Text` had to copy gui code.
  `LocaleFormatNumber(v, decimals)` uses the locale's decimal separator, digit
  grouping (`GroupSizes`, last size repeats) and minus sign.
  `LocaleFormatCurrency(v)` adds the currency symbol by `Position` and
  `Spacing`, with `Currency.Decimals` digits: `$1,234.50` in en-US, `1.234,50 €`
  in de-DE. A value that rounds to zero has no minus sign, and an exact tie
  rounds to even, the same as `NumericInput`. Both read the locale without a
  copy and allocate only the result, so a view can call them every frame. The
  data grid's row, page and match labels now allocate one string less each.

- **The showcase has a Locale page, and its locale button says what it does
  (#1084).** The button under the catalog switches the locale, but the showcase
  text is not translated, so a press seemed to do nothing. The new App & Window
  › Locale page reads everything from `CurrentLocale()`: number, currency and
  date formats, weekday and month names, the built-in dialog, action and data
  grid strings, app strings looked up with `LocaleT`, and a `NumericInput` that
  takes the locale's separators. Its own locale switch stays in step with the
  button. The button now has a tooltip that names what it changes and points at
  the page, and an accessible label that names the current locale.

- **`Event.IMEDeleteCount` and `Event.IMEDeleteInCodePoints` carry an input
  method's deletion (#1107).** On a Backspace or Delete key-down they say how
  many units an input method asked to delete before or after the selection,
  which stays, in UTF-16 code units or in code points. The Android backend sets
  them for `deleteSurroundingText`. `Input` honours them, so one emoji goes as
  one character. The count is exact, even inside a character made of several
  code points, because the input method asks for part of one on purpose
  (Backspace on Devanagari "कि" takes only the vowel sign). A masked `Input` and
  a handler that ignores them delete one character, as for a key press; a custom
  text widget can read the count to delete as asked.

### Changed

- **BREAKING: Linux apps use the native Wayland backend on a Wayland desktop
  (#1113).** The Wayland backend is no longer experimental. Before, an app ran
  through XWayland unless `GOGUI_WAYLAND=1` was set. Now an app on amd64 or
  arm64 opens native Wayland windows when `WAYLAND_DISPLAY` is set, as GTK, Qt
  and SDL do. This gives sharp text at fractional scales, and the GPU on devices
  with no X11 EGL platform. If Wayland setup fails (no libwayland, no
  compositor), the app logs one warning and uses X11, as before. To keep X11,
  set `GOGUI_WAYLAND=0`. `GOGUI_WAYLAND=1` still tries Wayland when
  `WAYLAND_DISPLAY` is unset. An X11 session with no `WAYLAND_DISPLAY` does not
  change.

- **BREAKING: Markdown no longer loads images unless the app allows it
  (#1105).** Markdown is often untrusted: chat messages, fetched READMEs, user
  notes. Before, an http(s) image was downloaded when the document rendered, so
  a tracking pixel told its host the reader's IP address and when the document
  was opened. An image path was read from disk. Now both are off by default. A
  blocked image shows a picture icon and its alt text, and does no I/O. Data
  URLs and `UseImage` sources do no I/O and still show. Migration: for a trusted
  document, set `MarkdownCfg.AllowRemoteImages` to load http(s) images, and
  `MarkdownCfg.AllowLocalImages` to load image paths.
  `WindowCfg.AllowedImageRoots` still limits the paths. Separately, the default
  image fetcher now refuses to connect to link-local, multicast and unspecified
  addresses, on the first request and on every redirect. This stops an image URL
  from reaching a cloud metadata service (`169.254.169.254`). Loopback and
  private LAN addresses are still allowed. Through an HTTP proxy, only URLs that
  name an IP address are checked, because the proxy looks up host names. A
  custom `WindowCfg.ImageFetcher` is not changed.

- **A widget with an accessible name no longer allocates its accessibility data
  on each frame (#1097).** A container, image, SVG, canvas, rich text, tree,
  slider or thinking orb with an `A11YLabel`, an `A11YDescription` or an
  `A11YValue` got a new heap object for this data on every frame. Now the data
  comes from the same frame pool as the shape that holds it. A tree row and a
  list box row each allocate one object less per frame: a 200-row tree goes from
  1022 to 822 allocations per frame, and a list box from 212 to 147.

- **A `Text` takes 44% less memory per frame (#1097).** Each `Text` is built
  anew every frame, and its view carried a text configuration and an accessible
  name that are also rebuilt every frame. Both now come from the frame pool with
  the shape that uses them, so a `Text` drops from 576 to 320 bytes. A 200-row
  tree builds 29% fewer bytes per frame (666 KB to 474 KB), and a list box 22%
  fewer, so the garbage collector runs less often. The same `Text` placed twice
  in one view now gives each copy its own configuration. A container with a
  `Title` no longer allocates on each frame; it allocated four objects before.

### Fixed

- **Typing on a phone's soft keyboard reaches web text fields (#1107).** Android
  Chrome reports soft-keyboard keys as `keydown` with key `Unidentified`
  (keyCode 229) and delivers the text only as `beforeinput` on the hidden input,
  which the web backend did not read. Letters, digits, spaces and picked
  suggestions were lost, so a field in a WASM app such as the showcase stayed
  empty on a phone. The backend now types `insertText` and
  `insertReplacementText` from `beforeinput`, and maps `deleteContentBackward`
  and `deleteContentForward` to Backspace and Delete, and a line break to Enter.
  A key a `keydown` already typed is not typed twice, and the hidden input stays
  empty, so the keyboard cannot autocorrect text the field does not hold. A
  `keydown` with keyCode 229 types nothing itself, because Android Chrome
  ignores its `preventDefault` and the `beforeinput` that follows types the key.
  A soft keyboard's Backspace or Enter `keydown` with an empty `code` now maps
  from its `key`, and Delete on the hidden input cancels its default, as Enter
  does, so its `beforeinput` does not delete a second character.

- **Backspace and Enter on the Android soft keyboard reach the field (#1107).**
  Gboard sends both as key events, because the host view reports none of the
  field's text, and the bridge dropped them. The Android backend now turns a
  `"\b"`, `"\x7f"` or `"\n"` commit into a Backspace, Delete or Enter key-down,
  and the demo host sends `KEYCODE_DEL`, `KEYCODE_FORWARD_DEL` and
  `KEYCODE_ENTER` that way. A keyboard's `deleteSurroundingText` and
  `deleteSurroundingTextInCodePoints` were dropped too: each removed character
  arrived as a `"\b"` character, which a field ignores, and a count in UTF-16
  units would have taken an emoji's neighbour with it. The host now passes the
  range and its unit to `android.IMEDeleteSurrounding`, which sends it as
  Backspace and Delete key-downs carrying the count (see
  `Event.IMEDeleteCount`). A host built from the demo picks this up by copying
  its `sendKeyEvent`, its two `deleteSurroundingText` overrides and the
  `IMEDeleteSurrounding` bridge function.

- **A touch pan scrolls as far as the finger moves (#1110).** It scrolled 20 px
  per pixel of finger travel, because the pan's pixel delta went through
  `Theme.ScrollMultiplier`, which turns wheel lines into pixels. A small finger
  movement threw the content a long way on Android, iOS, phones running the web
  backend and Wayland touchscreens. The content now stays under the finger.
  `DragScroll` containers already moved 1:1 and are unchanged, and so are the
  mouse wheel and the keyboard on every platform.

- **Wayland touchpads scroll like macOS trackpads (#1110).** They scrolled about
  20 px per point of finger travel: the backend reported raw points as a precise
  scroll, which the gui side multiplied by `ScrollMultiplier`. Wayland now
  converts points to fractional lines at the factor macOS uses (0.075), so a
  touchpad moves 1.5 px per point on both. An app that reads `ScrollY` of a
  precise Wayland event directly gets these lines, about 13 times smaller than
  before. `Event.ScrollX` / `ScrollY` now document the unit: lines everywhere,
  fractional for a trackpad. A seat before `wl_pointer` v5 sends a wheel as
  continuous values only, like a touchpad; its notch now scrolls three lines,
  eased, as on the other backends, where it scrolled ten.

- **A GLSL shader body may reuse the wrapper's variable names (#573).** The GLSL
  wrapper spliced the body into `main` beside its own locals (`d`, `q`, `pos`,
  `radius`, `sdf_alpha` and others), so a body that declared one of these common
  names failed to compile, and the container drew nothing on desktop GL, WebGL2
  and Android. The Metal wrapper already runs the body in a nested block. The
  GLSL wrapper now does the same, so the same body compiles on every backend.

- **`ShaderHash` keys on both shader bodies (#573).** The custom-shader cache
  key hashed only the Metal body on macOS and only the GLSL body everywhere
  else. iOS reports `GOOS` `ios`, so its Metal backend keyed on the GLSL body:
  two shaders with the same GLSL and different Metal shared one compiled
  pipeline, and every Metal-only shader shared the key of an empty string. The
  key now hashes the Metal body, a separator and the GLSL body, so a change to
  either body gives a new key on every backend.

- **Apps can set where toasts stack again, with `ThemeCfg.ToastAnchor`
  (#1103).** #702 made the anchor settable (#698), but #735 made the toast style
  private, so toasts were always in the bottom-right corner. Set
  `cfg.ToastAnchor = gui.Some(gui.ToastTopLeft)` and build the theme with
  `ThemeMaker(cfg)`. The anchor is part of the `Cfg`, so `WithPadding`,
  `WithBorders` and `AdjustFontSize` keep it. Unset or out of range gives
  `ToastBottomRight`, the old default. The showcase Toast page has an Anchor
  choice that moves the toasts to each corner. Reported in #1101 by @bahyqn.

- **A mouse lock now ends only on the release of the button that took it
  (#1099).** The lock's `MouseUp` got every release. A right or middle click
  during a left drag ended the drag while left was still down: a slider,
  scrollbar, splitter, dock or reorder drag, a text selection, a color drag or a
  data grid column resize stopped, and a pointer move with left still down did
  nothing. `MouseLock` now records the button of the press that took it. While
  that button is down, another button's release does not reach the lock's
  `MouseUp`, and neither its press nor its release reaches the widgets under the
  drag, even when the release comes after the drag ends. So it cannot fire an
  `OnClick` or close a popup. A lock with a `MouseDown` callback still gets
  every press. A lock taken with no button down, or one whose button is already
  up, gets every release as before, so it can still end. Code that calls
  `MouseLock` needs no change, and the button check in each `MouseUp` is no
  longer needed.

- **A second mouse button no longer ends a left press (#1098).** The window kept
  one held button. A right press during a left drag replaced it, and the right
  release cleared it and the press target while left was still down. So
  `IsPressed` went false and `EventCtx.MouseButton` read no button in the middle
  of the drag. The window now keeps the set of held buttons. A release clears
  only its own button, and only a left release ends the press target. While more
  than one button is down, `EventCtx.MouseButton` reports the lowest one, so
  left wins. On Windows, mouse capture now lasts until the last button is
  released, so a left release outside the window still arrives after a right
  click. Back, forward and other extra buttons are now ignored. On X11 they
  arrived as a left press with no release, so the window kept left held. On
  macOS they arrived as middle, so their release ended a real middle press. On
  every platform a press of one could take focus or fire an `OnClick` that
  accepts any button.

- **A table built from `RawData` styles its header with `TextStyleHead`
  (#1097).** The header row of a `RawData`, `TableCfgFromData` or CSV table got
  the default text style in bold, which won over the table's `TextStyleHead`. So
  neither the theme's header style nor an app's own reached these tables, while
  a table built from `TableCellCfg` cells used it. The header now takes
  `TextStyleHead` like any header cell without a style of its own. Each header
  cell also stops allocating a style on every frame.

- **The `InputDate` calendar button uses the icon font.** The button drew the 📅
  color emoji, the only color emoji among the control icons. It ignored the
  theme's text color and looked out of place on both light and dark themes. It
  now draws the icon font's calendar glyph in the field's text size and color,
  centred on its own ink like the other control icons.

- **The slider thumb is visible on light themes.** The thumb is a disc in the
  panel color, and its edge used the hairline border, so on a light page the
  value's position showed only as the end of the colored fill. The thumb and
  track now take the same 3:1 edge as an off checkbox. Hover used to repaint the
  track edge in the hover fill color, which faded it out under the pointer; it
  now makes the edge stronger.

- **Unchecked checkboxes, radios and switches are visible on every theme.** An
  off control drew its edge in the shared hairline border color, which measured
  1.1 to 1.4 against the page on the light presets, so an unchecked box nearly
  disappeared. The off switch's thumb was fainter still. These controls now take
  an edge color that reaches 3:1 on the page and on a panel, the WCAG 1.4.11
  floor for the parts of a control that show where it is. The switch's off thumb
  takes the same color. A radio under the pointer or pressed no longer fades its
  ring: the ring gets stronger instead. Other borders keep the hairline.

- **List box and select subheadings read as group labels.** A subheading used
  the same text style as the rows under it, so nothing set it apart, and the
  rule under it was drawn in the full text color: a heavy line across the list.
  The default subheading now takes the secondary text role, as the style guide
  records for list subheadings, and the rule takes the theme's separator color.
  `ListBoxCfg.SubheadingStyle` and `SelectCfg.SubheadingStyle` set by the app
  still win; the rule no longer follows their color.

- **Data grid filter text lines up with the column's values.** The filter input
  had no side inset, so its placeholder and typed text sat on the left grid
  line, 5px left of the values below it. The input now takes the cells' side
  inset. It still fills the cell, so its fill meets the grid lines.

- **Data grid cells draw clean grid lines.** Header, filter and data cells left
  their corner radius unset, so each took the theme's control radius. The cell
  borders overlap into shared grid lines, and the rounded corners cut a notch at
  every intersection: the grid looked like a sheet of separate boxes. Cells are
  now square. The grid's outer frame still takes `DataGridCfg.Radius`.

- **Markdown syntax colors are readable on every theme.** Keywords, strings,
  numbers, types and function names used fixed mid-tone colors that measured 2.5
  to 4.4 against the code fill, below the 4.5:1 that body text needs, on both
  light and dark presets. Each color now keeps its hue and moves on lightness
  until it reads at 4.5:1 over the code fill. The colors are derived once per
  theme, so `DefaultMarkdownStyle` costs no more per frame than before.

- **Markdown code on light themes is a light wash, not a gray slab.** Fenced
  blocks and inline code painted one black fill at 20% on every theme. On a
  light page that gave a mid-gray block (contrast 1.58 against the page), and on
  a dark page it was close to invisible (1.05). The fill now follows the theme's
  polarity: a faint dark wash on light themes and a faint light wash on dark
  themes, both near 1.1 against the page. A `MarkdownStyle.CodeBlockBG` set by
  the app is unchanged.

- **The Markdown copy button no longer covers the first line of text.** The
  document copy button floats over the top-right corner. A first paragraph or
  code block that used the full width ran under the icon, so a word was hidden.
  The first block now stops short of the button. Later blocks keep the full
  width.

- **Select keyboard scroll tracks the measured row height.** The scroll offset
  for keyboard navigation estimated the dropdown row from the nominal text size,
  while the laid-out row uses the measured font height, so under a custom text
  size the highlighted option could stop half-visible. The estimate now uses the
  text measurer with the nominal size as fallback.

- **Input drag-scroll stays inside the viewport under RTL (#1091).** Under
  right-to-left layout the viewport starts after `Padding.Right`, but the
  drag-scroll band at the edges used `Padding.Left`, so dragging near an edge
  scrolled against the wrong band. Both edges now use the mirrored origin and
  include `SizeBorder` like the content box. LTR layouts are unchanged.

- **The inspector padding wireframe mirrors under RTL (#1090).** The wireframe
  used `Padding.Left` as its origin in both directions, so under RTL it drew
  offset from the content box it annotates. It now uses `Padding.Right` under
  RTL and includes `SizeBorder` in the inset. LTR layouts are unchanged.

- **The vertical scrollbar moves to the left under RTL (#1080).** In a
  right-to-left layout, content in a scrollable column used to run under its
  vertical scrollbar. RTL mirrors padding, so the `Padding.Right` a container
  keeps free for the bar ends up on the left, but the bar stayed on the right.
  The bar now sits on the left edge under RTL, inset by `GapEdge`, as in other
  right-to-left UIs. The horizontal bar starts after that left gutter, so in a
  container that scrolls both ways, such as the inspector or `DataGrid`, the two
  bars no longer overlap in the corner. A press in the horizontal bar's gutter
  now maps against the viewport's true start. Containers that keep room for the
  bar with `Padding.Right` need no change: the inspector, `Table`, and the
  showcase's catalog and page panels now clear the bar in Arabic. LTR layouts
  are unchanged.

## Older releases

All published releases are in [CHANGELOG-archive.md](CHANGELOG-archive.md).
