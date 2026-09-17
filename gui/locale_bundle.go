package gui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"unicode/utf8"
)

// maxLocaleBundleBytes caps the JSON a locale bundle may carry.
// Bundles are small static files; the cap stops a corrupt or
// hostile file from exhausting memory on load.
const maxLocaleBundleBytes = 1 << 20

// JSON-friendly intermediate structs for locale bundle decoding.
// String types used where Locale uses rune/enum so JSON decoding
// works directly. Every field carries a json tag; decoding
// rejects unknown fields so a typo fails loudly instead of
// silently falling back to en-US.

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

// decodeBundle unmarshals data with unknown fields rejected.
func decodeBundle(data []byte) (localeBundle, error) {
	var b localeBundle
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		return localeBundle{}, err
	}
	return b, nil
}

// LocaleParse decodes a JSON string into a Locale struct.
// Missing keys fall back to en-US defaults; malformed values
// (unknown fields, out-of-range numbers, wrong-length name
// lists) return an error.
func LocaleParse(content string) (Locale, error) {
	if len(content) > maxLocaleBundleBytes {
		return Locale{}, fmt.Errorf("LocaleParse: bundle exceeds %d bytes",
			maxLocaleBundleBytes)
	}
	b, err := decodeBundle([]byte(content))
	if err != nil {
		return Locale{}, err
	}
	return b.toLocale()
}

// LocaleLoad reads a JSON bundle file and returns a Locale.
//
// #nosec G304 — the path is the caller's explicit API input, not
// request-derived; callers constrain it (see LocaleLoadDir).
// exportaudit:keep — documented public API (showcase docs)
func LocaleLoad(path string) (Locale, error) {
	f, err := os.Open(path)
	if err != nil {
		return Locale{}, err
	}
	// Read-only handle: a Close error cannot lose data.
	defer func() { _ = f.Close() }()
	// Read at most one byte past the cap: enough to detect an
	// oversize file without loading all of it into memory.
	data, err := io.ReadAll(io.LimitReader(f, maxLocaleBundleBytes+1))
	if err != nil {
		return Locale{}, err
	}
	if len(data) > maxLocaleBundleBytes {
		return Locale{}, fmt.Errorf("LocaleLoad: %s exceeds %d bytes",
			path, maxLocaleBundleBytes)
	}
	b, err := decodeBundle(data)
	if err != nil {
		return Locale{}, err
	}
	return b.toLocale()
}

func (b *localeBundle) toLocale() (Locale, error) {
	d := LocaleEnUS
	number, err := b.toNumberFormat(d.Number)
	if err != nil {
		return Locale{}, fmt.Errorf("number: %w", err)
	}
	date, err := b.toDateFormat(d.Date)
	if err != nil {
		return Locale{}, fmt.Errorf("date: %w", err)
	}
	currency, err := b.toCurrencyFormat(d.Currency)
	if err != nil {
		return Locale{}, fmt.Errorf("currency: %w", err)
	}
	weekdaysShort, err := toFixed7(b.WeekdaysShort, d.WeekdaysShort,
		"weekdays_short")
	if err != nil {
		return Locale{}, err
	}
	weekdaysMed, err := toFixed7(b.WeekdaysMed, d.WeekdaysMed,
		"weekdays_med")
	if err != nil {
		return Locale{}, err
	}
	weekdaysFull, err := toFixed7(b.WeekdaysFull, d.WeekdaysFull,
		"weekdays_full")
	if err != nil {
		return Locale{}, err
	}
	monthsShort, err := toFixed12(b.MonthsShort, d.MonthsShort,
		"months_short")
	if err != nil {
		return Locale{}, err
	}
	monthsFull, err := toFixed12(b.MonthsFull, d.MonthsFull,
		"months_full")
	if err != nil {
		return Locale{}, err
	}
	textDir, err := parseTextDir(b.TextDir, d.TextDir)
	if err != nil {
		return Locale{}, err
	}
	return Locale{
		ID:       strOr(b.ID, d.ID),
		TextDir:  textDir,
		Number:   number,
		Date:     date,
		Currency: currency,

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
		StrLightness: bundleStr(
			b.Strings, "lightness", d.StrLightness),

		WeekdaysShort: weekdaysShort,
		WeekdaysMed:   weekdaysMed,
		WeekdaysFull:  weekdaysFull,
		MonthsShort:   monthsShort,
		MonthsFull:    monthsFull,

		// b is decoded fresh per call, so its map is not shared.
		Translations: b.Translations,
	}, nil
}

func (b *localeBundle) toNumberFormat(d NumberFormat) (NumberFormat, error) {
	nb := b.Number
	if nb == nil {
		// Clone the sizes: d comes from the LocaleEnUS global.
		d.GroupSizes = slices.Clone(d.GroupSizes)
		return d, nil
	}
	decimalSep, err := firstRune(nb.DecimalSep, d.DecimalSep,
		"decimal_sep")
	if err != nil {
		return NumberFormat{}, err
	}
	groupSep, err := firstRune(nb.GroupSep, d.GroupSep, "group_sep")
	if err != nil {
		return NumberFormat{}, err
	}
	minusSign, err := firstRune(nb.MinusSign, d.MinusSign, "minus_sign")
	if err != nil {
		return NumberFormat{}, err
	}
	plusSign, err := firstRune(nb.PlusSign, d.PlusSign, "plus_sign")
	if err != nil {
		return NumberFormat{}, err
	}
	var sizes []int
	if len(nb.GroupSizes) == 0 {
		sizes = slices.Clone(d.GroupSizes)
	} else {
		for _, size := range nb.GroupSizes {
			if size <= 0 {
				return NumberFormat{}, fmt.Errorf(
					"group_sizes: size %d must be positive", size)
			}
		}
		sizes = slices.Clone(nb.GroupSizes)
	}
	return NumberFormat{
		DecimalSep: decimalSep,
		GroupSep:   groupSep,
		GroupSizes: sizes,
		MinusSign:  minusSign,
		PlusSign:   plusSign,
	}, nil
}

func (b *localeBundle) toDateFormat(d DateFormat) (DateFormat, error) {
	db := b.Date
	if db == nil {
		return d, nil
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
		if *db.FirstDayOfWeek < 0 || *db.FirstDayOfWeek > 6 {
			return DateFormat{}, fmt.Errorf(
				"first_day_of_week: %d out of range 0-6",
				*db.FirstDayOfWeek)
		}
		out.FirstDayOfWeek = uint8(*db.FirstDayOfWeek)
	}
	if db.Use24H != nil {
		out.Use24H = *db.Use24H
	}
	return out, nil
}

func (b *localeBundle) toCurrencyFormat(
	d CurrencyFormat,
) (CurrencyFormat, error) {
	cb := b.Currency
	if cb == nil {
		return d, nil
	}
	out := d
	if cb.Symbol != "" {
		out.Symbol = cb.Symbol
	}
	if cb.Code != "" {
		out.Code = cb.Code
	}
	position, err := parseAffixPosition(cb.Position, d.Position)
	if err != nil {
		return CurrencyFormat{}, err
	}
	out.Position = position
	if cb.Spacing != nil {
		out.Spacing = *cb.Spacing
	}
	if cb.Decimals != nil {
		if *cb.Decimals < 0 || *cb.Decimals > 20 {
			return CurrencyFormat{}, fmt.Errorf(
				"decimals: %d out of range 0-20", *cb.Decimals)
		}
		out.Decimals = *cb.Decimals
	}
	return out, nil
}

// bundleStr looks up key, falling back when missing or empty.
// An explicit empty string cannot blank a UI label by accident.
func bundleStr(m map[string]string, key, fallback string) string {
	if v, ok := m[key]; ok && v != "" {
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

func toFixed7(
	src []string,
	fallback [7]string,
	key string,
) ([7]string, error) {
	if len(src) == 0 {
		return fallback, nil
	}
	if len(src) != 7 {
		return fallback, fmt.Errorf(
			"%s: want 7 entries, got %d", key, len(src))
	}
	var out [7]string
	copy(out[:], src)
	return out, nil
}

func toFixed12(
	src []string,
	fallback [12]string,
	key string,
) ([12]string, error) {
	if len(src) == 0 {
		return fallback, nil
	}
	if len(src) != 12 {
		return fallback, fmt.Errorf(
			"%s: want 12 entries, got %d", key, len(src))
	}
	var out [12]string
	copy(out[:], src)
	return out, nil
}

// parseTextDir maps "ltr"/"rtl" (any case). Empty falls back; any
// other value is an error so a typo cannot silently flip direction.
func parseTextDir(s string, fallback TextDirection) (TextDirection, error) {
	switch strings.ToLower(s) {
	case "ltr":
		return TextDirLTR, nil
	case "rtl":
		return TextDirRTL, nil
	case "":
		return fallback, nil
	default:
		return fallback, fmt.Errorf("text_dir: want ltr or rtl, got %q", s)
	}
}

// parseAffixPosition maps "prefix"/"suffix" (any case). Empty
// falls back; any other value is an error, as for parseTextDir.
func parseAffixPosition(
	s string,
	fallback NumericAffixPosition,
) (NumericAffixPosition, error) {
	switch strings.ToLower(s) {
	case "prefix":
		return AffixPrefix, nil
	case "suffix":
		return AffixSuffix, nil
	case "":
		return fallback, nil
	default:
		return fallback, fmt.Errorf(
			"position: want prefix or suffix, got %q", s)
	}
}

// firstRune decodes the single UTF-8 codepoint in s. Empty falls
// back; more than one codepoint is an error so a typo like "ab"
// cannot silently become "a".
func firstRune(s string, fallback rune, key string) (rune, error) {
	if s == "" {
		return fallback, nil
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError || size != len(s) {
		return fallback, fmt.Errorf(
			"%s: want a single character, got %q", key, s)
	}
	return r, nil
}
