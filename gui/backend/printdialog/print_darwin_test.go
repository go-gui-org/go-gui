//go:build darwin && !ios

package printdialog

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// The error paths return before any panel shows, so they run headless.
func TestShowPrintDialogErrors(t *testing.T) {
	tests := []struct {
		name, path, msg string
	}{
		{"no path", "", "no PDF path provided"},
		{"missing file", filepath.Join(t.TempDir(), "none.pdf"), "failed to load PDF"},
		{"invalid UTF-8", "/tmp/\xff.pdf", "failed to load PDF"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ShowPrintDialog(gui.NativePrintParams{PDFPath: tc.path})
			if r.Status != gui.PrintRunError || r.ErrorCode != "print_error" ||
				r.ErrorMessage != tc.msg || r.PDFPath != "" {
				t.Errorf("ShowPrintDialog(%q) = %+v, want error %q", tc.path, r, tc.msg)
			}
		})
	}
}

// onePagePDF writes a valid one-page PDF and returns its path. The
// xref offsets are computed, so PDFKit opens it without repair.
func onePagePDF(t *testing.T) string {
	t.Helper()
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>",
	}
	var b strings.Builder
	b.WriteString("%PDF-1.4\n")
	offs := make([]int, len(objs))
	for i, o := range objs {
		offs[i] = b.Len()
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", i+1, o)
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(objs)+1)
	for _, off := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(objs)+1, xref)
	path := filepath.Join(t.TempDir(), "page.pdf")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// newOperation passes floats and an NSSize by value. Read each value
// back from the operation's print info to prove it crossed intact.
func TestNewOperationSetsPrintInfo(t *testing.T) {
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	url := pr.clsURL.Send(pr.selFileURL, darwinffi.NSString(onePagePDF(t)))
	doc := pr.clsPDFDocument.Send(pr.selAlloc).Send(pr.selInitURL, url)
	if doc == 0 {
		t.Fatal("PDFDocument did not load the test PDF")
	}
	defer doc.Send(pr.selRelease)
	info := pr.clsPrintInfo.Send(pr.selShared).Send(pr.selCopy)
	defer info.Send(pr.selRelease)

	cfg := gui.NativePrintParams{
		JobName: "job", Copies: 3, PaperWidth: 500, PaperHeight: 700, Orientation: 1,
		MarginTop: 11, MarginRight: 12, MarginBottom: 13, MarginLeft: 14,
	}
	op := newOperation(doc, info, cfg)
	if op == 0 {
		t.Fatal("newOperation = 0")
	}
	got := op.Send(darwinffi.Sel("printInfo"))
	margins := [4]float64{
		objc.Send[float64](got, darwinffi.Sel("topMargin")),
		objc.Send[float64](got, darwinffi.Sel("rightMargin")),
		objc.Send[float64](got, darwinffi.Sel("bottomMargin")),
		objc.Send[float64](got, darwinffi.Sel("leftMargin")),
	}
	if margins != [4]float64{11, 12, 13, 14} {
		t.Errorf("margins = %v, want [11 12 13 14]", margins)
	}
	// Landscape swaps the reported paper size, so compare as a set.
	size := objc.Send[nsSize](got, darwinffi.Sel("paperSize"))
	if !(size == nsSize{500, 700} || size == nsSize{700, 500}) {
		t.Errorf("paperSize = %+v, want 500x700", size)
	}
	if o := objc.Send[int](got, darwinffi.Sel("orientation")); o != nsPaperOrientationLandscape {
		t.Errorf("orientation = %d, want landscape", o)
	}
	copies := got.Send(pr.selDictionary).Send(darwinffi.Sel("objectForKey:"), pr.printCopies)
	if n := objc.Send[int](copies, darwinffi.Sel("integerValue")); n != 3 {
		t.Errorf("copies = %d, want 3", n)
	}
	title := op.Send(darwinffi.Sel("jobTitle"))
	if s := darwinffi.GoString(uintptr(title)); s != "job" {
		t.Errorf("jobTitle = %q, want %q", s, "job")
	}
}
