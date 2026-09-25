# Needle3 category routing and manual selector research

## Purpose

The active experiment routes requests to all needed tool categories. Preserve the
earlier manual-selector pipeline for reproducibility. Production AuraGo integration
belongs to a later task; no application prompt or dispatch behavior changes here.

## Ownership

- This directory owns synthesis, release gates, retrieval, serialization, QAT,
  checkpoints, export, local inference, and bounded RunPod run preparation.
- `disposable/export_tools/manual_router.go` exports the strict catalog with the
  canonical `prompts.ToolManualID` bindings and serves the actual catalog search.
- Existing `training/` datasets and validators remain independently authoritative.

## Local Contracts

- `config.json` retains the original manual-selector quota and budget contract: 70,000 accepted rows,
  17,500 semantic groups, 16 languages. Generated requests and seed imports are
  never accepted examples. Keep every translation/paraphrase in its group split.
- User steering: use `deepseek/deepseek-v4.1-flash` for generation, an independent
  Gemini Flash reviewer, and a hard USD 15 total API ceiling including prior
  experiments, retries and unresolved reservations. Do not reset the ledger.
- RunPod remains disabled until the user gives a later start instruction after
  resting. Do not create resources, schedule a launch, or train remotely now.
- Store keys only in process memory/environment or an approved credential vault.
  Do not embed keys in source, command files, archives, manifests, or Git.
- Full source manuals and curated operation contracts ground the corpus. Five
  documented tools without a manual remain explicit absences, not invented labels.
- Candidate recall is selected on validation only. Injecting missing gold labels
  is allowed only in the declared training half; preserve real retrieval failures.
- Blind reviewers must accept intentional translations within a scenario group
  and legitimate empty-label requests. Scenario uniqueness is between groups.
  Source audits require exact manual quotes; a repaired review cannot authorize
  generation with a different schedule or code revision.
- Manual-selector training and inference share `serialization.py`. No silent truncation; no calls
  with arguments; only known, allowed candidates can survive the output guard.
  Inference calls `complete()`, never `run()` or production tools.
- Reserve the runtime's 256 completion tokens at compilation. Pair pilot buckets
  as well as scenario order so shuffled training batches remain comparable.
- Pin dependencies with `uv.lock` and model/runtime revisions with `assets.py`.
  The available runtime wheel is 3.0.1; do not use the upstream automatic 3.0.2
  download until its artifact exists and has passed compatibility checks.
- GPU work requires all data gates, a verified compiled manifest, an active
  independent shutdown guard, and a later user start instruction. Train ends by
  hour nine from pod creation. Begin pod termination before the twelve-hour cap.
  A network/API outage is not proof of termination: confirm pod absence.
- Preserve the separate network volume until a local checksum-verified download.
  Deletion of that volume is an explicit later lifecycle action.
- Checkpoint identity binds base model, compiled data, optimization schedule,
  RNG and data position. Validation selects the model; the sealed test is claimed
  once for all comparison arms. Failed synthetic targets cannot imply readiness.
- Accepted compact language shards may be committed after gates. Raw generations,
  pilot results, audit queues, diagnostics and cost ledgers belong in ignored
  `reports/needle3/`; models, compiled sequences and environments stay ignored.
- `diagnostic.py` and its synthetic source fixtures provide offline development
  measurements before choosing training scale. Keep mechanical catalog probes,
  natural cases and translated siblings distinct. Counterfactual oracle and
  candidate-order probes must never enter the main retrieval measurement.
- A local baseline does not consume the sealed final test or authorize GPU work.
  Baseline errors inform targeted data and later validation learning curves;
  they cannot establish an exact required number of training examples.
- User latency requirement: less than 600 ms per selection, including retrieval,
  schema setup and output validation. Report median, p95, maximum and violations;
  a passing median alone does not meet this requirement.
- `latency.py` compares matched untrained W4 ladder exports on identical natural
  development cases. Use the upstream ladder selection through `export --layers`,
  never remove arbitrary blocks. Keep changing-candidate and repeated-request
  timings separate; prefix reuse is not general latency acceptance. Report startup
  separately. Depth selection remains subject to quality validation before training.
- `local_pilot.py` is a separate, offline CPU experiment authorized by the user.
  Its small synthetic DE/EN dataset is not accepted corpus data and does not waive
  GPU gates. Cap each run at 1,000 steps and 30 minutes; default to 240/15 minutes.
  Assign all reused natural diagnostic variants to training, freeze new validation
  and holdout goals before training, and never repair their retrieved candidates.
- The local pilot uses four ladder layers, rank 8/alpha 16 QAT LoRA, and the shared
  serializer/checkpoint/export implementations. Optimizer, RNG, data order, elapsed
  budget and model/data identity survive resume. Stop on nonfinite updates.
- `local_pilot_eval.py` selects native W4 checkpoints and a simple search reference
  only on validation, then claims its development holdout once. Group bootstrap
  keeps translations dependent. Always compare a trained checkpoint against the
  untrained baseline, even when training loses; do not hide a regression by
  selecting the baseline as both arms. Private resident-loader experiments require a
  fresh process per model and output equivalence with the normal binding; they
  remain prototypes. Keep all model artifacts and measurements under reports.
- User scope change: `category_*.py` predicts only the nine exported discovery
  categories. A shared manual maps to exactly one category; all 182 families must
  be covered. Do not silently introduce a different production taxonomy.
- `category_router.py` owns nine fixed zero-argument category functions, shared
  training/runtime rendering, and category guard. Any number of the nine categories
  may be returned. Unknown/duplicate categories are invalid; empty availability
  stays empty. Returned manual lists are filtered discovery locations, not selected
  manuals or permission grants. Use only public `complete()` and retain one instance.
  Every training target must include the runtime's forced `<think>` envelope:
  upstream `render_example` omits it when reasoning is empty. Keep the rationale
  short and nonempty; runtime failures must not count as valid empty predictions.
- The category pack is an offline synthetic DE/EN experiment. Freeze natural
  train/validation/holdout goals before training; translated siblings stay together.
  Mechanical manual-lookup probes only prove taxonomy coverage. They have 15% of
  training sampling mass; multi-area, context and no-tool cases have explicit mass.
- Category training uses four ladder layers, W4 QAT, rank 16/alpha 32, learning rate
  at most 1e-4 and effective batches of 4/8/16 via gradient accumulation. Preserve
  optimizer/RNG/elapsed budget on resume and keep the 30-minute/1,000-update CPU cap.
  Check native validation periodically. Select by validation F2, then all-required
  coverage, empty accuracy and exact set; claim holdout once for every frozen arm.
- Compare against a matched untrained export and frozen multilingual E5 with
  linear category heads and an optional manual-needed gate. Fit on training only;
  tune regularization and thresholds only on validation. No E5 weights change. Report full-area coverage,
  extra categories/manual fanout and complete request latency, including output
  checks; separate startup. DE/EN evidence does not establish other-language quality.

## Work Guidance

- Run source and CPU checks before paid calls. Start with the 500-row pilot and
  stop bulk generation when its quality/cost gate fails. Never weaken quotas to
  turn an incomplete pack green.
- Use reporting/storage operations for connected multistep requests. Reserve
  extra scenarios for large shared families such as YepAPI while preserving the
  minimum coverage of every family in every language.
- Report draft count, blind agreement, fully accepted count and actual spend
  separately. A successful untrained CPU smoke test is not training acceptance.

## Verification

- `go test ./disposable/export_tools`
- `go run ./disposable/export_tools --out training --check`
- `go run ./disposable/export_tools --manual-router-out training/needle3/catalog.json --check`
- `python training/validate_dataset.py --all`
- `python -m unittest training.needle3.test_pipeline`
- `python -m unittest training.needle3.test_diagnostic`
- `python -m unittest training.needle3.test_local_pilot`
- `python -m unittest training.needle3.test_category`
- In the locked environment, rerun the tests to include real tokenizer and
  checkpoint checks. `smoke.py` exercises the native runtime on the local OS.
- Run `quality.py` then `compile.py`; their failure is expected while accepted
  quotas, audits or retrieval targets are incomplete. Never bypass those gates.

## Child DOX Index

None.
