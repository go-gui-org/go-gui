package main

import (
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/tools/soak"
)

// Soak flags (#1262). Plain `go test` runs a short smoke, so the
// harness cannot rot; the nightly workflow sets GOGUI_SOAK=1 for the
// full run. See tools/soak/README.md.
//
//	GOGUI_SOAK=1 go test ./examples/showcase/ -run TestSoak -v
//	go test ./examples/showcase/ -run TestSoak -v -soak.duration=10m -soak.seed=42
//	go test ./examples/showcase/ -run TestSoak -v -soak.mode=random -soak.duration=10m
var (
	soakDuration = flag.Duration("soak.duration", 0,
		"simulated length of the soak run (default 5s, or 60m with GOGUI_SOAK=1)")
	soakWarmup = flag.Duration("soak.warmup", 0,
		"simulated warm-up before measuring (default 15m for runs of 30m or more)")
	soakMode = flag.String("soak.mode", "scripted", "soak input: scripted or random")
	soakSeed = flag.Uint64("soak.seed", 0, "soak seed (0 picks one; the report prints it)")
)

const (
	// soakFullDuration is the GOGUI_SOAK=1 run. The showcase heap keeps
	// growing for the first ten or so simulated minutes while the run
	// visits demos for the first time and their caches fill (measured
	// 2026-11-14: +1 to +3 MiB/min over 10 minute runs, five seeds). Over
	// 60 minutes it is flat (244 MiB at the start of measuring, 249 MiB
	// at the end, floor slope -0.3 MiB/min). 60 minutes is about 90 s of
	// wall time.
	soakFullDuration = 60 * time.Minute
	// soakFullWarmup skips the cache-filling phase before measuring.
	// With it, three 60 minute seeds measured floor slopes of -760,
	// -494 and -216 KiB/min, 61 to 67 allocs per frame and 137 to 146
	// session allocs per frame.
	soakFullWarmup = 15 * time.Minute

	// soakSlopeGateMin is the shortest run whose heap slope is gated.
	// A shorter run measures the caches filling, not a leak.
	soakSlopeGateMin = 30 * time.Minute
	// soakAllocGateMin is the shortest run whose allocations are gated.
	// A smoke run measures too few frames.
	soakAllocGateMin = time.Minute

	// soakMaxHeapSlope is the largest growth of the heap floor allowed
	// after warm-up, in bytes per simulated minute.
	soakMaxHeapSlope = 512 << 10
	// soakMaxAllocsMean is the largest mean of mallocs per TestStep
	// frame. Measured 50 to 166 over five 10 minute seeds.
	soakMaxAllocsMean = 300
	// soakMaxSessionAllocs is the largest mean of all mallocs (frames
	// and input) per frame. Measured 105 to 325 over five 10 minute
	// seeds; the high one spent long on a heavy demo.
	soakMaxSessionAllocs = 500
)

// soakSkipID keeps the scripted run out of the audio demo: its controls
// open the host sound device, which a CI runner does not have and a
// developer does not want to hear. Random mode does not use it.
func soakSkipID(id string) bool {
	id = strings.ToLower(id)
	for _, s := range []string{"audio", "sound", "music", "synth", "halt"} {
		if strings.Contains(id, s) {
			return true
		}
	}
	return false
}

func TestSoak(t *testing.T) {
	duration := *soakDuration
	if duration == 0 {
		duration = 5 * time.Second
		if os.Getenv("GOGUI_SOAK") == "1" {
			duration = soakFullDuration
		}
	}
	warmup := *soakWarmup
	if warmup == 0 && duration >= soakSlopeGateMin {
		warmup = soakFullWarmup
	}
	mode, err := soak.ParseMode(*soakMode)
	if err != nil {
		t.Fatal(err)
	}
	w := gui.NewTestWindow(t, showcaseWindowCfg())
	// Demos with images extract embedded assets to temp files, as in
	// main; remove them when the test ends.
	t.Cleanup(cleanupEmbeddedAssets)
	rep, err := soak.Run(w, soak.Config{
		Seed:     *soakSeed,
		Duration: duration,
		Warmup:   warmup,
		Mode:     mode,
		SkipID:   soakSkipID,
		Logf:     t.Logf,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log(rep.String())

	// Random mode looks for panics; its allocations follow the noise it
	// sends, so only the panic check applies.
	var gates soak.Gates
	if mode == soak.ModeScripted && duration >= soakAllocGateMin {
		gates.MaxAllocsMean = soakMaxAllocsMean
		gates.MaxSessionAllocsPerFrame = soakMaxSessionAllocs
	}
	if mode == soak.ModeScripted && duration >= soakSlopeGateMin {
		gates.MaxHeapSlope = soakMaxHeapSlope
	}
	// A panic fails every run, smoke included.
	if err := rep.Check(gates); err != nil {
		t.Fatal(err)
	}
	// Wall time is advisory: it moves with machine load. The target is
	// ten simulated minutes in under two minutes of wall time.
	if budget := duration / 5; duration >= soakAllocGateMin && rep.Wall > budget {
		t.Logf("advisory: wall time %s is over %s for %s simulated",
			rep.Wall.Round(time.Second), budget, duration)
	}
}
