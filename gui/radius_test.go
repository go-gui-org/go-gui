package gui

import "testing"

// withRadiusTheme installs a theme whose radius ladder and border
// width are off the defaults, so a result that still equals a default
// seed proves the theme was bypassed. It restores the prior theme on
// cleanup.
func withRadiusTheme(t *testing.T) Theme {
	t.Helper()
	prior := CurrentTheme()
	t.Cleanup(func() { SetTheme(prior) })
	cfg := baseDarkCfg()
	cfg.RadiusSmall = 3
	cfg.RadiusMedium = 9
	cfg.RadiusLarge = 17
	cfg.SizeBorder = 2.5
	th := ThemeMaker(cfg)
	SetTheme(th)
	return th
}

func TestRadiusZeroIsUnset(t *testing.T) {
	var r Radius
	if r.IsSet() {
		t.Error("zero Radius must be unset")
	}
	if got := r.Or(5); got != 5 {
		t.Errorf("unset Or(5) = %v, want 5", got)
	}
}

func TestNoRadiusIsSetZero(t *testing.T) {
	if !NoRadius.IsSet() {
		t.Error("NoRadius must be set")
	}
	if got := NoRadius.Or(5); got != 0 {
		t.Errorf("NoRadius.Or(5) = %v, want 0", got)
	}
}

// RadiusPx is fixed: a theme with another ladder must not move it.
func TestRadiusPxIgnoresTheme(t *testing.T) {
	withRadiusTheme(t)
	if got := RadiusPx(7).Or(0); got != 7 {
		t.Errorf("RadiusPx(7).Or = %v, want 7", got)
	}
}

// Each role reads its step from the active theme, not the seed
// (issue #867).
func TestRadiusRolesFollowTheme(t *testing.T) {
	th := withRadiusTheme(t)
	cases := []struct {
		name string
		r    Radius
		want float32
	}{
		{"Small", RadiusSmall, th.RadiusSmall},
		{"Medium", RadiusMedium, th.RadiusMedium},
		{"Large", RadiusLarge, th.RadiusLarge},
	}
	for _, c := range cases {
		if got := c.r.Or(0); got != c.want {
			t.Errorf("Radius%s.Or = %v, want %v (theme step)", c.name, got, c.want)
		}
	}
}

// The end-to-end regression for issue #867: a Column that names a
// radius role gets the custom theme's radius in its Shape. With a
// literal it got 4 whatever the theme said.
func TestContainerRadiusRoleFollowsTheme(t *testing.T) {
	th := withRadiusTheme(t)
	w := &Window{}
	shape := Column(ContainerCfg{Radius: RadiusSmall}).GenerateLayout(w).Shape
	if shape.Radius != th.RadiusSmall {
		t.Errorf("Column Radius = %v, want theme RadiusSmall %v",
			shape.Radius, th.RadiusSmall)
	}
}

// Building a Radius and reading it must not allocate: Cfg literals
// are rebuilt every frame.
func TestRadiusOrNoAlloc(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_ = RadiusMedium.Or(0) + RadiusPx(3).Or(0)
	})
	if allocs != 0 {
		t.Errorf("allocs = %v, want 0", allocs)
	}
}

// A role in a theme patch resolves against the patched theme's own
// ladder, not the installed theme, and moves with it on a rebuild.
func TestThemePatchRadiusRoleUsesPatchedLadder(t *testing.T) {
	cfg := baseDarkCfg()
	cfg.RadiusLarge = 17
	cfg.SizeBorder = 2.5
	patched := ThemeMaker(cfg).With(ButtonPatch{Radius: RadiusLarge, SizeBorder: BorderThin})
	if got := patched.buttonStyle.Radius; got != 17 {
		t.Errorf("button radius = %v, want patched theme RadiusLarge 17", got)
	}
	if got := patched.buttonStyle.SizeBorder; got != 2.5 {
		t.Errorf("button border = %v, want patched theme SizeBorder 2.5", got)
	}
	if got := patched.WithBorders(false).buttonStyle.SizeBorder; got != 0 {
		t.Errorf("WithBorders(false) button border = %v, want 0", got)
	}
}
