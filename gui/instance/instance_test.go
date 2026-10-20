package instance

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Environment for the helper process the tests re-exec.
const (
	envHelper = "GOGUI_INSTANCE_HELPER"
	envID     = "GOGUI_INSTANCE_ID"
)

func TestMain(m *testing.M) {
	if os.Getenv(envHelper) != "" {
		os.Exit(helperMain())
	}
	os.Exit(m.Run())
}

// helperMain is a primary in its own process: it reports the election
// result, prints each forwarded message, and exits when stdin closes.
func helperMain() int {
	l, primary, err := acquire(os.Getenv(envID), nil)
	if err != nil || !primary {
		fmt.Printf("not-primary %v\n", err)
		return 1
	}
	l.attachFunc(func(args []string) {
		fmt.Printf("got %s\n", strings.Join(args, "|"))
	}, nil)
	fmt.Println("primary")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
	_ = l.Close()
	return 0
}

// testID returns an app ID no other test or test run shares, and on
// Unix points the lock directory at a fresh private one.
func testID(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "js" {
		// The web build has no election: every process is primary.
		t.Skip("no single-instance election on js")
	}
	if runtime.GOOS != "windows" {
		// os.MkdirTemp(""): t.TempDir is too long for a macOS socket
		// path once the test name is in it.
		dir, err := os.MkdirTemp("", "gi")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		t.Setenv("XDG_RUNTIME_DIR", dir)
	}
	return fmt.Sprintf("org.go-gui.test.%s.%d.%d", t.Name(), os.Getpid(), time.Now().UnixNano())
}

// recorder collects forwarded messages.
type recorder struct {
	mu   sync.Mutex
	msgs [][]string
	ch   chan struct{}
}

func newRecorder() *recorder { return &recorder{ch: make(chan struct{}, 64)} }

func (r *recorder) fn(args []string) {
	r.mu.Lock()
	r.msgs = append(r.msgs, args)
	r.mu.Unlock()
	r.ch <- struct{}{}
}

func (r *recorder) wait(t *testing.T, n int) [][]string {
	t.Helper()
	for range n {
		select {
		case <-r.ch:
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for message")
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([][]string(nil), r.msgs...)
}

func mustPrimary(t *testing.T, id string) *Lock {
	t.Helper()
	l, primary, err := acquire(id, nil)
	if err != nil || !primary || l == nil {
		t.Fatalf("acquire = (%v, %v, %v), want primary", l, primary, err)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func TestSecondaryForwardsArgs(t *testing.T) {
	id := testID(t)
	l := mustPrimary(t, id)
	rec := newRecorder()
	l.attachFunc(rec.fn, nil)

	l2, primary, err := acquire(id, []string{"a.txt", "b c.txt", "my://x"})
	if err != nil || primary || l2 != nil {
		t.Fatalf("second acquire = (%v, %v, %v), want forwarded", l2, primary, err)
	}
	got := rec.wait(t, 1)
	if want := "a.txt|b c.txt|my://x"; strings.Join(got[0], "|") != want {
		t.Fatalf("forwarded %q, want %q", got[0], want)
	}
}

func TestForwardBeforeAttachIsKept(t *testing.T) {
	id := testID(t)
	l := mustPrimary(t, id)
	if _, primary, err := acquire(id, []string{"early"}); err != nil || primary {
		t.Fatalf("second acquire = (%v, %v), want forwarded", primary, err)
	}
	rec := newRecorder()
	if !l.attachFunc(rec.fn, nil) {
		t.Fatal("first attachFunc reported false")
	}
	if got := rec.wait(t, 1); got[0][0] != "early" {
		t.Fatalf("got %q, want early", got)
	}
	if l.attachFunc(rec.fn, nil) {
		t.Fatal("second attachFunc reported true")
	}
}

// This launch started before any launch it received, so its own
// arguments reach the delivery function before the forwarded ones
// that arrived while the app started.
func TestAttachDeliversOwnArgsFirst(t *testing.T) {
	id := testID(t)
	l := mustPrimary(t, id)
	if _, primary, err := acquire(id, []string{"second"}); err != nil || primary {
		t.Fatalf("second acquire = (%v, %v), want forwarded", primary, err)
	}
	rec := newRecorder()
	l.attachFunc(rec.fn, []string{"own"})
	got := rec.wait(t, 2)
	if got[0][0] != "own" || got[1][0] != "second" {
		t.Fatalf("delivery order %q, want own then second", got)
	}
}

// Close after a served client must wake the accept loop at once. On
// Windows the loop has no pipe instance between two clients, and the
// wake dial needs one: a closed check before creating it left Close
// to time out and keep the election.
func TestCloseAfterForwardIsPrompt(t *testing.T) {
	id := testID(t)
	l, primary, err := acquire(id, nil)
	if err != nil || !primary {
		t.Fatalf("acquire = (%v, %v)", primary, err)
	}
	rec := newRecorder()
	l.attachFunc(rec.fn, nil)
	if _, primary, err := acquire(id, []string{"x"}); err != nil || primary {
		t.Fatalf("second acquire = (%v, %v), want forwarded", primary, err)
	}
	rec.wait(t, 1)
	start := time.Now()
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if d := time.Since(start); d > dialTimeout/2 {
		t.Fatalf("Close took %v", d)
	}
	mustPrimary(t, id)
}

func TestCloseReleasesElection(t *testing.T) {
	id := testID(t)
	l, primary, err := acquire(id, nil)
	if err != nil || !primary {
		t.Fatalf("acquire = (%v, %v)", primary, err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := l.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	mustPrimary(t, id)
}

func TestConcurrentRacersElectOne(t *testing.T) {
	id := testID(t)
	const n = 8
	var primaries atomic.Int32
	var wg sync.WaitGroup
	locks := make(chan *Lock, n)
	rec := newRecorder()
	for i := range n {
		wg.Go(func() {
			l, primary, err := acquire(id, []string{strconv.Itoa(i)})
			if err != nil {
				t.Errorf("racer %d: %v", i, err)
				return
			}
			if primary {
				primaries.Add(1)
				l.attachFunc(rec.fn, nil)
				locks <- l
			}
		})
	}
	wg.Wait()
	close(locks)
	if got := primaries.Load(); got != 1 {
		t.Fatalf("%d primaries, want 1", got)
	}
	rec.wait(t, n-1)
	for l := range locks {
		_ = l.Close()
	}
}

func TestPrimaryInOtherProcess(t *testing.T) {
	id := testID(t)
	cmd, out, stdin := startHelper(t, id)
	if _, primary, err := acquire(id, []string{"x"}); err != nil || primary {
		t.Fatalf("acquire = (%v, %v), want forwarded", primary, err)
	}
	if line := readLine(t, out); line != "got x" {
		t.Fatalf("helper printed %q, want %q", line, "got x")
	}
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("helper: %v", err)
	}
	// The helper closed its lock: this process wins now.
	mustPrimary(t, id)
}

// TestStaleSocketAfterKill: a primary killed with SIGKILL runs no
// cleanup and leaves its socket file. The kernel drops its flock, so
// the next process must become primary and replace the socket.
func TestStaleSocketAfterKill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no socket file on Windows; the kernel frees the mutex")
	}
	id := testID(t)
	cmd, _, _ := startHelper(t, id)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	sock := os.Getenv("XDG_RUNTIME_DIR") + "/" + lockName(id) + ".sock"
	if _, err := os.Stat(sock); err != nil {
		t.Fatalf("killed primary left no socket (%v); test proves nothing", err)
	}
	l := mustPrimary(t, id)
	rec := newRecorder()
	l.attachFunc(rec.fn, nil)
	if _, primary, err := acquire(id, []string{"after"}); err != nil || primary {
		t.Fatalf("acquire = (%v, %v), want forwarded", primary, err)
	}
	if got := rec.wait(t, 1); got[0][0] != "after" {
		t.Fatalf("got %q", got)
	}
}

func TestEmptyIDRunsAsPrimary(t *testing.T) {
	l, primary, err := acquire("", nil)
	if err == nil || !primary || l != nil {
		t.Fatalf("acquire(\"\") = (%v, %v, %v), want (nil, true, err)", l, primary, err)
	}
}

func TestNilLockSafe(t *testing.T) {
	var l *Lock
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMsgRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	in := []string{"", "a", "ünïcode", strings.Repeat("x", 1000)}
	if err := writeMsg(&buf, in); err != nil {
		t.Fatal(err)
	}
	out, err := readMsg(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(out, "|") != strings.Join(in, "|") || len(out) != len(in) {
		t.Fatalf("round trip %q, want %q", out, in)
	}
}

func TestMsgLimits(t *testing.T) {
	if err := writeMsg(&bytes.Buffer{}, make([]string, maxArgs+1)); err == nil {
		t.Error("writeMsg accepted too many arguments")
	}
	if err := writeMsg(&bytes.Buffer{}, []string{strings.Repeat("x", maxArgBytes+1)}); err == nil {
		t.Error("writeMsg accepted an oversized argument")
	}
	u32 := func(vs ...uint32) *bytes.Reader {
		var b []byte
		for _, v := range vs {
			b = binary.BigEndian.AppendUint32(b, v)
		}
		return bytes.NewReader(b)
	}
	cases := map[string]*bytes.Reader{
		"count":         u32(maxArgs + 1),
		"arg length":    u32(1, maxArgBytes+1),
		"huge length":   u32(1, 0xFFFFFFFF),
		"truncated":     u32(2, 1),
		"truncated arg": u32(2, maxArgBytes),
	}
	for name, r := range cases {
		if _, err := readMsg(r); err == nil {
			t.Errorf("%s: readMsg accepted a bad message", name)
		}
	}
}

func TestPrimaryRejectsOversizedAndKeepsServing(t *testing.T) {
	id := testID(t)
	l := mustPrimary(t, id)
	rec := newRecorder()
	l.attachFunc(rec.fn, nil)
	// A client over the limit gets no acknowledgement...
	if err := forwardTo(id, make([]string, maxArgs+1)); err == nil {
		t.Fatal("oversized forward reported success")
	}
	// ...and the primary still serves the next one.
	if _, primary, err := acquire(id, []string{"ok"}); err != nil || primary {
		t.Fatalf("acquire = (%v, %v), want forwarded", primary, err)
	}
	if got := rec.wait(t, 1); got[0][0] != "ok" {
		t.Fatalf("got %q", got)
	}
}

// forwardTo sends args the way a secondary does, without running the
// election.
func forwardTo(id string, args []string) error {
	return forwardName(lockName(id), args)
}

func startHelper(t *testing.T, id string) (*exec.Cmd, *bufio.Reader, interface{ Close() error }) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), envHelper+"=1", envID+"="+id)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	out := bufio.NewReader(stdout)
	if line := readLine(t, out); line != "primary" {
		t.Fatalf("helper printed %q, want primary", line)
	}
	return cmd, out, stdin
}

func readLine(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	ch := make(chan string, 1)
	go func() {
		s, _ := r.ReadString('\n')
		ch <- strings.TrimSpace(s)
	}()
	select {
	case s := <-ch:
		return s
	case <-time.After(10 * time.Second):
		t.Fatal("timed out reading helper output")
		return ""
	}
}

// Messages past maxPending before Attach are dropped after the ack, so
// the sender exits believing they arrived. The drop must be logged.
func TestPendingOverflowLogged(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	l := &Lock{}
	for range maxPending + 3 {
		l.receive([]string{"x"})
	}
	if len(l.pending) != maxPending {
		t.Fatalf("pending = %d, want %d", len(l.pending), maxPending)
	}
	if n := strings.Count(buf.String(), "dropped"); n != 1 {
		t.Fatalf("logged %d drop lines, want 1: %q", n, buf.String())
	}
}
