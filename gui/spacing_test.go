package gui

import "testing"

// withSpacingTheme installs a theme whose four gap steps are off the
// default ladder, so a result that still equals a default seed proves
// the theme was bypassed. It restores the prior theme on cleanup.
func withSpacingTheme(t *testing.T) Theme {
	t.Helper()
	prior := CurrentTheme()
	t.Cleanup(func() { SetTheme(prior) })
	cfg := baseDarkCfg()
	cfg.SpacingTight = 3
	cfg.SpacingSmall = 9
	cfg.SpacingMedium = 17
	cfg.SpacingLarge = 41
	th := ThemeMaker(cfg)
	SetTheme(th)
	return th
}

func TestSpacingZeroIsUnset(t *testing.T) {
	var s Spacing
	if s.IsSet() {
		t.Error("zero Spacing must be unset")
	}
	if got := s.Or(5); got != 5 {
		t.Errorf("unset Or(5) = %v, want 5", got)
	}
}

func TestNoSpacingIsSetZero(t *testing.T) {
	if !NoSpacing.IsSet() {
		t.Error("NoSpacing must be set")
	}
	if got := NoSpacing.Or(5); got != 0 {
		t.Errorf("NoSpacing.Or(5) = %v, want 0", got)
	}
}

// SpacingPx is fixed: a theme with other steps must not move it.
func TestSpacingPxIgnoresTheme(t *testing.T) {
	withSpacingTheme(t)
	if got := SpacingPx(7).Or(0); got != 7 {
		t.Errorf("SpacingPx(7).Or = %v, want 7", got)
	}
}

// Each role reads its step from the active theme, not the seed
// (issue #866).
func TestSpacingRolesFollowTheme(t *testing.T) {
	th := withSpacingTheme(t)
	cases := []struct {
		name string
		s    Spacing
		want float32
	}{
		{"Tight", SpacingTight, th.SpacingTight},
		{"Small", SpacingSmall, th.SpacingSmall},
		{"Medium", SpacingMedium, th.SpacingMedium},
		{"Large", SpacingLarge, th.SpacingLarge},
	}
	for _, c := range cases {
		if got := c.s.Or(0); got != c.want {
			t.Errorf("Spacing%s.Or = %v, want %v (theme step)", c.name, got, c.want)
		}
	}
}

// The end-to-end regression for issue #866: a Column that names a
// role gets the custom theme's gap in its Shape. With the old float
// constant it got 28 whatever the theme said.
func TestContainerSpacingRoleFollowsTheme(t *testing.T) {
	th := withSpacingTheme(t)
	w := &Window{}
	shape := Column(ContainerCfg{Spacing: SpacingLarge}).GenerateLayout(w).Shape
	if shape.Spacing != th.SpacingLarge {
		t.Errorf("Column Spacing = %v, want theme SpacingLarge %v",
			shape.Spacing, th.SpacingLarge)
	}
}

// Building a Spacing and reading it must not allocate: Cfg literals
// are rebuilt every frame.
func TestSpacingOrNoAlloc(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_ = SpacingMedium.Or(0) + SpacingPx(3).Or(0)
	})
	if allocs != 0 {
		t.Errorf("allocs = %v, want 0", allocs)
	}
}
