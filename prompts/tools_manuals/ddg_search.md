## Tool: DDG Search

Search the web using DuckDuckgo's simplified HTML interface. Returns top search results with titles and descriptions.

When **summary mode** is enabled, the search results are automatically sent to a
separate summarisation model before being returned to you. In this mode you
**should** include the `search_query` parameter so the summariser knows which
information to synthesise from the results.

### Usage (normal mode — raw search results returned)

```json
{"action": "ddg_search", "query": "latest news about AI", "max_results": 5}
```

### Usage (summary mode — only a focused summary is returned)

```json
{"action": "ddg_search", "query": "latest news about AI", "max_results": 5, "search_query": "What are the most significant AI developments this week?"}
```

### Parameters
- `query` (string, required): The search query.
- `max_results` (integer, optional): Maximum number of results to return (default: 5).
- `search_query` (string, optional but **recommended in summary mode**): Tell the summarisation model exactly what information to synthesise from the search results. Be specific.
