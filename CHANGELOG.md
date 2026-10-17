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

### Fixed

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
