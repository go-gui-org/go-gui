// macOS bundling: wraps a Mach-O executable in a signed .app bundle.
//
// Everything that shells out to Apple's tool-chain (sips, iconutil,
// otool, install_name_tool, codesign) lives here.  The file carries no
// build constraint so that the whole tool still compiles and vets on
// Linux and Windows hosts; the macOS paths simply fail at run time
// there when the tools are absent.
package main

import (
	"debug/macho"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"
)

const infoPlistTmpl = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>{{xml .Exec}}</string>
	<key>CFBundleIdentifier</key><string>{{xml .ID}}</string>
	<key>CFBundleName</key><string>{{xml .Name}}</string>
	<key>CFBundlePackageType</key><string>APPL</string>
	<key>CFBundleVersion</key><string>{{xml .Build}}</string>
	<key>CFBundleShortVersionString</key><string>{{xml .Version}}</string>
	<key>LSMinimumSystemVersion</key><string>11.0</string>
	<key>NSHighResolutionCapable</key><true/>
{{- if .Icon}}
	<key>CFBundleIconFile</key><string>{{xml .Icon}}</string>
{{- end}}
{{- if .Category}}
	<key>LSApplicationCategoryType</key><string>{{xml .Category}}</string>
{{- end}}
</dict>
</plist>
`

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

// plistFields are the values infoPlistTmpl substitutes. Icon is the
// .icns basename in Resources; empty omits the key, as does an empty
// Category. An empty Build repeats Version.
type plistFields struct {
	Exec, ID, Name, Version, Build, Icon, Category string
}

// #nosec G304 — path is developer-controlled CLI flag
func writePlist(path string, pf plistFields) error {
	if pf.Build == "" {
		pf.Build = pf.Version
	}
	// text/template does not escape, and a display name such as
	// "Tom & Jerry" would make the plist malformed XML, which codesign
	// rejects. Every value goes through xmlEscape.
	t := template.Must(template.New("plist").
		Funcs(template.FuncMap{"xml": xmlEscape}).Parse(infoPlistTmpl))
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return t.Execute(f, pf)
}

// xmlEscape returns s escaped for XML character data.
func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s)) // a strings.Builder write never fails
	return b.String()
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
		if err := execLookPath("sips"); err != nil {
			return "", errors.New("sips not found (needed for .png icon)")
		}
		if err := execLookPath("iconutil"); err != nil {
			return "", errors.New("iconutil not found (needed for .png icon)")
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
		b, cerr := runTool("sips", "-z", strconv.Itoa(s.px), strconv.Itoa(s.px), png, "--out", out)
		if cerr != nil {
			return fmt.Errorf("sips: %v: %s", cerr, b)
		}
	}
	b, err := runTool("iconutil", "-c", "icns", iconset, "-o", outIcns)
	if err != nil {
		return fmt.Errorf("iconutil: %v: %s", err, b)
	}
	return nil
}

// bundleDeps copies non-system dylibs referenced by binary into
// Contents/Frameworks, rewrites all install names to @rpath form, adds
// an rpath of @executable_path/../Frameworks, and re-signs every
// modified file with signID. Recurses through transitive dependencies.
// #nosec G204,G301 — build tool, args from otool output on own binaries
func bundleDeps(binary, contents, signID string) error {
	for _, tool := range []string{"otool", "install_name_tool", "codesign"} {
		if err := execLookPath(tool); err != nil {
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
				if out, err := runTool("install_name_tool",
					"-id", "@rpath/"+base, dst); err != nil {
					return fmt.Errorf("install_name_tool -id %s: %v: %s", dst, err, out)
				}
				queue = append(queue, dst)
			}
			if out, err := runTool("install_name_tool",
				"-change", dep, "@rpath/"+base, cur); err != nil {
				return fmt.Errorf("install_name_tool -change %s: %v: %s", cur, err, out)
			}
		}
	}

	// rpath only on the executable; dylibs resolve via the same loader
	if out, err := runTool("install_name_tool",
		"-add_rpath", "@executable_path/../Frameworks", binary); err != nil {
		return fmt.Errorf("add_rpath: %v: %s", err, out)
	}

	// re-sign everything we touched: install_name_tool invalidates the
	// existing signature, and an unsigned Mach-O will not load on Apple
	// Silicon.  Capture codesign's output — with a caller-supplied
	// identity the usual failure is "identity not found", which is
	// unreadable from the exit status alone.
	signTargets := []string{binary}
	for _, base := range copied {
		signTargets = append(signTargets, filepath.Join(fw, base))
	}
	for _, t := range signTargets {
		if out, err := runTool("codesign", "-s", signID, "--force", t); err != nil {
			return fmt.Errorf("codesign %s: %v: %s", t, err, out)
		}
	}
	return nil
}

// otoolDeps returns the LC_LOAD_DYLIB paths recorded in path. The
// binary's own LC_ID_DYLIB (first line) is dropped.
func otoolDeps(path string) ([]string, error) {
	out, err := runTool("otool", "-L", path)
	if err != nil {
		return nil, fmt.Errorf("otool -L %s: %v: %s", path, err, out)
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

// signBundle signs the entire .app bundle with signID.  A missing
// bundle-level signature causes Gatekeeper to report the app as
// "damaged" even when every binary inside is individually signed.
//
// signID "-" is ad-hoc: no certificate, no team identifier, so TCC has
// no designated requirement to key a grant against and falls back to the
// cdhash.  The cdhash changes on every rebuild, so every ad-hoc rebuild
// silently revokes screen recording, microphone, camera, accessibility
// and friends — see README, "Signing".  Pass a real identity to keep
// grants across rebuilds.
//
// Without entitlements the signature uses --deep over the nested code
// under Contents/Frameworks that bundleDeps already signed.  Apple
// deprecates --deep for distribution signing; it is kept here because
// the bundle carries no entitlements and no nested code beyond those
// dylibs, so a same-identity re-sign costs nothing.
//
// With an entitlements file the signature switches to distribution
// form: --options runtime (the hardened runtime) with --entitlements
// and --timestamp, and no --deep.  That is the form notarization
// requires.
func signBundle(appDir, signID, entitlements string) error {
	if err := execLookPath("codesign"); err != nil {
		return errors.New("codesign not found")
	}
	args := []string{"-s", signID, "--force"}
	if entitlements != "" {
		args = append(args, "--options", "runtime",
			"--entitlements", entitlements, "--timestamp")
	} else {
		args = append(args, "--deep")
	}
	args = append(args, appDir)
	if out, err := runTool("codesign", args...); err != nil {
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

// #nosec G301 — standard macOS .app bundle permissions
func buildMacOS(o bundleOpts) error {
	if o.Notarize && o.Entitlements == "" {
		return errors.New("-notarize needs -entitlements: notarization requires the hardened runtime")
	}
	if o.NotaryProfile != "" && !o.Notarize {
		return errors.New("-notary-profile needs -notarize")
	}
	if o.Notarize && o.NotaryProfile == "" {
		return errors.New("-notarize needs -notary-profile: the notarytool keychain profile to submit with")
	}
	if o.Entitlements != "" {
		if fi, err := os.Stat(o.Entitlements); err != nil || fi.IsDir() {
			return fmt.Errorf("entitlements file %q not found", o.Entitlements)
		}
	}
	if err := validateMachO(o.Binary); err != nil {
		return err
	}
	execName := filepath.Base(o.Binary)
	o.SignID = signIdentityOr(o.SignID)

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

	if err = writePlist(filepath.Join(contents, "Info.plist"), plistFields{
		Exec: execName, ID: o.ID, Name: o.Name, Version: o.Version,
		Build: o.Build, Icon: iconField, Category: o.Category,
	}); err != nil {
		return err
	}
	stagedBin := filepath.Join(macosDir, execName)
	if err = copyFile(o.Binary, stagedBin, 0o755); err != nil {
		return err
	}

	if o.BundleDeps {
		if err = bundleDeps(stagedBin, contents, o.SignID); err != nil {
			return fmt.Errorf("bundle deps: %w", err)
		}
	}

	// Sign the entire .app bundle.  Without a bundle-level signature
	// macOS Gatekeeper reports the app as damaged even when individual
	// binaries inside are signed.
	if err = signBundle(appDir, o.SignID, o.Entitlements); err != nil {
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

	artifact := dst
	if o.Dmg {
		artifact, err = makeDmg(dst, o)
		if err != nil {
			return err
		}
	}
	if o.Notarize {
		if err = notarize(artifact, o.NotaryProfile); err != nil {
			return err
		}
	}
	return nil
}

// makeDmg wraps appDir in a UDZO disk image named
// <slug>-<version>.dmg, the same slug-version form the Windows and
// Linux archives use. The volume shows as "<Name> <Version>" in
// Finder. The .app inside carries the signature; the image itself is
// left unsigned, which Gatekeeper accepts.
func makeDmg(appDir string, o bundleOpts) (string, error) {
	if err := execLookPath("hdiutil"); err != nil {
		return "", errors.New("hdiutil not found")
	}
	dst := filepath.Join(o.OutDir, fmt.Sprintf("%s-%s.dmg", slug(o.Name), o.Version))
	if err := os.RemoveAll(dst); err != nil {
		return "", err
	}
	args := []string{"create", "-srcfolder", appDir,
		"-volname", o.Name + " " + o.Version,
		"-format", "UDZO", dst}
	if out, err := runTool("hdiutil", args...); err != nil {
		return "", fmt.Errorf("hdiutil: %v: %s", err, out)
	}
	fmt.Println(dst)
	return dst, nil
}

// notarize submits artifact (the .dmg when -dmg is set, else the .app)
// to Apple and staples the ticket, so a downloaded app launches
// without a Gatekeeper warning. Credentials stay in the keychain under
// profile; they never travel as flags.
func notarize(artifact, profile string) error {
	if err := execLookPath("xcrun"); err != nil {
		return errors.New("xcrun not found (needed for notarytool)")
	}
	args := []string{"notarytool", "submit", artifact,
		"--keychain-profile", profile, "--wait"}
	if out, err := runTool("xcrun", args...); err != nil {
		return fmt.Errorf("notarytool submit: %v: %s", err, out)
	}
	if out, err := runTool("xcrun", "stapler", "staple", artifact); err != nil {
		return fmt.Errorf("stapler staple: %v: %s", err, out)
	}
	return nil
}
