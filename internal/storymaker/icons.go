package storymaker

import (
	"encoding/json"
	"io"
	"io/fs"
	"path"
	"regexp"
)

// IconsSchema is the schema of the icon data file (Lucide icons converted to path data on the 24 grid).
const IconsSchema = "storymaker-icons/1"

const maxIconsBytes = 1 << 20

var (
	iconIDPattern   = regexp.MustCompile(`^icon\.[a-z0-9-]{1,60}$`)
	iconPathPattern = regexp.MustCompile(`^[MmLlHhVvCcSsQqTtAaZz0-9 .,eE-]+$`)
)

// Icon is one stroked icon: path strings on a 24 grid. Fills are the elements drawn filled and stroked.
type Icon struct {
	ID    string   `json:"id"`
	Paths []string `json:"paths"`
	Fills []string `json:"fills,omitempty"`
	Tags  []string `json:"tags"`
}

// iconsRef is the manifest key that names the icon data file.
type iconsRef struct {
	File string `json:"file"`
}

type iconsDoc struct {
	Schema string `json:"schema"`
	Icons  []Icon `json:"icons"`
}

// Icon returns the icon with the ID, for example "icon.cpu".
func (c *Catalog) Icon(id string) (Icon, bool) {
	i, ok := c.iconIdx[id]
	if !ok {
		return Icon{}, false
	}
	return c.icons[i], true
}

// IconIDs returns every icon ID, sorted.
func (c *Catalog) IconIDs() []string {
	ids := make([]string, len(c.icons))
	for i, ic := range c.icons {
		ids[i] = ic.ID
	}
	return ids
}

// IconsFile returns the icon data file name, relative to the folder of catalog.json, or "".
func (c *Catalog) IconsFile() string { return c.iconsFile }

func (c *Catalog) loadIcons(fsys fs.FS, catalogPath, file string, bad func(string, ...any)) {
	if !safeRelPath(file) {
		bad("icons needs a relative file path")
		return
	}
	full := path.Join(path.Dir(catalogPath), file)
	f, err := fsys.Open(full)
	if err != nil {
		bad("open icons: %v", err)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxIconsBytes+1))
	if err != nil {
		bad("read icons %s: %v", full, err)
		return
	}
	if len(data) > maxIconsBytes {
		bad("icons %s is larger than %d bytes", full, maxIconsBytes)
		return
	}
	var doc iconsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		bad("parse icons %s: %v", full, err)
		return
	}
	if doc.Schema != IconsSchema {
		bad("icons %s: schema is %q, want %q", full, doc.Schema, IconsSchema)
		return
	}
	for i, ic := range doc.Icons {
		if !iconIDPattern.MatchString(ic.ID) {
			bad("icons: id %q must match icon.[a-z0-9-]", ic.ID)
			continue
		}
		if i > 0 && doc.Icons[i-1].ID >= ic.ID {
			bad("icons: %s is a duplicate or not sorted after %s", ic.ID, doc.Icons[i-1].ID)
		}
		if n := len(ic.Paths); n < 1 || n > 40 {
			bad("icons: %s has %d paths, want 1 to 40", ic.ID, n)
		}
		for _, p := range append(append([]string(nil), ic.Paths...), ic.Fills...) {
			if len(p) > 2000 || !iconPathPattern.MatchString(p) {
				bad("icons: %s has an invalid path string", ic.ID)
				break
			}
		}
		if len(ic.Tags) > 32 {
			bad("icons: %s has %d tags, want at most 32", ic.ID, len(ic.Tags))
		}
	}
	c.icons, c.iconsFile = doc.Icons, file
	c.iconIdx = make(map[string]int, len(doc.Icons))
	for i, ic := range doc.Icons {
		c.iconIdx[ic.ID] = i
	}
}
