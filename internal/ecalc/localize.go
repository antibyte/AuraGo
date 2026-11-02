package ecalc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// The JSON shapes below are the catalog API (GET /api/ecalc/catalog?lang=xx). Every ordered collection is an
// array, and optional fields are written with their zero value so that clients never test for existence; only a
// variable's default and default_list are null when there is none.
type catalogJSON struct {
	Version    int            `json:"version"`
	Lang       string         `json:"lang"`
	Units      unitsJSON      `json:"units"`
	Categories []categoryJSON `json:"categories"`
	Items      []itemJSON     `json:"items"`
}

type unitsJSON struct {
	Prefixes []Prefix `json:"prefixes"`
	Dims     []Dim    `json:"dims"`
}

type categoryJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type varJSON struct {
	ID          string    `json:"id"`
	Dim         string    `json:"dim"`
	Label       string    `json:"label"`
	Name        string    `json:"name"`
	Default     *float64  `json:"default"`
	DefaultList []float64 `json:"default_list"`
	Unit        string    `json:"unit"`
	ESeries     bool      `json:"eseries"`
	List        bool      `json:"list"`
	Input       bool      `json:"input"`
}

type optionJSON struct {
	ID     string             `json:"id"`
	Name   string             `json:"name"`
	Consts map[string]float64 `json:"consts"`
}

type choiceJSON struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Default string       `json:"default"`
	Options []optionJSON `json:"options"`
}

type ruleJSON struct {
	Target string   `json:"target"`
	Alts   []string `json:"alts"`
}

type derivedJSON struct {
	ID    string   `json:"id"`
	Dim   string   `json:"dim"`
	Label string   `json:"label"`
	Name  string   `json:"name"`
	Alts  []string `json:"alts"`
	Unit  string   `json:"unit"`
}

type checkJSON struct {
	When string `json:"when"`
	Warn string `json:"warn"`
}

// itemJSON is a calculator. Phase 3 adds the other item kinds next to it. NameEN is the English name in every
// language (equal to Name for English), so that the app can find "voltage divider" in a German catalog.
type itemJSON struct {
	Kind      string        `json:"kind"`
	ID        string        `json:"id"`
	Category  string        `json:"category"`
	Name      string        `json:"name"`
	NameEN    string        `json:"name_en"`
	Doc       string        `json:"doc"`
	Tags      []string      `json:"tags"`
	Source    string        `json:"source"`
	Vars      []varJSON     `json:"vars"`
	Choices   []choiceJSON  `json:"choices"`
	DOF       int           `json:"dof"`
	Rules     []ruleJSON    `json:"rules"`
	Derived   []derivedJSON `json:"derived"`
	Checks    []checkJSON   `json:"checks"`
	Formula   []string      `json:"formula"`
	Schematic string        `json:"schematic"`
	Sets      [][]string    `json:"sets"`
}

type localized struct {
	body []byte
	etag string
}

// in returns the text for lang, or the English text when there is none.
func (t Text) in(lang string) string {
	if s := t[lang]; s != "" {
		return s
	}
	return t["en"]
}

// normalizeLang maps a language tag onto one of the 16 desktop locales: case and region are dropped (de-DE, de_DE
// and DE are German), the Norwegian variants nb and nn are no, anything else is English.
func normalizeLang(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	if i := strings.IndexAny(l, "-_"); i > 0 {
		l = l[:i]
	}
	switch {
	case l == "nb" || l == "nn":
		return "no"
	case isLocale(l):
		return l
	}
	return "en"
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func sources(alts []Expr) []string {
	out := make([]string, len(alts))
	for i, a := range alts {
		out[i] = a.Src
	}
	return out
}

// LocalizedJSON returns the catalog as JSON with every text resolved for lang (English where a language has no
// text), together with its ETag: "ecalc-" and the first 16 hex digits of the SHA-256 of the body, in quotes. The
// result is cached per normalized language; callers must not modify the returned body.
func (c *Catalog) LocalizedJSON(lang string) (body []byte, etag string, err error) {
	lang = normalizeLang(lang)
	c.locMu.Lock()
	defer c.locMu.Unlock()
	if e, ok := c.loc[lang]; ok {
		return e.body, e.etag, nil
	}
	if body, err = c.render(lang); err != nil {
		return nil, "", err
	}
	sum := sha256.Sum256(body)
	e := localized{body: body, etag: `"ecalc-` + hex.EncodeToString(sum[:8]) + `"`}
	if c.loc == nil {
		c.loc = map[string]localized{}
	}
	c.loc[lang] = e
	return e.body, e.etag, nil
}

func (c *Catalog) render(lang string) ([]byte, error) {
	out := catalogJSON{
		Version:    1,
		Lang:       lang,
		Units:      unitsJSON{Prefixes: orEmpty(c.Units.Prefixes), Dims: orEmpty(c.Units.Dims)},
		Categories: make([]categoryJSON, 0, len(c.Categories)),
		Items:      make([]itemJSON, 0, len(c.Calculators)),
	}
	for _, cat := range c.Categories {
		out.Categories = append(out.Categories, categoryJSON{ID: cat.ID, Name: cat.Name.in(lang)})
	}
	for _, calc := range c.Calculators {
		it := itemJSON{
			Kind: calc.Kind, ID: calc.ID, Category: calc.Category, Name: calc.Name.in(lang), NameEN: calc.Name.in("en"), Doc: calc.Doc.in(lang),
			Tags: orEmpty(calc.Tags), Source: calc.Source, DOF: calc.DOF, Formula: orEmpty(calc.Formula),
			Schematic: calc.Schematic, Sets: orEmpty(calc.Sets),
			Vars: []varJSON{}, Choices: []choiceJSON{}, Rules: []ruleJSON{}, Derived: []derivedJSON{}, Checks: []checkJSON{},
		}
		for _, v := range calc.Vars {
			it.Vars = append(it.Vars, varJSON{
				ID: v.ID, Dim: v.Dim, Label: v.Label, Name: v.Name.in(lang), Default: v.Default,
				DefaultList: v.DefaultList, Unit: v.Unit, ESeries: v.ESeries, List: v.List, Input: v.Input,
			})
		}
		for _, ch := range calc.Choices {
			cj := choiceJSON{ID: ch.ID, Name: ch.Name.in(lang), Default: ch.Default, Options: []optionJSON{}}
			for _, o := range ch.Options {
				consts := o.Consts
				if consts == nil {
					consts = map[string]float64{}
				}
				cj.Options = append(cj.Options, optionJSON{ID: o.ID, Name: o.Name.in(lang), Consts: consts})
			}
			it.Choices = append(it.Choices, cj)
		}
		for _, r := range calc.Rules {
			it.Rules = append(it.Rules, ruleJSON{Target: r.Target, Alts: sources(r.Alts)})
		}
		for _, d := range calc.Derived {
			it.Derived = append(it.Derived, derivedJSON{
				ID: d.ID, Dim: d.Dim, Label: d.Label, Name: d.Name.in(lang), Alts: sources(d.Alts), Unit: d.Unit,
			})
		}
		for _, ch := range calc.Checks {
			it.Checks = append(it.Checks, checkJSON{When: ch.When.Src, Warn: ch.Warn.in(lang)})
		}
		out.Items = append(out.Items, it)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false) // formulas contain < and >
	if err := enc.Encode(out); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
