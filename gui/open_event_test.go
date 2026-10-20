package gui

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

func TestParseOpenArgs(t *testing.T) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	// An absolute path in the host's own spelling, so the case holds
	// on Windows and Unix alike.
	abs := filepath.Join(cwd, "doc.txt")
	tests := []struct {
		name  string
		args  []string
		files []string
		urls  []string
	}{
		{name: "empty", args: nil},
		{name: "blank arg skipped", args: []string{""}},
		{name: "absolute file", args: []string{abs}, files: []string{abs}},
		{
			name:  "relative file made absolute",
			args:  []string{"doc.txt"},
			files: []string{abs},
		},
		{name: "flags skipped", args: []string{"-v", "--theme=dark", "-psn_0_12345"}},
		{
			name:  "double dash ends flags",
			args:  []string{"--", "-odd.txt"},
			files: []string{filepath.Join(cwd, "-odd.txt")},
		},
		{
			name: "custom scheme",
			args: []string{"myapp://open?doc=1"},
			urls: []string{"myapp://open?doc=1"},
		},
		{
			name: "https",
			args: []string{"https://example.com/a"},
			urls: []string{"https://example.com/a"},
		},
		{
			// url.Parse reads "C:\x" as scheme "c". A one-letter scheme is a
			// drive letter, never a URL.
			name:  "windows drive backslash",
			args:  []string{`C:\docs\a.txt`},
			files: []string{`C:\docs\a.txt`},
		},
		{
			name:  "windows drive slash",
			args:  []string{`d:/docs/a.txt`},
			files: []string{`d:/docs/a.txt`},
		},
		{
			name:  "mixed order kept per kind",
			args:  []string{"ab://1", abs, "-q", "cd://2"},
			files: []string{abs},
			urls:  []string{"ab://1", "cd://2"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseOpenArgs(tc.args)
			if !reflect.DeepEqual(got.Files, tc.files) {
				t.Errorf("Files = %q, want %q", got.Files, tc.files)
			}
			if !reflect.DeepEqual(got.URLs, tc.urls) {
				t.Errorf("URLs = %q, want %q", got.URLs, tc.urls)
			}
		})
	}
}

func TestParseOpenArgsFileURL(t *testing.T) {
	want := "/tmp/a b.txt"
	arg := "file:///tmp/a%20b.txt"
	if runtime.GOOS == "windows" {
		want = `C:\tmp\a b.txt`
		arg = "file:///C:/tmp/a%20b.txt"
	}
	got := ParseOpenArgs([]string{arg})
	if len(got.URLs) != 0 {
		t.Fatalf("URLs = %q, want none", got.URLs)
	}
	if len(got.Files) != 1 || got.Files[0] != want {
		t.Fatalf("Files = %q, want [%q]", got.Files, want)
	}
}

func TestOpenEventEmpty(t *testing.T) {
	if !(OpenEvent{}).empty() {
		t.Fatal("zero OpenEvent not empty")
	}
	if (OpenEvent{URLs: []string{"x://"}}).empty() {
		t.Fatal("OpenEvent with URL reports empty")
	}
}

// A relative file name with a colon ("notes:v2.txt", legal on Unix and
// macOS) parses as an opaque URL. When that file exists it is a file.
func TestParseOpenArgsColonFileName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("':' cannot appear in a Windows file name")
	}
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("notes:v2.txt", nil, 0o600); err != nil {
		t.Fatal(err)
	}
	got := ParseOpenArgs([]string{"notes:v2.txt", "mailto:a@b.c"})
	want := filepath.Join(dir, "notes:v2.txt")
	if len(got.Files) != 1 || got.Files[0] != want {
		t.Fatalf("Files = %q, want [%q]", got.Files, want)
	}
	if len(got.URLs) != 1 || got.URLs[0] != "mailto:a@b.c" {
		t.Fatalf("URLs = %q, want [mailto:a@b.c]", got.URLs)
	}
}

// A file URL with a host names a UNC share. On Windows it becomes
// \\host\share\…; elsewhere there is no such path, so it is skipped
// rather than turned into /share/… on the local disk.
func TestParseOpenArgsFileURLHost(t *testing.T) {
	got := ParseOpenArgs([]string{"file://server/share/a.txt", "file://localhost/tmp/b.txt"})
	var want []string
	if runtime.GOOS == "windows" {
		want = []string{`\\server\share\a.txt`, `\tmp\b.txt`}
	} else {
		want = []string{"/tmp/b.txt"}
	}
	if !reflect.DeepEqual(got.Files, want) {
		t.Fatalf("Files = %q, want %q", got.Files, want)
	}
}
