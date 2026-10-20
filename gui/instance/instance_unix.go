//go:build unix && !ios && !android

package instance

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// maxSocketPath is the shortest sun_path limit of the supported
// systems (macOS: 104 bytes including the NUL).
const maxSocketPath = 103

// platformLock holds the flock'd file and the socket listener.
type platformLock struct {
	file     *os.File
	ln       *net.UnixListener
	sockPath string
}

func (p *platformLock) acquire(name string, args []string) (bool, error) {
	dir, sockPath, err := paths(name)
	if err != nil {
		return true, err
	}
	// O_NOFOLLOW: a symlink planted at the lock path must not make
	// this process create or lock some other file.
	//
	// #nosec G304 — the path is the checked private lock directory plus
	// a name hashed from the app ID; no caller input reaches it.
	f, err := os.OpenFile(filepath.Join(dir, name+".lock"),
		os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o600)
	if err != nil {
		return true, fmt.Errorf("instance: open lock: %w", err)
	}
	// The lock file is never removed. Removing it would let a second
	// process lock a new file at the same path while the first still
	// holds the old one, and both would be primary.
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return false, forwardUnix(sockPath, args)
		}
		return true, fmt.Errorf("instance: flock: %w", err)
	}
	// Holding the lock proves no live primary owns the socket path,
	// so a socket left there is from a process that died. Remove it
	// before bind, or bind fails with EADDRINUSE.
	if err := os.Remove(sockPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		_ = f.Close()
		return true, fmt.Errorf("instance: remove stale socket: %w", err)
	}
	ln, err := net.ListenUnix("unix", &net.UnixAddr{Name: sockPath, Net: "unix"})
	if err != nil {
		_ = f.Close()
		return true, fmt.Errorf("instance: listen: %w", err)
	}
	// The directory is already private; this also covers an
	// $XDG_RUNTIME_DIR with a looser mode than the spec asks for.
	_ = os.Chmod(sockPath, 0o600)
	p.file, p.ln, p.sockPath = f, ln, sockPath
	return true, nil
}

// paths returns the lock directory and the socket path for name.
func paths(name string) (dir, sockPath string, err error) {
	dir, err = lockDir()
	if err != nil {
		return "", "", err
	}
	sockPath = filepath.Join(dir, name+".sock")
	if len(sockPath) > maxSocketPath {
		return "", "", fmt.Errorf("instance: socket path %q is longer than %d bytes", sockPath, maxSocketPath)
	}
	return dir, sockPath, nil
}

// forwardName sends args to the primary of the election name.
func forwardName(name string, args []string) error {
	_, sockPath, err := paths(name)
	if err != nil {
		return err
	}
	return forwardUnix(sockPath, args)
}

// lockDir returns the per-user directory for the lock and socket.
// $XDG_RUNTIME_DIR is private to the user by spec. Otherwise a
// gogui-<uid> directory in os.TempDir is created 0700 and checked: on
// a shared /tmp another user could have made it first.
func lockDir() (string, error) {
	if d := os.Getenv("XDG_RUNTIME_DIR"); filepath.IsAbs(d) {
		return d, nil
	}
	uid := os.Getuid()
	dir := filepath.Join(os.TempDir(), "gogui-"+strconv.Itoa(uid))
	if err := os.Mkdir(dir, 0o700); err != nil && !errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("instance: create %s: %w", dir, err)
	}
	var st unix.Stat_t
	if err := unix.Lstat(dir, &st); err != nil {
		return "", fmt.Errorf("instance: stat %s: %w", dir, err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return "", fmt.Errorf("instance: %s is not a directory", dir)
	}
	if int(st.Uid) != uid {
		return "", fmt.Errorf("instance: %s is owned by uid %d, not %d", dir, st.Uid, uid)
	}
	if st.Mode&0o077 != 0 {
		return "", fmt.Errorf("instance: %s has mode %o; want 0700", dir, st.Mode&0o777)
	}
	return dir, nil
}

// forwardUnix dials the primary and sends args. The primary may hold
// the lock but not listen yet (it is between flock and listen), so a
// refused or missing socket is retried until dialTimeout.
func forwardUnix(sockPath string, args []string) error {
	deadline := time.Now().Add(dialTimeout)
	for {
		conn, err := net.DialTimeout("unix", sockPath, ioTimeout)
		if err == nil {
			_ = conn.SetDeadline(time.Now().Add(ioTimeout))
			return forward(conn, args)
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("%w: %v", errNoPrimary, err)
		}
		time.Sleep(dialInterval)
	}
}

func (p *platformLock) accept() (io.ReadWriteCloser, error) {
	conn, err := p.ln.AcceptUnix()
	if err != nil {
		return nil, err
	}
	// A client that connects and then stalls must not hold the
	// accept loop.
	_ = conn.SetDeadline(time.Now().Add(ioTimeout))
	return conn, nil
}

// closeListener stops accept. UnixListener removes the socket file.
func (p *platformLock) closeListener(<-chan struct{}) error {
	return p.ln.Close()
}

// release drops the flock by closing the file.
func (p *platformLock) release() error {
	return p.file.Close()
}
