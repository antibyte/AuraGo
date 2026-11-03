package ecalc

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The query API behind the agent tool ecalc: Search, Describe, Calculate and Check. Output texts are English; the
// structs carry JSON tags because the tool sends them as they are. Nothing here touches the server, the file system
// or the network.

const (
	// DefaultSearchLimit and MaxSearchLimit bound the number of hits of Search.
	DefaultSearchLimit = 10
	MaxSearchLimit     = 25
	// MaxQueryValues is the largest number of values (and of choices) in one request. The desktop bridge that opens
	// ECalc pre-filled accepts the same number, so a request that passes Check is never cut there.
	MaxQueryValues = 32
	// ContradictionTolerance is the relative difference |given - computed| / max(|given|, |computed|) above which a
	// redundant value contradicts the computed one.
	ContradictionTolerance = 0.01
	// DefaultESeries is the series of the suggestions when the request names none.
	DefaultESeries = "E24"
	// MaxIDLength is the longest item id a request may carry, in characters. The ids of the catalog have at most 63; a
	// longer one is refused as unknown before the search for close matches runs on it.
	MaxIDLength = 256

	querySig     = 4   // significant digits of every formatted value
	summaryRunes = 200 // longest summary of a search hit
	// transcribedSig is the number of significant digits of a cell of a transcribed table (see formatTableValue); the
	// Desktop page shows such a cell with at least as many (TRANSCRIBED_SIG in ecalc-table-page.js).
	transcribedSig = 9
)

// Error codes of QueryError.
const (
	CodeUnknownID       = "unknown_id"
	CodeUnknownVariable = "unknown_variable"
	CodeNotAnInput      = "not_an_input"
	CodeUnknownChoice   = "unknown_choice"
	CodeUnknownOption   = "unknown_option"
	CodeUnknownESeries  = "unknown_eseries"
	CodeParse           = "parse_error"
	CodeUnderdetermined = "underdetermined"
	CodeDuplicate       = "duplicate_name"
	CodeTooMany         = "too_many_values"
	// The codes of the table and code operations (query_items.go, query_table.go, query_codes.go). The errors of the
	// code kinds keep their own codes (CodeInvalidCode, CodeInvalidValue, CodeUnsupported of codes.go).
	CodeNotACalculator  = "not_a_calculator"
	CodeNotATable       = "not_a_table"
	CodeNotACode        = "not_a_code"
	CodeUnknownColumn   = "unknown_column"
	CodeInvalidFilter   = "invalid_filter"
	CodeInvalidLookup   = "invalid_lookup"
	CodeUnknownCodeKind = "unknown_code_kind"
)

// QueryError is a problem with a request. Message names the valid alternatives; Sets is filled for
// CodeUnderdetermined (the solvable input sets, closest first).
type QueryError struct {
	Code    string
	Message string
	Sets    [][]string
}

func (e *QueryError) Error() string { return e.Message }

func queryErr(code, format string, args ...any) *QueryError {
	return &QueryError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// AsQueryError returns err as a *QueryError, or nil when it is none.
func AsQueryError(err error) *QueryError {
	var qe *QueryError
	if errors.As(err, &qe) {
		return qe
	}
	return nil
}

// ---- search ---------------------------------------------------------------------------------------------------

// SearchHit is one result of Search.
type SearchHit struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Category string `json:"category"`
	Name     string `json:"name"`
	Summary  string `json:"summary"`
}

// CategorySummary is one category with the number of its items (calculators, tables and codes).
type CategorySummary struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Overview lists the categories of the catalog that hold items (English names) with their item counts; a category
// without items (the pinouts before they exist) is left out.
func (c *Catalog) Overview() []CategorySummary {
	count := map[string]int{}
	for _, it := range c.searchItems() {
		count[it.category]++
	}
	out := make([]CategorySummary, 0, len(c.Categories))
	for _, cat := range c.Categories {
		if n := count[cat.ID]; n > 0 {
			out = append(out, CategorySummary{ID: cat.ID, Name: cat.Name.in("en"), Count: n})
		}
	}
	return out
}

// Ranking tiers of a search token, best first.
const (
	tierID         = iota // the id (or the whole query is the id)
	tierNamePrefix        // a name starts with it
	tierNameWord          // a word of a name or of the id starts with it
	tierTag               // a tag or the category starts with it
	tierSubstring         // it occurs anywhere
	tierNone
)

type searchFields struct {
	id        string
	idWords   []string
	names     []string
	nameWords [][]string
	tags      []string
	tagWords  [][]string
	cats      []string
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) }

func wordsOf(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !isWordRune(r) })
}

func (c *Catalog) searchFieldsOf(it searchItem) searchFields {
	f := searchFields{id: strings.ToLower(it.id), idWords: wordsOf(it.id)}
	for _, lang := range Locales {
		if n := strings.ToLower(strings.TrimSpace(it.name[lang])); n != "" {
			f.names = append(f.names, n)
			f.nameWords = append(f.nameWords, wordsOf(n))
		}
	}
	for _, tag := range it.tags {
		t := strings.ToLower(strings.TrimSpace(tag))
		f.tags = append(f.tags, t)
		f.tagWords = append(f.tagWords, wordsOf(t))
	}
	f.cats = append(f.cats, strings.ToLower(it.category))
	for _, cat := range c.Categories {
		if cat.ID == it.category {
			for _, lang := range Locales {
				if n := strings.ToLower(strings.TrimSpace(cat.Name[lang])); n != "" {
					f.cats = append(f.cats, n)
				}
			}
		}
	}
	return f
}

func (f *searchFields) wholeTier(query string) int {
	if query == strings.Join(f.idWords, " ") {
		return tierID
	}
	for _, words := range f.nameWords {
		if strings.HasPrefix(strings.Join(words, " "), query) {
			return tierNamePrefix
		}
	}
	return tierNone
}

func (f *searchFields) tokenTier(tok string) int {
	best := tierNone
	set := func(t int) {
		if t < best {
			best = t
		}
	}
	if tok == f.id {
		set(tierID)
	}
	for i, n := range f.names {
		if strings.HasPrefix(n, tok) {
			set(tierNamePrefix)
		}
		for _, w := range f.nameWords[i] {
			if strings.HasPrefix(w, tok) {
				set(tierNameWord)
			}
		}
		if strings.Contains(n, tok) {
			set(tierSubstring)
		}
	}
	for _, w := range f.idWords {
		if strings.HasPrefix(w, tok) {
			set(tierNameWord)
		}
	}
	if strings.Contains(f.id, tok) {
		set(tierSubstring)
	}
	for i, t := range f.tags {
		if strings.HasPrefix(t, tok) {
			set(tierTag)
		}
		for _, w := range f.tagWords[i] {
			if strings.HasPrefix(w, tok) {
				set(tierTag)
			}
		}
		if strings.Contains(t, tok) {
			set(tierSubstring)
		}
	}
	for _, cat := range f.cats {
		if strings.HasPrefix(cat, tok) {
			set(tierTag)
		}
	}
	return best
}

// SearchAll returns every item (calculator, table or code) that matches the query, best match first. The query is
// split into words that must all match (case-insensitively) the id, one of the names in the 16 locales, a tag or the
// category. A hit ranks by its weakest word: exact id, then name prefix, name word, tag or category, substring; equal
// ranks keep the order of the catalog (calculators, then tables, then codes). An empty query matches everything in
// catalog order.
func (c *Catalog) SearchAll(query string) []SearchHit {
	tokens := wordsOf(query)
	whole := strings.Join(tokens, " ")
	type ranked struct {
		item     searchItem
		pri, sum int
		position int
	}
	var hits []ranked
	for i, it := range c.searchItems() {
		if len(tokens) == 0 {
			hits = append(hits, ranked{item: it, position: i})
			continue
		}
		f := c.searchFieldsOf(it)
		pri, sum, ok := 0, 0, true
		for _, tok := range tokens {
			t := f.tokenTier(tok)
			if t == tierNone {
				ok = false
				break
			}
			pri = max(pri, t)
			sum += t
		}
		w := f.wholeTier(whole)
		switch {
		case w < pri:
			pri, sum, ok = w, 0, true
		case !ok:
			continue
		}
		hits = append(hits, ranked{item: it, pri: pri, sum: sum, position: i})
	}
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].pri != hits[b].pri {
			return hits[a].pri < hits[b].pri
		}
		if hits[a].sum != hits[b].sum {
			return hits[a].sum < hits[b].sum
		}
		return hits[a].position < hits[b].position
	})
	out := make([]SearchHit, 0, len(hits))
	for _, h := range hits {
		out = append(out, SearchHit{
			ID: h.item.id, Kind: h.item.kind, Category: h.item.category, Name: h.item.name.in("en"), Summary: h.item.summary(),
		})
	}
	return out
}

// Search returns the best limit matches of SearchAll. A limit below 1 means DefaultSearchLimit; the largest is
// MaxSearchLimit.
func (c *Catalog) Search(query string, limit int) []SearchHit {
	if limit < 1 {
		limit = DefaultSearchLimit
	}
	limit = min(limit, MaxSearchLimit)
	all := c.SearchAll(query)
	if len(all) > limit {
		all = all[:limit]
	}
	return all
}

// firstSentence is the text up to the first sentence end (a period followed by a space), cut to summaryRunes.
func firstSentence(doc string) string {
	doc = strings.Join(strings.Fields(doc), " ")
	end := len(doc)
	for i := 0; i+1 < len(doc); i++ {
		if doc[i] != '.' || doc[i+1] != ' ' {
			continue
		}
		before := strings.ToLower(doc[:i])
		abbreviation := false
		for _, ab := range []string{"e.g", "i.e", "approx", "vs", "cf", "etc"} {
			if strings.HasSuffix(before, " "+ab) || before == ab {
				abbreviation = true
			}
		}
		if !abbreviation {
			end = i + 1
			break
		}
	}
	s := doc[:end]
	if r := []rune(s); len(r) > summaryRunes {
		s = strings.TrimSpace(string(r[:summaryRunes-1])) + "…"
	}
	return s
}

// ---- describe -------------------------------------------------------------------------------------------------

// VarInfo describes one variable. Unit is the display unit a bare number is read in (empty: the SI unit with an
// automatic prefix); Units are the units a value may carry.
type VarInfo struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Name    string   `json:"name"`
	Dim     string   `json:"dim"`
	Unit    string   `json:"unit,omitempty"`
	Units   []string `json:"units,omitempty"`
	Input   bool     `json:"input"`
	List    bool     `json:"list,omitempty"`
	ESeries bool     `json:"eseries,omitempty"`
	Default string   `json:"default,omitempty"`
}

// OptionInfo is one option of a choice. Ignore lists the variables the option does not use (a value for them is not
// needed: Calculate takes their defaults).
type OptionInfo struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Ignore []string `json:"ignore,omitempty"`
}

// ChoiceInfo is a selector with its options.
type ChoiceInfo struct {
	ID      string       `json:"id"`
	Name    string       `json:"name"`
	Default string       `json:"default"`
	Options []OptionInfo `json:"options"`
}

// DerivedInfo is a computed-only quantity.
type DerivedInfo struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Name  string `json:"name"`
	Dim   string `json:"dim"`
	Unit  string `json:"unit,omitempty"`
}

// ChartInfo is the chart of a calculator: y over x from 0 to To (an expression), drawn with Samples points.
type ChartInfo struct {
	X       string `json:"x"`
	Y       string `json:"y"`
	To      string `json:"to"`
	Samples int    `json:"samples"`
}

// Description is what Describe returns for a calculator. Finder is "combine" for the standard-value combiner, whose
// result lists combinations of E-series values (see Calculate).
type Description struct {
	ID       string        `json:"id"`
	Kind     string        `json:"kind"`
	Category string        `json:"category"`
	Name     string        `json:"name"`
	Doc      string        `json:"doc"`
	Source   string        `json:"source"`
	DOF      int           `json:"dof"`
	Vars     []VarInfo     `json:"vars"`
	Choices  []ChoiceInfo  `json:"choices,omitempty"`
	Derived  []DerivedInfo `json:"derived,omitempty"`
	Sets     [][]string    `json:"sets"`
	Checks   []string      `json:"checks,omitempty"`
	Formula  []string      `json:"formula,omitempty"`
	Finder   string        `json:"finder,omitempty"`
	Chart    *ChartInfo    `json:"chart,omitempty"`
}

// unitSymbol is the symbol of an explicit unit of the dimension, or the base symbol when unitID is "".
func (c *Catalog) unitSymbol(dimID, unitID string) string {
	d := c.Units.Dim(dimID)
	if d == nil {
		return ""
	}
	if un := d.Unit(unitID); un != nil {
		return un.Symbol
	}
	return d.Symbol
}

// unitSymbols lists the units a value of the variable may carry: the units its catalog entry offers ("" is the base
// symbol with automatic prefixes) or, when it names none, the base symbol and every explicit unit of the dimension.
func (c *Catalog) unitSymbols(v *Var) []string {
	d := c.Units.Dim(v.Dim)
	if d == nil {
		return nil
	}
	var out []string
	add := func(symbol string) {
		if symbol != "" {
			out = append(out, symbol)
		}
	}
	if len(v.Units) > 0 {
		for _, id := range v.Units {
			add(c.unitSymbol(v.Dim, id))
		}
		return out
	}
	add(d.Symbol)
	for _, un := range d.Units {
		add(un.Symbol)
	}
	return out
}

func (c *Catalog) formatValue(x float64, dimID, unitID string) string {
	return c.Units.Format(x, dimID, unitID, querySig)
}

// formatTableValue writes a number of table t. A transcribed cell is a published number (a band edge, a limit) and is
// written with up to transcribedSig significant digits, so that it is never shortened below the digits it was written
// with (434.79 MHz, not 434.8 MHz); the cells of a generated table are computed and keep querySig digits. The shortest
// decimal prints, so 9 digits only show the digits a cell has.
func (c *Catalog) formatTableValue(t *Table, x float64, dimID, unitID string) string {
	if t.Generate != "" {
		return c.formatValue(x, dimID, unitID)
	}
	return c.Units.Format(x, dimID, unitID, transcribedSig)
}

// formatList writes a list the way it is typed: items separated by a comma and a space, each item without a space
// before its unit ("4.7kΩ, 10kΩ"), because ParseList splits at white space.
func (c *Catalog) formatList(list []float64, dimID, unitID string) string {
	parts := make([]string, len(list))
	for i, x := range list {
		parts[i] = strings.ReplaceAll(c.formatValue(x, dimID, unitID), " ", "")
	}
	return strings.Join(parts, ", ")
}

// Describe returns the structure of a calculator: variables with units and defaults, choices, the solvable input
// sets, derived quantities, checks and the display formulas.
func (c *Catalog) Describe(id string) (Description, error) {
	calc, err := c.lookup(id)
	if err != nil {
		return Description{}, err
	}
	d := Description{
		ID: calc.ID, Kind: calc.Kind, Category: calc.Category, Name: calc.Name.in("en"), Doc: calc.Doc.in("en"),
		Source: calc.Source, DOF: calc.DOF, Sets: calc.Sets, Formula: calc.Formula,
		Vars: make([]VarInfo, 0, len(calc.Vars)),
	}
	if d.Sets == nil {
		d.Sets = [][]string{}
	}
	for i := range calc.Vars {
		v := &calc.Vars[i]
		info := VarInfo{
			ID: v.ID, Label: v.Label, Name: v.Name.in("en"), Dim: v.Dim, Unit: c.unitSymbol(v.Dim, v.Unit),
			Units: c.unitSymbols(v), Input: v.Input, List: v.List, ESeries: v.ESeries,
		}
		switch {
		case v.DefaultList != nil:
			info.Default = c.formatList(v.DefaultList, v.Dim, v.Unit)
		case v.Default != nil:
			info.Default = c.formatValue(*v.Default, v.Dim, v.Unit)
		}
		d.Vars = append(d.Vars, info)
	}
	for _, ch := range calc.Choices {
		info := ChoiceInfo{ID: ch.ID, Name: ch.Name.in("en"), Default: ch.Default}
		for _, o := range ch.Options {
			info.Options = append(info.Options, OptionInfo{ID: o.ID, Name: o.Name.in("en"), Ignore: o.Ignore})
		}
		d.Choices = append(d.Choices, info)
	}
	for _, dv := range calc.Derived {
		d.Derived = append(d.Derived, DerivedInfo{
			ID: dv.ID, Label: dv.Label, Name: dv.Name.in("en"), Dim: dv.Dim, Unit: c.unitSymbol(dv.Dim, dv.Unit),
		})
	}
	for _, check := range calc.Checks {
		d.Checks = append(d.Checks, check.Warn.in("en"))
	}
	d.Finder = calc.Finder
	if ch := calc.Chart; ch != nil {
		d.Chart = &ChartInfo{X: ch.X, Y: ch.Y, To: ch.To.Src, Samples: ch.Samples}
	}
	return d, nil
}

// lookup finds a calculator by id (case-insensitive). The error of an unknown id suggests the best search hits; the
// id of a table or a code names the operations that read it.
func (c *Catalog) lookup(id string) (*Calculator, error) {
	if qe := idTooLong(id, "calculator"); qe != nil {
		return nil, qe
	}
	key := strings.ToLower(strings.TrimSpace(id))
	if key == "" {
		return nil, queryErr(CodeUnknownID, "id is required; use search to find a calculator id")
	}
	if calc := c.Calculator(key); calc != nil {
		return calc, nil
	}
	return nil, c.notFound(id, key, "calculator")
}

// idTooLong is the unknown-id error of an id of more than MaxIDLength characters (nil for any other id). The lookups
// answer with it before they search for close matches, which costs seconds for a very long id; the message does not
// repeat the id. want is the kind of item that was asked for ("" for any).
func idTooLong(id, want string) *QueryError {
	if utf8.RuneCountInString(strings.TrimSpace(id)) <= MaxIDLength {
		return nil
	}
	if want == "" {
		return queryErr(CodeUnknownID, "unknown id: it is longer than %d characters; use search to find a calculator, table or code id", MaxIDLength)
	}
	return queryErr(CodeUnknownID, "unknown %s: the id is longer than %d characters; use search to find a %s id", want, MaxIDLength, want)
}

// notFound is the error of an id that is no item of the wanted kind ("calculator", "table", "code" or "" for any). The
// id of an item of another kind says what it is; an unknown id suggests the best search hits of the wanted kind.
func (c *Catalog) notFound(id, key, want string) *QueryError {
	if kind, ok := c.Item(key); ok {
		hint := map[string]string{
			"calculator": "describe and calculate work on it",
			"table":      "describe lists its columns and table reads its rows",
			"code":       "describe lists its code kind, and decode and encode work on that kind",
		}[kind]
		code := map[string]string{"calculator": CodeNotACalculator, "table": CodeNotATable, "code": CodeNotACode}[want]
		return queryErr(code, "%s is a %s, not a %s; %s", key, kind, want, hint)
	}
	label := want
	if label == "" {
		label = "calculator, table or code"
	}
	msg := fmt.Sprintf("unknown %s %q; use search to find a %s id", want, id, label)
	if want == "" {
		msg = fmt.Sprintf("unknown id %q; use search to find a %s id", id, label)
	}
	var ids []string
	for _, h := range c.SearchAll(strings.NewReplacer("_", " ", "-", " ").Replace(key)) {
		if want == "" || h.Kind == want {
			ids = append(ids, h.ID)
		}
		if len(ids) == 3 {
			break
		}
	}
	if len(ids) > 0 {
		msg += "; did you mean " + strings.Join(ids, ", ") + "?"
	}
	return &QueryError{Code: CodeUnknownID, Message: msg}
}

// ResolveItem finds an item of any kind by id (case-insensitive) and returns its kind ("calculator", "table" or
// "code") and its canonical id. The error of an unknown id suggests the best search hits.
func (c *Catalog) ResolveItem(id string) (kind, canonical string, err error) {
	if qe := idTooLong(id, ""); qe != nil {
		return "", "", qe
	}
	key := strings.ToLower(strings.TrimSpace(id))
	if key == "" {
		return "", "", queryErr(CodeUnknownID, "id is required; use search to find a calculator, table or code id")
	}
	if kind, ok := c.Item(key); ok {
		return kind, key, nil
	}
	return "", "", c.notFound(id, key, "")
}

// ---- calculate ------------------------------------------------------------------------------------------------

// NamedValue is a name with a text: a variable id with a value as a person would type it ("4k7", "12 V", "25 °C"),
// or a choice id with an option id.
type NamedValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CalcRequest is the input of Calculate and Check. Values are read with the dimension of their variable; a bare
// number is in the variable's default display unit (the SI unit when it has none). Choices not given use their
// default. ESeries ("" = DefaultESeries) is the series of the standard-value suggestions.
type CalcRequest struct {
	Values  []NamedValue
	Choices []NamedValue
	ESeries string
}

// Checked is a request whose names were resolved: ids are canonical, texts are as typed (trimmed).
type Checked struct {
	ID      string
	Values  []NamedValue
	Choices []NamedValue
}

// ESeriesHint is the nearest standard value of a computed quantity.
type ESeriesHint struct {
	Series    string  `json:"series"`
	Value     float64 `json:"value"`
	Formatted string  `json:"formatted"`
	DevPct    float64 `json:"dev_pct"`
}

// Quantity is one variable or derived quantity of a result. SI is nil when the value is unknown or not finite. Label,
// Name and Dim are omitted when the tool has to shorten an answer to fit its output budget. Ignored marks a variable
// the selected choice option does not use; Defaulted that its value is the default because none was given.
type Quantity struct {
	ID        string       `json:"id"`
	Label     string       `json:"label,omitempty"`
	Name      string       `json:"name,omitempty"`
	Kind      string       `json:"kind"`
	Dim       string       `json:"dim,omitempty"`
	SI        *float64     `json:"si,omitempty"`
	SIList    []float64    `json:"si_list,omitempty"`
	Formatted string       `json:"formatted"`
	Computed  bool         `json:"computed"`
	Given     string       `json:"given,omitempty"`
	Ignored   bool         `json:"ignored,omitempty"`
	Defaulted bool         `json:"defaulted,omitempty"`
	Error     string       `json:"error,omitempty"`
	ESeries   *ESeriesHint `json:"eseries,omitempty"`
}

// Contradiction is a redundant given value that differs from the computed one by more than ContradictionTolerance.
type Contradiction struct {
	Name       string  `json:"name"`
	Given      string  `json:"given"`
	Computed   string  `json:"computed"`
	GivenSI    float64 `json:"given_si"`
	ComputedSI float64 `json:"computed_si"`
	DiffPct    float64 `json:"diff_pct"`
}

// ChoiceState is the option that was used for a choice.
type ChoiceState struct {
	ID         string `json:"id"`
	Option     string `json:"option"`
	OptionName string `json:"option_name"`
}

// ComboInfo is one combination of the standard-value combiner: the single nearest value (Kind "single", R1 only), two
// values in series or two in parallel. Values are formatted; DevPct is the deviation from the target in percent.
type ComboInfo struct {
	Kind   string  `json:"kind"`
	R1     string  `json:"r1"`
	R2     string  `json:"r2,omitempty"`
	Value  string  `json:"value"`
	DevPct float64 `json:"dev_pct"`
}

// CalcResult is what Calculate returns. Basis are the input variables the result was computed from.
type CalcResult struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	Choices        []ChoiceState   `json:"choices,omitempty"`
	Basis          []string        `json:"basis"`
	Quantities     []Quantity      `json:"quantities"`
	Contradictions []Contradiction `json:"contradictions,omitempty"`
	Warnings       []string        `json:"warnings,omitempty"`
	ESeries        string          `json:"eseries"`
	Combos         []ComboInfo     `json:"combos,omitempty"`
}

// HasErrors reports whether any quantity carries an error (a value out of range, not finite or undetermined).
func (r *CalcResult) HasErrors() bool {
	for _, q := range r.Quantities {
		if q.Error != "" {
			return true
		}
	}
	return false
}

type givenValue struct {
	v      *Var
	text   string
	si     float64
	list   []float64
	filled bool // the default of a variable that the selected option ignores, not a value of the request
}

type resolvedRequest struct {
	calc    *Calculator
	given   []givenValue
	choices []ChoiceState
	checked Checked
	series  string
}

// alnum lowercases s and drops separators, so that "V_in", "v in" and "vin" compare equal.
func alnum(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if r != '_' && r != ' ' && r != '-' && r != '{' && r != '}' && r != '$' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func idsOf[T any](items []T, id func(T) string) string {
	parts := make([]string, len(items))
	for i, it := range items {
		parts[i] = id(it)
	}
	return strings.Join(parts, ", ")
}

// findVar resolves a variable name: the id (case-insensitive) or, when that is unambiguous, the label.
func findVar(calc *Calculator, name string) (*Var, *QueryError) {
	key := strings.ToLower(strings.TrimSpace(name))
	if v := calc.Var(key); v != nil {
		return v, nil
	}
	var byLabel []*Var
	for i := range calc.Vars {
		if v := &calc.Vars[i]; alnum(v.ID) == alnum(key) || alnum(v.Label) == alnum(key) {
			byLabel = append(byLabel, v)
		}
	}
	if len(byLabel) == 1 {
		return byLabel[0], nil
	}
	for _, d := range calc.Derived {
		if d.ID == key {
			return nil, queryErr(CodeNotAnInput, "%q of %s is a computed output (%s), not an input; input variables: %s", name, calc.ID, d.Name.in("en"), inputList(calc))
		}
	}
	return nil, queryErr(CodeUnknownVariable, "unknown variable %q for %s; input variables: %s", name, calc.ID, inputList(calc))
}

func inputList(calc *Calculator) string {
	var parts []string
	for _, v := range calc.Vars {
		if v.Input {
			parts = append(parts, fmt.Sprintf("%s (%s)", v.ID, v.Name.in("en")))
		}
	}
	return strings.Join(parts, ", ")
}

func findChoice(calc *Calculator, name string) (*Choice, *QueryError) {
	key := alnum(name)
	for i := range calc.Choices {
		ch := &calc.Choices[i]
		if alnum(ch.ID) == key || alnum(ch.Name.in("en")) == key {
			return ch, nil
		}
	}
	if len(calc.Choices) == 0 {
		return nil, queryErr(CodeUnknownChoice, "%s has no choices, so %q cannot be set", calc.ID, name)
	}
	return nil, queryErr(CodeUnknownChoice, "unknown choice %q for %s; choices: %s", name, calc.ID, idsOf(calc.Choices, func(c Choice) string { return c.ID }))
}

func findOption(calc *Calculator, ch *Choice, name string) (*ChoiceOption, *QueryError) {
	key := alnum(name)
	for i := range ch.Options {
		o := &ch.Options[i]
		if alnum(o.ID) == key || alnum(o.Name.in("en")) == key {
			return o, nil
		}
	}
	return nil, queryErr(CodeUnknownOption, "unknown option %q of choice %s in %s; options: %s", name, ch.ID, calc.ID, idsOf(ch.Options, func(o ChoiceOption) string { return o.ID }))
}

// resolve validates a request: the calculator, every name, the number of values, the E-series and the text of every
// value (it must parse with the dimension of its variable).
func (c *Catalog) resolve(id string, req CalcRequest) (*resolvedRequest, error) {
	calc, err := c.lookup(id)
	if err != nil {
		return nil, err
	}
	if len(req.Values) > MaxQueryValues || len(req.Choices) > MaxQueryValues {
		return nil, queryErr(CodeTooMany, "at most %d values and %d choices per request", MaxQueryValues, MaxQueryValues)
	}
	rr := &resolvedRequest{calc: calc, checked: Checked{ID: calc.ID}, series: DefaultESeries}
	if s := strings.ToUpper(strings.TrimSpace(req.ESeries)); s != "" {
		rr.series = s
	}
	if _, ok := ESeriesValues(rr.series); !ok {
		return nil, queryErr(CodeUnknownESeries, "unknown E-series %q; use one of %s", req.ESeries, strings.Join(ESeriesIDs, ", "))
	}

	chosen := map[string]string{}
	for _, nv := range req.Choices {
		ch, qe := findChoice(calc, nv.Name)
		if qe != nil {
			return nil, qe
		}
		if _, dup := chosen[ch.ID]; dup {
			return nil, queryErr(CodeDuplicate, "choice %s is given twice", ch.ID)
		}
		opt, qe := findOption(calc, ch, nv.Value)
		if qe != nil {
			return nil, qe
		}
		chosen[ch.ID] = opt.ID
		rr.checked.Choices = append(rr.checked.Choices, NamedValue{Name: ch.ID, Value: opt.ID})
	}
	for i := range calc.Choices {
		ch := &calc.Choices[i]
		opt := ch.option(chosen[ch.ID])
		rr.choices = append(rr.choices, ChoiceState{ID: ch.ID, Option: opt.ID, OptionName: opt.Name.in("en")})
	}

	seen := map[string]bool{}
	for _, nv := range req.Values {
		v, qe := findVar(calc, nv.Name)
		if qe != nil {
			return nil, qe
		}
		if !v.Input {
			return nil, queryErr(CodeNotAnInput, "%q of %s is only ever computed; input variables: %s", nv.Name, calc.ID, inputList(calc))
		}
		if seen[v.ID] {
			return nil, queryErr(CodeDuplicate, "variable %s is given twice", v.ID)
		}
		seen[v.ID] = true
		text := strings.TrimSpace(nv.Value)
		g := givenValue{v: v, text: text}
		var perr error
		if v.List {
			g.list, perr = c.Units.ParseList(text, v.Dim, v.Unit)
		} else {
			g.si, perr = c.Units.Parse(text, v.Dim, v.Unit)
		}
		if perr != nil {
			shape := v.Dim
			if v.List {
				shape = "list of " + v.Dim + " values such as 4k7, 10k, 2k2"
			}
			return nil, queryErr(CodeParse, "cannot read %q for %s as a %s: %v", text, v.ID, shape, perr)
		}
		rr.given = append(rr.given, g)
		rr.checked.Values = append(rr.checked.Values, NamedValue{Name: v.ID, Value: text})
	}
	return rr, nil
}

// Check validates a request exactly like Calculate does for names, units and the E-series, without solving. It is
// what opening ECalc pre-filled uses: the values are sent as typed, so a request with fewer or more values than the
// calculator needs is fine.
func (c *Catalog) Check(id string, req CalcRequest) (Checked, error) {
	rr, err := c.resolve(id, req)
	if err != nil {
		return Checked{}, err
	}
	return rr.checked, nil
}

// firstSetWithin is the first solvable input set (catalog order) whose variables are all in names, or nil.
func firstSetWithin(calc *Calculator, names map[string]bool) []string {
	for _, set := range calc.Sets {
		inside := true
		for _, id := range set {
			if !names[id] {
				inside = false
				break
			}
		}
		if inside {
			return set
		}
	}
	return nil
}

// underdetermined builds the error of a request whose values contain no solvable input set. The sets are listed
// closest first (fewest missing values), so the message says what to add.
func underdetermined(calc *Calculator, given []givenValue, names map[string]bool) *QueryError {
	type scored struct {
		set     []string
		missing []string
	}
	var all []scored
	for _, set := range calc.Sets {
		s := scored{set: set}
		for _, id := range set {
			if !names[id] {
				s.missing = append(s.missing, id)
			}
		}
		all = append(all, s)
	}
	sort.SliceStable(all, func(a, b int) bool { return len(all[a].missing) < len(all[b].missing) })
	have := make([]string, len(given))
	for i, g := range given {
		have[i] = g.v.ID
	}
	var b strings.Builder
	if len(given) < calc.DOF {
		fmt.Fprintf(&b, "under-determined: %s needs %d values, got %d", calc.ID, calc.DOF, len(given))
	} else {
		fmt.Fprintf(&b, "the values do not determine %s: no combination of the values is a solvable input set", calc.ID)
	}
	if len(have) > 0 {
		fmt.Fprintf(&b, " (given: %s)", strings.Join(have, ", "))
	}
	b.WriteString("; solvable input sets, closest first: ")
	qe := &QueryError{Code: CodeUnderdetermined}
	for i, s := range all {
		if i == 6 {
			fmt.Fprintf(&b, "; and %d more (see describe)", len(all)-i)
			break
		}
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(strings.Join(s.set, " + "))
		if len(s.missing) > 0 && len(s.missing) < len(s.set) {
			fmt.Fprintf(&b, " (add %s)", strings.Join(s.missing, ", "))
		}
		qe.Sets = append(qe.Sets, s.set)
	}
	qe.Message = b.String()
	return qe
}

func finitePtr(x float64) *float64 {
	if math.IsNaN(x) || math.IsInf(x, 0) {
		return nil
	}
	return &x
}

func errorText(code string) string {
	switch code {
	case ErrUndetermined:
		return "undetermined: the given values do not fix this quantity"
	case ErrNonFinite:
		return "nonfinite: the result is not a finite number (division by zero or overflow)"
	case ErrRange:
		return "range: the value must be greater than zero"
	}
	return code
}

// Calculate solves a calculator from the values of the request. It uses the first solvable input set (catalog order)
// whose variables are all given; every other given value is compared with the computed one and listed as a
// contradiction when it differs by more than ContradictionTolerance. The error of a request that cannot be solved
// is a *QueryError.
func (c *Catalog) Calculate(id string, req CalcRequest) (CalcResult, error) {
	rr, err := c.resolve(id, req)
	if err != nil {
		return CalcResult{}, err
	}
	calc := rr.calc
	names := map[string]bool{}
	for _, g := range rr.given {
		names[g.v.ID] = true
	}
	ignored := map[string]bool{}
	filled := ignoredDefaults(calc, rr.choices, names, ignored)
	basis := firstSetWithin(calc, names)
	if basis == nil && len(filled) > 0 {
		// The values of the request are not enough, but a variable the selected option ignores may still complete a
		// set with its default.
		for _, g := range filled {
			names[g.v.ID] = true
		}
		basis = firstSetWithin(calc, names)
	}
	if basis == nil {
		return CalcResult{}, underdetermined(calc, rr.given, names)
	}
	inBasis := map[string]bool{}
	for _, id := range basis {
		inBasis[id] = true
	}

	in := Inputs{Values: map[string]float64{}, Lists: map[string][]float64{}, Choices: map[string]string{}}
	for _, ch := range rr.choices {
		in.Choices[ch.ID] = ch.Option
	}
	givenByID := map[string]givenValue{}
	for _, g := range append(append([]givenValue{}, rr.given...), filled...) {
		givenByID[g.v.ID] = g
		if !inBasis[g.v.ID] {
			continue
		}
		if g.v.List {
			in.Lists[g.v.ID] = g.list
		} else {
			in.Values[g.v.ID] = g.si
		}
	}
	res := calc.Solve(in)
	computed := map[string]bool{}
	for _, id := range res.Computed {
		computed[id] = true
	}

	out := CalcResult{ID: calc.ID, Name: calc.Name.in("en"), Choices: rr.choices, Basis: append([]string{}, basis...), ESeries: rr.series}
	for i := range calc.Vars {
		v := &calc.Vars[i]
		q := Quantity{ID: v.ID, Label: v.Label, Name: v.Name.in("en"), Kind: "var", Dim: v.Dim, Computed: computed[v.ID]}
		if g, ok := givenByID[v.ID]; ok {
			q.Given, q.Defaulted = g.text, g.filled && inBasis[v.ID]
		}
		q.Ignored = ignored[v.ID]
		if v.List {
			q.SIList = in.Lists[v.ID]
			q.Formatted = c.formatList(q.SIList, v.Dim, v.Unit)
		} else {
			x := res.Values[v.ID]
			q.SI = finitePtr(x)
			q.Formatted = c.formatValue(x, v.Dim, v.Unit)
			if q.Computed && v.ESeries && q.SI != nil && *q.SI > 0 {
				if val, dev, err := Nearest(rr.series, *q.SI); err == nil {
					q.ESeries = &ESeriesHint{Series: rr.series, Value: val, Formatted: c.formatValue(val, v.Dim, v.Unit), DevPct: roundSig(dev, 3)}
				}
			}
			// A redundant given value is compared with the computed one.
			if g, ok := givenByID[v.ID]; ok && !g.filled && !inBasis[v.ID] && q.SI != nil {
				diff := math.Abs(g.si-x) / math.Max(math.Abs(g.si), math.Abs(x))
				if diff > ContradictionTolerance {
					out.Contradictions = append(out.Contradictions, Contradiction{
						Name: v.ID, Given: c.formatValue(g.si, v.Dim, v.Unit), Computed: q.Formatted,
						GivenSI: g.si, ComputedSI: x, DiffPct: roundSig(diff*100, 3),
					})
				}
			}
		}
		if code := res.Errors[v.ID]; code != "" {
			q.Error = errorText(code)
		}
		out.Quantities = append(out.Quantities, q)
	}
	for _, d := range calc.Derived {
		x := res.Values[d.ID]
		q := Quantity{ID: d.ID, Label: d.Label, Name: d.Name.in("en"), Kind: "derived", Dim: d.Dim, Computed: true}
		q.SI = finitePtr(x)
		q.Formatted = c.formatValue(x, d.Dim, d.Unit)
		out.Quantities = append(out.Quantities, q)
	}
	for _, i := range res.Warnings {
		out.Warnings = append(out.Warnings, calc.Checks[i].Warn.in("en"))
	}
	if calc.Finder == finderCombine {
		out.Combos = c.combos(calc, res, rr.series)
	}
	return out, nil
}

// ignoredDefaults marks the variables that the selected choice options ignore in ignored and returns the defaults of
// those the request does not give, so that a value for a variable an option does not use is not asked for. They are
// only used when the given values alone do not make a solvable set. A variable without a default is not filled in.
func ignoredDefaults(calc *Calculator, choices []ChoiceState, names, ignored map[string]bool) []givenValue {
	var out []givenValue
	for _, st := range choices {
		for i := range calc.Choices {
			ch := &calc.Choices[i]
			if ch.ID != st.ID {
				continue
			}
			for _, id := range ch.option(st.Option).Ignore {
				ignored[id] = true
				v := calc.Var(id)
				if v == nil || !v.Input || names[id] {
					continue
				}
				switch {
				case v.DefaultList != nil:
					out = append(out, givenValue{v: v, list: v.DefaultList, filled: true})
				case v.Default != nil:
					out = append(out, givenValue{v: v, si: *v.Default, filled: true})
				}
			}
		}
	}
	return out
}

// comboLimit is the number of series and of parallel combinations the combiner lists, like the app.
const comboLimit = 5

// combos lists the nearest single value and the best series and parallel pairs of the series for the target of a
// standard-value combiner (its one resistance variable). A target that is not a valid resistance has none.
func (c *Catalog) combos(calc *Calculator, res Result, series string) []ComboInfo {
	var target *Var
	for i := range calc.Vars {
		if v := &calc.Vars[i]; v.Dim == "resistance" && !v.List {
			target = v
			break
		}
	}
	if target == nil {
		return nil
	}
	list, err := Combine(series, res.Values[target.ID], comboLimit)
	if err != nil {
		return nil
	}
	out := make([]ComboInfo, len(list))
	for i, cb := range list {
		info := ComboInfo{Kind: cb.Kind, R1: c.formatValue(cb.R1, target.Dim, target.Unit), Value: c.formatValue(cb.Value, target.Dim, target.Unit), DevPct: roundSig(cb.DevPct, 3)}
		if cb.Kind != ComboSingle {
			info.R2 = c.formatValue(cb.R2, target.Dim, target.Unit)
		}
		out[i] = info
	}
	return out
}
