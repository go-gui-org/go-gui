// Package framestats keeps the frame times of the benchmark examples and
// reports the frame rate and the frame time percentiles (#1024).
//
// The frame rate is frames divided by the sum of the frame times. It is not
// the mean of per-frame FPS values: that mean gives too much weight to short
// frames. A mean also hides stutter, so the summary also gives p50, p99 and
// the maximum frame time.
package framestats

import (
	"slices"
	"time"
)

// Size is the number of frame times a Stats keeps. When it is full, each new
// frame replaces the oldest one. 4096 frames is about 17 s at 240 Hz, so a
// 5 s measurement keeps all its frames up to about 800 FPS.
const Size = 4096

// Stats is a ring buffer of frame times. The zero value is ready to use.
// It is large (64 KiB), so keep it in a struct, not on the stack of each frame.
type Stats struct {
	buf  [Size]time.Duration
	pos  int // slot the next Add writes
	n    int // frames in buf, up to Size
	sort [Size]time.Duration
}

// Summary is the result of Stats.Summary. All fields are zero when no frame
// was added.
type Summary struct {
	Frames int
	FPS    float64 // Frames / sum of the frame times
	Mean   time.Duration
	P50    time.Duration
	P99    time.Duration
	Max    time.Duration
}

// Add records one frame time. A negative dt (a clock that stepped back) is kept
// as 0, so it cannot pull the sum, the mean and the FPS below their real values.
func (s *Stats) Add(dt time.Duration) {
	s.buf[s.pos] = max(dt, 0)
	s.pos = (s.pos + 1) % Size
	s.n = min(s.n+1, Size)
}

// Len is the number of frame times kept.
func (s *Stats) Len() int { return s.n }

// Reset drops all frame times.
func (s *Stats) Reset() {
	s.pos = 0
	s.n = 0
}

// Summary reports the kept frame times. It sorts a copy in a scratch array of
// the Stats, so it does not allocate, but it costs a sort of up to Size
// values. Call it when the readout changes, not on each frame.
func (s *Stats) Summary() Summary {
	if s.n == 0 {
		return Summary{}
	}
	// The kept frames are buf[:n] in any order: before the ring wraps they
	// are the first n slots, after it wraps they are all Size slots.
	v := s.sort[:s.n]
	copy(v, s.buf[:s.n])
	slices.Sort(v)

	var sum time.Duration
	for _, d := range v {
		sum += d
	}
	out := Summary{
		Frames: s.n,
		Mean:   sum / time.Duration(s.n),
		P50:    rank(v, 50),
		P99:    rank(v, 99),
		Max:    v[len(v)-1],
	}
	if sum > 0 {
		out.FPS = float64(s.n) / sum.Seconds()
	}
	return out
}

// rank returns the p-th percentile of the sorted v by the nearest-rank method:
// the smallest value that is at least p percent of the values.
func rank(v []time.Duration, p int) time.Duration {
	// ceil(p*n/100) - 1, in integers.
	i := (p*len(v)+99)/100 - 1
	return v[max(i, 0)]
}

// Ms converts d to milliseconds, for a readout or a CSV column.
func Ms(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}
