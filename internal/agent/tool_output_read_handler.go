package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"aurago/internal/memory"
)

type readToolOutputResponse struct {
	Status           string  `json:"status"`
	OutputRef        string  `json:"output_ref,omitempty"`
	ToolCallID       string  `json:"tool_call_id,omitempty"`
	ToolName         string  `json:"tool_name,omitempty"`
	View             string  `json:"view,omitempty"`
	Content          string  `json:"content,omitempty"`
	Truncated        bool    `json:"truncated,omitempty"`
	FilterUsed       string  `json:"filter_used,omitempty"`
	CompressionRatio float64 `json:"compression_ratio,omitempty"`
	Message          string  `json:"message,omitempty"`
}

const (
	defaultReadToolOutputMaxChars = 6000
	maxReadToolOutputChars        = 32000
)

func handleReadToolOutput(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	ref := firstNonEmptyToolString(
		stringValueFromMap(tc.Params, "ref"),
		stringValueFromMap(tc.Params, "output_ref"),
	)
	if ref == "" {
		return readToolOutputError("ref is required")
	}
	if dc == nil || dc.ShortTermMem == nil {
		return readToolOutputError("Short-term memory is not available")
	}
	out, err := retrieveToolOutputByRefOrLegacyID(ctx, dc.ShortTermMem, dc.SessionID, ref)
	if err != nil {
		return readToolOutputError(err.Error())
	}
	_ = dc.ShortTermMem.MarkCompressedOutputAccessed(ctx, out.ID)
	req := toolOutputViewRequest{
		View:      stringValueFromMap(tc.Params, "view"),
		Query:     stringValueFromMap(tc.Params, "query"),
		StartLine: toolArgInt(tc.Params, 0, "start_line"),
		EndLine:   toolArgInt(tc.Params, 0, "end_line"),
		MaxLines:  toolArgInt(tc.Params, 0, "max_lines"),
		MaxChars:  toolArgInt(tc.Params, 0, "max_chars"),
		Reason:    stringValueFromMap(tc.Params, "reason"),
	}
	req = normalizeReadToolOutputViewRequest(req)
	content, truncated, viewErr := renderToolOutputView(out, req)
	if viewErr != nil {
		return readToolOutputError(viewErr.Error())
	}
	view := req.View
	if view == "" {
		view = "summary"
	}
	resp := readToolOutputResponse{
		Status:           "success",
		OutputRef:        out.OutputRef,
		ToolCallID:       out.ToolCallID,
		ToolName:         out.ToolName,
		View:             view,
		Content:          content,
		Truncated:        truncated,
		FilterUsed:       out.FilterUsed,
		CompressionRatio: out.CompressionRatio,
	}
	b, err := json.Marshal(resp)
	if err != nil {
		return readToolOutputError(fmt.Sprintf("failed to encode response: %s", err))
	}
	return "Tool Output: " + string(b)
}

func normalizeReadToolOutputViewRequest(req toolOutputViewRequest) toolOutputViewRequest {
	if req.MaxChars <= 0 {
		req.MaxChars = defaultReadToolOutputMaxChars
	}
	if req.MaxChars > maxReadToolOutputChars {
		req.MaxChars = maxReadToolOutputChars
	}
	return req
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
	return header + content
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
