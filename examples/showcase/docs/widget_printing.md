Export the current window to PDF or send to the OS print
dialog. Supports paper sizes, margins, orientation, duplex,
color mode, page ranges, headers/footers, and DPI settings.

## Export PDF

```go
job := gui.NewPrintJob()
job.OutputPath = "/tmp/output.pdf"
job.Title = "My Document"
job.Paper = gui.PaperA4
job.Orientation = gui.PrintLandscape
r := w.ExportPrintJob(job)
if r.IsOk() {
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

## PrintJob Properties

| Property    | Type                 | Description                          |
|-------------|----------------------|--------------------------------------|
| OutputPath  | string               | PDF output path (export only)        |
| Title       | string               | Document title                       |
| JobName     | string               | OS print job name                    |
| Paper       | PaperSize            | Paper size                           |
| Orientation | PrintOrientation     | Portrait or Landscape                |
| Margins     | PrintMargins         | Page margins in points (1/72 inch)   |
| Copies      | int                  | Number of copies (default 1)         |
| Duplex      | PrintDuplexMode      | Simplex / LongEdge / ShortEdge      |
| ColorMode   | PrintColorMode       | Default / Color / Grayscale          |
| ScaleMode   | PrintScaleMode       | FitToPage or ActualSize              |
| PageRanges  | []PrintPageRange     | Specific page ranges                 |
| Header      | PrintHeaderFooterCfg | Header text (left/center/right)      |
| Footer      | PrintHeaderFooterCfg | Footer text (left/center/right)      |
| Paginate    | bool                 | Enable pagination                    |
| RasterDPI   | int                  | Raster DPI (default 300)             |
| JPEGQuality | int                  | JPEG quality (default 85)            |

## Paper Sizes

| Constant    | Size           |
|-------------|----------------|
| PaperLetter | 8.5 x 11 in   |
| PaperLegal  | 8.5 x 14 in   |
| PaperA4     | 210 x 297 mm  |
| PaperA3     | 297 x 420 mm  |

## PrintMargins

| Field  | Type    | Description                |
|--------|---------|----------------------------|
| Top    | float32 | Top margin in points       |
| Right  | float32 | Right margin in points     |
| Bottom | float32 | Bottom margin in points    |
| Left   | float32 | Left margin in points      |

`DefaultPrintMargins()` returns 36pt (0.5 inch) on all sides.

## PrintExportResult

| Field        | Type              | Description              |
|--------------|-------------------|--------------------------|
| Status       | PrintExportStatus | PrintExportOK or Error   |
| Path         | string            | Output file path         |
| ErrorCode    | string            | Error code if failed     |
| ErrorMessage | string            | Human-readable error     |

## PrintRunResult

| Field        | Type           | Description              |
|--------------|----------------|--------------------------|
| Status       | PrintRunStatus | OK / Cancel / Error      |
| ErrorCode    | string         | Error code if failed     |
| ErrorMessage | string         | Human-readable error     |
| PDFPath      | string         | Path to generated PDF    |
| Warnings     | []PrintWarning | Non-fatal issues         |
