//go:build linux && !js && !android

package gl

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// An unset maximum axis must not pin the window shut. X11 has no
// "no maximum" sentinel, so the substitute has to be a large extent.
func TestMaxHintOrSubstitutesLargeExtent(t *testing.T) {
	t.Parallel()
	if got := maxHintOr(800); got != 800 {
		t.Errorf("maxHintOr(800) = %d, want 800", got)
	}
	unset := maxHintOr(0)
	if unset < 1<<14 {
		t.Errorf("maxHintOr(0) = %d, want a large extent", unset)
	}
	if maxHintOr(-5) != unset {
		t.Error("a negative maximum should read as unset")
	}
}

// Window managers read WM_NORMAL_HINTS by fixed offset, so the flag
// bits and the word positions are the contract, not an implementation
// detail. A wrong offset here silently produces a window nobody can
// resize.
func TestSizeHintWordsForLayout(t *testing.T) {
	t.Parallel()

	// Both bounds set: flags carry PMinSize|PMaxSize and every one of
	// the four values lands in its own word.
	both := sizeHintWordsFor(gui.SizeLimits{
		MinW: 400, MinH: 300, MaxW: 800, MaxH: 600,
	})
	if want := uint32(sizeHintPMinSize | sizeHintPMaxSize); both[0] != want {
		t.Errorf("flags = %#x, want %#x", both[0], want)
	}
	for i, want := range map[int]uint32{5: 400, 6: 300, 7: 800, 8: 600} {
		if both[i] != want {
			t.Errorf("word %d = %d, want %d", i, both[i], want)
		}
	}

	// Only a floor: the maximum words stay zero and PMaxSize is unset,
	// so the window manager imposes no ceiling of its own.
	minOnly := sizeHintWordsFor(gui.SizeLimits{MinW: 400, MinH: 300})
	if minOnly[0]&sizeHintPMaxSize != 0 {
		t.Error("PMaxSize set with no maximum configured")
	}
	if minOnly[7] != 0 || minOnly[8] != 0 {
		t.Errorf("max words = %d,%d, want zero", minOnly[7], minOnly[8])
	}

	// One ceiling axis only: the other axis must be permissive, never
	// the zero that would pin the window shut.
	oneAxis := sizeHintWordsFor(gui.SizeLimits{MaxW: 800})
	if oneAxis[8] < 1<<14 {
		t.Errorf("unset max height = %d, want a large extent", oneAxis[8])
	}
}
