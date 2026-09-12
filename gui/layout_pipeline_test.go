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
			AmendLayout: func(ctx EventCtx) {
				order = append(order, "child")
			},
		},
		Opacity: 1,
	}
	parent := &Shape{
		shapeType: shapeRectangle,
		events: &eventHandlers{
			AmendLayout: func(ctx EventCtx) {
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
			OnHover: func(ctx EventCtx) {
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
			OnHover: func(ctx EventCtx) {
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
	w.MouseLock(MouseLockCfg{MouseMove: func(ctx EventCtx) {}})
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15

	shape := &Shape{
		shapeType: shapeRectangle,
		shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
		events: &eventHandlers{
			OnHover: func(ctx EventCtx) {},
		},
		Opacity: 1,
	}
	layout := Layout{Shape: shape}
	result := layoutHover(&layout, w)
	if result {
		t.Error("expected false when mouse locked")
	}
}

// TestLayoutHoverSynthesizesHeldMouseButton pins the hover pressed-state
// contract: the synthesized hover event reports the button the window
// learned from its own event stream, MouseInvalid when none is held.
// The window is driven through real dispatch (EventFn); the hover pass
// is the layoutHover call, matching the direct-call harness of the
// other layout-pipeline tests.
func TestLayoutHoverSynthesizesHeldMouseButton(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15

	got := MouseInvalid
	shape := &Shape{
		shapeType: shapeRectangle,
		shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
		events: &eventHandlers{
			OnHover: func(ctx EventCtx) { got = ctx.Event.MouseButton },
		},
		Opacity: 1,
	}
	layout := Layout{Shape: shape}
	hover := func() MouseButton {
		layoutHover(&layout, w)
		return got
	}

	// Regression pin: no press ever synthesizes a button.
	if btn := hover(); btn != MouseInvalid {
		t.Fatalf("no press: hover button = %v, want MouseInvalid", btn)
	}

	// Press: the next hover pass reports the held button.
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	if btn := hover(); btn != MouseLeft {
		t.Fatalf("left held: hover button = %v, want MouseLeft", btn)
	}

	// Release: back to MouseInvalid.
	w.EventFn(&Event{Type: EventMouseUp, MouseButton: MouseLeft})
	if btn := hover(); btn != MouseInvalid {
		t.Fatalf("after up: hover button = %v, want MouseInvalid", btn)
	}

	// A right-button hold is reported truthfully (D5): the widget
	// branches that check == MouseLeft fall through to hover colors.
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseRight})
	if btn := hover(); btn != MouseRight {
		t.Fatalf("right held: hover button = %v, want MouseRight", btn)
	}

	// Capture loss: MouseCancel clears without a mouse-up (D4), and
	// a fresh press re-arms the state.
	w.MouseCancel()
	if btn := hover(); btn != MouseInvalid {
		t.Fatalf("after cancel: hover button = %v, want MouseInvalid", btn)
	}
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	if btn := hover(); btn != MouseLeft {
		t.Fatalf("re-press: hover button = %v, want MouseLeft", btn)
	}
}

// TestLayoutHoverMouseLockedSilencesHeldButton pins D3: a press followed
// by a mouse lock (a drag started elsewhere) must never reach a hover
// callback, even though a button is held — the lock bail is what keeps
// pressed state out of drags.
func TestLayoutHoverMouseLockedSilencesHeldButton(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15

	fired := false
	shape := &Shape{
		shapeType: shapeRectangle,
		shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
		events: &eventHandlers{
			OnHover: func(ctx EventCtx) { fired = true },
		},
		Opacity: 1,
	}
	layout := Layout{Shape: shape}

	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	w.MouseLock(MouseLockCfg{MouseMove: func(EventCtx) {}})
	if layoutHover(&layout, w) {
		t.Error("hover fired while locked with a button held")
	}
	if fired {
		t.Error("OnHover ran while locked with a button held")
	}
}

// hoverButtonWindow returns a window whose layout tree is a plain
// gesture target (no handlers), with a local hover-recording layout
// ready for layoutHover, and a hover closure that reports the
// synthesized button. Shared by the gesture-path tests below.
func hoverButtonWindow() (w *Window, hover func() MouseButton) {
	w = NewTestWindow(WindowCfg{})
	w.viewState.mousePosX = 15
	w.viewState.mousePosY = 15
	got := MouseInvalid
	shape := &Shape{
		shapeType: shapeRectangle,
		shapeClip: drawClip{X: 0, Y: 0, Width: 50, Height: 50},
		events: &eventHandlers{
			OnHover: func(ctx EventCtx) { got = ctx.Event.MouseButton },
		},
		Opacity: 1,
	}
	layout := Layout{Shape: shape}
	w.layout = *gestureLayout(nil)
	return w, func() MouseButton {
		layoutHover(&layout, w)
		return got
	}
}

// TestTouchSynthesizedPressReportsHeldButton covers the gesture path:
// a touch press travels EventFn → handleTouch → handleTouchBegan →
// synthMouse(EventMouseDown), bypassing handleMouseDownEvent. It must
// record the held button just like a backend press, so the next hover
// pass carries it.
func TestTouchSynthesizedPressReportsHeldButton(t *testing.T) {
	w, hover := hoverButtonWindow()
	if btn := hover(); btn != MouseInvalid {
		t.Fatalf("before touch: hover button = %v, want MouseInvalid", btn)
	}
	w.handleTouch(&w.layout, touchEvent(EventTouchesBegan, 1, 15, 15))
	if btn := hover(); btn != MouseLeft {
		t.Fatalf("touch held: hover button = %v, want MouseLeft", btn)
	}
}

// TestTouchSynthesizedReleaseClearsHeldButton pins the mixed-input case
// that made the gesture path load-bearing: a backend mouse press sets
// the hold, and a touch release — synthesized by handleTouchEnded via
// synthMouse(EventMouseUp), which never passes through
// handleMouseUpEvent — must clear it. Without the clear, hover would
// keep reporting a left press no one is holding.
func TestTouchSynthesizedReleaseClearsHeldButton(t *testing.T) {
	w, hover := hoverButtonWindow()

	// Backend press: the real mouse button goes down.
	w.EventFn(&Event{Type: EventMouseDown, MouseButton: MouseLeft})
	if btn := hover(); btn != MouseLeft {
		t.Fatalf("mouse held: hover button = %v, want MouseLeft", btn)
	}

	// The user then interacts by touch; the finger's release is the
	// only release that arrives.
	w.handleTouch(&w.layout, touchEvent(EventTouchesBegan, 1, 15, 15))
	w.handleTouch(&w.layout, touchEvent(EventTouchesEnded, 1, 15, 15))
	if btn := hover(); btn != MouseInvalid {
		t.Fatalf("after touch release: hover button = %v, want MouseInvalid", btn)
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
			OnHover: func(ctx EventCtx) {
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
	// Simulate word wrapping using charWidth. maxW tracks the widest
	// line, which is what glyph reports as Layout.Width — the shrink in
	// layoutPlainText reads it, so the stub has to supply it.
	lines := 1
	var lineW, maxW float32
	start := 0
	for i := 0; i <= len(text); i++ {
		if i < len(text) && text[i] != ' ' && text[i] != '\n' {
			continue
		}
		wordW := float32(i-start) * m.charWidth
		if lineW > 0 && lineW+wordW > wrapWidth {
			lines++
			maxW = max(maxW, lineW)
			lineW = wordW
		} else {
			lineW += wordW
		}
		if i < len(text) && text[i] == '\n' {
			lines++
			maxW = max(maxW, lineW)
			lineW = 0
		} else if i < len(text) {
			if lineW > 0 {
				lineW += m.charWidth
			}
		}
		start = i + 1
	}
	maxW = max(maxW, lineW)
	return glyph.Layout{
		Width:  maxW,
		Height: float32(lines) * m.fontHeight,
	}, nil
}

func TestLayoutWrapPlainText(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}

	style := TextStyle{Size: 16}
	shape := &Shape{
		shapeType: shapeText,
		Width:     100, // fits ~10 chars per line
		TC: &shapeTextConfig{
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
			Axis:      axisTopToBottom,
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
			Axis:      axisTopToBottom,
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
		TC: &shapeTextConfig{
			Text:      longText,
			TextStyle: &style,
			TextMode:  TextModeWrap,
		},
	}
	col := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      axisTopToBottom,
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
		OnMouseLeave: func(ctx EventCtx) { left++ },
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
		OnMouseLeave: func(ctx EventCtx) { left++ },
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
		OnMouseLeave: func(ctx EventCtx) { left++ },
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
		OnHover:      func(ctx EventCtx) { entered++ },
		OnMouseLeave: func(ctx EventCtx) { left++ },
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

// --- layoutWrapRTF: RTFFlatText population ---

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
		TC: &shapeTextConfig{
			TextMode:     TextModeWrap,
			rTFRuns:      &rt,
			rTFBaseStyle: glyph.TextStyle{Size: 12},
		},
	}
	layoutWrapRTF(shape, shape.TC, w)

	want := "hello world"
	if shape.TC.rTFFlatText != want {
		t.Errorf("RTFFlatText = %q, want %q", shape.TC.rTFFlatText, want)
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
		TC: &shapeTextConfig{
			TextMode:     TextModeWrap,
			rTFRuns:      &rt,
			rTFBaseStyle: glyph.TextStyle{Size: 12},
		},
	}
	// First call: cache miss, RTFFlatText gets populated.
	layoutWrapRTF(shape, shape.TC, w)
	if shape.TC.rTFFlatText == "" {
		t.Fatal("RTFFlatText should be set after first call")
	}
	// Overwrite to verify the cache-hit path respects the if-empty guard.
	shape.TC.rTFFlatText = "already-set"
	// Second call: cache hit (same shape.Width), should not overwrite.
	layoutWrapRTF(shape, shape.TC, w)
	if shape.TC.rTFFlatText != "already-set" {
		t.Errorf("RTFFlatText overwritten on cache hit, got %q",
			shape.TC.rTFFlatText)
	}
}

// wrapHAlignLayout builds a fixed-size column with the given HAlign and
// one wrapped Text child, runs the whole pipeline, and hands back the
// text shape. The views are built through the factories on purpose:
// only Text knows whether the caller chose the Fill sizing or the wrap
// mode defaulted it, and the shrink in layoutPlainText reads that. #577
func wrapHAlignLayout(
	t *testing.T, hAlign HorizontalAlign, cfg TextCfg,
) (*Shape, *Shape) {
	t.Helper()
	w := &Window{}
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	w.windowWidth = 400
	w.windowHeight = 300

	// No padding and no border: the assertions below compare against
	// the column's own width, so the content box has to be the column.
	col := generateViewLayout(Column(ContainerCfg{
		Sizing:     FixedFixed,
		Width:      400,
		Height:     300,
		HAlign:     hAlign,
		Padding:    PaddingNone,
		SizeBorder: NoBorder,
		Content:    []View{Text(cfg)},
	}), w)
	col.Shape.MinWidth, col.Shape.MaxWidth = 400, 400
	col.Shape.MinHeight, col.Shape.MaxHeight = 300, 300
	layoutParents(&col, nil)
	layoutPipeline(&col, w)

	if len(col.Children) != 1 {
		t.Fatalf("column children = %d, want 1", len(col.Children))
	}
	return col.Shape, col.Children[0].Shape
}

// A short wrapped text in a centered column ends up centered: the box
// shrinks to its longest line, so HAlign has room to move it. #577
func TestWrapTextCentersInCenteredColumn(t *testing.T) {
	col, text := wrapHAlignLayout(t, HAlignCenter, TextCfg{
		Text:      "wrap me",
		TextStyle: TextStyle{Size: 16},
		Mode:      TextModeWrap,
	})

	if text.Width >= col.Width {
		t.Errorf("text width = %.1f, want < column width %.1f "+
			"(box did not shrink to its longest line)",
			text.Width, col.Width)
	}
	wantX := (col.Width - text.Width) / 2
	if abs32(text.X-wantX) > 1 {
		t.Errorf("text X = %.1f, want ~%.1f", text.X, wantX)
	}
}

// HAlignRight pushes the shrunken box to the right edge. #577
func TestWrapTextRightAlignsInRightAlignedColumn(t *testing.T) {
	col, text := wrapHAlignLayout(t, HAlignRight, TextCfg{
		Text:      "wrap me",
		TextStyle: TextStyle{Size: 16},
		Mode:      TextModeWrap,
	})

	wantX := col.Width - text.Width
	if abs32(text.X-wantX) > 1 {
		t.Errorf("text X = %.1f, want ~%.1f", text.X, wantX)
	}
}

// The common case is untouched: a left-aligned column leaves the
// wrapped box spanning the full content width, so a background or a
// border behind it keeps the extent it has always had. #577
func TestWrapTextKeepsFullWidthWhenLeftAligned(t *testing.T) {
	col, text := wrapHAlignLayout(t, HAlignLeft, TextCfg{
		Text:      "wrap me",
		TextStyle: TextStyle{Size: 16},
		Mode:      TextModeWrap,
	})

	if abs32(text.Width-col.Width) > 1 {
		t.Errorf("text width = %.1f, want ~%.1f (full width)",
			text.Width, col.Width)
	}
	if abs32(text.X) > 1 {
		t.Errorf("text X = %.1f, want ~0", text.X)
	}
}

// An explicit Sizing from the caller is an instruction, not a default:
// a caller who asked for FillFit keeps the full-width box. #577
func TestWrapTextExplicitFillKeepsFullWidth(t *testing.T) {
	col, text := wrapHAlignLayout(t, HAlignCenter, TextCfg{
		Text:      "wrap me",
		TextStyle: TextStyle{Size: 16},
		Mode:      TextModeWrap,
		Sizing:    FillFit,
	})

	if abs32(text.Width-col.Width) > 1 {
		t.Errorf("text width = %.1f, want ~%.1f (explicit FillFit)",
			text.Width, col.Width)
	}
}

// TextStyle.Align already centers the lines inside the full-width box.
// Shrinking there would move the box away from the offsets glyph
// computed against the wrap width, so that case opts out. #577
func TestWrapTextWithTextAlignKeepsFullWidth(t *testing.T) {
	col, text := wrapHAlignLayout(t, HAlignCenter, TextCfg{
		Text:      "wrap me",
		TextStyle: TextStyle{Size: 16, Align: TextAlignCenter},
		Mode:      TextModeWrap,
	})

	if abs32(text.Width-col.Width) > 1 {
		t.Errorf("text width = %.1f, want ~%.1f (TextStyle.Align set)",
			text.Width, col.Width)
	}
}

// Text long enough to fill every line has no slack to give back, so the
// box stays full width and nothing moves. #577
func TestWrapTextLongTextStaysFullWidth(t *testing.T) {
	long := "wrapping text that is long enough to fill every single " +
		"line of the box it is given and then some more words"
	col, text := wrapHAlignLayout(t, HAlignCenter, TextCfg{
		Text:      long,
		TextStyle: TextStyle{Size: 16},
		Mode:      TextModeWrap,
	})

	if text.Width > col.Width+1 {
		t.Errorf("text width = %.1f, want <= %.1f",
			text.Width, col.Width)
	}
	if text.Height <= 20 {
		t.Errorf("text height = %.1f, want > 20 (wrapped)",
			text.Height)
	}
}

// HAlign on a row is the main axis: it places the children as a group,
// so narrowing one of them would open a gap rather than move the text.
// A row parent therefore leaves the wrapped box alone. #577
func TestWrapTextInRowKeepsFullWidth(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	w.windowWidth = 400
	w.windowHeight = 300

	row := generateViewLayout(Row(ContainerCfg{
		Sizing:     FixedFixed,
		Width:      400,
		Height:     300,
		HAlign:     HAlignCenter,
		Padding:    PaddingNone,
		SizeBorder: NoBorder,
		Content: []View{Text(TextCfg{
			Text:      "wrap me",
			TextStyle: TextStyle{Size: 16},
			Mode:      TextModeWrap,
		})},
	}), w)
	row.Shape.MinWidth, row.Shape.MaxWidth = 400, 400
	row.Shape.MinHeight, row.Shape.MaxHeight = 300, 300
	layoutParents(&row, nil)
	layoutPipeline(&row, w)

	text := row.Children[0].Shape
	if abs32(text.Width-row.Shape.Width) > 1 {
		t.Errorf("text width = %.1f, want ~%.1f (row parent)",
			text.Width, row.Shape.Width)
	}
}

// A float is placed by its anchor, not by the alignment of the
// container it was declared in, so it keeps its box. #577
func TestShrinkWrapToInkSkipsFloat(t *testing.T) {
	tc := &shapeTextConfig{
		Text:              "wrap me",
		TextMode:          TextModeWrap,
		wrapSizingDefault: true,
	}
	shape := &Shape{Width: 400, Sizing: FillFit, TC: tc, Float: true}
	l := glyph.Layout{Width: 70, Height: 20}

	if shrinkWrapToInk(shape, tc, TextStyle{}, l, HAlignCenter) {
		t.Fatal("shrinkWrapToInk shrank a float box")
	}
	if shape.Width != 400 {
		t.Errorf("width = %.1f, want 400", shape.Width)
	}

	// Same shape, not floating: the shrink applies.
	shape.Float = false
	if !shrinkWrapToInk(shape, tc, TextStyle{}, l, HAlignCenter) {
		t.Fatal("shrinkWrapToInk skipped a non-float box")
	}
	if shape.Width != 70 {
		t.Errorf("width = %.1f, want 70", shape.Width)
	}
}

// Input's text shape is a scroll viewport the caret moves within, not
// a measurement of the text, so it never shrinks even when every other
// gate would allow it. #577
func TestShrinkWrapToInkSkipsOverflowScrollX(t *testing.T) {
	tc := &shapeTextConfig{
		Text:              "wrap me",
		TextMode:          TextModeWrap,
		wrapSizingDefault: true,
		overflowScrollX:   true,
	}
	shape := &Shape{Width: 400, Sizing: FillFit, TC: tc}
	l := glyph.Layout{Width: 70, Height: 20}

	if shrinkWrapToInk(shape, tc, TextStyle{}, l, HAlignCenter) {
		t.Fatal("shrinkWrapToInk shrank an overflowScrollX box")
	}
	if shape.Width != 400 {
		t.Errorf("width = %.1f, want 400", shape.Width)
	}
}

// TextModeWrapKeepSpaces takes the same shrink path as TextModeWrap:
// a short kept-spaces text in a centered column ends up centered. #577
func TestWrapTextWrapKeepSpacesCentersInCenteredColumn(t *testing.T) {
	col, text := wrapHAlignLayout(t, HAlignCenter, TextCfg{
		Text:      "wrap me",
		TextStyle: TextStyle{Size: 16},
		Mode:      TextModeWrapKeepSpaces,
	})

	if text.Width >= col.Width {
		t.Errorf("text width = %.1f, want < column width %.1f "+
			"(box did not shrink to its longest line)",
			text.Width, col.Width)
	}
	wantX := (col.Width - text.Width) / 2
	if abs32(text.X-wantX) > 1 {
		t.Errorf("text X = %.1f, want ~%.1f", text.X, wantX)
	}
}

// A shrink under a scrolling parent refreshes that parent's contentW
// cache, which the fill pass took before the glyph layout existed.
// Without the refresh the scroll clamp and scrollbar thumb read the
// stale full width. #577
func TestWrapTextShrinkRefreshesScrollableContentW(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	w.windowWidth = 400
	w.windowHeight = 300

	col := generateViewLayout(Column(ContainerCfg{
		ID:         "wrap-scroll",
		Sizing:     FixedFixed,
		Width:      400,
		Height:     300,
		HAlign:     HAlignCenter,
		Padding:    PaddingNone,
		SizeBorder: NoBorder,
		Scrollable: true,
		Content: []View{Text(TextCfg{
			Text:      "wrap me",
			TextStyle: TextStyle{Size: 16},
			Mode:      TextModeWrap,
		})},
	}), w)
	col.Shape.MinWidth, col.Shape.MaxWidth = 400, 400
	col.Shape.MinHeight, col.Shape.MaxHeight = 300, 300
	layoutParents(&col, nil)
	layoutPipeline(&col, w)

	// The scrollbars append after the content, so the text is first.
	text := col.Children[0].Shape
	if text.Width >= col.Shape.Width {
		t.Fatalf("text width = %.1f, want < column width %.1f "+
			"(box did not shrink to its longest line)",
			text.Width, col.Shape.Width)
	}
	if abs32(col.Shape.contentW-text.Width) > 1 {
		t.Errorf("contentW = %.1f, want ~%.1f (stale cache)",
			col.Shape.contentW, text.Width)
	}
}
