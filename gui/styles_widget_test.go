package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestDefaultInputStyleColors(t *testing.T) {
	s := ThemeDark.InputStyle
	if s.Color.eq(Color{}) {
		t.Error("input color should not be zero")
	}
	if s.Radius != radiusMedium {
		t.Errorf("radius = %f, want %f", s.Radius, radiusMedium)
	}
}

func TestDefaultScrollbarStyle(t *testing.T) {
	s := ThemeDark.ScrollbarStyle
	if s.Size != 7 {
		t.Errorf("scrollbar size = %f", s.Size)
	}
	if s.minThumbSize != 20 {
		t.Errorf("min thumb = %f", s.minThumbSize)
	}
}

// ThemeDark is bordered since 2026-08 (see baseDarkCfg); callers who
// want the old borderless look use Theme.WithBorders(false). What every
// widget style must have is a real corner radius, so a widget never renders
// with square corners just because a style went unpopulated.
func TestDefaultWidgetStylesRadius(t *testing.T) {
	styles := []struct {
		name   string
		radius float32
	}{
		{"button", ThemeDark.ButtonStyle.Radius},
		{"input", ThemeDark.InputStyle.Radius},
		{"toggle", ThemeDark.toggleStyle.Radius},
		{"select", ThemeDark.selectStyle.Radius},
		{"listbox", ThemeDark.listBoxStyle.Radius},
	}
	for _, s := range styles {
		if s.radius == 0 {
			t.Errorf("%s has zero radius", s.name)
		}
	}
}

func TestDefaultDialogStyle(t *testing.T) {
	s := ThemeDark.dialogStyle
	if s.Color.eq(Color{}) {
		t.Error("dialog color should not be zero")
	}
	if s.Radius != radiusMedium {
		t.Errorf("radius = %f, want %f", s.Radius, radiusMedium)
	}
	if s.AlignButtons != HAlignCenter {
		t.Errorf("align = %d, want center", s.AlignButtons)
	}
}

func TestDefaultToastStyle(t *testing.T) {
	s := ThemeDark.toastStyle
	if s.maxVisible != 5 {
		t.Errorf("max_visible = %d, want 5", s.maxVisible)
	}
	if s.Width != 260 {
		t.Errorf("width = %f, want 260", s.Width)
	}
	if s.Anchor != toastBottomRight {
		t.Errorf("anchor = %d, want bottom-right", s.Anchor)
	}
}

func TestDefaultTooltipStyle(t *testing.T) {
	s := ThemeDark.tooltipStyle
	if s.Delay == 0 {
		t.Error("delay should not be zero")
	}
	if s.Radius != radiusSmall {
		t.Errorf("radius = %f, want %f", s.Radius, radiusSmall)
	}
}

func TestAffineTransformIsIdentity(t *testing.T) {
	id := glyph.AffineIdentity()
	if !affineTransformIsIdentity(id) {
		t.Error("identity should be identity")
	}
	rot := glyph.AffineRotation(0.5)
	if affineTransformIsIdentity(rot) {
		t.Error("rotation should not be identity")
	}
}

func TestEffectiveTextTransformExplicit(t *testing.T) {
	af := glyph.AffineRotation(1.0)
	ts := TextStyle{AffineTransform: &af}
	got := ts.effectiveTextTransform()
	if got != af {
		t.Error("should return explicit affine transform")
	}
}

func TestEffectiveTextTransformRotation(t *testing.T) {
	ts := TextStyle{RotationRadians: 0.5}
	got := ts.effectiveTextTransform()
	want := glyph.AffineRotation(0.5)
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEffectiveTextTransformDefault(t *testing.T) {
	ts := TextStyle{}
	got := ts.effectiveTextTransform()
	want := glyph.AffineIdentity()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestDefaultTreeStyle(t *testing.T) {
	s := ThemeDark.treeStyle
	if !s.ColorHover.IsSet() {
		t.Error("tree hover color should be set")
	}
	if s.Radius != radiusMedium {
		t.Errorf("radius = %f, want %f", s.Radius, radiusMedium)
	}
	if s.textStyleIcon.Family != IconFontName {
		t.Errorf("icon family = %q, want %q", s.textStyleIcon.Family, IconFontName)
	}
}
