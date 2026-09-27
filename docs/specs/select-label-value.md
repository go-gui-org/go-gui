# Spec: label/value options for `Select` and `Combobox`

Issue: #809. Status: **implemented** — unreleased (breaking).

## Motivation

`SelectCfg.Options` and `ComboboxCfg.Options` were `[]string`. The text the user
saw and the text the app stored were one string. An app that showed "TypeScript
(Node.js)" had to keep "TypeScript (Node.js)" in its state, and a label change
broke saved state.

## Decision

Both Cfgs take `Items []string` and `Options []gui.SelectOption`:

```go
type SelectOption struct {
    Label string
    Value string
    isSubheading bool // set only by NewSelectSubheading
}
```

- The dropdown and the closed field show `Label`. `Selected`,
  `ComboboxCfg.Value` and `OnSelect` carry `Value`.
- A selected value that no option holds shows as written. The data grid select
  editor needs this: a cell value need not be in the option list.
- `Items` is the shorthand: each string is both label and value. When set,
  `Items` wins over `Options`.
- Subheadings: `NewSelectSubheading(label)` on the typed path. On `Items`, the
  "---" prefix still makes a Select subheading, so string callers lose nothing.
  A Combobox has no subheadings: its keyboard walks the filtered rows without
  skipping, so it leaves subheading options out.
- The Combobox query matches `Label`. Its row cache is keyed by a hash of the
  source list (label and value, with a domain byte per path). `Items` is read in
  place, so an unchanged list costs one hash pass and no allocation.
- Select reads `Items` in place too (`optionCount`/`optionAt`), never copying it
  into a `[]SelectOption`. A copy would be one allocation per frame for every
  string-list caller, where the old `[]string` path had none. There is no length
  cap on either widget.
- The closed field is empty when the shown text is empty, not when `Selected[0]`
  is `""`. An option with an empty value ("None") is a real selection and shows
  its label, as in Combobox.

This is the shape `RadioButtonGroupCfg` and `SegmentedControlCfg` already have
(`Items []string` + `Options []RadioOption`), so the four choice controls swap
for each other.

Migration: `Options: []string{...}` → `Items: []string{...}`. No sibling repo
built a `SelectCfg`. `go-speedtest` builds two `ComboboxCfg` with
`Options: names`; it changes to `Items:` when it takes the next go-gui tag.

## Rejected Approaches

- **Additive `Choices []SelectOption` next to `Options []string`.** No break,
  but `Options` would mean strings here and typed pairs on Radio and Segmented —
  the inconsistency the repo removes. Two list fields, and setting both silently
  drops one.
- **`OptionLabel func(value string) string`.** No new type, but a function hides
  the list shape, leaves no place for later per-option fields (disabled, a11y),
  and the label lookup cannot be tested as data.
- **"---" prefix on typed labels.** A magic string inside display text. A label
  that starts with dashes is shown as written; `NewSelectSubheading` is the
  typed spelling.
- **A separate `ComboboxOption` type.** Same two fields; one type keeps Select
  and Combobox interchangeable.
