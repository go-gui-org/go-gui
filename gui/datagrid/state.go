package datagrid

// StateMap namespace constants for data grid internal state.
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
	nsDgSource       = "gui.dg.source"

	capModerate = 50

	nsScrollY = "gui.dg.scroll.y"
	nsScrollX = "gui.dg.scroll.x"
	capScroll = 200
)

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
