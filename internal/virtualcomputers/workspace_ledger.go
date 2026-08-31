package virtualcomputers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func (l *Ledger) UpsertWorkspace(ctx context.Context, workspace Workspace) error {
	if l == nil || l.db == nil {
		return fmt.Errorf("virtual computers ledger is not open")
	}
	if strings.TrimSpace(workspace.ID) == "" || strings.TrimSpace(workspace.OwnerSessionID) == "" || strings.TrimSpace(workspace.MachineID) == "" {
		return fmt.Errorf("workspace id, owner session id, and machine id are required")
	}
	now := time.Now().UTC()
	if workspace.CreatedAt.IsZero() {
		workspace.CreatedAt = now
	}
	if workspace.UpdatedAt.IsZero() {
		workspace.UpdatedAt = now
	}
	if workspace.LastActivityAt.IsZero() {
		workspace.LastActivityAt = workspace.UpdatedAt
	}
	if workspace.ControlOwner == "" {
		workspace.ControlOwner = ControlOwnerAgent
	}
	capabilities, err := json.Marshal(workspace.Capabilities)
	if err != nil {
		return fmt.Errorf("encode workspace capabilities: %w", err)
	}
	_, err = l.db.ExecContext(ctx, `INSERT INTO workspaces
		(id, owner_session_id, mission_id, actor, machine_id, state, template, network_profile, volume_id,
		 capabilities_json, instance_nonce, control_owner, control_lease_expires_at, last_error,
		 created_at, updated_at, last_activity_at, lease_expires_at, max_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			owner_session_id = excluded.owner_session_id,
			mission_id = excluded.mission_id,
			actor = excluded.actor,
			machine_id = excluded.machine_id,
			state = excluded.state,
			template = excluded.template,
			network_profile = excluded.network_profile,
			volume_id = excluded.volume_id,
			capabilities_json = excluded.capabilities_json,
			instance_nonce = excluded.instance_nonce,
			control_owner = excluded.control_owner,
			control_lease_expires_at = excluded.control_lease_expires_at,
			last_error = excluded.last_error,
			updated_at = excluded.updated_at,
			last_activity_at = excluded.last_activity_at,
			lease_expires_at = excluded.lease_expires_at,
			max_expires_at = excluded.max_expires_at`,
		workspace.ID, workspace.OwnerSessionID, workspace.MissionID, workspace.Actor, workspace.MachineID,
		workspace.State, workspace.Template, workspace.NetworkProfile, workspace.VolumeID, string(capabilities),
		workspace.InstanceNonce, workspace.ControlOwner, timePtrText(workspace.ControlLeaseExpiresAt), workspace.LastError,
		timeText(workspace.CreatedAt), timeText(workspace.UpdatedAt), timeText(workspace.LastActivityAt),
		timeText(workspace.LeaseExpiresAt), timeText(workspace.MaxExpiresAt))
	if err != nil {
		return fmt.Errorf("upsert virtual workspace: %w", err)
	}
	return nil
}

func (l *Ledger) GetWorkspace(ctx context.Context, id string) (Workspace, bool, error) {
	if l == nil || l.db == nil {
		return Workspace{}, false, fmt.Errorf("virtual computers ledger is not open")
	}
	row := l.db.QueryRowContext(ctx, `SELECT id, owner_session_id, mission_id, actor, machine_id, state, template,
		network_profile, volume_id, capabilities_json, instance_nonce, control_owner, control_lease_expires_at,
		last_error, created_at, updated_at, last_activity_at, lease_expires_at, max_expires_at
		FROM workspaces WHERE id = ?`, strings.TrimSpace(id))
	workspace, err := scanWorkspace(row)
	if err == sql.ErrNoRows {
		return Workspace{}, false, nil
	}
	if err != nil {
		return Workspace{}, false, fmt.Errorf("get virtual workspace: %w", err)
	}
	return workspace, true, nil
}

func (l *Ledger) ListWorkspaces(ctx context.Context, ownerSessionID string, includeClosed bool, limit int) ([]Workspace, error) {
	if l == nil || l.db == nil {
		return nil, fmt.Errorf("virtual computers ledger is not open")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	query := `SELECT id, owner_session_id, mission_id, actor, machine_id, state, template,
		network_profile, volume_id, capabilities_json, instance_nonce, control_owner, control_lease_expires_at,
		last_error, created_at, updated_at, last_activity_at, lease_expires_at, max_expires_at FROM workspaces`
	args := make([]interface{}, 0, 3)
	clauses := make([]string, 0, 2)
	if strings.TrimSpace(ownerSessionID) != "" {
		clauses = append(clauses, "owner_session_id = ?")
		args = append(args, strings.TrimSpace(ownerSessionID))
	}
	if !includeClosed {
		clauses = append(clauses, "state NOT IN (?, ?)")
		args = append(args, WorkspaceStateClosed, WorkspaceStateFailed)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := l.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list virtual workspaces: %w", err)
	}
	defer rows.Close()
	workspaces := make([]Workspace, 0)
	for rows.Next() {
		workspace, err := scanWorkspace(rows)
		if err != nil {
			return nil, fmt.Errorf("scan virtual workspace: %w", err)
		}
		workspaces = append(workspaces, workspace)
	}
	return workspaces, rows.Err()
}

func (l *Ledger) CountActiveWorkspaces(ctx context.Context) (int, error) {
	var count int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workspaces WHERE state IN (?, ?, ?)`,
		WorkspaceStateOpening, WorkspaceStateReady, WorkspaceStateClosing).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active virtual workspaces: %w", err)
	}
	return count, nil
}

type workspaceScanner interface {
	Scan(dest ...interface{}) error
}

func scanWorkspace(scanner workspaceScanner) (Workspace, error) {
	var workspace Workspace
	var capabilitiesJSON, controlLease, created, updated, activity, lease, maxExpires string
	if err := scanner.Scan(
		&workspace.ID, &workspace.OwnerSessionID, &workspace.MissionID, &workspace.Actor, &workspace.MachineID,
		&workspace.State, &workspace.Template, &workspace.NetworkProfile, &workspace.VolumeID, &capabilitiesJSON,
		&workspace.InstanceNonce, &workspace.ControlOwner, &controlLease, &workspace.LastError,
		&created, &updated, &activity, &lease, &maxExpires,
	); err != nil {
		return Workspace{}, err
	}
	_ = json.Unmarshal([]byte(capabilitiesJSON), &workspace.Capabilities)
	workspace.ControlLeaseExpiresAt = parseStoredOptionalTime(controlLease)
	workspace.CreatedAt = parseStoredTime(created)
	workspace.UpdatedAt = parseStoredTime(updated)
	workspace.LastActivityAt = parseStoredTime(activity)
	workspace.LeaseExpiresAt = parseStoredTime(lease)
	workspace.MaxExpiresAt = parseStoredTime(maxExpires)
	return workspace, nil
}

func (l *Ledger) UpsertWorkspaceJob(ctx context.Context, job WorkspaceJob) error {
	if l == nil || l.db == nil {
		return fmt.Errorf("virtual computers ledger is not open")
	}
	if strings.TrimSpace(job.ID) == "" || strings.TrimSpace(job.WorkspaceID) == "" {
		return fmt.Errorf("workspace job id and workspace id are required")
	}
	now := time.Now().UTC()
	if job.CreatedAt.IsZero() {
		job.CreatedAt = now
	}
	if job.UpdatedAt.IsZero() {
		job.UpdatedAt = now
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO workspace_jobs
		(id, workspace_id, mode, state, command_hash, command_summary, exit_code, pid, process_group,
		 output_cursor, output_truncated, error, created_at, updated_at, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			state = excluded.state,
			exit_code = excluded.exit_code,
			pid = excluded.pid,
			process_group = excluded.process_group,
			output_cursor = excluded.output_cursor,
			output_truncated = excluded.output_truncated,
			error = excluded.error,
			updated_at = excluded.updated_at,
			started_at = excluded.started_at,
			completed_at = excluded.completed_at`,
		job.ID, job.WorkspaceID, job.Mode, job.State, job.CommandHash, job.CommandSummary, nullableInt(job.ExitCode),
		job.PID, job.ProcessGroup, job.OutputCursor, boolInt(job.OutputTruncated), job.Error,
		timeText(job.CreatedAt), timeText(job.UpdatedAt), timePtrText(job.StartedAt), timePtrText(job.CompletedAt))
	if err != nil {
		return fmt.Errorf("upsert virtual workspace job: %w", err)
	}
	return nil
}

func (l *Ledger) GetWorkspaceJob(ctx context.Context, id string) (WorkspaceJob, bool, error) {
	row := l.db.QueryRowContext(ctx, `SELECT id, workspace_id, mode, state, command_hash, command_summary,
		exit_code, pid, process_group, output_cursor, output_truncated, error, created_at, updated_at, started_at, completed_at
		FROM workspace_jobs WHERE id = ?`, strings.TrimSpace(id))
	job, err := scanWorkspaceJob(row)
	if err == sql.ErrNoRows {
		return WorkspaceJob{}, false, nil
	}
	if err != nil {
		return WorkspaceJob{}, false, fmt.Errorf("get virtual workspace job: %w", err)
	}
	return job, true, nil
}

func (l *Ledger) ListWorkspaceJobs(ctx context.Context, workspaceID string, limit int) ([]WorkspaceJob, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := l.db.QueryContext(ctx, `SELECT id, workspace_id, mode, state, command_hash, command_summary,
		exit_code, pid, process_group, output_cursor, output_truncated, error, created_at, updated_at, started_at, completed_at
		FROM workspace_jobs WHERE workspace_id = ? ORDER BY created_at DESC LIMIT ?`, strings.TrimSpace(workspaceID), limit)
	if err != nil {
		return nil, fmt.Errorf("list virtual workspace jobs: %w", err)
	}
	defer rows.Close()
	jobs := make([]WorkspaceJob, 0)
	for rows.Next() {
		job, err := scanWorkspaceJob(rows)
		if err != nil {
			return nil, fmt.Errorf("scan virtual workspace job: %w", err)
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (l *Ledger) CountActiveWorkspaceJobs(ctx context.Context, workspaceID string) (int, error) {
	var count int
	if err := l.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workspace_jobs WHERE workspace_id = ? AND state IN (?, ?)`,
		strings.TrimSpace(workspaceID), JobStateQueued, JobStateRunning).Scan(&count); err != nil {
		return 0, fmt.Errorf("count active virtual workspace jobs: %w", err)
	}
	return count, nil
}

func scanWorkspaceJob(scanner workspaceScanner) (WorkspaceJob, error) {
	var job WorkspaceJob
	var exitCode sql.NullInt64
	var truncated int
	var created, updated, started, completed string
	if err := scanner.Scan(&job.ID, &job.WorkspaceID, &job.Mode, &job.State, &job.CommandHash, &job.CommandSummary,
		&exitCode, &job.PID, &job.ProcessGroup, &job.OutputCursor, &truncated, &job.Error,
		&created, &updated, &started, &completed); err != nil {
		return WorkspaceJob{}, err
	}
	if exitCode.Valid {
		value := int(exitCode.Int64)
		job.ExitCode = &value
	}
	job.OutputTruncated = truncated != 0
	job.CreatedAt = parseStoredTime(created)
	job.UpdatedAt = parseStoredTime(updated)
	job.StartedAt = parseStoredOptionalTime(started)
	job.CompletedAt = parseStoredOptionalTime(completed)
	return job, nil
}

func (l *Ledger) InterruptActiveWorkspaceJobs(ctx context.Context, workspaceID, reason string) error {
	now := nowText()
	query := `UPDATE workspace_jobs SET state = ?, error = ?, updated_at = ?, completed_at = ? WHERE state IN (?, ?)`
	args := []interface{}{JobStateInterrupted, reason, now, now, JobStateQueued, JobStateRunning}
	if strings.TrimSpace(workspaceID) != "" {
		query += " AND workspace_id = ?"
		args = append(args, strings.TrimSpace(workspaceID))
	}
	if _, err := l.db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("interrupt virtual workspace jobs: %w", err)
	}
	return nil
}

func (l *Ledger) UpsertBrowserSession(ctx context.Context, session BrowserSession) error {
	if strings.TrimSpace(session.ID) == "" || strings.TrimSpace(session.WorkspaceID) == "" {
		return fmt.Errorf("browser session id and workspace id are required")
	}
	now := time.Now().UTC()
	if session.CreatedAt.IsZero() {
		session.CreatedAt = now
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = now
	}
	if session.ControlOwner == "" {
		session.ControlOwner = ControlOwnerAgent
	}
	_, err := l.db.ExecContext(ctx, `INSERT INTO workspace_browser_sessions
		(id, workspace_id, state, active_page_id, url_origin, control_owner, control_lease_expires_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET state = excluded.state, active_page_id = excluded.active_page_id,
			url_origin = excluded.url_origin, control_owner = excluded.control_owner,
			control_lease_expires_at = excluded.control_lease_expires_at, updated_at = excluded.updated_at`,
		session.ID, session.WorkspaceID, session.State, session.ActivePageID, session.URLOrigin, session.ControlOwner,
		timePtrText(session.ControlLeaseExpiresAt), timeText(session.CreatedAt), timeText(session.UpdatedAt))
	if err != nil {
		return fmt.Errorf("upsert virtual browser session: %w", err)
	}
	return nil
}

func (l *Ledger) ListBrowserSessions(ctx context.Context, workspaceID string) ([]BrowserSession, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT id, workspace_id, state, active_page_id, url_origin, control_owner,
		control_lease_expires_at, created_at, updated_at FROM workspace_browser_sessions
		WHERE workspace_id = ? ORDER BY created_at DESC`, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("list virtual browser sessions: %w", err)
	}
	defer rows.Close()
	sessions := make([]BrowserSession, 0)
	for rows.Next() {
		var session BrowserSession
		var controlLease, created, updated string
		if err := rows.Scan(&session.ID, &session.WorkspaceID, &session.State, &session.ActivePageID, &session.URLOrigin,
			&session.ControlOwner, &controlLease, &created, &updated); err != nil {
			return nil, fmt.Errorf("scan virtual browser session: %w", err)
		}
		session.ControlLeaseExpiresAt = parseStoredOptionalTime(controlLease)
		session.CreatedAt = parseStoredTime(created)
		session.UpdatedAt = parseStoredTime(updated)
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (l *Ledger) UpsertCredentialGrant(ctx context.Context, grant CredentialGrant) error {
	if strings.TrimSpace(grant.ID) == "" || strings.TrimSpace(grant.WorkspaceID) == "" || strings.TrimSpace(grant.CredentialID) == "" {
		return fmt.Errorf("grant id, workspace id, and credential id are required")
	}
	now := time.Now().UTC()
	if grant.CreatedAt.IsZero() {
		grant.CreatedAt = now
	}
	if grant.UpdatedAt.IsZero() {
		grant.UpdatedAt = now
	}
	fields, err := json.Marshal(grant.FieldNames)
	if err != nil {
		return fmt.Errorf("encode credential grant fields: %w", err)
	}
	_, err = l.db.ExecContext(ctx, `INSERT INTO workspace_credential_grants
		(id, workspace_id, credential_id, usage_type, origin, job_id, field_names_json, purpose, status,
		 requested_by, approved_by, created_at, updated_at, expires_at, revoked_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET status = excluded.status, approved_by = excluded.approved_by,
			updated_at = excluded.updated_at, expires_at = excluded.expires_at, revoked_at = excluded.revoked_at`,
		grant.ID, grant.WorkspaceID, grant.CredentialID, grant.UsageType, grant.Origin, grant.JobID, string(fields),
		grant.Purpose, grant.Status, grant.RequestedBy, grant.ApprovedBy, timeText(grant.CreatedAt),
		timeText(grant.UpdatedAt), timeText(grant.ExpiresAt), timePtrText(grant.RevokedAt))
	if err != nil {
		return fmt.Errorf("upsert virtual workspace credential grant: %w", err)
	}
	return nil
}

func (l *Ledger) GetCredentialGrant(ctx context.Context, id string) (CredentialGrant, bool, error) {
	row := l.db.QueryRowContext(ctx, `SELECT id, workspace_id, credential_id, usage_type, origin, job_id,
		field_names_json, purpose, status, requested_by, approved_by, created_at, updated_at, expires_at, revoked_at
		FROM workspace_credential_grants WHERE id = ?`, strings.TrimSpace(id))
	grant, err := scanCredentialGrant(row)
	if err == sql.ErrNoRows {
		return CredentialGrant{}, false, nil
	}
	if err != nil {
		return CredentialGrant{}, false, fmt.Errorf("get virtual workspace credential grant: %w", err)
	}
	return grant, true, nil
}

func (l *Ledger) ListCredentialGrants(ctx context.Context, workspaceID string) ([]CredentialGrant, error) {
	rows, err := l.db.QueryContext(ctx, `SELECT id, workspace_id, credential_id, usage_type, origin, job_id,
		field_names_json, purpose, status, requested_by, approved_by, created_at, updated_at, expires_at, revoked_at
		FROM workspace_credential_grants WHERE workspace_id = ? ORDER BY created_at DESC`, strings.TrimSpace(workspaceID))
	if err != nil {
		return nil, fmt.Errorf("list virtual workspace credential grants: %w", err)
	}
	defer rows.Close()
	grants := make([]CredentialGrant, 0)
	for rows.Next() {
		grant, err := scanCredentialGrant(rows)
		if err != nil {
			return nil, fmt.Errorf("scan virtual workspace credential grant: %w", err)
		}
		grants = append(grants, grant)
	}
	return grants, rows.Err()
}

func scanCredentialGrant(scanner workspaceScanner) (CredentialGrant, error) {
	var grant CredentialGrant
	var fieldsJSON, created, updated, expires, revoked string
	if err := scanner.Scan(&grant.ID, &grant.WorkspaceID, &grant.CredentialID, &grant.UsageType, &grant.Origin,
		&grant.JobID, &fieldsJSON, &grant.Purpose, &grant.Status, &grant.RequestedBy, &grant.ApprovedBy,
		&created, &updated, &expires, &revoked); err != nil {
		return CredentialGrant{}, err
	}
	_ = json.Unmarshal([]byte(fieldsJSON), &grant.FieldNames)
	grant.CreatedAt = parseStoredTime(created)
	grant.UpdatedAt = parseStoredTime(updated)
	grant.ExpiresAt = parseStoredTime(expires)
	grant.RevokedAt = parseStoredOptionalTime(revoked)
	return grant, nil
}

func (l *Ledger) ExpireCredentialGrants(ctx context.Context, now time.Time) error {
	if _, err := l.db.ExecContext(ctx, `UPDATE workspace_credential_grants SET status = ?, updated_at = ?
		WHERE status IN (?, ?) AND expires_at <= ?`, GrantExpired, timeText(now), GrantPending, GrantActive, timeText(now)); err != nil {
		return fmt.Errorf("expire virtual workspace credential grants: %w", err)
	}
	return nil
}

func (l *Ledger) CompleteJobCredentialGrants(ctx context.Context, workspaceID, jobID, status string, now time.Time) error {
	if status != GrantConsumed && status != GrantRevoked && status != GrantExpired {
		return fmt.Errorf("invalid terminal credential grant status %q", status)
	}
	if _, err := l.db.ExecContext(ctx, `UPDATE workspace_credential_grants SET status = ?, updated_at = ?
		WHERE workspace_id = ? AND job_id = ? AND usage_type = ? AND status IN (?, ?)`,
		status, timeText(now), strings.TrimSpace(workspaceID), strings.TrimSpace(jobID), GrantUsageShell, GrantPending, GrantActive); err != nil {
		return fmt.Errorf("complete shell credential grants: %w", err)
	}
	return nil
}

func (l *Ledger) RecordWorkspaceEvent(ctx context.Context, event WorkspaceEvent) error {
	if strings.TrimSpace(event.WorkspaceID) == "" || strings.TrimSpace(event.Type) == "" {
		return fmt.Errorf("workspace event workspace id and type are required")
	}
	if event.CreatedAt.IsZero() {
		event.CreatedAt = time.Now().UTC()
	}
	metadata, err := json.Marshal(event.Metadata)
	if err != nil {
		return fmt.Errorf("encode workspace event metadata: %w", err)
	}
	if _, err := l.db.ExecContext(ctx, `INSERT INTO workspace_events(workspace_id, event_type, summary, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?)`, event.WorkspaceID, event.Type, event.Summary, string(metadata), timeText(event.CreatedAt)); err != nil {
		return fmt.Errorf("record virtual workspace event: %w", err)
	}
	return nil
}

func (l *Ledger) ListWorkspaceEvents(ctx context.Context, workspaceID string, afterID int64, limit int) ([]WorkspaceEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := l.db.QueryContext(ctx, `SELECT id, workspace_id, event_type, summary, metadata_json, created_at
		FROM workspace_events WHERE workspace_id = ? AND id > ? ORDER BY id ASC LIMIT ?`, strings.TrimSpace(workspaceID), afterID, limit)
	if err != nil {
		return nil, fmt.Errorf("list virtual workspace events: %w", err)
	}
	defer rows.Close()
	events := make([]WorkspaceEvent, 0)
	for rows.Next() {
		var event WorkspaceEvent
		var metadataJSON, created string
		if err := rows.Scan(&event.ID, &event.WorkspaceID, &event.Type, &event.Summary, &metadataJSON, &created); err != nil {
			return nil, fmt.Errorf("scan virtual workspace event: %w", err)
		}
		_ = json.Unmarshal([]byte(metadataJSON), &event.Metadata)
		event.CreatedAt = parseStoredTime(created)
		events = append(events, event)
	}
	return events, rows.Err()
}

func nullableInt(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
