package markdown

// walker_postprocess.go — post-parse transforms: footnote
// references, abbreviations, typography, run merging.

import (
	"cmp"
	"slices"
	"strings"
)

// applyFootnoteRefs replaces [^id] patterns in run text
// with superscript tooltip runs.
func applyFootnoteRefs(
	runs []Run, defs map[string]string,
) []Run {
	if len(defs) == 0 {
		return runs
	}
	match := footnoteMatchFunc(defs)
	result := make([]Run, 0, len(runs))
	changed := false
	for _, run := range runs {
		if run.Link != "" || run.Tooltip != "" ||
			run.MathID != "" {
			result = append(result, run)
			continue
		}
		nr, split := splitRunByMatches(run, match)
		if split {
			changed = true
			result = append(result, nr...)
			continue
		}
		result = append(result, run)
	}
	if !changed {
		return runs
	}
	return result
}

// runMatchFunc finds the next match at or after pos in text.
// Returns start/end indices, the replacement run, and whether
// a match was found.
type runMatchFunc func(
	text string, pos int, base Run,
) (start, end int, replacement Run, found bool)

// splitRunByMatches splits a run using match to find and
// replace substrings. Returns nil, false if no matches.
func splitRunByMatches(
	run Run, match runMatchFunc,
) ([]Run, bool) {
	t := run.Text
	if len(t) == 0 {
		return nil, false
	}
	var result []Run
	pos := 0
	lastPos := 0
	split := false
	for pos < len(t) {
		start, end, repl, found := match(t, pos, run)
		if !found {
			break
		}
		if !split {
			result = make([]Run, 0, 3)
			split = true
		}
		if start > lastPos {
			r := run
			r.Text = t[lastPos:start]
			result = append(result, r)
		}
		result = append(result, repl)
		lastPos = end
		pos = end
	}
	if !split {
		return nil, false
	}
	if lastPos < len(t) {
		r := run
		r.Text = t[lastPos:]
		result = append(result, r)
	}
	return result, true
}

func footnoteMatchFunc(
	defs map[string]string,
) runMatchFunc {
	return func(
		text string, pos int, base Run,
	) (int, int, Run, bool) {
		for pos < len(text) {
			idx := strings.Index(text[pos:], "[^")
			if idx < 0 {
				return 0, 0, Run{}, false
			}
			start := pos + idx
			end := strings.Index(text[start+2:], "]")
			if end < 0 {
				pos = start + 2
				continue
			}
			end = start + 2 + end
			id := text[start+2 : end]
			content, ok := defs[id]
			if !ok {
				pos = end + 1
				continue
			}
			return start, end + 1, Run{
				Text:        id,
				Format:      base.Format,
				Superscript: true,
				Tooltip:     content,
			}, true
		}
		return 0, 0, Run{}, false
	}
}

func abbrMatchFunc(matcher *abbrMatcher) runMatchFunc {
	return func(
		text string, pos int, base Run,
	) (int, int, Run, bool) {
		for pos < len(text) {
			if !matcher.firstChars[text[pos]] {
				pos++
				continue
			}
			for _, abbr := range matcher.abbrs {
				if pos+len(abbr) > len(text) ||
					text[pos:pos+len(abbr)] != abbr {
					continue
				}
				if !isWordBoundary(text, pos-1) ||
					!isWordBoundary(text, pos+len(abbr)) {
					continue
				}
				return pos, pos + len(abbr), Run{
					Text:          abbr,
					Format:        base.Format,
					Strikethrough: base.Strikethrough,
					Highlight:     base.Highlight,
					Superscript:   base.Superscript,
					Subscript:     base.Subscript,
					Tooltip:       matcher.defs[abbr],
				}, true
			}
			pos++
		}
		return 0, 0, Run{}, false
	}
}

// replaceAbbreviations scans runs for abbreviation occurrences
// and splits/marks them with tooltips.
func replaceAbbreviations(
	runs []Run, matcher *abbrMatcher,
) []Run {
	if matcher == nil {
		return runs
	}
	match := abbrMatchFunc(matcher)
	result := make([]Run, 0, len(runs))
	for _, run := range runs {
		if run.Link != "" || run.Tooltip != "" ||
			run.MathID != "" {
			result = append(result, run)
			continue
		}
		nr, split := splitRunByMatches(run, match)
		if split {
			result = append(result, nr...)
			continue
		}
		result = append(result, run)
	}
	return result
}

func buildAbbrMatcher(defs map[string]string) *abbrMatcher {
	if len(defs) == 0 {
		return nil
	}
	abbrs := make([]string, 0, len(defs))
	var firstChars [256]bool
	for k := range defs {
		if len(k) == 0 {
			continue
		}
		abbrs = append(abbrs, k)
		firstChars[k[0]] = true
	}
	slices.SortFunc(abbrs, func(a, b string) int {
		return cmp.Compare(len(b), len(a))
	})
	return &abbrMatcher{
		abbrs:      abbrs,
		firstChars: firstChars,
		defs:       defs,
	}
}

func isWordBoundary(text string, pos int) bool {
	if pos < 0 || pos >= len(text) {
		return true
	}
	c := text[pos]
	return (c < 'a' || c > 'z') &&
		(c < 'A' || c > 'Z') &&
		(c < '0' || c > '9') && c != '_'
}

// --- Helpers ---

// mergeAdjacentRuns combines consecutive runs with identical
// formatting into single runs. Needed because goldmark may
// split text across multiple Text nodes (e.g. [^1] becomes
// "[" and "^1]" separately).
func mergeAdjacentRuns(runs []Run) []Run {
	if len(runs) <= 1 {
		return runs
	}
	result := make([]Run, 0, len(runs))
	cur := runs[0]
	var sb strings.Builder
	merging := false
	for _, r := range runs[1:] {
		if canMergeRuns(cur, r) {
			if !merging {
				sb.WriteString(cur.Text)
				merging = true
			}
			sb.WriteString(r.Text)
		} else {
			if merging {
				cur.Text = sb.String()
				sb.Reset()
				merging = false
			}
			result = append(result, cur)
			cur = r
		}
	}
	if merging {
		cur.Text = sb.String()
	}
	result = append(result, cur)
	return result
}

func canMergeRuns(a, b Run) bool {
	return a.Format == b.Format &&
		a.Strikethrough == b.Strikethrough &&
		a.Highlight == b.Highlight &&
		a.Superscript == b.Superscript &&
		a.Subscript == b.Subscript &&
		a.Link == b.Link &&
		a.Tooltip == b.Tooltip &&
		a.MathID == "" && b.MathID == "" &&
		a.CodeToken == b.CodeToken &&
		a.Underline == b.Underline
}

// applyTypography replaces --- with em dash, -- with en dash,
// and ... with ellipsis in non-code runs.
// Must replace --- before --.
func applyTypography(runs []Run) {
	for i := range runs {
		if runs[i].Format == FormatCode || runs[i].MathID != "" {
			continue
		}
		t := runs[i].Text
		if !strings.Contains(t, "--") && !strings.Contains(t, "...") {
			continue
		}
		t = strings.ReplaceAll(t, "---", "\u2014")
		t = strings.ReplaceAll(t, "--", "\u2013")
		t = strings.ReplaceAll(t, "...", "\u2026")
		runs[i].Text = t
	}
}

func trimTrailingBreaks(runs []Run) []Run {
	for len(runs) > 0 &&
		runs[len(runs)-1].Text == "\n" &&
		runs[len(runs)-1].Link == "" {
		runs = runs[:len(runs)-1]
	}
	return runs
}

// RunsToText concatenates run text into a single string.
func runsToText(runs []Run) string {
	var sb strings.Builder
	for _, r := range runs {
		sb.WriteString(r.Text)
	}
	return sb.String()
}

// parseImageSrc parses dimension suffixes from image URLs.
// Handles both "path.png =WxH" (original) and
// "path.png#dim=WxH" (preprocessed for goldmark).
func parseImageSrc(raw string) (string, float32, float32) {
	raw = strings.TrimSpace(raw)
	// Check preprocessed fragment form first.
	if idx := strings.LastIndex(raw, "#dim="); idx >= 0 {
		src := raw[:idx]
		dims := raw[idx+5:]
		if w, h, ok := parseDims(dims); ok {
			return src, w, h
		}
	}
	// Original space-separated form.
	if idx := strings.LastIndex(raw, " ="); idx >= 0 {
		dims := raw[idx+2:]
		if w, h, ok := parseDims(dims); ok {
			return strings.TrimSpace(raw[:idx]), w, h
		}
	}
	return raw, 0, 0
}

func parseDims(s string) (float32, float32, bool) {
	before, after, found := strings.Cut(s, "x")
	if !found {
		return 0, 0, false
	}
	w := parseFloat32(before)
	h := parseFloat32(after)
	return w, h, w > 0 || h > 0
}

func parseFloat32(s string) float32 {
	var v float32
	for _, c := range s {
		if c >= '0' && c <= '9' {
			v = v*10 + float32(c-'0')
		} else {
			return 0
		}
	}
	return v
}

// MathHash computes a FNV-1a hash of a string.
func MathHash(s string) uint64 {
	h := uint64(14695981039346656037) // FNV offset basis
	for i := range len(s) {
		h ^= uint64(s[i])
		h *= 1099511628211 // FNV prime
	}
	return h
}
