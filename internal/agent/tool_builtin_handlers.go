package agent

import (
	"context"
	"fmt"

	"aurago/internal/security"
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
		req := decodeWebScraperArgs(args)
		scraped := tools.ExecuteWebScraper(req.URL)
		if cfg.Tools.WebScraper.SummaryMode {
			searchQuery := req.SearchQuery
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
		req := decodeWikipediaSearchArgs(args)
		result := tools.ExecuteWikipediaSearch(req.Query, req.Language)
		if cfg.Tools.Wikipedia.SummaryMode {
			searchQuery := req.SearchQuery
			if searchQuery == "" {
				searchQuery = "summarise the key facts about: " + req.Query
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
		req := decodeDDGSearchArgs(args)
		if preferredResult, usedPreferred, err := tools.CallPreferredMCPWebSearch(cfg, req.Query, req.MaxResults, "", "", logger); usedPreferred {
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Preferred MCP web search failed: %v"}`, err), true
			}
			return "Tool Output: " + security.Scrub(preferredResult), true
		}
		result := tools.ExecuteDDGSearch(req.Query, req.MaxResults)
		if cfg.Tools.DDGSearch.SummaryMode {
			searchQuery := req.SearchQuery
			if searchQuery == "" {
				searchQuery = "synthesise the most relevant findings for: " + req.Query
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
		req := decodeVirusTotalScanArgs(args)
		return tools.ExecuteVirusTotalScanWithOptions(cfg.VirusTotal.APIKey, tools.VirusTotalOptions{
			Resource: req.Resource,
			FilePath: req.FilePath,
			Mode:     req.Mode,
		}), true

	case "golangci_lint":
		if !cfg.GolangciLint.Enabled {
			return `Tool Output: {"status": "error", "message": "golangci_lint tool is not enabled. Set golangci_lint.enabled=true in config.yaml."}`, true
		}
		lintPath, _ := args["path"].(string)
		configPath, _ := args["config"].(string)
		return tools.ExecuteGolangciLint(lintPath, configPath, cfg.Directories.WorkspaceDir), true

	case "brave_search":
		if !cfg.BraveSearch.Enabled {
			return `Tool Output: {"status": "error", "message": "Brave Search integration is not enabled. Enable it in Settings > Brave Search."}`, true
		}
		req := decodeBraveSearchArgs(args)
		country := req.Country
		if country == "" {
			country = cfg.BraveSearch.Country
		}
		lang := req.Lang
		if lang == "" {
			lang = cfg.BraveSearch.Lang
		}
		if preferredResult, usedPreferred, err := tools.CallPreferredMCPWebSearch(cfg, req.Query, req.Count, country, lang, logger); usedPreferred {
			if err != nil {
				return fmt.Sprintf(`Tool Output: {"status": "error", "message": "Preferred MCP web search failed: %v"}`, err), true
			}
			return "Tool Output: " + security.Scrub(preferredResult), true
		}
		return tools.ExecuteBraveSearch(cfg.BraveSearch.APIKey, req.Query, req.Count, country, lang), true

	case "paperless", "paperless_ngx":
		if !cfg.PaperlessNGX.Enabled {
			return `Tool Output: {"status": "error", "message": "Paperless-ngx integration is not enabled. Set paperless_ngx.enabled=true in config.yaml."}`, true
		}
		req := decodePaperlessArgs(args)
		if cfg.PaperlessNGX.ReadOnly {
			switch req.Operation {
			case "upload", "post", "update", "patch", "delete", "rm":
				return `Tool Output: {"status":"error","message":"Paperless-ngx is in read-only mode. Disable paperless_ngx.readonly to allow changes."}`, true
			}
		}
		plCfg := tools.PaperlessConfig{
			URL:      cfg.PaperlessNGX.URL,
			APIToken: cfg.PaperlessNGX.APIToken,
		}
		logSuffix := ""
		if viaSkill {
			logSuffix = " (via skill)"
		}
		switch req.Operation {
		case "search", "find", "query":
			logger.Info("LLM requested Paperless search"+logSuffix, "query", req.Query)
			return "Tool Output: " + tools.PaperlessSearch(plCfg, req.Query, req.Tags, req.Name, req.Category, req.Limit), true
		case "get", "info":
			logger.Info("LLM requested Paperless get"+logSuffix, "document_id", req.DocumentID)
			return "Tool Output: " + tools.PaperlessGet(plCfg, req.DocumentID), true
		case "download", "read", "content":
			logger.Info("LLM requested Paperless download"+logSuffix, "document_id", req.DocumentID)
			return "Tool Output: " + tools.PaperlessDownload(plCfg, req.DocumentID), true
		case "upload", "post":
			logger.Info("LLM requested Paperless upload"+logSuffix, "title", req.Title)
			return "Tool Output: " + tools.PaperlessUpload(plCfg, req.Title, req.Content, req.Tags, req.Name, req.Category), true
		case "update", "patch":
			logger.Info("LLM requested Paperless update"+logSuffix, "document_id", req.DocumentID)
			return "Tool Output: " + tools.PaperlessUpdate(plCfg, req.DocumentID, req.Title, req.Tags, req.Name, req.Category), true
		case "delete", "rm":
			logger.Info("LLM requested Paperless delete"+logSuffix, "document_id", req.DocumentID)
			return "Tool Output: " + tools.PaperlessDelete(plCfg, req.DocumentID), true
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

func synthesizeDirectBuiltinArgs(tc ToolCall) map[string]interface{} {
	args := synthesizeExecuteSkillArgs(tc)
	if args == nil {
		return map[string]interface{}{}
	}
	return args
}

func handleDirectBuiltinSkillAction(ctx context.Context, tc ToolCall, dc *DispatchContext) (string, bool) {
	return handleBuiltinSkillAction(ctx, dc, tc.Action, builtinArgsFromToolCall(tc), false)
}

func handleExecuteSkillBuiltinAction(ctx context.Context, dc *DispatchContext, skillName string, args map[string]interface{}) (string, bool) {
	return handleBuiltinSkillAction(ctx, dc, skillName, args, true)
}

func unexpectedBuiltinActionError(action string) string {
	return formatToolExecError(newUnexpectedBuiltinActionToolExecError(action))
}
