//go:build linux && !js && !android

package gl

import (
	"sync/atomic"
	"testing"
	"time"
)

// stubReducedMotion swaps in a query that counts its calls and returns
// what *reading holds, and clears the cache so the next call starts cold.
// Both are put back when the test ends.
func stubReducedMotion(t *testing.T, reading *atomic.Uint32) *atomic.Int32 {
	t.Helper()
	oldQuery, oldTTL := reducedMotionQuery, reducedMotionTTL
	var calls atomic.Int32
	resetReducedMotionCache()
	reducedMotionQuery = func() (bool, bool) {
		calls.Add(1)
		v := reading.Load()
		return v == rmReduced, v != rmNone
	}
	t.Cleanup(func() {
		resetReducedMotionCache()
		reducedMotionQuery, reducedMotionTTL = oldQuery, oldTTL
	})
	return &calls
}

// PrefersReducedMotion passes the gsettings reading through: only a
// known reduced report counts (issue #757).
func TestPrefersReducedMotionPassesQuery(t *testing.T) {
	n := &nativePlatform{}
	var reading atomic.Uint32
	stubReducedMotion(t, &reading)

	cases := []struct {
		name    string
		reading uint32
		want    bool
	}{
		{"reduced known", rmReduced, true},
		{"animations known on", rmAnimate, false},
		{"no setting", rmNone, false},
	}
	for _, c := range cases {
		// Each case starts cold so the first, synchronous read is
		// the one under test.
		resetReducedMotionCache()
		reading.Store(c.reading)
		if got := n.PrefersReducedMotion(); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// The render pass asks once per visible SVG per frame (LoadSvg →
// svgParseOpts). A process spawn per ask dropped an SVG-heavy page
// from 60 to ~20 fps (issue #892), so calls within the TTL must not
// re-run the query.
func TestPrefersReducedMotionCachesWithinTTL(t *testing.T) {
	n := &nativePlatform{}
	var reading atomic.Uint32
	reading.Store(rmReduced)
	calls := stubReducedMotion(t, &reading)
	reducedMotionTTL = time.Hour

	for range 1000 {
		if !n.PrefersReducedMotion() {
			t.Fatal("want reduced motion")
		}
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("query ran %d times for 1000 calls, want 1", got)
	}
}

// Once the TTL lapses, a call still answers from the cache at once and
// refreshes in the background, so a setting changed while the app runs
// is picked up without the caller waiting on gsettings.
func TestPrefersReducedMotionRefreshesAfterTTL(t *testing.T) {
	n := &nativePlatform{}
	var reading atomic.Uint32
	reading.Store(rmAnimate)
	stubReducedMotion(t, &reading)
	reducedMotionTTL = 0

	if n.PrefersReducedMotion() {
		t.Fatal("first read: want animations on")
	}
	reading.Store(rmReduced)

	// The stale answer may come back while the refresh runs; poll
	// until the background read lands.
	deadline := time.Now().Add(5 * time.Second)
	for !n.PrefersReducedMotion() {
		if time.Now().After(deadline) {
			t.Fatal("changed setting never picked up after the TTL")
		}
		time.Sleep(time.Millisecond)
	}
}
