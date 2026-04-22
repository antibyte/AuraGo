---
description: browser_automation: Full browser automation through an optional Playwright sidecar with persistent sessions.
---

# `browser_automation` Tool

The **browser_automation** tool gives you a full browser session for multi-step website workflows.

It is designed for tasks such as:
- opening a website and navigating through multiple pages
- clicking buttons and links
- typing into form fields
- selecting dropdown values
- waiting for UI state changes
- taking screenshots for later `analyze_image`
- uploading workspace files into web forms
- retrieving browser downloads

**Requires:** both `browser_automation.enabled: true` and `tools.browser_automation.enabled: true`.

## Operating Pattern

Use the tool as a session loop:

1. `create_session` with an optional `url`
2. `extract` or `current_state`
3. choose one small action
4. `click`, `type`, `select`, `press`, or `upload_file`
5. `wait_for` if the page needs time to change
6. `extract` again
7. `screenshot` when visual proof is useful

Do not guess a whole flow blindly. Inspect the page state after each important step.

## Operations

### `create_session`
Creates a new browser session and optionally navigates to `url`.

### `close_session`
Closes an existing session and frees resources.

### `navigate`
Loads a new page URL in the existing session.

### `click`
Clicks an element identified by a CSS selector.

### `type`
Fills an input or textarea identified by a CSS selector.

### `select`
Selects a value in a `<select>` element.

### `press`
Sends a keyboard key such as `Enter`, `Escape`, or `Tab`.

### `wait_for`
Waits for a selector state (`visible`, `hidden`, `attached`, `detached`) or a page load state (`load`, `networkidle`).

### `extract`
Returns a compact structured page snapshot:
- URL
- title
- visible text summary
- form fields
- buttons
- links
- interactive elements
- optional compact DOM snippet

### `current_state`
Cheap version of `extract` for recovery/debugging.

### `screenshot`
Saves a screenshot into the workspace and returns local and web paths.

### `upload_file`
Uploads a workspace file into a file input element.

### `list_downloads`
Lists files downloaded by this browser session.

### `get_download`
Returns the chosen download from the allowed download directory.

## Important Parameters

- `operation`: required operation name
- `session_id`: required for all operations except `create_session`
- `url`: used for `create_session` and required for `navigate`
- `selector`: CSS selector for the target element; required for `click`, `type`, `select`, `upload_file`, and selector-based `wait_for`
- `text`: text for `type`
- `value`: selected value for `select` and required there
- `key`: key name for `press`
- `wait_for`: required for `wait_for`; one of `visible`, `hidden`, `attached`, `detached`, `load`, `networkidle`
- `timeout_ms`: optional timeout in milliseconds
- `output_path`: workspace-relative screenshot path
- `full_page`: full page screenshot
- `file_path`: workspace-relative file path for `upload_file`
- `download_name`: choose one file for `get_download`
- `dom_snippet`: include a compact DOM snippet in `extract`
- `max_elements`: cap the number of returned interactive elements

## Usage Hints

- Prefer selectors from `extract` over inventing your own.
- When a page changes after a click, call `wait_for` or `current_state` before the next action.
- For visual verification, call `screenshot` and then `analyze_image`.
- Never request full HTML dumps. The tool already returns a compact structured summary.
- In read-only mode, mutating actions are blocked.
