package ecalc

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Code items: a catalog item of kind "code" (codes/*.yaml) names a registered code kind (resistor colour bands, SMD
// markings, ...) and carries the data of that kind. The engine only plumbs: the kind validates its data when the catalog
// loads (CodeKind.Load) and renders it for a language (CodeKind.JSON); the codes of the real kinds are registered by
// codes.go.

// Code is a catalog item of kind "code". Data is what the Load function of its kind returned.
type Code struct {
	ID       string
	Category string
	CodeKind string
	Name     Text
	Doc      Text
	Tags     []string
	Source   string
	Data     any
}

// CodeKind is the behaviour of one kind of code. Load validates the raw data of a code item, reporting problems with
// fail, and returns the typed value stored in Code.Data. JSON renders that value for a language in the shape the catalog
// JSON carries under "data".
type CodeKind struct {
	Load func(raw *yaml.Node, fail func(string, ...any)) any
	JSON func(data any, lang string) any
}

var codeKinds = struct {
	sync.RWMutex
	kinds map[string]CodeKind
}{kinds: map[string]CodeKind{}}

// RegisterCodeKind registers a code kind under an id (lower-case letters, digits and underscores). It is meant for init
// functions and panics on a bad id, a kind without Load or JSON, and a second registration of the same id.
func RegisterCodeKind(id string, k CodeKind) {
	if !idRe.MatchString(id) || k.Load == nil || k.JSON == nil {
		panic(fmt.Sprintf("ecalc: code kind %q needs an id of the form %s and both Load and JSON", id, idRe))
	}
	codeKinds.Lock()
	defer codeKinds.Unlock()
	if _, dup := codeKinds.kinds[id]; dup {
		panic(fmt.Sprintf("ecalc: code kind %q is registered twice", id))
	}
	codeKinds.kinds[id] = k
}

func codeKindOf(id string) (CodeKind, bool) {
	codeKinds.RLock()
	defer codeKinds.RUnlock()
	k, ok := codeKinds.kinds[id]
	return k, ok
}

// CodeKindIDs lists the registered code kinds, sorted.
func CodeKindIDs() []string {
	codeKinds.RLock()
	defer codeKinds.RUnlock()
	ids := make([]string, 0, len(codeKinds.kinds))
	for id := range codeKinds.kinds {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

// Code returns the code item with the given id, or nil.
func (c *Catalog) Code(id string) *Code { return c.codeByID[id] }

type rawCode struct {
	ID       string    `yaml:"id"`
	Kind     string    `yaml:"kind"`
	Category string    `yaml:"category"`
	Code     string    `yaml:"code"`
	Name     Text      `yaml:"name"`
	Doc      Text      `yaml:"doc"`
	Tags     []string  `yaml:"tags"`
	Source   string    `yaml:"source"`
	Data     yaml.Node `yaml:"data"`
}

// codes reads codes/*.yaml (sorted) into the catalog.
func (l *loader) codes(fsys fs.FS, cat *Catalog) error {
	files, err := fs.Glob(fsys, "codes/*.yaml")
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		raws, ok := readItems[rawCode](l, fsys, file)
		if !ok {
			continue
		}
		for i := range raws {
			if c := l.code(file, &raws[i]); c != nil {
				cat.Codes = append(cat.Codes, c)
				cat.codeByID[c.ID] = c
				cat.kinds[c.ID] = "code"
			}
		}
	}
	return nil
}

// code builds and validates one code item; it returns nil when something is wrong.
func (l *loader) code(file string, r *rawCode) *Code {
	before := len(l.errs)
	label := r.ID
	if label == "" {
		label = "(item without id)"
	}
	fail := func(format string, args ...any) {
		l.errs = append(l.errs, fmt.Errorf("%s: %s: %s", file, label, fmt.Sprintf(format, args...)))
	}
	text := func(what string, t Text, required []string) {
		l.checkText(fmt.Sprintf("%s: %s: %s", file, label, what), t, required)
	}
	c := &Code{ID: r.ID, Category: r.Category, CodeKind: r.Code, Name: r.Name, Doc: r.Doc, Tags: r.Tags, Source: r.Source}
	l.claimID(r.ID, file, fail)
	if r.Kind != "code" {
		fail("kind must be code in codes/, got %q", r.Kind)
	}
	if r.Category != codesCategory {
		fail("category must be %s, got %q", codesCategory, r.Category)
	}
	l.commonFields(fail, text, r.Name, r.Doc, r.Tags, r.Source)
	if kind, ok := codeKindOf(r.Code); !ok {
		fail("code %q is not a registered code kind (%s)", r.Code, strings.Join(CodeKindIDs(), ", "))
	} else {
		c.Data = kind.Load(&r.Data, fail)
	}
	if len(l.errs) > before {
		return nil
	}
	return c
}

// codeJSON is a code item of the catalog JSON; data is what the JSON function of its kind renders.
type codeJSON struct {
	Kind     string   `json:"kind"`
	ID       string   `json:"id"`
	Category string   `json:"category"`
	Code     string   `json:"code"`
	Name     string   `json:"name"`
	NameEN   string   `json:"name_en"`
	Doc      string   `json:"doc"`
	Tags     []string `json:"tags"`
	Source   string   `json:"source"`
	Data     any      `json:"data"`
}

func (c *Code) json(lang string) codeJSON {
	out := codeJSON{
		Kind: "code", ID: c.ID, Category: c.Category, Code: c.CodeKind, Name: c.Name.in(lang), NameEN: c.Name.in("en"),
		Doc: c.Doc.in(lang), Tags: orEmpty(c.Tags), Source: c.Source, Data: map[string]any{},
	}
	if kind, ok := codeKindOf(c.CodeKind); ok {
		if d := kind.JSON(c.Data, lang); d != nil {
			out.Data = d
		}
	}
	return out
}
