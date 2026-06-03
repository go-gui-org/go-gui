package gui

// markdown_math.go implements LaTeX math image fetching
// via the codecogs API.

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-gui-org/go-gui/gui/markdown"
)

// diagramCacheHash computes a cache key for a math expression.
func diagramCacheHash(mathID string) int64 {
	return int64(markdown.MathHash(mathID))
}

// blockedLatexCmds lists TeX commands blocked to prevent
// shell escape or file access on remote renderers.
var blockedLatexCmds = []string{
	`\write18`, `\input`, `\include`,
	`\openin`, `\openout`, `\read`, `\write`,
	`\csname`, `\immediate`, `\catcode`,
	`\special`, `\outer`, `\def`, `\edef`,
	`\gdef`, `\xdef`, `\let`, `\futurelet`,
	`\aliasfont`, `\batchmode`, `\copy`,
	`\count`, `\countdef`, `\dimen`, `\dimendef`,
	`\errorstopmode`, `\font`, `\fontdimen`,
	`\halign`, `\hrule`, `\hyphenation`,
	`\if`, `\ifcase`, `\ifcat`, `\ifdim`,
	`\ifeof`, `\iffalse`, `\ifhbox`, `\ifhmode`,
	`\ifinner`, `\ifmmode`, `\ifnum`, `\ifodd`,
	`\iftrue`, `\ifvbox`, `\ifvmode`, `\ifvoid`,
	`\ifx`, `\jobname`, `\kern`, `\long`,
	`\mag`, `\mark`, `\meaning`, `\messages`,
	`\newcount`, `\newdimen`, `\newif`,
	`\newread`, `\newskip`, `\newwrite`,
	`\noexpand`, `\nonstopmode`, `\output`,
	`\pausing`, `\primitive`, `\readline`,
	`\scrollmode`, `\setbox`, `\show`,
	`\showbox`, `\showlists`, `\showthe`,
	`\skip`, `\skipdef`, `\the`, `\toks`,
	`\toksdef`, `\tracingall`, `\tracingcommands`,
	`\tracinglostchars`, `\tracingmacros`,
	`\tracingonline`, `\tracingoutput`,
	`\tracingpages`, `\tracingparagraphs`,
	`\tracingrestores`, `\tracingstats`,
	`\vcenter`, `\valign`, `\vrule`,
}

// sanitizeLatex strips dangerous TeX commands that could
// enable shell escape or file access on the remote renderer.
func sanitizeLatex(s string) string {
	if len(s) > markdown.MaxLatexSourceLen {
		return ""
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	result := strings.Map(func(r rune) rune {
		switch {
		case r == '\r' || r == '\n' || r == '\t':
			return ' '
		case r < 0x20:
			return -1
		default:
			return r
		}
	}, s)
	result = strings.TrimSpace(result)
	for range 10 {
		prev := result
		for _, cmd := range blockedLatexCmds {
			result = strings.ReplaceAll(result, cmd, "")
		}
		if result == prev {
			break
		}
	}
	return result
}

// queueDiagramError queues a DiagramError cache entry.
func queueDiagramError(
	w *Window, hash int64, requestID uint64, errMsg string,
) {
	w.QueueCommand(func(w *Window) {
		if !diagramCacheShouldApplyResult(
			w.viewState.diagramCache,
			hash, requestID) {
			return
		}
		w.viewState.diagramCache.Set(hash,
			DiagramCacheEntry{
				State:     DiagramError,
				Error:     errMsg,
				RequestID: requestID,
			})
		w.UpdateWindow()
	})
}

// fetchMathAsync fetches a LaTeX math image from codecogs
// in a background goroutine.
//
// PRIVACY NOTE: LaTeX source is sent to external
// third-party API (latex.codecogs.com) for rendering.
func fetchMathAsync(
	w *Window, latex string, hash int64,
	requestID uint64, dpi int, fgColor Color,
) {
	ctx := w.Ctx()
	go func() {
		safeLatex := sanitizeLatex(latex)

		// Build codecogs URL with DPI and optional color.
		lum := 0.299*float64(fgColor.R) +
			0.587*float64(fgColor.G) +
			0.114*float64(fgColor.B)
		colorCmd := ""
		if lum > 128.0 {
			colorCmd = `\color{white}`
		}
		prefix := fmt.Sprintf(`\dpi{%d}%s`, dpi, colorCmd)

		encoded := strings.ReplaceAll(
			prefix+safeLatex, " ", "{}")
		encoded = strings.ReplaceAll(encoded, "#", "%23")
		encoded = strings.ReplaceAll(encoded, "&", "%26")
		reqURL := "https://latex.codecogs.com/png.image?" +
			encoded

		req, err := http.NewRequestWithContext(
			ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			queueDiagramError(w, hash, requestID, err.Error())
			return
		}
		client := &http.Client{Timeout: diagramFetchTimeout}
		resp, err := client.Do(req)
		if err != nil {
			queueDiagramError(w, hash, requestID, err.Error())
			return
		}
		defer func() { _ = resp.Body.Close() }()

		body, err := io.ReadAll(
			io.LimitReader(resp.Body, maxDiagramResponseBytes+1))
		if err != nil {
			queueDiagramError(w, hash, requestID,
				"read body: "+err.Error())
			return
		}

		if resp.StatusCode != 200 {
			preview := truncatePreview(string(body), 200)
			queueDiagramError(w, hash, requestID,
				fmt.Sprintf("HTTP %d: %s",
					resp.StatusCode, preview))
			return
		}

		if len(body) > maxDiagramResponseBytes {
			queueDiagramError(w, hash, requestID,
				"response exceeds 10 MB limit")
			return
		}

		finishDiagramFetch(
			w, body, hash, requestID, float32(dpi), "math")
	}()
}
