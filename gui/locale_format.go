package gui

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// LocaleFormatDate formats a date using locale-aware month
// substitution. MMMM -> full month, MMM -> short month.
// Other tokens use a simple V-style token replacement:
//
//	YYYY->year, M->month, D->day, HH->hour, mm->minute,
//	ss->second.
func LocaleFormatDate(t time.Time, format string) string {
	return localeDateReplace(t, format, activeLocaleShared())
}

// localeDateReplace performs single-pass V-style date token
// substitution. Tokens: YYYY, MMMM (full month), MMM (short
// month), MM (zero-padded month), M (month), DD (zero-padded
// day), D (day), HH, mm, ss. A single left-to-right scan with
// longest-match takes MMMM before MMM before MM before M, so
// month names never see their letters re-scanned.
func localeDateReplace(t time.Time, format string, loc Locale) string {
	monthIdx := int(t.Month()) - 1
	monthValid := monthIdx >= 0 && monthIdx < 12
	var sb strings.Builder
	sb.Grow(len(format) + 8)
	for i := 0; i < len(format); {
		rest := format[i:]
		switch {
		case strings.HasPrefix(rest, "YYYY"):
			appendPadN(&sb, t.Year(), 4)
			i += 4
		case strings.HasPrefix(rest, "MMMM"):
			if monthValid {
				sb.WriteString(loc.MonthsFull[monthIdx])
			} else {
				sb.WriteString("MMMM")
			}
			i += 4
		case strings.HasPrefix(rest, "MMM"):
			if monthValid {
				sb.WriteString(loc.MonthsShort[monthIdx])
			} else {
				sb.WriteString("MMM")
			}
			i += 3
		case strings.HasPrefix(rest, "MM"):
			appendPad2(&sb, int(t.Month()))
			i += 2
		case rest[0] == 'M':
			sb.WriteString(strconv.Itoa(int(t.Month())))
			i++
		case strings.HasPrefix(rest, "DD"):
			appendPad2(&sb, t.Day())
			i += 2
		case rest[0] == 'D':
			sb.WriteString(strconv.Itoa(t.Day()))
			i++
		case strings.HasPrefix(rest, "HH"):
			appendPad2(&sb, t.Hour())
			i += 2
		case strings.HasPrefix(rest, "mm"):
			appendPad2(&sb, t.Minute())
			i += 2
		case strings.HasPrefix(rest, "ss"):
			appendPad2(&sb, t.Second())
			i += 2
		default:
			sb.WriteByte(rest[0])
			i++
		}
	}
	return sb.String()
}

// appendPad2 writes v zero-padded to two digits.
func appendPad2(sb *strings.Builder, v int) {
	appendPadN(sb, v, 2)
}

// appendPadN writes v zero-padded to width digits.
func appendPadN(sb *strings.Builder, v, width int) {
	s := strconv.Itoa(v)
	for i := len(s); i < width; i++ {
		sb.WriteByte('0')
	}
	sb.WriteString(s)
}

// localeDatePadFormat promotes M→MM, D→DD in a date format.
// Month-name tokens are sheltered first so MMMM never gains
// digits.
func localeDatePadFormat(format string) string {
	r := strings.ReplaceAll(format, "MMMM", "\x01")
	r = strings.ReplaceAll(r, "MMM", "\x02")
	if !strings.Contains(r, "MM") {
		r = strings.ReplaceAll(r, "M", "MM")
	}
	if !strings.Contains(r, "DD") {
		r = strings.ReplaceAll(r, "D", "DD")
	}
	r = strings.ReplaceAll(r, "\x01", "MMMM")
	r = strings.ReplaceAll(r, "\x02", "MMM")
	return r
}

// localeDateMaskPattern converts a numeric date format to a mask
// pattern: YYYY→9999, MM→99, DD→99, separators pass through.
// Month-name formats have no fixed-width digit mask; the names
// pass through as literals. Masked fields take numeric formats
// only (requireDateFormat enforces that); this stays total so
// previews and tests never corrupt the names.
func localeDateMaskPattern(format string) string {
	r := localeDatePadFormat(format)
	r = strings.ReplaceAll(r, "MMMM", "\x01")
	r = strings.ReplaceAll(r, "MMM", "\x02")
	r = strings.ReplaceAll(r, "YYYY", "9999")
	r = strings.ReplaceAll(r, "MM", "99")
	r = strings.ReplaceAll(r, "DD", "99")
	r = strings.ReplaceAll(r, "\x01", "MMMM")
	r = strings.ReplaceAll(r, "\x02", "MMM")
	return r
}

// localeParseDate parses a date string using locale format tokens.
// Converts locale tokens (YYYY or YY, MM, M, DD, D, HH, mm, ss)
// to a Go reference-time layout and parses. Month names (MMM,
// MMMM) have no fixed parse: they return an explicit error
// instead of a wrong date.
func localeParseDate(text, format string) (time.Time, error) {
	if strings.Contains(format, "MMM") {
		return time.Time{}, errors.New(
			"localeParseDate: month-name formats do not parse")
	}
	layout := format
	layout = strings.ReplaceAll(layout, "YYYY", "2006")
	layout = strings.ReplaceAll(layout, "YY", "06")
	if strings.Contains(layout, "MM") {
		layout = strings.ReplaceAll(layout, "MM", "01")
	} else {
		layout = strings.ReplaceAll(layout, "M", "1")
	}
	if strings.Contains(layout, "DD") {
		layout = strings.ReplaceAll(layout, "DD", "02")
	} else {
		layout = strings.ReplaceAll(layout, "D", "2")
	}
	layout = strings.ReplaceAll(layout, "HH", "15")
	layout = strings.ReplaceAll(layout, "mm", "04")
	layout = strings.ReplaceAll(layout, "ss", "05")
	return time.Parse(layout, text)
}

// LocaleRowsFmt formats "Rows start-end/total" with locale digit
// grouping.
func LocaleRowsFmt(start, end, total int) string {
	loc := activeLocaleShared()
	return fmt.Sprintf("%s %s-%s/%s",
		loc.StrRows,
		formatGroupedInt(start, loc),
		formatGroupedInt(end, loc),
		formatGroupedInt(total, loc))
}

// LocalePageFmt formats "Page current/total" with locale digit
// grouping.
func LocalePageFmt(page, total int) string {
	loc := activeLocaleShared()
	return fmt.Sprintf("%s %s/%s",
		loc.StrPage,
		formatGroupedInt(page, loc),
		formatGroupedInt(total, loc))
}

// LocaleMatchesFmt formats "Matches count/total" with locale
// digit grouping on the count. Total stays a string so callers
// can pass "?" for an unknown total.
func LocaleMatchesFmt(count int, total string) string {
	loc := activeLocaleShared()
	return fmt.Sprintf("%s %s/%s",
		loc.StrMatches, formatGroupedInt(count, loc), total)
}

// formatGroupedInt renders v with the locale's group separator
// and sizes. Sizes apply right to left with the last repeating
// ([3,2] groups 12345678 as 1,23,45,678); an empty size list
// groups by 3. The sign uses the locale's minus sign.
func formatGroupedInt(v int, loc Locale) string {
	// An int has at most 19 digits and a sign, so both buffers
	// below always suffice and stay on the stack.
	var buf [20]byte
	digits := strconv.AppendInt(buf[:0], int64(v), 10)
	neg := v < 0
	if neg {
		digits = digits[1:]
	}
	var breaksBuf [20]int
	breaks := groupBreaks(breaksBuf[:0], len(digits), loc.Number.GroupSizes)
	var sb strings.Builder
	sb.Grow(len(digits) + (len(breaks)+1)*utf8.UTFMax)
	if neg {
		sb.WriteRune(loc.Number.MinusSign)
	}
	writeGroupedDigits(&sb, digits, loc.Number.GroupSep, breaks)
	return sb.String()
}

// groupBreaks appends to dst the offsets in an n-digit integer
// where a group separator goes, rightmost first. Sizes apply
// right to left with the last repeating; a missing or
// non-positive size groups by 3. Callers count the breaks before
// they size a strings.Builder, so the builder grows once even
// for sizes smaller than 3.
func groupBreaks(dst []int, n int, sizes []int) []int {
	pos := n
	sizeIdx := 0
	for {
		size := 3
		if sizeIdx < len(sizes) && sizes[sizeIdx] > 0 {
			size = sizes[sizeIdx]
		}
		pos -= size
		if pos <= 0 {
			return dst
		}
		dst = append(dst, pos)
		if sizeIdx < len(sizes)-1 {
			sizeIdx++
		}
	}
}

// writeGroupedDigits writes the integer digits with sep at each
// offset in breaks (from groupBreaks). It writes left to right,
// which keeps a multi-byte separator's UTF-8 bytes in order;
// reversing a byte buffer would scramble them. A zero sep writes
// the digits with no grouping.
func writeGroupedDigits(sb *strings.Builder, digits []byte, sep rune, breaks []int) {
	if sep == 0 {
		sb.Write(digits)
		return
	}
	prev := 0
	for _, brk := range slices.Backward(breaks) {
		sb.Write(digits[prev:brk])
		sb.WriteRune(sep)
		prev = brk
	}
	sb.Write(digits[prev:])
}

// localeNumberDecimalsMax caps the fraction digits. It matches the
// 0-20 range LocaleParse accepts for currency decimals.
const localeNumberDecimalsMax = 20

// LocaleFormatNumber formats v with the active locale's decimal
// separator, digit grouping and minus sign. It rounds to decimals
// fraction digits, clamped to 0-20. An exact tie such as 2.5
// rounds to even, the same as NumericInput. A value that rounds
// to zero has no minus sign. NaN and ±Inf come back as "NaN",
// "+Inf" and "-Inf", with no grouping. It reads the active locale
// without a deep copy, so view code can call it every frame.
// exportaudit:keep — documented public API (widget_locale.md).
func LocaleFormatNumber(v float64, decimals int) string {
	// A zero CurrencyFormat has no symbol: the number alone.
	return localeFormatNumber(v, decimals, activeLocaleShared().Number, CurrencyFormat{})
}

// LocaleFormatCurrency formats v as an amount in the active
// locale's currency: Currency.Decimals fraction digits, the
// number rules of LocaleFormatNumber, and Currency.Symbol before
// or after the number by Currency.Position. Currency.Spacing puts
// a space between the symbol and the number. The minus sign goes
// first ("-$5.00", "-5,00 €"). An empty Symbol formats the number
// alone.
// exportaudit:keep — documented public API (widget_locale.md).
func LocaleFormatCurrency(v float64) string {
	loc := activeLocaleShared()
	return localeFormatNumber(v, loc.Currency.Decimals, loc.Number, loc.Currency)
}

// localeFormatNumber is the shared body of LocaleFormatNumber and
// LocaleFormatCurrency. It reads only c's Symbol, Position and
// Spacing; an empty Symbol formats the number alone.
func localeFormatNumber(v float64, decimals int, nf NumberFormat, c CurrencyFormat) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	decimals = min(max(decimals, 0), localeNumberDecimalsMax)

	// Zero runes come from a hand-built Locale. Fall back to the
	// en-US symbols, the same as NumericInput, so the output never
	// holds NUL bytes. A group separator equal to the decimal
	// separator would make the text ambiguous, so grouping is off.
	decimalSep := nf.DecimalSep
	if decimalSep == 0 {
		decimalSep = '.'
	}
	groupSep := nf.GroupSep
	if groupSep == 0 {
		groupSep = ','
	}
	if groupSep == decimalSep {
		groupSep = 0
	}
	minus := nf.MinusSign
	if minus == 0 {
		minus = '-'
	}

	// Format the magnitude on the stack. 64 bytes hold any value
	// below about 1e40 with 20 decimals; a larger one spills to
	// the heap.
	var buf [64]byte
	digits := strconv.AppendFloat(buf[:0], math.Abs(v), 'f', decimals, 64)
	intDigits, fracDigits := digits, digits[:0]
	if dot := slices.Index(digits, '.'); dot >= 0 {
		intDigits, fracDigits = digits[:dot], digits[dot+1:]
	}
	// A value that rounds to zero (-0.001 at 2 decimals, or -0)
	// prints without a sign: "-0.00" reads as a real negative.
	neg := v < 0 && slices.ContainsFunc(digits, func(b byte) bool {
		return b >= '1' && b <= '9'
	})

	// 32 break slots cover any int-sized integer part; a longer one
	// (above about 1e32) spills to the heap, which is rare.
	var breaksBuf [32]int
	var breaks []int
	if groupSep != 0 {
		breaks = groupBreaks(breaksBuf[:0], len(intDigits), nf.GroupSizes)
	}

	affix := c.Symbol
	space := c.Spacing && affix != ""
	var sb strings.Builder
	// Separators, the minus sign and the decimal separator are each
	// at most utf8.UTFMax bytes; +1 is the affix space.
	sb.Grow(len(digits) + (len(breaks)+2)*utf8.UTFMax + len(affix) + 1)
	if neg {
		sb.WriteRune(minus)
	}
	// Any position other than AffixSuffix is a prefix. A bad value
	// from a hand-built Locale then still shows the symbol, in the
	// zero-value place, instead of dropping it.
	suffix := c.Position == AffixSuffix
	if affix != "" && !suffix {
		sb.WriteString(affix)
		if space {
			sb.WriteByte(' ')
		}
	}
	writeGroupedDigits(&sb, intDigits, groupSep, breaks)
	if len(fracDigits) > 0 {
		sb.WriteRune(decimalSep)
		sb.Write(fracDigits)
	}
	if affix != "" && suffix {
		if space {
			sb.WriteByte(' ')
		}
		sb.WriteString(affix)
	}
	return sb.String()
}

// LocaleT looks up a translation key in the current locale.
// Returns the key itself when not found.
// exportaudit:keep — documented public API (showcase docs)
func LocaleT(key string) string {
	if v, ok := activeLocaleShared().Translations[key]; ok {
		return v
	}
	return key
}
