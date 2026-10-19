# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
