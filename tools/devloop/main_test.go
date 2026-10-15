package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestParsePoll(t *testing.T) {
	cases := []struct {
		raw  string
		want time.Duration
		bad  bool
	}{
		{"", defaultPoll, false},
		{"0.5", 500 * time.Millisecond, false},
		{"2", 2 * time.Second, false},
		{"0", 0, true},
		{"-1", 0, true},
		{"abc", 0, true},
	}
	for _, tc := range cases {
		if got, err := parsePoll(tc.raw); (err != nil) != tc.bad || (!tc.bad && got != tc.want) {
			t.Errorf("parsePoll(%q) = %v, %v; want %v, bad=%v",
				tc.raw, got, err, tc.want, tc.bad)
		}
	}
}

// appMainBody is the test app's source after the package clause.
const appMainBody = `import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	b, _ := os.ReadFile(os.Args[1])
	n := 0
	if f := strings.Fields(string(b)); len(f) > 0 {
		n, _ = strconv.Atoi(f[0])
	}
	f, _ := os.Create(os.Args[1])
	fmt.Fprintf(f, "%d %d\n", n+1, os.Getpid())
	f.Close()
	time.Sleep(60 * time.Second)
}
`

// testModule builds a scratch module: an app that records each boot in
// a marker file, plus a library file to trigger rebuilds.
func testModule(t *testing.T) (app, marker string) {
	t.Helper()
	root := t.TempDir()
	t.Chdir(root)
	if err := os.WriteFile("go.mod", []byte("module example.com/dltest\n\ngo 1.21\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll("testapp", 0o755); err != nil {
		t.Fatal(err)
	}
	main := "package main\n\n" + appMainBody
	if err := os.WriteFile("testapp/main.go", []byte(main), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("lib.go", []byte("package dltest\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	marker = filepath.Join(root, "marker.txt")
	return "./testapp", marker
}

// readMarker returns the boot count and pid the app last wrote.
func readMarker(t *testing.T, marker string) (count, pid int) {
	t.Helper()
	b, err := os.ReadFile(marker)
	if err != nil {
		return -1, -1
	}
	parts := strings.Fields(string(b))
	if len(parts) != 2 {
		return -1, -1
	}
	count, _ = strconv.Atoi(parts[0])
	pid, _ = strconv.Atoi(parts[1])
	return count, pid
}

// waitFor polls until cond holds or the deadline passes.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestChanged(t *testing.T) {
	testModule(t)
	l := &loop{cfg: config{watch: ".", work: t.TempDir()}}
	if err := os.MkdirAll(l.cfg.work, 0o755); err != nil {
		t.Fatal(err)
	}
	base := time.Now()
	old := func(path string) {
		t.Helper()
		if err := os.Chtimes(path, base, base); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"testapp/main.go", "lib.go", "go.mod"} {
		old(p)
	}
	if err := os.WriteFile("notes.md", []byte("docs\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	old("notes.md")
	l.touchStamp()
	if err := os.Chtimes(l.stamp(), base, base); err != nil {
		t.Fatal(err)
	}

	changed := func() bool {
		c, err := l.changed()
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	if changed() {
		t.Error("changed() = true with nothing newer than the stamp")
	}

	// Docs never trigger.
	if err := os.Chtimes("notes.md", base.Add(time.Hour), base.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if changed() {
		t.Error("changed() = true for a newer .md file")
	}
	old("notes.md")

	// Go source, go.mod and go.sum trigger.
	for _, p := range []string{"testapp/main.go", "lib.go", "go.mod"} {
		if err := os.Chtimes(p, base.Add(time.Hour), base.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if !changed() {
			t.Errorf("changed() = false for newer %s", p)
		}
		old(p)
	}
	if err := os.WriteFile("go.sum", []byte("sums\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !changed() {
		t.Error("changed() = false for a new go.sum (newer than stamp)")
	}
	os.Remove("go.sum")

	// Root build/ and .git are pruned...
	for _, p := range []string{"build/skip.go", ".git/skip.go"} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("package x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if changed() {
		t.Error("changed() = true for files under root build/ or .git")
	}
	// ...but a nested build/ dir is scanned, like find's top-level prune.
	if err := os.MkdirAll("sub/build", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("sub/build/nested.go", []byte("package x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !changed() {
		t.Error("changed() = false for a new file under nested build/")
	}
}

func TestInitialBuildFailure(t *testing.T) {
	app, marker := testModule(t)
	if err := os.WriteFile("testapp/main.go", []byte("not go code\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := &loop{cfg: config{app: app, appArgs: []string{marker}, poll: time.Millisecond, work: t.TempDir()}}
	if err := l.run(t.Context()); !errors.Is(err, errInitialBuild) {
		t.Errorf("run() = %v, want errInitialBuild", err)
	}
	if _, err := os.Stat(l.bin()); !os.IsNotExist(err) {
		t.Error("binary exists after a failed initial build")
	}
}

func TestRelaunchCycle(t *testing.T) {
	app, marker := testModule(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	l := &loop{cfg: config{
		app:     app,
		appArgs: []string{marker},
		poll:    100 * time.Millisecond,
		watch:   ".",
		work:    t.TempDir(),
	}}

	runErr := make(chan error, 1)
	go func() { runErr <- l.run(ctx) }()

	// First boot.
	waitFor(t, "initial launch", func() bool {
		n, _ := readMarker(t, marker)
		return n == 1
	})
	count1, pid1 := readMarker(t, marker)
	if count1 != 1 || pid1 <= 0 {
		t.Fatalf("marker = (%d, %d), want boot 1 with a pid", count1, pid1)
	}
	if _, err := os.Stat(l.bin()); err != nil {
		t.Fatalf("binary missing after initial build: %v", err)
	}

	// Save a library file: the app rebuilds and relaunches. A plain
	// write carries the current time, which is newer than the stamp.
	if err := os.WriteFile("lib.go", []byte("package dltest\n\nvar Touched = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "relaunch", func() bool {
		n, _ := readMarker(t, marker)
		return n == 2
	})
	_, pid2 := readMarker(t, marker)
	t.Logf("pid1=%d pid2=%d", pid1, pid2)
	if pid2 == pid1 {
		t.Error("pid unchanged after rebuild: old process was not replaced")
	}
	if runtime.GOOS != "windows" {
		// The kill lands asynchronously and the runtime reaps on its
		// own schedule; poll briefly. A zombie (Z) counts as stopped:
		// it holds no windows, ports, or files.
		deadline := time.Now().Add(5 * time.Second)
		for procAlive(pid1) {
			if time.Now().After(deadline) {
				t.Error("old process still alive after relaunch")
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	// Break the build inside the app's own package: the old binary
	// keeps running, no relaunch. (A broken file outside the app's
	// dependency tree still rebuilds and relaunches, identically to the
	// script — go build just produces the same binary.)
	if err := os.WriteFile("testapp/main.go", []byte("not go code\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
	if n, _ := readMarker(t, marker); n != 2 {
		t.Errorf("marker count = %d after a failed build, want 2 (no relaunch)", n)
	}

	// Fix it with a real change: the next save rebuilds and relaunches.
	if err := os.WriteFile("testapp/main.go", []byte("package main\n\n// fixed\n\n"+appMainBody), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "relaunch after fix", func() bool {
		n, _ := readMarker(t, marker)
		return n == 3
	})

	select {
	case err := <-runErr:
		t.Fatalf("run() returned %v while context is live", err)
	default:
	}

	cancel()
	l.stop()
	if err := <-runErr; err != nil {
		t.Errorf("run() after cancel = %v, want nil", err)
	}
}

// procAlive reports whether pid is a living process. Signal(0) is not
// enough: it succeeds on zombies, so read the state with ps instead.
// Missing from ps, or zombie, means stopped.
func procAlive(pid int) bool {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "stat=").Output()
	if err != nil {
		return false
	}
	return !strings.Contains(string(out), "Z")
}
