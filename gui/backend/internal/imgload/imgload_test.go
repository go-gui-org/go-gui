package imgload

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveValidatedPath(t *testing.T) {
	// Not parallel: subtests mutate process-wide CWD via os.Chdir.
	tmp := t.TempDir()
	file := filepath.Join(tmp, "image.png")
	if err := os.WriteFile(file, []byte("fake png"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("valid absolute path", func(t *testing.T) {
		got, err := ResolveValidatedPath(file, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resolvedFile, _ := filepath.EvalSymlinks(file)
		resolvedGot, _ := filepath.EvalSymlinks(got)
		if resolvedGot != resolvedFile {
			t.Errorf("got %q, want %q", resolvedGot, resolvedFile)
		}
	})

	t.Run("valid relative path", func(t *testing.T) {
		cwd, _ := os.Getwd()
		if err := os.Chdir(tmp); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(cwd) //nolint:errcheck
		relPath := filepath.Base(file)
		got, err := ResolveValidatedPath(relPath, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !filepath.IsAbs(got) {
			t.Errorf("expected absolute path, got %q", got)
		}
	})

	allowed := []string{tmp}

	t.Run("allowed roots", func(t *testing.T) {
		got, err := ResolveValidatedPath(file, allowed)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		resolvedFile, _ := filepath.EvalSymlinks(file)
		resolvedGot, _ := filepath.EvalSymlinks(got)
		if resolvedGot != resolvedFile {
			t.Errorf("got %q, want %q", resolvedGot, resolvedFile)
		}
	})

	t.Run("blocked root", func(t *testing.T) {
		_, err := ResolveValidatedPath("/etc/passwd", allowed)
		if err == nil {
			t.Error("expected error for blocked path")
		}
	})

	t.Run("NUL byte rejected", func(t *testing.T) {
		_, err := ResolveValidatedPath("foo\x00bar", nil)
		if err == nil || !strings.Contains(err.Error(), "NUL") {
			t.Errorf("expected NUL error, got: %v", err)
		}
	})

	t.Run("empty path rejected", func(t *testing.T) {
		_, err := ResolveValidatedPath("", nil)
		if err == nil {
			t.Error("expected error for empty path")
		}
	})

	t.Run("dot path rejected", func(t *testing.T) {
		_, err := ResolveValidatedPath(".", nil)
		if err == nil {
			t.Error("expected error for '.' path")
		}
	})
}

func TestDecodeNRGBABadInput(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(tmp, "empty.png")
		if err := os.WriteFile(path, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_, err = DecodeNRGBA(path, f, 1024, 1024)
		if err == nil {
			t.Error("expected error for empty file")
		}
	})

	t.Run("too large bytes", func(t *testing.T) {
		path := filepath.Join(tmp, "large.png")
		if err := os.WriteFile(path, make([]byte, 2048), 0o644); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		_, err = DecodeNRGBA(path, f,
			1024, // maxBytes < 2048
			1024,
		)
		if err == nil {
			t.Error("expected error for oversized file")
		}
	})
}

func TestDecodeNRGBADefaultLimits(t *testing.T) {
	t.Parallel()
	// validateFile uses DefaultMaxImageBytes / DefaultMaxImagePixels
	// when maxBytes/maxPixels <= 0.
	tmp := t.TempDir()

	t.Run("zero max uses defaults", func(t *testing.T) {
		path := filepath.Join(tmp, "tiny.png")
		srcImg := image.NewRGBA(image.Rect(0, 0, 1, 1))
		fw, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(fw, srcImg); err != nil {
			fw.Close()
			t.Fatal(err)
		}
		fw.Close()

		fr, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer fr.Close()
		decoded, err := DecodeNRGBA(path, fr, 0, 0)
		if err != nil {
			t.Fatalf("unexpected error with zero defaults: %v", err)
		}
		if decoded == nil {
			t.Error("expected non-nil image")
		}
	})
}
