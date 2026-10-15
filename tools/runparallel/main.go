// runparallel runs make targets at the same time, then prints the output
// of each target as one block and a pass/fail summary.
//
// It replaces scripts/run-parallel.sh (issue #1026) with no change in
// behavior: each target's output prints as one block in the order given,
// followed by the same summary table, and the exit code is non-zero when
// one or more targets fail. Used by `make prepush`.
//
// Rationale, carried over from the script: the system make on macOS is
// GNU Make 3.81, which has no --output-sync, so `make -j` would mix the
// output of all targets line by line. This tool buffers each target's
// output instead — in memory, so target names never touch the filesystem
// the way the script's "$logdir/$target.log" files did.
//
// Usage:
//
//	go run ./tools/runparallel <target>...
//
// The make binary comes from the MAKE environment variable and defaults
// to "make"; the Makefile passes MAKE="$(MAKE)". Each target runs as
// `make --no-print-directory <target>`. Ctrl-C stops every child target
// and exits 130, like the script's trap.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"time"
)

// result is one finished target: its combined output, its exit code,
// and its whole-second runtime.
type result struct {
	target string
	output []byte
	code   int
	secs   int64
}

// runFunc starts one target and reports its result. It is a variable so
// tests can stub the subprocess boundary.
type runFunc func(ctx context.Context, makeBin, target string) result

func main() {
	ctx, cancel := signalContext()
	os.Exit(run(os.Args[1:], ctx, cancel, runTarget))
}

// signalContext returns a context cancelled on SIGINT/SIGTERM.
// Cancellation kills the running make processes (see runTarget); run
// then exits 130, the script's trap status.
func signalContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, termSignal)
	go func() {
		select {
		case <-sig:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

func run(targets []string, ctx context.Context, cancel context.CancelFunc, run runFunc) int {
	defer cancel()
	makeBin := os.Getenv("MAKE")
	if makeBin == "" {
		makeBin = "make"
	}

	results := make([]result, len(targets))
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Go(func() {
			results[i] = run(ctx, makeBin, target)
		})
	}
	wg.Wait()

	// A cancelled context means SIGINT/SIGTERM arrived: the script's
	// trap prints no blocks and exits 130.
	if ctx.Err() != nil {
		return 130
	}

	failed := false
	for _, r := range results {
		fmt.Printf("==> %s\n", r.target)
		_, _ = os.Stdout.Write(r.output)
	}
	fmt.Println("==> summary")
	for _, r := range results {
		if r.code == 0 {
			fmt.Printf("  ok    %-18s %4ds\n", r.target, r.secs)
		} else {
			fmt.Printf("  FAIL  %-18s %4ds (exit %d)\n", r.target, r.secs, r.code)
			failed = true
		}
	}
	if failed {
		return 1
	}
	return 0
}

// startCommand builds the subprocess for one target. It is a variable
// so tests can point it at the test binary itself (TestHelperProcess)
// instead of make.
var startCommand = func(ctx context.Context, makeBin, target string) *exec.Cmd {
	// #nosec G204 -- binary is the MAKE env override (default "make"),
	// the tool's documented input; args are fixed flags and targets.
	return exec.CommandContext(ctx, makeBin, "--no-print-directory", target)
}

// runTarget runs one make target, buffering its combined stdout and
// stderr. The command dies with the context, so Ctrl-C stops the child
// too.
func runTarget(ctx context.Context, makeBin, target string) result {
	start := time.Now()
	var buf bytes.Buffer
	cmd := startCommand(ctx, makeBin, target)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	code := 0
	if err := cmd.Run(); err != nil {
		code = 127 // could not start, like the shell
		if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
			code = exitErr.ExitCode()
		}
	}
	return result{
		target: target,
		output: buf.Bytes(),
		code:   code,
		secs:   int64(time.Since(start).Seconds()),
	}
}
