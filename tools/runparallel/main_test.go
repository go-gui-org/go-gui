package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// captureStdout runs fn with os.Stdout redirected to a buffer.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// stubRunner returns canned results and records the make binary it saw.
func stubRunner(results map[string]result, seenBin *string) runFunc {
	return func(_ context.Context, makeBin, target string) result {
		if seenBin != nil {
			*seenBin = makeBin
		}
		r := results[target]
		r.target = target
		return r
	}
}

func TestBlocksAndSummary(t *testing.T) {
	targets := []string{"alpha", "beta-failing"}
	results := map[string]result{
		"alpha":        {output: []byte("alpha out\n"), code: 0, secs: 3},
		"beta-failing": {output: []byte("beta err\n"), code: 2, secs: 12},
	}
	out := captureStdout(t, func() {
		if code := run(targets, context.Background(), context.CancelFunc(func() {}),
			stubRunner(results, nil)); code != 1 {
			t.Errorf("run() = %d, want 1", code)
		}
	})
	for _, want := range []string{
		"==> alpha\nalpha out\n",
		"==> beta-failing\nbeta err\n",
		"==> summary\n",
		fmt.Sprintf("  ok    %-18s %4ds\n", "alpha", 3),
		fmt.Sprintf("  FAIL  %-18s %4ds (exit %d)\n", "beta-failing", 12, 2),
	} {
		if !strings.Contains(out, want) {
			t.Errorf("run output missing %q\nfull output:\n%s", want, out)
		}
	}
	// Blocks print in target order even though beta finished... first:
	// the stub returns immediately, so order comes from results, not
	// completion.
	if strings.Index(out, "==> alpha") > strings.Index(out, "==> beta-failing") {
		t.Error("blocks out of target order")
	}
}

func TestAllPassExitZero(t *testing.T) {
	out := captureStdout(t, func() {
		if code := run([]string{"a"}, context.Background(), context.CancelFunc(func() {}),
			stubRunner(map[string]result{"a": {secs: 1}}, nil)); code != 0 {
			t.Errorf("run() = %d, want 0", code)
		}
	})
	if want := fmt.Sprintf("  ok    %-18s %4ds\n", "a", 1); !strings.Contains(out, want) {
		t.Errorf("summary row wrong, want %q:\n%s", want, out)
	}
}

func TestNoTargets(t *testing.T) {
	out := captureStdout(t, func() {
		if code := run(nil, context.Background(), context.CancelFunc(func() {}),
			stubRunner(nil, nil)); code != 0 {
			t.Errorf("run() = %d, want 0", code)
		}
	})
	if out != "==> summary\n" {
		t.Errorf("run(nil) output = %q, want just the summary header", out)
	}
}

func TestMakeBinary(t *testing.T) {
	var seen string
	t.Setenv("MAKE", "gmake")
	captureStdout(t, func() {
		run([]string{"a"}, context.Background(), context.CancelFunc(func() {}),
			stubRunner(map[string]result{"a": {}}, &seen))
	})
	if seen != "gmake" {
		t.Errorf("make binary = %q, want MAKE env override", seen)
	}

	os.Unsetenv("MAKE")
	captureStdout(t, func() {
		run([]string{"a"}, context.Background(), context.CancelFunc(func() {}),
			stubRunner(map[string]result{"a": {}}, &seen))
	})
	if seen != "make" {
		t.Errorf("make binary = %q, want default", seen)
	}
}

func TestCancelledContextExits130(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := captureStdout(t, func() {
		if code := run([]string{"a"}, ctx, context.CancelFunc(func() {}),
			stubRunner(map[string]result{"a": {output: []byte("x")}}, nil)); code != 130 {
			t.Errorf("run() on cancelled context = %d, want 130", code)
		}
	})
	if out != "" {
		t.Errorf("cancelled run printed %q, want nothing", out)
	}
}

// TestHelperProcess is re-executed as a fake `make`: its behavior comes
// from its last argument (the target), so the real exec path — output
// capture, exit codes, parallelism — is exercised without a shell.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	target := os.Args[len(os.Args)-1]
	switch target {
	case "rendezvous-a", "rendezvous-b":
		rendezvous(target)
	case "failing":
		fmt.Fprint(os.Stderr, "boom\n")
		os.Exit(3)
	default:
		fmt.Printf("out-%s\n", target)
	}
	os.Exit(0)
}

// rendezvous proves two targets overlap: each helper announces itself
// with a file and waits for the other's file. Sequential execution
// fails deterministically (the first helper times out), while any
// overlapping execution succeeds however slow the machine is — no
// wall-clock budget to flake under -race or load.
func rendezvous(target string) {
	dir := os.Getenv("HELPER_DIR")
	other := "rendezvous-a"
	if target == "rendezvous-a" {
		other = "rendezvous-b"
	}
	if f, err := os.Create(dir + "/" + target + ".ready"); err == nil {
		f.Close()
	} else {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for range 2000 {
		if _, err := os.Stat(dir + "/" + other + ".ready"); err == nil {
			os.Exit(0)
		}
		time.Sleep(10 * time.Millisecond)
	}
	fmt.Fprintln(os.Stderr, "rendezvous timed out waiting for "+other)
	os.Exit(1)
}

func helperCommand(t *testing.T, ctx context.Context, target string) *exec.Cmd {
	t.Helper()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess", "--", target)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestRunTargetExec(t *testing.T) {
	old := startCommand
	startCommand = func(ctx context.Context, _, target string) *exec.Cmd {
		return helperCommand(t, ctx, target)
	}
	defer func() { startCommand = old }()

	r := runTarget(context.Background(), "ignored", "hello")
	if string(r.output) != "out-hello\n" || r.code != 0 || r.target != "hello" {
		t.Errorf("runTarget() = %+v, want captured output with code 0", r)
	}

	r = runTarget(context.Background(), "ignored", "failing")
	if string(r.output) != "boom\n" || r.code != 3 {
		t.Errorf("runTarget() on failing target = %+v, want stderr and code 3", r)
	}
}

func TestTargetsRunConcurrently(t *testing.T) {
	old := startCommand
	startCommand = func(ctx context.Context, _, target string) *exec.Cmd {
		return helperCommand(t, ctx, target)
	}
	defer func() { startCommand = old }()

	// One shared dir: both helpers must see each other's file.
	t.Setenv("HELPER_DIR", t.TempDir())
	code := run([]string{"rendezvous-a", "rendezvous-b"}, context.Background(),
		context.CancelFunc(func() {}), runTarget)
	if code != 0 {
		t.Errorf("run() = %d, want 0 (helpers rendezvous only when overlapping)", code)
	}
}
