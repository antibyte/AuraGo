# Homepage Studio annotations — design

Date: 2026-10-26. Status: approved by the user (snapshot approach, element click + area drag, all preview targets,
annotations travel with the next chat message).

## Goal

In Homepage Studio the user can mark places on the previewed page and attach a short comment to each mark, so the agent
knows **where** and **what** to change. A mark is either a real page element (click) or a free area (drag). The marks
go to the agent together with the next chat message: an image with numbered crops plus structured element details.

## Constraint that shapes the design

The preview is an `<iframe sandbox="allow-scripts allow-forms">` (`ui/js/desktop/apps/homepage-studio.js:723-753`)
whose URL is on another origin: the local web server (`http://<host>:<WebServerPort>`), the Tailscale exposure
(`https://<host>:8443`), a tunnel URL, or a deployed target (Vercel, Netlify, here.now, remote). The studio cannot read
the frame's DOM, its scroll position or the sub-page the user navigated to.

## Approach

Chosen: **snapshot annotation.** Pressing *Annotate* captures the page server-side with headless Chromium at the
current device width and returns a full-page screenshot plus an element map. The user annotates on that frozen image;
hit-testing against the element map gives DevTools-like element picking without touching the site.

Rejected:

- **Live picker via proxy injection** — needs AuraGo to proxy and rewrite every target (root-relative URLs, CSP,
  frameworks, third-party hosts) and to inject script into served pages. Fragile and a larger attack surface; cannot
  cover all targets cleanly.
- **Free overlay on the live iframe** — the scroll offset of a cross-origin frame is unknown, so neither a matching
  image nor element mapping is possible below the fold.

## User experience

- **Entry.** A new *Annotate* button (pen icon) in the preview header next to the device buttons. Disabled, with a
  tooltip reason, when no preview target is reachable.
- **Capture.** Clicking it shows "Capturing page…" over the preview, then replaces the live iframe with the snapshot
  image inside the same device frame (same width and scale logic as `homepage-studio-preview.js`
  `applyDeviceViewport`; device widths 1280 / 768 / 390). The snapshot area scrolls vertically.
- **Mode toolbar.** Replaces the preview header while the mode is active: *Path* field (prefilled `/`, the studio cannot
  know the iframe's current sub-page), current device width, *Recapture*, *Done*. Changing the path or device and
  pressing *Recapture* takes a new snapshot; existing marks are discarded after a confirm when they belong to the old
  snapshot.
- **Hover.** An outline around the smallest element box under the pointer plus a small label (`tag.class` and the
  first words of its text).
- **Click** on an element → element mark. **Drag** (more than 6 px) → area mark (rectangle). Each new mark gets the
  next number and opens a comment popover anchored to it: textarea, *Save* (Enter), *Cancel* (Esc), *Delete* for
  existing marks. An empty comment cancels the mark.
- **Markers.** Element marks: accent outline plus a numbered badge. Area marks: dashed warning outline plus a
  numbered badge. Clicking a marker reopens its popover.
- **Chips.** Every saved mark appears as a numbered chip above the chat input (number + comment, ellipsised). Chips
  are focusable buttons; Enter opens the popover (switching to annotate mode if needed), Delete removes the mark. A
  *Discard all* control appears with two or more chips.
- **Sending.** The chat send button accepts an empty text field when at least one mark exists; the message then
  defaults to the localized sentence "Apply the marked changes." After sending, the user bubble shows a thumbnail of
  the composed image and the numbered comment list; chips and markers are cleared and the preview returns to the live
  iframe.
- **Keyboard.** In annotate mode the snapshot area is focusable. Tab / Shift+Tab step through a reading-order list of
  significant elements (headings, links, buttons, images, form fields, landmarks/sections) and show the hover outline
  on the focused one; Enter creates an element mark. Esc leaves the mode (marks stay as chips).
- **Lifetime.** Marks belong to one snapshot of one site and target. Switching site or target discards them with a
  notice. Marks are not written to the localStorage draft (`persistHomepageDraft`), so a reload drops them.

## Architecture

### Server: snapshot endpoint

`POST /api/homepage/annotation-snapshot`, registered next to the other homepage routes. The studio calls it through
the desktop proxy `/api/desktop/integrations/homepage/annotation-snapshot`, whose POST admission (`desktopWrite`)
refuses it in read-only mode like the Homepage Studio chat.

Request:

```json
{ "site_id": 12, "target": "local", "path": "/about", "viewport": { "width": 1280, "height": 800 } }
```

- The server decides what is captured:
  - `local`: `http://127.0.0.1:<cfg.Homepage.WebServerPort>` (the same backend the Tailscale homepage proxy uses,
    `internal/tsnetnode/tsnetnode.go:2500`), only when the web container or Python server is running. Any client URL
    is ignored.
  - `vercel`, `netlify`, `here_now`, `remote`: the client sends `preview_url`, the URL its iframe shows. The server
    accepts it only when it matches (normalized) a URL registered for a managed site (deploy targets, deployments,
    last deploy URL, remote observations) and captures its own registered copy. Anything else answers
    `preview_unavailable`. This keeps the snapshot identical to the previewed deployment.
- `path` must start with `/`, must not start with `//`, must not contain a scheme, backslash or control characters, at
  most 512 bytes. Query and fragment are allowed.
- `viewport.width` ∈ {1280, 768, 390}; `viewport.height` 400–1600.

Response (200):

```json
{
  "url": "http://127.0.0.1:8080/about",
  "path": "/about",
  "viewport": { "width": 1280, "height": 800 },
  "page": { "width": 1280, "height": 4210, "truncated": false },
  "image": "data:image/jpeg;base64,…",
  "elements": [
    { "i": 0, "selector": "main > section.hero", "tag": "section", "role": "region", "text": "…",
      "attrs": { "aria-label": "Hero" }, "box": [0, 64, 1280, 620], "parent": -1, "tab": true }
  ]
}
```

Errors: 400 invalid input, 404 no reachable preview for the target, 409 a capture is already running, 503 headless
browser unavailable, 504 page did not load in time. Bodies use the existing `jsonError` shape with a stable `code`
so the client can localize the message.

### Server: capture engine

A new file in `internal/tools` (homepage area), modeled on `internal/scraper/scraper.go` `FetchDynamic`:

- go-rod launcher (headless, same launcher flags), one capture at a time (slot channel), 30 s overall timeout, browser
  killed and cleaned up after each capture.
- Network policy through a per-capture filtering forward proxy (`internal/security/capture_proxy.go`): Chrome runs
  with `--proxy-server` and `--proxy-bypass-list=<-loopback>`, so every connection (documents, subresources, service
  workers, WebSockets, prerenders, workers) passes the proxy. The local preview origin (`127.0.0.1:<WebServerPort>`)
  is dialed as given; every other destination is resolved and checked with the SSRF rules and the validated IP is
  dialed, so private, loopback and link-local destinations stay blocked. The proxy caps concurrent connections (16),
  connections per capture (600) and relayed bytes (64 MiB). WebRTC is limited to proxied traffic. A CDP Fetch hijack
  is not enough: it never sees service-worker, WebSocket or prerender requests. A deployment on a private network
  answers `capture_blocked`.
- Navigate to origin + path at the requested viewport, device scale factor 1, wait for load plus network idle (bounded),
  then evaluate the element-map script and take a full-page screenshot as JPEG (quality ~80). Page height is capped at
  12 000 CSS px; a longer page sets `page.truncated`.
- Element map script (runs in the page):
  - visible elements only (non-zero box ≥ 4 × 4 px, not `display:none`/`visibility:hidden`/`opacity:0`), at most
    2000, document order;
  - `selector`: `#id` when unique, else a unique `[data-*]` attribute, else the shortest unique
    `tag.class:nth-of-type()` path from the nearest uniquely identifiable ancestor;
  - `text`: normalized own-and-descendant text, at most 120 chars; `attrs`: `href`, `src`, `alt`, `aria-label`,
    `name`, `type`, `placeholder` (each at most 200 chars);
  - `box` in page CSS pixels `[x, y, w, h]`; `parent` index of the nearest mapped ancestor; `tab` marks significant
    elements for keyboard stepping.

### Client: `homepage-studio-annotate.js`

New module in `ui/js/desktop/apps/`, same `create(deps)` / `dispose()` pattern as `homepage-studio-preview.js`,
`-history.js` and `-sites.js`, loaded through the existing module loader and bundle lists. `homepage-studio.js` wires
it in and owns only the button, the chips container and the send integration.

Responsibilities:

- mode state (idle → capturing → annotating → error), snapshot fetch with abort on dispose / site switch;
- overlay rendering on top of the snapshot image (scaled with the device frame), hit-testing (smallest box containing
  the point, ties → deepest), drag rectangles, markers, popover;
- mark model: `{ n, kind: 'element'|'area', comment, box, elementIndex? , overlapIndexes? }`; for an area, the up to
  five largest elements overlapping it by at least 50 % of their own area;
- image composition on send: one canvas, one crop per mark (mark box plus 48 px margin, clamped, scaled to at most
  1000 px wide), mark outline and number drawn in, crops stacked with 16 px gaps, total height at most 1600 px (crops
  shrink proportionally when needed), exported as JPEG base64 of at most 1.4 MB (the chat body limit is 2 MiB and
  the vision pipeline resizes to a 1600 px edge anyway). The snapshot comes from AuraGo's own origin as a data URI,
  so the canvas is not tainted.

### Chat integration

- `sendMessage` adds `image_base64` (the composed image, existing camera field) and a new `annotations` array to the
  chat context when marks exist:

  ```json
  { "page_path": "/about", "viewport_width": 1280, "items": [
      { "n": 1, "kind": "element", "comment": "Make this button bigger and green",
        "selector": "a.btn-primary", "tag": "a", "text": "Get started", "attrs": { "href": "/signup" },
        "box": [540, 410, 180, 48] },
      { "n": 2, "kind": "area", "comment": "An image is missing here", "box": [80, 900, 560, 240],
        "elements": [ { "selector": "section.features > div:nth-of-type(1)", "tag": "div", "text": "…" } ] } ] }
  ```

  Limits enforced server-side: at most 20 items, comment at most 1000 chars, element fields truncated as in the map.
- The message text that is persisted to history is the user's text followed by a localized "Marks:" list containing
  only the numbers and the user's comments. Page-derived strings never enter the persisted user message.
- `desktopChatContext` (`internal/server/desktop_handlers_chat.go:502`) gets an `Annotations` field.
  `buildHomepageStudioAgentContext` appends an annotations block wrapped in `desktopExternalData` (untrusted: selectors,
  texts and attributes come from the rendered page, which can be third-party content on deployed targets). The block
  lists per number: page path, viewport width, kind, selector, tag, text, attributes, box, and for areas the
  overlapping elements; plus the hint that the numbers match the attached image and that source files are found via
  `homepage_file` search for the text, id or class.
- Images: the existing camera path handles inline images and the fallback vision analysis. When the provider cannot
  take images at all (`mainProviderRequiresPublicImageURL`, failed vision fallback), annotation turns are sent
  **without** the image instead of failing, and the agent prompt says that the image is missing. Camera photos keep
  their current error behavior.
- Scope limit (v1): element details live only in the turn context. Follow-up turns see the persisted comments only.

## Error handling

| Situation | Behavior |
| --- | --- |
| No reachable preview for the target | *Annotate* disabled with tooltip reason; endpoint 404 |
| Headless browser cannot start | Mode shows "Page capture unavailable" plus hint; endpoint 503 |
| Page load timeout / navigation error | Error state with *Recapture*; endpoint 504 |
| Capture already running | Endpoint 409; client shows "Capture in progress" and keeps the button busy |
| Invalid path | Endpoint 400; inline error at the *Path* field |
| Page taller than 12 000 px | Notice "Page cut off at the bottom" in the toolbar |
| Provider without image input | Turn sent without image; the agent prompt notes it |
| Site / target switch with open marks | Marks discarded, notice announced |
| Readonly session | Same admission as the studio chat; *Annotate* hidden when chat is not allowed |

All user-facing strings live under `homepage_studio.annotate_*` in all 16 desktop locales (`ui/lang/desktop/*.json`).
Status changes are announced through the existing `announce()` live region, not per frame.

## Testing

Lean, essential tests only:

- Go unit: target → origin resolution, path and viewport validation, SSRF allow/deny decisions,
  `annotations` limits, `buildHomepageStudioAgentContext` rendering (block present, wrapped as external data, persisted
  message free of page-derived text).
- Go with rod against an `httptest` server: screenshot is produced, element map selectors are unique and resolve to
  the right node, text truncated, hidden elements skipped, private-address subresource blocked.
- UI browser test in `ui/homepage_studio_workbench_browser_test.go` with a mocked snapshot endpoint: element click,
  area drag, comment save, chip shown, send request carries `annotations` and `image_base64`, bubble thumbnail shown,
  keyboard Tab + Enter creates a mark.
- Existing guards must stay green: desktop i18n lint (16 locales), no external assets, theme contrast, Homepage Studio
  static tests (`ui/desktop_homepage_studio_test.go`).

## Documentation

- Homepage Studio contract in `ui/js/desktop/apps/AGENTS.md` (new module, button, chips, chat context field).
- Web UI chapter of the manual (`documentation/manuals/{en,de}/04-webui.md`): how to annotate.

## Changes after planning (2026-10-26)

Code facts found while writing `docs/superpowers/plans/2026-10-26-homepage-studio-annotations.md` changed five
details; the sections above already reflect them:

1. The studio reaches the endpoint through the desktop proxy `/api/desktop/integrations/homepage/…`.
2. The composed image is capped at 1000 × 1600 px and 1.4 MB base64 (2 MiB chat body limit, 1600 px vision resize).
3. Only the local preview origin bypasses the SSRF guard; deployed targets are captured through it.
4. A missing image is noted in the agent prompt instead of a separate studio notice.
5. `homepage-studio.js` is at its 1100-line budget, so the site-pill rendering moves into `homepage-studio-sites.js`
   and all annotate code lives in the new module.

The backend review of the first implementation batch changed three more:

6. Network control moved from the CDP Fetch hijack to a filtering forward proxy (a real-Chrome probe showed service
   workers, WebSockets and prerenders bypassing the hijack); the shared `GuardRodPageSSRF` stays untouched.
7. Deployed targets are captured from the client's `preview_url` once it matches a registered URL, so the snapshot
   shows the same deployment as the iframe.
8. The capture emulates a desktop-style viewport at every width (`Mobile: false`), matching the plain iframe; a
   vision-fallback description of the marks image enters the prompt only as untrusted external data.

## Out of scope

- Persistent per-site annotation task lists and agent tools to resolve marks.
- Live (non-snapshot) picking, hover/menu states, multi-page marks in one message.
- Automatic source-file hints computed by the server.
