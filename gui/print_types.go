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
	// exportaudit:keep — caller-facing print API
	PaperLetter PaperSize = iota
	// exportaudit:keep — caller-facing print API
	PaperLegal
	// exportaudit:keep — caller-facing print API
	PaperA4
	// exportaudit:keep — caller-facing print API
	PaperA3
)

// PrintOrientation selects portrait or landscape.
// exportaudit:keep — reachable from an exported signature
type PrintOrientation uint8

// PrintOrientation constants.
const (
	// exportaudit:keep — caller-facing print API
	PrintPortrait PrintOrientation = iota
	// exportaudit:keep — caller-facing print API
	PrintLandscape
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

// maxPrintCopies caps PrintJob.Copies. The value passes through to
// the native spooler (lpr -#N), so an unchecked int turns a typo
// into wasted paper.
const maxPrintCopies = 9999

// maxPrintPageRanges and maxPrintPage cap PrintJob.PageRanges. The
// ranges are flattened into one argv string for the spooler, so an
// unbounded list is an E2BIG away from a failed print, and a page
// number near MaxInt overflows the merge in
// normalizePrintPageRanges.
const (
	maxPrintPageRanges = 1024
	maxPrintPage       = 1_000_000
)

// PrintScaleMode controls content scaling.
// exportaudit:keep — reachable from an exported signature
type PrintScaleMode uint8

// PrintScaleMode constants.
const (
	// exportaudit:keep — caller-facing print API
	PrintScaleFitToPage PrintScaleMode = iota
	// exportaudit:keep — caller-facing print API
	PrintScaleActualSize
)

// PrintDuplexMode controls duplex printing.
// exportaudit:keep — reachable from an exported signature
type PrintDuplexMode uint8

// PrintDuplexMode constants.
const (
	// exportaudit:keep — caller-facing print API
	PrintDuplexOff PrintDuplexMode = iota
	PrintDuplexLongEdge
	PrintDuplexShortEdge
)

// PrintColorMode controls color output.
// exportaudit:keep — reachable from an exported signature
type PrintColorMode uint8

// PrintColorMode constants.
const (
	// exportaudit:keep — caller-facing print API
	PrintColorModeColor PrintColorMode = iota
	PrintColorModeGrayscale
)

// PrintPageRange defines a contiguous page range (1-based).
// exportaudit:keep — reachable from an exported signature
type PrintPageRange struct {
	From int
	To   int
}

// PrintHeaderFooterCfg configures page header or footer text.
// Tokens: {page}, {pages}, {date}, {title}, {job}.
// exportaudit:keep — caller-facing config (issue #372)
type PrintHeaderFooterCfg struct {
	Left   string
	Center string
	Right  string
	// Enabled toggles the header/footer on the page.
	// exportaudit:keep — caller-facing config (issue #372)
	Enabled bool
}

// PrintSource selects what a print job prints.
// exportaudit:keep — reachable from an exported signature
type PrintSource uint8

// PrintSource constants.
const (
	// exportaudit:keep — caller-facing print API
	PrintSourceCurrentView PrintSource = iota
	// exportaudit:keep — caller-facing print API
	PrintSourcePDFPath
)

// PrintJobSource identifies what to print: the current view, or an
// existing PDF file at PDFPath.
// exportaudit:keep — reachable from an exported signature
type PrintJobSource struct {
	Kind    PrintSource
	PDFPath string
}

// PrintJob configures a print or PDF export operation.
//
// ExportPrintJob always writes a single page: the whole source
// viewport is scaled to fit. Copies and PageRanges apply to
// RunPrintJob (the native dialog) only and are ignored on export.
//
// Build one with NewPrintJob, which sets the paper, orientation,
// margins and copies defaults below.
// exportaudit:keep — reachable from an exported signature
type PrintJob struct {
	Header PrintHeaderFooterCfg
	// Footer is the page footer text config.
	// exportaudit:keep — caller-facing config (issue #372)
	Footer     PrintHeaderFooterCfg
	Source     PrintJobSource
	OutputPath string
	Title      string
	JobName    string
	PageRanges []PrintPageRange
	Copies     int
	// Margins are the page margins in points (1/72 inch).
	// exportaudit:keep — caller-facing print API
	Margins PrintMargins
	// SourceWidth and SourceHeight override the source viewport
	// dimensions in pixels. Zero takes the window size.
	// exportaudit:keep — caller-facing print API
	SourceWidth float32
	// exportaudit:keep — caller-facing print API
	SourceHeight float32
	// exportaudit:keep — caller-facing print API
	Paper       PaperSize
	Orientation PrintOrientation
	ScaleMode   PrintScaleMode
	// exportaudit:keep — caller-facing print API
	Duplex    PrintDuplexMode
	ColorMode PrintColorMode
}

// NewPrintJob returns a PrintJob with sensible defaults: A4 portrait,
// 36-point margins and one copy of the current view.
func NewPrintJob() PrintJob {
	return PrintJob{
		Paper:       PaperA4,
		Orientation: PrintPortrait,
		Margins:     defaultPrintMargins(),
		Copies:      1,
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
// exportaudit:keep — reachable from an exported signature
type PrintExportStatus uint8

// PrintExportStatus constants.
const (
	// exportaudit:keep — caller-facing print API
	PrintExportOK PrintExportStatus = iota
	// exportaudit:keep — caller-facing print API
	PrintExportError
)

// PrintExportResult contains the outcome of ExportPrintJob.
// exportaudit:keep — reachable from an exported signature
type PrintExportResult struct {
	Path         string
	ErrorCode    string
	ErrorMessage string
	Status       PrintExportStatus
}

// IsOk returns true if the export succeeded.
// exportaudit:keep — caller-facing print API
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
func printPageSize(paper PaperSize, orientation PrintOrientation) (float32, float32) {
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
	// NaN fails every ordered comparison below, so it would slip
	// through and reach fpdf as a NaN coordinate.
	if !f32IsFinite(m.Left) || !f32IsFinite(m.Right) ||
		!f32IsFinite(m.Top) || !f32IsFinite(m.Bottom) {
		return errors.New("margins must be finite")
	}
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

// validatePrintEnums rejects an out-of-range enum value. Every one
// of these fields is passed to a backend as a plain int, where an
// unknown value is silently ignored rather than reported — so the
// job is refused here instead of printing with options the caller
// did not ask for.
func validatePrintEnums(job PrintJob) error {
	switch job.Paper {
	case PaperLetter, PaperLegal, PaperA4, PaperA3:
	default:
		return fmt.Errorf("unknown paper size %d", job.Paper)
	}
	switch job.Orientation {
	case PrintPortrait, PrintLandscape:
	default:
		return fmt.Errorf("unknown orientation %d", job.Orientation)
	}
	switch job.ScaleMode {
	case PrintScaleFitToPage, PrintScaleActualSize:
	default:
		return fmt.Errorf("unknown scale mode %d", job.ScaleMode)
	}
	switch job.Duplex {
	case PrintDuplexOff, PrintDuplexLongEdge, PrintDuplexShortEdge:
	default:
		return fmt.Errorf("unknown duplex mode %d", job.Duplex)
	}
	switch job.ColorMode {
	case PrintColorModeColor, PrintColorModeGrayscale:
	default:
		return fmt.Errorf("unknown color mode %d", job.ColorMode)
	}
	switch job.Source.Kind {
	case PrintSourceCurrentView, PrintSourcePDFPath:
	default:
		return fmt.Errorf("unknown print source %d", job.Source.Kind)
	}
	return nil
}

func validatePrintJob(job PrintJob) error {
	// Enums first: printPageSize falls back to A4 for an unknown
	// paper, so validating margins ahead of the paper would report
	// a margin error for a page size the job never asked for.
	if err := validatePrintEnums(job); err != nil {
		return err
	}
	pw, ph := printPageSize(job.Paper, job.Orientation)
	if err := validatePrintMargins(pw, ph, job.Margins); err != nil {
		return err
	}
	if job.Copies < 1 {
		return errors.New("copies must be >= 1")
	}
	if job.Copies > maxPrintCopies {
		return fmt.Errorf("copies must be <= %d", maxPrintCopies)
	}
	if job.Source.Kind == PrintSourcePDFPath {
		if strings.TrimSpace(job.Source.PDFPath) == "" {
			return errors.New("pdf_path is required for pdf_path source")
		}
	}
	if len(job.PageRanges) > maxPrintPageRanges {
		return fmt.Errorf("at most %d page ranges", maxPrintPageRanges)
	}
	for _, r := range job.PageRanges {
		if r.From < 1 || r.To < r.From {
			return fmt.Errorf("invalid page range %d-%d", r.From, r.To)
		}
		if r.To > maxPrintPage {
			return fmt.Errorf("page number must be <= %d", maxPrintPage)
		}
	}
	if err := validateHeaderFooterCfg(job.Header); err != nil {
		return err
	}
	return validateHeaderFooterCfg(job.Footer)
}

func validateExportPrintJob(job PrintJob) error {
	// A non-finite or absurd override becomes the fit-to-page
	// divisor in renderToPDF.
	if !f32IsFinite(job.SourceWidth) || !f32IsFinite(job.SourceHeight) {
		return errors.New("source dimensions must be finite")
	}
	if strings.TrimSpace(job.OutputPath) == "" {
		return errors.New("output_path is required")
	}
	return validatePrintJob(job)
}

// validatePrintPath rejects a PDF path the print backends cannot
// consume safely: empty, containing NUL (truncated by the C string
// boundary on macOS, rejected by exec elsewhere), or starting with
// '-' (parsed as a flag in the Linux lpr/xdg-open argv position).
func validatePrintPath(path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("pdf_path is required")
	}
	if strings.ContainsRune(path, 0) {
		return errors.New("pdf_path must not contain NUL")
	}
	if strings.HasPrefix(path, "-") {
		return errors.New("pdf_path must not start with '-'")
	}
	return nil
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
