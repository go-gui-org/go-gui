package gui

import "testing"

func TestBadgeDefaultLayout(t *testing.T) {
	v := Badge(BadgeCfg{Label: "3"})
	layout := GenerateViewLayout(v, &Window{})
	if layout.Shape.Axis != AxisLeftToRight {
		t.Error("badge should be a row")
	}
	if len(layout.Children) != 1 {
		t.Fatalf("children: got %d, want 1", len(layout.Children))
	}
	if layout.Children[0].Shape.shapeType != shapeText {
		t.Error("child should be text")
	}
}

func TestBadgeDotMode(t *testing.T) {
	v := Badge(BadgeCfg{Dot: true, DotSize: SomeF(10)})
	layout := GenerateViewLayout(v, &Window{})
	if layout.Shape.Width != 10 || layout.Shape.Height != 10 {
		t.Errorf("dot size = %fx%f, want 10x10",
			layout.Shape.Width, layout.Shape.Height)
	}
	if layout.Shape.Sizing != FixedFixed {
		t.Error("dot should be fixed sizing")
	}
	if len(layout.Children) != 0 {
		t.Error("dot should have no children")
	}
}

func TestBadgeVariantColors(t *testing.T) {
	style := guiTheme.BadgeStyle
	tests := []struct {
		variant BadgeVariant
		want    Color
	}{
		{BadgeDefault, style.Color},
		{BadgeInfo, style.ColorInfo},
		{BadgeSuccess, style.ColorSuccess},
		{BadgeWarning, style.ColorWarning},
		{BadgeError, style.ColorError},
	}
	for _, tt := range tests {
		v := Badge(BadgeCfg{Label: "x", Variant: tt.variant})
		layout := GenerateViewLayout(v, &Window{})
		if layout.Shape.Color != tt.want {
			t.Errorf("variant %d: color mismatch", tt.variant)
		}
	}
}

func TestBadgeLabelMax(t *testing.T) {
	tests := []struct {
		label string
		max   int
		want  string
	}{
		{"5", 10, "5"},
		{"15", 10, "10+"},
		{"abc", 10, "abc"},
		{"3", 0, "3"},
		{"", 10, ""},
		{"99999999999999999999", 10, "10+"},
	}
	for _, tt := range tests {
		got := badgeLabel(tt.label, tt.max)
		if got != tt.want {
			t.Errorf("badgeLabel(%q, %d) = %q, want %q",
				tt.label, tt.max, got, tt.want)
		}
	}
}

func TestBadgeA11Y(t *testing.T) {
	v := Badge(BadgeCfg{Label: "42"})
	layout := GenerateViewLayout(v, &Window{})
	if layout.Shape.A11Y == nil {
		t.Fatal("a11y should be set")
	}
	if layout.Shape.A11Y.Label != "42" {
		t.Errorf("a11y label = %q, want 42", layout.Shape.A11Y.Label)
	}
}

func TestBadgeA11YDescription(t *testing.T) {
	v := Badge(BadgeCfg{Label: "5", A11YDescription: "unread messages"})
	layout := GenerateViewLayout(v, &Window{})
	if layout.Shape.A11Y == nil {
		t.Fatal("a11y should be set")
	}
	if layout.Shape.A11Y.Description != "unread messages" {
		t.Errorf("a11y description = %q, want %q",
			layout.Shape.A11Y.Description, "unread messages")
	}
}

func TestBadgeA11YDescriptionDot(t *testing.T) {
	v := Badge(BadgeCfg{Dot: true, A11YDescription: "active"})
	layout := GenerateViewLayout(v, &Window{})
	if layout.Shape.A11Y == nil {
		t.Fatal("a11y should be set")
	}
	if layout.Shape.A11Y.Description != "active" {
		t.Errorf("a11y description = %q, want %q",
			layout.Shape.A11Y.Description, "active")
	}
}

func TestBadgeCustomColorWithVariant(t *testing.T) {
	custom := RGBA(1, 2, 3, 255)
	v := Badge(BadgeCfg{Label: "x", Color: custom, Variant: BadgeError})
	layout := GenerateViewLayout(v, &Window{})
	// Variant overrides custom color
	if layout.Shape.Color != guiTheme.BadgeStyle.ColorError {
		t.Error("variant should override custom color")
	}
}

func TestBadgeTextStyle(t *testing.T) {
	v := Badge(BadgeCfg{Label: "x"})
	layout := GenerateViewLayout(v, &Window{})
	if len(layout.Children) != 1 {
		t.Fatalf("children: got %d, want 1", len(layout.Children))
	}
	ts := layout.Children[0].Shape.TC.TextStyle
	want := guiTheme.BadgeStyle.TextStyle
	if ts.Size != want.Size || ts.Color != want.Color ||
		ts.Typeface != want.Typeface {
		t.Errorf("text style mismatch: got size=%v color=%v face=%v",
			ts.Size, ts.Color, ts.Typeface)
	}
}
