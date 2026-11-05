package storymaker

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// GlyphsSchema is the schema string of glyphs.json.
const GlyphsSchema = "storymaker-glyphs/1"

// maxGlyphsBytes caps the coverage file.
const maxGlyphsBytes = 1 << 20

// GlyphFace is one fallback face of glyphs.json.
type GlyphFace struct {
	ID, Family, File string
	Weight           string // FontFace descriptor: "400", "300 700", "100 900"
	UnicodeRange     string // "" means no restriction
	MaxBytes         int64  // 0 means the 8 MiB cap
}

// Glyphs is the decoded coverage file: the faces in fallback order and the code points of each.
// It is immutable after LoadGlyphs. A nil Glyphs covers everything, so the check is skipped.
type Glyphs struct {
	Faces []GlyphFace

	ranges [][][2]rune // per face: inclusive pairs in ascending order
}

// glyphsRef is the manifest key that names the coverage file.
type glyphsRef struct {
	File string `json:"file"`
}

type glyphsDoc struct {
	Schema string `json:"schema"`
	Faces  []struct {
		ID           string  `json:"id"`
		Family       string  `json:"family"`
		File         string  `json:"file"`
		Weight       string  `json:"weight"`
		UnicodeRange string  `json:"unicode_range"`
		MaxBytes     int64   `json:"max_bytes"`
		D            []int64 `json:"d"`
	} `json:"faces"`
}

// LoadGlyphs reads and checks the coverage file at path. The build generates it from the font cmaps,
// so Go needs no font parser.
func LoadGlyphs(fsys fs.FS, path string) (*Glyphs, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, fmt.Errorf("storymaker: open glyphs: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxGlyphsBytes+1))
	if err != nil {
		return nil, fmt.Errorf("storymaker: read glyphs: %w", err)
	}
	if len(data) > maxGlyphsBytes {
		return nil, fmt.Errorf("storymaker: glyphs %s is larger than %d bytes", path, maxGlyphsBytes)
	}
	var doc glyphsDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("storymaker: parse glyphs %s: %w", path, err)
	}
	if doc.Schema != GlyphsSchema {
		return nil, fmt.Errorf("storymaker: glyphs %s: schema is %q, want %q", path, doc.Schema, GlyphsSchema)
	}
	g := &Glyphs{}
	for i, face := range doc.Faces {
		if face.ID == "" || face.Family == "" || face.Weight == "" || face.MaxBytes < 0 {
			return nil, fmt.Errorf("storymaker: glyphs %s: faces[%d] needs id, family and weight", path, i)
		}
		if !strings.HasPrefix(face.File, "/fonts/") || strings.Contains(face.File, "..") {
			return nil, fmt.Errorf("storymaker: glyphs %s: face %s file must be a /fonts/ path", path, face.ID)
		}
		ranges, err := decodeGlyphRanges(face.D)
		if err != nil {
			return nil, fmt.Errorf("storymaker: glyphs %s: face %s: %w", path, face.ID, err)
		}
		g.Faces = append(g.Faces, GlyphFace{
			ID: face.ID, Family: face.Family, File: face.File, Weight: face.Weight,
			UnicodeRange: face.UnicodeRange, MaxBytes: face.MaxBytes,
		})
		g.ranges = append(g.ranges, ranges)
	}
	return g, nil
}

// decodeGlyphRanges reads the delta format: the first value is the first start, then
// gap = start - previous end and length = end - start.
func decodeGlyphRanges(d []int64) ([][2]rune, error) {
	if len(d)%2 != 0 {
		return nil, fmt.Errorf("d must hold pairs of values, got %d values", len(d))
	}
	const last = int64(unicode.MaxRune)
	ranges := make([][2]rune, 0, len(d)/2)
	var end int64
	for i := 0; i < len(d); i += 2 {
		gap, length := d[i], d[i+1]
		if gap < 0 || (i > 0 && gap == 0) || length < 0 {
			return nil, fmt.Errorf("ranges are not monotonic at value %d", i)
		}
		if gap > last || length > last || end+gap+length > last {
			return nil, fmt.Errorf("the range at value %d runs past U+10FFFF", i)
		}
		start := end + gap
		end = start + length
		ranges = append(ranges, [2]rune{rune(start), rune(end)})
	}
	return ranges, nil
}

func glyphRangesHave(ranges [][2]rune, r rune) bool {
	i := sort.Search(len(ranges), func(i int) bool { return ranges[i][1] >= r })
	return i < len(ranges) && ranges[i][0] <= r
}

// FaceOf returns the index of the first face in fallback order that covers r, or -1.
func (g *Glyphs) FaceOf(r rune) int {
	if g == nil {
		return -1
	}
	for i := range g.ranges {
		if glyphRangesHave(g.ranges[i], r) {
			return i
		}
	}
	return -1
}

// Covered reports a character that needs no missing-glyph error: a face has it, it draws nothing
// (default-ignorable, C0 control), or it is JS white space, which the player skips as well.
func (g *Glyphs) Covered(r rune) bool {
	if g == nil {
		return true
	}
	return g.FaceOf(r) >= 0 || IsDefaultIgnorable(r) || IsJSSpace(r) || r < 0x20
}

// Missing lists the characters of the NFC form of text that are not covered, distinct and in order.
func (g *Glyphs) Missing(text string) []rune {
	if g == nil {
		return nil
	}
	var out []rune
	seen := map[rune]bool{}
	for _, r := range norm.NFC.String(text) {
		if !seen[r] && !g.Covered(r) {
			seen[r] = true
			out = append(out, r)
		}
	}
	return out
}

// IsDefaultIgnorable reports a Default_Ignorable_Code_Point of the generated Unicode table in ranges_gen.go.
func IsDefaultIgnorable(r rune) bool {
	return inUCDTable(ucdDefaultIgnorable, r)
}

// Glyphs returns the coverage data, or nil when the manifest names no glyphs file.
func (c *Catalog) Glyphs() *Glyphs {
	if c == nil {
		return nil
	}
	return c.glyphs
}

// GlyphsFile returns the coverage file name, relative to the folder of catalog.json, or "".
func (c *Catalog) GlyphsFile() string {
	if c == nil {
		return ""
	}
	return c.glyphsFile
}

// loadGlyphs reads the file the manifest names, from the folder of catalog.json.
func (c *Catalog) loadGlyphs(fsys fs.FS, catalogPath, file string, bad func(string, ...any)) {
	if !safeRelPath(file) {
		bad("glyphs needs a relative file path")
		return
	}
	g, err := LoadGlyphs(fsys, path.Join(path.Dir(catalogPath), file))
	if err != nil {
		bad("%v", err)
		return
	}
	c.glyphs, c.glyphsFile = g, file
}
