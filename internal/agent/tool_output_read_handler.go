package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"aurago/internal/memory"
	"aurago/internal/security"
)

type readToolOutputResponse struct {
	Status           string  `json:"status"`
	OutputRef        string  `json:"output_ref,omitempty"`
	ToolCallID       string  `json:"tool_call_id,omitempty"`
	ToolName         string  `json:"tool_name,omitempty"`
	View             string  `json:"view,omitempty"`
	Content          string  `json:"content,omitempty"`
	Offset           int     `json:"offset,omitempty"`
	NextOffset       int     `json:"next_offset,omitempty"`
	TotalChars       int     `json:"total_chars,omitempty"`
	Truncated        bool    `json:"truncated,omitempty"`
	FilterUsed       string  `json:"filter_used,omitempty"`
	CompressionRatio float64 `json:"compression_ratio,omitempty"`
	Message          string  `json:"message,omitempty"`
}

const (
	// maxReadToolOutputChars caps an explicit max_chars; the inline budget of
	// the dispatch caps every reply as the model receives it.
	maxReadToolOutputChars = 32000
	// readToolOutputReserve stays free for what the dispatch may add after
	// the handler: a Guardian warning, the text-mode "[Tool Output]" prefix.
	readToolOutputReserve = 400
	// readToolOutputMinBudget keeps a page useful under a tiny inline limit.
	readToolOutputMinBudget = 1000
)

func handleReadToolOutput(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	ref := firstNonEmptyToolString(
		stringValueFromMap(tc.Params, "ref"),
		stringValueFromMap(tc.Params, "output_ref"),
	)
	if ref == "" {
		return readToolOutputError(`ref is required: pass the output_ref value of an archived tool result, for example ref="toolout_..."`)
	}
	if dc == nil || dc.ShortTermMem == nil {
		return readToolOutputError("Short-term memory is not available")
	}
	out, err := retrieveToolOutputByRefOrLegacyID(ctx, dc.ShortTermMem, dc.SessionID, ref)
	if err != nil {
		return readToolOutputError(err.Error())
	}
	_ = dc.ShortTermMem.MarkCompressedOutputAccessed(ctx, out.ID)
	req := normalizeReadToolOutputViewRequest(toolOutputViewRequest{
		View:      stringValueFromMap(tc.Params, "view"),
		Query:     stringValueFromMap(tc.Params, "query"),
		StartLine: toolArgInt(tc.Params, 0, "start_line"),
		EndLine:   toolArgInt(tc.Params, 0, "end_line"),
		MaxLines:  toolArgInt(tc.Params, 0, "max_lines"),
		MaxChars:  toolArgInt(tc.Params, 0, "max_chars", "limit"),
		Offset:    toolArgInt(tc.Params, 0, "offset"),
		Reason:    stringValueFromMap(tc.Params, "reason"),
	})
	payload, isolated := vaultReadablePayload(out.OriginalContent)
	content, viewErr := renderToolOutputView(out, payload, req)
	if viewErr != nil {
		return readToolOutputError(viewErr.Error())
	}
	resp := readToolOutputResponse{
		Status:           "success",
		OutputRef:        out.OutputRef,
		ToolCallID:       out.ToolCallID,
		ToolName:         out.ToolName,
		View:             req.View,
		FilterUsed:       out.FilterUsed,
		CompressionRatio: out.CompressionRatio,
	}
	// The Guardian isolates every read_tool_output reply (external output).
	// Without one the reply keeps the original's isolation by itself.
	isolate := isolated && (dc.Guardian == nil)
	reply := fitReadToolOutputReply(resp, content, req, false, readToolOutputBudget(dc))
	source := security.ExternalToolSource(out.ToolName, out.ToolOperation)
	checked := dc.Guardian.ProtectExternal(ctx, source, strings.TrimPrefix(reply, "Tool Output: "))
	if !checked.Quarantined && security.UntrustedContentTool(out.ToolName, out.ToolOperation) {
		modelIsolatedBodies.noteExternalContact(dc.SessionID)
	}
	content = checked.Text
	if isolate {
		content = isolateToolPayload(content, false)
	}
	if checked.Datamarked {
		content += "\n" + security.DatamarkNotice
	}
	return "Tool Output: " + content
}

// normalizeReadToolOutputViewRequest defaults the view to full (paged from
// offset) and caps an explicit max_chars.
func normalizeReadToolOutputViewRequest(req toolOutputViewRequest) toolOutputViewRequest {
	if req.View == "" {
		req.View = "full"
	}
	if req.MaxChars > maxReadToolOutputChars {
		req.MaxChars = maxReadToolOutputChars
	}
	if req.Offset < 0 {
		req.Offset = 0
	}
	return req
}

// readToolOutputBudget is the largest reply in bytes as the model receives
// it: one inline result of the current request (the vault threshold, at most
// the tool output limit), even while the primary vault is off.
func readToolOutputBudget(dc *DispatchContext) int {
	budget := toolResultInlineFloor
	if dc != nil {
		budget = min(effectiveToolOutputLimit(dc.Cfg), primaryOutputVaultMaxInline(dc.Cfg, dc.inlineBasis))
	}
	return max(budget-readToolOutputReserve, readToolOutputMinBudget)
}

// readToolOutputModelBytes is the size of a reply after the Guardian isolates
// it once more (escaping quotes and angle brackets).
func readToolOutputModelBytes(reply string) int {
	return len(security.IsolateExternalData(reply))
}

// fitReadToolOutputReply pages content from req.Offset into the largest reply
// within budget. A tail view without an offset keeps the end of the content.
// next_offset continues a page that stopped before the end.
func fitReadToolOutputReply(resp readToolOutputResponse, content string, req toolOutputViewRequest, isolate bool, budget int) string {
	total := len(content)
	offset := min(req.Offset, total)
	for offset < total && !utf8.RuneStart(content[offset]) {
		offset++
	}
	rest := content[offset:]
	limit := len(rest)
	if req.MaxChars > 0 {
		limit = min(limit, req.MaxChars)
	}
	fromEnd := req.View == "tail" && offset == 0
	render := func(n int) string {
		page := resp
		page.TotalChars = total
		if fromEnd {
			page.Content = utf8Suffix(rest, n)
			if len(page.Content) < total {
				page.Offset = total - len(page.Content)
				page.Truncated = true
			}
		} else {
			page.Content = truncateUTF8Prefix(rest, n)
			page.Offset = offset
			if end := offset + len(page.Content); end < total {
				page.NextOffset = end
				page.Truncated = true
			}
		}
		if total > 0 && offset >= total {
			page.Message = fmt.Sprintf("offset is past the end of this view (total_chars=%d)", total)
		}
		return encodeReadToolOutputReply(page, isolate)
	}
	best := render(0)
	lo, hi := 1, limit
	for lo <= hi {
		mid := lo + (hi-lo)/2
		if reply := render(mid); readToolOutputModelBytes(reply) <= budget {
			best = reply
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	return best
}

func encodeReadToolOutputReply(resp readToolOutputResponse, isolate bool) string {
	b, err := json.Marshal(resp)
	if err != nil {
		return readToolOutputError(fmt.Sprintf("failed to encode response: %s", err))
	}
	if isolate {
		return "Tool Output: " + isolateToolPayload(string(b), false)
	}
	return "Tool Output: " + string(b)
}

func handleRetrieveOriginalOutput(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	toolCallID := stringValueFromMap(tc.Params, "tool_call_id")
	reason := stringValueFromMap(tc.Params, "reason")
	if toolCallID == "" {
		return readToolOutputError("tool_call_id is required")
	}
	if dc == nil || dc.ShortTermMem == nil {
		return readToolOutputError("Short-term memory is not available")
	}
	out, err := dc.ShortTermMem.RetrieveCompressedOutput(ctx, dc.SessionID, toolCallID)
	if err != nil {
		return readToolOutputError(err.Error())
	}
	_ = dc.ShortTermMem.MarkCompressedOutputAccessed(ctx, out.ID)
	content := out.OriginalContent
	const maxRetrievableOriginalChars = 32000
	truncated := false
	if len(content) > maxRetrievableOriginalChars {
		content = content[:maxRetrievableOriginalChars] +
			fmt.Sprintf("\n[TRUNCATED: original was %d chars, retrieved first %d]",
				len(out.OriginalContent), maxRetrievableOriginalChars)
		truncated = true
	}
	header := fmt.Sprintf("[ORIGINAL OUTPUT for %s — ref=%s filter=%s ratio=%.2f%s]\n",
		out.ToolName, out.OutputRef, out.FilterUsed, out.CompressionRatio,
		map[bool]string{true: " retrieved_partially", false: ""}[truncated])
	if reason != "" && dc.Logger != nil {
		dc.Logger.Debug("Output vault retrieval", "tool_call_id", toolCallID, "output_ref", out.OutputRef, "reason", reason, "filter", out.FilterUsed)
	}
	source := security.ExternalToolSource(out.ToolName, out.ToolOperation)
	checked := dc.Guardian.ProtectExternal(ctx, source, content)
	if !checked.Quarantined && security.UntrustedContentTool(out.ToolName, out.ToolOperation) {
		modelIsolatedBodies.noteExternalContact(dc.SessionID)
	}
	if checked.Datamarked {
		checked.Text += "\n" + security.DatamarkNotice
	}
	return header + checked.Text
}

func readToolOutputError(message string) string {
	resp := readToolOutputResponse{Status: "error", Message: message}
	b, err := json.Marshal(resp)
	if err != nil {
		return `Tool Output: {"status":"error","message":"failed to encode error"}`
	}
	return "Tool Output: " + string(b)
}

func retrieveToolOutputByRefOrLegacyID(ctx context.Context, stm *memory.SQLiteMemory, sessionID, ref string) (*memory.CompressedToolOutput, error) {
	if stm == nil {
		return nil, fmt.Errorf("short-term memory is not available")
	}
	if out, err := stm.RetrieveCompressedOutputByRef(ctx, sessionID, ref); err == nil {
		return out, nil
	}
	return stm.RetrieveCompressedOutput(ctx, sessionID, ref)
}
