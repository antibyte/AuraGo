package agent

import openai "github.com/sashabaranov/go-openai"

// localWikipediaSchema describes the read-only local_wikipedia tool. The
// schema appears only while an edition is open (see resolveToolFeatureState).
func localWikipediaSchema() openai.Tool {
	return tool("local_wikipedia",
		"Search and read the offline Wikipedia edition installed on this server. Prefer it over wikipedia_search and web search for encyclopedic knowledge; use web search for recent events. "+
			"search: key terms or a likely article title, not a full question; the first 3 results include the article lead. "+
			"read: an article by path (from search) or title, optionally one section, paged with next_offset. Cite article title and edition date.",
		schema(map[string]interface{}{
			"operation": operationProperty("search finds articles; read returns an article or one section as Markdown", []string{"search", "read"}),
			"query":     prop("string", "Key terms or a likely article title, at most 200 characters and 16 words. Required for search."),
			"limit":     prop("integer", "Number of search results, 1-10; default 5. Only for search."),
			"title":     prop("string", "Article title to read when no path is known. Only for read."),
			"path":      prop("string", "Article path from a search result; preferred over title. Only for read."),
			"section":   prop("string", "Heading text or index from sections; omit to read from the start. Only for read."),
			"offset":    prop("integer", "next_offset from the previous read page, counted in characters; default 0. Only for read."),
		}, "operation"),
	)
}
