package gui

import "testing"

// Tests for issue #1109: layout runs in the part of the window above the
// soft keyboard, and the focused field is scrolled into that part.

// newRevealWindow builds a 400x600 window whose view is content in a
// FillFill column, with a spy platform so the edit-context gate runs.
func newRevealWindow(t *testing.T, cfg WindowCfg, content func() []View) *Window {
	t.Helper()
	cfg.State = new(int)
	cfg.Width, cfg.Height = 400, 600
	w := NewWindow(cfg)
	t.Cleanup(w.WindowCleanup)
	w.SetNativePlatform(&softKeyboardSpy{})
	w.viewGenerator = func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing:     FillFill,
			Padding:    PaddingNone,
			SizeBorder: NoBorder,
			Content:    content(),
		})
	}
	return w
}

// setInset sends the inset the way a backend does, then runs a frame.
func setInset(w *Window, h float32) {
	w.EventFn(&Event{Type: EventSoftKeyboard, SoftKeyboardInset: h})
	frame(w)
}

// rootHeight is the height of the main layer, the app's FillFill root.
func rootHeight(t *testing.T, w *Window) float32 {
	t.Helper()
	if len(w.layout.Children) == 0 {
		t.Fatal("no layers")
	}
	return w.layout.Children[0].Shape.Height
}

func TestSoftKeyboardShrinksFillRoot(t *testing.T) {
	none := func() []View { return nil }
	t.Run("default", func(t *testing.T) {
		w := newRevealWindow(t, WindowCfg{}, none)
		frame(w)
		if got := rootHeight(t, w); got != 600 {
			t.Fatalf("root height, keyboard down = %v, want 600", got)
		}
		setInset(w, 250)
		if got := rootHeight(t, w); got != 350 {
			t.Fatalf("root height, keyboard up = %v, want 350", got)
		}
		setInset(w, 0)
		if got := rootHeight(t, w); got != 600 {
			t.Fatalf("root height, keyboard closed = %v, want 600", got)
		}
	})
	t.Run("opt-out", func(t *testing.T) {
		w := newRevealWindow(t, WindowCfg{SoftKeyboardOverlays: true}, none)
		setInset(w, 250)
		if got := rootHeight(t, w); got != 600 {
			t.Fatalf("root height with SoftKeyboardOverlays = %v, want 600", got)
		}
		// The inset is still reported, for the app to use.
		if got := w.SoftKeyboardInset(); got != 250 {
			t.Fatalf("inset = %v, want 250", got)
		}
	})
}

// A built-in dialog is a top-level float placed against the window. With
// the keyboard up it centres in the part above the keyboard, so its
// buttons are not under it.
func TestSoftKeyboardCentresDialogAbove(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, func() []View { return nil })
	w.Dialog(DialogCfg{Title: "Name", Body: "Enter a name"})
	setInset(w, 250)
	// The dialog layer by its reserved ID: another float layer centred
	// in the same area would also pass the checks below.
	var dlg *Shape
	for i := range w.layout.Children {
		if s := w.layout.Children[i].Shape; s != nil && s.idKey() == reservedDialogID {
			dlg = s
		}
	}
	if dlg == nil {
		t.Fatal("no dialog layer")
	}
	mid := dlg.Y + dlg.Height/2
	if want := float32(350) / 2; f32Abs(mid-want) > 0.5 {
		t.Fatalf("dialog middle y = %v, want %v", mid, want)
	}
	if dlg.Y+dlg.Height > 350 {
		t.Fatalf("dialog bottom %v is under the keyboard top 350", dlg.Y+dlg.Height)
	}
}

// revealContent is a scroll column with a tall spacer and a field at the
// bottom, and a field "top" above the spacer. The bottom field shows
// while the keyboard is down.
func revealContent() []View { return revealContentSpaced(480) }

// revealContentSpaced is revealContent with a spacer of the given height.
func revealContentSpaced(spacer float32) []View {
	return []View{Column(ContainerCfg{
		ID:         "sc",
		Scrollable: true,
		Sizing:     FillFill,
		Padding:    PaddingNone,
		SizeBorder: NoBorder,
		Content: []View{
			Input(InputCfg{ID: "top"}),
			Column(ContainerCfg{
				Sizing: FixedFixed, Width: 10, Height: spacer,
				Padding: PaddingNone, SizeBorder: NoBorder,
			}),
			Input(InputCfg{ID: "f"}),
		},
	})}
}

// fieldBottom is the window y of the focused field's lower edge.
func fieldBottom(t *testing.T, w *Window, id string) float32 {
	t.Helper()
	ly, ok := findLayoutByFocusID(&w.layout, id)
	if !ok {
		t.Fatalf("no field %q", id)
	}
	return ly.Shape.Y + ly.Shape.Height
}

func TestSoftKeyboardRevealsFocusedField(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, revealContent)
	w.SetFocus(ScopeID("sc", "f"))
	frame(w)
	if got := w.ScrollVerticalOffset("sc"); got != 0 {
		t.Fatalf("offset before keyboard = %v, want 0", got)
	}
	// The keyboard opens. The field is scrolled up above it in the same
	// frame that first lays out the smaller area.
	setInset(w, 250)
	if got := w.ScrollVerticalOffset("sc"); got >= 0 {
		t.Fatalf("offset after keyboard = %v, want < 0", got)
	}
	if b := fieldBottom(t, w, ScopeID("sc", "f")); b > 350+0.5 {
		t.Fatalf("field bottom = %v, want <= 350", b)
	}

	// The user scrolls back to the top while typing. Nothing pulls the
	// view back: the reveal happens once.
	w.ScrollVerticalTo("sc", 0)
	frame(w)
	frame(w)
	if got := w.ScrollVerticalOffset("sc"); got != 0 {
		t.Fatalf("offset after user scroll = %v, want 0", got)
	}

	// A shrinking keyboard (a suggestion bar hidden) reveals nothing.
	setInset(w, 200)
	if got := w.ScrollVerticalOffset("sc"); got != 0 {
		t.Fatalf("offset after inset shrink = %v, want 0", got)
	}

	// A growing one reveals again.
	setInset(w, 250)
	if got := w.ScrollVerticalOffset("sc"); got >= 0 {
		t.Fatalf("offset after inset growth = %v, want < 0", got)
	}
}

// Moving focus to another field while the keyboard is up reveals that
// field, in the frame that first lays out the new focus.
func TestSoftKeyboardRevealsOnFocusChange(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, revealContent)
	w.SetFocus(ScopeID("sc", "top"))
	frame(w)
	setInset(w, 250)
	if got := w.ScrollVerticalOffset("sc"); got != 0 {
		t.Fatalf("offset with top field focused = %v, want 0", got)
	}
	w.SetFocus(ScopeID("sc", "f"))
	frame(w)
	if b := fieldBottom(t, w, ScopeID("sc", "f")); b > 350+0.5 {
		t.Fatalf("field bottom after focus move = %v, want <= 350", b)
	}
}

func TestSoftKeyboardNoRevealCases(t *testing.T) {
	t.Run("no focus", func(t *testing.T) {
		w := newRevealWindow(t, WindowCfg{}, revealContent)
		frame(w)
		setInset(w, 250)
		if got := w.ScrollVerticalOffset("sc"); got != 0 {
			t.Fatalf("offset = %v, want 0", got)
		}
	})
	t.Run("opt-out", func(t *testing.T) {
		w := newRevealWindow(t, WindowCfg{SoftKeyboardOverlays: true}, revealContent)
		w.SetFocus(ScopeID("sc", "f"))
		frame(w)
		setInset(w, 250)
		if got := w.ScrollVerticalOffset("sc"); got != 0 {
			t.Fatalf("offset = %v, want 0", got)
		}
	})
	t.Run("desktop focus change", func(t *testing.T) {
		// Inset 0: focus moves never scroll (no behaviour change on
		// desktop). The bottom field starts below the 600 px window,
		// so a reveal on this focus move would scroll.
		w := newRevealWindow(t, WindowCfg{}, func() []View {
			return revealContentSpaced(900)
		})
		w.SetFocus(ScopeID("sc", "top"))
		frame(w)
		if b := fieldBottom(t, w, ScopeID("sc", "f")); b <= 600 {
			t.Fatalf("field bottom = %v, want below the window (> 600)", b)
		}
		w.ScrollVerticalTo("sc", 0)
		w.SetFocus(ScopeID("sc", "f"))
		frame(w)
		if got := w.ScrollVerticalOffset("sc"); got != 0 {
			t.Fatalf("offset = %v, want 0", got)
		}
	})
}

// An anchored float with FloatAutoFlip (a dropdown) under a box near the
// keyboard flips above the box instead of opening under the keyboard.
func TestSoftKeyboardFlipsAnchoredFloat(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, func() []View {
		return []View{
			Column(ContainerCfg{
				Sizing: FixedFixed, Width: 10, Height: 300,
				Padding: PaddingNone, SizeBorder: NoBorder,
			}),
			Column(ContainerCfg{
				Sizing: FixedFixed, Width: 100, Height: 20,
				Padding: PaddingNone, SizeBorder: NoBorder,
				Content: []View{Column(ContainerCfg{
					ID:            "pop",
					Float:         true,
					FloatAutoFlip: true,
					FloatAnchor:   FloatBottomLeft,
					FloatTieOff:   FloatTopLeft,
					Sizing:        FixedFixed, Width: 100, Height: 120,
					Padding: PaddingNone, SizeBorder: NoBorder,
				})},
			}),
		}
	})
	frame(w)
	ly := findLayoutByID(&w.layout, "pop")
	if ly == nil {
		t.Fatal("no float")
	}
	box := ly.Parent.Shape
	if want := box.Y + box.Height; ly.Shape.Y != want {
		t.Fatalf("float y, keyboard down = %v, want %v (below the box)", ly.Shape.Y, want)
	}
	setInset(w, 200)
	ly = findLayoutByID(&w.layout, "pop")
	if got := ly.Shape.Y + ly.Shape.Height; got > 400 {
		t.Fatalf("float bottom, keyboard up = %v, want <= 400", got)
	}
	// Flipped, not only clamped: a clamp alone would leave it over the
	// box. The bottom meets the box top.
	box = ly.Parent.Shape
	if got := ly.Shape.Y + ly.Shape.Height; got > box.Y+0.5 {
		t.Fatalf("float bottom, keyboard up = %v, want <= box top %v (flipped above)", got, box.Y)
	}
}

// A focusable float that is its own layer root holds focus. Its Parent
// points at the container it was declared in, in the main tree. The
// reveal must not walk out of the float's layer into that tree and
// scroll it (review of #1109).
func TestSoftKeyboardRevealStaysInFloatLayer(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, func() []View {
		return []View{Column(ContainerCfg{
			ID:         "sc",
			Scrollable: true,
			Sizing:     FillFill,
			Padding:    PaddingNone,
			SizeBorder: NoBorder,
			Content: []View{
				Column(ContainerCfg{
					Sizing: FixedFixed, Width: 10, Height: 900,
					Padding: PaddingNone, SizeBorder: NoBorder,
					Content: []View{Column(ContainerCfg{
						ID:          "pop",
						Focusable:   true,
						Float:       true,
						FloatAnchor: FloatBottomLeft,
						FloatTieOff: FloatTopLeft,
						Sizing:      FixedFixed, Width: 50, Height: 50,
						Padding: PaddingNone, SizeBorder: NoBorder,
					})},
				}),
			},
		})}
	})
	frame(w)
	setInset(w, 250)
	w.SetFocus(ScopeID("sc", "pop"))
	frame(w)
	if got := w.ScrollVerticalOffset("sc"); got != 0 {
		t.Fatalf("main-tree offset = %v, want 0", got)
	}
}

// Focus repair moves focus off a removed field to the first tab stop.
// That new focus was never laid out as focused, so the next frame must
// still reveal it (review of #1109).
func TestSoftKeyboardRevealsRepairedFocus(t *testing.T) {
	show := true
	w := newRevealWindow(t, WindowCfg{}, func() []View {
		content := make([]View, 0, 3)
		if show {
			content = append(content, Input(InputCfg{ID: "gone"}))
		}
		content = append(content,
			Column(ContainerCfg{
				Sizing: FixedFixed, Width: 10, Height: 480,
				Padding: PaddingNone, SizeBorder: NoBorder,
			}),
			Input(InputCfg{ID: "f"}))
		return []View{Column(ContainerCfg{
			ID:         "sc",
			Scrollable: true,
			Sizing:     FillFill,
			Padding:    PaddingNone,
			SizeBorder: NoBorder,
			Content:    content,
		})}
	})
	w.SetFocus(ScopeID("sc", "gone"))
	frame(w)
	setInset(w, 250)
	show = false
	frame(w) // the repair moves focus to "f"
	if got := w.FocusID(); got != ScopeID("sc", "f") {
		t.Fatalf("focus after repair = %q, want sc:f", got)
	}
	// No event follows the repair, so the pass that repaired the focus
	// must ask for another itself. FrameFn runs that second pass at
	// once, so no further frame is driven here: a third frame would
	// hide a missing request.
	if b := fieldBottom(t, w, ScopeID("sc", "f")); b > 350+0.5 {
		t.Fatalf("repaired field bottom = %v, want <= 350", b)
	}
}

// A field above the visible part (the view was scrolled down to reveal
// another field) scrolls down into view when it takes focus.
func TestSoftKeyboardRevealsFieldAbove(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, func() []View {
		return revealContentSpaced(900)
	})
	w.SetFocus(ScopeID("sc", "f"))
	frame(w)
	setInset(w, 250)
	if got := w.ScrollVerticalOffset("sc"); got >= 0 {
		t.Fatalf("offset after reveal of bottom field = %v, want < 0", got)
	}
	w.SetFocus(ScopeID("sc", "top"))
	frame(w)
	ly, ok := findLayoutByFocusID(&w.layout, ScopeID("sc", "top"))
	if !ok {
		t.Fatal("no top field")
	}
	if ly.Shape.Y < -0.5 {
		t.Fatalf("top field y = %v, want >= 0 (in view)", ly.Shape.Y)
	}
}

// A field taller than the space above the keyboard shows its top, where
// the caret's first line is, not its bottom.
func TestSoftKeyboardRevealsTallFieldTop(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, func() []View {
		spacer := func(h float32) View {
			return Column(ContainerCfg{
				Sizing: FixedFixed, Width: 10, Height: h,
				Padding: PaddingNone, SizeBorder: NoBorder,
			})
		}
		return []View{Column(ContainerCfg{
			ID:         "sc",
			Scrollable: true,
			Sizing:     FillFill,
			Padding:    PaddingNone,
			SizeBorder: NoBorder,
			Content: []View{
				spacer(480),
				Input(InputCfg{ID: "f", Sizing: FillFixed, Height: 500}),
				spacer(200),
			},
		})}
	})
	w.SetFocus(ScopeID("sc", "f"))
	frame(w)
	setInset(w, 250)
	ly, ok := findLayoutByFocusID(&w.layout, ScopeID("sc", "f"))
	if !ok {
		t.Fatal("no field")
	}
	if ly.Shape.Height <= 350 {
		t.Fatalf("field height = %v, want > 350 for this case", ly.Shape.Height)
	}
	if f32Abs(ly.Shape.Y) > 0.5 {
		t.Fatalf("tall field top = %v, want 0 (top of the view)", ly.Shape.Y)
	}
}

// With a scroll container inside another, the inner one moves first and
// the outer one moves the rest, so the field ends above the keyboard.
func TestSoftKeyboardRevealsThroughNestedScrolls(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, func() []View {
		spacer := func(h float32) View {
			return Column(ContainerCfg{
				Sizing: FixedFixed, Width: 10, Height: h,
				Padding: PaddingNone, SizeBorder: NoBorder,
			})
		}
		return []View{Column(ContainerCfg{
			ID:         "out",
			Scrollable: true,
			Sizing:     FillFill,
			Padding:    PaddingNone,
			SizeBorder: NoBorder,
			Content: []View{
				spacer(300),
				Column(ContainerCfg{
					ID:         "in",
					Scrollable: true,
					Sizing:     FillFixed,
					Height:     400,
					Padding:    PaddingNone,
					SizeBorder: NoBorder,
					Content:    []View{spacer(600), Input(InputCfg{ID: "f"})},
				}),
				spacer(400),
			},
		})}
	})
	in := ScopeID("out", "in")
	f := ScopeID(in, "f")
	w.SetFocus(f)
	frame(w)
	setInset(w, 250)
	if got := w.ScrollVerticalOffset(in); got >= 0 {
		t.Fatalf("inner offset = %v, want < 0", got)
	}
	if got := w.ScrollVerticalOffset("out"); got >= 0 {
		t.Fatalf("outer offset = %v, want < 0", got)
	}
	if b := fieldBottom(t, w, f); b > 350+0.5 {
		t.Fatalf("field bottom = %v, want <= 350", b)
	}
}

// A dialog taller than the area above the keyboard keeps its top in the
// window: centred, its title and first field would be cut off, and no
// scroll container could bring them back.
func TestSoftKeyboardTallDialogKeepsTop(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, func() []View { return nil })
	w.Dialog(DialogCfg{Title: "Name", Body: "Enter a name", MinHeight: 500})
	setInset(w, 250)
	var dlg *Shape
	for i := range w.layout.Children {
		if s := w.layout.Children[i].Shape; s != nil && s.idKey() == reservedDialogID {
			dlg = s
		}
	}
	if dlg == nil {
		t.Fatal("no dialog layer")
	}
	if dlg.Height <= 350 {
		t.Fatalf("dialog height = %v, want > 350 for this case", dlg.Height)
	}
	if f32Abs(dlg.Y) > 0.5 {
		t.Fatalf("tall dialog top = %v, want 0", dlg.Y)
	}
}

// With the keyboard down, or with SoftKeyboardOverlays, a dialog taller
// than the window still centres, as before #1109.
func TestSoftKeyboardTallDialogCentresWithoutKeyboard(t *testing.T) {
	for _, tc := range []struct {
		name  string
		cfg   WindowCfg
		inset float32
	}{
		{"keyboard down", WindowCfg{}, 0},
		{"opt-out", WindowCfg{SoftKeyboardOverlays: true}, 250},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := newRevealWindow(t, tc.cfg, func() []View { return nil })
			w.Dialog(DialogCfg{Title: "Name", Body: "Enter a name", MinHeight: 800})
			setInset(w, tc.inset)
			var dlg *Shape
			for i := range w.layout.Children {
				if s := w.layout.Children[i].Shape; s != nil && s.idKey() == reservedDialogID {
					dlg = s
				}
			}
			if dlg == nil {
				t.Fatal("no dialog layer")
			}
			if want := (600 - dlg.Height) / 2; f32Abs(dlg.Y-want) > 0.5 {
				t.Fatalf("dialog top = %v, want %v (centred in the window)", dlg.Y, want)
			}
		})
	}
}

// An older smooth scroll still easing when the keyboard opens is stopped
// even when the field already shows: it would carry the field under the
// keyboard after the reveal.
func TestSoftKeyboardRevealStopsEase(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, func() []View {
		return []View{Column(ContainerCfg{
			ID:         "sc",
			Scrollable: true,
			Sizing:     FillFill,
			Padding:    PaddingNone,
			SizeBorder: NoBorder,
			Content: []View{
				Column(ContainerCfg{
					Sizing: FixedFixed, Width: 10, Height: 500,
					Padding: PaddingNone, SizeBorder: NoBorder,
				}),
				Input(InputCfg{ID: "f"}),
				Column(ContainerCfg{
					Sizing: FixedFixed, Width: 10, Height: 600,
					Padding: PaddingNone, SizeBorder: NoBorder,
				}),
			},
		})}
	})
	w.SetFocus(ScopeID("sc", "f"))
	frame(w)
	w.ScrollVerticalTo("sc", -250)
	frame(w)
	if b := fieldBottom(t, w, ScopeID("sc", "f")); b > 350 {
		t.Fatalf("field bottom = %v, want <= 350 (shows above the keyboard)", b)
	}
	w.ScrollVerticalToSmooth("sc", 0)
	if !revealEaseActive(w, "sc") {
		t.Fatal("no ease armed")
	}
	setInset(w, 250)
	if revealEaseActive(w, "sc") {
		t.Fatal("ease still active after the reveal")
	}
}

// revealEaseActive reports whether a vertical ease runs for id.
func revealEaseActive(w *Window, id string) bool {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	if w.scrollSmooth == nil {
		return false
	}
	e := w.scrollSmooth.findEntry(id, scrollAxisY)
	return e != nil && e.active
}

// An inner scroll container wholly under the keyboard is skipped on the
// first walk. The outer move then brings it into view with the field
// still below its viewport, and a second walk scrolls it.
func TestSoftKeyboardRevealsSkippedInnerScroll(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, func() []View {
		spacer := func(h float32) View {
			return Column(ContainerCfg{
				Sizing: FixedFixed, Width: 10, Height: h,
				Padding: PaddingNone, SizeBorder: NoBorder,
			})
		}
		return []View{Column(ContainerCfg{
			ID:         "out",
			Scrollable: true,
			Sizing:     FillFill,
			Padding:    PaddingNone,
			SizeBorder: NoBorder,
			Content: []View{
				spacer(450),
				Column(ContainerCfg{
					ID:         "in",
					Scrollable: true,
					Sizing:     FillFixed,
					Height:     200,
					Padding:    PaddingNone,
					SizeBorder: NoBorder,
					Content:    []View{spacer(260), Input(InputCfg{ID: "f"})},
				}),
				spacer(600),
			},
		})}
	})
	in := ScopeID("out", "in")
	f := ScopeID(in, "f")
	w.SetFocus(f)
	frame(w)
	setInset(w, 250)
	if got := w.ScrollVerticalOffset(in); got >= 0 {
		t.Fatalf("inner offset = %v, want < 0 (rechecked after the outer move)", got)
	}
	inner, ok := findLayoutByFocusID(&w.layout, f)
	if !ok {
		t.Fatal("no field")
	}
	sc := inner.Parent.Shape
	if b := inner.Shape.Y + inner.Shape.Height; b > sc.Y+sc.Height+0.5 || b > 350+0.5 {
		t.Fatalf("field bottom = %v, want inside inner viewport (bottom %v) and <= 350", b, sc.Y+sc.Height)
	}
}

// A field disabled in the frame the keyboard opens is not revealed: the
// focus repair moves focus off it, and its container must not have
// scrolled for a field that no longer holds focus.
func TestSoftKeyboardNoRevealForDisabledField(t *testing.T) {
	disabled := false
	w := newRevealWindow(t, WindowCfg{}, func() []View {
		return []View{Column(ContainerCfg{
			ID:         "sc",
			Scrollable: true,
			Sizing:     FillFill,
			Padding:    PaddingNone,
			SizeBorder: NoBorder,
			Content: []View{
				Column(ContainerCfg{
					Sizing: FixedFixed, Width: 10, Height: 480,
					Padding: PaddingNone, SizeBorder: NoBorder,
				}),
				Column(ContainerCfg{
					Disabled:   disabled,
					Padding:    PaddingNone,
					SizeBorder: NoBorder,
					Content:    []View{Input(InputCfg{ID: "f"})},
				}),
			},
		})}
	})
	w.SetFocus(ScopeID("sc", "f"))
	frame(w)
	disabled = true
	setInset(w, 250)
	if got := w.ScrollVerticalOffset("sc"); got != 0 {
		t.Fatalf("offset = %v, want 0 (field disabled)", got)
	}
}

// revealTwoFieldContent is a scroll column holding fields "a" and "b"
// 100 px apart, a spacer above them and the given trailing spacer.
func revealTwoFieldContent(tail float32) func() []View {
	return func() []View {
		spacer := func(h float32) View {
			return Column(ContainerCfg{
				Sizing: FixedFixed, Width: 10, Height: h,
				Padding: PaddingNone, SizeBorder: NoBorder,
			})
		}
		return []View{Column(ContainerCfg{
			ID:         "sc",
			Scrollable: true,
			Sizing:     FillFill,
			Padding:    PaddingNone,
			SizeBorder: NoBorder,
			Content: []View{
				spacer(500),
				Input(InputCfg{ID: "a"}),
				spacer(100),
				Input(InputCfg{ID: "b"}),
				spacer(tail),
			},
		})}
	}
}

// An ease the app starts with a focus change, toward an offset that
// keeps the new field in view, is the app's request: the reveal moves no
// container for a field that shows, and the ease runs on.
func TestSoftKeyboardRevealKeepsShowingEase(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, revealTwoFieldContent(600))
	w.SetFocus(ScopeID("sc", "b"))
	frame(w)
	setInset(w, 250) // reveals "b"; "a" shows above it
	off := w.ScrollVerticalOffset("sc")
	w.SetFocus(ScopeID("sc", "a"))
	// 10 px further: "a" still shows.
	w.ScrollVerticalToSmooth("sc", off+10)
	if !revealEaseActive(w, "sc") {
		t.Fatal("no ease armed")
	}
	frame(w)
	if !revealEaseActive(w, "sc") {
		t.Fatal("reveal stopped an ease that keeps the field in view")
	}
}

// A pending index scroll on an ancestor of a field that already shows
// survives the reveal: the reveal moves no container there and writes no
// offset, so the request must keep converging (a virtualized list keeps
// one queued while its rows are measured). An ease that would hide the
// field is still stopped there, without dropping the request
// (scrollSmoothStop, not scrollSmoothCancel). The pass is called
// directly: a plain column settles a request before the reveal could see
// it.
func TestSoftKeyboardRevealKeepsIndexScroll(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, revealTwoFieldContent(600))
	w.SetFocus(ScopeID("sc", "b"))
	frame(w)
	setInset(w, 250) // reveals "b"; "a" shows above it
	w.SetFocus(ScopeID("sc", "a"))
	// Back to the top: "a" at y 500, under the keyboard.
	w.ScrollVerticalToSmooth("sc", 0)
	if !revealEaseActive(w, "sc") {
		t.Fatal("no ease armed")
	}
	w.queueVirtualScroll(virtualScrollReq{scrollID: "sc", frame: w.frameCount})
	before := w.ScrollVerticalOffset("sc")
	layoutRevealFocused(&w.layout.Children[0], w)
	if got := w.ScrollVerticalOffset("sc"); got != before {
		t.Fatalf("offset = %v, want %v (field shows, nothing moves)", got, before)
	}
	if revealEaseActive(w, "sc") {
		t.Fatal("ease that hides the field still active")
	}
	queued := false
	for _, r := range w.virtualScrolls {
		if r.scrollID == "sc" {
			queued = true
		}
	}
	if !queued {
		t.Fatal("index scroll dropped by the reveal")
	}
}

// The keyboard is up and the container is scrolled to its limit, with
// the fields at the end of the content. Focus moves to a field that
// shows, while an ease runs back to the top: there the field would sit
// under the keyboard, so the ease is stopped. The check is clamped
// against the ease's target, not the current offset, which already sits
// at the limit and leaves the reveal no room to move.
func TestSoftKeyboardRevealStopsEaseAtRangeEnd(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, revealTwoFieldContent(10))
	w.SetFocus(ScopeID("sc", "b"))
	frame(w)
	setInset(w, 250)
	w.ScrollVerticalTo("sc", -10000) // clamped to the limit
	frame(w)
	w.SetFocus(ScopeID("sc", "a"))
	if b := fieldBottom(t, w, ScopeID("sc", "a")); b > 350 {
		t.Fatalf("field bottom = %v, want <= 350 (shows above the keyboard)", b)
	}
	w.ScrollVerticalToSmooth("sc", 0)
	if !revealEaseActive(w, "sc") {
		t.Fatal("no ease armed")
	}
	frame(w)
	if revealEaseActive(w, "sc") {
		t.Fatal("ease to the top still active after the reveal")
	}
}

// revealSetEntry puts the vertical smooth-scroll entry for id in the
// given state, as the animation tick would leave it.
func revealSetEntry(w *Window, id string, set func(e *scrollSmoothEntry)) {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	if w.scrollSmooth == nil {
		w.scrollSmooth = &scrollSmoothAnimation{}
	}
	set(w.scrollSmooth.entryFor(id, scrollAxisY))
}

// revealEntryDirty reports whether id's vertical entry holds a final
// value the apply pass has not flushed.
func revealEntryDirty(w *Window, id string) bool {
	w.animMu.Lock()
	defer w.animMu.Unlock()
	if w.scrollSmooth == nil {
		return false
	}
	e := w.scrollSmooth.findEntry(id, scrollAxisY)
	return e != nil && e.dirty
}

// A fling aims past the end of the range; the apply pass stops it at
// the edge. Where it can reach, the field still shows, so the reveal
// lets it run: the check projects the field to the reachable offset.
func TestSoftKeyboardRevealKeepsFlingPastRange(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, revealTwoFieldContent(10))
	w.SetFocus(ScopeID("sc", "b"))
	frame(w)
	setInset(w, 250) // reveals "b", short of the limit
	w.SetFocus(ScopeID("sc", "a"))
	off := w.ScrollVerticalOffset("sc")
	revealSetEntry(w, "sc", func(e *scrollSmoothEntry) {
		e.current, e.target, e.tau = off, -10000, 0.3
		e.active, e.settled, e.dirty = true, false, false
	})
	layoutRevealFocused(&w.layout.Children[0], w)
	if !revealEaseActive(w, "sc") {
		t.Fatal("reveal stopped a fling whose reachable end keeps the field in view")
	}
}

// A retired ease whose final value the apply pass has not flushed yet
// would still move the container on the next flush. When that value
// hides the field, the reveal drops it.
func TestSoftKeyboardRevealDropsPendingFinalValue(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, revealTwoFieldContent(600))
	w.SetFocus(ScopeID("sc", "b"))
	frame(w)
	setInset(w, 250) // reveals "b"; "a" shows above it
	w.SetFocus(ScopeID("sc", "a"))
	// Final value 0: "a" back at y 500, under the keyboard.
	revealSetEntry(w, "sc", func(e *scrollSmoothEntry) {
		e.current, e.target, e.tau = 0, 0, 0
		e.active, e.settled, e.dirty = false, false, true
	})
	layoutRevealFocused(&w.layout.Children[0], w)
	if revealEntryDirty(w, "sc") {
		t.Fatal("unflushed final value that hides the field survived the reveal")
	}
}

// Only a vertically centred float keeps its top in the area above the
// keyboard. A float tied to the top edge keeps the offset it asked for,
// even when it is taller than the area.
func TestSoftKeyboardKeepFloatTopOnlyCentred(t *testing.T) {
	w := newRevealWindow(t, WindowCfg{}, func() []View { return nil })
	setInset(w, 250)
	for _, tc := range []struct {
		name   string
		attach FloatAttach
		fy     float32
		want   float32
	}{
		{"top-left keeps its offset", FloatTopLeft, -24, -24},
		{"centred keeps its top", FloatMiddleCenter, -75, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ly := Layout{Shape: &Shape{
				Float: true, FloatAnchor: tc.attach, FloatTieOff: tc.attach,
				Height: 500,
			}}
			if got := revealKeepFloatTop(&ly, tc.fy, w); got != tc.want {
				t.Fatalf("y = %v, want %v", got, tc.want)
			}
		})
	}
}
