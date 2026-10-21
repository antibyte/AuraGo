package agent

import (
	"aurago/internal/tools"
	"context"
	"encoding/json"
	openai "github.com/sashabaranov/go-openai"
)

func ocrSchema() openai.Tool {
	return tool("ocr", "Local Indra OCR for PDFs and images. Extract starts a background job; use status and result with its job_id. No cloud upload.",
		schema(map[string]interface{}{
			"operation": operationProperty("OCR operation.", []string{"extract", "status", "result", "cancel"}),
			"file_path": prop("string", "extract: authorized workspace PDF or image path"),
			"job_id":    prop("string", "Job returned by extract; required for result/cancel"),
			"file_id":   prop("string", "result: file ID from job status"),
			"mode":      operationProperty("extract: auto (default) or force OCR.", []string{"auto", "ocr"}),
		}, "operation"))
}

func dispatchOCR(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	var args struct {
		Operation string `json:"operation"`
		FilePath  string `json:"file_path"`
		JobID     string `json:"job_id"`
		FileID    string `json:"file_id"`
		Mode      string `json:"mode"`
	}
	raw, _ := json.Marshal(tc)
	if tc.NativeArgsRaw != "" {
		raw = []byte(tc.NativeArgsRaw)
	} else if tc.RawJSON != "" {
		raw = []byte(tc.RawJSON)
	}
	_ = json.Unmarshal(raw, &args)
	return "Tool Output: " + tools.ExecuteOCR(ctx, dc.Cfg, dc.SessionID, args.Operation, args.FilePath, args.JobID, args.FileID, args.Mode)
}
