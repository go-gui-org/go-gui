package gui

import "testing"

func TestNativeIsValidExtension(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{"txt", true},
		{"json", true},
		{"c++", true},
		{"my-ext", true},
		{"my_ext", true},
		{"", false},
		{"TXT", false},  // must be lowercase
		{"a b", false},  // spaces not allowed
		{"a.b", false},  // dots not allowed
		{"a*b", false},  // special chars
		{"txt!", false}, // exclamation
	}
	for _, tt := range tests {
		got := nativeIsValidExtension(tt.ext)
		if got != tt.want {
			t.Errorf("nativeIsValidExtension(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestNativeNormalizeExtension(t *testing.T) {
	tests := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{"txt", "txt", false},
		{".TXT", "txt", false},
		{"..pdf", "pdf", false},
		{"  .JSON ", "json", false},
		{"", "", false},
		{"   ", "", false},
		{"a*b", "", true},
	}
	for _, tt := range tests {
		got, err := nativeNormalizeExtension(tt.raw)
		if (err != nil) != tt.wantErr {
			t.Errorf("nativeNormalizeExtension(%q): err=%v, wantErr=%v", tt.raw, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("nativeNormalizeExtension(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestNativeExtensionsFromFilters(t *testing.T) {
	filters := []NativeFileFilter{
		{Name: "Images", Extensions: []string{".png", ".jpg"}},
		{Name: "Text", Extensions: []string{"txt"}},
	}
	exts, err := nativeExtensionsFromFilters(filters)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exts) != 3 {
		t.Fatalf("got %d extensions, want 3", len(exts))
	}
	if exts[0] != "png" || exts[1] != "jpg" || exts[2] != "txt" {
		t.Errorf("got %v", exts)
	}
}

func TestNativeExtensionsFromFiltersBadExt(t *testing.T) {
	filters := []NativeFileFilter{
		{Name: "Bad", Extensions: []string{"a*b"}},
	}
	_, err := nativeExtensionsFromFilters(filters)
	if err == nil {
		t.Error("expected error for bad extension")
	}
}

func TestNativeExtensionsFromFiltersEmpty(t *testing.T) {
	exts, err := nativeExtensionsFromFilters(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exts) != 0 {
		t.Errorf("expected empty, got %v", exts)
	}
}

func TestNativeSaveExtensions(t *testing.T) {
	filters := []NativeFileFilter{
		{Name: "PDF", Extensions: []string{"pdf"}},
	}
	exts, err := nativeSaveExtensions(filters, ".docx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exts) != 2 {
		t.Fatalf("got %d, want 2", len(exts))
	}
}

func TestNativeSaveExtensionsNoDuplicate(t *testing.T) {
	filters := []NativeFileFilter{
		{Name: "PDF", Extensions: []string{"pdf"}},
	}
	exts, err := nativeSaveExtensions(filters, "pdf")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exts) != 1 {
		t.Errorf("expected no duplicate, got %v", exts)
	}
}

func TestNativeDialogResultPathStrings(t *testing.T) {
	r := NativeDialogResult{
		Paths: []accessiblePath{
			{Path: "/a/b.txt"},
			{Path: "/c/d.pdf", Grant: Grant{ID: 1}},
		},
	}
	ps := r.PathStrings()
	if len(ps) != 2 || ps[0] != "/a/b.txt" || ps[1] != "/c/d.pdf" {
		t.Errorf("got %v", ps)
	}
}

func TestNativeDialogResultPathStringsEmpty(t *testing.T) {
	r := NativeDialogResult{}
	ps := r.PathStrings()
	if len(ps) != 0 {
		t.Errorf("expected empty, got %v", ps)
	}
}

func TestNativeOpenDialogNoPlatform(t *testing.T) {
	w := &Window{}
	var result NativeDialogResult
	cfg := NativeOpenDialogCfg{
		OnDone: func(r NativeDialogResult, _ *Window) { result = r },
	}
	// Call impl directly (bypasses queue).
	nativeOpenDialogImpl(w, cfg)
	if result.Status != DialogError {
		t.Errorf("expected error status, got %d", result.Status)
	}
	if result.ErrorCode != "unsupported" {
		t.Errorf("expected 'unsupported', got %q", result.ErrorCode)
	}
}

func TestNativeSaveDialogNoPlatform(t *testing.T) {
	w := &Window{}
	var result NativeDialogResult
	cfg := NativeSaveDialogCfg{
		OnDone: func(r NativeDialogResult, _ *Window) { result = r },
	}
	nativeSaveDialogImpl(w, cfg)
	if result.Status != DialogError {
		t.Errorf("expected error, got %d", result.Status)
	}
}

func TestNativeMessageDialogNoPlatform(t *testing.T) {
	w := &Window{}
	var result NativeAlertResult
	cfg := NativeMessageDialogCfg{
		Title:  "Test",
		OnDone: func(r NativeAlertResult, _ *Window) { result = r },
	}
	nativeMessageDialogImpl(w, cfg)
	if result.Status != DialogError {
		t.Errorf("expected error, got %d", result.Status)
	}
}

func TestNativeConfirmDialogNoPlatform(t *testing.T) {
	w := &Window{}
	var result NativeAlertResult
	cfg := NativeConfirmDialogCfg{
		Title:  "Test",
		OnDone: func(r NativeAlertResult, _ *Window) { result = r },
	}
	nativeConfirmDialogImpl(w, cfg)
	if result.Status != DialogError {
		t.Errorf("expected error, got %d", result.Status)
	}
}

func TestNativeFolderDialogNoPlatform(t *testing.T) {
	w := &Window{}
	var result NativeDialogResult
	cfg := NativeFolderDialogCfg{
		OnDone: func(r NativeDialogResult, _ *Window) { result = r },
	}
	nativeFolderDialogImpl(w, cfg)
	if result.Status != DialogError {
		t.Errorf("expected error, got %d", result.Status)
	}
}

func TestNativeDialogNilOnDone(_ *testing.T) {
	w := &Window{}
	// Nil OnDone must not panic.
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{})
	nativeSaveDialogImpl(w, NativeSaveDialogCfg{})
	nativeFolderDialogImpl(w, NativeFolderDialogCfg{})
}

func TestNativeAlertNilOnDone(_ *testing.T) {
	w := &Window{}
	// Nil OnDone must not panic.
	nativeMessageDialogImpl(w, NativeMessageDialogCfg{})
	nativeConfirmDialogImpl(w, NativeConfirmDialogCfg{})
	nativeSaveDiscardDialogImpl(w, NativeSaveDiscardDialogCfg{})
}

func TestNativeSaveDiscardDialogNoPlatform(t *testing.T) {
	w := &Window{}
	var result NativeAlertResult
	cfg := NativeSaveDiscardDialogCfg{
		Title:  "Test",
		OnDone: func(r NativeAlertResult, _ *Window) { result = r },
	}
	nativeSaveDiscardDialogImpl(w, cfg)
	if result.Status != DialogError {
		t.Errorf("expected DialogError, got %d", result.Status)
	}
	if result.ErrorCode != "unsupported" {
		t.Errorf("expected 'unsupported', got %q", result.ErrorCode)
	}
}

// saveDiscardPlatform is a minimal NativePlatform stub that returns
// DialogDiscard from ShowSaveDiscardDialog.
type saveDiscardPlatform struct{ NoopNativePlatform }

func (saveDiscardPlatform) ShowSaveDiscardDialog(_, _ string, _ NativeAlertLevel) NativeAlertResult {
	return NativeAlertResult{Status: DialogDiscard}
}

func TestNativeSaveDiscardDialogDiscardStatus(t *testing.T) {
	w := &Window{}
	w.nativePlatform = saveDiscardPlatform{}
	var result NativeAlertResult
	cfg := NativeSaveDiscardDialogCfg{
		OnDone: func(r NativeAlertResult, _ *Window) { result = r },
	}
	nativeSaveDiscardDialogImpl(w, cfg)
	if result.Status != DialogDiscard {
		t.Errorf("expected DialogDiscard (%d), got %d", DialogDiscard, result.Status)
	}
}

func TestNoopShowSaveDiscardDialogReturnsZero(t *testing.T) {
	var p NoopNativePlatform
	r := p.ShowSaveDiscardDialog("", "", AlertInfo)
	if r.Status != DialogOK {
		t.Errorf("expected DialogOK (zero), got %d", r.Status)
	}
}

func TestNativeOpenDialogBadExtension(t *testing.T) {
	w := &Window{}
	var result NativeDialogResult
	cfg := NativeOpenDialogCfg{
		Filters: []NativeFileFilter{{Extensions: []string{"a*b"}}},
		OnDone:  func(r NativeDialogResult, _ *Window) { result = r },
	}
	nativeOpenDialogImpl(w, cfg)
	if result.Status != DialogError || result.ErrorCode != "invalid_cfg" {
		t.Errorf("expected invalid_cfg error, got %+v", result)
	}
}
