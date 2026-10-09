package xapian

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupportedFormat marks a database this reader cannot interpret: not a
	// single-file glass database, another glass format version, or a layout
	// this package does not implement. Callers fall back to title search.
	ErrUnsupportedFormat = errors.New("xapian: unsupported database format")
	// ErrCorrupt marks structurally invalid data inside a glass database.
	ErrCorrupt = errors.New("xapian: corrupt database")
	// ErrDocNotFound is returned for document ids that do not exist.
	ErrDocNotFound = errors.New("xapian: document not found")
)

func corruptf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrCorrupt}, args...)...)
}

func unsupportedf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrUnsupportedFormat}, args...)...)
}
