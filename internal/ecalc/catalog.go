package ecalc

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

//go:embed catalog
var catalogFS embed.FS

// Locales are the 16 desktop locales. Every name in the catalog is written in all of them; docs and warnings need
// English and German and fall back to English for the rest.
var Locales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

var docLocales = []string{"en", "de"}

var (
	idRe        = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	schematicRe = regexp.MustCompile(`^[a-z0-9_]*$`)
)

// Text is a localized string: language -> text.
type Text map[string]string

// Category is one entry of the category rail.
type Category struct {
	ID   string
	Name Text
}

// Expr is one parsed expression with the names it uses.
type Expr struct {
	Src  string
	Node *Node
	Deps []string
}

// Var is a primary variable of a calculator.
type Var struct {
	ID          string
	Dim         string
	Label       string
	Name        Text
	Default     *float64  // SI value, nil when the variable has no default
	DefaultList []float64 // SI values of a list variable's default
	Unit        string    // default display unit id, "" = automatic
	Units       []string  // display units the page offers ("" = base symbol with automatic prefixes); empty = all
	ESeries     bool
	List        bool
	Input       bool // can be an input (false: only ever computed)

	dim *Dim
}

// ChoiceOption is one option of a choice; its consts become known names while it is selected. Ignore lists the
// primary variables this option does not use (the page dims them); the solver does not change. Refs defines table
// aliases: while the option is selected, the alias names the table or column in the table functions (every option of a
// choice defines the same aliases, so the choice selects the table or the column).
type ChoiceOption struct {
	ID     string
	Name   Text
	Consts map[string]float64
	Ignore []string
	Refs   map[string]string
}

// Choice is a selector (for example the waveform) whose selected option supplies constants to the rules.
type Choice struct {
	ID      string
	Name    Text
	Default string
	Options []ChoiceOption
}

// Rule computes one variable. Alts are alternatives; the first one whose names are all known is applied.
type Rule struct {
	Target string
	Alts   []Expr
}

// Derived is a quantity computed after (and, through the rules, during) solving.
type Derived struct {
	ID    string
	Dim   string
	Label string
	Name  Text
	Alts  []Expr
	Unit  string   // fixed display unit id, "" = automatic
	Units []string // display units the page offers ("" = base symbol with automatic prefixes); empty = all

	dim *Dim
}

// Chart is the line chart of a calculator: the quantity Y (a rule target or a derived quantity) over the variable X
// from 0 to To, drawn with Samples points. To is an expression that does not depend on X.
type Chart struct {
	X, Y    string
	To      Expr
	Samples int
}

// Chart limits: a chart has between ChartMinSamples and ChartMaxSamples points, ChartDefaultSamples when unset.
const (
	ChartMinSamples     = 11
	ChartMaxSamples     = 401
	ChartDefaultSamples = 101
)

// finderCombine is the finder of the standard-value combiner (see Combine).
const finderCombine = "combine"

// Check shows Warn when When evaluates to a non-zero number.
type Check struct {
	When Expr
	Warn Text
}

// Calculator is a catalog item of kind "calculator".
type Calculator struct {
	ID         string
	Kind       string
	Category   string
	Name       Text
	Doc        Text
	Tags       []string
	Source     string
	Vars       []Var
	Choices    []Choice
	DOF        int
	Rules      []Rule
	Derived    []Derived
	Checks     []Check
	Formula    []string
	FormulaAST []*Node
	Schematic  string
	Finder     string // "" or "combine": the page shows the standard-value combinations of the single resistance
	Chart      *Chart // nil when the calculator has no chart
	// Sets lists every combination of dof input variables from which all variables can be computed, in declaration
	// order (see SolvableSets). Load fills it in.
	Sets [][]string

	varByID     map[string]*Var
	aliasChoice map[string]int    // table alias -> index of the choice that defines it
	tables      map[string]*Table // the tables of the catalog, set by Load
}

// Var returns the primary variable with the given id, or nil.
func (c *Calculator) Var(id string) *Var { return c.varByID[id] }

// Catalog is the validated ECalc catalog: calculators, tables and codes, each in catalog order (file order, files
// sorted).
type Catalog struct {
	Units       *Units
	Categories  []Category
	Calculators []*Calculator
	Tables      []*Table
	Codes       []*Code

	byID      map[string]*Calculator
	tableByID map[string]*Table
	codeByID  map[string]*Code
	kinds     map[string]string // item id -> "calculator", "table" or "code"

	locMu sync.Mutex
	loc   map[string]localized // LocalizedJSON cache, by normalized language
}

// Calculator returns the calculator with the given id, or nil.
func (c *Catalog) Calculator(id string) *Calculator { return c.byID[id] }

var (
	defaultOnce sync.Once
	defaultCat  *Catalog
	defaultErr  error
)

// Default returns the embedded catalog, loaded and validated once.
func Default() (*Catalog, error) {
	defaultOnce.Do(func() {
		sub, err := fs.Sub(catalogFS, "catalog")
		if err != nil {
			defaultErr = err
			return
		}
		defaultCat, defaultErr = Load(sub)
	})
	return defaultCat, defaultErr
}

type rawCheck struct {
	When string `yaml:"when"`
	Warn Text   `yaml:"warn"`
}

type rawCalc struct {
	ID        string     `yaml:"id"`
	Kind      string     `yaml:"kind"`
	Category  string     `yaml:"category"`
	Name      Text       `yaml:"name"`
	Doc       Text       `yaml:"doc"`
	Tags      []string   `yaml:"tags"`
	Source    string     `yaml:"source"`
	Vars      yaml.Node  `yaml:"vars"`
	Choices   yaml.Node  `yaml:"choices"`
	DOF       int        `yaml:"dof"`
	Rules     yaml.Node  `yaml:"rules"`
	Derived   yaml.Node  `yaml:"derived"`
	Checks    []rawCheck `yaml:"checks"`
	Formula   []string   `yaml:"formula"`
	Schematic string     `yaml:"schematic"`
	Finder    string     `yaml:"finder"`
	Chart     *rawChart  `yaml:"chart"`
}

type rawChart struct {
	X       string `yaml:"x"`
	Y       string `yaml:"y"`
	To      string `yaml:"to"`
	Samples int    `yaml:"samples"`
}

type rawVar struct {
	Dim     string    `yaml:"dim"`
	Label   string    `yaml:"label"`
	Name    Text      `yaml:"name"`
	Default yaml.Node `yaml:"default"`
	Unit    string    `yaml:"unit"`
	Units   []string  `yaml:"units"`
	ESeries bool      `yaml:"eseries"`
	List    bool      `yaml:"list"`
	Input   *bool     `yaml:"input"`
}

type rawChoice struct {
	Name    Text      `yaml:"name"`
	Default string    `yaml:"default"`
	Options yaml.Node `yaml:"options"`
}

type rawOption struct {
	Name   Text               `yaml:"name"`
	Consts map[string]float64 `yaml:"consts"`
	Ignore []string           `yaml:"ignore"`
	Refs   map[string]string  `yaml:"refs"`
}

type rawDerived struct {
	Dim   string    `yaml:"dim"`
	Label string    `yaml:"label"`
	Name  Text      `yaml:"name"`
	Expr  yaml.Node `yaml:"expr"`
	Unit  string    `yaml:"unit"`
	Units []string  `yaml:"units"`
}

type loader struct {
	units *Units
	cats  map[string]bool
	ids   map[string]string // item id -> file that defines it
	errs  []error
}

func (l *loader) fail(format string, args ...any) {
	l.errs = append(l.errs, fmt.Errorf(format, args...))
}

// Load reads and validates a catalog: units.yaml, categories.yaml, calculators/*.yaml, tables/*.yaml and codes/*.yaml
// (in that order, each sorted) at the root of fsys. All problems found are reported together.
func Load(fsys fs.FS) (*Catalog, error) {
	unitsData, err := fs.ReadFile(fsys, "units.yaml")
	if err != nil {
		return nil, fmt.Errorf("ecalc catalog: %w", err)
	}
	units, err := loadUnits(unitsData)
	if err != nil {
		return nil, err
	}
	l := &loader{units: units, cats: map[string]bool{}, ids: map[string]string{}}
	cat := &Catalog{Units: units, byID: map[string]*Calculator{}, tableByID: map[string]*Table{}, codeByID: map[string]*Code{}, kinds: map[string]string{}}

	catData, err := fs.ReadFile(fsys, "categories.yaml")
	if err != nil {
		return nil, fmt.Errorf("ecalc catalog: %w", err)
	}
	cat.Categories = l.categories(catData)

	files, err := fs.Glob(fsys, "calculators/*.yaml")
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	for _, file := range files {
		raws, ok := readItems[rawCalc](l, fsys, file)
		if !ok {
			continue
		}
		for i := range raws {
			if c := l.calculator(file, &raws[i]); c != nil {
				cat.Calculators = append(cat.Calculators, c)
				cat.byID[c.ID] = c
				cat.kinds[c.ID] = "calculator"
			}
		}
	}
	if err := l.tables(fsys, cat); err != nil {
		return nil, err
	}
	if err := l.codes(fsys, cat); err != nil {
		return nil, err
	}
	l.checkTableUse(cat)
	l.checkFinders(cat)
	if len(l.errs) > 0 {
		return nil, errors.Join(l.errs...)
	}
	return cat, nil
}

func (l *loader) categories(data []byte) []Category {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		l.fail("categories.yaml: %v", err)
		return nil
	}
	if len(doc.Content) == 0 {
		l.fail("categories.yaml: the file is empty")
		return nil
	}
	entries, err := orderedMap(doc.Content[0], "categories.yaml")
	if err != nil {
		l.fail("%v", err)
		return nil
	}
	var out []Category
	for _, e := range entries {
		var rc struct {
			Name Text `yaml:"name"`
		}
		if err := decodeNode(e.Val, &rc); err != nil {
			l.fail("categories.yaml: %s: %v", e.Key, err)
			continue
		}
		if !idRe.MatchString(e.Key) {
			l.fail("categories.yaml: category id %q must match %s", e.Key, idRe)
		}
		l.checkText("categories.yaml: "+e.Key+": name", rc.Name, Locales)
		l.cats[e.Key] = true
		out = append(out, Category{ID: e.Key, Name: rc.Name})
	}
	return out
}

// checkText reports missing required languages, empty texts and keys that are not desktop locales.
func (l *loader) checkText(what string, t Text, required []string) {
	for _, lang := range required {
		if strings.TrimSpace(t[lang]) == "" {
			l.fail("%s: missing %q text", what, lang)
		}
	}
	langs := make([]string, 0, len(t))
	for lang := range t {
		langs = append(langs, lang)
	}
	sort.Strings(langs)
	for _, lang := range langs {
		if !isLocale(lang) {
			l.fail("%s: unknown language key %q", what, lang)
		} else if strings.TrimSpace(t[lang]) == "" && !contains(required, lang) {
			l.fail("%s: empty %q text", what, lang)
		}
	}
}

func isLocale(lang string) bool { return contains(Locales, lang) }

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func newExpr(src string) (Expr, error) {
	n, err := ParseExpr(src)
	if err != nil {
		return Expr{}, fmt.Errorf("%q: %w", src, err)
	}
	return Expr{Src: src, Node: n, Deps: Deps(n)}, nil
}

// exprSources reads a YAML node that is one expression or a list of alternative expressions.
func exprSources(n *yaml.Node) ([]string, error) {
	switch n.Kind {
	case yaml.ScalarNode:
		return []string{n.Value}, nil
	case yaml.SequenceNode:
		out := make([]string, 0, len(n.Content))
		for _, it := range n.Content {
			if it.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("alternatives must be expression strings (line %d)", it.Line)
			}
			out = append(out, it.Value)
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("the list of alternatives is empty (line %d)", n.Line)
		}
		return out, nil
	}
	return nil, fmt.Errorf("an expression or a list of expressions is required (line %d)", n.Line)
}

// scalarValue reads a default: a string is parsed like user input (with the variable's display unit selected), a
// number is already in SI units.
func (l *loader) scalarValue(n *yaml.Node, dim *Dim, unit string) (float64, error) {
	if n.Kind != yaml.ScalarNode {
		return 0, fmt.Errorf("a number or a string is required (line %d)", n.Line)
	}
	if n.Tag == "!!str" {
		return l.units.Parse(n.Value, dim.ID, unit)
	}
	return strconv.ParseFloat(n.Value, 64)
}

// calculator builds and validates one calculator; it returns nil when something is wrong.
func (l *loader) calculator(file string, r *rawCalc) *Calculator {
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

	c := &Calculator{
		ID: r.ID, Kind: r.Kind, Category: r.Category, Name: r.Name, Doc: r.Doc, Tags: r.Tags,
		Source: r.Source, DOF: r.DOF, Formula: r.Formula, Schematic: r.Schematic, Finder: r.Finder,
		varByID: map[string]*Var{}, aliasChoice: map[string]int{},
	}
	l.claimID(r.ID, file, fail)
	if r.Kind != "calculator" {
		fail("kind must be calculator in calculators/, got %q", r.Kind)
	}
	switch {
	case !l.cats[r.Category]:
		fail("unknown category %q", r.Category)
	case r.Category == tablesCategory || r.Category == codesCategory || r.Category == "pinouts":
		fail("category %q is reserved for other kinds of items, a calculator belongs to a calculator category", r.Category)
	}
	text("name", r.Name, Locales)
	text("doc", r.Doc, docLocales)
	if strings.TrimSpace(r.Source) == "" {
		fail("source is required (a standard, datasheet or textbook)")
	}
	if len(r.Tags) == 0 {
		fail("at least one tag is required")
	}
	for _, tag := range r.Tags {
		if strings.TrimSpace(tag) == "" {
			fail("tags must not be empty")
		}
	}
	if !schematicRe.MatchString(r.Schematic) {
		fail("schematic %q must match %s", r.Schematic, schematicRe)
	}

	// names maps every name an expression may use to what it is.
	names := map[string]string{}
	claim := func(name, what string) bool {
		switch {
		case !idRe.MatchString(name):
			fail("%s %q must match %s", what, name, idRe)
		case name == "pi" || isFunc(name):
			fail("%s %q is reserved (constant or function name)", what, name)
		case names[name] != "":
			fail("%s %q collides with the %s of the same name", what, name, names[name])
		default:
			names[name] = what
			return true
		}
		return false
	}

	// Variables.
	varEntries, err := orderedMap(&r.Vars, "vars")
	if err != nil {
		fail("%v", err)
	}
	if len(varEntries) == 0 {
		fail("vars are required")
	}
	lists := map[string]bool{}
	c.Vars = make([]Var, 0, len(varEntries))
	for _, e := range varEntries {
		var rv rawVar
		if err := decodeNode(e.Val, &rv); err != nil {
			fail("var %s: %v", e.Key, err)
			continue
		}
		v := Var{ID: e.Key, Dim: rv.Dim, Label: rv.Label, Name: rv.Name, Unit: rv.Unit, Units: rv.Units, ESeries: rv.ESeries,
			List: rv.List, Input: rv.Input == nil || *rv.Input}
		claim(v.ID, "var")
		v.dim = l.units.Dim(v.Dim)
		if v.dim == nil {
			fail("var %s: unknown dimension %q", v.ID, v.Dim)
		} else {
			if v.Unit != "" && v.dim.Unit(v.Unit) == nil {
				fail("var %s: unit %q is not an explicit unit of %s", v.ID, v.Unit, v.Dim)
			}
			checkDisplayUnits(fail, "var "+v.ID, v.dim, v.Units, v.Unit)
			l.defaults(fail, &v, &rv.Default)
		}
		if strings.TrimSpace(v.Label) == "" {
			fail("var %s: label is required", v.ID)
		}
		text("var "+v.ID+": name", v.Name, docLocales)
		if v.List {
			lists[v.ID] = true
		}
		c.Vars = append(c.Vars, v)
	}
	for i := range c.Vars {
		c.varByID[c.Vars[i].ID] = &c.Vars[i]
	}

	// Choices: their consts are known names whatever option is selected.
	choiceEntries, err := orderedMap(&r.Choices, "choices")
	if err != nil {
		fail("%v", err)
	}
	for _, e := range choiceEntries {
		var rc rawChoice
		if err := decodeNode(e.Val, &rc); err != nil {
			fail("choice %s: %v", e.Key, err)
			continue
		}
		if !idRe.MatchString(e.Key) {
			fail("choice id %q must match %s", e.Key, idRe)
		}
		ch := Choice{ID: e.Key, Name: rc.Name, Default: rc.Default}
		text("choice "+ch.ID+": name", ch.Name, Locales)
		optEntries, err := orderedMap(&rc.Options, "choice "+ch.ID+": options")
		if err != nil {
			fail("%v", err)
		}
		if len(optEntries) < 2 {
			fail("choice %s needs at least two options", ch.ID)
		}
		var constNames, refNames []string
		for i, oe := range optEntries {
			var ro rawOption
			if err := decodeNode(oe.Val, &ro); err != nil {
				fail("choice %s option %s: %v", ch.ID, oe.Key, err)
				continue
			}
			opt := ChoiceOption{ID: oe.Key, Name: ro.Name, Consts: ro.Consts, Ignore: ro.Ignore, Refs: ro.Refs}
			text("choice "+ch.ID+" option "+opt.ID+": name", opt.Name, Locales)
			ignored := map[string]bool{}
			for _, id := range opt.Ignore {
				switch {
				case c.Var(id) == nil:
					fail("choice %s option %s: ignore %q is not a variable", ch.ID, opt.ID, id)
				case ignored[id]:
					fail("choice %s option %s: ignore %q is listed twice", ch.ID, opt.ID, id)
				}
				ignored[id] = true
			}
			keys := make([]string, 0, len(opt.Consts))
			for k := range opt.Consts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			refKeys := make([]string, 0, len(opt.Refs))
			for k, v := range opt.Refs {
				refKeys = append(refKeys, k)
				if !idRe.MatchString(v) {
					fail("choice %s option %s: ref %s = %q must be a table or column id (%s)", ch.ID, opt.ID, k, v, idRe)
				}
			}
			sort.Strings(refKeys)
			if i == 0 {
				constNames, refNames = keys, refKeys
				for _, k := range keys {
					claim(k, "choice const")
				}
				for _, k := range refKeys {
					if claim(k, tableAlias) {
						c.aliasChoice[k] = len(c.Choices)
					}
				}
			} else {
				if strings.Join(keys, ",") != strings.Join(constNames, ",") {
					fail("choice %s: option %s defines different consts than %s", ch.ID, opt.ID, ch.Options[0].ID)
				}
				if strings.Join(refKeys, ",") != strings.Join(refNames, ",") {
					fail("choice %s: option %s defines different refs than %s", ch.ID, opt.ID, ch.Options[0].ID)
				}
			}
			ch.Options = append(ch.Options, opt)
		}
		if !hasOption(ch, ch.Default) {
			fail("choice %s: default %q is not one of its options", ch.ID, ch.Default)
		}
		c.Choices = append(c.Choices, ch)
	}

	// Derived quantities.
	derivedEntries, err := orderedMap(&r.Derived, "derived")
	if err != nil {
		fail("%v", err)
	}
	for _, e := range derivedEntries {
		var rd rawDerived
		if err := decodeNode(e.Val, &rd); err != nil {
			fail("derived %s: %v", e.Key, err)
			continue
		}
		d := Derived{ID: e.Key, Dim: rd.Dim, Label: rd.Label, Name: rd.Name, Unit: rd.Unit, Units: rd.Units}
		claim(d.ID, "derived")
		if d.dim = l.units.Dim(d.Dim); d.dim == nil {
			fail("derived %s: unknown dimension %q", d.ID, d.Dim)
		} else {
			if d.Unit != "" && d.dim.Unit(d.Unit) == nil {
				fail("derived %s: unit %q is not an explicit unit of %s", d.ID, d.Unit, d.Dim)
			}
			checkDisplayUnits(fail, "derived "+d.ID, d.dim, d.Units, d.Unit)
		}
		if strings.TrimSpace(d.Label) == "" {
			fail("derived %s: label is required", d.ID)
		}
		text("derived "+d.ID+": name", d.Name, docLocales)
		d.Alts = l.alternatives(fail, "derived "+d.ID, &rd.Expr)
		c.Derived = append(c.Derived, d)
	}

	// Rules.
	ruleEntries, err := orderedMap(&r.Rules, "rules")
	if err != nil {
		fail("%v", err)
	}
	isTarget := map[string]bool{}
	for _, e := range ruleEntries {
		v := c.Var(e.Key)
		switch {
		case v == nil:
			fail("rule target %q is not a variable", e.Key)
		case v.List:
			fail("rule target %q is a list variable", e.Key)
		}
		isTarget[e.Key] = true
		rule := Rule{Target: e.Key, Alts: l.alternatives(fail, "rule "+e.Key, e.Val)}
		for _, alt := range rule.Alts {
			if contains(alt.Deps, e.Key) {
				fail("rule %s uses its own target", e.Key)
			}
		}
		c.Rules = append(c.Rules, rule)
	}

	// Every expression may only use known names; lists only inside sum() and par().
	known := func(what string, ex Expr) {
		for _, dep := range ex.Deps {
			switch names[dep] {
			case "":
				fail("%s: unknown name %q in %q", what, dep, ex.Src)
			case tableAlias:
				fail("%s: %q is a table alias, it can only name a table or a column in lookup, lookup_ge or interp (in %q)", what, dep, ex.Src)
			}
		}
		checkListUse(ex.Node, lists, func(msg string) { fail("%s: %s in %q", what, msg, ex.Src) })
	}
	for _, rule := range c.Rules {
		for _, alt := range rule.Alts {
			known("rule "+rule.Target, alt)
		}
	}
	for _, d := range c.Derived {
		for _, alt := range d.Alts {
			known("derived "+d.ID, alt)
		}
	}

	// Checks.
	for i, rc := range r.Checks {
		ex, err := newExpr(rc.When)
		if err != nil {
			fail("check %d: %v", i+1, err)
			continue
		}
		known(fmt.Sprintf("check %d", i+1), ex)
		text(fmt.Sprintf("check %d: warn", i+1), rc.Warn, docLocales)
		c.Checks = append(c.Checks, Check{When: ex, Warn: rc.Warn})
	}

	// Display formula. The combiner shows its combinations instead and may have none.
	if len(r.Formula) == 0 && r.Finder != finderCombine {
		fail("formula is required")
	}
	for _, line := range r.Formula {
		n, err := ParseFormula(line)
		if err != nil {
			fail("formula %q: %v", line, err)
			continue
		}
		c.FormulaAST = append(c.FormulaAST, n)
		for _, part := range n.Parts {
			known("formula", Expr{Src: line, Node: part, Deps: Deps(part)})
		}
	}

	// Inputs and defaults: exactly dof input-capable variables carry a default, and every variable can be known.
	inputs, withDefault := 0, 0
	for _, v := range c.Vars {
		if v.Input {
			inputs++
			if v.Default != nil || v.DefaultList != nil {
				withDefault++
			}
		} else if v.Default != nil || v.DefaultList != nil {
			fail("var %s is not an input and must not have a default", v.ID)
		}
		if !v.Input && !isTarget[v.ID] {
			fail("var %s is neither an input nor the target of a rule", v.ID)
		}
	}
	switch {
	case c.DOF < 1:
		fail("dof must be at least 1")
	case inputs < c.DOF:
		fail("dof is %d but only %d variables can be inputs", c.DOF, inputs)
	case withDefault != c.DOF:
		fail("dof is %d, so exactly %d input variables need a default, found %d", c.DOF, c.DOF, withDefault)
	}

	// Finder and chart.
	switch r.Finder {
	case "":
	case finderCombine:
		scalar := 0
		for _, v := range c.Vars {
			if v.Dim == "resistance" && !v.List {
				scalar++
			}
		}
		if scalar != 1 {
			fail("finder combine needs exactly one scalar resistance variable, found %d", scalar)
		}
		if c.DOF != 1 {
			fail("finder combine needs dof 1, got %d", c.DOF)
		}
	default:
		fail("finder %q must be empty or %s", r.Finder, finderCombine)
	}
	if r.Chart != nil {
		c.Chart = buildChart(fail, c, r.Chart, known)
	}

	if len(l.errs) == before {
		l.solvableSets(fail, c)
	}
	if len(l.errs) > before {
		return nil
	}
	return c
}

func hasOption(ch Choice, id string) bool {
	for _, o := range ch.Options {
		if o.ID == id {
			return true
		}
	}
	return false
}

// checkDisplayUnits validates the optional units whitelist of a variable or derived quantity: every entry is "" (the
// base symbol with automatic prefixes) or an explicit unit of the dimension, none twice, and the default display unit
// (when set) is one of them. An empty list offers every unit.
func checkDisplayUnits(fail func(string, ...any), what string, dim *Dim, units []string, unit string) {
	if len(units) == 0 {
		return
	}
	listed := map[string]bool{}
	for _, id := range units {
		switch {
		case listed[id]:
			fail("%s: units entry %q is listed twice", what, id)
		case id != "" && dim.Unit(id) == nil:
			fail("%s: units entry %q is not an explicit unit of %s", what, id, dim.ID)
		}
		listed[id] = true
	}
	if unit != "" && !listed[unit] {
		fail("%s: unit %q is not in its units list", what, unit)
	}
}

// buildChart validates the chart of a calculator: x is a scalar variable, y a rule target or derived quantity that uses
// x, to an expression over known names that does not use x, samples within the chart limits (0 = the default).
func buildChart(fail func(string, ...any), c *Calculator, rc *rawChart, known func(string, Expr)) *Chart {
	ch := &Chart{X: rc.X, Y: rc.Y, Samples: rc.Samples}
	if ch.Samples == 0 {
		ch.Samples = ChartDefaultSamples
	}
	if ch.Samples < ChartMinSamples || ch.Samples > ChartMaxSamples {
		fail("chart: samples must be between %d and %d, got %d", ChartMinSamples, ChartMaxSamples, ch.Samples)
	}
	x := c.Var(ch.X)
	switch {
	case x == nil:
		fail("chart: x %q is not a variable", ch.X)
	case x.List:
		fail("chart: x %q is a list variable", ch.X)
	}
	var alts []Expr
	for _, r := range c.Rules {
		if r.Target == ch.Y {
			alts = r.Alts
		}
	}
	for _, d := range c.Derived {
		if d.ID == ch.Y {
			alts = d.Alts
		}
	}
	usesX := false
	for _, alt := range alts {
		usesX = usesX || contains(alt.Deps, ch.X)
	}
	switch {
	case len(alts) == 0:
		fail("chart: y %q needs a rule or a derived expression", ch.Y)
	case x != nil && !usesX:
		fail("chart: y %q does not depend on x %q", ch.Y, ch.X)
	}
	to, err := newExpr(rc.To)
	if err != nil {
		fail("chart: to: %v", err)
		return ch
	}
	known("chart to", to)
	if contains(to.Deps, ch.X) {
		fail("chart: to %q must not depend on x %q", rc.To, ch.X)
	}
	if hasTableCall(to.Node) {
		fail("chart: to %q must not use table functions", rc.To)
	}
	ch.To = to
	return ch
}

// alternatives parses the expression or list of alternatives of a rule or derived quantity.
func (l *loader) alternatives(fail func(string, ...any), what string, n *yaml.Node) []Expr {
	if n.Kind == 0 {
		fail("%s: an expression is required", what)
		return nil
	}
	srcs, err := exprSources(n)
	if err != nil {
		fail("%s: %v", what, err)
		return nil
	}
	var out []Expr
	for _, src := range srcs {
		ex, err := newExpr(src)
		if err != nil {
			fail("%s: %v", what, err)
			continue
		}
		out = append(out, ex)
	}
	return out
}

// defaults reads and checks the default of a variable.
func (l *loader) defaults(fail func(string, ...any), v *Var, n *yaml.Node) {
	if n.Kind == 0 {
		return
	}
	check := func(x float64) bool {
		switch {
		case math.IsNaN(x) || math.IsInf(x, 0):
			fail("var %s: the default must be finite", v.ID)
		case v.dim.Positive && x <= 0:
			fail("var %s: the default must be greater than zero", v.ID)
		default:
			return true
		}
		return false
	}
	if v.List {
		if n.Kind != yaml.SequenceNode {
			fail("var %s: the default of a list variable must be a list", v.ID)
			return
		}
		list := make([]float64, 0, len(n.Content))
		for _, it := range n.Content {
			x, err := l.scalarValue(it, v.dim, v.Unit)
			if err != nil {
				fail("var %s: default: %v", v.ID, err)
				return
			}
			if !check(x) {
				return
			}
			list = append(list, x)
		}
		v.DefaultList = list
		return
	}
	x, err := l.scalarValue(n, v.dim, v.Unit)
	if err != nil {
		fail("var %s: default: %v", v.ID, err)
		return
	}
	if check(x) {
		v.Default = &x
	}
}

// checkListUse reports a list variable used outside sum()/par() and a sum()/par() argument that is not a list.
func checkListUse(n *Node, lists map[string]bool, report func(string)) {
	if n == nil {
		return
	}
	if n.T == "call" && (n.Name == "sum" || n.Name == "par") {
		if a := n.Args[0]; a.T != "id" || !lists[a.Name] {
			report(n.Name + "() needs a list variable")
		}
		return
	}
	if n.T == "id" && lists[n.Name] {
		report(fmt.Sprintf("list variable %q is used outside sum() or par()", n.Name))
	}
	checkListUse(n.A, lists, report)
	checkListUse(n.L, lists, report)
	checkListUse(n.R, lists, report)
	for _, a := range valueArgs(n) {
		checkListUse(a, lists, report)
	}
	for _, a := range n.Parts {
		checkListUse(a, lists, report)
	}
}
