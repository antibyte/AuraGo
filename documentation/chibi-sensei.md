# Chibi Sensei

Chibi Sensei is the Virtual Desktop app that guides you through building a training dataset for a small language model. The first phase (P1) covers the dataset studio: you create a project, collect and clean rows, check the dataset, look at the split and an estimated training plan, and export the result as JSONL. P1 does not train anything yet; the plan is an estimate (`trainable` stays `false`).

The app is on by default. Switch it off with `chibi_sensei.enabled: false` (see [Configuration](configuration.md#chibi_sensei)); the app disappears from the desktop and its API answers `chibi_disabled`.

## Projects and recipes

A project has a name, a goal text and one of three recipes:

| Recipe | Rows | Typical use |
|---|---|---|
| `assistant` | Chat rows: optional system message, then user and assistant turns, ending with the assistant. | A helpful assistant in a given style. |
| `specialist` | Chat rows for one task kind: `classify` (with labels), `extract`, `rewrite` or `to_json`. | A small model that does exactly one job. |
| `free` | Text rows (`{"text": "..."}`). | Continued pre-training on plain text. |

Every project is linked to a model profile. The three shipped profiles (Gemma 3 270M, Qwen 3.5 0.8B and LFM 2.5 1.2B) are marked **experimental**: their throughput and size figures are not measured yet. When you do not pick a profile, the app suggests one from the recipe, the length of the inputs and the language, and shows the reason.

## The dataset table

The table pages through the rows (50 per page, at most 200) and filters by text, split, source, flag, label and batch. A project holds at most `chibi_sensei.dataset.max_rows` rows (default 50,000). A row has at most 64 messages and 32,000 characters per message or text. You can edit a row (the edit is refused when it changed in the meantime), add rows by hand, and apply a bulk action to a selection: set the split, add or remove a tag, exclude, or delete. A delete of more than 50 rows, or of every row matching a filter, first asks the server for a preview; the preview returns a token that is valid for five minutes and only fits the rows it counted.

Rows with a broken structure stay editable and carry the flag `structure`; imports and generation skip such rows instead.

### History and undo

Every change that touches rows is one history batch (`add`, `edit`, `bulk_edit`, `bulk_delete`, `dedupe`, `import`, `generate` or `clean`). The last 50 batches are kept, and a batch that is still being written by a running job is never removed. Undo restores the rows of a batch. It is refused with `undo_conflict` when a later change touched one of those rows, and with `already_undone` when it was undone before.

## Importing

Imports read a file from the Desktop workspace and run as a job; a preview shows the first rows and the column mapping first.

- **JSONL**: `messages`, `conversations` (ShareGPT style), `instruction` (Alpaca style), `prompt`/`completion` and `text` objects.
- **CSV**: you map columns to system, user, assistant, text and label.
- **Text** (`.txt`, `.md`, `.markdown`, `free` recipe only): the file is split into one row per paragraph, at most 2,000 characters each.

Rows that cannot be read or are invalid are skipped and listed as issues (the first 200). A file above `chibi_sensei.import.max_file_mb` is refused (`file_too_large`), and an import reads at most `chibi_sensei.import.max_rows` rows.

## Documents

Up to 20 TXT or Markdown files (1,000,000 characters each) can be attached to a project as source material. Documents are only used by generation, as facts the model may draw from.

## Checks

A check run reads the whole dataset and reports findings on three levels: `block` stops you from going on, `warn` is advice, and the rest is information. The checks are `count_min` (too few rows for the profile), `structure`, `short_answer`, `duplicates`, `conflicts` (the same prompt with different answers), `too_long` (rows above the profile's context), `label_balance` (a classifier needs at least two labels; a ratio above 5 to 1 warns), `json_valid` (for `to_json`) and `split_sizes`. A finding with row ids can be shown in the table with the matching `flag` filter. Row-level flags (`structure`, `short_answer`, `invalid_json`) are set on every write; the cross-row flags (`duplicate`, `conflict`, `too_long`, `rare_label`) are set only by a check run, and a result is marked stale as soon as the rows change.

## Split and plan

The split into train, validation and evaluation rows is deterministic: it hashes the project's seed with the row id, so the same data always splits the same way. You can pin a row to `train`, `eval` or `exclude`, and you can reseed. The plan shows the preset (`quick`, `balanced`, `thorough`), the hyper-parameters, the split sizes and an estimated duration. It is an estimate and will stay one until measured values from real runs exist.

## Export

An export job writes the canonical JSONL (`{"messages": [...]}` or `{"text": "..."}`, one row per line) of `all`, `train`, `val` or `eval` rows into the workspace. It refuses to replace an existing file unless you confirm the overwrite.

## Generating rows

Generation asks the configured model for new rows. It is bounded:

- one job per project at a time (`job_active`), at most `chibi_sensei.generator.max_rows_per_job` rows per job (default 500);
- at most 10 rows per model call and `ceil(count / 10) + 2` calls per job;
- at most `chibi_sensei.generator.max_daily_rows` generated or cleaned rows per local day (default 5,000). A job reserves its row count when it starts and gives back what it did not add (`generator_daily_limit` when the cap would be exceeded);
- up to 10 example rows and any of the project's documents can steer the style and the facts; you can add instructions and a language.

The model must answer with JSON that matches a strict schema. Every row it returns is validated like a row you typed (shape, length, structure; a classifier row must use one of the labels, a `to_json` row must be valid JSON), duplicates of existing rows are skipped and the dataset limit is respected. All added rows form one history batch of kind `generate`, so one undo removes them. The job result lists `rows_added`, `rows_skipped`, the `batch_id` and, when the injection scan held calls back, `withheld`. A job stops after the current call when you cancel it, and ends as cancelled when you undo its batch while it runs.

## Cleaning

Cleaning lets the model review up to 500 selected rows, 10 per call. For each row it answers `keep`, `fix` (corrected messages with the same roles in the same order) or `drop` (with a reason). Rows over 8,000 characters are skipped. Nothing changes until you review the suggestions: the review shows each row next to its suggestion, and you apply an explicit list of rows. "Accept all" applies only the fixes; a drop is applied only when you list that row. The apply step writes one history batch of kind `clean` (fixed rows get the source `cleaned`) and skips every row that was edited or deleted after the suggestion was made, so a newer edit is never overwritten. The answer reports how many suggestions were applied and how many were skipped.

### Job errors

A job that fails carries an `error_code` the app translates: `provider_unavailable`, `budget_exceeded`, `prompt_withheld` (the security scan held every call back), `output_cut`, `invalid_answer` (no usable answer in any call), `model_error`, `interrupted` (the server restarted). The request itself can be refused with `provider_unavailable` (no usable generator model), `budget_exceeded`, `generator_daily_limit`, `storage_full` or `job_active`.

### The generator model

`chibi_sensei.generator.provider` names a provider entry that allows general chat; empty means the main model. `chibi_sensei.generator.model` overrides the model. When no usable model exists, the app disables both buttons. Calls are charged to the budget category `chibi_sensei` and stop when the daily budget is used up.

## Storage

Everything lives in `data/chibi_sensei.db` (SQLite). It is part of the backup inventory, and `chibi_sensei.storage.max_gb` (default 20) caps its size including the write-ahead log. Deleting a project removes its rows, history, documents, checks, jobs and suggestions.

## Settings

| Key | Default | Range |
|---|---|---|
| `enabled` | `true` | |
| `storage.max_gb` | `20` | 1-500 |
| `dataset.max_rows` | `50000` | 1000-50000 |
| `import.max_rows` | `20000` | 100-50000 |
| `import.max_file_mb` | `50` | 1-500 |
| `generator.provider` | empty (main route) | |
| `generator.model` | empty (the provider's model) | |
| `generator.max_rows_per_job` | `500` | 10-5000 |
| `generator.max_daily_rows` | `5000` | 100-100000 |
| `training.machine_shape` | `NvidiaTeslaT4` | `NvidiaTeslaT4` or `NvidiaL4`; also listed in `kaggle.allowed_machine_shapes` |
| `training.delete_remote_after_run` | `true` | needs `kaggle.allow_delete` |

See [Configuration](configuration.md#chibi_sensei) for the details and the [API reference](manuals/en/21-api-reference.md#chibi-sensei-api) for the routes.

## Privacy and security

In P1 nothing leaves the machine except the calls to the configured generator route during generation and cleaning. For those calls the app treats your rows and documents as material, never as instructions: they are fenced in `<external_data>` tags and the model is told not to follow anything inside them, and the composed prompt passes the injection scan before it is sent. Your goal text and your own instructions are your task and are not fenced. Model answers are parsed strictly against a schema and checked row by row before anything is stored. Keys and provider error texts never reach job results, events or responses.
