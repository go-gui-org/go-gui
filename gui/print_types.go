package gui

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Error code constants for print operations.
const (
	printErrorInvalidCfg = "invalid_cfg"
	printErrorIO         = "io_error"
	printErrorRender     = "render_error"
	printErrorInternal   = "internal"
)

// PaperSize selects standard paper dimensions.
// exportaudit:keep — reachable from an exported signature
type PaperSize uint8

// PaperSize constants.
const (
	paperLetter PaperSize = iota
	paperLegal
	paperA4
	paperA3
)

// PrintOrientation selects portrait or landscape.
// exportaudit:keep — reachable from an exported signature
type PrintOrientation uint8

// PrintOrientation constants.
const (
	printPortrait PrintOrientation = iota
	printLandscape
)

// PrintMargins defines page margins in points (1/72 inch).
// exportaudit:keep — reachable from an exported signature
type PrintMargins struct {
	Top    float32
	Right  float32
	Bottom float32
	Left   float32
}

// DefaultPrintMargins returns 36-point margins (0.5 inch).
func defaultPrintMargins() PrintMargins {
	return PrintMargins{Top: 36, Right: 36, Bottom: 36, Left: 36}
}

// PrintScaleMode controls content scaling.
type printScaleMode uint8

// PrintScaleMode constants.
const (
	printScaleFitToPage printScaleMode = iota
	printScaleActualSize
)

// PrintDuplexMode controls duplex printing.
type printDuplexMode uint8

// PrintColorMode controls color output.
type printColorMode uint8

// PrintPageRange defines a contiguous page range (1-based).
// exportaudit:keep — reachable from an exported signature
type PrintPageRange struct {
	From int
	To   int
}

// PrintHeaderFooterCfg configures page header or footer text.
// Tokens: {page}, {pages}, {date}, {title}, {job}.
type printHeaderFooterCfg struct {
	Left    string
	Center  string
	Right   string
	enabled bool
}

// PrintJobSourceKind selects the print source.
type printJobSourceKind uint8

// PrintJobSourceKind constants.
const (
	printSourceCurrentView printJobSourceKind = iota
	printSourcePDFPath
)

// PrintJobSource identifies what to print.
type printJobSource struct {
	PDFPath string
	Kind    printJobSourceKind
}

// PrintJob configures a print or PDF export operation.
// exportaudit:keep — reachable from an exported signature
type PrintJob struct {
	Header       printHeaderFooterCfg
	footer       printHeaderFooterCfg
	Source       printJobSource
	OutputPath   string
	Title        string
	JobName      string
	PageRanges   []PrintPageRange
	Copies       int
	rasterDPI    int
	jPEGQuality  int
	margins      PrintMargins
	sourceWidth  float32
	sourceHeight float32
	paper        PaperSize
	Orientation  PrintOrientation
	ScaleMode    printScaleMode
	duplex       printDuplexMode
	ColorMode    printColorMode
}

// NewPrintJob returns a PrintJob with sensible defaults.
func NewPrintJob() PrintJob {
	return PrintJob{
		paper:       paperA4,
		Orientation: printPortrait,
		margins:     defaultPrintMargins(),
		Copies:      1,
		rasterDPI:   300,
		jPEGQuality: 85,
	}
}

// PrintRunStatus reports the outcome of a native print dialog.
type PrintRunStatus uint8

// PrintRunStatus constants.
const (
	PrintRunOK PrintRunStatus = iota
	PrintRunCancel
	PrintRunError
)

// PrintRunResult contains the outcome of RunPrintJob.
type PrintRunResult struct {
	ErrorCode    string
	ErrorMessage string
	PDFPath      string
	Status       PrintRunStatus
}

// PrintExportStatus reports the outcome of ExportPrintJob.
type printExportStatus uint8

// PrintExportStatus constants.
const (
	printExportOK printExportStatus = iota
	printExportError
)

// PrintExportResult contains the outcome of ExportPrintJob.
// exportaudit:keep — reachable from an exported signature
type PrintExportResult struct {
	Path         string
	ErrorCode    string
	ErrorMessage string
	Status       printExportStatus
}

// IsOk returns true if the export succeeded.
func (r PrintExportResult) isOk() bool {
	return r.Status == printExportOK
}

// --- result constructors ---

func printRunErrorResult(code, message string) PrintRunResult {
	return PrintRunResult{Status: PrintRunError, ErrorCode: code, ErrorMessage: message}
}

func printExportErrorResult(path, code, message string) PrintExportResult {
	return PrintExportResult{Status: printExportError, Path: path, ErrorCode: code, ErrorMessage: message}
}

func printExportOKResult(path string) PrintExportResult {
	return PrintExportResult{Status: printExportOK, Path: path}
}

// --- page geometry ---

// PrintPageSize returns (width, height) in points for the given
// paper size and orientation.
func printPageSize(paper PaperSize, orientation PrintOrientation) (float32, float32) {
	var w, h float32
	switch paper {
	case paperLetter:
		w, h = 612, 792
	case paperLegal:
		w, h = 612, 1008
	case paperA4:
		w, h = 595, 842
	case paperA3:
		w, h = 842, 1191
	default:
		w, h = 595, 842
	}
	if orientation == printLandscape {
		return h, w
	}
	return w, h
}

// --- validation ---

func validatePrintMargins(pageW, pageH float32, m PrintMargins) error {
	if m.Left < 0 || m.Right < 0 || m.Top < 0 || m.Bottom < 0 {
		return errors.New("margins must be non-negative")
	}
	if m.Left+m.Right >= pageW {
		return errors.New("horizontal margins exceed printable width")
	}
	if m.Top+m.Bottom >= pageH {
		return errors.New("vertical margins exceed printable height")
	}
	return nil
}

func validatePrintJob(job PrintJob) error {
	pw, ph := printPageSize(job.paper, job.Orientation)
	if err := validatePrintMargins(pw, ph, job.margins); err != nil {
		return err
	}
	if job.Copies < 1 {
		return errors.New("copies must be >= 1")
	}
	if job.Source.Kind == printSourcePDFPath {
		if strings.TrimSpace(job.Source.PDFPath) == "" {
			return errors.New("pdf_path is required for pdf_path source")
		}
	}
	for _, r := range job.PageRanges {
		if r.From < 1 || r.To < r.From {
			return fmt.Errorf("invalid page range %d-%d", r.From, r.To)
		}
	}
	if err := validateHeaderFooterCfg(job.Header); err != nil {
		return err
	}
	if err := validateHeaderFooterCfg(job.footer); err != nil {
		return err
	}
	if job.rasterDPI < 72 || job.rasterDPI > 1200 {
		return errors.New("raster_dpi must be 72..1200")
	}
	if job.jPEGQuality < 10 || job.jPEGQuality > 100 {
		return errors.New("jpeg_quality must be 10..100")
	}
	return nil
}

func validateExportPrintJob(job PrintJob) error {
	if strings.TrimSpace(job.OutputPath) == "" {
		return errors.New("output_path is required")
	}
	return validatePrintJob(job)
}

func validateHeaderFooterCfg(cfg printHeaderFooterCfg) error {
	if !cfg.enabled {
		return nil
	}
	for _, token := range extractPrintTokens(cfg.Left) {
		if err := validatePrintToken(token); err != nil {
			return err
		}
	}
	for _, token := range extractPrintTokens(cfg.Center) {
		if err := validatePrintToken(token); err != nil {
			return err
		}
	}
	for _, token := range extractPrintTokens(cfg.Right) {
		if err := validatePrintToken(token); err != nil {
			return err
		}
	}
	return nil
}

func extractPrintTokens(text string) []string {
	var tokens []string
	i := 0
	for i < len(text) {
		if text[i] == '{' {
			j := i + 1
			for j < len(text) && text[j] != '}' {
				j++
			}
			if j < len(text) && j > i+1 {
				tokens = append(tokens, text[i+1:j])
				i = j + 1
				continue
			}
		}
		i++
	}
	return tokens
}

func validatePrintToken(token string) error {
	switch token {
	case "page", "pages", "date", "title", "job":
		return nil
	}
	return fmt.Errorf("unsupported print token {%s}", token)
}

// NormalizePrintPageRanges sorts and merges overlapping ranges.
func normalizePrintPageRanges(ranges []PrintPageRange) []PrintPageRange {
	if len(ranges) == 0 {
		return nil
	}
	out := slices.Clone(ranges)
	slices.SortFunc(out, func(a, b PrintPageRange) int {
		return cmp.Compare(a.From, b.From)
	})
	merged := []PrintPageRange{out[0]}
	for _, r := range out[1:] {
		last := &merged[len(merged)-1]
		if r.From <= last.To+1 {
			last.To = max(last.To, r.To)
		} else {
			merged = append(merged, r)
		}
	}
	return merged
}

// printOrientationToInt converts orientation to int for bridge.
func printOrientationToInt(o PrintOrientation) int {
	return int(o)
}

// printPageRangesToString formats ranges as "1-3,5-7".
func printPageRangesToString(ranges []PrintPageRange) string {
	if len(ranges) == 0 {
		return ""
	}
	var b strings.Builder
	for i, r := range ranges {
		if i > 0 {
			b.WriteByte(',')
		}
		if r.From == r.To {
			fmt.Fprintf(&b, "%d", r.From)
		} else {
			fmt.Fprintf(&b, "%d-%d", r.From, r.To)
		}
	}
	return b.String()
}
