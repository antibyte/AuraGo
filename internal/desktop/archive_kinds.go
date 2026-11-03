package desktop

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"path/filepath"
	"strings"
)

// ErrArchiveTooManyEntries is returned when a listing or extract exceeds ArchiveEntryLimit.
var ErrArchiveTooManyEntries = errors.New("archive contains too many entries")

var (
	errArchiveUnsupported = errors.New("unsupported archive format")
	errArchiveEncrypted   = errors.New("encrypted archives are not supported")
)

type archiveKind uint8

const (
	archiveUnknown archiveKind = iota
	archiveProbe
	archiveZip
	archiveTar
	archiveTarGz
	archiveTarBz2
	archiveTarXz
	archiveTarZst
	archiveGz
	archiveBz2
	archiveXz
	archiveZst
	archive7z
	archiveRar
	archiveCab
	archiveIso
	archiveWim
	archiveArj
	archiveLzh
	archiveCpio
	archiveDeb
	archiveRpm
	archiveXar
	archiveLzma
	archiveLz
	archiveExt
	archiveSquashfs
	archiveBtrfs
	archiveXfs
	archiveErofs
	archiveCramfs
	archiveQcow
	archiveVmdk
	archiveVhd
	archiveVdi
	archiveDmg
	archiveUdf
)

func (k archiveKind) String() string {
	switch k {
	case archiveZip:
		return "zip"
	case archiveTar:
		return "tar"
	case archiveTarGz:
		return "tar.gz"
	case archiveTarBz2:
		return "tar.bz2"
	case archiveTarXz:
		return "tar.xz"
	case archiveTarZst:
		return "tar.zst"
	case archiveGz:
		return "gz"
	case archiveBz2:
		return "bz2"
	case archiveXz:
		return "xz"
	case archiveZst:
		return "zst"
	case archive7z:
		return "7z"
	case archiveRar:
		return "rar"
	case archiveCab:
		return "cab"
	case archiveIso:
		return "iso"
	case archiveWim:
		return "wim"
	case archiveArj:
		return "arj"
	case archiveLzh:
		return "lzh"
	case archiveCpio:
		return "cpio"
	case archiveDeb:
		return "deb"
	case archiveRpm:
		return "rpm"
	case archiveXar:
		return "xar"
	case archiveLzma:
		return "lzma"
	case archiveLz:
		return "lz"
	case archiveExt:
		return "ext"
	case archiveSquashfs:
		return "squashfs"
	case archiveBtrfs:
		return "btrfs"
	case archiveXfs:
		return "xfs"
	case archiveErofs:
		return "erofs"
	case archiveCramfs:
		return "cramfs"
	case archiveQcow:
		return "qcow"
	case archiveVmdk:
		return "vmdk"
	case archiveVhd:
		return "vhd"
	case archiveVdi:
		return "vdi"
	case archiveDmg:
		return "dmg"
	case archiveUdf:
		return "udf"
	case archiveProbe:
		return "image"
	default:
		return "unknown"
	}
}

func (k archiveKind) creatable() bool {
	switch k {
	case archiveZip, archiveTar, archiveTarGz, archiveTarXz, archiveTarZst:
		return true
	default:
		return false
	}
}

func (k archiveKind) native() bool {
	switch k {
	case archiveZip, archiveTar, archiveTarGz, archiveTarBz2, archiveTarXz, archiveTarZst,
		archiveGz, archiveBz2, archiveXz, archiveZst:
		return true
	default:
		return false
	}
}

func (k archiveKind) singleFile() bool {
	switch k {
	case archiveGz, archiveBz2, archiveXz, archiveZst:
		return true
	default:
		return false
	}
}

// filesystem reports disk and filesystem images. Their symlinks are listed and
// skipped. Classic archives fail closed instead.
func (k archiveKind) filesystem() bool {
	switch k {
	case archiveExt, archiveSquashfs, archiveBtrfs, archiveXfs, archiveErofs, archiveCramfs,
		archiveIso, archiveUdf, archiveQcow, archiveVmdk, archiveVhd, archiveVdi, archiveDmg:
		return true
	default:
		return false
	}
}

func (k archiveKind) bsdtarReadable() bool {
	switch k {
	case archiveIso, archiveCab, archiveCpio, archiveXar, archiveArj, archiveLzh, archiveDeb, archiveRpm:
		return true
	default:
		return false
	}
}

func archiveKindForCreate(dest string) archiveKind {
	kind := kindFromName(dest)
	if kind == archiveUnknown || kind == archiveProbe {
		return archiveZip
	}
	return kind
}

func kindFromName(name string) archiveKind {
	lower := strings.ToLower(filepath.Base(name))
	switch {
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return archiveTarGz
	case strings.HasSuffix(lower, ".tar.bz2"), strings.HasSuffix(lower, ".tbz2"), strings.HasSuffix(lower, ".tbz"):
		return archiveTarBz2
	case strings.HasSuffix(lower, ".tar.xz"), strings.HasSuffix(lower, ".txz"):
		return archiveTarXz
	case strings.HasSuffix(lower, ".tar.zst"), strings.HasSuffix(lower, ".tzst"):
		return archiveTarZst
	case strings.HasSuffix(lower, ".tar.lzma"):
		return archiveLzma
	case strings.HasSuffix(lower, ".tar.lz"):
		return archiveLz
	}
	switch strings.TrimPrefix(filepath.Ext(lower), ".") {
	case "zip":
		return archiveZip
	case "tar":
		return archiveTar
	case "gz":
		return archiveGz
	case "bz2":
		return archiveBz2
	case "xz":
		return archiveXz
	case "zst":
		return archiveZst
	case "lzma":
		return archiveLzma
	case "lz":
		return archiveLz
	case "7z":
		return archive7z
	case "rar":
		return archiveRar
	case "cab":
		return archiveCab
	case "iso":
		return archiveIso
	case "wim":
		return archiveWim
	case "arj":
		return archiveArj
	case "lzh", "lha":
		return archiveLzh
	case "cpio":
		return archiveCpio
	case "deb":
		return archiveDeb
	case "rpm":
		return archiveRpm
	case "xar":
		return archiveXar
	case "img", "raw", "bin":
		return archiveProbe
	case "ext2", "ext3", "ext4":
		return archiveExt
	case "squashfs", "sqsh", "sfs":
		return archiveSquashfs
	case "btrfs":
		return archiveBtrfs
	case "xfs":
		return archiveXfs
	case "erofs":
		return archiveErofs
	case "cramfs":
		return archiveCramfs
	case "qcow", "qcow2":
		return archiveQcow
	case "vmdk":
		return archiveVmdk
	case "vhd", "vhdx":
		return archiveVhd
	case "vdi":
		return archiveVdi
	case "dmg":
		return archiveDmg
	case "udf":
		return archiveUdf
	default:
		return archiveUnknown
	}
}

func kindFromMagic(header []byte) archiveKind {
	if len(header) >= 6 && bytes.Equal(header[:6], []byte{0x37, 0x7A, 0xBC, 0xAF, 0x27, 0x1C}) {
		return archive7z
	}
	if len(header) >= 6 && bytes.Equal(header[:6], []byte("Rar!\x1a\x07")) {
		return archiveRar
	}
	if len(header) >= 4 && header[0] == 'P' && header[1] == 'K' && (header[2] == 3 || header[2] == 5 || header[2] == 7) {
		return archiveZip
	}
	if len(header) >= 2 && header[0] == 0x1f && header[1] == 0x8b {
		return archiveGz
	}
	if len(header) >= 3 && header[0] == 'B' && header[1] == 'Z' && header[2] == 'h' {
		return archiveBz2
	}
	if len(header) >= 6 && bytes.Equal(header[:6], []byte{0xFD, '7', 'z', 'X', 'Z', 0x00}) {
		return archiveXz
	}
	if len(header) >= 4 && bytes.Equal(header[:4], []byte{0x28, 0xB5, 0x2F, 0xFD}) {
		return archiveZst
	}
	if len(header) >= 4 && bytes.Equal(header[:4], []byte("MSCF")) {
		return archiveCab
	}
	if len(header) >= 5 && bytes.Equal(header[:5], []byte("MSWIM")) {
		return archiveWim
	}
	if len(header) >= 4 && bytes.Equal(header[:4], []byte{'Q', 'F', 'I', 0xfb}) {
		return archiveQcow
	}
	if len(header) >= 4 && bytes.Equal(header[:4], []byte("KDMV")) {
		return archiveVmdk
	}
	if len(header) >= 4 && (bytes.Equal(header[:4], []byte("hsqs")) || bytes.Equal(header[:4], []byte("sqsh"))) {
		return archiveSquashfs
	}
	if len(header) >= 4 && bytes.Equal(header[:4], []byte("XFSB")) {
		return archiveXfs
	}
	if len(header) >= 4 && binary.LittleEndian.Uint32(header[:4]) == 0x28CD3D45 {
		return archiveCramfs
	}
	if len(header) >= 64 && bytes.Contains(header[:64], []byte("VirtualBox")) {
		return archiveVdi
	}
	if len(header) >= 1028 && binary.LittleEndian.Uint32(header[1024:1028]) == 0xE0F5E1E2 {
		return archiveErofs
	}
	if len(header) >= 0x43A && binary.LittleEndian.Uint16(header[0x438:0x43A]) == 0xEF53 {
		return archiveExt
	}
	if len(header) >= 0x10048 && bytes.Equal(header[0x10040:0x10048], []byte("_BHRfS_M")) {
		return archiveBtrfs
	}
	if len(header) >= 0x8006 && bytes.Equal(header[0x8001:0x8006], []byte("CD001")) {
		return archiveIso
	}
	if len(header) >= 262 && bytes.Equal(header[257:262], []byte("ustar")) {
		return archiveTar
	}
	return archiveUnknown
}

func detectArchiveKind(name string, rs io.ReadSeeker) (archiveKind, error) {
	if rs == nil {
		return archiveUnknown, errArchiveUnsupported
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return archiveUnknown, err
	}
	header := make([]byte, 0x11000)
	n, err := io.ReadFull(rs, header)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return archiveUnknown, err
	}
	header = header[:n]
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return archiveUnknown, err
	}
	kind := kindFromName(name)
	if kind == archiveProbe || kind == archiveUnknown {
		kind = kindFromMagic(header)
	}
	switch kind {
	case archiveGz, archiveBz2, archiveXz, archiveZst:
		if compressedLooksLikeTar(rs, kind) {
			return tarKindForCompression(kind), nil
		}
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return archiveUnknown, err
	}
	return kind, nil
}

func tarKindForCompression(kind archiveKind) archiveKind {
	switch kind {
	case archiveGz:
		return archiveTarGz
	case archiveBz2:
		return archiveTarBz2
	case archiveXz:
		return archiveTarXz
	case archiveZst:
		return archiveTarZst
	default:
		return kind
	}
}
