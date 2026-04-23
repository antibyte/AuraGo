## Tool: Wikipedia Search

Search and retrieve summaries from Wikipedia.

When **summary mode** is enabled, the article content is automatically sent to a
separate summarisation model before being returned to you. In this mode you
**should** include the `search_query` parameter so the summariser knows which
information to extract. Without a clear search query the summary will be generic.

### Usage (normal mode — article summary returned)

```json
{"action": "wikipedia_search", "query": "Artificial Intelligence", "language": "de"}
```

### Usage (summary mode — only a focused summary is returned)

```json
{"action": "wikipedia_search", "query": "Artificial Intelligence", "language": "en", "search_query": "What are the main subfields and recent breakthroughs?"}
```

### Parameters
- `query` (string, required): The search query or page title.
- `language` (string, optional): Wikipedia language code such as `de`, `en`, `fr`, or `ja`. If omitted, AuraGo uses the agent's system language.
- `search_query` (string, optional but **recommended in summary mode**): Tell the summarisation model exactly what information you are looking for. Be specific.

### Notes
- AuraGo first tries a direct Wikipedia page summary lookup.
- If the title does not match exactly, it automatically performs a Wikipedia search and uses the best matching article as a fallback.
