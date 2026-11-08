package storymaker

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
)

// CatalogSchema is the schema string of catalog.json.
const CatalogSchema = "storymaker-catalog/2"

const maxCatalogBytes = 16 << 20

// kindPrefixes maps every entry kind to its ID prefix. Palettes live in the manifest's
// palettes list and fonts in its fonts list; both are indexed here as well.
var kindPrefixes = map[string]string{
	"character":   "char.",
	"prop":        "prop.",
	"environment": "env.",
	"pose":        "pose.",
	"gesture":     "gesture.",
	"expression":  "expr.",
	"mouth":       "mouth.",
	"cycle":       "cycle.",
	"text_style":  "text.",
	"transition":  "trans.",
	"palette":     "palette.",
	"sfx":         "sfx.",
}

// enumKinds are the kinds whose name is the bare name used in story fields.
var enumKinds = nameSet("pose", "gesture", "expression", "mouth", "cycle", "text_style", "transition", "palette")

// kindAliases lets callers pass an ID prefix or a plural instead of the kind.
var kindAliases = map[string]string{
	"char": "character", "chars": "character", "characters": "character", "people": "character",
	"props": "prop", "env": "environment", "envs": "environment", "environments": "environment", "background": "environment",
	"poses": "pose", "gestures": "gesture", "expr": "expression", "expressions": "expression",
	"mouths": "mouth", "cycles": "cycle", "text": "text_style", "text_styles": "text_style",
	"trans": "transition", "transitions": "transition", "palettes": "palette", "fonts": "font",
}

// NormalizeKind returns the canonical catalog kind for a kind, alias or ID prefix, or "".
func NormalizeKind(kind string) string {
	k := strings.ToLower(strings.TrimSpace(kind))
	k = strings.TrimSuffix(k, ".")
	if _, ok := kindPrefixes[k]; ok || k == "font" {
		return k
	}
	return kindAliases[k]
}

// Spot is a named standing position of an environment, in percent of the design frame.
type Spot struct {
	X     float64  `json:"x"`
	Y     float64  `json:"y"` // feet line
	Depth *float64 `json:"depth,omitempty"`
	Face  string   `json:"face,omitempty"` // left | right
}

// Floor describes how actor size changes with depth.
type Floor struct {
	YNear     float64 `json:"y_near"`
	YFar      float64 `json:"y_far"`
	ScaleNear float64 `json:"scale_near"`
	ScaleFar  float64 `json:"scale_far"`
}

// AspectOverride adjusts an environment for 9:16 or 1:1.
type AspectOverride struct {
	FocusX float64         `json:"focus_x"`
	Spots  map[string]Spot `json:"spots,omitempty"`
}

// SFXTake is one rendered take of a sound effect, with its measured values.
type SFXTake struct {
	File         string  `json:"file"`
	DurMS        float64 `json:"dur_ms"`
	OnsetMS      float64 `json:"onset_ms"`
	PeakMS       float64 `json:"peak_ms"`
	ImpactMS     float64 `json:"impact_ms"`
	L50LUFS      float64 `json:"l50_lufs"`
	TruePeakDBTP float64 `json:"true_peak_dbtp"`
}

// SFXHit is the level of a sound above the bed and the dip it makes in the bed.
type SFXHit struct {
	OverDB float64 `json:"over_db"`
	DipDB  float64 `json:"dip_db"`
}

// SFXPattern makes a sound repeat as spaced events, like typing keys.
type SFXPattern struct {
	SpacingMS         [2]float64 `json:"spacing_ms"`
	DefaultDurationMS float64    `json:"default_duration_ms"`
}

// SFXSource records where a sound comes from.
type SFXSource struct {
	Kind  string `json:"kind"`
	Basis string `json:"basis"`
	URL   string `json:"url,omitempty"`
}

// Entry is one catalog entry. Common fields and the environment data are typed; Raw keeps
// the complete entry for describe answers (kind-specific fields differ and some names
// clash between kinds, for example size).
type Entry struct {
	ID           string                    `json:"id"`
	Kind         string                    `json:"kind"`
	Name         string                    `json:"name"`
	NameKey      string                    `json:"name_key"`
	Tags         []string                  `json:"tags"`
	Thumb        string                    `json:"thumb"`
	UsageExample json.RawMessage           `json:"usage_example"`
	File         string                    `json:"file,omitempty"`
	Params       map[string]string         `json:"params,omitempty"`
	Spots        map[string]Spot           `json:"spots,omitempty"`
	Floor        *Floor                    `json:"floor,omitempty"`
	Variants     []string                  `json:"variants,omitempty"`
	Aspects      map[string]AspectOverride `json:"aspects,omitempty"`
	Takes        []SFXTake                 `json:"takes,omitempty"`
	ImpactMS     float64                   `json:"impact_ms,omitempty"` // nominal; the player and the mix read the take's
	Align        string                    `json:"align,omitempty"`     // "peak" | "onset"
	Hit          *SFXHit                   `json:"hit,omitempty"`
	Pattern      *SFXPattern               `json:"pattern,omitempty"`
	Source       *SFXSource                `json:"source,omitempty"`
	raw          json.RawMessage
}

// MarshalJSON writes the entry exactly as the catalog has it.
func (e Entry) MarshalJSON() ([]byte, error) {
	if len(e.raw) > 0 {
		return e.raw, nil
	}
	type plain Entry
	return json.Marshal(plain(e))
}

// Raw returns the complete catalog JSON of the entry.
func (e Entry) Raw() json.RawMessage { return e.raw }

// Font is one font face of the catalog.
type Font struct {
	Name    string `json:"name"`
	Family  string `json:"family"`
	Weight  int    `json:"weight"`
	File    string `json:"file"` // /fonts/<file> in the ui tree
	Licence string `json:"licence"`
}

// Palette is a named colour palette.
type Palette struct {
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	NameKey string            `json:"name_key"`
	Tokens  map[string]string `json:"tokens"`
}

// Stage is the design frame and its safe area (percent).
type Stage struct {
	Width  int `json:"width"`
	Height int `json:"height"`
	Safe   struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
		W float64 `json:"w"`
		H float64 `json:"h"`
	} `json:"safe"`
}

// RigRef names the rig file.
type RigRef struct {
	ID   string `json:"id"`
	File string `json:"file"`
}

// Catalog is the loaded catalog.json index. It is immutable after LoadCatalog, so it can be
// shared between goroutines; callers must not modify the exported slices and maps.
type Catalog struct {
	Schema   string
	Stage    Stage
	Rig      RigRef
	Fonts    []Font
	Palettes []Palette

	version  string
	entries  []Entry
	byID     map[string]int
	files    []CatalogFile
	manifest []byte

	glyphs     *Glyphs // nil when the manifest names no glyphs file
	glyphsFile string
}

type catalogFile struct {
	Schema   string            `json:"schema"`
	Version  string            `json:"version"`
	Stage    Stage             `json:"stage"`
	Rig      RigRef            `json:"rig"`
	Glyphs   *glyphsRef        `json:"glyphs"`
	Fonts    []Font            `json:"fonts"`
	Palettes []Palette         `json:"palettes"`
	Entries  []json.RawMessage `json:"entries"`
	Files    []CatalogFile     `json:"files"`
}

var paletteTokenNames = []string{"bg", "bg2", "fg", "accent", "accent2", "text", "outline"}

var characterParamNames = []string{"body", "skin", "hair", "hair_color", "eyes", "brows", "facial_hair", "glasses", "outfit", "accessory"}

// LoadCatalog reads and checks the manifest at path (for the ui tree: "img/storymaker/catalog.json").
// It validates the manifest only: the schema string, kinds, duplicate IDs, the ID prefix of each
// kind, the name keys and the required fields of each kind. It does not open the files that
// entries reference; VerifyFiles does that. It does read the glyphs file the manifest names, from the same folder.
func LoadCatalog(fsys fs.FS, path string) (*Catalog, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, fmt.Errorf("storymaker: open catalog: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxCatalogBytes+1))
	if err != nil {
		return nil, fmt.Errorf("storymaker: read catalog: %w", err)
	}
	if len(data) > maxCatalogBytes {
		return nil, fmt.Errorf("storymaker: catalog %s is larger than %d bytes", path, maxCatalogBytes)
	}
	var m catalogFile
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("storymaker: parse catalog %s: %w", path, err)
	}

	var problems []string
	bad := func(format string, args ...any) { problems = append(problems, fmt.Sprintf(format, args...)) }
	if m.Schema != CatalogSchema && m.Schema != "storymaker-catalog/1" {
		bad("schema is %q, want %q", m.Schema, CatalogSchema)
	}
	if strings.TrimSpace(m.Version) == "" || len(m.Version) > 64 {
		bad("version must be a non-empty string of at most 64 characters")
	}
	if m.Stage.Width <= 0 || m.Stage.Height <= 0 || m.Stage.Safe.W <= 0 || m.Stage.Safe.H <= 0 {
		bad("stage needs a positive width, height and safe area")
	}
	if m.Rig.ID == "" || !safeRelPath(m.Rig.File) {
		bad("rig needs an id and a relative file path")
	}
	c := &Catalog{Schema: m.Schema, Stage: m.Stage, Rig: m.Rig, version: m.Version, byID: map[string]int{}, files: m.Files, manifest: data}

	hasDefaultFont := false
	for i, ft := range m.Fonts {
		switch {
		case ft.Name == "" || ft.Family == "" || ft.Weight <= 0 || ft.Licence == "":
			bad("fonts[%d] needs name, family, weight and licence", i)
		case !strings.HasPrefix(ft.File, "/fonts/") || strings.Contains(ft.File, ".."):
			bad("fonts[%d] file must be a /fonts/ path", i)
		}
		hasDefaultFont = hasDefaultFont || ft.Name == "default"
	}
	if !hasDefaultFont {
		bad("fonts needs a font named default (story style.font defaults to it)")
	}
	c.Fonts = m.Fonts

	seen := map[string]bool{}
	duplicate := func(id string) bool {
		if seen[id] {
			bad("duplicate id %s", id)
			return true
		}
		seen[id] = true
		return false
	}
	add := func(e Entry) {
		c.byID[e.ID] = len(c.entries)
		c.entries = append(c.entries, e)
	}

	for i, p := range m.Palettes {
		switch {
		case !strings.HasPrefix(p.ID, "palette.") || p.Name != strings.TrimPrefix(p.ID, "palette."):
			bad("palettes[%d] id %q must be palette.<name>", i, p.ID)
		case p.NameKey != catalogNameKey(p.ID):
			bad("palette %s name_key must be %s", p.ID, catalogNameKey(p.ID))
		default:
			for _, tok := range paletteTokenNames {
				if !strings.HasPrefix(p.Tokens[tok], "#") {
					bad("palette %s needs a hex colour for token %s", p.ID, tok)
				}
			}
		}
		if duplicate(p.ID) {
			continue
		}
		raw, _ := json.Marshal(p)
		add(Entry{ID: p.ID, Kind: "palette", Name: p.Name, NameKey: p.NameKey, raw: raw})
	}

	for i, raw := range m.Entries {
		var e Entry
		if err := json.Unmarshal(raw, &e); err != nil {
			bad("entries[%d]: %v", i, err)
			continue
		}
		e.raw = append(json.RawMessage(nil), raw...)
		if duplicate(e.ID) || !c.checkEntry(&e, raw, i, bad) {
			continue
		}
		add(e)
	}

	if m.Glyphs != nil {
		c.loadGlyphs(fsys, path, m.Glyphs.File, bad)
	}
	c.checkFiles(bad)

	if len(problems) > 0 {
		shown := problems
		if len(shown) > 12 {
			shown = append(append([]string(nil), shown[:12]...), fmt.Sprintf("... and %d more", len(problems)-12))
		}
		return nil, fmt.Errorf("storymaker: catalog %s is invalid:\n - %s", path, strings.Join(shown, "\n - "))
	}
	return c, nil
}

func catalogNameKey(id string) string {
	return "storymaker.catalog." + strings.ReplaceAll(id, ".", "_")
}

// safeRelPath reports a non-empty slash path that stays inside its directory.
func safeRelPath(p string) bool {
	if p == "" || strings.ContainsAny(p, "\\:\x00") || strings.HasPrefix(p, "/") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// checkEntry validates the common and kind-specific manifest fields of one entry.
func (c *Catalog) checkEntry(e *Entry, raw json.RawMessage, i int, bad func(string, ...any)) bool {
	label := fmt.Sprintf("entries[%d]", i)
	if e.ID != "" {
		label = e.ID
	}
	n := 0
	fail := func(format string, args ...any) {
		n++
		bad("%s: "+format, append([]any{label}, args...)...)
	}
	prefix, known := kindPrefixes[e.Kind]
	switch {
	case e.Kind == "palette":
		fail("palettes belong to the palettes list, not to entries")
		return false
	case !known:
		fail("unknown kind %q", e.Kind)
		return false
	case !strings.HasPrefix(e.ID, prefix) || len(e.ID) == len(prefix):
		fail("id must start with %s for kind %s", prefix, e.Kind)
		return false
	}
	suffix := strings.TrimPrefix(e.ID, prefix)
	if enumKinds[e.Kind] && e.Name != suffix {
		fail("name must be %q (the id without its prefix)", suffix)
	}
	if !enumKinds[e.Kind] && e.Name == "" {
		fail("name is required")
	}
	if e.NameKey != catalogNameKey(e.ID) {
		fail("name_key must be %s", catalogNameKey(e.ID))
	}
	// An sfx entry has no image, so its thumb is empty.
	if e.Kind != "sfx" && (!safeRelPath(e.Thumb) || (c.Schema == "storymaker-catalog/1" && !strings.HasSuffix(e.Thumb, ".svg")) || (c.Schema == CatalogSchema && !strings.HasSuffix(e.Thumb, ".png") && !strings.HasSuffix(e.Thumb, ".webp"))) {
		fail("thumb must name a relative catalog image")
	}
	if len(e.UsageExample) == 0 || e.UsageExample[0] != '{' {
		fail("usage_example must be an object")
	}
	inRange := func(what string, v float64) {
		if v < 0 || v > 100 {
			fail("%s must be between 0 and 100", what)
		}
	}

	switch e.Kind {
	case "character":
		for _, k := range characterParamNames {
			if strings.TrimSpace(e.Params[k]) == "" {
				fail("params.%s is required", k)
			}
		}
	case "prop":
		var p struct {
			Size *struct {
				H float64 `json:"h"`
			} `json:"size"`
			Anchor *struct {
				X *float64 `json:"x"`
				Y *float64 `json:"y"`
			} `json:"anchor"`
		}
		_ = json.Unmarshal(raw, &p)
		if !safeRelPath(e.File) {
			fail("file must be a relative path")
		}
		if p.Size == nil || p.Size.H <= 0 {
			fail("size.h must be a positive number")
		}
		if p.Anchor == nil || p.Anchor.X == nil || p.Anchor.Y == nil {
			fail("anchor needs x and y")
		}
	case "environment":
		if !safeRelPath(e.File) {
			fail("file must be a relative path")
		}
		for _, base := range []string{"far-left", "left", "center", "right", "far-right"} {
			if _, ok := e.Spots[base]; !ok {
				fail("spots.%s is required", base)
			}
		}
		for name, s := range e.Spots {
			if !validSpotName(name) {
				fail("spot name %q must be lowercase letters, digits and dashes", name)
			}
			inRange("spot "+name+" x", s.X)
			inRange("spot "+name+" y", s.Y)
			if s.Face != "" && s.Face != "left" && s.Face != "right" {
				fail("spot %s face must be left or right", name)
			}
		}
		if e.Floor == nil || e.Floor.ScaleNear <= 0 || e.Floor.ScaleFar <= 0 {
			fail("floor needs y_near, y_far, scale_near and scale_far")
		}
		if len(e.Variants) == 0 {
			fail("variants is required")
		}
		for _, v := range e.Variants {
			if v != "day" && v != "night" {
				fail("variant %q must be day or night", v)
			}
		}
		for asp, ov := range e.Aspects {
			if asp != "9:16" && asp != "1:1" {
				fail("aspects key %q must be 9:16 or 1:1", asp)
			}
			for name, s := range ov.Spots {
				inRange("aspect "+asp+" spot "+name+" x", s.X)
				inRange("aspect "+asp+" spot "+name+" y", s.Y)
			}
		}
	case "sfx":
		if !safeRelPath(e.File) {
			fail("file must be a relative path")
		}
		if len(e.Takes) < 2 {
			fail("at least two takes are required")
		} else if e.File != e.Takes[0].File {
			fail("file must equal takes[0].file")
		}
		for j, take := range e.Takes {
			if !safeRelPath(take.File) {
				fail("takes[%d].file must be a relative path", j)
			}
		}
		if e.Align != "peak" && e.Align != "onset" {
			fail("align must be peak or onset")
		}
		if e.Source == nil {
			fail("source is required")
		}
	case "pose":
		var p struct {
			Bones map[string]float64 `json:"bones"`
		}
		_ = json.Unmarshal(raw, &p)
		if p.Bones == nil {
			fail("bones is required (use {} for the rest pose)")
		}
	case "gesture":
		var g struct {
			Keys     []json.RawMessage `json:"keys"`
			Duration float64           `json:"duration"`
		}
		_ = json.Unmarshal(raw, &g)
		if len(g.Keys) < 2 || g.Duration <= 0 {
			fail("keys (at least two) and a positive duration are required")
		}
	case "expression":
		var x struct {
			Eyes  string `json:"eyes"`
			Brows string `json:"brows"`
			Mouth string `json:"mouth"`
		}
		_ = json.Unmarshal(raw, &x)
		if x.Eyes == "" || x.Brows == "" || x.Mouth == "" {
			fail("eyes, brows and mouth are required")
		}
	case "mouth":
		var m struct {
			Shape string `json:"shape"`
		}
		_ = json.Unmarshal(raw, &m)
		if !containsName(MouthNames, m.Shape) {
			fail("shape must be one of %s", strings.Join(MouthNames, ", "))
		}
	case "cycle":
		var cy struct {
			Period float64           `json:"period"`
			Keys   []json.RawMessage `json:"keys"`
		}
		_ = json.Unmarshal(raw, &cy)
		if cy.Period <= 0 || len(cy.Keys) < 2 {
			fail("a positive period and at least two keys are required")
		}
	case "text_style":
		var t struct {
			Font     string  `json:"font"`
			Size     float64 `json:"size"`
			Weight   int     `json:"weight"`
			Color    string  `json:"color"`
			Align    string  `json:"align"`
			MaxLines int     `json:"max_lines"`
		}
		_ = json.Unmarshal(raw, &t)
		if t.Font == "" || t.Size <= 0 || t.Weight <= 0 || t.Color == "" || t.MaxLines <= 0 ||
			(t.Align != "left" && t.Align != "center" && t.Align != "right") {
			fail("font, size, weight, color, align (left, center or right) and max_lines are required")
		}
	case "transition":
		var t struct {
			Duration *float64 `json:"duration"`
		}
		_ = json.Unmarshal(raw, &t)
		if t.Duration == nil || *t.Duration < 0 {
			fail("duration (seconds, 0 or more) is required")
		}
	}
	return n == 0
}

func validSpotName(s string) bool {
	if s == "" || len(s) > 32 {
		return false
	}
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-' && i > 0 {
			continue
		}
		return false
	}
	return true
}

// VerifyFiles checks that every file the catalog references exists: the rig, entry files and
// thumbnails (relative to dir) and the font files (absolute /fonts/ paths relative to the root
// of fsys). fsys is the ui tree and dir the catalog directory in it ("img/storymaker").
func (c *Catalog) VerifyFiles(fsys fs.FS, dir string) error {
	if c.Schema == CatalogSchema {
		return c.verifyDelivery(fsys, dir)
	}
	var missing []string
	check := func(p string) {
		if !fs.ValidPath(p) {
			missing = append(missing, p+" (invalid path)")
			return
		}
		if st, err := fs.Stat(fsys, p); err != nil || st.IsDir() {
			missing = append(missing, p)
		}
	}
	check(path.Join(dir, c.Rig.File))
	for _, e := range c.entries {
		if e.Kind == "palette" {
			continue
		}
		if e.File != "" {
			check(path.Join(dir, e.File))
		}
		for _, take := range e.Takes {
			if take.File != e.File {
				check(path.Join(dir, take.File))
			}
		}
		if e.Thumb != "" {
			check(path.Join(dir, e.Thumb))
		}
	}
	for _, f := range c.Fonts {
		check(strings.TrimPrefix(f.File, "/"))
	}
	if len(missing) == 0 {
		return nil
	}
	shown := missing
	if len(shown) > 12 {
		shown = append(append([]string(nil), shown[:12]...), fmt.Sprintf("... and %d more", len(missing)-12))
	}
	return fmt.Errorf("storymaker: catalog references %d missing files:\n - %s", len(missing), strings.Join(shown, "\n - "))
}

// Version is the catalog version string (a hash of the output files).
func (c *Catalog) Version() string { return c.version }

// Entries returns all entries in catalog order (palettes first).
func (c *Catalog) Entries() []Entry { return append([]Entry(nil), c.entries...) }

// Lookup finds an entry by its full ID.
func (c *Catalog) Lookup(id string) (Entry, bool) {
	if c == nil {
		return Entry{}, false
	}
	i, ok := c.byID[id]
	if !ok {
		return Entry{}, false
	}
	return c.entries[i], true
}

// Names returns the bare names of an enum-like kind (pose, gesture, expression, mouth, cycle,
// text_style, transition, palette) or the font names for "font". For the other kinds it returns
// the IDs. The order is the catalog order. An unknown kind returns nil.
func (c *Catalog) Names(kind string) []string {
	if c == nil {
		return nil
	}
	kind = NormalizeKind(kind)
	if kind == "font" {
		seen := map[string]bool{}
		var out []string
		for _, f := range c.Fonts {
			if !seen[f.Name] {
				seen[f.Name] = true
				out = append(out, f.Name)
			}
		}
		return out
	}
	var out []string
	for _, e := range c.entries {
		if e.Kind != kind {
			continue
		}
		if enumKinds[kind] {
			out = append(out, e.Name)
		} else {
			out = append(out, e.ID)
		}
	}
	return out
}

// HasName reports whether an enum-like kind has the bare name.
func (c *Catalog) HasName(kind, name string) bool {
	if c == nil {
		return false
	}
	prefix, ok := kindPrefixes[NormalizeKind(kind)]
	if !ok {
		return false
	}
	_, found := c.byID[prefix+name]
	return found
}

const (
	defaultSearchLimit = 6
	maxSearchLimit     = 12
)

// Search finds entries by terms over their id, name and tags. kind restricts the result (a kind,
// alias or ID prefix; an unknown kind finds nothing). limit defaults to 6 and is capped at 12.
// Without terms it lists the entries of the kind in catalog order. Results are sorted by score,
// then by catalog order.
func (c *Catalog) Search(query, kind string, limit int) []Entry {
	if c == nil {
		return nil
	}
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	limit = min(limit, maxSearchLimit)
	wanted := strings.TrimSpace(kind)
	kind = NormalizeKind(kind)
	if wanted != "" && kind == "" {
		return nil
	}
	terms := strings.Fields(strings.ToLower(query))
	type scored struct {
		e     Entry
		score int
		order int
	}
	var hits []scored
	for i, e := range c.entries {
		if kind != "" && e.Kind != kind {
			continue
		}
		score := 1
		if len(terms) > 0 {
			score = 0
			for _, t := range terms {
				score += termScore(e, t)
			}
			if score == 0 {
				continue
			}
		}
		hits = append(hits, scored{e, score, i})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].order < hits[j].order
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	out := make([]Entry, len(hits))
	for i, h := range hits {
		out[i] = h.e
	}
	return out
}

// termScore scores one lower-case term against an entry: exact id or name, tag, then substrings.
func termScore(e Entry, term string) int {
	best := 0
	try := func(t string) {
		s := 0
		lowID, lowName := strings.ToLower(e.ID), strings.ToLower(e.Name)
		switch {
		case lowID == t:
			s = 10
		case lowName == t:
			s = 8
		}
		if s == 0 {
			for _, tag := range e.Tags {
				lt := strings.ToLower(tag)
				if lt == t {
					s = max(s, 4)
				} else if strings.Contains(lt, t) {
					s = max(s, 2)
				}
			}
			if strings.Contains(lowName, t) {
				s = max(s, 3)
			}
			if strings.Contains(lowID, t) {
				s = max(s, 2)
			}
		}
		best = max(best, s)
	}
	try(term)
	if best == 0 && len(term) > 3 && strings.HasSuffix(term, "s") {
		try(strings.TrimSuffix(term, "s"))
	}
	return best
}

// defaultBackgroundSpots are the spots of a project or library background (my.bg.*, lib.bg.*).
var defaultBackgroundSpots = map[string]Spot{
	"left": {X: 30, Y: 88}, "center": {X: 50, Y: 88}, "right": {X: 70, Y: 88},
}

// isBackgroundAsset reports a project or library background ID.
func isBackgroundAsset(id string) bool {
	return strings.HasPrefix(id, "my.bg.") || strings.HasPrefix(id, "lib.bg.")
}

// Spots returns the named spots of an environment for an aspect ("16:9", "9:16", "1:1"); the
// aspect overrides replace single spots. A project or library background (my.bg.*, lib.bg.*)
// has the default spots left, center and right. It reports false for an unknown environment.
func (c *Catalog) Spots(envID, aspect string) (map[string]Spot, bool) {
	if isBackgroundAsset(envID) {
		out := make(map[string]Spot, len(defaultBackgroundSpots))
		for k, v := range defaultBackgroundSpots {
			out[k] = v
		}
		return out, true
	}
	e, ok := c.Lookup(envID)
	if !ok || e.Kind != "environment" {
		return nil, false
	}
	out := make(map[string]Spot, len(e.Spots))
	for k, v := range e.Spots {
		out[k] = v
	}
	if ov, ok := e.Aspects[aspect]; ok {
		for k, v := range ov.Spots {
			out[k] = v
		}
	}
	return out, true
}

// SpotNames lists the spot names of an environment: the five base spots first, then the
// environment's own spots in alphabetical order.
func (c *Catalog) SpotNames(envID string) []string {
	spots, ok := c.Spots(envID, "16:9")
	if !ok {
		return nil
	}
	base := []string{"far-left", "left", "center", "right", "far-right"}
	var out, rest []string
	for _, b := range base {
		if _, ok := spots[b]; ok {
			out = append(out, b)
		}
	}
	for name := range spots {
		if !containsName(base, name) {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	return append(out, rest...)
}

// FocusX is the horizontal focus (percent) of an environment for an aspect; 50 when unknown.
func (c *Catalog) FocusX(envID, aspect string) float64 {
	if e, ok := c.Lookup(envID); ok {
		if ov, ok := e.Aspects[aspect]; ok {
			return ov.FocusX
		}
	}
	return 50
}

// Variants lists the variants (day, night) an environment supports.
func (c *Catalog) Variants(envID string) []string {
	if isBackgroundAsset(envID) {
		return []string{"day", "night"}
	}
	if e, ok := c.Lookup(envID); ok {
		return e.Variants
	}
	return nil
}
