package datagrid

// StateMap namespace constants for data grid internal state.
//
// This block owns the "gui.dg.*" namespace strings: gui kept a
// mirror that drifted stale (it missed nsDgQuickDraft), so the
// mirror was deleted and this is the single source. A pin test in
// this package fails on a rename, so stored state is never renamed
// by accident.
const (
	nsDgColWidths    = "gui.dg.col_widths"
	nsDgPresentation = "gui.dg.presentation"
	nsDgResize       = "gui.dg.resize"
	nsDgHeaderHover  = "gui.dg.header_hover"
	nsDgRange        = "gui.dg.range"
	nsDgChooserOpen  = "gui.dg.chooser_open"
	nsDgEdit         = "gui.dg.edit"
	nsDgCrud         = "gui.dg.crud"
	nsDgJump         = "gui.dg.jump"
	nsDgPendingJump  = "gui.dg.pending_jump"
	nsDgQuickDraft   = "gui.dg.quick_draft"
	nsDgQuickPending = "gui.dg.quick_pending"
	nsDgSource       = "gui.dg.source"

	// capModerate mirrors gui's capacity tier of the same name.
	// Local sizing for grid maps; safe to tune here.
	capModerate = 50
)

// Input and batch bounds. The ORM path validates these; the
// local paths below mirror them so a paste cannot wedge the UI.
const (
	// dataGridMaxQuickFilterLen caps committed quick-filter text.
	dataGridMaxQuickFilterLen = 500
	// dataGridMaxLocalFilterCount caps filters honored per query.
	dataGridMaxLocalFilterCount = 100
	// dataGridMaxCellValueLen caps a single edited cell value.
	dataGridMaxCellValueLen = 1 << 15
	// dataGridMaxJumpDigits caps stored jump-box digits.
	dataGridMaxJumpDigits = 10
	// dataGridMaxMutationBatch caps one save's creates, updates
	// and deletes each; beyond it the save fails fast instead of
	// fanning out an unbounded mutation storm.
	dataGridMaxMutationBatch = 100000
	// dataGridMaxCSVRows caps imported CSV rows.
	dataGridMaxCSVRows = 100000
	// dataGridMaxCSVBytes caps imported CSV input size.
	dataGridMaxCSVBytes = 10 << 20
)

// dataGridTruncateRunes cuts s to n runes, rune-aware so a cut
// never splits a multi-byte sequence. Short strings return as-is.
// Allocation-free: it scans rune boundaries and subslices.
func dataGridTruncateRunes(s string, n int) string {
	if len(s) <= n || n < 0 {
		return s
	}
	count := 0
	for idx := range s {
		if count == n {
			return s[:idx]
		}
		count++
	}
	return s
}

func f32Max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func f32Clamp(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
