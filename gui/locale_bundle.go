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
	// exportaudit:keep — json-serialized field (stdlib reflection)
	GroupSep string `json:"group_sep"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	// exportaudit:keep — json-tagged or same-named member
	// exportaudit:keep — json-tagged or same-named member
	// exportaudit:keep — json-tagged or same-named member
	MinusSign string `json:"minus_sign"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	// exportaudit:keep — json-tagged or same-named member
	PlusSign string `json:"plus_sign"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	// exportaudit:keep — json-tagged or same-named member
	GroupSizes []int `json:"group_sizes"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
}

type dateBundle struct {
	FirstDayOfWeek *int `json:"first_day_of_week"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	Use24H *bool `json:"use_24h"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	ShortDate string `json:"short_date"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	LongDate string `json:"long_date"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	MonthYear string `json:"month_year"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
}

type currencyBundle struct {
	Spacing *bool `json:"spacing"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	Decimals *int `json:"decimals"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	Symbol string `json:"symbol"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	// exportaudit:keep — json-tagged or same-named member
	Code string `json:"code"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	Position string `json:"position"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
}

type localeBundle struct {
	Number *numberBundle `json:"number"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	Date *dateBundle `json:"date"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	// exportaudit:keep — json-tagged or same-named member
	// exportaudit:keep — json-tagged or same-named member
	// exportaudit:keep — json-tagged or same-named member
	Currency *currencyBundle `json:"currency"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	Strings map[string]string `json:"strings"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	Translations map[string]string `json:"translations"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	ID string `json:"id"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	TextDir string `json:"text_dir"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	WeekdaysShort []string `json:"weekdays_short"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	WeekdaysMed []string `json:"weekdays_med"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	WeekdaysFull []string `json:"weekdays_full"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	MonthsShort []string `json:"months_short"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
	MonthsFull []string `json:"months_full"`
	// exportaudit:keep — json-serialized field (stdlib reflection)
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
//
// #nosec G304 — path from filepath.Glob constrained to *.json
// exportaudit:keep — documented public API (showcase docs)
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
	d := localeEnUS
	return Locale{
		ID:       strOr(b.ID, d.ID),
		TextDir:  parseTextDir(b.TextDir),
		Number:   b.toNumberFormat(d.Number),
		Date:     b.toDateFormat(d.Date),
		Currency: b.toCurrencyFormat(d.Currency),

		strOK:     bundleStr(b.Strings, "ok", d.strOK),
		strYes:    bundleStr(b.Strings, "yes", d.strYes),
		strNo:     bundleStr(b.Strings, "no", d.strNo),
		StrCancel: bundleStr(b.Strings, "cancel", d.StrCancel),

		StrSave:   bundleStr(b.Strings, "save", d.StrSave),
		StrDelete: bundleStr(b.Strings, "delete", d.StrDelete),
		StrAdd:    bundleStr(b.Strings, "add", d.StrAdd),
		StrClear:  bundleStr(b.Strings, "clear", d.StrClear),
		strSearch: bundleStr(b.Strings, "search", d.strSearch),
		StrFilter: bundleStr(b.Strings, "filter", d.StrFilter),
		StrJump:   bundleStr(b.Strings, "jump", d.StrJump),
		StrReset:  bundleStr(b.Strings, "reset", d.StrReset),
		StrSubmit: bundleStr(b.Strings, "submit", d.StrSubmit),

		StrLoading:        bundleStr(b.Strings, "loading", d.StrLoading),
		strLoadingDiagram: bundleStr(b.Strings, "loading_diagram", d.strLoadingDiagram),
		StrSaving:         bundleStr(b.Strings, "saving", d.StrSaving),
		StrSaveFailed:     bundleStr(b.Strings, "save_failed", d.StrSaveFailed),
		StrSourceChanged:  bundleStr(b.Strings, "source_changed", d.StrSourceChanged),
		StrLoadError:      bundleStr(b.Strings, "load_error", d.StrLoadError),
		StrError:          bundleStr(b.Strings, "error", d.StrError),
		StrClean:          bundleStr(b.Strings, "clean", d.StrClean),

		strOpenLink:   bundleStr(b.Strings, "open_link", d.strOpenLink),
		strGoToTarget: bundleStr(b.Strings, "go_to_target", d.strGoToTarget),
		strCopyLink:   bundleStr(b.Strings, "copy_link", d.strCopyLink),
		strCopied:     bundleStr(b.Strings, "copied", d.strCopied),

		strHorizontalScrollbar: bundleStr(b.Strings, "horizontal_scrollbar", d.strHorizontalScrollbar),
		strVerticalScrollbar:   bundleStr(b.Strings, "vertical_scrollbar", d.strVerticalScrollbar),

		StrColumns:  bundleStr(b.Strings, "columns", d.StrColumns),
		StrSelected: bundleStr(b.Strings, "selected", d.StrSelected),
		StrDraft:    bundleStr(b.Strings, "draft", d.StrDraft),
		StrDirty:    bundleStr(b.Strings, "dirty", d.StrDirty),
		StrMatches:  bundleStr(b.Strings, "matches", d.StrMatches),
		strPage:     bundleStr(b.Strings, "page", d.strPage),
		StrRows:     bundleStr(b.Strings, "rows", d.StrRows),

		strRed:   bundleStr(b.Strings, "red", d.strRed),
		strGreen: bundleStr(b.Strings, "green", d.strGreen),
		strBlue:  bundleStr(b.Strings, "blue", d.strBlue),
		strAlpha: bundleStr(b.Strings, "alpha", d.strAlpha),
		strHue:   bundleStr(b.Strings, "hue", d.strHue),
		strSat:   bundleStr(b.Strings, "sat", d.strSat),
		strValue: bundleStr(b.Strings, "value", d.strValue),

		WeekdaysShort: toFixed7(b.WeekdaysShort, d.WeekdaysShort),
		WeekdaysMed:   toFixed7(b.WeekdaysMed, d.WeekdaysMed),
		WeekdaysFull:  toFixed7(b.WeekdaysFull, d.WeekdaysFull),
		MonthsShort:   toFixed12(b.MonthsShort, d.MonthsShort),
		MonthsFull:    toFixed12(b.MonthsFull, d.MonthsFull),

		Translations: b.Translations,
	}
}

func (b *localeBundle) toNumberFormat(d numberFormat) numberFormat {
	nb := b.Number
	if nb == nil {
		return d
	}
	return numberFormat{
		DecimalSep: firstRune(nb.DecimalSep, d.DecimalSep),
		GroupSep:   firstRune(nb.GroupSep, d.GroupSep),
		GroupSizes: nonEmptyInts(nb.GroupSizes, d.GroupSizes),
		MinusSign:  firstRune(nb.MinusSign, d.MinusSign),
		PlusSign:   firstRune(nb.PlusSign, d.PlusSign),
	}
}

func (b *localeBundle) toDateFormat(d dateFormat) dateFormat {
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

func (b *localeBundle) toCurrencyFormat(d currencyFormat) currencyFormat {
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

func parseTextDir(s string) textDirection {
	switch strings.ToLower(s) {
	case "ltr":
		return TextDirLTR
	case "rtl":
		return TextDirRTL
	default:
		return textDirAuto
	}
}

func parseAffixPosition(s string, fallback numericAffixPosition) numericAffixPosition {
	switch strings.ToLower(s) {
	case "prefix":
		return affixPrefix
	case "suffix":
		return affixSuffix
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
