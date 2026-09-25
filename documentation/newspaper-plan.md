# Newspaper: a personal daily edition for the Virtual Desktop

Status: locally implemented, 2026-09-25. Package, API, browser, translation and
bundle checks have passed. No live provider research or external delivery has
been verified. This document preserves the original design targets; the current
behavior and remaining limitations are recorded in `documentation/newspaper.md`.

## Product goal

Add a first-party Virtual Desktop app called **Newspaper**. The owner selects
standard news sections and writes additional interests in plain language. AuraGo
researches the selected subjects each local day, edits an original, sourced
edition, and keeps it readable in the app. Email and Telegram delivery are
separate, explicit opt-ins; either channel can also send the current edition on
demand. A reader can see what was published, when the information was checked,
and which sources support each story.

The initial product has one installation-owned publication profile, matching the
administrative Desktop research surface. Profile changes affect the next run;
an explicit **Create new revision** action is required to replace today's
published edition. The edition is a snapshot, not a live page that quietly
changes when a source or preference changes. Corrections produce a new revision
with a visible correction note.

Success means a person can set up their interests in a few minutes, receive a
useful issue every day when the required integrations are available, and read it
comfortably from a wide Desktop window or a narrow touch viewport. An issue
with fewer verified stories is preferable to invented filler.

## Existing AuraGo seams

| Area | Present behavior | Newspaper decision |
| --- | --- | --- |
| Research | `internal/detective` has isolated agent runs, durable budgets, server-recorded evidence, source references, partial reports and exports. Its current service has a one-case execution queue, a 100-case limit and no automatic retention. | Reuse its execution and evidence patterns; extract small shared helpers only where they are genuinely common. Give Newspaper its own edition ledger and worker. Do not accumulate daily issues as Detective cases. |
| News retrieval | `internal/server/personal_radio_news.go` searches and fetches recent pages for spoken Radio bulletins, with a Brave-specific path and an eight-page fetch cap. | Reuse date/URL extraction ideas, but research across the configured, permitted search, RSS and page-reading capabilities. The Radio bulletin is too narrow to be the newspaper source of record. |
| Scheduling | `internal/tools/cron.go` supports persisted prompt jobs and source-specific runners, but a source job without its runner falls back to the general prompt callback. | Use a small Newspaper-owned timer and durable local-date ledger. A newspaper run must never become an agent prompt through scheduler fallback. |
| Email | Configured `EmailAccount` entries include SMTP and a read-only flag. `tools.SendEmail` and `SendEmailTLS` currently compose plain-text messages. AgentMail has a separate send API. | Add a bounded multipart HTML/plain-text composer over permitted configured accounts. Support AgentMail through the same delivery interface where enabled. Never have the research agent call a generic send tool. |
| Telegram | `tools.sendTelegramNotification` uses the configured `telegram_user_id` for a message. | Add a narrow edition-delivery adapter for the authorized chat and a full-edition document. Do not accept a model-supplied or API-supplied arbitrary chat ID. |
| Desktop | Built-in apps are registered in `internal/desktop/types.go`, lazily loaded by `ui/js/desktop/core/module-loader.js`, and use per-window cleanup. | Register `newspaper`, create its own lazy CSS/JS, add theme icons and all 16 Desktop locale entries. Keep generation alive when the window closes. |

GitNexus's index was three commits behind HEAD during planning; an index-only
refresh failed when workers crashed. Its pre-change walk labeled shared Desktop
registration, cron registration and SMTP sending as HIGH/CRITICAL-risk surfaces.
Implementation should be additive around these seams, inspect direct callers and
tests again, and refresh the graph before changing shared functions. Keep the
existing text-only `SendEmail` behavior intact when adding HTML delivery.

## Reader experience

### First opening and preferences

Use one calm setup screen with three clear sections rather than exposing agent
or scheduler terminology:

1. **What matters to you:** selectable sections for Regional, National,
   International, Politics, Economy, Culture, Technology, Science, Environment,
   Health and Sport. The user can add up to 20 bounded free-text interests
   such as `ESP32-P4`, `local theatre`, or `energy policy`, and optional excluded
   topics. Each interest has a clear remove control. Require at least one section
   or interest. Keep editorial breadth visible so a very narrow selection is an
   informed choice.
2. **Your place and edition:** publication language; country; optional state,
   city or region; local IANA time zone; desired daily ready time (suggested
   default 07:00); and reading length (Brief, Standard, In depth). Regional news
   asks for a location before it is enabled. Show the resulting coverage area.
3. **How to receive it:** In-app only by default. Email and Telegram have
   independent daily switches and an explicit **Send test** action. Email uses a
   selected writable account and a validated destination address confirmed by
   a one-time code sent there. Telegram uses the configured authorized user.
   Show actual channel readiness and the next expected run. Saving preferences
   never sends an issue.

**Create today's edition** starts a run immediately. During generation, show
real phases (finding, reading, editing, checking, publishing), retrieved-source
and accepted-story counts, and an honest delay/failure message. Do not invent a
percentage. The user can close the window, return later, and see the same run.
Stopping an on-demand run preserves its evidence and clearly labels any partial
result. Scheduled runs stop at their configured budget and publish a marked
partial issue only if every included story passes the source checks.

### Newspaper layout

The app has four views: **Today's edition**, **Article**, **Archive**, and
**Preferences**. The window shell retains its real minimize, maximize, drag and
resize behavior. A small Dashboard tile may show the latest issue, next run and
an actionable setup or failure state; the reading experience remains in the app.

Visual direction: an editorial paper surface inside the Desktop, with a strong
masthead, narrow date and place line, fine rules, a restrained ink and burgundy
palette, generous margins, and a local serif display face paired with a clear
sans-serif for controls. Use typesetting and hierarchy instead of a grid of
rounded dashboard cards. The user may name the publication; **Newspaper** is the
app name. No remote font, photo or script is needed for a complete first page.

| Element | Initial visual target |
| --- | --- |
| Page | A centered editorial canvas up to about 1180 px, with 32-48 px outer space on wide windows and 16 px on narrow ones. |
| Masthead | Locally bundled serif display face at roughly 56-72 px wide, 36-42 px narrow; date, place and issue number above a fine double rule. |
| Front page | A 12-column composition at wide sizes: lead story across about seven columns, secondary stories across three, a two-column brief rail. Collapse by content width, not device label. |
| Article | Body copy around 17 px with a 1.55-1.7 line height and a 60-70 character measure. Source markers remain adjacent to the supported sentence. |
| Color | Warm paper `#F7F4EC`, deep ink `#1D211F` and restrained burgundy `#8B3338` as a starting palette; provide a separately tuned dark-paper palette with equivalent contrast. |

At a wide size, the first page has a prominent lead story, a smaller secondary
column, a compact **In brief** rail and clearly separated section starts. Each
story shows a section label, headline, short deck, checked/published time and
source count. On opening a story, provide a comfortable single reading measure,
original concise prose, clear subheads, source markers next to claims, a
**Sources and context** panel, and a path back to the same place on the front
page. Show a correction, uncertainty or single-source label in text when relevant.

The Archive groups dated immutable issues and retains reading position. A
reader can search headlines and filter by section. The Preferences view keeps
the everyday choices up front; source preferences, budget and retention are
advanced controls. Empty, preparing, partial, failed, disabled and read-only
states each have a specific message and useful next action.

Use local assets and respect Standard/Fruity shell styling and light/dark
settings. The paper can use warm light or dark ink surfaces while preserving
contrast. For narrow windows, switch to one column and a compact section menu;
never shrink a fake multi-column newspaper until it becomes unreadable. Keep DOM
reading order logical, keyboard and touch targets generous, visible focus, text
resize to 200%, reduced-motion support, and no hover-only action. Verify the
layout with realistic short and long headlines, missing images, right-to-left
and non-Latin text. Custom Newspaper icons need both Papirus and WhiteSur entries.

### Delivery presentation

- **Email:** the full edition as self-contained, responsive HTML with inline
  styling, a complete plain-text alternative, visible sources and no tracking
  pixels or remote display dependencies. A protected app link is supplementary;
  the issue must remain useful when AuraGo is not reachable from the mail client.
- **Telegram:** a concise front-page message with the lead and section index,
  followed by the full issue as a readable PDF document. Use the same saved
  revision and source list. If the selected language cannot be rendered safely
  as PDF, show a delivery limitation and provide the full issue in bounded text
  messages instead of a damaged file. Telegram message size and rate limits must
  be handled by the adapter, without splitting a sentence or a source URL.
- **App:** the canonical interactive reading surface. PDF download and print
  come from the same immutable edition. A failed channel never hides a ready
  issue in the app.

The print renderer should have a real page grid, running edition/date line,
section hierarchy, source notes and deliberate page breaks. Validate glyph
coverage for the selected publication language before enabling PDF delivery.
Use the configured Gotenberg backend when appropriate, but retain an in-process
route so ordinary local installs do not require a new sidecar.

## Editorial and research contract

1. Snapshot the validated profile, configured permissions, source options,
   provider/model, local edition date and research cutoff. Discover available
   search and page-reading capabilities. Show setup blockers before accepting
   daily delivery. Private connected sources and local files stay excluded unless
   the user selects and the administrator permits an exact read scope.
2. Plan queries per selected section and free-text interest. Search in relevant
   languages, collect candidates from configured web search and RSS, then read
   promising original pages. A search snippet is a lead, not evidence. Record
   canonical URL, publisher, title, publication time when known, retrieval time,
   bounded supporting passage and retrieval status. Mark unknown publication
   dates as unknown; do not imply they are today's events.
3. Deduplicate canonical URLs, syndication copies and substantially identical
   stories. Compare against recent editions so unchanged articles are not
   presented as new. Rank by relevance, timeliness, significance, locality and
   source quality, with diversity across the chosen sections. Avoid click-driven
   ranking. Important contested claims get a second independent source or an
   explicit unresolved/single-source label.
4. The agent proposes original headlines, decks, stories and section order in a
   strict structured format under a bundled, hash-verified editorial guide
   registered through the Agent Skill Manager. Each substantive claim links to
   a server-recorded passage. Server validation rejects invented source IDs,
   unread URLs, unsupported citations, empty sections disguised as news, unsafe
   links, and stories after the cutoff. Clearly separate established facts,
   context and analysis. Never reproduce complete third-party articles or use
   unapproved source imagery.
5. A deterministic renderer creates the front page, article view, email and PDF
   from the validated edition. If retrieval is sparse, omit a section or label
   its gap. If the model or source services fail, preserve the run record and
   evidence; do not publish a plausible-looking unsourced newspaper.

Target the Standard reading length at roughly 10-16 substantive stories across
selected sections, with a concise brief rail. This is an upper editorial target,
not a quota: the number falls when verification or relevance is insufficient.
Default resource limits should be finite and visible (proposed initial ceiling:
30 active minutes, 60 fetched pages and 16 published stories); administrator
caps and the global provider/spending policy remain authoritative. Measure real
runtime cost before settling final defaults. Use one cumulative budget through
research, retries and editing, with a reserved synthesis/checking phase.

## State, scheduling and API

Create `internal/newspaper` with its own SQLite store under the configured data
directory. The initial schema should include `Profile`, `Edition`, `Run`,
`Section`, `Story`, `Source`, `Evidence`, `Correction`, `Delivery` and bounded
`Event` records. Store the profile snapshot and the validated structured issue,
not raw unbounded page copies or a browser DOM. Retain only bounded evidence
excerpts. Give editions immutable revision IDs and content hashes; a delivery
receipt records the exact revision hash it sent. A profile/version change cannot
silently mutate an older edition. Propose a visible, configurable default
retention of 365 daily editions with per-source and total store bounds; explicit
deletion removes related evidence and cached exports together.

The edition family key is `(profile_id, local_date)`; revisions add a third key.
A durable unique run key prevents overlapping scheduled/manual work; explicit
regeneration creates the next revision. The Newspaper worker owns one cancelable
timer, computes its next due instant from the saved IANA zone and recomputes it
after every profile change.
It starts research before the requested ready time by the effective run budget
plus a small publishing margin. The app labels this as a target, shows the real
publication time and sends only after validation; no fixed delivery time is
promised when search or providers are slow.
The database claim is authoritative if timers race. Define a stable daylight-
saving rule: a nonexistent local ready time moves to the next valid local time;
an ambiguous local ready time uses its first occurrence. On startup,
reconcile the current local day once within a bounded catch-up window; never
auto-create a backlog of old papers. A missed run is visible in the app.
Pause/disable and read-only settings stop new research and sends while
preserving archive reads. Closing the app only cancels browser requests.

Proposed admin-only routes under `/api/desktop/newspaper/`:

| Route | Purpose |
| --- | --- |
| `GET capabilities` | Effective research, account, Telegram, read-only and next-run state with sanitized setup hints. |
| `GET/PUT profile` | Read/save a validated profile with revision/ETag conflict protection. |
| `GET editions`, `GET editions/{id}` | Bounded archive and one validated issue/revision; never expose private agent continuation. |
| `POST editions` | Idempotent on-demand run or explicit new revision. |
| `GET editions/{id}/events`, `POST editions/{id}/stop` | Real progress and cancellation; events are bounded. |
| `GET editions/{id}/export?format=pdf` | Version-bound, no-store artifact for the selected revision. |
| `POST editions/{id}/deliver` | Send an already published revision through a configured channel, using an idempotency key and no arbitrary destination. |
| `GET editions/{id}/deliveries` | Sanitized channel status and retry guidance for that edition. |

Use the Desktop admin permission pattern from Detective, same-origin/CSRF checks
for writes, request-size bounds, and both Desktop and Newspaper read-only gates.
Research uses only enabled, authorized read operations; wrapped tools and live
permission changes are checked as Detective does. External pages are untrusted
data and cannot alter tools, recipients, schedule or editorial policy. Validate
all links as HTTP(S), render untrusted strings as text, and keep credentials in
the Vault-backed integrations. Add any new production HTTP client to the network
client inventory.

Daily delivery is an explicit per-channel subscription. The server, rather than
the model, hands one published edition to channel adapters. Each
`(edition_revision, channel, destination)` has a durable receipt with queued,
sending, sent, failed or uncertain status and provider message ID when available.
Retry only failures known to be safe; after an ambiguous SMTP DATA or Telegram
response, mark the result uncertain and require a user decision before another
send. Test sends and manual sends are distinguishable from automatic delivery.
If an issue is published late, send it once when ready and show its actual time;
do not claim it arrived at the requested time.

The `newspaper` feature is disabled by default as a nonessential background
capability. Configuration owns enablement, read-only mode, resource/retention
caps and permitted delivery channels. The Desktop profile owns interests, place,
language, reading length, local delivery time and selected destinations.
Provide a Config UI entry with connection checks and a Dashboard status tile.

## Implementation sequence

1. **Store and policy:** add config/defaults, bounded schema, profile validation,
   permissions and safe APIs. Test persistence, revision conflicts, read-only,
   auth/CSRF and migrations with a backed-up fixture.
2. **Research and edition:** add the dedicated worker, source capture, bounded
   budget, deduplication, claim validation, partial results and immutable issue
   publication. Reuse narrowly extracted Detective helpers and Radio date parsing
   where tests prove the seam. Test malformed pages, unavailable sources,
   conflicting claims, restart and cancellation.
3. **Desktop reader:** register the built-in app, lazy assets, icons, full reader,
   archive and setup. Translate all changed text in 16 locales. Verify real
   window lifecycle and visual states before adding distribution.
4. **Schedule and delivery:** wire the local-time timer and durable date claim,
   restart reconciliation, HTML/plain-text mail, Telegram digest/document,
   immutable PDF and per-channel receipts. Test DST, missed runs, double triggers,
   network uncertainty, revoked permissions and mixed channel outcomes.
5. **Acceptance:** run focused Go and production-module browser tests, canonical
   UI bundle check, version-bound asset packaging and `--check-assets`. Review
   Standard/Fruity light/dark at wide and narrow sizes; inspect mail clients,
   Telegram and printed PDF with controlled destinations. Real search/provider
   and delivery smoke tests are separate from mock tests and must report the
   actual installed configuration and result. Update owning DOX contracts and
   user documentation with the implemented behavior.

## Acceptance criteria

- A new user can select preset sections plus free-text interests, see the next
  local run, create a first issue, and change preferences without sending mail.
- One local calendar day produces one automatic edition; restart, DST, duplicate
  trigger and a manual run do not create duplicate scheduled sends.
- Every published story has a retrieved supporting source and visible source
  metadata. Unverified, stale and contradictory material is labeled or omitted;
  source failure cannot turn a search hit into a cited fact.
- The app, email and Telegram use the same immutable revision. Sending failure
  does not delete the issue; ambiguous delivery cannot silently resend it.
- All setup, loading, partial, empty, error and read-only states are legible and
  actionable. Long and non-Latin text works with keyboard, touch, screen reader,
  narrow viewport and text zoom. The design reads as a newspaper, not generic
  dashboard cards.
- The implementation passes the package, API, browser, locale, bundle and asset
  checks relevant to the touched paths. A deployment or live daily delivery is
  claimed only after a separate observed run.
