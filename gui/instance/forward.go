//go:build (unix || windows) && !ios && !android

package instance

import (
	"fmt"
	"io"
)

// forward sends args to the primary over conn and waits for its
// acknowledgement, so a secondary exits only once the primary has the
// arguments.
func forward(conn io.ReadWriteCloser, args []string) error {
	defer func() { _ = conn.Close() }()
	if err := writeMsg(conn, args); err != nil {
		return fmt.Errorf("instance: forward: %w", err)
	}
	var ack [1]byte
	if _, err := io.ReadFull(conn, ack[:]); err != nil || ack[0] != ackByte {
		return fmt.Errorf("instance: forward: no acknowledgement: %v", err)
	}
	return nil
}
