// devloop rebuilds and relaunches a Go app whenever the module changes.
//
// It replaces scripts/dev-loop.sh (issue #1026) with the same behavior:
// the app package (plus its dependency tree, which covers gui/) is
// rebuilt into build/dev-loop/, the old process is stopped, and the new
// binary is launched with the original args. A failed build prints the
// error and keeps watching; the last good binary keeps running, and the
// next save retries the build.
//
// Differences from the script, all Windows-driven:
//   - Change detection polls modification times with filepath.WalkDir,
//     not find.
//   - The old process stops with Process.Kill: there are no POSIX
//     signals on Windows. Unix gets Kill too, keeping every OS uniform.
//   - The fresh binary installs after the old process stops, not
//     before: Windows locks the running executable, so renaming over it
//     fails. If the install fails, the previous binary (still in place)
//     is restarted instead.
//
// Usage:
//
//	go run ./tools/devloop <app path> [args...]
//
// Poll interval comes from DEV_LOOP_POLL_SECONDS and defaults to 0.5.
// Ctrl-C stops the app and removes build/dev-loop/.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// defaultPoll is the change-detection interval when
// DEV_LOOP_POLL_SECONDS is unset.
const defaultPoll = 500 * time.Millisecond

// errChanged aborts the watch walk at the first change found.
var errChanged = errors.New("change detected")

// errInitialBuild is fatal: with no previous binary there is nothing
// to keep running.
var errInitialBuild = errors.New("initial build failed; nothing to run")

// config carries everything runLoop needs. watch is the tree to scan,
// work holds the binary and stamp; both default to the script's
// locations and are fields so tests can isolate them.
type config struct {
	app     string
	appArgs []string
	poll    time.Duration
	watch   string
	work    string
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: devloop <app path> [args...]")
		os.Exit(1)
	}
	poll, err := parsePoll(os.Getenv("DEV_LOOP_POLL_SECONDS"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "devloop: %v\n", err)
		os.Exit(2)
	}

	root, err := repoRoot()
	if err != nil {
		fmt.Fprintf(os.Stderr, "devloop: %v\n", err)
		os.Exit(2)
	}
	if err := os.Chdir(root); err != nil {
		fmt.Fprintf(os.Stderr, "devloop: %v\n", err)
		os.Exit(2)
	}

	cfg := config{
		app:     os.Args[1],
		appArgs: os.Args[2:],
		poll:    poll,
		watch:   ".",
		work:    filepath.Join("build", "dev-loop"),
	}
	os.Exit(runSignalLoop(cfg))
}

// parsePoll turns DEV_LOOP_POLL_SECONDS into a duration. Empty means
// the default; anything else must be a positive number of seconds.
func parsePoll(raw string) (time.Duration, error) {
	if raw == "" {
		return defaultPoll, nil
	}
	secs, err := strconv.ParseFloat(raw, 64)
	if err != nil || secs <= 0 {
		return 0, fmt.Errorf("bad DEV_LOOP_POLL_SECONDS %q", raw)
	}
	return time.Duration(secs * float64(time.Second)), nil
}

// repoRoot returns the repo top level, falling back to the working
// directory outside git (the script always ran from its own checkout).
func repoRoot() (string, error) {
	// #nosec G204 -- fixed binary, no shell; no variable arguments.
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		wd, werr := os.Getwd()
		if werr != nil {
			return "", err
		}
		return wd, nil
	}
	return strings.TrimSpace(string(out)), nil
}

// runSignalLoop runs the loop until a signal arrives, then stops the
// app, removes the work dir, and maps the signal to the script's exit
// codes: 0 on SIGINT, 1 on SIGTERM.
func runSignalLoop(cfg config) int {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// One receiver for the signal: it records the exit code, then
	// cancels the loop. A second select on the channel would race it
	// and miss the signal.
	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, termSignal)
	codeCh := make(chan int, 1)
	go func() {
		s := <-sig
		if s == termSignal && termSignal != os.Interrupt {
			codeCh <- 1 // SIGTERM: the script exits 1
		} else {
			codeCh <- 0
		}
		cancel()
	}()
	l := &loop{cfg: cfg}
	err := l.run(ctx)
	l.stop()
	if rerr := os.RemoveAll(cfg.work); rerr != nil {
		fmt.Fprintf(os.Stderr, "devloop: %v\n", rerr)
	}
	if err != nil {
		// errInitialBuild already carries the script's exact words.
		if errors.Is(err, errInitialBuild) {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Fprintf(os.Stderr, "devloop: %v\n", err)
		}
		return 1
	}
	select {
	case code := <-codeCh:
		return code
	default:
		return 0
	}
}

// loop is one watching session: at most one app process at a time.
type loop struct {
	cfg config
	cmd *exec.Cmd
}

func (l *loop) bin() string {
	return filepath.Join(l.cfg.work, filepath.Base(filepath.Clean(l.cfg.app)))
}

func (l *loop) stamp() string {
	return filepath.Join(l.cfg.work, ".stamp")
}

// run builds and launches the app, then rebuilds on every change until
// ctx ends. A nil return means a clean (signal) shutdown; any other
// error is fatal.
func (l *loop) run(ctx context.Context) error {
	if err := os.MkdirAll(l.cfg.work, 0o755); err != nil { // #nosec G301 -- dev scratch dir, like build outputs elsewhere
		return err
	}
	// Initial build: fail loudly, since there is no previous binary.
	if err := l.rebuild(); err != nil {
		if errors.Is(err, errBuildFailed) {
			return errInitialBuild
		}
		return err
	}
	l.touchStamp()
	l.start()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(l.cfg.poll):
		}
		changed, err := l.changed()
		if err != nil {
			return err
		}
		if !changed {
			continue
		}
		fmt.Println("change detected; rebuilding...")
		if err := l.rebuild(); err != nil {
			if errors.Is(err, errBuildFailed) {
				// Keep watching: touching the stamp stops the
				// failed build from retriggering every poll.
				// The running app is untouched, and the next
				// save retries the build.
				fmt.Fprintln(os.Stderr, "build failed; keeping previous binary and watching for changes")
				l.touchStamp()
				continue
			}
			return err
		}
		l.touchStamp()
		l.start()
	}
}

// errBuildFailed means `go build` itself failed; the previous binary
// (if any) is still good and watching continues.
var errBuildFailed = errors.New("build failed")

// rebuild compiles the app and installs the fresh binary: the old
// process stops first, because Windows locks the running executable.
// On an install failure the previous binary is still in place, so the
// caller restarts that one instead.
func (l *loop) rebuild() error {
	tmp, err := l.build()
	if err != nil {
		return err
	}
	l.stop()
	if err := l.swap(tmp); err != nil {
		fmt.Fprintf(os.Stderr, "devloop: install new binary: %v; restarting previous\n", err)
	}
	return nil
}

// build compiles the app into a fresh temp file, never clobbering the
// running binary. Build output streams through, like `go build` in the
// script.
func (l *loop) build() (string, error) {
	f, err := os.CreateTemp(l.cfg.work, "bin.*")
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	_ = f.Close()
	// #nosec G204 -- fixed binary; target is the tool's documented input.
	cmd := exec.Command("go", "build", "-o", tmp, l.cfg.app)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmp)
		return "", errBuildFailed
	}
	return tmp, nil
}

// swap moves a fresh build into place.
func (l *loop) swap(tmp string) error {
	return os.Rename(tmp, l.bin())
}

// start launches the installed binary with the original args.
func (l *loop) start() {
	// #nosec G204 -- binary this tool just built; args are its documented input.
	cmd := exec.Command(l.bin(), l.cfg.appArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "devloop: start %s: %v\n", l.bin(), err)
		l.cmd = nil
		return
	}
	l.cmd = cmd
	fmt.Printf("relaunched %s (pid %d)\n", filepath.Base(l.bin()), cmd.Process.Pid)
}

// stop kills the running instance, if any. Go reaps the child; no wait
// needed.
func (l *loop) stop() {
	if l.cmd == nil || l.cmd.Process == nil {
		return
	}
	_ = l.cmd.Process.Kill()
	l.cmd = nil
}

func (l *loop) touchStamp() {
	f, err := os.OpenFile(l.stamp(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	now := time.Now()
	_ = os.Chtimes(l.stamp(), now, now)
	_ = f.Close()
}

// changed reports whether any watched file is newer than the stamp:
// Go source plus the module files, which pin dependency versions.
// Root build/ and .git are skipped, like the script's find -prune;
// everything else (docs, scripts) never matches the name filter.
func (l *loop) changed() (bool, error) {
	stampInfo, err := os.Stat(l.stamp())
	if err != nil {
		return false, err
	}
	stampTime := stampInfo.ModTime()
	err = filepath.WalkDir(l.cfg.watch, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Prune only the top-level build/ and .git, matching
			// find's "-path ./build -prune".
			if path == "build" || path == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") && name != "go.mod" && name != "go.sum" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(stampTime) {
			return errChanged
		}
		return nil
	})
	if errors.Is(err, errChanged) {
		return true, nil
	}
	return false, err
}
