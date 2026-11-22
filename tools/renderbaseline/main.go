// renderbaseline records the render baselines of the batching
// investigation (#1303): it runs a fixed set of example workloads with
// GOGUI_RENDER_STATS=1, reads the per-second "rstats" lines each backend
// prints to stderr, and writes the median of every field per workload.
//
// It is a Go program and not a shell script, so that the same command
// runs on macOS (Metal), Linux and Windows (GL). Each workload opens a
// real window, so run it in a desktop session, with the window visible.
//
// Usage:
//
//	go run ./tools/renderbaseline [-seconds 8] [-skip 2] [-runs 3] [-only re] [-label "GPU name"] [-csv out.csv]
//
// Every workload runs -runs times for -seconds each. The first -skip
// lines of each run are warmup and are dropped. The medians go to stdout
// as a Markdown table and, with -csv, to a CSV file with every field.
// GOGUI_RENDERER passes through, so "GOGUI_RENDERER=gl" on Windows
// measures GL even where the default would pick another renderer.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

// workload is one example run. args go to the example binary.
type workload struct {
	name string
	pkg  string
	args []string
}

// workloads cover the four kinds #1303 names: the widget benchmark,
// scrolling lists, text, and animation or effects. Each runs as it is,
// except for the flags that make it drive itself.
var workloads = []workload{
	{"mixed5000", "./examples/benchmark", []string{"-type", "Mixed", "-count", "5000", "-seconds", "3600", "-warmup", "0"}},
	{"button1000", "./examples/benchmark", []string{"-type", "Button", "-count", "1000", "-seconds", "3600", "-warmup", "0"}},
	{"virtual_list", "./examples/virtual_list", []string{"-autoscroll", "40"}},
	{"data_grid", "./examples/data_grid_data_source", []string{"-autoscroll", "40"}},
	{"digital_rain", "./examples/digital_rain", nil},
	{"markdown", "./examples/markdown", nil},
	{"animation_stress", "./examples/animation_stress", nil},
	{"svg_spinners", "./examples/svg_spinners", nil},
	{"shadow_demo", "./examples/shadow_demo", nil},
	{"gradient_demo", "./examples/gradient_demo", nil},
	{"blur_demo", "./examples/blur_demo", nil},
	{"custom_shader", "./examples/custom_shader", nil},
	{"qcpainter64", "./examples/qcpainterbench", []string{"-count", "64"}},
}

// tableFields are the columns of the Markdown table; the CSV has all.
var tableFields = []string{
	"frames", "draws", "quads", "verts", "pipe_binds", "upload_kib",
	"submit_us_p50", "gpu_us_p50", "gpu_us_p99",
}

func main() {
	seconds := flag.Float64("seconds", 8, "seconds per run")
	skip := flag.Int("skip", 2, "rstats lines dropped at the start of each run (warmup)")
	runs := flag.Int("runs", 3, "runs per workload")
	only := flag.String("only", "", "run only workloads whose name matches this regexp")
	label := flag.String("label", "", "machine label for the CSV, e.g. the GPU name")
	csvPath := flag.String("csv", "", "also write every median field to this CSV file")
	flag.Parse()
	if err := run(*seconds, *skip, *runs, *only, *label, *csvPath); err != nil {
		log.Fatal(err)
	}
}

// run measures the selected workloads and writes the results. It is apart
// from main so that its deferred cleanup runs before an error exits.
func run(seconds float64, skip, runs int, only, label, csvPath string) error {
	var sel *regexp.Regexp
	if only != "" {
		var err error
		if sel, err = regexp.Compile(only); err != nil {
			return fmt.Errorf("-only: %w", err)
		}
	}
	dir, err := os.MkdirTemp("", "renderbaseline")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	var rows []result
	for _, wl := range workloads {
		if sel != nil && !sel.MatchString(wl.name) {
			continue
		}
		bin, berr := build(dir, wl.pkg)
		if berr != nil {
			log.Printf("%s: build: %v", wl.name, berr)
			continue
		}
		var lines []map[string]string
		for r := range runs {
			got, rerr := runOnce(bin, wl.args, time.Duration(seconds*float64(time.Second)), skip)
			if rerr != nil {
				log.Printf("%s run %d: %v", wl.name, r+1, rerr)
			}
			lines = append(lines, got...)
		}
		if len(lines) == 0 {
			log.Printf("%s: no rstats lines; is the backend instrumented?", wl.name)
			continue
		}
		rows = append(rows, summarize(wl.name, lines))
		log.Printf("%s: %d samples", wl.name, len(lines))
	}

	machine := runtime.GOOS + "/" + runtime.GOARCH
	if label != "" {
		machine += " " + label
	}
	writeTable(os.Stdout, machine, rows)
	if csvPath != "" {
		return writeCSVFile(csvPath, machine, rows)
	}
	return nil
}

// build compiles pkg into dir and returns the binary path.
func build(dir, pkg string) (string, error) {
	bin := filepath.Join(dir, filepath.Base(pkg))
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, pkg)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	return bin, cmd.Run()
}

// runOnce runs bin for d with the stats on and returns the parsed rstats
// lines after the first skip. A run that the timeout ends is the normal
// case, not an error.
func runOnce(bin string, args []string, d time.Duration, skip int) ([]map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = append(os.Environ(), "GOGUI_RENDER_STATS=1")
	cmd.Stdout = io.Discard
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, err
	}
	lines := parseStream(stderr, skip)
	werr := cmd.Wait()
	if ctx.Err() != nil {
		werr = nil // ended by the timeout
	}
	var exitErr *exec.ExitError
	if errors.As(werr, &exitErr) && exitErr.ExitCode() == 0 {
		werr = nil
	}
	return lines, werr
}

// parseStream returns the key=value fields of each rstats line in r,
// after the first skip of them. Other lines are ignored.
func parseStream(r io.Reader, skip int) []map[string]string {
	var out []map[string]string
	sc := bufio.NewScanner(r)
	seen := 0
	for sc.Scan() {
		line, ok := strings.CutPrefix(sc.Text(), "rstats ")
		if !ok {
			continue
		}
		seen++
		if seen <= skip {
			continue
		}
		m := map[string]string{}
		for f := range strings.FieldsSeq(line) {
			if k, v, ok := strings.Cut(f, "="); ok {
				m[k] = v
			}
		}
		out = append(out, m)
	}
	return out
}

// result is the medians of one workload.
type result struct {
	name    string
	backend string
	samples int
	keys    []string // sorted numeric field names
	median  map[string]float64
}

// summarize takes the median of each numeric field over lines. A field
// of -1 (no GPU time) is left out of its median; when every sample is
// -1 the median is -1.
func summarize(name string, lines []map[string]string) result {
	res := result{name: name, samples: len(lines), median: map[string]float64{}}
	vals := map[string][]float64{}
	for _, m := range lines {
		for k, v := range m {
			if k == "backend" {
				res.backend = v
				continue
			}
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				continue
			}
			if _, ok := vals[k]; !ok {
				vals[k] = nil
			}
			if f >= 0 {
				vals[k] = append(vals[k], f)
			}
		}
	}
	for k, v := range vals {
		res.keys = append(res.keys, k)
		res.median[k] = median(v)
	}
	slices.Sort(res.keys)
	return res
}

// median of v; -1 for no values.
func median(v []float64) float64 {
	if len(v) == 0 {
		return -1
	}
	s := slices.Clone(v)
	slices.Sort(s)
	n := len(s)
	if n%2 == 1 {
		return s[n/2]
	}
	return (s[n/2-1] + s[n/2]) / 2
}

// topCauses names the flush causes above 0, largest first, as
// "scissor 257.6, pipeline 26.0".
func topCauses(r result) string {
	type kv struct {
		k string
		v float64
	}
	var cs []kv
	for _, k := range r.keys {
		if c, ok := strings.CutPrefix(k, "flush_"); ok && r.median[k] > 0 {
			cs = append(cs, kv{c, r.median[k]})
		}
	}
	slices.SortFunc(cs, func(a, b kv) int {
		switch {
		case a.v > b.v:
			return -1
		case a.v < b.v:
			return 1
		}
		return strings.Compare(a.k, b.k)
	})
	parts := make([]string, 0, len(cs))
	for _, c := range cs {
		parts = append(parts, fmt.Sprintf("%s %.1f", c.k, c.v))
	}
	return strings.Join(parts, ", ")
}

// writeTable writes the Markdown table of the medians.
func writeTable(w io.Writer, machine string, rows []result) {
	_, _ = fmt.Fprintf(w, "Machine: %s. Medians of per-second means, per frame.\n\n", machine)
	_, _ = fmt.Fprintf(w, "| workload | backend | %s | flush causes |\n", strings.Join(tableFields, " | "))
	_, _ = fmt.Fprintf(w, "|%s\n", strings.Repeat(" --- |", len(tableFields)+3))
	for _, r := range rows {
		cells := make([]string, 0, len(tableFields))
		for _, f := range tableFields {
			cells = append(cells, strconv.FormatFloat(r.median[f], 'f', -1, 64))
		}
		_, _ = fmt.Fprintf(w, "| %s | %s | %s | %s |\n", r.name, r.backend, strings.Join(cells, " | "), topCauses(r))
	}
}

// writeCSVFile writes every field of every row. Columns are the union of
// the field names, so a backend that lacks a field leaves it empty.
func writeCSVFile(path, machine string, rows []result) error {
	var cols []string
	for _, r := range rows {
		for _, k := range r.keys {
			if !slices.Contains(cols, k) {
				cols = append(cols, k)
			}
		}
	}
	slices.Sort(cols)
	var b strings.Builder
	b.WriteString("machine,workload,backend,samples," + strings.Join(cols, ",") + "\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "%q,%s,%s,%d", machine, r.name, r.backend, r.samples)
		for _, c := range cols {
			b.WriteByte(',')
			if v, ok := r.median[c]; ok {
				b.WriteString(strconv.FormatFloat(v, 'f', -1, 64))
			}
		}
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}
