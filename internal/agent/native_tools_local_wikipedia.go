package agent

import openai "github.com/sashabaranov/go-openai"

// localWikipediaSchema describes the read-only local_wikipedia tool. The
// schema appears only while an edition is open (see resolveToolFeatureState).
// It stays compact (TestLocalWikipediaSchemaStaysCompact) so the
// encyclopedia-intent swap for wikipedia_search fits the adaptive schema
// token budget; the tool manual carries the details.
func localWikipediaSchema() openai.Tool {
	return tool("local_wikipedia",
		"Offline Wikipedia on this server: search and read articles. Prefer it over wikipedia_search and web search for encyclopedic facts (not recent events). Cite title and edition date.",
		schema(map[string]interface{}{
			"operation": operationProperty("search finds articles, read returns one as Markdown", []string{"search", "read"}),
			"query":     prop("string", "search: key terms or a likely title"),
			"limit":     prop("integer", "search: 1-10, default 5"),
			"title":     prop("string", "read: title if no path"),
			"path":      prop("string", "read: path from search"),
			"section":   prop("string", "read: heading or index"),
			"offset":    prop("integer", "read: next_offset of the last page"),
		}, "operation"),
	)
}
