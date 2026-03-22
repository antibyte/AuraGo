---
tool: web_performance_audit
version: 1
tags: ["web_capture"]
---

# Web Performance Audit Tool

Measure page load performance of any URL using a headless Chromium browser. Returns Core Web Vitals and related metrics for diagnosing slow pages or comparing performance.

Requires the same headless browser as `web_capture` (go-rod).

## Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `url` | string | Yes | Full HTTP/HTTPS URL to audit |
| `viewport` | string | No | Browser viewport as `WIDTHxHEIGHT` (default: `1280x720`) |

## Output Metrics

| Metric | Description |
|--------|-------------|
| `ttfb_ms` | Time To First Byte — server response time |
| `fcp_ms` | First Contentful Paint — when first content appears |
| `dom_content_loaded_ms` | DOM parsed and ready |
| `load_complete_ms` | Full page load with all resources |
| `dom_interactive_ms` | DOM ready for interaction |
| `resource_count` | Total number of loaded resources |
| `transfer_size_bytes` | Total transfer size of all resources |
| `dom_elements` | Total DOM elements on the page |
| `js_heap_used_bytes` | JavaScript heap memory used |
| `js_heap_total_bytes` | JavaScript heap memory allocated |
| `largest_resources` | Top 5 largest resources by size (name, type, duration, size) |

## Examples

Basic audit:
```json
{"url": "https://example.com"}
```

Audit with mobile viewport:
```json
{"url": "https://example.com", "viewport": "375x812"}
```

Audit with desktop viewport:
```json
{"url": "https://example.com", "viewport": "1920x1080"}
```

## Interpreting Results

| Metric | Good | Needs Work | Poor |
|--------|------|-----------|------|
| TTFB | < 200ms | 200-600ms | > 600ms |
| FCP | < 1800ms | 1800-3000ms | > 3000ms |
| Load | < 3000ms | 3000-6000ms | > 6000ms |
