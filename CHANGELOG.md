# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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

### Changed

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
