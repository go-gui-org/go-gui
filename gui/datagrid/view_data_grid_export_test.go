package datagrid

import (
	"archive/zip"
	"bytes"
	"math"
	"strings"
	"testing"
)

// --- gridDataFromCSV ---

func TestGridDataFromCSVBasic(t *testing.T) {
	csv := "Name,Age\nAlice,30\nBob,25\n"
	data, err := gridDataFromCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.Columns) != 2 {
		t.Fatalf("got %d columns, want 2", len(data.Columns))
	}
	if data.Columns[0].Title != "Name" {
		t.Errorf("col0 title = %q, want Name", data.Columns[0].Title)
	}
	if data.Columns[1].Title != "Age" {
		t.Errorf("col1 title = %q, want Age", data.Columns[1].Title)
	}
	if len(data.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(data.Rows))
	}
	if data.Rows[0].Cells[data.Columns[0].ID] != "Alice" {
		t.Errorf("row0 Name = %q, want Alice",
			data.Rows[0].Cells[data.Columns[0].ID])
	}
	if data.Rows[1].Cells[data.Columns[1].ID] != "25" {
		t.Errorf("row1 Age = %q, want 25",
			data.Rows[1].Cells[data.Columns[1].ID])
	}
}

func TestGridDataFromCSVBlankHeaders(t *testing.T) {
	csv := ",Data,\na,b,c\n"
	data, err := gridDataFromCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Columns[0].Title != "Column 1" {
		t.Errorf("col0 title = %q, want Column 1", data.Columns[0].Title)
	}
	if data.Columns[2].Title != "Column 3" {
		t.Errorf("col2 title = %q, want Column 3", data.Columns[2].Title)
	}
}

func TestGridDataFromCSVDuplicateIDs(t *testing.T) {
	csv := "X,X,X\n1,2,3\n"
	data, err := gridDataFromCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids := map[string]bool{}
	for _, col := range data.Columns {
		if ids[col.ID] {
			t.Fatalf("duplicate column ID %q", col.ID)
		}
		ids[col.ID] = true
	}
	if len(ids) != 3 {
		t.Errorf("got %d unique IDs, want 3", len(ids))
	}
}

func TestGridDataFromCSVEmpty(t *testing.T) {
	_, err := gridDataFromCSV("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
	_, err = gridDataFromCSV("   \n  ")
	if err == nil {
		t.Fatal("expected error for whitespace-only input")
	}
}

func TestGridDataFromCSVBOMStripping(t *testing.T) {
	csv := "\xef\xbb\xbfName,Value\na,1\n"
	data, err := gridDataFromCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data.Columns[0].Title != "Name" {
		t.Errorf("col0 title = %q, want Name (BOM not stripped)",
			data.Columns[0].Title)
	}
}

func TestGridDataFromCSVRaggedRowsExpandColumns(t *testing.T) {
	csv := ",\n1,2,3\n4,5\n"
	data, err := gridDataFromCSV(csv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(data.Columns) != 3 {
		t.Fatalf("got %d columns, want 3", len(data.Columns))
	}
	if len(data.Rows) != 2 {
		t.Fatalf("got %d rows, want 2", len(data.Rows))
	}
	if data.Rows[0].Cells[data.Columns[2].ID] != "3" {
		t.Fatalf("row0 col3 = %q, want 3", data.Rows[0].Cells[data.Columns[2].ID])
	}
	if data.Rows[1].Cells[data.Columns[2].ID] != "" {
		t.Fatalf("row1 col3 = %q, want empty", data.Rows[1].Cells[data.Columns[2].ID])
	}
}

// --- gridRowsToTSV / gridRowsToTSVWithCfg ---

func TestGridRowsToTSV(t *testing.T) {
	cols := []GridColumnCfg{
		{ID: "a", Title: "Alpha"},
		{ID: "b", Title: "Beta"},
	}
	rows := []GridRow{
		{ID: "1", Cells: map[string]string{"a": "x", "b": "y"}},
	}
	got := gridRowsToTSV(cols, rows)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if lines[0] != "Alpha\tBeta" {
		t.Errorf("header = %q, want Alpha\\tBeta", lines[0])
	}
	if lines[1] != "x\ty" {
		t.Errorf("row = %q, want x\\ty", lines[1])
	}
}

func TestGridRowsToTSVTabEscape(t *testing.T) {
	cols := []GridColumnCfg{{ID: "a", Title: "Col"}}
	rows := []GridRow{
		{ID: "1", Cells: map[string]string{"a": "has\ttab"}},
	}
	got := gridRowsToTSV(cols, rows)
	lines := strings.Split(got, "\n")
	if !strings.Contains(lines[1], `"has`) {
		t.Errorf("tab in value should be quoted, got %q", lines[1])
	}
}

func TestGridRowsToTSVEmptyColumns(t *testing.T) {
	got := gridRowsToTSV(nil, nil)
	if got != "" {
		t.Errorf("expected empty for nil columns, got %q", got)
	}
}

func TestGridRowsToTSVDefaultSanitizeFormula(t *testing.T) {
	cols := []GridColumnCfg{{ID: "a", Title: "Col"}}
	rows := []GridRow{{ID: "1", Cells: map[string]string{"a": "=SUM(A1)"}}}
	got := gridRowsToTSV(cols, rows)
	lines := strings.Split(got, "\n")
	if lines[1] != "'=SUM(A1)" {
		t.Fatalf("got %q, want %q", lines[1], "'=SUM(A1)")
	}
}

// --- gridRowsToCSV / gridRowsToCSVWithCfg ---

func TestGridRowsToCSVCommaEscape(t *testing.T) {
	cols := []GridColumnCfg{{ID: "a", Title: "Col"}}
	rows := []GridRow{
		{ID: "1", Cells: map[string]string{"a": "a,b"}},
	}
	got := gridRowsToCSV(cols, rows)
	lines := strings.Split(got, "\n")
	if lines[1] != `"a,b"` {
		t.Errorf("comma value = %q, want \"a,b\"", lines[1])
	}
}

func TestGridRowsToCSVQuotesInValues(t *testing.T) {
	cols := []GridColumnCfg{{ID: "a", Title: "Col"}}
	rows := []GridRow{
		{ID: "1", Cells: map[string]string{"a": `say "hi"`}},
	}
	got := gridRowsToCSV(cols, rows)
	lines := strings.Split(got, "\n")
	if lines[1] != `"say ""hi"""` {
		t.Errorf("quoted value = %q, want %q", lines[1], `"say ""hi"""`)
	}
}

func TestGridRowsToCSVDefaultSanitizeFormula(t *testing.T) {
	cols := []GridColumnCfg{{ID: "a", Title: "Col"}}
	rows := []GridRow{{ID: "1", Cells: map[string]string{"a": "=1+1"}}}
	got := gridRowsToCSV(cols, rows)
	lines := strings.Split(got, "\n")
	if lines[1] != "'=1+1" {
		t.Fatalf("got %q, want %q", lines[1], "'=1+1")
	}
}

// --- gridRowsToPDF ---

func TestGridRowsToPDFNonEmpty(t *testing.T) {
	cols := []GridColumnCfg{{ID: "a", Title: "Name"}}
	rows := []GridRow{
		{ID: "1", Cells: map[string]string{"a": "Alice"}},
	}
	got := gridRowsToPDF(cols, rows)
	if !strings.HasPrefix(got, "%PDF-") {
		t.Errorf("expected PDF header, got prefix %q",
			got[:min(len(got), 20)])
	}
	if !strings.Contains(got, "%"+"%EOF") {
		t.Error("expected EOF marker in PDF output")
	}
}

func TestGridRowsToPDFEmptyColumns(t *testing.T) {
	got := gridRowsToPDF(nil, nil)
	if got != "" {
		t.Errorf("expected empty for nil columns, got len %d", len(got))
	}
}

// --- gridRowsToXLSX / gridRowsToXLSXWithCfg ---

func TestGridRowsToXLSXValidZip(t *testing.T) {
	cols := []GridColumnCfg{
		{ID: "a", Title: "Name"},
		{ID: "b", Title: "Score"},
	}
	rows := []GridRow{
		{ID: "1", Cells: map[string]string{"a": "Alice", "b": "95"}},
	}
	data, err := gridRowsToXLSX(cols, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not valid ZIP: %v", err)
	}
	names := map[string]bool{}
	for _, f := range r.File {
		names[f.Name] = true
	}
	required := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"xl/workbook.xml",
		"xl/_rels/workbook.xml.rels",
		"xl/worksheets/sheet1.xml",
	}
	for _, name := range required {
		if !names[name] {
			t.Errorf("missing ZIP entry %q", name)
		}
	}
}

func TestGridRowsToXLSXAutoType(t *testing.T) {
	cols := []GridColumnCfg{
		{ID: "n", Title: "Num"},
		{ID: "b", Title: "Bool"},
		{ID: "s", Title: "Str"},
	}
	rows := []GridRow{
		{ID: "1", Cells: map[string]string{
			"n": "42", "b": "true", "s": "hello",
		}},
	}
	data, err := gridRowsToXLSXWithCfg(cols, rows, gridExportCfg{
		XLSXAutoType: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Read sheet1.xml from ZIP to verify cell types.
	r, _ := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	var sheetXML string
	for _, f := range r.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, _ := f.Open()
			var buf bytes.Buffer
			buf.ReadFrom(rc)
			rc.Close()
			sheetXML = buf.String()
			break
		}
	}
	// Number cell: no t= attribute, just <v>42</v>.
	if !strings.Contains(sheetXML, "<v>42</v>") {
		t.Error("expected numeric cell <v>42</v>")
	}
	// Bool cell: t="b", <v>1</v>.
	if !strings.Contains(sheetXML, `t="b"><v>1</v>`) {
		t.Error("expected bool cell t=\"b\" with <v>1</v>")
	}
	// String cell: t="inlineStr".
	if !strings.Contains(sheetXML, `t="inlineStr"`) {
		t.Error("expected inline string cell")
	}
}

func TestGridRowsToXLSXDefaultSanitizeFormula(t *testing.T) {
	cols := []GridColumnCfg{{ID: "a", Title: "Col"}}
	rows := []GridRow{{ID: "1", Cells: map[string]string{"a": "=1+1"}}}
	data, err := gridRowsToXLSX(cols, rows)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not valid ZIP: %v", err)
	}
	var sheetXML string
	for _, f := range r.File {
		if f.Name == "xl/worksheets/sheet1.xml" {
			rc, _ := f.Open()
			var buf bytes.Buffer
			buf.ReadFrom(rc)
			rc.Close()
			sheetXML = buf.String()
			break
		}
	}
	if strings.Contains(sheetXML, "<v>=1+1</v>") {
		t.Fatal("formula should be sanitized by default")
	}
	if !strings.Contains(sheetXML, "&apos;=1+1") {
		t.Fatal("expected sanitized apostrophe-prefixed value in sheet XML")
	}
}

// --- dataGridXLSXColRef ---

func TestDataGridXLSXColRef(t *testing.T) {
	tests := []struct {
		idx  int
		want string
	}{
		{0, "A"},
		{25, "Z"},
		{26, "AA"},
		{51, "AZ"},
		{52, "BA"},
		{701, "ZZ"},
		{702, "AAA"},
		{-1, "A"},
	}
	for _, tc := range tests {
		got := dataGridXLSXColRef(tc.idx)
		if got != tc.want {
			t.Errorf("colRef(%d) = %q, want %q", tc.idx, got, tc.want)
		}
	}
}

// --- dataGridSpreadsheetSafeText ---

func TestDataGridSpreadsheetSafeText(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"=SUM(A1)", "'=SUM(A1)"},
		{"+1", "'+1"},
		{"-1", "'-1"},
		{"@import", "'@import"},
		{"normal", "normal"},
		{"", ""},
		{"  =padded", "'  =padded"},
		{"  \t@tab", "'  \t@tab"},
		{"   ", "   "},
	}
	for _, tc := range tests {
		got := dataGridSpreadsheetSafeText(tc.input)
		if got != tc.want {
			t.Errorf("safeText(%q) = %q, want %q",
				tc.input, got, tc.want)
		}
	}
}

// --- dataGridCSVEscape ---

func TestDataGridCSVEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plain", "plain"},
		{"has,comma", `"has,comma"`},
		{`has"quote`, `"has""quote"`},
		{"has\nnewline", `"has` + "\n" + `newline"`},
		{"", ""},
		{"has\ttab", `"has` + "\t" + `tab"`},
	}
	for _, tc := range tests {
		got := dataGridCSVEscape(tc.input)
		if got != tc.want {
			t.Errorf("csvEscape(%q) = %q, want %q",
				tc.input, got, tc.want)
		}
	}
}

// --- dataGridTSVEscape ---

func TestDataGridTSVEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plain", "plain"},
		{"has\ttab", `"has` + "\t" + `tab"`},
		{`has"quote`, `"has""quote"`},
		{"has\nnewline", `"has` + "\n" + `newline"`},
		{"", ""},
		{"no,special", "no,special"}, // commas not special in TSV
	}
	for _, tc := range tests {
		got := dataGridTSVEscape(tc.input)
		if got != tc.want {
			t.Errorf("tsvEscape(%q) = %q, want %q",
				tc.input, got, tc.want)
		}
	}
}

// --- dataGridCSVColumnID ---

func TestDataGridCSVColumnID(t *testing.T) {
	tests := []struct {
		title string
		idx   int
		want  string
	}{
		{"First Name", 0, "first_name"},
		{"123", 0, "123"},
		{"!!!!", 0, "col_1"},
		{"a--b", 0, "a_b"},
		{"UPPER", 0, "upper"},
	}
	for _, tc := range tests {
		got := dataGridCSVColumnID(tc.title, tc.idx)
		if got != tc.want {
			t.Errorf("csvColumnID(%q, %d) = %q, want %q",
				tc.title, tc.idx, got, tc.want)
		}
	}
}

// --- dataGridPDFPad ---

func TestDataGridPDFPad(t *testing.T) {
	tests := []struct {
		value string
		width int
		want  string
	}{
		{"abc", 5, "abc  "},
		{"abcde", 5, "abcde"},
		{"abcdef", 5, "ab..."},
		{"ab", 3, "ab "},
		{"abcde", 3, "..."},
		{"x", 2, "x "},
		{"xy", 1, "."},
	}
	for _, tc := range tests {
		got := dataGridPDFPad(tc.value, tc.width)
		if got != tc.want {
			t.Errorf("pdfPad(%q, %d) = %q, want %q",
				tc.value, tc.width, got, tc.want)
		}
	}
}

// --- pdfNum ---

func TestPdfNum(t *testing.T) {
	tests := []struct {
		value float32
		want  string
	}{
		{10, "10"},
		{10.5, "10.5"},
		{10.500, "10.5"},
		{0.123, "0.123"},
		{float32(math.NaN()), "0"},
		{float32(math.Inf(1)), "0"},
		{float32(math.Inf(-1)), "0"},
		{0, "0"},
	}
	for _, tc := range tests {
		got := pdfNum(tc.value)
		if got != tc.want {
			t.Errorf("pdfNum(%v) = %q, want %q",
				tc.value, got, tc.want)
		}
	}
}
