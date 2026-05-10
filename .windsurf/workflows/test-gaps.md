---
description: Find test gaps in uncommitted code changes. Reports untested public functions, missing edge cases, and uncovered error paths.
---

Analyze all uncommitted changes (staged and unstaged) for test coverage gaps:

Steps:
- Run `git diff` and `git diff --cached` to identify changed/added files and functions
- For each changed file, find out corresponding `_test.go` file (if any)
- Read the changed code and existing tests to understand current coverage
- Report gaps grouped by severity:

**Missing tests** (no test exists):
- New exported functions or methods with no corresponding test
- New unexported functions with non-trivial logic and no test

**Missing edge cases** (test exists but incomplete):
- Error return paths not exercised
- Boundary values: zero, negative, empty, nil, max-size inputs
- NaN/Inf float inputs (for numeric code)
- Single-element and two-element cases for slice/collection code

**Missing integration tests**:
- New cross-package interactions without an integration test
- New I/O paths (file, network) without a test that exercises them

For each gap:
- State the file:function and what is untested
- Rate as **high** (crash/panic risk), **medium** (silent wrong result), or **low** (cosmetic)
- Suggest a one-line test name (e.g., `TestExportSVG_EmptySeriesNoPanic`)
