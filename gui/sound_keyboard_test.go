package gui

import "testing"

// Phase 3 of widget audio feedback (issue #468): the paths event
// dispatch never sees — keyboard activation that calls a callback
// directly with a nil Layout, a keystroke a mask or validator refuses,
// an arrow key already at a bound, and a drag that ends on a real move.
//
// Every case drives the real keyboard or drag path. None calls
// playSoundCue or playShapeSound directly: the point of the phase is
// that these paths reach the cue at all, which only the real path
// proves. Theme installation is package-global, so nothing here runs
// in parallel and every test starts with restoreTheme(t).

// --- Table -----------------------------------------------------------

func tableSoundRows() []TableRowCfg {
	return []TableRowCfg{
		{Cells: []TableCellCfg{{Value: "r0"}}},
		{Cells: []TableCellCfg{{Value: "r1"}}},
	}
}

func TestSoundTableKeyboardActivation(t *testing.T) {
	restoreTheme(t)
	activated := 0
	rows := tableSoundRows()
	for i := range rows {
		rows[i].OnClick = func(ctx EventCtx) { activated++ }
	}
	w, spy := soundWindow(t, func(*Window) View {
		return Table(TableCfg{
			ID:        "tbl",
			Focusable: true,
			Data:      rows,
		})
	})
	if err := w.TestKey("tbl", KeyEnter, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if activated != 1 {
		t.Fatalf("row activated %d times, want 1", activated)
	}
	wantCues(t, spy, SoundSelection)
}

func TestSoundTableKeyboardEdgeClampErrors(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, func(*Window) View {
		return Table(TableCfg{
			ID:        "tbl",
			Focusable: true,
			Data:      tableSoundRows(),
		})
	})
	// The active row starts at 0, so Up is refused.
	if err := w.TestKey("tbl", KeyUp, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy, SoundError)
}

func TestSoundTableKeyboardMovementSilent(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, func(*Window) View {
		return Table(TableCfg{
			ID:        "tbl",
			Focusable: true,
			Data:      tableSoundRows(),
		})
	})
	if err := w.TestKey("tbl", KeyDown, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy)
}

// --- Input -----------------------------------------------------------

// inputSoundView renders one Input, letting the caller shape the Cfg.
func inputSoundView(build func(*InputCfg)) func(*Window) View {
	return func(*Window) View {
		cfg := InputCfg{ID: "fld"}
		build(&cfg)
		return Input(cfg)
	}
}

func TestSoundInputMaskRejectErrors(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, inputSoundView(func(c *InputCfg) {
		// A digits-only mask: a letter has no slot to land in.
		c.Mask = "999"
	}))
	if err := w.TestType("fld", "x"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	wantCues(t, spy, SoundError)
}

func TestSoundInputMaskAcceptSilent(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, inputSoundView(func(c *InputCfg) {
		c.Mask = "999"
		c.OnTextChanged = func(string, EventCtx) {}
	}))
	if err := w.TestType("fld", "1"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	wantCues(t, spy)
}

func TestSoundInputValidatorRejectErrors(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, inputSoundView(func(c *InputCfg) {
		c.PreTextChange = func(current, _ string) (string, bool) {
			return current, false
		}
	}))
	if err := w.TestType("fld", "a"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	wantCues(t, spy, SoundError)
}

func TestSoundInputDeleteOverMaskLiteralErrors(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, inputSoundView(func(c *InputCfg) {
		// The cursor sits at 0, so a backspace has no character
		// behind it that the mask allows removing.
		c.Mask = "(999)"
		c.Text = "(123)"
	}))
	if err := w.TestKey("fld", KeyBackspace, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy, SoundError)
}

func TestSoundInputUnmaskedDeleteAtEdgeSilent(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, inputSoundView(func(c *InputCfg) {}))
	// Backspace in an empty unmasked field is an edge, not a
	// rejection.
	if err := w.TestKey("fld", KeyBackspace, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy)
}

func TestSoundInputEnterCommitClicks(t *testing.T) {
	restoreTheme(t)
	committed := ""
	w, spy := soundWindow(t, inputSoundView(func(c *InputCfg) {
		c.Text = "hi"
		c.OnTextCommit = func(s string, _ InputCommitReason, _ EventCtx) {
			committed = s
		}
	}))
	if err := w.TestKey("fld", KeyEnter, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	if committed != "hi" {
		t.Fatalf("committed %q, want %q", committed, "hi")
	}
	wantCues(t, spy, SoundClick)
}

// The Enter commit is the one Input path that takes Cfg.Sound; the
// reject path stays on the theme's Error role either way.
func TestSoundInputCfgOverridesCommitNotReject(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, inputSoundView(func(c *InputCfg) {
		c.Mask = "999"
		c.Sound = SoundSelection
		c.OnTextCommit = func(string, InputCommitReason, EventCtx) {}
	}))
	if err := w.TestType("fld", "x"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if err := w.TestKey("fld", KeyEnter, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy, SoundError, SoundSelection)
}

func TestSoundInputDisabledSuppressesBoth(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, inputSoundView(func(c *InputCfg) {
		c.Mask = "999"
		c.SoundDisabled = true
		c.OnTextCommit = func(string, InputCommitReason, EventCtx) {}
	}))
	if err := w.TestType("fld", "x"); err != nil {
		t.Fatalf("TestType: %v", err)
	}
	if err := w.TestKey("fld", KeyEnter, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy)
}

// The acceptance criterion for the phase: SoundError reaches
// NewBeepSoundPlayer, the player that plays that cue and nothing else,
// through the real reject path.
func TestSoundInputRejectReachesBeepPlayer(t *testing.T) {
	restoreTheme(t)
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	w.SetSoundPlayer(NewBeepSoundPlayer(w))
	w.TestRender(inputSoundView(func(c *InputCfg) {
		c.Mask = "999"
		c.OnTextChanged = func(string, EventCtx) {}
	}))
	// No native platform in a headless window, so the beep is a
	// no-op; the assertion is that neither the accepted keystroke nor
	// the refused one panics on the way there.
	if err := w.TestType("fld", "1"); err != nil {
		t.Fatalf("TestType accepted: %v", err)
	}
	if err := w.TestType("fld", "x"); err != nil {
		t.Fatalf("TestType refused: %v", err)
	}
}

// --- NumericInput ----------------------------------------------------

func numericSoundView(value, minVal, maxVal float64) func(*Window) View {
	return func(*Window) View {
		return NumericInput(NumericInputCfg{
			ID:      "num",
			Value:   Some(value),
			Min:     Some(minVal),
			Max:     Some(maxVal),
			StepCfg: NumericStepCfg{Step: 1, ShowButtons: true},
			OnValueCommit: func(Opt[float64], string, EventCtx) {
			},
		})
	}
}

func TestSoundNumericStepClampErrors(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, numericSoundView(10, 0, 10))
	if err := w.TestClick(ScopeID("num", "step_up")); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if len(spy.cues) == 0 || spy.cues[len(spy.cues)-1] != SoundError {
		t.Fatalf("cues = %v, want the last to be SoundError", spy.cues)
	}
}

func TestSoundNumericStepThatMovesDoesNotError(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, numericSoundView(5, 0, 10))
	if err := w.TestClick(ScopeID("num", "step_up")); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	for _, c := range spy.cues {
		if c == SoundError {
			t.Fatalf("cues = %v, want no SoundError", spy.cues)
		}
	}
}

// --- Slider ----------------------------------------------------------

func sliderSoundView(value float32) func(*Window) View {
	return func(*Window) View {
		return Slider(SliderCfg{
			ID:       "sld",
			Value:    value,
			Min:      0,
			Max:      10,
			Step:     1,
			OnChange: func(float32, EventCtx) {},
		})
	}
}

func TestSoundSliderArrowAtBoundErrors(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, sliderSoundView(10))
	if err := w.TestKey("sld", KeyRight, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy, SoundError)
}

func TestSoundSliderArrowThatMovesIsSilent(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, sliderSoundView(5))
	if err := w.TestKey("sld", KeyRight, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy)
}

// --- ListBox ---------------------------------------------------------

func listBoxSoundView(sel []string) func(*Window) View {
	return func(*Window) View {
		return ListBox(ListBoxCfg{
			ID: "lb",
			Data: []ListBoxOption{
				{ID: "a", Name: "Alpha"},
				{ID: "b", Name: "Beta"},
			},
			SelectedIDs: sel,
			OnSelect:    func([]string, EventCtx) {},
		})
	}
}

func TestSoundListBoxKeyboardActivationSelects(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, listBoxSoundView(nil))
	if err := w.TestKey("lb", KeyEnter, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy, SoundSelection)
}

func TestSoundListBoxArrowMovementSilentEdgeErrors(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, listBoxSoundView(nil))
	// Down moves off row 0 and stays silent.
	if err := w.TestKey("lb", KeyDown, ModNone); err != nil {
		t.Fatalf("TestKey down: %v", err)
	}
	wantCues(t, spy)
	// Down again is refused: row 1 is the last.
	if err := w.TestKey("lb", KeyDown, ModNone); err != nil {
		t.Fatalf("TestKey down again: %v", err)
	}
	wantCues(t, spy, SoundError)
}

// --- Drag reorder ----------------------------------------------------

// dragReorderSoundState arms a drag the way dragReorderStart does,
// then drives the real mouse-up. Building the state directly is the
// only way to reach the drop without a backend to move the mouse.
func dragReorderSoundState(
	w *Window, key string, cue SoundCue, src, gap int,
) {
	w.layout = Layout{Shape: &Shape{ID: "root"}}
	state := dragReorderState{
		started:      true,
		active:       true,
		sourceIndex:  src,
		currentIndex: gap,
		itemCount:    3,
		dropCue:      cue,
	}
	dragReorderSet(w, key, state)
}

func TestSoundDragReorderDropSelects(t *testing.T) {
	restoreTheme(t)
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	spy := &soundSpy{}
	w.SetSoundPlayer(spy)
	dragReorderSoundState(w, "lb", SoundSelection, 0, 2)

	moved := false
	dragReorderOnMouseUp("lb", []string{"a", "b", "c"},
		func(string, string, EventCtx) { moved = true }, w)
	if !moved {
		t.Fatal("a real move must fire OnReorder")
	}
	wantCues(t, spy, SoundSelection)
}

func TestSoundDragReorderNoOpDropSilent(t *testing.T) {
	restoreTheme(t)
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	spy := &soundSpy{}
	w.SetSoundPlayer(spy)
	// gap == src: the item lands where it already was.
	dragReorderSoundState(w, "lb", SoundSelection, 1, 1)

	dragReorderOnMouseUp("lb", []string{"a", "b", "c"},
		func(string, string, EventCtx) {
			t.Fatal("a no-op drop must not reorder")
		}, w)
	wantCues(t, spy)
}

func TestSoundDragReorderCancelSilent(t *testing.T) {
	restoreTheme(t)
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	spy := &soundSpy{}
	w.SetSoundPlayer(spy)
	dragReorderSoundState(w, "lb", SoundSelection, 0, 2)
	state := dragReorderGet(w, "lb")
	state.cancelled = true
	dragReorderSet(w, "lb", state)

	dragReorderOnMouseUp("lb", []string{"a", "b", "c"},
		func(string, string, EventCtx) {
			t.Fatal("a cancelled drag must not reorder")
		}, w)
	wantCues(t, spy)
}

func TestSoundDragReorderKeyboardMoveSelects(t *testing.T) {
	restoreTheme(t)
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	spy := &soundSpy{}
	w.SetSoundPlayer(spy)
	w.layout = Layout{Shape: &Shape{ID: "root"}}

	if !dragReorderKeyboardMove(KeyDown, ModAlt, dragReorderVertical,
		0, []string{"a", "b", "c"},
		func(string, string, EventCtx) {}, SoundSelection, w) {
		t.Fatal("Alt+Down must reorder")
	}
	wantCues(t, spy, SoundSelection)

	// At the last index there is nowhere to go: no move, no cue.
	spy.cues = nil
	if dragReorderKeyboardMove(KeyDown, ModAlt, dragReorderVertical,
		2, []string{"a", "b", "c"},
		func(string, string, EventCtx) {}, SoundSelection, w) {
		t.Fatal("Alt+Down at the last index must be a no-op")
	}
	wantCues(t, spy)
}

// --- Splitter --------------------------------------------------------

func splitterSoundView(collapsed SplitterCollapsed) func(*Window) View {
	return func(*Window) View {
		return Splitter(SplitterCfg{
			ID:        "spl",
			Focusable: true,
			Collapsed: collapsed,
			First: SplitterPaneCfg{
				Collapsible: true,
				Content:     []View{Text(TextCfg{Text: "one"})},
			},
			Second: SplitterPaneCfg{
				Content: []View{Text(TextCfg{Text: "two"})},
			},
			OnChange: func(float32, SplitterCollapsed, EventCtx) {},
		})
	}
}

func TestSoundSplitterCollapseToggles(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, splitterSoundView(splitterCollapseNone))
	if err := w.TestKey("spl", KeyHome, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy, SoundToggleOn)
}

func TestSoundSplitterExpandTogglesOff(t *testing.T) {
	restoreTheme(t)
	w, spy := soundWindow(t, splitterSoundView(SplitterCollapseFirst))
	// Enter on an already-collapsed splitter expands it.
	if err := w.TestKey("spl", KeyEnter, ModNone); err != nil {
		t.Fatalf("TestKey: %v", err)
	}
	wantCues(t, spy, SoundToggleOff)
}

// --- Silence guarantees ----------------------------------------------

// Every new site must stay silent and panic-free with no player, with
// a theme that names no cues, and at zero volume. Driven through the
// same real paths as the sounding cases above.
func TestSoundPhase3SilentConfigurations(t *testing.T) {
	restoreTheme(t)
	drive := func(t *testing.T, w *Window) {
		t.Helper()
		w.TestRender(func(*Window) View {
			return Column(ContainerCfg{
				ID: "root",
				Content: []View{
					Input(InputCfg{ID: "fld", Mask: "999"}),
					Slider(SliderCfg{
						ID: "sld", Value: 10, Min: 0, Max: 10,
						Step:     1,
						OnChange: func(float32, EventCtx) {},
					}),
					Table(TableCfg{
						ID: "tbl", Focusable: true,
						Data: tableSoundRows(),
					}),
				},
			})
		})
		if err := w.TestType(ScopeID("root", "fld"), "x"); err != nil {
			t.Fatalf("TestType: %v", err)
		}
		if err := w.TestKey(ScopeID("root", "sld"),
			KeyRight, ModNone); err != nil {
			t.Fatalf("TestKey slider: %v", err)
		}
		if err := w.TestKey(ScopeID("root", "tbl"),
			KeyUp, ModNone); err != nil {
			t.Fatalf("TestKey table: %v", err)
		}
	}

	t.Run("no player", func(t *testing.T) {
		restoreTheme(t)
		w := NewTestWindow(WindowCfg{})
		w.SetTheme(soundingTheme(t))
		drive(t, w)
	})

	t.Run("silent theme", func(t *testing.T) {
		restoreTheme(t)
		w := NewTestWindow(WindowCfg{})
		w.SetTheme(silentTheme(t))
		spy := &soundSpy{}
		w.SetSoundPlayer(spy)
		drive(t, w)
		wantCues(t, spy)
	})

	t.Run("zero volume", func(t *testing.T) {
		restoreTheme(t)
		w := NewTestWindow(WindowCfg{})
		w.SetTheme(soundingTheme(t))
		spy := &soundSpy{}
		w.SetSoundPlayer(spy)
		w.SetSoundVolume(0)
		drive(t, w)
		wantCues(t, spy)
	})
}
