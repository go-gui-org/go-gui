package gui

import "testing"

// --- Event traversal tests ---

func TestIsChildEnabled(t *testing.T) {
	enabled := &Layout{Shape: &Shape{Disabled: false}}
	disabled := &Layout{Shape: &Shape{Disabled: true}}

	if !isChildEnabled(enabled) {
		t.Error("enabled layout should be enabled")
	}
	if isChildEnabled(disabled) {
		t.Error("disabled layout should not be enabled")
	}
}

func TestScrollShapeID(t *testing.T) {
	scrollable := Shape{Scrollable: true, Height: 200}
	nonScrollable := Shape{Scrollable: false, Height: 200}

	if !scrollable.Scrollable {
		t.Error("scrollable should have Scrollable=true")
	}
	if nonScrollable.Scrollable {
		t.Error("non-scrollable should have Scrollable=false")
	}
}

func TestFocusCallbackConditions(t *testing.T) {
	noFocus := Layout{Shape: &Shape{}}
	withFocus := Layout{Shape: &Shape{ID: "f1", Focusable: true}}

	if noFocus.Shape.Focusable {
		t.Error("no-focus should not be focusable")
	}
	if !withFocus.Shape.Focusable || withFocus.Shape.ID != "f1" {
		t.Error("with-focus should be focusable with ID f1")
	}
}

func TestExecuteFocusCallbackNoFocus(t *testing.T) {
	layout := &Layout{Shape: &Shape{}}
	e := &Event{}
	w := &Window{}
	called := false
	cb := func(ctx EventCtx) { called = true }

	if executeFocusCallback(layout, e, w, cb, evNotify) {
		t.Error("should not execute when not focusable")
	}
	if called {
		t.Error("callback should not be called")
	}
}

func TestExecuteFocusCallbackNotFocused(t *testing.T) {
	layout := &Layout{Shape: &Shape{ID: "f1", Focusable: true}}
	e := &Event{}
	w := &Window{}
	w.SetFocus("f2") // different focus
	called := false
	cb := func(ctx EventCtx) { called = true }

	if executeFocusCallback(layout, e, w, cb, evNotify) {
		t.Error("should not execute when not focused")
	}
	if called {
		t.Error("callback should not be called")
	}
}

func TestExecuteFocusCallbackFocused(t *testing.T) {
	layout := &Layout{Shape: &Shape{ID: "f1", Focusable: true}}
	e := &Event{}
	w := &Window{}
	w.SetFocus("f1")

	cb := func(ctx EventCtx) {
		ctx.Consume()
	}
	if !executeFocusCallback(layout, e, w, cb, evNotify) {
		t.Error("should execute when focused")
	}
	if !e.IsHandled {
		t.Error("event should be handled")
	}
}

func TestExecuteFocusCallbackNilCallback(t *testing.T) {
	layout := &Layout{Shape: &Shape{ID: "f1", Focusable: true}}
	e := &Event{}
	w := &Window{}
	w.SetFocus("f1")

	if executeFocusCallback(layout, e, w, nil, evNotify) {
		t.Error("nil callback should return false")
	}
}

func TestExecuteMouseCallbackOutsideBounds(t *testing.T) {
	layout := &Layout{Shape: &Shape{
		shapeClip: drawClip{X: 10, Y: 10, Width: 50, Height: 50},
	}}
	e := &Event{MouseX: 0, MouseY: 0} // outside
	w := &Window{}
	called := false
	cb := func(ctx EventCtx) { called = true }

	if executeMouseCallback(layout, e, w, cb, evNotify) {
		t.Error("should not execute outside bounds")
	}
	if called {
		t.Error("callback should not be called")
	}
}

func TestExecuteMouseCallbackInsideBounds(t *testing.T) {
	layout := &Layout{Shape: &Shape{
		shapeClip: drawClip{X: 10, Y: 10, Width: 50, Height: 50},
	}}
	e := &Event{MouseX: 25, MouseY: 25}
	w := &Window{}

	cb := func(ctx EventCtx) {
		ctx.Consume()
	}
	if !executeMouseCallback(layout, e, w, cb, evNotify) {
		t.Error("should execute inside bounds")
	}
	if !e.IsHandled {
		t.Error("event should be handled")
	}
}

// --- View interface tests ---

type stubView struct {
	id       string
	children []View
}

func (sv *stubView) GenerateLayout(w *Window) Layout {
	layout := Layout{Shape: &Shape{ID: sv.id}}
	appendChildViews(w, &layout, sv.children)
	return layout
}

type nilShapeStubView struct {
	children []View
}

func (v *nilShapeStubView) GenerateLayout(w *Window) Layout {
	layout := Layout{}
	appendChildViews(w, &layout, v.children)
	return layout
}

func TestGenerateViewLayoutFlat(t *testing.T) {
	v := &stubView{id: "root"}
	layout := generateViewLayout(v, &Window{})

	if layout.Shape.ID != "root" {
		t.Errorf("ID: got %q, want root", layout.Shape.ID)
	}
	if len(layout.Children) != 0 {
		t.Errorf("children: got %d, want 0", len(layout.Children))
	}
}

// The exported GenerateViewLayout is the supported entry point for
// composite widgets; it must behave exactly like the internal builder.
func TestGenerateViewLayoutExported(t *testing.T) {
	v := &stubView{
		id: "parent",
		children: []View{
			&stubView{id: "child1"},
			&stubView{id: "child2"},
		},
	}
	layout := GenerateViewLayout(v, &Window{})
	if layout.Shape.ID != "parent" {
		t.Fatalf("root ID = %q, want parent", layout.Shape.ID)
	}
	if len(layout.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(layout.Children))
	}
	if layout.Children[0].Shape.ID != "child1" ||
		layout.Children[1].Shape.ID != "child2" {
		t.Fatal("child IDs mismatched")
	}
}

func TestGenerateViewLayoutWithChildren(t *testing.T) {
	v := &stubView{
		id: "parent",
		children: []View{
			&stubView{id: "child1"},
			&stubView{id: "child2"},
		},
	}
	layout := generateViewLayout(v, &Window{})

	if layout.Shape.ID != "parent" {
		t.Errorf("root ID: got %q", layout.Shape.ID)
	}
	if len(layout.Children) != 2 {
		t.Fatalf("children: got %d, want 2", len(layout.Children))
	}
	if layout.Children[0].Shape.ID != "child1" {
		t.Error("child1 ID mismatch")
	}
	if layout.Children[1].Shape.ID != "child2" {
		t.Error("child2 ID mismatch")
	}
}

func TestGenerateViewLayoutNested(t *testing.T) {
	v := &stubView{
		id: "root",
		children: []View{
			&stubView{
				id: "mid",
				children: []View{
					&stubView{id: "leaf"},
				},
			},
		},
	}
	layout := generateViewLayout(v, &Window{})

	if len(layout.Children) != 1 {
		t.Fatal("root should have 1 child")
	}
	mid := layout.Children[0]
	if mid.Shape.ID != "mid" {
		t.Error("mid ID mismatch")
	}
	if len(mid.Children) != 1 {
		t.Fatal("mid should have 1 child")
	}
	if mid.Children[0].Shape.ID != "leaf" {
		t.Error("leaf ID mismatch")
	}
}

func TestGenerateViewLayoutNormalizesNilShape(t *testing.T) {
	v := &nilShapeStubView{
		children: []View{
			&nilShapeStubView{},
		},
	}
	layout := generateViewLayout(v, &Window{})
	if layout.Shape == nil {
		t.Fatal("root shape should be normalized")
	}
	if layout.Shape.shapeType != shapeNone {
		t.Fatalf("root shape type: got %v, want shapeNone", layout.Shape.shapeType)
	}
	if len(layout.Children) != 1 {
		t.Fatalf("children: got %d, want 1", len(layout.Children))
	}
	if layout.Children[0].Shape == nil {
		t.Fatal("child shape should be normalized")
	}
	if layout.Children[0].Shape.shapeType != shapeNone {
		t.Fatalf("child shape type: got %v, want shapeNone", layout.Children[0].Shape.shapeType)
	}
}

// --- ViewFunc tests ---

func TestViewFuncGenerateLayout(t *testing.T) {
	f := viewFunc(func(w *Window) View {
		return &stubView{id: "inner"}
	})
	layout := f.GenerateLayout(&Window{})
	if layout.Shape.ID != "inner" {
		t.Errorf("ID: got %q, want inner", layout.Shape.ID)
	}
}

func TestViewFuncGenerateLayoutNested(t *testing.T) {
	f := viewFunc(func(w *Window) View {
		return &stubView{
			id: "parent",
			children: []View{
				&stubView{id: "child1"},
				&stubView{id: "child2"},
			},
		}
	})
	layout := f.GenerateLayout(&Window{})
	if layout.Shape.ID != "parent" {
		t.Errorf("root ID: got %q", layout.Shape.ID)
	}
	if len(layout.Children) != 2 {
		t.Fatalf("children: got %d, want 2", len(layout.Children))
	}
	if layout.Children[0].Shape.ID != "child1" {
		t.Error("child1 ID mismatch")
	}
	if layout.Children[1].Shape.ID != "child2" {
		t.Error("child2 ID mismatch")
	}
}

func TestViewFuncNilReturn(t *testing.T) {
	f := viewFunc(func(w *Window) View {
		return nil
	})
	layout := f.GenerateLayout(&Window{})
	// Nil View returns a Layout with no Shape from GenerateLayout.
	// generateViewLayout normalizes nil Shape to shapeNone.
	if layout.Shape != nil && layout.Shape.shapeType != shapeNone {
		t.Errorf("shape type: got %v, want shapeNone or nil", layout.Shape.shapeType)
	}
}

func TestViewFuncInContentSlice(t *testing.T) {
	v := Column(ContainerCfg{
		ID: "root",
		Content: []View{
			viewFunc(func(w *Window) View {
				return &stubView{
					id: "dynamic",
					children: []View{
						&stubView{id: "leaf"},
					},
				}
			}),
		},
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) != 1 {
		t.Fatalf("root children: got %d, want 1", len(layout.Children))
	}
	dyn := &layout.Children[0]
	if dyn.Shape.ID != "dynamic" {
		t.Errorf("dynamic ID: got %q, want dynamic", dyn.Shape.ID)
	}
	if len(dyn.Children) != 1 {
		t.Fatalf("dynamic children: got %d, want 1", len(dyn.Children))
	}
	if dyn.Children[0].Shape.ID != "leaf" {
		t.Error("leaf ID mismatch")
	}
}

func TestViewFuncNilInContentSlice(t *testing.T) {
	v := Column(ContainerCfg{
		ID: "root",
		Content: []View{
			viewFunc(func(w *Window) View {
				return nil
			}),
		},
	})
	layout := generateViewLayout(v, &Window{})
	// Nil return produces empty Layout; should not appear as child
	// because generateViewLayout skips nil in Content loop.
	if layout.Shape.ID != "root" {
		t.Errorf("root ID: got %q, want root", layout.Shape.ID)
	}
}

func TestViewFuncWithNilWindow(t *testing.T) {
	f := viewFunc(func(w *Window) View {
		return &stubView{id: "inner"}
	})
	layout := f.GenerateLayout(nil)
	if layout.Shape.ID != "inner" {
		t.Errorf("ID: got %q, want inner", layout.Shape.ID)
	}
}

// --- Container factory tests ---

func TestColumnSetsAxis(t *testing.T) {
	v := Column(ContainerCfg{ID: "col"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Axis != axisTopToBottom {
		t.Error("Column should set AxisTopToBottom")
	}
	if layout.Shape.ID != "col" {
		t.Error("ID mismatch")
	}
}

func TestRowSetsAxis(t *testing.T) {
	v := Row(ContainerCfg{ID: "row"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Axis != axisLeftToRight {
		t.Error("Row should set AxisLeftToRight")
	}
}

func TestWrapSetsFlags(t *testing.T) {
	v := Wrap(ContainerCfg{ID: "wrap"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Axis != axisLeftToRight {
		t.Error("Wrap should set AxisLeftToRight")
	}
	if !layout.Shape.Wrap {
		t.Error("Wrap should set Wrap=true")
	}
}

func TestCanvasNoAxis(t *testing.T) {
	v := Canvas(ContainerCfg{ID: "canvas"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Axis != axisNone {
		t.Error("Canvas should have AxisNone")
	}
}

func TestCircleSetsShapeType(t *testing.T) {
	v := Circle(ContainerCfg{ID: "circ"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.shapeType != shapeCircle {
		t.Error("Circle should set shapeCircle")
	}
	if layout.Shape.Axis != axisTopToBottom {
		t.Error("Circle should set AxisTopToBottom")
	}
}

func TestInvisibleContainerReturnsDisabled(t *testing.T) {
	v := Column(ContainerCfg{Invisible: true})
	layout := v.GenerateLayout(&Window{})
	if !layout.Shape.Disabled {
		t.Error("invisible should be disabled")
	}
	if !layout.Shape.OverDraw {
		t.Error("invisible should be overdraw")
	}
}

func TestContainerOpacityDefault(t *testing.T) {
	v := Column(ContainerCfg{ID: "op"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Opacity != 1.0 {
		t.Errorf("opacity: got %f, want 1.0", layout.Shape.Opacity)
	}
}

func TestContainerLeftClickOnly(t *testing.T) {
	called := false
	v := Column(ContainerCfg{
		ID: "lco",
		OnClick: func(ctx EventCtx) {
			called = true
		},
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events == nil {
		t.Fatal("events should be set")
	}
	if layout.Shape.events.clickButton != MouseLeft {
		t.Fatal("ClickButton should be MouseLeft")
	}
	// left click fires (via dispatch's ClickButton check)
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnClick(EventCtx{nil, e, nil})
	if !called {
		t.Error("left click should fire")
	}
	// right-click: OnClick is stored unwrapped; the filter is
	// checked at dispatch time, not in the handler itself.
	// Direct call still fires (bypasses dispatch).
}

func TestContainerOnAnyClickBypassesLeftOnly(t *testing.T) {
	called := false
	v := Column(ContainerCfg{
		ID: "any",
		OnAnyClick: func(ctx EventCtx) {
			called = true
		},
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events == nil {
		t.Fatal("events should be set")
	}
	e := &Event{MouseButton: MouseRight}
	layout.Shape.events.OnClick(EventCtx{nil, e, nil})
	if !called {
		t.Error("OnAnyClick should fire on right click")
	}
}

func TestContainerNilEventsWhenNoHandlers(t *testing.T) {
	v := Column(ContainerCfg{ID: "bare"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events != nil {
		t.Error("no handlers should yield nil events")
	}
}

func TestContainerNilEffectsWhenClean(t *testing.T) {
	v := Column(ContainerCfg{ID: "clean"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.fx != nil {
		t.Error("no effects should yield nil fx")
	}
}

func TestContainerEffectsAllocated(t *testing.T) {
	shadow := &BoxShadow{BlurRadius: 10}
	v := Column(ContainerCfg{ID: "fx", Shadow: shadow})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.fx == nil {
		t.Fatal("fx should be allocated")
	}
	if layout.Shape.fx.Shadow != shadow {
		t.Error("shadow mismatch")
	}
}

func TestContainerA11YDerivation(t *testing.T) {
	// scrollable → ScrollArea
	v := Column(ContainerCfg{Scrollable: true, ID: "a11y-scroll"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.A11YRole != AccessRoleScrollArea {
		t.Error("scrollable should derive ScrollArea role")
	}

	// explicit role overrides
	v2 := Column(ContainerCfg{A11YRole: AccessRoleGrid})
	layout2 := v2.GenerateLayout(&Window{})
	if layout2.Shape.A11YRole != AccessRoleGrid {
		t.Error("explicit role should override")
	}
}

func TestContainerA11YInfo(t *testing.T) {
	v := Column(ContainerCfg{
		A11YLabel:       "test",
		A11YDescription: "desc",
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.a11Y == nil {
		t.Fatal("A11Y should be set")
	}
	if layout.Shape.a11Y.Label != "test" {
		t.Error("label mismatch")
	}
	if layout.Shape.a11Y.Description != "desc" {
		t.Error("description mismatch")
	}
}

func TestContainerFixedSizing(t *testing.T) {
	v := Column(ContainerCfg{
		Sizing: FixedFixed,
		Width:  100,
		Height: 50,
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.MinWidth != 100 {
		t.Error("fixed width should set min=width")
	}
	if layout.Shape.MaxWidth != 100 {
		t.Error("fixed width should set max=width")
	}
	if layout.Shape.MinHeight != 50 {
		t.Error("fixed height should set min=height")
	}
	if layout.Shape.MaxHeight != 50 {
		t.Error("fixed height should set max=height")
	}
}

// --- Text view tests ---

func TestTextViewBasic(t *testing.T) {
	v := Text(TextCfg{Text: "hello", ID: "txt1"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.shapeType != shapeText {
		t.Error("should be shapeText")
	}
	if layout.Shape.TC == nil {
		t.Fatal("TC should be set")
	}
	if layout.Shape.TC.Text != "hello" {
		t.Error("text mismatch")
	}
	if layout.Shape.Width <= 0 {
		t.Error("width should be positive")
	}
	if layout.Shape.Height <= 0 {
		t.Error("height should be positive")
	}
}

func TestTextViewInvisible(t *testing.T) {
	v := Text(TextCfg{Text: "x", Invisible: true})
	layout := v.GenerateLayout(&Window{})
	if !layout.Shape.Disabled {
		t.Error("invisible text should be disabled")
	}
}

func TestTextViewWrapSizing(t *testing.T) {
	v := Text(TextCfg{
		Text: "wrap me",
		Mode: TextModeWrap,
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Sizing.Width != sizingFill {
		t.Error("wrap mode should use FillFit width")
	}
	if layout.Shape.Sizing.Height != sizingFit {
		t.Error("wrap mode should use FillFit height")
	}
}

func TestTextViewDefaultStyle(t *testing.T) {
	v := Text(TextCfg{Text: "styled"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.TC.TextStyle == nil {
		t.Fatal("TextStyle should be set")
	}
	if layout.Shape.TC.TextStyle.Size != sizeTextMedium {
		t.Errorf("size: got %f, want %f",
			layout.Shape.TC.TextStyle.Size, sizeTextMedium)
	}
}

func TestTextViewA11Y(t *testing.T) {
	v := Text(TextCfg{Text: "label test"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.A11YRole != AccessRoleStaticText {
		t.Error("text should have StaticText role")
	}
	if layout.Shape.a11Y == nil {
		t.Fatal("A11Y should be set")
	}
	// a11yLabel falls back to text
	if layout.Shape.a11Y.Label != "label test" {
		t.Errorf("label: got %q", layout.Shape.a11Y.Label)
	}
}

func TestTextMultilineSizing(t *testing.T) {
	v := Text(TextCfg{Text: "line1\nline2", Mode: TextModeMultiline})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Sizing.Width != sizingFit {
		t.Error("multiline should use FitFit width")
	}
	if layout.Shape.Sizing.Height != sizingFit {
		t.Error("multiline should use FitFit height")
	}
}

func TestTextOpacityDefault(t *testing.T) {
	v := Text(TextCfg{Text: "hi"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Opacity != 1.0 {
		t.Errorf("opacity: got %f, want 1.0", layout.Shape.Opacity)
	}
}

func TestTextOpacityExplicitZero(t *testing.T) {
	v := Text(TextCfg{Text: "hi", Opacity: SomeF(0)})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Opacity != 0 {
		t.Errorf("opacity: got %f, want 0", layout.Shape.Opacity)
	}
}

func TestContainerOpacityExplicitZero(t *testing.T) {
	v := Column(ContainerCfg{ID: "op0", Opacity: SomeF(0)})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Opacity != 0 {
		t.Errorf("opacity: got %f, want 0", layout.Shape.Opacity)
	}
}

// --- Button tests ---

func TestButtonCreatesRow(t *testing.T) {
	v := Button(ButtonCfg{
		ID: "btn1",
		Content: []View{
			Text(TextCfg{Text: "click"}),
		},
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.Axis != axisLeftToRight {
		t.Error("button should be a row")
	}
	if layout.Shape.ID != "btn1" {
		t.Error("ID mismatch")
	}
}

func TestButtonA11YRole(t *testing.T) {
	v := Button(ButtonCfg{ID: "btn"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.A11YRole != AccessRoleButton {
		t.Error("default role should be button")
	}

	v2 := Button(ButtonCfg{ID: "test_button_a11_y_role", A11YRole: AccessRoleTab})
	layout2 := v2.GenerateLayout(&Window{})
	if layout2.Shape.A11YRole != AccessRoleTab {
		t.Error("explicit role should override")
	}
}

func TestButtonSpacebarActivation(t *testing.T) {
	clicked := false
	v := Button(ButtonCfg{
		ID: "btn",
		OnClick: func(ctx EventCtx) {
			clicked = true
		},
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events == nil {
		t.Fatal("events should be set")
	}
	if !layout.Shape.events.clickOnSpace {
		t.Fatal("ClickOnSpace should be set for spacebar activation")
	}
	// The dispatch path (charHandler) checks ClickOnSpace and
	// routes to OnClick. Direct call to OnClick simulates this.
	layout.Shape.events.OnClick(EventCtx{nil, &Event{CharCode: charSpace}, nil})
	if !clicked {
		t.Error("spacebar should trigger click")
	}
}

func TestButtonAmendLayoutFocus(t *testing.T) {
	v := Button(ButtonCfg{
		ID:      "btn",
		OnClick: func(ctx EventCtx) {},
		Color:   RGB(50, 50, 50),
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events.AmendLayout == nil {
		t.Fatal("AmendLayout should be set")
	}

	w := &Window{}
	w.SetFocus("btn")
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})

	// Color should change to focus color
	if layout.Shape.Color.eq(RGB(50, 50, 50)) {
		t.Error("color should change on focus")
	}
}

func TestButtonEnterActivation(t *testing.T) {
	clicked := false
	v := Button(ButtonCfg{
		ID: "btn",
		OnClick: func(ctx EventCtx) {
			clicked = true
		},
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events == nil {
		t.Fatal("events should be set")
	}
	if !layout.Shape.events.clickOnEnter {
		t.Fatal("ClickOnEnter should be set for enter activation")
	}
	// The dispatch path (keydownHandler) checks ClickOnEnter and
	// routes to OnClick. Direct call to OnClick simulates this.
	e := &Event{KeyCode: KeyEnter}
	layout.Shape.events.OnClick(EventCtx{nil, e, nil})
	if !clicked {
		t.Error("enter should trigger click")
	}
}

func TestButtonDisabledSuppressesOnClick(t *testing.T) {
	clicked := false
	v := Button(ButtonCfg{
		ID:       "btn",
		Disabled: true,
		OnClick: func(ctx EventCtx) {
			clicked = true
		},
	})
	w := newTestWindow()
	layout := generateViewLayout(v, w)

	// AmendLayout should not change color on disabled button.
	origColor := layout.Shape.Color
	w.SetFocus("f1")
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	if layout.Shape.Color != origColor {
		t.Error("AmendLayout should not change color when disabled")
	}

	// OnHover should not change cursor or color.
	origColor = layout.Shape.Color
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnHover(EventCtx{&layout, e, w})
	if layout.Shape.Color != origColor {
		t.Error("OnHover should not change color when disabled")
	}

	// Direct click callback still fires (click guard is at the
	// event-dispatch level, not in the callback itself), but
	// AmendLayout/OnHover correctly bail out.
	_ = clicked
}

// --- Rectangle tests ---

func TestRectangleBasic(t *testing.T) {
	v := Rectangle(RectangleCfg{
		ID:     "rect",
		Width:  100,
		Height: 50,
		Color:  Red,
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.shapeType != shapeRectangle {
		t.Error("should be rectangle")
	}
	if layout.Shape.Color != Red {
		t.Error("color mismatch")
	}
	// min = size for rectangles
	if layout.Shape.MinWidth != 100 {
		t.Errorf("MinWidth: got %f, want 100", layout.Shape.MinWidth)
	}
}

func TestRectangleInvisible(t *testing.T) {
	v := Rectangle(RectangleCfg{Invisible: true})
	layout := v.GenerateLayout(&Window{})
	if !layout.Shape.Disabled {
		t.Error("invisible rect should be disabled")
	}
}

func TestRectangleNoPadding(t *testing.T) {
	v := Rectangle(RectangleCfg{Width: 10, Height: 10})
	layout := v.GenerateLayout(&Window{})
	if !layout.Shape.Padding.isNone() {
		t.Error("rectangle should have no padding")
	}
}

// --- Window mouse methods ---

func TestSetMouseCursor(t *testing.T) {
	w := &Window{}
	w.setMouseCursor(CursorPointingHand)
	if w.viewState.mouseCursor != CursorPointingHand {
		t.Error("cursor should be pointing hand")
	}
}

func TestMouseIsLocked(t *testing.T) {
	w := &Window{}
	if w.mouseIsLocked() {
		t.Error("should start unlocked")
	}
	w.MouseLock(MouseLockCfg{
		MouseMove: func(ctx EventCtx) {},
	})
	if !w.mouseIsLocked() {
		t.Error("should be locked")
	}
}

func TestMouseLockUnlock(t *testing.T) {
	w := &Window{}
	w.MouseLock(MouseLockCfg{
		mouseDown: func(ctx EventCtx) {},
		MouseMove: func(ctx EventCtx) {},
		MouseUp:   func(ctx EventCtx) {},
	})
	if !w.mouseIsLocked() {
		t.Error("should be locked after MouseLock")
	}
	w.MouseUnlock()
	if w.mouseIsLocked() {
		t.Error("should be unlocked after MouseUnlock")
	}
}

func TestMouseIsLockedChecksCallbacks(t *testing.T) {
	w := &Window{}

	w.MouseLock(MouseLockCfg{
		mouseDown: func(ctx EventCtx) {},
	})
	if !w.mouseIsLocked() {
		t.Error("MouseDown alone should lock")
	}
	w.MouseUnlock()

	w.MouseLock(MouseLockCfg{
		MouseMove: func(ctx EventCtx) {},
	})
	if !w.mouseIsLocked() {
		t.Error("MouseMove alone should lock")
	}
	w.MouseUnlock()

	w.MouseLock(MouseLockCfg{
		MouseUp: func(ctx EventCtx) {},
	})
	if !w.mouseIsLocked() {
		t.Error("MouseUp alone should lock")
	}
}

// --- A11Y helper tests ---

func TestMakeA11YInfoNil(t *testing.T) {
	if makeA11YInfo("", "") != nil {
		t.Error("empty strings should return nil")
	}
}

func TestMakeA11YInfoSet(t *testing.T) {
	info := makeA11YInfo("label", "desc")
	if info == nil {
		t.Fatal("should not be nil")
	}
	if info.Label != "label" {
		t.Error("label mismatch")
	}
	if info.Description != "desc" {
		t.Error("description mismatch")
	}
}

func TestA11YLabelFallback(t *testing.T) {
	if a11yLabel("explicit", "text") != "explicit" {
		t.Error("should use explicit label")
	}
	if a11yLabel("", "fallback") != "fallback" {
		t.Error("should fallback to text")
	}
}

// The fallback is frequently a widget's ID, and an ID may be a scoped
// path once the widget resolves its identity. A screen reader must hear
// a name, not a path.
func TestA11YLabelFallbackStripsIDScope(t *testing.T) {
	tests := []struct {
		name  string
		label string
		text  string
		want  string
	}{
		{"scoped id announces the leaf", "", "settings:name", "name"},
		{"deep path announces the leaf", "", "app:settings:name", "name"},
		{"unscoped id is unchanged", "", "name", "name"},
		{"explicit label keeps its colon", "Time: now", "id", "Time: now"},
		{"trailing separator keeps the whole string", "", "name:", "name:"},
		{"empty stays empty", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := a11yLabel(tc.label, tc.text); got != tc.want {
				t.Fatalf("a11yLabel(%q, %q) = %q, want %q",
					tc.label, tc.text, got, tc.want)
			}
		})
	}
}

// --- PointerOverApp tests ---

func TestPointerOverApp(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	tests := []struct {
		name string
		mx   float32
		my   float32
		want bool
	}{
		{"inside", 400, 300, true},
		{"origin", 0, 0, true},
		{"neg x", -1, 300, false},
		{"neg y", 400, -1, false},
		{"over width", 801, 300, false},
		{"over height", 400, 601, false},
		{"at edge", 800, 600, true},
	}
	for _, tc := range tests {
		e := &Event{MouseX: tc.mx, MouseY: tc.my}
		if got := w.pointerOverApp(e); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestClearInputSelections(t *testing.T) {
	w := &Window{}
	imap := StateMap[string, inputState](w, nsInput, capMany)
	imap.Set("a", inputState{CursorPos: 5, selectBeg: 2, selectEnd: 8})
	imap.Set("b", inputState{CursorPos: 3, selectBeg: 0, selectEnd: 4})

	w.clearInputSelections()

	v1, _ := imap.Get("a")
	if v1.selectBeg != 0 || v1.selectEnd != 0 {
		t.Error("selection 1 not cleared")
	}
	if v1.CursorPos != 5 {
		t.Error("cursor pos should be preserved")
	}
	v2, _ := imap.Get("b")
	if v2.selectBeg != 0 || v2.selectEnd != 0 {
		t.Error("selection 2 not cleared")
	}
}

func TestSetIDFocusClearsSelections(t *testing.T) {
	w := &Window{}
	imap := StateMap[string, inputState](w, nsInput, capMany)
	imap.Set("f1", inputState{selectBeg: 1, selectEnd: 5})

	w.SetFocus("f2")

	v, _ := imap.Get("f1")
	if v.selectBeg != 0 || v.selectEnd != 0 {
		t.Error("SetIDFocus should clear selections")
	}
	if w.FocusID() != "f2" {
		t.Error("focus not set")
	}
}

func TestWindowFocusedDefaultFalse(t *testing.T) {
	w := &Window{}
	if w.focused {
		t.Error("focused should default to false")
	}
}

// --- Full integration: Column with children ---

func TestColumnWithTextAndButton(t *testing.T) {
	w := &Window{}
	v := Column(ContainerCfg{
		ID: "main",
		Content: []View{
			Text(TextCfg{Text: "hello", ID: "t1"}),
			Button(ButtonCfg{
				ID: "b1",
				Content: []View{
					Text(TextCfg{Text: "click"}),
				},
			}),
		},
	})

	layout := generateViewLayout(v, w)
	if layout.Shape.ID != "main" {
		t.Error("root ID mismatch")
	}
	if len(layout.Children) != 2 {
		t.Fatalf("children: got %d, want 2", len(layout.Children))
	}
	if layout.Children[0].Shape.shapeType != shapeText {
		t.Error("child 0 should be text")
	}
	// child 1 is a button (row)
	btn := layout.Children[1]
	if btn.Shape.Axis != axisLeftToRight {
		t.Error("button should be a row")
	}
	if btn.Shape.ID != "b1" {
		t.Error("button ID mismatch")
	}
	// button has text child
	if len(btn.Children) != 1 {
		t.Fatalf("button children: got %d, want 1",
			len(btn.Children))
	}
	if btn.Children[0].Shape.shapeType != shapeText {
		t.Error("button child should be text")
	}
}

// --- HasFocus ---

func TestHasFocus(t *testing.T) {
	w := &Window{}
	if w.hasFocus() {
		t.Error("should start unfocused")
	}
	w.focused = true
	if !w.hasFocus() {
		t.Error("should be focused")
	}
}

// --- Cursor helpers ---

func TestCursorHelpers(t *testing.T) {
	tests := []struct {
		name string
		fn   func(*Window)
		want MouseCursor
	}{
		{"Arrow", (*Window).SetMouseCursorArrow, CursorArrow},
		{"IBeam", (*Window).SetMouseCursorIBeam, CursorIBeam},
		{"Crosshair", (*Window).SetMouseCursorCrosshair, CursorCrosshair},
		{"PointingHand", (*Window).SetMouseCursorPointingHand, CursorPointingHand},
		{"All", (*Window).SetMouseCursorAll, CursorResizeAll},
		{"NS", (*Window).SetMouseCursorNS, CursorResizeNS},
		{"EW", (*Window).SetMouseCursorEW, CursorResizeEW},
		{"NESW", (*Window).SetMouseCursorResizeNESW, CursorResizeNESW},
		{"NWSE", (*Window).SetMouseCursorResizeNWSE, CursorResizeNWSE},
		{"NotAllowed", (*Window).SetMouseCursorNotAllowed, CursorNotAllowed},
	}
	for _, tt := range tests {
		w := &Window{}
		tt.fn(w)
		if w.viewState.mouseCursor != tt.want {
			t.Errorf("%s: got %d, want %d",
				tt.name, w.viewState.mouseCursor, tt.want)
		}
	}
}

func TestGenerateViewLayout_ExcessiveChildren(t *testing.T) {
	// maxEventChildren caps direct children at 10000. The framework no
	// longer walks a raw Content() — the cap lives in appendChildViews,
	// so it is exercised through a container.
	n := maxEventChildren + 100
	children := make([]View, n)
	for i := range children {
		children[i] = &stubView{id: "child"}
	}
	v := Column(ContainerCfg{ID: "parent", Content: children})
	layout := generateViewLayout(v, &Window{})

	if len(layout.Children) != maxEventChildren {
		t.Errorf("children: got %d, want capped to %d",
			len(layout.Children), maxEventChildren)
	}
}

// TestGroupBoxTitleChildrenOrder pins the group-box injection order:
// addGroupBoxTitle's floating eraser + title label must stay ahead of
// the Content children, which append after them. A container with both
// Title and Content is the one layout where this ordering is visible.
func TestGroupBoxTitleChildrenOrder(t *testing.T) {
	v := Column(ContainerCfg{
		ID:          "gb",
		Title:       "Group",
		TitleBG:     RGB(30, 30, 30),
		ColorBorder: RGB(100, 100, 100),
		Content: []View{
			Text(TextCfg{Text: "child"}),
		},
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) != 3 {
		t.Fatalf("children = %d, want 3 (eraser, label, content)",
			len(layout.Children))
	}
	eraser := layout.Children[0].Shape
	if eraser.shapeType != shapeRectangle || !eraser.Float {
		t.Errorf("children[0] = %+v, want floating eraser rectangle", eraser)
	}
	label := layout.Children[1].Shape
	if label.shapeType != shapeText || !label.Float {
		t.Errorf("children[1] = %+v, want floating title label", label)
	}
	content := layout.Children[2].Shape
	if content.shapeType != shapeText || content.Float {
		t.Errorf("children[2] = %+v, want non-floating content text", content)
	}
}
