package gui

import "testing"

// The spacing ladder is public vocabulary (issue #344, audit §4): each
// tier names a gap, and the rungs differ by how closely related the
// things are. Pin both the values and the strict ordering so a tier
// cannot silently drift past its neighbours.
func TestSpacingTierValues(t *testing.T) {
	tiers := []struct {
		name  string
		value float32
	}{
		{"SpacingTight", SpacingTight},
		{"SpacingSmall", SpacingSmall},
		{"SpacingMedium", SpacingMedium},
		{"SpacingLarge", SpacingLarge},
	}
	want := []float32{2, 5, 10, 15}
	for i, tier := range tiers {
		if tier.value != want[i] {
			t.Errorf("%s = %v, want %v", tier.name, tier.value, want[i])
		}
		if i > 0 && tier.value <= tiers[i-1].value {
			t.Errorf("%s (%v) is not above %s (%v)",
				tier.name, tier.value, tiers[i-1].name, tiers[i-1].value)
		}
	}
}
