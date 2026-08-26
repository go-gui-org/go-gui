package gui

import (
	"os"
	"strings"
)

// ExportPrintJob exports renderer output to PDF using PrintJob settings.
// Returns a PrintExportResult with status and path.
func (w *Window) ExportPrintJob(job PrintJob) PrintExportResult {
	if err := validateExportPrintJob(job); err != nil {
		return printExportErrorResult(job.OutputPath, printErrorInvalidCfg, err.Error())
	}

	sourceW := job.sourceWidth
	sourceH := job.sourceHeight

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
		// frame loop rewrite the vertices mid-export.
		for i := range out {
			out[i].Triangles = append([]float32(nil), out[i].Triangles...)
			out[i].VertexColors = append([]Color(nil), out[i].VertexColors...)
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

// RunPrintJob runs the native print flow for the provided PrintJob.
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
		if job.Source.Kind == printSourcePDFPath {
			code = printErrorIO
		}
		return printRunErrorResult(code, err.Error())
	}
	// Clean up temp PDF after dialog returns.
	if job.Source.Kind == printSourceCurrentView {
		defer func() { _ = os.Remove(pdfPath) }()
	}

	pw, ph := printPageSize(job.paper, job.Orientation)
	ranges := normalizePrintPageRanges(job.PageRanges)

	result := w.nativePlatform.ShowPrintDialog(NativePrintParams{
		Title:        job.Title,
		JobName:      job.JobName,
		PDFPath:      pdfPath,
		PaperWidth:   pw,
		PaperHeight:  ph,
		MarginTop:    job.margins.Top,
		MarginRight:  job.margins.Right,
		MarginBottom: job.margins.Bottom,
		MarginLeft:   job.margins.Left,
		Orientation:  printOrientationToInt(job.Orientation),
		Copies:       job.Copies,
		PageRanges:   printPageRangesToString(ranges),
		DuplexMode:   int(job.duplex),
		ColorMode:    int(job.ColorMode),
		ScaleMode:    int(job.ScaleMode),
	})
	return result
}

// printJobResolvePDFPath resolves the PDF path for the print job.
// For current_view source, exports to a temp PDF first.
// For pdf_path source, validates the provided path.
func printJobResolvePDFPath(w *Window, job PrintJob) (string, error) {
	switch job.Source.Kind {
	case printSourceCurrentView:
		tmp, err := os.CreateTemp("", "go-gui-print-*.pdf")
		if err != nil {
			return "", &printError{"failed to create temp file: " + err.Error()}
		}
		_ = tmp.Close()
		exportJob := job
		exportJob.OutputPath = tmp.Name()
		result := w.ExportPrintJob(exportJob)
		if !result.isOk() {
			_ = os.Remove(tmp.Name())
			return "", &printError{result.ErrorMessage}
		}
		return tmp.Name(), nil
	case printSourcePDFPath:
		path := strings.TrimSpace(job.Source.PDFPath)
		if path == "" {
			return "", &printError{"pdf_path is required"}
		}
		return path, nil
	default:
		return "", &printError{"unknown source kind"}
	}
}

type printError struct{ msg string }

func (e *printError) Error() string { return e.msg }
