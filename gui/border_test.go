package gui

import "testing"

func TestBorderZeroIsUnset(t *testing.T) {
	var b Border
	if b.IsSet() {
		t.Error("zero Border must be unset")
	}
	if got := b.Or(5); got != 5 {
		t.Errorf("unset Or(5) = %v, want 5", got)
	}
}

func TestNoBorderIsSetZero(t *testing.T) {
	if !NoBorder.IsSet() {
		t.Error("NoBorder must be set")
	}
	if got := NoBorder.Or(5); got != 0 {
		t.Errorf("NoBorder.Or(5) = %v, want 0", got)
	}
}

// BorderPx is fixed: a theme with another width must not move it.
func TestBorderPxIgnoresTheme(t *testing.T) {
	withRadiusTheme(t)
	if got := BorderPx(3).Or(0); got != 3 {
		t.Errorf("BorderPx(3).Or = %v, want 3", got)
	}
}

// BorderThin reads the active theme's width (issue #867).
func TestBorderThinFollowsTheme(t *testing.T) {
	th := withRadiusTheme(t)
	if got := BorderThin.Or(0); got != th.SizeBorder {
		t.Errorf("BorderThin.Or = %v, want theme SizeBorder %v", got, th.SizeBorder)
	}
}

// The end-to-end regression for issue #867: a BorderThin container
// draws no border under WithBorders(false). With SomeF(1) it kept
// drawing one.
func TestContainerBorderThinFollowsWithBorders(t *testing.T) {
	prior := CurrentTheme()
	t.Cleanup(func() { SetTheme(prior) })
	SetTheme(prior.WithBorders(false))
	w := &Window{}
	shape := Column(ContainerCfg{SizeBorder: BorderThin}).GenerateLayout(w).Shape
	if shape.SizeBorder != 0 {
		t.Errorf("Column SizeBorder = %v, want 0 under WithBorders(false)",
			shape.SizeBorder)
	}
}

func TestBorderOrNoAlloc(t *testing.T) {
	allocs := testing.AllocsPerRun(100, func() {
		_ = BorderThin.Or(0) + BorderPx(2).Or(0)
	})
	if allocs != 0 {
		t.Errorf("allocs = %v, want 0", allocs)
	}
}
