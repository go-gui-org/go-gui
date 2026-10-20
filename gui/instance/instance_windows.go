//go:build windows

package instance

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// pipeBuf is the pipe's in/out buffer size hint. One message is
// normally a few hundred bytes.
const pipeBuf = 4096

// platformLock holds the named mutex and the current pipe instance.
type platformLock struct {
	mutex    windows.Handle
	pipe     windows.Handle // instance waiting for the next client
	pipeName *uint16
	sa       *windows.SecurityAttributes
	closed   atomic.Bool
}

func (p *platformLock) acquire(name string, args []string) (bool, error) {
	pipeName, sa, err := pipeSetup(name)
	if err != nil {
		return true, err
	}
	mutexName, err := windows.UTF16PtrFromString(`Local\` + name)
	if err != nil {
		return true, err
	}
	// CreateMutex returns a valid handle together with
	// ERROR_ALREADY_EXISTS when another process made the mutex first.
	// The kernel deletes the mutex when its last handle closes, also
	// when the process is killed.
	h, err := windows.CreateMutex(sa, false, mutexName)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		_ = windows.CloseHandle(h)
		return false, forwardWindows(pipeName, args)
	}
	if err != nil {
		return true, fmt.Errorf("instance: create mutex: %w", err)
	}
	p.mutex, p.pipeName, p.sa = h, pipeName, sa
	if err := p.newPipe(); err != nil {
		_ = windows.CloseHandle(h)
		return true, err
	}
	return true, nil
}

// pipeSetup returns the pipe name for election name and the security
// attributes for the mutex and pipe.
func pipeSetup(name string) (*uint16, *windows.SecurityAttributes, error) {
	sid, err := currentUserSID()
	if err != nil {
		return nil, nil, fmt.Errorf("instance: current user: %w", err)
	}
	// Only the current user gets any access, to the mutex and to the
	// pipe. P protects the DACL from inherited entries.
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;GA;;;" + sid + ")")
	if err != nil {
		return nil, nil, fmt.Errorf("instance: security descriptor: %w", err)
	}
	sa := &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}
	// The pipe namespace is machine-wide, the Local\ mutex namespace is
	// per session. The user SID in the pipe name keeps two users'
	// copies of one app apart.
	sum := sha256.Sum256([]byte(sid))
	pipeName, err := windows.UTF16PtrFromString(`\\.\pipe\` + name + "-" + hex.EncodeToString(sum[:4]))
	if err != nil {
		return nil, nil, err
	}
	return pipeName, sa, nil
}

// forwardName sends args to the primary of the election name.
func forwardName(name string, args []string) error {
	pipeName, _, err := pipeSetup(name)
	if err != nil {
		return err
	}
	return forwardWindows(pipeName, args)
}

// newPipe creates the instance the next client connects to.
// FILE_FLAG_FIRST_PIPE_INSTANCE makes creation fail when another
// process already owns the name, so a squatter cannot sit between
// this primary and its secondaries.
func (p *platformLock) newPipe() error {
	h, err := windows.CreateNamedPipe(p.pipeName,
		windows.PIPE_ACCESS_DUPLEX|windows.FILE_FLAG_FIRST_PIPE_INSTANCE,
		windows.PIPE_TYPE_BYTE|windows.PIPE_READMODE_BYTE|windows.PIPE_WAIT|
			windows.PIPE_REJECT_REMOTE_CLIENTS,
		1, pipeBuf, pipeBuf, 0, p.sa)
	if err != nil {
		return fmt.Errorf("instance: create pipe: %w", err)
	}
	p.pipe = h
	return nil
}

// currentUserSID returns the process user's SID in string form.
func currentUserSID() (string, error) {
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return "", err
	}
	return u.User.Sid.String(), nil
}

// forwardWindows opens the primary's pipe and sends args. The pipe
// is missing while the primary starts or between two clients, and
// busy while it serves another, so both are retried until
// dialTimeout.
func forwardWindows(pipeName *uint16, args []string) error {
	deadline := time.Now().Add(dialTimeout)
	for {
		h, err := windows.CreateFile(pipeName,
			windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
			windows.OPEN_EXISTING, 0, 0)
		if err == nil {
			return forwardTimed(h, args)
		}
		retry := errors.Is(err, windows.ERROR_FILE_NOT_FOUND) ||
			errors.Is(err, windows.ERROR_PIPE_BUSY)
		if !retry || time.Now().After(deadline) {
			return fmt.Errorf("%w: %v", errNoPrimary, err)
		}
		time.Sleep(dialInterval)
	}
}

// forwardTimed runs forward on h with the ioTimeout the Unix socket
// gets from SetDeadline. A synchronous pipe has no deadline, so a
// primary that stopped serving (hung, or suspended in a debugger) would
// block the secondary in the write or the ack read for ever. On timeout
// CancelIoEx aborts the blocked call; forward then closes h itself.
func forwardTimed(h windows.Handle, args []string) error {
	errc := make(chan error, 1)
	go func() { errc <- forward(os.NewFile(uintptr(h), "instance-pipe"), args) }()
	t := time.NewTimer(ioTimeout)
	defer t.Stop()
	select {
	case err := <-errc:
		return err
	case <-t.C:
		_ = windows.CancelIoEx(h, nil)
		return fmt.Errorf("%w: no reply in %v", errNoPrimary, ioTimeout)
	}
}

// accept waits for a client on the current instance and hands it
// out as a connection. Closing that connection closes the instance,
// so a new one is created first for the next client.
//
// The pipe is synchronous, so a client that connects and stalls
// holds the loop. Only the current user can connect, and that user
// can stop the primary in other ways anyway.
//
// closed is checked only after the connect, never before: closeListener
// wakes the loop by dialing, and the dial needs an instance to reach.
func (p *platformLock) accept() (io.ReadWriteCloser, error) {
	if p.pipe == 0 {
		if err := p.newPipe(); err != nil {
			return nil, err
		}
	}
	h := p.pipe
	err := windows.ConnectNamedPipe(h, nil)
	// ERROR_PIPE_CONNECTED: the client connected between create and
	// connect; that is a success.
	if err != nil && !errors.Is(err, windows.ERROR_PIPE_CONNECTED) {
		// The instance is dead (ERROR_NO_DATA: a client connected and
		// closed before the connect call). ConnectNamedPipe on it fails
		// again every time, so drop it; the next accept makes a new one.
		_ = windows.CloseHandle(h)
		p.pipe = 0
		return nil, err
	}
	if p.closed.Load() {
		return nil, errors.New("instance: closed")
	}
	p.pipe = 0
	return os.NewFile(uintptr(h), "instance-pipe"), nil
}

// closeListener wakes the accept loop. ConnectNamedPipe on a
// synchronous pipe cannot be cancelled, so this connects to the pipe
// itself; accept sees closed and returns.
//
// The dial is retried: between two clients the loop has no instance
// yet (not found), and while it serves one the instance is busy.
// Either way the loop reaches ConnectNamedPipe soon after, sees this
// connection and returns.
//
// The loop can also stop without the dial: newPipe fails (for example
// ERROR_ACCESS_DENIED from FILE_FLAG_FIRST_PIPE_INSTANCE while the last
// client still holds its end) and the loop sees closed. Then no instance
// will ever appear, so done ends the retry; without it Close would time
// out and keep the election until the process exits.
func (p *platformLock) closeListener(done <-chan struct{}) error {
	p.closed.Store(true)
	deadline := time.Now().Add(dialTimeout)
	for {
		select {
		case <-done:
			return nil
		default:
		}
		h, err := windows.CreateFile(p.pipeName,
			windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil,
			windows.OPEN_EXISTING, 0, 0)
		if err == nil {
			_ = windows.CloseHandle(h)
			return nil
		}
		retry := errors.Is(err, windows.ERROR_FILE_NOT_FOUND) ||
			errors.Is(err, windows.ERROR_PIPE_BUSY)
		if !retry || time.Now().After(deadline) {
			return fmt.Errorf("instance: wake accept loop: %w", err)
		}
		time.Sleep(dialInterval)
	}
}

// release closes the last pipe instance and the mutex. Runs after the
// accept loop exits, so p.pipe is no longer shared.
func (p *platformLock) release() error {
	if p.pipe != 0 {
		_ = windows.CloseHandle(p.pipe)
		p.pipe = 0
	}
	return windows.CloseHandle(p.mutex)
}
