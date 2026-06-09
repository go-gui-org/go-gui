package gui

import "testing"

// TestRequireIDPanics verifies every stateful widget factory panics
// when invoked with an empty Cfg.ID, closing the state-collision
// bug window.
func TestRequireIDPanics(t *testing.T) {
	emptyID := ""
	w := &Window{}
	cases := []struct {
		name string
		call func()
	}{
		{"ColorPicker", func() { ColorPicker(ColorPickerCfg{ID: emptyID}) }},
		{"Combobox", func() { Combobox(ComboboxCfg{ID: emptyID}) }},
		{"CommandPalette", func() { CommandPalette(CommandPaletteCfg{ID: emptyID}) }},
		{"ContextMenu", func() { ContextMenu(w, ContextMenuCfg{ID: emptyID}) }},
		{"DatePicker", func() { DatePicker(DatePickerCfg{ID: emptyID}) }},
		{"Form", func() { Form(FormCfg{ID: emptyID}) }},
		{"ListBox", func() { ListBox(ListBoxCfg{ID: emptyID}) }},
		{"ProgressBar", func() { ProgressBar(ProgressBarCfg{ID: emptyID}) }},
		{"Slider", func() { Slider(SliderCfg{ID: emptyID}) }},
		{"Table", func() { Table(TableCfg{ID: emptyID}) }},
		{"Tree", func() { Tree(TreeCfg{ID: emptyID}) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("%s did not panic on empty ID", c.name)
				}
			}()
			c.call()
		})
	}
}
