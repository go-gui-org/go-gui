package shader

import (
	"strings"
	"testing"
)

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
