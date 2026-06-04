package gui

import (
	"math"
	"slices"
	"strconv"
	"strings"
)

// NumericInputMode determines how numeric values are displayed.
type NumericInputMode uint8

// NumericInputMode values.
const (
	NumericNumber NumericInputMode = iota
	NumericCurrency
	NumericPercent
)

// NumericAffixPosition determines prefix/suffix placement.
type NumericAffixPosition uint8

// NumericAffixPosition values.
const (
	AffixPrefix NumericAffixPosition = iota
	AffixSuffix
)

// NumericLocaleCfg defines symbols for parse/format.
type NumericLocaleCfg struct {
	GroupSizes []int
	DecimalSep rune
	GroupSep   rune
	MinusSign  rune
	PlusSign   rune
}

// NumericStepCfg configures stepping interactions.
type NumericStepCfg struct {
	Step            float64
	ShiftMultiplier float64
	AltMultiplier   float64
	MouseWheel      bool
	Keyboard        bool
	ShowButtons     bool
}

// NumericCurrencyModeCfg defines currency symbol placement.
type NumericCurrencyModeCfg struct {
	Symbol        string
	Position      NumericAffixPosition
	SymbolSpacing bool
}

// NumericPercentModeCfg defines percent symbol placement.
type NumericPercentModeCfg struct {
	Symbol        string
	Position      NumericAffixPosition
	SymbolSpacing bool
}

// numericModeCfg is an internal config for mode-aware
// parse/format.
type numericModeCfg struct {
	affix             string
	displayMultiplier float64
	mode              NumericInputMode
	affixPosition     NumericAffixPosition
	affixSpacing      bool
}

var defaultGroupSizes = []int{3}

func numericLocaleNormalize(cfg NumericLocaleCfg) NumericLocaleCfg {
	// Fast path: reuse slice when all sizes are already positive.
	sizes := cfg.GroupSizes
	allPositive := len(sizes) > 0
	for _, s := range sizes {
		if s <= 0 {
			allPositive = false
			break
		}
	}
	if !allPositive {
		filtered := make([]int, 0, len(sizes))
		for _, s := range sizes {
			if s > 0 {
				filtered = append(filtered, s)
			}
		}
		sizes = filtered
	}
	if len(sizes) == 0 {
		sizes = defaultGroupSizes
	}
	groupSep := cfg.GroupSep
	if groupSep == 0 {
		groupSep = ','
	}
	decimalSep := cfg.DecimalSep
	if decimalSep == 0 {
		decimalSep = '.'
	}
	if groupSep == decimalSep {
		groupSep = 0
	}
	minus := cfg.MinusSign
	if minus == 0 {
		minus = '-'
	}
	plus := cfg.PlusSign
	if plus == 0 {
		plus = '+'
	}
	return NumericLocaleCfg{
		DecimalSep: decimalSep,
		GroupSep:   groupSep,
		GroupSizes: sizes,
		MinusSign:  minus,
		PlusSign:   plus,
	}
}

func numericStepCfgNormalize(cfg NumericStepCfg) NumericStepCfg {
	step := cfg.Step
	if step <= 0 {
		step = 1.0
	}
	shift := cfg.ShiftMultiplier
	if shift <= 0 {
		shift = 10.0
	}
	alt := cfg.AltMultiplier
	if alt <= 0 {
		alt = 0.1
	}
	return NumericStepCfg{
		Step:            step,
		ShiftMultiplier: shift,
		AltMultiplier:   alt,
		MouseWheel:      cfg.MouseWheel,
		Keyboard:        cfg.Keyboard,
		ShowButtons:     cfg.ShowButtons,
	}
}

func numericDecimalsClamped(decimals int) int {
	if decimals < 0 {
		return 0
	}
	if decimals > 9 {
		return 9
	}
	return decimals
}

func numericRoundToDecimals(value float64, decimals int) float64 {
	d := numericDecimalsClamped(decimals)
	str := strconv.FormatFloat(value, 'f', d, 64)
	rounded, _ := strconv.ParseFloat(str, 64)
	return rounded
}

func numericGroupSize(groupSizes []int, idx int) int {
	if idx >= 0 && idx < len(groupSizes) && groupSizes[idx] > 0 {
		return groupSizes[idx]
	}
	return 3
}

func numericGroupIntegerPart(raw string, groupSep rune, groupSizes []int) string {
	if groupSep == 0 {
		return raw
	}
	digits := []rune(raw)
	if len(digits) <= numericGroupSize(groupSizes, 0) {
		return raw
	}
	var reversed []rune
	count := 0
	groupIdx := 0
	gs := numericGroupSize(groupSizes, groupIdx)
	for i := range slices.Backward(digits) {
		reversed = append(reversed, digits[i])
		count++
		if i > 0 && count == gs {
			reversed = append(reversed, groupSep)
			count = 0
			if groupIdx+1 < len(groupSizes) {
				groupIdx++
			}
			gs = numericGroupSize(groupSizes, groupIdx)
		}
	}
	// Reverse in place.
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return string(reversed)
}

func numericFormatValue(value float64, decimals int, loc NumericLocaleCfg) string {
	d := numericDecimalsClamped(decimals)

	str := strconv.FormatFloat(math.Abs(value), 'f', d, 64)

	sign := ""
	if value < 0 {
		sign = string(loc.MinusSign)
	}

	parts := strings.Split(str, ".")
	intPart := parts[0]
	grouped := numericGroupIntegerPart(intPart, loc.GroupSep, loc.GroupSizes)
	if d == 0 || len(parts) == 1 {
		return sign + grouped
	}

	fracPart := parts[1]
	return sign + grouped + string(loc.DecimalSep) + fracPart
}

func numericIntegerGroupsValid(intSegment []rune, groupSep rune, groupSizes []int) bool {
	var groupLengths []int
	count := 0
	for _, ch := range slices.Backward(intSegment) {
		if ch == groupSep {
			if count == 0 {
				return false
			}
			groupLengths = append(groupLengths, count)
			count = 0
			continue
		}
		if ch < '0' || ch > '9' {
			return false
		}
		count++
	}
	if count == 0 {
		return false
	}
	groupLengths = append(groupLengths, count)
	for idx := range len(groupLengths) {
		length := groupLengths[idx]
		expected := numericGroupSize(groupSizes, idx)
		if idx == len(groupLengths)-1 {
			if length <= expected {
				continue
			}
			return false
		}
		if length != expected {
			return false
		}
	}
	return true
}

// numericParse parses a locale-formatted number string.
// Caller must pass a normalized locale.
func numericParse(raw string, loc NumericLocaleCfg) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	rs := []rune(raw)
	start := 0
	var normalized []rune
	seenDigit := false
	seenDecimal := false
	prevGroup := false
	sawGroupSep := false
	decimalIndex := -1

	switch rs[0] {
	case loc.MinusSign:
		normalized = append(normalized, '-')
		start = 1
	case loc.PlusSign:
		start = 1
	}
	for i := start; i < len(rs); i++ {
		ch := rs[i]
		if ch >= '0' && ch <= '9' {
			normalized = append(normalized, ch)
			seenDigit = true
			prevGroup = false
			continue
		}
		if ch == loc.DecimalSep {
			if seenDecimal || prevGroup {
				return 0, false
			}
			normalized = append(normalized, '.')
			seenDecimal = true
			prevGroup = false
			decimalIndex = i
			continue
		}
		if loc.GroupSep != 0 && ch == loc.GroupSep {
			if seenDecimal || !seenDigit || prevGroup {
				return 0, false
			}
			prevGroup = true
			sawGroupSep = true
			continue
		}
		return 0, false
	}
	if !seenDigit || prevGroup {
		return 0, false
	}
	if loc.GroupSep != 0 && sawGroupSep {
		intEnd := len(rs)
		if decimalIndex >= 0 {
			intEnd = decimalIndex
		}
		if intEnd < start {
			return 0, false
		}
		intSeg := rs[start:intEnd]
		if len(intSeg) == 0 {
			return 0, false
		}
		if !numericIntegerGroupsValid(intSeg, loc.GroupSep, loc.GroupSizes) {
			return 0, false
		}
	}
	number, err := strconv.ParseFloat(string(normalized), 64)
	if err != nil {
		return 0, false
	}
	return number, true
}

// numericClamp clamps value between optional min and max. Unset
// Opt means unbounded.
func numericClamp(value float64, minVal, maxVal Opt[float64]) float64 {
	lo := math.Inf(-1)
	hi := math.Inf(1)
	if v, ok := minVal.Value(); ok {
		lo = v
	}
	if v, ok := maxVal.Value(); ok {
		hi = v
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	if value < lo {
		return lo
	}
	if value > hi {
		return hi
	}
	return value
}

// --- mode-aware functions ---

func numericModeToDisplay(value float64, mc numericModeCfg) float64 {
	return value * mc.displayMultiplier
}

func numericModeFromDisplay(value float64, mc numericModeCfg) float64 {
	if mc.displayMultiplier == 0 {
		return value
	}
	return value / mc.displayMultiplier
}

func numericModeStepDelta(stepDisplay float64, mc numericModeCfg) float64 {
	return numericModeFromDisplay(stepDisplay, mc)
}

func numericStripAffix(raw string, loc NumericLocaleCfg, mc numericModeCfg) (string, bool) {
	text := strings.TrimSpace(raw)
	if len(text) == 0 {
		return "", false
	}
	sign := ""
	minus := string(loc.MinusSign)
	plus := string(loc.PlusSign)
	if len(minus) > 0 && strings.HasPrefix(text, minus) {
		sign = minus
		text = strings.TrimLeft(text[len(minus):], " \t")
	} else if len(plus) > 0 && strings.HasPrefix(text, plus) {
		sign = plus
		text = strings.TrimLeft(text[len(plus):], " \t")
	}
	if len(mc.affix) > 0 {
		switch mc.affixPosition {
		case AffixPrefix:
			if strings.HasPrefix(text, mc.affix) {
				text = strings.TrimLeft(text[len(mc.affix):], " \t")
			}
		case AffixSuffix:
			right := strings.TrimRight(text, " \t")
			if strings.HasSuffix(right, mc.affix) {
				right = strings.TrimRight(right[:len(right)-len(mc.affix)], " \t")
				text = right
			}
		}
	}
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return "", false
	}
	return sign + text, true
}

func numericApplyAffix(formatted string, loc NumericLocaleCfg, mc numericModeCfg) string {
	if len(mc.affix) == 0 {
		return formatted
	}
	minus := string(loc.MinusSign)
	plus := string(loc.PlusSign)
	sign := ""
	number := formatted
	if len(minus) > 0 && strings.HasPrefix(number, minus) {
		sign = minus
		number = number[len(minus):]
	} else if len(plus) > 0 && strings.HasPrefix(number, plus) {
		sign = plus
		number = number[len(plus):]
	}
	space := ""
	if mc.affixSpacing && len(number) > 0 {
		space = " "
	}
	switch mc.affixPosition {
	case AffixPrefix:
		return sign + mc.affix + space + number
	case AffixSuffix:
		return sign + number + space + mc.affix
	}
	return formatted
}

func numericModeParseValue(raw string, decimals int, loc NumericLocaleCfg, mc numericModeCfg) (float64, bool) {
	plain, ok := numericStripAffix(strings.TrimSpace(raw), loc, mc)
	if !ok {
		return 0, false
	}
	parsed, ok := numericParse(plain, loc)
	if !ok {
		return 0, false
	}
	displayValue := numericRoundToDecimals(parsed, decimals)
	return numericModeFromDisplay(displayValue, mc), true
}

func numericModeIsTransientInput(raw string, decimals int, loc NumericLocaleCfg, mc numericModeCfg) bool {
	text := strings.TrimSpace(raw)
	if len(text) == 0 {
		return true
	}
	minus := string(loc.MinusSign)
	plus := string(loc.PlusSign)
	if len(minus) > 0 && text == minus {
		return true
	}
	if len(plus) > 0 && text == plus {
		return true
	}
	if len(minus) > 0 && strings.HasPrefix(text, minus) {
		text = strings.TrimLeft(text[len(minus):], " \t")
	} else if len(plus) > 0 && strings.HasPrefix(text, plus) {
		text = strings.TrimLeft(text[len(plus):], " \t")
	}
	if len(text) == 0 {
		return true
	}
	if len(mc.affix) > 0 {
		switch mc.affixPosition {
		case AffixPrefix:
			if text == mc.affix {
				return true
			}
			if strings.HasPrefix(text, mc.affix) {
				text = strings.TrimLeft(text[len(mc.affix):], " \t")
				if len(text) == 0 {
					return true
				}
			}
		case AffixSuffix:
			if text == mc.affix {
				return true
			}
			right := strings.TrimRight(text, " \t")
			if strings.HasSuffix(right, mc.affix) {
				right = strings.TrimRight(
					right[:len(right)-len(mc.affix)], " \t")
				text = right
				if len(text) == 0 {
					return true
				}
			}
		}
	}
	if decimals <= 0 {
		return false
	}
	decSep := string(loc.DecimalSep)
	if len(decSep) == 0 {
		return false
	}
	if text == decSep {
		return true
	}
	if !strings.HasSuffix(text, decSep) {
		return false
	}
	prefix := text[:len(text)-len(decSep)]
	if len(prefix) == 0 {
		return true
	}
	if _, ok := numericParse(prefix, loc); ok {
		return true
	}
	return false
}

func numericModeFormatValue(value float64, decimals int, loc NumericLocaleCfg, mc numericModeCfg) string {
	displayValue := numericModeToDisplay(value, mc)
	displayValueRounded := numericRoundToDecimals(displayValue, decimals)
	formatted := numericFormatValue(displayValueRounded, decimals, loc)
	return numericApplyAffix(formatted, loc, mc)
}

func numericStepDelta(cfg NumericStepCfg, modifiers Modifier) float64 {
	step := cfg.Step
	if modifiers.Has(ModShift) {
		step *= cfg.ShiftMultiplier
	}
	if modifiers.Has(ModAlt) {
		step *= cfg.AltMultiplier
	}
	if step < 0 {
		return -step
	}
	return step
}

func numericStepSeedMode(text string, value, minVal Opt[float64], decimals int, loc NumericLocaleCfg, mc numericModeCfg) float64 {
	if v, ok := value.Value(); ok {
		return v
	}
	if parsed, ok := numericModeParseValue(text, decimals, loc, mc); ok {
		return parsed
	}
	if v, ok := minVal.Value(); ok {
		return v
	}
	return 0.0
}

func numericInputCommitResultMode(text string, value, minVal, maxVal Opt[float64], decimals int, locale NumericLocaleCfg, mc numericModeCfg) (Opt[float64], string) {
	loc := numericLocaleNormalize(locale)
	trimmed := strings.TrimSpace(text)
	if len(trimmed) == 0 {
		return Opt[float64]{}, ""
	}
	if parsed, ok := numericModeParseValue(trimmed, decimals, loc, mc); ok {
		clamped := numericClamp(parsed, minVal, maxVal)
		return Some(clamped), numericModeFormatValue(clamped, decimals, loc, mc)
	}
	if v, ok := value.Value(); ok {
		clamped := numericClamp(v, minVal, maxVal)
		return Some(clamped), numericModeFormatValue(clamped, decimals, loc, mc)
	}
	return Opt[float64]{}, ""
}

func numericInputStepResultMode(text string, value, minVal, maxVal Opt[float64], decimals int, stepCfg NumericStepCfg, locale NumericLocaleCfg, direction float64, modifiers Modifier, mc numericModeCfg) (Opt[float64], string) {
	loc := numericLocaleNormalize(locale)
	if direction == 0 {
		return numericInputCommitResultMode(text, value, minVal, maxVal, decimals, loc, mc)
	}
	normalized := numericStepCfgNormalize(stepCfg)
	stepDisplay := numericStepDelta(normalized, modifiers)
	delta := numericModeStepDelta(stepDisplay, mc)
	seed := numericStepSeedMode(text, value, minVal, decimals, loc, mc)
	clamped := numericClamp(seed+(delta*direction), minVal, maxVal)
	return Some(clamped), numericModeFormatValue(clamped, decimals, loc, mc)
}

func numericInputPreCommitTransformMode(current, proposed string, decimals int, locale NumericLocaleCfg, mc numericModeCfg) (string, bool) {
	if proposed == current {
		return proposed, true
	}
	trimmed := strings.TrimSpace(proposed)
	if len(trimmed) == 0 {
		return "", true
	}
	loc := numericLocaleNormalize(locale)
	// Reject decimal separator when no decimals allowed.
	if decimals <= 0 && strings.ContainsRune(trimmed, loc.DecimalSep) {
		return "", false
	}
	// Try parsing as-is first (handles already-valid formatted text).
	if _, ok := numericModeParseValue(trimmed, decimals, loc, mc); ok {
		return proposed, true
	}
	// Strip group separators for lenient editing — allows typing
	// in fields that contain formatted numbers without strict
	// group validation blocking mid-edit keystrokes.
	if loc.GroupSep != 0 {
		stripped := numericStripGroupSep(trimmed, loc.GroupSep)
		if stripped != trimmed {
			if _, ok := numericModeParseValue(stripped, decimals, loc, mc); ok {
				return proposed, true
			}
		}
	}
	if numericModeIsTransientInput(proposed, decimals, loc, mc) {
		return proposed, true
	}
	// Also check transient with group separators stripped.
	if loc.GroupSep != 0 {
		stripped := numericStripGroupSep(proposed, loc.GroupSep)
		if stripped != proposed && numericModeIsTransientInput(stripped, decimals, loc, mc) {
			return proposed, true
		}
	}
	return "", false
}

// numericStripGroupSep removes all group separator runes from s.
func numericStripGroupSep(s string, groupSep rune) string {
	return strings.Map(func(r rune) rune {
		if r == groupSep {
			return -1
		}
		return r
	}, s)
}
