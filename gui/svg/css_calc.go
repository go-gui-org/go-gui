package svg

import (
	"math"
	"strconv"
	"strings"
	"unicode"
)

// resolveCalcRefs evaluates `calc(...)` expressions in v at parse time,
// substituting the numeric literal result. Supports operators
// `+ - * /`, parenthesized subexpressions, units `px` and unitless.
// Mixed-unit operands are rejected per spec strictness (e.g.
// `calc(10px + 50%)` returns "" → caller drops the declaration).
//
// On any parse / type error the whole value is dropped (empty string)
// so a malformed calc() invalidates the declaration just as the spec
// requires for invalid-at-computed-value-time.
//
// Nested calc() and calc() inside var() fallback are resolved
// recursively. Resolution depth is bounded by maxCalcRecursion.
func resolveCalcRefs(v string) string {
	if !strings.Contains(v, "calc(") {
		return v
	}
	return resolveCalcAt(v, 0)
}

const maxCalcRecursion = 16

func resolveCalcAt(v string, depth int) string {
	if !strings.Contains(v, "calc(") {
		return v
	}
	if depth >= maxCalcRecursion {
		return ""
	}
	var b strings.Builder
	i := 0
	for i < len(v) {
		idx := strings.Index(v[i:], "calc(")
		if idx < 0 {
			b.WriteString(v[i:])
			break
		}
		b.WriteString(v[i : i+idx])
		argStart := i + idx + len("calc(")
		end, ok := findClosingParen(v, argStart)
		if !ok {
			return ""
		}
		// Recurse into nested calc() inside this body before
		// evaluating. An empty result here means a nested calc() was
		// malformed; propagate the failure up.
		raw := v[argStart:end]
		body := resolveCalcAt(raw, depth+1)
		if body == "" && raw != "" {
			return ""
		}
		val, ok := evalCalc(body)
		if !ok {
			return ""
		}
		b.WriteString(val)
		i = end + 1
	}
	return b.String()
}

// calcUnit identifies the unit attached to a numeric value in a calc()
// expression. Mixing values with different units across `+`/`-` is a
// spec error; `*` and `/` may multiply unitless against unit values.
type calcUnit uint8

const (
	calcUnitNone calcUnit = iota
	calcUnitPx
)

type calcValue struct {
	num  float64
	unit calcUnit
}

// evalCalc parses and evaluates a calc() body (no surrounding "calc(",
// ")"). Returns the numeric literal in the result's natural unit, or
// ok=false on any parse / type error.
func evalCalc(body string) (string, bool) {
	toks, ok := tokenizeCalc(body)
	if !ok {
		return "", false
	}
	rpn, ok := shuntCalc(toks)
	if !ok {
		return "", false
	}
	res, ok := evalCalcRPN(rpn)
	if !ok {
		return "", false
	}
	out := formatCalcValue(res)
	if out == "" {
		return "", false
	}
	return out, true
}

// calcTok is one token in a calc() body.
type calcTok struct {
	kind byte // 'n' = number, '+' '-' '*' '/' '(' ')'
	val  calcValue
}

func tokenizeCalc(s string) ([]calcTok, bool) {
	var out []calcTok
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '+' || c == '-':
			// Unary context: preceding token is another operator, '(',
			// or there is no preceding token. In that case consume the
			// sign with the following number rather than emitting it
			// as a binary operator. Spec allows `calc(-5px)` and
			// `calc(2 * -3px)`.
			if isUnaryContext(out) {
				v, adv, ok := parseSignedNumber(s, i)
				if !ok {
					return nil, false
				}
				out = append(out, calcTok{kind: 'n', val: v})
				i = adv
				continue
			}
			out = append(out, calcTok{kind: c})
			i++
		case c == '*' || c == '/' || c == '(' || c == ')':
			out = append(out, calcTok{kind: c})
			i++
		case c == '.' || (c >= '0' && c <= '9'):
			v, adv, ok := parseSignedNumber(s, i)
			if !ok {
				return nil, false
			}
			out = append(out, calcTok{kind: 'n', val: v})
			i = adv
		default:
			return nil, false
		}
	}
	return out, true
}

// isUnaryContext reports whether the next `+`/`-` should be consumed
// as the sign of a numeric literal rather than emitted as a binary
// operator. True when the prior token is an operator, '(', or there
// is no prior token.
func isUnaryContext(out []calcTok) bool {
	if len(out) == 0 {
		return true
	}
	switch out[len(out)-1].kind {
	case '+', '-', '*', '/', '(':
		return true
	}
	return false
}

// parseSignedNumber consumes an optional leading `+`/`-`, a numeric
// literal, optional whitespace, and an optional unit suffix starting
// at s[i]. Returns the parsed value, the index past the consumed
// region, and ok=false on malformed input.
func parseSignedNumber(s string, i int) (calcValue, int, bool) {
	sign, i := consumeNumberSign(s, i)
	numStart := i
	j := consumeDigitsAndDot(s, i)
	if j == numStart {
		return calcValue{}, 0, false
	}
	j = consumeSciSuffix(s, j)
	num, err := strconv.ParseFloat(s[numStart:j], 64)
	if err != nil || math.IsInf(num, 0) || math.IsNaN(num) {
		return calcValue{}, 0, false
	}
	num *= sign
	unit, k, ok := consumeUnit(s, j)
	if !ok {
		return calcValue{}, 0, false
	}
	return calcValue{num: num, unit: unit}, k, true
}

func consumeNumberSign(s string, i int) (float64, int) {
	sign := 1.0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		if s[i] == '-' {
			sign = -1
		}
		i++
		for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
			i++
		}
	}
	return sign, i
}

func consumeDigitsAndDot(s string, i int) int {
	sawDot := false
	for i < len(s) {
		ch := s[i]
		if ch >= '0' && ch <= '9' {
			i++
			continue
		}
		if ch == '.' && !sawDot {
			sawDot = true
			i++
			continue
		}
		break
	}
	return i
}

// consumeSciSuffix disambiguates `1e10` (scientific) from `1em` (unit
// "em") by requiring at least one digit after the optional sign;
// `1em`'s `m` is not a digit, so the `e` falls through to the unit
// consumer.
func consumeSciSuffix(s string, j int) int {
	if j >= len(s) || (s[j] != 'e' && s[j] != 'E') {
		return j
	}
	k := j + 1
	if k < len(s) && (s[k] == '+' || s[k] == '-') {
		k++
	}
	if k >= len(s) || s[k] < '0' || s[k] > '9' {
		return j
	}
	for k < len(s) && s[k] >= '0' && s[k] <= '9' {
		k++
	}
	return k
}

func consumeUnit(s string, k int) (calcUnit, int, bool) {
	if k >= len(s) || !unicode.IsLetter(rune(s[k])) {
		return calcUnitNone, k, true
	}
	us := k
	for k < len(s) && unicode.IsLetter(rune(s[k])) {
		k++
	}
	if strings.ToLower(s[us:k]) == "px" {
		return calcUnitPx, k, true
	}
	return calcUnitNone, 0, false
}

// precedence yields operator precedence for shunting yard. Higher
// number = tighter binding.
func precedence(op byte) int {
	switch op {
	case '+', '-':
		return 1
	case '*', '/':
		return 2
	}
	return 0
}

func shuntCalc(toks []calcTok) ([]calcTok, bool) {
	var out []calcTok
	var ops []calcTok
	for _, t := range toks {
		switch t.kind {
		case 'n':
			out = append(out, t)
		case '+', '-', '*', '/':
			for len(ops) > 0 {
				top := ops[len(ops)-1]
				if top.kind == '(' {
					break
				}
				if precedence(top.kind) < precedence(t.kind) {
					break
				}
				out = append(out, top)
				ops = ops[:len(ops)-1]
			}
			ops = append(ops, t)
		case '(':
			ops = append(ops, t)
		case ')':
			matched := false
			for len(ops) > 0 {
				top := ops[len(ops)-1]
				ops = ops[:len(ops)-1]
				if top.kind == '(' {
					matched = true
					break
				}
				out = append(out, top)
			}
			if !matched {
				return nil, false
			}
		}
	}
	for len(ops) > 0 {
		top := ops[len(ops)-1]
		ops = ops[:len(ops)-1]
		if top.kind == '(' {
			return nil, false
		}
		out = append(out, top)
	}
	return out, true
}

func evalCalcRPN(rpn []calcTok) (calcValue, bool) {
	var stack []calcValue
	for _, t := range rpn {
		if t.kind == 'n' {
			stack = append(stack, t.val)
			continue
		}
		if len(stack) < 2 {
			return calcValue{}, false
		}
		b := stack[len(stack)-1]
		a := stack[len(stack)-2]
		stack = stack[:len(stack)-2]
		res, ok := applyCalcOp(t.kind, a, b)
		if !ok {
			return calcValue{}, false
		}
		stack = append(stack, res)
	}
	if len(stack) != 1 {
		return calcValue{}, false
	}
	return stack[0], true
}

// applyCalcOp evaluates a single binary op between two calcValues,
// enforcing CSS calc unit rules (matching units for `+`/`-`, at most
// one operand carrying a unit for `*`, unitless divisor for `/`) and
// rejecting any non-finite result so an arithmetic overflow cannot
// emit `+Inf`/`NaN` as the substituted CSS literal.
func applyCalcOp(op byte, a, b calcValue) (calcValue, bool) {
	var res calcValue
	switch op {
	case '+', '-':
		if a.unit != b.unit {
			return calcValue{}, false
		}
		if op == '+' {
			res = calcValue{num: a.num + b.num, unit: a.unit}
		} else {
			res = calcValue{num: a.num - b.num, unit: a.unit}
		}
	case '*':
		// At least one side must be unitless; result inherits the
		// other side's unit.
		switch {
		case a.unit == calcUnitNone:
			res = calcValue{num: a.num * b.num, unit: b.unit}
		case b.unit == calcUnitNone:
			res = calcValue{num: a.num * b.num, unit: a.unit}
		default:
			return calcValue{}, false
		}
	case '/':
		// Right side must be unitless per spec.
		if b.num == 0 || b.unit != calcUnitNone {
			return calcValue{}, false
		}
		res = calcValue{num: a.num / b.num, unit: a.unit}
	default:
		return calcValue{}, false
	}
	if math.IsInf(res.num, 0) || math.IsNaN(res.num) {
		return calcValue{}, false
	}
	return res, true
}

// formatCalcValue renders a calcValue back as a CSS literal.
// Trailing zeros and unnecessary decimals are stripped so the result
// reads like an authored literal (e.g. "14px" not "14.000000px").
// Returns "" for non-finite inputs as a final guard — applyCalcOp
// rejects upstream, but defensive zeroing keeps the substituted
// CSS string from ever carrying "+Inf" or "NaN".
func formatCalcValue(v calcValue) string {
	if math.IsInf(v.num, 0) || math.IsNaN(v.num) {
		return ""
	}
	s := strconv.FormatFloat(v.num, 'f', -1, 64)
	if v.unit == calcUnitPx {
		return s + "px"
	}
	return s
}
