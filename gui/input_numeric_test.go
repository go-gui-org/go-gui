package gui

import (
	"math"
	"testing"
)

// numericFormat is the test-facing entry point. It normalizes
// the locale once, then delegates to numericFormatValue.
func numericFormat(value float64, decimals int, locale NumericLocaleCfg) string {
	loc := numericLocaleNormalize(locale)
	if value == 0 {
		value = 0 // collapse -0.0 → 0.0
	}
	return numericFormatValue(value, decimals, loc)
}

// numericInputCommitResult resolves text → (value, formatted).
func numericInputCommitResult(text string, value, minVal, maxVal Opt[float64], decimals int, locale NumericLocaleCfg) (Opt[float64], string) {
	return numericInputCommitResultMode(text, value, minVal, maxVal, decimals, locale, numericModeCfg{displayMultiplier: 1.0})
}

// numericInputStepResult steps a value in the given direction.
func numericInputStepResult(text string, value, minVal, maxVal Opt[float64], decimals int, stepCfg NumericStepCfg, locale NumericLocaleCfg, direction float64, modifiers Modifier) (Opt[float64], string) {
	return numericInputStepResultMode(text, value, minVal, maxVal, decimals, stepCfg, locale, direction, modifiers, numericModeCfg{displayMultiplier: 1.0})
}

func assertF64Near(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) >= 0.000001 {
		t.Errorf("got %f, want %f", got, want)
	}
}

func TestNumericLocaleNormalizeClonesSlices(t *testing.T) {
	// The result must not alias the caller's slice or the package
	// default: an edit through one handle must stay invisible
	// through the other.
	src := []int{3, 2}
	got := numericLocaleNormalize(NumericLocaleCfg{GroupSizes: src})
	got.GroupSizes[0] = 99
	if src[0] != 3 {
		t.Fatalf("caller slice changed to %v through the result", src)
	}
	def := numericLocaleNormalize(NumericLocaleCfg{})
	def.GroupSizes[0] = 99
	again := numericLocaleNormalize(NumericLocaleCfg{})
	if len(again.GroupSizes) != 1 || again.GroupSizes[0] != 3 {
		t.Fatalf("default group sizes = %v, want [3]", again.GroupSizes)
	}
}

func TestNumericParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		input  string
		locale NumericLocaleCfg
		wantOK bool
		want   float64
	}{
		{"EN_locale", "1,234.50", NumericLocaleCfg{}, true, 1234.5},
		{"DE_locale", "1.234,50",
			NumericLocaleCfg{DecimalSep: ',', GroupSep: '.'}, true, 1234.5},
		{"double_separator", "1,,234", NumericLocaleCfg{}, false, 0},
		{"non_numeric", "abc", NumericLocaleCfg{}, false, 0},
		{"invalid_grouping", "12,345,67",
			NumericLocaleCfg{GroupSizes: []int{3, 2}}, false, 0},
		{"colliding_separators", "1.234",
			NumericLocaleCfg{DecimalSep: '.', GroupSep: '.'}, true, 1.234},
		{"indian_numbering", "12,34,567",
			NumericLocaleCfg{GroupSizes: []int{3, 2}}, true, 1234567},
		// Groups past the configured list repeat the last size, the same rule the
		// formatter follows; a fallback to 3 here rejected "1,23,45,67,890".
		{"indian_numbering_repeats_last_size", "1,23,45,67,890",
			NumericLocaleCfg{GroupSizes: []int{3, 2}}, true, 1234567890},
		{"indian_numbering_rejects_default_size", "1,234,567,890",
			NumericLocaleCfg{GroupSizes: []int{3, 2}}, false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			loc := numericLocaleNormalize(tt.locale)
			v, ok := numericParse(tt.input, loc)
			if ok != tt.wantOK {
				t.Fatalf("numericParse(%q): ok = %v, want %v",
					tt.input, ok, tt.wantOK)
			}
			if ok {
				assertF64Near(t, v, tt.want)
			}
		})
	}
}

func TestNumericFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		val      float64
		decimals int
		locale   NumericLocaleCfg
		want     string
	}{
		{"EN_locale", 1234.5, 2, NumericLocaleCfg{}, "1,234.50"},
		{"DE_locale", 1234.5, 2,
			NumericLocaleCfg{DecimalSep: ',', GroupSep: '.'}, "1.234,50"},
		{"group_sizes", 1234567, 0,
			NumericLocaleCfg{GroupSep: ',', GroupSizes: []int{3, 2}},
			"12,34,567"},
		{"large_float", 1.5e20, 2, NumericLocaleCfg{},
			"150,000,000,000,000,000,000.00"},
		{"negative_zero", math.Copysign(0, -1), 2,
			NumericLocaleCfg{}, "0.00"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := numericFormat(tt.val, tt.decimals, tt.locale)
			if got != tt.want {
				t.Errorf("numericFormat(%v, %d) = %q, want %q",
					tt.val, tt.decimals, got, tt.want)
			}
		})
	}
}

// TestNumericFormatParseRoundTrip pins that every string the formatter emits is
// accepted by the parser for the same locale. The two paths each look up group
// sizes, and a disagreement past the end of GroupSizes made a displayed value
// fail to re-parse on commit or step.
func TestNumericFormatParseRoundTrip(t *testing.T) {
	t.Parallel()
	locales := []NumericLocaleCfg{
		{},
		{GroupSizes: []int{3, 2}},
		{GroupSizes: []int{2, 3, 4}},
		{DecimalSep: ',', GroupSep: '.', GroupSizes: []int{3, 2}},
	}
	values := []float64{0, 12, 1234, 1234567, 1234567890, -9876543210.25}
	for _, src := range locales {
		loc := numericLocaleNormalize(src)
		for _, val := range values {
			str := numericFormat(val, 2, loc)
			got, ok := numericParse(str, loc)
			if !ok {
				t.Errorf("numericParse(%q) rejected formatter output, sizes %v",
					str, loc.GroupSizes)
				continue
			}
			assertF64Near(t, got, val)
		}
	}
}

func TestNumericCommitResult(t *testing.T) {
	t.Parallel()
	t.Run("clamps", func(t *testing.T) {
		t.Parallel()
		val, text := numericInputCommitResult(
			"1,250.30", Opt[float64]{},
			Some(0.0), Some(1000.0), 2, NumericLocaleCfg{})
		if text != "1,000.00" {
			t.Fatalf("got %q, want %q", text, "1,000.00")
		}
		if !val.IsSet() {
			t.Fatal("expected set value")
		}
		assertF64Near(t, val.Get(0), 1000.0)
	})
	t.Run("invalid_fallback", func(t *testing.T) {
		t.Parallel()
		val, text := numericInputCommitResult(
			"abc", Some(12.5),
			Opt[float64]{}, Opt[float64]{}, 1, NumericLocaleCfg{})
		if text != "12.5" {
			t.Fatalf("got %q, want %q", text, "12.5")
		}
		if !val.IsSet() {
			t.Fatal("expected set value")
		}
		assertF64Near(t, val.Get(0), 12.5)
	})
	t.Run("invalid_no_value", func(t *testing.T) {
		t.Parallel()
		val, text := numericInputCommitResult(
			"abc", Opt[float64]{},
			Opt[float64]{}, Opt[float64]{}, 2, NumericLocaleCfg{})
		if val.IsSet() {
			t.Fatal("expected unset value")
		}
		if text != "" {
			t.Fatalf("got %q, want empty", text)
		}
	})
}

func TestNumericCurrencyCommit(t *testing.T) {
	t.Parallel()
	t.Run("prefix_symbol", func(t *testing.T) {
		t.Parallel()
		mc := numericModeCfg{
			mode:              NumericCurrency,
			affix:             "$",
			affixPosition:     affixPrefix,
			displayMultiplier: 1.0,
		}
		val, text := numericInputCommitResultMode(
			"-$1,234.5", Opt[float64]{},
			Opt[float64]{}, Opt[float64]{}, 2, NumericLocaleCfg{}, mc)
		if text != "-$1,234.50" {
			t.Fatalf("got %q, want %q", text, "-$1,234.50")
		}
		if !val.IsSet() {
			t.Fatal("expected set value")
		}
		assertF64Near(t, val.Get(0), -1234.5)
	})
	t.Run("suffix_symbol", func(t *testing.T) {
		t.Parallel()
		locale := NumericLocaleCfg{DecimalSep: ',', GroupSep: '.'}
		mc := numericModeCfg{
			mode:              NumericCurrency,
			affix:             "EUR",
			affixPosition:     affixSuffix,
			affixSpacing:      true,
			displayMultiplier: 1.0,
		}
		val, text := numericInputCommitResultMode(
			"1.234,5 EUR", Opt[float64]{},
			Opt[float64]{}, Opt[float64]{}, 2, locale, mc)
		if text != "1.234,50 EUR" {
			t.Fatalf("got %q, want %q", text, "1.234,50 EUR")
		}
		if !val.IsSet() {
			t.Fatal("expected set value")
		}
		assertF64Near(t, val.Get(0), 1234.5)
	})
}

func TestNumericPreCommit(t *testing.T) {
	t.Parallel()
	t.Run("rejects_invalid_delta", func(t *testing.T) {
		t.Parallel()
		mc := numericModeCfg{displayMultiplier: 1.0}
		_, ok := numericInputPreCommitTransformMode(
			"12", "12a", 2, NumericLocaleCfg{}, mc)
		if ok {
			t.Fatal("expected rejection")
		}
	})
	t.Run("transient_forms", func(t *testing.T) {
		t.Parallel()
		mc := numericModeCfg{displayMultiplier: 1.0}
		got, ok := numericInputPreCommitTransformMode(
			"", "-", 2, NumericLocaleCfg{}, mc)
		if !ok {
			t.Fatal("expected accept")
		}
		if got != "-" {
			t.Fatalf("got %q, want %q", got, "-")
		}
		got, ok = numericInputPreCommitTransformMode(
			"12", "12.", 2, NumericLocaleCfg{}, mc)
		if !ok {
			t.Fatal("expected accept")
		}
		if got != "12." {
			t.Fatalf("got %q, want %q", got, "12.")
		}
	})
	t.Run("currency_affix", func(t *testing.T) {
		t.Parallel()
		mc := numericModeCfg{
			mode:              NumericCurrency,
			affix:             "$",
			affixPosition:     affixPrefix,
			displayMultiplier: 1.0,
		}
		got, ok := numericInputPreCommitTransformMode(
			"", "$", 2, NumericLocaleCfg{}, mc)
		if !ok {
			t.Fatal("expected accept")
		}
		if got != "$" {
			t.Fatalf("got %q, want %q", got, "$")
		}
		got, ok = numericInputPreCommitTransformMode(
			"", "-$", 2, NumericLocaleCfg{}, mc)
		if !ok {
			t.Fatal("expected accept")
		}
		if got != "-$" {
			t.Fatalf("got %q, want %q", got, "-$")
		}
	})
	t.Run("percent_affix", func(t *testing.T) {
		t.Parallel()
		mc := numericModeCfg{
			mode:              NumericPercent,
			affix:             "%",
			affixPosition:     affixSuffix,
			displayMultiplier: 100.0,
		}
		got, ok := numericInputPreCommitTransformMode(
			"", "%", 2, NumericLocaleCfg{}, mc)
		if !ok {
			t.Fatal("expected accept")
		}
		if got != "%" {
			t.Fatalf("got %q, want %q", got, "%")
		}
		got, ok = numericInputPreCommitTransformMode(
			"12", "12.%", 2, NumericLocaleCfg{}, mc)
		if !ok {
			t.Fatal("expected accept")
		}
		if got != "12.%" {
			t.Fatalf("got %q, want %q", got, "12.%")
		}
	})
}

func TestNumericClampUnbounded(t *testing.T) {
	t.Parallel()
	v := 1.0e308
	assertF64Near(t, numericClamp(v, Opt[float64]{}, Opt[float64]{}), v)
}

func TestNumericStepResultUsesMinSeed(t *testing.T) {
	t.Parallel()
	val, text := numericInputStepResult(
		"", Opt[float64]{}, Some(10.0), Opt[float64]{}, 2,
		NumericStepCfg{}, NumericLocaleCfg{}, 1.0, ModNone)
	if text != "11.00" {
		t.Fatalf("got %q, want %q", text, "11.00")
	}
	if !val.IsSet() {
		t.Fatal("expected set value")
	}
	assertF64Near(t, val.Get(0), 11.0)
}

func TestNumericStepResultModifiers(t *testing.T) {
	t.Parallel()
	cfg := NumericStepCfg{
		Step:            1.0,
		ShiftMultiplier: 10.0,
		AltMultiplier:   0.1,
	}
	t.Run("shift", func(t *testing.T) {
		t.Parallel()
		val, text := numericInputStepResult(
			"5", Opt[float64]{}, Opt[float64]{}, Opt[float64]{},
			2, cfg, NumericLocaleCfg{}, 1.0, ModShift)
		if text != "15.00" {
			t.Fatalf("got %q, want %q", text, "15.00")
		}
		if !val.IsSet() {
			t.Fatal("expected set value")
		}
		assertF64Near(t, val.Get(0), 15.0)
	})
	t.Run("alt", func(t *testing.T) {
		t.Parallel()
		val, text := numericInputStepResult(
			"5", Opt[float64]{}, Opt[float64]{}, Opt[float64]{},
			2, cfg, NumericLocaleCfg{}, 1.0, ModAlt)
		if text != "5.10" {
			t.Fatalf("got %q, want %q", text, "5.10")
		}
		if !val.IsSet() {
			t.Fatal("expected set value")
		}
		assertF64Near(t, val.Get(0), 5.1)
	})
}

func TestNumericPercentCommitRatioValue(t *testing.T) {
	t.Parallel()
	mc := numericModeCfg{
		mode:              NumericPercent,
		affix:             "%",
		affixPosition:     affixSuffix,
		displayMultiplier: 100.0,
	}
	val, text := numericInputCommitResultMode(
		"12.5%", Opt[float64]{},
		Opt[float64]{}, Opt[float64]{}, 2, NumericLocaleCfg{}, mc)
	if text != "12.50%" {
		t.Fatalf("got %q, want %q", text, "12.50%")
	}
	if !val.IsSet() {
		t.Fatal("expected set value")
	}
	assertF64Near(t, val.Get(0), 0.125)
}

func TestNumericPercentStepResult(t *testing.T) {
	t.Parallel()
	mc := numericModeCfg{
		mode:              NumericPercent,
		affix:             "%",
		affixPosition:     affixSuffix,
		displayMultiplier: 100.0,
	}
	val, text := numericInputStepResultMode(
		"12.50%", Opt[float64]{},
		Opt[float64]{}, Opt[float64]{}, 2,
		NumericStepCfg{}, NumericLocaleCfg{}, 1.0, ModNone, mc)
	if text != "13.50%" {
		t.Fatalf("got %q, want %q", text, "13.50%")
	}
	if !val.IsSet() {
		t.Fatal("expected set value")
	}
	assertF64Near(t, val.Get(0), 0.135)
}

func TestNumericPercentRoundTrip(t *testing.T) {
	t.Parallel()
	mc := numericModeCfg{
		mode:              NumericPercent,
		affix:             "%",
		affixPosition:     affixSuffix,
		displayMultiplier: 100.0,
	}
	loc := numericLocaleNormalize(NumericLocaleCfg{})
	source := -0.125
	formatted := numericModeFormatValue(source, 2, loc, mc)
	if formatted != "-12.50%" {
		t.Fatalf("formatted: got %q, want %q", formatted, "-12.50%")
	}
	parsed, ok := numericModeParseValue(formatted, 2, loc, mc)
	if !ok {
		t.Fatal("parse failed")
	}
	assertF64Near(t, parsed, source)
}

func TestNumericGroupIntegerPartNonStandard(t *testing.T) {
	t.Parallel()
	got := numericGroupIntegerPart("100", ',', []int{2})
	if got != "1,00" {
		t.Fatalf("got %q, want %q", got, "1,00")
	}
}

func TestNumericEmptyPrefixSpacing(t *testing.T) {
	t.Parallel()
	mc := numericModeCfg{
		mode:              NumericCurrency,
		affix:             "$",
		affixPosition:     affixPrefix,
		affixSpacing:      true,
		displayMultiplier: 1.0,
	}
	got := numericApplyAffix("",
		numericLocaleNormalize(NumericLocaleCfg{}), mc)
	if got != "$" {
		t.Fatalf("got %q, want %q", got, "$")
	}
}

func TestNumericClampNaNBoundsIgnored(t *testing.T) {
	t.Parallel()
	// A NaN bound is caller garbage, not a constraint: NaN fails
	// every comparison, so it must behave as unset.
	nan := Some(math.NaN())
	if got := numericClamp(5, nan, Opt[float64]{}); got != 5 {
		t.Errorf("NaN min clamped 5 to %v", got)
	}
	if got := numericClamp(-500, nan, Some(10.0)); got != -500 {
		t.Errorf("NaN min clamped -500 to %v", got)
	}
	if got := numericClamp(500, Some(5.0), nan); got != 500 {
		t.Errorf("NaN max clamped 500 to %v", got)
	}
	if got := numericClamp(3, Some(5.0), Some(10.0)); got != 5 {
		t.Errorf("real bounds clamped 3 to %v, want 5", got)
	}
}

func TestNumericStepNaNSeedsFallBack(t *testing.T) {
	t.Parallel()
	// Unparseable text forces the seed onto Value/Min. NaN there
	// must fall through to 0 rather than committing "NaN".
	_, s := numericInputStepResult("abc", Some(math.NaN()),
		Opt[float64]{}, Opt[float64]{}, 0,
		NumericStepCfg{Step: 1}, NumericLocaleCfg{}, 1, ModNone)
	if s != "1" {
		t.Errorf("NaN value seeded %q, want \"1\"", s)
	}
	_, s = numericInputStepResult("abc", Opt[float64]{},
		Some(math.NaN()), Opt[float64]{}, 0,
		NumericStepCfg{Step: 1}, NumericLocaleCfg{}, 1, ModNone)
	if s != "1" {
		t.Errorf("NaN min seeded %q, want \"1\"", s)
	}
}

func TestNumericCommitNaNValueStaysUnset(t *testing.T) {
	t.Parallel()
	v, s := numericInputCommitResult("abc", Some(math.NaN()),
		Opt[float64]{}, Opt[float64]{}, 0, NumericLocaleCfg{})
	if _, ok := v.Value(); ok {
		t.Errorf("NaN value fallback committed %v, want unset", v)
	}
	if s != "" {
		t.Errorf("NaN value fallback formatted %q, want \"\"", s)
	}
}

func TestNumericStepPrecisionLossNotRefused(t *testing.T) {
	// 1e16 + 1 rounds back to 1e16: nothing moved, but the value
	// is interior, not stuck at a bound, so no refusal cue.
	mc := numericModeCfg{displayMultiplier: 1.0}
	_, _, refused := numericInputStepResultClamped("10000000000000000",
		Opt[float64]{}, Opt[float64]{}, Opt[float64]{}, 0,
		NumericStepCfg{Step: 1}, NumericLocaleCfg{}, 1, ModNone, mc)
	if refused {
		t.Fatal("a precision-lost step must not report refused")
	}
	// A value already sitting at Max still refuses.
	_, _, refused = numericInputStepResultClamped("5",
		Opt[float64]{}, Some(5.0), Some(5.0), 0,
		NumericStepCfg{Step: 1}, NumericLocaleCfg{}, 1, ModNone, mc)
	if !refused {
		t.Fatal("a step against Max must report refused")
	}
	// Same, pushing down against Min.
	_, _, refused = numericInputStepResultClamped("5",
		Opt[float64]{}, Some(5.0), Some(5.0), 0,
		NumericStepCfg{Step: 1}, NumericLocaleCfg{}, -1, ModNone, mc)
	if !refused {
		t.Fatal("a step against Min must report refused")
	}
}

func TestNumericBoundsInvertedSwapsDefensively(t *testing.T) {
	// Inverted bounds never reach here through NumericInput —
	// requireNumericBounds panics at construction — but the
	// clamp keeps its old swap so a direct caller still gets a
	// sane interval instead of a field clamped the wrong way.
	lo, hi := numericBounds(Some(10.0), Some(5.0))
	if lo != 5 || hi != 10 {
		t.Fatalf("inverted bounds resolved to [%v %v], want [5 10]",
			lo, hi)
	}
	if got := numericClamp(7, Some(10.0), Some(5.0)); got != 7 {
		t.Fatalf("clamp(7) in a swapped [5 10] = %v, want 7", got)
	}
	if got := numericClamp(3, Some(10.0), Some(5.0)); got != 5 {
		t.Fatalf("clamp(3) in a swapped [5 10] = %v, want 5", got)
	}
}

func TestNumericInfValueIsNotASeed(t *testing.T) {
	// A ±Inf Value is caller garbage, not data: seeding from it
	// formats "+Inf", which no locale parses back. The seed falls
	// through to the text, the Min, and finally zero.
	loc := numericLocaleNormalize(NumericLocaleCfg{})
	mc := numericModeCfg{displayMultiplier: 1.0}
	if got := numericStepSeedMode("", Some(math.Inf(1)),
		Opt[float64]{}, 0, loc, mc); got != 0 {
		t.Fatalf("seed from +Inf = %v, want 0", got)
	}
	if got := numericStepSeedMode("12", Some(math.Inf(-1)),
		Opt[float64]{}, 0, loc, mc); got != 12 {
		t.Fatalf("seed from -Inf over %q = %v, want 12", "12", got)
	}
}

func TestNumericInfValueCommitFallsBackToEmpty(t *testing.T) {
	// Unparsable text with a ±Inf Value commits nothing rather
	// than formatting "+Inf".
	loc := numericLocaleNormalize(NumericLocaleCfg{})
	mc := numericModeCfg{displayMultiplier: 1.0}
	v, s := numericInputCommitResultMode("abc", Some(math.Inf(1)),
		Opt[float64]{}, Opt[float64]{}, 0, loc, mc)
	if _, ok := v.Value(); ok {
		t.Fatalf("commit with +Inf Value = %v, want empty", v)
	}
	if s != "" {
		t.Fatalf("commit text = %q, want empty", s)
	}
}

func TestNumericBoundsInfCountsAsUnset(t *testing.T) {
	// A ±Inf bound past the wrong end (Min=+Inf, Max=-Inf) would
	// clamp every value into a number no locale formats. Both
	// count as unset, like NaN; ±Inf on the open end already
	// equals the default and stays put.
	lo, hi := numericBounds(Some(math.Inf(1)), Opt[float64]{})
	if !math.IsInf(lo, -1) || !math.IsInf(hi, 1) {
		t.Fatalf("Min=+Inf bounds = [%v %v], want [-Inf +Inf]", lo, hi)
	}
	lo, hi = numericBounds(Opt[float64]{}, Some(math.Inf(-1)))
	if !math.IsInf(lo, -1) || !math.IsInf(hi, 1) {
		t.Fatalf("Max=-Inf bounds = [%v %v], want [-Inf +Inf]", lo, hi)
	}
	if got := numericClamp(7, Some(math.Inf(1)), Opt[float64]{}); got != 7 {
		t.Fatalf("clamp(7) with Min=+Inf = %v, want 7", got)
	}
}
