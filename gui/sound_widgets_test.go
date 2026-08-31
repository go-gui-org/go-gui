package gui

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Phase 2 of widget audio feedback (issue #467): every widget whose
// activation lands on a ContainerCfg, a ButtonCfg or a raw
// eventHandlers literal emits a resolved cue.
//
// Theme installation is package-global, so nothing here runs in
// parallel, and every test starts with restoreTheme(t).

// soundWindow builds a window with the sounding theme installed and a
// spy attached, then renders view. It is the three lines every case
// below would otherwise repeat.
func soundWindow(t *testing.T, view func(*Window) View) (*Window, *soundSpy) {
	t.Helper()
	spy := &soundSpy{}
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	w.SetSoundPlayer(spy)
	w.TestRender(view)
	return w, spy
}

// findClickable returns the first shape in tree order whose accessible
// label matches. Rows in a ListBox, options in a Select and rows in a
// Tree carry no ID of their own — identity there belongs to the owning
// widget — so TestClick cannot reach them and the test has to aim at
// the geometry instead.
func findClickable(l *Layout, label string) *Layout {
	if l == nil || l.Shape == nil {
		return nil
	}
	if l.Shape.a11Y != nil && l.Shape.a11Y.Label == label &&
		l.Shape.hasEvents() && l.Shape.events.OnClick != nil {
		return l
	}
	for i := range l.Children {
		if got := findClickable(&l.Children[i], label); got != nil {
			return got
		}
	}
	return nil
}

// clickLabel dispatches a real press/release at the centre of the
// shape carrying the given accessible label, driving the same dispatch
// path the backend does.
func clickLabel(t *testing.T, w *Window, label string) {
	t.Helper()
	ly := findClickable(&w.layout, label)
	if ly == nil {
		t.Fatalf("no clickable shape labelled %q", label)
	}
	x, y, err := testHitPoint(ly, label)
	if err != nil {
		t.Fatalf("hit point for %q: %v", label, err)
	}
	down := Event{Type: EventMouseDown, MouseButton: MouseLeft,
		MouseX: x, MouseY: y}
	w.EventFn(&down)
	w.settle()
	up := Event{Type: EventMouseUp, MouseButton: MouseLeft,
		MouseX: x, MouseY: y}
	w.EventFn(&up)
	w.settle()
	if !down.IsHandled {
		t.Fatalf("click on %q was not handled", label)
	}
}

// wantCues fails unless the spy recorded exactly want, in order.
func wantCues(t *testing.T, spy *soundSpy, want ...SoundCue) {
	t.Helper()
	if len(spy.cues) != len(want) {
		t.Fatalf("cues = %v, want %v", spy.cues, want)
	}
	for i := range want {
		if spy.cues[i] != want[i] {
			t.Fatalf("cues = %v, want %v", spy.cues, want)
		}
	}
}

func soundNoop(ctx EventCtx) { ctx.Consume() }

// One case per phase-2 widget: render it, activate it once, and assert
// the exact cue. A widget whose cue depends on its own state is driven
// twice by TestSoundPhase2StateDependent below instead.
func TestSoundPhase2WidgetCues(t *testing.T) {
	restoreTheme(t)

	cases := []struct {
		name  string
		view  func(*Window) View
		click func(*testing.T, *Window)
		want  SoundCue
	}{{
		name: "radio",
		view: func(*Window) View {
			return Radio(RadioCfg{ID: "rb", Label: "One", OnClick: soundNoop})
		},
		click: clickID("rb"),
		want:  SoundSelection,
	}, {
		name: "radio_group",
		view: func(*Window) View {
			return RadioButtonGroupColumn(RadioButtonGroupCfg{
				ID:       "grp",
				Items:    []string{"One", "Two"},
				OnSelect: func(string, EventCtx) {},
			})
		},
		click: clickID(ScopeIDN("grp", "opt", 0)),
		want:  SoundSelection,
	}, {
		name: "color_swatch",
		view: func(*Window) View {
			return ColorSwatch(ColorSwatchCfg{
				ID: "cs", Color: RGB(10, 20, 30), OnClick: soundNoop})
		},
		click: clickID("cs"),
		want:  SoundSelection,
	}, {
		name: "breadcrumb",
		view: func(*Window) View {
			return Breadcrumb(BreadcrumbCfg{
				ID:       "bc",
				Items:    []BreadcrumbItemCfg{{ID: "home", Label: "Home"}},
				Selected: "home",
				OnSelect: func(string, EventCtx) {},
			})
		},
		click: clickID(bcCrumbID("bc", "home")),
		want:  SoundSelection,
	}, {
		name: "listbox_row",
		view: func(*Window) View {
			return ListBox(ListBoxCfg{
				ID: "lb",
				Data: []ListBoxOption{
					NewListBoxOption("a", "Alpha", "Alpha")},
				OnSelect: func([]string, EventCtx) {},
			})
		},
		click: clickLabelFn("Alpha"),
		want:  SoundSelection,
	}, {
		name: "tab_control",
		view: func(*Window) View {
			return TabControl(TabControlCfg{
				ID: "tc",
				Items: []TabItemCfg{
					{ID: "one", Label: "One"},
					{ID: "two", Label: "Two"}},
				Selected: "one",
				OnSelect: func(string, EventCtx) {},
			})
		},
		click: clickID(tabButtonID("tc", "two")),
		want:  SoundSelection,
	}, {
		name: "date_picker_day",
		view: func(*Window) View {
			return DatePicker(DatePickerCfg{
				ID:       "dp",
				OnSelect: func([]time.Time, EventCtx) {},
			})
		},
		click: clickID(ScopeIDN("dp", "day", 1)),
		want:  SoundSelection,
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreTheme(t)
			w, spy := soundWindow(t, tc.view)
			tc.click(t, w)
			wantCues(t, spy, tc.want)
		})
	}
}

func clickID(id string) func(*testing.T, *Window) {
	return func(t *testing.T, w *Window) {
		t.Helper()
		if err := w.TestClick(id); err != nil {
			t.Fatalf("TestClick(%q): %v", id, err)
		}
	}
}

func clickLabelFn(label string) func(*testing.T, *Window) {
	return func(t *testing.T, w *Window) {
		t.Helper()
		clickLabel(t, w, label)
	}
}

// The raw eventHandlers path: Image and Svg build their handler record
// directly instead of going through makeContainerEvents, so the cue
// reaches them by a different line of code and needs its own case.
func TestSoundPhase2RawEventHandlers(t *testing.T) {
	restoreTheme(t)

	// A real PNG on disk: the decode has to succeed, or the shape
	// never reaches the tree for the click to land on.
	dir := t.TempDir()
	path := filepath.Join(dir, "dot.png")
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write png: %v", err)
	}

	w, spy := soundWindow(t, func(*Window) View {
		return Image(ImageCfg{
			ID: "img", Src: path, Width: 20, Height: 20,
			OnClick: soundNoop,
		})
	})
	clickID("img")(t, w)
	wantCues(t, spy, SoundClick)
}

// A widget whose cue depends on its own state names what the click
// will do, not what the state is. Each case is clicked twice with a
// re-render between, so the second click sees the flipped state.
func TestSoundPhase2StateDependent(t *testing.T) {
	restoreTheme(t)

	cases := []struct {
		name string
		view func(*bool) func(*Window) View
		id   string
	}{{
		name: "switch",
		view: func(on *bool) func(*Window) View {
			return func(*Window) View {
				return Switch(SwitchCfg{
					ID: "sw", Label: "On", Selected: *on,
					OnClick: func(ctx EventCtx) {
						*on = !*on
						ctx.Consume()
					},
				})
			}
		},
		id: "sw",
	}, {
		name: "expand_panel",
		view: func(on *bool) func(*Window) View {
			return func(*Window) View {
				return ExpandPanel(ExpandPanelCfg{
					ID:   "ep",
					Open: *on,
					Head: Text(TextCfg{Text: "Head"}),
					OnToggle: func(ctx EventCtx) {
						*on = !*on
						ctx.Consume()
					},
				})
			}
		},
		id: ScopeID("ep", "head"),
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreTheme(t)
			on := false
			w, spy := soundWindow(t, tc.view(&on))
			for i := range 2 {
				if err := w.TestClick(tc.id); err != nil {
					t.Fatalf("click %d: %v", i, err)
				}
				w.TestRender(nil)
			}
			wantCues(t, spy, SoundToggleOn, SoundToggleOff)
		})
	}
}

// Select, Combobox and ThemePicker own their open flag in window
// state, so the toggle drives itself: no app-side bool to flip.
func TestSoundPhase2DropdownOpenClose(t *testing.T) {
	restoreTheme(t)

	cases := []struct {
		name string
		view func(*Window) View
	}{{
		name: "select",
		view: func(*Window) View {
			return Select(SelectCfg{
				ID: "sel", Options: []string{"One", "Two"},
				OnSelect: func([]string, EventCtx) {},
			})
		},
	}, {
		name: "combobox",
		view: func(*Window) View {
			return Combobox(ComboboxCfg{
				ID: "cb", Options: []string{"Alpha", "Beta"},
				OnSelect: func(string, EventCtx) {},
			})
		},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreTheme(t)
			id := "sel"
			if tc.name == "combobox" {
				id = "cb"
			}
			w, spy := soundWindow(t, tc.view)
			for i := range 2 {
				if err := w.TestClick(id); err != nil {
					t.Fatalf("click %d: %v", i, err)
				}
				w.TestRender(nil)
			}
			wantCues(t, spy, SoundToggleOn, SoundToggleOff)
		})
	}
}

// The click absorbers stay silent under a sounding theme. Each one
// exists to swallow a click that would otherwise reach what it covers;
// a cue there would fire on every scroll drag or caret placement.
//
// Asserted over the whole rendered tree rather than by clicking: a
// scrollbar track carries no ID, so there is nothing for TestClick to
// aim at, and "no shape anywhere in this widget carries a cue" is the
// stronger claim anyway.
//
// Two more absorbers are silent by construction and need no case here:
// ContextMenu's dismissal is an OnAnyClick, which playShapeSound never
// consults, and the CommandPalette card and Toast scrim leave Sound
// unset on containers whose OnClick only calls ctx.Consume.
func TestSoundPhase2AbsorbersStaySilent(t *testing.T) {
	restoreTheme(t)

	cases := []struct {
		name string
		view func(*Window) View
	}{{
		name: "scrollbar",
		view: func(*Window) View {
			return Column(ContainerCfg{
				ID: "scr", Height: 40, Width: 100,
				Sizing: FixedFixed, Scrollable: true,
				Content: []View{Rectangle(RectangleCfg{
					Width: 100, Height: 400, Sizing: FixedFixed})},
			})
		},
	}, {
		name: "input_caret",
		view: func(*Window) View {
			return Input(InputCfg{ID: "in", Text: "hello"})
		},
	}}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restoreTheme(t)
			w, _ := soundWindow(t, tc.view)
			if got := collectCues(&w.layout, nil); len(got) != 0 {
				t.Errorf("%s carries cues %v, want none", tc.name, got)
			}
		})
	}
}

// collectCues returns every non-silent cue in the subtree, in tree
// order.
func collectCues(l *Layout, acc []SoundCue) []SoundCue {
	if l == nil || l.Shape == nil {
		return acc
	}
	if l.Shape.hasEvents() && l.Shape.events.soundCue != SoundNone {
		acc = append(acc, l.Shape.events.soundCue)
	}
	for i := range l.Children {
		acc = collectCues(&l.Children[i], acc)
	}
	return acc
}

// Cfg.Sound and SoundDisabled have to work on every site kind, not
// only on the ButtonCfg phase 1 proved them on.
func TestSoundPhase2OverridesPerSiteKind(t *testing.T) {
	restoreTheme(t)

	cases := []struct {
		name string
		view func(cue SoundCue, off bool) func(*Window) View
		id   string
	}{{
		name: "container_cfg",
		view: func(cue SoundCue, off bool) func(*Window) View {
			return func(*Window) View {
				return Radio(RadioCfg{ID: "rb", Label: "One",
					Sound: cue, SoundDisabled: off,
					OnClick: soundNoop})
			}
		},
		id: "rb",
	}, {
		name: "button_cfg",
		view: func(cue SoundCue, off bool) func(*Window) View {
			return func(*Window) View {
				return TabControl(TabControlCfg{
					ID: "tc",
					Items: []TabItemCfg{
						{ID: "one", Label: "One"},
						{ID: "two", Label: "Two"}},
					Selected:      "one",
					Sound:         cue,
					SoundDisabled: off,
					OnSelect:      func(string, EventCtx) {},
				})
			}
		},
		id: tabButtonID("tc", "two"),
	}}

	for _, tc := range cases {
		t.Run(tc.name+"_override", func(t *testing.T) {
			restoreTheme(t)
			w, spy := soundWindow(t, tc.view(SoundError, false))
			clickID(tc.id)(t, w)
			wantCues(t, spy, SoundError)
		})
		t.Run(tc.name+"_disabled", func(t *testing.T) {
			restoreTheme(t)
			// SoundDisabled beats an explicit Sound, not only the
			// theme.
			w, spy := soundWindow(t, tc.view(SoundError, true))
			clickID(tc.id)(t, w)
			wantCues(t, spy)
		})
	}
}

// The raw eventHandlers path resolves its own precedence rather than
// inheriting ContainerCfg's, so it gets its own override case.
func TestSoundPhase2RawHandlerOverride(t *testing.T) {
	restoreTheme(t)

	dir := t.TempDir()
	path := filepath.Join(dir, "dot.png")
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatalf("write png: %v", err)
	}

	w, spy := soundWindow(t, func(*Window) View {
		return Image(ImageCfg{
			ID: "img", Src: path, Width: 20, Height: 20,
			Sound: SoundError, OnClick: soundNoop,
		})
	})
	clickID("img")(t, w)
	wantCues(t, spy, SoundError)
}
