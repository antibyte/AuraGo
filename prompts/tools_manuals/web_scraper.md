## Tool: Web Scraper

Extract plain text content from a web page by removing HTML tags, scripts, and styles.
It can also parse RSS/Atom feeds into structured items.
RSS/Atom text fields are treated as external data and wrapped in isolation tags;
decode or summarise the `content` field when you need plain article text.

When **summary mode** is enabled, the scraped content is automatically sent to a
separate summarisation model before being returned to you. In this mode you
**must** include the `search_query` parameter so the summariser knows which
information to extract. Without a clear search query the summary will be generic
and may miss the data you need.

### Usage (normal mode — full page text returned)

```json
{"action": "web_scraper", "url": "https://example.com/news/123"}
```

### Usage (RSS/Atom feed)

```json
{"action": "web_scraper", "url": "https://www.tagesschau.de/xml/rss2", "mode": "rss"}
```

### Usage (JavaScript-rendered page)

```json
{"action": "web_scraper", "url": "https://example.com/app-news", "mode": "dynamic", "wait_for_selector": "article"}
```

### Usage (summary mode — only a focused summary is returned)

```json
{"action": "web_scraper", "url": "https://example.com/news/123", "search_query": "What is the release date and price of the new product?"}
```

### Parameters
- `url` (string, required): The full URL of the page to scrape.
- `mode` (string, optional): `auto` (default), `static`, `dynamic`, or `rss`. Use `rss` for RSS/Atom feeds. `auto` detects feed URLs or RSS/XML responses and may use dynamic rendering when static content is too thin.
- `wait_for_selector` (string, optional): CSS selector to wait for in `dynamic` mode or auto dynamic fallback.
- `selector` (string, optional): CSS selector to extract matching element(s). When omitted, the full readable page is returned as Markdown.
- `fields` (object, optional): Field mapping for structured row extraction. Keys are field names; values are CSS selectors relative to `selector`. Append `@attr` to read an attribute, e.g. `"link": "a@href"`.
- `output_format` (string, optional): `auto` (default), `text`, `html`, `rows`, `table`. `auto` picks `rows` if `fields` is set, `list` if `attribute` is set, otherwise `text`.
- `attribute` (string, optional): Attribute name to extract when `output_format` is `list`, e.g. `href` or `src`.
- `limit` (integer, optional): Maximum number of matches to return when `selector` is set (1–1000, default: 50).
- `search_query` (string, optional but **required in summary mode**): Tell the summarisation model exactly what information you are looking for on the page. Be specific — e.g. "pricing details and system requirements" rather than just "info".

### Structured row extraction

Use `selector` to pick repeating elements and `fields` to define which sub-values to capture.

```json
{"action":"web_scraper", "url":"https://shop.example.com", "selector":".product", "fields":{"name":"h2","price":".price","link":"a@href"}, "output_format":"rows"}
```

Returns:

```json
{
  "status": "success",
  "mode": "static",
  "selector": ".product",
  "output_format": "rows",
  "count": 2,
  "fields": ["name", "price", "link"],
  "matches": [
    {"name": "Widget", "price": "9,99 €", "link": "..."}
  ]
}
```

### Table extraction

Point `selector` at a `<table>` and set `output_format` to `table`.

```json
{"action":"web_scraper", "url":"https://example.com/data", "selector":"table.stats", "output_format":"table"}
```

Returns `headers` and `rows` arrays.

### Attribute list extraction

Extract all values of an attribute, e.g. every image source.

```json
{"action":"web_scraper", "url":"https://example.com", "selector":"img", "attribute":"src", "output_format":"list"}
```
