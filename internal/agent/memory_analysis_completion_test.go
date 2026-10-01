package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/config"
	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

const validRealtimeExtraction = `{"facts":[{"content":"The NAS uses XFS.","category":"infrastructure","confidence":0.99}],"preferences":[],"corrections":[],"pending_actions":[]}`
const emptyRealtimeExtraction = `{"facts":[],"preferences":[],"corrections":[],"pending_actions":[]}`

type realtimeExtractionCase struct {
	name         string
	content      string
	finish       openai.FinishReason
	recover      bool
	model        string
	contextLimit int
	outputLimit  int
	wantCalls    int
	wantWrites   int
	cancelBefore bool
	cancelDuring bool
}

func TestAuditMemoryAnalysisRejectsTruncatedCompletion(t *testing.T) {
	checkRealtimeExtraction(t, realtimeExtractionCase{content: validRealtimeExtraction, finish: openai.FinishReasonLength, wantCalls: 2})
}

func TestRealtimeMemoryAnalysisCompletionBoundaries(t *testing.T) {
	for _, test := range []realtimeExtractionCase{
		{name: "valid", content: validRealtimeExtraction, wantCalls: 1, wantWrites: 1},
		{name: "empty arrays", content: emptyRealtimeExtraction, wantCalls: 1},
		{name: "empty content", content: "", wantCalls: 2},
		{name: "malformed", content: "{broken", wantCalls: 2},
		{name: "null arrays", content: `{"facts":null,"preferences":[],"corrections":[],"pending_actions":[]}`, wantCalls: 2},
		{name: "missing arrays", content: `{}`, wantCalls: 2},
		{name: "invalid fact", content: `{"facts":[{"content":"","category":"fact","confidence":2}],"preferences":[],"corrections":[],"pending_actions":[]}`, wantCalls: 2},
		{name: "normalized", content: "<think>private reasoning</think>\n```json\n" + validRealtimeExtraction + "\n```", wantCalls: 1, wantWrites: 1},
		{name: "bounded recovery", content: validRealtimeExtraction, finish: openai.FinishReasonLength, recover: true, wantCalls: 2, wantWrites: 1},
		{name: "reasoning budget", content: validRealtimeExtraction, model: "o3-mini", outputLimit: 4096, wantCalls: 1, wantWrites: 1},
		{name: "no JSON support", content: emptyRealtimeExtraction, model: "synthetic-unsupported-model", outputLimit: 128, wantCalls: 1},
		{name: "context exhausted", content: validRealtimeExtraction, contextLimit: 512, wantCalls: 0},
		{name: "cancel before", content: validRealtimeExtraction, cancelBefore: true, wantCalls: 0},
		{name: "cancel during", content: validRealtimeExtraction, cancelDuring: true, wantCalls: 1},
	} {
		t.Run(test.name, func(t *testing.T) { checkRealtimeExtraction(t, test) })
	}
}

func checkRealtimeExtraction(t *testing.T, test realtimeExtractionCase) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var extractionLog strings.Builder
	logger := slog.New(slog.NewTextHandler(&extractionLog, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	var calls atomic.Int32
	requests := make(chan openai.ChatCompletionRequest, 10)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openai.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Error(err)
		}
		requests <- req
		call := calls.Add(1)
		if test.cancelDuring {
			cancel()
		}
		finish := test.finish
		if finish == "" || test.recover && call > 1 {
			finish = openai.FinishReasonStop
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(openai.ChatCompletionResponse{Choices: []openai.ChatCompletionChoice{{FinishReason: finish, Message: openai.ChatCompletionMessage{Role: "assistant", Content: test.content}}}})
	}))
	defer server.Close()
	model := test.model
	if model == "" {
		model = "gpt-4o-mini"
	}
	contextLimit := test.contextLimit
	if contextLimit == 0 {
		contextLimit = 32768
	}
	outputLimit := test.outputLimit
	if outputLimit == 0 {
		outputLimit = 4096
	}
	cfg := &config.Config{}
	cfg.MemoryAnalysis.Enabled, cfg.MemoryAnalysis.RealTime = true, true
	cfg.MemoryAnalysis.Provider, cfg.MemoryAnalysis.ProviderType = "analysis", "openai"
	cfg.MemoryAnalysis.BaseURL, cfg.MemoryAnalysis.ResolvedModel = server.URL+"/v1", model
	cfg.Providers = []config.ProviderEntry{{ID: "analysis", Type: "openai", Model: model, ContextWindow: contextLimit, MaxOutputTokens: outputLimit}, {ID: "main", MaxOutputTokens: 1}}
	cfg.LLM.Provider = "main"
	vdb := &fakeVectorDB{}
	if test.cancelBefore {
		cancel()
	}
	runMemoryAnalysis(ctx, cfg, logger, stm, nil, vdb, "The NAS uses XFS for the daily backup volume.", "Assistant source marker", "synthetic-extraction")
	if int(calls.Load()) != test.wantCalls || len(vdb.storedConcepts) != test.wantWrites {
		t.Fatalf("calls=%d writes=%d want=%d/%d logs=%s", calls.Load(), len(vdb.storedConcepts), test.wantCalls, test.wantWrites, extractionLog.String())
	}
	if count, err := stm.CountPendingMemoryWrites(); err != nil || count != 0 {
		t.Fatalf("unusable extraction queued work: count=%d err=%v", count, err)
	}
	for i := 0; i < test.wantCalls; i++ {
		req := <-requests
		if i > 0 && (strings.Contains(req.Messages[0].Content, "Assistant source marker") || !strings.Contains(req.Messages[0].Content, "at most one item")) {
			t.Fatal("retry did not reduce the extraction unit")
		}
		if test.model == "o3-mini" && max(req.MaxTokens, req.MaxCompletionTokens) != 4096 {
			t.Fatalf("reasoning budget=%d/%d", req.MaxTokens, req.MaxCompletionTokens)
		}
		if test.model == "synthetic-unsupported-model" && (req.ResponseFormat != nil || max(req.MaxTokens, req.MaxCompletionTokens) != 128) {
			t.Fatalf("unsupported route requested JSON mode or exceeded cap: %+v", req)
		}
		if test.model == "" && req.ResponseFormat == nil {
			t.Fatal("confirmed JSON route did not request JSON mode")
		}
	}
}
