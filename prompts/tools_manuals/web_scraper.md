## Tool: Web Scraper

Extract plain text content from a web page by removing HTML tags, scripts, and styles.

When **summary mode** is enabled, the scraped content is automatically sent to a
separate summarisation model before being returned to you. In this mode you
**must** include the `search_query` parameter so the summariser knows which
information to extract. Without a clear search query the summary will be generic
and may miss the data you need.

### Usage (normal mode — full page text returned)

```json
{"action": "web_scraper", "url": "https://example.com/news/123"}
```

### Usage (summary mode — only a focused summary is returned)

```json
{"action": "web_scraper", "url": "https://example.com/news/123", "search_query": "What is the release date and price of the new product?"}
```

### Parameters
- `url` (string, required): The full URL of the page to scrape.
- `search_query` (string, optional but **required in summary mode**): Tell the summarisation model exactly what information you are looking for on the page. Be specific — e.g. "pricing details and system requirements" rather than just "info".
