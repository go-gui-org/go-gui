package main

import (
	"slices"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

// localeSwitchID is the leaf ID of the Locale page's locale switch.
const localeSwitchID = "showcase-locale-switch"

// localeNumDecimals is the fraction digits of the Locale page's
// numeric input. setShowcaseLocale formats the input text with it.
const localeNumDecimals = 2

// localeLabelWidth lines up the value column of the Locale page.
const localeLabelWidth = 180

// localeTranslationKeys are the app strings the Locale page looks up
// with LocaleT. Each key is its own English text: LocaleT gives back
// the key when the locale has no entry, so en-US needs no
// translations block. The de-DE, ar-SA and ja-JP files in locales/
// translate each one.
var localeTranslationKeys = []string{
	"Hello",
	"Cart",
	"Checkout",
	"Thank you for your order",
}

// localeSampleDate is the date the Locale page formats. A fixed date
// keeps the page the same from run to run.
var localeSampleDate = time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC)

// demoLocale shows what the active locale changes. Every value on the
// page is read from gui.CurrentLocale() or formatted with it, so a
// switch here or on the catalog's locale button changes the whole page.
// The page's own labels stay English on purpose: they name the thing
// each row shows.
func demoLocale(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := appState(w)
	loc := gui.CurrentLocale()

	dir := "Left to right"
	if loc.TextDir == gui.TextDirRTL {
		dir = "Right to left"
	}

	translations := make([]gui.View, 0, len(localeTranslationKeys))
	for _, key := range localeTranslationKeys {
		translations = append(translations, localeRow(key, gui.LocaleT(key)))
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SpacingMedium,
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text: "The locale sets number and date formats, the " +
					"strings built into widgets and the text direction. " +
					"It does not translate an app's own text: the app " +
					"looks up its strings with LocaleT. Pick a locale " +
					"here or with the button under the catalog.",
				TextStyle: t.TextStyleBody,
				Mode:      gui.TextModeWrap,
			}),
			gui.SegmentedControl(gui.SegmentedControlCfg{
				ID:      localeSwitchID,
				A11YCfg: gui.A11YCfg{A11YLabel: "Locale"},
				Items:   showcaseLocaleIDs,
				// The active locale, not LocaleIndex: the switch must mark
				// the locale the rows below show. A locale set outside the
				// showcase list marks no segment.
				Value: loc.ID,
				OnSelect: func(id string, ctx gui.EventCtx) {
					// An ID not in the list gives -1, which
					// setShowcaseLocale ignores.
					setShowcaseLocale(ctx.Window, slices.Index(showcaseLocaleIDs, id))
					ctx.Consume()
				},
			}),

			sectionLabel(t, "Formats"),
			localeRow("Locale ID", loc.ID),
			localeRow("Text direction", dir),
			localeRow("Number", gui.LocaleFormatNumber(1234567.891, 2)),
			localeRow("Currency", gui.LocaleFormatCurrency(-1234.5)),
			localeRow("Short date", gui.LocaleFormatDate(localeSampleDate, loc.Date.ShortDate)),
			localeRow("Long date", gui.LocaleFormatDate(localeSampleDate, loc.Date.LongDate)),
			localeRow("Month and year", gui.LocaleFormatDate(localeSampleDate, loc.Date.MonthYear)),
			localeRow("First day of week", loc.WeekdaysFull[loc.Date.FirstDayOfWeek%7]),
			localeRow("Weekdays", strings.Join(loc.WeekdaysMed[:], " ")),
			localeRow("Months", strings.Join(loc.MonthsShort[:], " ")),

			sectionLabel(t, "Built-in strings"),
			localeRow("Dialog", strings.Join([]string{loc.StrOK, loc.StrCancel, loc.StrYes, loc.StrNo}, " · ")),
			localeRow("Actions", strings.Join([]string{loc.StrSave, loc.StrDelete, loc.StrSearch, loc.StrReset, loc.StrSubmit}, " · ")),
			localeRow("Data grid", strings.Join([]string{
				gui.LocaleRowsFmt(1, 50, 12500),
				gui.LocalePageFmt(1, 250),
				gui.LocaleMatchesFmt(1200, "?"),
				loc.StrColumns,
			}, " · ")),

			sectionLabel(t, "App translations"),
			gui.Text(gui.TextCfg{
				Text: "Keys looked up with LocaleT, from the translations " +
					"block of each locale file. A key with no entry " +
					"comes back as itself, so en-US needs no block.",
				TextStyle: t.TextStyleSecondary,
				Mode:      gui.TextModeWrap,
			}),
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FillFit,
				Spacing:    gui.SpacingMedium,
				Padding:    gui.NoPadding,
				SizeBorder: gui.NoBorder,
				Content:    translations,
			}),

			sectionLabel(t, "Locale-aware input"),
			gui.Text(gui.TextCfg{
				Text: "NumericInput does not read the locale by itself. " +
					"This one copies CurrentLocale().Number into its " +
					"Locale field, so it parses and formats with the " +
					"active separators.",
				TextStyle: t.TextStyleSecondary,
				Mode:      gui.TextModeWrap,
			}),
			gui.NumericInput(gui.NumericInputCfg{
				ID:       "showcase-locale-number",
				A11YCfg:  gui.A11YCfg{A11YLabel: "Number in the active locale"},
				Text:     app.LocaleNumText,
				Value:    app.LocaleNumValue,
				Decimals: localeNumDecimals,
				Locale: gui.NumericLocaleCfg{
					DecimalSep: loc.Number.DecimalSep,
					GroupSep:   loc.Number.GroupSep,
					GroupSizes: loc.Number.GroupSizes,
					MinusSign:  loc.Number.MinusSign,
					PlusSign:   loc.Number.PlusSign,
				},
				Width:  220,
				Sizing: gui.FixedFit,
				OnTextChanged: func(text string, ctx gui.EventCtx) {
					appState(ctx.Window).LocaleNumText = text
				},
				OnValueCommit: func(value gui.Opt[float64], text string, ctx gui.EventCtx) {
					a := appState(ctx.Window)
					a.LocaleNumValue = value
					a.LocaleNumText = text
				},
			}),
		},
	})
}

// localeRow is one label and value line of the Locale page. The label
// column has a fixed width so the values line up.
func localeRow(label, value string) gui.View {
	t := gui.CurrentTheme()
	return gui.Row(gui.ContainerCfg{
		Sizing:     gui.FillFit,
		Spacing:    gui.SpacingMedium,
		Padding:    gui.NoPadding,
		SizeBorder: gui.NoBorder,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: t.TextStyleSecondary,
				Sizing:    gui.FixedFit,
				MinWidth:  localeLabelWidth,
			}),
			gui.Text(gui.TextCfg{
				Text:      value,
				TextStyle: t.TextStyleBody,
				Mode:      gui.TextModeWrap,
			}),
		},
	})
}
