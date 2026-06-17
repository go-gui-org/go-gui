package highlight

import (
	"strings"
	"testing"

	"github.com/alecthomas/chroma/v2"
)

func TestTokenizeGoKeyword(t *testing.T) {
	t.Parallel()
	src := "package main\n\nfunc main() {}\n"
	toks := Default().Tokenize("go", src)
	if len(toks) == 0 {
		t.Fatal("no tokens")
	}
	var sawKeyword, sawFunc bool
	for _, tk := range toks {
		if tk.Kind == KindKeyword && (tk.Text == "package" || tk.Text == "func") {
			sawKeyword = true
		}
		if tk.Kind == KindFunction && tk.Text == "main" {
			sawFunc = true
		}
	}
	if !sawKeyword {
		t.Error("expected keyword token")
	}
	if !sawFunc {
		t.Error("expected function token")
	}
}

func TestTokenizeUnknownLang(t *testing.T) {
	t.Parallel()
	toks := Default().Tokenize("", "hello")
	if len(toks) != 1 || toks[0].Kind != KindPlain || toks[0].Text != "hello" {
		t.Errorf("unexpected fallback: %+v", toks)
	}
	toks = Default().Tokenize("fortran", "hello")
	if len(toks) != 1 || toks[0].Kind != KindPlain {
		t.Errorf("expected plain fallback for uncurated lang, got %+v", toks)
	}
}

func TestTokenize_EmptySrc_ReturnsNil(t *testing.T) {
	t.Parallel()
	if toks := Default().Tokenize("go", ""); toks != nil {
		t.Errorf("empty src: want nil, got %+v", toks)
	}
}

func TestTokenize_OverMaxBytes_FallsBackToPlain(t *testing.T) {
	t.Parallel()
	src := strings.Repeat("a", maxTokenizeBytes+1)
	toks := Default().Tokenize("go", src)
	if len(toks) != 1 || toks[0].Kind != KindPlain {
		t.Errorf("oversized: want single plain token, got %d tokens", len(toks))
	}
	if toks[0].Text != src {
		t.Error("oversized: src not preserved in fallback")
	}
}

func TestTokenize_AllKinds_GoSnippet(t *testing.T) {
	t.Parallel()
	src := `// c
package main
import "fmt"
func f() {
	var x int = 42 + len("hi")
	fmt.Println(x)
}
`
	toks := Default().Tokenize("go", src)
	seen := map[Kind]bool{}
	for _, tk := range toks {
		seen[tk.Kind] = true
	}
	want := []Kind{
		KindKeyword, KindString, KindNumber, KindComment,
		KindOperator, KindPunctuation, KindType, KindFunction,
		KindBuiltin,
	}
	for _, k := range want {
		if !seen[k] {
			t.Errorf("kind %d not produced", k)
		}
	}
}

func TestMapKind_KeywordTypeBecomesType(t *testing.T) {
	t.Parallel()
	if got := mapKind(chroma.KeywordType); got != KindType {
		t.Errorf("KeywordType: want KindType, got %d", got)
	}
	if got := mapKind(chroma.NameFunction); got != KindFunction {
		t.Errorf("NameFunction: want KindFunction, got %d", got)
	}
	if got := mapKind(chroma.NameBuiltin); got != KindBuiltin {
		t.Errorf("NameBuiltin: want KindBuiltin, got %d", got)
	}
}

func TestLookupLexer_UncuratedReturnsNil(t *testing.T) {
	t.Parallel()
	if lookupLexer("fortran") != nil {
		t.Error("fortran should not be in curated set")
	}
	if lookupLexer("") != nil {
		t.Error("empty lang should return nil")
	}
	if lookupLexer("go") == nil {
		t.Error("go should be in curated set")
	}
}

func TestRoundtripText(t *testing.T) {
	t.Parallel()
	src := "x := 1 + 2 // note\n"
	toks := Default().Tokenize("go", src)
	var b strings.Builder
	for _, tk := range toks {
		b.WriteString(tk.Text)
	}
	joined := b.String()
	if joined != src {
		t.Errorf("roundtrip mismatch:\n got %q\nwant %q", joined, src)
	}
}
