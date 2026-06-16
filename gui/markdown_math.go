package gui

// markdown_math.go implements LaTeX math image fetching
// via the codecogs API.

import (
	"context"
	"errors"
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

// defaultMathFetcher renders LaTeX via the CodeCogs API.
// latex must already be sanitized (caller responsibility).
func defaultMathFetcher(
	ctx context.Context, latex string, dpi int, fgColor Color,
) ([]byte, error) {
	// Clamp DPI to a reasonable range. Values outside this
	// can produce enormous or invisible images on the renderer.
	if dpi < 24 {
		dpi = 24
	} else if dpi > 1200 {
		dpi = 1200
	}

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
		prefix+latex, " ", "{}")
	encoded = strings.ReplaceAll(encoded, "#", "%23")
	encoded = strings.ReplaceAll(encoded, "&", "%26")
	reqURL := "https://latex.codecogs.com/png.image?" +
		encoded

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := diagramHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(
		io.LimitReader(resp.Body, maxDiagramResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode != 200 {
		preview := truncatePreview(string(body), 200)
		return nil, fmt.Errorf("HTTP %d: %s",
			resp.StatusCode, preview)
	}

	if len(body) > maxDiagramResponseBytes {
		return nil, errors.New("response exceeds 10 MB limit")
	}
	return body, nil
}

// fetchMathAsync fetches a LaTeX math image in a background
// goroutine. Uses cfg.MathFetcher when non-nil, otherwise
// defaults to the CodeCogs API.
//
// PRIVACY NOTE: LaTeX source may be sent to external
// third-party API (latex.codecogs.com) for rendering.
func fetchMathAsync(
	w *Window, latex string, hash int64,
	requestID uint64, dpi int, fgColor Color,
	fetcher MathFetcher,
) {
	actualFetcher := fetcher
	if actualFetcher == nil {
		actualFetcher = defaultMathFetcher
	}
	ctx := w.Ctx()
	go func() {
		safe := sanitizeLatex(latex)
		if safe == "" {
			queueDiagramError(w, hash, requestID,
				"empty or invalid LaTeX")
			return
		}
		body, err := actualFetcher(ctx, safe, dpi, fgColor)
		if err != nil {
			queueDiagramError(w, hash, requestID, err.Error())
			return
		}
		finishDiagramFetch(
			w, body, hash, requestID, float32(dpi), "math")
	}()
}
