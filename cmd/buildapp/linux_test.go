package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readTarGz returns every regular file in the archive, keyed by path.
func readTarGz(t *testing.T, path string) (map[string]string, map[string]int64) {
	t.Helper()
	f, err := os.Open(path) // #nosec G304 — test temp dir
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]string{}
	mode := map[string]int64{}
	tr := tar.NewReader(gz)
	for {
		hdr, rerr := tr.Next()
		if errors.Is(rerr, io.EOF) {
			break
		}
		if rerr != nil {
			t.Fatal(rerr)
		}
		b, rerr := io.ReadAll(tr)
		if rerr != nil {
			t.Fatal(rerr)
		}
		body[hdr.Name] = string(b)
		mode[hdr.Name] = hdr.Mode
	}
	return body, mode
}

func TestBuildLinuxLayout(t *testing.T) {
	bin := crossStub(t, "linux", "amd64", "showcase")
	out := t.TempDir()
	icon := writeTestPNG(t, 256)

	err := build(bundleOpts{
		Platform: "linux", Binary: bin, OutDir: out,
		Name: "Go-Gui Showcase", Version: "1.2.3", Icon: icon,
	})
	if err != nil {
		t.Fatal(err)
	}

	tgz := filepath.Join(out, "go-gui-showcase-1.2.3-linux-amd64.tar.gz")
	body, mode := readTarGz(t, tgz)

	const root = "go-gui-showcase-1.2.3-linux-amd64"
	want := []string{
		root + "/bin/showcase",
		root + "/share/applications/local.gogui.go-gui-showcase.desktop",
		root + "/share/icons/hicolor/256x256/apps/local.gogui.go-gui-showcase.png",
		root + "/install.sh",
	}
	for _, w := range want {
		if _, ok := body[w]; !ok {
			t.Errorf("missing archive entry %q; got %v", w, keys(body))
		}
	}
	if got := mode[root+"/bin/showcase"]; got != 0o755 {
		t.Errorf("binary mode = %o, want 755", got)
	}
	if got := mode[root+"/install.sh"]; got != 0o755 {
		t.Errorf("install.sh mode = %o, want 755", got)
	}
}

func TestBuildLinuxDesktopEntry(t *testing.T) {
	bin := crossStub(t, "linux", "amd64", "showcase")
	out := t.TempDir()
	err := build(bundleOpts{
		Platform: "linux", Binary: bin, OutDir: out,
		Name: "Showcase", ID: "org.gogui.showcase", Version: "9",
	})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := readTarGz(t, filepath.Join(out, "showcase-9-linux-amd64.tar.gz"))
	desktop := body["showcase-9-linux-amd64/share/applications/org.gogui.showcase.desktop"]

	// Terminal=false is the line that stops a terminal emulator opening
	// beside the app, which is the whole point of the entry.
	for _, want := range []string{
		"[Desktop Entry]", "Type=Application", "Name=Showcase",
		"Exec=showcase", "Icon=org.gogui.showcase", "Terminal=false",
		"Categories=Utility;",
	} {
		if !strings.Contains(desktop, want) {
			t.Errorf("desktop entry missing %q:\n%s", want, desktop)
		}
	}
}

// With no -icon the archive must carry no icon file, and install.sh
// must not try to install one.
func TestBuildLinuxWithoutIcon(t *testing.T) {
	bin := crossStub(t, "linux", "amd64", "showcase")
	out := t.TempDir()
	if err := build(bundleOpts{
		Platform: "linux", Binary: bin, OutDir: out, Version: "1",
	}); err != nil {
		t.Fatal(err)
	}
	body, _ := readTarGz(t, filepath.Join(out, "showcase-1-linux-amd64.tar.gz"))
	for name := range body {
		if strings.Contains(name, "/icons/") {
			t.Errorf("unexpected icon entry %q", name)
		}
	}
	if s := body["showcase-1-linux-amd64/install.sh"]; strings.Contains(s, "hicolor") {
		t.Errorf("install.sh references an icon that is not in the archive:\n%s", s)
	}
}

func TestBuildLinuxRejectsNonPNGIcon(t *testing.T) {
	bin := crossStub(t, "linux", "amd64", "showcase")
	icns := filepath.Join(t.TempDir(), "icon.icns")
	if err := os.WriteFile(icns, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := build(bundleOpts{
		Platform: "linux", Binary: bin, OutDir: t.TempDir(), Icon: icns,
	})
	if err == nil || !strings.Contains(err.Error(), "must be .png") {
		t.Fatalf("err = %v, want a .png complaint", err)
	}
}

func TestElfArchRejectsNonELF(t *testing.T) {
	p := filepath.Join(t.TempDir(), "text")
	if err := os.WriteFile(p, []byte("not an elf"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := elfArch(p); err == nil {
		t.Fatal("expected an error for a text file")
	}
}

func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Go-Gui Showcase": "go-gui-showcase",
		"Showcase":        "showcase",
		"  spaced  out  ": "spaced-out",
		"v1.2_beta":       "v1.2_beta",
		"!!!":             "",
		"A/B\\C":          "a-b-c",
	}
	for in, want := range cases {
		if got := slug(in); got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
