// Windows packaging: a .zip holding the .exe with an embedded icon.
//
// Two things make a Go binary look like an application on Windows: the
// GUI subsystem flag (`go build -ldflags "-H windowsgui"`, which stops
// the loader opening a console window) and an icon resource in the PE
// image (which is what Explorer, the taskbar and Alt-Tab draw).  The
// first belongs to the compile step.  This file does the second.
//
// The icon is injected into an already-linked PE by appending a .rsrc
// section, rather than by emitting a .syso for the Go linker to consume.
// A .syso would have to exist before `go build` runs, which would make
// packaging a two-phase step and would put a generated file in the
// user's package directory.  Appending keeps buildapp's shape: one
// compiled binary in, one release artefact out.
package main

import (
	"archive/zip"
	"bytes"
	"debug/pe"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Windows resource type ids (winuser.h).
const (
	rtIcon      = 3
	rtGroupIcon = 14
)

// resLangID is the language the resources are filed under.  Windows
// falls back to the first available language when the requested one is
// missing, so a single neutral entry is enough; 1033 (en-US) is what
// every resource compiler emits by default.
const resLangID = 1033

// Sizes of the three resource-directory structures, in bytes.
const (
	resDirSize      = 16 // IMAGE_RESOURCE_DIRECTORY
	resEntrySize    = 8  // IMAGE_RESOURCE_DIRECTORY_ENTRY
	resDataEntrySz  = 16 // IMAGE_RESOURCE_DATA_ENTRY
	resSubdirFlag   = 0x80000000
	peResourceDirIx = 2 // IMAGE_DIRECTORY_ENTRY_RESOURCE
)

// buildWindows writes <outdir>/<slug>-<version>-windows-<arch>.zip.
func buildWindows(o bundleOpts) error {
	exe, err := os.ReadFile(o.Binary) // #nosec G304 — CLI flag
	if err != nil {
		return err
	}
	arch, err := peArch(o.Binary)
	if err != nil {
		return err
	}

	if o.Icon != "" {
		images, ierr := loadIconImages(o.Icon)
		if ierr != nil {
			return ierr
		}
		exe, ierr = injectIcon(exe, images)
		if ierr != nil {
			return fmt.Errorf("embed icon: %w", ierr)
		}
	}

	if err = os.MkdirAll(o.OutDir, 0o755); err != nil { // #nosec G301
		return err
	}
	base := fmt.Sprintf("%s-%s-windows-%s", slug(o.Name), o.Version, arch)
	dst := filepath.Join(o.OutDir, base+".zip")
	if err = writeZip(dst, filepath.Base(o.Binary), exe); err != nil {
		return err
	}
	fmt.Println(dst)
	return nil
}

// peArch validates that path is a PE executable and maps its machine
// field onto the Go arch name used in the archive filename.
func peArch(path string) (string, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if fi.IsDir() {
		return "", fmt.Errorf("%s is a directory", path)
	}
	f, err := pe.Open(path)
	if err != nil {
		return "", fmt.Errorf("%s is not a PE executable: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	switch f.Machine {
	case pe.IMAGE_FILE_MACHINE_AMD64:
		return "amd64", nil
	case pe.IMAGE_FILE_MACHINE_I386:
		return "386", nil
	case pe.IMAGE_FILE_MACHINE_ARM64:
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported PE machine 0x%x", f.Machine)
	}
}

// iconImage is one image inside an icon group: the encoded bytes plus
// the descriptive fields the RT_GROUP_ICON directory repeats.
type iconImage struct {
	Width    byte // 0 means 256
	Height   byte // 0 means 256
	Colors   byte
	Planes   uint16
	BitCount uint16
	Data     []byte
}

// loadIconImages reads a .ico (all of its images) or a .png (one
// image).  A PNG needs no conversion: since Vista an icon directory
// entry may hold a PNG stream verbatim, which is how every 256x256
// icon is stored in practice.
func loadIconImages(path string) ([]iconImage, error) {
	raw, err := os.ReadFile(path) // #nosec G304 — CLI flag
	if err != nil {
		return nil, err
	}
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".ico":
		return parseICO(raw)
	case ".png":
		img, perr := pngIconImage(raw)
		if perr != nil {
			return nil, perr
		}
		return []iconImage{img}, nil
	default:
		return nil, fmt.Errorf("unsupported icon type %q (need .png or .ico)", ext)
	}
}

// pngSignature is the fixed 8-byte header every PNG file starts with.
var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// pngIconImage wraps a PNG as a single icon image, reading its pixel
// dimensions out of the IHDR chunk (the first chunk, always at a fixed
// offset).  Dimensions of 256 or more are stored as 0, which is the
// icon format's escape for "256 or larger".
func pngIconImage(raw []byte) (iconImage, error) {
	const ihdrEnd = 24 // 8 signature + 4 length + 4 type + 8 dimensions
	if len(raw) < ihdrEnd || !bytes.Equal(raw[:8], pngSignature) {
		return iconImage{}, errors.New("not a PNG file")
	}
	w := binary.BigEndian.Uint32(raw[16:20])
	h := binary.BigEndian.Uint32(raw[20:24])
	return iconImage{
		Width: iconDim(w), Height: iconDim(h),
		Planes: 1, BitCount: 32, Data: raw,
	}, nil
}

// iconDim encodes a pixel dimension into the single byte the icon
// directory has for it.  256 and above collapse to 0.
func iconDim(px uint32) byte {
	if px >= 256 {
		return 0
	}
	return byte(px)
}

// parseICO reads an ICONDIR plus its ICONDIRENTRY table.
func parseICO(raw []byte) ([]iconImage, error) {
	const dirSize, entrySize = 6, 16
	if len(raw) < dirSize {
		return nil, errors.New("truncated .ico header")
	}
	if binary.LittleEndian.Uint16(raw[2:4]) != 1 {
		return nil, errors.New("not an icon file (idType != 1)")
	}
	n := int(binary.LittleEndian.Uint16(raw[4:6]))
	if n == 0 {
		return nil, errors.New(".ico contains no images")
	}
	if len(raw) < dirSize+n*entrySize {
		return nil, errors.New("truncated .ico directory")
	}
	images := make([]iconImage, 0, n)
	for i := range n {
		e := raw[dirSize+i*entrySize:]
		size := binary.LittleEndian.Uint32(e[8:12])
		off := binary.LittleEndian.Uint32(e[12:16])
		end := uint64(off) + uint64(size)
		if end > uint64(len(raw)) {
			return nil, fmt.Errorf(".ico image %d runs past end of file", i)
		}
		images = append(images, iconImage{
			Width: e[0], Height: e[1], Colors: e[2],
			Planes:   binary.LittleEndian.Uint16(e[4:6]),
			BitCount: binary.LittleEndian.Uint16(e[6:8]),
			Data:     raw[off:end],
		})
	}
	return images, nil
}

// groupIconData builds the RT_GROUP_ICON payload: a GRPICONDIR
// followed by one GRPICONDIRENTRY per image.  The entry is the .ico
// directory entry with the 4-byte file offset replaced by the 2-byte
// resource id of the matching RT_ICON.
func groupIconData(images []iconImage) []byte {
	buf := make([]byte, 0, 6+14*len(images))
	buf = binary.LittleEndian.AppendUint16(buf, 0)                   // reserved
	buf = binary.LittleEndian.AppendUint16(buf, 1)                   // type: icon
	buf = binary.LittleEndian.AppendUint16(buf, uint16(len(images))) // count
	for i, im := range images {
		buf = append(buf, im.Width, im.Height, im.Colors, 0)
		buf = binary.LittleEndian.AppendUint16(buf, im.Planes)
		buf = binary.LittleEndian.AppendUint16(buf, im.BitCount)
		buf = binary.LittleEndian.AppendUint32(buf, uint32(len(im.Data)))
		buf = binary.LittleEndian.AppendUint16(buf, uint16(i+1)) // RT_ICON id
	}
	return buf
}

// resLeaf is one resource to place in the directory tree.
type resLeaf struct {
	Type uint32 // RT_ICON or RT_GROUP_ICON
	ID   uint32 // resource id within the type
	Data []byte
}

// buildResourceSection lays out a .rsrc section for leaves, which must
// be sorted by (Type, ID) — the loader binary-searches the directories.
//
// baseRVA is where the section will be mapped.  It is needed up front
// because IMAGE_RESOURCE_DATA_ENTRY records an RVA, not a
// section-relative offset; every other pointer in the tree *is*
// section-relative.
func buildResourceSection(leaves []resLeaf, baseRVA uint32) []byte {
	// Group leaves by type, preserving the sorted order.
	type group struct {
		Type   uint32
		Leaves []resLeaf
	}
	var groups []group
	for _, l := range leaves {
		if n := len(groups); n > 0 && groups[n-1].Type == l.Type {
			groups[n-1].Leaves = append(groups[n-1].Leaves, l)
			continue
		}
		groups = append(groups, group{Type: l.Type, Leaves: []resLeaf{l}})
	}

	// Pass one: fix every offset.  The tree is three levels deep
	// (type -> id -> language), then the data-entry block, then the
	// raw bytes.
	off := uint32(resDirSize + len(groups)*resEntrySize) // root dir
	idDirOff := make([]uint32, len(groups))
	for i, g := range groups {
		idDirOff[i] = off
		off += resDirSize + uint32(len(g.Leaves))*resEntrySize
	}
	langDirOff := make([]uint32, len(leaves))
	for i := range leaves {
		langDirOff[i] = off
		off += resDirSize + resEntrySize // one language each
	}
	dataEntryOff := make([]uint32, len(leaves))
	for i := range leaves {
		dataEntryOff[i] = off
		off += resDataEntrySz
	}
	dataOff := make([]uint32, len(leaves))
	for i, l := range leaves {
		off = align(off, 4) // keep image data 4-byte aligned
		dataOff[i] = off
		off += uint32(len(l.Data))
	}

	sec := make([]byte, off)
	putResDir(sec, 0, len(groups))
	// Root entries point at the per-type id directories.
	for i, g := range groups {
		e := resDirSize + i*resEntrySize
		binary.LittleEndian.PutUint32(sec[e:], g.Type)
		binary.LittleEndian.PutUint32(sec[e+4:], idDirOff[i]|resSubdirFlag)
	}
	leafIx := 0
	for i, g := range groups {
		putResDir(sec, idDirOff[i], len(g.Leaves))
		for j, l := range g.Leaves {
			e := int(idDirOff[i]) + resDirSize + j*resEntrySize
			binary.LittleEndian.PutUint32(sec[e:], l.ID)
			binary.LittleEndian.PutUint32(sec[e+4:], langDirOff[leafIx]|resSubdirFlag)

			putResDir(sec, langDirOff[leafIx], 1)
			le := int(langDirOff[leafIx]) + resDirSize
			binary.LittleEndian.PutUint32(sec[le:], resLangID)
			binary.LittleEndian.PutUint32(sec[le+4:], dataEntryOff[leafIx])

			d := int(dataEntryOff[leafIx])
			binary.LittleEndian.PutUint32(sec[d:], baseRVA+dataOff[leafIx])
			binary.LittleEndian.PutUint32(sec[d+4:], uint32(len(l.Data)))
			// CodePage and Reserved stay zero.
			copy(sec[dataOff[leafIx]:], l.Data)
			leafIx++
		}
	}
	return sec
}

// putResDir writes an IMAGE_RESOURCE_DIRECTORY with n id-keyed entries
// and no name-keyed entries.
func putResDir(sec []byte, off uint32, n int) {
	// Characteristics, TimeDateStamp, MajorVersion, MinorVersion and
	// NumberOfNamedEntries all stay zero.
	binary.LittleEndian.PutUint16(sec[off+14:], uint16(n))
}

// align rounds v up to the next multiple of a.
func align(v, a uint32) uint32 {
	if a == 0 {
		return v
	}
	return (v + a - 1) / a * a
}

// injectIcon appends a .rsrc section carrying images to a PE image and
// returns the rewritten file.
//
// Appending is only safe on an image that has no resources yet and no
// data past its last section; both are checked.  Go-linked binaries
// satisfy both unless a .syso was already linked in.
func injectIcon(exe []byte, images []iconImage) ([]byte, error) {
	h, err := parsePEHeader(exe)
	if err != nil {
		return nil, err
	}
	if h.ResourceSize != 0 {
		return nil, errors.New("binary already has a resource directory; " +
			"rebuild without a .syso, or supply no -icon")
	}
	if h.SectionTableEnd+peSectionHeaderSize > h.SizeOfHeaders {
		return nil, errors.New("no room in the PE header for another section")
	}
	if uint32(len(exe)) > h.RawEnd {
		return nil, fmt.Errorf("binary has %d bytes past its last section",
			uint32(len(exe))-h.RawEnd)
	}

	leaves := make([]resLeaf, 0, len(images)+1)
	for i, im := range images {
		leaves = append(leaves, resLeaf{Type: rtIcon, ID: uint32(i + 1), Data: im.Data})
	}
	leaves = append(leaves, resLeaf{Type: rtGroupIcon, ID: 1, Data: groupIconData(images)})

	newVA := align(h.VirtEnd, h.SectionAlignment)
	rsrc := buildResourceSection(leaves, newVA)
	rawOff := align(h.RawEnd, h.FileAlignment)
	rawSize := align(uint32(len(rsrc)), h.FileAlignment)

	out := make([]byte, rawOff+rawSize)
	copy(out, exe)
	copy(out[rawOff:], rsrc)

	// Section header for the new .rsrc.
	sh := out[h.SectionTableEnd:]
	copy(sh[:8], ".rsrc\x00\x00\x00")
	binary.LittleEndian.PutUint32(sh[8:], uint32(len(rsrc))) // VirtualSize
	binary.LittleEndian.PutUint32(sh[12:], newVA)            // VirtualAddress
	binary.LittleEndian.PutUint32(sh[16:], rawSize)          // SizeOfRawData
	binary.LittleEndian.PutUint32(sh[20:], rawOff)           // PointerToRawData
	// Relocations, line numbers and their counts stay zero.
	// IMAGE_SCN_CNT_INITIALIZED_DATA | IMAGE_SCN_MEM_READ
	binary.LittleEndian.PutUint32(sh[36:], 0x40000040)

	binary.LittleEndian.PutUint16(out[h.NumSectionsOff:], h.NumSections+1)
	binary.LittleEndian.PutUint32(out[h.SizeOfImageOff:],
		align(newVA+uint32(len(rsrc)), h.SectionAlignment))
	binary.LittleEndian.PutUint32(out[h.ResourceDirOff:], newVA)
	binary.LittleEndian.PutUint32(out[h.ResourceDirOff+4:], uint32(len(rsrc)))

	setPEChecksum(out, h.CheckSumOff)
	return out, nil
}

// peSectionHeaderSize is the size of one IMAGE_SECTION_HEADER.
const peSectionHeaderSize = 40

// peHeader collects the header fields injectIcon reads or rewrites.
// Offsets are absolute file offsets so the caller can patch in place.
type peHeader struct {
	NumSections      uint16
	NumSectionsOff   uint32
	SectionTableEnd  uint32 // where a new section header would go
	SizeOfHeaders    uint32
	SectionAlignment uint32
	FileAlignment    uint32
	SizeOfImageOff   uint32
	CheckSumOff      uint32
	ResourceDirOff   uint32 // DataDirectory[2]
	ResourceSize     uint32
	VirtEnd          uint32 // end of the highest-mapped section
	RawEnd           uint32 // end of the last section's raw data
}

// parsePEHeader reads the DOS stub, COFF header, optional header and
// section table by hand.  debug/pe is used for validation elsewhere but
// gives no file offsets, and offsets are what a rewrite needs.
func parsePEHeader(exe []byte) (peHeader, error) {
	var h peHeader
	if len(exe) < 0x40 || exe[0] != 'M' || exe[1] != 'Z' {
		return h, errors.New("not a PE file (no MZ signature)")
	}
	poff := binary.LittleEndian.Uint32(exe[0x3c:])
	if uint64(poff)+24 > uint64(len(exe)) ||
		!bytes.Equal(exe[poff:poff+4], []byte("PE\x00\x00")) {
		return h, errors.New("not a PE file (no PE signature)")
	}
	coff := poff + 4
	h.NumSectionsOff = coff + 2
	h.NumSections = binary.LittleEndian.Uint16(exe[h.NumSectionsOff:])
	optSize := uint32(binary.LittleEndian.Uint16(exe[coff+16:]))
	opt := coff + 20
	if uint64(opt)+uint64(optSize) > uint64(len(exe)) {
		return h, errors.New("truncated PE optional header")
	}

	// The optional header is identical up to offset 64 for PE32 and
	// PE32+; only the data directory moves, because PE32 carries an
	// extra 4-byte BaseOfData and PE32+ widens four fields to 8 bytes.
	var dataDir uint32
	switch magic := binary.LittleEndian.Uint16(exe[opt:]); magic {
	case 0x10b: // PE32
		dataDir = opt + 96
	case 0x20b: // PE32+
		dataDir = opt + 112
	default:
		return h, fmt.Errorf("unknown optional header magic 0x%x", magic)
	}
	h.SectionAlignment = binary.LittleEndian.Uint32(exe[opt+32:])
	h.FileAlignment = binary.LittleEndian.Uint32(exe[opt+36:])
	h.SizeOfImageOff = opt + 56
	h.SizeOfHeaders = binary.LittleEndian.Uint32(exe[opt+60:])
	h.CheckSumOff = opt + 64
	h.ResourceDirOff = dataDir + peResourceDirIx*8
	if uint64(h.ResourceDirOff)+8 > uint64(len(exe)) {
		return h, errors.New("truncated PE data directory")
	}
	h.ResourceSize = binary.LittleEndian.Uint32(exe[h.ResourceDirOff+4:])

	secTable := opt + optSize
	h.SectionTableEnd = secTable + uint32(h.NumSections)*peSectionHeaderSize
	if uint64(h.SectionTableEnd) > uint64(len(exe)) {
		return h, errors.New("truncated PE section table")
	}
	for i := range int(h.NumSections) {
		s := exe[secTable+uint32(i)*peSectionHeaderSize:]
		vend := binary.LittleEndian.Uint32(s[12:]) + binary.LittleEndian.Uint32(s[8:])
		rend := binary.LittleEndian.Uint32(s[20:]) + binary.LittleEndian.Uint32(s[16:])
		h.VirtEnd = max(h.VirtEnd, vend)
		h.RawEnd = max(h.RawEnd, rend)
	}
	if h.SectionAlignment == 0 || h.FileAlignment == 0 {
		return h, errors.New("PE alignment fields are zero")
	}
	return h, nil
}

// setPEChecksum recomputes the optional header CheckSum: a 16-bit
// ones-complement sum over the whole file with the field zeroed, plus
// the file size.  Windows only enforces it for drivers and boot
// images, but tools that do read it flag a stale value.
func setPEChecksum(out []byte, off uint32) {
	binary.LittleEndian.PutUint32(out[off:], 0)
	var sum uint32
	n := len(out) &^ 1 // an odd trailing byte pads with zero, so contributes nothing
	for i := 0; i < n; i += 2 {
		sum += uint32(binary.LittleEndian.Uint16(out[i:]))
		sum = (sum & 0xffff) + (sum >> 16)
	}
	sum = (sum >> 16) + (sum & 0xffff)
	sum += sum >> 16
	sum &= 0xffff
	binary.LittleEndian.PutUint32(out[off:], sum+uint32(len(out)))
}

// writeZip writes a one-entry archive holding data under name.
func writeZip(path, name string, data []byte) error {
	f, err := os.Create(path) // #nosec G304 — CLI flag
	if err != nil {
		return err
	}
	zw := zip.NewWriter(f)
	w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
	if err != nil {
		return closeAll(err, zw, f)
	}
	if _, err = w.Write(data); err != nil {
		return closeAll(err, zw, f)
	}
	return closeAll(nil, zw, f)
}
