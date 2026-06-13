# pkgcov.awk — extract per-package coverage % from a Go coverage.out file.
# Output: package<TAB>coverage_pct
NR == 1 { next }
{
    # $1 = package/path/file.go:line.col,line.col
    # $2 = num_statements
    # $3 = count (0 = uncovered, >0 = covered)
    pkg = $1
    # Strip everything after the last / (i.e. the filename).
    idx = match(pkg, /\/[^\/]+$/)
    if (idx > 0) pkg = substr(pkg, 1, idx - 1)
    stmts[pkg] += $2
    if ($3 > 0) covered[pkg] += $2
}
END {
    for (p in stmts) {
        if (stmts[p] == 0) continue
        pct = 100.0 * covered[p] / stmts[p]
        printf "%s\t%.1f\n", p, pct
    }
}
