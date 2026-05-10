---
description: Harden uncommitted changes against bad data and denial of service attacks.
---

For all uncommitted changes (staged and unstaged), harden inputs against bad data and DoS:

Steps:
- Run `git diff` and `git diff --cached` to identify changed files and functions
- Read surrounding context in modified files as needed
- For each public function or entry point in the diff, check for and fix:
  - **NaN/Inf floats** — clamp or replace with safe defaults (pixelMin convention)
  - **Nil/empty slices and maps** — early return or no-op, never panic
  - **Unbounded input sizes** — cap slice lengths, iteration counts, string lengths
  - **Division by zero** — guard before dividing
  - **Integer overflow** — check before arithmetic on user-supplied counts
  - **Excessive allocations** — pre-check sizes before allocating large buffers
  - **Duplicate or degenerate data** — handle gracefully, no infinite loops
- Apply fixes directly to the source files
- Run `go build ./...` and `go vet ./...` to verify changes compile
- Summarize what was hardened and where
