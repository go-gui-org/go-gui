package markdown

import (
	"strings"
	"testing"
)

// --- mergeFormat ---

func TestMergeFormatPlainChild(t *testing.T) {
	t.Parallel()
	tests := []struct {
		parent, child, want Format
	}{
		{FormatPlain, FormatPlain, FormatPlain},
		{FormatPlain, FormatBold, FormatBold},
		{FormatPlain, FormatItalic, FormatItalic},
		{FormatPlain, FormatBoldItalic, FormatBoldItalic},
		{FormatPlain, FormatCode, FormatCode},
	}
	for _, tc := range tests {
		got := mergeFormat(tc.parent, tc.child)
		if got != tc.want {
			t.Errorf("mergeFormat(%d,%d)=%d, want %d",
				tc.parent, tc.child, got, tc.want)
		}
	}
}

func TestMergeFormatBoldCombinations(t *testing.T) {
	t.Parallel()
	// Bold + Italic → BoldItalic.
	if mergeFormat(FormatBold, FormatItalic) != FormatBoldItalic {
		t.Error("Bold+Italic should be BoldItalic")
	}
	// Bold + BoldItalic → BoldItalic.
	if mergeFormat(FormatBold, FormatBoldItalic) != FormatBoldItalic {
		t.Error("Bold+BoldItalic should be BoldItalic")
	}
	// Bold + Bold → Bold.
	if mergeFormat(FormatBold, FormatBold) != FormatBold {
		t.Error("Bold+Bold should be Bold")
	}
	// Bold + Plain → Bold.
	if mergeFormat(FormatBold, FormatPlain) != FormatBold {
		t.Error("Bold+Plain should be Bold")
	}
}

func TestMergeFormatItalicCombinations(t *testing.T) {
	t.Parallel()
	if mergeFormat(FormatItalic, FormatBold) != FormatBoldItalic {
		t.Error("Italic+Bold should be BoldItalic")
	}
	if mergeFormat(FormatItalic, FormatBoldItalic) != FormatBoldItalic {
		t.Error("Italic+BoldItalic should be BoldItalic")
	}
	if mergeFormat(FormatItalic, FormatItalic) != FormatItalic {
		t.Error("Italic+Italic should be Italic")
	}
}

func TestMergeFormatBoldItalicStays(t *testing.T) {
	t.Parallel()
	// BoldItalic + anything (except Code) → BoldItalic.
	for _, child := range []Format{
		FormatPlain, FormatBold, FormatItalic, FormatBoldItalic,
	} {
		if mergeFormat(FormatBoldItalic, child) != FormatBoldItalic {
			t.Errorf("BoldItalic+%d should be BoldItalic", child)
		}
	}
}

func TestMergeFormatCodeWins(t *testing.T) {
	t.Parallel()
	// Code in either position wins.
	for _, other := range []Format{
		FormatPlain, FormatBold, FormatItalic, FormatBoldItalic,
	} {
		if mergeFormat(FormatCode, other) != FormatCode {
			t.Errorf("Code+%d should be Code", other)
		}
		if mergeFormat(other, FormatCode) != FormatCode {
			t.Errorf("%d+Code should be Code", other)
		}
	}
}

// --- scanSource preprocessing ---

func TestScanSourceAbbrStripped(t *testing.T) {
	t.Parallel()
	src := "Hello HTML.\n\n*[HTML]: HyperText Markup Language"
	result, _, _ := scanSource(src)
	if strings.Contains(result, "*[HTML]") {
		t.Error("abbreviation definition should be stripped")
	}
}

func TestScanSourceFootnoteStripped(t *testing.T) {
	t.Parallel()
	src := "Text[^1] more.\n\n[^1]: Footnote content."
	result, _, _ := scanSource(src)
	if strings.Contains(result, "[^1]: Footnote") {
		t.Error("footnote definition should be stripped")
	}
}

func TestScanSourceFootnoteMultiline(t *testing.T) {
	t.Parallel()
	src := "[^1]: First line.\n    Continuation.\n\nParagraph."
	result, _, _ := scanSource(src)
	if strings.Contains(result, "Continuation") {
		t.Error("footnote continuation should be stripped")
	}
	if !strings.Contains(result, "Paragraph") {
		t.Error("paragraph after footnote should remain")
	}
}

func TestScanSourceMathFence(t *testing.T) {
	t.Parallel()
	src := "$$\nE = mc^2\n$$"
	result, _, _ := scanSource(src)
	if !strings.Contains(result, "```math") {
		t.Error("$$ should convert to ```math fence")
	}
	if strings.Contains(result, "$$") {
		t.Error("$$ markers should be replaced")
	}
}

func TestScanSourceImageDims(t *testing.T) {
	t.Parallel()
	src := "![alt](image.png =200x100)"
	result, _, _ := scanSource(src)
	if !strings.Contains(result, "#dim=200x100") {
		t.Errorf("image dims not encoded: %q", result)
	}
	if strings.Contains(result, " =200x100") {
		t.Error("original space-separated dims should be removed")
	}
}

func TestScanSourcePassthrough(t *testing.T) {
	t.Parallel()
	src := "# Hello\n\nPlain paragraph."
	result, _, _ := scanSource(src)
	if result != src {
		t.Errorf("passthrough mismatch:\ngot  %q\nwant %q",
			result, src)
	}
}

// --- scanSource abbr defs ---

func TestScanSourceAbbrDefs(t *testing.T) {
	t.Parallel()
	src := "*[HTML]: HyperText Markup Language\n*[CSS]: Cascading Style Sheets"
	_, defs, _ := scanSource(src)
	if defs["HTML"] != "HyperText Markup Language" {
		t.Errorf("HTML abbr: %q", defs["HTML"])
	}
	if defs["CSS"] != "Cascading Style Sheets" {
		t.Errorf("CSS abbr: %q", defs["CSS"])
	}
}

func TestScanSourceAbbrDefsEmpty(t *testing.T) {
	t.Parallel()
	_, defs, _ := scanSource("No abbreviations here.")
	if len(defs) != 0 {
		t.Errorf("expected 0 abbr defs, got %d", len(defs))
	}
}

func TestScanSourceAbbrDefsMalformed(t *testing.T) {
	t.Parallel()
	_, defs, _ := scanSource("*[HTML HyperText")
	if len(defs) != 0 {
		t.Error("malformed abbr def should not be collected")
	}
}

func TestScanSourceAbbrDefsEmptyExpansion(t *testing.T) {
	t.Parallel()
	_, defs, _ := scanSource("*[HTML]:")
	if len(defs) != 0 {
		t.Error("empty expansion should not be collected")
	}
}

// --- scanSource footnote defs ---

func TestScanSourceFootnoteDefs(t *testing.T) {
	t.Parallel()
	src := "[^1]: First footnote.\n[^note]: Named footnote."
	_, _, defs := scanSource(src)
	if defs["1"] != "First footnote." {
		t.Errorf("footnote 1: %q", defs["1"])
	}
	if defs["note"] != "Named footnote." {
		t.Errorf("footnote note: %q", defs["note"])
	}
}

func TestScanSourceFootnoteDefsEmpty(t *testing.T) {
	t.Parallel()
	_, _, defs := scanSource("No footnotes.")
	if len(defs) != 0 {
		t.Errorf("expected 0 footnote defs, got %d", len(defs))
	}
}

func TestScanSourceFootnoteDefsContinuation(t *testing.T) {
	t.Parallel()
	src := "[^1]: Line one.\n    Line two."
	_, _, defs := scanSource(src)
	if !strings.Contains(defs["1"], "Line two") {
		t.Errorf("footnote should include continuation: %q",
			defs["1"])
	}
}

func TestScanSourceFootnoteDefsMalformed(t *testing.T) {
	t.Parallel()
	_, _, defs := scanSource("[^]: no id")
	if len(defs) != 0 {
		t.Error("malformed footnote should not be collected")
	}
}

// --- isAbbrDef ---

func TestIsAbbrDef(t *testing.T) {
	t.Parallel()
	if !isAbbrDef("*[HTML]: HyperText Markup Language") {
		t.Error("valid abbr def not recognized")
	}
	if isAbbrDef("*HTML]: not abbr") {
		t.Error("missing [ should not match")
	}
	if isAbbrDef("regular text") {
		t.Error("plain text should not match")
	}
}

// --- isFootnoteDef ---

func TestIsFootnoteDef(t *testing.T) {
	t.Parallel()
	if !isFootnoteDef("[^1]: Footnote content") {
		t.Error("valid footnote def not recognized")
	}
	if isFootnoteDef("[^]:") {
		t.Error("empty id should not match")
	}
	if isFootnoteDef("plain text") {
		t.Error("plain text should not match")
	}
	if isFootnoteDef("[1]: not a footnote") {
		t.Error("missing ^ should not match")
	}
}

// --- isWordBoundary ---

func TestIsWordBoundary(t *testing.T) {
	t.Parallel()
	// Before start and after end are boundaries.
	if !isWordBoundary("hello", -1) {
		t.Error("pos -1 should be boundary")
	}
	if !isWordBoundary("hello", 5) {
		t.Error("pos past end should be boundary")
	}
	// Space is boundary.
	if !isWordBoundary("a b", 1) {
		t.Error("space should be boundary")
	}
	// Letter is not boundary.
	if isWordBoundary("abc", 1) {
		t.Error("letter should not be boundary")
	}
	// Digit is not boundary.
	if isWordBoundary("a1b", 1) {
		t.Error("digit should not be boundary")
	}
	// Underscore is not boundary.
	if isWordBoundary("a_b", 1) {
		t.Error("underscore should not be boundary")
	}
	// Punctuation is boundary.
	if !isWordBoundary("a.b", 1) {
		t.Error("period should be boundary")
	}
}

// --- applyFootnoteRefs ---

func TestApplyFootnoteRefsBasic(t *testing.T) {
	t.Parallel()
	defs := map[string]string{"1": "Footnote text."}
	runs := []Run{{Text: "See[^1] here."}}
	result := applyFootnoteRefs(runs, defs)
	foundTooltip := false
	for _, r := range result {
		if r.Tooltip == "Footnote text." && r.Superscript {
			foundTooltip = true
		}
	}
	if !foundTooltip {
		t.Error("expected footnote tooltip in result")
	}
}

func TestApplyFootnoteRefsNoDefs(t *testing.T) {
	t.Parallel()
	runs := []Run{{Text: "No footnotes."}}
	result := applyFootnoteRefs(runs, nil)
	if len(result) != 1 || result[0].Text != "No footnotes." {
		t.Error("nil defs should return runs unchanged")
	}
}

func TestApplyFootnoteRefsUndefined(t *testing.T) {
	t.Parallel()
	defs := map[string]string{"1": "Footnote."}
	runs := []Run{{Text: "See[^2] here."}}
	result := applyFootnoteRefs(runs, defs)
	for _, r := range result {
		if r.Tooltip != "" {
			t.Error("undefined footnote should not have tooltip")
		}
	}
}

func TestApplyFootnoteRefsSkipsLinks(t *testing.T) {
	t.Parallel()
	defs := map[string]string{"1": "Footnote."}
	runs := []Run{{Text: "See[^1] here.", Link: "http://x"}}
	result := applyFootnoteRefs(runs, defs)
	// Link run should not be split.
	if len(result) != 1 {
		t.Errorf("link run should not be split, got %d runs",
			len(result))
	}
}

func TestApplyFootnoteRefsSkipsTooltip(t *testing.T) {
	t.Parallel()
	defs := map[string]string{"1": "Footnote."}
	runs := []Run{{Text: "See[^1].", Tooltip: "existing"}}
	result := applyFootnoteRefs(runs, defs)
	if len(result) != 1 {
		t.Error("run with existing tooltip should not be split")
	}
}

func TestApplyFootnoteRefsSkipsMath(t *testing.T) {
	t.Parallel()
	defs := map[string]string{"1": "Footnote."}
	runs := []Run{{Text: "See[^1].", MathID: "math_123"}}
	result := applyFootnoteRefs(runs, defs)
	if len(result) != 1 {
		t.Error("math run should not be split")
	}
}

func TestApplyFootnoteRefsMultiple(t *testing.T) {
	t.Parallel()
	defs := map[string]string{
		"1": "First.", "2": "Second.",
	}
	runs := []Run{{Text: "A[^1] and B[^2]."}}
	result := applyFootnoteRefs(runs, defs)
	tooltips := 0
	for _, r := range result {
		if r.Tooltip != "" {
			tooltips++
		}
	}
	if tooltips != 2 {
		t.Errorf("expected 2 tooltips, got %d", tooltips)
	}
}

// --- replaceAbbreviations ---

func TestReplaceAbbreviationsBasic(t *testing.T) {
	t.Parallel()
	defs := map[string]string{"HTML": "HyperText Markup Language"}
	runs := []Run{{Text: "The HTML spec."}}
	result := replaceAbbreviations(runs, buildAbbrMatcher(defs))
	found := false
	for _, r := range result {
		if r.Text == "HTML" &&
			r.Tooltip == "HyperText Markup Language" {
			found = true
		}
	}
	if !found {
		t.Error("expected abbreviation tooltip")
	}
}

func TestReplaceAbbreviationsNoDefs(t *testing.T) {
	t.Parallel()
	runs := []Run{{Text: "HTML text."}}
	result := replaceAbbreviations(runs, nil)
	if len(result) != 1 || result[0].Text != "HTML text." {
		t.Error("nil defs should return runs unchanged")
	}
}

func TestReplaceAbbreviationsWordBoundary(t *testing.T) {
	t.Parallel()
	defs := map[string]string{"HTML": "HyperText Markup Language"}
	runs := []Run{{Text: "HTMLX is not HTML."}}
	result := replaceAbbreviations(runs, buildAbbrMatcher(defs))
	for _, r := range result {
		if r.Text == "HTMLX" && r.Tooltip != "" {
			t.Error("HTMLX should not get tooltip")
		}
	}
	foundHTML := false
	for _, r := range result {
		if r.Text == "HTML" &&
			r.Tooltip == "HyperText Markup Language" {
			foundHTML = true
		}
	}
	if !foundHTML {
		t.Error("standalone HTML should get tooltip")
	}
}

func TestReplaceAbbreviationsSkipsLinks(t *testing.T) {
	t.Parallel()
	defs := map[string]string{"HTML": "Markup"}
	runs := []Run{{Text: "HTML spec.", Link: "http://x"}}
	result := replaceAbbreviations(runs, buildAbbrMatcher(defs))
	if len(result) != 1 {
		t.Error("link run should not be split for abbreviations")
	}
}

func TestReplaceAbbreviationsLongerFirst(t *testing.T) {
	t.Parallel()
	// Longer abbreviations should match first.
	defs := map[string]string{
		"CSS":  "Cascading Style Sheets",
		"CSS3": "CSS Level 3",
	}
	runs := []Run{{Text: "Use CSS3 today."}}
	result := replaceAbbreviations(runs, buildAbbrMatcher(defs))
	for _, r := range result {
		if r.Tooltip == "CSS Level 3" {
			return
		}
	}
	t.Error("CSS3 should match before CSS")
}

func TestReplaceAbbreviationsMultipleOccurrences(t *testing.T) {
	t.Parallel()
	defs := map[string]string{"HTML": "Markup"}
	runs := []Run{{Text: "HTML and HTML."}}
	result := replaceAbbreviations(runs, buildAbbrMatcher(defs))
	count := 0
	for _, r := range result {
		if r.Tooltip == "Markup" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 abbr matches, got %d", count)
	}
}

// --- mergeAdjacentRuns ---

func TestMergeAdjacentRunsSameFormat(t *testing.T) {
	t.Parallel()
	runs := []Run{
		{Text: "hello ", Format: FormatPlain},
		{Text: "world", Format: FormatPlain},
	}
	result := mergeAdjacentRuns(runs)
	if len(result) != 1 {
		t.Fatalf("expected 1 merged run, got %d", len(result))
	}
	if result[0].Text != "hello world" {
		t.Errorf("merged text: %q", result[0].Text)
	}
}

func TestMergeAdjacentRunsDifferentFormat(t *testing.T) {
	t.Parallel()
	runs := []Run{
		{Text: "plain ", Format: FormatPlain},
		{Text: "bold", Format: FormatBold},
	}
	result := mergeAdjacentRuns(runs)
	if len(result) != 2 {
		t.Errorf("expected 2 runs, got %d", len(result))
	}
}

func TestMergeAdjacentRunsEmpty(t *testing.T) {
	t.Parallel()
	result := mergeAdjacentRuns(nil)
	if len(result) != 0 {
		t.Error("nil input should return nil")
	}
}

func TestMergeAdjacentRunsSingle(t *testing.T) {
	t.Parallel()
	runs := []Run{{Text: "only"}}
	result := mergeAdjacentRuns(runs)
	if len(result) != 1 || result[0].Text != "only" {
		t.Error("single run should pass through")
	}
}

func TestMergeAdjacentRunsMathNotMerged(t *testing.T) {
	t.Parallel()
	runs := []Run{
		{Text: "a", MathID: "m1"},
		{Text: "b", MathID: "m2"},
	}
	result := mergeAdjacentRuns(runs)
	if len(result) != 2 {
		t.Error("math runs should not be merged")
	}
}

func TestMergeAdjacentRunsDifferentCodeToken(t *testing.T) {
	t.Parallel()
	runs := []Run{
		{Text: "func", Format: FormatCode, CodeToken: TokenKeyword},
		{Text: " ", Format: FormatCode, CodeToken: TokenPlain},
	}
	result := mergeAdjacentRuns(runs)
	if len(result) != 2 {
		t.Error("different CodeToken should not merge")
	}
}

// --- applyTypography ---

func TestApplyTypographyEmDash(t *testing.T) {
	t.Parallel()
	runs := []Run{{Text: "hello---world"}}
	applyTypography(runs)
	if runs[0].Text != "hello\u2014world" {
		t.Errorf("em dash: %q", runs[0].Text)
	}
}

func TestApplyTypographyEnDash(t *testing.T) {
	t.Parallel()
	runs := []Run{{Text: "10--20"}}
	applyTypography(runs)
	if runs[0].Text != "10\u201320" {
		t.Errorf("en dash: %q", runs[0].Text)
	}
}

func TestApplyTypographyEllipsis(t *testing.T) {
	t.Parallel()
	runs := []Run{{Text: "wait..."}}
	applyTypography(runs)
	if runs[0].Text != "wait\u2026" {
		t.Errorf("ellipsis: %q", runs[0].Text)
	}
}

func TestApplyTypographySkipsCode(t *testing.T) {
	t.Parallel()
	runs := []Run{{Text: "a---b", Format: FormatCode}}
	applyTypography(runs)
	if runs[0].Text != "a---b" {
		t.Error("code runs should not get typography")
	}
}

func TestApplyTypographySkipsMath(t *testing.T) {
	t.Parallel()
	runs := []Run{{Text: "a---b", MathID: "m1"}}
	applyTypography(runs)
	if runs[0].Text != "a---b" {
		t.Error("math runs should not get typography")
	}
}

func TestApplyTypographyOrderMatters(t *testing.T) {
	t.Parallel()
	// --- must be replaced before -- to avoid partial match.
	runs := []Run{{Text: "a---b--c"}}
	applyTypography(runs)
	if runs[0].Text != "a\u2014b\u2013c" {
		t.Errorf("order: %q", runs[0].Text)
	}
}

// --- trimTrailingBreaks ---

func TestTrimTrailingBreaks(t *testing.T) {
	t.Parallel()
	runs := []Run{
		{Text: "hello"},
		{Text: "\n"},
		{Text: "\n"},
	}
	result := trimTrailingBreaks(runs)
	if len(result) != 1 || result[0].Text != "hello" {
		t.Errorf("trim: got %d runs", len(result))
	}
}

func TestTrimTrailingBreaksKeepsLinkBreak(t *testing.T) {
	t.Parallel()
	runs := []Run{
		{Text: "hello"},
		{Text: "\n", Link: "http://x"},
	}
	result := trimTrailingBreaks(runs)
	if len(result) != 2 {
		t.Error("break with link should not be trimmed")
	}
}

func TestTrimTrailingBreaksEmpty(t *testing.T) {
	t.Parallel()
	result := trimTrailingBreaks(nil)
	if len(result) != 0 {
		t.Error("nil should return nil")
	}
}

// --- RunsToText ---

func TestRunsToTextBasic(t *testing.T) {
	t.Parallel()
	runs := []Run{
		{Text: "hello "},
		{Text: "world"},
	}
	if RunsToText(runs) != "hello world" {
		t.Errorf("RunsToText: %q", RunsToText(runs))
	}
}

func TestRunsToTextEmpty(t *testing.T) {
	t.Parallel()
	if RunsToText(nil) != "" {
		t.Error("nil runs should return empty string")
	}
}

// --- MathHash ---

func TestMathHashDeterministic(t *testing.T) {
	t.Parallel()
	a := MathHash("E = mc^2")
	b := MathHash("E = mc^2")
	if a != b {
		t.Error("same input should produce same hash")
	}
}

func TestMathHashDifferent(t *testing.T) {
	t.Parallel()
	a := MathHash("E = mc^2")
	b := MathHash("a^2 + b^2 = c^2")
	if a == b {
		t.Error("different inputs should produce different hashes")
	}
}

func TestMathHashEmpty(t *testing.T) {
	t.Parallel()
	// FNV-1a offset basis for empty string.
	if MathHash("") != 14695981039346656037 {
		t.Errorf("empty string hash = %d, want FNV offset basis",
			MathHash(""))
	}
}

// --- parseImageSrc ---

func TestParseImageSrcNoDims(t *testing.T) {
	t.Parallel()
	src, w, h := parseImageSrc("image.png")
	if src != "image.png" || w != 0 || h != 0 {
		t.Errorf("no dims: src=%q w=%v h=%v", src, w, h)
	}
}

func TestParseImageSrcFragmentDims(t *testing.T) {
	t.Parallel()
	src, w, h := parseImageSrc("image.png#dim=200x100")
	if src != "image.png" || w != 200 || h != 100 {
		t.Errorf("fragment dims: src=%q w=%v h=%v", src, w, h)
	}
}

func TestParseImageSrcSpaceDims(t *testing.T) {
	t.Parallel()
	src, w, h := parseImageSrc("image.png =200x100")
	if src != "image.png" || w != 200 || h != 100 {
		t.Errorf("space dims: src=%q w=%v h=%v", src, w, h)
	}
}

func TestParseImageSrcURL(t *testing.T) {
	t.Parallel()
	src, w, h := parseImageSrc("https://example.com/img.png")
	if src != "https://example.com/img.png" || w != 0 || h != 0 {
		t.Errorf("url: src=%q w=%v h=%v", src, w, h)
	}
}

// --- parseDims ---

func TestParseDims(t *testing.T) {
	t.Parallel()
	w, h, ok := parseDims("200x100")
	if !ok || w != 200 || h != 100 {
		t.Errorf("parseDims: w=%v h=%v ok=%v", w, h, ok)
	}
}

func TestParseDimsNoX(t *testing.T) {
	t.Parallel()
	_, _, ok := parseDims("200")
	if ok {
		t.Error("no 'x' should not parse")
	}
}

func TestParseDimsZero(t *testing.T) {
	t.Parallel()
	_, _, ok := parseDims("0x0")
	if ok {
		t.Error("0x0 should not be valid")
	}
}

// --- parseFloat32 ---

func TestParseFloat32(t *testing.T) {
	t.Parallel()
	if parseFloat32("200") != 200 {
		t.Error("200 should parse")
	}
	if parseFloat32("0") != 0 {
		t.Error("0 should parse to 0")
	}
	if parseFloat32("abc") != 0 {
		t.Error("non-numeric should return 0")
	}
	if parseFloat32("") != 0 {
		t.Error("empty should return 0")
	}
}

// --- canMergeRuns ---

func TestCanMergeRunsIdentical(t *testing.T) {
	t.Parallel()
	a := Run{Text: "a", Format: FormatBold}
	b := Run{Text: "b", Format: FormatBold}
	if !canMergeRuns(a, b) {
		t.Error("identical format runs should merge")
	}
}

func TestCanMergeRunsDifferentFormat(t *testing.T) {
	t.Parallel()
	a := Run{Text: "a", Format: FormatBold}
	b := Run{Text: "b", Format: FormatItalic}
	if canMergeRuns(a, b) {
		t.Error("different format should not merge")
	}
}

func TestCanMergeRunsDifferentStrikethrough(t *testing.T) {
	t.Parallel()
	a := Run{Text: "a", Strikethrough: true}
	b := Run{Text: "b", Strikethrough: false}
	if canMergeRuns(a, b) {
		t.Error("different strikethrough should not merge")
	}
}

func TestCanMergeRunsDifferentLink(t *testing.T) {
	t.Parallel()
	a := Run{Text: "a", Link: "http://x"}
	b := Run{Text: "b", Link: "http://y"}
	if canMergeRuns(a, b) {
		t.Error("different links should not merge")
	}
}

func TestCanMergeRunsMathBlocks(t *testing.T) {
	t.Parallel()
	a := Run{Text: "a", MathID: "m1"}
	b := Run{Text: "b"}
	if canMergeRuns(a, b) {
		t.Error("math run should not merge")
	}
}

func TestCanMergeRunsDifferentUnderline(t *testing.T) {
	t.Parallel()
	a := Run{Text: "a", Underline: true}
	b := Run{Text: "b", Underline: false}
	if canMergeRuns(a, b) {
		t.Error("different underline should not merge")
	}
}
