package chibisensei

const schemaVersion = 1

const schemaSQL = `PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000;
CREATE TABLE IF NOT EXISTS schema_version(version INTEGER NOT NULL);
INSERT INTO schema_version SELECT 1 WHERE NOT EXISTS(SELECT 1 FROM schema_version);
CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS projects(id TEXT PRIMARY KEY, name TEXT NOT NULL, recipe TEXT NOT NULL,
  task_kind TEXT NOT NULL DEFAULT '', labels_json TEXT NOT NULL DEFAULT '[]', goal_text TEXT NOT NULL DEFAULT '',
  profile_id TEXT NOT NULL DEFAULT '', preset TEXT NOT NULL DEFAULT 'balanced', overrides_json TEXT NOT NULL DEFAULT '{}',
  split_seed INTEGER NOT NULL, data_rev INTEGER NOT NULL DEFAULT 0, rev INTEGER NOT NULL DEFAULT 1,
  created TEXT NOT NULL, updated TEXT NOT NULL, archived INTEGER NOT NULL DEFAULT 0);
CREATE INDEX IF NOT EXISTS projects_list ON projects(archived, updated);
CREATE TABLE IF NOT EXISTS dataset_rows(id INTEGER PRIMARY KEY, project_id TEXT NOT NULL, seq INTEGER NOT NULL,
  kind TEXT NOT NULL, messages_json TEXT, text TEXT, label TEXT NOT NULL DEFAULT '', split TEXT NOT NULL DEFAULT 'auto',
  source TEXT NOT NULL, source_ref TEXT NOT NULL DEFAULT '', batch_id TEXT NOT NULL DEFAULT '', tags TEXT NOT NULL DEFAULT '[]',
  content_hash TEXT NOT NULL, prompt_hash TEXT NOT NULL DEFAULT '', est_tokens INTEGER NOT NULL,
  flags INTEGER NOT NULL DEFAULT 0, rev INTEGER NOT NULL DEFAULT 1, created TEXT NOT NULL, updated TEXT NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS rows_seq ON dataset_rows(project_id, seq);
CREATE INDEX IF NOT EXISTS rows_hash ON dataset_rows(project_id, content_hash);
CREATE INDEX IF NOT EXISTS rows_prompt ON dataset_rows(project_id, prompt_hash);
CREATE INDEX IF NOT EXISTS rows_split ON dataset_rows(project_id, split);
CREATE INDEX IF NOT EXISTS rows_batch ON dataset_rows(project_id, batch_id);
CREATE INDEX IF NOT EXISTS rows_tokens ON dataset_rows(project_id, est_tokens);
CREATE TABLE IF NOT EXISTS row_history(id INTEGER PRIMARY KEY, project_id TEXT NOT NULL, row_id INTEGER NOT NULL,
  batch_id TEXT NOT NULL, op TEXT NOT NULL, before_json TEXT, created TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS history_batch ON row_history(batch_id, id);
CREATE TABLE IF NOT EXISTS batches(batch_id TEXT PRIMARY KEY, project_id TEXT NOT NULL, kind TEXT NOT NULL,
  row_count INTEGER NOT NULL DEFAULT 0, undone INTEGER NOT NULL DEFAULT 0, created TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS batches_project ON batches(project_id, created);
CREATE TABLE IF NOT EXISTS documents(id TEXT PRIMARY KEY, project_id TEXT NOT NULL, name TEXT NOT NULL,
  workspace_path TEXT NOT NULL, sha256 TEXT NOT NULL, text TEXT NOT NULL, chars INTEGER NOT NULL, created TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS documents_project ON documents(project_id, created);
CREATE TABLE IF NOT EXISTS checks(project_id TEXT PRIMARY KEY, data_rev INTEGER NOT NULL, ready INTEGER NOT NULL DEFAULT 0,
  result_json TEXT NOT NULL, computed_at TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS jobs(id TEXT PRIMARY KEY, project_id TEXT NOT NULL, kind TEXT NOT NULL, state TEXT NOT NULL,
  params_json TEXT NOT NULL DEFAULT '{}', progress_json TEXT NOT NULL DEFAULT '{}', result_json TEXT NOT NULL DEFAULT '{}',
  error_code TEXT NOT NULL DEFAULT '', error TEXT NOT NULL DEFAULT '', created TEXT NOT NULL, updated TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS jobs_project ON jobs(project_id, created);
CREATE INDEX IF NOT EXISTS jobs_state ON jobs(state);
CREATE TABLE IF NOT EXISTS suggestions(job_id TEXT NOT NULL, row_id INTEGER NOT NULL, action TEXT NOT NULL,
  messages_json TEXT, text TEXT, reason TEXT NOT NULL DEFAULT '', row_rev INTEGER NOT NULL,
  applied INTEGER NOT NULL DEFAULT 0, PRIMARY KEY(job_id, row_id));
CREATE TABLE IF NOT EXISTS consents(profile_id TEXT, license_id TEXT, license_version TEXT, accepted_at TEXT,
  PRIMARY KEY(profile_id, license_id, license_version));
CREATE TABLE IF NOT EXISTS runs(id TEXT PRIMARY KEY, project_id TEXT, backend TEXT, state TEXT, step_attempt INTEGER,
  profile_id TEXT, profile_sha TEXT, preset TEXT, hyper_json TEXT, job_json TEXT, data_sha TEXT, remote_json TEXT,
  deadline TEXT, cancel_requested INTEGER DEFAULT 0, error_code TEXT, error TEXT, leftovers_json TEXT, created TEXT, updated TEXT);
CREATE TABLE IF NOT EXISTS run_events(id INTEGER PRIMARY KEY, run_id TEXT, ts TEXT, kind TEXT, data_json TEXT);
CREATE TABLE IF NOT EXISTS metrics(run_id TEXT, step INTEGER, kind TEXT, epoch REAL, loss REAL, lr REAL, grad_norm REAL,
  tok_s REAL, ts TEXT, PRIMARY KEY(run_id, step, kind));
CREATE TABLE IF NOT EXISTS artifacts(id TEXT PRIMARY KEY, run_id TEXT, project_id TEXT, kind TEXT, rel_path TEXT,
  bytes INTEGER, sha256 TEXT, created TEXT);
CREATE TABLE IF NOT EXISTS eval_reports(run_id TEXT, kind TEXT, result_json TEXT, created TEXT, PRIMARY KEY(run_id, kind));
CREATE TABLE IF NOT EXISTS base_gguf(profile_id TEXT, base_revision TEXT, converter TEXT, rel_path TEXT, sha256 TEXT,
  PRIMARY KEY(profile_id, base_revision, converter));`
