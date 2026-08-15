package gui

import "testing"

// TestA11YDescriptionForwarded guards issue #316: every widget whose
// root shape carries the a11y node must forward A11YDescription from
// its Cfg into Shape.a11Y. Before the fix these ten widgets accepted
// the field and silently discarded it — the label travelled, the
// description never reached the accessibility tree.
//
// The widget list is the root-container set: the widgets that delegate
// to a ContainerCfg carrying their a11y info. Widgets whose a11y node
// lands on an inner shape (Input, Radio, ...) or is built directly
// (Skeleton, ProgressBar) forward the pair elsewhere and are covered by
// their own tests plus the ergonomics-audit a11y mode.
func TestA11YDescriptionForwarded(t *testing.T) {
	tests := []struct {
		name string
		view func(desc string) View
	}{
		{"button", func(d string) View {
			return Button(ButtonCfg{ID: "b", A11YCfg: A11YCfg{A11YDescription: d}})
		}},
		{"toggle", func(d string) View {
			return Toggle(ToggleCfg{ID: "t", A11YCfg: A11YCfg{A11YDescription: d}})
		}},
		{"color picker", func(d string) View {
			return ColorPicker(ColorPickerCfg{ID: "cp", A11YCfg: A11YCfg{A11YDescription: d}})
		}},
		{"combobox", func(d string) View {
			return Combobox(ComboboxCfg{ID: "cb", A11YCfg: A11YCfg{A11YDescription: d}})
		}},
		{"date picker", func(d string) View {
			return DatePicker(DatePickerCfg{ID: "dp", A11YCfg: A11YCfg{A11YDescription: d}})
		}},
		{"date picker roller", func(d string) View {
			return DatePickerRoller(DatePickerRollerCfg{
				ID: "dpr", A11YCfg: A11YCfg{A11YDescription: d},
			})
		}},
		{"input date", func(d string) View {
			return InputDate(InputDateCfg{ID: "id", A11YCfg: A11YCfg{A11YDescription: d}})
		}},
		{"numeric input", func(d string) View {
			return NumericInput(NumericInputCfg{
				ID: "ni", A11YCfg: A11YCfg{A11YDescription: d},
			})
		}},
		// The stepper path is a second forwarding site: NumericInput
		// with buttons wraps the field in a Row carrying its own a11y
		// info, while the default path forwards into the inner Input.
		{"numeric input steppers", func(d string) View {
			return NumericInput(NumericInputCfg{
				ID:      "nis",
				StepCfg: NumericStepCfg{ShowButtons: true},
				A11YCfg: A11YCfg{A11YDescription: d},
			})
		}},
		{"listbox", func(d string) View {
			return ListBox(ListBoxCfg{ID: "lb", A11YCfg: A11YCfg{A11YDescription: d}})
		}},
		// The non-virtualized factory path is a second forwarding
		// site: a focus-disabled ListBox builds its items inline
		// instead of delegating to listBoxView.
		{"listbox non-virtualized", func(d string) View {
			return ListBox(ListBoxCfg{
				ID: "lbn", FocusDisabled: true,
				A11YCfg: A11YCfg{A11YDescription: d},
			})
		}},
		{"select", func(d string) View {
			return Select(SelectCfg{ID: "se", A11YCfg: A11YCfg{A11YDescription: d}})
		}},
		{"table", func(d string) View {
			return Table(TableCfg{
				ID:      "tb",
				Data:    []TableRowCfg{{Cells: []TableCellCfg{{Value: "x"}}}},
				A11YCfg: A11YCfg{A11YDescription: d},
			})
		}},
		{"theme picker", func(d string) View {
			return ThemePicker(ThemePickerCfg{
				ID: "tp", A11YCfg: A11YCfg{A11YDescription: d},
			})
		}},
		// Tree carries its a11y on the root via the explicit a11Y
		// field (cfg.a11yInfo) rather than a forwarding literal; the
		// description must travel that path too.
		{"tree", func(d string) View {
			return Tree(TreeCfg{ID: "tr", A11YCfg: A11YCfg{A11YDescription: d}})
		}},
	}

	const desc = "known description"
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := NewTestWindow(WindowCfg{})
			layout := generateViewLayout(tc.view(desc), w)
			info := layout.Shape.a11Y
			if info == nil {
				t.Fatal("root shape has no a11y node")
			}
			if info.Description != desc {
				t.Errorf("Shape.a11Y.Description = %q, want %q",
					info.Description, desc)
			}
		})
	}
}
