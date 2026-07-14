package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// --- markdownBlockAmendSel ---

func TestMarkdownBlockAmendSel_NilTC_NoOp(t *testing.T) {
	w := &Window{}
	l := &Layout{Shape: &Shape{}}
	markdownBlockAmendSel(l, w) // must not panic
}

func TestMarkdownBlockAmendSel_ZeroMarkdownID_NoOp(t *testing.T) {
	w := &Window{}
	tc := &ShapeTextConfig{MarkdownID: "", TextSelBeg: 99, TextSelEnd: 99}
	l := &Layout{Shape: &Shape{TC: tc}}
	markdownBlockAmendSel(l, w)
	// No state written; values unchanged.
	if tc.TextSelBeg != 99 {
		t.Error("unexpected mutation when MarkdownID is empty")
	}
}

func TestMarkdownBlockAmendSel_SelectionBeforeBlock_ZeroOut(t *testing.T) {
	w := &Window{}
	mdID := "md1"
	StateMap[string, mdSelState](w, nsMdSel, capMany).Set(mdID,
		mdSelState{SelBeg: 0, SelEnd: 5})
	tc := &ShapeTextConfig{
		MarkdownID:         mdID,
		MarkdownBlockStart: 10,
		MarkdownRuneLen:    5,
		TextSelBeg:         99,
		TextSelEnd:         99,
	}
	markdownBlockAmendSel(&Layout{Shape: &Shape{TC: tc}}, w)
	if tc.TextSelBeg != 0 || tc.TextSelEnd != 0 {
		t.Errorf("got %d/%d, want 0/0", tc.TextSelBeg, tc.TextSelEnd)
	}
}

func TestMarkdownBlockAmendSel_SelectionAfterBlock_ZeroOut(t *testing.T) {
	w := &Window{}
	mdID := "md2"
	StateMap[string, mdSelState](w, nsMdSel, capMany).Set(mdID,
		mdSelState{SelBeg: 20, SelEnd: 30})
	tc := &ShapeTextConfig{
		MarkdownID:         mdID,
		MarkdownBlockStart: 0,
		MarkdownRuneLen:    5, // blockEnd = 5; beg(20) >= 5 → no overlap
		TextSelBeg:         99,
		TextSelEnd:         99,
	}
	markdownBlockAmendSel(&Layout{Shape: &Shape{TC: tc}}, w)
	if tc.TextSelBeg != 0 || tc.TextSelEnd != 0 {
		t.Errorf("got %d/%d, want 0/0", tc.TextSelBeg, tc.TextSelEnd)
	}
}

func TestMarkdownBlockAmendSel_SelectionAtExactBlockBoundary_ZeroOut(t *testing.T) {
	w := &Window{}
	mdID := "md3"
	// end == blockStart: guard is end <= blockStart, so no overlap.
	StateMap[string, mdSelState](w, nsMdSel, capMany).Set(mdID,
		mdSelState{SelBeg: 0, SelEnd: 10})
	tc := &ShapeTextConfig{
		MarkdownID:         mdID,
		MarkdownBlockStart: 10,
		MarkdownRuneLen:    5,
		TextSelBeg:         99,
		TextSelEnd:         99,
	}
	markdownBlockAmendSel(&Layout{Shape: &Shape{TC: tc}}, w)
	if tc.TextSelBeg != 0 || tc.TextSelEnd != 0 {
		t.Errorf("got %d/%d, want 0/0", tc.TextSelBeg, tc.TextSelEnd)
	}
}

func TestMarkdownBlockAmendSel_SelectionSpansEntireBlock(t *testing.T) {
	w := &Window{}
	mdID := "md4"
	StateMap[string, mdSelState](w, nsMdSel, capMany).Set(mdID,
		mdSelState{SelBeg: 0, SelEnd: 20})
	tc := &ShapeTextConfig{
		MarkdownID:         mdID,
		MarkdownBlockStart: 5,
		MarkdownRuneLen:    8, // block covers [5, 13)
	}
	markdownBlockAmendSel(&Layout{Shape: &Shape{TC: tc}}, w)
	// localBeg = max(0,5)-5 = 0, localEnd = min(20,13)-5 = 8
	if tc.TextSelBeg != 0 || tc.TextSelEnd != 8 {
		t.Errorf("got %d/%d, want 0/8", tc.TextSelBeg, tc.TextSelEnd)
	}
}

func TestMarkdownBlockAmendSel_PartialOverlapStart(t *testing.T) {
	w := &Window{}
	mdID := "md5"
	// Selection starts before block, ends inside block.
	StateMap[string, mdSelState](w, nsMdSel, capMany).Set(mdID,
		mdSelState{SelBeg: 5, SelEnd: 12})
	tc := &ShapeTextConfig{
		MarkdownID:         mdID,
		MarkdownBlockStart: 10, // block covers [10, 20)
		MarkdownRuneLen:    10,
	}
	markdownBlockAmendSel(&Layout{Shape: &Shape{TC: tc}}, w)
	// localBeg = max(5,10)-10 = 0, localEnd = min(12,20)-10 = 2
	if tc.TextSelBeg != 0 || tc.TextSelEnd != 2 {
		t.Errorf("got %d/%d, want 0/2", tc.TextSelBeg, tc.TextSelEnd)
	}
}

func TestMarkdownBlockAmendSel_PartialOverlapEnd(t *testing.T) {
	w := &Window{}
	mdID := "md6"
	// Selection starts inside block, ends after block.
	StateMap[string, mdSelState](w, nsMdSel, capMany).Set(mdID,
		mdSelState{SelBeg: 13, SelEnd: 25})
	tc := &ShapeTextConfig{
		MarkdownID:         mdID,
		MarkdownBlockStart: 10, // block covers [10, 20)
		MarkdownRuneLen:    10,
	}
	markdownBlockAmendSel(&Layout{Shape: &Shape{TC: tc}}, w)
	// localBeg = max(13,10)-10 = 3, localEnd = min(25,20)-10 = 10
	if tc.TextSelBeg != 3 || tc.TextSelEnd != 10 {
		t.Errorf("got %d/%d, want 3/10", tc.TextSelBeg, tc.TextSelEnd)
	}
}

func TestMarkdownBlockAmendSel_InvertedSelection_SortsCorrectly(t *testing.T) {
	w := &Window{}
	mdID := "md7"
	// SelEnd < SelBeg (drag rightward from end to start); u32Sort normalizes.
	StateMap[string, mdSelState](w, nsMdSel, capMany).Set(mdID,
		mdSelState{SelBeg: 15, SelEnd: 5})
	tc := &ShapeTextConfig{
		MarkdownID:         mdID,
		MarkdownBlockStart: 0,
		MarkdownRuneLen:    20,
	}
	markdownBlockAmendSel(&Layout{Shape: &Shape{TC: tc}}, w)
	// After u32Sort: beg=5, end=15. localBeg=5, localEnd=15.
	if tc.TextSelBeg != 5 || tc.TextSelEnd != 15 {
		t.Errorf("got %d/%d, want 5/15", tc.TextSelBeg, tc.TextSelEnd)
	}
}

// --- mdExtractText ---

func TestMdExtractText_EmptyBlocks(t *testing.T) {
	got := mdExtractText(nil, 0, 10)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestMdExtractText_BegEqualsEnd_Empty(t *testing.T) {
	blocks := []mdBlockInfo{{StartRune: 0, RuneLen: 5, FlatText: "hello"}}
	got := mdExtractText(blocks, 3, 3)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestMdExtractText_SingleBlockFull(t *testing.T) {
	blocks := []mdBlockInfo{{StartRune: 0, RuneLen: 5, FlatText: "hello"}}
	got := mdExtractText(blocks, 0, 5)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestMdExtractText_SingleBlockPartial(t *testing.T) {
	blocks := []mdBlockInfo{{StartRune: 0, RuneLen: 11, FlatText: "hello world"}}
	got := mdExtractText(blocks, 2, 7)
	want := "llo w"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMdExtractText_SelectionBeforeBlock_Empty(t *testing.T) {
	blocks := []mdBlockInfo{{StartRune: 10, RuneLen: 5, FlatText: "hello"}}
	got := mdExtractText(blocks, 0, 5)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestMdExtractText_SelectionAfterBlock_Empty(t *testing.T) {
	blocks := []mdBlockInfo{{StartRune: 0, RuneLen: 5, FlatText: "hello"}}
	got := mdExtractText(blocks, 10, 20)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestMdExtractText_SpansMultipleBlocks_NewlineJoined(t *testing.T) {
	blocks := []mdBlockInfo{
		{StartRune: 0, RuneLen: 5, FlatText: "hello"},
		{StartRune: 5, RuneLen: 5, FlatText: "world"},
	}
	// beg=3 (inside block 0), end=8 (inside block 1)
	// block 0: localBeg=3, localEnd=5 → "lo"
	// block 1: localBeg=0, localEnd=3 → "wor"
	got := mdExtractText(blocks, 3, 8)
	want := "lo\nwor"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMdExtractText_SpansAllBlocks(t *testing.T) {
	blocks := []mdBlockInfo{
		{StartRune: 0, RuneLen: 3, FlatText: "foo"},
		{StartRune: 3, RuneLen: 3, FlatText: "bar"},
		{StartRune: 6, RuneLen: 3, FlatText: "baz"},
	}
	got := mdExtractText(blocks, 0, 9)
	want := "foo\nbar\nbaz"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMdExtractText_MultibyteUTF8(t *testing.T) {
	// "héllo": é is 2 bytes, 5 runes total.
	text := "héllo"
	blocks := []mdBlockInfo{
		{StartRune: 0, RuneLen: uint32(utf8RuneCount(text)), FlatText: text},
	}
	// Extract runes [1, 4): "éll"
	got := mdExtractText(blocks, 1, 4)
	want := "éll"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMdExtractText_LocalEndClamped(t *testing.T) {
	// RuneLen overstates; localEnd clamp prevents out-of-bounds slice.
	blocks := []mdBlockInfo{{StartRune: 0, RuneLen: 100, FlatText: "hi"}}
	got := mdExtractText(blocks, 0, 100)
	if got != "hi" {
		t.Errorf("got %q, want %q", got, "hi")
	}
}

// --- mdHitAbsRune ---

func newMdBlock(startRune uint32, shapeY, h float32) mdBlockInfo {
	return mdBlockInfo{
		StartRune: startRune,
		RuneLen:   5,
		FlatText:  "hello",
		ShapeY:    shapeY,
		H:         h,
		Layout:    glyph.Layout{},
	}
}

func TestMdHitAbsRune_EmptyBlocks_FallbackStart(t *testing.T) {
	got := mdHitAbsRune(0, 0, nil, glyph.Layout{}, "", 42)
	if got != 42 {
		t.Errorf("got %d, want 42 (fallbackStart)", got)
	}
}

func TestMdHitAbsRune_MouseInsideBlock(t *testing.T) {
	blocks := []mdBlockInfo{
		newMdBlock(0, 0, 20),    // covers y [0, 20)
		newMdBlock(100, 25, 20), // covers y [25, 45)
	}
	// my=10 → inside block 0
	if got := mdHitAbsRune(0, 10, blocks, glyph.Layout{}, "", 0); got != 0 {
		t.Errorf("my=10: got %d, want 0 (block 0)", got)
	}
	// my=30 → inside block 1
	if got := mdHitAbsRune(0, 30, blocks, glyph.Layout{}, "", 0); got != 100 {
		t.Errorf("my=30: got %d, want 100 (block 1)", got)
	}
}

func TestMdHitAbsRune_MouseAboveAllBlocks_SnapsToFirst(t *testing.T) {
	blocks := []mdBlockInfo{
		newMdBlock(0, 100, 20),
		newMdBlock(50, 130, 20),
	}
	got := mdHitAbsRune(0, -10, blocks, glyph.Layout{}, "", 0)
	if got != 0 {
		t.Errorf("got %d, want 0 (first block's StartRune)", got)
	}
}

func TestMdHitAbsRune_MouseBelowAllBlocks_SnapsToLast(t *testing.T) {
	blocks := []mdBlockInfo{
		newMdBlock(0, 0, 20),
		newMdBlock(100, 30, 20),
		newMdBlock(200, 60, 20),
	}
	got := mdHitAbsRune(0, 999, blocks, glyph.Layout{}, "", 0)
	if got != 200 {
		t.Errorf("got %d, want 200 (last block's StartRune)", got)
	}
}

func TestMdHitAbsRune_MouseInGap_SnapsToBlockAbove(t *testing.T) {
	blocks := []mdBlockInfo{
		newMdBlock(0, 0, 20),    // covers y [0, 20)
		newMdBlock(100, 30, 20), // covers y [30, 50)
	}
	// my=22 is in the gap [20, 30); should snap to block 0 (last block above).
	got := mdHitAbsRune(0, 22, blocks, glyph.Layout{}, "", 0)
	if got != 0 {
		t.Errorf("got %d, want 0 (block above the gap)", got)
	}
}

func TestMdHitAbsRune_ExactBlockBoundary_PicksNextBlock(t *testing.T) {
	blocks := []mdBlockInfo{
		newMdBlock(0, 0, 20),
		newMdBlock(100, 20, 20), // starts exactly at y=20
	}
	// my=20 satisfies my >= ShapeY(20) && my < ShapeY+H(40) → block 1.
	got := mdHitAbsRune(0, 20, blocks, glyph.Layout{}, "", 0)
	if got != 100 {
		t.Errorf("got %d, want 100 (block 1 at exact boundary)", got)
	}
}
