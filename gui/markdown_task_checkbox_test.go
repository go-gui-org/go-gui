package gui

import "testing"

// The GFM task-list box is the body text size, with the toggle's inner
// padding and a check at mdTaskCheckScale of the edge (#1254). The box
// used to be 0.8 of the text size with a check 1.1 times the box. This
// test pins sizes only; the drawn ink inset is pinned against the real
// icon font in gui/backend/soft/markdown_task_ink_test.go.
func TestMdTaskCheckboxSizes(t *testing.T) {
	w := &Window{}
	style := DefaultMarkdownStyle()
	cfg := MarkdownCfg{Style: style}
	d := &defaultToggleStyle
	wantEdge := style.Text.Size
	wantCheck := wantEdge * mdTaskCheckScale

	for _, checked := range []bool{false, true} {
		box := generateViewLayout(mdTaskCheckbox(checked, cfg), w)
		s := box.Shape
		if s.Width != wantEdge || s.Height != wantEdge {
			t.Errorf("checked=%v: box %vx%v, want %vx%v",
				checked, s.Width, s.Height, wantEdge, wantEdge)
		}
		if s.Padding != d.Padding {
			t.Errorf("checked=%v: padding %+v, want toggle %+v",
				checked, s.Padding, d.Padding)
		}
		if !checked {
			continue
		}
		if len(box.Children) != 1 || box.Children[0].Shape.TC == nil {
			t.Fatalf("checked box: want one text child")
		}
		ts := box.Children[0].Shape.TC.TextStyle
		if ts.Size != wantCheck {
			t.Errorf("check size %v, want %v", ts.Size, wantCheck)
		}
	}
}
