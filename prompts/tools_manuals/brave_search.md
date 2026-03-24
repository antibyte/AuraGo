## Tool: Brave Search

Search the web using the Brave Search API. Returns real search results including titles, URLs, descriptions and publication dates.

Requires a **Brave Search API key** configured under `brave_search.api_key` in the settings.
Get a free or paid key at https://brave.com/search/api/

### When to use
- Web searches requiring up-to-date or high-quality results
- When `ddg_search` returns poor or no results  
- When freshness of results matters (published dates are included)

### Usage

```json
{"action": "brave_search", "query": "latest Go 1.26 release notes"}
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
- Free tier allows 2000 queries/month. Check your quota at https://api.search.brave.com/
