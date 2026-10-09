// Package zim reads ZIM archives (openZIM file format, major versions 5 and 6).
//
// The reader is read-only, CGO-free and safe for concurrent use. It never
// holds a global lock while reading or decompressing: file access uses
// ReadAt, decompressed clusters live in a byte-bounded per-archive LRU, and
// concurrent requests for the same cluster share one decompression.
package zim

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound     = errors.New("zim: entry not found")
	ErrNotZIM       = errors.New("zim: not a ZIM file")
	ErrUnsupported  = errors.New("zim: unsupported ZIM feature")
	ErrCorrupt      = errors.New("zim: corrupt archive")
	ErrRedirectLoop = errors.New("zim: redirect chain too long")
	// ErrIsRedirect is returned by Archive.Open for redirect entries; call Resolve first.
	ErrIsRedirect = errors.New("zim: entry is a redirect")
	// ErrClosed is returned by every read after Archive.Close.
	ErrClosed = errors.New("zim: archive closed")
)

func errCorrupt(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrCorrupt, fmt.Sprintf(format, args...))
}

func errUnsupported(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrUnsupported, fmt.Sprintf(format, args...))
}
