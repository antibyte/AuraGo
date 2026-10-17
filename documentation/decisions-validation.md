# Decisions: versions, media and evaluation

## Availability and versions

Settings, the test action, EasyDrag and the task router share eligibility.
`model_availability` in status is `unknown`, `usable`, or `blocked`, with a code,
check timestamp and detached model metadata. Model responses add `usable`,
`unavailable_code`, `checked_at`, `canonical_slug`, `alias_target`,
`expiration_date` and `media_limits`. Missing/non-positive effective context is
`model_context_unverified`; absence from a successfully loaded catalog is
`model_not_available`. Saved unavailable IDs remain visible for repair.

The catalog expires after five minutes. Passive status does not refresh it:
expired/missing metadata is unverified. An explicit evaluation may refresh within
its own request budget before inference. Provider, credential and model changes
revoke their generation and metadata. The active provider shares one catalog
cache with the selector.

Aliases remain supported. **Use fixed version** explicitly changes the draft to
the catalog-backed version; save to apply it. Stored selections are never
silently rewritten. Each request uses its observed catalog version. Settings
show the current alias target and actual responding `model_revision`. Detected
target changes invalidate router caches; initial resolution does not revoke its
own response. Expiry dates are advisory; catalog absence blocks use. Re-evaluate
thresholds with a version change, as recommended by
[OpenRouter model-selection guidance](https://raw.githubusercontent.com/OpenRouterTeam/skills/main/skills/openrouter-decisions/references/models.md).

## Rounding compatibility

Provider distributions require complete keys, finite values in range and
positive mass before normalization. Hundredth-grid values use
`max(0.01, candidates × 0.005)` plus floating-point epsilon. Higher-precision
maps retain 0.01. Rounded intervals, clipped to zero and one, must also admit a
total mass of one. Score consistency follows the same rounding intervals.
Incompatible scores and selected non-maximum options are rejected. This is a
tested AuraGo compatibility rule, not a confirmed general provider guarantee of
two-decimal output. Invalid paid responses retain reported cost and are never
automatically replayed.

## Discovery

Device discovery has a separate **60-second budget for the complete pass**.
Native probes retain at most 1 MiB stdout and 64 KiB stderr while reading.
Overflow stops the probe. Cancellation/timeout terminates the process group
(a Windows job on Windows) and drains readers. `runtime_discovery_timeout`
maps to HTTP 504 and never permits CPU fallback. The existing independent
five-minute model startup budget remains in place.

## Automatic image preparation

Only authorized in-memory shipping copies are prepared by the external adapter.
Original files and local-runtime inputs remain unchanged. Fitting images retain
their bytes. Otherwise AuraGo applies JPEG orientation and reduces dimensions
without changing aspect ratio. Transparency stays PNG; opaque images use JPEG
quality 85 then 70, followed by at most six further 20% reductions. Images are
never cropped, enlarged or removed. Animated PNG, damaged data, excess image
counts and unattainable minimum dimensions fail before provider inference.

These bounded AuraGo profiles stay within documented upstream capacity:

| Model family | Images | Per image | Complete JSON request | Extra bounds |
| --- | ---: | ---: | ---: | --- |
| Clef and unknown models | 4 | 300 KiB | 3 MiB | Conservative context estimate |
| Liquid d1/d1-ultra | 8 | 3 MiB | 4,500,000 bytes | 10,000 total 32×32 patches |
| Jev | 8 | 12 MiB | 16 MiB | 256–1,568 per side; 78,400–2,458,624 pixels; 32 MiB processed total |
| Mercury Decide | 8 | 3 MiB | 10,000,000 bytes | Conservative context estimate |
| GPT-6 Luna Decisions | 8 | 300 KiB | 3 MiB | Conservative context estimate |

Liquid patches are `ceil(width/32) × ceil(height/32)`, costing
`ceil(1.5 × patches)` per image and question. Without a verified token formula,
AuraGo budgets one token per four base64 characters. Text/structure reserves and
the actual serialized request also count. Jev conservatively reserves four bytes
per processed pixel. Global input ceilings still apply: eight images, 12 MiB
each, 24 MiB total and 32 megapixels per image. Processing is sequential with
bounded encoding buffers. See [OpenRouter media limits](https://openrouter.ai/docs/guides/community/multimodal-decisions#limits).

**Reduction can remove decisive details.** API and EasyDrag results add
`image_adjustments`: a zero-based `index`, `original_width`, `original_height`,
`original_bytes`, `original_format`, and sent `width`, `height`, `bytes`, `format`
for every changed image. No paths or contents are exposed. Errors distinguish
`image_invalid`, `image_minimum_unmet` and `image_limit_exceeded`.

## Routing evaluation

`TestDecisionsRoutingEvaluationLive` reuses the unchanged 240-case corpus plus
`task_router_decisions_edge_v1.tsv`: 24 separate German/English injection,
ambiguity, long-input and rare-category cases. It calls the ordinary Decisions
API using production `taskDecisionRequest` and `parseTaskDecision`. It measures
Decisions classification, not the complete heuristic-first router. Thresholds
remain **0.60 probability** and **0.15 margin**.

To opt in on an authorized configured server, set `AURAGO_DECISIONS_EVAL=1`,
`AURAGO_DECISIONS_EVAL_URL` to its origin and `AURAGO_DECISIONS_EVAL_LIMIT` to an
explicit value from 1 to 264, then run:

```text
go test ./internal/agent -run '^TestDecisionsRoutingEvaluationLive$' -count=1 -v
```

If login is enabled, supply the existing signed administrator session cookie via
`AURAGO_DECISIONS_EVAL_COOKIE` (`aurago_session=...`). Keep it out of shell
history, source and reports. Provider credentials stay server-side. HTTPS
verification remains enabled; plain HTTP is accepted only on loopback.
Redirects are refused. The limit picks evenly spaced cases across both corpora.

Calls run sequentially, with a 45-second limit each and no retries. The first
HTTP, transport or interpretation failure stops the run; inspect its unknown
outcome before another run. Ignored `reports/decisions-evaluation/` reports
contain corpus hashes, per-case model/version, misroutes, abstentions, confusion
matrix, latency (nearest-rank P50/P95) and reported/unknown cost. Prompts, server
addresses and credentials are omitted. Live runs may incur charges; ordinary CI
skips them.
