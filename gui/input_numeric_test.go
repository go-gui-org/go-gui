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
		shiftMultiplier: 10.0,
		altMultiplier:   0.1,
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
