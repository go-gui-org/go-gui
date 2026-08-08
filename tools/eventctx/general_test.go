package eventctx

import (
	"go/format"
	"strings"
	"testing"
)

// convertSet is the §4.3 include list in miniature: enough names to
// exercise the filter in both directions.
func convertSet() *Options {
	return &Options{Fields: map[string]bool{
		"OnSelect": true, "OnTextCommit": true, "OnCopyRows": true,
		"OnLayoutChange": true,
	}}
}

func rewriteGeneral(t *testing.T, src string) string {
	t.Helper()
	res, err := RewriteWith("p.go", []byte(src), nil, convertSet())
	if err != nil {
		t.Fatal(err)
	}
	out, err := format.Source(res.Src)
	if err != nil {
		t.Fatalf("result does not parse: %v\n%s", err, res.Src)
	}
	return string(out)
}

// The plain *Window-tailed case: the shape the strict matcher rejected
// because it has neither a *Layout nor an *Event to fold.
func TestGeneralConvertsWindowTail(t *testing.T) {
	got := rewriteGeneral(t, `package p

type Cfg struct {
	OnSelect func(string, *Window)
	OnInit   func(*Window)
}
`)
	if !strings.Contains(got, "OnSelect func(string, EventCtx)") {
		t.Errorf("OnSelect not converted:\n%s", got)
	}
	// The filter is the safety margin — an excluded lifecycle callback
	// of the identical shape must be left exactly as it was.
	if !strings.Contains(got, "OnInit   func(*Window)") {
		t.Errorf("OnInit was touched:\n%s", got)
	}
}

// Multiple payloads keep their order, and the *Layout in front folds
// away with the *Window at the back.
func TestGeneralPreservesPayloadOrder(t *testing.T) {
	got := rewriteGeneral(t, `package p

type Cfg struct {
	OnTextCommit func(*Layout, string, InputCommitReason, *Window)
}
`)
	want := "OnTextCommit func(string, InputCommitReason, EventCtx)"
	if !strings.Contains(got, want) {
		t.Errorf("want %q, got:\n%s", want, got)
	}
}

// The agreed OnCopyRows invariant: EventCtx replaces the *Window and
// *Event parameters and says nothing about the results.
func TestGeneralKeepsResults(t *testing.T) {
	got := rewriteGeneral(t, `package p

type Cfg struct {
	OnCopyRows func([]GridRow, *Event, *Window) (string, bool)
}
`)
	want := "OnCopyRows func([]GridRow, EventCtx) (string, bool)"
	if !strings.Contains(got, want) {
		t.Errorf("want %q, got:\n%s", want, got)
	}
}

// Widget internals pass a callback onward under the lowercased name.
func TestGeneralMatchesLowercasedParam(t *testing.T) {
	got := rewriteGeneral(t, `package p

func rows(onSelect func(string, *Window)) {}
`)
	if !strings.Contains(got, "onSelect func(string, EventCtx)") {
		t.Errorf("lowercased parameter not converted:\n%s", got)
	}
}

// A closure body renames through the context, and only under an
// included key.
func TestGeneralRewritesClosureBodies(t *testing.T) {
	got := rewriteGeneral(t, `package p

func f() {
	_ = Cfg{
		OnSelect: func(id string, w *Window) {
			use(w, id)
		},
		OnInit: func(w *Window) {
			use(w, "")
		},
	}
}
`)
	if !strings.Contains(got, "func(id string, ctx EventCtx)") ||
		!strings.Contains(got, "use(ctx.Window, id)") {
		t.Errorf("OnSelect closure not converted:\n%s", got)
	}
	if !strings.Contains(got, "OnInit: func(w *Window)") {
		t.Errorf("OnInit closure was touched:\n%s", got)
	}
}

// A closure written inside a converted callback belongs to whatever
// takes it, not to the callback. Inheriting the owner name down the
// tree converted every QueueCommand argument in the examples.
func TestGeneralDoesNotInheritOwnerIntoNestedClosures(t *testing.T) {
	got := rewriteGeneral(t, `package p

func f() {
	_ = Cfg{
		OnSelect: func(id string, w *Window) {
			w.QueueCommand(func(w2 *Window) {
				use(w2, id)
			})
		},
	}
}
`)
	if !strings.Contains(got, "func(id string, ctx EventCtx)") {
		t.Errorf("outer callback not converted:\n%s", got)
	}
	if !strings.Contains(got, "func(w2 *Window)") {
		t.Errorf("nested closure was converted:\n%s", got)
	}
}

// A *Window-tailed helper that is not wired to an included field must
// survive the wide matcher untouched. This is the regression the name
// filter exists to prevent.
func TestGeneralLeavesUnattributedHelpersAlone(t *testing.T) {
	src := `package p

func helper(l *Layout, e *Event, w *Window) {
	use(l, e, w)
}
`
	if got := rewriteGeneral(t, src); got != src {
		t.Errorf("helper rewritten:\n%s", got)
	}
}

// The declaration plan in general mode keys off the field wiring, not
// off the signature: only a function actually assigned to an included
// field may convert.
func TestGeneralScanDeclsUsesFieldWiring(t *testing.T) {
	src := []byte(`package p

func handleSelect(id string, w *Window) {}
func handleOther(id string, w *Window)  {}

var c = Cfg{OnSelect: handleSelect, OnInit: handleOther}
`)
	cands, used, err := ScanDeclsWith("p.go", src, convertSet())
	if err != nil {
		t.Fatal(err)
	}
	// The wide matcher accepts both declarations as candidates...
	if _, ok := cands["handleOther"]; !ok {
		t.Fatal("handleOther missing from candidates")
	}
	// ...and the field wiring is what narrows the plan to one.
	if !used["handleSelect"] {
		t.Error("handleSelect not recorded as wired to an included field")
	}
	if used["handleOther"] {
		t.Error("handleOther wired to OnInit, which is excluded")
	}
}
