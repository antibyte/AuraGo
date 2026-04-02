package agent

import (
	"context"
	"strings"

	"aurago/internal/config"
	"aurago/internal/tools"
)

const maxPendingSummaryBatchItems = 3

type pendingSummaryBatchCandidate struct {
	BatchKey        string
	ToolCall        ToolCall
	EffectiveAction string
	SearchQuery     string
	SourceName      string
	LLMCfg          tools.SummaryLLMConfig
}

func maybeBuildPendingSummaryBatch(ctx context.Context, pending []ToolCall, dc *DispatchContext, helperManager *helperLLMManager, lastUserMsg string) map[string]string {
	if helperManager == nil || dc == nil || dc.Cfg == nil || len(pending) < 2 {
		return nil
	}

	candidates := collectNativePendingSummaryBatchCandidates(dc.Cfg, pending)
	if len(candidates) < 2 {
		return nil
	}

	rawResults := make(map[string]string, len(candidates))
	batchInputs := make([]helperContentSummaryBatchInput, 0, len(candidates))
	batchable := make(map[string]pendingSummaryBatchCandidate, len(candidates))

	for _, candidate := range candidates {
		rawResult := dispatchRawSummaryBatchCandidate(ctx, candidate, dc, lastUserMsg)
		rawResults[candidate.BatchKey] = rawResult

		content, err := tools.ExtractSummarySourceContent(rawResult)
		if err != nil || strings.TrimSpace(content) == "" {
			continue
		}

		batchInputs = append(batchInputs, helperContentSummaryBatchInput{
			BatchID:     candidate.BatchKey,
			SourceName:  candidate.SourceName,
			SearchQuery: candidate.SearchQuery,
			Content:     content,
		})
		batchable[candidate.BatchKey] = candidate
	}

	summaryOutputs := make(map[string]string, len(candidates))
	if len(batchInputs) >= 2 {
		if batchResult, err := helperManager.SummarizeContentBatches(ctx, batchInputs); err == nil {
			for _, item := range batchResult.Summaries {
				if item.BatchID == "" || strings.TrimSpace(item.Summary) == "" {
					continue
				}
				summaryOutputs[item.BatchID] = tools.EncodeSummaryContent(item.Summary)
			}
		} else if dc.Logger != nil {
			helperManager.ObserveFallback("content_summaries", err.Error())
			dc.Logger.Warn("[HelperLLM] Native pending content summary batch failed, falling back", "error", err)
		}
	}

	for _, candidate := range candidates {
		if _, ok := summaryOutputs[candidate.BatchKey]; ok {
			continue
		}

		rawResult := rawResults[candidate.BatchKey]
		fallback, err := tools.SummariseContent(ctx, candidate.LLMCfg, dc.Logger, rawResult, candidate.SearchQuery, candidate.SourceName)
		if err != nil {
			summaryOutputs[candidate.BatchKey] = rawResult
			if dc.Logger != nil {
				dc.Logger.Warn("summary fallback failed, returning raw content", "action", candidate.EffectiveAction, "error", err)
			}
			continue
		}
		summaryOutputs[candidate.BatchKey] = fallback
	}

	if len(summaryOutputs) == 0 {
		return nil
	}
	return summaryOutputs
}

func collectNativePendingSummaryBatchCandidates(cfg *config.Config, pending []ToolCall) []pendingSummaryBatchCandidate {
	candidates := make([]pendingSummaryBatchCandidate, 0, minInt(len(pending), maxPendingSummaryBatchItems))
	for _, tc := range pending {
		candidate, ok := buildPendingSummaryBatchCandidate(cfg, tc)
		if !ok {
			break
		}
		candidates = append(candidates, candidate)
		if len(candidates) >= maxPendingSummaryBatchItems {
			break
		}
	}
	return candidates
}

func buildPendingSummaryBatchCandidate(cfg *config.Config, tc ToolCall) (pendingSummaryBatchCandidate, bool) {
	action := effectiveSummaryBatchAction(tc)
	if action == "" {
		return pendingSummaryBatchCandidate{}, false
	}

	candidate := pendingSummaryBatchCandidate{
		BatchKey:        pendingSummaryBatchKey(tc),
		ToolCall:        tc,
		EffectiveAction: action,
	}
	if candidate.BatchKey == "" {
		return pendingSummaryBatchCandidate{}, false
	}

	switch action {
	case "web_scraper":
		if !cfg.Tools.WebScraper.SummaryMode {
			return pendingSummaryBatchCandidate{}, false
		}
		candidate.SearchQuery = stringParam(tc, "search_query")
		if candidate.SearchQuery == "" {
			candidate.SearchQuery = "general summary of the page content"
		}
		candidate.SourceName = "web page"
		candidate.LLMCfg = tools.ResolveSummaryLLMConfig(cfg, tools.SummaryLLMConfig{
			APIKey:  cfg.Tools.WebScraper.SummaryAPIKey,
			BaseURL: cfg.Tools.WebScraper.SummaryBaseURL,
			Model:   cfg.Tools.WebScraper.SummaryModel,
		})
	case "wikipedia_search":
		if !cfg.Tools.Wikipedia.SummaryMode {
			return pendingSummaryBatchCandidate{}, false
		}
		queryStr := firstNonEmptySummaryBatch(tc.Query, stringParam(tc, "query"))
		candidate.SearchQuery = stringParam(tc, "search_query")
		if candidate.SearchQuery == "" {
			candidate.SearchQuery = "summarise the key facts about: " + queryStr
		}
		candidate.SourceName = "Wikipedia article"
		candidate.LLMCfg = tools.ResolveSummaryLLMConfig(cfg, tools.SummaryLLMConfig{
			APIKey:  cfg.Tools.Wikipedia.SummaryAPIKey,
			BaseURL: cfg.Tools.Wikipedia.SummaryBaseURL,
			Model:   cfg.Tools.Wikipedia.SummaryModel,
		})
	case "ddg_search":
		if !cfg.Tools.DDGSearch.SummaryMode {
			return pendingSummaryBatchCandidate{}, false
		}
		queryStr := firstNonEmptySummaryBatch(tc.Query, stringParam(tc, "query"))
		candidate.SearchQuery = stringParam(tc, "search_query")
		if candidate.SearchQuery == "" {
			candidate.SearchQuery = "synthesise the most relevant findings for: " + queryStr
		}
		candidate.SourceName = "search results"
		candidate.LLMCfg = tools.ResolveSummaryLLMConfig(cfg, tools.SummaryLLMConfig{
			APIKey:  cfg.Tools.DDGSearch.SummaryAPIKey,
			BaseURL: cfg.Tools.DDGSearch.SummaryBaseURL,
			Model:   cfg.Tools.DDGSearch.SummaryModel,
		})
	case "pdf_extractor":
		if !cfg.Tools.PDFExtractor.SummaryMode {
			return pendingSummaryBatchCandidate{}, false
		}
		candidate.SearchQuery = stringParam(tc, "search_query")
		if candidate.SearchQuery == "" {
			candidate.SearchQuery = "summarise the key content of this document"
		}
		candidate.SourceName = "PDF document"
		candidate.LLMCfg = tools.ResolveSummaryLLMConfig(cfg, tools.SummaryLLMConfig{
			APIKey:  cfg.Tools.PDFExtractor.SummaryAPIKey,
			BaseURL: cfg.Tools.PDFExtractor.SummaryBaseURL,
			Model:   cfg.Tools.PDFExtractor.SummaryModel,
		})
	default:
		return pendingSummaryBatchCandidate{}, false
	}

	return candidate, true
}

func dispatchRawSummaryBatchCandidate(ctx context.Context, candidate pendingSummaryBatchCandidate, dc *DispatchContext, lastUserMsg string) string {
	rawCfg := cloneConfigWithSummaryModeDisabled(dc.Cfg, candidate.EffectiveAction)
	rawDC := *dc
	rawDC.Cfg = rawCfg
	return DispatchToolCall(ctx, candidate.ToolCall, &rawDC, lastUserMsg)
}

func cloneConfigWithSummaryModeDisabled(cfg *config.Config, action string) *config.Config {
	if cfg == nil {
		return nil
	}
	cloned := *cfg
	switch action {
	case "web_scraper":
		cloned.Tools.WebScraper.SummaryMode = false
	case "wikipedia_search":
		cloned.Tools.Wikipedia.SummaryMode = false
	case "ddg_search":
		cloned.Tools.DDGSearch.SummaryMode = false
	case "pdf_extractor":
		cloned.Tools.PDFExtractor.SummaryMode = false
	}
	return &cloned
}

func effectiveSummaryBatchAction(tc ToolCall) string {
	action := strings.TrimSpace(tc.Action)
	switch action {
	case "web_scraper", "wikipedia_search", "ddg_search", "pdf_extractor":
		return action
	case "execute_skill":
		skill := strings.TrimSpace(tc.Skill)
		if skill == "" {
			skill = strings.TrimSpace(tc.Name)
		}
		switch skill {
		case "web_scraper", "wikipedia_search", "ddg_search", "pdf_extractor":
			return skill
		}
	}
	return ""
}

func pendingSummaryBatchKey(tc ToolCall) string {
	if strings.TrimSpace(tc.NativeCallID) != "" {
		return strings.TrimSpace(tc.NativeCallID)
	}
	if strings.TrimSpace(tc.RawJSON) != "" {
		return strings.TrimSpace(tc.RawJSON)
	}
	var fallback strings.Builder
	fallback.WriteString(strings.TrimSpace(tc.Action))
	fallback.WriteString("|")
	fallback.WriteString(strings.TrimSpace(tc.Skill))
	fallback.WriteString("|")
	fallback.WriteString(strings.TrimSpace(tc.Query))
	fallback.WriteString("|")
	fallback.WriteString(strings.TrimSpace(tc.URL))
	fallback.WriteString("|")
	fallback.WriteString(strings.TrimSpace(tc.FilePath))
	return strings.Trim(fallback.String(), "|")
}

func stringParam(tc ToolCall, key string) string {
	if tc.Params == nil {
		return ""
	}
	value, _ := tc.Params[key].(string)
	return strings.TrimSpace(value)
}

func firstNonEmptySummaryBatch(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
