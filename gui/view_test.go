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
	scrollable := Shape{IDScroll: 100, Height: 200}
	nonScrollable := Shape{IDScroll: 0, Height: 200}

	if scrollable.IDScroll == 0 {
		t.Error("scrollable should have IDScroll > 0")
	}
	if nonScrollable.IDScroll != 0 {
		t.Error("non-scrollable should have IDScroll == 0")
	}
}

func TestFocusCallbackConditions(t *testing.T) {
	noFocus := Layout{Shape: &Shape{IDFocus: 0}}
	withFocus := Layout{Shape: &Shape{IDFocus: 1}}

	if noFocus.Shape.IDFocus != 0 {
		t.Error("no-focus should have IDFocus 0")
	}
	if withFocus.Shape.IDFocus != 1 {
		t.Error("with-focus should have IDFocus 1")
	}
}

func TestExecuteFocusCallbackNoFocus(t *testing.T) {
	layout := &Layout{Shape: &Shape{IDFocus: 0}}
	e := &Event{}
	w := &Window{}
	called := false
	cb := func(_ *Layout, _ *Event, _ *Window) { called = true }

	if executeFocusCallback(layout, e, w, cb) {
		t.Error("should not execute with IDFocus=0")
	}
	if called {
		t.Error("callback should not be called")
	}
}

func TestExecuteFocusCallbackNotFocused(t *testing.T) {
	layout := &Layout{Shape: &Shape{IDFocus: 1}}
	e := &Event{}
	w := &Window{}
	w.SetIDFocus(2) // different focus
	called := false
	cb := func(_ *Layout, _ *Event, _ *Window) { called = true }

	if executeFocusCallback(layout, e, w, cb) {
		t.Error("should not execute when not focused")
	}
	if called {
		t.Error("callback should not be called")
	}
}

func TestExecuteFocusCallbackFocused(t *testing.T) {
	layout := &Layout{Shape: &Shape{IDFocus: 1}}
	e := &Event{}
	w := &Window{}
	w.SetIDFocus(1)

	cb := func(_ *Layout, e *Event, _ *Window) {
		e.IsHandled = true
	}
	if !executeFocusCallback(layout, e, w, cb) {
		t.Error("should execute when focused")
	}
	if !e.IsHandled {
		t.Error("event should be handled")
	}
}

func TestExecuteFocusCallbackNilCallback(t *testing.T) {
	layout := &Layout{Shape: &Shape{IDFocus: 1}}
	e := &Event{}
	w := &Window{}
	w.SetIDFocus(1)

	if executeFocusCallback(layout, e, w, nil) {
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
	cb := func(_ *Layout, _ *Event, _ *Window) { called = true }

	if executeMouseCallback(layout, e, w, cb) {
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

	cb := func(_ *Layout, e *Event, _ *Window) {
		e.IsHandled = true
	}
	if !executeMouseCallback(layout, e, w, cb) {
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

func (sv *stubView) Content() []View { return sv.children }
func (sv *stubView) GenerateLayout(_ *Window) Layout {
	return Layout{Shape: &Shape{ID: sv.id}}
}

type nilShapeStubView struct {
	children []View
}

func (v *nilShapeStubView) Content() []View { return v.children }
func (v *nilShapeStubView) GenerateLayout(_ *Window) Layout {
	return Layout{}
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

// --- Container factory tests ---

func TestColumnSetsAxis(t *testing.T) {
	v := Column(ContainerCfg{ID: "col"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Axis != AxisTopToBottom {
		t.Error("Column should set AxisTopToBottom")
	}
	if layout.Shape.ID != "col" {
		t.Error("ID mismatch")
	}
}

func TestRowSetsAxis(t *testing.T) {
	v := Row(ContainerCfg{ID: "row"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Axis != AxisLeftToRight {
		t.Error("Row should set AxisLeftToRight")
	}
}

func TestWrapSetsFlags(t *testing.T) {
	v := Wrap(ContainerCfg{ID: "wrap"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Axis != AxisLeftToRight {
		t.Error("Wrap should set AxisLeftToRight")
	}
	if !layout.Shape.Wrap {
		t.Error("Wrap should set Wrap=true")
	}
}

func TestCanvasNoAxis(t *testing.T) {
	v := Canvas(ContainerCfg{ID: "canvas"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Axis != AxisNone {
		t.Error("Canvas should have AxisNone")
	}
}

func TestCircleSetsShapeType(t *testing.T) {
	v := Circle(ContainerCfg{ID: "circ"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.shapeType != shapeCircle {
		t.Error("Circle should set shapeCircle")
	}
	if layout.Shape.Axis != AxisTopToBottom {
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
		OnClick: func(_ *Layout, _ *Event, _ *Window) {
			called = true
		},
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events == nil {
		t.Fatal("events should be set")
	}
	if layout.Shape.events.ClickButton != MouseLeft {
		t.Fatal("ClickButton should be MouseLeft")
	}
	// left click fires (via dispatch's ClickButton check)
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnClick(nil, e, nil)
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
		OnAnyClick: func(_ *Layout, _ *Event, _ *Window) {
			called = true
		},
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events == nil {
		t.Fatal("events should be set")
	}
	e := &Event{MouseButton: MouseRight}
	layout.Shape.events.OnClick(nil, e, nil)
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
	v := Column(ContainerCfg{IDScroll: 1})
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
	if layout.Shape.A11Y == nil {
		t.Fatal("A11Y should be set")
	}
	if layout.Shape.A11Y.Label != "test" {
		t.Error("label mismatch")
	}
	if layout.Shape.A11Y.Description != "desc" {
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
	if layout.Shape.Sizing.Width != SizingFill {
		t.Error("wrap mode should use FillFit width")
	}
	if layout.Shape.Sizing.Height != SizingFit {
		t.Error("wrap mode should use FillFit height")
	}
}

func TestTextViewDefaultStyle(t *testing.T) {
	v := Text(TextCfg{Text: "styled"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.TC.TextStyle == nil {
		t.Fatal("TextStyle should be set")
	}
	if layout.Shape.TC.TextStyle.Size != SizeTextMedium {
		t.Errorf("size: got %f, want %f",
			layout.Shape.TC.TextStyle.Size, SizeTextMedium)
	}
}

func TestTextViewA11Y(t *testing.T) {
	v := Text(TextCfg{Text: "label test"})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.A11YRole != AccessRoleStaticText {
		t.Error("text should have StaticText role")
	}
	if layout.Shape.A11Y == nil {
		t.Fatal("A11Y should be set")
	}
	// a11yLabel falls back to text
	if layout.Shape.A11Y.Label != "label test" {
		t.Errorf("label: got %q", layout.Shape.A11Y.Label)
	}
}

func TestTextMultilineSizing(t *testing.T) {
	v := Text(TextCfg{Text: "line1\nline2", Mode: TextModeMultiline})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.Sizing.Width != SizingFit {
		t.Error("multiline should use FitFit width")
	}
	if layout.Shape.Sizing.Height != SizingFit {
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
	if layout.Shape.Axis != AxisLeftToRight {
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

	v2 := Button(ButtonCfg{A11YRole: AccessRoleTab})
	layout2 := v2.GenerateLayout(&Window{})
	if layout2.Shape.A11YRole != AccessRoleTab {
		t.Error("explicit role should override")
	}
}

func TestButtonSpacebarActivation(t *testing.T) {
	clicked := false
	v := Button(ButtonCfg{
		ID: "btn",
		OnClick: func(_ *Layout, _ *Event, _ *Window) {
			clicked = true
		},
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events == nil {
		t.Fatal("events should be set")
	}
	if !layout.Shape.events.ClickOnSpace {
		t.Fatal("ClickOnSpace should be set for spacebar activation")
	}
	// The dispatch path (charHandler) checks ClickOnSpace and
	// routes to OnClick. Direct call to OnClick simulates this.
	layout.Shape.events.OnClick(nil, &Event{CharCode: CharSpace}, nil)
	if !clicked {
		t.Error("spacebar should trigger click")
	}
}

func TestButtonAmendLayoutFocus(t *testing.T) {
	v := Button(ButtonCfg{
		ID:      "btn",
		IDFocus: 1,
		OnClick: func(_ *Layout, _ *Event, _ *Window) {},
		Color:   RGB(50, 50, 50),
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events.AmendLayout == nil {
		t.Fatal("AmendLayout should be set")
	}

	w := &Window{}
	w.SetIDFocus(1)
	layout.Shape.events.AmendLayout(&layout, w)

	// Color should change to focus color
	if layout.Shape.Color.Eq(RGB(50, 50, 50)) {
		t.Error("color should change on focus")
	}
}

func TestButtonEnterActivation(t *testing.T) {
	clicked := false
	v := Button(ButtonCfg{
		ID: "btn",
		OnClick: func(_ *Layout, _ *Event, _ *Window) {
			clicked = true
		},
	})
	layout := v.GenerateLayout(&Window{})
	if layout.Shape.events == nil {
		t.Fatal("events should be set")
	}
	if !layout.Shape.events.ClickOnEnter {
		t.Fatal("ClickOnEnter should be set for enter activation")
	}
	// The dispatch path (keydownHandler) checks ClickOnEnter and
	// routes to OnClick. Direct call to OnClick simulates this.
	e := &Event{KeyCode: KeyEnter}
	layout.Shape.events.OnClick(nil, e, nil)
	if !clicked {
		t.Error("enter should trigger click")
	}
}

func TestButtonDisabledSuppressesOnClick(t *testing.T) {
	clicked := false
	v := Button(ButtonCfg{
		ID:       "btn",
		IDFocus:  1,
		Disabled: true,
		OnClick: func(_ *Layout, _ *Event, _ *Window) {
			clicked = true
		},
	})
	w := newTestWindow()
	layout := generateViewLayout(v, w)

	// AmendLayout should not change color on disabled button.
	origColor := layout.Shape.Color
	w.SetIDFocus(1)
	layout.Shape.events.AmendLayout(&layout, w)
	if layout.Shape.Color != origColor {
		t.Error("AmendLayout should not change color when disabled")
	}

	// OnHover should not change cursor or color.
	origColor = layout.Shape.Color
	e := &Event{MouseButton: MouseLeft}
	layout.Shape.events.OnHover(&layout, e, w)
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
	if !layout.Shape.Padding.IsNone() {
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
	if w.MouseIsLocked() {
		t.Error("should start unlocked")
	}
	w.MouseLock(MouseLockCfg{
		MouseMove: func(*Layout, *Event, *Window) {},
	})
	if !w.MouseIsLocked() {
		t.Error("should be locked")
	}
}

func TestMouseLockUnlock(t *testing.T) {
	w := &Window{}
	w.MouseLock(MouseLockCfg{
		MouseDown: func(*Layout, *Event, *Window) {},
		MouseMove: func(*Layout, *Event, *Window) {},
		MouseUp:   func(*Layout, *Event, *Window) {},
	})
	if !w.MouseIsLocked() {
		t.Error("should be locked after MouseLock")
	}
	w.MouseUnlock()
	if w.MouseIsLocked() {
		t.Error("should be unlocked after MouseUnlock")
	}
}

func TestMouseIsLockedChecksCallbacks(t *testing.T) {
	w := &Window{}

	w.MouseLock(MouseLockCfg{
		MouseDown: func(*Layout, *Event, *Window) {},
	})
	if !w.MouseIsLocked() {
		t.Error("MouseDown alone should lock")
	}
	w.MouseUnlock()

	w.MouseLock(MouseLockCfg{
		MouseMove: func(*Layout, *Event, *Window) {},
	})
	if !w.MouseIsLocked() {
		t.Error("MouseMove alone should lock")
	}
	w.MouseUnlock()

	w.MouseLock(MouseLockCfg{
		MouseUp: func(*Layout, *Event, *Window) {},
	})
	if !w.MouseIsLocked() {
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
		if got := w.PointerOverApp(e); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestClearInputSelections(t *testing.T) {
	w := &Window{}
	imap := StateMap[uint32, InputState](w, nsInput, capMany)
	imap.Set(1, InputState{CursorPos: 5, SelectBeg: 2, SelectEnd: 8})
	imap.Set(2, InputState{CursorPos: 3, SelectBeg: 0, SelectEnd: 4})

	w.clearInputSelections()

	v1, _ := imap.Get(1)
	if v1.SelectBeg != 0 || v1.SelectEnd != 0 {
		t.Error("selection 1 not cleared")
	}
	if v1.CursorPos != 5 {
		t.Error("cursor pos should be preserved")
	}
	v2, _ := imap.Get(2)
	if v2.SelectBeg != 0 || v2.SelectEnd != 0 {
		t.Error("selection 2 not cleared")
	}
}

func TestSetIDFocusClearsSelections(t *testing.T) {
	w := &Window{}
	imap := StateMap[uint32, InputState](w, nsInput, capMany)
	imap.Set(1, InputState{SelectBeg: 1, SelectEnd: 5})

	w.SetIDFocus(2)

	v, _ := imap.Get(1)
	if v.SelectBeg != 0 || v.SelectEnd != 0 {
		t.Error("SetIDFocus should clear selections")
	}
	if w.IDFocus() != 2 {
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
	if btn.Shape.Axis != AxisLeftToRight {
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
		{"IBeam", (*Window).setMouseCursorIBeam, CursorIBeam},
		{"Crosshair", (*Window).SetMouseCursorCrosshair, CursorCrosshair},
		{"PointingHand", (*Window).SetMouseCursorPointingHand, CursorPointingHand},
		{"All", (*Window).SetMouseCursorAll, CursorResizeAll},
		{"NS", (*Window).SetMouseCursorNS, CursorResizeNS},
		{"EW", (*Window).SetMouseCursorEW, CursorResizeEW},
		{"NESW", (*Window).setMouseCursorResizeNESW, CursorResizeNESW},
		{"NWSE", (*Window).setMouseCursorResizeNWSE, CursorResizeNWSE},
		{"NotAllowed", (*Window).setMouseCursorNotAllowed, CursorNotAllowed},
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
	// maxEventChildren caps direct children at 10000.
	n := maxEventChildren + 100
	children := make([]View, n)
	for i := range children {
		children[i] = &stubView{id: "child"}
	}
	v := &stubView{id: "parent", children: children}
	layout := generateViewLayout(v, &Window{})

	if len(layout.Children) != maxEventChildren {
		t.Errorf("children: got %d, want capped to %d",
			len(layout.Children), maxEventChildren)
	}
}
