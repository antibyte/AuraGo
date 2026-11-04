package ecalc

import (
	"errors"
	"io"
	"io/fs"
	"strings"
)

// Helpers shared by the three kinds of catalog items (calculators/, tables/, codes/): reading a file that is a YAML list
// of items, claiming an id that is unique across the kinds, and checking the fields every kind has.

// maxIDLength is the longest item id (the desktop bridge drops longer ones).
const maxIDLength = 63

// readItems decodes the YAML list of one catalog file strictly; a problem is reported and false returned.
func readItems[T any](l *loader, fsys fs.FS, file string) ([]T, bool) {
	data, err := fs.ReadFile(fsys, file)
	if err != nil {
		l.fail("%s: %v", file, err)
		return nil, false
	}
	var raws []T
	if err := decodeStrict(data, &raws); err != nil {
		if errors.Is(err, io.EOF) {
			err = errors.New("the file is empty")
		}
		l.fail("%s: %v", file, err)
		return nil, false
	}
	return raws, true
}

// claimID registers the id of an item as defined in file; ids are unique across the four kinds.
func (l *loader) claimID(id, file string, fail func(string, ...any)) {
	switch {
	case !idRe.MatchString(id):
		fail("id must match %s", idRe)
	case len(id) > maxIDLength:
		fail("id must be at most %d characters", maxIDLength)
	case l.ids[id] != "":
		fail("id is already used in %s", l.ids[id])
	default:
		l.ids[id] = file
	}
}

// commonFields checks what a table or a code item has besides its own fields: the name in all 16 locales, an optional
// doc (English and German when it is present), at least one tag and a source.
func (l *loader) commonFields(fail func(string, ...any), text func(string, Text, []string), name, doc Text, tags []string, source string) {
	text("name", name, Locales)
	if len(doc) > 0 {
		text("doc", doc, docLocales)
	}
	if strings.TrimSpace(source) == "" {
		fail("source is required (a standard, datasheet or textbook)")
	}
	if len(tags) == 0 {
		fail("at least one tag is required")
	}
	for _, tag := range tags {
		if strings.TrimSpace(tag) == "" {
			fail("tags must not be empty")
		}
	}
}

// Item returns the kind of the catalog item with the given id: "calculator", "table", "code" or "pinout".
func (c *Catalog) Item(id string) (kind string, ok bool) {
	kind, ok = c.kinds[id]
	return kind, ok
}
