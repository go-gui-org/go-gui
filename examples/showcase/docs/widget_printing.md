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

## PrintJob Properties

| Property    | Type                 | Description                     |
| ----------- | -------------------- | ------------------------------- |
| OutputPath  | string               | PDF output path (export only)   |
| Title       | string               | Document title                  |
| JobName     | string               | OS print job name               |
| Orientation | PrintOrientation     | Portrait or Landscape           |
| Copies      | int                  | Number of copies (default 1)    |
| PageRanges  | []PrintPageRange     | Specific page ranges            |
| Header      | PrintHeaderFooterCfg | Header text (left/center/right) |
| Footer      | PrintHeaderFooterCfg | Footer text (left/center/right) |

## Paper Sizes

`NewPrintJob` defaults to A4 portrait paper.

## PrintMargins

| Field  | Type    | Description             |
| ------ | ------- | ----------------------- |
| Top    | float32 | Top margin in points    |
| Right  | float32 | Right margin in points  |
| Bottom | float32 | Bottom margin in points |
| Left   | float32 | Left margin in points   |

The default margins are 36 points (0.5 inch) on all sides.

## PrintExportResult

| Field        | Type   | Description          |
| ------------ | ------ | -------------------- |
| Path         | string | Output file path     |
| ErrorCode    | string | Error code if failed |
| ErrorMessage | string | Human-readable error |

## PrintRunResult

| Field        | Type           | Description           |
| ------------ | -------------- | --------------------- |
| Status       | PrintRunStatus | OK / Cancel / Error   |
| ErrorCode    | string         | Error code if failed  |
| ErrorMessage | string         | Human-readable error  |
| PDFPath      | string         | Path to generated PDF |
