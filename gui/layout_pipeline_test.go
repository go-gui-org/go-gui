package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestLayoutPipelineNoPanic(t *testing.T) {
	_ = t
	w := &Window{}
	shape := &Shape{
		shapeType: shapeRectangle,
		Width:     100, Height: 100,
		Sizing:  FillFill,
		Opacity: 1,
	}
	layout := Layout{
		Shape: shape,
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 50, Opacity: 1}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 30, Height: 30, Opacity: 1}},
		},
	}
	w.windowWidth = 200
	w.windowHeight = 200
	layoutPipeline(&layout, w)
}

func TestLayoutAmendFiresChildrenFirst(t *testing.T) {
	w := &Window{}
	var order []string

	child := &Shape{
		shapeType: shapeRectangle,
		events: &eventHandlers{
			AmendLayout: func(_ *Layout, _ *Window) {
				order = append(order, "child")
			},
		},
		Opacity: 1,
	}
	parent := &Shape{
		shapeType: shapeRectangle,
		events: &eventHandlers{
			AmendLayout: func(_ *Layout, _ *Window) {
				order = append(order, "parent")
			},
		},
		Opacity: 1,
	}
	layout := Layout{
		Shape: parent,
		Children: []Layout{
			{Shape: child},
		},
	}
	layoutAmend(&layout, w)

	if len(order) != 2 {
		t.Fatalf("expected 2 callbacks, got %d", len(order))
	}
	if order[0] != "child" || order[1] != "parent" {
		t.Errorf("expected [child parent], got %v", order)
	}
}

func TestLayoutHoverInsideShape(t *testing.T) {
	w := &Window{}
	w.windowWidth = 200
	w.windowHeight = 200
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15

	hovered := false
	shape := &Shape{
		shapeType: shapeRectangle,
		shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
		events: &eventHandlers{
			OnHover: func(_ *Layout, _ *Event, _ *Window) {
				hovered = true
			},
		},
		Opacity: 1,
	}
	layout := Layout{Shape: shape}
	result := layoutHover(&layout, w)

	if !result {
		t.Error("expected hover to return true")
	}
	if !hovered {
		t.Error("expected OnHover to fire")
	}
}

func TestLayoutHoverOutsideShape(t *testing.T) {
	w := &Window{}
	w.viewState.mousePosX = 100
	w.viewState.mousePosY = 100

	hovered := false
	shape := &Shape{
		shapeType: shapeRectangle,
		shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
		events: &eventHandlers{
			OnHover: func(_ *Layout, _ *Event, _ *Window) {
				hovered = true
			},
		},
		Opacity: 1,
	}
	layout := Layout{Shape: shape}
	result := layoutHover(&layout, w)

	if result {
		t.Error("expected hover to return false")
	}
	if hovered {
		t.Error("expected OnHover not to fire")
	}
}

func TestLayoutHoverMouseLocked(t *testing.T) {
	w := &Window{}
	w.MouseLock(MouseLockCfg{MouseMove: func(*Layout, *Event, *Window) {}})
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15

	shape := &Shape{
		shapeType: shapeRectangle,
		shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
		events: &eventHandlers{
			OnHover: func(_ *Layout, _ *Event, _ *Window) {},
		},
		Opacity: 1,
	}
	layout := Layout{Shape: shape}
	result := layoutHover(&layout, w)
	if result {
		t.Error("expected false when mouse locked")
	}
}

func TestLayoutHoverBlockedByDialog(t *testing.T) {
	w := &Window{}
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15
	w.dialogCfg.visible = true

	hovered := false
	shape := &Shape{
		shapeType: shapeRectangle,
		shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
		events: &eventHandlers{
			OnHover: func(_ *Layout, _ *Event, _ *Window) {
				hovered = true
			},
		},
		Opacity: 1,
	}
	// Shape NOT inside a dialog layout.
	layout := Layout{Shape: shape}
	result := layoutHover(&layout, w)

	if result {
		t.Error("expected false outside dialog")
	}
	if hovered {
		t.Error("should not hover outside dialog")
	}
}

func TestLayoutInDialogLayoutPositive(t *testing.T) {
	dialog := &Shape{ID: reservedDialogID, shapeType: shapeRectangle}
	child := &Shape{shapeType: shapeRectangle}
	parent := Layout{Shape: dialog}
	childLayout := Layout{Shape: child, Parent: &parent}

	if !layoutInDialogLayout(&childLayout) {
		t.Error("expected true when inside dialog")
	}
}

func TestLayoutInDialogLayoutNegative(t *testing.T) {
	outer := &Shape{ID: "something_else", shapeType: shapeRectangle}
	child := &Shape{shapeType: shapeRectangle}
	parent := Layout{Shape: outer}
	childLayout := Layout{Shape: child, Parent: &parent}

	if layoutInDialogLayout(&childLayout) {
		t.Error("expected false outside dialog")
	}
}

func TestLayoutArrangeReturnsLayers(t *testing.T) {
	w := &Window{}
	w.windowWidth = 400
	w.windowHeight = 400
	shape := &Shape{
		shapeType: shapeRectangle,
		Width:     400, Height: 400,
		Sizing:  FillFill,
		Opacity: 1,
	}
	layout := Layout{Shape: shape}
	layouts := layoutArrange(&layout, w)

	if len(layouts) < 1 {
		t.Fatal("expected at least main layout")
	}
}

func TestLayoutArrangeNormalizesNilShape(t *testing.T) {
	w := &Window{}
	w.windowWidth = 400
	w.windowHeight = 400
	layout := Layout{}
	layouts := layoutArrange(&layout, w)
	if len(layouts) != 1 {
		t.Fatalf("layers: got %d, want 1", len(layouts))
	}
	if layouts[0].Shape == nil {
		t.Fatal("shape should be normalized")
	}
	if layouts[0].Shape.shapeType != shapeNone {
		t.Fatalf("shape type: got %v, want shapeNone", layouts[0].Shape.shapeType)
	}
}

func TestLayoutArrangeWithDialog(t *testing.T) {
	w := &Window{}
	w.windowWidth = 400
	w.windowHeight = 400
	w.Dialog(DialogCfg{
		Title: "Test",
		Body:  "Body",
	})

	shape := &Shape{
		shapeType: shapeRectangle,
		Width:     400, Height: 400,
		Sizing:  FillFill,
		Opacity: 1,
	}
	layout := Layout{Shape: shape}
	layouts := layoutArrange(&layout, w)

	if len(layouts) < 2 {
		t.Errorf("expected >= 2 layers with dialog, got %d",
			len(layouts))
	}
}

func TestLayoutArrangeWithToast(t *testing.T) {
	w := &Window{}
	w.windowWidth = 400
	w.windowHeight = 400
	// Manually add a toast (skip animation).
	w.toasts = append(w.toasts, toastNotification{
		id:       1,
		cfg:      ToastCfg{Title: "Hi", Body: "World"},
		animFrac: 1.0,
		phase:    toastVisible,
	})

	shape := &Shape{
		shapeType: shapeRectangle,
		Width:     400, Height: 400,
		Sizing:  FillFill,
		Opacity: 1,
	}
	layout := Layout{Shape: shape}
	layouts := layoutArrange(&layout, w)

	if len(layouts) < 2 {
		t.Errorf("expected >= 2 layers with toast, got %d",
			len(layouts))
	}
}

func TestLayoutWrapTextNoop(_ *testing.T) {
	// Should not panic.
	layoutWrapText(nil, nil)
}

type stubTextMeasurer struct {
	charWidth  float32
	fontHeight float32
}

func (m *stubTextMeasurer) TextWidth(text string, _ TextStyle) float32 {
	return float32(len(text)) * m.charWidth
}
func (m *stubTextMeasurer) TextHeight(_ string, _ TextStyle) float32 {
	return m.fontHeight
}
func (m *stubTextMeasurer) FontAscent(s TextStyle) float32 { return s.Size * 0.8 }
func (m *stubTextMeasurer) FontHeight(_ TextStyle) float32 {
	return m.fontHeight
}
func (m *stubTextMeasurer) LayoutText(text string, _ TextStyle, wrapWidth float32) (glyph.Layout, error) {
	if wrapWidth <= 0 || len(text) == 0 {
		return glyph.Layout{Height: m.fontHeight}, nil
	}
	// Simulate word wrapping using charWidth.
	lines := 1
	var lineW float32
	start := 0
	for i := 0; i <= len(text); i++ {
		if i < len(text) && text[i] != ' ' && text[i] != '\n' {
			continue
		}
		wordW := float32(i-start) * m.charWidth
		if lineW > 0 && lineW+wordW > wrapWidth {
			lines++
			lineW = wordW
		} else {
			lineW += wordW
		}
		if i < len(text) && text[i] == '\n' {
			lines++
			lineW = 0
		} else if i < len(text) {
			if lineW > 0 {
				lineW += m.charWidth
			}
		}
		start = i + 1
	}
	return glyph.Layout{Height: float32(lines) * m.fontHeight}, nil
}

func TestLayoutWrapPlainText(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}

	style := TextStyle{Size: 16}
	shape := &Shape{
		shapeType: shapeText,
		Width:     100, // fits ~10 chars per line
		TC: &ShapeTextConfig{
			Text:      "hello world this wraps",
			TextStyle: &style,
			TextMode:  TextModeWrap,
		},
	}
	layout := Layout{Shape: shape}
	layoutWrapText(&layout, w)

	// "hello" (50) + " " (10) + "world" (50) = 110 > 100 → 2nd line
	// Each word measured separately; expect multiple lines.
	if shape.Height <= 20 {
		t.Errorf("expected Height > 20 (wrapped), got %f",
			shape.Height)
	}
}

func TestFixedColumnCentering(t *testing.T) {
	w := &Window{}
	w.windowWidth = 300
	w.windowHeight = 300

	// Simulate the get_started Column with centered content
	col := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisTopToBottom,
			Width:     300,
			Height:    300,
			Sizing:    FixedFixed,
			MinWidth:  300, MaxWidth: 300,
			MinHeight: 300, MaxHeight: 300,
			HAlign:  HAlignCenter,
			VAlign:  VAlignMiddle,
			Spacing: 10,
			Opacity: 1,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeText, Width: 150, Height: 20, Opacity: 1}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 80, Height: 30, Opacity: 1}},
		},
	}

	layoutPipeline(&col, w)

	// Children should be vertically centered
	// remaining = 300 - 10 (spacing) - 20 - 30 = 240, offset = 120
	text := col.Children[0].Shape
	btn := col.Children[1].Shape

	t.Logf("Text: X=%.1f Y=%.1f W=%.1f H=%.1f", text.X, text.Y, text.Width, text.Height)
	t.Logf("Button: X=%.1f Y=%.1f W=%.1f H=%.1f", btn.X, btn.Y, btn.Width, btn.Height)

	// Vertical: VAlignMiddle
	expectedTextY := float32(120)
	if abs32(text.Y-expectedTextY) > 1 {
		t.Errorf("text Y = %.1f, want ~%.1f", text.Y, expectedTextY)
	}

	// Horizontal: HAlignCenter
	expectedTextX := float32((300 - 150) / 2)
	if abs32(text.X-expectedTextX) > 1 {
		t.Errorf("text X = %.1f, want ~%.1f", text.X, expectedTextX)
	}

	// Now "resize" to 500x500
	w.windowWidth = 500
	w.windowHeight = 500

	col2 := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisTopToBottom,
			Width:     500,
			Height:    500,
			Sizing:    FixedFixed,
			MinWidth:  500, MaxWidth: 500,
			MinHeight: 500, MaxHeight: 500,
			HAlign:  HAlignCenter,
			VAlign:  VAlignMiddle,
			Spacing: 10,
			Opacity: 1,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeText, Width: 150, Height: 20, Opacity: 1}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 80, Height: 30, Opacity: 1}},
		},
	}

	layoutPipeline(&col2, w)

	text2 := col2.Children[0].Shape
	btn2 := col2.Children[1].Shape

	t.Logf("After resize - Text: X=%.1f Y=%.1f W=%.1f H=%.1f", text2.X, text2.Y, text2.Width, text2.Height)
	t.Logf("After resize - Button: X=%.1f Y=%.1f W=%.1f H=%.1f", btn2.X, btn2.Y, btn2.Width, btn2.Height)

	// Vertical: remaining = 500 - 10 - 20 - 30 = 440, offset = 220
	expectedTextY2 := float32(220)
	if abs32(text2.Y-expectedTextY2) > 1 {
		t.Errorf("resized text Y = %.1f, want ~%.1f", text2.Y, expectedTextY2)
	}

	// Horizontal: (500 - 150) / 2 = 175
	expectedTextX2 := float32(175)
	if abs32(text2.X-expectedTextX2) > 1 {
		t.Errorf("resized text X = %.1f, want ~%.1f", text2.X, expectedTextX2)
	}
}

func TestTooltipColumnMaxWidthConstrainsText(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	w.windowWidth = 800
	w.windowHeight = 600

	longText := "This is a very long tooltip text that should definitely wrap when constrained to max width"
	style := TextStyle{Size: 16}

	// Simulate the tooltip Column with MaxWidth=300 and a
	// FillFit text child (TextModeWrap).
	textShape := &Shape{
		shapeType: shapeText,
		Width:     float32(len(longText)) * 10, // ~900px
		Height:    20,
		Sizing:    FillFit,
		Opacity:   1,
		TC: &ShapeTextConfig{
			Text:      longText,
			TextStyle: &style,
			TextMode:  TextModeWrap,
		},
	}
	col := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisTopToBottom,
			MaxWidth:  300,
			Opacity:   1,
		},
		Children: []Layout{
			{Shape: textShape},
		},
	}
	col.Children[0].Parent = &col

	layoutPipeline(&col, w)

	if col.Shape.Width > 300 {
		t.Errorf("column width %f exceeds MaxWidth 300",
			col.Shape.Width)
	}
	if textShape.Width > 300 {
		t.Errorf("text width %f exceeds 300", textShape.Width)
	}
	if textShape.Height <= 20 {
		t.Errorf("text height %f not wrapped (expected > 20)",
			textShape.Height)
	}
	t.Logf("col.Width=%.1f text.Width=%.1f text.Height=%.1f",
		col.Shape.Width, textShape.Width, textShape.Height)
}

func makeHoverShape(id string) *Shape {
	return &Shape{
		shapeType: shapeRectangle,
		ID:        id,
		shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
		Opacity:   1,
	}
}

func TestLayoutMouseLeaveEnterFiresNothing(t *testing.T) {
	w := &Window{}
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15

	left := 0
	shape := makeHoverShape("c1")
	shape.events = &eventHandlers{
		OnMouseLeave: func(_ *Layout, _ *Event, _ *Window) { left++ },
	}
	layout := Layout{Shape: shape}
	layoutMouseLeave(&layout, w)

	if left != 0 {
		t.Errorf("OnMouseLeave fired on enter: got %d, want 0", left)
	}
}

func TestLayoutMouseLeaveExitFiresOnMouseLeave(t *testing.T) {
	w := &Window{}

	left := 0
	shape := makeHoverShape("c2")
	shape.events = &eventHandlers{
		OnMouseLeave: func(_ *Layout, _ *Event, _ *Window) { left++ },
	}
	layout := Layout{Shape: shape}

	// Frame 1: inside.
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15
	layoutMouseLeave(&layout, w)

	// Frame 2: outside.
	w.viewState.mousePosX = 100
	w.viewState.mousePosY = 100
	layoutMouseLeave(&layout, w)

	if left != 1 {
		t.Errorf("OnMouseLeave fire count: got %d, want 1", left)
	}
}

func TestLayoutMouseLeaveNoRepeatWhileOutside(t *testing.T) {
	w := &Window{}

	left := 0
	shape := makeHoverShape("c3")
	shape.events = &eventHandlers{
		OnMouseLeave: func(_ *Layout, _ *Event, _ *Window) { left++ },
	}
	layout := Layout{Shape: shape}

	// Frame 1: inside.
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15
	layoutMouseLeave(&layout, w)

	// Frames 2-4: outside — should fire once only.
	for range 3 {
		w.viewState.mousePosX = 100
		w.viewState.mousePosY = 100
		layoutMouseLeave(&layout, w)
	}

	if left != 1 {
		t.Errorf("OnMouseLeave fired %d times while outside, want 1", left)
	}
}

func TestLayoutMouseLeaveReEnterResetsState(t *testing.T) {
	w := &Window{}

	entered, left := 0, 0
	shape := makeHoverShape("c4")
	shape.events = &eventHandlers{
		OnHover:      func(_ *Layout, _ *Event, _ *Window) { entered++ },
		OnMouseLeave: func(_ *Layout, _ *Event, _ *Window) { left++ },
	}
	layout := Layout{Shape: shape}

	// Frame 1: inside.
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15
	layoutMouseLeave(&layout, w)

	// Frame 2: outside → fires leave.
	w.viewState.mousePosX = 100
	w.viewState.mousePosY = 100
	layoutMouseLeave(&layout, w)

	// Frame 3: inside again → no leave.
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15
	layoutMouseLeave(&layout, w)

	// Frame 4: outside again → fires leave again.
	w.viewState.mousePosX = 100
	w.viewState.mousePosY = 100
	layoutMouseLeave(&layout, w)

	if left != 2 {
		t.Errorf("OnMouseLeave fire count: got %d, want 2", left)
	}
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// --- layoutWrapRTF: RtfFlatText population ---

func TestLayoutWrapRTF_RtfFlatText_SetOnCacheMiss(t *testing.T) {
	w := &Window{windowBackend: windowBackend{
		textMeasurer: &rtfStubTextMeasurer{
			layout: glyph.Layout{Width: 100, Height: 20},
		},
	}}
	rt := RichText{Runs: []RichTextRun{
		{Text: "hello"},
		{Text: " world"},
	}}
	shape := &Shape{
		shapeType: shapeRTF,
		Width:     200,
		TC: &ShapeTextConfig{
			TextMode:     TextModeWrap,
			RtfRuns:      &rt,
			RtfBaseStyle: glyph.TextStyle{Size: 12},
		},
	}
	layoutWrapRTF(shape, shape.TC, w)

	want := "hello world"
	if shape.TC.RtfFlatText != want {
		t.Errorf("RtfFlatText = %q, want %q", shape.TC.RtfFlatText, want)
	}
}

func TestLayoutWrapRTF_RtfFlatText_NotOverwrittenOnCacheHit(t *testing.T) {
	w := &Window{windowBackend: windowBackend{
		textMeasurer: &rtfStubTextMeasurer{
			layout: glyph.Layout{Width: 100, Height: 20},
		},
	}}
	rt := RichText{Runs: []RichTextRun{{Text: "hello"}}}
	shape := &Shape{
		shapeType: shapeRTF,
		Width:     200,
		TC: &ShapeTextConfig{
			TextMode:     TextModeWrap,
			RtfRuns:      &rt,
			RtfBaseStyle: glyph.TextStyle{Size: 12},
		},
	}
	// First call: cache miss, RtfFlatText gets populated.
	layoutWrapRTF(shape, shape.TC, w)
	if shape.TC.RtfFlatText == "" {
		t.Fatal("RtfFlatText should be set after first call")
	}
	// Overwrite to verify the cache-hit path respects the if-empty guard.
	shape.TC.RtfFlatText = "already-set"
	// Second call: cache hit (same shape.Width), should not overwrite.
	layoutWrapRTF(shape, shape.TC, w)
	if shape.TC.RtfFlatText != "already-set" {
		t.Errorf("RtfFlatText overwritten on cache hit, got %q",
			shape.TC.RtfFlatText)
	}
}
