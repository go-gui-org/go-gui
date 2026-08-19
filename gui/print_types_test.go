package gui

import "testing"

func TestPrintPageSize(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name        string
		paper       PaperSize
		orientation PrintOrientation
		wantW       float32
		wantH       float32
	}{
		{"A4_portrait", paperA4, printPortrait, 595, 842},
		{"A4_landscape", paperA4, printLandscape, 842, 595},
		{"letter", paperLetter, printPortrait, 612, 792},
		{"legal", paperLegal, printPortrait, 612, 1008},
		{"A3", paperA3, printPortrait, 842, 1191},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w, h := printPageSize(tt.paper, tt.orientation)
			if w != tt.wantW || h != tt.wantH {
				t.Errorf("got %gx%g, want %gx%g",
					w, h, tt.wantW, tt.wantH)
			}
		})
	}
}

func TestDefaultPrintMargins(t *testing.T) {
	t.Parallel()
	m := defaultPrintMargins()
	if m.Top != 36 || m.Right != 36 || m.Bottom != 36 || m.Left != 36 {
		t.Errorf("defaults: %+v", m)
	}
}

func TestValidatePrintMargins(t *testing.T) {
	t.Parallel()
	t.Run("negative", func(t *testing.T) {
		t.Parallel()
		if err := validatePrintMargins(595, 842,
			PrintMargins{Left: -1}); err == nil {
			t.Error("expected error for negative margin")
		}
	})
	t.Run("exceed_width", func(t *testing.T) {
		t.Parallel()
		if err := validatePrintMargins(100, 842,
			PrintMargins{Left: 60, Right: 60}); err == nil {
			t.Error("expected error for margins exceeding width")
		}
	})
	t.Run("exceed_height", func(t *testing.T) {
		t.Parallel()
		if err := validatePrintMargins(595, 100,
			PrintMargins{Top: 60, Bottom: 60}); err == nil {
			t.Error("expected error for margins exceeding height")
		}
	})
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		if err := validatePrintMargins(595, 842,
			defaultPrintMargins()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidatePrintJob(t *testing.T) {
	t.Parallel()
	t.Run("defaults", func(t *testing.T) {
		t.Parallel()
		if err := validatePrintJob(NewPrintJob()); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("zero_copies", func(t *testing.T) {
		t.Parallel()
		job := NewPrintJob()
		job.Copies = 0
		if err := validatePrintJob(job); err == nil {
			t.Error("expected error for zero copies")
		}
	})
	t.Run("bad_page_range", func(t *testing.T) {
		t.Parallel()
		job := NewPrintJob()
		job.PageRanges = []PrintPageRange{{From: 0, To: 5}}
		if err := validatePrintJob(job); err == nil {
			t.Error("expected error for page range starting at 0")
		}
	})
	t.Run("reversed_range", func(t *testing.T) {
		t.Parallel()
		job := NewPrintJob()
		job.PageRanges = []PrintPageRange{{From: 5, To: 3}}
		if err := validatePrintJob(job); err == nil {
			t.Error("expected error for reversed page range")
		}
	})
	t.Run("bad_DPI", func(t *testing.T) {
		t.Parallel()
		job := NewPrintJob()
		job.rasterDPI = 50
		if err := validatePrintJob(job); err == nil {
			t.Error("expected error for low DPI")
		}
	})
	t.Run("bad_quality", func(t *testing.T) {
		t.Parallel()
		job := NewPrintJob()
		job.jPEGQuality = 5
		if err := validatePrintJob(job); err == nil {
			t.Error("expected error for low quality")
		}
	})
	t.Run("pdf_path_required", func(t *testing.T) {
		t.Parallel()
		job := NewPrintJob()
		job.Source = printJobSource{Kind: printSourcePDFPath}
		if err := validatePrintJob(job); err == nil {
			t.Error("expected error for empty pdf_path")
		}
	})
}

func TestValidateExportPrintJob(t *testing.T) {
	t.Parallel()
	t.Run("no_output_path", func(t *testing.T) {
		t.Parallel()
		if err := validateExportPrintJob(NewPrintJob()); err == nil {
			t.Error("expected error for empty output_path")
		}
	})
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		job := NewPrintJob()
		job.OutputPath = "/tmp/out.pdf"
		if err := validateExportPrintJob(job); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestNormalizePrintPageRanges(t *testing.T) {
	t.Parallel()
	t.Run("empty", func(t *testing.T) {
		t.Parallel()
		if result := normalizePrintPageRanges(nil); result != nil {
			t.Errorf("expected nil, got %v", result)
		}
	})
	t.Run("single", func(t *testing.T) {
		t.Parallel()
		result := normalizePrintPageRanges(
			[]PrintPageRange{{From: 1, To: 5}})
		if len(result) != 1 ||
			result[0].From != 1 || result[0].To != 5 {
			t.Errorf("got %v", result)
		}
	})
	t.Run("merge_adjacent", func(t *testing.T) {
		t.Parallel()
		result := normalizePrintPageRanges(
			[]PrintPageRange{{From: 1, To: 3}, {From: 4, To: 6}})
		if len(result) != 1 ||
			result[0].From != 1 || result[0].To != 6 {
			t.Errorf("expected merged [1-6], got %v", result)
		}
	})
	t.Run("merge_overlap", func(t *testing.T) {
		t.Parallel()
		result := normalizePrintPageRanges(
			[]PrintPageRange{{From: 1, To: 5}, {From: 3, To: 8}})
		if len(result) != 1 ||
			result[0].From != 1 || result[0].To != 8 {
			t.Errorf("expected merged [1-8], got %v", result)
		}
	})
	t.Run("disjoint", func(t *testing.T) {
		t.Parallel()
		result := normalizePrintPageRanges(
			[]PrintPageRange{{From: 1, To: 3}, {From: 10, To: 12}})
		if len(result) != 2 {
			t.Errorf("expected 2 ranges, got %v", result)
		}
	})
	t.Run("sort", func(t *testing.T) {
		t.Parallel()
		result := normalizePrintPageRanges(
			[]PrintPageRange{{From: 10, To: 12}, {From: 1, To: 3}})
		if len(result) != 2 ||
			result[0].From != 1 || result[1].From != 10 {
			t.Errorf("expected sorted, got %v", result)
		}
	})
}

func TestExtractPrintTokens(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"two_tokens", "Page {page} of {pages}", 2},
		{"date_token", "{date}", 1},
		{"no_tokens", "no tokens", 0},
		{"adjacent", "{page}{title}", 2},
		{"empty_braces", "{}", 0},
		{"nested", "{{nested}}", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tokens := extractPrintTokens(tt.input)
			if len(tokens) != tt.want {
				t.Errorf("extractPrintTokens(%q): got %d tokens %v, want %d",
					tt.input, len(tokens), tokens, tt.want)
			}
		})
	}
}

func TestValidatePrintToken(t *testing.T) {
	t.Parallel()
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		for _, tok := range []string{
			"page", "pages", "date", "title", "job",
		} {
			if err := validatePrintToken(tok); err != nil {
				t.Errorf("validatePrintToken(%q) unexpected error: %v",
					tok, err)
			}
		}
	})
	t.Run("bad", func(t *testing.T) {
		t.Parallel()
		if err := validatePrintToken("foo"); err == nil {
			t.Error("expected error for unsupported token")
		}
	})
}

func TestValidateHeaderFooter(t *testing.T) {
	t.Parallel()
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		cfg := PrintHeaderFooterCfg{
			Enabled: true, Left: "Page {page}",
		}
		if err := validateHeaderFooterCfg(cfg); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("bad_token", func(t *testing.T) {
		t.Parallel()
		cfg := PrintHeaderFooterCfg{
			Enabled: true, Center: "{invalid}",
		}
		if err := validateHeaderFooterCfg(cfg); err == nil {
			t.Error("expected error for bad token")
		}
	})
	t.Run("disabled_skips_validation", func(t *testing.T) {
		t.Parallel()
		cfg := PrintHeaderFooterCfg{
			Enabled: false, Center: "{invalid}",
		}
		if err := validateHeaderFooterCfg(cfg); err != nil {
			t.Errorf("disabled header should skip validation: %v", err)
		}
	})
}

func TestPrintPageRangesToString(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input []PrintPageRange
		want  string
	}{
		{"nil", nil, ""},
		{"single_page", []PrintPageRange{{1, 1}}, "1"},
		{"range", []PrintPageRange{{1, 5}}, "1-5"},
		{"multiple", []PrintPageRange{{1, 3}, {5, 7}}, "1-3,5-7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := printPageRangesToString(tt.input)
			if got != tt.want {
				t.Errorf("printPageRangesToString(%v) = %q, want %q",
					tt.input, got, tt.want)
			}
		})
	}
}

func TestPrintExportResultIsOk(t *testing.T) {
	t.Parallel()
	ok := printExportOKResult("/tmp/out.pdf")
	if !ok.isOk() {
		t.Error("expected IsOk true")
	}
	fail := printExportErrorResult("/tmp/out.pdf", "err", "msg")
	if fail.isOk() {
		t.Error("expected IsOk false")
	}
}

func TestNewPrintJob(t *testing.T) {
	t.Parallel()
	job := NewPrintJob()
	if job.paper != paperA4 {
		t.Errorf("Paper: got %d, want %d", job.paper, paperA4)
	}
	if job.Copies != 1 {
		t.Errorf("Copies: got %d, want 1", job.Copies)
	}
	if job.rasterDPI != 300 {
		t.Errorf("RasterDPI: got %d, want 300", job.rasterDPI)
	}
	if job.jPEGQuality != 85 {
		t.Errorf("JPEGQuality: got %d, want 85", job.jPEGQuality)
	}
}
