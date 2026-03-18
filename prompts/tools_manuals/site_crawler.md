# Tool: `site_crawler`

## Purpose
Crawl a website starting from a URL, following links to discover and extract content from multiple pages. Returns page titles, link counts, and text previews.

## When to Use
- Map the structure of a website or documentation site.
- Extract content from multi-page sites for analysis.
- Find specific information spread across multiple pages.
- Discover all pages on a domain or subdomain.

## Parameters
| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | ✅ | Starting URL (http or https) |
| `max_depth` | integer | ❌ | Maximum link depth to follow (1–5, default: 2) |
| `max_pages` | integer | ❌ | Maximum pages to crawl (1–100, default: 20) |
| `allowed_domains` | string | ❌ | Comma-separated domain whitelist (default: same domain as start URL) |
| `selector` | string | ❌ | CSS selector to extract specific content from each page |

## Output
JSON with: `status`, `pages_crawled`, `links_found`, `pages` (array of `{url, title, content_preview}`).

## Behaviour
- Respects robots.txt.
- Same-domain restriction by default (prevents crawling external sites).
- Content previews are limited to 500 characters per page.
- External content is wrapped in security isolation tags.
- Requires `web_scraper` permission to be enabled.

## Example Calls
```json
{ "url": "https://docs.example.com" }
{ "url": "https://wiki.local", "max_depth": 3, "max_pages": 50 }
{ "url": "https://example.com", "allowed_domains": "example.com,blog.example.com", "max_depth": 2 }
```
