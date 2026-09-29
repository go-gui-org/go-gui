// Package appinfo reads an app manifest: the one file that holds an
// app's ID, name, version, build number and icon.
//
// The file is appinfo.toml beside the app's main.go. Two programs read
// it. buildapp reads it from disk when it packages the app. The app
// reads the same bytes at run time through //go:embed and gives them to
// gui.WindowCfg.AppInfo. Both sides then use the same ID, so macOS
// preferences and permission grants stay under one identity.
//
//	# appinfo.toml
//	id      = "org.go-gui.falcon"
//	name    = "Falcon"
//	version = "1.4.0"
//	icon    = "assets/icon.png"
//
//	[darwin]
//	category = "public.app-category.developer-tools"
//
// # Grammar
//
// The format is a strict subset of TOML. Every file this package accepts
// is also valid TOML, so a TOML library can read the same files later.
//
//  1. A line is key = "value". Values are always double-quoted strings.
//     The only escapes are \" and \\.
//  2. A [section] header starts a platform block. Sections do not nest.
//  3. A # starts a comment, on its own line or after a value.
//  4. There are no arrays, no numbers, no nesting and no multi-line
//     values.
//  5. An unknown key or section is an error. The error gives the line
//     number.
//
// The parser is hand-written so that the format adds no module
// dependency (docs/dependencies.md).
package appinfo

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Info is the parsed manifest. A field that the file does not set is
// empty.
type Info struct {
	// ID is the reverse-DNS app identifier, for example
	// "org.go-gui.falcon". It is the macOS bundle ID, the Linux
	// .desktop file name and the key for per-app storage.
	ID string
	// Name is the display name: the window title, the macOS app menu
	// and the bundle name.
	Name string
	// Version is the user-facing version, for example "1.4.0".
	Version string
	// Build is the build number. Empty means the same as Version.
	Build string
	// Icon is the icon file path. A relative path is relative to the
	// manifest file.
	Icon string
	// Darwin holds the keys of the [darwin] section.
	Darwin DarwinInfo
	// Linux holds the keys of the [linux] section.
	Linux LinuxInfo
}

// DarwinInfo holds the macOS-only keys.
// exportaudit:keep — reachable from an exported field (Info.Darwin)
type DarwinInfo struct {
	// Category is the LSApplicationCategoryType, for example
	// "public.app-category.developer-tools".
	Category string
}

// LinuxInfo holds the Linux-only keys.
// exportaudit:keep — reachable from an exported field (Info.Linux)
type LinuxInfo struct {
	// Categories is the .desktop Categories value, for example
	// "Development;Utility;".
	Categories string
}

// maxSize caps the input. A manifest is a few hundred bytes; the cap
// stops a wrong file (a binary, a log) from being scanned in full.
const maxSize = 64 << 10

// Parse reads a manifest. It returns the first error it finds, with the
// line number when the error belongs to one line.
func Parse(data []byte) (Info, error) {
	var info Info
	if len(data) > maxSize {
		return info, fmt.Errorf("appinfo: file is larger than %d bytes", maxSize)
	}
	if !utf8.Valid(data) {
		return info, errors.New("appinfo: file is not valid UTF-8")
	}
	src := strings.TrimPrefix(string(data), "\xef\xbb\xbf")

	// section is "" for the top level. seen records every key already
	// set, as "section.key", and every section header, as "[section]",
	// so a second definition is an error as TOML requires.
	section := ""
	seen := map[string]bool{}
	for n := 1; src != ""; n++ {
		var line string
		var gotLF bool
		line, src, gotLF = strings.Cut(src, "\n")
		// Strip one CR, and only when an LF follows it: that pair is a
		// CRLF line end. Any other CR stays in the line and fails below,
		// because TOML allows a CR only as part of CRLF.
		if gotLF {
			line = strings.TrimSuffix(line, "\r")
		}
		line = strings.Trim(line, " \t")
		if line == "" {
			continue
		}
		if line[0] == '#' {
			if err := checkComment(line); err != nil {
				return Info{}, lineErr(n, err.Error())
			}
			continue
		}
		if line[0] == '[' {
			name, err := parseHeader(line)
			if err != nil {
				return Info{}, lineErr(n, err.Error())
			}
			if fieldsOf(&info, name) == nil {
				return Info{}, lineErr(n, fmt.Sprintf("unknown section %q", name))
			}
			if seen["["+name+"]"] {
				return Info{}, lineErr(n, fmt.Sprintf("duplicate section %q", name))
			}
			seen["["+name+"]"] = true
			section = name
			continue
		}
		key, value, err := parseKeyValue(line)
		if err != nil {
			return Info{}, lineErr(n, err.Error())
		}
		field := fieldsOf(&info, section)[key]
		if field == nil {
			if section == "" {
				return Info{}, lineErr(n, fmt.Sprintf("unknown key %q", key))
			}
			return Info{}, lineErr(n, fmt.Sprintf("unknown key %q in [%s]", key, section))
		}
		if seen[section+"."+key] {
			return Info{}, lineErr(n, fmt.Sprintf("duplicate key %q", key))
		}
		seen[section+"."+key] = true
		if key == "id" && section == "" && !validID(value) {
			return Info{}, lineErr(n, fmt.Sprintf(
				"id %q may hold only letters, digits, '.' and '-'", value))
		}
		*field = value
	}
	return info, nil
}

// MustParse is Parse for a manifest embedded with //go:embed. The file
// is part of the program, so a bad file is a programming error and
// MustParse panics.
func MustParse(data []byte) Info {
	info, err := Parse(data)
	if err != nil {
		panic(err)
	}
	return info
}

// fieldsOf maps each key of a section to the Info field it sets. It
// returns nil for an unknown section. The table is the schema: a key
// added here is a key the file accepts.
func fieldsOf(info *Info, section string) map[string]*string {
	switch section {
	case "":
		return map[string]*string{
			"id":      &info.ID,
			"name":    &info.Name,
			"version": &info.Version,
			"build":   &info.Build,
			"icon":    &info.Icon,
		}
	case "darwin":
		return map[string]*string{"category": &info.Darwin.Category}
	case "linux":
		return map[string]*string{"categories": &info.Linux.Categories}
	}
	return nil
}

// parseHeader reads a "[name]" line, with an optional trailing comment.
func parseHeader(line string) (string, error) {
	if strings.HasPrefix(line, "[[") {
		return "", errors.New("arrays of tables are not supported")
	}
	end := strings.IndexByte(line, ']')
	if end < 0 {
		return "", errors.New("section header has no closing ']'")
	}
	if err := checkComment(line[end+1:]); err != nil {
		if err == errNotComment {
			return "", errors.New("unexpected text after section header")
		}
		return "", err
	}
	name := strings.Trim(line[1:end], " \t")
	if strings.Contains(name, ".") {
		return "", fmt.Errorf("nested section %q is not supported", name)
	}
	if !isBareKey(name) {
		return "", fmt.Errorf("invalid section name %q", name)
	}
	return name, nil
}

// parseKeyValue reads a `key = "value"` line, with an optional trailing
// comment.
func parseKeyValue(line string) (key, value string, err error) {
	i := 0
	for i < len(line) && isBareKeyByte(line[i]) {
		i++
	}
	key = line[:i]
	if key == "" {
		return "", "", errors.New("line must start with a key")
	}
	rest := strings.TrimLeft(line[i:], " \t")
	if rest == "" || rest[0] != '=' {
		if rest != "" && rest[0] == '.' {
			return "", "", fmt.Errorf("dotted key %q is not supported", key+rest[:1])
		}
		return "", "", fmt.Errorf("key %q must be followed by '='", key)
	}
	rest = strings.TrimLeft(rest[1:], " \t")
	if rest == "" || rest[0] != '"' {
		return "", "", fmt.Errorf(
			"value of %q must be a double-quoted string", key)
	}
	if strings.HasPrefix(rest, `"""`) {
		return "", "", errors.New("multi-line strings are not supported")
	}
	value, rest, err = parseString(rest[1:])
	if err != nil {
		return "", "", err
	}
	if err = checkComment(rest); err != nil {
		if err == errNotComment {
			return "", "", fmt.Errorf("unexpected text after the value of %q", key)
		}
		return "", "", err
	}
	return key, value, nil
}

// parseString reads the body of a basic string after its opening quote.
// It returns the unescaped value and the text after the closing quote.
func parseString(s string) (value, rest string, err error) {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			return b.String(), s[i+1:], nil
		case c == '\\':
			if i+1 == len(s) {
				return "", "", errors.New("unterminated string")
			}
			i++
			if s[i] != '"' && s[i] != '\\' {
				return "", "", fmt.Errorf(`unsupported escape \%c (only \" and \\)`, s[i])
			}
			b.WriteByte(s[i])
		case c < 0x20 && c != '\t', c == 0x7f:
			// TOML forbids control characters in a basic string.
			return "", "", errors.New("control character in string")
		default:
			b.WriteByte(c)
		}
	}
	return "", "", errors.New("unterminated string")
}

// errNotComment is checkComment's answer for text that is not a
// comment; each caller turns it into a message that names its context.
var errNotComment = errors.New("not a comment")

// checkComment accepts s when it is blank or a comment. TOML allows no
// control character in a comment except tab, so a file that passes here
// stays valid TOML. A lone CR counts: only a trailing CR is stripped as
// part of a CRLF line end.
func checkComment(s string) error {
	s = strings.TrimLeft(s, " \t")
	if s == "" {
		return nil
	}
	if s[0] != '#' {
		return errNotComment
	}
	for i := 1; i < len(s); i++ {
		if c := s[i]; c < 0x20 && c != '\t' || c == 0x7f {
			return errors.New("control character in comment")
		}
	}
	return nil
}

// isBareKey reports whether s is a TOML bare key: A-Z a-z 0-9 _ -.
func isBareKey(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isBareKeyByte(s[i]) {
			return false
		}
	}
	return true
}

func isBareKeyByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
		c >= '0' && c <= '9' || c == '_' || c == '-'
}

// validID reports whether id is usable as a macOS CFBundleIdentifier and
// as a Linux file name: letters, digits, '.' and '-'.
func validID(id string) bool {
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c == '_' || (!isBareKeyByte(c) && c != '.') {
			return false
		}
	}
	return true
}

func lineErr(n int, msg string) error {
	return fmt.Errorf("appinfo: line %d: %s", n, msg)
}
