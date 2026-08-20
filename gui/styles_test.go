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
	want := []float32{2, 6, 14, 28}
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

// textSizes is the mechanism every theme's type ladder is derived from
// (visual-refresh §2.1): the body size is the per-theme decision and
// the rungs are the fixed offsets. Pin the spec's table so the
// derivation and the documented values cannot drift.
func TestTextSizesTable(t *testing.T) {
	cases := []struct {
		body float32
		want textSizeLadder
	}{
		{14, textSizeLadder{10, 11, 12, 14, 17, 22}}, // dark, light
		{13, textSizeLadder{9, 10, 11, 13, 16, 21}},  // macos(-dark)
		{12, textSizeLadder{8, 9, 10, 12, 15, 20}},   // windows(-dark)
		{15, textSizeLadder{11, 12, 13, 15, 18, 23}}, // gnome(-dark)
	}
	for _, c := range cases {
		if got := textSizes(c.body); got != c.want {
			t.Errorf("textSizes(%v) = %v, want %v", c.body, got, c.want)
		}
	}
}
