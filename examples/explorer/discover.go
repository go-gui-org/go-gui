package main

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ExampleMeta is the explorer's view of one example folder.
type ExampleMeta struct {
	Name           string   // directory name, e.g. "calculator"
	Title          string   // from H1
	Description    string   // from Description: line or first paragraph
	Framework      string   // from Framework: line
	Tags           []string // from <!-- explorer: tags=... -->
	Category       string   // from explorer comment category=
	ScreenshotPath string   // absolute or repo-relative path to screenshot
	ReadmePath     string   // absolute path to README.md
	Runnable       bool     // false for android_demo, ios_demo, web_demo
	HasReadme      bool
	HasScreenshot  bool
}

// Discover scans root (e.g. "examples") and returns sorted ExampleMeta.
// It skips "bin" and any non-directory entry.
func Discover(root string) ([]ExampleMeta, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var out []ExampleMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "bin" || name == "explorer" || strings.HasPrefix(name, ".") {
			continue
		}
		dir := filepath.Join(root, name)
		meta := ExampleMeta{
			Name:       name,
			Title:      titleFromName(name),
			Runnable:   isRunnable(name),
			ReadmePath: filepath.Join(dir, "README.md"),
		}
		if info, err := os.Stat(meta.ReadmePath); err == nil && !info.IsDir() {
			meta.HasReadme = true
			t, d, fw, tags, cat, _ := parseReadme(meta.ReadmePath)
			if t != "" {
				meta.Title = t
			}
			if d != "" {
				meta.Description = d
			}
			if fw != "" {
				meta.Framework = fw
			}
			if len(tags) > 0 {
				meta.Tags = tags
			}
			if cat != "" {
				meta.Category = cat
			}
		}
		// Probe screenshot candidates.
		if p := probeScreenshot(dir); p != "" {
			meta.ScreenshotPath = p
			meta.HasScreenshot = true
		}
		out = append(out, meta)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func isRunnable(name string) bool {
	switch name {
	case "android_demo", "ios_demo", "web_demo":
		return false
	default:
		return true
	}
}

func titleFromName(name string) string {
	// "svg_css_vars" -> "Svg Css Vars"
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '_' || r == '-' })
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// probeScreenshot checks candidates in priority order.
func probeScreenshot(dir string) string {
	candidates := []string{
		"screenshot.png",
		"preview.png",
		"screenshot1.png",
		"icon.png",
	}
	for _, c := range candidates {
		p := filepath.Join(dir, c)
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	// Fallback: any *.png
	matches, _ := filepath.Glob(filepath.Join(dir, "*.png"))
	for _, m := range matches {
		if info, err := os.Stat(m); err == nil && !info.IsDir() {
			return m
		}
	}
	return ""
}

// parseReadme extracts explorer block fields. It stops at first "---" rule.
func parseReadme(path string) (title, desc, framework string, tags []string, category, imgPath string) {
	f, err := os.Open(path) // #nosec G304 -- path comes from Discover
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	var firstPara string
	var paraDone bool

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Stop at horizontal rule — explorer block ends there.
		if trimmed == "---" || trimmed == "***" || trimmed == "___" {
			break
		}

		// Title: first H1
		if title == "" && strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
			continue
		}

		// Framework: > **Framework:** ...
		if after, ok := strings.CutPrefix(trimmed, "> **Framework:**"); ok {
			framework = strings.TrimSpace(after)
			continue
		}
		if after, ok := strings.CutPrefix(trimmed, "> **Framework**:"); ok {
			framework = strings.TrimSpace(after)
			continue
		}

		// Description: > **Description:** ...
		if after, ok := strings.CutPrefix(trimmed, "> **Description:**"); ok {
			desc = strings.TrimSpace(after)
			continue
		}
		if after, ok := strings.CutPrefix(trimmed, "> **Description**:"); ok {
			desc = strings.TrimSpace(after)
			continue
		}

		// Explorer comment: <!-- explorer: tags=... category=... run=... -->
		if strings.Contains(trimmed, "<!-- explorer:") {
			// Extract inside the comment.
			_, after, _ := strings.Cut(trimmed, "<!-- explorer:")
			rest := after
			if end := strings.Index(rest, "-->"); end >= 0 {
				rest = rest[:end]
			}
			tags, category = parseExplorerComment(rest)
			continue
		}

		// Image: ![...](path)
		if imgPath == "" && strings.Contains(trimmed, "![") {
			if p := extractImagePath(trimmed); p != "" {
				imgPath = p
			}
		}

		// Fallback description: first non-empty paragraph after H1,
		// before any blockquote or image.
		if title != "" && desc == "" && !paraDone {
			if trimmed == "" || strings.HasPrefix(trimmed, ">") || strings.HasPrefix(trimmed, "![") || strings.HasPrefix(trimmed, "<!--") || strings.HasPrefix(trimmed, "#") {
				continue
			}
			if firstPara == "" {
				firstPara = trimmed
			} else {
				firstPara += " " + trimmed
			}
			// Consider paragraph done when we hit empty line next iteration;
			// for simplicity, take first line as fallback.
			// We keep accumulating until blank, but cap.
			if len(firstPara) > 200 {
				paraDone = true
				desc = firstPara
			}
		}
	}
	if desc == "" && firstPara != "" {
		desc = firstPara
	}
	return title, desc, framework, tags, category, imgPath
}

func parseExplorerComment(s string) (tags []string, category string) {
	// s like " tags=layout,animation category=graphics run=go"
	fields := strings.FieldsSeq(s)
	for f := range fields {
		if after, ok := strings.CutPrefix(f, "tags="); ok {
			raw := after
			for t := range strings.SplitSeq(raw, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		} else if after, ok := strings.CutPrefix(f, "category="); ok {
			category = after
		}
	}
	return tags, category
}

func extractImagePath(line string) string {
	// Find ![...](path)
	_, after, ok := strings.Cut(line, "](")
	if !ok {
		return ""
	}
	rest := after
	before0, _, ok0 := strings.Cut(rest, ")")
	if !ok0 {
		return ""
	}
	p := strings.TrimSpace(before0)
	// Strip title after space
	if idx := strings.Index(p, " "); idx >= 0 {
		p = p[:idx]
	}
	p = strings.Trim(p, "\"'")
	return p
}
