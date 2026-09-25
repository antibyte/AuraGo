# AuraGo Needle3 manual selector

An isolated research/training pipeline selecting zero to three **manual families**.
It does not execute AuraGo tools or change the production agent.

## Current operating limits

- Generation: `deepseek/deepseek-v4.1-flash` on OpenRouter; blind review:
  `google/gemini-3-flash-preview`.
- **USD 15 total**, including the earlier Gemini pilot and all retries. SQLite
  reservations count in-flight and uncertain requests. Keys stay in memory.
- **RunPod is disabled until the user's later start instruction.** No scheduled
  launch exists. A request file is not a rented machine.
- Target: 70,000 fully accepted examples, not requests, drafts or translations
  counted as independent scenarios. A failed pilot stops bulk generation.
- The source pipeline is prepared. A complete accepted corpus, full-run adapter,
  CUDA throughput measurement and checks of the trained export are separate pending
  deliverables. Inspect the local reports before planning a billable run.

## Files and evidence

| Path | Purpose |
| --- | --- |
| `catalog.json` | Full canonical manuals, strict schemas, operations, aliases and source hashes |
| `config.json`, `schedule_manifest.json` | Exact language/split/category requests and budgets |
| `seed_manifest.json` | Provenance of 5,000 existing repository-owned synthetic seeds |
| `uv.lock`, `assets.py` | Dependency, source, tokenizer, model and native-runtime pins |
| `reports/needle3/` (repository root, ignored) | API ledger, pilot drafts/reviews, source audits, outcomes |
| `generated/`, `models/`, `.cache/` (ignored) | Repeated schemas/tokens, weights and local indexes |
| `data/` (created after acceptance) | Compact accepted JSONL shards grouped by language |

Each scenario group has two German phrasings, one English variant and one of the
other fourteen languages. This produces 28,000 DE / 14,000 EN / 14,000 other
training rows and the requested 7,000-row validation and test splits. Group IDs
do not prove semantic distinctness: the semantic audit is a separate gate.

### Local setup (PowerShell, from the repository root)

```powershell
uv sync --project training/needle3 --locked --group retrieval --group cpu-train
go run ./disposable/export_tools --manual-router-out training/needle3/catalog.json
go build -o training/needle3/.cache/catalog-search.exe ./disposable/export_tools
python -m training.needle3.schedule
python -m training.needle3.seeds
training/needle3/.venv/Scripts/python.exe -m training.needle3.assets
training/needle3/.venv/Scripts/python.exe -m unittest training.needle3.test_pipeline
```

On restricted Windows installations use the existing workspace-local Go cache
wrapper or set `GOTMPDIR` and `GOCACHE` under ignored `reports/`. Linux uses
`.venv/bin/python` and a native Linux catalog-search binary. The model download is
public and does not require a Hugging Face token.

## Data workflow

Provide `OPENROUTER_API_KEY` through a temporary process environment or a secure
non-echoing prompt. Never write the key into a script, `.env`, or a command file.

```powershell
python -m training.needle3.generate pilot --workers 3
python -m training.needle3.generate full --pilot-report reports/needle3/pilot-deepseek-deepseek-v4.1-flash/latest-report.json
```

Every request reuses the same global cost ledger, including independent pilot
experiments. Price ceilings include advertised time-of-day overrides. Ambiguous
network failures keep their reserved cost; do not blindly replay them. Resume
uses content-addressed batch results. `full` refuses a failed or mismatched pilot.
The identity binds catalog, seed bank, scenario schedule, configuration and source
code. Rechecking an older pilot is diagnostic and cannot authorize a changed run.
Existing repository seeds supply same-split task references; they do not count as
new accepted scenarios. Generation must create different goals from those seeds.

The generation output is **provisional**, even after blind agreement. It still
needs all language, source, coverage, duplicate and tokenization checks. Source
audit receipts must identify the reviewer, exact row and manual hashes, evidence
and verdict. The semantic audit must resolve duplicates across splits and confirm
at least 15,000 genuinely different scenarios. Inspect the generated queue against
the original manuals; never fill approval fields automatically.
For each positive label, `source_evidence[manual_id]` contains an exact `quote`
from the original manual and an `explanation`, both at least twenty characters.
Empty-label cases need `no_manual_explanation`. Intentional within-group
translations are valid; scenario distinctness is checked between groups.

```powershell
$full = (Get-Content reports/needle3/full/latest-status.json | ConvertFrom-Json).directory
training/needle3/.venv/Scripts/python.exe -m training.needle3.semantic "$full/provisional.jsonl" --out reports/needle3/semantic-pairs.jsonl
training/needle3/.venv/Scripts/python.exe -m training.needle3.quality "$full/provisional.jsonl" --audit-queue reports/needle3/source-audit.jsonl --out reports/needle3/quality.json
# After the source and semantic audit receipts have actually been completed:
training/needle3/.venv/Scripts/python.exe -m training.needle3.quality "$full/provisional.jsonl" --audit reports/needle3/source-audit.jsonl --semantic reports/needle3/semantic-pairs.jsonl.receipt.json --out reports/needle3/quality.json --accept-out training/needle3/data
training/needle3/.venv/Scripts/python.exe -m training.needle3.compile --input "$full/provisional.jsonl" --quality reports/needle3/quality.json --search-binary training/needle3/.cache/catalog-search.exe
```

The compiler uses full-source E5 passages plus the **actual Go catalog search**.
E5 stays frozen. It chooses K=12/16/20 exclusively on validation; if none meets
98% DE/EN and 95% per other-language candidate recall, compilation stops.
Validation and test never receive oracle candidates. Every fully rendered Needle
example is tokenized without truncation before bundling.
The compiler also reserves the same 256 completion tokens as the native runtime;
a training row that cannot run under that budget is rejected. Accepted language
shards are deterministic `.jsonl.gz` files with per-file hashes. GPU readiness
still requires successful compilation and retrieval validation.

## Local native test

```powershell
training/needle3/.venv/Scripts/python.exe -m training.needle3.export --out training/needle3/models/baseline-w4.cact
training/needle3/.venv/Scripts/python.exe -m training.needle3.smoke --weights training/needle3/models/baseline-w4.cact --out reports/needle3/native-smoke.json
training/needle3/.venv/Scripts/python.exe -m training.needle3.runtime --weights training/needle3/models/baseline-w4.cact --search-binary training/needle3/.cache/catalog-search.exe
```

The test program reads one JSON object per line:

```json
{"query":"Nein, schick den Bericht über Discord.","context":["Schicke den Bericht per E-Mail."],"available_manuals":["email","discord","filesystem"]}
```

Omitted availability means the exported research catalog; an explicit empty list
means no available manuals. Output contains ordered `manual_ids`, candidates,
catalog hash, format validity, CPU timing, prompt tokens and a fallback indicator.
There is no calibrated fine-tuned confidence score. This is not yet an integration
recommendation; use the final comparative results and later real-world testing.

### Broader offline development diagnostic

```powershell
training/needle3/.venv/Scripts/python.exe -m training.needle3.diagnostic --out reports/needle3/local-diagnostic --models untrained_w4 --workers 1
training/needle3/.venv/Scripts/python.exe -m unittest training.needle3.test_diagnostic
```

This calls the pinned native `complete()` implementation with the untrained W4
export. Omit `--models` to compare both the published archive and W4. It makes no
API requests and starts no GPU resources. Source/model/catalog identity and cached
case checksums prevent resuming with changed inputs. Use a fresh output directory
after changing code or committing. Results and per-call progress are written under
the ignored report folder. Measure worker counts locally: one worker can outperform
four because concurrent native prefill competes for CPU resources.

The 540 requests represent **242 scenario groups**, including 364 deliberately
mechanical DE/EN catalog probes (all 182 families) and 176 natural challenge rows.
Four shared scenarios are translated into each of the other fourteen languages;
they are a language smoke test, not sufficient samples for per-language acceptance.
Report these cohorts separately. Existing tool-execution clarification/refusal
rows are not imported as empty-manual labels: missing arguments still need manuals.

Real top-12 candidates are never repaired in the main measurement. Missing-gold
counterfactuals, reversed order, and top-20 natural-case probes are separate arms.
Report retrieval misses, conditional selection recall, empty-output accuracy,
format validity, group-aware comparisons, and errors before estimating data needs.
These development fixtures are neither accepted training data nor the sealed
final test. Do not infer a precise training sample count from baseline accuracy;
that requires a validation learning curve after actual training.

### CPU latency and layer depth

The required request budget is **less than 600 ms end to end**, including search,
schema preparation and output validation. Report p50, p95, maximum and deadline
misses. First service startup is separate; cached-prefix timings cannot stand in
for requests whose candidate lists change.

```powershell
training/needle3/.venv/Scripts/python.exe -m training.needle3.export --layers 12 --out training/needle3/models/baseline-w4-12l.cact
training/needle3/.venv/Scripts/python.exe -m training.needle3.export --layers 8 --out training/needle3/models/baseline-w4-8l.cact
training/needle3/.venv/Scripts/python.exe -m training.needle3.export --layers 4 --out training/needle3/models/baseline-w4-4l.cact
$env:OMP_NUM_THREADS='2'
$env:MKL_NUM_THREADS='2'
$env:OPENBLAS_NUM_THREADS='2'
training/needle3/.venv/Scripts/python.exe -m training.needle3.latency --diagnostic reports/needle3/local-diagnostic --out reports/needle3/fresh-latency --weights training/needle3/models/baseline-w4.cact training/needle3/models/baseline-w4-12l.cact training/needle3/models/baseline-w4-8l.cact training/needle3/models/baseline-w4-4l.cact
```

The pinned Python wrapper has no depth constructor argument. The existing export
path uses the upstream ladder to produce the requested 2–20-layer archive; its
header and manifest are checked before timing. No training is needed for this
depth comparison. It pairs 58 natural DE/EN scenarios across every model and
repeats each request without a response cache to isolate prefix reuse. Both modes
rerun the real E5/catalog search and preserve its original candidate set. These
are exploratory quality and timing measurements, not final-test acceptance.

The normal `weights=` binding starts an isolated model worker for each new tool
set. A resident-weight experiment must record any private API hooks, preserve
single-model process isolation, and compare its outputs to that normal binding.
Keep such prototypes separate from the production integration and from the
repeated-request prefix-reuse measurement.

### Small local CPU learning experiment

This separate development experiment tests whether a small amount of actual QAT
LoRA training improves native manual selection. It never starts RunPod or calls
an API, and it does not replace the accepted-corpus or GPU readiness gates.

The fixture contains 116 DE/EN training rows from the earlier natural diagnostics
(18 manual families), 24 new validation rows and 48 new holdout rows. Bilingual
siblings stay together. The holdout has 24 scenario groups and new task goals;
it is a small hand-authored development set, not the sealed 7,000-row final test.
Once reused for training, the old natural diagnostics cannot measure independent
quality of this adapter. This sample cannot determine the full catalog's minimum
training-data requirement.

```powershell
$env:OMP_NUM_THREADS='4'
$env:MKL_NUM_THREADS='4'
$env:OPENBLAS_NUM_THREADS='4'
$env:JAX_PLATFORMS='cpu'
$pilot = 'reports/needle3/local-pilot'
training/needle3/.venv/Scripts/python.exe -m training.needle3.local_pilot prepare --out $pilot
training/needle3/.venv/Scripts/python.exe -m training.needle3.local_pilot train --out $pilot --steps 240 --max-seconds 900
training/needle3/.venv/Scripts/python.exe -m training.needle3.local_pilot_eval select --out $pilot
training/needle3/.venv/Scripts/python.exe -m training.needle3.local_pilot_eval finalize --out $pilot
```

Preparation freezes data/source hashes, real top-12 retrieval, full tokenization
and any training-only gold injections. Rank 8/alpha 16 adapters train four ladder
layers with batch one, learning rate `3e-4`, 5% warmup, cosine decay and norm clipping
at 1. Defaults permit 240 updates within 15 minutes, with an absolute entry-point
limit of 1,000 steps/30 minutes. Training can resume under identical settings;
elapsed budget, optimizer, RNG and shuffled data position are restored together.
Checkpoints are saved at least every 80 updates or four minutes, plus on exit.

Selection evaluates matched untrained W4 and each saved adapter using the actual
native `complete()` implementation. Validation F2, recall, then empty-output
accuracy select the trained checkpoint; ties prefer earlier checkpoints. The
training arm always contains an actual adapter, even when every checkpoint loses
to the untrained baseline, so the comparison cannot conceal regressions. Search-only output
count and similarity threshold are also selected on validation. `finalize`
exclusively claims the development holdout for that frozen selection. A crash
requires reconciling saved evidence; it must not silently retune on the holdout.

The comparison records per-language quality, group-aware F2 intervals and complete
request timings with live retrieval. It also compares a single-model resident
prototype against the normal Python binding, verifying identical guarded outputs.
That private loader hook and any observed sub-600-ms latency are development
evidence, not production integration or a real-time guarantee. Artifacts, adapter,
checkpoints, exports and reports remain in the ignored experiment directory.

## Later RunPod run

The prepared image is official CUDA 12.8.1 / Ubuntu 24.04 pinned by digest.
`bootstrap_runpod.sh` installs Python 3.12 and the locked JAX CUDA-12 environment.
Runtime compatibility and GPU throughput are checked on the actual reserved GPU.

1. After the later start instruction, require a complete `generated/compiled`
   manifest. Re-read availability and price: Secure Cloud on-demand RTX 4090,
   24 GB VRAM, at least 6 vCPU and 32 GB RAM, no more than USD 1/GPU-hour.
2. Create a separate 30-GB network volume in a compatible data center; save its
   returned ID. Prepare/upload the allowlisted archive with `bundle.py` and verify
   its hashes. Do not upload local config, the Vault, API ledger or raw logs.
3. Enable `launch_enabled` only under that later authorization and recompile the
   manifest after the config change. `runpod.py request` validates a fresh quote
   and emits the MCP `create_pod` body. It does not make the billable call.
4. Record the actual create response: `pod_id`, `created_pod_id`, `created_at_unix`
   (server creation time), `run_id`, `network_volume_id`, `gpu_hourly_usd`, and
   `maximum_seconds: 43200` in `lease.json`. Verify ownership against a fresh GET.
5. Start `python -m training.needle3.runpod watchdog --lease lease.json` as a
   detached process independent of training with a securely supplied
   `RUNPOD_API_KEY`. It needs a live control API and a shared heartbeat file. A
   separate always-on host is preferable when a shared volume is available;
   otherwise a separate pod process remains independent of trainer failures.
   Keep the machine awake. Do not pass the RunPod key to the trainer.
6. Start bootstrap from the extracted repository-shaped folder at
   `/workspace/aurago-needle3`. All outputs go to the mounted network volume.
7. The controller first checks the real Linux native runtime, benchmarks
   microbatches, then runs equal-step DE/EN and multilingual
   pilots with identical shuffled scenario batches and sizes the main run from
   measured throughput. All training ends by
   creation hour nine. Export, validation selection and the sealed test follow.
8. The guard begins termination at 11h57m and confirms absence. **A failed API
   request is not a stopped pod.** Retain the network volume, download results,
   run `bundle.py --verify-download ... --manifest ...`, then explicitly handle
   volume cleanup. The scripts never delete a network volume.

Every phase is interruptible by the lease controller. Checkpoints include LoRA,
optimizer, RNG and data position; microbatch reduction does not reset training.
If setup, compilation, evaluation or transfer misses its deadline, preserve partial
results and report incompleteness. Never turn an incomplete final test into a pass.

## Evaluation and integration contract

- Compare retrieval alone, a **matched untrained W4** export, and tuned W4 with the
  same native runtime and candidate budget. Both exports drop the confidence head.
- The simple reference chooses output count and embedding abstention threshold
  on validation. A scenario-group bootstrap reports paired F2 confidence intervals;
  translations are not treated as independent observations.
- Choose checkpoints on a fixed validation probe then full validation, prioritize
  recall and F2, and claim the frozen final test only once for all comparison arms.
- Report per-language/per-family performance, challenge cases, retrieval misses,
  conditional selector recall, unnecessary manuals, empty accuracy, invalid output,
  prompt tokens, CPU timing and observed Linux worker memory. Pilot DE/EN
  regressions remain explicit. Runtime timing includes candidate schema setup;
  the JSONL tester also reports retrieval and end-to-end timing.
- Integration targets: recall@3 >=95% DE/EN and >=90% per other language, >=95%
  correct empty output, >=99.5% formal validity, no unknown/disallowed output after
  guarding, and improvement over the strongest simple reference at budget <=3.
- A future AuraGo caller must bind the exact catalog version, filter to its actual
  allowed manuals, preserve empty availability, and recheck permissions before
  using results. On mismatch/error/oversize input, fall back to existing catalog
  discovery. `discover_tools`/`get_manual` must remain available to recover misses.
- Synthetic acceptance and Linux/Windows native smoke results are separate from
  production qualification. No model should be activated by this pipeline.

Primary interfaces: [Needle source](https://github.com/cactus-compute/needle),
[fine-tuning documentation](https://www.cactuscompute.com/blog/finetuning-needle),
[OpenRouter price routing](https://openrouter.ai/docs/guides/routing/provider-selection),
[RunPod pod deletion API](https://docs.runpod.io/api-reference/pods/DELETE/pods/podId).
