Export the current window to PDF or send it to the OS print dialog. Set the
title, output path, copies, page ranges, headers, and footers on the job.
`NewPrintJob` provides sensible defaults.

## Export PDF

```go
job := gui.NewPrintJob()
job.OutputPath = "/tmp/output.pdf"
job.Title = "My Document"
r := w.ExportPrintJob(job)
if r.ErrorMessage != "" {
    fmt.Println("Error:", r.ErrorMessage)
} else {
    fmt.Println("Saved to", r.Path)
}
```

## Print via OS Dialog

```go
job := gui.NewPrintJob()
job.Title = "My Document"
r := w.RunPrintJob(job)
if r.Status == gui.PrintRunOK {
    // printed successfully
}
```

When the job prints the current view, `RunPrintJob` exports to a temp PDF first.
On success its path is returned in `r.PDFPath` and the caller owns it: remove
the file when done. On cancel or error the temp file is removed and `PDFPath` is
empty. `Copies` and `PageRanges` apply to the native dialog only;
`ExportPrintJob` always writes a single page. On Windows the system PDF handler
takes no options, so copies, duplex, color mode, orientation and page ranges are
ignored there.

## PrintJob Properties

| Property     | Type                 | Description                           |
| ------------ | -------------------- | ------------------------------------- |
| OutputPath   | string               | PDF output path (export only)         |
| Title        | string               | Document title                        |
| JobName      | string               | OS print job name                     |
| Source       | PrintJobSource       | Current view or a PDF file            |
| Paper        | PaperSize            | Letter, Legal, A4, A3 (default A4)    |
| Orientation  | PrintOrientation     | Portrait or Landscape                 |
| Margins      | PrintMargins         | Page margins in points                |
| Copies       | int                  | Number of copies (default 1)          |
| PageRanges   | []PrintPageRange     | Specific page ranges                  |
| ScaleMode    | PrintScaleMode       | Fit to page or actual size            |
| Duplex       | PrintDuplexMode      | Off, long edge, short edge            |
| ColorMode    | PrintColorMode       | Color or grayscale                    |
| SourceWidth  | float32              | Viewport width override (0 = window)  |
| SourceHeight | float32              | Viewport height override (0 = window) |
| Header       | PrintHeaderFooterCfg | Header text (left/center/right)       |
| Footer       | PrintHeaderFooterCfg | Footer text (left/center/right)       |

## Paper Sizes

`NewPrintJob` defaults to A4 portrait paper. Set `Paper` to `PaperLetter`,
`PaperLegal`, `PaperA4` or `PaperA3`, and `Orientation` to `PrintPortrait` or
`PrintLandscape`.

## PrintMargins

| Field  | Type    | Description             |
| ------ | ------- | ----------------------- |
| Top    | float32 | Top margin in points    |
| Right  | float32 | Right margin in points  |
| Bottom | float32 | Bottom margin in points |
| Left   | float32 | Left margin in points   |

The default margins are 36 points (0.5 inch) on all sides.

## PrintExportResult

| Field        | Type              | Description          |
| ------------ | ----------------- | -------------------- |
| Path         | string            | Output file path     |
| Status       | PrintExportStatus | OK or Error          |
| ErrorCode    | string            | Error code if failed |
| ErrorMessage | string            | Human-readable error |

`r.IsOk()` reports success.

## PrintRunResult

| Field        | Type           | Description           |
| ------------ | -------------- | --------------------- |
| Status       | PrintRunStatus | OK / Cancel / Error   |
| ErrorCode    | string         | Error code if failed  |
| ErrorMessage | string         | Human-readable error  |
| PDFPath      | string         | Path to generated PDF |
