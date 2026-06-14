// buildapp wraps a compiled go-gui binary into a macOS .app bundle.
//
// Usage:
//
//	buildapp [-o outdir] [-name Name] [-id bundle.id] [-icon icon.png] <binary>
package main

import (
	"debug/macho"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

type bundleOpts struct {
	Binary     string
	OutDir     string
	Name       string
	ID         string
	Icon       string
	Version    string
	BundleDeps bool
}

const infoPlistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>{{.Exec}}</string>
	<key>CFBundleIdentifier</key><string>{{.ID}}</string>
	<key>CFBundleName</key><string>{{.Name}}</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleVersion</key><string>{{.Version}}</string>
	<key>CFBundleShortVersionString</key><string>{{.Version}}</string>
	<key>LSMinimumSystemVersion</key><string>11.0</string>
	<key>NSHighResolutionCapable</key><true/>
{{- if .Icon}}
	<key>CFBundleIconFile</key><string>{{.Icon}}</string>
{{- end}}
</dict>
</plist>
`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "buildapp:", err)
		os.Exit(1)
	}
}

func run() error {
	var o bundleOpts
	flag.StringVar(&o.OutDir, "o", ".", "output directory")
	flag.StringVar(&o.Name, "name", "", "bundle display name (default: binary basename)")
	flag.StringVar(&o.ID, "id", "", "bundle identifier (default: local.gogui.<name>)")
	flag.StringVar(&o.Icon, "icon", "", "icon file (.png or .icns)")
	flag.StringVar(&o.Version, "version", "1.0", "bundle version")
	flag.BoolVar(&o.BundleDeps, "bundle-deps", false,
		"copy non-system dylibs into Contents/Frameworks and rewrite paths")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: buildapp [flags] <binary>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		return fmt.Errorf("expected exactly one binary argument")
	}
	o.Binary = flag.Arg(0)
	return build(o)
}

// #nosec G301 — standard macOS .app bundle permissions
func build(o bundleOpts) error {
	if err := validateMachO(o.Binary); err != nil {
		return err
	}
	execName := filepath.Base(o.Binary)
	if o.Name == "" {
		o.Name = strings.ToUpper(execName[:1]) + execName[1:]
	}
	if o.ID == "" {
		o.ID = "local.gogui." + strings.ToLower(o.Name)
	}

	stage, err := os.MkdirTemp("", "buildapp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(stage) }()

	appDir := filepath.Join(stage, o.Name+".app")
	contents := filepath.Join(appDir, "Contents")
	macosDir := filepath.Join(contents, "MacOS")
	resDir := filepath.Join(contents, "Resources")
	if err = os.MkdirAll(macosDir, 0o755); err != nil {
		return err
	}
	if err = os.MkdirAll(resDir, 0o755); err != nil {
		return err
	}

	iconField := ""
	if o.Icon != "" {
		icnsName, ierr := installIcon(o.Icon, resDir, execName)
		if ierr != nil {
			return ierr
		}
		iconField = icnsName
	}

	if err = writePlist(filepath.Join(contents, "Info.plist"), execName, o.ID, o.Name, o.Version, iconField); err != nil {
		return err
	}
	stagedBin := filepath.Join(macosDir, execName)
	if err = copyFile(o.Binary, stagedBin, 0o755); err != nil {
		return err
	}

	if o.BundleDeps {
		if err = bundleDeps(stagedBin, contents); err != nil {
			return fmt.Errorf("bundle deps: %w", err)
		}
	}

	// Sign the entire .app bundle (ad-hoc).  Without a bundle-level
	// signature macOS Gatekeeper reports the app as damaged even when
	// individual binaries inside are signed.
	if err = signBundle(appDir); err != nil {
		return fmt.Errorf("sign bundle: %w", err)
	}

	if err = os.MkdirAll(o.OutDir, 0o755); err != nil {
		return err
	}
	dst := filepath.Join(o.OutDir, o.Name+".app")
	if err = os.RemoveAll(dst); err != nil {
		return err
	}
	if err = moveDir(appDir, dst); err != nil {
		return err
	}
	fmt.Println(dst)
	return nil
}

func validateMachO(path string) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	if fi.IsDir() {
		return fmt.Errorf("%s is a directory", path)
	}
	f, err := macho.Open(path)
	if err != nil {
		// also accept fat binaries
		if ff, ferr := macho.OpenFat(path); ferr == nil {
			_ = ff.Close()
			return nil
		}
		return fmt.Errorf("%s is not a Mach-O executable: %w", path, err)
	}
	_ = f.Close()
	return nil
}

// #nosec G304 — path is developer-controlled CLI flag
func writePlist(path, execName, id, name, version, icon string) error {
	t := template.Must(template.New("plist").Parse(infoPlistTmpl))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return t.Execute(f, map[string]string{
		"Exec": execName, "ID": id, "Name": name, "Version": version, "Icon": icon,
	})
}

// #nosec G304 — src/dst are developer-controlled CLI flags
func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err = io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

// installIcon places an .icns file in resDir and returns its basename.
// Accepts an .icns passthrough or converts a .png via sips+iconutil.
func installIcon(icon, resDir, execName string) (string, error) {
	ext := strings.ToLower(filepath.Ext(icon))
	icnsName := execName + ".icns"
	dst := filepath.Join(resDir, icnsName)
	switch ext {
	case ".icns":
		if err := copyFile(icon, dst, 0o644); err != nil {
			return "", err
		}
	case ".png":
		if _, err := exec.LookPath("sips"); err != nil {
			return "", fmt.Errorf("sips not found (needed for .png icon)")
		}
		if _, err := exec.LookPath("iconutil"); err != nil {
			return "", fmt.Errorf("iconutil not found (needed for .png icon)")
		}
		if err := pngToIcns(icon, dst); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported icon type %q (need .png or .icns)", ext)
	}
	return icnsName, nil
}

// #nosec G204,G301 — build tool, all args from flags or temp dirs
func pngToIcns(png, outIcns string) error {
	tmp, err := os.MkdirTemp("", "buildapp-icon-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	iconset := filepath.Join(tmp, "icon.iconset")
	if err = os.Mkdir(iconset, 0o755); err != nil {
		return err
	}
	sizes := []struct {
		px   int
		name string
	}{
		{16, "icon_16x16.png"}, {32, "icon_16x16@2x.png"},
		{32, "icon_32x32.png"}, {64, "icon_32x32@2x.png"},
		{128, "icon_128x128.png"}, {256, "icon_128x128@2x.png"},
		{256, "icon_256x256.png"}, {512, "icon_256x256@2x.png"},
		{512, "icon_512x512.png"}, {1024, "icon_512x512@2x.png"},
	}
	for _, s := range sizes {
		out := filepath.Join(iconset, s.name)
		cmd := exec.Command("sips", "-z", fmt.Sprint(s.px), fmt.Sprint(s.px), png, "--out", out)
		if b, cerr := cmd.CombinedOutput(); cerr != nil {
			return fmt.Errorf("sips: %v: %s", cerr, b)
		}
	}
	cmd := exec.Command("iconutil", "-c", "icns", iconset, "-o", outIcns)
	if b, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("iconutil: %v: %s", err, b)
	}
	return nil
}

// moveDir moves src to dst, falling back to copy+remove across devices.
func moveDir(src, dst string) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyTree(src, dst); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

// bundleDeps copies non-system dylibs referenced by binary into
// Contents/Frameworks, rewrites all install names to @rpath form, adds
// an rpath of @executable_path/../Frameworks, and ad-hoc re-signs every
// modified file. Recurses through transitive dependencies.
// #nosec G204,G301 — build tool, args from otool output on own binaries
func bundleDeps(binary, contents string) error {
	for _, tool := range []string{"otool", "install_name_tool", "codesign"} {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("%s not found", tool)
		}
	}
	fw := filepath.Join(contents, "Frameworks")
	if err := os.MkdirAll(fw, 0o755); err != nil {
		return err
	}

	// queue of Mach-O files to process; map tracks dylibs already copied
	// (key = original absolute path, value = bundled basename).
	copied := map[string]string{}
	queue := []string{binary}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		deps, err := otoolDeps(cur)
		if err != nil {
			return err
		}
		for _, dep := range deps {
			if isSystemLib(dep) {
				continue
			}
			base := filepath.Base(dep)
			if _, seen := copied[dep]; !seen {
				dst := filepath.Join(fw, base)
				if err = copyFile(dep, dst, 0o755); err != nil {
					return fmt.Errorf("copy %s: %w", dep, err)
				}
				copied[dep] = base
				if err = exec.Command("install_name_tool",
					"-id", "@rpath/"+base, dst).Run(); err != nil {
					return fmt.Errorf("install_name_tool -id %s: %w", dst, err)
				}
				queue = append(queue, dst)
			}
			if err = exec.Command("install_name_tool",
				"-change", dep, "@rpath/"+base, cur).Run(); err != nil {
				return fmt.Errorf("install_name_tool -change %s: %w", cur, err)
			}
		}
	}

	// rpath only on the executable; dylibs resolve via the same loader
	if err := exec.Command("install_name_tool",
		"-add_rpath", "@executable_path/../Frameworks", binary).Run(); err != nil {
		return fmt.Errorf("add_rpath: %w", err)
	}

	// ad-hoc re-sign everything we touched
	signTargets := []string{binary}
	for _, base := range copied {
		signTargets = append(signTargets, filepath.Join(fw, base))
	}
	for _, t := range signTargets {
		if err := exec.Command("codesign", "-s", "-", "--force", t).Run(); err != nil {
			return fmt.Errorf("codesign %s: %w", t, err)
		}
	}
	return nil
}

// otoolDeps returns the LC_LOAD_DYLIB paths recorded in path. The
// binary's own LC_ID_DYLIB (first line) is dropped.
// #nosec G204 — build tool, path from own binary
func otoolDeps(path string) ([]string, error) {
	out, err := exec.Command("otool", "-L", path).Output()
	if err != nil {
		return nil, fmt.Errorf("otool -L %s: %w", path, err)
	}
	lines := strings.Split(string(out), "\n")
	if len(lines) < 2 {
		return nil, nil
	}
	deps := make([]string, 0, len(lines))
	// lines[0] is "<path>:"; for dylibs lines[1] is the LC_ID_DYLIB self-ref
	start := 1
	if strings.HasSuffix(path, ".dylib") && len(lines) > 1 {
		start = 2
	}
	for _, ln := range lines[start:] {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		// "<path> (compatibility version ..., current version ...)"
		if i := strings.Index(ln, " ("); i > 0 {
			ln = ln[:i]
		}
		// skip @rpath/@loader_path/@executable_path entries already rewritten
		if strings.HasPrefix(ln, "@") {
			continue
		}
		deps = append(deps, ln)
	}
	return deps, nil
}

// signBundle ad-hoc signs the entire .app bundle.  A missing
// bundle-level signature causes Gatekeeper to report the app as
// "damaged" even when every binary inside is individually signed.
// #nosec G204 — build tool, appDir from temp dir
func signBundle(appDir string) error {
	if _, err := exec.LookPath("codesign"); err != nil {
		return fmt.Errorf("codesign not found")
	}
	cmd := exec.Command("codesign", "-s", "-", "--force", "--deep", appDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}

// isSystemLib reports whether path lives in a macOS-shipped location and
// can be safely left as an absolute reference.
func isSystemLib(path string) bool {
	switch {
	case strings.HasPrefix(path, "/usr/lib/"),
		strings.HasPrefix(path, "/System/Library/"),
		strings.HasPrefix(path, "/Library/Apple/"):
		return true
	}
	return false
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(src, p)
		if rerr != nil {
			return rerr
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(p, target, info.Mode())
	})
}
