//go:build !js

package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// The End caret on a soft-wrapped right-to-left line sits at the left
// edge of that line, after its last letter. Before #1371 it sat at the
// right edge, where the line starts. The stub measurers have no bidi
// data, so this test shapes real Hebrew with a go-glyph Context.
func TestCursorPosTrailingRTLSoftWrap(t *testing.T) {
	ctx, err := glyph.NewContext(1)
	if err != nil {
		t.Fatalf("NewContext: %v", err)
	}
	t.Cleanup(ctx.Free)
	cfg := glyph.TextConfig{Style: glyph.TextStyle{FontName: "Sans 16"}}
	word, err := ctx.LayoutText("אבג", cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Room for one word per line, so the space after the first word
	// is a soft wrap.
	width := word.Width + 2
	cfg.Block = glyph.BlockStyle{Width: width, Wrap: glyph.WrapWord}
	const text = "אבג דהו"
	l, err := ctx.LayoutText(text, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Lines) < 2 {
		t.Fatalf("got %d lines, want a soft wrap", len(l.Lines))
	}
	b := newTextBlockFromLayout(text, &l, width, DefaultTextStyle, TextModeWrap, nil)

	first := l.Lines[0]
	idx := l.Lines[1].StartIndex
	cp, ok := b.cursorPosTrailing(idx, true)
	if !ok {
		t.Fatal("cursorPosTrailing: not ok")
	}
	want, _ := l.LineEndCursorPos(0)
	if !f32AreClose(cp.X, want.X) {
		t.Errorf("trailing X = %.2f, want %.2f (end of RTL line)", cp.X, want.X)
	}
	if f32AreClose(cp.X, first.Rect.X+first.Rect.Width) {
		t.Errorf("trailing X = %.2f is the right edge, the start of the RTL line", cp.X)
	}
	if cp.Y != first.Rect.Y || cp.Height != first.Rect.Height {
		t.Errorf("trailing Y/Height = %v/%v, want line 0 %v/%v",
			cp.Y, cp.Height, first.Rect.Y, first.Rect.Height)
	}

	// Without trailing the caret is at the start of the next line.
	lead, ok := b.cursorPosTrailing(idx, false)
	if !ok || lead.Y != l.Lines[1].Rect.Y {
		t.Errorf("leading caret %+v ok=%v, want on line 1", lead, ok)
	}
}

// An offset outside the text is not a soft-wrap boundary: the queries
// built on prevLine fall back, and the trailing caret is not ok.
func TestPrevLineOutOfRange(t *testing.T) {
	b := trailingBlock("aaaaabbbb", []glyph.Line{
		{StartIndex: 0, Length: 5},
		{StartIndex: 5, Length: 4},
	})
	for _, idx := range []int{-1, len("aaaaabbbb") + 1} {
		if got := b.trailingLineStart(idx, 42); got != 42 {
			t.Errorf("trailingLineStart(%d) = %d, want fallback 42", idx, got)
		}
		if _, ok := b.cursorPosTrailing(idx, true); ok {
			t.Errorf("cursorPosTrailing(%d) ok, want false", idx)
		}
	}
}
