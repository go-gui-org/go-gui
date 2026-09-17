package gui

import (
	"os"
	"strings"

	"github.com/go-gui-org/go-glyph"
)

// ExportPrintJob exports renderer output to PDF using PrintJob settings.
// Returns a PrintExportResult with status and path. The export is always
// a single page; Copies and PageRanges are native-print options and are
// ignored here.
func (w *Window) ExportPrintJob(job PrintJob) PrintExportResult {
	if err := validateExportPrintJob(job); err != nil {
		return printExportErrorResult(job.OutputPath, printErrorInvalidCfg, err.Error())
	}

	sourceW := job.SourceWidth
	sourceH := job.SourceHeight

	renderersCopy, err := func() ([]RenderCmd, error) {
		w.Lock()
		defer w.Unlock()

		if sourceW <= 0 {
			sourceW = float32(w.windowWidth)
		}
		if sourceH <= 0 {
			sourceH = float32(w.windowHeight)
		}
		if len(w.renderers) == 0 {
			return nil, &printError{"no renderers available for export"}
		}
		// Prepend window background as first render command so the
		// PDF matches on-screen appearance (the backend paints the
		// background via Clear(), which is not in the renderers).
		bg := w.Config.BgColor
		if bg == (Color{}) {
			// Export runs outside generation: this window's theme.
			bg = w.Theme().ColorBackground
		}
		out := make([]RenderCmd, 0, len(w.renderers)+1)
		out = append(out, RenderCmd{
			Kind:  RenderRect,
			X:     0,
			Y:     0,
			W:     sourceW,
			H:     sourceH,
			Color: bg,
			Fill:  true,
		})
		out = append(out, w.renderers...)
		// Deep-copy the geometry. A DrawCanvas batch's triangles are
		// recycled by the next redraw of that canvas (see
		// DrawContext.resetFor), and renderToPDF runs after this
		// closure has released the lock — a shallow copy would let the
		// frame loop rewrite the vertices mid-export. Layouts, text
		// styles and canvas transforms are snapshotted for the same
		// reason: the export must not observe a concurrent frame.
		for i := range out {
			snapshotRenderCmd(&out[i])
		}
		return out, nil
	}()
	if err != nil {
		return printExportErrorResult(job.OutputPath, printErrorRender, err.Error())
	}

	if sourceW <= 0 || sourceH <= 0 {
		return printExportErrorResult(job.OutputPath, printErrorInvalidCfg, "source dimensions must be positive")
	}

	if pdfErr := renderToPDF(renderersCopy, job, sourceW, sourceH); pdfErr != nil {
		return printExportErrorResult(job.OutputPath, printErrorRender, pdfErr.Error())
	}
	return printExportOKResult(job.OutputPath)
}

// snapshotRenderCmd deep-copies the frame-owned data a render
// command points at, so the export can read it after the window
// lock is released. See the caller for why a shallow copy is not
// enough.
func snapshotRenderCmd(c *RenderCmd) {
	c.Triangles = append([]float32(nil), c.Triangles...)
	c.VertexColors = append([]Color(nil), c.VertexColors...)
	if c.LayoutPtr != nil {
		lp := *c.LayoutPtr
		// renderToPDF reads Text and Items only.
		lp.Items = append([]glyph.Item(nil), lp.Items...)
		c.LayoutPtr = &lp
	}
	if c.TextStylePtr != nil {
		ts := *c.TextStylePtr
		c.TextStylePtr = &ts
	}
	if c.LayoutTransform != nil {
		xf := *c.LayoutTransform
		c.LayoutTransform = &xf
	}
	if c.textPath != nil {
		tp := *c.textPath
		tp.Polyline = append([]float32(nil), tp.Polyline...)
		tp.Table = append([]float32(nil), tp.Table...)
		c.textPath = &tp
	}
}

// RunPrintJob runs the native print flow for the provided PrintJob.
//
// When the job prints the current view, the window is exported to a
// temp PDF first. On success the temp path is returned in
// PrintRunResult.PDFPath and the caller owns it: remove the file when
// done. The file must outlive the call because some backends hand it
// to an external viewer (Linux xdg-open, Windows ShellExecute) that
// opens it after ShowPrintDialog returns. On cancel or error the temp
// file is removed and PDFPath is empty.
func (w *Window) RunPrintJob(job PrintJob) PrintRunResult {
	if err := validatePrintJob(job); err != nil {
		return printRunErrorResult(printErrorInvalidCfg, err.Error())
	}
	if w.nativePlatform == nil {
		return printRunErrorResult("unsupported", "native print requires a platform backend")
	}

	pdfPath, err := printJobResolvePDFPath(w, job)
	if err != nil {
		code := printErrorInternal
		if job.Source.Kind == PrintSourcePDFPath {
			code = printErrorIO
		}
		return printRunErrorResult(code, err.Error())
	}
	// The temp PDF belongs to the caller on success (see above);
	// on cancel or error nothing consumed it, so remove it here.
	isTemp := job.Source.Kind == PrintSourceCurrentView

	pw, ph := printPageSize(job.Paper, job.Orientation)
	ranges := normalizePrintPageRanges(job.PageRanges)

	result := w.nativePlatform.ShowPrintDialog(NativePrintParams{
		Title:        job.Title,
		JobName:      job.JobName,
		PDFPath:      pdfPath,
		PaperWidth:   pw,
		PaperHeight:  ph,
		MarginTop:    job.Margins.Top,
		MarginRight:  job.Margins.Right,
		MarginBottom: job.Margins.Bottom,
		MarginLeft:   job.Margins.Left,
		Orientation:  printOrientationToInt(job.Orientation),
		Copies:       job.Copies,
		PageRanges:   printPageRangesToString(ranges),
		DuplexMode:   int(job.Duplex),
		ColorMode:    int(job.ColorMode),
		ScaleMode:    int(job.ScaleMode),
	})
	if isTemp && result.Status != PrintRunOK {
		_ = os.Remove(pdfPath)
		result.PDFPath = ""
		return result
	}
	// A backend that reports success without echoing the path would
	// leave the caller owning a temp file it cannot name. Fill it in
	// so the ownership contract above holds for every backend.
	if isTemp && result.PDFPath == "" {
		result.PDFPath = pdfPath
	}
	return result
}

// printJobResolvePDFPath resolves the PDF path for the print job.
// For current_view source, exports to a temp PDF first.
// For pdf_path source, validates the provided path.
func printJobResolvePDFPath(w *Window, job PrintJob) (string, error) {
	switch job.Source.Kind {
	case PrintSourceCurrentView:
		tmp, err := os.CreateTemp("", "go-gui-print-*.pdf")
		if err != nil {
			return "", &printError{"failed to create temp file: " + err.Error()}
		}
		_ = tmp.Close()
		exportJob := job
		exportJob.OutputPath = tmp.Name()
		result := w.ExportPrintJob(exportJob)
		if !result.IsOk() {
			_ = os.Remove(tmp.Name())
			return "", &printError{result.ErrorMessage}
		}
		return tmp.Name(), nil
	case PrintSourcePDFPath:
		path := strings.TrimSpace(job.Source.PDFPath)
		if err := validatePrintPath(path); err != nil {
			return "", &printError{err.Error()}
		}
		info, err := os.Stat(path)
		if err != nil {
			return "", &printError{"pdf_path is not readable: " + err.Error()}
		}
		// A directory, FIFO or device node stats fine and then
		// blocks or misbehaves in the backend that opens it.
		if !info.Mode().IsRegular() {
			return "", &printError{"pdf_path is not a regular file"}
		}
		return path, nil
	default:
		return "", &printError{"unknown source kind"}
	}
}

type printError struct{ msg string }

func (e *printError) Error() string { return e.msg }
