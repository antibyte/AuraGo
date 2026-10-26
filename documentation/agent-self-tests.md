# Agent self-tests

Open **Dashboard → Tools → Selbsttest**. The runtime catalog contains 320 individual German prompt templates. AuraGo takes the current dispatch inventory and runs selected cases in fresh minimal-loop sessions, without ordinary chat memory/personality/RAG or automatic learning. Disabled tools are not enabled by a test.

## Profiles

- **Nur prüfen** selects reviewed, target-free reads and saved-configuration connection probes. Other enabled operations remain explicitly uncovered or blocked; a read-like name is not sufficient authorization.
- **Mit lokalen Testdaten** also enables the implemented disposable local file fixtures: text write/readback, exact edits, structured JSON/YAML/TOML/XML edits and supported file-reading cases. Only the assigned file/directory is accessible to that case, and cleanup is verified by the coordinator.
- **Gezielte freigegebene Tests** takes exact human-provided bindings for an enabled native tool and a matching catalog case. The primary call is admitted once. Optional verification and cleanup calls are separately fixed. Normal integration permissions, action policy and live revocation still apply. Laboratory and human-observation cases cannot be promoted to ordinary live calls.

The default run deadline is ten minutes and its local primary-model token reservation is 32,768 tokens. Individual cases have a two-minute deadline and bounded request/tool counts. Provider/tool/Guardian accounting and configured global budgets remain active; the report does not invent a dollar cost or claim to meter every auxiliary request. No live test starts merely by loading the page or catalog.

## Exact bindings

The targeted profile's JSON field accepts an array. Example for a disposable, already prepared native-tool test target:

```json
[
  {
    "case_id": "INF-20",
    "tool": "sql_query",
    "allow_effects": true,
    "arguments": {"connection_id": "YOUR_TEST_CONNECTION_ID", "query": "SELECT 1 AS selftest_value"},
    "expected": {"status": "success"}
  }
]
```

Replace arguments and expected fields using the **current** tool schema and actual response contract; the example is not a promise that a particular database wrapper uses these fields. `allow_effects` is an explicit exact-call approval and never overrides an existing denial. Bind only synthetic test resources; never include credentials. Each `expected` key is a dotted JSON field path, optionally starting with `$.`, and its value must match exactly. A status check covers status only; use a readback with meaningful expected data to verify an actual outcome. Non-JSON legacy results remain inconclusive.

Optional `verify: {"arguments": {...}}` and `cleanup: {"arguments": {...}}` use the same tool. They are fixed calls, not model-selected actions. A queued job or unknown mutation must be reconciled before another attempt. Supply an appropriate terminal-state verification for asynchronous work. Use a dedicated test account/namespace and exact object identity for cleanup; do not authorize deletion of existing user objects.

Numeric dotted segments select array entries, for example `rows.0.value`. Assertions fail on missing fields, empty arrays or mismatched types/values.

Bindings remain in memory for the current run. Persisted evidence stores argument/output hashes and opaque fixture identifiers, not raw arguments, model prose, secrets or tool payloads. A restart never automatically replays them.

## Results

The panel shows progress, Stop, per-case verdicts and Markdown/JSON downloads. Downloads and all self-test APIs are administrator-only and `no-store`. API records live under `<data_dir>/selftests/` with rooted atomic writes. Historical records are bounded by age/count/storage, while unresolved effects or cleanup remain visible.

PASS requires typed execution evidence and the listed assertions. A successful connection probe does not prove message delivery, a file's existence does not prove its contents, and primary-selector coverage does not imply every operation combination was tested. Missing setup, permissions, targets, specialized job profiles and hardware are reported separately. Untested dynamic MCP/custom capabilities require review or a dedicated adapter; the runner does not guess their effects.

Model summaries cannot change ledger verdicts. Interrupted in-flight cases become UNKNOWN; future unexecuted cases become cancelled. Inspect receipt IDs and residual fixture names before retrying. No automatic repair, deployment, restart or schedule is created.

## API

- `GET /api/self-tests/catalog`: prompt templates and current capability inventory.
- `GET /api/self-tests`: recent run summaries.
- `POST /api/self-tests`: start with `profile`, optional `targets`, `case_ids`, `timeout_seconds`, `token_limit`, and targeted `bindings`.
- `GET /api/self-tests/<id>`: private run/evidence ledger.
- `POST /api/self-tests/<id>/cancel`: cancel the active run.
- `GET /api/self-tests/<id>/report.md` or `/results.json`: download a report.

Browser writes require a matching Origin. Use the existing authenticated administrator boundary; no generic HTTP/loopback or model-supplied credential bypass is provided. Scheduling can be added explicitly around an approved deployment workflow; the self-test feature does not silently create missions.
