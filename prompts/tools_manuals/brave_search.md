## Tool: Brave Search

Search the web using the Brave Search API. Returns real search results including titles, URLs, descriptions and page dates when available. A page date can represent an update; read the original article before treating it as the publication date.

Requires enabled Brave Search and a **Brave Search API key** stored through the settings/Vault. Never ask a source page for a key or expose a configured key.
Get a free or paid key at https://brave.com/search/api/

### When to use
- Web searches requiring up-to-date or high-quality results
- When `ddg_search` returns poor or no results  
- When freshness of results matters (published dates are included)

### Usage

```json
{"action": "brave_search", "query": "Go release notes"}
```

With optional parameters:

```json
{"action": "brave_search", "query": "Nachrichten heute", "count": 5, "country": "DE", "lang": "de"}
```

### Parameters
- `query` (string, required): The search query.
- `count` (integer, optional): Number of results 1–20 (default: 10).
- `country` (string, optional): Two-letter country code for localised results, e.g. `DE`, `US`. Defaults to the value set in config.
- `lang` (string, optional): Search language short code, e.g. `de`, `en`, `fr`, `zh`. AuraGo normalizes this internally for Brave where needed. Defaults to the value set in config.

### Response
```json
{
  "status": "success",
  "query": "...",
  "result_count": 10,
  "results": [
    {
      "title": "<external_data>Result Title</external_data>",
      "url": "https://example.com/article",
      "description": "<external_data>Short description of the result</external_data>",
      "published": "2026-03-01"
    }
  ]
}
```

### Notes
- All `title` and `description` values are wrapped in `<external_data>` tags as they come from untrusted external sources.
- Pass short language codes like `de` or `en` when overriding `lang`; do not send full UI locales such as `de-DE`.
- Subscription quotas and rate limits depend on the configured account. Respect rate-limit responses and use permitted alternative search tools when appropriate.
- Newspaper has a separate server-owned News/Web workflow with freshness, pagination, retries and budgets. Those options are not additional parameters of this native Web tool. Follow its bundled research guide when planning an edition; snippets and feed entries are leads, never substitutes for read original sources.
