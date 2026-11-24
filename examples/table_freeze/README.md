# Table Freeze

> **Framework:** data, scrolling **Description:** Table whose row header and
> first columns stay in place while the others scroll sideways.

<!-- explorer: tags=data,scroll category=data run=go -->

---

## Run

```sh
go run ./examples/table_freeze/
```

## What it demonstrates

- `TableCfg.FreezeColumns` keeps the first columns in place. The **Fewer** and
  **More** buttons change it from 0 to 3. The row header stays with them. A line
  marks the edge, because this table has no vertical cell borders.
- `TableCfg.FreezeRowHeader` keeps the row header in place on its own, with
  `FreezeColumns` at 0.
- `TableCfg.FreezeHeader` keeps the column header above the rows. It works with
  or without frozen columns.
- Make the window very narrow: the frozen columns then scroll away first, so the
  last column can still be reached.

See `main.go` for the implementation.
