# Newspaper

Newspaper is an optional Virtual Desktop app that makes a personal daily edition from public web sources. It opens as an editorial reading surface with a front page, full articles, source notes, an archive and preferences. The edition is saved as an immutable revision, so later profile edits do not silently change what you read or send.

## Set up

1. Enable Virtual Desktop and Newspaper in Config. Newspaper starts disabled. The Config page also controls read-only mode, automatic or fixed research budgets, optional Google News/Hacker News/Techmeme overview sources, archive retention (365 editions), and whether email or Telegram delivery is allowed.
2. Configure a selectable LLM, web scraping and network requests. Brave Search with a configured key provides News and Web search; permitted DuckDuckGo search and optional public RSS/Atom feeds provide alternatives. DuckDuckGo uses the existing network permission and has no separate toggle. The Config readiness check and the app's **Research tools** disclosure distinguish disabled tools, missing setup and errors from the last attempt. Research requires the model, original-page reader, network permission and verified bundled guide; configured capabilities do not guarantee a remote service is healthy.
3. Open **Newspaper** on the Virtual Desktop. Choose sections and optional free-text interests or exclusions. Add up to ten public RSS feed URLs for selected sections if desired. Set the country code, regional place if needed, publication language, IANA time zone, ready time and reading length. Save the profile, then choose **Create today's edition**. The default selection covers national and international news, politics, culture, technology and science.
4. For automatic editions, enable the daily switch in Preferences. The displayed ready time is a target. Research starts before it; the app shows the actual publication time. A missed or failed run stays visible. Closing the window does not stop server research.

The first page shows the lead and secondary stories, a brief rail and a link to each full article. When today's issue is missing, the last saved edition remains readable with a clear action to create today's issue. Article paragraphs carry source references; the source panel shows the original URL, publisher, publication time when known and retrieval time. Single-source stories and partial coverage are labeled. The archive keeps dated revisions and reading position. PDF export is available for a saved edition when its characters are supported by the local PDF font.

If research fails or is interrupted before an issue is published, use **Create today's edition** again to retry. Each attempt gets a new revision number, and the earlier run remains available for diagnosis. The daily schedule makes one attempt per local date and does not repeatedly retry a failure. Once an issue is published, use **Create new revision** for a fresh version of today's edition. If the new version corrects a published error, enter a short correction note in the revision panel. The note appears on the new issue in the app, email and PDF; the earlier issue stays in the archive.

## Automatic and fixed budgets

New configuration templates select `budget_mode: auto`. Existing installations
with a missing mode remain `fixed`, retaining their saved limits (defaults:
60 shared pages, 32 searches and 30 minutes). Switching an existing installation
to automatic mode is an explicit Config action. Preferences previews the budget
for the current selected sections and individual interests before saving.

For `T` distinct selected topics, automatic mode allows up to `24 + 12*T`
original-page attempts (maximum 400), `8 + 4*T` search attempts (128),
`2 + 2*T` overview attempts (64), and `max(20, 10 + 2*T)` minutes (60).
Eight topics therefore receive 120 original reads, 40 searches, 18 overviews
and 26 minutes. Editor calls, including repairs, are capped at four times the
story limit. Concurrency stays at four original reads (one per publisher), two
editors and two feed requests. The scheduler uses the same resolved time budget.
Existing monetary limits and provider quotas remain authoritative. If monetary
budgeting is disabled, request, context, output-token and time ceilings still
apply, but there is no promised dollar ceiling.

Overview providers are individually selectable. Existing installations do not
enable them automatically; Config offers an explicit preset. Google News
supplies localized headlines for targeted publisher searches. Hacker News and
Techmeme help with relevant technical topics. The overview stage takes at most
30 seconds and feeds bounded, untrusted metadata to the existing planning call.
It fetches shared feeds once per run. Google wrappers, Techmeme summaries and HN
discussions are never original-article evidence. HN submission times remain
hints, not publication dates. Feeds that fail or return 429 do not prevent
permitted search alternatives. Optional user feeds remain available.

## Delivery

Delivery is off until you enable a channel in Config and opt into it in Newspaper Preferences. Manual sends use the saved revision; they do not run a new search.

- **Email:** choose a writable configured SMTP account or AgentMail and enter the recipient address. The **Send code** action saves the selected sender and address before requesting the confirmation email; other unsaved preferences stay as drafts. Enter the code in the app. Daily email can be enabled only after verification. Changing the account or address clears verification. Messages include responsive HTML and complete plain text with sources. AgentMail sending uses one request without automatic POST retries.
- If AgentMail rejects a confirmation email, Newspaper shows its HTTP status without exposing provider details. A recipient blocked after a bounce gets a specific message that does not assume an invalid address: ask AgentMail support to review the original SMTP rejection and the suppression before sending again. A sender policy rejection can produce a bounce even when the mailbox is valid. Other known rejections can be retried after correcting the sender setup or limits. If the sending result is unclear, check the mailbox first; the code remains valid for ten minutes and another request is available after about one minute.
- **Telegram:** configure the bot and authorized Telegram user in AuraGo, then use the test action and enable daily Telegram if wanted. A send delivers a short lead message and the full edition as a PDF. When the PDF font cannot represent the language, it sends the complete issue as bounded text messages.

Each send has a durable receipt bound to its edition hash and destination. A send interrupted after reaching a provider can be marked **outcome unknown**. Inspect the mailbox or chat before choosing to resend; a server restart never automatically replays an open send. The app archive remains available if delivery fails.

## Research and privacy

The bundled guide tells the model where and how to look: local newsrooms and authorities for regional events, newsrooms for daily affairs, specialist media and original announcements for technology, and research institutions and papers for science. Source choice remains open. Plans use places, events, technical terms, synonyms, suitable languages and targeted `site:` searches. International and specialist coverage can use English or other original languages while the edition follows the selected output language. The editor receives the selected topic or free-text interest and rejects unrelated stories; literal keyword matching does not discard translated original articles. Delivery addresses and account IDs are excluded from planning prompts.

The server validates a structured search plan and falls back to deterministic language/place templates on model failure or invalid output. It first searches the past 24 hours, with up to 20 results per search and 40 recent entries per feed. The bounded queue holds 200 candidates in fixed mode and up to 800 in automatic mode and alternates topics while considering freshness, relevance and publisher diversity. Original pages are read with at most four concurrent requests and one per publisher domain. Overview pages may contribute one further layer of links; known feeds and aggregators have their own overview allowance in automatic mode. Fixed mode also charges them against its existing page allowance. Failed original reads always consume a page attempt. RSS redirects retain strict public-IP checks. Static page reads retain the existing guarded scraper.

The first round can spend at most 60% of page/search attempts and yields by half the run deadline. A single follow-up uses the reserved capacity to address coverage gaps or insufficient yield, changing terms/languages and widening to at most seven days. At most two planning calls are allowed, each bounded to 45 seconds and the remaining research deadline. Search retries are limited to one per transient failure and consume the same search budget; provider rate-limit headers pace requests. A long Brave cooldown allows independent alternatives to proceed. New discovery and page reads stop at 80% of the run deadline, reserving the remainder for editing/checking. All completed model responses, including unusable JSON, are charged to the existing `newspaper` budget category. Live permissions and limits may narrow the initial snapshot, never expand it.

At most two editors process fetched originals concurrently, prioritizing uncovered topics. Already fetched 24-hour-to-seven-day originals retain their bounded evidence for follow-up without a second fetch. Articles use one to four paragraphs as the evidence permits. Repairable JSON, structure or exact-quote errors receive at most one regeneration per article and at most one per selected topic per run, within the editor-call budget. Editorial declines are not retried. Relevant verbatim passages from throughout the article are retained within the evidence bound; the server rejects invented source references, missing supporting passages and invalid structure. A single-source report stays labeled. Exact URL/title/content copies are suppressed, including recent source URLs. Publication dates come from original metadata (including structured article data); search `page_age` is only a selection hint and unknown original dates stay unknown. Untrusted source data cannot choose tools, recipients or delivery settings. Disabling Newspaper or enabling read-only before publication prevents a new edition while preserving run diagnostics and evidence.

Fixed reading sizes remain at most **6 / 12 / 16 stories** for Brief / Standard / In depth. Automatic mode makes at least one story slot available per selected section or individual interest, up to 31 stories; short reading length still asks for concise articles. These are ceilings, not quotas. The app displays found candidates, successfully read pages, accepted articles and remaining topic gaps. Effective budgets, overview/search/read/editor/repair counts, per-round and per-topic progress, precise rejection reasons and any source-retention truncation use the existing optional run JSON; no database migration is needed. Up to 400 bounded captured sources are retained per run; saved edition evidence is unchanged.

If a run finds pages but publishes no issue, its reason separates model request errors, truncated or invalid JSON, and rejected evidence quotes or story structures. A retrieved page is not counted as a published source until its story passes validation. Newspaper uses the selected model's configured output and context limits for story editing and requests JSON mode only when its provider capabilities support it.

Multi-source story merging, fuzzy syndication matching and claim-level semantic verification remain future editorial improvements. PDF generation is local and deliberately rejects unsupported glyphs; the Telegram text fallback retains the complete issue. Email/PDF utility labels are localized for German and English; other publication languages currently use English utility labels while article prose follows the selected language. Visual UI strings are provided in all 16 desktop locales.

## Verification status

Local fixtures produce 12 evidenced stories across eight selected topics and cover 20- and 31-topic editions. A 31-story issue is validated, published, reloaded and rendered as HTML, text and PDF. A deliberately unusable first-round fixture verifies that reserved follow-up capacity produces additional supported stories; other checks cover overview metadata, planning fallback, exclusions, original dates, encoded quotes, bounded repairs, rate limits, permission revocation, concurrency, cancellation, shared budgets and legacy configuration upgrades. Browser checks cover wide/narrow windows, both Desktop themes, the 31-story reader, unsaved budget previews, overview presets and research counters. New keys and placeholders match across all 16 locales, and generated bundle checks pass. Fixtures do not prove live search quality, configured-provider availability, native-speaker acceptance or delivery. A separate live acceptance run must record source/publisher diversity, selected-topic coverage, elapsed time and model spend. Publication still requires verified evidence; a numerical target never justifies filler.

Provider references: [Brave News API](https://api-dashboard.search.brave.com/api-reference/news/news_search/get), [Brave rate limiting](https://api-dashboard.search.brave.com/documentation/guides/rate-limiting).
