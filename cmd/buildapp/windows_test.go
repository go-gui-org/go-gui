package main

import (
	"archive/zip"
	"bytes"
	"debug/pe"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rsrcSection returns the .rsrc section bytes of exe plus the RVA it is
// mapped at, after checking that the resource data directory agrees
// with the section header.
func rsrcSection(t *testing.T, exe []byte) ([]byte, uint32) {
	t.Helper()
	f, err := pe.NewFile(bytes.NewReader(exe))
	if err != nil {
		t.Fatalf("rewritten binary no longer parses as PE: %v", err)
	}
	defer func() { _ = f.Close() }()

	sec := f.Section(".rsrc")
	if sec == nil {
		t.Fatal("no .rsrc section")
	}
	data, err := sec.Data()
	if err != nil {
		t.Fatal(err)
	}

	var dirRVA, dirSize uint32
	switch oh := f.OptionalHeader.(type) {
	case *pe.OptionalHeader64:
		dirRVA = oh.DataDirectory[peResourceDirIx].VirtualAddress
		dirSize = oh.DataDirectory[peResourceDirIx].Size
	case *pe.OptionalHeader32:
		dirRVA = oh.DataDirectory[peResourceDirIx].VirtualAddress
		dirSize = oh.DataDirectory[peResourceDirIx].Size
	default:
		t.Fatalf("unexpected optional header %T", oh)
	}
	if dirRVA != sec.VirtualAddress {
		t.Errorf("resource directory RVA = %#x, section VA = %#x", dirRVA, sec.VirtualAddress)
	}
	if dirSize != sec.VirtualSize {
		t.Errorf("resource directory size = %d, section VirtualSize = %d", dirSize, sec.VirtualSize)
	}
	// Section data is padded up to FileAlignment; trim to the mapped size.
	return data[:sec.VirtualSize], sec.VirtualAddress
}

// lookupResource walks type -> id -> first language and returns the
// leaf bytes, mirroring what the Windows loader does.
func lookupResource(t *testing.T, sec []byte, baseRVA, typ, id uint32) []byte {
	t.Helper()
	idDir, ok := dirEntry(sec, 0, typ)
	if !ok {
		t.Fatalf("resource type %d not found", typ)
	}
	langDir, ok := dirEntry(sec, idDir, id)
	if !ok {
		t.Fatalf("resource %d/%d not found", typ, id)
	}
	// Take whatever the single language entry is.
	if n := binary.LittleEndian.Uint16(sec[langDir+14:]); n != 1 {
		t.Fatalf("language directory has %d entries, want 1", n)
	}
	dataEntry := binary.LittleEndian.Uint32(sec[langDir+resDirSize+4:])
	if dataEntry&resSubdirFlag != 0 {
		t.Fatal("language entry points at a directory, not a leaf")
	}
	rva := binary.LittleEndian.Uint32(sec[dataEntry:])
	size := binary.LittleEndian.Uint32(sec[dataEntry+4:])
	off := rva - baseRVA
	if uint64(off)+uint64(size) > uint64(len(sec)) {
		t.Fatalf("leaf %d/%d runs past the section", typ, id)
	}
	return sec[off : off+size]
}

// dirEntry finds the id-keyed entry named want in the directory at off
// and returns the sub-directory offset it points at.
func dirEntry(sec []byte, off, want uint32) (uint32, bool) {
	named := uint32(binary.LittleEndian.Uint16(sec[off+12:]))
	ids := uint32(binary.LittleEndian.Uint16(sec[off+14:]))
	for i := named; i < named+ids; i++ {
		e := off + resDirSize + i*resEntrySize
		if binary.LittleEndian.Uint32(sec[e:]) != want {
			continue
		}
		sub := binary.LittleEndian.Uint32(sec[e+4:])
		return sub &^ resSubdirFlag, sub&resSubdirFlag != 0
	}
	return 0, false
}

func TestInjectIconBuildsResourceTree(t *testing.T) {
	bin := crossStub(t, "windows", "amd64", "showcase.exe")
	exe, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	pngPath := writeTestPNG(t, 256)
	png, err := os.ReadFile(pngPath)
	if err != nil {
		t.Fatal(err)
	}

	out, err := injectIcon(exe, []iconImage{
		{Width: 0, Height: 0, Planes: 1, BitCount: 32, Data: png},
	})
	if err != nil {
		t.Fatal(err)
	}

	sec, base := rsrcSection(t, out)

	// The RT_ICON leaf must be the PNG, byte for byte.
	if got := lookupResource(t, sec, base, rtIcon, 1); !bytes.Equal(got, png) {
		t.Errorf("RT_ICON payload is %d bytes, want the %d-byte PNG", len(got), len(png))
	}

	// The RT_GROUP_ICON leaf must describe exactly that one image and
	// point at resource id 1.
	grp := lookupResource(t, sec, base, rtGroupIcon, 1)
	if len(grp) != 6+14 {
		t.Fatalf("group icon is %d bytes, want 20", len(grp))
	}
	if n := binary.LittleEndian.Uint16(grp[4:6]); n != 1 {
		t.Errorf("group icon count = %d, want 1", n)
	}
	if sz := binary.LittleEndian.Uint32(grp[14:18]); sz != uint32(len(png)) {
		t.Errorf("group icon dwBytesInRes = %d, want %d", sz, len(png))
	}
	if id := binary.LittleEndian.Uint16(grp[18:20]); id != 1 {
		t.Errorf("group icon nID = %d, want 1", id)
	}
}

// The original sections must survive the rewrite untouched: an injected
// section is only safe if it is purely additive.
func TestInjectIconPreservesOriginalSections(t *testing.T) {
	bin := crossStub(t, "windows", "amd64", "showcase.exe")
	exe, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	before, err := pe.NewFile(bytes.NewReader(exe))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = before.Close() }()

	png := writeTestPNG(t, 32)
	images, err := loadIconImages(png)
	if err != nil {
		t.Fatal(err)
	}
	out, err := injectIcon(exe, images)
	if err != nil {
		t.Fatal(err)
	}
	after, err := pe.NewFile(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = after.Close() }()

	if len(after.Sections) != len(before.Sections)+1 {
		t.Fatalf("section count %d -> %d, want +1", len(before.Sections), len(after.Sections))
	}
	for i, s := range before.Sections {
		a := after.Sections[i]
		if a.Name != s.Name || a.VirtualAddress != s.VirtualAddress ||
			a.Offset != s.Offset || a.Size != s.Size {
			t.Errorf("section %d (%s) moved", i, s.Name)
		}
	}
	// The bytes of the original image are a prefix of the new file.
	if !bytes.Equal(out[:len(exe)], exe) {
		// The header is patched in place, so compare past the headers.
		t.Log("headers differ, as expected; checking section payloads")
		for _, s := range before.Sections {
			want, rerr := s.Data()
			if rerr != nil {
				t.Fatal(rerr)
			}
			got := out[s.Offset : s.Offset+s.Size]
			if !bytes.Equal(got, want) {
				t.Errorf("section %s payload changed", s.Name)
			}
		}
	}
}

func TestInjectIconRejectsExistingResources(t *testing.T) {
	bin := crossStub(t, "windows", "amd64", "showcase.exe")
	exe, err := os.ReadFile(bin)
	if err != nil {
		t.Fatal(err)
	}
	images, err := loadIconImages(writeTestPNG(t, 32))
	if err != nil {
		t.Fatal(err)
	}
	once, err := injectIcon(exe, images)
	if err != nil {
		t.Fatal(err)
	}
	// A second pass must refuse rather than produce two resource
	// directories, only one of which the loader would ever see.
	if _, err = injectIcon(once, images); err == nil ||
		!strings.Contains(err.Error(), "already has a resource directory") {
		t.Fatalf("err = %v, want a refusal", err)
	}
}

func TestBuildWindowsZip(t *testing.T) {
	bin := crossStub(t, "windows", "amd64", "showcase.exe")
	out := t.TempDir()
	err := build(bundleOpts{
		Platform: "windows", Binary: bin, OutDir: out,
		Name: "Go-Gui Showcase", Version: "1.2.3", Icon: writeTestPNG(t, 256),
	})
	if err != nil {
		t.Fatal(err)
	}
	zpath := filepath.Join(out, "go-gui-showcase-1.2.3-windows-amd64.zip")
	zr, err := zip.OpenReader(zpath)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = zr.Close() }()
	if len(zr.File) != 1 || zr.File[0].Name != "showcase.exe" {
		t.Fatalf("zip holds %d entries, want showcase.exe", len(zr.File))
	}
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	body, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if _, base := rsrcSection(t, body); base == 0 {
		t.Error("zipped exe carries no mapped .rsrc section")
	}
}

func TestPngIconImageDimensions(t *testing.T) {
	for _, px := range []int{16, 48, 256} {
		raw, err := os.ReadFile(writeTestPNG(t, px))
		if err != nil {
			t.Fatal(err)
		}
		img, err := pngIconImage(raw)
		if err != nil {
			t.Fatal(err)
		}
		want := byte(px)
		if px >= 256 {
			want = 0 // the format's escape for "256 or larger"
		}
		if img.Width != want || img.Height != want {
			t.Errorf("%dpx PNG -> %dx%d, want %d", px, img.Width, img.Height, want)
		}
	}
}

func TestLoadIconImagesRejectsUnsupported(t *testing.T) {
	p := filepath.Join(t.TempDir(), "icon.icns")
	if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadIconImages(p); err == nil {
		t.Fatal("expected an error for .icns on windows")
	}
}

func TestPeArchRejectsNonPE(t *testing.T) {
	p := filepath.Join(t.TempDir(), "text")
	if err := os.WriteFile(p, []byte("not a pe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := peArch(p); err == nil {
		t.Fatal("expected an error for a text file")
	}
}

func TestBuildRejectsUnknownPlatform(t *testing.T) {
	err := build(bundleOpts{Platform: "plan9", Binary: "x"})
	if err == nil || !strings.Contains(err.Error(), "unsupported -platform") {
		t.Fatalf("err = %v, want an unsupported-platform error", err)
	}
}
