package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"aurago/internal/cydcreator"
	"aurago/internal/security"
)

// cydCreatorWriteTools change the working copy or start builds; they are
// refused while the desktop is read-only.
var cydCreatorWriteTools = []string{cydcreator.ToolUpdateLayout, cydcreator.ToolWriteFile, cydcreator.ToolDeleteFile,
	cydcreator.ToolBuild, cydcreator.ToolSubmitPlan, cydcreator.ToolFinish}

func cydCreatorToolJSON(value any) string {
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return toolErrorJSON("could not serialize the CYD Creator result")
	}
	return "Tool Output: " + strings.TrimSuffix(data.String(), "\n")
}

// cydCreatorToolError reports a service error with its CYD_* code and details
// so the agent can correct the call.
func cydCreatorToolError(logger *slog.Logger, err error) string {
	var typed *cydcreator.Error
	if errors.As(err, &typed) {
		body := map[string]any{"status": "error", "code": typed.Code, "message": typed.Message}
		if len(typed.Details) > 0 {
			body["details"] = typed.Details
		}
		return cydCreatorToolJSON(body)
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return cydCreatorToolJSON(map[string]any{"status": "cancelled", "message": "the job was stopped"})
	}
	// An untyped error can carry host paths: log it, answer generically (like
	// the service's CYD_INTERNAL).
	if logger != nil {
		logger.Warn("CYD Creator tool failed", "error", err)
	}
	return cydCreatorToolJSON(map[string]any{"status": "error", "code": cydcreator.CodeInternal, "message": "the CYD Creator tool failed"})
}

func cydCreatorPolicyDenied(message string) string {
	return cydCreatorToolJSON(map[string]any{"status": "policy_denied", "message": message})
}

func cydCreatorBadRequest(message string, details map[string]any) error {
	return cydcreator.NewError(cydcreator.CodeBadRequest, message, details)
}

// cydLooseObjectKeys are object-valued layout op fields a provider may send as
// JSON strings (provider-fragile free objects become string schemas).
var cydLooseObjectKeys = map[string]bool{"widget": true, "screen_data": true, "set": true, "props": true, "style": true, "action": true}

func decodeCYDLooseObjects(v any) any {
	switch typed := v.(type) {
	case map[string]any:
		for key, value := range typed {
			if s, ok := value.(string); ok && cydLooseObjectKeys[key] {
				var obj map[string]any
				if trimmed := strings.TrimSpace(s); strings.HasPrefix(trimmed, "{") && json.Unmarshal([]byte(trimmed), &obj) == nil {
					value = obj
				}
			}
			typed[key] = decodeCYDLooseObjects(value)
		}
		return typed
	case []any:
		for i := range typed {
			typed[i] = decodeCYDLooseObjects(typed[i])
		}
		return typed
	}
	return v
}

// decodeCYDLayoutOps accepts the ops array (or a JSON string holding it) with
// string-encoded nested objects and decodes it strictly into LayoutOps.
func decodeCYDLayoutOps(raw any) ([]cydcreator.LayoutOp, error) {
	if s, ok := raw.(string); ok {
		var decoded any
		if err := json.Unmarshal([]byte(s), &decoded); err != nil {
			return nil, fmt.Errorf("ops is not a JSON array: %w", err)
		}
		raw = decoded
	}
	list, ok := raw.([]any)
	if !ok || len(list) == 0 {
		return nil, errors.New("ops must be a non-empty array")
	}
	data, err := json.Marshal(decodeCYDLooseObjects(list))
	if err != nil {
		return nil, err
	}
	var ops []cydcreator.LayoutOp
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ops); err != nil {
		return nil, fmt.Errorf("invalid op: %w", err)
	}
	if err := dec.Decode(new(json.RawMessage)); err != io.EOF {
		return nil, errors.New("invalid ops: trailing data")
	}
	return ops, nil
}

// ---- output budget and paging ----

const (
	// cydReadPageBytes caps the content of one cyd_creator_read_file page.
	cydReadPageBytes = 32 << 10
	// cydMinPageBytes is the smallest page budget; an inline budget that
	// leaves less (a tool_output_limit or vault inline limit below about
	// 1 KB) gets a CYD_FILE_LIMIT error naming the minimum instead.
	cydMinPageBytes = 256
	// cydBudgetSlack absorbs the few bytes the dispatch pipeline may add.
	cydBudgetSlack = 64
	// cydMaxLineArg bounds start_line/end_line like game_maker_file read.
	cydMaxLineArg = 10000000
)

// cydCreatorOutputBudget is the largest answer, in cydModelBytes, that reaches
// the model whole: the agent's inline budget (tool_output_limit and, while the
// primary output vault is on, its archive threshold), like local_wikipedia.
// A bigger answer would be replaced by a bare truncation stub, or archived
// where a CYD Creator run cannot read it back.
func cydCreatorOutputBudget(dc *DispatchContext) int {
	return toolResultInlineBudget(dc) - cydBudgetSlack
}

// cydModelBytes measures an answer as the Guardian hands it to the model, in
// its largest form: the escaped IsolateExternalData fallback (used for
// injection-shaped source) is never shorter than the copyable form, plus the
// longer of the "[SECURITY WARNING: ...]" lines either form would get.
func cydModelBytes(out string) int {
	escaped := security.IsolateExternalData(out)
	warning := max(len(security.ToolOutputWarning(escaped)), len(security.ToolOutputWarning(security.IsolateSourceData(out))))
	return len(escaped) + warning
}

// cydOutputLimitError reports an inline budget too small for the smallest
// answer of a tool; needed is that answer's size in cydModelBytes.
func cydOutputLimitError(needed int) error {
	minimum := needed + cydBudgetSlack
	return cydcreator.NewError(cydcreator.CodeFileLimit,
		fmt.Sprintf("the agent's tool output limit is too small for this CYD Creator result; agent.tool_output_limit (and the output vault's inline limit) must be at least %d bytes", minimum),
		map[string]any{"limit": "tool_output_limit", "min": minimum})
}

// cydEscapedLen is the share of s in cydModelBytes when s is a JSON string
// value: JSON escaping (without HTML escaping) followed by HTML escaping. Both
// work per character, so the cost of a concatenation is the sum of the parts.
func cydEscapedLen(s string) int {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	body := bytes.TrimSuffix(b.Bytes(), []byte("\n"))
	return len(html.EscapeString(string(body[1 : len(body)-1])))
}

// cydLineArg reads an optional nonnegative integer argument (a JSON number or
// numeric text, as game_maker_file read accepts); absent or null is 0.
func cydLineArg(params map[string]any, name string) (int, error) {
	value, ok := params[name]
	if !ok || value == nil {
		return 0, nil
	}
	var n float64
	valid := true
	switch v := value.(type) {
	case float64:
		n = v
	case int:
		n = float64(v)
	case int64:
		n = float64(v)
	case json.Number:
		f, err := v.Float64()
		n, valid = f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		n, valid = f, err == nil
	default:
		valid = false
	}
	if !valid || math.IsNaN(n) || n < 0 || n > cydMaxLineArg || n != math.Trunc(n) {
		return 0, cydCreatorBadRequest(name+" must be a nonnegative integer", nil)
	}
	return int(n), nil
}

// cydFilePage is one cyd_creator_read_file window of whole lines. A single
// line longer than the page is cut at a character boundary (ClippedBytes > 0).
type cydFilePage struct {
	Content      string
	StartLine    int
	EndLine      int
	TotalLines   int
	Truncated    bool // the page ends before the requested range (or the file)
	Clipped      bool // line EndLine was cut after ClippedBytes of LineBytes
	ClippedBytes int
	LineBytes    int
}

// cydSplitLines splits after every "\n" (line endings stay in the lines); a
// final newline does not start another line.
func cydSplitLines(content string) []string {
	lines := strings.SplitAfter(content, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// pageCYDText selects lines start..end (1-based, inclusive; start 0 = 1, end 0
// = as far as the budget reaches; an end beyond the file is clamped) whose
// cydEscapedLen fits budget.
func pageCYDText(content string, start, end, budget int) (cydFilePage, error) {
	lines := cydSplitLines(content)
	total := len(lines)
	if start == 0 {
		start = 1
	}
	if end != 0 && end < start {
		return cydFilePage{}, cydCreatorBadRequest("end_line must not be before start_line",
			map[string]any{"start_line": start, "end_line": end})
	}
	if total == 0 && start == 1 {
		return cydFilePage{}, nil
	}
	if start > total {
		return cydFilePage{}, cydCreatorBadRequest(fmt.Sprintf("start_line is beyond the end of the file (%d lines)", total),
			map[string]any{"start_line": start, "total_lines": total})
	}
	last := total
	if end != 0 && end < total {
		last = end
	}
	page := cydFilePage{StartLine: start, TotalLines: total}
	used, stop := 0, start-1
	for i := start - 1; i < last; i++ {
		cost := cydEscapedLen(lines[i])
		if used+cost > budget {
			break
		}
		used += cost
		stop = i + 1
	}
	if stop < start {
		// The first line alone is longer than a page: cut it at the last
		// character boundary that fits (the cost grows with the prefix).
		line := lines[start-1]
		bounds := make([]int, 0, len(line)+1)
		for i := range line {
			bounds = append(bounds, i)
		}
		bounds = append(bounds, len(line))
		lo, hi := 0, len(bounds)-1 // the empty prefix fits, the whole line does not
		for hi-lo > 1 {
			mid := (lo + hi) / 2
			if cydEscapedLen(line[:bounds[mid]]) <= budget {
				lo = mid
			} else {
				hi = mid
			}
		}
		page.Content, page.EndLine, page.Truncated, page.Clipped = line[:bounds[lo]], start, true, true
		page.ClippedBytes, page.LineBytes = bounds[lo], len(line)
		return page, nil
	}
	page.Content = strings.Join(lines[start-1:stop], "")
	page.EndLine = stop
	page.Truncated = stop < last
	return page, nil
}

// cydPageNextAction tells the agent how to continue after a page ("" when the
// page is complete); clippedNote follows the clipped-line sentence.
func cydPageNextAction(page cydFilePage, clippedNote string) string {
	more := ""
	if page.EndLine < page.TotalLines {
		more = fmt.Sprintf(" Read the next range with start_line=%d.", page.EndLine+1)
	}
	switch {
	case page.Clipped:
		return fmt.Sprintf("Line %d is longer than one page and was cut after %d of %d bytes; the rest of that line cannot be read%s.%s",
			page.EndLine, page.ClippedBytes, page.LineBytes, clippedNote, more)
	case more != "":
		return fmt.Sprintf("Lines %d-%d of %d shown.%s", page.StartLine, page.EndLine, page.TotalLines, more)
	}
	return ""
}

// cydReadFileAnswer is the cyd_creator_read_file answer for one page; sha256
// and total_bytes describe the complete file.
func cydReadFileAnswer(file cydcreator.JobFile, page cydFilePage) map[string]any {
	f := map[string]any{"path": file.Path, "content": page.Content, "sha256": file.SHA256, "generated": file.Generated,
		"start_line": page.StartLine, "end_line": page.EndLine, "total_lines": page.TotalLines,
		"total_bytes": len(file.Content), "truncated": page.Truncated}
	answer := map[string]any{"status": "ok", "file": f}
	if page.Clipped {
		f["clipped_line"] = map[string]any{"line": page.EndLine, "shown_bytes": page.ClippedBytes, "line_bytes": page.LineBytes}
	}
	if next := cydPageNextAction(page, ", so never write this page back as the complete file (rewrite it with shorter lines)"); next != "" {
		answer["next_action"] = next
	}
	return answer
}

// cydReferenceAnswer is the cyd_creator_api_reference answer for one page of
// a topic.
func cydReferenceAnswer(topic string, page cydFilePage) map[string]any {
	answer := map[string]any{"status": "ok", "topic": topic, "reference": page.Content, "start_line": page.StartLine,
		"end_line": page.EndLine, "total_lines": page.TotalLines, "truncated": page.Truncated}
	if page.Clipped {
		answer["clipped_line"] = map[string]any{"line": page.EndLine, "shown_bytes": page.ClippedBytes, "line_bytes": page.LineBytes}
	}
	if next := cydPageNextAction(page, ""); next != "" {
		answer["next_action"] = strings.Replace(next, "start_line=", "topic "+topic+" and start_line=", 1)
	}
	return answer
}

// cydPagedAnswer pages text into budget: the envelope (answer for an empty
// page with the largest counters) is measured first, then the content gets
// the rest (at most cydReadPageBytes). Flagged phrases add a security warning
// line that grows with the page, so the measured answer shrinks the page
// until it fits.
func cydPagedAnswer(text string, start, end, budget int, answerFor func(cydFilePage) map[string]any) (map[string]any, error) {
	big := cydFilePage{StartLine: cydMaxLineArg, EndLine: cydMaxLineArg - 1, TotalLines: cydMaxLineArg, Truncated: true,
		Clipped: true, ClippedBytes: cydMaxLineArg, LineBytes: cydMaxLineArg}
	envelope := cydModelBytes(cydCreatorToolJSON(answerFor(big)))
	contentBudget := min(cydReadPageBytes, budget-envelope)
	if contentBudget < cydMinPageBytes {
		return nil, cydOutputLimitError(envelope + cydMinPageBytes)
	}
	for {
		page, err := pageCYDText(text, start, end, contentBudget)
		if err != nil {
			return nil, err
		}
		answer := answerFor(page)
		over := cydModelBytes(cydCreatorToolJSON(answer)) - budget
		if over <= 0 {
			return answer, nil
		}
		if contentBudget <= 1 {
			return nil, cydOutputLimitError(budget + over)
		}
		// Warnings grow with the flagged lines on the page: scale the page
		// down by the overflow ratio (the envelope makes this conservative).
		contentBudget = max(min(contentBudget-1, contentBudget*budget/(budget+over)), 1)
	}
}

// cydReadFilePage pages one file for cyd_creator_read_file.
func cydReadFilePage(file cydcreator.JobFile, start, end, budget int) (map[string]any, error) {
	return cydPagedAnswer(file.Content, start, end, budget, func(page cydFilePage) map[string]any {
		return cydReadFileAnswer(file, page)
	})
}

// cydClipText shortens s to at most runes characters plus "…".
func cydClipText(s string, runes int) string {
	if r := []rune(s); len(r) > runes {
		return string(r[:runes]) + "…"
	}
	return s
}

// cydLayoutUpdateAnswer is the cyd_creator_update_layout answer. The ops are
// applied already, so it never turns into an error: when it does not fit the
// budget the longer of results and issues is halved (*_total counts).
func cydLayoutUpdateAnswer(update cydcreator.LayoutUpdate, budget int) map[string]any {
	results, issues := update.Results, update.Issues
	build := func() map[string]any {
		answer := map[string]any{"status": "ok", "results": results, "issues": issues}
		if len(results) < len(update.Results) || len(issues) < len(update.Issues) {
			answer["results_total"], answer["issues_total"] = len(update.Results), len(update.Issues)
			answer["next_action"] = "The ops were applied; lists ending early show their *_total. cyd_creator_read_project lists the issues."
		}
		return answer
	}
	answer := build()
	for cydModelBytes(cydCreatorToolJSON(answer)) > budget && len(results)+len(issues) > 0 {
		if len(issues) >= len(results) {
			issues = issues[:len(issues)/2]
		} else {
			results = results[:len(results)/2]
		}
		answer = build()
	}
	return answer
}

// cydBuildAnswer is the cyd_creator_build answer. Diagnostics come in
// OrderAgentDiagnostics order; when the answer does not fit the budget the
// list is halved down to the entries it must keep (first error, library
// notes), then the kept messages are shortened (*_total counts). The status
// always stays.
func cydBuildAnswer(res cydcreator.JobBuildResult, budget int) map[string]any {
	status := "failed"
	if res.Status == cydcreator.BuildSucceeded {
		status = "ok"
	}
	ordered, keep := cydcreator.OrderAgentDiagnostics(res.Diagnostics)
	total := max(res.DiagnosticsTotal, len(ordered)) // before the service's cap
	n, messageRunes := len(ordered), 0               // 0 = full messages
	clipFiles := false
	build := func() map[string]any {
		diags := slices.Clone(ordered[:n])
		for i := range diags {
			if messageRunes > 0 {
				diags[i].Message = cydClipText(diags[i].Message, messageRunes)
			}
			if clipFiles {
				// A #line directive can name a file of up to 1 KB.
				diags[i].File = cydClipBytes(diags[i].File, cydMaxDiagFileBytes)
			}
		}
		answer := map[string]any{"status": status, "build_status": res.Status, "build_id": res.BuildID,
			"error_code": res.ErrorCode, "diagnostics": diags, "usage": res.Usage, "next_action": res.NextAction}
		if n < total || messageRunes > 0 || clipFiles {
			answer["diagnostics_total"] = total
			if messageRunes > 0 {
				answer["messages_clipped"] = true
			}
			if clipFiles {
				answer["files_clipped"] = true
			}
		}
		return answer
	}
	answer := build()
	for cydModelBytes(cydCreatorToolJSON(answer)) > budget {
		switch {
		case n > keep:
			n = max(keep, n/2)
		case messageRunes == 0:
			messageRunes = 256
		case messageRunes > 32:
			messageRunes /= 4
		case !clipFiles:
			clipFiles = true
		case n > 1:
			n /= 2 // last resort: library notes go, the first entry stays
		default:
			return answer // the agent's bound keeps the status
		}
		answer = build()
	}
	return answer
}

// cydMaxDiagFileBytes bounds a diagnostic's file name in a shortened build
// answer.
const cydMaxDiagFileBytes = 160

// cydClipBytes shortens s to at most maxBytes bytes at a character boundary,
// marking the cut with "…".
func cydClipBytes(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	cut := maxBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// cydLayoutSummary lists the screens of a layout that is too large to show.
func cydLayoutSummary(p cydcreator.Project) map[string]any {
	counts := map[string]int{}
	p.WalkWidgets(func(screen *cydcreator.Screen, _ *cydcreator.Widget, _ *cydcreator.Widget) bool {
		counts[screen.ID]++
		return true
	})
	screens := make([]map[string]any, 0, len(p.Screens))
	for _, s := range p.Screens {
		screens = append(screens, map[string]any{"id": s.ID, "name": s.Name, "title": s.Title, "widgets": counts[s.ID]})
	}
	return map[string]any{"name": p.Name, "board": p.Board, "orientation": p.Orientation, "theme": p.Theme,
		"assets": len(p.Assets), "screens": screens}
}

// cydReadProjectAnswer returns the working copy view; when it does not fit
// budget the layout is replaced by a summary (page cyd.project.json with
// cyd_creator_read_file), then the longest lists (including the plan's) are
// halved and finally the plan summary is shortened until it fits. A budget
// too small even for that gets CYD_FILE_LIMIT with the minimum.
func cydReadProjectAnswer(view cydcreator.JobProjectView, budget int) (map[string]any, error) {
	answer := map[string]any{"status": "ok", "project": view}
	if cydModelBytes(cydCreatorToolJSON(answer)) <= budget {
		return answer, nil
	}
	layoutBytes := 0
	if data, err := cydcreator.MarshalProject(view.Layout); err == nil {
		layoutBytes = len(data)
	} else if data, err := json.Marshal(view.Layout); err == nil {
		layoutBytes = len(data)
	}
	summary := cydLayoutSummary(view.Layout)
	screens := summary["screens"].([]map[string]any)
	issues, files := view.Issues, view.Files
	var changes []cydcreator.UserFileChange
	if view.UserChanges != nil {
		changes = view.UserChanges.Files
	}
	var plan cydcreator.JobPlan // clipped copy of view.Plan
	if view.Plan != nil {
		plan = *view.Plan
	}
	build := func() map[string]any {
		summary["screens"] = screens
		project := map[string]any{"phase": view.Phase, "layout": nil, "layout_summary": summary,
			"issues": issues, "issues_total": len(view.Issues), "files": files, "files_total": len(view.Files),
			"board": view.Board, "user_changes": view.UserChanges, "plan": view.Plan}
		if view.UserChanges != nil {
			uc := *view.UserChanges
			uc.Files = changes
			project["user_changes"] = uc
			project["user_changes_total"] = len(view.UserChanges.Files)
		}
		if len(screens) < len(view.Layout.Screens) {
			project["screens_total"] = len(view.Layout.Screens)
		}
		if view.Plan != nil {
			clipped := plan
			project["plan"] = &clipped
			for key, n := range map[string][2]int{"plan_screens_total": {len(plan.Screens), len(view.Plan.Screens)},
				"plan_widgets_total":      {len(plan.Widgets), len(view.Plan.Widgets)},
				"plan_data_sources_total": {len(plan.DataSources), len(view.Plan.DataSources)}} {
				if n[0] < n[1] {
					project[key] = n[1]
				}
			}
			if plan.Summary != view.Plan.Summary {
				project["plan_summary_clipped"] = true
			}
		}
		return map[string]any{"status": "ok", "project": project, "truncated": true, "total_bytes": layoutBytes,
			"next_action": fmt.Sprintf("The layout (%d bytes) does not fit one tool result and is left out (layout_summary instead); read cyd.project.json with cyd_creator_read_file, page by page with start_line/end_line. Lists ending early show their *_total.", layoutBytes)}
	}
	answer = build()
	for size := cydModelBytes(cydCreatorToolJSON(answer)); size > budget; size = cydModelBytes(cydCreatorToolJSON(answer)) {
		longest := max(len(issues), len(files), len(changes), len(screens), len(plan.Screens), len(plan.Widgets), len(plan.DataSources))
		switch {
		case longest == 0 && plan.Summary == "":
			return nil, cydOutputLimitError(size)
		case longest == 0:
			// The plan summary (up to 1000 characters) goes last.
			if runes := []rune(plan.Summary); len(runes) < 32 {
				plan.Summary = ""
			} else {
				plan.Summary = string(runes[:len(runes)/2]) + "…"
			}
		case longest == len(issues):
			issues = issues[:len(issues)/2]
		case longest == len(files):
			files = files[:len(files)/2]
		case longest == len(changes):
			changes = changes[:len(changes)/2]
		case longest == len(screens):
			screens = screens[:len(screens)/2]
		case longest == len(plan.Widgets):
			plan.Widgets = plan.Widgets[:len(plan.Widgets)/2]
		case longest == len(plan.DataSources):
			plan.DataSources = plan.DataSources[:len(plan.DataSources)/2]
		default:
			plan.Screens = plan.Screens[:len(plan.Screens)/2]
		}
		answer = build()
	}
	return answer, nil
}

func isCYDCreatorTool(name string) bool { return slices.Contains(cydcreator.ToolNames(), name) }

// dispatchCYDCreator executes the CYD Creator job tools. The job comes from
// the context bound by the server's runner; an explicit job_id argument must
// match it. The cyd_creator switch is re-checked on every call (live
// authorization intersects it), writes also need a writable desktop.
func dispatchCYDCreator(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	if !isCYDCreatorTool(tc.Action) {
		return "", false
	}
	jobID := cydcreator.JobIDFromContext(ctx)
	if jobID == "" {
		return cydCreatorPolicyDenied("CYD Creator tools only work inside CYD Creator agent jobs"), true
	}
	if raw, present := tc.Params["job_id"]; present && raw != nil {
		if arg, isString := raw.(string); !isString || (arg != "" && arg != jobID) {
			return cydCreatorToolJSON(map[string]any{"status": "error", "message": "job_id does not match this CYD Creator job"}), true
		}
	}
	if dc == nil || dc.Cfg == nil || !dc.Cfg.CYDCreator.Enabled {
		return cydCreatorPolicyDenied("CYD Creator is switched off"), true
	}
	if dc.Cfg.VirtualDesktop.ReadOnly && slices.Contains(cydCreatorWriteTools, tc.Action) {
		return cydCreatorPolicyDenied("the desktop is read-only"), true
	}
	service := cydcreator.DefaultService()
	if service == nil {
		return cydCreatorToolJSON(map[string]any{"status": "error", "message": "CYD Creator is not available"}), true
	}
	toolError := func(err error) string { return cydCreatorToolError(dc.Logger, err) }
	path := toolArgString(tc.Params, "path")
	switch tc.Action {
	case cydcreator.ToolReadProject:
		view, err := service.ReadProjectForJob(ctx, jobID)
		if err != nil {
			return toolError(err), true
		}
		answer, err := cydReadProjectAnswer(view, cydCreatorOutputBudget(dc))
		if err != nil {
			return toolError(err), true
		}
		return cydCreatorToolJSON(answer), true
	case cydcreator.ToolAPIReference:
		topic := toolArgString(tc.Params, "topic")
		if topic == "" {
			topic = "overview"
		}
		start, err := cydLineArg(tc.Params, "start_line")
		if err != nil {
			return toolError(err), true
		}
		end, err := cydLineArg(tc.Params, "end_line")
		if err != nil {
			return toolError(err), true
		}
		text, err := service.APIReference(ctx, jobID, topic)
		if err != nil {
			return toolError(err), true
		}
		answer, err := cydPagedAnswer(text, start, end, cydCreatorOutputBudget(dc), func(page cydFilePage) map[string]any {
			return cydReferenceAnswer(topic, page)
		})
		if err != nil {
			return toolError(err), true
		}
		return cydCreatorToolJSON(answer), true
	case cydcreator.ToolSubmitPlan:
		raw, ok := tc.Params["plan"]
		if !ok || raw == nil {
			return cydCreatorToolJSON(map[string]any{"status": "error", "message": "plan is required"}), true
		}
		// A string is passed as is and an object is encoded without HTML
		// escaping: < escapes would push a valid plan over its size cap.
		var data []byte
		if s, isString := raw.(string); isString {
			data = []byte(s)
		} else {
			var buf bytes.Buffer
			encoder := json.NewEncoder(&buf)
			encoder.SetEscapeHTML(false)
			if err := encoder.Encode(raw); err != nil {
				return toolError(err), true
			}
			data = bytes.TrimSpace(buf.Bytes())
		}
		plan, err := service.SubmitPlan(ctx, jobID, data)
		if err != nil {
			return toolError(err), true
		}
		// Counts only: echoing a maximal plan would exceed the output budget.
		return cydCreatorToolJSON(map[string]any{"status": "ok", "plan_accepted": map[string]int{"screens": len(plan.Screens),
			"widgets": len(plan.Widgets), "data_sources": len(plan.DataSources), "libraries": len(plan.Libraries)},
			"next_action": "Plan accepted. Stop now: the user confirms the plan before anything is built."}), true
	case cydcreator.ToolUpdateLayout:
		ops, err := decodeCYDLayoutOps(tc.Params["ops"])
		if err != nil {
			// Decoder errors can echo model-written field names: clip them.
			return cydCreatorToolJSON(map[string]any{"status": "error", "code": cydcreator.CodeBadRequest, "message": cydClipText(err.Error(), 300)}), true
		}
		update, err := service.UpdateLayoutForJob(ctx, jobID, ops)
		if err != nil {
			return toolError(err), true
		}
		return cydCreatorToolJSON(cydLayoutUpdateAnswer(update, cydCreatorOutputBudget(dc))), true
	case cydcreator.ToolReadFile:
		start, err := cydLineArg(tc.Params, "start_line")
		if err != nil {
			return toolError(err), true
		}
		end, err := cydLineArg(tc.Params, "end_line")
		if err != nil {
			return toolError(err), true
		}
		file, err := service.ReadJobFile(ctx, jobID, path)
		if err != nil {
			return toolError(err), true
		}
		answer, err := cydReadFilePage(file, start, end, cydCreatorOutputBudget(dc))
		if err != nil {
			return toolError(err), true
		}
		return cydCreatorToolJSON(answer), true
	case cydcreator.ToolWriteFile:
		content, ok := tc.Params["content"].(string)
		if !ok {
			// Never turn a missing content into an empty file; "" stays allowed.
			return toolError(cydCreatorBadRequest("content must be a string with the complete file", nil)), true
		}
		sha, err := service.WriteJobFile(ctx, jobID, path, content)
		if err != nil {
			return toolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "path": path, "sha256": sha,
			"next_action": "Run cyd_creator_build when your change is complete."}), true
	case cydcreator.ToolDeleteFile:
		if err := service.DeleteJobFile(ctx, jobID, path); err != nil {
			return toolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "path": path}), true
	case cydcreator.ToolBuild:
		res, err := service.BuildForJob(ctx, jobID)
		if err != nil {
			return toolError(err), true
		}
		return cydCreatorToolJSON(cydBuildAnswer(res, cydCreatorOutputBudget(dc))), true
	case cydcreator.ToolFinish:
		summary, _ := tc.Params["summary"].(string)
		if err := service.FinishJob(ctx, jobID, summary); err != nil {
			return toolError(err), true
		}
		return cydCreatorToolJSON(map[string]any{"status": "ok", "next_action": "Done. Do not call more tools."}), true
	}
	return "", false
}
