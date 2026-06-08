package gui

import "testing"

func TestMarkdownViewGeneratesLayout(t *testing.T) {
	w := &Window{}
	v := w.Markdown(MarkdownCfg{
		ID:     "md1",
		Source: "# Hello",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
}

func TestMarkdownViewEmptySource(t *testing.T) {
	w := &Window{}
	v := w.Markdown(MarkdownCfg{
		ID:     "md2",
		Source: "",
	})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape even with empty source")
	}
}

func TestMarkdownViewInvisible(t *testing.T) {
	w := &Window{}
	v := w.Markdown(MarkdownCfg{
		ID:        "md3",
		Source:    "text",
		Invisible: true,
	})
	layout := generateViewLayout(v, w)
	if !layout.Shape.Disabled {
		t.Error("invisible markdown should be disabled")
	}
	if !layout.Shape.OverDraw {
		t.Error("invisible markdown should be overdraw")
	}
}
