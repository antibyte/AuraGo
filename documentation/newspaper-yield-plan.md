# Newspaper: coverage-driven research

Status: implemented locally, 2026-10-06. This document records the coverage and
budget contract. Operator guidance is in `newspaper.md`; the original product
design remains in `newspaper-plan.md`. Deployment and live yield acceptance are
separate from local fixture validation.

## Intended result

Increase supported, relevant stories and selected-topic coverage. A large list
of search hits is not success. First obtain a small current-news overview, then
search for the specific events and original sources worth reading. Scale the
available work with the profile's sections and individual interests, and retain
capacity for the second research round. Never fill the issue with unsupported
material to meet a numerical target.

## Source overview before targeted research

Reuse the existing guarded RSS/Atom fetcher, Brave adapter, planner and candidate
selector. No new MCP server, executable skill, service or dependency is needed.
The Newspaper server must explicitly wire these sources into its capability
snapshot; being reachable by HTTP does not make them an existing agent tool.

| Source | Use in the planned pipeline | Important boundary |
| --- | --- | --- |
| Google News RSS | Broad country/language overview and topic-specific headline discovery, including local place queries. | Feed item links point to Google wrappers. Retain headline, publisher and feed date as leads; use headline/entities plus publisher in targeted Brave/DDG queries to locate originals. Do not decode opaque Google tokens or scrape internal RPCs. The public RSS surface has no API stability guarantee. |
| Brave News, already integrated | Broad fallback overview when feeds fail, then precise event, place and publisher queries. | Reuse the configured Vault-backed key and existing rate limits. A result's page-age hint can describe an update, not original publication. |
| Hacker News RSS | Optional software, computing and technical-interest overview. | Items usually link directly to the original. Submission date and popularity are discovery hints, not proof of a new or significant event. Prefer its one-request RSS feed over fetching many individual API items. |
| Techmeme RSS | Optional technology-industry overview. | Item links point to Techmeme; its description contains publisher links. Extract a narrowly recognized primary link when available, otherwise use targeted title/publisher search. Never treat the aggregator summary as article evidence. |
| Publisher RSS/Atom | Existing user feeds and optional suggested newsroom/specialist feeds supply direct originals, especially regional coverage. | A single publisher is not an aggregator or sufficient diversity. Preserve international open source selection. Do not silently add feeds to the saved profile. |
| GDELT DOC API | Defer from the initial implementation; possible later international fallback. | Public documentation supports multilingual article discovery, but the bounded access probe returned 429. It must not become a required or blocking dependency. |

References: [Brave News API](https://api-dashboard.search.brave.com/api-reference/news/news_search/get),
[Google News RSS](https://news.google.com/rss?hl=de&gl=DE&ceid=DE:de),
[Hacker News RSS](https://news.ycombinator.com/rss) and [official API](https://github.com/HackerNews/API),
[Techmeme overview](https://www.techmeme.com/about) and [RSS](https://www.techmeme.com/feed.xml),
[GDELT DOC API](https://blog.gdeltproject.org/gdelt-doc-2-0-api-debuts/).

Provide a source-selection control for built-in overview feeds, alongside the
existing user feed configuration. Offer Google News for general discovery and
HN/Techmeme only for relevant topics. Operators can disable each source. Existing
installations retain their source choices until opting into the overview preset;
new setup offers the preset explicitly. Enabled Newspaper, network and read-only
policies still apply to every read. Do not require every source to be healthy.

The overview stage does not call an additional LLM. It collects bounded metadata
and supplies it to the existing first planning call. Retain the existing maximum
of two planning calls, each bounded to 45 seconds. Fetch each shared feed once
per run, reuse it across sections, and make at most two feed requests concurrently.
Keep the existing 1 MiB feed body and 40 admitted entries per feed bounds.
Overview work gets at most 30 seconds before continuing with available results.
One localized Google front-page request may serve several topics; each distinct
topic-specific Google query is another overview attempt counted in `O`. Normalize
and deduplicate identical feed URLs/queries within the run. Limit first-pass
requests to one overview per topic plus shared relevant specialist feeds; use the
remaining allowance only to fill gaps. When no article budget remains, do not
start additional overview work either.

Deduplicate canonical URLs and normalized titles before planning. Supply at most
three leads per topic and 96 in total, further trimmed evenly to the chosen
model's context limit. Include server-owned IDs, headline, publisher, date hint
and short description. Validate selected IDs and topic associations. All source
content remains isolated external data. No source text may add permissions or
change the trusted instructions.

The planner chooses concrete events/entities for each topic, and emits precise
queries rather than only broad section names. First try a direct original link;
otherwise search headline/event plus publisher, then widen publisher/language
when necessary. Preserve a deterministic direct-search fallback for empty feeds,
unsupported locales, provider failures and invalid planning JSON. Aggregator
URLs do not enter the article-reader queue as if they were original stories.

## Budget proportional to selected topics

`T` is the count of distinct selected sections plus distinct individual interests
after existing profile validation, currently 1-31. Do not count an additional
generic `interests` section. Do not guess that a specific interest is equivalent
to a broad section; each requested topic gets its own coverage opportunity.

Add `newspaper.budget_mode` with `auto` and `fixed`. Missing mode in existing
configuration means `fixed`: preserve its saved absolute `max_pages`,
`max_searches` and `max_minutes` behavior, including legacy defaults/clamps.
Fixed mode keeps the existing 60-page/64-search maxima; calculated auto budgets
must bypass those legacy clamps through the shared resolver, not globally change
their meaning. New setup/template selects `auto` and
shows the resulting effective limits. Changing an existing install to `auto` is
an explicit configuration choice. Fixed mode remains available for intentionally
tight resource limits. Do not multiply old absolute limits by the topic count.

In auto mode resolve one immutable effective budget at run start:

| Resource | Initial formula | Hard maximum |
| --- | --- | ---: |
| Original-page attempts `P` | `24 + 12*T` | 400 |
| Search attempts `S` | `8 + 4*T`, including fallbacks/retries | 128 |
| Overview/feed attempts `O` | `2 + 2*T`, including user feeds and retries | 64 |
| Run minutes `M` | `max(20, 10 + 2*T)` | 60 |
| Pending candidates | `max(80, 32*T)` | 800 |
| Maximum accepted stories `A` | `max(lengthCeiling, T)` | 31 |
| Editor calls, including repairs | `4*A` | 124 |

Each formula is clamped to its hard maximum. These are ceilings, not a work quota.
Stop when coverage and issue capacity are satisfied or no useful work remains.
Preserve four concurrent original reads, one per publisher domain, and two
concurrent editors. More topics increase total work, not outbound concurrency.

| Topics | Original-page attempts | Search attempts | Overview attempts | Run limit |
| ---: | ---: | ---: | ---: | ---: |
| 2 | 48 | 16 | 6 | 20 min |
| 4 | 72 | 24 | 10 | 20 min |
| 8 | 120 | 40 | 18 | 26 min |
| 12 | 168 | 56 | 26 | 34 min |
| 20 | 264 | 88 | 42 | 50 min |
| 31 | 396 | 128 | 64 | 60 min |

The existing monetary budget, provider quotas and remaining context/output
limits stay authoritative. Auto mode does not raise a monetary spending limit.
If the operator has disabled monetary budgeting, auto mode remains bounded by
request counts, model context limits, at most 8,192 requested output tokens per
completion and the deadline; it does not promise a dollar ceiling. Show that
distinction and the effective request allowances before opting into auto mode.
Every completed model response, including repair attempts and invalid output,
is charged once. Enforce the run's editor-call ceiling before admission and
count repairs against it. Preserve a small repair allowance within that ceiling,
at most one repair per article and at most `T` repairs per run. Never retry an
intentional editorial decline. No retry after cancellation or budget exhaustion.

Live configuration may shrink remaining work or revoke access. A larger profile
or higher cap takes effect only on the next run; switching budget modes cannot
expand an active run. The scheduler, capability preview, runner and UI must use
the same profile-aware budget resolver, including the 60-minute maximum. A fixed
mode run keeps its legacy limits and story ceilings, and visibly explains when
those ceilings prevent full topic coverage.

## Preserve capacity and improve acceptance

1. Separate overview/feed attempts from original-page attempts. A failed original
   fetch still consumes a page attempt; free retries would make the budget false.
   A fetched overview misclassified as an article still costs that attempt, but
   known overview sources are handled in the dedicated stage.
   This separation applies to auto mode. In fixed mode, overview/feed attempts
   have a sublimit `min(max_pages, 2 + 2*T)` and also consume the existing shared
   `max_pages` allowance, preserving the old absolute I/O bound. Show this in the
   effective preview; fixed mode does not gain unbudgeted network work.
2. Reserve `ceil(0.4*P)` page attempts and `ceil(0.4*S)` search attempts for the
   follow-up. Enforce admission before launching concurrent requests so in-flight
   work cannot spend the reserve. Round one uses the remainder; round two can use
   all unspent capacity. Widen uncovered topics from 24 hours to at most seven
   days, with changed terms/publishers/languages. Do not search when no original
   can still be read. Already captured evidence may still be edited.
   Reserve wall time as well: the first round yields by 50% of the run deadline,
   follow-up discovery/reading ends at 80%, and the final 20% remains for editing
   and validation. Overview/planning time is included. The coordinator must cross
   the round boundary without waiting indefinitely for one editor: carry bounded
   captured evidence/in-flight work forward while retaining the same global
   four-read/two-editor concurrency limits. Phase cancellation must not become
   explicit user cancellation or discard already accepted stories.
3. Reuse round-robin topic admission/selection. Prioritize uncovered topics before
   extra stories for covered ones. Reassign unused capacity only when a topic has
   no eligible candidates or has received its fair opportunity. A prolific topic
   must not fill the queue or consume all editors first.
4. Preserve already fetched 24-hour-to-seven-day articles as bounded in-memory
   excerpts with provenance and metadata. In the wider round, edit that evidence
   without a second fetch. Cache/deduplicate by canonical original URL, retaining
   valid topic associations; a shared link need not be fetched again for another
   topic. Keep unknown publication dates unknown and prevent unchanged recent
   stories from appearing as fresh news.
5. Ask for 1-4 paragraphs according to available evidence, with 20-500-character
   exact source quotes and decoded source text. Identify intentional declines
   explicitly. Permit one bounded regeneration for repairable JSON, structure or
   quote-format errors using the same source and precise validation reason.
   Revalidate with the same evidence/security checks; never relax them or copy
   complete articles. Do not modify the global source-isolation policy.
6. Split stable diagnostic codes: editorial decline, JSON empty/invalid/truncated,
   invalid structure, quote missing/mismatch, stale, unknown date, unreadable,
   blocked/fetch failure, duplicate, budget exhaustion. Store bounded per-topic
   and per-round search/read/accepted counts and the resolved budget in optional
   run JSON. Do not persist raw model output, recipients or free-form errors.

In auto mode the current length ceilings 6/12/16 become minimum capacity settings:
at least one story slot is available per requested topic, up to 31. Coverage is
an opportunity, not a promise or permission to invent news. Short length keeps
articles concise when many topics are selected. Explain this in Preferences
before saving. Preserve saved editions unchanged. Do not claim that one story
automatically covers multiple topics just because the query overlapped.

## Integration and sequence

1. **Yield and diagnostics first:** update `newspaper_pipeline.go`,
   `newspaper_research.go`, `newspaper_search.go`, `newspaper_model.go` and their
   fixtures. Add reserve enforcement, reuse captured originals, coverage-first
   editing and bounded rejection reasons before increasing limits.
2. **One budget resolver:** place the shared value calculation in
   `internal/newspaper`, with configuration input supplied by the server. Update
   configuration loading/validation/template/save, the server policy adapter,
   scheduler lead time, capability preview, run snapshots and narrowing behavior
   together. Audit every fixed 60/64/200/16/24/80 limit before choosing which are
   legacy fixed limits versus storage/security bounds.
   In particular, `ValidateDraft` independently caps stories at 16 today. Raise
   its supported edition bound to 31 while keeping fixed-mode run ceilings and
   old edition/hash compatibility. Its 60-source edition bound already covers
   the planned one-source-per-story output and need not be raised speculatively.
3. **Overview then targeted queries:** extend `newspaper_feeds.go` metadata and
   the existing planning payload, source capabilities and editorial guide. Use
   the strict public HTTP helper and static scraper. Preserve redirect checks,
   bounded bodies, cancellation and all network/client-inventory contracts.
4. **UI and persistence bounds:** show `auto/fixed`, topic count and effective
   budgets in Config/Preferences, plus coverage and separate overview/search/read
   counters. Translate changed strings in all 16 locales. Align source evidence
   retention, run progress bounds, edition validators, PDF/email/Telegram and
   reader behavior with up to 31 stories. Keep accepted evidence fully retained;
   bound auxiliary evidence by the run budget and per-source byte ceiling, with
   an explicit truncation counter. Optional JSON fields need no new table.
5. **Validation, documentation, controlled activation:** update owning contracts,
   `newspaper.md`, this plan's status and tests. Commit locally; deployment and a
   controlled live acceptance run are separate. Existing legacy installations
   do not switch to auto or enable overview providers merely by reading this plan.

## Acceptance gates

- Budget calculations for 1, 2, 4, 8, 12, 20 and 31 topics are deterministic,
  monotonic up to caps and identical in scheduler/UI/runner. Existing fixed
  configurations preserve their values. Revocation, cap reduction, profile edits,
  cancellation and concurrent reservations cannot expand or overspend a run.
- With eight topics and a deliberately mixed fixture of stale/overview/unreadable
  pages, prove the second round receives its reserved capacity, accepts additional
  valid stories and improves coverage versus the old pipeline on identical data.
  This is a constructed regression fixture, not a replay of unavailable live logs.
- A standard auto-mode fixture with enough distinct verifiable originals produces
  12 stories covering all eight topics. A 20-topic fixture can produce at least
  one valid story per topic; a 31-topic case must actually validate, publish,
  reload and render 31 sourced stories within all bounds.
  A no-news fixture remains explicitly partial and does not retry declines.
- One fetched 48-hour-old original is reused across rounds and fetched exactly
  once. Duplicate aggregator headlines/URLs do not become duplicate stories.
  Google wrappers, HN discussion/item pages and Techmeme summary pages never pass
  as original-article evidence; HN submission dates are not
  mistaken for publication dates; disabled/429/invalid feeds fall back promptly.
  Delayed first-round readers/editors must leave actual follow-up time, not just
  unused counters; the transition must not double worker concurrency. Fixed-mode
  feed/overview requests remain within the saved shared page limit.
- Test supported encoded quotes, genuinely mismatched quotes, invalid/truncated
  JSON, repair exhaustion and a remaining-cost limit. Repaired output must pass
  the same validator and retain exact accounting and cancellation behavior.
- Run focused Newspaper/server/tools/config checks, browser checks, translation
  and bundle checks, and export/delivery rendering checks for larger editions.
- Before claiming live improvement, record at least three comparable controlled
  runs with model, topic count, effective budgets, duration, monetary/token cost,
  distinct publishers, coverage and rejection reasons. For the eight-topic
  standard profile, target median >=10 accepted stories and >=7 covered topics,
  with 12/8 as the full target where enough current originals exist. These are
  evaluation thresholds, not forced quotas. If they fail, inspect precise losses
  before raising ceilings again. Disabling delivery during comparison avoids
  distributing experimental editions.
