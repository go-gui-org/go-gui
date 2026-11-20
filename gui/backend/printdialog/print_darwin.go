//go:build darwin && !ios

// Package printdialog provides native print dialog support.
package printdialog

import (
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// The print panel is called through purego, so it works with cgo off
// (#1312). A print is a modal, one-off call, so the objc.Send
// form, which allocates per call, is fine.

// nsSize is NSSize: two CGFloats. It goes in two float registers.
type nsSize struct {
	Width, Height float64
}

// Enum values from AppKit and PDFKit headers.
const (
	nsPaperOrientationPortrait  = 0
	nsPaperOrientationLandscape = 1
	pdfPrintPageScaleToFit      = 1 // kPDFPrintPageScaleToFit
)

var pr struct {
	once sync.Once

	clsURL, clsPDFDocument, clsPrintInfo, clsNumber objc.ID
	printCopies                                     objc.ID // NSPrintCopies key

	selFileURL, selAlloc, selInitURL, selRelease, selShared, selCopy objc.SEL
	selTop, selRight, selBottom, selLeft, selPaperSize               objc.SEL
	selOrientation, selPrintOp, selShowsPanel, selShowsProgress      objc.SEL
	selJobTitle, selDictionary, selSetObject, selNumberInt, selRun   objc.SEL
}

func load() {
	pr.once.Do(func() {
		appKit := darwinffi.Open(darwinffi.PathAppKit)
		darwinffi.Open(darwinffi.PathPDFKit)
		pr.clsURL = darwinffi.Class("NSURL")
		pr.clsPDFDocument = darwinffi.Class("PDFDocument")
		pr.clsPrintInfo = darwinffi.Class("NSPrintInfo")
		pr.clsNumber = darwinffi.Class("NSNumber")
		pr.printCopies = objc.ID(darwinffi.Data(appKit, "NSPrintCopies"))
		pr.selFileURL = darwinffi.Sel("fileURLWithPath:")
		pr.selAlloc = darwinffi.Sel("alloc")
		pr.selInitURL = darwinffi.Sel("initWithURL:")
		pr.selRelease = darwinffi.Sel("release")
		pr.selShared = darwinffi.Sel("sharedPrintInfo")
		pr.selCopy = darwinffi.Sel("copy")
		pr.selTop = darwinffi.Sel("setTopMargin:")
		pr.selRight = darwinffi.Sel("setRightMargin:")
		pr.selBottom = darwinffi.Sel("setBottomMargin:")
		pr.selLeft = darwinffi.Sel("setLeftMargin:")
		pr.selPaperSize = darwinffi.Sel("setPaperSize:")
		pr.selOrientation = darwinffi.Sel("setOrientation:")
		pr.selPrintOp = darwinffi.Sel("printOperationForPrintInfo:scalingMode:autoRotate:")
		pr.selShowsPanel = darwinffi.Sel("setShowsPrintPanel:")
		pr.selShowsProgress = darwinffi.Sel("setShowsProgressPanel:")
		pr.selJobTitle = darwinffi.Sel("setJobTitle:")
		pr.selDictionary = darwinffi.Sel("dictionary")
		pr.selSetObject = darwinffi.Sel("setObject:forKey:")
		pr.selNumberInt = darwinffi.Sel("numberWithInteger:")
		pr.selRun = darwinffi.Sel("runOperation")
	})
}

// ShowPrintDialog shows the native macOS print dialog for a PDF.
func ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	status, errMsg := runPrint(cfg)
	var pdfPath, errCode string
	switch status {
	case gui.PrintRunOK:
		pdfPath = cfg.PDFPath
	case gui.PrintRunError:
		errCode = "print_error"
	}
	return gui.PrintRunResult{
		Status:       status,
		ErrorCode:    errCode,
		ErrorMessage: errMsg,
		PDFPath:      pdfPath,
	}
}

// runPrint loads the PDF at cfg.PDFPath and runs the print panel for
// it. cfg.Title, PageRanges, DuplexMode, ColorMode and ScaleMode are
// not used: the panel lets the user pick them, as in the cgo form.
func runPrint(cfg gui.NativePrintParams) (gui.PrintRunStatus, string) {
	if cfg.PDFPath == "" {
		return gui.PrintRunError, "no PDF path provided"
	}
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	path := darwinffi.NSString(cfg.PDFPath)
	if path == 0 {
		return gui.PrintRunError, "failed to load PDF"
	}
	url := pr.clsURL.Send(pr.selFileURL, path)
	doc := pr.clsPDFDocument.Send(pr.selAlloc).Send(pr.selInitURL, url)
	if doc == 0 {
		return gui.PrintRunError, "failed to load PDF"
	}
	defer doc.Send(pr.selRelease)

	// A copy, so the process-wide shared print info does not change.
	info := pr.clsPrintInfo.Send(pr.selShared).Send(pr.selCopy)
	defer info.Send(pr.selRelease)
	op := newOperation(doc, info, cfg)
	if op == 0 {
		return gui.PrintRunError, "failed to create print operation"
	}

	// runOperation returns NO for a cancel and for an error. The two
	// cannot be told apart, so NO counts as a cancel.
	if objc.Send[bool](op, pr.selRun) {
		return gui.PrintRunOK, ""
	}
	return gui.PrintRunCancel, ""
}

// newOperation sets up print info and returns an autoreleased print
// operation for doc, ready to run. It shows nothing, so a test can call
// it. Call it inside an autorelease pool.
func newOperation(doc, info objc.ID, cfg gui.NativePrintParams) objc.ID {
	info.Send(pr.selTop, float64(cfg.MarginTop))
	info.Send(pr.selRight, float64(cfg.MarginRight))
	info.Send(pr.selBottom, float64(cfg.MarginBottom))
	info.Send(pr.selLeft, float64(cfg.MarginLeft))
	if cfg.PaperWidth > 0 && cfg.PaperHeight > 0 {
		info.Send(pr.selPaperSize, nsSize{
			Width:  float64(cfg.PaperWidth),
			Height: float64(cfg.PaperHeight),
		})
	}
	orientation := nsPaperOrientationPortrait
	if cfg.Orientation == 1 {
		orientation = nsPaperOrientationLandscape
	}
	info.Send(pr.selOrientation, orientation)
	// Before the operation is built: the operation takes its own copy
	// of info, so a value set after it is lost. The cgo form set the
	// copies after, and every print came out as one copy.
	if cfg.Copies > 1 {
		n := pr.clsNumber.Send(pr.selNumberInt, cfg.Copies)
		info.Send(pr.selDictionary).Send(pr.selSetObject, n, pr.printCopies)
	}

	op := doc.Send(pr.selPrintOp, info, pdfPrintPageScaleToFit, true)
	if op == 0 {
		return 0
	}
	op.Send(pr.selShowsPanel, true)
	op.Send(pr.selShowsProgress, true)
	if cfg.JobName != "" {
		if name := darwinffi.NSString(cfg.JobName); name != 0 {
			op.Send(pr.selJobTitle, name)
		}
	}
	return op
}
