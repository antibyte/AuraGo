package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/localwiki"
	"aurago/internal/memory"
	"aurago/internal/security"
	"aurago/internal/tools"
)

// hindiWikiLibrary is a Hindi edition with long leads and a long article; it
// pages like the library (PageRunes runes, rune offsets).
type hindiWikiLibrary struct{ text string }

const agentHindiSentence = `भारत (आधिकारिक नाम: "भारत गणराज्य") दक्षिण एशिया में स्थित भारतीय उपमहाद्वीप का सबसे बड़ा देश है & इसकी राजधानी नई दिल्ली है। `

func hindiRunes(n int) string {
	return string([]rune(strings.Repeat(agentHindiSentence, n/40+1))[:n])
}

func (l *hindiWikiLibrary) Edition() localwiki.Edition {
	return localwiki.Edition{Language: "hi", Variant: localwiki.VariantNoPic, Date: "2026-10"}
}

func (l *hindiWikiLibrary) Fulltext() bool { return true }

func (l *hindiWikiLibrary) Search(_ context.Context, _ string, limit, leads int) (localwiki.SearchResult, error) {
	result := localwiki.SearchResult{Edition: l.Edition(), Fulltext: true}
	for i := range limit {
		hit := localwiki.SearchHit{Ref: localwiki.Ref{Title: fmt.Sprintf("भारत %d", i), Path: fmt.Sprintf("भारत_%d", i)}, Snippet: hindiRunes(200)}
		if i < leads {
			hit.Lead = hindiRunes(2000)
		}
		result.Results = append(result.Results, hit)
	}
	return result, nil
}

func (l *hindiWikiLibrary) Read(_ context.Context, req localwiki.ReadRequest) (localwiki.Article, error) {
	size := req.PageRunes
	if size <= 0 || size > 8000 {
		size = 8000
	}
	runes := []rune(l.text)
	end := min(req.Offset+size, len(runes))
	var next *int
	if end < len(runes) {
		next = &end
	}
	sections := make([]localwiki.Section, 40)
	for i := range sections {
		sections[i] = localwiki.Section{Index: i, Heading: fmt.Sprintf("इतिहास %d", i), Level: 2, Chars: 900}
	}
	return localwiki.Article{Ref: localwiki.Ref{Title: "भारत", Path: "भारत"}, Sections: sections, Content: string(runes[req.Offset:end]), NextOffset: next}, nil
}

type hindiWikiSource struct{ lib *hindiWikiLibrary }

func (s hindiWikiSource) AcquireLibrary() (tools.LocalWikipediaLibrary, func(), bool) {
	return s.lib, func() {}, true
}

func outputVaultTestConfig(maxInline int) *config.Config {
	cfg := localWikipediaTestConfig()
	cfg.Agent.ToolOutputLimit = 50000
	cfg.Agent.OutputCompression.Reversible.Enabled = true
	cfg.Agent.OutputCompression.Reversible.PrimaryOutputVault = true
	cfg.Agent.OutputCompression.Reversible.MaxInlineChars = maxInline
	return cfg
}

// The live acceptance found a Hindi search (11,905 bytes) and read page
// (23,039 bytes) archived by the output vault, so the model saw only their
// start. On the real dispatch path (Guardian isolation included) both now stay
// below the inline limit and reach the model whole.
func TestLocalWikipediaAnswersStayInline(t *testing.T) {
	useAgentWikiSource(t, hindiWikiSource{&hindiWikiLibrary{text: hindiRunes(30_000)}})
	stm, err := memory.NewSQLiteMemory(":memory:", testLogger)
	if err != nil {
		t.Fatal(err)
	}
	defer stm.Close()
	for _, maxInline := range []int{6000, 12000} {
		cfg := outputVaultTestConfig(maxInline)
		dc := &DispatchContext{Cfg: cfg, Logger: testLogger, Guardian: security.NewGuardian(nil), SessionID: t.Name()}
		for i, params := range []map[string]interface{}{
			{"operation": "search", "query": "भारत"},
			{"operation": "search", "query": "भारत", "limit": float64(10)},
			{"operation": "read", "path": "भारत"},
			{"operation": "read", "path": "भारत", "offset": float64(12345)},
		} {
			tc := ToolCall{Action: "local_wikipedia", NativeCallID: fmt.Sprintf("call_%d_%d", maxInline, i), Params: params}
			dispatched := DispatchToolCallResult(context.Background(), &tc, dc, "भारत")
			if dispatched.Status != ToolResultSuccess || !strings.HasPrefix(dispatched.Output, "<external_data>\n") {
				t.Fatalf("%v: %+v", params, dispatched)
			}
			t.Logf("%v at %d: %d bytes", params, maxInline, len(dispatched.Output))
			if len(dispatched.Output) > maxInline {
				t.Fatalf("%v at %d: dispatch output is %d bytes", params, maxInline, len(dispatched.Output))
			}
			result := finalizeToolExecution(context.Background(), tc, dispatched.Output, false, cfg, stm, t.Name(), nil, nil, testLogger, AgentTelemetryScope{}, "", 0, RunConfig{})
			if result.OutputRef != "" || strings.Contains(result.Content, "Full output archived") {
				t.Fatalf("%v at %d: archived (%d bytes)", params, maxInline, len(dispatched.Output))
			}
			if params["operation"] == "read" && !strings.Contains(result.Content, "next_offset&#34;:") {
				t.Fatalf("%v: page lost next_offset: %.300s", params, result.Content)
			}
			if params["operation"] == "search" && params["limit"] == nil && !strings.Contains(result.Content, "lead&#34;:") {
				t.Fatalf("%v: no lead left: %.300s", params, result.Content)
			}
		}
	}
}

func TestToolResultInlineBudget(t *testing.T) {
	if got := toolResultInlineBudget(nil); got != 6000 {
		t.Fatalf("nil config budget = %d", got)
	}
	cfg := outputVaultTestConfig(0)
	if got := toolResultInlineBudget(cfg); got != 6000 {
		t.Fatalf("unset max_inline_chars budget = %d", got)
	}
	cfg.Agent.OutputCompression.Reversible.MaxInlineChars = 9000
	if got := toolResultInlineBudget(cfg); got != 9000 {
		t.Fatalf("max_inline_chars budget = %d", got)
	}
	cfg.Agent.ToolOutputLimit = 4000
	if got := toolResultInlineBudget(cfg); got != 4000 {
		t.Fatalf("tool_output_limit below the vault threshold: budget = %d", got)
	}
	cfg.Agent.ToolOutputLimit = 0
	cfg.Agent.OutputCompression.Reversible.PrimaryOutputVault = false
	if got := toolResultInlineBudget(cfg); got != 50000 {
		t.Fatalf("vault off: budget = %d, want the tool output limit", got)
	}
}

// The dispatch passes the budget on to the tool.
func TestDispatchLocalWikipediaPassesTheInlineBudget(t *testing.T) {
	lib := &agentWikiLibrary{}
	useAgentWikiSource(t, &agentWikiSource{lib: lib, open: true})
	cfg := outputVaultTestConfig(3000)
	_, _ = dispatchPlatform(context.Background(), ToolCall{Action: "local_wikipedia", Params: map[string]interface{}{"operation": "read", "path": "Berlin"}}, &DispatchContext{Cfg: cfg, Logger: testLogger})
	if lib.read.PageRunes == 0 || lib.read.PageRunes > 3000 {
		t.Fatalf("page size %d does not follow a 3,000-byte inline limit", lib.read.PageRunes)
	}
}
