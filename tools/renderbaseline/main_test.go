package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `noise
rstats backend=gl frames=60 draws=10.0 gpu_us_p50=-1 flush_scissor=4.0 flush_text=1.0
rstats backend=gl frames=60 draws=20.0 gpu_us_p50=-1 flush_scissor=6.0 flush_text=1.0
rstats backend=gl frames=58 draws=30.0 gpu_us_p50=700 flush_scissor=8.0 flush_text=0.0
rstats backend=gl frames=62 draws=40.0 gpu_us_p50=900 flush_scissor=2.0 flush_text=0.0
`

func TestParseStreamSkipsWarmup(t *testing.T) {
	lines := parseStream(strings.NewReader(sample), 1)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (4 rstats lines, 1 skipped)", len(lines))
	}
	if lines[0]["draws"] != "20.0" || lines[0]["backend"] != "gl" {
		t.Fatalf("first kept line = %v", lines[0])
	}
}

func TestSummarizeMedians(t *testing.T) {
	r := summarize("w", parseStream(strings.NewReader(sample), 0))
	if r.backend != "gl" || r.samples != 4 {
		t.Fatalf("backend, samples = %q, %d", r.backend, r.samples)
	}
	for k, want := range map[string]float64{
		"draws": 25, "frames": 60, "flush_scissor": 5,
		// -1 is "no GPU time": left out, so the median is of 700 and 900.
		"gpu_us_p50": 800,
	} {
		if got := r.median[k]; got != want {
			t.Errorf("median %s = %v, want %v", k, got, want)
		}
	}
	if got := topCauses(r); got != "scissor 5.0, text 0.5" {
		t.Errorf("topCauses = %q", got)
	}
}

func TestMedianAllMissing(t *testing.T) {
	r := summarize("w", parseStream(strings.NewReader(
		"rstats backend=gl gpu_us_p50=-1\nrstats backend=gl gpu_us_p50=-1\n"), 0))
	if got := r.median["gpu_us_p50"]; got != -1 {
		t.Fatalf("all -1: median %v, want -1", got)
	}
}

func TestWriteCSVFile(t *testing.T) {
	r := summarize("w", parseStream(strings.NewReader(sample), 0))
	path := filepath.Join(t.TempDir(), "out.csv")
	if err := writeCSVFile(path, "linux/amd64 RX 7600", []result{r}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(got) != 2 || !strings.HasPrefix(got[0], "machine,workload,backend,samples,draws,") ||
		!strings.HasPrefix(got[1], `"linux/amd64 RX 7600",w,gl,4,25,`) {
		t.Fatalf("csv =\n%s", b)
	}
}
