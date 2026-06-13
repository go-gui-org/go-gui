package shader

import (
	"strings"
	"testing"
)

func TestMetalVertexShaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
	}{
		{"VsMetal", VsMetal},
		{"VsShadowMetal", VsShadowMetal},
		{"VsBlurMetal", VsBlurMetal},
		{"VsGradientMetal", VsGradientMetal},
	}
	for _, tt := range tests {
		if tt.source == "" {
			t.Errorf("%s is empty", tt.name)
		}
		if !strings.Contains(tt.source, "vertex") {
			t.Errorf("%s missing vertex keyword", tt.name)
		}
	}
}

func TestMetalFragmentShaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
	}{
		{"FsMetal", FsMetal},
		{"FsShadowMetal", FsShadowMetal},
		{"FsBlurMetal", FsBlurMetal},
		{"FsGradientMetal", FsGradientMetal},
	}
	for _, tt := range tests {
		if tt.source == "" {
			t.Errorf("%s is empty", tt.name)
		}
		if !strings.Contains(tt.source, "fragment") {
			t.Errorf("%s missing fragment keyword", tt.name)
		}
	}
}

func TestGLSLShaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
	}{
		{"VsGLSL", VsGLSL},
		{"FsGLSL", FsGLSL},
	}
	for _, tt := range tests {
		if tt.source == "" {
			t.Errorf("%s is empty", tt.name)
		}
		if !strings.Contains(tt.source, "main") {
			t.Errorf("%s missing main function", tt.name)
		}
	}
}

func TestGLSLShadowShaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
	}{
		{"VsShadowGLSL", VsShadowGLSL},
		{"FsShadowGLSL", FsShadowGLSL},
	}
	for _, tt := range tests {
		if tt.source == "" {
			t.Errorf("%s is empty", tt.name)
		}
	}
}

func TestGLSLBlurShaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
	}{
		{"VsBlurGLSL", VsBlurGLSL},
		{"FsBlurGLSL", FsBlurGLSL},
	}
	for _, tt := range tests {
		if tt.source == "" {
			t.Errorf("%s is empty", tt.name)
		}
	}
}

func TestGLSLGradientShaders(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		source string
	}{
		{"VsGradientGLSL", VsGradientGLSL},
		{"FsGradientGLSL", FsGradientGLSL},
	}
	for _, tt := range tests {
		if tt.source == "" {
			t.Errorf("%s is empty", tt.name)
		}
	}
}
