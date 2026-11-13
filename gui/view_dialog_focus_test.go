package gui

import "testing"

// A DialogCustom dialog keeps keyboard focus inside the modal (#1242).
// Before the fix the dialog focused an ID its custom body never
// carries, and the frame's focus repair moved focus to the first tab
// stop behind the modal, every frame. A Menubar there took focus,
// selected its first item and opened its menu.
func TestDialogCustomFocusStaysInDialog(t *testing.T) {
	inside := ScopeID(reservedDialogID, "inside")
	tests := []struct {
		name    string
		focusID string
		// body builds the custom view; lock disables the Row around
		// the "next" button, for the case that turns it off.
		body func(lock bool) []View
		// focusNext puts focus on the "next" button and then disables
		// its Row, so the button loses focus through its container.
		focusNext bool
		want      string
	}{
		{
			name: "no FocusID focuses the first tab stop",
			body: func(bool) []View {
				return []View{Input(InputCfg{ID: "inside"})}
			},
			want: inside,
		},
		{
			name:    "a misspelled FocusID focuses the first tab stop",
			focusID: "insde",
			body: func(bool) []View {
				return []View{Input(InputCfg{ID: "inside"})}
			},
			want: inside,
		},
		{
			name: "no tab stops leaves focus empty",
			body: func(bool) []View {
				return []View{Text(TextCfg{Text: "only text"})}
			},
			want: "",
		},
		{
			// The dialog focus runs before the disabled container
			// turns its stops off. Picking one there had the frame's
			// repair clear it again, every frame.
			name: "every stop disabled through its container leaves focus empty",
			body: func(bool) []View {
				return []View{Row(ContainerCfg{Disabled: true, Content: []View{
					Input(InputCfg{ID: "inside"}),
					Button(ButtonCfg{
						ID:      "next",
						Content: []View{Text(TextCfg{Text: "Next"})},
					}),
				}})}
			},
			want: "",
		},
		{
			// As above, through the FocusID check rather than the
			// first tab stop.
			name:    "a FocusID under a disabled container leaves focus empty",
			focusID: "locked",
			body: func(bool) []View {
				return []View{
					Row(ContainerCfg{Disabled: true, Content: []View{
						Input(InputCfg{ID: "locked"}),
					}}),
					Text(TextCfg{Text: "nothing else takes focus"}),
				}
			},
			want: "",
		},
		{
			name:    "a stop disabled through its container moves focus in the dialog",
			focusID: "inside",
			body: func(lock bool) []View {
				return []View{
					Input(InputCfg{ID: "inside"}),
					Row(ContainerCfg{Disabled: lock, Content: []View{
						Button(ButtonCfg{
							ID:      "next",
							Content: []View{Text(TextCfg{Text: "Next"})},
						}),
					}}),
				}
			},
			focusNext: true,
			want:      inside,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := NewTestWindow(t, WindowCfg{})
			t.Cleanup(w.WindowCleanup)
			w.TestRender(func(vw *Window) View {
				return Column(ContainerCfg{Sizing: FillFill, Content: []View{
					Menubar(vw, MenubarCfg{
						ID: "bar",
						Items: []MenuItemCfg{
							MenuItemText("file", "File"),
						},
					}),
					Input(InputCfg{ID: "background"}),
				}})
			})
			lock := false
			body := tc.body
			w.Dialog(DialogCfg{
				Title:      "Custom",
				DialogType: DialogCustom,
				FocusID:    tc.focusID,
				CustomView: func(*Window) View {
					return Column(ContainerCfg{Content: body(lock)})
				},
			})
			if tc.focusNext {
				w.TestRender(nil)
				w.SetFocus(ScopeID(reservedDialogID, "next"))
				w.TestRender(nil)
				lock = true
			}
			// The repair has to hold on the first frame: one frame on
			// the Menubar is enough to open its menu. Later frames
			// must not move focus again.
			var settled uint64
			for frame := range 3 {
				w.TestRender(nil)
				if got := w.FocusID(); got != tc.want {
					t.Fatalf("frame %d: FocusID() = %q, want %q",
						frame+1, got, tc.want)
				}
				if frame == 0 {
					settled = w.viewState.focusSetCount
				} else if n := w.viewState.focusSetCount; n != settled {
					t.Fatalf("frame %d: focus set %d more times after it settled",
						frame+1, n-settled)
				}
				if sel := StateReadOr(w, nsMenu, "bar", ""); sel != "" {
					t.Fatalf("frame %d: the Menubar behind the dialog selected %q",
						frame+1, sel)
				}
			}
		})
	}
}

// A FocusID on the turned-away face of a FlipPanel is no target. The
// panel takes focus off that face with a queued command; a dialog that
// focused it again fought the panel every other frame. Focus goes to
// the first visible tab stop instead (#1242).
func TestDialogFocusSkipsTurnedAwayFace(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{})
	t.Cleanup(w.WindowCleanup)
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill})
	})
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		// The back face is turned away while Flipped is false. The
		// panel has no ID scope, so the leaf resolves under the dialog.
		FocusID: "secret",
		CustomView: func(*Window) View {
			return Column(ContainerCfg{Content: []View{
				Button(ButtonCfg{
					ID:      "visible",
					Content: []View{Text(TextCfg{Text: "Visible"})},
				}),
				// FocusDisabled: the panel clears focus rather than
				// taking it, which is what let the two fight.
				FlipPanel(FlipPanelCfg{
					FocusDisabled: true,
					Front:         Text(TextCfg{Text: "front"}),
					Back:          Input(InputCfg{ID: "secret"}),
				}),
			}})
		},
	})
	want := ScopeID(reservedDialogID, "visible")
	var settled uint64
	for frame := range 4 {
		// A frame runs the queued commands first, as FrameFn does.
		w.flushCommands()
		w.TestRender(nil)
		if got := w.FocusID(); got != want {
			t.Fatalf("frame %d: FocusID() = %q, want %q", frame+1, got, want)
		}
		if frame == 0 {
			settled = w.viewState.focusSetCount
		} else if n := w.viewState.focusSetCount; n != settled {
			t.Fatalf("frame %d: focus set %d more times after it settled",
				frame+1, n-settled)
		}
	}
}

// retainDialogFocus moves stolen focus to the dialog's first tab stop
// when the dialog's focus ID names no widget in it (#1242).
func TestRetainDialogFocus_CustomTakesFirstTabStop(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		CustomView: func(*Window) View {
			return Column(ContainerCfg{Content: []View{
				Button(ButtonCfg{ID: "one", Content: []View{Text(TextCfg{Text: "1"})}}),
				Button(ButtonCfg{ID: "two", Content: []View{Text(TextCfg{Text: "2"})}}),
			}})
		},
	})
	dialog := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)

	w.SetFocus("f42")
	w.retainDialogFocus(&dialog, nil)

	if got, want := w.FocusID(), ScopeID(reservedDialogID, "one"); got != want {
		t.Fatalf("focus = %q, want first tab stop %q", got, want)
	}
}

// A dialog with no tab stops clears stolen focus instead of parking it
// on an ID no widget carries, and leaves empty focus alone (#1242).
func TestRetainDialogFocus_NoTabStopsClearsFocus(t *testing.T) {
	w := NewWindow(WindowCfg{})
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		CustomView: func(*Window) View {
			return Text(TextCfg{Text: "only text"})
		},
	})
	dialog := generateViewLayout(dialogViewGenerator(w.dialogCfg), w)

	w.SetFocus("f42")
	w.retainDialogFocus(&dialog, nil)
	if got := w.FocusID(); got != "" {
		t.Fatalf("focus = %q, want cleared", got)
	}

	before := w.viewState.focusSetCount
	w.retainDialogFocus(&dialog, nil)
	if n := w.viewState.focusSetCount - before; n != 0 {
		t.Fatalf("empty focus re-set %d times, want 0", n)
	}
}
