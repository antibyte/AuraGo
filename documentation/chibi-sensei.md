# Chibi Sensei

Chibi Sensei is the Virtual Desktop app that guides you through building a training dataset for a small language model and training the model on Kaggle. You create a project, collect and clean rows, check the dataset, look at the split and the training plan, export the result as JSONL, and train on a free Kaggle GPU; the result is a GGUF file you can download. Testing and using the model inside AuraGo come in a later phase.

The app is on by default. Switch it off with `chibi_sensei.enabled: false` (see [Configuration](configuration.md#chibi_sensei)); the app disappears from the desktop and its API answers `chibi_disabled`.

## Projects and recipes

A project has a name, a goal text and one of three recipes:

| Recipe | Rows | Typical use |
|---|---|---|
| `assistant` | Chat rows: optional system message, then user and assistant turns, ending with the assistant. | A helpful assistant in a given style. |
| `specialist` | Chat rows for one task kind: `classify` (with labels), `extract`, `rewrite` or `to_json`. | A small model that does exactly one job. |
| `free` | Text rows (`{"text": "..."}`). | Continued pre-training on plain text. |

Every project is linked to a model profile. The three shipped profiles (Gemma 3 270M, Qwen 3.5 0.8B and LFM 2.5 1.2B) are **verified**: each passed a full training run on Kaggle with a Tesla T4, and its speed, memory and size figures are measured (see [What to expect](#what-to-expect)). A profile carries the **experimental** badge until it has passed such a run. When you do not pick a profile, the app suggests one from the recipe, the length of the inputs and the language, and shows the reason.

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

The split into train, validation and evaluation rows is deterministic: it hashes the project's seed with the row id, so the same data always splits the same way. You can pin a row to `train`, `eval` or `exclude`, and you can reseed. The plan shows the preset (`quick`, `balanced`, `thorough`), the hyper-parameters, the split sizes and an estimated duration. It is an estimate and an upper bound; the proof runs on Kaggle finished well inside it (see [What to expect](#what-to-expect)). The plan also tells whether a run must convert the base model as well (`need_base_gguf`); once a base GGUF of the same revision and converter is stored, later runs skip it.

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

## Training on Kaggle

Training runs on a GPU in your own Kaggle account. AuraGo prepares the data and a notebook, starts it, follows it and downloads the result.

### What you need

- A Kaggle account with phone verification (Kaggle gives GPU and internet to notebooks only then) and its API token saved under Config → Kaggle (it goes into the Vault).
- The Kaggle integration switched on with these permissions: `kaggle.enabled`, not `kaggle.read_only`, `allow_create`, `allow_remote_execute`, `allow_gpu`, `allow_internet`, `allow_download`, and the GPU of `chibi_sensei.training.machine_shape` listed in `kaggle.allowed_machine_shapes`. `allow_delete` is optional; without it the uploaded data stays on Kaggle.
- AuraGo's network and filesystem-write permissions, a writable desktop and the Vault.
- A ready dataset: the latest check run belongs to the current rows and found nothing blocking.
- Acceptance of the base model's license where it requires one (Gemma, LFM). The Gemma Terms of Use have no version number; the app shows the date of their last change, 2026-04-01, as the version you agree to.

### What to expect

The three shipped profiles each passed a full training run on Kaggle with a Tesla T4. These are the measured values:

| Measure | Gemma 3 270M | Qwen 3.5 0.8B | LFM 2.5 1.2B |
|---|---|---|---|
| default method | full fine-tuning (LoRA also works) | LoRA | LoRA |
| training speed on the T4 | about 2100 tokens per second | about 500 tokens per second | about 390 tokens per second |
| longest row the GPU holds | 2048 tokens | 1024 tokens | 2048 tokens |
| size of the trained model file (GGUF, 8 bit) | about 290 MB | about 810 MB | about 1250 MB |
| quick preset with 200 short rows, whole run | about 5 minutes | about 7 minutes | about 11 minutes |
| GPU quota used by that run | about 2.5 minutes | about 4 minutes | about 4 minutes |

A profile has a maximum row length in tokens. Longer rows are cut off at the end, and the run shows the hint `rows_truncated`. Gemma 3 270M and LFM 2.5 1.2B take 2048 tokens. Qwen 3.5 0.8B takes 1024 tokens, because one longer Qwen row does not fit into the memory of a T4.

If the planned batch does not fit into the GPU's memory, the trainer halves the batch by itself and goes on. The run shows the hint `batch_reduced`; training takes a little longer and the result is the same kind. This happened in the test runs with long rows for Qwen and LFM. Only a single row that does not fit makes the run fail, with `oom`. Such a run ends after about three minutes, is cleaned up on Kaggle and shows the reason, for example: "one sequence of 1834 tokens does not fit into the memory of Tesla T4 (method lora)".

The estimate in the start dialog is an upper bound. The proof runs took 5 to 11 minutes where the estimate said 21 to 24.
- Enough weekly GPU quota and no other Kaggle GPU run of AuraGo (the agent's or the Kaggle app's); `kaggle.max_remote_jobs` (default 1) is the limit. A job in state `unknown` keeps its slot until an administrator releases it in the Kaggle app.

### Preflight and the enable button

The Training step lists every requirement, grouped by area. A red item blocks the start, a note only warns. An administrator sees **Enable for Chibi Sensei**: a dialog lists each Kaggle setting it will switch on, and confirming saves exactly those to `config.yaml` (Kaggle on, read-only off, the permissions on, the machine shape added to the allow-list). Others see what to ask the administrator for. Nothing is switched on implicitly. The connection test, the quota and the job list are read at most every 20 seconds; a start always reads them fresh.

### What leaves the machine

Data leaves only after you confirm the start dialog. It names the private dataset `<you>/chibi-data-<run>` and the private notebook `<you>/chibi-<project>-<run>` (`<project>` is the first 8 hex characters of the project ID), the row count and size, the license, the estimated time, the time limit, the remaining quota and what happens afterwards. The confirmation is valid for 15 minutes and for exactly that plan: if rows, settings or the profile change, prepare the start again.

- To the dataset: the frozen train, validation and evaluation files and a small marker file.
- To the notebook: the trainer script and `job.json` (profile, hyper-parameters, pinned versions). No AuraGo secret, host path or Hugging Face token.
- The notebook itself downloads the base model from Hugging Face at a pinned revision, pinned pip packages and the pinned llama.cpp converter (checked by sha256).

### During the run

The monitor shows the run's phase (prepare, upload, start, train, download, cleanup) and the trainer's stages. Kaggle serves the notebook log while the session runs. AuraGo reads it once a minute and shows the progress and the loss chart as far as the log has reached. The first trainer lines arrive 20 to 40 seconds after the start. A stage that takes less than a minute, such as the training of a tiny dataset, can pass between two reads, and its progress bar is then never seen. The loss chart is complete after the run in every case.

After the last trainer stage, Kaggle saves the notebook's files before it reports the session as finished. For the largest model, LFM 2.5 1.2B, that took about three minutes. The run stays in `train`, with all stages done, during that time, and then moves to `download`. Downloading two model files of 1.25 GB took another two and a half minutes on the test server.

AuraGo's Kaggle operations show up in the Kaggle app's job list under `chibi-` names. A restart of AuraGo resumes the run; nothing is sent twice. If the status read fails after the run's deadline (its time limit plus 20 minutes), AuraGo waits up to 30 more minutes for a successful read before the run ends with `remote_timeout`; a successful read decides at once.

### Cancel

- Before the notebook started, the run stops at once on your machine, and the uploaded data is cleaned up.
- After that, AuraGo asks Kaggle once to stop the session. Kaggle normally does not allow that through the API, so the run shows **Stop on Kaggle** with the notebook link. GPU time ends when you stop the notebook there, and at the latest at the run's time limit. AuraGo keeps following the notebook until Kaggle reports that it ended; then it ends the run as cancelled and downloads nothing but the log tail.
- A stop that arrives while the run is cleaning up changes nothing: the response shows the run unchanged, and the run ends as it was already going to. A stop that arrives before the run reaches cleaning ends it as `cancelled`, even when its results were just checked; files that were already downloaded stay with the run until you delete it.
- Cancel also works while the desktop is read-only.

### Results

A successful run stores `data/chibi_sensei/models/<run>/model-q8_0.gguf` (q8_0) and, for LoRA, the adapter, and shows a report card (loss before and after, and per task exact match, label accuracy or perplexity on the evaluation rows). A score that no evaluation row could be scored for shows `n/a`, not 0. The run's files list offers each file as a download. Deleting a finished run removes its files and records; it never touches Kaggle. With `delete_remote_after_run` and `allow_delete`, AuraGo deletes the Kaggle dataset and notebook after the run; otherwise, or when a delete fails, the run lists them as leftovers to delete on Kaggle. It lists a dataset or notebook only when Kaggle has it or it is unknown whether Kaggle created it; one that Kaggle refused to create is not listed.

### Settings

| Key | Default | Meaning |
|---|---|---|
| `chibi_sensei.training.machine_shape` | `NvidiaTeslaT4` | Kaggle GPU (`NvidiaTeslaT4` or `NvidiaL4`); must also be in `kaggle.allowed_machine_shapes` |
| `chibi_sensei.training.delete_remote_after_run` | `true` | delete dataset and notebook on Kaggle after the run (needs `kaggle.allow_delete`) |
| `kaggle.remote_timeout_minutes` | `60` | upper limit of a run's session; the preflight warns below 1.5 × estimate + 15 |
| `kaggle.max_transfer_mb`, `kaggle.max_storage_mb` | `2048`, `10240` | largest single download, and the Kaggle staging storage |
| `kaggle.max_remote_jobs` | `1` | Kaggle GPU runs at the same time (all of AuraGo) |
| `chibi_sensei.storage.max_gb` | `20` | database plus `data/chibi_sensei/` |

### When a run fails

| Code | Meaning | What to do |
|---|---|---|
| `env_no_gpu`, `env_unsupported_gpu` | the notebook got no usable GPU | check the machine shape and the allow-list; try again later |
| `env_no_internet` | the notebook had no internet | verify the Kaggle account by phone |
| `install_failed`, `base_download_failed` | pip or the model download failed | usually temporary: start again |
| `data_changed`, `data_mismatch` | the data changed after the start | start again |
| `oom` | one row is too long for the GPU's memory | lower `max_seq_len` or shorten the longest rows; the quick preset does not help when a single row is too long |
| `train_diverged` | the loss became NaN | lower the learning rate |
| `lora_targets_missing`, `merge_failed`, `gguf_failed`, `gguf_arch_mismatch` | the model could not be adapted, saved or converted | report it with the log tail; try another profile |
| `kaggle_submit_unknown` | AuraGo cannot tell whether the notebook started | look at the notebook on Kaggle and stop it if it runs |
| `remote_timeout` | the session passed its time limit | stop it on Kaggle; raise `kaggle.remote_timeout_minutes` or use `quick` |
| `status_missing`, `manifest_invalid`, `trainer_failed` | the run left no usable result | read the log tail |
| `download_failed`, `checksum_mismatch`, `artifact_too_large`, `storage_full` | a result could not be copied | check free disk, `kaggle.max_transfer_mb` and `chibi_sensei.storage.max_gb`; start again |
| `upload_failed`, `dataset_failed`, `dataset_timeout`, `submit_failed`, `kaggle_slot_busy` | Kaggle refused a step | check the Kaggle app's job list and the quota; wait for other GPU runs |
| `kaggle_unavailable` | Kaggle stayed unreachable for six hours before the notebook was running, or while the results were downloaded | wait, then start again |
| `desktop_readonly` | the read-only desktop stopped the run before the notebook was submitted | make the desktop writable and start again |
| `kaggle_refused` | a Kaggle setting the run needs was switched off before the notebook was submitted | switch the missing setting on in the Training step's list "Before you start" and start again |
| `profile_changed`, `packaging_failed`, `internal_error` | AuraGo could not build or continue the run | start again; report it with the log |
| `cancelled_by_user` | you cancelled | – |

The run can show these hints while it runs and after it has ended. A hint alone does not fail a run. `kaggle_unavailable`, `desktop_readonly`, `kaggle_refused` and `remote_timeout` also appear in the table above: the run fails with that code when the condition lasts too long. The meaning column gives the text the app shows (English locale).

| Code | Meaning |
|---|---|
| `time_budget` | Training stopped at its time budget. The model uses what it learned until then. |
| `base_gguf_skipped` | No time was left to convert the base model for comparisons. Your trained model is complete. |
| `base_gguf_invalid` | The base model file for comparisons could not be used. Your trained model is fine. |
| `remote_timeout` | The notebook ran until its time limit before it stopped. |
| `manifest_missing` | Kaggle sent no list of result files. |
| `cleanup_failed` | Cleaning up on Kaggle did not work. Some items may have stayed on your account. |
| `kaggle_unavailable` | Kaggle cannot be reached right now. AuraGo keeps trying. |
| `desktop_readonly` | The desktop is in read-only mode. The run waits until changes are allowed again. |
| `kaggle_refused` | A Kaggle setting this run needs is switched off. The run waits until it is switched on again. |
| `staged_left` | Some temporary files of this run could not be removed from AuraGo's Kaggle folder on this computer. |
| `batch_reduced` | The graphics card ran out of memory with the planned batch size. Training continues with smaller batches and takes a little longer. |
| `eval_batch_reduced` | The graphics card ran out of memory while measuring quality. The measurement continues one row at a time. |
| `oom_step_skipped` | One training step ran out of memory and was skipped. Very long rows can cause this. |
| `rows_truncated` | Some rows were longer than the maximum length and were cut off at the end. |

A run that waits for Kaggle or the desktop shows a note for `kaggle_unavailable`, `desktop_readonly` or `kaggle_refused` until the wait ends. `cleanup_failed` means a Kaggle dataset or notebook could not be deleted; the run then lists it as a leftover. `staged_left` means temporary files of the run could not be removed from AuraGo's local Kaggle project folder after five attempts; the warning names the files. Cleaning retries those removals and waits while a local transfer of the run is active or cannot be read. Neither warning changes the result.

## Storage

The dataset lives in `data/chibi_sensei.db` (SQLite), which is part of the backup inventory. Training runs add files under `data/chibi_sensei/`: `runs/<run>/` (the frozen split files, the notebook sent to Kaggle, status, manifest, metrics, evaluation samples and the log tail), `models/<run>/` (GGUF and adapter) and `base/` (one GGUF per base model, revision and converter, shared by all runs). A stored base GGUF whose size and sha256 match its record is authoritative: a run whose manifest declares another hash fails with `checksum_mismatch` and the stored file stays as it is. A run removes a base GGUF it found invalid only when it stored that file itself and no other run uses it. `chibi_sensei.storage.max_gb` (default 20) caps the database (with its write-ahead log) and these files together; the preflight checks free disk and the cap against 1.2 × the expected download. **`data/chibi_sensei/` is not part of AuraGo backups: copy a model you want to keep.** Deleting a project removes its rows, history, documents, checks, jobs, suggestions and runs with their files, and is refused while one of its runs is active. Deleting a finished run removes its files and records; base GGUFs stay. Neither touches Kaggle.

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

Without a training run nothing leaves the machine except the calls to the configured generator route during generation and cleaning. For those calls the app treats your rows and documents as material, never as instructions: they are fenced in `<external_data>` tags and the model is told not to follow anything inside them, and the composed prompt passes the injection scan before it is sent. Your goal text and your own instructions are your task and are not fenced. Model answers are parsed strictly against a schema and checked row by row before anything is stored. Keys and provider error texts never reach job results, events or responses.

A training run sends the frozen split files to a private Kaggle dataset and the trainer to a private Kaggle notebook, only after you confirmed the start (see [Training on Kaggle](#training-on-kaggle)). The Kaggle token stays in the Vault; no secret, host path or Hugging Face token enters `job.json`, the notebook, the logs or the events, and Kaggle log text is scrubbed before AuraGo stores it. Result files get names AuraGo generates and are kept only when size and sha256 match the run's manifest.
