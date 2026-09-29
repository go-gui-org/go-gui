// buildapp packages a compiled go-gui binary for release.
//
// macOS gets a signed .app bundle, Windows a .zip holding a
// GUI-subsystem .exe with an embedded icon, Linux a .tar.gz holding the
// binary plus a .desktop entry and icon.
//
// Usage:
//
//	buildapp [-platform darwin|windows|linux] [-o outdir] [-manifest appinfo.toml]
//	         [-name Name] [-id bundle.id] [-version v] [-build n]
//	         [-icon icon.png] [-sign identity] <binary>
//
// Without -manifest, buildapp reads appinfo.toml from the working
// directory when one is there. A flag given on the command line wins
// over the file.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type bundleOpts struct {
	// Platform selects the packager.  Empty means the host GOOS, which
	// keeps every pre-existing invocation working unchanged.
	Platform   string
	Binary     string
	OutDir     string
	Name       string
	ID         string
	Icon       string
	Version    string
	BundleDeps bool
	// SignID is the codesign identity ("-" = ad-hoc).  Empty is treated
	// as "-" so a zero-valued bundleOpts keeps the historical behaviour.
	SignID string
	// Build is the build number (macOS CFBundleVersion). Empty means
	// Version, which is what the plist held before the manifest.
	Build string
	// Category is the macOS LSApplicationCategoryType, from the
	// manifest's [darwin] section. Empty writes no key.
	Category string
	// Categories is the Linux .desktop Categories value, from the
	// manifest's [linux] section. Empty means defaultCategories.
	Categories string
}

// envSignIdentity supplies the -sign default, so a developer with a
// certificate can set it once per machine instead of per Makefile.
const envSignIdentity = "BUILDAPP_SIGN_IDENTITY"

// adHocIdentity is codesign's spelling of "sign with no certificate".
const adHocIdentity = "-"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "buildapp:", err)
		os.Exit(1)
	}
}

func run() error {
	var o bundleOpts
	manifest := flag.String("manifest", "",
		"app manifest (default: ./"+defaultManifest+" when present)")
	flag.StringVar(&o.Platform, "platform", runtime.GOOS,
		"target platform: darwin, windows or linux")
	flag.StringVar(&o.OutDir, "o", ".", "output directory")
	flag.StringVar(&o.Name, "name", "", "bundle display name (default: binary basename)")
	flag.StringVar(&o.ID, "id", "", "bundle identifier (default: local.gogui.<name>)")
	flag.StringVar(&o.Icon, "icon", "", "icon file (.png or .icns)")
	flag.StringVar(&o.Version, "version", "1.0", "bundle version")
	flag.StringVar(&o.Build, "build", "", "build number (default: the version)")
	flag.BoolVar(&o.BundleDeps, "bundle-deps", false,
		"copy non-system dylibs into Contents/Frameworks and rewrite paths")
	flag.StringVar(&o.SignID, "sign", defaultSignIdentity(),
		"codesign identity; \"-\" is ad-hoc, which drops TCC grants on every rebuild (see README)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: buildapp [flags] <binary>\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		return errors.New("expected exactly one binary argument")
	}
	o.Binary = flag.Arg(0)
	info, dir, err := loadManifest(*manifest)
	if err != nil {
		return err
	}
	// flag.Visit walks only the flags given on the command line, which
	// is what tells an explicit -version apart from its default.
	set := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { set[f.Name] = true })
	applyManifest(&o, info, dir, set)
	return build(o)
}

// defaultSignIdentity resolves the -sign default from the environment,
// falling back to ad-hoc.  An explicit -sign wins because flag.Parse
// overwrites the default.
func defaultSignIdentity() string {
	return signIdentityOr(os.Getenv(envSignIdentity))
}

// signIdentityOr maps an unset identity onto ad-hoc.  Used for both the
// env default and for programmatic callers that leave SignID empty.
func signIdentityOr(id string) string {
	if id == "" {
		return adHocIdentity
	}
	return id
}

// build dispatches to the packager for o.Platform.  Name and ID
// defaulting is shared, because every platform derives the same two
// values from the binary's basename.
func build(o bundleOpts) error {
	if o.Platform == "" {
		o.Platform = runtime.GOOS
	}
	if o.Binary == "" {
		return errors.New("no binary given")
	}
	execName := filepath.Base(o.Binary)
	if o.Name == "" {
		o.Name = strings.ToUpper(execName[:1]) + execName[1:]
	}
	if o.ID == "" {
		// slug, not plain lower-casing: a display name with spaces
		// would otherwise produce an identifier with spaces, which is
		// invalid as a CFBundleIdentifier and unusable as a
		// freedesktop icon key or filename.
		o.ID = "local.gogui." + slug(o.Name)
	}
	switch o.Platform {
	case "darwin":
		return buildMacOS(o)
	case "windows":
		return buildWindows(o)
	case "linux":
		return buildLinux(o)
	default:
		return fmt.Errorf("unsupported -platform %q (need darwin, windows or linux)", o.Platform)
	}
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
