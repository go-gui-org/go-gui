package gui

import "testing"

// Phase 4 (issue #469): the cues for interactions with no click site —
// a toast appearing, a dialog opening, a form submit landing or being
// refused. Theme installation is package-global, so these must not run
// in parallel; see the note at the top of sound_test.go.

// soundNonClickWindow builds a window with the sounding theme and a
// spy, but renders nothing: Toast and Dialog are imperative APIs, not
// views.
func soundNonClickWindow(t *testing.T) (*Window, *soundSpy) {
	t.Helper()
	restoreTheme(t)
	spy := &soundSpy{}
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	w.SetSoundPlayer(spy)
	return w, spy
}

func TestSoundToastAppearBySeverity(t *testing.T) {
	cases := []struct {
		name     string
		severity ToastSeverity
		want     SoundCue
	}{
		{"info", ToastInfo, SoundNotify},
		{"success", ToastSuccess, SoundNotify},
		{"warning", ToastWarning, SoundNotify},
		// The one place severity picks a cue: an error toast reports a
		// failure and takes the theme's Error role.
		{"error", ToastError, SoundError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, spy := soundNonClickWindow(t)
			w.Toast(ToastCfg{Title: "T", Severity: tc.severity})
			if len(spy.cues) != 1 || spy.cues[0] != tc.want {
				t.Errorf("cues = %v, want [%v]", spy.cues, tc.want)
			}
		})
	}
}

func TestSoundToastAppearOncePerToast(t *testing.T) {
	w, spy := soundNonClickWindow(t)
	w.Toast(ToastCfg{Title: "A"})
	w.Toast(ToastCfg{Title: "B"})
	// Rendering the toasts again must not re-emit: the cue rides the
	// Toast call, not the per-frame toast view.
	w.TestRender(func(*Window) View { return Column(ContainerCfg{}) })
	w.TestRender(func(*Window) View { return Column(ContainerCfg{}) })
	if len(spy.cues) != 2 {
		t.Errorf("cues = %v, want exactly two appear cues", spy.cues)
	}
}

func TestSoundToastSilentCases(t *testing.T) {
	t.Run("sound disabled", func(t *testing.T) {
		w, spy := soundNonClickWindow(t)
		w.Toast(ToastCfg{Title: "T", SoundDisabled: true})
		if len(spy.cues) != 0 {
			t.Errorf("SoundDisabled emitted %v, want nothing", spy.cues)
		}
	})
	t.Run("silent theme", func(t *testing.T) {
		restoreTheme(t)
		spy := &soundSpy{}
		w := NewTestWindow(WindowCfg{})
		w.SetTheme(silentTheme(t))
		w.SetSoundPlayer(spy)
		w.Toast(ToastCfg{Title: "T"})
		if len(spy.cues) != 0 {
			t.Errorf("silent theme emitted %v, want nothing", spy.cues)
		}
	})
	t.Run("dismiss", func(t *testing.T) {
		w, spy := soundNonClickWindow(t)
		id := w.Toast(ToastCfg{Title: "T"})
		spy.cues = nil
		w.ToastDismiss(id)
		w.ToastDismissAll()
		if len(spy.cues) != 0 {
			t.Errorf("dismiss emitted %v, want nothing", spy.cues)
		}
	})
}

// ToastCfg.Sound names the buttons' activation cue, so it must not
// reach the appear cue.
func TestSoundToastCfgSoundDoesNotOverrideAppear(t *testing.T) {
	w, spy := soundNonClickWindow(t)
	w.Toast(ToastCfg{Title: "T", Sound: SoundClick})
	if len(spy.cues) != 1 || spy.cues[0] != SoundNotify {
		t.Errorf("cues = %v, want [SoundNotify]", spy.cues)
	}
}

func TestSoundDialogOpen(t *testing.T) {
	w, spy := soundNonClickWindow(t)
	w.Dialog(DialogCfg{DialogType: DialogConfirm, Title: "Quit?"})
	if len(spy.cues) != 1 || spy.cues[0] != SoundOpen {
		t.Errorf("cues = %v, want [SoundOpen]", spy.cues)
	}
	// Dismiss is silent: the button that closed the dialog has already
	// sounded through the ordinary click path.
	spy.cues = nil
	w.DialogDismiss()
	if len(spy.cues) != 0 {
		t.Errorf("DialogDismiss emitted %v, want nothing", spy.cues)
	}
}

func TestSoundDialogSoundDisabled(t *testing.T) {
	w, spy := soundNonClickWindow(t)
	w.Dialog(DialogCfg{
		DialogType: DialogConfirm, Title: "Quit?", SoundDisabled: true,
	})
	if len(spy.cues) != 0 {
		t.Errorf("SoundDisabled emitted %v, want nothing", spy.cues)
	}
}

// soundFormSubmit registers one required field with value, applies cfg,
// then latches and processes a submit. It returns the cues emitted.
func soundFormSubmit(t *testing.T, cfg FormCfg, value string) []SoundCue {
	t.Helper()
	restoreTheme(t)
	spy := &soundSpy{}
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	w.SetSoundPlayer(spy)

	required := func(f FormFieldSnapshot, _ FormSnapshot) []FormIssue {
		if f.Value == "" {
			return []FormIssue{{Msg: "required"}}
		}
		return nil
	}
	cfg.ID = "sound-form"
	FormRegisterFieldByID(w, cfg.ID, FormFieldAdapterCfg{
		FieldID:        "name",
		Value:          value,
		SyncValidators: []FormSyncValidator{required},
	})
	formApplyCfg(w, cfg.ID, cfg)
	// Resolved off the window's theme rather than the guiTheme mirror:
	// this helper renders nothing, so no frame has installed the mirror
	// yet. The rendered case below covers the factory's own resolution.
	sounds := w.Theme().Sounds
	cues := soundCues{
		act:    ResolveSoundCue(sounds.Success, cfg.Sound, cfg.SoundDisabled),
		reject: ResolveSoundCue(sounds.Error, SoundNone, cfg.SoundDisabled),
	}
	state := formRuntime(w, cfg.ID)
	state.submitReq = true
	formProcessRequests(w, cfg.ID, func(FormSubmitEvent, EventCtx) {}, nil, cues)
	// A second pass with no request must stay silent: AmendLayout runs
	// every frame and the submitReq latch is what makes the cue
	// one-shot.
	formProcessRequests(w, cfg.ID, func(FormSubmitEvent, EventCtx) {}, nil, cues)
	return spy.cues
}

func TestSoundFormSubmitAcceptedAndBlocked(t *testing.T) {
	t.Run("accepted", func(t *testing.T) {
		got := soundFormSubmit(t, FormCfg{}, "Alice")
		if len(got) != 1 || got[0] != SoundSuccess {
			t.Errorf("cues = %v, want [SoundSuccess]", got)
		}
	})
	t.Run("blocked by validation", func(t *testing.T) {
		got := soundFormSubmit(t, FormCfg{}, "")
		if len(got) != 1 || got[0] != SoundError {
			t.Errorf("cues = %v, want [SoundError]", got)
		}
	})
	t.Run("invalid allowed through", func(t *testing.T) {
		// AllowInvalidSubmit means the submit lands, so it is an
		// acceptance and takes the success cue.
		got := soundFormSubmit(t, FormCfg{AllowInvalidSubmit: true}, "")
		if len(got) != 1 || got[0] != SoundSuccess {
			t.Errorf("cues = %v, want [SoundSuccess]", got)
		}
	})
}

// A submit blocked by a pending async validator is a refusal like any
// other and takes the Error role. The submit sweep re-pends the field
// synchronously before the summary is computed, so the block lands in
// the pass that requested it. This pins the sound gate to the same
// condition as the callback gate: a refactor that dropped
// blockedPending from one but not the other would split them.
func TestSoundFormSubmitBlockedByPending(t *testing.T) {
	restoreTheme(t)
	spy := &soundSpy{}
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	w.SetSoundPlayer(spy)

	formID := "sound-form-pending"
	asyncVal := func(
		FormFieldSnapshot, FormSnapshot, *GridAbortSignal,
	) []FormIssue {
		return nil
	}
	FormRegisterFieldByID(w, formID, FormFieldAdapterCfg{
		FieldID:         "name",
		Value:           "Alice",
		AsyncValidators: []FormAsyncValidator{asyncVal},
	})
	formApplyCfg(w, formID, FormCfg{})
	// Resolved off the window's theme, as in soundFormSubmit: nothing
	// renders here, so no frame has installed the guiTheme mirror.
	sounds := w.Theme().Sounds
	cues := soundCues{
		act:    ResolveSoundCue(sounds.Success, SoundNone, false),
		reject: ResolveSoundCue(sounds.Error, SoundNone, false),
	}
	state := formRuntime(w, formID)
	state.submitReq = true
	formProcessRequests(w, formID, nil, nil, cues)
	if len(spy.cues) != 1 || spy.cues[0] != SoundError {
		t.Errorf("cues = %v, want [SoundError]", spy.cues)
	}
}

func TestSoundFormCfgOverrideAndDisable(t *testing.T) {
	t.Run("Sound overrides the acceptance", func(t *testing.T) {
		got := soundFormSubmit(t, FormCfg{Sound: SoundClick}, "Alice")
		if len(got) != 1 || got[0] != SoundClick {
			t.Errorf("cues = %v, want [SoundClick]", got)
		}
	})
	t.Run("Sound does not override the refusal", func(t *testing.T) {
		got := soundFormSubmit(t, FormCfg{Sound: SoundClick}, "")
		if len(got) != 1 || got[0] != SoundError {
			t.Errorf("cues = %v, want [SoundError]", got)
		}
	})
	t.Run("SoundDisabled suppresses both", func(t *testing.T) {
		if got := soundFormSubmit(t, FormCfg{
			Sound: SoundClick, SoundDisabled: true,
		}, "Alice"); len(got) != 0 {
			t.Errorf("accepted emitted %v, want nothing", got)
		}
		if got := soundFormSubmit(t, FormCfg{SoundDisabled: true}, ""); len(got) != 0 {
			t.Errorf("blocked emitted %v, want nothing", got)
		}
	})
}

// The Form view resolves its own cues at generation time, and the
// submit travels the path an app actually takes: a button asks for the
// submit, the next frame's AmendLayout processes it.
func TestSoundFormThroughRenderedView(t *testing.T) {
	restoreTheme(t)
	spy := &soundSpy{}
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	w.SetSoundPlayer(spy)
	formID := "rendered-form"
	view := func(*Window) View {
		return Form(FormCfg{ID: formID, Content: []View{
			Button(ButtonCfg{ID: "go", Label: "Go", OnClick: func(ctx EventCtx) {
				FormRequestSubmit(ctx.Window, formID)
				ctx.Consume()
			}}),
		}})
	}
	w.TestRender(view)
	spy.cues = nil
	w.TestClick(ScopeID(formLayoutID(formID), "go"))
	// The button's own click cue, then the submit's success cue.
	want := []SoundCue{SoundClick, SoundSuccess}
	if len(spy.cues) != len(want) || spy.cues[0] != want[0] || spy.cues[1] != want[1] {
		t.Errorf("cues = %v, want %v", spy.cues, want)
	}
}
