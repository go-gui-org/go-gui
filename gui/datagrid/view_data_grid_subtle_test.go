package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// The selected row paints the subtle wash by default, and a caller-set
// ColorRowSelected is an explicit override that wins over the wash.
// Pins the resolution order inside applyDataGridDefaults — the wash
// once became dead code when the plain color was resolved from the
// theme first (visual-refresh §4.3).
func TestDataGridRowSubtleDefaults(t *testing.T) {
	cfg := &DataGridCfg{ID: "g"}
	applyDataGridDefaults(cfg)
	if !cfg.ColorRowSelectedSubtle.IsSet() ||
		cfg.ColorRowSelectedSubtle == cfg.ColorRowSelected {
		t.Errorf("unset subtle = %v, want the theme wash, not the full select %v",
			cfg.ColorRowSelectedSubtle, cfg.ColorRowSelected)
	}

	cfg = &DataGridCfg{ID: "g", ColorRowSelected: gg.Blue}
	applyDataGridDefaults(cfg)
	if cfg.ColorRowSelectedSubtle != gg.Blue {
		t.Errorf("caller subtle = %v, want the caller select %v",
			cfg.ColorRowSelectedSubtle, gg.Blue)
	}
}
