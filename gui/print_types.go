package gui

import (
	"cmp"
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
type PaperSize uint8

// PaperSize constants.
const (
	PaperLetter PaperSize = iota
	PaperLegal
	PaperA4
	PaperA3
)

// PrintOrientation selects portrait or landscape.
type PrintOrientation uint8

// PrintOrientation constants.
const (
	PrintPortrait PrintOrientation = iota
	PrintLandscape
)

// PrintMargins defines page margins in points (1/72 inch).
type PrintMargins struct {
	Top    float32
	Right  float32
	Bottom float32
	Left   float32
}

// DefaultPrintMargins returns 36-point margins (0.5 inch).
func DefaultPrintMargins() PrintMargins {
	return PrintMargins{Top: 36, Right: 36, Bottom: 36, Left: 36}
}

// PrintScaleMode controls content scaling.
type PrintScaleMode uint8

// PrintScaleMode constants.
const (
	PrintScaleFitToPage PrintScaleMode = iota
	PrintScaleActualSize
)

// PrintDuplexMode controls duplex printing.
type PrintDuplexMode uint8

// PrintDuplexMode constants.
const (
	PrintDuplexDefault PrintDuplexMode = iota
	PrintDuplexSimplex
	PrintDuplexLongEdge
	PrintDuplexShortEdge
)

// PrintColorMode controls color output.
type PrintColorMode uint8

// PrintColorMode constants.
const (
	PrintColorDefault PrintColorMode = iota
	PrintColorColor
	PrintColorGrayscale
)

// PrintPageRange defines a contiguous page range (1-based).
type PrintPageRange struct {
	From int
	To   int
}

// PrintHeaderFooterCfg configures page header or footer text.
// Tokens: {page}, {pages}, {date}, {title}, {job}.
type PrintHeaderFooterCfg struct {
	Left    string
	Center  string
	Right   string
	Enabled bool
}

// PrintJobSourceKind selects the print source.
type PrintJobSourceKind uint8

// PrintJobSourceKind constants.
const (
	PrintSourceCurrentView PrintJobSourceKind = iota
	PrintSourcePDFPath
)

// PrintJobSource identifies what to print.
type PrintJobSource struct {
	PDFPath string
	Kind    PrintJobSourceKind
}

// PrintJob configures a print or PDF export operation.
type PrintJob struct {
	Header       PrintHeaderFooterCfg
	Footer       PrintHeaderFooterCfg
	Source       PrintJobSource
	OutputPath   string
	Title        string
	JobName      string
	PageRanges   []PrintPageRange
	Copies       int
	RasterDPI    int
	JPEGQuality  int
	Margins      PrintMargins
	SourceWidth  float32
	SourceHeight float32
	Paper        PaperSize
	Orientation  PrintOrientation
	Paginate     bool
	ScaleMode    PrintScaleMode
	Duplex       PrintDuplexMode
	ColorMode    PrintColorMode
}

// NewPrintJob returns a PrintJob with sensible defaults.
func NewPrintJob() PrintJob {
	return PrintJob{
		Paper:       PaperA4,
		Orientation: PrintPortrait,
		Margins:     DefaultPrintMargins(),
		Copies:      1,
		RasterDPI:   300,
		JPEGQuality: 85,
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

// PrintWarning describes a non-fatal issue during printing.
type PrintWarning struct {
	Code    string
	Message string
}

// PrintRunResult contains the outcome of RunPrintJob.
type PrintRunResult struct {
	ErrorCode    string
	ErrorMessage string
	PDFPath      string
	Warnings     []PrintWarning
	Status       PrintRunStatus
}

// PrintExportStatus reports the outcome of ExportPrintJob.
type PrintExportStatus uint8

// PrintExportStatus constants.
const (
	PrintExportOK PrintExportStatus = iota
	PrintExportError
)

// PrintExportResult contains the outcome of ExportPrintJob.
type PrintExportResult struct {
	Path         string
	ErrorCode    string
	ErrorMessage string
	Status       PrintExportStatus
}

// IsOk returns true if the export succeeded.
func (r PrintExportResult) IsOk() bool {
	return r.Status == PrintExportOK
}

// --- result constructors ---

func printRunErrorResult(code, message string) PrintRunResult {
	return PrintRunResult{Status: PrintRunError, ErrorCode: code, ErrorMessage: message}
}

func printExportErrorResult(path, code, message string) PrintExportResult {
	return PrintExportResult{Status: PrintExportError, Path: path, ErrorCode: code, ErrorMessage: message}
}

func printExportOKResult(path string) PrintExportResult {
	return PrintExportResult{Status: PrintExportOK, Path: path}
}

// --- page geometry ---

// PrintPageSize returns (width, height) in points for the given
// paper size and orientation.
func PrintPageSize(paper PaperSize, orientation PrintOrientation) (float32, float32) {
	var w, h float32
	switch paper {
	case PaperLetter:
		w, h = 612, 792
	case PaperLegal:
		w, h = 612, 1008
	case PaperA4:
		w, h = 595, 842
	case PaperA3:
		w, h = 842, 1191
	default:
		w, h = 595, 842
	}
	if orientation == PrintLandscape {
		return h, w
	}
	return w, h
}

// --- validation ---

func validatePrintMargins(pageW, pageH float32, m PrintMargins) error {
	if m.Left < 0 || m.Right < 0 || m.Top < 0 || m.Bottom < 0 {
		return fmt.Errorf("margins must be non-negative")
	}
	if m.Left+m.Right >= pageW {
		return fmt.Errorf("horizontal margins exceed printable width")
	}
	if m.Top+m.Bottom >= pageH {
		return fmt.Errorf("vertical margins exceed printable height")
	}
	return nil
}

func validatePrintJob(job PrintJob) error {
	pw, ph := PrintPageSize(job.Paper, job.Orientation)
	if err := validatePrintMargins(pw, ph, job.Margins); err != nil {
		return err
	}
	if job.Copies < 1 {
		return fmt.Errorf("copies must be >= 1")
	}
	if job.Source.Kind == PrintSourcePDFPath {
		if strings.TrimSpace(job.Source.PDFPath) == "" {
			return fmt.Errorf("pdf_path is required for pdf_path source")
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
	if err := validateHeaderFooterCfg(job.Footer); err != nil {
		return err
	}
	if job.RasterDPI < 72 || job.RasterDPI > 1200 {
		return fmt.Errorf("raster_dpi must be 72..1200")
	}
	if job.JPEGQuality < 10 || job.JPEGQuality > 100 {
		return fmt.Errorf("jpeg_quality must be 10..100")
	}
	return nil
}

func validateExportPrintJob(job PrintJob) error {
	if strings.TrimSpace(job.OutputPath) == "" {
		return fmt.Errorf("output_path is required")
	}
	return validatePrintJob(job)
}

func validateHeaderFooterCfg(cfg PrintHeaderFooterCfg) error {
	if !cfg.Enabled {
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
func NormalizePrintPageRanges(ranges []PrintPageRange) []PrintPageRange {
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
