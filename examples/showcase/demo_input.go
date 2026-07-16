package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func demoInput(w *gui.Window) gui.View {
	t := gui.CurrentTheme()
	app := gui.State[ShowcaseApp](w)

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      "Accessibility: supports IME composition, keyboard tab focus, masked input, and multiline editing.",
				TextStyle: t.N3,
				Mode:      gui.TextModeWrap,
			}),
			gui.Switch(gui.SwitchCfg{
				ID:       "input-spell-check",
				Label:    "Spell Check",
				Selected: app.InputSpellCheck,
				OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).InputSpellCheck = !gui.State[ShowcaseApp](w).InputSpellCheck
				},
			}),
			labeledRow(t, "Text", gui.Input(gui.InputCfg{
				ID:          "input-text",
				Sizing:      gui.FillFit,
				Text:        app.InputText,
				Placeholder: "Enter text...",
				SpellCheck:  app.InputSpellCheck,
				OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
					gui.State[ShowcaseApp](w).InputText = s
				},
			})),
			labeledRow(t, "Password", gui.Input(gui.InputCfg{
				ID:          "input-password",
				Sizing:      gui.FillFit,
				Text:        app.InputPassword,
				Placeholder: "Enter password...",
				IsPassword:  true,
				OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
					gui.State[ShowcaseApp](w).InputPassword = s
				},
			})),
			labeledRow(t, "Phone", gui.Input(gui.InputCfg{
				ID:          "input-phone",
				Sizing:      gui.FillFit,
				Text:        app.InputPhone,
				Placeholder: "(555) 000-0000",
				MaskPreset:  gui.MaskPhoneUS,
				OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
					gui.State[ShowcaseApp](w).InputPhone = s
				},
			})),
			labeledRow(t, "Expiry", gui.Input(gui.InputCfg{
				ID:          "input-expiry",
				Sizing:      gui.FillFit,
				Text:        app.InputExpiry,
				Placeholder: "MM/YY",
				MaskPreset:  gui.MaskExpiryMMYY,
				OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
					gui.State[ShowcaseApp](w).InputExpiry = s
				},
			})),
			labeledRow(t, "Multiline", gui.Input(gui.InputCfg{
				ID:          "input-multi",
				Sizing:      gui.FillFit,
				Text:        app.InputMultiline,
				Placeholder: "Multiple lines...",
				Mode:        gui.InputMultiline,
				Height:      90,
				SpellCheck:  app.InputSpellCheck,
				OnTextChanged: func(_ *gui.Layout, s string, w *gui.Window) {
					gui.State[ShowcaseApp](w).InputMultiline = s
				},
			})),
		},
	})
}

func demoNumericInput(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	titleStyle := gui.CurrentTheme().B3
	bodyStyle := gui.CurrentTheme().N3

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Text(gui.TextCfg{Text: "Default (en_US)", TextStyle: titleStyle}),
			gui.NumericInput(gui.NumericInputCfg{
				ID:       "num-en",
				Text:     app.NumericENText,
				Value:    app.NumericENValue,
				Decimals: 2,
				Min:      gui.Some(0.0),
				Max:      gui.Some(10000.0),
				Width:    220,
				Sizing:   gui.FixedFit,
				OnTextChanged: func(_ *gui.Layout, text string, w *gui.Window) {
					gui.State[ShowcaseApp](w).NumericENText = text
				},
				OnValueCommit: func(_ *gui.Layout, value gui.Opt[float64], text string, w *gui.Window) {
					app := gui.State[ShowcaseApp](w)
					app.NumericENValue = value
					app.NumericENText = text
				},
			}),
			gui.Text(gui.TextCfg{Text: "Committed: " + numericValueText(app.NumericENValue), TextStyle: bodyStyle}),

			gui.Text(gui.TextCfg{Text: "German (de_DE)", TextStyle: titleStyle}),
			gui.NumericInput(gui.NumericInputCfg{
				ID:       "num-de",
				Text:     app.NumericDEText,
				Value:    app.NumericDEValue,
				Decimals: 2,
				Locale: gui.NumericLocaleCfg{
					DecimalSep: ',',
					GroupSep:   '.',
				},
				Min:    gui.Some(0.0),
				Max:    gui.Some(10000.0),
				Width:  220,
				Sizing: gui.FixedFit,
				OnTextChanged: func(_ *gui.Layout, text string, w *gui.Window) {
					gui.State[ShowcaseApp](w).NumericDEText = text
				},
				OnValueCommit: func(_ *gui.Layout, value gui.Opt[float64], text string, w *gui.Window) {
					app := gui.State[ShowcaseApp](w)
					app.NumericDEValue = value
					app.NumericDEText = text
				},
			}),
			gui.Text(gui.TextCfg{Text: "Committed: " + numericValueText(app.NumericDEValue), TextStyle: bodyStyle}),

			gui.Text(gui.TextCfg{Text: "Currency mode", TextStyle: titleStyle}),
			gui.NumericInput(gui.NumericInputCfg{
				ID:       "num-currency",
				Text:     app.NumericCurrencyText,
				Value:    app.NumericCurrencyValue,
				Mode:     gui.NumericCurrency,
				Decimals: 2,
				Min:      gui.Some(0.0),
				Max:      gui.Some(10000.0),
				Width:    220,
				Sizing:   gui.FixedFit,
				OnTextChanged: func(_ *gui.Layout, text string, w *gui.Window) {
					gui.State[ShowcaseApp](w).NumericCurrencyText = text
				},
				OnValueCommit: func(_ *gui.Layout, value gui.Opt[float64], text string, w *gui.Window) {
					app := gui.State[ShowcaseApp](w)
					app.NumericCurrencyValue = value
					app.NumericCurrencyText = text
				},
			}),
			gui.Text(gui.TextCfg{Text: "Committed: " + numericValueText(app.NumericCurrencyValue), TextStyle: bodyStyle}),

			gui.Text(gui.TextCfg{Text: "Percent mode (ratio value)", TextStyle: titleStyle}),
			gui.NumericInput(gui.NumericInputCfg{
				ID:       "num-percent",
				Text:     app.NumericPercentText,
				Value:    app.NumericPercentValue,
				Mode:     gui.NumericPercent,
				Decimals: 2,
				Min:      gui.Some(0.0),
				Max:      gui.Some(1.0),
				Width:    220,
				Sizing:   gui.FixedFit,
				OnTextChanged: func(_ *gui.Layout, text string, w *gui.Window) {
					gui.State[ShowcaseApp](w).NumericPercentText = text
				},
				OnValueCommit: func(_ *gui.Layout, value gui.Opt[float64], text string, w *gui.Window) {
					app := gui.State[ShowcaseApp](w)
					app.NumericPercentValue = value
					app.NumericPercentText = text
				},
			}),
			gui.Text(gui.TextCfg{Text: "Committed ratio: " + numericValueText(app.NumericPercentValue), TextStyle: bodyStyle}),

			gui.Text(gui.TextCfg{Text: "Plain", TextStyle: titleStyle}),
			gui.NumericInput(gui.NumericInputCfg{
				ID:       "num-plain",
				Text:     app.NumericPlainText,
				Value:    app.NumericPlainValue,
				Decimals: 0,
				StepCfg: gui.NumericStepCfg{
					ShowButtons: false,
				},
				Width:  220,
				Sizing: gui.FixedFit,
				OnTextChanged: func(_ *gui.Layout, text string, w *gui.Window) {
					gui.State[ShowcaseApp](w).NumericPlainText = text
				},
				OnValueCommit: func(_ *gui.Layout, value gui.Opt[float64], text string, w *gui.Window) {
					app := gui.State[ShowcaseApp](w)
					app.NumericPlainValue = value
					app.NumericPlainText = text
				},
			}),
			gui.Text(gui.TextCfg{Text: "Committed: " + numericValueText(app.NumericPlainValue), TextStyle: bodyStyle}),
		},
	})
}

func demoColorPicker(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	c := app.ColorPickerColor
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Switch(gui.SwitchCfg{
				ID:       "color-picker-hsv",
				Label:    "Show HSV",
				Selected: app.ColorPickerHSV,
				OnClick: func(_ *gui.Layout, _ *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).ColorPickerHSV = !gui.State[ShowcaseApp](w).ColorPickerHSV
				},
			}),
			gui.ColorPicker(gui.ColorPickerCfg{
				ID:      "color-picker",
				Color:   c,
				ShowHSV: app.ColorPickerHSV,
				OnColorChange: func(color gui.Color, _ *gui.Event, w *gui.Window) {
					gui.State[ShowcaseApp](w).ColorPickerColor = color
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("RGBA(%d, %d, %d, %d)", c.R, c.G, c.B, c.A),
				TextStyle: gui.CurrentTheme().N3,
			}),
		},
	})
}

func demoDatePicker(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	selected := "none"
	if len(app.DatePickerDates) > 0 {
		parts := make([]string, 0, len(app.DatePickerDates))
		for _, date := range app.DatePickerDates {
			parts = append(parts, gui.LocaleFormatDate(date, gui.CurrentLocale().Date.ShortDate))
		}
		selected = strings.Join(parts, ", ")
	}

	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(10),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.Column(gui.ContainerCfg{
				Sizing:     gui.FitFit,
				SizeBorder: gui.NoBorder,
				Padding:    gui.NoPadding,
				Content: []gui.View{
					gui.DatePicker(gui.DatePickerCfg{
						ID:             "date-picker",
						Dates:          app.DatePickerDates,
						SelectMultiple: true,
						OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
							gui.State[ShowcaseApp](w).DatePickerDates = append([]time.Time(nil), dates...)
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      "Selected: " + selected,
				Mode:      gui.TextModeWrap,
				TextStyle: gui.CurrentTheme().N4}),
		},
	})
}

func demoDatePickerRoller(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(10),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.DatePickerRoller(gui.DatePickerRollerCfg{
				ID:           "date-roller",
				Focusable:    true,
				SelectedDate: app.RollerDate,
				OnChange: func(date time.Time, w *gui.Window) {
					gui.State[ShowcaseApp](w).RollerDate = date
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      "Selected: " + gui.LocaleFormatDate(app.RollerDate, gui.CurrentLocale().Date.LongDate),
				TextStyle: gui.CurrentTheme().N3,
			}),
		},
	})
}

func demoInputDate(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	return gui.Column(gui.ContainerCfg{
		Sizing:  gui.FitFit,
		Spacing: gui.SomeF(10),
		Padding: gui.NoPadding,
		Content: []gui.View{
			gui.InputDate(gui.InputDateCfg{
				ID:          "input-date",
				Date:        app.InputDate,
				Placeholder: "Pick a date",
				Sizing:      gui.FillFit,
				OnSelect: func(dates []time.Time, _ *gui.Event, w *gui.Window) {
					if len(dates) == 0 {
						return
					}
					gui.State[ShowcaseApp](w).InputDate = dates[0]
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      "Selected: " + gui.LocaleFormatDate(app.InputDate, gui.CurrentLocale().Date.ShortDate),
				TextStyle: gui.CurrentTheme().N3,
			}),
		},
	})
}

func demoForms(w *gui.Window) gui.View {
	app := gui.State[ShowcaseApp](w)
	form := &app.Form

	// Register fields each frame so the form runtime tracks them.
	gui.FormRegisterFieldByID(w, showcaseFormID, usernameAdapterCfg(form.Username))
	gui.FormRegisterFieldByID(w, showcaseFormID, emailAdapterCfg(form.Email))
	gui.FormRegisterFieldByID(w, showcaseFormID, ageAdapterCfg(form.AgeText))

	summary := w.FormSummary(showcaseFormID)
	pending := w.FormPendingState(showcaseFormID)
	pendingText := ""
	if len(pending.FieldIDs) > 0 {
		pendingText = "Validating: " + strings.Join(pending.FieldIDs, ", ")
	}

	return gui.Form(gui.FormCfg{
		ID:      showcaseFormID,
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(12),
		Padding: gui.NoPadding,
		OnSubmit: func(e gui.FormSubmitEvent, w *gui.Window) {
			app := gui.State[ShowcaseApp](w)
			app.Form.SubmitMessage = fmt.Sprintf(
				"Submitted username=%s, email=%s",
				strings.TrimSpace(e.Values["username"]),
				strings.TrimSpace(e.Values["email"]))
		},
		OnReset: func(e gui.FormResetEvent, w *gui.Window) {
			app := gui.State[ShowcaseApp](w)
			app.Form.Username = e.Values["username"]
			app.Form.Email = e.Values["email"]
			app.Form.AgeText = e.Values["age"]
			app.Form.AgeValue = gui.Opt[float64]{}
			app.Form.SubmitMessage = "Form reset"
		},
		Content: []gui.View{
			showcaseFormRow("Username", gui.Input(gui.InputCfg{
				ID:          "showcase-form-username",
				Width:       260,
				Sizing:      gui.FixedFit,
				Text:        form.Username,
				Placeholder: "username",
				OnTextChanged: func(l *gui.Layout, s string, w *gui.Window) {
					gui.State[ShowcaseApp](w).Form.Username = s
					gui.FormOnFieldEvent(w, l, usernameAdapterCfg(s), gui.FormTriggerChange)
				},
				OnBlur: func(l *gui.Layout, w *gui.Window) {
					app := gui.State[ShowcaseApp](w)
					gui.FormOnFieldEvent(w, l, usernameAdapterCfg(app.Form.Username), gui.FormTriggerBlur)
				},
			})),
			showcaseFormFieldState(w, "username"),
			showcaseFormFieldIssues(w, "username"),

			showcaseFormRow("Email", gui.Input(gui.InputCfg{
				ID:          "showcase-form-email",
				Width:       260,
				Sizing:      gui.FixedFit,
				Text:        form.Email,
				Placeholder: "user@example.com",
				OnTextChanged: func(l *gui.Layout, s string, w *gui.Window) {
					gui.State[ShowcaseApp](w).Form.Email = s
					gui.FormOnFieldEvent(w, l, emailAdapterCfg(s), gui.FormTriggerChange)
				},
				OnBlur: func(l *gui.Layout, w *gui.Window) {
					app := gui.State[ShowcaseApp](w)
					gui.FormOnFieldEvent(w, l, emailAdapterCfg(app.Form.Email), gui.FormTriggerBlur)
				},
			})),
			showcaseFormFieldState(w, "email"),
			showcaseFormFieldIssues(w, "email"),

			showcaseFormRow("Age", gui.NumericInput(gui.NumericInputCfg{
				ID:       "showcase-form-age",
				Width:    120,
				Sizing:   gui.FixedFit,
				Decimals: 0,
				Min:      gui.Some(0.0),
				Max:      gui.Some(120.0),
				Text:     form.AgeText,
				Value:    form.AgeValue,
				OnTextChanged: func(l *gui.Layout, text string, w *gui.Window) {
					gui.State[ShowcaseApp](w).Form.AgeText = text
					gui.FormOnFieldEvent(w, l, ageAdapterCfg(text), gui.FormTriggerChange)
				},
				OnValueCommit: func(l *gui.Layout, value gui.Opt[float64], text string, w *gui.Window) {
					app := gui.State[ShowcaseApp](w)
					app.Form.AgeValue = value
					app.Form.AgeText = text
					gui.FormOnFieldEvent(w, l, ageAdapterCfg(text), gui.FormTriggerBlur)
				},
			})),
			showcaseFormFieldState(w, "age"),
			showcaseFormFieldIssues(w, "age"),

			gui.Row(gui.ContainerCfg{
				Sizing:  gui.FillFit,
				Padding: gui.NoPadding,
				Spacing: gui.SomeF(8),
				Content: []gui.View{
					gui.Button(gui.ButtonCfg{
						ID:      "showcase-form-submit",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{gui.Text(gui.TextCfg{Text: gui.CurrentLocale().StrSubmit, TextStyle: gui.CurrentTheme().B3})},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							gui.FormRequestSubmit(w, showcaseFormID)
							e.IsHandled = true
						},
					}),
					gui.Button(gui.ButtonCfg{
						ID:      "showcase-form-reset",
						Padding: gui.SomeP(8, 16, 8, 16),
						Content: []gui.View{gui.Text(gui.TextCfg{Text: gui.CurrentLocale().StrReset, TextStyle: gui.CurrentTheme().N3})},
						OnClick: func(_ *gui.Layout, e *gui.Event, w *gui.Window) {
							gui.FormRequestReset(w, showcaseFormID)
							e.IsHandled = true
						},
					}),
				},
			}),
			gui.Text(gui.TextCfg{
				Text:      fmt.Sprintf("Validation summary: invalid=%d, pending=%d", summary.InvalidCount, summary.PendingCount),
				TextStyle: gui.CurrentTheme().N3,
			}),
			gui.Text(gui.TextCfg{Text: pendingText, TextStyle: gui.CurrentTheme().N3}),
			gui.Text(gui.TextCfg{
				Text: func() string {
					if form.SubmitMessage != "" {
						return form.SubmitMessage
					}
					return "Submit form to view committed values"
				}(),
				TextStyle: gui.CurrentTheme().N3,
			}),
		},
	})
}

func labeledRow(t gui.Theme, label string, content gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Spacing: gui.SomeF(8),
		Padding: gui.NoPadding,
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: t.B3,
				MinWidth:  80,
				Sizing:    gui.FixedFit,
			}),
			content,
		},
	})
}

func showcaseFormRow(label string, field gui.View) gui.View {
	return gui.Row(gui.ContainerCfg{
		Sizing:  gui.FillFit,
		Padding: gui.NoPadding,
		Spacing: gui.SomeF(8),
		VAlign:  gui.VAlignMiddle,
		Content: []gui.View{
			gui.Text(gui.TextCfg{
				Text:      label,
				TextStyle: gui.CurrentTheme().N3,
				MinWidth:  90,
				Sizing:    gui.FixedFit,
			}),
			field,
		},
	})
}

func showcaseFormFieldState(w *gui.Window, fieldID string) gui.View {
	fs, ok := w.FormFieldState(showcaseFormID, fieldID)
	text := fieldID + ": (not registered)"
	if ok {
		text = fmt.Sprintf("%s: touched=%t, dirty=%t, pending=%t",
			fieldID, fs.Touched, fs.Dirty, fs.Pending)
	}
	return gui.Text(gui.TextCfg{
		Text:      text,
		TextStyle: gui.CurrentTheme().N3,
	})
}

func showcaseFormFieldIssues(w *gui.Window, fieldID string) gui.View {
	issues := w.FormFieldErrors(showcaseFormID, fieldID)
	if len(issues) == 0 {
		return nil
	}
	msgs := make([]string, len(issues))
	for i, issue := range issues {
		msgs[i] = issue.Msg
	}
	style := gui.CurrentTheme().N3
	style.Color = gui.RGB(219, 87, 87)
	return gui.Text(gui.TextCfg{
		Text:      fieldID + ": " + strings.Join(msgs, "; "),
		TextStyle: style,
	})
}

func numericValueText(value gui.Opt[float64]) string {
	v, ok := value.Value()
	if !ok {
		return "none"
	}
	return fmt.Sprintf("%.2f", v)
}
