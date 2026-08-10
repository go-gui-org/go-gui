package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"Add 10 Items": "add_10_items",
		"cardView":     "card_view",
		"TestFoo":      "test_foo",
		// Acronym runs stay whole rather than becoming r_t_f_view.
		"RTFView":  "rtf_view",
		"HTTPGet":  "http_get",
		"Save":     "save",
		"  Save  ": "save",
		"×":        "",
		"":         "",
		"a--b":     "a_b",
		"Reset!!!": "reset",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIDScope(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		// Every example is a main.go, so the directory carries meaning.
		"examples/todo/main.go":      "todo",
		"examples/todo/main_test.go": "todo",
		"gui/view_toast.go":          "toast",
		"gui/command_test.go":        "command_test",
	}
	for in, want := range cases {
		if got := idScope(filepath.FromSlash(in)); got != want {
			t.Errorf("idScope(%q) = %q, want %q", in, got, want)
		}
	}
}

// uniqueID must not collide with an ID already written by hand in the
// same file, or the codemod would introduce the very duplicate-ID
// defect the debug gate reports.
func TestUniqueIDAvoidsExisting(t *testing.T) {
	t.Parallel()
	taken := map[string]bool{"todo_save": true}

	if got := uniqueID(taken, "todo", "save"); got != "todo_save_2" {
		t.Errorf("first = %q, want todo_save_2", got)
	}
	if got := uniqueID(taken, "todo", "save"); got != "todo_save_3" {
		t.Errorf("second = %q, want todo_save_3", got)
	}
	// A hint that already carries the scope must not double it.
	if got := uniqueID(taken, "command_test", "command_test_opens"); got != "command_test_opens" {
		t.Errorf("prefixed hint = %q, want command_test_opens", got)
	}
}

func TestFixWants(t *testing.T) {
	t.Parallel()
	only := regexp.MustCompile(`_test\.go$|^examples/`)
	skip := regexp.MustCompile(`^examples/generated/`)

	cases := []struct {
		rel  string
		want bool
	}{
		{"gui/view_toast_test.go", true},
		{"examples/todo/main.go", true},
		{"gui/view_toast.go", false},
		{"examples/generated/main.go", false},
	}
	for _, c := range cases {
		if got := fixWants(only, skip, c.rel); got != c.want {
			t.Errorf("fixWants(%q) = %v, want %v", c.rel, got, c.want)
		}
	}
	if !fixWants(nil, nil, "anything.go") {
		t.Error("no filters must accept everything")
	}
}

// runFix writes src to a temp file, runs the codemod over it, and
// returns the rewritten source.
func runFix(t *testing.T, name, src string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := fixFile(dir, path, []string{"ButtonCfg", "ToggleCfg"}, false); err != nil {
		t.Fatalf("fixFile: %v", err)
	}
	out, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	return string(out)
}

// The nested TextCfg label is the most legible ID source, because
// ButtonCfg — the dominant type in the broken set — has no Text field
// of its own; its label lives in Content.
func TestFixInsertsIDFromNestedText(t *testing.T) {
	t.Parallel()
	got := runFix(t, "todo.go", `package main

func view() {
	_ = Button(ButtonCfg{
		Content: []View{Text(TextCfg{Text: "Save File"})},
	})
}
`)
	// Assert on the value, not "ID: " — gofmt pads the key to align
	// with the widest field name in the literal.
	if !strings.Contains(got, `"todo_save_file"`) {
		t.Fatalf("want ID from nested text, got:\n%s", got)
	}
}

// An ID present but empty must be replaced, not supplemented:
// inserting would produce a duplicate key and a file that does not
// compile.
func TestFixReplacesEmptyIDRatherThanInserting(t *testing.T) {
	t.Parallel()
	got := runFix(t, "todo.go", `package main

func view() {
	_ = Button(ButtonCfg{
		ID:      "",
		Content: []View{Text(TextCfg{Text: "Go"})},
	})
}
`)
	if strings.Count(got, "ID:") != 1 {
		t.Fatalf("want exactly one ID field, got:\n%s", got)
	}
	if !strings.Contains(got, `"todo_go"`) {
		t.Fatalf("want the empty ID replaced, got:\n%s", got)
	}
}

// A literal already carrying an ID is not broken and must be left
// alone, including a computed one the tool cannot evaluate.
func TestFixLeavesGoodLiteralsAlone(t *testing.T) {
	t.Parallel()
	src := `package main

func view() {
	_ = Button(ButtonCfg{ID: "kept"})
	_ = Button(ButtonCfg{ID: computed()})
	_ = Button(ButtonCfg{FocusDisabled: true})
}
`
	if got := runFix(t, "todo.go", src); got != src {
		t.Fatalf("unchanged input was rewritten:\n%s", got)
	}
}

// Several literals in one file exercise the reverse-order splice: a
// forward-order apply would shift every offset after the first.
func TestFixMultipleLiteralsInOneFile(t *testing.T) {
	t.Parallel()
	got := runFix(t, "todo.go", `package main

func view() {
	_ = Button(ButtonCfg{
		Content: []View{Text(TextCfg{Text: "One"})},
	})
	_ = Button(ButtonCfg{
		Content: []View{Text(TextCfg{Text: "Two"})},
	})
	_ = Toggle(ToggleCfg{})
}
`)
	for _, want := range []string{`"todo_one"`, `"todo_two"`, `"todo_view"`} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
}

// A single-line literal must stay on one line: expanding it would
// reformat code the change is not about.
func TestFixKeepsSingleLineLiteralsInline(t *testing.T) {
	t.Parallel()
	got := runFix(t, "todo.go", `package main

func view() {
	_ = Toggle(ToggleCfg{})
	_ = Toggle(ToggleCfg{Checked: true})
}
`)
	for _, want := range []string{
		`ToggleCfg{ID: "todo_view"}`,
		`ToggleCfg{ID: "todo_view_2", Checked: true}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in:\n%s", want, got)
		}
	}
}

// The result must be gofmt-clean and parseable; applyEdits runs
// format.Source and fails loudly rather than writing broken source.
func TestApplyEditsRejectsUnparseableResult(t *testing.T) {
	t.Parallel()
	src := []byte("package main\n")
	_, err := applyEdits(src, []edit{{start: 0, end: 0, text: "!!!"}})
	if err == nil {
		t.Fatal("want an error when the splice does not parse")
	}
	if !strings.Contains(err.Error(), "does not parse") {
		t.Errorf("unhelpful error: %v", err)
	}
}
