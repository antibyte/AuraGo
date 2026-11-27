package storymaker

import (
	"encoding/json"
	"io"
	"io/fs"
	"path"
	"regexp"
	"strings"
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
	if c == nil {
		return Icon{}, false
	}
	i, ok := c.iconIdx[id]
	if !ok {
		return Icon{}, false
	}
	return c.icons[i], true
}

// iconEntry is the search view of an icon: the name is the ID, so a tie keeps catalog entries first.
func iconEntry(ic Icon) Entry {
	return Entry{ID: ic.ID, Kind: "icon", Name: ic.ID, Tags: ic.Tags}
}

// iconTermScore is termScore, but an exact name counts like an exact tag: the term "lock" finds icon.lock before
// icon.lock-open. A catalog entry with an exact tag (prop.lock) still comes first, because it sorts first on a tie.
func iconTermScore(e Entry, term string) int {
	score := termScore(e, term)
	if term == strings.TrimPrefix(e.ID, "icon.") {
		score = max(score, 4)
	}
	return score
}

// IconIDs returns every icon ID, sorted.
func (c *Catalog) IconIDs() []string {
	if c == nil {
		return nil
	}
	ids := make([]string, len(c.icons))
	for i, ic := range c.icons {
		ids[i] = ic.ID
	}
	return ids
}

// IconsFile returns the icon data file name, relative to the folder of catalog.json, or "".
func (c *Catalog) IconsFile() string {
	if c == nil {
		return ""
	}
	return c.iconsFile
}

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

// iconLabel turns the Lucide name of an icon ID into a label: "server-cog" becomes "Server cog".
func iconLabel(id string) string {
	name := strings.ReplaceAll(strings.TrimPrefix(id, "icon."), "-", " ")
	if name == "" {
		return ""
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// iconUsageExample is the example a catalog search hit carries for an icon.
func iconUsageExample(id string) json.RawMessage {
	data, _ := json.Marshal(struct {
		Do    string `json:"do"`
		Icon  string `json:"icon"`
		Style string `json:"style"`
		Text  string `json:"text"`
		At    string `json:"at"`
		Entry string `json:"entry"`
	}{"show", id, "card", iconLabel(id), "board", "draw"})
	return data
}

// iconFreeUsageExample is the example a free project gets: a line of scene code.
func iconFreeUsageExample(id string) json.RawMessage {
	data, _ := json.Marshal(struct {
		Code string `json:"code"`
	}{"api.icon('" + id + "', {style: 'card', label: '" + iconLabel(id) + "', at: 'right'})"})
	return data
}
