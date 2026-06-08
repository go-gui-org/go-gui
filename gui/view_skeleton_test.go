package gui

import "testing"

func TestSkeletonDefaultLayout(t *testing.T) {
	v := Skeleton(SkeletonCfg{ID: "s1"})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.Axis != AxisLeftToRight {
		t.Error("default should be horizontal (row)")
	}
}

func TestSkeletonCircleVariant(t *testing.T) {
	v := Skeleton(SkeletonCfg{
		ID:      "s2",
		Variant: SkeletonCircle,
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.shapeType != shapeCircle {
		t.Errorf("shapeType = %d, want shapeCircle (%d)",
			layout.Shape.shapeType, shapeCircle)
	}
}

func TestSkeletonA11YRole(t *testing.T) {
	v := Skeleton(SkeletonCfg{ID: "s3"})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.A11YRole != AccessRoleProgressBar {
		t.Errorf("role = %d, want ProgressBar",
			layout.Shape.A11YRole)
	}
}

func TestSkeletonA11YState(t *testing.T) {
	v := Skeleton(SkeletonCfg{ID: "s4"})
	layout := generateViewLayout(v, &Window{})
	want := AccessStateBusy | AccessStateLive
	if layout.Shape.A11YState != want {
		t.Errorf("state = %d, want %d",
			layout.Shape.A11YState, want)
	}
}

func TestSkeletonA11YLabel(t *testing.T) {
	v := Skeleton(SkeletonCfg{
		ID:        "s5",
		A11YLabel: "avatar",
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.A11Y == nil {
		t.Fatal("a11y nil")
	}
	if layout.Shape.A11Y.Label != "avatar" {
		t.Errorf("label = %q, want %q",
			layout.Shape.A11Y.Label, "avatar")
	}
}

func TestSkeletonA11YLabelDefault(t *testing.T) {
	v := Skeleton(SkeletonCfg{ID: "s6"})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.A11Y == nil {
		t.Fatal("a11y nil")
	}
	if layout.Shape.A11Y.Label != "Loading" {
		t.Errorf("label = %q, want %q",
			layout.Shape.A11Y.Label, "Loading")
	}
}

func TestSkeletonThemeColor(t *testing.T) {
	v := Skeleton(SkeletonCfg{ID: "s7"})
	layout := generateViewLayout(v, &Window{})
	want := guiTheme.SkeletonStyle.Color
	if layout.Shape.Color != want {
		t.Errorf("color = %v, want %v",
			layout.Shape.Color, want)
	}
}

func TestSkeletonCustomColor(t *testing.T) {
	c := Color{R: 200, G: 50, B: 50, A: 255, set: true}
	v := Skeleton(SkeletonCfg{ID: "s8", Color: c})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.Color != c {
		t.Errorf("color = %v, want %v",
			layout.Shape.Color, c)
	}
}

func TestSkeletonRadiusZeroOverride(t *testing.T) {
	v := Skeleton(SkeletonCfg{
		ID:     "s9",
		Radius: NoRadius,
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.Radius != 0 {
		t.Errorf("radius = %f, want 0", layout.Shape.Radius)
	}
}

func TestSkeletonSizeBorderNone(t *testing.T) {
	v := Skeleton(SkeletonCfg{ID: "s10"})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.SizeBorder != 0 {
		t.Errorf("SizeBorder = %f, want 0",
			layout.Shape.SizeBorder)
	}
}

func TestSkeletonInvisible(t *testing.T) {
	v := Skeleton(SkeletonCfg{
		ID:        "s11",
		Invisible: true,
	})
	layout := generateViewLayout(v, &Window{})
	// Invisible containers return a disabled, zero-size view.
	if !layout.Shape.Disabled {
		t.Error("invisible skeleton should be disabled")
	}
}

func TestSkeletonDisabled(t *testing.T) {
	v := Skeleton(SkeletonCfg{
		ID:       "s12",
		Disabled: true,
	})
	layout := generateViewLayout(v, &Window{})
	if !layout.Shape.Disabled {
		t.Error("Disabled not propagated")
	}
}

func TestSkeletonAnimationIsViewBound(t *testing.T) {
	v := Skeleton(SkeletonCfg{ID: "sk1"})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil || layout.Shape.events.AmendLayout == nil {
		t.Fatal("AmendLayout not set")
	}
	layout.Shape.events.AmendLayout(&layout, w)
	if w.animViewBound == nil {
		t.Fatal("animViewBound nil after skeleton AmendLayout — animation not view-bound")
	}
	if _, ok := w.animViewBound["skeleton_sk1"]; !ok {
		t.Error("skeleton animation not registered as view-bound")
	}
}

func TestSkeletonAmendLayoutSetsGradient(t *testing.T) {
	v := Skeleton(SkeletonCfg{ID: "s13"})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil ||
		layout.Shape.events.AmendLayout == nil {
		t.Fatal("AmendLayout not set")
	}
	layout.Shape.events.AmendLayout(&layout, w)
	if layout.Shape.fx == nil {
		t.Fatal("fx nil after AmendLayout")
	}
	if layout.Shape.fx.Gradient == nil {
		t.Fatal("Gradient nil after AmendLayout")
	}
	if len(layout.Shape.fx.Gradient.Stops) != 5 {
		t.Errorf("gradient stops = %d, want 5",
			len(layout.Shape.fx.Gradient.Stops))
	}
	if layout.Shape.fx.Gradient.Direction != GradientToRight {
		t.Error("gradient direction should be ToRight")
	}
}
