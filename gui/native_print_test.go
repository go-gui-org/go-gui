package gui

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

// tempPDFPath creates an empty PDF placeholder for resolve tests.
func tempPDFPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "in.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestExportPrintJobNoOutputPath(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	job := NewPrintJob()
	result := w.ExportPrintJob(job)
	if result.IsOk() {
		t.Error("expected error for empty output path")
	}
	if result.ErrorCode != printErrorInvalidCfg {
		t.Errorf("ErrorCode: got %q, want %q", result.ErrorCode, printErrorInvalidCfg)
	}
}

func TestExportPrintJobBadCopies(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	job := NewPrintJob()
	job.OutputPath = "/tmp/out.pdf"
	job.Copies = 0
	result := w.ExportPrintJob(job)
	if result.IsOk() {
		t.Error("expected error for zero copies")
	}
}

func TestExportPrintJobNoRenderers(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	job := NewPrintJob()
	job.OutputPath = "/tmp/out.pdf"
	result := w.ExportPrintJob(job)
	if result.IsOk() {
		t.Error("expected error for no renderers")
	}
	if result.ErrorCode != printErrorRender {
		t.Errorf("ErrorCode: got %q, want %q", result.ErrorCode, printErrorRender)
	}
}

func TestRunPrintJobNoPlatform(t *testing.T) {
	w := &Window{}
	job := NewPrintJob()
	result := w.RunPrintJob(job)
	if result.Status != PrintRunError {
		t.Errorf("Status: got %d, want %d", result.Status, PrintRunError)
	}
	if result.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q", result.ErrorCode)
	}
}

func TestRunPrintJobBadCopies(t *testing.T) {
	w := &Window{}
	job := NewPrintJob()
	job.Copies = 0
	result := w.RunPrintJob(job)
	if result.Status != PrintRunError {
		t.Errorf("expected error status")
	}
	if result.ErrorCode != printErrorInvalidCfg {
		t.Errorf("ErrorCode: got %q", result.ErrorCode)
	}
}

func TestRunPrintJobPDFPathSource(t *testing.T) {
	w := &Window{}
	job := NewPrintJob()
	job.Source = PrintJobSource{Kind: PrintSourcePDFPath, PDFPath: tempPDFPath(t)}
	result := w.RunPrintJob(job)
	// No platform → unsupported.
	if result.Status != PrintRunError {
		t.Errorf("expected error")
	}
}

func TestPrintJobResolvePDFPathEmpty(t *testing.T) {
	w := &Window{}
	job := NewPrintJob()
	job.Source = PrintJobSource{Kind: PrintSourcePDFPath}
	_, err := printJobResolvePDFPath(w, job)
	if err == nil {
		t.Error("expected error for empty pdf_path")
	}
}

func TestPrintJobResolvePDFPathOK(t *testing.T) {
	w := &Window{}
	want := tempPDFPath(t)
	job := NewPrintJob()
	job.Source = PrintJobSource{Kind: PrintSourcePDFPath, PDFPath: want}
	path, err := printJobResolvePDFPath(w, job)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if path != want {
		t.Errorf("path: got %q", path)
	}
}

func TestPrintJobResolvePDFPathMissing(t *testing.T) {
	w := &Window{}
	job := NewPrintJob()
	job.Source = PrintJobSource{
		Kind:    PrintSourcePDFPath,
		PDFPath: filepath.Join(t.TempDir(), "absent.pdf"),
	}
	if _, err := printJobResolvePDFPath(w, job); err == nil {
		t.Error("expected error for unreadable pdf_path")
	}
}

func TestPrintJobResolvePDFPathLeadingDash(t *testing.T) {
	w := &Window{}
	job := NewPrintJob()
	job.Source = PrintJobSource{Kind: PrintSourcePDFPath, PDFPath: "-evil.pdf"}
	if _, err := printJobResolvePDFPath(w, job); err == nil {
		t.Error("expected error for leading-dash pdf_path")
	}
}

func TestPrintJobResolvePDFPathNUL(t *testing.T) {
	w := &Window{}
	job := NewPrintJob()
	job.Source = PrintJobSource{
		Kind:    PrintSourcePDFPath,
		PDFPath: "a\x00b.pdf",
	}
	if _, err := printJobResolvePDFPath(w, job); err == nil {
		t.Error("expected error for NUL pdf_path")
	}
}

func TestPrintJobResolveCurrentViewNotImpl(t *testing.T) {
	w := &Window{}
	job := NewPrintJob()
	_, err := printJobResolvePDFPath(w, job)
	if err == nil {
		t.Error("expected not-implemented error")
	}
}

// mockPrintPlatform returns a preset PrintRunResult.
type mockPrintPlatform struct {
	noopNativePlatform
	result PrintRunResult
}

func (m mockPrintPlatform) ShowPrintDialog(_ NativePrintParams) PrintRunResult {
	return m.result
}

// funcPrintPlatform routes ShowPrintDialog to fn.
type funcPrintPlatform struct {
	noopNativePlatform
	fn func(NativePrintParams) PrintRunResult
}

func (m funcPrintPlatform) ShowPrintDialog(p NativePrintParams) PrintRunResult {
	return m.fn(p)
}

func TestRunPrintJobSuccess(t *testing.T) {
	pdfPath := tempPDFPath(t)
	w := &Window{}
	w.nativePlatform = mockPrintPlatform{
		result: PrintRunResult{
			Status:  PrintRunOK,
			PDFPath: "/tmp/out.pdf",
		},
	}
	job := NewPrintJob()
	job.Copies = 1
	job.Source = PrintJobSource{Kind: PrintSourcePDFPath, PDFPath: pdfPath}
	result := w.RunPrintJob(job)
	if result.Status != PrintRunOK {
		t.Errorf("Status: got %d, want %d", result.Status, PrintRunOK)
	}
	if result.PDFPath != "/tmp/out.pdf" {
		t.Errorf("PDFPath: got %q", result.PDFPath)
	}
}

func TestRunPrintJobCancelled(t *testing.T) {
	pdfPath := tempPDFPath(t)
	w := &Window{}
	w.nativePlatform = mockPrintPlatform{
		result: PrintRunResult{Status: PrintRunCancel},
	}
	job := NewPrintJob()
	job.Copies = 1
	job.Source = PrintJobSource{Kind: PrintSourcePDFPath, PDFPath: pdfPath}
	result := w.RunPrintJob(job)
	if result.Status != PrintRunCancel {
		t.Errorf("Status: got %d, want %d", result.Status, PrintRunCancel)
	}
}

func TestRunPrintJobPlatformError(t *testing.T) {
	w := &Window{}
	w.nativePlatform = mockPrintPlatform{
		result: PrintRunResult{
			Status:       PrintRunError,
			ErrorCode:    "no_printer",
			ErrorMessage: "no default printer configured",
		},
	}
	job := NewPrintJob()
	job.Copies = 1
	job.Source = PrintJobSource{Kind: PrintSourcePDFPath, PDFPath: tempPDFPath(t)}
	result := w.RunPrintJob(job)
	if result.Status != PrintRunError {
		t.Errorf("Status: got %d, want %d", result.Status, PrintRunError)
	}
	if result.ErrorCode != "no_printer" {
		t.Errorf("ErrorCode: got %q", result.ErrorCode)
	}
	if result.ErrorMessage == "" {
		t.Error("expected non-empty ErrorMessage")
	}
}

// On success the current-view temp PDF transfers to the caller:
// backends that hand the file to an external viewer (xdg-open,
// ShellExecute) need it alive after ShowPrintDialog returns.
func TestRunPrintJobTempOwnedOnSuccess(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	w.renderers = []RenderCmd{{
		Kind: RenderRect, X: 0, Y: 0, W: 800, H: 600,
		Color: RGBA(255, 255, 255, 255),
	}}
	w.nativePlatform = funcPrintPlatform{fn: func(p NativePrintParams) PrintRunResult {
		return PrintRunResult{Status: PrintRunOK, PDFPath: p.PDFPath}
	}}
	result := w.RunPrintJob(NewPrintJob())
	if result.Status != PrintRunOK {
		t.Fatalf("Status: got %d, want %d", result.Status, PrintRunOK)
	}
	if result.PDFPath == "" {
		t.Fatal("expected owned temp PDF path")
	}
	if _, err := os.Stat(result.PDFPath); err != nil {
		t.Errorf("owned temp removed too early: %v", err)
	}
	_ = os.Remove(result.PDFPath)
}

// On cancel or error nothing consumed the temp PDF, so RunPrintJob
// removes it and clears the path instead of returning a dangling one.
func TestRunPrintJobTempRemovedOnCancel(t *testing.T) {
	for _, status := range []PrintRunStatus{PrintRunCancel, PrintRunError} {
		w := &Window{windowWidth: 800, windowHeight: 600}
		w.renderers = []RenderCmd{{
			Kind: RenderRect, X: 0, Y: 0, W: 800, H: 600,
			Color: RGBA(255, 255, 255, 255),
		}}
		var shown NativePrintParams
		w.nativePlatform = funcPrintPlatform{fn: func(p NativePrintParams) PrintRunResult {
			shown = p
			return PrintRunResult{Status: status}
		}}
		result := w.RunPrintJob(NewPrintJob())
		if result.Status != status {
			t.Errorf("Status: got %d, want %d", result.Status, status)
		}
		if result.PDFPath != "" {
			t.Errorf("PDFPath: got %q, want empty", result.PDFPath)
		}
		if _, err := os.Stat(shown.PDFPath); !os.IsNotExist(err) {
			t.Errorf("temp %q not removed (stat err %v)", shown.PDFPath, err)
			_ = os.Remove(shown.PDFPath)
		}
	}
}

// A backend can report success without echoing the path back;
// RunPrintJob still has to name the temp file the caller now owns.
func TestRunPrintJobTempPathFilledWhenBackendSilent(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	w.renderers = []RenderCmd{{
		Kind: RenderRect, X: 0, Y: 0, W: 800, H: 600,
		Color: RGBA(255, 255, 255, 255),
	}}
	w.nativePlatform = funcPrintPlatform{fn: func(_ NativePrintParams) PrintRunResult {
		return PrintRunResult{Status: PrintRunOK}
	}}
	result := w.RunPrintJob(NewPrintJob())
	if result.Status != PrintRunOK {
		t.Fatalf("Status: got %d, want %d", result.Status, PrintRunOK)
	}
	if result.PDFPath == "" {
		t.Fatal("expected temp path filled in for the caller")
	}
	if _, err := os.Stat(result.PDFPath); err != nil {
		t.Errorf("owned temp missing: %v", err)
	}
	_ = os.Remove(result.PDFPath)
}

// A directory stats fine and then misbehaves in the backend that
// opens it, so pdf_path takes regular files only.
func TestPrintJobResolvePDFPathDirectory(t *testing.T) {
	w := &Window{}
	job := NewPrintJob()
	job.Source = PrintJobSource{Kind: PrintSourcePDFPath, PDFPath: t.TempDir()}
	if _, err := printJobResolvePDFPath(w, job); err == nil {
		t.Error("expected error for directory pdf_path")
	}
}

func TestExportPrintJobNonFiniteSourceDims(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	job := NewPrintJob()
	job.OutputPath = filepath.Join(t.TempDir(), "out.pdf")
	job.SourceWidth = float32(math.NaN())
	if result := w.ExportPrintJob(job); result.IsOk() {
		t.Error("expected error for NaN source width")
	}
}
