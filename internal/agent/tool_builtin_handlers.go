package agent

import (
	"context"
	"fmt"

	"aurago/internal/tools"
)

func handleBuiltinSkillAction(ctx context.Context, dc *DispatchContext, action string, args map[string]interface{}, viaSkill bool) (string, bool) {
	cfg := dc.Cfg
	logger := dc.Logger

	switch action {
	case "web_scraper":
		if !cfg.Tools.WebScraper.Enabled {
			return "Tool Output: [PERMISSION DENIED] web_scraper is disabled in settings (tools.web_scraper.enabled: false).", true
		}
		urlStr := skillArgString(args, "url")
		scraped := tools.ExecuteWebScraper(urlStr)
		if cfg.Tools.WebScraper.SummaryMode {
			searchQuery := skillArgString(args, "search_query")
			if searchQuery == "" {
				searchQuery = "general summary of the page content"
			}
			summary, err := tools.SummariseScrapedContent(ctx, cfg, logger, scraped, searchQuery)
			if err != nil {
				logger.Warn("web_scraper summary failed, returning raw content", "error", err)
			} else {
				scraped = summary
			}
		}
		return scraped, true

	case "wikipedia_search":
		queryStr := skillArgString(args, "query")
		langStr := skillArgString(args, "language")
		result := tools.ExecuteWikipediaSearch(queryStr, langStr)
		if cfg.Tools.Wikipedia.SummaryMode {
			searchQuery := skillArgString(args, "search_query")
			if searchQuery == "" {
				searchQuery = "summarise the key facts about: " + queryStr
			}
			summary, err := tools.SummariseContent(ctx, tools.ResolveSummaryLLMConfig(cfg, tools.SummaryLLMConfig{
				APIKey:  cfg.Tools.Wikipedia.SummaryAPIKey,
				BaseURL: cfg.Tools.Wikipedia.SummaryBaseURL,
				Model:   cfg.Tools.Wikipedia.SummaryModel,
			}), logger, result, searchQuery, "Wikipedia article")
			if err != nil {
				logger.Warn("wikipedia summary failed, returning raw content", "error", err)
			} else {
				result = summary
			}
		}
		return result, true

	case "ddg_search":
		queryStr := skillArgString(args, "query")
		maxRes := skillArgInt(args, 5, "max_results")
		result := tools.ExecuteDDGSearch(queryStr, maxRes)
		if cfg.Tools.DDGSearch.SummaryMode {
			searchQuery := skillArgString(args, "search_query")
			if searchQuery == "" {
				searchQuery = "synthesise the most relevant findings for: " + queryStr
			}
			summary, err := tools.SummariseContent(ctx, tools.ResolveSummaryLLMConfig(cfg, tools.SummaryLLMConfig{
				APIKey:  cfg.Tools.DDGSearch.SummaryAPIKey,
				BaseURL: cfg.Tools.DDGSearch.SummaryBaseURL,
				Model:   cfg.Tools.DDGSearch.SummaryModel,
			}), logger, result, searchQuery, "search results")
			if err != nil {
				logger.Warn("ddg_search summary failed, returning raw content", "error", err)
			} else {
				result = summary
			}
		}
		return result, true

	case "virustotal_scan":
		if !cfg.VirusTotal.Enabled {
			return `Tool Output: {"status": "error", "message": "VirusTotal integration is not enabled. Set virustotal.enabled=true in config.yaml."}`, true
		}
		return tools.ExecuteVirusTotalScanWithOptions(cfg.VirusTotal.APIKey, tools.VirusTotalOptions{
			Resource: skillArgString(args, "resource"),
			FilePath: skillArgString(args, "file_path", "path", "filepath"),
			Mode:     skillArgString(args, "mode"),
		}), true

	case "brave_search":
		if !cfg.BraveSearch.Enabled {
			return `Tool Output: {"status": "error", "message": "Brave Search integration is not enabled. Enable it in Settings › Brave Search."}`, true
		}
		queryStr := skillArgString(args, "query")
		count := skillArgInt(args, 10, "count")
		country := skillArgString(args, "country")
		if country == "" {
			country = cfg.BraveSearch.Country
		}
		lang := skillArgString(args, "lang")
		if lang == "" {
			lang = cfg.BraveSearch.Lang
		}
		return tools.ExecuteBraveSearch(cfg.BraveSearch.APIKey, queryStr, count, country, lang), true

	case "paperless", "paperless_ngx":
		if !cfg.PaperlessNGX.Enabled {
			return `Tool Output: {"status": "error", "message": "Paperless-ngx integration is not enabled. Set paperless_ngx.enabled=true in config.yaml."}`, true
		}
		op := skillArgString(args, "operation")
		if cfg.PaperlessNGX.ReadOnly {
			switch op {
			case "upload", "post", "update", "patch", "delete", "rm":
				return `Tool Output: {"status":"error","message":"Paperless-ngx is in read-only mode. Disable paperless_ngx.readonly to allow changes."}`, true
			}
		}
		plCfg := tools.PaperlessConfig{
			URL:      cfg.PaperlessNGX.URL,
			APIToken: cfg.PaperlessNGX.APIToken,
		}
		docID := skillArgString(args, "document_id", "id")
		query := skillArgString(args, "query")
		if query == "" {
			query = skillArgString(args, "content")
		}
		content := skillArgString(args, "content")
		title := skillArgString(args, "title")
		tagsStr := skillArgString(args, "tags")
		corrName := skillArgString(args, "name")
		category := skillArgString(args, "category")
		limit := skillArgInt(args, 0, "limit")
		logSuffix := ""
		if viaSkill {
			logSuffix = " (via skill)"
		}
		switch op {
		case "search", "find", "query":
			logger.Info("LLM requested Paperless search"+logSuffix, "query", query)
			return "Tool Output: " + tools.PaperlessSearch(plCfg, query, tagsStr, corrName, category, limit), true
		case "get", "info":
			logger.Info("LLM requested Paperless get"+logSuffix, "document_id", docID)
			return "Tool Output: " + tools.PaperlessGet(plCfg, docID), true
		case "download", "read", "content":
			logger.Info("LLM requested Paperless download"+logSuffix, "document_id", docID)
			return "Tool Output: " + tools.PaperlessDownload(plCfg, docID), true
		case "upload", "post":
			logger.Info("LLM requested Paperless upload"+logSuffix, "title", title)
			return "Tool Output: " + tools.PaperlessUpload(plCfg, title, content, tagsStr, corrName, category), true
		case "update", "patch":
			logger.Info("LLM requested Paperless update"+logSuffix, "document_id", docID)
			return "Tool Output: " + tools.PaperlessUpdate(plCfg, docID, title, tagsStr, corrName, category), true
		case "delete", "rm":
			logger.Info("LLM requested Paperless delete"+logSuffix, "document_id", docID)
			return "Tool Output: " + tools.PaperlessDelete(plCfg, docID), true
		case "list_tags", "tags":
			logger.Info("LLM requested Paperless list tags" + logSuffix)
			return "Tool Output: " + tools.PaperlessListTags(plCfg), true
		case "list_correspondents", "correspondents":
			logger.Info("LLM requested Paperless list correspondents" + logSuffix)
			return "Tool Output: " + tools.PaperlessListCorrespondents(plCfg), true
		case "list_document_types", "document_types":
			logger.Info("LLM requested Paperless list document types" + logSuffix)
			return "Tool Output: " + tools.PaperlessListDocumentTypes(plCfg), true
		default:
			return `Tool Output: {"status": "error", "message": "Unknown paperless operation. Use: search, get, download, upload, update, delete, list_tags, list_correspondents, list_document_types"}`, true
		}
	}

	return "", false
}

func skillArgString(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if raw, ok := args[key]; ok {
			if value, ok := raw.(string); ok && value != "" {
				return value
			}
		}
	}
	return ""
}

func skillArgInt(args map[string]interface{}, fallback int, keys ...string) int {
	for _, key := range keys {
		if raw, ok := args[key]; ok {
			switch value := raw.(type) {
			case int:
				return value
			case int32:
				return int(value)
			case int64:
				return int(value)
			case float64:
				return int(value)
			case float32:
				return int(value)
			}
		}
	}
	return fallback
}

func synthesizeDirectBuiltinArgs(tc ToolCall) map[string]interface{} {
	args := synthesizeExecuteSkillArgs(tc)
	if args == nil {
		return map[string]interface{}{}
	}
	return args
}

func handleDirectBuiltinSkillAction(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	return handleBuiltinSkillAction(ctx, dc, tc.Action, synthesizeDirectBuiltinArgs(tc), false)
}

func handleExecuteSkillBuiltinAction(ctx context.Context, dc *DispatchContext, skillName string, args map[string]interface{}) (string, bool) {
	return handleBuiltinSkillAction(ctx, dc, skillName, args, true)
}

func unexpectedBuiltinActionError(action string) string {
	return fmt.Sprintf("Tool Output: ERROR builtin tool handler missing implementation for %q", action)
}
