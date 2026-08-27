package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover(t *testing.T) {
	// Synthetic directory with two examples.
	tmp := t.TempDir()
	for _, name := range []string{"alpha", "beta"} {
		dir := filepath.Join(tmp, name)
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	// alpha has full explorer block
	if err := os.WriteFile(filepath.Join(tmp, "alpha", "README.md"), []byte(
		"# Alpha Title\n\n"+
			"> **Framework:** layout, animation\n"+
			"> **Description:** Alpha does things with layout.\n\n"+
			"![Preview](screenshot.png)\n\n"+
			"<!-- explorer: tags=layout,animation category=layout run=go -->\n\n"+
			"---\n\nBody after.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// beta has minimal readme
	if err := os.WriteFile(filepath.Join(tmp, "beta", "README.md"), []byte(
		"# Beta\n\nJust a paragraph.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// alpha screenshot
	if err := os.WriteFile(filepath.Join(tmp, "alpha", "screenshot.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	// bin should be ignored
	if err := os.MkdirAll(filepath.Join(tmp, "bin"), 0o750); err != nil {
		t.Fatal(err)
	}

	metas, err := Discover(tmp)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2, got %d", len(metas))
	}
	// Sorted
	if metas[0].Name != "alpha" || metas[1].Name != "beta" {
		t.Fatalf("order: %v", metas)
	}
	a := metas[0]
	if a.Title != "Alpha Title" {
		t.Fatalf("title %q", a.Title)
	}
	if a.Description != "Alpha does things with layout." {
		t.Fatalf("desc %q", a.Description)
	}
	if a.Framework != "layout, animation" {
		t.Fatalf("framework %q", a.Framework)
	}
	if len(a.Tags) != 2 || a.Tags[0] != "layout" {
		t.Fatalf("tags %v", a.Tags)
	}
	if !a.HasScreenshot {
		t.Fatalf("expected screenshot")
	}
	b := metas[1]
	if b.Title != "Beta" {
		t.Fatalf("beta title %q", b.Title)
	}
	if b.HasScreenshot {
		t.Fatalf("beta should not have screenshot")
	}
}

func TestProbeScreenshotFallback(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "ex")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	// Only icon.png
	if err := os.WriteFile(filepath.Join(dir, "icon.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := probeScreenshot(dir); got == "" {
		t.Fatalf("expected fallback")
	}
	// Prefer screenshot.png
	if err := os.WriteFile(filepath.Join(dir, "screenshot.png"), []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := probeScreenshot(dir); filepath.Base(got) != "screenshot.png" {
		t.Fatalf("prefer screenshot.png got %q", got)
	}
	// legacy screenshot1.png
	tmp2 := t.TempDir()
	dir2 := filepath.Join(tmp2, "ex2")
	if err := os.MkdirAll(dir2, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "screenshot1.png"), []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := probeScreenshot(dir2); filepath.Base(got) != "screenshot1.png" {
		t.Fatalf("legacy got %q", got)
	}
}

func TestParseReadme(t *testing.T) {
	tmp := t.TempDir()
	p := filepath.Join(tmp, "README.md")
	if err := os.WriteFile(p, []byte(
		"# Title\n\n"+
			"> **Framework:** input\n"+
			"> **Description:** Does input things.\n\n"+
			"![Preview](preview.png)\n\n"+
			"<!-- explorer: tags=input category=input run=go -->\n\n"+
			"---\n\nRest.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	title, desc, fw, tags, cat, img := parseReadme(p)
	if title != "Title" || desc != "Does input things." || fw != "input" || cat != "input" || img != "preview.png" || len(tags) != 1 {
		t.Fatalf("parse %#v %#v %#v %#v %#v %#v", title, desc, fw, tags, cat, img)
	}
}
