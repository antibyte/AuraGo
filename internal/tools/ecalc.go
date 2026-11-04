package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"aurago/internal/config"
	"aurago/internal/desktop"
	"aurago/internal/ecalc"
)

// The ecalc tool: search, describe, calculate, table (rows and lookups of the reference tables), pinout (the pins of a
// connector, board header or IC package), decode and encode (component codes) on the embedded ECalc catalog (pure
// computation, no I/O) and open the ECalc desktop app pre-filled.
// Only open touches the desktop and is gated like the other desktop-opening tools.

const (
	// ecalcDefaultBudget is the agent's inline floor in bytes, used when the caller passes none.
	ecalcDefaultBudget = 6000
	// ecalcBudgetSlack is kept free for the wrapper the agent puts around a tool answer.
	ecalcBudgetSlack = 600
	ecalcMinBudget   = 1500
	// ecalcSetsLimit is the number of solvable input sets that describe and the under-determined error list when
	// the answer has to be shortened.
	ecalcSetsLimit = 12
	ecalcQueryEcho = 100
	// ecalcMaxQuery is the longest search query in characters; a longer one is refused.
	ecalcMaxQuery = 256
)

// ECalcRequest is a decoded ecalc tool call. Inputs are variable ids with the values as a person would type them
// ("4k7", "12 V"); Choices pair a choice id with an option id. Filter, Offset, Limit and Lookup belong to table (Limit
// also to search; Filter, Offset and Limit to pinout, which adds Pin and Variant); Kind (a code kind), Code, Value, Bands, Tolerance, TempCo, Voltage and Class to decode and encode.
// OutputBudget is the agent's inline limit in bytes (0 = 6,000): search results, descriptions, rows and results are
// shortened to stay below it.
type ECalcRequest struct {
	Operation    string
	Query        string
	ID           string
	Inputs       []ecalc.NamedValue
	Choices      []ecalc.NamedValue
	ESeries      string
	Limit        int
	OutputBudget int

	Filter string
	Offset int
	Lookup *ecalc.TableLookup

	Pin     string
	Variant string

	Kind      string
	Code      string
	Value     string
	Bands     int
	Tolerance string
	TempCo    string
	Voltage   string
	Class     string
}

// ExecuteECalc runs the ecalc tool. Output is a JSON string with status ok, error or policy_denied; open also
// carries the desktop event that opens the app.
func ExecuteECalc(cfg *config.Config, req ECalcRequest) VirtualDesktopExecution {
	cat, err := ecalc.Default()
	if err != nil {
		return ecalcBoundOutput(ecalcFailure("error", "ecalc_unavailable", "The ECalc catalog could not be loaded."), ecalcBudget(req.OutputBudget))
	}
	return executeECalc(cfg, cat, req)
}

// executeECalc runs the tool on a catalog; the tests hand it a catalog with fixture pinouts.
func executeECalc(cfg *config.Config, cat *ecalc.Catalog, req ECalcRequest) (result VirtualDesktopExecution) {
	budget := ecalcBudget(req.OutputBudget)
	defer func() { result = ecalcBoundOutput(result, budget) }()
	switch op := strings.ToLower(strings.TrimSpace(req.Operation)); op {
	case "search":
		return ecalcSearch(cat, req, budget)
	case "describe":
		return ecalcDescribe(cat, req, budget)
	case "calculate":
		return ecalcCalculate(cat, req, budget)
	case "table":
		return ecalcTable(cat, req, budget)
	case "pinout":
		return ecalcPinout(cat, req, budget)
	case "decode":
		return ecalcDecodeCode(cat, req)
	case "encode":
		return ecalcEncodeCode(cat, req)
	case "open":
		return ecalcOpen(cfg, cat, req)
	default:
		return ecalcFailure("error", "invalid_operation", "operation must be search, describe, calculate, table, pinout, decode, encode or open.")
	}
}

func ecalcBudget(requested int) int {
	if requested <= 0 {
		requested = ecalcDefaultBudget
	}
	return max(requested-ecalcBudgetSlack, ecalcMinBudget)
}

// ecalcEncode writes v as one line of JSON without HTML escaping, so that formulas keep their < and > readable.
func ecalcEncode(v any) (string, bool) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return `{"status":"error","code":"ecalc_failed","message":"result could not be encoded"}`, false
	}
	return strings.TrimRight(buf.String(), "\n"), true
}

func ecalcResult(v any, event *desktop.Event) VirtualDesktopExecution {
	out, _ := ecalcEncode(v)
	return VirtualDesktopExecution{Output: out, Event: event}
}

// ecalcFit encodes the answer and applies the shortening steps one after the other until it fits the budget.
func ecalcFit(budget int, build func() any, steps ...func()) VirtualDesktopExecution {
	out, _ := ecalcEncode(build())
	for _, step := range steps {
		if len(out) <= budget {
			break
		}
		step()
		out, _ = ecalcEncode(build())
	}
	return VirtualDesktopExecution{Output: out}
}

type ecalcFailureOut struct {
	Status  string     `json:"status"`
	Code    string     `json:"code"`
	Message string     `json:"message"`
	Sets    [][]string `json:"sets,omitempty"`
}

func ecalcFailure(status, code, message string) VirtualDesktopExecution {
	return ecalcResult(ecalcFailureOut{Status: status, Code: code, Message: message}, nil)
}

// ecalcQueryFailure turns a request error of the ecalc package into the answer, with the solvable sets of an
// under-determined request.
func ecalcQueryFailure(err error) VirtualDesktopExecution {
	qe := ecalc.AsQueryError(err)
	if qe == nil {
		return ecalcFailure("error", "ecalc_failed", "ECalc could not answer this request.")
	}
	out := ecalcFailureOut{Status: "error", Code: qe.Code, Message: qe.Message, Sets: qe.Sets}
	if len(out.Sets) > ecalcSetsLimit {
		out.Sets = out.Sets[:ecalcSetsLimit]
	}
	return ecalcResult(out, nil)
}

func ecalcClip(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(strings.ToValidUTF8(s, "")), " ")
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	return strings.TrimSpace(string([]rune(s)[:maxRunes-1])) + "…"
}

// ---- search ---------------------------------------------------------------------------------------------------

type ecalcSearchOut struct {
	Status     string                  `json:"status"`
	Operation  string                  `json:"operation"`
	Query      string                  `json:"query,omitempty"`
	Total      int                     `json:"total"`
	Returned   int                     `json:"returned"`
	Results    []ecalc.SearchHit       `json:"results,omitempty"`
	Categories []ecalc.CategorySummary `json:"categories,omitempty"`
	Truncated  bool                    `json:"truncated,omitempty"`
	Note       string                  `json:"note,omitempty"`
}

func ecalcSearch(cat *ecalc.Catalog, req ECalcRequest, budget int) VirtualDesktopExecution {
	query := strings.TrimSpace(req.Query)
	// A search is a few words; every word is matched against the whole catalog, so a long text only costs time (100 KB took
	// seconds). Characters are counted, not bytes.
	if utf8.RuneCountInString(query) > ecalcMaxQuery {
		return ecalcFailure("error", "invalid_query", fmt.Sprintf("query must be at most %d characters; search with a few words (a topic, a part or a formula).", ecalcMaxQuery))
	}
	if query == "" {
		return ecalcResult(ecalcSearchOut{
			Status: "ok", Operation: "search", Total: cat.ItemCount(), Categories: cat.Overview(),
			Note: "Give a query (a topic, part, formula or category name, in English or German) to list calculators, tables, codes and pinouts; the categories are above.",
		}, nil)
	}
	all := cat.SearchAll(query)
	limit := req.Limit
	if limit < 1 {
		limit = ecalc.DefaultSearchLimit
	}
	limit = min(limit, ecalc.MaxSearchLimit)
	out := ecalcSearchOut{Status: "ok", Operation: "search", Query: ecalcClip(query, ecalcQueryEcho), Total: len(all)}
	if len(all) > limit {
		out.Results = all[:limit]
		out.Truncated = true
	} else {
		out.Results = all
	}
	if len(all) == 0 {
		out.Note = "No calculator, table, code or pinout matches. Try other words, an English or German term, or a category name; an empty query lists the categories."
	}
	// One step per result: each drops the last hit until the answer fits.
	cut := func() {
		if len(out.Results) > 1 {
			out.Results = out.Results[:len(out.Results)-1]
			out.Truncated = true
		}
	}
	steps := make([]func(), len(out.Results))
	for i := range steps {
		steps[i] = cut
	}
	return ecalcFit(budget, func() any { out.Returned = len(out.Results); return out }, steps...)
}

// ---- describe -------------------------------------------------------------------------------------------------

type ecalcDescribeOut struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	ecalc.Description
	SetsTotal int  `json:"sets_total,omitempty"`
	Truncated bool `json:"truncated,omitempty"`
}

func ecalcDescribe(cat *ecalc.Catalog, req ECalcRequest, budget int) VirtualDesktopExecution {
	kind, id, err := cat.ResolveItem(req.ID)
	if err != nil {
		return ecalcQueryFailure(err)
	}
	switch kind {
	case "table":
		return ecalcDescribeTable(cat, id, budget)
	case "code":
		return ecalcDescribeCode(cat, id)
	case "pinout":
		return ecalcDescribePinout(cat, id, budget)
	}
	d, err := cat.Describe(id)
	if err != nil {
		return ecalcQueryFailure(err)
	}
	out := ecalcDescribeOut{Status: "ok", Operation: "describe", Description: d}
	return ecalcFit(budget, func() any { return out },
		func() { out.Doc, out.Truncated = ecalcClip(out.Doc, 240), true },
		func() { out.Formula, out.Source = nil, ecalcClip(out.Source, 120) },
		func() {
			if len(out.Sets) > ecalcSetsLimit {
				out.SetsTotal, out.Sets = len(out.Sets), out.Sets[:ecalcSetsLimit]
			}
		},
		func() {
			out.Checks = nil
			vars := make([]ecalc.VarInfo, len(out.Vars))
			copy(vars, out.Vars)
			for i := range vars {
				vars[i].Units = nil
			}
			out.Vars = vars
		},
		func() { out.Doc, out.Source, out.Derived, out.Chart = "", "", nil, nil },
		func() {
			for i := range out.Vars {
				out.Vars[i].Label, out.Vars[i].Name = "", ""
			}
		},
	)
}

// ---- calculate ------------------------------------------------------------------------------------------------

type ecalcCalcOut struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	ecalc.CalcResult
}

func ecalcRequestOf(req ECalcRequest) ecalc.CalcRequest {
	return ecalc.CalcRequest{Values: req.Inputs, Choices: req.Choices, ESeries: req.ESeries}
}

func ecalcCalculate(cat *ecalc.Catalog, req ECalcRequest, budget int) VirtualDesktopExecution {
	res, err := cat.Calculate(req.ID, ecalcRequestOf(req))
	if err != nil {
		return ecalcQueryFailure(err)
	}
	out := ecalcCalcOut{Status: "ok", Operation: "calculate", CalcResult: res}
	var notes []string
	if n := len(res.Contradictions); n > 0 {
		names := make([]string, n)
		for i, c := range res.Contradictions {
			names[i] = c.Name
		}
		notes = append(notes, fmt.Sprintf("Redundant values contradict the result by more than %g %%: %s. The result was computed from %s; see contradictions.",
			ecalc.ContradictionTolerance*100, strings.Join(names, ", "), strings.Join(res.Basis, ", ")))
	}
	if res.HasErrors() {
		var bad []string
		for _, q := range res.Quantities {
			if q.Error != "" {
				bad = append(bad, q.ID+" ("+q.Error+")")
			}
		}
		out.Status, out.Code = "error", "calculation_failed"
		notes = append(notes, "Some quantities could not be computed: "+strings.Join(bad, "; ")+".")
	}
	out.Message = strings.Join(notes, " ")
	quantities := func(f func(q *ecalc.Quantity)) func() {
		return func() {
			qs := make([]ecalc.Quantity, len(out.Quantities))
			copy(qs, out.Quantities)
			for i := range qs {
				f(&qs[i])
			}
			out.Quantities = qs
		}
	}
	return ecalcFit(budget, func() any { return out },
		quantities(func(q *ecalc.Quantity) { q.Label = "" }),
		quantities(func(q *ecalc.Quantity) { q.Name = "" }),
		quantities(func(q *ecalc.Quantity) { q.Dim, q.SIList = "", nil }),
		func() { out.Combos = ecalcTrimCombos(out.Combos, 2) },
	)
}

// ecalcTrimCombos keeps the single value and the first per entries of every other kind of the combiner's list.
func ecalcTrimCombos(combos []ecalc.ComboInfo, per int) []ecalc.ComboInfo {
	var out []ecalc.ComboInfo
	count := map[string]int{}
	for _, c := range combos {
		if count[c.Kind]++; c.Kind == ecalc.ComboSingle || count[c.Kind] <= per {
			out = append(out, c)
		}
	}
	return out
}

// ---- tables ---------------------------------------------------------------------------------------------------

type ecalcDescribeTableOut struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	ecalc.TableDescription
	Truncated bool `json:"truncated,omitempty"`
}

func ecalcDescribeTable(cat *ecalc.Catalog, id string, budget int) VirtualDesktopExecution {
	d, err := cat.DescribeTable(id)
	if err != nil {
		return ecalcQueryFailure(err)
	}
	out := ecalcDescribeTableOut{Status: "ok", Operation: "describe", TableDescription: d}
	return ecalcFit(budget, func() any { return out },
		func() { out.Doc, out.Notes, out.Truncated = ecalcClip(out.Doc, 240), ecalcClip(out.Notes, 240), true },
		func() {
			cols := make([]ecalc.TableColumnInfo, len(out.Columns))
			copy(cols, out.Columns)
			for i := range cols {
				cols[i].Units = nil
			}
			out.Columns = cols
		},
		func() { out.Source = ecalcClip(out.Source, 120) },
		// Beyond the plan, like the further steps of the calculator description: a table with many columns can still be too
		// long for the smallest budget, and the outer bound of ExecuteECalc would then refuse it. The optional text goes
		// first, then the column names and dimensions (the ids and units stay), then the text of the caution (the guide and
		// mains flags stay).
		func() { out.Doc, out.Notes, out.Source = "", "", "" },
		func() {
			cols := make([]ecalc.TableColumnInfo, len(out.Columns))
			copy(cols, out.Columns)
			for i := range cols {
				cols[i].Name = ""
			}
			out.Columns = cols
		},
		func() {
			cols := make([]ecalc.TableColumnInfo, len(out.Columns))
			copy(cols, out.Columns)
			for i := range cols {
				cols[i].Dim = ""
			}
			out.Columns = cols
		},
		func() { out.Caution = "" },
	)
}

type ecalcTableOut struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	ecalc.TableResult
}

// ecalcTable answers the rows and the lookup of a table. When the answer is over the budget it first leaves out the SI
// matrix (the rows hold every value with its unit), then shortens the notes and the source, then lists fewer rows (the
// answer says where to continue). A wide table can leave one row that is still too long, and the outer bound of
// ExecuteECalc would answer output_budget_exceeded for a page that can be shortened; so the descriptive parts go next
// (column names, notes, source, the row of a lookup that names its output column), and as the very last resorts the
// listed row and the column list. The row is only given up for a budget that ExecuteECalc can really hand over (from
// ecalcMinBudget); below it one row always stays.
func ecalcTable(cat *ecalc.Catalog, req ECalcRequest, budget int) VirtualDesktopExecution {
	res, err := cat.QueryTable(req.ID, ecalc.TableRequest{Filter: req.Filter, Offset: req.Offset, Limit: req.Limit, Lookup: req.Lookup})
	if err != nil {
		return ecalcQueryFailure(err)
	}
	out := ecalcTableOut{Status: "ok", Operation: "table", TableResult: res}
	note, shortened, advice := res.Note, []string{}, ""
	build := func() any {
		out.Note = note
		if len(shortened) > 0 {
			out.Note = strings.TrimSpace(note + " Shortened to fit the output limit: " + strings.Join(shortened, ", ") + "." + advice)
		}
		return out
	}
	steps := []func(){
		func() { out.SI, shortened = nil, append(shortened, "si values left out") },
		func() {
			out.Notes, out.Source, shortened = ecalcClip(out.Notes, 160), ecalcClip(out.Source, 120), append(shortened, "notes and source clipped")
		},
	}
	for range len(out.Rows) {
		steps = append(steps, func() {
			if n := len(out.Rows); n > 1 {
				out.Keep(n - 1)
			}
		})
	}
	steps = append(steps,
		func() {
			cols := make([]ecalc.TableColumnInfo, len(out.Columns))
			copy(cols, out.Columns)
			for i := range cols {
				cols[i].Name = ""
			}
			out.Columns, shortened = cols, append(shortened, "column names left out")
		},
		func() { out.Notes, out.Source, shortened = "", "", append(shortened, "notes and source left out") },
		func() {
			if lk := out.Lookup; lk != nil && lk.Output != "" && lk.Row != nil {
				dropped := *lk
				dropped.Row = nil
				out.Lookup, shortened = &dropped, append(shortened, "lookup row left out")
			}
		},
	)
	if budget >= ecalcMinBudget {
		steps = append(steps,
			func() {
				if len(out.Rows) > 0 {
					out.Keep(0)
					shortened = append(shortened, "rows left out")
					advice = " No row fits the output limit: use a lookup that names its output column to read a single cell."
				}
			},
			func() {
				if lk := out.Lookup; lk != nil && lk.Row != nil {
					dropped := *lk
					dropped.Row = nil
					dropped.Message = strings.TrimSpace(dropped.Message + " The row does not fit the output limit; name an output column in the lookup.")
					out.Lookup, shortened = &dropped, append(shortened, "lookup row left out")
				}
			},
			func() { out.Columns, shortened = []ecalc.TableColumnInfo{}, append(shortened, "column list left out") },
		)
	}
	return ecalcFit(budget, build, steps...)
}

// ---- pinouts --------------------------------------------------------------------------------------------------

type ecalcDescribePinoutOut struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	ecalc.PinoutDescription
	Truncated bool `json:"truncated,omitempty"`
}

// ecalcDescribePinout answers what a pinout is, without its pins. A long description loses its optional text first (the
// texts are clipped, then the per-variant counts, then the texts go), the view last.
func ecalcDescribePinout(cat *ecalc.Catalog, id string, budget int) VirtualDesktopExecution {
	d, err := cat.DescribePinout(id)
	if err != nil {
		return ecalcQueryFailure(err)
	}
	out := ecalcDescribePinoutOut{Status: "ok", Operation: "describe", PinoutDescription: d}
	return ecalcFit(budget, func() any { return out },
		func() { out.Doc, out.Notes, out.Truncated = ecalcClip(out.Doc, 240), ecalcClip(out.Notes, 240), true },
		func() { out.Source = ecalcClip(out.Source, 120) },
		func() {
			variants := make([]ecalc.PinoutVariantInfo, len(out.Variants))
			copy(variants, out.Variants)
			for i := range variants {
				variants[i].Counts = nil
			}
			out.Variants = variants
		},
		func() { out.Doc, out.Notes, out.Source = "", "", "" },
		func() { out.View = "" },
	)
}

type ecalcPinoutOut struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	ecalc.PinoutResult
}

// ecalcPinout answers the pins of a pinout. When the answer is over the budget it shortens the notes first (the clipped
// notes and source, the notes of the pins, the notes of the pinout), then lists fewer pins (the answer says where to
// continue), then leaves out the source and, as the last resort for a budget that ExecuteECalc can really hand over (from
// ecalcMinBudget), the listed pins themselves. A page of pins is shortened, never refused for its size.
func ecalcPinout(cat *ecalc.Catalog, req ECalcRequest, budget int) VirtualDesktopExecution {
	res, err := cat.QueryPinout(req.ID, ecalc.PinoutRequest{Pin: req.Pin, Variant: req.Variant, Filter: req.Filter, Offset: req.Offset, Limit: req.Limit})
	if err != nil {
		return ecalcQueryFailure(err)
	}
	out := ecalcPinoutOut{Status: "ok", Operation: "pinout", PinoutResult: res}
	note, shortened, advice := res.Note, []string{}, ""
	build := func() any {
		out.Note = note
		if len(shortened) > 0 {
			out.Note = strings.TrimSpace(note + " Shortened to fit the output limit: " + strings.Join(shortened, ", ") + "." + advice)
		}
		return out
	}
	steps := []func(){
		func() {
			out.Notes, out.Source, shortened = ecalcClip(out.Notes, 160), ecalcClip(out.Source, 120), append(shortened, "notes and source clipped")
		},
		func() {
			pins := make([]ecalc.PinAnswer, len(out.Pins))
			copy(pins, out.Pins)
			dropped := false
			for i := range pins {
				dropped = dropped || pins[i].Notes != ""
				pins[i].Notes = ""
			}
			out.Pins = pins
			if dropped {
				shortened = append(shortened, "pin notes left out")
			}
		},
		func() {
			if out.Notes != "" {
				out.Notes, shortened = "", append(shortened, "pinout notes left out")
			}
		},
	}
	for range len(out.Pins) {
		steps = append(steps, func() {
			if n := len(out.Pins); n > 1 {
				out.Keep(n - 1)
			}
		})
	}
	steps = append(steps, func() { out.Source, shortened = "", append(shortened, "source left out") })
	if budget >= ecalcMinBudget {
		steps = append(steps, func() {
			if len(out.Pins) > 0 {
				out.Keep(0)
				shortened = append(shortened, "pins left out")
				advice = " No pin fits the output limit: name one with pin."
			}
		})
	}
	return ecalcFit(budget, build, steps...)
}

// ---- codes ----------------------------------------------------------------------------------------------------

type ecalcDescribeCodeOut struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	ecalc.CodeDescription
}

func ecalcDescribeCode(cat *ecalc.Catalog, id string) VirtualDesktopExecution {
	d, err := cat.DescribeCode(id)
	if err != nil {
		return ecalcQueryFailure(err)
	}
	return ecalcResult(ecalcDescribeCodeOut{Status: "ok", Operation: "describe", CodeDescription: d}, nil)
}

type ecalcCodeOut struct {
	Status    string `json:"status"`
	Operation string `json:"operation"`
	ecalc.CodeAnswer
}

func ecalcDecodeCode(cat *ecalc.Catalog, req ECalcRequest) VirtualDesktopExecution {
	res, err := cat.QueryDecode(req.Kind, req.Code)
	if err != nil {
		return ecalcQueryFailure(err)
	}
	return ecalcResult(ecalcCodeOut{Status: "ok", Operation: "decode", CodeAnswer: res}, nil)
}

func ecalcEncodeCode(cat *ecalc.Catalog, req ECalcRequest) VirtualDesktopExecution {
	res, err := cat.QueryEncode(req.Kind, ecalc.EncodeRequest{
		Value: req.Value, Bands: req.Bands, Tolerance: req.Tolerance, TempCo: req.TempCo, Voltage: req.Voltage, Class: req.Class,
	})
	if err != nil {
		return ecalcQueryFailure(err)
	}
	return ecalcResult(ecalcCodeOut{Status: "ok", Operation: "encode", CodeAnswer: res}, nil)
}

// ---- open -----------------------------------------------------------------------------------------------------

// ecalcOrdered is a JSON object whose keys keep their order: the app takes the order of the values as the order in
// which they were typed (the most recent ones win when more values are given than the calculator needs).
type ecalcOrdered []ecalc.NamedValue

func (o ecalcOrdered) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i, nv := range o {
		if i > 0 {
			buf.WriteByte(',')
		}
		key, _ := json.Marshal(nv.Name)
		val, _ := json.Marshal(nv.Value)
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(val)
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

type ecalcOpenOut struct {
	Status    string             `json:"status"`
	Operation string             `json:"operation"`
	Page      string             `json:"page,omitempty"`
	Values    []ecalc.NamedValue `json:"values,omitempty"`
	Choices   []ecalc.NamedValue `json:"choices,omitempty"`
	Message   string             `json:"message"`
}

func ecalcOpen(cfg *config.Config, cat *ecalc.Catalog, req ECalcRequest) VirtualDesktopExecution {
	if cfg == nil || !cfg.VirtualDesktop.Enabled {
		return ecalcFailure("policy_denied", "desktop_unavailable", "The virtual desktop is switched off, so ECalc cannot be opened.")
	}
	if !cfg.VirtualDesktop.AllowAgentControl || !cfg.Tools.VirtualDesktop.Enabled {
		return ecalcFailure("policy_denied", "desktop_agent_control_disabled", "The user has not allowed the agent to control the desktop, so ECalc cannot be opened. Give the user the numbers from calculate instead.")
	}
	if strings.TrimSpace(req.ID) == "" && len(req.Inputs) == 0 && len(req.Choices) == 0 {
		event := virtualDesktopEvent("open_app", map[string]interface{}{"app_id": "ecalc"})
		return ecalcResult(ecalcOpenOut{Status: "ok", Operation: "open", Message: "Asked the desktop to open the ECalc app."}, event)
	}
	kind, id, err := cat.ResolveItem(req.ID)
	if err != nil {
		return ecalcQueryFailure(err)
	}
	if kind != "calculator" {
		return ecalcOpenItem(kind, id, req, cat)
	}
	checked, err := cat.Check(id, ecalcRequestOf(req))
	if err != nil {
		return ecalcQueryFailure(err)
	}
	launch := map[string]interface{}{
		"page":    checked.ID,
		"values":  ecalcOrdered(checked.Values),
		"choices": ecalcOrdered(checked.Choices),
	}
	event := virtualDesktopEvent("open_app", map[string]interface{}{"app_id": "ecalc", "ecalc": launch})
	message := "Asked the desktop to open ECalc at the " + cat.Calculator(checked.ID).Name["en"] + " calculator"
	switch n := len(checked.Values); {
	case n == 1:
		message += ", pre-filled with 1 value"
	case n > 1:
		message += fmt.Sprintf(", pre-filled with %d values", n)
	}
	message += ". Nothing was computed; use calculate to get the numbers yourself."
	return ecalcResult(ecalcOpenOut{
		Status: "ok", Operation: "open", Page: checked.ID, Values: checked.Values, Choices: checked.Choices, Message: message,
	}, event)
}

// ecalcOpenItem opens the page of a table, a code or a pinout. Only calculators take values and choices; the page of anything else
// opens as it is.
func ecalcOpenItem(kind, id string, req ECalcRequest, cat *ecalc.Catalog) VirtualDesktopExecution {
	if len(req.Inputs) > 0 || len(req.Choices) > 0 {
		return ecalcFailure("error", ecalc.CodeNotACalculator, fmt.Sprintf("inputs and choices are only for calculators; %s is a %s, so open it with its id alone.", id, kind))
	}
	name, advice := "", ""
	switch kind {
	case "table":
		name, advice = cat.Table(id).Name["en"], " Use table to read its rows yourself."
	case "code":
		name, advice = cat.Code(id).Name["en"], " Use decode and encode to work with the code yourself."
	case "pinout":
		name, advice = cat.Pinout(id).Name["en"], " Use pinout to read its pins yourself."
	}
	launch := map[string]interface{}{"page": id, "values": ecalcOrdered(nil), "choices": ecalcOrdered(nil)}
	event := virtualDesktopEvent("open_app", map[string]interface{}{"app_id": "ecalc", "ecalc": launch})
	return ecalcResult(ecalcOpenOut{
		Status: "ok", Operation: "open", Page: id, Message: "Asked the desktop to open ECalc at the " + name + " " + kind + "." + advice,
	}, event)
}
