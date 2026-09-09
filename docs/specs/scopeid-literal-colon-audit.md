# ScopeID literal colon audit

Status: implemented. No issue was filed; implemented directly from the proposal.
First run over `gui/`: 0 findings.

## Problem

`ScopeID` parts must not contain `IDSep` (`gui/id_scope.go:27`), and "nothing
enforces that". The only content-level guard in the repo is
`TestDockMintedNodeIDsHaveNoIDSep` (`gui/dock_layout_identity_test.go:121`),
which covers minted dock node IDs only. Everywhere else a colon in a part is
silent until it collides (duplicate ID) or re-scopes (leaf promoted to absolute,
drops out of its scope, state key diverges) — both detected by consequence, not
by cause.

## Decision

Extend `ergonomics-audit -mode ids` (`tools/ergonomics-audit/ids.go`) with one
new rule: flag a `ScopeID` / `ScopeIDN` call whose **part** argument is a string
literal containing `":"`.

- The **owner** (first argument) is exempt: it may itself be composed, which is
  how nesting works (`ScopeID(base, "resize")` in `gui/id_scope.go:24`).
- `ScopeIDN`'s numeric argument cannot contain a separator; only its string
  parts are checked.
- Matching is on the selector name (`Sel.Name == "ScopeID"` / `"ScopeIDN"`),
  regardless of package qualifier, so `gui.ScopeID` and the datagrid's
  `gg.ScopeID` both count.
- Non-literal parts (variables, row keys, slugs) stay out of scope: invisible
  statically, and the one runtime hook (`ScopeID` itself) is pure with no
  findings channel — see Rejected Approaches.

A literal part containing `":"` is always rewritable (drop the colon or use `-`
/ `_`, per the parts rule in `docs/specs/widget-id-scoping.md`), so findings are
fixed, not marked: no new marker. If a legitimate literal exception ever
appears, that is a spec amendment, not a quiet marker.

## Implementation steps

1. Add the rule to `inspectIDs` in `tools/ergonomics-audit/ids.go`: on
   `*ast.CallExpr` with a `*ast.SelectorExpr` fun named `ScopeID` or `ScopeIDN`,
   unquote each part argument (skip index 0) and report literals containing
   `":"`. Reuse `shortExpr` and the existing `seen`/`exempt` dedup in `scanIDs`.
2. Add table tests in `tools/ergonomics-audit/ids_test.go` through `scanSrc`,
   following the file's case-as-source convention: flags
   `ScopeID(cfg.ID, "row:x")`; quiet on `ScopeID(base, "resize")`,
   `ScopeIDN(cfg.ID, "opt", i)`, and a variable part.
3. Run `make ergonomics-audit` over the repo; fix any findings in the same pass
   (each is a literal rewrite).
4. No `CHANGELOG.md` entry: audit-only, no exported surface or observable
   behaviour change.

## Rejected Approaches

- **Panic in `ScopeID` on a colon-bearing part.** Fail-fast fits the repo ethos,
  but `ScopeID` is pure — no `*Window`, no findings channel — so panic is its
  only signal, and it fires on data-derived input: a row key containing `":"`
  becomes a crash instead of a diagnostic. Adds a per-part scan to a per-frame
  path.
- **Sanitize in `ScopeID` (strip/replace `:`).** Silent identity change with new
  collision shapes; trades a loud failure for a quiet one, the exact hazard
  `widget-id-scoping.md` rejects for implicit IDs.
- **A `Debug` category for absolute leaves under a scope.** The stamp pass
  cannot distinguish misuse from the documented absolute-leaf exception
  (`gui/datagrid` children, `form:<id>`), so the category is either noisy or
  allowlist-driven. Left as future work, not this spec.

## Unresolved questions

- Should the rule also cover `+ IDSep +` hand-joins (already flagged as
  hand-rolled composition today — confirm no double-report)?
- Does the datagrid's absolute-spelling exception need any literal colon-bearing
  part that this rule would flag?
