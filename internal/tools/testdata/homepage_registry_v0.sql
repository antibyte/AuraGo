CREATE TABLE IF NOT EXISTS homepage_projects (
		id                INTEGER PRIMARY KEY AUTOINCREMENT,
		created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
		name              TEXT NOT NULL UNIQUE,
		description       TEXT DEFAULT '',
		framework         TEXT DEFAULT '',
		framework_version TEXT DEFAULT '',
		url               TEXT DEFAULT '',
		netlify_site_id   TEXT DEFAULT '',
		deploy_host       TEXT DEFAULT '',
		project_dir       TEXT NOT NULL DEFAULT '',
		status            TEXT NOT NULL DEFAULT 'active',
		tags              TEXT DEFAULT '[]',
		last_edited_at    DATETIME DEFAULT NULL,
		last_edit_reason  TEXT DEFAULT '',
		last_deployed_at  DATETIME DEFAULT NULL,
		last_deploy_url   TEXT DEFAULT '',
		known_problems    TEXT DEFAULT '',
		lighthouse_score  TEXT DEFAULT '',
		notes             TEXT DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_hp_name ON homepage_projects(name);
	CREATE INDEX IF NOT EXISTS idx_hp_status ON homepage_projects(status);
	CREATE INDEX IF NOT EXISTS idx_hp_project_dir ON homepage_projects(project_dir);
	CREATE INDEX IF NOT EXISTS idx_hp_last_edited ON homepage_projects(last_edited_at);

	CREATE TABLE IF NOT EXISTS homepage_revisions (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id     INTEGER DEFAULT NULL,
		project_dir    TEXT NOT NULL,
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
		message        TEXT DEFAULT '',
		reason         TEXT DEFAULT '',
		author         TEXT DEFAULT 'agent',
		file_count     INTEGER DEFAULT 0,
		restorable     INTEGER DEFAULT 1,
		metadata_json  TEXT DEFAULT '',
		FOREIGN KEY (project_id) REFERENCES homepage_projects(id) ON DELETE SET NULL
	);
	CREATE INDEX IF NOT EXISTS idx_hr_project_dir ON homepage_revisions(project_dir);
	CREATE INDEX IF NOT EXISTS idx_hr_created_at ON homepage_revisions(created_at);
	CREATE INDEX IF NOT EXISTS idx_hr_project_id ON homepage_revisions(project_id);

	CREATE TABLE IF NOT EXISTS homepage_revision_files (
		id                  INTEGER PRIMARY KEY AUTOINCREMENT,
		revision_id         INTEGER NOT NULL,
		path                TEXT NOT NULL,
		change_type         TEXT NOT NULL,
		content_before      TEXT DEFAULT '',
		content_after       TEXT DEFAULT '',
		content_hash_before TEXT DEFAULT '',
		content_hash_after  TEXT DEFAULT '',
		size_before         INTEGER DEFAULT 0,
		size_after          INTEGER DEFAULT 0,
		FOREIGN KEY (revision_id) REFERENCES homepage_revisions(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_hrf_revision_id ON homepage_revision_files(revision_id);
	CREATE INDEX IF NOT EXISTS idx_hrf_path ON homepage_revision_files(path);

	CREATE TABLE IF NOT EXISTS homepage_history (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id  INTEGER NOT NULL,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
		author      TEXT DEFAULT 'agent',
		entry_type  TEXT NOT NULL DEFAULT 'note',
		content     TEXT NOT NULL,
		tags        TEXT DEFAULT '[]',
		source      TEXT DEFAULT '',
		FOREIGN KEY (project_id) REFERENCES homepage_projects(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_hh_project_id ON homepage_history(project_id);
	CREATE INDEX IF NOT EXISTS idx_hh_created_at ON homepage_history(created_at);
	CREATE INDEX IF NOT EXISTS idx_hh_entry_type ON homepage_history(entry_type);

	CREATE TABLE IF NOT EXISTS homepage_project_state (
		project_id          INTEGER PRIMARY KEY,
		local_root          TEXT NOT NULL DEFAULT '',
		current_revision_id INTEGER DEFAULT NULL,
		git_sha             TEXT DEFAULT '',
		last_reconciled_at  DATETIME DEFAULT NULL,
		drift_status        TEXT NOT NULL DEFAULT 'not_deployed',
		drift_message       TEXT DEFAULT '',
		updated_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (project_id) REFERENCES homepage_projects(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_hps_drift_status ON homepage_project_state(drift_status);

	CREATE TABLE IF NOT EXISTS homepage_site_file_state (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id         INTEGER NOT NULL,
		rel_path           TEXT NOT NULL,
		content_hash       TEXT NOT NULL,
		size_bytes         INTEGER DEFAULT 0,
		mod_time           DATETIME DEFAULT NULL,
		is_binary          INTEGER DEFAULT 0,
		last_revision_id   INTEGER DEFAULT NULL,
		last_observed_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(project_id, rel_path),
		FOREIGN KEY (project_id) REFERENCES homepage_projects(id) ON DELETE CASCADE,
		FOREIGN KEY (last_revision_id) REFERENCES homepage_revisions(id) ON DELETE SET NULL
	);
	CREATE INDEX IF NOT EXISTS idx_hsfs_project_id ON homepage_site_file_state(project_id);
	CREATE INDEX IF NOT EXISTS idx_hsfs_hash ON homepage_site_file_state(content_hash);

	CREATE TABLE IF NOT EXISTS homepage_site_events (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id     INTEGER NOT NULL,
		created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
		actor          TEXT NOT NULL DEFAULT 'agent',
		source         TEXT DEFAULT '',
		event_type     TEXT NOT NULL,
		summary        TEXT NOT NULL,
		correlation_id TEXT DEFAULT '',
		payload_json   TEXT DEFAULT '',
		FOREIGN KEY (project_id) REFERENCES homepage_projects(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_hse_project_id ON homepage_site_events(project_id);
	CREATE INDEX IF NOT EXISTS idx_hse_event_type ON homepage_site_events(event_type);
	CREATE INDEX IF NOT EXISTS idx_hse_created_at ON homepage_site_events(created_at);

	CREATE TABLE IF NOT EXISTS homepage_deploy_targets (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id         INTEGER NOT NULL,
		provider           TEXT NOT NULL,
		provider_target_id TEXT DEFAULT '',
		url                TEXT DEFAULT '',
		remote_path        TEXT DEFAULT '',
		metadata_json      TEXT DEFAULT '',
		last_seen_at       DATETIME DEFAULT NULL,
		updated_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(project_id, provider),
		FOREIGN KEY (project_id) REFERENCES homepage_projects(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_hdt_project_id ON homepage_deploy_targets(project_id);
	CREATE INDEX IF NOT EXISTS idx_hdt_provider ON homepage_deploy_targets(provider);

	CREATE TABLE IF NOT EXISTS homepage_deployments (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id         INTEGER NOT NULL,
		target_id          INTEGER DEFAULT NULL,
		created_at         DATETIME DEFAULT CURRENT_TIMESTAMP,
		revision_id        INTEGER DEFAULT NULL,
		git_sha            TEXT DEFAULT '',
		provider           TEXT NOT NULL,
		provider_deploy_id TEXT DEFAULT '',
		url                TEXT DEFAULT '',
		build_dir          TEXT DEFAULT '',
		artifact_hash      TEXT DEFAULT '',
		status             TEXT NOT NULL DEFAULT 'unknown',
		verification_json  TEXT DEFAULT '',
		metadata_json      TEXT DEFAULT '',
		FOREIGN KEY (project_id) REFERENCES homepage_projects(id) ON DELETE CASCADE,
		FOREIGN KEY (target_id) REFERENCES homepage_deploy_targets(id) ON DELETE SET NULL,
		FOREIGN KEY (revision_id) REFERENCES homepage_revisions(id) ON DELETE SET NULL
	);
	CREATE INDEX IF NOT EXISTS idx_hd_project_id ON homepage_deployments(project_id);
	CREATE INDEX IF NOT EXISTS idx_hd_provider ON homepage_deployments(provider);
	CREATE INDEX IF NOT EXISTS idx_hd_created_at ON homepage_deployments(created_at);

	CREATE TABLE IF NOT EXISTS homepage_remote_observations (
		id                 INTEGER PRIMARY KEY AUTOINCREMENT,
		project_id         INTEGER NOT NULL,
		target_id          INTEGER DEFAULT NULL,
		observed_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
		provider           TEXT NOT NULL,
		url                TEXT DEFAULT '',
		provider_deploy_id TEXT DEFAULT '',
		content_hash       TEXT DEFAULT '',
		status             TEXT NOT NULL DEFAULT 'unknown',
		payload_json       TEXT DEFAULT '',
		FOREIGN KEY (project_id) REFERENCES homepage_projects(id) ON DELETE CASCADE,
		FOREIGN KEY (target_id) REFERENCES homepage_deploy_targets(id) ON DELETE SET NULL
	);
	CREATE INDEX IF NOT EXISTS idx_hro_project_id ON homepage_remote_observations(project_id);
	CREATE INDEX IF NOT EXISTS idx_hro_target_id ON homepage_remote_observations(target_id);
	CREATE INDEX IF NOT EXISTS idx_hro_observed_at ON homepage_remote_observations(observed_at);
