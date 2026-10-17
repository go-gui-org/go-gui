package gui

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLocaleFormatDate(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })

	tests := []struct {
		name   string
		locale Locale
		date   time.Time
		format string
		want   string
	}{
		{"short", LocaleEnUS,
			time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			"M/D/YYYY", "3/15/2025"},
		{"long_month", LocaleEnUS,
			time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC),
			"MMMM D, YYYY", "March 15, 2025"},
		{"short_month", LocaleEnUS,
			time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
			"MMM YYYY", "Dec 2025"},
		{"german", LocaleDeDE,
			time.Date(2025, 3, 5, 0, 0, 0, 0, time.UTC),
			"D. MMMM YYYY", "5. M\u00E4rz 2025"},
		{"datetime", LocaleEnUS,
			time.Date(2025, 1, 2, 14, 5, 9, 0, time.UTC),
			"YYYY-MM-DD HH:mm:ss", "2025-01-02 14:05:09"},
		{"year_padded", LocaleEnUS,
			time.Date(42, 1, 2, 0, 0, 0, 0, time.UTC),
			"YYYY-MM-DD", "0042-01-02"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLocale(tt.locale)
			got := LocaleFormatDate(tt.date, tt.format)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLocaleFmt(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	SetLocale(LocaleEnUS)

	t.Run("rows", func(t *testing.T) {
		got := LocaleRowsFmt(1, 50, 200)
		if got != "Rows 1-50/200" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("page", func(t *testing.T) {
		got := LocalePageFmt(3, 10)
		if got != "Page 3/10" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("matches", func(t *testing.T) {
		got := LocaleMatchesFmt(5, "100")
		if got != "Matches 5/100" {
			t.Fatalf("got %q", got)
		}
	})
}

func TestLocaleFmtGrouped(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	SetLocale(LocaleDeDE)

	if got := LocaleRowsFmt(1, 1234567, 2000000); got != "Zeilen 1-1.234.567/2.000.000" {
		t.Fatalf("got %q", got)
	}
	if got := LocalePageFmt(3, 10); got != "Seite 3/10" {
		t.Fatalf("got %q", got)
	}
	if got := LocaleMatchesFmt(1500, "?"); got != "Treffer 1.500/?" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatGroupedInt(t *testing.T) {
	en := LocaleEnUS
	if got := formatGroupedInt(1234567, en); got != "1,234,567" {
		t.Fatalf("got %q", got)
	}
	if got := formatGroupedInt(-1234567, en); got != "-1,234,567" {
		t.Fatalf("got %q", got)
	}
	if got := formatGroupedInt(999, en); got != "999" {
		t.Fatalf("got %q", got)
	}
	in := LocaleEnUS
	in.Number.GroupSizes = []int{3, 2}
	if got := formatGroupedInt(12345678, in); got != "1,23,45,678" {
		t.Fatalf("got %q", got)
	}
	// A multi-byte separator must survive intact: reversing the
	// output byte by byte would scramble its UTF-8 encoding.
	fr := LocaleEnUS
	fr.Number.GroupSep = ' '
	fr.Number.MinusSign = '−'
	if got := formatGroupedInt(-1234567, fr); got != "−1 234 567" {
		t.Fatalf("got %q", got)
	}
}

func TestLocaleDatePadFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		in, want string
	}{
		{"M/D/YYYY", "M/D/YYYY", "MM/DD/YYYY"},
		{"D.M.YYYY", "D.M.YYYY", "DD.MM.YYYY"},
		{"DD/MM/YYYY", "DD/MM/YYYY", "DD/MM/YYYY"},
		{"YYYY/M/D", "YYYY/M/D", "YYYY/MM/DD"},
		{"YYYY-M-D", "YYYY-M-D", "YYYY-MM-DD"},
		{"YYYY.M.D", "YYYY.M.D", "YYYY.MM.DD"},
		// Month-name tokens must not gain digits or be doubled.
		{"MMMM D, YYYY", "MMMM D, YYYY", "MMMM DD, YYYY"},
		{"D MMM YYYY", "D MMM YYYY", "DD MMM YYYY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := localeDatePadFormat(tt.in)
			if got != tt.want {
				t.Errorf("localeDatePadFormat(%q) = %q, want %q",
					tt.in, got, tt.want)
			}
		})
	}
}

func TestLocaleDateMaskPattern(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		in, want string
	}{
		{"M/D/YYYY", "M/D/YYYY", "99/99/9999"},
		{"D.M.YYYY", "D.M.YYYY", "99.99.9999"},
		{"DD/MM/YYYY", "DD/MM/YYYY", "99/99/9999"},
		{"YYYY/M/D", "YYYY/M/D", "9999/99/99"},
		{"YYYY-M-D", "YYYY-M-D", "9999-99-99"},
		{"YYYY.M.D", "YYYY.M.D", "9999.99.99"},
		// Month names pass through as literals: masked fields
		// take numeric formats only.
		{"MMMM D, YYYY", "MMMM D, YYYY", "MMMM 99, 9999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := localeDateMaskPattern(tt.in)
			if got != tt.want {
				t.Errorf("localeDateMaskPattern(%q) = %q, want %q",
					tt.in, got, tt.want)
			}
		})
	}
}

func TestLocaleT(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })

	SetLocale(Locale{
		Translations: map[string]string{
			"greeting": "hello",
		},
	})
	if got := LocaleT("greeting"); got != "hello" {
		t.Fatalf("got %q, want hello", got)
	}
	if got := LocaleT("missing"); got != "missing" {
		t.Fatalf("got %q, want missing", got)
	}
}

// localeParseDate is the only parse in the date path and had no test
// before issue #578 gave callers control of the format.
func TestLocaleParseDate(t *testing.T) {
	tests := []struct {
		format  string
		text    string
		wantErr bool
		year    int
		month   time.Month
		day     int
	}{
		{"MM/DD/YYYY", "12/24/2026", false, 2026, time.December, 24},
		{"DD.MM.YYYY", "24.12.2026", false, 2026, time.December, 24},
		{"DD/MM/YYYY", "24/12/2026", false, 2026, time.December, 24},
		{"YYYY-MM-DD", "2026-12-24", false, 2026, time.December, 24},
		{"M/D/YYYY", "1/5/2026", false, 2026, time.January, 5},
		{"DD.MM.YY", "24.12.26", false, 2026, time.December, 24},
		{"YYYY-MM-DD HH:mm:ss", "2026-12-24 14:05:09", false, 2026, time.December, 24},
		// The separators must match the format.
		{"DD.MM.YYYY", "24/12/2026", true, 0, 0, 0},
		// A day the month does not have.
		{"DD.MM.YYYY", "31.02.2026", true, 0, 0, 0},
		// Month names format but do not parse.
		{"MMMM D, YYYY", "March 15, 2025", true, 0, 0, 0},
	}
	for _, tt := range tests {
		got, err := localeParseDate(tt.text, tt.format)
		if tt.wantErr {
			if err == nil {
				t.Errorf("%s / %s: want error, got %v",
					tt.format, tt.text, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s / %s: %v", tt.format, tt.text, err)
			continue
		}
		if got.Year() != tt.year || got.Month() != tt.month ||
			got.Day() != tt.day {
			t.Errorf("%s / %s = %v, want %d-%02d-%02d",
				tt.format, tt.text, got, tt.year, tt.month, tt.day)
		}
	}
}

// LocaleT runs in view code per label per frame; reading the active
// locale must not deep-copy the Translations map on each lookup.
func TestLocaleTDoesNotAllocate(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	SetLocale(Locale{Translations: map[string]string{"k": "v"}})
	if got := testing.AllocsPerRun(100, func() {
		_ = LocaleT("k")
	}); got != 0 {
		t.Errorf("allocs = %v, want 0", got)
	}
}

func TestLocaleFormatNumber(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })

	indian := LocaleEnUS
	indian.Number.GroupSizes = []int{3, 2}
	// A multi-byte group separator and minus sign must survive intact.
	thin := LocaleEnUS
	thin.Number.GroupSep = '\u202f' // 3 bytes in UTF-8
	thin.Number.MinusSign = '−'
	// A hand-built Locale with zero runes falls back to en-US symbols,
	// the same as NumericInput, instead of writing NUL bytes.
	zero := Locale{}
	// A group separator equal to the decimal separator would make the
	// output ambiguous, so grouping is dropped.
	clash := LocaleEnUS
	clash.Number.GroupSep = '.'
	badSizes := LocaleEnUS
	badSizes.Number.GroupSizes = []int{0, -2}

	tests := []struct {
		name     string
		loc      Locale
		v        float64
		decimals int
		want     string
	}{
		{"en", LocaleEnUS, 1234567.891, 2, "1,234,567.89"},
		{"de", LocaleDeDE, 1234567.891, 2, "1.234.567,89"},
		{"fr nbsp", LocaleFrFR, 1234.5, 1, "1 234,5"},
		{"no decimals rounds", LocaleEnUS, 1234.6, 0, "1,235"},
		// An exact binary tie rounds to even, as strconv and
		// NumericInput do, so a label and an input agree.
		{"tie rounds to even", LocaleEnUS, 2.5, 0, "2"},
		{"tie rounds to even up", LocaleEnUS, 3.5, 0, "4"},
		{"short int", LocaleEnUS, 999, 2, "999.00"},
		{"zero", LocaleEnUS, 0, 2, "0.00"},
		{"negative", LocaleEnUS, -1234.5, 2, "-1,234.50"},
		{"negative rounds to zero", LocaleEnUS, -0.001, 2, "0.00"},
		{"negative zero", LocaleEnUS, math.Copysign(0, -1), 0, "0"},
		{"indian grouping", indian, 12345678, 0, "1,23,45,678"},
		{"multi-byte runes", thin, -1234567, 0, "−1\u202f234\u202f567"},
		{"zero runes", zero, -1234.5, 1, "-1,234.5"},
		{"sep clash", clash, 1234.5, 1, "1234.5"},
		{"negative decimals", LocaleEnUS, 12.6, -1, "13"},
		{"decimals clamp", LocaleEnUS, 1, 25, "1." + strings.Repeat("0", 20)},
		// The largest finite value overflows the stack buffers; the
		// result must still be exact and grouped.
		{"max float", LocaleEnUS, math.MaxFloat64, 20, maxFloatGrouped + "." + strings.Repeat("0", 20)},
		{"bad sizes fall back to 3", badSizes, 1234567, 0, "1,234,567"},
		{"NaN", LocaleEnUS, math.NaN(), 2, "NaN"},
		{"+Inf", LocaleEnUS, math.Inf(1), 2, "+Inf"},
		{"-Inf", LocaleEnUS, math.Inf(-1), 2, "-Inf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLocale(tt.loc)
			if got := LocaleFormatNumber(tt.v, tt.decimals); got != tt.want {
				t.Fatalf("LocaleFormatNumber(%v, %d) = %q, want %q",
					tt.v, tt.decimals, got, tt.want)
			}
		})
	}
}

func TestLocaleFormatCurrency(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })

	noDecimals := LocaleEnUS
	noDecimals.Currency.Decimals = 0
	noSymbol := LocaleEnUS
	noSymbol.Currency.Symbol = ""
	noSymbol.Currency.Spacing = true
	spacedPrefix := LocaleEnUS
	spacedPrefix.Currency.Spacing = true
	unspacedSuffix := LocaleDeDE
	unspacedSuffix.Currency.Spacing = false
	badPosition := LocaleEnUS
	badPosition.Currency.Position = NumericAffixPosition(9)

	tests := []struct {
		name string
		loc  Locale
		v    float64
		want string
	}{
		{"en prefix", LocaleEnUS, 1234.5, "$1,234.50"},
		{"en negative", LocaleEnUS, -1234.5, "-$1,234.50"},
		{"de suffix spaced", LocaleDeDE, 1234.5, "1.234,50 €"},
		{"de negative", LocaleDeDE, -1234.5, "-1.234,50 €"},
		{"ar multi-rune symbol", LocaleArSA, 1234.5, "1,234.50 ر.س"},
		{"prefix spaced", spacedPrefix, 5, "$ 5.00"},
		{"zero decimals", noDecimals, 1234.6, "$1,235"},
		{"empty symbol", noSymbol, 5, "5.00"},
		{"bad position is prefix", badPosition, 5, "$5.00"},
		{"suffix unspaced", unspacedSuffix, 5, "5,00€"},
		{"NaN", LocaleEnUS, math.NaN(), "NaN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLocale(tt.loc)
			if got := LocaleFormatCurrency(tt.v); got != tt.want {
				t.Fatalf("LocaleFormatCurrency(%v) = %q, want %q",
					tt.v, got, tt.want)
			}
		})
	}
}

// Views call these per label per frame. The result string is the only
// allocation: the active locale is read without a deep copy and the
// digits are formatted on the stack.
func TestLocaleFormatNumberCurrencyAllocs(t *testing.T) {
	saved := CurrentLocale()
	t.Cleanup(func() { SetLocale(saved) })
	SetLocale(LocaleDeDE)
	if got := testing.AllocsPerRun(100, func() {
		_ = LocaleFormatNumber(-1234567.891, 2)
	}); got != 1 {
		t.Errorf("LocaleFormatNumber allocs = %v, want 1", got)
	}
	if got := testing.AllocsPerRun(100, func() {
		_ = LocaleFormatCurrency(-1234567.891)
	}); got != 1 {
		t.Errorf("LocaleFormatCurrency allocs = %v, want 1", got)
	}
	// Groups of 1 with a 3-byte separator put more separator bytes
	// than digits. The builder is sized from the real break count,
	// so it still grows once.
	ones := LocaleDeDE
	ones.Number.GroupSizes = []int{1}
	ones.Number.GroupSep = '\u2009' // 3 bytes in UTF-8
	SetLocale(ones)
	if got := testing.AllocsPerRun(100, func() {
		_ = LocaleFormatCurrency(-1234567891234.5)
	}); got != 1 {
		t.Errorf("LocaleFormatCurrency [1] groups allocs = %v, want 1", got)
	}
}

// The data grid formats row, page and match counts each frame. The
// digits go through a stack buffer, so the result is the only
// allocation.
func TestFormatGroupedIntAllocs(t *testing.T) {
	if got := testing.AllocsPerRun(100, func() {
		_ = formatGroupedInt(-1234567, LocaleEnUS)
	}); got != 1 {
		t.Errorf("allocs = %v, want 1", got)
	}
	ones := LocaleEnUS
	ones.Number.GroupSizes = []int{1}
	ones.Number.GroupSep = '\u2009' // 3 bytes in UTF-8
	if got := testing.AllocsPerRun(100, func() {
		_ = formatGroupedInt(-1234567891234, ones)
	}); got != 1 {
		t.Errorf("[1] groups allocs = %v, want 1", got)
	}
}

// maxFloatGrouped is math.MaxFloat64's integer digits in en-US
// grouping, built from strconv so the test does not restate 309
// digits by hand.
var maxFloatGrouped = func() string {
	digits := strconv.FormatFloat(math.MaxFloat64, 'f', 0, 64)
	var sb strings.Builder
	for i, d := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			sb.WriteByte(',')
		}
		sb.WriteRune(d)
	}
	return sb.String()
}()
