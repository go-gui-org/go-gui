package gui

import (
	"testing"
	"unsafe"
)

// A plain Text allocates one textView per frame. Its shapeTextConfig
// carries the rich-text fields only through a pointer, so a plain
// Text does not pay for them. Before the split textView was 744 bytes
// (the 768-byte size class); after it, 576.
func TestTextViewFitsSizeClass(t *testing.T) {
	if got := unsafe.Sizeof(textView{}); got > 576 {
		t.Errorf("textView = %d bytes, want <= 576", got)
	}
}

var rtfShapeTextSink *shapeTextConfig

// An RTF shape's text config and its rich-text fields come from one
// allocation, so moving the fields behind a pointer costs RTF nothing.
func TestNewRTFShapeTextOneAllocation(t *testing.T) {
	n := testing.AllocsPerRun(20, func() {
		rtfShapeTextSink = newRTFShapeText(TextModeWrap, shapeRichText{markdownID: "md"})
	})
	if n != 1 {
		t.Errorf("newRTFShapeText allocs = %v, want 1", n)
	}
	tc := rtfShapeTextSink
	if tc.rtf == nil || tc.rtf.markdownID != "md" || tc.TextMode != TextModeWrap {
		t.Fatalf("rtf not linked: %+v", tc.rtf)
	}
}
