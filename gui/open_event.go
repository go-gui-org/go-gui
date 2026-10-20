package gui

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// OpenEvent is a request from outside the app to open documents or
// URLs: a file opened from Finder or Explorer, a custom-scheme link
// (myapp://…), or the arguments of a second launch that a
// single-instance lock forwarded to this process (gui/instance).
// App.OnOpen receives it on the main thread.
type OpenEvent struct {
	// Files holds absolute file paths, in arrival order.
	Files []string
	// URLs holds non-file URLs, in arrival order, as received.
	URLs []string
}

// empty reports whether the event carries nothing to open.
func (e OpenEvent) empty() bool {
	return len(e.Files) == 0 && len(e.URLs) == 0
}

// ParseOpenArgs sorts command-line arguments into files and URLs.
// Pass os.Args[1:]. The rules:
//
//   - An argument that starts with "-" is a flag and is skipped, until
//     a "--" argument. After "--", every argument is a path or a URL.
//     The skip also drops the "-psn_…" argument old macOS versions
//     add to a Finder launch.
//   - An argument with a scheme of two or more letters is a URL. A
//     "file:" URL becomes its path; one with a host other than
//     "localhost" is a UNC share, kept on Windows and skipped
//     elsewhere. A scheme with no "//" ("mailto:x") is a URL only
//     when no file of that name exists, so "notes:v2.txt", a legal
//     Unix file name, stays a file.
//   - Anything else is a file path, made absolute against the current
//     directory. "C:\…" and "C:/…" are Windows paths, not URLs with
//     scheme "c".
//
// exportaudit:keep — app-facing: an app without gui/instance feeds its own
// os.Args to App.DispatchOpen through it
func ParseOpenArgs(args []string) OpenEvent {
	var ev OpenEvent
	flags := true
	for _, arg := range args {
		if arg == "" {
			continue
		}
		if flags && arg == "--" {
			flags = false
			continue
		}
		if flags && arg[0] == '-' {
			continue
		}
		if u, ok := parseSchemeURL(arg); ok {
			if strings.EqualFold(u.Scheme, "file") {
				if p := fileURLPath(u); p != "" {
					ev.Files = append(ev.Files, p)
				}
				continue
			}
			ev.URLs = append(ev.URLs, arg)
			continue
		}
		ev.Files = append(ev.Files, absOpenPath(arg))
	}
	return ev
}

// parseSchemeURL returns arg as a URL when it has a real scheme. A
// one-letter scheme is a Windows drive letter ("C:\x" parses as scheme
// "c"), so it does not count.
func parseSchemeURL(arg string) (*url.URL, bool) {
	if isDrivePath(arg) {
		return nil, false
	}
	u, err := url.Parse(arg)
	if err != nil || len(u.Scheme) < 2 {
		return nil, false
	}
	if u.Opaque != "" {
		// No "//" after the scheme: "mailto:x", or a relative file
		// name with a colon. A file that exists wins.
		if _, statErr := os.Stat(arg); statErr == nil {
			return nil, false
		}
	}
	return u, true
}

// isDrivePath reports whether s starts with a Windows drive letter
// followed by ':' and a separator, or is a bare "C:".
func isDrivePath(s string) bool {
	if len(s) < 2 || s[1] != ':' {
		return false
	}
	c := s[0] | 0x20 // ASCII lower-case
	if c < 'a' || c > 'z' {
		return false
	}
	return len(s) == 2 || s[2] == '\\' || s[2] == '/'
}

// fileURLPath converts a file: URL to a host path. On Windows the URL
// path "/C:/x" loses its leading slash before conversion, and a host
// makes a UNC path (\\host\share\x). Elsewhere a host names no local
// file, so the result is "" and the caller skips the argument: dropping
// the host would point at a different file.
func fileURLPath(u *url.URL) string {
	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		if filepath.Separator != '\\' {
			return ""
		}
		return `\\` + u.Host + filepath.FromSlash(u.Path)
	}
	p := u.Path
	if len(p) >= 3 && p[0] == '/' && isDrivePath(p[1:]) {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}

// absOpenPath makes a path absolute. A Windows drive path is kept as
// given on every OS: on Unix filepath.IsAbs reports false for it, and
// joining it to the working directory would make a path that never
// existed.
func absOpenPath(p string) string {
	if isDrivePath(p) || filepath.IsAbs(p) {
		return p
	}
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}
