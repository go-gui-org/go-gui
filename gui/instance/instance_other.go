//go:build !(unix || windows) || ios || android

package instance

import (
	"errors"
	"io"
)

// platformLock on web and mobile: the platform runs one copy of the
// app already, so every process is the primary and nothing listens.
type platformLock struct {
	done chan struct{}
}

func (p *platformLock) acquire(string, []string) (bool, error) {
	p.done = make(chan struct{})
	return true, nil
}

// accept blocks until Close, so the accept loop has nothing to serve.
func (p *platformLock) accept() (io.ReadWriteCloser, error) {
	<-p.done
	return nil, errors.New("instance: closed")
}

func (p *platformLock) closeListener(<-chan struct{}) error {
	close(p.done)
	return nil
}

func (p *platformLock) release() error { return nil }

// forwardName has no primary to reach: no other process can hold the
// election here.
func forwardName(string, []string) error { return errNoPrimary }
