//go:build linux

package atspi

import "github.com/go-gui-org/go-gui/gui"

// AT-SPI2 state bit indices (within a [2]uint32 bitfield).
const (
	stateEnabled      = 7
	stateFocusable    = 8
	stateFocused      = 6
	stateSensitive    = 17
	stateShowing      = 14
	stateVisible      = 20
	stateChecked      = 5
	stateExpanded     = 9
	stateSelected     = 22
	stateReadOnly     = 39
	stateRequired     = 40
	stateModal        = 32
	stateBusy         = 4
	stateInvalidEntry = 36
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
