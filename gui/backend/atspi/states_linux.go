//go:build linux

package atspi

import "github.com/go-gui-org/go-gui/gui"

// AT-SPI2 state bit indices (within a [2]uint32 bitfield). Each value is the
// position of that member in AtspiStateType (at-spi2-core
// atspi/atspi-constants.h). Clients such as Orca read the State property by
// these positions, so a value must never be renumbered or guessed.
// states_linux_test.go pins them against the header's numbers.
const (
	stateBusy         = 3  // ATSPI_STATE_BUSY
	stateChecked      = 4  // ATSPI_STATE_CHECKED
	stateEnabled      = 8  // ATSPI_STATE_ENABLED
	stateExpanded     = 10 // ATSPI_STATE_EXPANDED
	stateFocusable    = 11 // ATSPI_STATE_FOCUSABLE
	stateFocused      = 12 // ATSPI_STATE_FOCUSED
	stateModal        = 16 // ATSPI_STATE_MODAL
	stateSelected     = 23 // ATSPI_STATE_SELECTED
	stateSensitive    = 24 // ATSPI_STATE_SENSITIVE
	stateShowing      = 25 // ATSPI_STATE_SHOWING
	stateVisible      = 30 // ATSPI_STATE_VISIBLE
	stateRequired     = 33 // ATSPI_STATE_REQUIRED
	stateInvalidEntry = 36 // ATSPI_STATE_INVALID_ENTRY
	// READ_ONLY was added to the enum after IS_DEFAULT (39), VISITED (40),
	// CHECKABLE (41) and HAS_POPUP (42). Older headers do not have it; a client
	// built against one ignores the bit, which is the same as leaving it unset.
	stateReadOnly = 43 // ATSPI_STATE_READ_ONLY
)

// atspiState converts AccessState + focused flag into AT-SPI2
// [2]uint32 bitfield.
func atspiState(s gui.AccessState, focused bool) [2]uint32 {
	var bits [2]uint32

	// Base: visible, showing, sensitive, enabled.
	setBit(&bits, stateVisible)
	setBit(&bits, stateShowing)
	if !s.Has(gui.AccessStateDisabled) {
		setBit(&bits, stateSensitive)
		setBit(&bits, stateEnabled)
	}
	if focused {
		setBit(&bits, stateFocusable)
		setBit(&bits, stateFocused)
	}
	if s.Has(gui.AccessStateChecked) {
		setBit(&bits, stateChecked)
	}
	if s.Has(gui.AccessStateExpanded) {
		setBit(&bits, stateExpanded)
	}
	if s.Has(gui.AccessStateSelected) {
		setBit(&bits, stateSelected)
	}
	if s.Has(gui.AccessStateReadOnly) {
		setBit(&bits, stateReadOnly)
	}
	if s.Has(gui.AccessStateRequired) {
		setBit(&bits, stateRequired)
	}
	if s.Has(gui.AccessStateModal) {
		setBit(&bits, stateModal)
	}
	if s.Has(gui.AccessStateBusy) {
		setBit(&bits, stateBusy)
	}
	if s.Has(gui.AccessStateInvalid) {
		setBit(&bits, stateInvalidEntry)
	}
	return bits
}

func setBit(bits *[2]uint32, bit int) {
	bits[bit/32] |= 1 << (bit % 32)
}
