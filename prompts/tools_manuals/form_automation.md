---
description: form_automation: Fill and submit web forms using a headless Chromium browser.
---

# `form_automation` Tool

The **form_automation** tool allows you to interact with web forms using a headless Chromium browser (go-rod). It can inspect form fields on any page, fill them programmatically, and submit them.

**Requires:** `tools.form_automation.enabled: true` and `tools.web_capture.enabled: true` in config.

## Operations

### `get_fields` — Inspect form inputs
Returns a list of all input, textarea, select, and submit button elements on the page with their CSS selectors, types, names, and current values.

### `fill_submit` — Fill fields and submit
Fills in form fields using CSS selectors as keys and the desired values as values. Optionally clicks a specific submit button; if no submit selector is given, the first submit button is used automatically. Optionally saves a screenshot of the result page.

### `click` — Click any element
Clicks a page element identified by a CSS selector. Useful for accepting cookie banners, toggling checkboxes, or clicking links.

## Parameters

- **`operation`** *(string, required)*: `get_fields`, `fill_submit`, or `click`
- **`url`** *(string, required)*: The page URL to load (http/https)
- **`fields`** *(string, optional)*: JSON object mapping CSS selector to value for `fill_submit`. Example: `'{"#username":"alice","#password":"s3cr3t"}'`
- **`selector`** *(string, optional)*: CSS selector for `click`, or submit button selector for `fill_submit` (default: first submit button)
- **`screenshot_dir`** *(string, optional)*: Directory to save a PNG screenshot after the action. Empty means no screenshot.

## Usage Examples

Inspect available fields on a login page:
```json
{
  "action": "form_automation",
  "operation": "get_fields",
  "url": "http://192.168.1.1/login"
}
```

Fill a login form and submit:
```json
{
  "action": "form_automation",
  "operation": "fill_submit",
  "url": "http://homelab.local/login",
  "fields": "{\"#user\": \"admin\", \"#password\": \"mypassword\"}",
  "screenshot_dir": "agent_workspace/workdir"
}
```

Click a button:
```json
{
  "action": "form_automation",
  "operation": "click",
  "url": "http://homelab.local/settings",
  "selector": "#save-btn"
}
```

## Returns

A JSON object with:
- `status`: `"success"` or `"error"`
- `operation`: the operation that was performed
- `url`: the target URL
- `fields`: (get_fields) list of discovered form fields with selector, type, name, placeholder, required
- `filled`: (fill_submit) map of selectors that were filled
- `screenshot`: path to the saved screenshot file (if screenshot_dir was set)
- `message`: human-readable summary or error message

## Notes

- Uses the same headless Chromium browser as `web_capture`. Chromium must be available on the system.
- CSS selectors: prefer `#id` or `[name="..."]` for reliability. Use `get_fields` first if you're not sure.
- Passwords are filled in memory only; they are not logged or stored.
- This tool is disabled by default. Enable it in config: `tools.form_automation.enabled: true`
