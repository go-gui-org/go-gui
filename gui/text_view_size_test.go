package gui

import (
	"testing"
	"unsafe"
)

// Every Text is a heap object built by its factory, before any window
// exists, so its size is paid once per Text per frame. The per-frame
// text config and accessible name come from the view-phase arena
// instead of riding on the view. 320 is a Go size class: growing past it
// moves every Text to the next class.
func TestTextViewSize(t *testing.T) {
	if got := unsafe.Sizeof(textView{}); got > 320 {
		t.Errorf("textView is %d bytes, want <= 320", got)
	}
}

// A Text view generated twice in one frame (the same View placed in two
// spots) must give each shape its own text config. A config stored on
// the view was shared, so the second shape overwrote the first.
func TestTextViewGeneratedTwiceHasSeparateTextConfig(t *testing.T) {
	w := &Window{scratch: newScratchPools()}
	v := Text(TextCfg{Text: "hello"})
	w.scratch.resetViewPools()
	a := v.GenerateLayout(w)
	b := v.GenerateLayout(w)
	if a.Shape.TC == b.Shape.TC {
		t.Fatal("both shapes share one shapeTextConfig")
	}
	if a.Shape.TC.Text != "hello" || b.Shape.TC.Text != "hello" {
		t.Fatalf("text = %q, %q, want hello", a.Shape.TC.Text, b.Shape.TC.Text)
	}
}

// A titled container's group-box label is built every frame. Its text
// config and style come from the view-phase arena, so the title costs
// no heap allocation per frame.
func TestGroupBoxTitleDoesNotAllocate(t *testing.T) {
	w := &Window{scratch: newScratchPools()}
	v := Column(ContainerCfg{Title: "Group"})
	w.scratch.resetViewPools()
	_ = generateViewLayout(v, w)

	var layout Layout
	allocs := testing.AllocsPerRun(100, func() {
		w.scratch.resetViewPools()
		layout = generateViewLayout(v, w)
	})
	if allocs != 0 {
		t.Errorf("titled container: %v allocs per frame, want 0", allocs)
	}
	var title *Shape
	for i := range layout.Children {
		if tc := layout.Children[i].Shape.TC; tc != nil {
			title = layout.Children[i].Shape
		}
	}
	if title == nil || title.TC.Text != "Group" || title.TC.TextStyle == nil {
		t.Fatal("title text shape missing or empty")
	}
}
