package gui

// rtf_select_test.go covers the pure helpers and guard branches of the
// standalone selectable RTF widget (view_rtf.go / view_rtf_select.go)
// with direct calls — no layout tree, no event dispatch. Anything that
// needs a rendered window, hit testing or the mouse lock lives in the
// sibling interaction file (rtf_select_interaction_test.go).

import (
	"slices"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// --- rtfRuneCountFromRuns ---

func TestRtfRuneCountFromRuns_Nil(t *testing.T) {
	if got := rtfRuneCountFromRuns(nil); got != 0 {
		t.Errorf("nil RichText counted %d runes, want 0", got)
	}
}

func TestRtfRuneCountFromRuns_MultiRunUTF8(t *testing.T) {
	rt := &RichText{Runs: []RichTextRun{
		{Text: "héllo"},  // 5 runes, 6 bytes
		{Text: " wörld"}, // 6 runes, 7 bytes
	}}
	if got := rtfRuneCountFromRuns(rt); got != 11 {
		t.Errorf("rune count = %d, want 11 across runs", got)
	}
}

// --- rtfFindRunAtIndex ---

func TestRtfFindRunAtIndex_Guards(t *testing.T) {
	if got := rtfFindRunAtIndex(nil, 0); got != (RichTextRun{}) {
		t.Error("nil layout must return an empty run")
	}
	l := &Layout{Shape: &Shape{}}
	if got := rtfFindRunAtIndex(l, 0); got != (RichTextRun{}) {
		t.Error("layout without TC must return an empty run")
	}
	l = &Layout{Shape: &Shape{TC: &shapeTextConfig{}}}
	if got := rtfFindRunAtIndex(l, 0); got != (RichTextRun{}) {
		t.Error("layout with nil runs must return an empty run")
	}
}

func TestRtfFindRunAtIndex_Hit(t *testing.T) {
	rt := &RichText{Runs: []RichTextRun{
		{Text: "one"},
		{Text: "two"},
		{Text: "three"},
	}}
	l := &Layout{Shape: &Shape{TC: &shapeTextConfig{rTFRuns: rt}}}
	if got := rtfFindRunAtIndex(l, 0); got.Text != "one" {
		t.Errorf("index 0 = %q, want first run", got.Text)
	}
	// 't' of "two": run 1 spans bytes [3,6).
	if got := rtfFindRunAtIndex(l, 4); got.Text != "two" {
		t.Errorf("index 4 = %q, want second run", got.Text)
	}
	// 'r' of "three": run 2 spans bytes [6,11).
	if got := rtfFindRunAtIndex(l, 9); got.Text != "three" {
		t.Errorf("index 9 = %q, want third run", got.Text)
	}
}

func TestRtfFindRunAtIndex_PastEnd(t *testing.T) {
	rt := &RichText{Runs: []RichTextRun{{Text: "one"}}}
	l := &Layout{Shape: &Shape{TC: &shapeTextConfig{rTFRuns: rt}}}
	if got := rtfFindRunAtIndex(l, 99); got != (RichTextRun{}) {
		t.Errorf("index past the last run = %q, want empty run", got.Text)
	}
}

// --- rtfSuppressInlineObjectGlyphs ---

func TestRtfSuppressInlineObjectGlyphs_NilLayout(t *testing.T) {
	rtfSuppressInlineObjectGlyphs(nil) // must not panic
}

// --- rtfStyleKey ---

func TestRtfStyleKey_Stable(t *testing.T) {
	s := glyph.TextStyle{
		FontName: "Helvetica", Typeface: glyph.TypefaceItalic,
		Size: 14, LetterSpacing: 0.5,
	}
	if k1, k2 := rtfStyleKey(s), rtfStyleKey(s); k1 != k2 {
		t.Errorf("same style hashed to %d then %d", k1, k2)
	}
}

func TestRtfStyleKey_DistinctFields(t *testing.T) {
	base := glyph.TextStyle{
		FontName: "Helvetica", Typeface: glyph.TypefaceRegular,
		Size: 14, LetterSpacing: 0.5,
	}
	variants := []struct {
		name  string
		style glyph.TextStyle
	}{
		{"font name", func() glyph.TextStyle {
			s := base
			s.FontName = "Arial"
			return s
		}()},
		{"typeface", func() glyph.TextStyle {
			s := base
			s.Typeface = glyph.TypefaceBold
			return s
		}()},
		{"size", func() glyph.TextStyle {
			s := base
			s.Size = 16
			return s
		}()},
		{"letter spacing", func() glyph.TextStyle {
			s := base
			s.LetterSpacing = 1.0
			return s
		}()},
	}
	baseKey := rtfStyleKey(base)
	for _, v := range variants {
		if k := rtfStyleKey(v.style); k == baseKey {
			t.Errorf("different %s produced the same cache key", v.name)
		}
	}
}

// --- rtfTooltipAnimation ---

func TestRtfTooltipAnimation_Activation(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip = tooltipState{hoverID: "tip", text: "tip text"}
	a := rtfTooltipAnimation("tip")
	if a.AnimID != "___tooltip___" {
		t.Errorf("AnimID = %q, want the shared tooltip id", a.AnimID)
	}
	a.Callback(a, w)
	ts := &w.viewState.tooltip
	if ts.id != "tip" {
		t.Errorf("id = %q, want activated tooltip", ts.id)
	}
	if ts.popupID != ScopeID("tip", "rtf_popup") {
		t.Errorf("popupID = %q, want %q",
			ts.popupID, ScopeID("tip", "rtf_popup"))
	}
}

func TestRtfTooltipAnimation_StaleHoverNoActivation(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip = tooltipState{hoverID: "other", text: "tip text"}
	a := rtfTooltipAnimation("tip")
	a.Callback(a, w)
	if w.viewState.tooltip.id != "" {
		t.Error("stale hover activated the tooltip")
	}
}

func TestRtfTooltipAnimation_EmptyTextNoActivation(t *testing.T) {
	w := &Window{}
	w.viewState.tooltip = tooltipState{hoverID: "tip", text: ""}
	a := rtfTooltipAnimation("tip")
	a.Callback(a, w)
	if w.viewState.tooltip.id != "" {
		t.Error("empty tooltip text still activated the popup")
	}
}

// --- rtfMouseMove: repeated hover on the same tooltip run ---

func TestRtfMouseMove_TooltipHoverRepeatedConsumes(t *testing.T) {
	w := &Window{}
	rt := RichText{Runs: []RichTextRun{
		{Text: "hello", Tooltip: "tip text"},
	}}
	glyphLayout := glyph.Layout{
		Items: []glyph.Item{
			{
				X: 10, Y: 12, Width: 40,
				Ascent: 12, Descent: 4,
				StartIndex: 0,
			},
		},
	}
	l := &Layout{
		Shape: &Shape{
			shapeType: shapeRTF,
			X:         100, Y: 200,
			TC: &shapeTextConfig{
				rTFLayout: &glyphLayout,
				rTFRuns:   &rt,
			},
		},
	}
	// First hover arms the tooltip; the second must take the
	// already-hovering branch: consume without touching state.
	e := &Event{MouseX: 20, MouseY: 5}
	rtfMouseMove(EventCtx{Layout: l, Event: e, Window: w})
	if !e.IsHandled || w.viewState.tooltip.hoverID != "tip text" {
		t.Fatalf("first hover: handled=%v hoverID=%q",
			e.IsHandled, w.viewState.tooltip.hoverID)
	}
	e = &Event{MouseX: 25, MouseY: 5}
	rtfMouseMove(EventCtx{Layout: l, Event: e, Window: w})
	if !e.IsHandled {
		t.Error("repeat hover over the same tooltip run was not consumed")
	}
	if got := w.viewState.tooltip.hoverID; got != "tip text" {
		t.Errorf("repeat hover changed hoverID to %q", got)
	}
}

// --- rtfSelectAmendLayout ---

func TestRtfSelectAmendLayout_StampsSelection(t *testing.T) {
	w := &Window{}
	StateMap[string, inputState](w, nsInput, capMany).Set(
		"rtf", inputState{selectBeg: 2, selectEnd: 7})
	ly := &Layout{Shape: &Shape{
		ID: "rtf", Focusable: true,
		TC: &shapeTextConfig{},
	}}
	rtfSelectAmendLayout(EventCtx{Layout: ly, Window: w})
	if ly.Shape.TC.textSelBeg != 2 || ly.Shape.TC.textSelEnd != 7 {
		t.Errorf("stamps = [%d,%d), want [2,7)",
			ly.Shape.TC.textSelBeg, ly.Shape.TC.textSelEnd)
	}
}

func TestRtfSelectAmendLayout_GuardsSkip(t *testing.T) {
	w := &Window{}
	StateMap[string, inputState](w, nsInput, capMany).Set(
		"rtf", inputState{selectBeg: 2, selectEnd: 7})

	// Empty ID: the shape cannot join focus traversal, so no stamping.
	ly := &Layout{Shape: &Shape{Focusable: true, TC: &shapeTextConfig{}}}
	rtfSelectAmendLayout(EventCtx{Layout: ly, Window: w})
	if ly.Shape.TC.textSelBeg != 0 || ly.Shape.TC.textSelEnd != 0 {
		t.Error("empty-ID shape was stamped")
	}

	// Not focusable.
	ly = &Layout{Shape: &Shape{ID: "rtf", TC: &shapeTextConfig{}}}
	rtfSelectAmendLayout(EventCtx{Layout: ly, Window: w})
	if ly.Shape.TC.textSelBeg != 0 || ly.Shape.TC.textSelEnd != 0 {
		t.Error("non-focusable shape was stamped")
	}

	// No TC.
	ly = &Layout{Shape: &Shape{ID: "rtf", Focusable: true}}
	rtfSelectAmendLayout(EventCtx{Layout: ly, Window: w}) // must not panic
}

// --- rtfSelectOnClick guards ---

func TestRtfSelectOnClick_GuardInert(t *testing.T) {
	cases := []struct {
		name string
		sh   *Shape
	}{
		{"nil TC", &Shape{shapeType: shapeRTF, ID: "rtf", Focusable: true}},
		{"no rtf layout", &Shape{shapeType: shapeRTF, ID: "rtf",
			Focusable: true, TC: &shapeTextConfig{}}},
		{"empty ID", &Shape{shapeType: shapeRTF, Focusable: true,
			TC: &shapeTextConfig{
				rTFLayout: &glyph.Layout{},
				rTFRuns:   &RichText{},
			}}},
		{"not focusable", &Shape{shapeType: shapeRTF, ID: "rtf",
			TC: &shapeTextConfig{
				rTFLayout: &glyph.Layout{},
				rTFRuns:   &RichText{},
			}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := &Window{}
			e := &Event{Type: EventMouseDown, MouseButton: MouseLeft,
				MouseX: 5, MouseY: 5}
			rtfSelectOnClick(EventCtx{Layout: &Layout{Shape: c.sh},
				Event: e, Window: w})
			if e.IsHandled {
				t.Error("guard shape consumed the click")
			}
			if w.FocusID() != "" {
				t.Errorf("guard shape took focus %q", w.FocusID())
			}
		})
	}
}

// --- rtfSelectOnKeyDown guards and unhandled keys ---

func TestRtfSelectOnKeyDown_GuardInert(t *testing.T) {
	gl := glyph.Layout{Text: "hello"}
	cases := []struct {
		name    string
		sh      *Shape
		focused bool
	}{
		{"nil TC", &Shape{shapeType: shapeRTF, ID: "rtf", Focusable: true},
			true},
		{"empty ID", &Shape{shapeType: shapeRTF, Focusable: true,
			TC: &shapeTextConfig{rTFFlatText: "hello",
				rTFLayout: &gl}}, true},
		{"not focusable", &Shape{shapeType: shapeRTF, ID: "rtf",
			TC: &shapeTextConfig{rTFFlatText: "hello",
				rTFLayout: &gl}}, true},
		{"not focused", &Shape{shapeType: shapeRTF, ID: "rtf",
			Focusable: true, TC: &shapeTextConfig{rTFFlatText: "hello",
				rTFLayout: &gl}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := &Window{}
			if c.focused {
				w.SetFocus(c.sh.ID)
			}
			e := &Event{Type: EventKeyDown, KeyCode: KeyA,
				Modifiers: ModCtrl}
			rtfSelectOnKeyDown(EventCtx{Layout: &Layout{Shape: c.sh},
				Event: e, Window: w})
			if e.IsHandled {
				t.Error("guard shape consumed the key")
			}
			if is := StateReadOr(w, nsInput, c.sh.ID,
				inputState{}); is.selectEnd != 0 {
				t.Errorf("guard shape selection = [%d,%d), want untouched",
					is.selectBeg, is.selectEnd)
			}
		})
	}
}

// TestRtfSelectOnKeyDown_SingleLineVerticalKeysUnhandled pins the
// single-line behavior: Up/Down are not handled (textKeyVertical
// declines outside TextModeWrap), so the event travels on without
// moving the cursor.
func TestRtfSelectOnKeyDown_SingleLineVerticalKeysUnhandled(t *testing.T) {
	w := &Window{}
	w.SetFocus("rtf")
	StateMap[string, inputState](w, nsInput, capMany).Set(
		"rtf", inputState{CursorPos: 2})
	gl := glyph.Layout{Text: "hello world"}
	sh := &Shape{shapeType: shapeRTF, ID: "rtf", Focusable: true,
		TC: &shapeTextConfig{rTFFlatText: "hello world",
			rTFLayout: &gl, TextMode: TextModeSingleLine}}
	for _, key := range []KeyCode{KeyUp, KeyDown} {
		e := &Event{Type: EventKeyDown, KeyCode: key}
		rtfSelectOnKeyDown(EventCtx{Layout: &Layout{Shape: sh},
			Event: e, Window: w})
		if e.IsHandled {
			t.Errorf("KeyUp/KeyDown handled in single-line mode")
		}
	}
	if is := StateReadOr(w, nsInput, "rtf", inputState{}); is.CursorPos != 2 {
		t.Errorf("cursor = %d, want 2 (vertical key moved it)",
			is.CursorPos)
	}
}

// --- rtfLinkMenuView structure ---

func TestRtfLinkMenuView_Structure(t *testing.T) {
	w := newTestWindow()
	v := rtfLinkMenuView(w, rtfLinkMenuState{
		Link: "https://example.com", X: 40, Y: 60, Open: true,
	})
	layout := generateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("no menu shape")
	}
	sh := layout.Shape
	if sh.ID != rtfLinkMenuFocusID {
		t.Errorf("menu ID = %q, want %q", sh.ID, rtfLinkMenuFocusID)
	}
	if !sh.Float {
		t.Error("menu must float at the click position")
	}
	if !sh.Focusable {
		t.Error("menu must be focusable to receive keyboard navigation")
	}
	// The item leaves ("open_link", "copy_link") resolve under the
	// menu's absolute ID; without an arrange pass their effID is not
	// stamped, so walk the menu subtree for the leaf IDs instead.
	var itemIDs []string
	var walk func(*Layout)
	walk = func(l *Layout) {
		if l.Shape != nil && l.Shape.ID != "" {
			itemIDs = append(itemIDs, l.Shape.ID)
		}
		for i := range l.Children {
			walk(&l.Children[i])
		}
	}
	walk(&layout)
	for _, want := range []string{"open_link", "copy_link"} {
		if !slices.Contains(itemIDs, want) {
			t.Errorf("menu missing item %q (leaf IDs: %v)", want, itemIDs)
		}
	}
}

// TestRtfGenerateLayout_MenuOnlyOnOwningBlock renders a block whose
// content does not match the open menu's BlockKey: the popup must not
// be attached, because the state belongs to a different RTF block.
func TestRtfGenerateLayout_MenuOnlyOnOwningBlock(t *testing.T) {
	w := &Window{}
	rt := RichText{Runs: []RichTextRun{{Text: "hello"}}}
	StateMap[string, rtfLinkMenuState](w, nsRtfLinkMenu,
		capFew).Set(nsRtfLinkMenu, rtfLinkMenuState{
		Open: true, Link: "https://example.com",
		BlockKey: rtfRunsKey(&RichText{Runs: []RichTextRun{
			{Text: "different block"},
		}}),
	})
	layout := generateViewLayout(RTF(RTFCfg{RichText: rt}), w)
	for i := range layout.Children {
		if layout.Children[i].Shape != nil &&
			layout.Children[i].Shape.ID == rtfLinkMenuFocusID {
			t.Error("link menu attached to a non-owning block")
		}
	}
}
