# Chibi Sensei package contract

## Purpose

Domain package of the Chibi Sensei desktop app (guided studio for training small language models): the SQLite store, rows, history with undo, checks, split, presets, plan, profiles, imports, documents, export, the job runner, the event hub and the prompts, schemas and parsers of generation and cleaning. It never imports `internal/server`, `internal/desktop` or `internal/llm`; the model call reaches it only through values the server hands in (`wrap func(string) string` for untrusted text, parsed answers as strings). Spec: `docs/superpowers/specs/2026-11-22-chibi-sensei-design.md`; plan: `docs/superpowers/plans/2026-11-22-chibi-sensei-p1*.md`.

## Ownership

- `data/chibi_sensei.db`, schema v1 (`schema.go`): every table of the spec (the run, metric, artifact and base-GGUF tables are created for P2 and unused in P1) plus `batches` (`open`, `undone`, `row_count`), `suggestions` (`row_rev`, `applied`), `checks.ready` and `row_history.op` / `after_rev`. `dataset_rows.id` is `AUTOINCREMENT`, so an id is never reused after a delete.
- `profiles/*.json` and their embedded catalog (`profiles.go`); check IDs and message keys (`checks.go`); the prompt texts and JSON schemas of `generate.go`.
- Files: `store.go` open and size; `projects.go` CRUD, steps, delete; `rows.go` validation, structure, hashes, labels; `rowstore.go` add, update, page, filter, select, bulk, delete; `history.go` batches, undo, dedupe; `confirm.go` delete-preview tokens; `checks.go`, `split.go`, `presets.go`, `plan.go`, `suggest.go`, `lang.go`, `tokens.go`; `imports.go`, `documents.go`, `export.go`; `jobs.go`, `events.go`; `generate.go` prompts, schemas, parsing, chunking, the suggestions store and the daily cap.

## Local Contracts

- The store uses one connection (`SetMaxOpenConns(1)`). Inside a transaction use `tx`; never call a store method, the hub or a job's `report` while a `*sql.Rows` or a transaction is open, and never hold a transaction across a model call. Page by `seq > last` with `LIMIT 1000`.
- Every row mutation bumps `data_rev` in the same transaction and calls the notifier after commit. A project change to recipe, task_kind, labels, profile, preset or overrides bumps `data_rev` too.
- A delete of rows (bulk, dedupe, delete by filter) resolves the selection and deletes in one transaction; a delete token binds project, canonical select and `data_rev`.
- History keeps 50 batches. A batch a running job writes (`AddRows` with a caller-named `BatchID`) is `open` and never pruned; the job must call `CloseBatch` when it ends, whatever its outcome (`RunImport` and the generation job do). `AddRows` into an undone batch returns `ErrAlreadyUndone`; a job that meets it ends as cancelled (generation and `RunImport`, which keeps nothing of the undone batch).
- Undo is refused with `ErrUndoConflict` when a later change touched a row of the batch: a row the batch inserted or updated must still have the `after_rev` the batch left, a row it deleted must still be gone. Label, the local flags and `est_tokens` are derived data: undo recomputes them from the restored content under the project's current recipe and task kind (`deriveFields`, as `AddRows` and `UpdateRow`) and clears the cross-row flags, so a recipe change (`recomputeRows` does not bump `rev`) is never undone by an older snapshot.
- Hashes normalize NFC, lower case and collapsed whitespace. The token estimate is a fixed formula in `tokens.go`. The split is by `sha256(seed || id)`. Flags: local flags on every write, cross-row flags only from `RunChecks`.
- Untrusted text (rows, documents) reaches a prompt only through the `wrap` func, and the goal text and the user's instructions are never wrapped. Model output is untrusted: `ParseGenerated` and `ParseClean` decode strictly, validate every row with `ValidateRowInput` and `StructureProblems` (the validator of table edits and imports), cap the count, and `ParseClean` never lets an id outside the batch through.
- Cleaning: `accept "all"` (nil) applies only `fix`; a `drop` needs its row id listed. `ApplySuggestions` skips, and counts as skipped, a suggestion whose row is gone or whose `rev` differs from the recorded `row_rev`, so it never overwrites a newer edit. It writes one batch of kind `clean` with the per-row `after_rev`.
- The daily cap (`settings` key `generator_daily`, local day) is reserved up front by `ReserveDaily` and the unused part refunded by `ReleaseDaily` (a count of another day is left alone).
- Job outcomes: a job whose own deadline passes ends `failed` with `timeout` (a user cancel or shutdown stays `cancelled`). `generate` and `clean` jobs hold one of `llmJobSlots` (2) runner slots while they run; without a free slot the job is stored `queued`, waits with its context (so a cancel works at once) and turns `running` when it gets one. The slot is freed right after the job function returns, panic included. A queued job cancelled before it got a slot still calls its function, with the done context and without a slot (and releases none); the function checks the context first and returns, so only its defers run (refund of the daily reservation, release of the route).
- Limits: `Hub.Subscribe` returns `ErrTooManyStreams` above 16 subscribers per project or 64 in total. `AddOptions.Full` (jobs) is checked before every batch of rows: `ErrStorageFull`, which the job maps to the code `storage_full` and keeps the rows added so far. A JSONL line or CSV record over 1 MiB (`importLineMax`) is skipped and reported as `chibi_sensei.import.line_too_long`; the CSV reader stops a record at the cap and resumes after it (`csvLimitReader`, `csvRecordEnd`; the quote-parity resync is only right for strict quoting). CSV quoting is strict (`LazyQuotes = false`): a record the reader rejects (bare quote, bad quoting) is skipped and reported as `chibi_sensei.import.bad_csv` with its line, and the import goes on with the next record. A document file is read up to `MaxDocumentBytes`.
- Profiles stay `experimental` until a P2 proof run.

## Work Guidance

A new check gets a table row in `checks.go`, a flag only when it is row-level, a fixture test, and a message key handed to the UI locales. A new prompt text or schema change needs a `generate_test.go` case; keep the guard instruction first in every system message. A new job kind gets a `Runner.Start` call in `internal/server` that follows `chibi_sensei_files.go` (runtime store captured at start, stop on runtime shutdown).

## Verification

`go test ./internal/chibisensei/... -count=1`

## Child DOX Index

None.
