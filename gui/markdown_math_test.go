package gui

import (
	"strings"
	"testing"
)

func TestSanitizeLatexNormal(t *testing.T) {
	got := sanitizeLatex(`x^2 + y^2 = z^2`)
	if got != `x^2 + y^2 = z^2` {
		t.Errorf("got %q", got)
	}
}

func TestSanitizeLatexBlockedCommands(t *testing.T) {
	tests := []struct {
		input string
		clean string
	}{
		{`\input{file}`, `{file}`},
		{`\write18{cmd}`, `18{cmd}`},
		{`\include{f}`, `{f}`},
		{`\def\x{y}`, `{y}`},
		{`\immediate\write{x}`, `{x}`},
		{`\csname input\endcsname`, ` input`},
		{`\mycommand{x}`, `{x}`},
		{`\usepackage{amsmath}x`, `{amsmath}x`},
	}
	for _, tt := range tests {
		got := sanitizeLatex(tt.input)
		if got != tt.clean {
			t.Errorf("sanitizeLatex(%q) = %q, want %q",
				tt.input, got, tt.clean)
		}
	}
}

func TestSanitizeLatexAllowlistKeepsMath(t *testing.T) {
	// Former substring blocklist corrupted these: \theta
	// contains \the, \iff contains \if, \longrightarrow contains
	// \long, \coprod contains \copy. Token parsing keeps them.
	inputs := []string{
		`\theta \Theta \vartheta`,
		`x \iff y`,
		`a \longrightarrow b \longleftarrow c`,
		`\coprod_{i} X_i`,
		`\frac{1}{2} + \sqrt{x} \in [0, \pi]`,
		`\sum_{i=1}^{n} i = \frac{n(n+1)}{2}`,
		`\% \{ \} \\ \, \; \: \!`,
		`\'{e} \"{o} \~{n}`,
		`a \choose b`,
		`\begin{matrix} a \\ b \end{matrix}`,
	}
	for _, in := range inputs {
		if got := sanitizeLatex(in); got != in {
			t.Errorf("sanitizeLatex(%q) = %q, want passthrough",
				in, got)
		}
	}
}

func TestSanitizeLatexTruncation(t *testing.T) {
	long := strings.Repeat("x", 2001)
	got := sanitizeLatex(long)
	if got != "" {
		t.Error("expected empty string for input exceeding MaxLatexSourceLen")
	}
}

func TestSanitizeLatexAtLimit(t *testing.T) {
	exact := strings.Repeat("x", 2000)
	got := sanitizeLatex(exact)
	if got != exact {
		t.Error("input at exactly MaxLatexSourceLen should pass through")
	}
}

func TestSanitizeLatexControlChars(t *testing.T) {
	// Control chars < 0x20 (except \r, \n, \t) should be stripped.
	input := "a\x01b\x02c"
	got := sanitizeLatex(input)
	if got != "abc" {
		t.Errorf("got %q, want %q", got, "abc")
	}
}

func TestSanitizeLatexWhitespaceNormalization(t *testing.T) {
	// \r\n, \r, \n, \t all become spaces.
	input := "a\r\nb\tc\nd"
	got := sanitizeLatex(input)
	if got != "a b c d" {
		t.Errorf("got %q, want %q", got, "a b c d")
	}
}

func TestSanitizeLatexTrimsWhitespace(t *testing.T) {
	got := sanitizeLatex("  x + y  ")
	if got != "x + y" {
		t.Errorf("got %q, want %q", got, "x + y")
	}
}

func TestSanitizeLatexNestedBlockedCommands(t *testing.T) {
	// Single-pass tokenizing drops each disallowed word where it
	// stands; no re-scan pass is needed to expose nesting.
	input := `\input\write{x}`
	got := sanitizeLatex(input)
	if got != "{x}" {
		t.Errorf("got %q, want %q", got, "{x}")
	}
}

func TestSanitizeLatexEmpty(t *testing.T) {
	got := sanitizeLatex("")
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestSanitizeLatexEnvironmentAllowlist(t *testing.T) {
	// \begin is a legitimate math command, so the environment
	// name needs its own allowlist: filecontents writes files on
	// the renderer, verbatim escapes math mode.
	tests := []struct {
		input string
		clean string
	}{
		{`\begin{pmatrix} a \end{pmatrix}`,
			`\begin{pmatrix} a \end{pmatrix}`},
		{`\begin{align*} x \end{align*}`,
			`\begin{align*} x \end{align*}`},
		{`\begin{filecontents}{/tmp/x} y \end{filecontents}`,
			`{/tmp/x} y `},
		{`\begin{verbatim} z \end{verbatim}`, ` z `},
		// No braced argument: the bare command word survives and
		// the renderer rejects it.
		{`\begin x`, `\begin x`},
		// Unterminated argument is left to the renderer too.
		{`\begin{matrix`, `\begin{matrix`},
	}
	for _, tt := range tests {
		if got := sanitizeLatex(tt.input); got != tt.clean {
			t.Errorf("sanitizeLatex(%q) = %q, want %q",
				tt.input, got, tt.clean)
		}
	}
}

func TestSanitizeLatexTrailingBackslash(t *testing.T) {
	// A lone trailing backslash has no control word to parse and
	// must not index past the end.
	if got := sanitizeLatex(`x \`); got != `x \` {
		t.Errorf("sanitizeLatex(%q) = %q, want passthrough", `x \`, got)
	}
	if got := sanitizeLatex(`\`); got != `\` {
		t.Errorf("sanitizeLatex(%q) = %q, want passthrough", `\`, got)
	}
}
