package gui

import "testing"

func TestValidateImageExtensionValid(t *testing.T) {
	for _, ext := range []string{
		"photo.png", "photo.jpg", "photo.jpeg",
	} {
		if err := validateImageExtension(ext); err != nil {
			t.Errorf("expected valid: %s, got %v", ext, err)
		}
	}
}

func TestValidateImageExtensionInvalid(t *testing.T) {
	for _, ext := range []string{
		"photo.svg", "photo.tiff", "photo.txt", "photo",
		"photo.gif", "photo.bmp", "photo.webp",
	} {
		if err := validateImageExtension(ext); err == nil {
			t.Errorf("expected error for: %s", ext)
		}
	}
}

func TestValidateImageExtensionCaseInsensitive(t *testing.T) {
	if err := validateImageExtension("photo.PNG"); err != nil {
		t.Errorf("expected case-insensitive match: %v", err)
	}
}

func TestValidateImagePathRejectsTraversal(t *testing.T) {
	for _, p := range []string{
		"../secret/photo.png",
		"../../etc/photo.png",
		"..",
	} {
		if err := validateImagePath(p); err == nil {
			t.Errorf("expected error for path traversal: %s", p)
		}
	}
}

func TestValidateImagePathAllowsDoubleDotInName(t *testing.T) {
	// Legitimate paths with ".." in a filename component
	// should be accepted after filepath.Clean.
	if err := validateImagePath("/images/my..dir/photo.png"); err != nil {
		t.Errorf("expected valid for double-dot in dirname: %v", err)
	}
}

func TestValidateImagePathValid(t *testing.T) {
	if err := validateImagePath("/images/photo.png"); err != nil {
		t.Errorf("expected valid path: %v", err)
	}
}

func TestValidateImagePathBadExtension(t *testing.T) {
	if err := validateImagePath("/images/photo.svg"); err == nil {
		t.Fatal("expected error for .svg extension")
	}
}
