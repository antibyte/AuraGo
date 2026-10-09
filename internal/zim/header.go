package zim

import (
	"encoding/binary"
	"fmt"
)

const (
	headerSize       = 80
	legacyHeaderSize = 72 // archives without checksumPos (mimeListPos == 72)
	zimMagic         = 0x044D495A
	noMainPage       = 0xFFFFFFFF
	noTitleListV0    = ^uint64(0)
	checksumSize     = 16
)

// header is the fixed 80-byte ZIM header (all integers little-endian).
type header struct {
	major         uint16
	minor         uint16
	uuid          [16]byte
	entryCount    uint32
	clusterCount  uint32
	pathPtrPos    uint64
	titlePtrPos   uint64
	clusterPtrPos uint64
	mimeListPos   uint64
	mainPage      uint32
	layoutPage    uint32
	checksumPos   uint64
}

// hasChecksum mirrors libzim: only 80-byte headers carry checksumPos.
func (h header) hasChecksum() bool { return h.mimeListPos >= headerSize }

func (h header) hasTitleListV0() bool { return h.titlePtrPos != noTitleListV0 }

// dataEnd is the exclusive end of archive data: the checksum position when
// present, otherwise the file size.
func (h header) dataEnd(fileSize int64) int64 {
	if h.hasChecksum() {
		return int64(h.checksumPos)
	}
	return fileSize
}

// tableFits reports whether count items of width bytes starting at pos lie
// inside [minPos, end) without overflowing.
func tableFits(pos, count, width, minPos, end uint64) bool {
	if pos < minPos || pos > end {
		return false
	}
	return count <= (end-pos)/width
}

// parseHeader decodes and validates the header against the file size.
func parseHeader(b []byte, fileSize int64) (header, error) {
	if len(b) < headerSize || fileSize < headerSize {
		return header{}, fmt.Errorf("%w: file too small for a ZIM header", ErrNotZIM)
	}
	le := binary.LittleEndian
	if le.Uint32(b[0:4]) != zimMagic {
		return header{}, fmt.Errorf("%w: bad magic number", ErrNotZIM)
	}
	h := header{
		major:         le.Uint16(b[4:6]),
		minor:         le.Uint16(b[6:8]),
		entryCount:    le.Uint32(b[24:28]),
		clusterCount:  le.Uint32(b[28:32]),
		pathPtrPos:    le.Uint64(b[32:40]),
		titlePtrPos:   le.Uint64(b[40:48]),
		clusterPtrPos: le.Uint64(b[48:56]),
		mimeListPos:   le.Uint64(b[56:64]),
		mainPage:      le.Uint32(b[64:68]),
		layoutPage:    le.Uint32(b[68:72]),
		checksumPos:   le.Uint64(b[72:80]),
	}
	copy(h.uuid[:], b[8:24])
	if h.major != 5 && h.major != 6 {
		return header{}, errUnsupported("major version %d", h.major)
	}
	if h.mimeListPos != headerSize && h.mimeListPos != legacyHeaderSize {
		return header{}, errCorrupt("MIME list position %d", h.mimeListPos)
	}
	size := uint64(fileSize)
	if h.hasChecksum() && h.checksumPos != size-checksumSize {
		return header{}, errCorrupt("checksum position %d does not match file size %d (truncated or incomplete file)", h.checksumPos, size)
	}
	end := uint64(h.dataEnd(fileSize))
	if (h.entryCount == 0) != (h.clusterCount == 0) {
		return header{}, errCorrupt("entry count %d with cluster count %d", h.entryCount, h.clusterCount)
	}
	if h.clusterCount > h.entryCount {
		return header{}, errCorrupt("cluster count %d exceeds entry count %d", h.clusterCount, h.entryCount)
	}
	if h.mainPage != noMainPage && h.mainPage >= h.entryCount {
		return header{}, errCorrupt("main page index %d is outside the %d entries", h.mainPage, h.entryCount)
	}
	if !tableFits(h.pathPtrPos, uint64(h.entryCount), 8, h.mimeListPos, end) {
		return header{}, errCorrupt("path pointer list at %d outside the archive", h.pathPtrPos)
	}
	if !tableFits(h.clusterPtrPos, uint64(h.clusterCount), 8, h.mimeListPos, end) {
		return header{}, errCorrupt("cluster pointer list at %d outside the archive", h.clusterPtrPos)
	}
	if h.hasTitleListV0() && h.titlePtrPos < h.mimeListPos {
		return header{}, errCorrupt("title pointer list at %d overlaps the header", h.titlePtrPos)
	}
	return h, nil
}
