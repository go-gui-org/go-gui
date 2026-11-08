# Exported API surface policy (exportaudit)

Status: accepted (v0.56.0 freeze follow-up)

## Purpose

`tools/exportaudit` gates the exported surface of `gui/` so that what stays
exported is deliberate. This spec records the classification, the gate contract,
and the one accepted-deferred class.

## Classes

Every export in a scanned gui/ package is ranked by the widest circle that
references it:

| class    | referenced by                               | gate (with consumers) |
| -------- | ------------------------------------------- | --------------------- |
| none     | nothing                                     | hard failure          |
| self     | its own package only                        | hard failure          |
| selftest | its own package or its tests                | hard failure          |
| internal | other gui/ packages                         | accepted (deferred)   |
| reach    | its own package's exported surface, by type | pass                  |
| testext  | gui/ external test packages                 | pass                  |
| example  | examples/                                   | pass                  |
| consumer | sibling repos (go-charts, go-edit, go-kite, | pass                  |
|          | go-term, go-map)                            |                       |

`maxClass` is the highest class with any reference. A hard failure means
`make export-audit` exits non-zero. The symbol is internal-only and must be
unexported or carry a `// exportaudit:keep` marker explaining why. The accepted
reasons: json reflection, stdlib interface conformance, name collision,
documented API. Signature reachability is not a marker reason: the `reach` class
computes it, and the 96 markers that once asserted it were deleted in #1235
after #735 had made 25 of them false.

The consumer scan is authoritative. Without the sibling repos, an export used
only there misclassifies as none/self. The in-repo run alone can only report (it
never hard-fails). `make export-audit` clones nothing. It uses `../go-*`
checkouts when present. CI shallow-clones the five sibling repos into `../`, so
the gate is authoritative there.

## Reach class: type closure

`reach` (`tools/exportaudit/reach.go`) is not a reference count but a closure.
It starts at the package's exported functions, constants and variables and
follows type names through signatures, exported struct fields (callback fields
included), exported interface methods and the exported methods of every type it
reaches. A type in the closure, its exported fields and methods, and the
exported constants of its type pass the gate with no reference at all: an app
implementing `DataGridDataSource` must fill `GridDataCapabilities`, whether or
not any Go file in the scan does. Exported types that nothing names stay outside
the closure, so an orphan struct still fails.

The 2026-08 sweep (#230) checked function signatures only. Payload structs that
only `gui/` itself filled lost their fields, and enums set through a Cfg field
lost their type name; #1221 restored them and added this class so the gate
protects that shape instead of eroding it.

## Internal class: accepted by policy

`internal` — used by other gui/ packages — is deliberately not a hard failure.
These exports are shared implementation between the root `gui` package and its
leaf subpackages (backend, datagrid, svg, markdown, highlight, shader). The
architecture keeps the root package as the hub that subpackages import
(`gui/backend → gui`, `gui/svg → gui`), so unexporting them is impossible
without an `gui/internal/` package move, which the flat-leaf design rejects.
They are reported in the gate advisory and listed fully by `-mode report`. Keep
markers are not required for this class.

## Contract for future changes

- Any new export whose references stay inside gui/ (none/self/selftest) fails
  the gate. Unexport it or keep-mark it.
- Any export that starts being used by a sibling repo moves out of enforcement
  automatically. The consumer scan reclassifies it.
- Removing a consumer's export is caught by CI: the authoritative scan sees the
  sibling reference vanish and reclassifies the export down into enforcement.
- `-mode report` output is stable (fixed section order) so two runs can be
  diffed. Use it before and after a sweep.
