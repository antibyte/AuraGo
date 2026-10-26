package agent

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

type webScraperArgs struct {
	URL             string
	SearchQuery     string
	Mode            string
	WaitForSelector string
	Selector        string
	Fields          map[string]string
	OutputFormat    string
	Attribute       string
	Limit           int
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
		URL:             toolArgString(args, "url"),
		SearchQuery:     toolArgString(args, "search_query"),
		Mode:            toolArgString(args, "mode"),
		WaitForSelector: toolArgString(args, "wait_for_selector"),
		Selector:        toolArgString(args, "selector"),
		Fields:          toolArgStringMap(args, "fields"),
		OutputFormat:    toolArgString(args, "output_format"),
		Attribute:       toolArgString(args, "attribute"),
		Limit:           toolArgInt(args, 50, "limit"),
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
	return paperlessArgs{
		Operation:  toolArgString(args, "operation"),
		DocumentID: paperlessDocumentIDArg(args),
		Query:      toolArgString(args, "query"),
		Content:    toolArgString(args, "content"),
		Title:      toolArgString(args, "title"),
		Tags:       paperlessTagsArg(args),
		Name:       toolArgString(args, "correspondent", "name"),
		Category:   toolArgString(args, "document_type", "category"),
		Limit:      paperlessLimitArg(args),
	}
}

func paperlessDocumentIDArg(args map[string]interface{}) string {
	for _, key := range []string{"document_id", "id"} {
		raw, ok := args[key]
		if !ok || raw == nil {
			continue
		}
		if value, ok := raw.(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
		if value, ok := paperlessWholeNumber(raw); ok {
			return value
		}
	}
	return ""
}

func paperlessTagsArg(args map[string]interface{}) string {
	raw, ok := args["tags"]
	if !ok || raw == nil {
		return ""
	}
	switch value := raw.(type) {
	case string:
		return value
	case []string:
		return paperlessJoinTagParts(value)
	case []interface{}:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			switch part := item.(type) {
			case string:
				if trimmed := strings.TrimSpace(part); trimmed != "" {
					parts = append(parts, trimmed)
				}
			default:
				if number, ok := paperlessWholeNumber(part); ok {
					parts = append(parts, number)
				}
			}
		}
		return strings.Join(parts, ",")
	default:
		if number, ok := paperlessWholeNumber(value); ok {
			return number
		}
	}
	return ""
}

func paperlessJoinTagParts(parts []string) string {
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return strings.Join(out, ",")
}

func paperlessLimitArg(args map[string]interface{}) int {
	if raw, ok := args["limit"]; ok {
		if value, ok := raw.(string); ok {
			value = strings.TrimSpace(value)
			if value != "" {
				if number, err := strconv.Atoi(value); err == nil {
					return number
				}
			}
		}
	}
	return toolArgInt(args, 0, "limit")
}

func paperlessWholeNumber(raw interface{}) (string, bool) {
	switch number := raw.(type) {
	case int:
		return paperlessFormatPositive(int64(number))
	case int8:
		return paperlessFormatPositive(int64(number))
	case int16:
		return paperlessFormatPositive(int64(number))
	case int32:
		return paperlessFormatPositive(int64(number))
	case int64:
		return paperlessFormatPositive(number)
	case uint:
		if number == 0 || uint64(number) > uint64(math.MaxInt64) {
			return "", false
		}
		return paperlessFormatPositive(int64(number))
	case uint8:
		return paperlessFormatPositive(int64(number))
	case uint16:
		return paperlessFormatPositive(int64(number))
	case uint32:
		return paperlessFormatPositive(int64(number))
	case uint64:
		if number == 0 || number > uint64(math.MaxInt64) {
			return "", false
		}
		return paperlessFormatPositive(int64(number))
	case float32:
		return paperlessWholeFloat(float64(number))
	case float64:
		return paperlessWholeFloat(number)
	case json.Number:
		if whole, err := number.Int64(); err == nil {
			return paperlessFormatPositive(whole)
		}
		fraction, err := number.Float64()
		if err != nil {
			return "", false
		}
		return paperlessWholeFloat(fraction)
	default:
		return "", false
	}
}

func paperlessFormatPositive(number int64) (string, bool) {
	if number <= 0 {
		return "", false
	}
	return strconv.FormatInt(number, 10), true
}

func paperlessWholeFloat(number float64) (string, bool) {
	if number <= 0 || math.IsNaN(number) || math.IsInf(number, 0) || number != math.Trunc(number) || number > float64(math.MaxInt64) {
		return "", false
	}
	return strconv.FormatInt(int64(number), 10), true
}
