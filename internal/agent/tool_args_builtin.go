package agent

type webScraperArgs struct {
	URL         string
	SearchQuery string
}

type wikipediaSearchArgs struct {
	Query       string
	Language    string
	SearchQuery string
}

type ddgSearchArgs struct {
	Query       string
	MaxResults  int
	SearchQuery string
}

type virusTotalScanArgs struct {
	Resource string
	FilePath string
	Mode     string
}

type braveSearchArgs struct {
	Query   string
	Count   int
	Country string
	Lang    string
}

type paperlessArgs struct {
	Operation  string
	DocumentID string
	Query      string
	Content    string
	Title      string
	Tags       string
	Name       string
	Category   string
	Limit      int
}

func builtinArgsFromToolCall(tc ToolCall) map[string]interface{} {
	args := make(map[string]interface{})
	for key, value := range tc.Params {
		if isEmptySkillArgValue(value) {
			continue
		}
		args[key] = value
	}
	for key, value := range synthesizeDirectBuiltinArgs(tc) {
		if isEmptySkillArgValue(value) {
			continue
		}
		args[key] = value
	}
	return args
}

func toolArgString(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if raw, ok := args[key]; ok {
			if value, ok := raw.(string); ok && value != "" {
				return value
			}
		}
	}
	return ""
}

func toolArgInt(args map[string]interface{}, fallback int, keys ...string) int {
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

func decodeWebScraperArgs(args map[string]interface{}) webScraperArgs {
	return webScraperArgs{
		URL:         toolArgString(args, "url"),
		SearchQuery: toolArgString(args, "search_query"),
	}
}

func decodeWikipediaSearchArgs(args map[string]interface{}) wikipediaSearchArgs {
	return wikipediaSearchArgs{
		Query:       toolArgString(args, "query"),
		Language:    toolArgString(args, "language"),
		SearchQuery: toolArgString(args, "search_query"),
	}
}

func decodeDDGSearchArgs(args map[string]interface{}) ddgSearchArgs {
	return ddgSearchArgs{
		Query:       toolArgString(args, "query"),
		MaxResults:  toolArgInt(args, 5, "max_results"),
		SearchQuery: toolArgString(args, "search_query"),
	}
}

func decodeVirusTotalScanArgs(args map[string]interface{}) virusTotalScanArgs {
	return virusTotalScanArgs{
		Resource: toolArgString(args, "resource"),
		FilePath: toolArgString(args, "file_path", "path", "filepath"),
		Mode:     toolArgString(args, "mode"),
	}
}

func decodeBraveSearchArgs(args map[string]interface{}) braveSearchArgs {
	return braveSearchArgs{
		Query:   toolArgString(args, "query"),
		Count:   toolArgInt(args, 10, "count"),
		Country: toolArgString(args, "country"),
		Lang:    toolArgString(args, "lang"),
	}
}

func decodePaperlessArgs(args map[string]interface{}) paperlessArgs {
	query := toolArgString(args, "query")
	if query == "" {
		query = toolArgString(args, "content")
	}

	return paperlessArgs{
		Operation:  toolArgString(args, "operation"),
		DocumentID: toolArgString(args, "document_id", "id"),
		Query:      query,
		Content:    toolArgString(args, "content"),
		Title:      toolArgString(args, "title"),
		Tags:       toolArgString(args, "tags"),
		Name:       toolArgString(args, "name"),
		Category:   toolArgString(args, "category"),
		Limit:      toolArgInt(args, 0, "limit"),
	}
}
