package gui

// markdown_math.go implements LaTeX math image fetching
// via the codecogs API.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-gui-org/go-gui/gui/markdown"
)

// diagramCacheHash computes a cache key for a math expression.
// The uint64 digest is reinterpreted as int64 for the diagram
// cache map key; no bits change, only the sign reads differently
// in logs.
func diagramCacheHash(mathID string) int64 {
	return int64(markdown.MathHash(mathID))
}

// allowedLatexCmds is the allowlist of safe LaTeX control words
// for math rendering. Only parsing, spacing, math symbols and
// common amsmath commands are listed: anything that can read files
// or escape the shell (\input, \write, \def, \csname and friends)
// is absent, so it is dropped by the sanitizer below. Parsing is by
// whole token, so allowing "longrightarrow" can never match a
// prefix of it the way a substring blocklist did (\the inside
// \theta corrupted every theta before).
//
// Unknown commands fail closed: the control word is dropped and the
// rest (arguments, text) stays. An exotic but harmless macro renders
// as its arguments rather than executing. Add entries here when a
// legitimate formula needs them.
var allowedLatexCmds = map[string]struct{}{
	// Greek and Hebrew letters.
	"alpha": {}, "beta": {}, "gamma": {}, "delta": {},
	"epsilon": {}, "varepsilon": {}, "zeta": {}, "eta": {},
	"theta": {}, "vartheta": {}, "iota": {}, "kappa": {},
	"lambda": {}, "mu": {}, "nu": {}, "xi": {}, "pi": {},
	"varpi": {}, "rho": {}, "varrho": {}, "sigma": {},
	"varsigma": {}, "tau": {}, "upsilon": {}, "phi": {},
	"varphi": {}, "chi": {}, "psi": {}, "omega": {},
	"digamma": {}, "varkappa": {},
	"Gamma": {}, "Delta": {}, "Theta": {}, "Lambda": {},
	"Xi": {}, "Pi": {}, "Sigma": {}, "Upsilon": {},
	"Phi": {}, "Psi": {}, "Omega": {},
	"aleph": {}, "beth": {}, "gimel": {}, "daleth": {},
	// Fractions, roots and big operators.
	"frac": {}, "dfrac": {}, "tfrac": {}, "cfrac": {},
	"sqrt": {}, "sum": {}, "prod": {}, "coprod": {},
	"int": {}, "oint": {}, "iint": {}, "iiint": {},
	"lim": {}, "liminf": {}, "limsup": {},
	"inf": {}, "sup": {}, "max": {}, "min": {},
	"arg": {}, "det": {}, "dim": {}, "exp": {}, "gcd": {},
	"hom": {}, "ker": {}, "lg": {}, "ln": {}, "log": {},
	"deg": {}, "mod": {}, "bmod": {}, "pmod": {},
	// Plain-TeX infix fractions.
	"over": {}, "atop": {}, "choose": {},
	// Binomials and stacks.
	"binom": {}, "dbinom": {}, "tbinom": {},
	"substack": {}, "sideset": {},
	"overset": {}, "underset": {}, "stackrel": {},
	"begin": {}, "end": {},
	// Trig and hyperbolic functions.
	"sin": {}, "cos": {}, "tan": {}, "sec": {},
	"csc": {}, "cot": {}, "arcsin": {}, "arccos": {},
	"arctan": {}, "sinh": {}, "cosh": {}, "tanh": {},
	"coth": {},
	// Relations and binary operators.
	"leq": {}, "geq": {}, "le": {}, "ge": {}, "ll": {},
	"gg": {}, "prec": {}, "succ": {}, "preceq": {},
	"succeq": {}, "sim": {}, "approx": {}, "simeq": {},
	"cong": {}, "equiv": {}, "propto": {}, "asymp": {},
	"bowtie": {}, "doteq": {}, "coloneq": {}, "coloneqq": {},
	"models": {}, "perp": {}, "mid": {}, "nmid": {},
	"parallel": {}, "nparallel": {}, "ne": {}, "neq": {},
	"notin": {}, "not": {}, "in": {}, "ni": {},
	"subset": {}, "supset": {}, "subseteq": {},
	"supseteq": {}, "nsubseteq": {}, "nsupseteq": {},
	"sqsubset": {}, "sqsupset": {}, "sqsubseteq": {},
	"sqsupseteq": {},
	"times":      {}, "div": {}, "cdot": {}, "ast": {},
	"star": {}, "circ": {}, "bullet": {}, "pm": {},
	"mp": {}, "cap": {}, "cup": {}, "Cap": {}, "Cup": {},
	"setminus": {}, "wr": {}, "diamond": {}, "Diamond": {},
	"box": {}, "Box": {}, "square": {}, "blacksquare": {},
	"triangle": {}, "triangledown": {}, "triangleleft": {},
	"triangleright": {}, "circledast": {}, "centerdot": {},
	"dotplus": {}, "divideontimes": {}, "boxminus": {},
	"boxtimes": {}, "boxdot": {}, "boxplus": {},
	"land": {}, "lor": {}, "lnot": {}, "neg": {},
	"wedge": {}, "vee": {}, "forall": {}, "exists": {},
	"emptyset": {}, "varnothing": {},
	// Arrows.
	"leftarrow": {}, "rightarrow": {}, "uparrow": {},
	"downarrow": {}, "Leftarrow": {}, "Rightarrow": {},
	"Uparrow": {}, "Downarrow": {}, "leftrightarrow": {},
	"Leftrightarrow": {}, "updownarrow": {},
	"Updownarrow": {}, "mapsto": {}, "longleftarrow": {},
	"longrightarrow": {}, "longleftrightarrow": {},
	"Longleftarrow": {}, "Longrightarrow": {},
	"hookleftarrow": {}, "hookrightarrow": {},
	"xleftarrow": {}, "xrightarrow": {},
	"underleftarrow": {}, "underrightarrow": {},
	"to": {}, "gets": {}, "implies": {}, "impliedby": {},
	"iff": {}, "nearrow": {}, "searrow": {},
	"swarrow": {}, "nwarrow": {}, "leadsto": {},
	// Delimiters and sizes.
	"left": {}, "right": {}, "big": {}, "Big": {},
	"bigg": {}, "Bigg": {}, "bigl": {}, "Bigl": {},
	"bigr": {}, "Bigr": {}, "biggl": {}, "Biggl": {},
	"biggr": {}, "Biggr": {}, "bigm": {}, "Bigm": {},
	"langle": {}, "rangle": {}, "lceil": {}, "rceil": {},
	"lfloor": {}, "rfloor": {}, "lvert": {}, "rvert": {},
	"vert": {}, "Vert": {}, "backslash": {},
	// Accents and styles.
	"hat": {}, "check": {}, "breve": {}, "acute": {},
	"grave": {}, "tilde": {}, "widetilde": {},
	"widehat": {}, "bar": {}, "overline": {},
	"underline": {}, "overbrace": {}, "underbrace": {},
	"dot": {}, "ddot": {}, "dddot": {}, "ddddot": {},
	"vec": {}, "mathbf": {}, "mathit": {}, "mathrm": {},
	"mathsf": {}, "mathtt": {}, "mathcal": {}, "mathbb": {},
	"mathfrak": {}, "mathscr": {}, "boldsymbol": {},
	"operatorname": {}, "text": {}, "mbox": {},
	"hbox": {}, "fbox": {}, "boxed": {}, "raisebox": {},
	"phantom": {}, "vphantom": {}, "hphantom": {},
	"smash":        {},
	"displaystyle": {}, "textstyle": {},
	"scriptstyle": {}, "scriptscriptstyle": {},
	"limits": {}, "nolimits": {},
	// Spacing and layout helpers without file or shell access.
	"quad": {}, "qquad": {}, "hspace": {}, "vspace": {},
	"mkern": {}, "mskip": {}, "newline": {}, "linebreak": {},
	"smallskip": {}, "medskip": {}, "bigskip": {},
	// Dots and misc symbols.
	"ldots": {}, "cdots": {}, "vdots": {}, "ddots": {},
	"iddots": {}, "dots": {}, "dotsc": {}, "dotsb": {},
	"dotsm": {}, "dotsi": {}, "dotso": {},
	"partial": {}, "nabla": {}, "infty": {},
	"prime": {}, "backprime": {}, "dagger": {},
	"ddagger": {}, "ell": {}, "hbar": {}, "imath": {},
	"jmath": {}, "wp": {}, "Re": {}, "Im": {},
	"angle": {}, "measuredangle": {}, "sphericalangle": {},
	"therefore": {}, "because": {}, "flat": {},
	"natural": {}, "sharp": {}, "clubsuit": {},
	"heartsuit": {}, "spadesuit": {}, "surd": {},
	"top": {}, "bot": {}, "vdash": {}, "dashv": {},
	// Table rules inside arrays.
	"hline": {}, "cline": {}, "multicolumn": {},
	// Equation numbering and color (own formula only).
	"tag": {}, "notag": {}, "nonumber": {}, "label": {},
	"ref": {}, "eqref": {}, "color": {},
	// Text letters and symbols with no primitive meaning.
	"aa": {}, "AA": {}, "ae": {}, "AE": {}, "oe": {},
	"OE": {}, "o": {}, "O": {}, "l": {}, "L": {},
	"ss": {}, "i": {}, "j": {}, "S": {}, "P": {},
	"dag": {}, "ddag": {},
}

// allowedLatexEnvs is the allowlist of environment names that may
// follow \begin or \end. The command allowlist alone is not enough:
// \begin is a legitimate math command, but the environment it opens
// picks the code that runs, and environments such as filecontents
// write files on the renderer. A starred form (align*) is accepted
// by stripping the star before the lookup.
var allowedLatexEnvs = map[string]struct{}{
	"matrix": {}, "pmatrix": {}, "bmatrix": {}, "Bmatrix": {},
	"vmatrix": {}, "Vmatrix": {}, "smallmatrix": {},
	"array": {}, "subarray": {}, "cases": {}, "dcases": {},
	"rcases": {}, "aligned": {}, "alignedat": {}, "gathered": {},
	"split": {}, "align": {}, "alignat": {}, "flalign": {},
	"gather": {}, "multline": {}, "equation": {}, "eqnarray": {},
	"displaymath": {}, "math": {},
}

// latexEnvArg reads the {name} argument that follows \begin or \end
// at position i (leading spaces allowed) and reports the end offset
// and whether the environment is on the allowlist. ok is false when
// no braced argument follows, in which case the caller keeps the
// command word alone and the text is left to the renderer.
func latexEnvArg(s string, i int) (end int, allowed, ok bool) {
	j := i
	for j < len(s) && s[j] == ' ' {
		j++
	}
	if j >= len(s) || s[j] != '{' {
		return i, false, false
	}
	k := j + 1
	for k < len(s) && s[k] != '}' && s[k] != '\\' {
		k++
	}
	if k >= len(s) || s[k] != '}' {
		return i, false, false
	}
	name := strings.TrimSuffix(s[j+1:k], "*")
	_, allowed = allowedLatexEnvs[name]
	return k + 1, allowed, true
}

// sanitizeLatex strips dangerous TeX commands that could
// enable shell escape or file access on the remote renderer.
// Allowlist, fail-closed: a control word on allowedLatexCmds stays,
// any other control word is dropped (its arguments and surrounding
// text stay). A single pass tokenizes, so stripping can neither
// expose a nested command nor match inside a longer name.
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
	return stripDisallowedLatexCmds(result)
}

// stripDisallowedLatexCmds drops every control word not on the
// allowlist. A control word is a backslash followed by ASCII letters
// (TeX parses the maximal run, so \theta is one token and never
// contains \the). A backslash followed by anything else is a control
// symbol — an escape like \%, \{, \\ or an accent like \' — which
// cannot name a primitive, so it stays verbatim.
func stripDisallowedLatexCmds(s string) string {
	if strings.IndexByte(s, '\\') < 0 {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	i := 0
	for i < len(s) {
		c := s[i]
		if c != '\\' {
			b.WriteByte(c)
			i++
			continue
		}
		if i+1 >= len(s) {
			b.WriteByte(c)
			break
		}
		// A non-letter next byte is a control symbol, not a
		// command: an escape like \%, \{ or \\, an accent like
		// \', or the lead byte of a non-ASCII rune. None can
		// name a primitive, so both bytes stay verbatim.
		n := s[i+1]
		if (n < 'A' || n > 'Z') && (n < 'a' || n > 'z') {
			b.WriteByte(c)
			b.WriteByte(n)
			i += 2
			continue
		}
		j := i + 1
		for j < len(s) &&
			((s[j] >= 'A' && s[j] <= 'Z') ||
				(s[j] >= 'a' && s[j] <= 'z')) {
			j++
		}
		word := s[i+1 : j]
		if _, ok := allowedLatexCmds[word]; !ok {
			i = j
			continue
		}
		if word == "begin" || word == "end" {
			// Validate the environment name too, then copy or
			// drop the command and its argument together.
			end, allowed, ok := latexEnvArg(s, j)
			if ok {
				if allowed {
					b.WriteString(s[i:end])
				}
				i = end
				continue
			}
		}
		b.WriteString(s[i:j])
		i = j
	}
	return b.String()
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
				State:     diagramError,
				Error:     errMsg,
				RequestID: requestID,
			})
		w.InvalidateLayout()
	})
}

// defaultMathFetcher renders LaTeX via the CodeCogs API.
// latex must already be sanitized (caller responsibility).
func defaultMathFetcher(
	ctx context.Context, latex string, dpi int, fgColor Color,
) ([]byte, error) {
	// Defense-in-depth: the async caller sanitizes and caps
	// this, but clamp here so a direct call cannot pass an
	// unbounded payload into the URL builder below.
	if len(latex) > markdown.MaxLatexSourceLen {
		return nil, errors.New("latex source too large")
	}
	if strings.TrimSpace(latex) == "" {
		return nil, errors.New("empty latex source")
	}
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

	// Encode the user formula for the URL query. Escape only
	// the formula: the prefix holds literal codecogs commands
	// that must stay raw. QueryEscape turns spaces into "+",
	// which codecogs does not read as a space, so restore the
	// "{}" space form after encoding. Full encoding also keeps
	// reserved characters ("%", "+", "?", "=" and more) in the
	// formula from changing the request itself.
	escaped := url.QueryEscape(latex)
	escaped = strings.ReplaceAll(escaped, "+", "{}")
	// The prefix stays raw on purpose: \dpi and \color are
	// codecogs directives, not user data, and the endpoint reads
	// the raw query (no key) as the formula. Only the user
	// formula above is encoded.
	reqURL := "https://latex.codecogs.com/png.image?" +
		prefix + escaped

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := getDiagramHTTPClient().Do(req)
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
