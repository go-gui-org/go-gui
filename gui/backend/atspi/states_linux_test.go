//go:build linux

package atspi

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Bit positions copied from AtspiStateType in at-spi2-core atspi/atspi-constants.h.
// They are written as numbers on purpose: comparing against the package's own
// constants would pass no matter what those constants hold. Orca reads the
// State property by these positions, so a wrong one makes it announce the
// wrong state.
const (
	wantBusy         = 3
	wantChecked      = 4
	wantEditable     = 7
	wantEnabled      = 8
	wantExpanded     = 10
	wantFocusable    = 11
	wantFocused      = 12
	wantModal        = 16
	wantSelected     = 23
	wantSensitive    = 24
	wantShowing      = 25
	wantVisible      = 30
	wantRequired     = 33
	wantInvalidEntry = 36
	wantReadOnly     = 43
)

// bitsOf builds the expected [2]uint32 bitfield from a list of positions.
func bitsOf(positions ...int) [2]uint32 {
	var bits [2]uint32
	for _, p := range positions {
		bits[p/32] |= 1 << (p % 32)
	}
	return bits
}

func TestAtspiStateMatchesCanonicalEnum(t *testing.T) {
	// Every enabled, non-disabled node carries these four.
	base := []int{wantVisible, wantShowing, wantSensitive, wantEnabled}
	with := func(extra ...int) [2]uint32 {
		return bitsOf(append(append([]int{}, base...), extra...)...)
	}

	tests := []struct {
		name    string
		state   gui.AccessState
		focused bool
		want    [2]uint32
	}{
		{"plain", 0, false, with()},
		{"disabled", gui.AccessStateDisabled, false, bitsOf(wantVisible, wantShowing)},
		{"focused", 0, true, with(wantFocusable, wantFocused)},
		{"checked", gui.AccessStateChecked, false, with(wantChecked)},
		{"expanded", gui.AccessStateExpanded, false, with(wantExpanded)},
		{"selected", gui.AccessStateSelected, false, with(wantSelected)},
		{"read only", gui.AccessStateReadOnly, false, with(wantReadOnly)},
		{"required", gui.AccessStateRequired, false, with(wantRequired)},
		{"modal", gui.AccessStateModal, false, with(wantModal)},
		{"busy", gui.AccessStateBusy, false, with(wantBusy)},
		{"invalid", gui.AccessStateInvalid, false, with(wantInvalidEntry)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := atspiState(tc.state, tc.focused)
			if got != tc.want {
				t.Errorf("atspiState: got %032b %032b, want %032b %032b",
					got[1], got[0], tc.want[1], tc.want[0])
			}
		})
	}
}

// A read-only node must not claim EDITABLE; the two contradict each other.
func TestAtspiStateReadOnlyNotEditable(t *testing.T) {
	got := atspiState(gui.AccessStateReadOnly, false)
	if got[wantEditable/32]&(1<<(wantEditable%32)) != 0 {
		t.Errorf("read-only node has EDITABLE (bit %d) set", wantEditable)
	}
}
