package zim

import (
	"bytes"
	"encoding/binary"
	"errors"
)

const (
	mimeRedirect   = 0xFFFF
	mimeLinkTarget = 0xFFFE // deprecated, carries no content
	mimeDeleted    = 0xFFFD // deprecated, carries no content
)

type direntKind uint8

const (
	kindContent direntKind = iota
	kindRedirect
	kindDeprecated
)

// Entry is one directory entry. Values are only meaningful for the Archive
// that returned them.
type Entry struct {
	Index      uint32
	Namespace  byte // 'C', 'M', 'W', 'X', or legacy 'A', 'I', '-'
	Path       string
	Title      string // Path if the stored title is empty
	MimeType   string // "" for redirects
	IsRedirect bool
	RedirectTo uint32 // valid if IsRedirect

	kind    direntKind
	cluster uint32
	blob    uint32
}

var errShortDirent = errors.New("zim: directory entry extends beyond buffer")

// parseDirent decodes one directory entry from the start of b.
// errShortDirent means b ended before the entry did.
func parseDirent(b []byte, mimeTypes []string) (Entry, error) {
	if len(b) < 8 {
		return Entry{}, errShortDirent
	}
	le := binary.LittleEndian
	mime := le.Uint16(b[0:2])
	paramLen := int(b[2])
	e := Entry{Namespace: b[3]}
	var rest []byte
	switch mime {
	case mimeRedirect:
		if len(b) < 12 {
			return Entry{}, errShortDirent
		}
		e.kind = kindRedirect
		e.IsRedirect = true
		e.RedirectTo = le.Uint32(b[8:12])
		rest = b[12:]
	case mimeLinkTarget, mimeDeleted:
		e.kind = kindDeprecated
		rest = b[8:]
	default:
		if int(mime) >= len(mimeTypes) {
			return Entry{}, errCorrupt("MIME type index %d out of range (%d types)", mime, len(mimeTypes))
		}
		if len(b) < 16 {
			return Entry{}, errShortDirent
		}
		e.kind = kindContent
		e.MimeType = mimeTypes[mime]
		e.cluster = le.Uint32(b[8:12])
		e.blob = le.Uint32(b[12:16])
		rest = b[16:]
	}
	path, rest, ok := cutCString(rest)
	if !ok {
		return Entry{}, errShortDirent
	}
	title, rest, ok := cutCString(rest)
	if !ok {
		return Entry{}, errShortDirent
	}
	if len(rest) < paramLen {
		return Entry{}, errShortDirent
	}
	e.Path = path
	e.Title = title
	if e.Title == "" {
		e.Title = path
	}
	return e, nil
}

func cutCString(b []byte) (string, []byte, bool) {
	i := bytes.IndexByte(b, 0)
	if i < 0 {
		return "", nil, false
	}
	return string(b[:i]), b[i+1:], true
}

// compareKey orders entries like the path pointer list: namespace byte first,
// then the path bytes.
func compareKey(ns byte, path string, otherNS byte, otherPath string) int {
	switch {
	case ns < otherNS:
		return -1
	case ns > otherNS:
		return 1
	case path < otherPath:
		return -1
	case path > otherPath:
		return 1
	}
	return 0
}
