# Spec: per-field date format for InputDate

Status: **implemented** — issue #578. Landed on `main` with the tests, the
golden case, the showcase demo and the `CHANGELOG.md` entry.

## Problem

`InputDate` read its date format from one process-global value,
`ActiveLocale.Date.ShortDate` (`gui/locale.go`), which is `"M/D/YYYY"` for
en-US. The format drives four things in the field: the text it shows, the input
mask, the placeholder hint, and the parse of what the user types.

A caller who wanted `24.12.2026` had two bad options. Change the locale, which
changes every date widget in the process. Or write to `Locale.Date.ShortDate`
through `CurrentLocale()` and `SetLocale`, an undocumented round trip through an
unexported struct type. Neither lets two fields in one form differ.

## Candidates

| Design                                       | Exported surface         | Failure when misused                                   |
| -------------------------------------------- | ------------------------ | ------------------------------------------------------ |
| **A. `InputDateCfg.DateFormat string`**      | 1 struct field           | Bad token string, caught by a panic at construction    |
| B. App-wide `SetDateFormat(string)`          | 1 func plus a new global | Silent: two libraries write the same global            |
| C. Named constants (`DateFormatDMYDot`, ...) | Type plus N constants    | Closed set: a format not in the list cannot be spelled |

## Decision: A

The smallest surface. It reuses the formatter that was already there
(`LocaleFormatDate`, `localeDatePadFormat`, `localeDateMaskPattern`,
`localeParseDate`, all in `gui/locale_format.go`), so there is no second format
path to keep in step. Two date fields in one form can differ, which a global
cannot express. The locale stays the default, so an app that wants an app-wide
change still sets the locale.

## Shape

`InputDateCfg.DateFormat` takes the token language the locale bundles already
use: `YYYY`, `MM`, `M`, `DD`, `D` and literal separators.

`inputDateFormat` (`gui/view_input_date.go`) is the one resolve point. It pads
`M` to `MM` and `D` to `DD`, and falls back to the locale when the field is
empty. `GenerateLayout` calls it once per frame and threads the result to the
display text, the mask, the placeholder and the parse. Re-deriving the format at
each site is how the four drift apart, so none of them reads the locale directly
any more.

`requireDateFormat` panics from the `InputDate` factory on a format the field
cannot honour: a month-name token (`MMM`, `MMMM`), a 2-digit year (`YY` without
`YYYY`), a time token (`HH`, `mm`, `ss` — the display path substitutes them but
`localeParseDate` leaves them as literals, so a commit can never round-trip), or
a format with no `YYYY`, `MM` or `DD`. This matches the `RequireID` precedent in
`gui/state_registry.go`.

The `opticalDigitCenter: true` assumption stays true because of that check.
`docs/specs/text-optical-centring.md` records that the date field centres on its
ink because the mask admits digits and separators only.

## Rejected Approaches

- **An app-wide `SetDateFormat`.** The locale layer already is the app-wide
  seam: `(*Window).SetLocale`, `SetLocaleID`, and the `gui/locales/*.json`
  bundles, which carry `de-DE` as `D.M.YYYY` today. A second global would be a
  parallel source of truth for the same value.
- **Named format constants.** The token language is open. An enum closes it, and
  every new order or separator then needs a release.
- **Go reference layouts (`"02.01.2006"`).** This would disagree with the
  `short_date` spelling in the locale bundles and need a second pad and mask
  path for the same result.
- **The same field on `DatePickerCfg` or the roller.** The picker header shows a
  month and a year (`ActiveLocale.Date.MonthYear`), not a short date. The roller
  already sets its order with `DisplayMode` and `LongMonths`. Neither is what
  the issue asks for.
- **A silent fallback to the locale on a bad format.** A format the mask cannot
  express gives a field that looks correct and refuses every keystroke. The
  failure must be at construction, where the cause is visible.

## Tests

`gui/view_input_date_test.go` pins the unset fallback, the display text, the
placeholder, the mask, and both panics. `TestInputDateFormatParsesCommit` drives
an Enter commit through the inner `Input` and asserts that `"24.12.2026"`
reaches `OnSelect` as 24 December: under the locale's `MM/DD/YYYY` the same text
fails to parse and `OnSelect` never fires.

`gui/locale_format_test.go` gains `TestLocaleParseDate`. That function is the
only parse in the date path and had no test before this change.

The golden case `input_date_format` pairs with `input_date`. Only the date text
differs between the two recordings; no geometry moves.
