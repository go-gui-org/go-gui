// Package main implements the headless screenshot capture tool for explorer.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func main() {
	var (
		root  = flag.String("root", "examples", "examples root")
		scale = flag.Float64("scale", 2, "device pixel ratio")
		out   = flag.String("out", "screenshot.png", "output filename inside each example")
	)
	flag.Parse()
	_ = scale

	entries, err := os.ReadDir(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *root, err)
		os.Exit(1)
	}
	fmt.Printf("capturing %d examples at scale %.1f\n", len(entries), *scale)
	var ok, failed, skipped int
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "bin" || e.Name()[0] == '.' {
			continue
		}
		name := e.Name()
		dir := filepath.Join(*root, name)
		target := filepath.Join(dir, *out)

		// Try per-example -screenshot flag.
		cmd := exec.Command("go", "run", "./examples/"+name, "-screenshot", target) // #nosec G204 -- name is directory basename
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			// Not all examples support -screenshot yet; keep existing file.
			fmt.Printf("%-24s skip (%v)\n", name, err)
			skipped++
			_ = failed
			continue
		}
		if _, err := os.Stat(target); err == nil {
			fmt.Printf("%-24s ok %s\n", name, target)
			ok++
		} else {
			fmt.Printf("%-24s missing %s\n", name, target)
			failed++
		}
	}
	fmt.Printf("\ndone: %d ok, %d failed, %d skipped (no -screenshot)\n", ok, failed, skipped)
	if ok == 0 && failed == 0 {
		fmt.Println("hint: add -screenshot flag to each example's main.go (see docs/specs/examples-explorer.md)")
	}
}
