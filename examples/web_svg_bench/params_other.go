//go:build !(js && wasm)

package main

import (
	"os"
	"strings"
)

// queryParam reads BENCH_<NAME> from the environment on native
// builds, so the same workload can be compared against the desktop
// backends (BENCH_ROWS=20 go run ./examples/web_svg_bench/).
func queryParam(name string) string {
	return os.Getenv("BENCH_" + strings.ToUpper(name))
}
