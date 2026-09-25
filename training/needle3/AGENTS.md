# Needle3 manual routing preparation

## Purpose

Prepare and evaluate a standalone manual selector. Production AuraGo integration
belongs to a later task; no application prompt or dispatch behavior changes here.

## Ownership

- This directory owns synthesis, release gates, retrieval, serialization, QAT,
  checkpoints, export, local inference, and bounded RunPod run preparation.
- `disposable/export_tools/manual_router.go` exports the strict catalog with the
  canonical `prompts.ToolManualID` bindings and serves the actual catalog search.
- Existing `training/` datasets and validators remain independently authoritative.

## Local Contracts

- `config.json` is the requested quota and budget contract: 70,000 accepted rows,
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
- Training and inference share `serialization.py`. No silent truncation; no calls
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
- In the locked environment, rerun the tests to include real tokenizer and
  checkpoint checks. `smoke.py` exercises the native runtime on the local OS.
- Run `quality.py` then `compile.py`; their failure is expected while accepted
  quotas, audits or retrieval targets are incomplete. Never bypass those gates.

## Child DOX Index

None.
