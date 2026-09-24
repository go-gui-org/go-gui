// Package main implements the headless screenshot capture tool for explorer.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	var (
		root = flag.String("root", "examples", "examples root")
		out  = flag.String("out", "screenshot.png", "output filename inside each example")
	)
	flag.Parse()

	// The output must stay a bare filename inside each example dir.
	if *out == "" || *out == "." || filepath.Base(*out) != *out || strings.HasPrefix(*out, "-") {
		fmt.Fprintf(os.Stderr, "out must be a bare filename: %q\n", *out)
		os.Exit(1)
	}

	entries, err := os.ReadDir(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", *root, err)
		os.Exit(1)
	}
	fmt.Printf("capturing %d examples\n", len(entries))
	var ok, failed, skipped int
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || name == "" || name == "bin" || name[0] == '.' {
			continue
		}
		if strings.Contains(name, "..") || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, "-") {
			fmt.Printf("%-24s skip (invalid name)\n", name)
			skipped++
			continue
		}
		dir := filepath.Join(*root, name)
		target := filepath.Join(dir, *out)

		// Try per-example -screenshot flag.
		cmd := exec.Command("go", "run", "./examples/"+name, "-screenshot", target) // #nosec G204 -- name is validated basename
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
