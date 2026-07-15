package gui

import "testing"

func TestContainerIDScrollAutoAppendsScrollbars(t *testing.T) {
	v := Column(ContainerCfg{
		Scrollable: true,
		ID:         "test-scroll-1",
		Content:    []View{Rectangle(RectangleCfg{})},
	})
	cv := v.(*containerView)
	// 1 user child + 2 scrollbars = 3
	if len(cv.content) != 3 {
		t.Errorf("expected 3 children, got %d", len(cv.content))
	}
}

func TestContainerScrollbarCfgXHiddenSuppresses(t *testing.T) {
	v := Column(ContainerCfg{
		Scrollable:    true,
		ID:            "test-scroll-2",
		ScrollbarCfgX: &ScrollbarCfg{Overflow: ScrollbarHidden},
		Content:       []View{Rectangle(RectangleCfg{})},
	})
	cv := v.(*containerView)
	// 1 user child + 0 horizontal + 1 vertical = 2
	if len(cv.content) != 2 {
		t.Errorf("expected 2 children, got %d", len(cv.content))
	}
}

func TestContainerScrollbarCfgYOverrides(t *testing.T) {
	customThumb := RGB(255, 0, 0)
	v := Column(ContainerCfg{
		Scrollable:    true,
		ID:            "test-scroll-3",
		ScrollbarCfgY: &ScrollbarCfg{ColorThumb: customThumb},
		Content:       []View{Rectangle(RectangleCfg{})},
	})
	cv := v.(*containerView)
	// 1 user child + 1 horizontal (default) + 1 vertical (custom) = 3
	if len(cv.content) != 3 {
		t.Errorf("expected 3 children, got %d", len(cv.content))
	}
}

func TestContainerNoIDScrollNoScrollbars(t *testing.T) {
	v := Column(ContainerCfg{
		Content: []View{Rectangle(RectangleCfg{})},
	})
	cv := v.(*containerView)
	if len(cv.content) != 1 {
		t.Errorf("expected 1 child, got %d", len(cv.content))
	}
}

func TestContainerGenerateLayoutShapeIsolation(t *testing.T) {
	v := Row(ContainerCfg{
		Content: []View{Text(TextCfg{Text: "a"})},
	})
	w := &Window{}

	l1 := generateViewLayout(v, w)
	l1.Shape.X = 100
	l1.Shape.Y = 200
	l1.Shape.Width = 300

	l2 := generateViewLayout(v, w)
	if l2.Shape.X != 0 {
		t.Errorf("X leaked across frames: got %g, want 0", l2.Shape.X)
	}
	if l2.Shape.Y != 0 {
		t.Errorf("Y leaked across frames: got %g, want 0", l2.Shape.Y)
	}
	if l2.Shape.Width != 0 {
		t.Errorf("Width leaked across frames: got %g, want 0", l2.Shape.Width)
	}
}
