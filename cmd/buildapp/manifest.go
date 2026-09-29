package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/go-gui-org/go-gui/gui/appinfo"
)

// defaultManifest is the file buildapp reads from the working directory
// when -manifest is not given. Makefiles run from the repo root, which
// is where an app keeps it, so most invocations need no flag.
const defaultManifest = "appinfo.toml"

// loadManifest reads the app manifest and returns it with the directory
// it came from; a relative icon path in the file is relative to that
// directory. An empty path means defaultManifest in the working
// directory, and a missing default file is not an error: it returns the
// zero Info, so an invocation that predates the manifest is unchanged.
// A path given with -manifest must exist.
func loadManifest(path string) (appinfo.Info, string, error) {
	explicit := path != ""
	if !explicit {
		path = defaultManifest
	}
	data, err := os.ReadFile(path) // #nosec G304 — CLI flag or fixed name
	if err != nil {
		if !explicit && errors.Is(err, fs.ErrNotExist) {
			return appinfo.Info{}, "", nil
		}
		return appinfo.Info{}, "", err
	}
	info, err := appinfo.Parse(data)
	if err != nil {
		// appinfo errors carry the line; add the file so the message
		// points somewhere a developer can open.
		return appinfo.Info{}, "", fmt.Errorf("%s: %w", path, err)
	}
	return info, filepath.Dir(path), nil
}

// applyManifest copies manifest values into o. set holds the names of
// the flags given on the command line: those win, so a release script
// can still pass -version "$(git describe --tags)". A flag left at its
// default loses to the file, which is why the -version default "1.0"
// never hides the manifest's version. A key the file leaves empty
// changes nothing.
func applyManifest(o *bundleOpts, info appinfo.Info, dir string, set map[string]bool) {
	fill := func(flag string, dst *string, v string) {
		if v != "" && !set[flag] {
			*dst = v
		}
	}
	icon := info.Icon
	if icon != "" && !filepath.IsAbs(icon) {
		icon = filepath.Join(dir, icon)
	}
	fill("name", &o.Name, info.Name)
	fill("id", &o.ID, info.ID)
	fill("version", &o.Version, info.Version)
	fill("build", &o.Build, info.Build)
	fill("icon", &o.Icon, icon)
	// Platform keys have no flag, so the file always supplies them.
	fill("", &o.Category, info.Darwin.Category)
	fill("", &o.Categories, info.Linux.Categories)
}
