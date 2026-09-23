package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestIsLightGroundPolarity(t *testing.T) {
	if isLightGround(gui.ThemeDark) {
		t.Error("ThemeDark classified as light ground")
	}
	if !isLightGround(gui.ThemeLight) {
		t.Error("ThemeLight classified as dark ground")
	}
}

func TestSkeletonCustomColorsDarkGround(t *testing.T) {
	base, hl := skeletonCustomColors(gui.ThemeDark)
	if base != gui.RGB(60, 60, 80) {
		t.Errorf("dark base = %v, want RGB(60, 60, 80)", base)
	}
	if hl != gui.RGB(100, 100, 140) {
		t.Errorf("dark highlight = %v, want RGB(100, 100, 140)", hl)
	}
}

func TestSkeletonCustomColorsLightGround(t *testing.T) {
	base, hl := skeletonCustomColors(gui.ThemeLight)
	if base != gui.RGB(200, 205, 218) {
		t.Errorf("light base = %v, want RGB(200, 205, 218)", base)
	}
	if hl != gui.RGB(240, 245, 255) {
		t.Errorf("light highlight = %v, want RGB(240, 245, 255)", hl)
	}
	// The light pair must read as a lifted slate, not a
	// near-black slab: its channels sit well above the dark pair.
	sum := uint(base.R) + uint(base.G) + uint(base.B)
	if sum < 450 {
		t.Errorf("light base channel sum = %d, want >= 450", sum)
	}
	if hl == base {
		t.Errorf("highlight = base %v: no visible shimmer", base)
	}
}
