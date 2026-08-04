//go:build linux && !android && !otoaudio

package audio

import (
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/jfreymuth/pulse"
)

// Pure-Go PulseAudio output sink (github.com/jfreymuth/pulse), the
// default Linux path.  See the package doc in backend.go for how the
// sinks are selected.

// pulseRequestTimeout bounds every protocol round-trip so a wedged
// server fails Init instead of hanging app startup indefinitely.
const pulseRequestTimeout = 2 * time.Second

// pulseOut is the process-wide sink state.  mu guards mixer and scratch,
// which the pulse reader goroutine and outputPlay both touch; client and
// stream are written only by outputInit/outputClose.
var pulseOut struct {
	mu      sync.Mutex
	mixer   beep.Mixer
	scratch [][2]float64
	client  *pulse.Client
	stream  *pulse.PlaybackStream
}

func outputInit(sr beep.SampleRate, bufferSize int) error {
	c, err := pulse.NewClient(
		pulse.ClientApplicationName("go-gui"),
		pulse.ClientTimeout(pulseRequestTimeout),
	)
	if err != nil {
		return err
	}

	// The reader runs on pulse's own goroutine; pulseOut.mu guards the
	// mixer against outputPlay, mirroring what beep's speaker does
	// internally.
	reader := pulse.Float32Reader(func(out []float32) (int, error) {
		frames := len(out) / 2
		pulseOut.mu.Lock()
		if cap(pulseOut.scratch) < frames {
			// Safety net only: outputInit presizes to the stream's own
			// buffer, so this must not allocate on the audio thread.
			pulseOut.scratch = make([][2]float64, frames)
		}
		buf := pulseOut.scratch[:frames]
		pulseOut.mixer.Stream(buf) // always fills buf (silence when empty)
		pulseOut.mu.Unlock()
		for i, s := range buf {
			out[2*i] = clampSample(s[0])
			out[2*i+1] = clampSample(s[1])
		}
		return len(out), nil
	})

	// Order matters: sample rate and channels before buffer size.
	s, err := c.NewPlayback(reader,
		pulse.PlaybackStereo,
		pulse.PlaybackSampleRate(int(sr)),
		pulse.PlaybackBufferSize(bufferSize),
	)
	if err != nil {
		c.Close()
		return err
	}

	pulseOut.client = c
	pulseOut.stream = s
	// Presize the mix scratch before playback starts; growing it from
	// inside the reader would allocate on the real-time audio thread.
	pulseOut.mu.Lock()
	pulseOut.scratch = make([][2]float64, max(s.BufferSize(), bufferSize))
	pulseOut.mu.Unlock()
	s.Start()
	return nil
}

func outputPlay(s beep.Streamer) {
	pulseOut.mu.Lock()
	pulseOut.mixer.Add(s)
	pulseOut.mu.Unlock()
}

func outputClose() {
	if pulseOut.stream != nil {
		pulseOut.stream.Stop()
		pulseOut.stream.Close()
		pulseOut.stream = nil
	}
	if pulseOut.client != nil {
		pulseOut.client.Close()
		pulseOut.client = nil
	}
	pulseOut.mu.Lock()
	pulseOut.mixer.Clear()
	pulseOut.scratch = nil
	pulseOut.mu.Unlock()
}

// clampSample limits a sample to [-1, 1] and converts to float32,
// preventing wrap-around distortion when summed channels exceed unit
// range.
func clampSample(v float64) float32 {
	return float32(max(-1, min(1, v)))
}
