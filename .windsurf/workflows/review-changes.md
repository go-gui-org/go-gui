---
description: Review uncommitted changes for quality, consistency, security, and performance.
---

Review all uncommitted changes (staged and unstaged) for:

1. **Quality** — bugs, logic errors, missing error handling at boundaries, dead code
2. **Consistency** — naming, patterns, style matching the rest of the codebase
3. **Security** — injection, unsafe input, resource leaks
4. **Performance** — unnecessary allocations, O(n²) where O(n) suffices, hot-path copies

Steps:
- Run `git diff` and `git diff --cached` to collect all changes
- Read surrounding context in modified files as needed
- Report findings grouped by category (Quality, Consistency, Security, Performance)
- For each finding: file:line, severity (error/warning/nit), one-line description
- If no issues found, say so
- Do NOT modify any files
