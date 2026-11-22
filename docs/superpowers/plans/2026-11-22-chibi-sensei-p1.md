# Chibi Sensei P1 (foundation and dataset studio) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship phase P1 of Chibi Sensei: a desktop app in which the user creates a training project, builds and checks a dataset (by hand, from files, and with bounded LLM help) and exports it as JSONL.

**Architecture:** Domain logic lives in the new package `internal/chibisensei` with its own SQLite file `data/chibi_sensei.db`; HTTP handlers and adapters live in `internal/server/chibi_sensei_*.go` under `/api/desktop/chibi-sensei`; the UI is a set of classic-script modules in `ui/js/desktop/apps/chibi-sensei/`. Backend and frontend meet only at the HTTP contract below.

**Tech Stack:** Go (modernc SQLite, net/http, SSE), vanilla JS classic scripts, CSS with `--vd-mat-*` tokens, 16 locale JSON files.

Design spec: `docs/superpowers/specs/2026-11-22-chibi-sensei-design.md`.

This plan has three files:

- this index: conventions, the HTTP contract, the batch order;
- `2026-11-22-chibi-sensei-p1-backend.md`: tasks B1 to B6;
- `2026-11-22-chibi-sensei-p1-frontend.md`: tasks F1 to F5.

---

## Conventions for every batch

**Where to work.** Only in the worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\chibi-p1` (branch `feature/chibi-sensei-p1`). Never read or write the main checkout at `C:\Users\Andi\Documents\repo\AuraGo` outside that folder: other sessions work there.

**Who commits.** Implementers do not commit, stage, stash, rebase or switch branches. The coordinating session reviews each batch and commits its paths.

**One batch, one dispatch.** Each task (B1 … F5) goes to one implementer in one dispatch, followed by one combined review that reports only Critical and Important issues.

**Tests.** Write the tests the task names, plus one test for each bug you fix on the way. No property, mutation or stress suites, no `-race` runs.

**Go commands** (Git Bash, from the worktree root):

- `go build ./...` must stay green after every batch.
- Run only the packages a task names, for example `go test ./internal/chibisensei/... -count=1`. For `-run`, group patterns as `(?i)(a|b)`.
- If Go reports `no such tool "vet"`, prefix the command with `GOTOOLCHAIN=go1.27.4`.
- Do not set `GOTMPDIR` or `GOCACHE` yourself.

**UI commands.** `node --check <file>` for every changed JS file, `npm run build:ui` after touching a bundled source, `npm run check:ui` before reporting done.

**Text encoding.** Write real UTF-8 characters (umlauts, CJK) into source and JSON files. Never write `\u` escape sequences through the file-writing tool: it decodes them.

**UI rules.** `textContent` only (no `innerHTML`, no `eval`, no `localStorage`), no external assets, CSS classes start with `csn-`, each JS module stays under 1100 lines, every user-visible string is a translated key.

**Scratch files** go to `C:\disposable\chibi-sensei\`, never into the repository.

**Processes.** Stop only processes you started, by PID. Never kill by image name.

**Code intelligence.** GitNexus cannot index a worktree. The coordinating session runs the impact analysis for existing symbols before a batch starts; implementers do not run GitNexus.

**Report format.** End with: files created, files modified, the exact test commands run with their PASS/FAIL lines, deviations from the plan with the reason, and anything left open.

## Impact analysis of existing symbols (2026-11-22)

B1 is the only batch that changes existing functions. Every change is additive (one new field, list entry, `case` or call).

| Symbol | Risk | Change |
|---|---|---|
| `BuiltinApps` (internal/desktop/types.go) | LOW, 22 impacted | one new manifest entry |
| `desktopCapabilities.HasCapability` | LOW, 10 impacted | one new `case` |
| `(*Config).ProviderReferences` | LOW, 18 impacted | one new reference |
| `SQLiteArchivePaths` | HIGH, reaches `main` | one new filename |
| `desktopRequestOperation` | HIGH, 156 impacted (every desktop route) | one new path rule for job cancel |
| `handleUpdateConfig` | HIGH, the config save route | one new validation call |
| `Load`, `Config`, `Server`, `Start`, the route registration function | hubs; the index could not size `Load` and does not know `registerRoutes` by that name | new defaults, one normalize call, new fields, one boot call, one route line |

Because of the HIGH entries, B1's verification runs the whole `internal/config` and `internal/desktop` packages and, in `internal/server`, the desktop-operation, config-save and Chibi Sensei tests.

## Batch order

1. B1 (backend foundation).
2. Then two chains in parallel, with strict path ownership:
   - backend B2 → B3 → B4 → B5 → B6 owns `internal/**`, `config_template.yaml`, `documentation/**`;
   - frontend F1 → F2 → F3 → F4 → F5 owns `ui/**`, `scripts/**`.
3. Integration: one browser smoke, one whole-branch review, rebase onto `main`, land.

`go test ./ui` fails between B1 and F1, because the built-in app list then names an app whose launcher key is not yet in the 16 desktop locale files. F1 follows B1 directly.

---

## HTTP contract

Base contract as set before the two plan halves were written. The accepted changes follow it and win where they differ.

### Conventions

- Base path: `/api/desktop/chibi-sensei`. JSON in and out, `Cache-Control: no-store`.
- Errors: flat `{ "error": "<code>", "code": "<code>", "message": "<english fallback>" }`, plus `issues` on 422.
- Writes need the same-origin check the Storymaker handlers use. A read-only desktop answers 403 `desktop_readonly`.
- Optimistic concurrency: `If-Match: <rev>` on `PATCH /projects/{id}` and `PUT .../rows/{rowId}`; mismatch is 412 `conflict`.
- IDs: project, document and job IDs are UUID strings; row IDs are integers.
- Times are RFC 3339 UTC strings.

### Routes in P1

#### Meta
`GET /meta` →
```json
{"readonly":false,"admin":true,
 "limits":{"max_rows":50000,"max_message_chars":32000,"max_messages":64,"import_max_file_mb":50,
           "generator_max_rows_per_job":500,"page_max":200},
 "recipes":[{"id":"assistant","task_kinds":[]},
            {"id":"specialist","task_kinds":["classify","extract","rewrite","to_json"]},
            {"id":"free","task_kinds":[]}],
 "profiles":[{"id":"gemma3-270m","display_name":"Gemma 3 270M","status":"experimental","params_b":0.27,
              "context_max":32768,"languages":["*"],"text_only":true,"system_role":"merge_into_first_user",
              "method_default":"full","gguf_estimated_mb":300,
              "license":{"id":"gemma","name":"Gemma Terms of Use","url":"https://ai.google.dev/gemma/terms","requires_consent":true}}],
 "generator":{"available":true,"model":"<model name or empty>"}}
```

#### Projects
- `GET /projects?archived=0|1` → `{"projects":[ProjectSummary]}`
- `POST /projects` `{name, recipe, task_kind?, labels?, goal_text?, profile_id?}` → 201 `{"project":Project}`
- `GET /projects/{id}` → `{"project":Project}`
- `PATCH /projects/{id}` (If-Match) `{name?, recipe?, task_kind?, labels?, goal_text?, profile_id?, preset?, overrides?}` → `{"project":Project}`
- `POST /projects/{id}/archive` `{archived:bool}` → `{"project":Project}`
- `POST /projects/{id}/delete` `{confirm_name}` → 204. Only an archived project can be deleted (409 `not_archived`); `confirm_name` must equal the project name (422).

```json
ProjectSummary = {"id":"…","name":"Support bot","recipe":"assistant","task_kind":"","profile_id":"qwen35-0p8b",
  "rows":412,"data_rev":57,"rev":4,"updated":"…","archived":false,
  "steps":{"goal":"done","data":"in_progress","training":"todo","test":"todo","use":"todo"}}
Project = ProjectSummary + {"labels":["…"],"goal_text":"…","preset":"balanced","overrides":{},"split_seed":123,
  "created":"…","suggested_profile":{"id":"qwen35-0p8b","reason_key":"chibi_sensei.suggest.assistant_other_language"}}
```
Step states: `todo | in_progress | ready | done | blocked`. In P1 `training`, `test` and `use` are always `todo`.

#### Rows
- `GET /projects/{id}/rows?offset=&limit=&q=&split=&source=&flag=&label=&batch=&sort=seq|est_tokens&dir=asc|desc`
  → `{"rows":[Row],"total":n,"offset":0,"limit":50,"data_rev":57}` (limit 1..200, default 50)
- `POST /projects/{id}/rows` `{"rows":[RowInput]}` (1..200) → 201 `{"rows":[Row],"data_rev":n}`
- `GET /projects/{id}/rows/{rowId}` → `{"row":Row}`
- `PUT /projects/{id}/rows/{rowId}` (If-Match: row rev) RowInput → `{"row":Row,"data_rev":n}`
- `POST /projects/{id}/rows/bulk` `{"select":Select,"action":"set_split|add_tag|remove_tag|exclude|delete","value":"…","token":"…"}`
  → `{"affected":n,"batch_id":"…","data_rev":n}`. `delete` of more than 50 rows, or any delete by filter, needs a `token`.
- `POST /projects/{id}/rows/delete-preview` `{"select":Select}` → `{"count":n,"token":"…","expires_in_s":300}`
- `POST /projects/{id}/rows/dedupe` → `{"removed":n,"batch_id":"…","data_rev":n}` (keeps the first row by `seq`)
- `GET /projects/{id}/history?limit=20` → `{"batches":[{"batch_id":"…","kind":"bulk_delete|bulk_edit|edit|dedupe|import|generate|clean|add","rows":n,"created":"…"}]}`
- `POST /projects/{id}/history/{batchId}/undo` → `{"restored":n,"data_rev":n}`

```json
Row = {"id":123,"seq":5,"kind":"chat","messages":[{"role":"system|user|assistant","content":"…"}],"text":null,
       "label":"positive","split":"auto|train|eval|exclude","source":"manual|file|generated|cleaned|hf|sensei",
       "source_ref":"Imports/data.csv#L12","tags":["…"],"est_tokens":180,"flags":["duplicate","too_long"],"rev":3}
RowInput = {"messages":[…]} or {"text":"…"}, plus optional "split" and "tags"
Select = {"ids":[1,2,3]} or {"filter":{"q":"","split":"","source":"","flag":"","label":"","batch":""}}
```

#### Checks and split
- `POST /projects/{id}/checks/run` → CheckResult (schema in the spec, section "Dataset studio")
- `GET /projects/{id}/checks` → `{"result":CheckResult|null,"stale":bool}`
- `GET /projects/{id}/split` → `{"seed":123,"counts":{"train":371,"val":21,"eval":20,"excluded":0},"pinned":{"train":0,"eval":0}}`
- `POST /projects/{id}/split/reseed` → same shape with a new seed

#### Documents (source material for generation)
- `GET /projects/{id}/documents` → `{"documents":[{"id":"…","name":"faq.md","workspace_path":"Documents/faq.md","chars":18234,"created":"…"}]}`
- `POST /projects/{id}/documents` `{"workspace_path":"…"}` → 201 `{"document":{…}}` (TXT and Markdown only)
- `DELETE /projects/{id}/documents/{docId}` → 204

#### Imports and exports
- `POST /projects/{id}/imports/preview` `{"workspace_path":"…","format":"auto|jsonl|csv","mapping":Mapping?}` →
  `{"format":"csv","columns":["question","answer"],"mapping":Mapping,"sample":[RowInput x ≤5],"issues":[{"line":3,"message_key":"…"}]}`
- `POST /projects/{id}/imports` `{"workspace_path":"…","format":"jsonl|csv","mapping":Mapping}` → 202 `{"job":Job}`
- `POST /projects/{id}/exports` `{"workspace_path":"…","which":"all|train|val|eval","overwrite":false}` → 202 `{"job":Job}`

```json
Mapping = {"shape":"messages|conversations|instruction|prompt_completion|text|text_label|columns",
           "system":"<column or empty>","user":"<column>","assistant":"<column>","text":"<column>","label":"<column>",
           "system_text":"<fixed system prompt or empty>"}
```

#### Generation and cleaning (bounded one-shot LLM jobs)
- `POST /projects/{id}/generate` `{"count":90,"seed_row_ids":[…≤10],"document_ids":["…"],"instructions":"…","language":"de"}` → 202 `{"job":Job}`
- `POST /projects/{id}/clean` `{"select":Select,"instructions":"…"}` (at most 500 rows) → 202 `{"job":Job}`
- `GET /jobs/{jobId}/suggestions?offset=&limit=` → `{"suggestions":[{"row_id":12,"action":"keep|fix|drop","messages":[…],"reason":"…","before":Row}],"total":n}`
- `POST /jobs/{jobId}/apply` `{"accept":"all"}` or `{"accept":[12,40]}` → `{"applied":n,"batch_id":"…","data_rev":n}`

#### Jobs
- `GET /projects/{id}/jobs?active=1` → `{"jobs":[Job]}`
- `GET /jobs/{jobId}` → `{"job":Job}`
- `POST /jobs/{jobId}/cancel` → `{"job":Job}`

```json
Job = {"id":"…","project_id":"…","kind":"generate|clean|import_file|export","state":"queued|running|succeeded|failed|cancelled",
       "progress":{"done":40,"total":90,"phase":"generating"},"result":{"rows_added":38,"rows_skipped":2,"batch_id":"…"},
       "error_code":"","error":"","created":"…","updated":"…"}
```
One job of a kind per project at a time (409 `job_active`).

#### Plan (read-only in P1)
- `GET /projects/{id}/plan?preset=quick|balanced|thorough` →
```json
{"preset":"balanced","profile_id":"qwen35-0p8b","method":"lora",
 "hyper":{"epochs":2,"learning_rate":0.0002,"per_device_batch":4,"grad_accum":4,"max_seq_len":1024,
          "lora":{"r":16,"alpha":16,"dropout":0.05}},
 "split":{"train":371,"val":21,"eval":20},"estimate_minutes":34,"session_timeout_minutes":66,"data_rev":57,
 "trainable":false,"reason_key":"chibi_sensei.plan.training_later"}
```

#### Events
- `GET /projects/{id}/events` (SSE): events `job` (data: Job), `rows` (data: `{"data_rev":n}`), `checks`
  (data: `{"stale":true}`), comment heartbeat every 15 s. The UI also polls active jobs every 2.5 s.

### Accepted contract changes (from the backend plan)

All of these are additive except item 3, which only fills in codes the contract leaves open.

1. **Imports gain `format: "text"`** in `POST /projects/{id}/imports/preview` and `POST /projects/{id}/imports`.
   - Allowed for `.txt`, `.md` and `.markdown` files, and only in the `free` recipe.
   - It splits the file into text rows by paragraph, at most 2,000 characters each. This is the spec's "split into text rows" opt-in.
   - `format: "auto"` never picks `text`.
2. **Extra response fields.**
   - Suggestion objects gain `applied` (bool) and, for free-recipe rows, `text`.
   - `Job.result` gains optional fields: `issues` (import line issues, at most 200), `suggestions` (clean), `withheld` (LLM calls skipped by the injection scan), `rows` (export) and `path` (export).
3. **New error codes:**
   - `precondition_required` 428 when `If-Match` is missing.
   - `confirmation_required` 428 when a delete needs a token and none was sent; `confirmation_invalid` 409 when the token is wrong or expired.
   - `generator_daily_limit` 429, `storage_full` 409, `data_changed` 409 (rows changed during a check run), `already_undone` 409.
   - `file_too_large` 413, `unsupported_file` 422, `profile_unknown` 422, `job_not_found` 404.
4. **Row filter `flag`** accepts a flag name (`duplicate`) or a check ID (`duplicates`). The spec says `flag=<check id>`, and the contract's Row example uses flag names.
5. **`GET /meta` returns `"profiles": []` after B1** until B5 wires the profile catalog. F1 must tolerate an empty list.
