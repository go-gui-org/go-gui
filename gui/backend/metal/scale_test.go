//go:build darwin && !ios

package metal

import "testing"

func TestDrawableScaleWith(t *testing.T) {
	// 1.25 at 801pt makes a 1001px drawable (1001.25 truncated). The
	// ratio would be 1.2497; the override must come back as it is (#971).
	if got := drawableScaleWith(1.25, true, 1001, 801, 1); got != 1.25 {
		t.Errorf("override = %v, want 1.25", got)
	}
	if got := drawableScaleWith(0, false, 1600, 800, 1); got != 2 {
		t.Errorf("backing ratio = %v, want 2", got)
	}
	if got := drawableScaleWith(0, false, 1600, 0, 1.5); got != 1.5 {
		t.Errorf("zero logical width = %v, want fallback 1.5", got)
	}
}
