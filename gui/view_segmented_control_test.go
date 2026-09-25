package gui

import (
	"math"
	"testing"
)

// segTestOptions is three options with the middle one disabled, so the
// keyboard tests prove a disabled segment is skipped.
func segTestOptions() []SegmentOption {
	return []SegmentOption{
		NewSegmentOption("Day", "day"),
		{Label: "Week", Value: "week", Disabled: true},
		NewSegmentOption("Month", "month"),
	}
}

// segTestWindow renders a SegmentedControl whose Value lives in *value,
// so a click or key that calls OnSelect changes the next frame.
func segTestWindow(t *testing.T, value *string, cfg SegmentedControlCfg) *Window {
	t.Helper()
	w := NewTestWindow(WindowCfg{})
	cfg.ID = "seg"
	cfg.OnSelect = func(v string, ctx EventCtx) {
		*value = v
		ctx.Consume()
	}
	w.TestRender(func(*Window) View {
		c := cfg
		c.Value = *value
		return SegmentedControl(c)
	})
	return w
}

func segFind(t *testing.T, w *Window, id string) *Layout {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("no shape %q; frame has %v", id, w.EffectiveIDs())
	}
	return ly
}

func TestSegmentedControlItemsWinOverOptions(t *testing.T) {
	value := "b"
	w := segTestWindow(t, &value, SegmentedControlCfg{
		Items:   []string{"a", "b"},
		Options: segTestOptions(),
	})
	segFind(t, w, "seg:opt:0")
	segFind(t, w, "seg:opt:1")
	if _, ok := w.layout.FindByID("seg:opt:2"); ok {
		t.Error("Options segment rendered; Items must take precedence")
	}
}

func TestSegmentedControlA11Y(t *testing.T) {
	value := "month"
	w := segTestWindow(t, &value, SegmentedControlCfg{Options: segTestOptions()})
	track := segFind(t, w, "seg")
	if track.Shape.A11YRole != AccessRoleRadioGroup {
		t.Errorf("track role = %v, want RadioGroup", track.Shape.A11YRole)
	}
	if !track.Shape.Focusable {
		t.Error("track not focusable by default")
	}
	for i, want := range []AccessState{
		AccessStateNone, AccessStateNone, AccessStateSelected,
	} {
		seg := segFind(t, w, ScopeIDN("seg", "opt", i))
		if seg.Shape.A11YRole != AccessRoleRadioButton {
			t.Errorf("segment %d role = %v, want RadioButton",
				i, seg.Shape.A11YRole)
		}
		if seg.Shape.A11YState != want {
			t.Errorf("segment %d state = %v, want %v",
				i, seg.Shape.A11YState, want)
		}
		// One tab stop for the whole control: the segments never
		// take focus themselves.
		if seg.Shape.Focusable {
			t.Errorf("segment %d is focusable", i)
		}
	}
}

func TestSegmentedControlClickSelectsAndFocuses(t *testing.T) {
	value := "day"
	w := segTestWindow(t, &value, SegmentedControlCfg{Options: segTestOptions()})
	if err := w.TestClick("seg:opt:2"); err != nil {
		t.Fatal(err)
	}
	if value != "month" {
		t.Errorf("value = %q, want month", value)
	}
	if !w.IsFocus("seg") {
		t.Errorf("focus = %q, want the track", w.FocusID())
	}
}

func TestSegmentedControlDisabledSegmentHasNoHandler(t *testing.T) {
	value := "day"
	w := segTestWindow(t, &value, SegmentedControlCfg{Options: segTestOptions()})
	if err := w.TestClick("seg:opt:1"); err == nil {
		t.Error("click on a disabled segment was accepted")
	}
	if value != "day" {
		t.Errorf("value = %q, want day", value)
	}
}

func TestSegmentedControlKeyboard(t *testing.T) {
	cases := []struct {
		name  string
		start string
		key   KeyCode
		want  string
	}{
		{"right skips disabled", "day", KeyRight, "month"},
		{"right wraps", "month", KeyRight, "day"},
		{"left skips disabled", "month", KeyLeft, "day"},
		{"left wraps", "day", KeyLeft, "month"},
		{"home", "month", KeyHome, "day"},
		{"end", "day", KeyEnd, "month"},
		{"unknown value starts from first", "zzz", KeyRight, "month"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			value := tc.start
			w := segTestWindow(t, &value,
				SegmentedControlCfg{Options: segTestOptions()})
			if err := w.TestKey("seg", tc.key, ModNone); err != nil {
				t.Fatal(err)
			}
			if value != tc.want {
				t.Errorf("value = %q, want %q", value, tc.want)
			}
		})
	}
}

func TestSegmentedControlEnterRefires(t *testing.T) {
	value := "day"
	calls := 0
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return SegmentedControl(SegmentedControlCfg{
			ID: "seg", Value: value, Options: segTestOptions(),
			OnSelect: func(v string, ctx EventCtx) {
				calls++
				value = v
				ctx.Consume()
			},
		})
	})
	if err := w.TestKey("seg", KeyEnter, ModNone); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || value != "day" {
		t.Errorf("calls = %d value = %q, want 1 day", calls, value)
	}
}

func TestSegmentedControlDisabledWhole(t *testing.T) {
	value := "day"
	w := segTestWindow(t, &value, SegmentedControlCfg{
		Options: segTestOptions(), Disabled: true,
	})
	if err := w.TestClick("seg:opt:2"); err == nil {
		t.Error("click on a disabled control was accepted")
	}
	if value != "day" {
		t.Errorf("value = %q, want day", value)
	}
}

// TestSegmentedControlDisabledKeepsSelection: a disabled control still
// shows which value it holds, as a disabled radio does. The first cut
// dropped the pill and the selected state along with the handlers.
func TestSegmentedControlDisabledKeepsSelection(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  SegmentedControlCfg
		idx  int
	}{
		{"whole control", SegmentedControlCfg{
			Options: segTestOptions(), Disabled: true}, 0},
		{"selected segment", SegmentedControlCfg{
			Options: segTestOptions()}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value := segTestOptions()[tc.idx].Value
			w := segTestWindow(t, &value, tc.cfg)
			seg := segFind(t, w, ScopeIDN("seg", "opt", tc.idx))
			if seg.Shape.A11YState != AccessStateSelected {
				t.Errorf("state = %v, want Selected", seg.Shape.A11YState)
			}
			if seg.Shape.Color == ColorTransparent || !seg.Shape.Color.IsSet() {
				t.Errorf("pill fill = %v, want the selected fill",
					seg.Shape.Color)
			}
		})
	}
}

func TestSegmentedControlFocusDisabled(t *testing.T) {
	value := "day"
	w := segTestWindow(t, &value, SegmentedControlCfg{
		Options: segTestOptions(), FocusDisabled: true,
	})
	if segFind(t, w, "seg").Shape.Focusable {
		t.Error("track focusable with FocusDisabled")
	}
	if err := w.TestClick("seg:opt:2"); err != nil {
		t.Fatal(err)
	}
	if value != "month" {
		t.Errorf("value = %q, want month", value)
	}
	if w.FocusID() != "" {
		t.Errorf("focus = %q, want none", w.FocusID())
	}
}

// TestSegmentedControlFillEqualWidths pins the fill-width mode: every
// segment gets the same width whatever its label length.
func TestSegmentedControlFillEqualWidths(t *testing.T) {
	value := "a"
	// Guard: at fit width the labels must measure differently, or the
	// equal widths below would hold for any sizing.
	fit := segTestWindow(t, &value, SegmentedControlCfg{
		Items: []string{"a", "a much longer label", "mid"},
	})
	if segFind(t, fit, "seg:opt:1").Shape.Width <=
		segFind(t, fit, "seg:opt:0").Shape.Width {
		t.Fatal("fit widths do not differ; the test cannot see fill")
	}
	w := segTestWindow(t, &value, SegmentedControlCfg{
		Items:  []string{"a", "a much longer label", "mid"},
		Sizing: FillFit,
	})
	w0 := segFind(t, w, "seg:opt:0").Shape.Width
	for i := 1; i < 3; i++ {
		wi := segFind(t, w, ScopeIDN("seg", "opt", i)).Shape.Width
		if math.Abs(float64(wi-w0)) > 0.5 {
			t.Errorf("segment %d width = %v, segment 0 = %v", i, wi, w0)
		}
	}
}

// TestSegmentedControlDividerHiddenBesideSelection: a divider touching
// the pill is transparent, so the pill never has a line through its
// edge; the other dividers draw.
func TestSegmentedControlDividerHiddenBesideSelection(t *testing.T) {
	value := "b"
	w := segTestWindow(t, &value, SegmentedControlCfg{
		Items: []string{"a", "b", "c", "d"},
	})
	track := segFind(t, w, "seg")
	// Children: seg0, div, seg1, div, seg2, div, seg3.
	if n := len(track.Children); n != 7 {
		t.Fatalf("track children = %d, want 7", n)
	}
	for i, want := range []bool{false, false, true} {
		div := track.Children[2*i+1].Shape
		visible := div.Color != ColorTransparent
		if visible != want {
			t.Errorf("divider %d visible = %v, want %v", i, visible, want)
		}
	}
}

func TestSegmentedControlRequiresID(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic for an empty ID")
		}
	}()
	_ = SegmentedControl(SegmentedControlCfg{}) // requiredid:ignore
}

// TestSegmentedControlNoDebugFindings runs the dev-mode checks over a
// rendered control after a click and a key: no duplicate or drifted
// IDs, no unknown focus, no event acted on without Consume.
func TestSegmentedControlNoDebugFindings(t *testing.T) {
	value := "day"
	w := segTestWindow(t, &value, SegmentedControlCfg{Options: segTestOptions()})
	if err := w.TestClick("seg:opt:2"); err != nil {
		t.Fatal(err)
	}
	if err := w.TestKey("seg", KeyLeft, ModNone); err != nil {
		t.Fatal(err)
	}
	if found := w.TestFindings(DebugAll); len(found) > 0 {
		t.Errorf("debug findings: %v", found)
	}
	if found := w.TestUnconsumedEvents(); len(found) > 0 {
		t.Errorf("unconsumed events: %v", found)
	}
}

// segLabel returns the TextStyle of segment i's label (its last child).
func segLabel(t *testing.T, w *Window, i int) TextStyle {
	t.Helper()
	seg := segFind(t, w, ScopeIDN("seg", "opt", i))
	if len(seg.Children) == 0 || seg.Children[len(seg.Children)-1].Shape.TC == nil {
		t.Fatalf("segment %d has no label", i)
	}
	return *seg.Children[len(seg.Children)-1].Shape.TC.TextStyle
}

// TestSegmentedControlKeepsCallerTextStyle: the selected and disabled
// labels keep the caller's face and size and change only the color.
// The first cut swapped in the whole theme style, so the pill label
// changed size and the segment widths moved with the selection.
func TestSegmentedControlKeepsCallerTextStyle(t *testing.T) {
	value := "day"
	w := segTestWindow(t, &value, SegmentedControlCfg{
		Options:   segTestOptions(),
		TextStyle: TextStyle{Size: 21},
	})
	for i, name := range []string{"selected", "disabled", "rest"} {
		if got := segLabel(t, w, i).Size; got != 21 {
			t.Errorf("%s label size = %v, want 21", name, got)
		}
	}
	s := &defaultSegmentedControlStyle
	if got := segLabel(t, w, 0).Color; got != s.textStyleSelected.Color {
		t.Errorf("selected label color = %v, want %v",
			got, s.textStyleSelected.Color)
	}
	dis := segLabel(t, w, 1)
	if dis.Color.A != s.textStyleDisabled.Color.A || !dis.disabledRole {
		t.Errorf("disabled label = %+v, want the disabled role alpha and marker", dis)
	}
}

// TestSegmentedControlCallerDisabledColorWins: a caller's own
// ColorsSegment.Disabled paints a disabled pill too; the dimmed-accent
// default applies only when the caller left it unset.
func TestSegmentedControlCallerDisabledColorWins(t *testing.T) {
	want := RGB(10, 200, 30)
	value := "day"
	w := segTestWindow(t, &value, SegmentedControlCfg{
		Options:       segTestOptions(),
		Disabled:      true,
		ColorsSegment: ColorSet{Disabled: want},
	})
	if got := segFind(t, w, "seg:opt:0").Shape.Color; got != want {
		t.Errorf("disabled pill = %v, want the caller's %v", got, want)
	}
}

// TestSegmentedControlRadiusOverrideConcentric: a caller's track
// Radius moves the pill radius with it, so the corners stay
// concentric (the pill radius is the track radius less the inset).
func TestSegmentedControlRadiusOverrideConcentric(t *testing.T) {
	value := "day"
	w := segTestWindow(t, &value, SegmentedControlCfg{
		Options: segTestOptions(),
		Radius:  SomeF(12),
	})
	if got := segFind(t, w, "seg").Shape.Radius; got != 12 {
		t.Errorf("track radius = %v, want 12", got)
	}
	want := float32(12 - segmentedInset)
	if got := segFind(t, w, "seg:opt:0").Shape.Radius; got != want {
		t.Errorf("pill radius = %v, want %v", got, want)
	}
}

// TestSegmentedControlIconOnly: an icon-only segment is named by its
// value for a screen reader, and the icon on the pill takes the
// on-accent color.
func TestSegmentedControlIconOnly(t *testing.T) {
	value := "list"
	w := segTestWindow(t, &value, SegmentedControlCfg{
		Options: []SegmentOption{
			{Icon: IconListBullet, Value: "list"},
			{Icon: IconTable, Value: "table"},
		},
	})
	seg := segFind(t, w, "seg:opt:0")
	if seg.Shape.a11Y == nil || seg.Shape.a11Y.Label != "list" {
		t.Errorf("a11y = %+v, want label %q", seg.Shape.a11Y, "list")
	}
	s := &defaultSegmentedControlStyle
	if got := segLabel(t, w, 0).Color; got != s.textStyleSelected.Color {
		t.Errorf("pill icon color = %v, want %v", got, s.textStyleSelected.Color)
	}
	if got := segLabel(t, w, 1).Color; got != s.textStyleIcon.Color {
		t.Errorf("rest icon color = %v, want %v", got, s.textStyleIcon.Color)
	}
}

// TestSegmentedControlUnknownValueSelectsNone: a Value that names no
// segment highlights nothing, as in a radio group.
func TestSegmentedControlUnknownValueSelectsNone(t *testing.T) {
	value := "nope"
	w := segTestWindow(t, &value, SegmentedControlCfg{Options: segTestOptions()})
	for i := range 3 {
		if st := segFind(t, w, ScopeIDN("seg", "opt", i)).Shape.A11YState; st != AccessStateNone {
			t.Errorf("segment %d state = %v, want None", i, st)
		}
	}
}

// TestSegmentedControlNilOnSelect: a click with no OnSelect still
// focuses the track and does not panic.
func TestSegmentedControlNilOnSelect(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(*Window) View {
		return SegmentedControl(SegmentedControlCfg{
			ID: "seg", Value: "day", Options: segTestOptions()})
	})
	if err := w.TestClick("seg:opt:2"); err != nil {
		t.Fatal(err)
	}
	if !w.IsFocus("seg") {
		t.Errorf("focus = %q, want the track", w.FocusID())
	}
}
