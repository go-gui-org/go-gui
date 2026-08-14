//go:build !js && !android && !ios

package audio

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/flac"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/vorbis"
	"github.com/gopxl/beep/v2/wav"
)

// ---------------------------------------------------------------------------
// decode helpers
// ---------------------------------------------------------------------------

// decodeReader detects the audio format from file extension and decodes.
func decodeReader(ext string, rc interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
	Close() error
}) (beep.StreamSeekCloser, beep.Format, error) {
	switch ext {
	case ".wav":
		stream, format, err := wav.Decode(rc)
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("audio: wav: %w", err)
		}
		return stream, format, nil
	case ".mp3":
		stream, format, err := mp3.Decode(rc.(io.ReadCloser))
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("audio: mp3: %w", err)
		}
		return stream, format, nil
	case ".ogg":
		stream, format, err := vorbis.Decode(rc.(io.ReadCloser))
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("audio: ogg: %w", err)
		}
		return stream, format, nil
	case ".flac":
		stream, format, err := flac.Decode(rc.(io.ReadCloser))
		if err != nil {
			return nil, beep.Format{}, fmt.Errorf("audio: flac: %w", err)
		}
		return stream, format, nil
	default:
		return nil, beep.Format{}, fmt.Errorf("audio: unsupported audio format %q", ext)
	}
}

// decodeBytes decodes in-memory audio data.  Detects format by magic
// bytes and delegates to the appropriate decoder.
func decodeBytes(data []byte) (beep.StreamSeekCloser, beep.Format, error) {
	if len(data) < 4 {
		return nil, beep.Format{}, fmt.Errorf(
			"audio: data too short (%d bytes)", len(data))
	}

	switch {
	case string(data[:4]) == "RIFF":
		return wav.Decode(newReadSeekCloser(data))
	case data[0] == 0xFF && len(data) > 1 && data[1]&0xE0 == 0xE0:
		return mp3.Decode(newReadCloser(data))
	case string(data[:4]) == "OggS":
		return vorbis.Decode(newReadCloser(data))
	case string(data[:4]) == "fLaC":
		return flac.Decode(newReadCloser(data))
	default:
		return nil, beep.Format{}, fmt.Errorf(
			"audio: unrecognized audio format (magic: % x)", data[:min(4, len(data))])
	}
}

// readSeekCloser wraps a [bytes.Reader] into [io.ReadSeekCloser].
type readSeekCloser struct {
	r      *bytes.Reader
	closed bool
}

func newReadSeekCloser(data []byte) *readSeekCloser {
	return &readSeekCloser{r: bytes.NewReader(data)}
}

// readCloser wraps a [bytes.Reader] into [io.ReadCloser].
type readCloser struct {
	r      *bytes.Reader
	closed bool
}

func newReadCloser(data []byte) *readCloser {
	return &readCloser{r: bytes.NewReader(data)}
}

func (r *readCloser) Read(p []byte) (int, error) {
	if r.closed {
		return 0, errors.New("audio: reader closed")
	}
	return r.r.Read(p)
}

func (r *readCloser) Close() error {
	r.closed = true
	return nil
}

func (r *readSeekCloser) Read(p []byte) (int, error) {
	if r.closed {
		return 0, errors.New("audio: reader closed")
	}
	return r.r.Read(p)
}

func (r *readSeekCloser) Seek(offset int64, whence int) (int64, error) {
	if r.closed {
		return 0, errors.New("audio: reader closed")
	}
	return r.r.Seek(offset, whence)
}

func (r *readSeekCloser) Close() error {
	r.closed = true
	return nil
}
