// Linux packaging: a .tar.gz holding the binary, a .desktop entry, an
// icon and an install script.
//
// A bare binary in a tarball is not installable in any desktop sense:
// nothing puts it in the application menu, and nothing gives it an
// icon.  The freedesktop.org layout below is what every menu
// implementation reads, and it works unprivileged out of ~/.local.
package main

import (
	"archive/tar"
	"compress/gzip"
	"debug/elf"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// desktopTmpl is a freedesktop.org Desktop Entry.  Terminal=false is
// the field that stops a terminal emulator opening alongside the app.
// Icon names the icon *theme key*, not a path, so it must match the
// basename installed under share/icons/hicolor.
const desktopTmpl = `[Desktop Entry]
Type=Application
Name={{.Name}}
Exec={{.Exec}}
Icon={{.Icon}}
Terminal=false
Categories={{.Categories}}
`

// installShTmpl copies the payload files into a prefix.  The default
// prefix is ~/.local, which needs no privileges and which every current
// desktop searches.  update-desktop-database is optional: menus pick
// the entry up on next login without it.
const installShTmpl = `#!/bin/sh
# Install {{.Name}}.  Usage: ./install.sh [prefix]   (default ~/.local)
set -eu
prefix="${1:-$HOME/.local}"
here="$(cd "$(dirname "$0")" && pwd)"

install -Dm755 "$here/bin/{{.Exec}}" "$prefix/bin/{{.Exec}}"
install -Dm644 "$here/share/applications/{{.ID}}.desktop" \
  "$prefix/share/applications/{{.ID}}.desktop"
{{- if .HasIcon}}
install -Dm644 "$here/share/icons/hicolor/{{.IconSize}}/apps/{{.ID}}.png" \
  "$prefix/share/icons/hicolor/{{.IconSize}}/apps/{{.ID}}.png"
{{- end}}

command -v update-desktop-database >/dev/null 2>&1 &&
  update-desktop-database "$prefix/share/applications" || true

echo "installed to $prefix"
echo "if {{.Exec}} is not found, add $prefix/bin to PATH"
`

// linuxIconDir is the hicolor sub-directory the icon is filed under.
// Icons are copied, not rescaled, so one nominal size is enough; menus
// scale whatever they find.
const linuxIconDir = "256x256"

// defaultCategories keeps the entry out of the "Other" menu bucket.
const defaultCategories = "Utility;"

// buildLinux writes <outdir>/<slug>-<version>-linux-<arch>.tar.gz.
func buildLinux(o bundleOpts) error {
	arch, err := elfArch(o.Binary)
	if err != nil {
		return err
	}
	execName := filepath.Base(o.Binary)
	root := fmt.Sprintf("%s-%s-linux-%s", slug(o.Name), o.Version, arch)

	// Files are assembled in memory: the payload is one binary plus two
	// short text files, and staging on disk would only add cleanup.
	var files []tarEntry

	bin, err := os.ReadFile(o.Binary) // #nosec G304 — CLI flag
	if err != nil {
		return err
	}
	files = append(files, tarEntry{root + "/bin/" + execName, 0o755, bin})

	hasIcon := o.Icon != ""
	if hasIcon {
		if ext := strings.ToLower(filepath.Ext(o.Icon)); ext != ".png" {
			return fmt.Errorf("linux icon must be .png, got %q", ext)
		}
		png, ierr := os.ReadFile(o.Icon) // #nosec G304 — CLI flag
		if ierr != nil {
			return ierr
		}
		files = append(files, tarEntry{
			root + "/share/icons/hicolor/" + linuxIconDir + "/apps/" + o.ID + ".png",
			0o644, png,
		})
	}

	desktop, err := renderTmpl(desktopTmpl, map[string]string{
		"Name": o.Name, "Exec": execName, "Icon": o.ID,
		"Categories": defaultCategories,
	})
	if err != nil {
		return err
	}
	files = append(files, tarEntry{
		root + "/share/applications/" + o.ID + ".desktop", 0o644, desktop,
	})

	script, err := renderTmpl(installShTmpl, map[string]any{
		"Name": o.Name, "Exec": execName, "ID": o.ID,
		"HasIcon": hasIcon, "IconSize": linuxIconDir,
	})
	if err != nil {
		return err
	}
	files = append(files, tarEntry{root + "/install.sh", 0o755, script})

	if err = os.MkdirAll(o.OutDir, 0o755); err != nil { // #nosec G301
		return err
	}
	dst := filepath.Join(o.OutDir, root+".tar.gz")
	if err = writeTarGz(dst, files); err != nil {
		return err
	}
	fmt.Println(dst)
	return nil
}

// tarEntry is one regular file in the archive.
type tarEntry struct {
	Name string
	Mode int64
	Data []byte
}

// writeTarGz writes entries to path.  Modification times are left at
// the zero value so two builds of the same input produce byte-identical
// archives.
func writeTarGz(path string, entries []tarEntry) error {
	f, err := os.Create(path) // #nosec G304 — CLI flag
	if err != nil {
		return err
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		hdr := &tar.Header{
			Typeflag: tar.TypeReg,
			Name:     e.Name,
			Mode:     e.Mode,
			Size:     int64(len(e.Data)),
			Format:   tar.FormatPAX,
		}
		if err = tw.WriteHeader(hdr); err != nil {
			return closeAll(err, tw, gz, f)
		}
		if _, err = tw.Write(e.Data); err != nil {
			return closeAll(err, tw, gz, f)
		}
	}
	return closeAll(nil, tw, gz, f)
}

// closeAll closes cs in order, returning the first error seen — either
// the caller's err or the first close failure.
func closeAll(err error, cs ...io.Closer) error {
	for _, c := range cs {
		if cerr := c.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

// elfArch validates that path is an ELF executable and maps its machine
// field onto the Go arch name used in the archive filename.
func elfArch(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", fmt.Errorf("%s is a directory", path)
	}
	f, err := elf.Open(path)
	if err != nil {
		return "", fmt.Errorf("%s is not an ELF executable: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	switch f.Machine {
	case elf.EM_X86_64:
		return "amd64", nil
	case elf.EM_AARCH64:
		return "arm64", nil
	case elf.EM_386:
		return "386", nil
	case elf.EM_ARM:
		return "arm", nil
	case elf.EM_RISCV:
		return "riscv64", nil
	default:
		return "", fmt.Errorf("unsupported ELF machine %v", f.Machine)
	}
}

// slug maps a display name onto a filename-safe token: lower case, with
// runs of anything outside [a-z0-9._-] collapsed to a single dash.
func slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// renderTmpl executes a text/template against data and returns bytes.
func renderTmpl(tmpl string, data any) ([]byte, error) {
	t, err := template.New("t").Parse(tmpl)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	if err = t.Execute(&b, data); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}
