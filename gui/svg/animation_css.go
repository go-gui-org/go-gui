package svg

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
)

// applyCSSAnimProp folds one animation-* CSS declaration into spec.
// Recognized: animation-name, animation-duration, animation-delay,
// animation-iteration-count, animation-direction, animation-fill-
// mode, animation-timing-function, plus the `animation:` shorthand.
// Unknown sub-properties are dropped.
func applyCSSAnimProp(name, value string, spec *cssAnimSpec) bool {
	v := strings.TrimSpace(value)
	if v == "" {
		return false
	}
	switch name {
	case "animation-name":
		if v == "none" {
			spec.Name = ""
			return true
		}
		spec.Name = v
	case "animation-duration":
		spec.DurationSec = parseTimeValue(v)
	case "animation-delay":
		spec.DelaySec = parseTimeValue(v)
	case "animation-iteration-count":
		spec.IterCount = parseIterCount(v)
		spec.IterCountSet = true
	case "animation-direction":
		spec.Direction = parseAnimDirection(v)
	case "animation-fill-mode":
		spec.FillMode = parseAnimFillMode(v)
	case "animation-timing-function":
		applyTimingFunction(v, spec)
	case "animation":
		// CSS shorthand semantics: any sub-property not set in the
		// shorthand resets to its initial value. Without this reset,
		// a later `animation: none` cascading over an earlier
		// `animation: spin 1s` would leak Name="spin" because "none"
		// is interpreted as the name slot of an empty spec only.
		*spec = cssAnimSpec{}
		applyAnimShorthand(v, spec)
	default:
		return false
	}
	return true
}

func parseIterCount(s string) uint16 {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "infinite" {
		return gui.SvgAnimIterInfinite
	}
	n := parseFloatTrimmed(s)
	if n <= 0 {
		return 1
	}
	if n >= float32(gui.SvgAnimIterInfinite) {
		return gui.SvgAnimIterInfinite - 1
	}
	if n < 1 {
		// CSS allows fractional counts; round up to nearest int —
		// the engine quantizes iterations and a partial last cycle
		// is approximated by the freeze fallback.
		return 1
	}
	return uint16(n)
}

func parseAnimDirection(s string) cssAnimDir {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "reverse":
		return cssAnimDirReverse
	case "alternate":
		return cssAnimDirAlternate
	case "alternate-reverse":
		return cssAnimDirAlternateReverse
	}
	return cssAnimDirNormal
}

func parseAnimFillMode(s string) cssAnimFill {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "forwards":
		return cssAnimFillForwards
	case "backwards":
		return cssAnimFillBackwards
	case "both":
		return cssAnimFillBoth
	}
	return cssAnimFillNone
}

// applyTimingFunction parses linear / ease* keywords plus
// cubic-bezier(...) / steps(...). Unknown timing fall back to
// linear.
func applyTimingFunction(v string, spec *cssAnimSpec) {
	v = strings.TrimSpace(v)
	lower := strings.ToLower(v)
	switch lower {
	case "linear":
		spec.TimingFn = cssAnimTimingLinear
		return
	case "ease":
		spec.TimingFn = cssAnimTimingCubic
		spec.TimingArgs = [4]float32{0.25, 0.1, 0.25, 1.0}
		return
	case "ease-in":
		spec.TimingFn = cssAnimTimingCubic
		spec.TimingArgs = [4]float32{0.42, 0.0, 1.0, 1.0}
		return
	case "ease-out":
		spec.TimingFn = cssAnimTimingCubic
		spec.TimingArgs = [4]float32{0.0, 0.0, 0.58, 1.0}
		return
	case "ease-in-out":
		spec.TimingFn = cssAnimTimingCubic
		spec.TimingArgs = [4]float32{0.42, 0.0, 0.58, 1.0}
		return
	case "step-start":
		spec.TimingFn = cssAnimTimingSteps
		spec.StepsCount = 1
		spec.StepsAtStart = true
		return
	case "step-end":
		spec.TimingFn = cssAnimTimingSteps
		spec.StepsCount = 1
		return
	}
	if strings.HasPrefix(lower, "cubic-bezier(") &&
		strings.HasSuffix(lower, ")") {
		args := parseCSSFnArgs(v[len("cubic-bezier(") : len(v)-1])
		if len(args) == 4 {
			spec.TimingFn = cssAnimTimingCubic
			copy(spec.TimingArgs[:], args)
			return
		}
	}
	if strings.HasPrefix(lower, "steps(") &&
		strings.HasSuffix(lower, ")") {
		args := strings.Split(v[len("steps("):len(v)-1], ",")
		if len(args) >= 1 {
			n := uint16(parseFloatTrimmed(args[0]))
			if n == 0 {
				n = 1
			}
			spec.TimingFn = cssAnimTimingSteps
			spec.StepsCount = n
			if len(args) >= 2 {
				kw := strings.ToLower(strings.TrimSpace(args[1]))
				spec.StepsAtStart = kw == "start" || kw == "jump-start"
			}
			return
		}
	}
	// Unknown timing keyword: leave default.
	spec.TimingFn = cssAnimTimingLinear
}

func parseCSSFnArgs(s string) []float32 {
	parts := strings.Split(s, ",")
	out := make([]float32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, parseFloatTrimmed(p))
	}
	return out
}

// applyAnimShorthand parses the `animation:` shorthand. CSS spec
// permits any order of: <time>, <time>, <timing-function>,
// <iteration-count>, <direction>, <fill-mode>, <name>. Fields after
// a function token (cubic-bezier(...)) parse as a single token.
func applyAnimShorthand(v string, spec *cssAnimSpec) {
	tokens := splitShorthandTokens(v)
	timeSlots := 0
	for _, tok := range tokens {
		switch {
		case isCSSTimeToken(tok) && timeSlots < 2:
			if timeSlots == 0 {
				spec.DurationSec = parseTimeValue(tok)
			} else {
				spec.DelaySec = parseTimeValue(tok)
			}
			timeSlots++
		case isTimingFnToken(tok):
			applyTimingFunction(tok, spec)
		case isIterCountToken(tok):
			spec.IterCount = parseIterCount(tok)
			spec.IterCountSet = true
		case isAnimDirectionToken(tok):
			spec.Direction = parseAnimDirection(tok)
		case isAnimFillModeToken(tok):
			spec.FillMode = parseAnimFillMode(tok)
		default:
			// First unrecognized ident becomes the name.
			if spec.Name == "" {
				spec.Name = tok
			}
		}
	}
}

// splitShorthandTokens splits on whitespace but preserves nested
// parens so cubic-bezier(0,0,1,1) stays as one token.
func splitShorthandTokens(s string) []string {
	var out []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			if depth > 0 {
				depth--
			}
		case ' ', '\t', '\n', '\r':
			if depth == 0 {
				if i > start {
					out = append(out, s[start:i])
				}
				start = i + 1
			}
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

func isCSSTimeToken(s string) bool {
	if !strings.HasSuffix(s, "ms") && !strings.HasSuffix(s, "s") {
		return false
	}
	if s == "" {
		return false
	}
	c := s[0]
	return (c >= '0' && c <= '9') || c == '.' || c == '+' || c == '-'
}

func isTimingFnToken(s string) bool {
	switch strings.ToLower(s) {
	case "linear", "ease", "ease-in", "ease-out", "ease-in-out",
		"step-start", "step-end":
		return true
	}
	lower := strings.ToLower(s)
	return strings.HasPrefix(lower, "cubic-bezier(") ||
		strings.HasPrefix(lower, "steps(")
}

func isIterCountToken(s string) bool {
	if strings.EqualFold(s, "infinite") {
		return true
	}
	if s == "" {
		return false
	}
	c := s[0]
	return (c >= '0' && c <= '9') || c == '.'
}

func isAnimDirectionToken(s string) bool {
	switch strings.ToLower(s) {
	case "normal", "reverse", "alternate", "alternate-reverse":
		return true
	}
	return false
}

func isAnimFillModeToken(s string) bool {
	switch strings.ToLower(s) {
	case "none", "forwards", "backwards", "both":
		return true
	}
	return false
}
