package appinfo

import (
	"strings"
	"testing"
)

func TestParseFull(t *testing.T) {
	src := `# appinfo.toml
id      = "org.go-gui.falcon"
name    = "Falcon"   # shown in the title bar
version = "1.4.0"
build   = "42"
icon    = "assets/icon.png"

[darwin]
category = "public.app-category.developer-tools"

[linux]
categories = "Development;Utility;"
`
	info, err := Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	want := Info{
		ID:      "org.go-gui.falcon",
		Name:    "Falcon",
		Version: "1.4.0",
		Build:   "42",
		Icon:    "assets/icon.png",
		Darwin:  DarwinInfo{Category: "public.app-category.developer-tools"},
		Linux:   LinuxInfo{Categories: "Development;Utility;"},
	}
	if info != want {
		t.Fatalf("got %+v\nwant %+v", info, want)
	}
}

func TestParseEmpty(t *testing.T) {
	for _, src := range []string{"", "\n\n", "# only a comment\n", "\xef\xbb\xbf"} {
		info, err := Parse([]byte(src))
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if info != (Info{}) {
			t.Fatalf("%q: got %+v, want zero Info", src, info)
		}
	}
}

func TestParseValues(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"hash inside quotes kept", `name = "C# Tool"`, "C# Tool"},
		{"escaped quote", `name = "say \"hi\""`, `say "hi"`},
		{"escaped backslash", `name = "a\\b"`, `a\b`},
		{"empty value", `name = ""`, ""},
		{"no spaces", `name="Tight"`, "Tight"},
		{"tabs", "name\t=\t\"Tabbed\"\t# c", "Tabbed"},
		{"crlf", "name = \"Win\"\r\n", "Win"},
		{"bom", "\xef\xbb\xbfname = \"Bom\"", "Bom"},
		{"utf8", `name = "Café"`, "Café"},
		{"indented", `   name = "Indent"`, "Indent"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := Parse([]byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			if info.Name != tt.want {
				t.Fatalf("got %q, want %q", info.Name, tt.want)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string // substring of the error, including the line number
	}{
		{"unquoted value", "\nname = Falcon", "line 2"},
		{"single quotes", `name = 'Falcon'`, "line 1"},
		{"unknown key", "id = \"a.b\"\nfoo = \"x\"", `line 2: unknown key "foo"`},
		{"unknown section", "[plan9]\n", `line 1: unknown section "plan9"`},
		{"key unknown in section", "[darwin]\nname = \"x\"", `line 2: unknown key "name" in [darwin]`},
		{"nested section", "[darwin.extra]", "line 1"},
		{"array table", "[[darwin]]", "line 1"},
		{"array value", `name = ["a"]`, "line 1"},
		{"number value", `build = 42`, "line 1"},
		{"dotted key", `darwin.category = "x"`, "line 1"},
		{"missing equals", `name "x"`, "line 1"},
		{"missing key", `= "x"`, "line 1"},
		{"unterminated string", `name = "abc`, "line 1"},
		{"text after value", `name = "a" b`, "line 1"},
		{"text after section", `[darwin] x`, "line 1"},
		{"unclosed section", `[darwin`, "line 1"},
		{"unknown escape", `name = "a\nb"`, "line 1"},
		{"multi-line string", `name = """x"""`, "line 1"},
		{"control char", "name = \"a\x01b\"", "line 1"},
		{"control char in comment line", "# a\x01b", "line 1"},
		{"control char in trailing comment", "name = \"a\" # \x7f", "line 1"},
		{"lone CR in comment", "# a\rb\n", "line 1"},
		{"control char in section comment", "[darwin] # \x1b", "line 1"},
		{"two CRs before LF", "name = \"a\"\r\r\n", "line 1"},
		{"lone CR at EOF", "name = \"a\"\r", "line 1"},
		{"duplicate key", "name = \"a\"\nname = \"b\"", `line 2: duplicate key "name"`},
		{"duplicate section", "[darwin]\n[linux]\n[darwin]", `line 3: duplicate section "darwin"`},
		{"bad id", `id = "my app"`, "line 1"},
		{"invalid utf8", "name = \"\xff\"", "UTF-8"},
		{"too large", "# " + strings.Repeat("x", maxSize), "larger than"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.src))
			if err == nil {
				t.Fatal("want error, got nil")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not contain %q", err, tt.want)
			}
			if !strings.HasPrefix(err.Error(), "appinfo: ") {
				t.Fatalf("error %q lacks the appinfo: prefix", err)
			}
		})
	}
}

// Every key after a section header belongs to that section, as in TOML.
// A top-level key written below [linux] is therefore unknown there.
func TestParseSectionScopesKeys(t *testing.T) {
	_, err := Parse([]byte("[linux]\nid = \"x\""))
	if err == nil || !strings.Contains(err.Error(), `unknown key "id" in [linux]`) {
		t.Fatalf("got %v, want unknown key in [linux]", err)
	}
}

func TestMustParse(t *testing.T) {
	info := MustParse([]byte(`name = "Ok"`))
	if info.Name != "Ok" {
		t.Fatalf("got %q", info.Name)
	}
	defer func() {
		if recover() == nil {
			t.Fatal("MustParse did not panic on a bad file")
		}
	}()
	MustParse([]byte("name = bad"))
}

func FuzzParse(f *testing.F) {
	f.Add([]byte(`id = "a.b"` + "\n[darwin]\ncategory = \"c\" # x\n"))
	f.Add([]byte(`name = "a\"b\\c"`))
	f.Fuzz(func(t *testing.T, data []byte) {
		// Parse must never panic; errors are fine.
		_, _ = Parse(data)
	})
}
