package datagrid

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	dataGridPdfPageWidth  = float32(612)
	dataGridPdfPageHeight = float32(792)
	dataGridPdfMargin     = float32(40)
	dataGridPdfFontSize   = float32(10)
	dataGridPdfLineHeight = float32(12)
	dataGridMaxCSVColumns = 1000
)

var dataGridXLSXReplacer = strings.NewReplacer(
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	`"`, "&quot;",
	"'", "&apos;",
	"\r", "",
	"\n", "&#10;",
	"\t", "&#9;",
)

// gridDataFromCSV parses CSV data into data-grid columns
// and rows. First CSV row becomes column headers.
func gridDataFromCSV(data string) (gridCsvData, error) {
	if strings.TrimSpace(data) == "" {
		return gridCsvData{}, errors.New("csv data is required")
	}
	source := data
	if !strings.HasSuffix(source, "\n") {
		source += "\n"
	}
	reader := csv.NewReader(strings.NewReader(source))
	reader.FieldsPerRecord = -1 // variable field count
	header, err := reader.Read()
	if err == io.EOF {
		return gridCsvData{}, errors.New("csv data contains no rows")
	}
	if err != nil {
		return gridCsvData{}, fmt.Errorf("failed to parse CSV: %w", err)
	}
	maxCols := len(header)
	columns := dataGridCSVColumns(header, maxCols)
	rows := make([]GridRow, 0, 64)
	nextRowID := 1
	for {
		fields, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return gridCsvData{}, fmt.Errorf("failed to parse CSV: %w", readErr)
		}
		if len(fields) > maxCols {
			if len(fields) > dataGridMaxCSVColumns {
				return gridCsvData{}, fmt.Errorf(
					"csv exceeds max column count (%d)", dataGridMaxCSVColumns)
			}
			prevCols := len(columns)
			maxCols = len(fields)
			columns = dataGridCSVColumns(header, maxCols)
			for rowIdx := range rows {
				for colIdx := prevCols; colIdx < len(columns); colIdx++ {
					rows[rowIdx].Cells[columns[colIdx].ID] = ""
				}
			}
		}
		if len(columns) == 0 {
			continue
		}
		cells := make(map[string]string, len(columns))
		for colIdx, col := range columns {
			if colIdx < len(fields) {
				cells[col.ID] = fields[colIdx]
			} else {
				cells[col.ID] = ""
			}
		}
		rows = append(rows, GridRow{
			ID:    strconv.Itoa(nextRowID),
			Cells: cells,
		})
		nextRowID++
	}
	if len(columns) == 0 {
		return gridCsvData{}, errors.New("csv header row is empty")
	}
	return gridCsvData{Columns: columns, Rows: rows}, nil
}

// gridRowsToTSV converts rows to tab-separated text with
// a header row.
func gridRowsToTSV(columns []GridColumnCfg, rows []GridRow) string {
	return gridRowsToTSVWithCfg(columns, rows, gridExportCfg{
		SanitizeSpreadsheetFormulas: true,
	})
}

// gridRowsToTSVWithCfg converts rows to tab-separated text.
func gridRowsToTSVWithCfg(columns []GridColumnCfg, rows []GridRow, exportCfg gridExportCfg) string {
	return dataGridRowsToDelimited(columns, rows, exportCfg, "\t", dataGridTSVEscape)
}

// gridRowsToCSV converts rows to comma-separated text with
// a header row.
func gridRowsToCSV(columns []GridColumnCfg, rows []GridRow) string {
	return gridRowsToCSVWithCfg(columns, rows, gridExportCfg{
		SanitizeSpreadsheetFormulas: true,
	})
}

// gridRowsToCSVWithCfg converts rows to comma-separated text.
func gridRowsToCSVWithCfg(columns []GridColumnCfg, rows []GridRow, exportCfg gridExportCfg) string {
	return dataGridRowsToDelimited(columns, rows, exportCfg, ",", dataGridCSVEscape)
}

// gridRowsToPDF converts rows to a simple PDF table export.
func gridRowsToPDF(columns []GridColumnCfg, rows []GridRow) string {
	if len(columns) == 0 {
		return ""
	}
	lines := dataGridPDFLines(columns, rows)
	return dataGridPDFDocument(lines)
}

// gridRowsToXLSX creates a minimal XLSX workbook and
// returns the file bytes.
func gridRowsToXLSX(columns []GridColumnCfg, rows []GridRow) ([]byte, error) {
	return gridRowsToXLSXWithCfg(columns, rows, gridExportCfg{
		SanitizeSpreadsheetFormulas: true,
	})
}

// gridRowsToXLSXWithCfg creates a minimal XLSX workbook.
func gridRowsToXLSXWithCfg(columns []GridColumnCfg, rows []GridRow, exportCfg gridExportCfg) ([]byte, error) {
	var buf bytes.Buffer
	if err := gridRowsWriteXLSX(&buf, columns, rows, exportCfg); err != nil {
		return nil, fmt.Errorf("xlsx export: %w", err)
	}
	return buf.Bytes(), nil
}

func gridRowsWriteXLSX(w io.Writer, columns []GridColumnCfg, rows []GridRow, exportCfg gridExportCfg) error {
	zw := zip.NewWriter(w)
	defer func() { _ = zw.Close() }()
	entries := [][2]string{
		{"[Content_Types].xml", dataGridXLSXContentTypesXML()},
		{"_rels/.rels", dataGridXLSXRootRelsXML()},
		{"xl/workbook.xml", dataGridXLSXWorkbookXML()},
		{"xl/_rels/workbook.xml.rels", dataGridXLSXWorkbookRelsXML()},
		{"xl/worksheets/sheet1.xml", dataGridXLSXSheetXML(columns, rows, exportCfg)},
	}
	for _, entry := range entries {
		fw, err := zw.Create(entry[0])
		if err != nil {
			return fmt.Errorf("xlsx: create entry %s: %w", entry[0], err)
		}
		if _, err := fw.Write([]byte(entry[1])); err != nil {
			return fmt.Errorf("xlsx: write entry: %w", err)
		}
	}
	return nil
}

// dataGridRowsToDelimited converts columns and rows to
// delimited text using the given separator and escape function.
func dataGridRowsToDelimited(columns []GridColumnCfg, rows []GridRow, exportCfg gridExportCfg, sep string, escape func(string) string) string {
	if len(columns) == 0 {
		return ""
	}
	lines := make([]string, 0, len(rows)+1)
	header := make([]string, len(columns))
	for i, col := range columns {
		header[i] = escape(dataGridExportText(col.Title, exportCfg))
	}
	lines = append(lines, strings.Join(header, sep))
	for _, row := range rows {
		fields := make([]string, len(columns))
		for i, col := range columns {
			fields[i] = escape(dataGridExportText(row.Cells[col.ID], exportCfg))
		}
		lines = append(lines, strings.Join(fields, sep))
	}
	return strings.Join(lines, "\n")
}

// --- PDF helpers ---

func dataGridPDFLines(columns []GridColumnCfg, rows []GridRow) []string {
	if len(columns) == 0 {
		return nil
	}
	widths := dataGridPDFColWidths(columns, rows)
	lines := make([]string, 0, len(rows)+1)
	header := make([]string, len(columns))
	for ci, col := range columns {
		header[ci] = dataGridPDFPad(col.Title, widths[ci])
	}
	lines = append(lines, strings.Join(header, " | "))
	for _, row := range rows {
		parts := make([]string, len(columns))
		for ci, col := range columns {
			parts[ci] = dataGridPDFPad(row.Cells[col.ID], widths[ci])
		}
		lines = append(lines, strings.Join(parts, " | "))
	}
	return lines
}

func dataGridPDFColWidths(columns []GridColumnCfg, rows []GridRow) []int {
	ncols := len(columns)
	charsPerPoint := float32(1.0) / float32(6.0)
	totalChars := int((dataGridPdfPageWidth - dataGridPdfMargin*2) * charsPerPoint)
	sepChars := 0
	if ncols > 1 {
		sepChars = (ncols - 1) * 3
	}
	budget := max(totalChars-sepChars, ncols)
	sampleLimit := min(len(rows), 100)
	natural := make([]int, ncols)
	for ci, col := range columns {
		w := utf8.RuneCountInString(col.Title)
		if w > natural[ci] {
			natural[ci] = w
		}
	}
	for ri := range sampleLimit {
		for ci, col := range columns {
			w := utf8.RuneCountInString(rows[ri].Cells[col.ID])
			if w > natural[ci] {
				natural[ci] = w
			}
		}
	}
	totalNatural := 0
	for _, w := range natural {
		totalNatural += w
	}
	widths := make([]int, ncols)
	if totalNatural <= budget {
		copy(widths, natural)
	} else {
		assigned := 0
		for ci := range ncols {
			w := int(float64(natural[ci]) * float64(budget) / float64(totalNatural))
			w = max(w, 3)
			widths[ci] = w
			assigned += w
		}
		remainder := budget - assigned
		for remainder > 0 {
			best := -1
			bestNat := 0
			for ci := range ncols {
				if natural[ci] > bestNat && widths[ci] < natural[ci] {
					best = ci
					bestNat = natural[ci]
				}
			}
			if best < 0 {
				break
			}
			widths[best]++
			remainder--
		}
	}
	return widths
}

func dataGridPDFPad(value string, width int) string {
	runes := []rune(value)
	if len(runes) == width {
		return value
	}
	if len(runes) > width {
		if width <= 3 {
			return "..."[:width]
		}
		return string(runes[:width-3]) + "..."
	}
	var sb strings.Builder
	sb.Grow(width)
	sb.WriteString(value)
	for range width - len(runes) {
		sb.WriteByte(' ')
	}
	return sb.String()
}

func dataGridPDFDocument(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	usableHeight := dataGridPdfPageHeight - dataGridPdfMargin*2
	maxLines := int(usableHeight / dataGridPdfLineHeight)
	maxLines = max(maxLines, 1)
	var pages [][]string
	for i := 0; i < len(lines); i += maxLines {
		end := min(i+maxLines, len(lines))
		pages = append(pages, lines[i:end])
	}
	objects := make([]string, 0, 2+len(pages)*2)
	objects = append(objects, "<< /Type /Catalog /Pages 2 0 R >>")

	kids := make([]string, len(pages))
	for i := range pages {
		pageObjIdx := 3 + i*2
		kids[i] = fmt.Sprintf("%d 0 R", pageObjIdx)
	}
	objects = append(objects, fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(kids, " "), len(pages)))

	for i, pageLines := range pages {
		var stream strings.Builder
		stream.WriteString("BT\n")
		fmt.Fprintf(&stream, "/F1 %s Tf\n", pdfNum(dataGridPdfFontSize))
		fmt.Fprintf(&stream, "%s TL\n", pdfNum(dataGridPdfLineHeight))
		fmt.Fprintf(&stream, "%s %s Td\n",
			pdfNum(dataGridPdfMargin), pdfNum(dataGridPdfPageHeight-dataGridPdfMargin))
		for j, line := range pageLines {
			if j > 0 {
				stream.WriteString("T*\n")
			}
			fmt.Fprintf(&stream, "(%s) Tj\n", pdfEscapeText(line))
		}
		stream.WriteString("ET\n")
		content := stream.String()

		contentObjIdx := 4 + i*2
		pageObj := fmt.Sprintf(
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 %s %s] /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Courier >> >> >> /Contents %d 0 R >>",
			pdfNum(dataGridPdfPageWidth), pdfNum(dataGridPdfPageHeight), contentObjIdx)
		contentObj := fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(content), content)

		objects = append(objects, pageObj)
		objects = append(objects, contentObj)
	}

	return pdfEncode(objects)
}

func pdfNum(value float32) string {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		return "0"
	}
	s := fmt.Sprintf("%.3f", value)
	for strings.HasSuffix(s, "0") && strings.Contains(s, ".") {
		s = s[:len(s)-1]
	}
	s = strings.TrimSuffix(s, ".")
	return s
}

func pdfEscapeText(text string) string {
	var out strings.Builder
	out.Grow(len(text) + 8)
	for _, ch := range text {
		switch ch {
		case '\\':
			out.WriteString("\\\\")
		case '(':
			out.WriteString("\\(")
		case ')':
			out.WriteString("\\)")
		default:
			out.WriteRune(ch)
		}
	}
	return out.String()
}

func pdfEncode(objects []string) string {
	var out strings.Builder
	out.Grow(2048)
	out.WriteString("%PDF-1.4\n")

	offsets := make([]int, len(objects)+1)
	for i, body := range objects {
		offsets[i+1] = out.Len()
		fmt.Fprintf(&out, "%d 0 obj\n", i+1)
		out.WriteString(body)
		if !strings.HasSuffix(body, "\n") {
			out.WriteByte('\n')
		}
		out.WriteString("endobj\n")
	}

	xrefStart := out.Len()
	fmt.Fprintf(&out, "xref\n0 %d\n", len(objects)+1)
	out.WriteString("0000000000 65535 f \n")
	for i := 1; i <= len(objects); i++ {
		fmt.Fprintf(&out, "%010d 00000 n \n", offsets[i])
	}
	out.WriteString("trailer\n")
	fmt.Fprintf(&out, "<< /Size %d /Root 1 0 R >>\n", len(objects)+1)
	out.WriteString("startxref\n")
	fmt.Fprintf(&out, "%d\n", xrefStart)
	out.WriteString("%%EOF\n")

	return out.String()
}

// --- XLSX helpers ---

func dataGridXLSXContentTypesXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
		`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
		`</Types>`
}

func dataGridXLSXRootRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
		`</Relationships>`
}

func dataGridXLSXWorkbookXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`
}

func dataGridXLSXWorkbookRelsXML() string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
		`</Relationships>`
}

func dataGridXLSXSheetXML(columns []GridColumnCfg, rows []GridRow, exportCfg gridExportCfg) string {
	cellsPerRow := len(columns)
	cellsPerRow = max(cellsPerRow, 1)
	var out strings.Builder
	out.Grow(1024 + (len(rows)+1)*cellsPerRow*56)
	out.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	out.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	if len(columns) > 0 {
		out.WriteString(`<row r="1">`)
		for colIdx, col := range columns {
			cellRef := dataGridXLSXCellRef(colIdx, 1)
			out.WriteString(dataGridXLSXStringCellXML(cellRef, dataGridExportText(col.Title, exportCfg)))
		}
		out.WriteString(`</row>`)
	}
	for rowIdx, row := range rows {
		xmlRow := rowIdx + 2
		fmt.Fprintf(&out, `<row r="%d">`, xmlRow)
		for colIdx, col := range columns {
			cellRef := dataGridXLSXCellRef(colIdx, xmlRow)
			value := dataGridExportText(row.Cells[col.ID], exportCfg)
			out.WriteString(dataGridXLSXCellXML(cellRef, value, exportCfg))
		}
		out.WriteString(`</row>`)
	}
	out.WriteString(`</sheetData></worksheet>`)
	return out.String()
}

func dataGridXLSXCellXML(cellRef, value string, exportCfg gridExportCfg) string {
	if !exportCfg.XLSXAutoType {
		return dataGridXLSXStringCellXML(cellRef, value)
	}
	trimmed := strings.TrimSpace(value)
	if dataGridXLSXIsBool(trimmed) {
		return fmt.Sprintf(`<c r="%s" t="b"><v>%s</v></c>`, cellRef, dataGridXLSXBoolValue(trimmed))
	}
	if dataGridXLSXIsNumber(trimmed) && dataGridXLSXSafeNumber(trimmed) {
		return fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, cellRef, trimmed)
	}
	return dataGridXLSXStringCellXML(cellRef, value)
}

func dataGridXLSXStringCellXML(cellRef, value string) string {
	escaped := dataGridXLSXEscape(value)
	if dataGridXLSXPreserveSpaces(value) {
		return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, cellRef, escaped)
	}
	return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, cellRef, escaped)
}

func dataGridXLSXEscape(value string) string {
	return dataGridXLSXReplacer.Replace(value)
}

func dataGridXLSXPreserveSpaces(value string) bool {
	return len(value) > 0 && (value[0] == ' ' || value[len(value)-1] == ' ')
}

func dataGridXLSXIsBool(value string) bool {
	if value == "" {
		return false
	}
	switch strings.ToLower(value) {
	case "true", "false", "yes", "no", "on", "off":
		return true
	}
	return false
}

func dataGridXLSXBoolValue(value string) string {
	switch strings.ToLower(value) {
	case "true", "yes", "on":
		return "1"
	}
	return "0"
}

func dataGridXLSXIsNumber(value string) bool {
	if value == "" {
		return false
	}
	n, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return false
	}
	return !math.IsNaN(n) && !math.IsInf(n, 0)
}

func dataGridXLSXSafeNumber(value string) bool {
	for i := range len(value) {
		c := value[i]
		switch {
		case c >= '0' && c <= '9':
		case c == '.' || c == '+' || c == '-' || c == 'e' || c == 'E':
		default:
			return false
		}
	}
	return true
}

func dataGridXLSXCellRef(colIdx, rowIdx int) string {
	return dataGridXLSXColRef(colIdx) + strconv.Itoa(rowIdx)
}

func dataGridXLSXColRef(colIdx int) string {
	if colIdx < 0 {
		return "A"
	}
	n := colIdx + 1
	var label []byte
	for n > 0 {
		rem := (n - 1) % 26
		label = append([]byte{byte('A' + rem)}, label...)
		n = (n - 1) / 26
	}
	return string(label)
}

// --- Export text helpers ---

func dataGridExportText(value string, exportCfg gridExportCfg) string {
	if !exportCfg.SanitizeSpreadsheetFormulas {
		return value
	}
	return dataGridSpreadsheetSafeText(value)
}

func dataGridSpreadsheetSafeText(value string) string {
	if value == "" {
		return value
	}
	first := 0
	for first < len(value) && (value[first] == ' ' || value[first] == '\t') {
		first++
	}
	if first >= len(value) {
		return value
	}
	switch value[first] {
	case '=', '+', '-', '@':
		return "'" + value
	}
	return value
}

func dataGridTSVEscape(value string) string {
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, "\t\"\n\r") {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return value
}

func dataGridCSVEscape(value string) string {
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, ",\"\n\r\t") {
		return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
	}
	return value
}

// --- CSV import helpers ---

func dataGridCSVColumns(header []string, maxCols int) []GridColumnCfg {
	columns := make([]GridColumnCfg, 0, maxCols)
	usedIDs := map[string]bool{}
	for idx := range maxCols {
		headerValue := ""
		if idx < len(header) {
			headerValue = dataGridCSVStripBOM(header[idx], idx)
		}
		title := dataGridCSVColumnTitle(headerValue, idx)
		var baseID string
		if strings.TrimSpace(headerValue) == "" {
			baseID = fmt.Sprintf("col_%d", idx+1)
		} else {
			baseID = dataGridCSVColumnID(title, idx)
		}
		colID := dataGridCSVUniqueID(baseID, usedIDs)
		columns = append(columns, GridColumnCfg{ID: colID, Title: title})
	}
	return columns
}

func dataGridCSVColumnTitle(value string, idx int) string {
	title := strings.TrimSpace(value)
	if title != "" {
		return title
	}
	return fmt.Sprintf("Column %d", idx+1)
}

func dataGridCSVColumnID(title string, idx int) string {
	lower := strings.ToLower(title)
	var out []byte
	lastIsUnderscore := false
	for i := range len(lower) {
		ch := lower[i]
		isAlpha := ch >= 'a' && ch <= 'z'
		isDigit := ch >= '0' && ch <= '9'
		if isAlpha || isDigit {
			out = append(out, ch)
			lastIsUnderscore = false
			continue
		}
		if !lastIsUnderscore {
			out = append(out, '_')
			lastIsUnderscore = true
		}
	}
	id := dataGridTrimCharEdges(string(out), '_')
	if id == "" {
		id = fmt.Sprintf("col_%d", idx+1)
	}
	return id
}

func dataGridTrimCharEdges(value string, ch byte) string {
	if value == "" {
		return ""
	}
	start := 0
	end := len(value)
	for start < end && value[start] == ch {
		start++
	}
	for end > start && value[end-1] == ch {
		end--
	}
	return value[start:end]
}

func dataGridCSVUniqueID(base string, used map[string]bool) string {
	if !used[base] {
		used[base] = true
		return base
	}
	suffix := 2
	for {
		candidate := fmt.Sprintf("%s_%d", base, suffix)
		if !used[candidate] {
			used[candidate] = true
			return candidate
		}
		suffix++
	}
}

func dataGridCSVStripBOM(value string, idx int) string {
	if idx != 0 || len(value) < 3 {
		return value
	}
	if value[0] == 0xef && value[1] == 0xbb && value[2] == 0xbf {
		return value[3:]
	}
	return value
}
