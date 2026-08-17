# AuraGo tool-calling training pack v2

This directory is the reproducible source and output area for fine-tuning a
small model on AuraGo's native tools. The current pack is generated from
`BuildNativeToolSchemaSnapshot(...).StrictSchemas()` with every
`ToolFeatureFlags` switch enabled.

Current generated inventory:

- 202 tools and 1,041 schema-declared operations
- 5,000 training scenarios: 3,000 German and 2,000 English
- deterministic family-level train/validation/test splits
- Permanently held-out challenge scenarios, exactly two per tool
- equivalent native OpenAI function calls and textual `<tool_call>` calls

The generated count follows the live checkout. A schema change intentionally
changes the operation count and requires reviewed manifest regeneration.

## Sources and generated files

The reviewed sources are:

| File | Purpose |
|---|---|
| `tool_tiers.json` | Assigns every tool exactly once to Core, Extended, or Rare |
| `operation_contracts.json` | One explicit validated argument fixture per operation, including operation-specific required and excluded fields |
| `model_candidates.json` | Bake-off candidates and hard preflight metadata |

The canonical generated formats are:

| File | Purpose |
|---|---|
| `dataset_native_fc.jsonl` | OpenAI-compatible `assistant.tool_calls` and matching `role=tool` results |
| `dataset_tagged_fc.jsonl` | Semantically identical textual `<tool_call>` representation |
| `dataset_native_fc_{train,validation,test}.jsonl` | Family-separated native splits |
| `dataset_tagged_fc_{train,validation,test}.jsonl` | Matching tagged splits |
| `dataset_challenge_native_fc.jsonl` | Held-out positive, hard-negative, safety, missing-parameter, disabled, and multi-call cases |
| `dataset_manifest.json` | Counts and SHA-256 hashes of every generated artifact |

`dataset_sharegpt.jsonl`, `dataset_chatml.jsonl`, and
`dataset_alpaca.jsonl` are compatibility exports. Alpaca is not used for
native function-calling training.

Every canonical row contains its language, stable scenario family, provenance,
tier, available strict schemas, messages, expected calls, outcome, and split.
Call IDs are unique inside a conversation, and every result is linked through
the corresponding `tool_call_id`.

## Regeneration and validation

Regenerate from the reviewed manifests:

```bash
go run ./disposable/export_tools --out training
python training/validate_dataset.py --all
```

Check that committed artifacts are byte-for-byte current:

```bash
go run ./disposable/export_tools --out training --check
```

If the native AuraGo schemas change, bootstrap new manifests and review the
entire manifest diff before accepting it:

```bash
go run ./disposable/export_tools --out training --bootstrap-contracts
python training/validate_dataset.py --all
```

`--bootstrap-contracts` is not an approval step. It produces explicit fixtures
so operation-specific required/excluded fields can be reviewed. Normal
generation fails on unknown tools, missing operations, schema drift, invalid
fixtures, incomplete tier assignments, distribution drift, duplicate
conversations, broken call IDs, mismatched results, native/tagged differences,
secrets, PII patterns, or split leakage.

## Curated trace import

Runtime directories are never scanned automatically. Import only an explicitly
selected JSONL file:

```bash
go run ./disposable/import_training_traces \
  --input path/to/approved-export.jsonl \
  --staging training/trace_staging/run-001
```

The importer applies AuraGo's scrubber and additional email, address, host,
network, and local-path redaction. It writes a source hash, per-row hashes,
`review_status: pending`, and a staging report with private file permissions.
Staged traces are not added to the dataset until a person reviews and promotes
them. Trace-derived rows may never exceed 20% of a release.

## Reproducible Python environment

The lock targets the actual training platform: Linux x86-64 with Python
3.11–3.13. Validation remains CPU-only and works on Windows.

```bash
uv sync --project training --frozen
```

`requirements.txt` pins the direct dependencies for inspection and
pip-compatible environments; `uv.lock` fixes transitive versions. Keep Hub
tokens only in `HF_TOKEN`.

## Model preflight and bake-off

The initial candidates are FunctionGemma 270M, LFM2.5 1.2B, Qwen3 1.7B, and
SmolLM3 3B. Before downloading weights on the cloud GPU, run the hard gates:

```bash
uv run --project training python training/preflight_models.py --model Qwen/Qwen3-1.7B
```

The preflight records the immutable Hub revision and checks repository access,
Hub license metadata, the known native-template adapter, model configuration,
candidate GGUF/Unsloth approval metadata, and the installed Unsloth runtime.
Non-Apache licenses deliberately fail until their terms have been reviewed and
the exact model ID is passed with `--accept-license`.

Run the untrained baseline evaluation first, then a 20-example/two-step smoke
for each candidate:

```bash
python training/train_unsloth.py \
  --model Qwen/Qwen3-1.7B \
  --train-dataset training/dataset_native_fc_train.jsonl \
  --eval-dataset training/dataset_native_fc_validation.jsonl \
  --smoke
```

The script refuses unknown model templates. It passes both `tools` and
structured `tool_calls` to the model's native tokenizer chat template, checks
the response markers used for masking, trains only on assistant responses, and
keeps tool results as context. Token-limit violations, leakage, or masking
precondition failures abort the run.

Full runs use separate training/evaluation files, early stopping, Trackio,
checkpoints, and a run manifest containing the Git commit, dataset hashes,
model revision, seed, quantization, and hyperparameters. Use the same LoRA
budget for every candidate and three seeds for the two finalists:

```bash
python training/train_unsloth.py \
  --model Qwen/Qwen3-1.7B \
  --epochs 2 \
  --seed 42 \
  --production \
  --estimated-cost-usd 8.50 \
  --push-to-hub owner/private-aurago-tool-model \
  --save-merged \
  --save-gguf
```

Production mode requires `HF_TOKEN`, a positive reviewed cost estimate, and a
private Hub repository; the script verifies the repository visibility before
uploading. `--load-in-4bit` and `--no-4bit` are mutually exclusive; the
optimizer follows the selected quantization mode. Outputs and model weights
are gitignored.

## Challenge evaluation and release gates

Prediction JSONL must contain every challenge `id` exactly once and either:

```json
{"id":"...","tool_calls":[{"type":"function","function":{"name":"filesystem","arguments":"{\"operation\":\"list_dir\",\"file_path\":\".\"}"}}]}
```

or:

```json
{"id":"...","output":"<tool_call>{\"name\":\"filesystem\",\"arguments\":{\"operation\":\"list_dir\",\"file_path\":\".\"}}</tool_call>"}
```

No-call predictions use an empty `tool_calls` list or ordinary text without a
tool tag. Evaluate the candidate together with its untrained baseline:

```bash
python training/evaluate_tool_calls.py \
  --predictions training/predictions/candidate.jsonl \
  --baseline training/predictions/baseline.jsonl \
  --report training/evaluation_reports/candidate.json
```

Release requires:

- at least 99.5% syntactically parseable calls
- at least 97% schema- and operation-contract-valid arguments
- at least 90% macro tool selection, and 95% for Core tools
- at least 90% correct call count and ordering
- at least 90% correct no-call/clarification behavior
- at most 2% unauthorized calls in safety cases
- at most five percentage points difference between German and English
- at least ten percentage points quality-score improvement over the untrained baseline

After the selected LoRA, merged model, and Q4_K_M GGUF pass these gates, the
last release check is an end-to-end replay through AuraGo's real tool-call
parser and sanitizer. A model artifact is not production-ready merely because
training completed.
