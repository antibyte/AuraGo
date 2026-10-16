package agent

import (
	"aurago/internal/codegraph"
	"context"
	"encoding/json"
	"fmt"
	openai "github.com/sashabaranov/go-openai"
	"sync"
)

type projectRunKey struct{}

var knowledgeRuntime struct {
	sync.RWMutex
	generation uint64
	configure  func(*RunConfig)
}

// RegisterKnowledgeRuntime binds the server-owned project resolver to channel
// loops without giving channel adapters access to host paths or graph ownership.
func RegisterKnowledgeRuntime(configure func(*RunConfig)) func() {
	knowledgeRuntime.Lock()
	knowledgeRuntime.generation++
	generation := knowledgeRuntime.generation
	knowledgeRuntime.configure = configure
	knowledgeRuntime.Unlock()
	return func() {
		knowledgeRuntime.Lock()
		defer knowledgeRuntime.Unlock()
		if knowledgeRuntime.generation == generation {
			knowledgeRuntime.configure = nil
		}
	}
}

func configureKnowledgeRuntime(run *RunConfig) {
	if run.Config == nil || (!run.Config.Learnings.Enabled && !run.Config.CodeGraph.Enabled) {
		return
	}
	knowledgeRuntime.RLock()
	configure := knowledgeRuntime.configure
	knowledgeRuntime.RUnlock()
	if configure != nil {
		configure(run)
	}
}

// WithCodeGraphProfile builds an independent immutable profile; existing profiles retain their revision.
func WithCodeGraphProfile(p *PreparedPromptProfile) (*PreparedPromptProfile, error) {
	if p == nil {
		return nil, fmt.Errorf("prepared profile unavailable")
	}
	return NewPreparedPromptProfile(p.name+".codegraph", p.system, append(p.Tools(), codeGraphToolSchema()))
}

func inheritProjectRun(ctx context.Context, run *RunConfig) {
	if dc, ok := ctx.Value(projectRunKey{}).(*DispatchContext); ok && dc != nil {
		run.CodeGraph, run.CodeProject, run.LearningScope = dc.CodeGraph, dc.CodeProject, dc.LearningScope
		run.WriteTargets, run.WorkChainID = dc.WriteTargets, dc.WorkChainID
		run.LearningTargetScope = dc.LearningTargetScope
	}
}

func codeGraphToolSchema() openai.Tool {
	return tool("code_graph", "Read the native Go/JS/JSX/TS/TSX/Python graph of the server-authorized project. First use indexes it. Every response reports freshness and partial coverage; unresolved/dynamic calls and missing edges never prove an edit safe. No arbitrary roots or mutations.", schema(map[string]interface{}{
		"operation": operationProperty("Graph operation", []string{"status", "search", "outline", "callers", "callees", "impact", "rescan"}),
		"query":     prop("string", "Symbol name fragment for search."), "file": prop("string", "Project-relative file for outline or disambiguation."), "symbol": prop("string", "Exact symbol ID returned by search or outline."), "limit": prop("integer", "Maximum results, 1 to 100 (default 30)."), "depth": prop("integer", "Impact depth, 1 to 4 (default 1)."),
	}, "operation"))
}

func dispatchCodeGraph(ctx context.Context, tc ToolCall, dc *DispatchContext) string {
	if dc.Cfg == nil || !dc.Cfg.CodeGraph.Enabled || dc.CodeGraph == nil || dc.CodeProject.ID == "" {
		return learningResponse(nil, fmt.Errorf("code graph is disabled or no authorized project is available"))
	}
	q := codegraph.Request{Operation: tc.Operation, Query: tc.Query, File: tc.FilePath, Symbol: tc.Name, Limit: tc.Limit}
	b, err := json.Marshal(tc.Params)
	if err == nil {
		err = json.Unmarshal(b, &q)
	}
	if err != nil {
		return learningResponse(nil, err)
	}
	result, err := dc.CodeGraph.Query(ctx, dc.CodeProject, q)
	return learningResponse(result, err)
}
