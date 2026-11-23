# Table Row Header

> **Framework:** data, selection **Description:** Table row header column with a
> styled marker and an app-owned current row.

<!-- explorer: tags=data,selection category=data run=go -->

---

## Run

```sh
go run ./examples/table_row_header/
```

## What it demonstrates

- `TableCfg.RowHeader` adds a row selector column and a select-all corner. A
  click on a row header cell selects the row. Ctrl (Cmd on macOS) toggles it and
  Shift selects a range. A click on the corner selects every data row.
- `TableCfg.RowHeaderTextStyle` and `TableCfg.RowHeaderMarker` style the marker.
  A large `TextStyle` sets a tall row; the small marker stays centred in it. The
  **Marker** radio group switches the glyph between `>`, `▶` and `😀`.
- `TableCfg.CurrentRow` lets the app own the marker row. The table is not
  focusable. The **Marker up** and **Marker down** buttons stand in for an app's
  own keyboard navigation. At `-1`, `0` (the header row) and one past the last
  row, no row shows the marker.

See `main.go` for the implementation.
