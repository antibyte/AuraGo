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

// The ecalc tool: search, describe and calculate on the embedded ECalc catalog (pure computation, no I/O) and open
// the ECalc desktop app pre-filled. Only open touches the desktop and is gated like the other desktop-opening tools.

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
)

// ECalcRequest is a decoded ecalc tool call. Inputs are variable ids with the values as a person would type them
// ("4k7", "12 V"); Choices pair a choice id with an option id. OutputBudget is the agent's inline limit in bytes
// (0 = 6,000): search results, descriptions and results are shortened to stay below it.
type ECalcRequest struct {
	Operation    string
	Query        string
	ID           string
	Inputs       []ecalc.NamedValue
	Choices      []ecalc.NamedValue
	ESeries      string
	Limit        int
	OutputBudget int
}

// ExecuteECalc runs the ecalc tool. Output is a JSON string with status ok, error or policy_denied; open also
// carries the desktop event that opens the app.
func ExecuteECalc(cfg *config.Config, req ECalcRequest) VirtualDesktopExecution {
	cat, err := ecalc.Default()
	if err != nil {
		return ecalcFailure("error", "ecalc_unavailable", "The ECalc catalog could not be loaded.")
	}
	budget := ecalcBudget(req.OutputBudget)
	switch op := strings.ToLower(strings.TrimSpace(req.Operation)); op {
	case "search":
		return ecalcSearch(cat, req, budget)
	case "describe":
		return ecalcDescribe(cat, req, budget)
	case "calculate":
		return ecalcCalculate(cat, req, budget)
	case "open":
		return ecalcOpen(cfg, cat, req)
	default:
		return ecalcFailure("error", "invalid_operation", "operation must be search, describe, calculate or open.")
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
	if query == "" {
		return ecalcResult(ecalcSearchOut{
			Status: "ok", Operation: "search", Total: len(cat.Calculators), Categories: cat.Overview(),
			Note: "Give a query (a topic, part, formula or category name, in English or German) to list calculators; the categories are above.",
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
		out.Note = "No calculator matches. Try other words, an English or German term, or a category name; an empty query lists the categories."
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
	d, err := cat.Describe(req.ID)
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
	checked, err := cat.Check(req.ID, ecalcRequestOf(req))
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
