package gui

import (
	"encoding/json"
	"os"
	"strings"
	"unicode/utf8"
)

// JSON-friendly intermediate structs for locale bundle decoding.
// String types used where Locale uses rune/enum so json.Unmarshal
// works directly.

type numberBundle struct {
	DecimalSep string `json:"decimal_sep"`
	GroupSep   string `json:"group_sep"`
	MinusSign  string `json:"minus_sign"`
	PlusSign   string `json:"plus_sign"`
	GroupSizes []int  `json:"group_sizes"`
}

type dateBundle struct {
	FirstDayOfWeek *int   `json:"first_day_of_week"`
	Use24H         *bool  `json:"use_24h"`
	ShortDate      string `json:"short_date"`
	LongDate       string `json:"long_date"`
	MonthYear      string `json:"month_year"`
}

type currencyBundle struct {
	Spacing  *bool  `json:"spacing"`
	Decimals *int   `json:"decimals"`
	Symbol   string `json:"symbol"`
	Code     string `json:"code"`
	Position string `json:"position"`
}

type localeBundle struct {
	Number        *numberBundle     `json:"number"`
	Date          *dateBundle       `json:"date"`
	Currency      *currencyBundle   `json:"currency"`
	Strings       map[string]string `json:"strings"`
	Translations  map[string]string `json:"translations"`
	ID            string            `json:"id"`
	TextDir       string            `json:"text_dir"`
	WeekdaysShort []string          `json:"weekdays_short"`
	WeekdaysMed   []string          `json:"weekdays_med"`
	WeekdaysFull  []string          `json:"weekdays_full"`
	MonthsShort   []string          `json:"months_short"`
	MonthsFull    []string          `json:"months_full"`
}

// LocaleParse decodes a JSON string into a Locale struct.
// Missing keys fall back to en-US defaults.
func LocaleParse(content string) (Locale, error) {
	var b localeBundle
	if err := json.Unmarshal([]byte(content), &b); err != nil {
		return Locale{}, err
	}
	return b.toLocale(), nil
}

// LocaleLoad reads a JSON bundle file and returns a Locale.
func LocaleLoad(path string) (Locale, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Locale{}, err
	}
	var b localeBundle
	if err = json.Unmarshal(data, &b); err != nil {
		return Locale{}, err
	}
	return b.toLocale(), nil
}

func (b *localeBundle) toLocale() Locale {
	d := LocaleEnUS
	return Locale{
		ID:       strOr(b.ID, d.ID),
		TextDir:  parseTextDir(b.TextDir),
		Number:   b.toNumberFormat(d.Number),
		Date:     b.toDateFormat(d.Date),
		Currency: b.toCurrencyFormat(d.Currency),

		StrOK:     bundleStr(b.Strings, "ok", d.StrOK),
		StrYes:    bundleStr(b.Strings, "yes", d.StrYes),
		StrNo:     bundleStr(b.Strings, "no", d.StrNo),
		StrCancel: bundleStr(b.Strings, "cancel", d.StrCancel),

		StrSave:   bundleStr(b.Strings, "save", d.StrSave),
		StrDelete: bundleStr(b.Strings, "delete", d.StrDelete),
		StrAdd:    bundleStr(b.Strings, "add", d.StrAdd),
		StrClear:  bundleStr(b.Strings, "clear", d.StrClear),
		StrSearch: bundleStr(b.Strings, "search", d.StrSearch),
		StrFilter: bundleStr(b.Strings, "filter", d.StrFilter),
		StrJump:   bundleStr(b.Strings, "jump", d.StrJump),
		StrReset:  bundleStr(b.Strings, "reset", d.StrReset),
		StrSubmit: bundleStr(b.Strings, "submit", d.StrSubmit),

		StrLoading:        bundleStr(b.Strings, "loading", d.StrLoading),
		StrLoadingDiagram: bundleStr(b.Strings, "loading_diagram", d.StrLoadingDiagram),
		StrSaving:         bundleStr(b.Strings, "saving", d.StrSaving),
		StrSaveFailed:     bundleStr(b.Strings, "save_failed", d.StrSaveFailed),
		StrSourceChanged:  bundleStr(b.Strings, "source_changed", d.StrSourceChanged),
		StrLoadError:      bundleStr(b.Strings, "load_error", d.StrLoadError),
		StrError:          bundleStr(b.Strings, "error", d.StrError),
		StrClean:          bundleStr(b.Strings, "clean", d.StrClean),

		StrOpenLink:   bundleStr(b.Strings, "open_link", d.StrOpenLink),
		StrGoToTarget: bundleStr(b.Strings, "go_to_target", d.StrGoToTarget),
		StrCopyLink:   bundleStr(b.Strings, "copy_link", d.StrCopyLink),
		StrCopied:     bundleStr(b.Strings, "copied", d.StrCopied),

		StrHorizontalScrollbar: bundleStr(b.Strings, "horizontal_scrollbar", d.StrHorizontalScrollbar),
		StrVerticalScrollbar:   bundleStr(b.Strings, "vertical_scrollbar", d.StrVerticalScrollbar),

		StrColumns:  bundleStr(b.Strings, "columns", d.StrColumns),
		StrSelected: bundleStr(b.Strings, "selected", d.StrSelected),
		StrDraft:    bundleStr(b.Strings, "draft", d.StrDraft),
		StrDirty:    bundleStr(b.Strings, "dirty", d.StrDirty),
		StrMatches:  bundleStr(b.Strings, "matches", d.StrMatches),
		StrPage:     bundleStr(b.Strings, "page", d.StrPage),
		StrRows:     bundleStr(b.Strings, "rows", d.StrRows),

		StrRed:   bundleStr(b.Strings, "red", d.StrRed),
		StrGreen: bundleStr(b.Strings, "green", d.StrGreen),
		StrBlue:  bundleStr(b.Strings, "blue", d.StrBlue),
		StrAlpha: bundleStr(b.Strings, "alpha", d.StrAlpha),
		StrHue:   bundleStr(b.Strings, "hue", d.StrHue),
		StrSat:   bundleStr(b.Strings, "sat", d.StrSat),
		StrValue: bundleStr(b.Strings, "value", d.StrValue),

		WeekdaysShort: toFixed7(b.WeekdaysShort, d.WeekdaysShort),
		WeekdaysMed:   toFixed7(b.WeekdaysMed, d.WeekdaysMed),
		WeekdaysFull:  toFixed7(b.WeekdaysFull, d.WeekdaysFull),
		MonthsShort:   toFixed12(b.MonthsShort, d.MonthsShort),
		MonthsFull:    toFixed12(b.MonthsFull, d.MonthsFull),

		Translations: b.Translations,
	}
}

func (b *localeBundle) toNumberFormat(d NumberFormat) NumberFormat {
	nb := b.Number
	if nb == nil {
		return d
	}
	return NumberFormat{
		DecimalSep: firstRune(nb.DecimalSep, d.DecimalSep),
		GroupSep:   firstRune(nb.GroupSep, d.GroupSep),
		GroupSizes: nonEmptyInts(nb.GroupSizes, d.GroupSizes),
		MinusSign:  firstRune(nb.MinusSign, d.MinusSign),
		PlusSign:   firstRune(nb.PlusSign, d.PlusSign),
	}
}

func (b *localeBundle) toDateFormat(d DateFormat) DateFormat {
	db := b.Date
	if db == nil {
		return d
	}
	out := d
	if db.ShortDate != "" {
		out.ShortDate = db.ShortDate
	}
	if db.LongDate != "" {
		out.LongDate = db.LongDate
	}
	if db.MonthYear != "" {
		out.MonthYear = db.MonthYear
	}
	if db.FirstDayOfWeek != nil {
		out.FirstDayOfWeek = uint8(*db.FirstDayOfWeek)
	}
	if db.Use24H != nil {
		out.Use24H = *db.Use24H
	}
	return out
}

func (b *localeBundle) toCurrencyFormat(d CurrencyFormat) CurrencyFormat {
	cb := b.Currency
	if cb == nil {
		return d
	}
	out := d
	if cb.Symbol != "" {
		out.Symbol = cb.Symbol
	}
	if cb.Code != "" {
		out.Code = cb.Code
	}
	out.Position = parseAffixPosition(cb.Position, d.Position)
	if cb.Spacing != nil {
		out.Spacing = *cb.Spacing
	}
	if cb.Decimals != nil {
		out.Decimals = *cb.Decimals
	}
	return out
}

func bundleStr(m map[string]string, key, fallback string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return fallback
}

func strOr(s, fallback string) string {
	if s != "" {
		return s
	}
	return fallback
}

func nonEmptyInts(src, fallback []int) []int {
	if len(src) > 0 {
		return src
	}
	return fallback
}

func toFixed7(src []string, fallback [7]string) [7]string {
	if len(src) != 7 {
		return fallback
	}
	var out [7]string
	copy(out[:], src)
	return out
}

func toFixed12(src []string, fallback [12]string) [12]string {
	if len(src) != 12 {
		return fallback
	}
	var out [12]string
	copy(out[:], src)
	return out
}

func parseTextDir(s string) TextDirection {
	switch strings.ToLower(s) {
	case "ltr":
		return TextDirLTR
	case "rtl":
		return TextDirRTL
	default:
		return TextDirAuto
	}
}

func parseAffixPosition(s string, fallback NumericAffixPosition) NumericAffixPosition {
	switch strings.ToLower(s) {
	case "prefix":
		return AffixPrefix
	case "suffix":
		return AffixSuffix
	default:
		return fallback
	}
}

// firstRune decodes the first UTF-8 codepoint from s.
// Returns fallback when s is empty.
func firstRune(s string, fallback rune) rune {
	if s == "" {
		return fallback
	}
	r, _ := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError {
		return fallback
	}
	return r
}
