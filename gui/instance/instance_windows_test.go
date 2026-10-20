package instance

import (
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// A client that keeps its pipe handle open after the ack can make the
// accept loop's next newPipe fail, so the loop exits on closed without
// an instance for closeListener to dial. Close must still return
// promptly and release the election.
func TestCloseWhileClientHoldsPipe(t *testing.T) {
	id := testID(t)
	l := mustPrimary(t, id)
	rec := newRecorder()
	l.attachFunc(rec.fn, nil)

	h, err := windows.CreateFile(l.plat.pipeName,
		windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
		windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	client := os.NewFile(uintptr(h), "client")
	defer func() { _ = client.Close() }()
	if err := writeMsg(client, []string{"held"}); err != nil {
		t.Fatal(err)
	}
	var ack [1]byte
	if _, err := client.Read(ack[:]); err != nil {
		t.Fatal(err)
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

// A client that connects and closes without writing leaves the pipe
// instance dead (ERROR_NO_DATA). The primary must drop it and still
// serve the next launch.
func TestPrimaryServesAfterEmptyClient(t *testing.T) {
	id := testID(t)
	l := mustPrimary(t, id)
	rec := newRecorder()
	l.attachFunc(rec.fn, nil)
	for range 3 {
		h, err := windows.CreateFile(l.plat.pipeName,
			windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
			windows.OPEN_EXISTING, 0, 0)
		if err == nil {
			_ = windows.CloseHandle(h)
		}
	}
	if err := forwardTo(id, []string{"after"}); err != nil {
		t.Fatalf("forward after empty clients: %v", err)
	}
	if got := rec.wait(t, 1); got[0][0] != "after" {
		t.Fatalf("got %q", got)
	}
}

// A primary that accepts but never answers must not hang the
// secondary: forward gives up after ioTimeout.
func TestForwardTimesOutOnSilentPrimary(t *testing.T) {
	name, err := windows.UTF16PtrFromString(`\\.\pipe\gogui-test-silent-` + strconv.Itoa(os.Getpid()))
	if err != nil {
		t.Fatal(err)
	}
	srv, err := windows.CreateNamedPipe(name, windows.PIPE_ACCESS_DUPLEX,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT,
		1, pipeBuf, pipeBuf, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = windows.CloseHandle(srv) }()
	go func() { _ = windows.ConnectNamedPipe(srv, nil) }() // never reads, never acks

	start := time.Now()
	err = forwardWindows(name, []string{"x"})
	if !errors.Is(err, errNoPrimary) {
		t.Fatalf("err = %v, want errNoPrimary", err)
	}
	if d := time.Since(start); d > ioTimeout+dialTimeout {
		t.Fatalf("forward took %v", d)
	}
}
